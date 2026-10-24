package api

import (
	"bytes"
	"context"
	"cost/internal/auth"
	"cost/internal/domain"
	"cost/internal/store"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMemberProfiles(t *testing.T) { testMemberProfiles(t, store.NewMemory(), auth.NewMemory()) }
func testMemberProfiles(t *testing.T, data store.Store, repo auth.Repository) {
	t.Helper()
	h := NewWithAuth(data, auth.New(repo, fakeWeChat{}, strings.Repeat("s", 32)))
	call := func(method, path, token, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s want %d got %d: %s", method, path, status, w.Code, w.Body)
		}
		return w
	}
	login := func(code string) (string, int64) {
		w := call("POST", "/api/v1/auth/wechat", "", `{"code":"`+code+`"}`, 200)
		var v struct {
			Token string
			User  struct{ ID int64 }
		}
		json.Unmarshal(w.Body.Bytes(), &v)
		return v.Token, v.User.ID
	}
	owner, oid := login("owner")
	member, mid := login("member")
	stranger, sid := login("outsider")
	call("POST", "/api/v1/family/bootstrap", owner, `{"secret":"`+strings.Repeat("s", 32)+`"}`, 200)
	join := func(token string) {
		w := call("POST", "/api/v1/family/invites", owner, "", 201)
		var v struct{ Token string }
		json.Unmarshal(w.Body.Bytes(), &v)
		call("POST", "/api/v1/family/join", token, `{"token":"`+v.Token+`"}`, 200)
	}
	join(member)
	path := func(id int64) string { return fmt.Sprintf("/api/v1/family/members/%d/nickname", id) }
	patch := func(token string, id int64, name string, status int) {
		body, _ := json.Marshal(map[string]string{"nickname": name})
		call("PATCH", path(id), token, string(body), status)
	}
	patch(member, mid, "  妈妈🍀  ", 200)
	patch(member, oid, "冒充", 403)
	patch(stranger, sid, "路人", 403)
	patch("", mid, "匿名", 401)
	patch(owner, sid, "未加入", 404)
	patch(owner, 99999, "不存在", 404)
	for _, body := range []string{`{}`, `{"nickname":null}`, `{"nickname":42}`, `{"nickname":"名字","role":"admin"}`} {
		call("PATCH", path(mid), member, body, 400)
	}
	for _, name := range []string{strings.Repeat("名", 21), "坏\n名字", "坏\x00名字", "\t名字", "名字\u2028换行"} {
		patch(member, mid, name, 400)
	}
	patch(member, mid, strings.Repeat("🍀", 20), 200)
	patch(member, mid, "", 200)
	patch(owner, mid, "\ufeff妈妈🍀\ufeff", 200)
	if !strings.Contains(call("GET", "/api/v1/auth/me", member, "", 200).Body.String(), `"nickname":"妈妈🍀"`) {
		t.Fatal("me missing nickname")
	}
	again, _ := login("member")
	if !strings.Contains(call("GET", "/api/v1/auth/me", again, "", 200).Body.String(), "妈妈🍀") {
		t.Fatal("nickname lost across login")
	}
	if !strings.Contains(call("GET", "/api/v1/family/members", owner, "", 200).Body.String(), "妈妈🍀") {
		t.Fatal("member list missing nickname")
	}
	directory := func() map[int64]map[string]any {
		w := call("GET", "/api/v1/entry-authors", owner, "", 200)
		var rows []map[string]any
		json.Unmarshal(w.Body.Bytes(), &rows)
		out := map[int64]map[string]any{}
		var last int64 = -1
		for _, r := range rows {
			id := int64(r["id"].(float64))
			if id <= last {
				t.Fatal("directory not unique/sorted")
			}
			last = id
			out[id] = r
		}
		if strings.Contains(w.Body.String(), "openid") || strings.Contains(w.Body.String(), "token") {
			t.Fatal("identity leaked")
		}
		return out
	}
	d := directory()
	if len(d) != 2 || d[mid]["nickname"] != "妈妈🍀" || d[mid]["status"] != "active" {
		t.Fatal("include current members without entries", d)
	}
	for i, creator := range []int64{0, mid, mid, oid} {
		_, _, err := data.Save(context.Background(), domain.Entry{CategoryID: 2, Kind: "expense", Amount: 100, CreatedBy: creator, OccurredAt: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC), RequestID: fmt.Sprintf("f05-%d", i)})
		if err != nil {
			t.Fatal(err)
		}
	}
	w := call("GET", fmt.Sprintf("/api/v1/entries?month=2026-10&created_by=%d&page_size=1", mid), member, "", 200)
	var page domain.EntryPage
	json.Unmarshal(w.Body.Bytes(), &page)
	if page.Total != 2 || len(page.Items) != 1 || page.Summary.Expense != 200 {
		t.Fatal("member totals across pagination", w.Body)
	}
	patch(member, mid, "=SUM(1)", 200)
	export := call("GET", fmt.Sprintf("/api/v1/entries/export?month=2026-10&created_by=%d", mid), member, "", 200)
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(export.Body.String(), "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || len(records[0]) != 10 || records[1][6] != fmt.Sprint(mid) || records[1][8] != "'=SUM(1)" || records[1][9] != "在家成员" {
		t.Fatal("CSV identity or injection", records)
	}
	call("DELETE", fmt.Sprintf("/api/v1/family/members/%d", mid), owner, "", 204)
	call("GET", "/api/v1/entry-authors", member, "", 403)
	patch(member, mid, "被移除", 403)
	patch(owner, mid, "不允许改历史成员", 404)
	d = directory()
	if d[mid]["nickname"] != "=SUM(1)" || d[mid]["status"] != "removed" || d[0]["status"] != "unspecified" {
		t.Fatal("historical identity lost", d)
	}
	call("GET", "/api/v1/entry-authors", stranger, "", 403)
	transferOnly, tid := login("transfer-only")
	join(transferOnly)
	patch(transferOnly, tid, "转账家人", 200)
	first, err := data.SaveAccount(context.Background(), domain.Account{Name: "现金", Type: "cash"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := data.SaveAccount(context.Background(), domain.Account{Name: "微信", Type: "wechat"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = data.SaveTransfer(context.Background(), domain.Transfer{FromAccountID: first.ID, ToAccountID: second.ID, Amount: 100, CreatedBy: tid, OccurredAt: time.Now(), RequestID: "transfer-only"}); err != nil {
		t.Fatal(err)
	}
	call("DELETE", fmt.Sprintf("/api/v1/family/members/%d", tid), owner, "", 204)
	if row := directory()[tid]; row["nickname"] != "转账家人" || row["status"] != "removed" {
		t.Fatal("transfer-only historical member missing", row)
	}
	join(member)
	if directory()[mid]["status"] != "active" {
		t.Fatal("rejoin status stale")
	}
}
