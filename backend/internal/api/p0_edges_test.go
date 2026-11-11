package api

import (
	"context"
	"cost/internal/domain"
	"cost/internal/store"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestBillExportRemoved(t *testing.T) {
	// The removed path is handled as an invalid entry ID, never as a CSV export.
	p0Call(t, New(store.NewMemory(), "test"), "GET", "/api/v1/entries/export?month=2026-10", nil, 400)
}
func testP0Concurrent(t *testing.T, s store.Store) {
	ctx := context.Background()
	a, err := s.SaveAccount(ctx, domain.Account{Name: "现金", Type: "cash", OpeningBalance: 10000})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.SaveAccount(ctx, domain.Account{Name: "银行", Type: "bank"})
	if err != nil {
		t.Fatal(err)
	}
	tr := domain.Transfer{FromAccountID: a.ID, ToAccountID: b.ID, Amount: 123, OccurredAt: time.Now().Truncate(time.Microsecond), RequestID: "concurrent"}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := s.SaveTransfer(ctx, tr); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	list, err := s.Transfers(ctx, 1, 50)
	if err != nil || list.Total != 1 {
		t.Fatalf("transfers %+v %v", list, err)
	}
	accounts, err := s.Accounts(ctx)
	if err != nil || accounts[0].Balance != 9877 || accounts[1].Balance != 123 {
		t.Fatalf("balances %+v %v", accounts, err)
	}
	tr.Amount++
	if _, _, err = s.SaveTransfer(ctx, tr); err != domain.ErrConflict {
		t.Fatalf("conflicting retry %v", err)
	}
	a.Disabled = true
	if _, err = s.SaveAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	tr.Amount--
	if _, created, err := s.SaveTransfer(ctx, tr); err != nil || created {
		t.Fatalf("retry after disable %v %v", created, err)
	}
}
func TestP0ConcurrentMemory(t *testing.T) { testP0Concurrent(t, store.NewMemory()) }
func TestP0ConcurrentMySQL(t *testing.T)  { testP0Concurrent(t, testMySQL(t, false)) }
func testFilterBoundaries(t *testing.T, s store.Store) {
	ctx := context.Background()
	start, _, _ := domain.Bounds("2026-10")
	for i, at := range []time.Time{start.Add(-time.Microsecond), start, start.AddDate(0, 0, 1).Add(-time.Microsecond), start.AddDate(0, 0, 1)} {
		_, _, err := s.Save(ctx, domain.Entry{CategoryID: 2, Kind: "expense", Amount: int64(100 + i), OccurredAt: at, Note: "%' OR 1=1 --_", RequestID: fmt.Sprintf("boundary-%d", i)})
		if err != nil {
			t.Fatal(err)
		}
	}
	h := New(s, "test")
	q := "/api/v1/entries?start_date=2026-10-01&end_date=2026-10-01&category_id=2&account_id=0&created_by=0&kind=expense&min_amount_cent=101&max_amount_cent=102&page_size=1"
	var p domain.EntryPage
	w := p0Call(t, h, "GET", q, nil, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Total != 2 || p.Summary.Expense != 203 || len(p.Items) != 1 || p.Items[0].Amount != 102 {
		t.Fatalf("boundary filtered page %+v", p)
	}
	w = p0Call(t, h, "GET", q+"&page=2", nil, 200)
	json.Unmarshal(w.Body.Bytes(), &p)
	if len(p.Items) != 1 || p.Items[0].Amount != 101 || p.Summary.Expense != 203 {
		t.Fatalf("second page %+v", p)
	}
	w = p0Call(t, h, "GET", q+"&q=%25%27%20OR%201%3D1%20--_", nil, 200)
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.Total != 2 {
		t.Fatalf("literal query %+v", p)
	}
	w = p0Call(t, h, "GET", q+"&q=missing", nil, 200)
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.Total != 0 || len(p.Items) != 0 || p.Summary.Expense != 0 {
		t.Fatalf("empty %+v", p)
	}
}
func TestFilterBoundariesMemory(t *testing.T) { testFilterBoundaries(t, store.NewMemory()) }
func TestFilterBoundariesMySQL(t *testing.T)  { testFilterBoundaries(t, testMySQL(t, false)) }
func TestMySQLFeaturesMigrationResume(t *testing.T) {
	db := legacySchema(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	// Model interruption before the version marker, with only the earlier ALTERs done.
	for _, q := range []string{"DELETE FROM t_schema_migrations WHERE version='004_ledger_features.sql'", "ALTER TABLE t_entries DROP FOREIGN KEY fk_entry_account", "ALTER TABLE t_entries DROP INDEX idx_entry_account_time", "ALTER TABLE t_categories DROP COLUMN disabled"} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	assertPrefixed(t, db)
	e, err := db.Get(ctx, 42)
	if err != nil || e.Amount != 1234 || e.AccountID != 0 || e.CreatedBy != 7 {
		t.Fatalf("legacy damaged %+v %v", e, err)
	}
}

func TestMySQLMergeRejectsFinancialAccounts(t *testing.T) {
	ctx := context.Background()
	source := testMySQL(t, false)
	target := testMySQL(t, false)
	var name string
	if err := source.DB().QueryRow("SELECT DATABASE()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if _, err := source.SaveAccount(ctx, domain.Account{Name: "银行", Type: "bank", OpeningBalance: 12345}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.MergeLedger(ctx, name); err == nil {
		t.Fatal("merge silently discarded payment accounts")
	}
}

type concurrentReadStore struct {
	store.Store
	mu    sync.Mutex
	count int
	gate  chan struct{}
}

func (s *concurrentReadStore) barrier() {
	s.mu.Lock()
	s.count++
	if s.count == 2 {
		close(s.gate)
	}
	s.mu.Unlock()
	select {
	case <-s.gate:
	case <-time.After(time.Second):
	}
}
func (s *concurrentReadStore) Categories(ctx context.Context) ([]domain.Category, error) {
	out, err := s.Store.Categories(ctx)
	s.barrier()
	return out, err
}
func (s *concurrentReadStore) Accounts(ctx context.Context) ([]domain.Account, error) {
	out, err := s.Store.Accounts(ctx)
	s.barrier()
	return out, err
}
func TestConcurrentPartialMetadataPatch(t *testing.T) {
	for _, kind := range []string{"categories", "accounts"} {
		t.Run(kind, func(t *testing.T) {
			data := store.NewMemory()
			id := int64(2)
			if kind == "accounts" {
				a, err := data.SaveAccount(context.Background(), domain.Account{Name: "before", Type: "cash"})
				if err != nil {
					t.Fatal(err)
				}
				id = a.ID
			}
			proxy := &concurrentReadStore{Store: data, gate: make(chan struct{})}
			h := New(proxy, "test")
			var wg sync.WaitGroup
			for _, patch := range []map[string]any{{"name": "after"}, {"disabled": true}} {
				wg.Add(1)
				go func(patch map[string]any) {
					defer wg.Done()
					p0Call(t, h, "PATCH", fmt.Sprintf("/api/v1/%s/%d", kind, id), patch, 200)
				}(patch)
			}
			wg.Wait()
			if kind == "categories" {
				cats, _ := data.Categories(context.Background())
				for _, c := range cats {
					if c.ID == id && (c.Name != "after" || !c.Disabled) {
						t.Fatalf("partial patch lost change: %+v", c)
					}
				}
			} else {
				accounts, _ := data.Accounts(context.Background())
				if accounts[0].Name != "after" || !accounts[0].Disabled {
					t.Fatalf("partial patch lost change: %+v", accounts[0])
				}
			}
		})
	}
}
