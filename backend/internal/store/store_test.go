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
