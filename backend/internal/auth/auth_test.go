package auth

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestWeChatExchange(t *testing.T) {
	for _, tc := range []struct{ body, openid, err string }{
		{`{"openid":"wx-user","session_key":"never-return"}`, "wx-user", ""},
		{`{"errcode":40029,"errmsg":"invalid code"}`, "", "invalid_wechat_code"},
		{`{"errcode":40163}`, "", "invalid_wechat_code"},
		{`{"errcode":-1}`, "", "wechat_unavailable"}, {`{}`, "", "wechat_unavailable"}, {`bad`, "", "wechat_unavailable"},
	} {
		wx := WeChat{AppID: "appid", Secret: "secret", Client: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
			q := r.URL.Query()
			if r.URL.Scheme != "https" || r.URL.Host != "api.weixin.qq.com" || q.Get("js_code") != "one-time-code" || q.Get("secret") != "secret" || q.Get("grant_type") != "authorization_code" {
				t.Error("incorrect exchange request")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})}}
		id, err := wx.Exchange(context.Background(), "one-time-code")
		if id != tc.openid {
			t.Fatal("unexpected identity")
		}
		if tc.err == "" && err != nil || tc.err != "" && (err == nil || err.Error() != tc.err) {
			t.Fatalf("unexpected error %v", err)
		}
	}
	if _, err := (WeChat{}).Exchange(context.Background(), "code"); err == nil {
		t.Fatal("missing configuration accepted")
	}
}
func TestSessionLifecycle(t *testing.T) {
	r := NewMemory()
	ctx := context.Background()
	now := time.Now()
	p, err := r.Login(ctx, "wx-user", "", "s1", now.Add(time.Hour))
	if err != nil || p.ID == 0 {
		t.Fatalf("login failed: %v %v", p, err)
	}
	got, err := r.Resolve(ctx, "s1", now)
	if err != nil || got.ID != p.ID {
		t.Fatalf("resolve failed: %v %v", got, err)
	}
	if _, err = r.Resolve(ctx, "s1", now.Add(2*time.Hour)); err != ErrUnauthorized {
		t.Fatal("expired session accepted")
	}
	if err = r.Logout(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Resolve(ctx, "s1", now); err != ErrUnauthorized {
		t.Fatal("logged-out session accepted")
	}
	if again, err := r.Login(ctx, "wx-user", "", "s2", now.Add(time.Hour)); err != nil || again.ID != p.ID {
		t.Fatalf("same openid must map to same user: %v %v", again, err)
	}
}
