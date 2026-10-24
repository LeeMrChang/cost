package api

import (
	"bytes"
	"cost/internal/auth"
	"cost/internal/domain"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func adminOnly(w http.ResponseWriter, r *http.Request) bool {
	if auth.Current(r.Context()).Role != "admin" {
		respond(w, 403, map[string]string{"error": "admin_required"})
		return false
	}
	return true
}
func (a *API) createCategory(w http.ResponseWriter, r *http.Request) {
	if !adminOnly(w, r) {
		return
	}
	var c domain.Category
	if !decode(w, r, &c) {
		return
	}
	c.ID = 0
	c.Name = strings.TrimSpace(c.Name)
	out, err := a.store.SaveCategory(r.Context(), c)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 201, out)
}
func (a *API) updateCategory(w http.ResponseWriter, r *http.Request) {
	if !adminOnly(w, r) {
		return
	}
	n, err := id(r)
	if err != nil {
		fail(w, err)
		return
	}
	var p domain.CategoryPatch
	if !decode(w, r, &p) {
		return
	}
	out, err := a.store.PatchCategory(r.Context(), n, p)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, out)
}
func (a *API) accounts(w http.ResponseWriter, r *http.Request) {
	out, err := a.store.Accounts(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, out)
}
func (a *API) createAccount(w http.ResponseWriter, r *http.Request) {
	if !adminOnly(w, r) {
		return
	}
	var ac domain.Account
	if !decode(w, r, &ac) {
		return
	}
	ac.ID = 0
	ac.Name = strings.TrimSpace(ac.Name)
	out, err := a.store.SaveAccount(r.Context(), ac)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 201, out)
}
func (a *API) updateAccount(w http.ResponseWriter, r *http.Request) {
	if !adminOnly(w, r) {
		return
	}
	n, err := id(r)
	if err != nil {
		fail(w, err)
		return
	}
	var p domain.AccountPatch
	if !decode(w, r, &p) {
		return
	}
	out, err := a.store.PatchAccount(r.Context(), n, p)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, out)
}
func (a *API) createTransfer(w http.ResponseWriter, r *http.Request) {
	var t domain.Transfer
	if !decode(w, r, &t) {
		return
	}
	t.ID = 0
	t.CreatedBy = auth.Current(r.Context()).ID
	t.OccurredAt = t.OccurredAt.Truncate(time.Microsecond)
	out, created, err := a.store.SaveTransfer(r.Context(), t)
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
func pagination(q url.Values) (int, int, error) {
	page, size := 1, 50
	for _, v := range []struct {
		name   string
		target *int
		max    int
	}{{"page", &page, 1000000000}, {"page_size", &size, 100}} {
		if q.Has(v.name) {
			n, err := strconv.Atoi(q.Get(v.name))
			if err != nil || n < 1 || n > v.max {
				return 0, 0, domain.ErrInvalid
			}
			*v.target = n
		}
	}
	return page, size, nil
}
func (a *API) transfers(w http.ResponseWriter, r *http.Request) {
	p, s, err := pagination(r.URL.Query())
	if err != nil {
		fail(w, err)
		return
	}
	out, err := a.store.Transfers(r.Context(), p, s)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, out)
}
func (a *API) deleteTransfer(w http.ResponseWriter, r *http.Request) {
	n, err := id(r)
	if err != nil {
		fail(w, err)
		return
	}
	t, err := a.store.GetTransfer(r.Context(), n)
	if err != nil {
		fail(w, err)
		return
	}
	p := auth.Current(r.Context())
	if p.Role != "admin" && (p.ID == 0 || p.ID != t.CreatedBy) {
		respond(w, 403, map[string]string{"error": "entry_permission_denied"})
		return
	}
	if err = a.store.DeleteTransfer(r.Context(), n); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}
