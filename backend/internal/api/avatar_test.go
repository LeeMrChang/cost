package api

import (
	"bytes"
	"context"
	"cost/internal/auth"
	"cost/internal/store"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAvatarWorkflow(t *testing.T) { avatarWorkflow(t, store.NewMemory(), auth.NewMemory()) }
func TestMySQLAvatarWorkflow(t *testing.T) {
	db := testMySQL(t, false)
	avatarWorkflow(t, db, &auth.MySQL{DB: db.DB()})
}
func avatarWorkflow(t *testing.T, data store.Store, repo auth.Repository) {
	t.Helper()
	h := NewWithAuth(data, auth.New(repo, fakeWeChat{}))
	call := func(method, path, token, body string, want int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s want %d got %d: %s", method, path, want, w.Code, w.Body.String())
		}
		var v map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	login := func(code string) (string, map[string]any) {
		v := call("POST", "/api/v1/auth/wechat", "", `{"code":"`+code+`"}`, 200)
		return v["token"].(string), v["user"].(map[string]any)
	}
	token, _ := login("avatar-owner")
	other, _ := login("avatar-other")
	var src bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	img.Set(8, 4, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&src, img); err != nil {
		t.Fatal(err)
	}
	upload := func(token string, files [][]byte, extra bool, want int) map[string]any {
		t.Helper()
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		for _, raw := range files {
			part, err := mw.CreateFormFile("avatar", "avatar.png")
			if err != nil {
				t.Fatal(err)
			}
			part.Write(raw)
		}
		if extra {
			mw.WriteField("user_id", "999")
		}
		mw.Close()
		r := httptest.NewRequest("POST", "/api/v1/auth/me/avatar", &b)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("upload want %d got %d: %s", want, w.Code, w.Body.String())
		}
		var v map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	get := func(path string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("GET %s want%d got%d", path, want, w.Code)
		}
		return w
	}
	first := upload(token, [][]byte{src.Bytes()}, false, 200)
	url, ok := first["avatar_url"].(string)
	if !ok || !strings.HasPrefix(url, "/api/v1/avatars/") || strings.Contains(url, "avatar-owner") {
		t.Fatal("missing safe persistent path", first)
	}
	w := get(url, 200)
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal(w.Header())
	}
	decoded, err := jpeg.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil || decoded.Bounds().Dx() != 256 || decoded.Bounds().Dy() != 256 {
		t.Fatal("invalid normalized avatar", err)
	}
	head := httptest.NewRecorder()
	h.ServeHTTP(head, httptest.NewRequest("HEAD", url, nil))
	if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") == "" {
		t.Fatal("invalid public HEAD response", head.Code, head.Header())
	}
	call("POST", url, "", "", 401)
	call("PATCH", "/api/v1/auth/me", token, `{"nickname":"小夏"}`, 200)
	if got := call("GET", "/api/v1/auth/me", token, "", 200); got["avatar_url"] != url || got["nickname"] != "小夏" {
		t.Fatal("nickname update lost avatar", got)
	}
	_, again := login("avatar-owner")
	if again["avatar_url"] != url {
		t.Fatal("relogin lost avatar", again)
	}
	upload("", [][]byte{src.Bytes()}, false, 401)
	upload(token, [][]byte{[]byte("not an image")}, false, 400)
	upload(token, nil, false, 400)
	upload(token, [][]byte{src.Bytes(), src.Bytes()}, false, 400)
	upload(token, [][]byte{src.Bytes()}, true, 400)
	upload(token, [][]byte{bytes.Repeat([]byte("x"), (2<<20)+1)}, false, 413)
	var huge bytes.Buffer
	png.Encode(&huge, image.NewGray(image.Rect(0, 0, 2049, 1)))
	upload(token, [][]byte{huge.Bytes()}, false, 400)
	if got := call("GET", "/api/v1/auth/me", other, "", 200); got["avatar_url"] != "" {
		t.Fatal("another user changed", got)
	}
	if got := call("GET", "/api/v1/auth/me", token, "", 200); got["avatar_url"] != url {
		t.Fatal("invalid upload changed avatar", got)
	}
	var jpegSource bytes.Buffer
	if err := jpeg.Encode(&jpegSource, img, nil); err != nil {
		t.Fatal(err)
	}
	replacement := upload(token, [][]byte{jpegSource.Bytes()}, false, 200)
	if replacement["nickname"] != "小夏" || replacement["avatar_url"] == url {
		t.Fatal(replacement)
	}
	get(url, 404)
	get(replacement["avatar_url"].(string), 200)
	get("/api/v1/avatars/"+strings.Repeat("0", 64)+".jpg", 404)
	call("PATCH", "/api/v1/auth/me", token, `{"avatar_url":"https://forged.example/avatar.jpg"}`, 400)
}
func TestMySQLAvatarMigrationPreservesUsers(t *testing.T) {
	db := testMySQL(t, false)
	ctx := context.Background()
	if _, err := db.DB().Exec("INSERT INTO t_users(id,openid,nickname,created_at) VALUES(101,'avatar-history','历史昵称','2026-10-22 20:03:24.912610')"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"ALTER TABLE t_users DROP INDEX uk_user_avatar_url, DROP COLUMN avatar_url, DROP COLUMN avatar_data", "DELETE FROM t_schema_migrations WHERE version='008_user_avatar.sql'"} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var nickname, created string
	var count int
	if err := db.DB().QueryRow("SELECT nickname,CAST(created_at AS CHAR),avatar_url IS NULL AND avatar_data IS NULL FROM t_users WHERE id=101 AND openid='avatar-history'").Scan(&nickname, &created, &count); err != nil || nickname != "历史昵称" || created != "2026-10-22 20:03:24.912610" || count != 1 {
		t.Fatal(nickname, created, count, err)
	}
	if _, err := db.DB().Exec("DELETE FROM t_schema_migrations WHERE version='008_user_avatar.sql'"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec("ALTER TABLE t_users DROP COLUMN avatar_data"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
}
