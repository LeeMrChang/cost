package api

import (
	"bytes"
	"context"
	"cost/internal/auth"
	"cost/internal/store"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNicknameWorkflow(t *testing.T) { nicknameWorkflow(t, store.NewMemory(), auth.NewMemory()) }
func TestMySQLNicknameWorkflow(t *testing.T) {
	db := testMySQL(t, false)
	nicknameWorkflow(t, db, &auth.MySQL{DB: db.DB()})
}
func nicknameWorkflow(t *testing.T, data store.Store, repo auth.Repository) {
	t.Helper()
	h := NewWithAuth(data, auth.New(repo, fakeWeChat{}))
	call := func(method, path, token, body string, want int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: want %d got %d: %s", method, path, want, w.Code, w.Body.String())
		}
		var value map[string]any
		if w.Body.Len() > 0 {
			if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
		}
		if strings.Contains(w.Body.String(), "openid") || strings.Contains(w.Body.String(), "session_key") {
			t.Fatal("identity secret leaked")
		}
		return value
	}
	login := func(body string) (string, map[string]any) {
		v := call("POST", "/api/v1/auth/wechat", "", body, 200)
		return v["token"].(string), v["user"].(map[string]any)
	}
	first, p := login(`{"code":"owner","nickname":"  小夏🐱  "}`)
	if p["nickname"] != "小夏🐱" {
		t.Fatalf("nickname not stored: %v", p)
	}
	second, same := login(`{"code":"owner"}`)
	if same["id"] != p["id"] || same["nickname"] != "小夏🐱" {
		t.Fatalf("relogin lost profile: %v", same)
	}
	call("PATCH", "/api/v1/auth/me", "", `{"nickname":"forged"}`, 401)
	for _, body := range []string{`{}`, `{"nickname":""}`, `{"nickname":"  "}`, `{"nickname":"bad\nname"}`, `{"nickname":"` + strings.Repeat("猫", 65) + `"}`, `{"nickname":"ok","id":999}`, `{"nickname":"ok","openid":"other"}`} {
		call("PATCH", "/api/v1/auth/me", first, body, 400)
	}
	for _, name := range []string{"", " ", "bad\nname", strings.Repeat("猫", 65)} {
		body, _ := json.Marshal(map[string]string{"code": "owner", "nickname": name})
		call("POST", "/api/v1/auth/wechat", "", string(body), 400)
	}
	call("PATCH", "/api/v1/auth/me", first, "{\"nickname\":\"\xff\"}", 400)
	call("PATCH", "/api/v1/auth/me", first, `{"nickname":"小夏的新昵称"}`, 200)
	if got := call("GET", "/api/v1/auth/me", second, "", 200); got["nickname"] != "小夏的新昵称" {
		t.Fatalf("old session stale: %v", got)
	}
	_, updated := login(`{"code":"owner","nickname":"重新登录🐱"}`)
	if updated["nickname"] != "重新登录🐱" {
		t.Fatal(updated)
	}
	if got := call("GET", "/api/v1/auth/me", first, "", 200); got["nickname"] != "重新登录🐱" {
		t.Fatal(got)
	}
	other, _ := login(`{"code":"other","nickname":"另一个用户"}`)
	boundary := strings.Repeat("🐱", 64)
	raw, _ := json.Marshal(map[string]string{"nickname": boundary})
	call("PATCH", "/api/v1/auth/me", first, string(raw), 200)
	if got := call("GET", "/api/v1/auth/me", first, "", 200); got["nickname"] != boundary {
		t.Fatal("emoji boundary lost", got)
	}
	if got := call("GET", "/api/v1/auth/me", other, "", 200); got["nickname"] != "另一个用户" {
		t.Fatal("updated another user", got)
	}
	call("GET", "/api/v1/entries", first, "", 200)
	legacy, old := login(`{"code":"legacy"}`)
	if old["nickname"] != "" {
		t.Fatal(old)
	}
	if got := call("GET", "/api/v1/entries", legacy, "", 403); got["error"] != "nickname_required" {
		t.Fatal(got)
	}
	call("GET", "/api/v1/auth/me", legacy, "", 200)
	call("PATCH", "/api/v1/auth/me", legacy, `{"nickname":"补填昵称"}`, 200)
	call("GET", "/api/v1/entries", legacy, "", 200)
	logout, _ := login(`{"code":"legacy-logout"}`)
	call("POST", "/api/v1/auth/logout", logout, "", 204)
}
func TestMySQLNicknameMigrationPreservesUsers(t *testing.T) {
	db := testMySQL(t, false)
	ctx := context.Background()
	_, err := db.DB().Exec("INSERT INTO t_users(id,openid,created_at) VALUES(100,'historical','2026-10-22 20:03:24.912610')")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB().Exec("ALTER TABLE t_users DROP COLUMN nickname"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB().Exec("DELETE FROM t_schema_migrations WHERE version='007_user_nickname.sql'"); err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var nickname, created string
	if err = db.DB().QueryRow("SELECT nickname,CAST(created_at AS CHAR) FROM t_users WHERE id=100 AND openid='historical'").Scan(&nickname, &created); err != nil {
		t.Fatal(err)
	}
	if nickname != "" || created != "2026-10-22 20:03:24.912610" {
		t.Fatalf("history changed: %q %q", nickname, created)
	}
	if _, err = db.DB().Exec("UPDATE t_users SET nickname='历史用户🐱' WHERE id=100"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB().Exec("DELETE FROM t_schema_migrations WHERE version='007_user_nickname.sql'"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = db.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.DB().QueryRow("SELECT nickname FROM t_users WHERE id=100").Scan(&nickname); err != nil || nickname != "历史用户🐱" {
		t.Fatal("recovery lost profile", nickname, err)
	}
}
