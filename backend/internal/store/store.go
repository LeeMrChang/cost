package store

import (
	"context"
	"cost/internal/domain"
	"sort"
	"sync"
	"time"
)

type Store interface {
	Query(context.Context, domain.EntryFilter) (domain.EntryPage, error)
	Authors(context.Context) ([]domain.Author, error)
	PatchCategory(context.Context, int64, domain.CategoryPatch) (domain.Category, error)
	PatchAccount(context.Context, int64, domain.AccountPatch) (domain.Account, error)
	SaveCategory(context.Context, domain.Category) (domain.Category, error)
	Accounts(context.Context) ([]domain.Account, error)
	SaveAccount(context.Context, domain.Account) (domain.Account, error)
	SaveTransfer(context.Context, domain.Transfer) (domain.Transfer, bool, error)
	Transfers(context.Context, int, int) (domain.TransferPage, error)
	GetTransfer(context.Context, int64) (domain.Transfer, error)
	DeleteTransfer(context.Context, int64) error
	Get(context.Context, int64) (domain.Entry, error)
	Categories(context.Context) ([]domain.Category, error)
	List(context.Context, time.Time, time.Time) ([]domain.Entry, error)
	Save(context.Context, domain.Entry) (domain.Entry, bool, error)
	Update(context.Context, domain.Entry) (domain.Entry, error)
	Delete(context.Context, int64) error
	Budget(context.Context, string) (*int64, error)
	SetBudget(context.Context, string, int64) error
	Close() error
}

type Memory struct {
	mu                                      sync.RWMutex
	next                                    int64
	entries                                 map[int64]domain.Entry
	requests                                map[string]int64
	budgets                                 map[string]int64
	categories                              map[int64]domain.Category
	accounts                                map[int64]domain.Account
	transfers                               map[int64]domain.Transfer
	transferRequests                        map[string]int64
	nextCategory, nextAccount, nextTransfer int64
}

func NewMemory() *Memory {
	m := &Memory{next: 1, nextCategory: 12, nextAccount: 1, nextTransfer: 1, entries: map[int64]domain.Entry{}, requests: map[string]int64{}, budgets: map[string]int64{}, categories: map[int64]domain.Category{}, accounts: map[int64]domain.Account{}, transfers: map[int64]domain.Transfer{}, transferRequests: map[string]int64{}}
	for _, c := range domain.DefaultCategories() {
		m.categories[c.ID] = c
	}
	return m
}
func (m *Memory) Categories(context.Context) ([]domain.Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Category{}
	for _, c := range m.categories {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].ID < out[j].ID
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out, nil
}
func (m *Memory) List(_ context.Context, start, end time.Time) ([]domain.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Entry{}
	for _, e := range m.entries {
		if !e.OccurredAt.Before(start) && e.OccurredAt.Before(end) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OccurredAt.Equal(out[j].OccurredAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})
	return out, nil
}
func same(a, b domain.Entry) bool {
	return a.AccountID == b.AccountID && a.CreatedBy == b.CreatedBy && a.CategoryID == b.CategoryID && a.Kind == b.Kind && a.Amount == b.Amount && a.OccurredAt.Equal(b.OccurredAt) && a.Note == b.Note
}
func (m *Memory) Save(_ context.Context, e domain.Entry) (domain.Entry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.requests[e.RequestID]; ok {
		old := m.entries[id]
		if !same(old, e) {
			return domain.Entry{}, false, domain.ErrConflict
		}
		return old, false, nil
	}
	if err := m.validateReferences(e, domain.Entry{}); err != nil {
		return e, false, err
	}
	e.ID = m.next
	m.next++
	m.entries[e.ID] = e
	m.requests[e.RequestID] = e.ID
	return e, true, nil
}
func (m *Memory) Update(_ context.Context, e domain.Entry) (domain.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.entries[e.ID]
	if !ok {
		return e, domain.ErrNotFound
	}
	if err := m.validateReferences(e, old); err != nil {
		return e, err
	}
	e.RequestID = old.RequestID
	e.CreatedBy = old.CreatedBy
	m.entries[e.ID] = e
	return e, nil
}
func (m *Memory) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(m.requests, e.RequestID)
	delete(m.entries, id)
	return nil
}
func (m *Memory) Budget(_ context.Context, month string) (*int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.budgets[month]
	if !ok {
		return nil, nil
	}
	return &n, nil
}
func (m *Memory) SetBudget(_ context.Context, month string, n int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budgets[month] = n
	return nil
}
func (m *Memory) Close() error { return nil }

func (m *Memory) Get(_ context.Context, id int64) (domain.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[id]
	if !ok {
		return e, domain.ErrNotFound
	}
	return e, nil
}
