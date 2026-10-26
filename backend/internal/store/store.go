package store

import (
	"context"
	"cost/internal/domain"
	"fmt"
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
	Restore(context.Context, int64) error
	Budgets(context.Context, string, string) ([]domain.BudgetRow, error)
	SetBudget(context.Context, domain.BudgetRow) error
	LedgerForUser(context.Context, int64) (int64, error)
	Ledger(context.Context, int64) (domain.Ledger, error)
	Close() error
}

// Memory 是单进程内存实现：账本 1 为开发/直接调用默认账本，其余账本按用户按需创建。
type Memory struct {
	mu                                      sync.RWMutex
	next                                    int64
	entries                                 map[int64]domain.Entry
	deletedAt                               map[int64]time.Time
	requests                                map[string]int64
	budgets                                 map[int64]map[string]map[int64]int64
	categories                              map[int64]domain.Category
	accounts                                map[int64]domain.Account
	transfers                               map[int64]domain.Transfer
	transferRequests                        map[string]int64
	ledgers                                 map[int64]domain.Ledger
	ledgerOwner                             map[int64]int64
	entryLedger                             map[int64]int64
	categoryLedger                          map[int64]int64
	accountLedger                           map[int64]int64
	transferLedger                          map[int64]int64
	nextCategory, nextAccount, nextTransfer int64
	nextLedger                              int64
}