func entryFilter(r *http.Request, export bool) (domain.EntryFilter, string, error) {
	q := r.URL.Query()
	f := domain.EntryFilter{Query: q.Get("q"), Kind: q.Get("kind")}
	month := ""
	var err error
	if export {
		f.Page, f.PageSize = 1, 50001
	} else {
		f.Page, f.PageSize, err = pagination(q)
		if err != nil {
			return f, month, err
		}
	}
	if len([]rune(f.Query)) > 500 || (f.Kind != "" && f.Kind != "income" && f.Kind != "expense") {
		return f, month, domain.ErrInvalid
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if q.Has("start_date") || q.Has("end_date") {
		for _, v := range []struct {
			key    string
			target *time.Time
		}{{"start_date", &f.Start}, {"end_date", &f.End}} {
			if q.Has(v.key) {
				raw := q.Get(v.key)
				dt, e := time.ParseInLocation("2006-01-02", raw, loc)
				if e != nil || dt.Year() < 1000 || dt.Format("2006-01-02") != raw {
					return f, month, domain.ErrInvalid
				}
				*v.target = dt
			}
		}
		if !f.End.IsZero() {
			f.End = f.End.AddDate(0, 0, 1)
		}
		if !f.Start.IsZero() && !f.End.IsZero() && !f.Start.Before(f.End) {
			return f, month, domain.ErrInvalid
		}
	} else {
		month = q.Get("month")
		if month == "" {
			month = time.Now().In(loc).Format("2006-01")
		}
		f.Start, f.End, err = domain.Bounds(month)
		if err != nil || f.Start.Year() < 1000 {
			return f, month, domain.ErrInvalid
		}
	}
	for _, v := range []struct {
		key      string
		target   **int64
		min, max int64
	}{{"category_id", &f.CategoryID, 1, 1<<63 - 1}, {"created_by", &f.CreatedBy, 0, 1<<63 - 1}, {"account_id", &f.AccountID, 0, 1<<63 - 1}, {"min_amount_cent", &f.MinAmount, 0, 100000000000}, {"max_amount_cent", &f.MaxAmount, 0, 100000000000}} {
		if q.Has(v.key) {
			n, e := strconv.ParseInt(q.Get(v.key), 10, 64)
			if e != nil || n < v.min || n > v.max {
				return f, month, domain.ErrInvalid
			}
			*v.target = &n
		}
	}
	if f.MinAmount != nil && f.MaxAmount != nil && *f.MinAmount > *f.MaxAmount {
		return f, month, domain.ErrInvalid
	}
	return f, month, nil
}
func (a *API) filteredList(w http.ResponseWriter, r *http.Request) {
	f, month, err := entryFilter(r, false)
	if err != nil {
		fail(w, err)
		return
	}
	out, err := a.store.Query(r.Context(), f)
	if err != nil {
		fail(w, err)
		return
	}
	out.Month = month
	respond(w, 200, out)
}
func (a *API) authors(w http.ResponseWriter, r *http.Request) {
	out, err := a.store.Authors(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, out)
}

// Spreadsheet programs may trim whitespace before interpreting a formula.
func csvText(s string) string {
	trim := strings.TrimLeftFunc(s, unicode.IsSpace)
	if trim != "" && strings.ContainsRune("=+-@", rune(trim[0])) {
		return "'" + s
	}
	if strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}
func (a *API) exportEntries(w http.ResponseWriter, r *http.Request) {
	f, _, err := entryFilter(r, true)
	if err != nil {
		fail(w, err)
		return
	}
	page, err := a.store.Query(r.Context(), f)
	if err != nil {
		fail(w, err)
		return
	}
	if page.Total > 50000 {
		respond(w, 400, map[string]string{"error": "export_too_large"})
		return
	}
	cats, err := a.store.Categories(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	accounts, err := a.store.Accounts(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	names := map[int64]string{}
	for _, c := range cats {
		names[c.ID] = c.Name
	}
	accountNames := map[int64]string{0: "未指定"}
	for _, ac := range accounts {
		accountNames[ac.ID] = ac.Name
	}
	var b bytes.Buffer
	b.WriteString("\ufeff")
	writer := csv.NewWriter(&b)
	writer.UseCRLF = true
	writer.Write([]string{"账单编号", "时间（北京时间）", "类型", "分类", "金额（元）", "账户", "记录人编号", "备注"})
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, e := range page.Items {
		kind := "支出"
		if e.Kind == "income" {
			kind = "收入"
		}
		writer.Write([]string{strconv.FormatInt(e.ID, 10), e.OccurredAt.In(loc).Format("2006-01-02 15:04:05"), kind, csvText(names[e.CategoryID]), fmt.Sprintf("%d.%02d", e.Amount/100, e.Amount%100), csvText(accountNames[e.AccountID]), strconv.FormatInt(e.CreatedBy, 10), csvText(e.Note)})
	}
	writer.Flush()
	if err = writer.Error(); err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="cost-entries.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	w.Write(b.Bytes())
}
