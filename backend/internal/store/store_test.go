package store

import (
	"context"
	"cost/internal/domain"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestConcurrentRetriesCreateOneEntry(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	e := domain.Entry{CategoryID: 2, Kind: "expense", Amount: 1800, OccurredAt: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC), RequestID: "same-request"}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := m.Save(ctx, e); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	start, end, _ := domain.Bounds("2026-10")
	entries, err := m.List(ctx, start, end)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one entry, got %d, error %v", len(entries), err)
	}
	e.Amount = 1900
	if _, _, err = m.Save(ctx, e); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("same request with different amount must conflict")
	}
}

func TestUpdateAndMonthBoundaries(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	start, end, _ := domain.Bounds("2026-10")
	for i, at := range []time.Time{start.Add(-time.Microsecond), start, end.Add(-time.Microsecond), end} {
		if _, _, err := m.Save(ctx, domain.Entry{CategoryID: 2, Kind: "expense", Amount: 1800, OccurredAt: at, RequestID: string(rune('a' + i))}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := m.List(ctx, start, end)
	if err != nil || len(entries) != 2 {
		t.Fatal("month interval must be half-open")
	}
	e := entries[0]
	e.OccurredAt = end
	if _, err = m.Update(ctx, e); err != nil {
		t.Fatal(err)
	}
	entries, _ = m.List(ctx, start, end)
	if len(entries) != 1 {
		t.Fatal("moving entry to next month must update list")
	}
}

func TestSoftDeleteExpiryAndLedgerIsolation(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	e, _, err := m.Save(ctx, domain.Entry{CategoryID: 1, Kind: "expense", Amount: 100, OccurredAt: time.Now(), RequestID: "expiry"})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Delete(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.Restore(WithLedgerID(ctx, 99), e.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross ledger restore: %v", err)
	}
	m.deletedAt[e.ID] = time.Now().Add(-6 * time.Second)
	if err = m.Restore(ctx, e.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expired restore: %v", err)
	}
	if _, _, err = m.Save(ctx, e); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("retry resurrected tombstone: %v", err)
	}
	entries, _ := m.List(ctx, time.Time{}, time.Time{})
	if len(entries) != 0 {
		t.Fatal("deleted entry visible in list")
	}
	authors, _ := m.Authors(ctx)
	if len(authors) != 0 {
		t.Fatal("deleted entry visible in authors")
	}
}
