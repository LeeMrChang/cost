// Package auth implements WeChat identity, opaque sessions and one private family.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")
var ErrConflict = errors.New("already initialized or already a member")
var ErrInvite = errors.New("invalid or expired invite")

type Principal struct {
	ID   int64  `json:"id"`
	Role string `json:"role"`
}
type Repository interface {
	Login(context.Context, string, string, time.Time) (Principal, error)
	Resolve(context.Context, string, time.Time) (Principal, error)
	Logout(context.Context, string) error
	Bootstrap(context.Context, int64) error
	Invite(context.Context, int64, string, time.Time) error
	Join(context.Context, int64, string, time.Time) error
	Members(context.Context) ([]Principal, error)
	Remove(context.Context, int64) error
}
type Exchanger interface {
	Exchange(context.Context, string) (string, error)
}

type WeChat struct {
	AppID, Secret string
	Client        *http.Client
}

func (w WeChat) Exchange(ctx context.Context, code string) (string, error) {
	if w.AppID == "" || w.Secret == "" {
		return "", errors.New("wechat_not_configured")
	}
	q := url.Values{"appid": {w.AppID}, "secret": {w.Secret}, "js_code": {code}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com/sns/jscode2session?"+q.Encode(), nil)
	if err != nil {
		return "", errors.New("wechat_unavailable")
	}
	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", errors.New("wechat_unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", errors.New("wechat_unavailable")
	}
	var result struct {
		OpenID    string `json:"openid"`
		ErrorCode int    `json:"errcode"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&result) != nil {
		return "", errors.New("wechat_unavailable")
	}
	if result.ErrorCode == 40029 || result.ErrorCode == 40163 {
		return "", errors.New("invalid_wechat_code")
	}
	if result.ErrorCode != 0 || result.OpenID == "" || len(result.OpenID) > 128 {
		return "", errors.New("wechat_unavailable")
	}
	return result.OpenID, nil
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

type contextKey struct{}

func Current(ctx context.Context) Principal { p, _ := ctx.Value(contextKey{}).(Principal); return p }
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

type bucket struct {
	start time.Time
	count int
}
type Service struct {
	Repo        Repository
	WeChat      Exchanger
	SetupSecret string
	mu          sync.Mutex
	attempts    map[string]bucket
}

func New(repo Repository, wx Exchanger, setupSecret string) *Service {
	return &Service{Repo: repo, WeChat: wx, SetupSecret: setupSecret, attempts: map[string]bucket{}}
}
func (s *Service) limited(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, b := range s.attempts {
		if now.Sub(b.start) >= time.Minute {
			delete(s.attempts, k)
		}
	}
	b, ok := s.attempts[ip]
	if !ok {
		if len(s.attempts) >= 1024 {
			return true
		}
		b = bucket{start: now}
	}
	b.count++
	s.attempts[ip] = b
	return b.count > 30
}
func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}
func (s *Service) Authenticate(r *http.Request) (Principal, error) {
	t := bearer(r)
	if len(t) != 64 {
		return Principal{}, ErrUnauthorized
	}
	return s.Repo.Resolve(r.Context(), digest(t), time.Now().UTC())
}
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/wechat", s.login)
	mux.HandleFunc("GET /api/v1/auth/me", s.me)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("POST /api/v1/family/bootstrap", s.bootstrap)
	mux.HandleFunc("POST /api/v1/family/join", s.join)
	mux.HandleFunc("POST /api/v1/family/invites", s.invite)
	mux.HandleFunc("GET /api/v1/family/members", s.members)
	mux.HandleFunc("DELETE /api/v1/family/members/{id}", s.remove)
}
func response(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func failure(w http.ResponseWriter, err error) {
	status, code := 500, "internal_error"
	switch {
	case errors.Is(err, ErrUnauthorized):
		status, code = 401, "unauthorized"
	case errors.Is(err, ErrForbidden):
		status, code = 403, "family_access_required"
	case errors.Is(err, ErrConflict):
		status, code = 409, "family_already_initialized_or_member"
	case errors.Is(err, ErrInvite):
		status, code = 400, "invalid_or_expired_invite"
	}
	response(w, status, map[string]string{"error": code})
}
func body(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		response(w, 400, map[string]string{"error": "invalid_json"})
		return false
	}
	return true
}
func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if s.limited(ip) {
		response(w, 429, map[string]string{"error": "too_many_login_attempts"})
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if !body(w, r, &input) {
		return
	}
	if input.Code == "" || len(input.Code) > 256 {
		response(w, 400, map[string]string{"error": "invalid_wechat_code"})
		return
	}
	id, err := s.WeChat.Exchange(r.Context(), input.Code)
	if err != nil {
		code := err.Error()
		status := 503
		if code == "invalid_wechat_code" {
			status = 400
		} else if code != "wechat_not_configured" {
			code = "wechat_unavailable"
		}
		response(w, status, map[string]string{"error": code})
		return
	}
	token, err := randomToken()
	if err != nil {
		failure(w, err)
		return
	}
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	p, err := s.Repo.Login(r.Context(), id, digest(token), expires)
	if err != nil {
		failure(w, err)
		return
	}
	response(w, 200, map[string]any{"token": token, "expires_at": expires, "user": p})
}
func (s *Service) me(w http.ResponseWriter, r *http.Request) { response(w, 200, Current(r.Context())) }
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.Repo.Logout(r.Context(), digest(bearer(r))); err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(204)
}
func (s *Service) bootstrap(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if s.limited(ip) {
		response(w, 429, map[string]string{"error": "too_many_login_attempts"})
		return
	}
	var input struct {
		Secret string `json:"secret"`
	}
	if !body(w, r, &input) {
		return
	}
	if len(s.SetupSecret) < 32 || subtle.ConstantTimeCompare([]byte(digest(input.Secret)), []byte(digest(s.SetupSecret))) != 1 {
		failure(w, ErrForbidden)
		return
	}
	if err := s.Repo.Bootstrap(r.Context(), Current(r.Context()).ID); err != nil {
		failure(w, err)
		return
	}
	response(w, 200, map[string]string{"role": "admin"})
}
func (s *Service) join(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if s.limited(ip) {
		response(w, 429, map[string]string{"error": "too_many_login_attempts"})
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if !body(w, r, &input) {
		return
	}
	if len(input.Token) != 64 {
		failure(w, ErrInvite)
		return
	}
	if err := s.Repo.Join(r.Context(), Current(r.Context()).ID, digest(input.Token), time.Now().UTC()); err != nil {
		failure(w, err)
		return
	}
	response(w, 200, map[string]string{"role": "member"})
}
func admin(w http.ResponseWriter, r *http.Request) bool {
	if Current(r.Context()).Role != "admin" {
		failure(w, ErrForbidden)
		return false
	}
	return true
}
func (s *Service) invite(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	token, err := randomToken()
	if err != nil {
		failure(w, err)
		return
	}
	expires := time.Now().UTC().Add(24 * time.Hour)
	if err = s.Repo.Invite(r.Context(), Current(r.Context()).ID, digest(token), expires); err != nil {
		failure(w, err)
		return
	}
	response(w, 201, map[string]any{"token": token, "expires_at": expires})
}
func (s *Service) members(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	members, err := s.Repo.Members(r.Context())
	if err != nil {
		failure(w, err)
		return
	}
	response(w, 200, members)
}
func (s *Service) remove(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		response(w, 400, map[string]string{"error": "invalid_member"})
		return
	}
	if err = s.Repo.Remove(r.Context(), id); err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(204)
}
