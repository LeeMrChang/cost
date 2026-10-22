package api

import (
	"bytes"
	"context"
	"cost/internal/auth"
	"cost/internal/domain"
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
func TestFamilyLoginPermissionsAndRevocation(t *testing.T) {
	testFamilyWorkflow(t, store.NewMemory(), auth.NewMemory())
}
func testFamilyWorkflow(t *testing.T, data store.Store, repo auth.Repository) {
	t.Helper()
	service := auth.New(repo, fakeWeChat{}, strings.Repeat("s", 32))
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
		w := call("POST", "/api/v1/auth/wechat", "", `{"code":"`+code+`"}`)
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
	admin, _ := login("owner")
	member, memberID := login("family")
	stranger, _ := login("stranger")
	expect(call("GET", "/api/v1/entries", stranger, ""), 403)
	expect(call("GET", "/api/v1/auth/me", stranger, ""), 200)
	expect(call("POST", "/api/v1/family/bootstrap", admin, `{"secret":"wrong"}`), 403)
	expect(call("POST", "/api/v1/family/bootstrap", admin, `{"secret":"`+strings.Repeat("s", 32)+`"}`), 200)
	expect(call("POST", "/api/v1/family/bootstrap", stranger, `{"secret":"`+strings.Repeat("s", 32)+`"}`), 409)
	invite := func() string {
		w := call("POST", "/api/v1/family/invites", admin, "")
		expect(w, 201)
		var v struct{ Token string }
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		return v.Token
	}
	token := invite()
	expect(call("POST", "/api/v1/family/join", member, `{"token":"`+token+`"}`), 200)
	expect(call("POST", "/api/v1/family/join", stranger, `{"token":"`+token+`"}`), 400)
	expect(call("GET", "/api/v1/entries", member, ""), 200)
	expect(call("POST", "/api/v1/family/invites", member, ""), 403)
	expect(call("PUT", "/api/v1/budgets/2026-10", member, `{"amount_cent":100}`), 403)
	expect(call("DELETE", "/api/v1/family/members/1", admin, ""), 403)
	entry := `{"category_id":2,"kind":"expense","amount_cent":1800,"occurred_at":"2026-10-13T12:00:00+08:00","client_request_id":"request-1","created_by":1}`
	expect(call("POST", "/api/v1/entries", member, entry), 201)
	e, _ := data.Get(context.Background(), 1)
	if e.CreatedBy != memberID {
		t.Fatal("creator forged")
	}
	expect(call("POST", "/api/v1/entries", member, entry), 200)
	expect(call("POST", "/api/v1/entries", admin, entry), 409)
	entry2 := strings.Replace(entry, "request-1", "request-2", 1)
	created := call("POST", "/api/v1/entries", admin, entry2)
	expect(created, 201)
	var ownerEntry domain.Entry
	if err := json.Unmarshal(created.Body.Bytes(), &ownerEntry); err != nil {
		t.Fatal(err)
	}
	ownerPath := fmt.Sprintf("/api/v1/entries/%d", ownerEntry.ID)
	expect(call("PATCH", ownerPath, member, entry2), 403)
	expect(call("DELETE", ownerPath, member, ""), 403)
	expect(call("PATCH", "/api/v1/entries/1", member, entry), 200)
	expect(call("PATCH", "/api/v1/entries/1", admin, entry), 200)
	e, _ = data.Get(context.Background(), 1)
	if e.CreatedBy != memberID {
		t.Fatal("update changed creator")
	}
	// Historical entries remain visible and can be maintained by the administrator.
	legacy, _, err := data.Save(context.Background(), domain.Entry{CategoryID: 2, Kind: "expense", Amount: 50, OccurredAt: time.Now(), RequestID: "legacy"})
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := fmt.Sprintf("/api/v1/entries/%d", legacy.ID)
	expect(call("DELETE", legacyPath, member, ""), 403)
	expect(call("DELETE", legacyPath, admin, ""), 204)
	expect(call("DELETE", fmt.Sprintf("/api/v1/family/members/%d", memberID), admin, ""), 204)
	expect(call("GET", "/api/v1/entries", member, ""), 403)
	expect(call("POST", "/api/v1/auth/logout", admin, ""), 204)
	expect(call("GET", "/api/v1/auth/me", admin, ""), 401)
}
func TestAuthRoutesMethodAndExpiry(t *testing.T) {
	repo := auth.NewMemory()
	expired := strings.Repeat("e", 64)
	// Repository sessions store digests, never the raw bearer token.
	// Resolve checks expiry independently of the client's expiry timestamp.
	_, err := repo.Login(context.Background(), "test", expired, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Resolve(context.Background(), expired, time.Now()); err != auth.ErrUnauthorized {
		t.Fatal("expired session accepted")
	}
}