func NewMemory() *Memory {
	m := &Memory{deletedAt: map[int64]time.Time{}, next: 1, nextCategory: int64(len(domain.DefaultCategories()) + 1), nextAccount: 1, nextTransfer: 1, nextLedger: 2, entries: map[int64]domain.Entry{}, requests: map[string]int64{}, budgets: map[int64]map[string]map[int64]int64{}, categories: map[int64]domain.Category{}, accounts: map[int64]domain.Account{}, transfers: map[int64]domain.Transfer{}, transferRequests: map[string]int64{}, ledgers: map[int64]domain.Ledger{1: {ID: 1, Name: domain.DefaultLedgerName(), Icon: domain.DefaultLedgerIcon(), Currency: "CNY", Timezone: "Asia/Shanghai"}}, ledgerOwner: map[int64]int64{}, entryLedger: map[int64]int64{}, categoryLedger: map[int64]int64{}, accountLedger: map[int64]int64{}, transferLedger: map[int64]int64{}}
	for _, c := range domain.DefaultCategories() {
		m.categories[c.ID] = c
		m.categoryLedger[c.ID] = 1
	}
	return m
}
func (m *Memory) Categories(ctx context.Context) ([]domain.Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ledgerID := LedgerID(ctx)
	out := []domain.Category{}
	for _, c := range m.categories {
		if m.categoryLedger[c.ID] == ledgerID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].ID < out[j].ID
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out, nil
}
func (m *Memory) List(ctx context.Context, start, end time.Time) ([]domain.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ledgerID := LedgerID(ctx)
	out := []domain.Entry{}
	for _, e := range m.entries {
		if m.entryLedger[e.ID] != ledgerID || !m.deletedAt[e.ID].IsZero() {
			continue
		}
		if (!start.IsZero() && e.OccurredAt.Before(start)) || (!end.IsZero() && !e.OccurredAt.Before(end)) {
			continue
		}
		out = append(out, e)
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
	return a.AccountID == b.AccountID && a.CreatedBy == b.CreatedBy && a.CategoryID == b.CategoryID && a.Kind == b.Kind && a.Amount == b.Amount && a.OccurredAt.Equal(b.OccurredAt) && a.Note == b.Note && a.RefundOfID == b.RefundOfID && a.Reimbursable == b.Reimbursable && a.ExcludeStats == b.ExcludeStats && a.ExcludeBudget == b.ExcludeBudget && a.Currency == b.Currency
}
func requestKey(ledgerID int64, requestID string) string {
	return fmt.Sprintf("%d|%s", ledgerID, requestID)
}

func (m *Memory) Save(ctx context.Context, e domain.Entry) (domain.Entry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.Currency == "" {
		e.Currency = "CNY"
	}
	if id, ok := m.requests[requestKey(LedgerID(ctx), e.RequestID)]; ok {
		old := m.entries[id]
		if !m.deletedAt[id].IsZero() {
			return domain.Entry{}, false, domain.ErrConflict
		}
		if !same(old, e) {
			return domain.Entry{}, false, domain.ErrConflict
		}
		return old, false, nil
	}
	if err := m.validateReferences(ctx, e, domain.Entry{}); err != nil {
		return e, false, err
	}
	e.ID = m.next
	m.next++
	m.entries[e.ID] = e
	m.entryLedger[e.ID] = LedgerID(ctx)
	m.requests[requestKey(LedgerID(ctx), e.RequestID)] = e.ID
	return e, true, nil
}
func (m *Memory) Update(ctx context.Context, e domain.Entry) (domain.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.entries[e.ID]
	if !ok || m.entryLedger[e.ID] != LedgerID(ctx) || !m.deletedAt[e.ID].IsZero() {
		return e, domain.ErrNotFound
	}
	if e.Currency == "" {
		e.Currency = "CNY"
	}
	if err := m.validateReferences(ctx, e, old); err != nil {
		return e, err
	}
	e.RequestID = old.RequestID
	e.CreatedBy = old.CreatedBy
	m.entries[e.ID] = e
	return e, nil
}
func (m *Memory) Delete(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.entries[id]; !ok || m.entryLedger[id] != LedgerID(ctx) || !m.deletedAt[id].IsZero() {
		return domain.ErrNotFound
	}
	m.deletedAt[id] = time.Now()
	return nil
}
func (m *Memory) Restore(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	deleted := m.deletedAt[id]
	if m.entryLedger[id] != LedgerID(ctx) || deleted.IsZero() {
		return domain.ErrNotFound
	}
	if time.Since(deleted) > 5*time.Second {
		return domain.ErrConflict
	}
	delete(m.deletedAt, id)
	return nil
}
func (m *Memory) Budgets(ctx context.Context, scope, period string) ([]domain.BudgetRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.BudgetRow{}
	for categoryID, amount := range m.budgets[LedgerID(ctx)][budgetKey(scope, period)] {
		out = append(out, domain.BudgetRow{Scope: scope, Period: period, CategoryID: categoryID, Amount: amount})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CategoryID < out[j].CategoryID })
	return out, nil
}
func (m *Memory) SetBudget(ctx context.Context, row domain.BudgetRow) error {
	if err := domain.ValidateBudgetRow(row); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ledgerID := LedgerID(ctx)
	if m.budgets[ledgerID] == nil {
		m.budgets[ledgerID] = map[string]map[int64]int64{}
	}
	key := budgetKey(row.Scope, row.Period)
	if m.budgets[ledgerID][key] == nil {
		m.budgets[ledgerID][key] = map[int64]int64{}
	}
	if row.Amount == 0 {
		delete(m.budgets[ledgerID][key], row.CategoryID)
		return nil
	}
	m.budgets[ledgerID][key][row.CategoryID] = row.Amount
	return nil
}
func budgetKey(scope, period string) string { return scope + "|" + period }

func (m *Memory) LedgerForUser(_ context.Context, userID int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.ledgerOwner[userID]; ok {
		return id, nil
	}
	id := m.nextLedger
	m.nextLedger++
	m.ledgerOwner[userID] = id
	m.ledgers[id] = domain.Ledger{ID: id, Name: domain.DefaultLedgerName(), Icon: domain.DefaultLedgerIcon(), Currency: "CNY", Timezone: "Asia/Shanghai"}
	for _, c := range domain.DefaultCategories() {
		c.ID = m.nextCategory
		m.nextCategory++
		m.categories[c.ID] = c
		m.categoryLedger[c.ID] = id
	}
	return id, nil
}
func (m *Memory) Ledger(_ context.Context, id int64) (domain.Ledger, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.ledgers[id]
	if !ok {
		return l, domain.ErrNotFound
	}
	return l, nil
}
func (m *Memory) Close() error { return nil }

func (m *Memory) Get(ctx context.Context, id int64) (domain.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[id]
	if !ok || m.entryLedger[id] != LedgerID(ctx) || !m.deletedAt[id].IsZero() {
		return e, domain.ErrNotFound
	}
	return e, nil
}
