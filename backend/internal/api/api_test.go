package api

import (
	"bytes"
	"cost/internal/store"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLedgerWorkflow(t *testing.T) {
	h := New(store.NewMemory(), "test-token")
	call := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("X-Dev-Token", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/v1/entries", "", "wrong"); w.Code != 401 {
		t.Fatal("unauthorized request accepted")
	}
	body := `{"category_id":2,"kind":"expense","amount_cent":1800,"occurred_at":"2026-10-13T12:00:00+08:00","note":"午餐","client_request_id":"unique-1"}`
	if w := call("POST", "/api/v1/entries", body, "test-token"); w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if w := call("POST", "/api/v1/entries", body, "test-token"); w.Code != 200 {
		t.Fatal("retry not idempotent")
	}
	invalid := `{"category_id":13,"kind":"expense","amount_cent":1800,"occurred_at":"2026-10-13T12:00:00+08:00","client_request_id":"bad"}`
	if w := call("POST", "/api/v1/entries", invalid, "test-token"); w.Code != 400 {
		t.Fatal("mismatched category accepted")
	}
	w := call("GET", "/api/v1/stats?range=month&ref=2026-10-13", "", "test-token")
	var stats struct {
		Summary struct {
			Expense int64 `json:"expense_cent"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil || stats.Summary.Expense != 1800 {
		t.Fatalf("retry double counted: %s", w.Body.String())
	}
	if w = call("DELETE", "/api/v1/entries/1", "", "test-token"); w.Code != http.StatusNoContent {
		t.Fatal("delete failed")
	}
	w = call("GET", "/api/v1/stats?range=month&ref=2026-10-13", "", "test-token")
	_ = json.Unmarshal(w.Body.Bytes(), &stats)
	if stats.Summary.Expense != 0 {
		t.Fatal("delete not reflected in stats")
	}
}

func TestValidationAndBudget(t *testing.T) {
	h := New(store.NewMemory(), "test-token")
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("X-Dev-Token", "test-token")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, test := range []struct{ body string }{
		{`{"scope":"month","period":"2026-13","category_id":0,"amount_cent":100}`},
		{`{"scope":"quarter","period":"2026-10","category_id":0,"amount_cent":100}`},
		{`{"scope":"month","period":"2026-10","category_id":0,"amount_cent":-1}`},
		{`{"scope":"month","period":"2026-10","category_id":0,"amount_cent":1.5}`},
		{`{"scope":"month","period":"2026-10","category_id":0,"amount_cent":100} {}`},
	} {
		if w := call("PUT", "/api/v1/budgets", test.body); w.Code != 400 {
			t.Fatalf("invalid budget accepted: %s", test.body)
		}
	}
	if w := call("PUT", "/api/v1/budgets", `{"scope":"month","period":"2026-10","category_id":0,"amount_cent":100}`); w.Code != 200 {
		t.Fatal("valid budget rejected")
	}
	w := call("GET", "/api/v1/budgets?scope=month&period=2026-10", "")
	var budgets struct {
		Total *int64 `json:"total_cent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &budgets); err != nil || budgets.Total == nil || *budgets.Total != 100 {
		t.Fatalf("budget not visible: %s", w.Body.String())
	}
	w = call("GET", "/api/v1/stats?range=month&ref=2026-10-13", "")
	var s struct {
		Budget struct {
			Total     *int64 `json:"total_cent"`
			Remaining *int64 `json:"remaining_cent"`
		} `json:"budget"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil || s.Budget.Total == nil || s.Budget.Remaining == nil || *s.Budget.Total != 100 || *s.Budget.Remaining != 100 {
		t.Fatalf("month stats budget missing: %s", w.Body.String())
	}
	if w := call("PUT", "/api/v1/budgets", `{"scope":"month","period":"2026-10","category_id":0,"amount_cent":0}`); w.Code != 200 {
		t.Fatal("deleting budget via zero amount rejected")
	}
	w = call("GET", "/api/v1/budgets?scope=month&period=2026-10", "")
	_ = json.Unmarshal(w.Body.Bytes(), &budgets)
	if budgets.Total != nil {
		t.Fatal("zero amount must delete the budget row")
	}
}
