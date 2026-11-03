package api

import (
	"bytes"
	"context"
	"cost/internal/auth"
	"cost/internal/store"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeWeChat struct{}

func (fakeWeChat) Exchange(_ context.Context, code string) (string, error) {
	if code == "bad" {
		return "", fmt.Errorf("invalid_wechat_code")
	}
	return code, nil
}
func TestLoginLedgerIsolationAndRevocation(t *testing.T) {
	testWorkflow(t, store.NewMemory(), auth.NewMemory())
}
func testWorkflow(t *testing.T, data store.Store, repo auth.Repository) {
	t.Helper()
	service := auth.New(repo, fakeWeChat{})
	h := NewWithAuth(data, service)
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("X-Dev-Token", "cost-local-dev")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("expected %d got %d: %s", status, w.Code, w.Body.String())
		}
	}
	login := func(code string) (string, int64) {
		w := call("POST", "/api/v1/auth/wechat", "", `{"code":"`+code+`","nickname":"`+code+`"}`)
		expect(w, 200)
		var v struct {
			Token string
			User  auth.Principal
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(w.Body.String(), "openid") || strings.Contains(w.Body.String(), "session_key") {
			t.Fatal("identity secret leaked")
		}
		return v.Token, v.User.ID
	}
	expect(call("GET", "/api/v1/entries", "", ""), 401)
	expect(call("POST", "/api/v1/auth/wechat", "", `{"code":"bad"}`), 400)
	expect(call("POST", "/api/v1/auth/wechat", "", `{"code":"a","openid":"forged"}`), 400)
	first, _ := login("owner")
	second, _ := login("other")

	// 登录即可直达业务接口，无初始化/角色中间态。
	expect(call("GET", "/api/v1/entries", first, ""), 200)
	expect(call("GET", "/api/v1/auth/me", first, ""), 200)
	me := call("GET", "/api/v1/auth/me", first, "")
	var principal auth.Principal
	_ = json.Unmarshal(me.Body.Bytes(), &principal)
	if principal.ID == 0 || strings.Contains(me.Body.String(), "role") {
		t.Fatal("principal must carry id without role")
	}

	// 每个用户自动获得独立账本，数据与幂等键互不干扰。
	catList := call("GET", "/api/v1/categories", first, "")
	expect(catList, 200)
	var cats []struct {
		ID   int64  `json:"id"`
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(catList.Body.Bytes(), &cats); err != nil {
		t.Fatal(err)
	}
	catID := int64(0)
	for _, c := range cats {
		if c.Kind == "expense" {
			catID = c.ID
			break
		}
	}
	if catID == 0 {
		t.Fatal("no expense category in fresh ledger")
	}
	entry := fmt.Sprintf(`{"category_id":%d,"kind":"expense","amount_cent":1800,"occurred_at":"2026-10-13T12:00:00+08:00","client_request_id":"request-1"}`, catID)
	expect(call("POST", "/api/v1/entries", first, entry), 201)
	expect(call("POST", "/api/v1/entries", first, entry), 200)
	// 第二个用户拥有独立账本与分类：相同幂等键不冲突，但必须使用自己的分类。
	otherCats := call("GET", "/api/v1/categories", second, "")
	expect(otherCats, 200)
	_ = json.Unmarshal(otherCats.Body.Bytes(), &cats)
	otherCatID := int64(0)
	for _, c := range cats {
		if c.Kind == "expense" {
			otherCatID = c.ID
			break
		}
	}
	otherEntry := fmt.Sprintf(`{"category_id":%d,"kind":"expense","amount_cent":1800,"occurred_at":"2026-10-13T12:00:00+08:00","client_request_id":"request-1"}`, otherCatID)
	expect(call("POST", "/api/v1/entries", second, otherEntry), 201)
	expect(call("POST", "/api/v1/entries", second, entry), 400)
	if !strings.Contains(call("GET", "/api/v1/entries?month=2026-10", second, "").Body.String(), `"total":1`) {
		t.Fatal("second user must see only their own entry")
	}

	// 预算与账本接口。
	expect(call("PUT", "/api/v1/budgets", first, `{"scope":"month","period":"2026-10","category_id":0,"amount_cent":10000}`), 200)
	expect(call("PUT", "/api/v1/budgets", first, `{"scope":"month","period":"2026-13","category_id":0,"amount_cent":10000}`), 400)
	expect(call("PUT", "/api/v1/budgets", first, `{"scope":"quarter","period":"2026-10","category_id":0,"amount_cent":10000}`), 400)
	expect(call("GET", "/api/v1/budgets?scope=month&period=2026-10", first, ""), 200)
	ledger := call("GET", "/api/v1/ledgers/me", first, "")
	expect(ledger, 200)
	if !strings.Contains(ledger.Body.String(), `"name":"我的账本"`) {
		t.Fatal("ledger must be auto-provisioned with default name")
	}
	expect(call("GET", "/api/v1/stats?range=month&ref=2026-10-15", first, ""), 200)
	expect(call("GET", "/api/v1/stats?range=bad", first, ""), 400)

	// 登出后令牌失效。
	expect(call("POST", "/api/v1/auth/logout", first, ""), 204)
	expect(call("GET", "/api/v1/auth/me", first, ""), 401)
}
func TestAuthRoutesMethodAndExpiry(t *testing.T) {
	repo := auth.NewMemory()
	expired := strings.Repeat("e", 64)
	// Repository sessions store digests, never the raw bearer token.
	// Resolve checks expiry independently of the client's expiry timestamp.
	_, err := repo.Login(context.Background(), "test", "", expired, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Resolve(context.Background(), expired, time.Now()); err != auth.ErrUnauthorized {
		t.Fatal("expired session accepted")
	}
}
