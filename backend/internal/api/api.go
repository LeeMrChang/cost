package api

import (
	"context"
	"cost/internal/auth"
	"cost/internal/domain"
	"cost/internal/store"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type API struct {
	profiles auth.ProfileRepository
	store    store.Store
	token    string
}

// New enables the explicit local-development token used by existing tests.
func New(s store.Store, token string) http.Handler { return newHandler(s, token, nil) }
func NewWithAuth(s store.Store, service *auth.Service) http.Handler {
	return newHandler(s, "", service)
}
func newHandler(s store.Store, token string, service *auth.Service) http.Handler {
	a := &API{store: s, token: token}
	if service != nil {
		a.profiles = service.Repo
	}
	mux := http.NewServeMux()
	if service != nil {
		service.Register(mux)
	}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]string{"status": "ok", "auth_mode": func() string {
			if service != nil {
				return "wechat"
			}
			return "local-development"
		}()})
	})
	mux.HandleFunc("GET /api/v1/categories", a.categories)
	mux.HandleFunc("POST /api/v1/categories", a.createCategory)
	mux.HandleFunc("PATCH /api/v1/categories/{id}", a.updateCategory)
	mux.HandleFunc("GET /api/v1/accounts", a.accounts)
	mux.HandleFunc("POST /api/v1/accounts", a.createAccount)
	mux.HandleFunc("PATCH /api/v1/accounts/{id}", a.updateAccount)
	mux.HandleFunc("GET /api/v1/transfers", a.transfers)
	mux.HandleFunc("POST /api/v1/transfers", a.createTransfer)
	mux.HandleFunc("DELETE /api/v1/transfers/{id}", a.deleteTransfer)
	mux.HandleFunc("GET /api/v1/entry-authors", a.authors)
	mux.HandleFunc("GET /api/v1/entries/export", a.exportEntries)
	mux.HandleFunc("GET /api/v1/entries", a.filteredList)
	mux.HandleFunc("POST /api/v1/entries", a.create)
	mux.HandleFunc("PATCH /api/v1/entries/{id}", a.update)
	mux.HandleFunc("DELETE /api/v1/entries/{id}", a.delete)
	mux.HandleFunc("GET /api/v1/stats", a.stats)
	mux.HandleFunc("PUT /api/v1/budgets/{month}", a.budget)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() { slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start)) }()
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic", "error", v)
				respond(w, 500, map[string]string{"error": "internal_error"})
			}
		}()
		if r.URL.Path != "/health" {
			if service != nil {
				if r.URL.Path != "/api/v1/auth/wechat" {
					p, err := service.Authenticate(r)
					if err != nil {
						status, code := 401, "unauthorized"
						if !errors.Is(err, auth.ErrUnauthorized) {
							status, code = 503, "authentication_unavailable"
						}
						respond(w, status, map[string]string{"error": code})
						return
					}
					r = r.WithContext(auth.WithPrincipal(r.Context(), p))
					if p.Role == "" && !strings.HasPrefix(r.URL.Path, "/api/v1/auth/") && !strings.HasPrefix(r.URL.Path, "/api/v1/family/") {
						respond(w, 403, map[string]string{"error": "family_access_required"})
						return
					}
				}
			} else {
				if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Dev-Token")), []byte(a.token)) != 1 {
					respond(w, 401, map[string]string{"error": "unauthorized"})
					return
				}
				r = r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{Role: "admin"}))
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func fail(w http.ResponseWriter, err error) {
	status, code := 500, "internal_error"
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status, code = 404, "not_found"
	case errors.Is(err, domain.ErrConflict):
		status, code = 409, "request_conflict"
	case errors.Is(err, domain.ErrInvalid):
		status, code = 400, "invalid_entry"
	}
	if status == 500 {
		slog.Error("storage error", "error", err)
	}
	respond(w, status, map[string]string{"error": code})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		respond(w, 400, map[string]string{"error": "invalid_json"})
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		respond(w, 400, map[string]string{"error": "invalid_json"})
		return false
	}
	return true
}
func (a *API) categories(w http.ResponseWriter, r *http.Request) {
	c, err := a.store.Categories(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, c)
}
func (a *API) monthEntries(w http.ResponseWriter, r *http.Request) ([]domain.Entry, string, error) {
	month := r.URL.Query().Get("month")
	if month == "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		month = time.Now().In(loc).Format("2006-01")
	}
	start, end, err := domain.Bounds(month)
	if err != nil {
		return nil, month, domain.ErrInvalid
	}
	e, err := a.store.List(r.Context(), start, end)
	return e, month, err
}
func (a *API) valid(r *http.Request, e domain.Entry) error {
	c, err := a.store.Categories(r.Context())
	if err != nil {
		return err
	}
	return domain.Validate(e, c)
}
func (a *API) create(w http.ResponseWriter, r *http.Request) {
	var e domain.Entry
	if !decode(w, r, &e) {
		return
	}
	e.ID = 0
	e.CreatedBy = auth.Current(r.Context()).ID
	e.OccurredAt = e.OccurredAt.Truncate(time.Microsecond)
	if err := a.valid(r, e); err != nil {
		fail(w, err)
		return
	}
	out, created, err := a.store.Save(r.Context(), e)
	if err != nil {
		fail(w, err)
		return
	}
	status := 200
	if created {
		status = 201
	}
	respond(w, status, out)
}
func id(r *http.Request) (int64, error) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || n <= 0 {
		return 0, domain.ErrInvalid
	}
	return n, nil
}
func (a *API) update(w http.ResponseWriter, r *http.Request) {
	var e domain.Entry
	if !decode(w, r, &e) {
		return
	}
	n, err := id(r)
	if err != nil {
		fail(w, err)
		return
	}
	old, err := a.store.Get(r.Context(), n)
	if err != nil {
		fail(w, err)
		return
	}
	if !canEdit(r, old) {
		respond(w, 403, map[string]string{"error": "entry_permission_denied"})
		return
	}
	e.CreatedBy = old.CreatedBy
	e.ID = n
	e.OccurredAt = e.OccurredAt.Truncate(time.Microsecond)
	e.RequestID = "update"
	if err = a.valid(r, e); err != nil {
		fail(w, err)
		return
	}
	out, err := a.store.Update(r.Context(), e)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, out)
}
func (a *API) delete(w http.ResponseWriter, r *http.Request) {
	n, err := id(r)
	if err == nil {
		old, readErr := a.store.Get(r.Context(), n)
		if readErr != nil {
			fail(w, readErr)
			return
		}
		if !canEdit(r, old) {
			respond(w, 403, map[string]string{"error": "entry_permission_denied"})
			return
		}
		err = a.store.Delete(r.Context(), n)
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}
func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	entries, month, err := a.monthEntries(w, r)
	if err != nil {
		fail(w, err)
		return
	}
	budget, err := a.store.Budget(r.Context(), month)
	if err != nil {
		fail(w, err)
		return
	}
	s := domain.Summarize(entries, budget)
	sort.Slice(s.Categories, func(i, j int) bool { return s.Categories[i].Amount > s.Categories[j].Amount })
	sort.Slice(s.Days, func(i, j int) bool { return s.Days[i].Date < s.Days[j].Date })
	respond(w, 200, s)
}
func (a *API) budget(w http.ResponseWriter, r *http.Request) {
	if auth.Current(r.Context()).Role != "admin" {
		respond(w, 403, map[string]string{"error": "admin_required"})
		return
	}
	month := r.PathValue("month")
	if _, _, err := domain.Bounds(month); err != nil {
		fail(w, domain.ErrInvalid)
		return
	}
	var b struct {
		Amount int64 `json:"amount_cent"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.Amount < 0 || b.Amount > 100000000000 {
		fail(w, domain.ErrInvalid)
		return
	}
	if err := a.store.SetBudget(r.Context(), month, b.Amount); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, b)
}

func canEdit(r *http.Request, e domain.Entry) bool {
	p := auth.Current(r.Context())
	return p.Role == "admin" || (p.ID != 0 && p.ID == e.CreatedBy)
}
