package api

import (
	"bytes"
	"cost/internal/store"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func p0Call(t *testing.T, h http.Handler, method, path string, body any, status int) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("X-Dev-Token", "test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: want %d got %d %s", method, path, status, w.Code, w.Body.String())
	}
	return w
}
func testP0(t *testing.T, s store.Store) {
	h := New(s, "test")
	call := func(m, p string, b any, status int) *httptest.ResponseRecorder { return p0Call(t, h, m, p, b, status) }
	a := map[string]any{}
	json.Unmarshal(call("POST", "/api/v1/accounts", map[string]any{"name": "现金", "type": "cash", "opening_balance_cent": 10000}, 201).Body.Bytes(), &a)
	b := map[string]any{}
	json.Unmarshal(call("POST", "/api/v1/accounts", map[string]any{"name": "银行", "type": "bank", "opening_balance_cent": 20000}, 201).Body.Bytes(), &b)
	cat := map[string]any{}
	json.Unmarshal(call("POST", "/api/v1/categories", map[string]any{"name": "家庭用品", "kind": "expense", "icon": "🛒", "sort_order": 1}, 201).Body.Bytes(), &cat)
	e := map[string]any{"account_id": a["id"], "category_id": cat["id"], "kind": "expense", "amount_cent": 1800, "occurred_at": "2026-09-30T23:59:59+08:00", "note": "=1+2,中文\n第二行%_", "client_request_id": "p0-1"}
	saved := map[string]any{}
	json.Unmarshal(call("POST", "/api/v1/entries", e, 201).Body.Bytes(), &saved)
	e["client_request_id"] = "p0-2"
	e["occurred_at"] = "2026-10-01T00:00:00+08:00"
	call("POST", "/api/v1/entries", e, 201)
	q := "/api/v1/entries?start_date=2026-09-30&end_date=2026-10-01&q=%25_&page_size=1"
	var list struct {
		Items   []map[string]any
		Total   int
		Summary struct {
			Expense int64 `json:"expense_cent"`
		}
	}
	json.Unmarshal(call("GET", q, nil, 200).Body.Bytes(), &list)
	if len(list.Items) != 1 || list.Total != 2 || list.Summary.Expense != 3600 {
		t.Fatalf("filtered totals %+v", list)
	}
	call("GET", q+"&page=2", nil, 200)
	json.Unmarshal(call("GET", "/api/v1/entries?start_date=2026-09-30&end_date=2026-10-01&q="+url.QueryEscape("家庭"), nil, 200).Body.Bytes(), &list)
	if len(list.Items) != 2 || list.Total != 2 {
		t.Fatalf("category-name keyword: %+v", list)
	}
	json.Unmarshal(call("GET", "/api/v1/entries?start_date=2026-09-30&end_date=2026-10-01&q="+url.QueryEscape("不存在的词"), nil, 200).Body.Bytes(), &list)
	if list.Total != 0 {
		t.Fatalf("missing keyword: %+v", list)
	}
	for _, bad := range []string{"page=0", "page_size=101", "min_amount_cent=-1", "min_amount_cent=20&max_amount_cent=10", "created_by=wat", "account_id=-1", "category_id=0", "kind=transfer", "start_date=2026-02-30", "start_date=2026-10-02&end_date=2026-10-01"} {
		call("GET", "/api/v1/entries?"+bad, nil, 400)
	}
	w := call("GET", strings.Replace(q, "/entries?", "/entries/export?", 1), nil, 200)
	if !strings.HasPrefix(w.Body.String(), "\ufeff") {
		t.Fatal("CSV needs UTF-8 BOM")
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(w.Body.String(), "\ufeff"))).ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatalf("csv %v %v", rows, err)
	}
	if !strings.Contains(w.Body.String(), "'=1+2") {
		t.Fatal("formula not neutralized")
	}
	transfer := map[string]any{"from_account_id": a["id"], "to_account_id": b["id"], "amount_cent": 1000, "occurred_at": "2026-10-01T12:00:00+08:00", "note": "转账", "client_request_id": "transfer-1"}
	tr := map[string]any{}
	json.Unmarshal(call("POST", "/api/v1/transfers", transfer, 201).Body.Bytes(), &tr)
	call("POST", "/api/v1/transfers", transfer, 200)
	var accounts []struct {
		ID      int64
		Balance int64 `json:"balance_cent"`
	}
	json.Unmarshal(call("GET", "/api/v1/accounts", nil, 200).Body.Bytes(), &accounts)
	if accounts[0].Balance != 5400 || accounts[1].Balance != 21000 {
		t.Fatalf("balances %+v", accounts)
	}
	call("PATCH", fmt.Sprintf("/api/v1/categories/%.0f", cat["id"]), map[string]any{"disabled": true, "name": "家用"}, 200)
	call("PATCH", fmt.Sprintf("/api/v1/accounts/%.0f", a["id"]), map[string]any{"disabled": true}, 200)
	e["client_request_id"] = "p0-new"
	call("POST", "/api/v1/entries", e, 400)
	e["amount_cent"] = 1000
	call("PATCH", fmt.Sprintf("/api/v1/entries/%.0f", saved["id"]), e, 200)
	transfer["client_request_id"] = "transfer-disabled"
	call("POST", "/api/v1/transfers", transfer, 400)
	call("DELETE", fmt.Sprintf("/api/v1/transfers/%.0f", tr["id"]), nil, 204)
	call("DELETE", fmt.Sprintf("/api/v1/entries/%.0f", saved["id"]), nil, 204)
	json.Unmarshal(call("GET", "/api/v1/accounts", nil, 200).Body.Bytes(), &accounts)
	if accounts[0].Balance != 8200 || accounts[1].Balance != 20000 {
		t.Fatalf("delete balance %+v", accounts)
	}
	call("GET", "/api/v1/entry-authors", nil, 200)
}
func TestP0Memory(t *testing.T) { testP0(t, store.NewMemory()) }
func TestP0MySQL(t *testing.T)  { testP0(t, testMySQL(t, false)) }
