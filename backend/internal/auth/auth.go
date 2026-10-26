// Package auth implements WeChat identity and opaque sessions for a single-user ledger.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized")

// Principal 表示已认证用户；单用户模型下无角色。
type Principal struct {
	ID int64 `json:"id"`
}
type Repository interface {
	Login(context.Context, string, string, time.Time) (Principal, error)
	Resolve(context.Context, string, time.Time) (Principal, error)
	Logout(context.Context, string) error
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
	Repo     Repository
	WeChat   Exchanger
	mu       sync.Mutex
	attempts map[string]bucket
}

func New(repo Repository, wx Exchanger) *Service {
	return &Service{Repo: repo, WeChat: wx, attempts: map[string]bucket{}}
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
}
func response(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
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
		response(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	p, err := s.Repo.Login(r.Context(), id, digest(token), expires)
	if err != nil {
		response(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	response(w, 200, map[string]any{"token": token, "expires_at": expires, "user": p})
}
func (s *Service) me(w http.ResponseWriter, r *http.Request) { response(w, 200, Current(r.Context())) }
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.Repo.Logout(r.Context(), digest(bearer(r))); err != nil {
		response(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	w.WriteHeader(204)
}
