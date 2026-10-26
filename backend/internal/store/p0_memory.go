package store

import (
	"context"
	"cost/internal/domain"
	"sort"
	"strings"
)

func (m *Memory) validateReferences(ctx context.Context, e, old domain.Entry) error {
	ledgerID := LedgerID(ctx)
	c, ok := m.categories[e.CategoryID]
	if !ok || m.categoryLedger[e.CategoryID] != ledgerID || c.Kind != e.Kind || (c.Disabled && old.CategoryID != c.ID) {
		return domain.ErrInvalid
	}
	if e.AccountID < 0 {
		return domain.ErrInvalid
	}
	if e.AccountID != 0 {
		a, ok := m.accounts[e.AccountID]
		if !ok || m.accountLedger[e.AccountID] != ledgerID || (a.Disabled && old.AccountID != a.ID) {
			return domain.ErrInvalid
		}
	}
	return domain.Validate(e, []domain.Category{c})
}
func (m *Memory) SaveCategory(ctx context.Context, c domain.Category) (domain.Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := domain.ValidateCategory(c); err != nil {
		return c, err
	}
	ledgerID := LedgerID(ctx)
	if c.ID == 0 {
		c.ID = m.nextCategory
		m.nextCategory++
	} else {
		old, ok := m.categories[c.ID]
		if !ok || m.categoryLedger[c.ID] != ledgerID {
			return c, domain.ErrNotFound
		}
		if old.Kind != c.Kind {
			return c, domain.ErrInvalid
		}
	}
	m.categories[c.ID] = c
	m.categoryLedger[c.ID] = ledgerID
	return c, nil
}
func (m *Memory) SaveAccount(ctx context.Context, a domain.Account) (domain.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := domain.ValidateAccount(a); err != nil {
		return a, err
	}
	ledgerID := LedgerID(ctx)
	if a.ID == 0 {
		a.ID = m.nextAccount
		m.nextAccount++
	} else {
		if _, ok := m.accounts[a.ID]; !ok || m.accountLedger[a.ID] != ledgerID {
			return a, domain.ErrNotFound
		}
	}
	m.accounts[a.ID] = a
	m.accountLedger[a.ID] = ledgerID
	a.Balance = m.balance(a)
	return a, nil
}
func (m *Memory) balance(a domain.Account) int64 {
	n := a.OpeningBalance
	for _, e := range m.entries {
		if e.AccountID == a.ID && m.deletedAt[e.ID].IsZero() {
			if e.Kind == "income" {
				n += e.Amount
			} else {
				n -= e.Amount
			}
		}
	}
	for _, t := range m.transfers {
		if t.FromAccountID == a.ID {
			n -= t.Amount
		}
		if t.ToAccountID == a.ID {
			n += t.Amount
		}
	}
	return n
}
func (m *Memory) Accounts(ctx context.Context) ([]domain.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ledgerID := LedgerID(ctx)
	out := []domain.Account{}
	for _, a := range m.accounts {
		if m.accountLedger[a.ID] != ledgerID {
			continue
		}
		a.Balance = m.balance(a)
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func sameTransfer(a, b domain.Transfer) bool {
	return a.CreatedBy == b.CreatedBy && a.FromAccountID == b.FromAccountID && a.ToAccountID == b.ToAccountID && a.Amount == b.Amount && a.OccurredAt.Equal(b.OccurredAt) && a.Note == b.Note
}
func (m *Memory) SaveTransfer(ctx context.Context, t domain.Transfer) (domain.Transfer, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := domain.ValidateTransfer(t); err != nil {
		return t, false, err
	}
	ledgerID := LedgerID(ctx)
	if id, ok := m.transferRequests[requestKey(ledgerID, t.RequestID)]; ok {
		old := m.transfers[id]
		if !sameTransfer(old, t) {
			return t, false, domain.ErrConflict
		}
		return old, false, nil
	}
	for _, id := range []int64{t.FromAccountID, t.ToAccountID} {
		a, ok := m.accounts[id]
		if !ok || m.accountLedger[id] != ledgerID || a.Disabled {
			return t, false, domain.ErrInvalid
		}
	}
	t.ID = m.nextTransfer
	m.nextTransfer++
	m.transfers[t.ID] = t
	m.transferLedger[t.ID] = ledgerID
	m.transferRequests[requestKey(ledgerID, t.RequestID)] = t.ID
	return t, true, nil
}
func (m *Memory) GetTransfer(ctx context.Context, id int64) (domain.Transfer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.transfers[id]
	if !ok || m.transferLedger[id] != LedgerID(ctx) {
		return t, domain.ErrNotFound
	}
	return t, nil
}
func (m *Memory) DeleteTransfer(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.transfers[id]
	if !ok || m.transferLedger[id] != LedgerID(ctx) {
		return domain.ErrNotFound
	}
	delete(m.transferRequests, requestKey(LedgerID(ctx), t.RequestID))
	delete(m.transferLedger, id)
	delete(m.transfers, id)
	return nil
}
func (m *Memory) Transfers(ctx context.Context, page, size int) (domain.TransferPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ledgerID := LedgerID(ctx)
	out := []domain.Transfer{}
	for _, t := range m.transfers {
		if m.transferLedger[t.ID] == ledgerID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OccurredAt.Equal(out[j].OccurredAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})
	total := len(out)
	start, end := pageBounds(page, size, total)
	return domain.TransferPage{Items: out[start:end], Total: total, Page: page, PageSize: size}, nil
}
func pageBounds(page, size, total int) (int, int) {
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	return start, end
}
func (m *Memory) Query(ctx context.Context, f domain.EntryFilter) (domain.EntryPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ledgerID := LedgerID(ctx)
	q := f.Query
	f.Query = ""
	queryCats := map[int64]bool{}
	if q != "" {
		for id, c := range m.categories {
			if m.categoryLedger[id] == ledgerID && strings.Contains(c.Name, q) {
				queryCats[id] = true
			}
		}
	}
	out := []domain.Entry{}
	for _, e := range m.entries {
		if m.entryLedger[e.ID] == ledgerID && m.deletedAt[e.ID].IsZero() && f.Match(e) && (q == "" || strings.Contains(e.Note, q) || queryCats[e.CategoryID]) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OccurredAt.Equal(out[j].OccurredAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})
	total := len(out)
	s := domain.Summarize(out, nil)
	start, end := pageBounds(f.Page, f.PageSize, total)
	return domain.EntryPage{Items: out[start:end], Total: total, Page: f.Page, PageSize: f.PageSize, Summary: s}, nil
}
func (m *Memory) Authors(ctx context.Context) ([]domain.Author, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ledgerID := LedgerID(ctx)
	ids := map[int64]bool{}
	for _, e := range m.entries {
		if m.entryLedger[e.ID] == ledgerID && m.deletedAt[e.ID].IsZero() {
			ids[e.CreatedBy] = true
		}
	}
	out := []domain.Author{}
	for id := range ids {
		out = append(out, domain.Author{ID: id})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) PatchCategory(ctx context.Context, id int64, p domain.CategoryPatch) (domain.Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.categories[id]
	if !ok || m.categoryLedger[id] != LedgerID(ctx) {
		return old, domain.ErrNotFound
	}
	c := p.Apply(old)
	if c.Kind != old.Kind {
		return c, domain.ErrInvalid
	}
	if err := domain.ValidateCategory(c); err != nil {
		return c, err
	}
	m.categories[id] = c
	return c, nil
}
func (m *Memory) PatchAccount(ctx context.Context, id int64, p domain.AccountPatch) (domain.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.accounts[id]
	if !ok || m.accountLedger[id] != LedgerID(ctx) {
		return old, domain.ErrNotFound
	}
	a := p.Apply(old)
	if err := domain.ValidateAccount(a); err != nil {
		return a, err
	}
	m.accounts[id] = a
	a.Balance = m.balance(a)
	return a, nil
}
