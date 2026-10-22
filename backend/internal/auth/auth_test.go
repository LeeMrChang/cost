package auth

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
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
func TestInviteAtomicAndExpiry(t *testing.T) {
	r := NewMemory()
	ctx := context.Background()
	p, _ := r.Login(ctx, "owner", "s1", time.Now().Add(time.Hour))
	if err := r.Bootstrap(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Invite(ctx, p.ID, "expired", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	q, _ := r.Login(ctx, "family", "s2", time.Now().Add(time.Hour))
	if err := r.Join(ctx, q.ID, "expired", time.Now()); err != ErrInvite {
		t.Fatal("expired invite accepted")
	}
	_ = r.Invite(ctx, p.ID, "once", time.Now().Add(time.Hour))
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r.Join(ctx, q.ID, "once", time.Now()) == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("invite was not consumed atomically")
	}
}
