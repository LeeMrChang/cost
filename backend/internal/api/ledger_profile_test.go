package api

import (
	"bytes"
	"cost/internal/auth"
	"cost/internal/store"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLedgerProfileEditing(t *testing.T) {
	ledgerProfileWorkflow(t, store.NewMemory(), auth.NewMemory())
}
func TestMySQLLedgerProfileEditing(t *testing.T) {
	db := testMySQL(t, false)
	ledgerProfileWorkflow(t, db, &auth.MySQL{DB: db.DB()})
}
func ledgerProfileWorkflow(t *testing.T, data store.Store, repo auth.Repository) {
	t.Helper()
	handler := NewWithAuth(data, auth.New(repo, fakeWeChat{}))
	call := func(method, path, token, body string, want int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s want %d got %d: %s", method, path, want, w.Code, w.Body.String())
		}
		var v map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	login := func(code string) string {
		return call("POST", "/api/v1/auth/wechat", "", `{"code":"`+code+`","nickname":"测试用户"}`, 200)["token"].(string)
	}
	first, second := login("first"), login("second")
	path := "/api/v1/ledgers/me"
	before := call("GET", path, first, "", 200)
	other := call("GET", path, second, "", 200)
	call("PATCH", path, "", `{"name":"未登录"}`, 401)
	got := call("PATCH", path, first, `{"name":"  旅行基金🐱  ","icon":"✈️"}`, 200)
	if got["name"] != "旅行基金🐱" || got["icon"] != "✈️" || got["id"] != before["id"] || got["currency"] != before["currency"] || got["timezone"] != before["timezone"] {
		t.Fatal(got)
	}
	got = call("PATCH", path, first, `{"name":"日常开销"}`, 200)
	if got["icon"] != "✈️" {
		t.Fatal("name patch cleared icon", got)
	}
	got = call("PATCH", path, first, `{"icon":"🏠"}`, 200)
	if got["name"] != "日常开销" {
		t.Fatal("icon patch cleared name", got)
	}
	for _, body := range []string{`{}`, `{"name":null}`, `{"name":""}`, `{"name":"  "}`, `{"name":"bad\nname"}`, `{"icon":""}`, `{"icon":"\t"}`, `{"name":"` + strings.Repeat("猫", 65) + `"}`, `{"icon":"` + strings.Repeat("🐱", 33) + `"}`, `{"name":"ok","id":999}`, `{"name":"ok","owner_id":999}`, `{"name":"ok","currency":"USD"}`, `{"name":"ok","timezone":"UTC"}`, "{\"name\":\"\xff\"}"} {
		call("PATCH", path, first, body, 400)
	}
	got = call("GET", path, first, "", 200)
	if got["name"] != "日常开销" || got["icon"] != "🏠" {
		t.Fatal("invalid request changed ledger", got)
	}
	name, icon := strings.Repeat("🐱", 64), strings.Repeat("🐱", 32)
	raw, _ := json.Marshal(map[string]string{"name": name, "icon": icon})
	call("PATCH", path, first, string(raw), 200)
	got = call("GET", path, first, "", 200)
	if got["name"] != name || got["icon"] != icon {
		t.Fatal("unicode boundary failed", got)
	}
	untouched := call("GET", path, second, "", 200)
	if untouched["id"] != other["id"] || untouched["name"] != other["name"] || untouched["icon"] != other["icon"] {
		t.Fatal("another user's ledger changed", untouched)
	}
	relogin := login("first")
	got = call("GET", path, relogin, "", 200)
	if got["name"] != name || got["icon"] != icon {
		t.Fatal("profile not retained", got)
	}
}
