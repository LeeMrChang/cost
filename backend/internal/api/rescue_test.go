package api

import (
	"bytes"
	"context"
	"cost/internal/domain"
	"cost/internal/store"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func rescueCall(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.Header.Set("X-Dev-Token", "rescue")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func rescueWorkflow(t *testing.T, s store.Store) {
	t.Helper()
	ctx := context.Background()
	h := New(s, "rescue")
	a, err := s.SaveAccount(ctx, domain.Account{Name: "Cash", Type: "cash", OpeningBalance: 10000})
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"category_id":1,"account_id":%d,"kind":"expense","amount_cent":1200,"occurred_at":"2026-10-26T12:00:00+08:00","note":"before","client_request_id":"rescue-entry","reimbursable":true,"exclude_budget":true}`, a.ID)
	w := rescueCall(h, "POST", "/api/v1/entries", body)
	if w.Code != 201 {
		t.Fatalf("create: %s", w.Body.String())
	}
	var e domain.Entry
	json.Unmarshal(w.Body.Bytes(), &e)
	path := fmt.Sprintf("/api/v1/entries/%d", e.ID)
	patch := `{"category_id":1,"kind":"expense","amount_cent":1500,"occurred_at":"2026-10-26T12:00:00+08:00","note":"after"}`
	w = rescueCall(h, "PATCH", path, patch)
	if w.Code != 200 {
		t.Fatalf("patch: %s", w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &e)
	if e.AccountID != a.ID || !e.Reimbursable || !e.ExcludeBudget {
		t.Errorf("omitted fields lost: %+v", e)
	}
	w = rescueCall(h, "GET", path, "")
	if w.Code != 200 {
		t.Errorf("detail: %d", w.Code)
	}
	if w = rescueCall(h, "DELETE", path, ""); w.Code != 204 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if w = rescueCall(h, "GET", path, ""); w.Code != 404 {
		t.Errorf("deleted entry visible: %d", w.Code)
	}
	q, err := s.Query(ctx, domain.EntryFilter{Page: 1, PageSize: 50})
	if err != nil || q.Total != 0 || q.Summary.Expense != 0 {
		t.Errorf("deleted in search: %+v %v", q, err)
	}
	ac, _ := s.Accounts(ctx)
	if ac[0].Balance != 10000 {
		t.Errorf("deleted in balance: %+v", ac)
	}
	if w = rescueCall(h, "POST", path+"/restore", ""); w.Code != 200 {
		t.Fatalf("restore: %d %s", w.Code, w.Body.String())
	}
	q, _ = s.Query(ctx, domain.EntryFilter{Page: 1, PageSize: 50})
	if q.Total != 1 || q.Summary.Expense != 1500 {
		t.Errorf("restore totals: %+v", q)
	}
	ac, _ = s.Accounts(ctx)
	if ac[0].Balance != 8500 {
		t.Errorf("restore balance: %+v", ac)
	}
	other := store.WithLedgerID(ctx, 999)
	if _, err = s.Get(other, e.ID); err != domain.ErrNotFound {
		t.Errorf("cross ledger get: %v", err)
	}
}
func TestRescueMemory(t *testing.T) { rescueWorkflow(t, store.NewMemory()) }
func TestRescueMySQL(t *testing.T)  { rescueWorkflow(t, testMySQL(t, false)) }
func TestBudgetIncludesSpendingWithoutCategoryBudget(t *testing.T) {
	s := store.NewMemory()
	ctx := context.Background()
	_, _, err := s.Save(ctx, domain.Entry{CategoryID: 1, Kind: "expense", Amount: 8000, OccurredAt: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), RequestID: "budget-unset"})
	if err != nil {
		t.Fatal(err)
	}
	w := rescueCall(New(s, "rescue"), "GET", "/api/v1/budgets?scope=month&period=2026-10", "")
	var out struct {
		Categories []struct {
			ID    int64 `json:"category_id"`
			Spent int64 `json:"spent_cent"`
			Set   bool  `json:"is_set"`
		} `json:"categories"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, c := range out.Categories {
		if c.ID == 1 {
			if c.Spent != 8000 || c.Set {
				t.Fatalf("unset budget spending: %+v", c)
			}
			return
		}
	}
	t.Fatal("expense category missing when budget unset")
}

func TestMySQLSoftDeleteMigrationResumes(t *testing.T) {
	db := legacySchema(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec("DELETE FROM t_schema_migrations WHERE version='006_entry_soft_delete.sql'"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	e, err := db.Get(ctx, 42)
	if err != nil || e.Amount != 1234 {
		t.Fatalf("legacy entry lost: %+v %v", e, err)
	}
	if err = db.Delete(ctx, 42); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.DB().QueryRow("SELECT COUNT(*) FROM t_entries WHERE id=42 AND deleted_at IS NOT NULL").Scan(&count); err != nil || count != 1 {
		t.Fatalf("not a soft deletion: %d %v", count, err)
	}
	if _, err = db.DB().Exec("UPDATE t_entries SET deleted_at=UTC_TIMESTAMP(6)-INTERVAL 6 SECOND WHERE id=42"); err != nil {
		t.Fatal(err)
	}
	if err = db.Restore(ctx, 42); err != domain.ErrConflict {
		t.Fatalf("expired restore: %v", err)
	}
}
