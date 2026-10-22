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
	invalid := `{"category_id":9,"kind":"expense","amount_cent":1800,"occurred_at":"2026-10-13T12:00:00+08:00","client_request_id":"bad"}`
	if w := call("POST", "/api/v1/entries", invalid, "test-token"); w.Code != 400 {
		t.Fatal("mismatched category accepted")
	}
	w := call("GET", "/api/v1/stats?month=2026-10", "", "test-token")
	var stats struct {
		Expense int64 `json:"expense_cent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil || stats.Expense != 1800 {
		t.Fatal("retry double counted")
	}
	if w = call("DELETE", "/api/v1/entries/1", "", "test-token"); w.Code != http.StatusNoContent {
		t.Fatal("delete failed")
	}
	w = call("GET", "/api/v1/stats?month=2026-10", "", "test-token")
	_ = json.Unmarshal(w.Body.Bytes(), &stats)
	if stats.Expense != 0 {
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
	for _, test := range []struct{ path, body string }{
		{"/api/v1/budgets/2026-13", `{"amount_cent":100}`},
		{"/api/v1/budgets/2026-10", `{"amount_cent":-1}`},
		{"/api/v1/budgets/2026-10", `{"amount_cent":1.5}`},
		{"/api/v1/budgets/2026-10", `{"amount_cent":100} {}`},
	} {
		if w := call("PUT", test.path, test.body); w.Code != 400 {
			t.Fatalf("invalid budget accepted: %s %s", test.path, test.body)
		}
	}
	if w := call("PUT", "/api/v1/budgets/2026-10", `{"amount_cent":0}`); w.Code != 200 {
		t.Fatal("explicit zero budget should be allowed")
	}
	w := call("GET", "/api/v1/stats?month=2026-10", "")
	var s struct {
		Budget    *int64 `json:"budget_cent"`
		Remaining *int64 `json:"remaining_cent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil || s.Budget == nil || s.Remaining == nil || *s.Budget != 0 {
		t.Fatal("zero budget must differ from unset budget")
	}
}
