package store

import (
	"context"
	"cost/internal/domain"
	"sort"
)

func (m *Memory) validateReferences(e, old domain.Entry) error {
	c, ok := m.categories[e.CategoryID]
	if !ok || c.Kind != e.Kind || (c.Disabled && old.CategoryID != c.ID) {
		return domain.ErrInvalid
	}
	if e.AccountID < 0 {
		return domain.ErrInvalid
	}
	if e.AccountID != 0 {
		a, ok := m.accounts[e.AccountID]
		if !ok || (a.Disabled && old.AccountID != a.ID) {
			return domain.ErrInvalid
		}
	}
	return domain.Validate(e, []domain.Category{c})
}
func (m *Memory) SaveCategory(_ context.Context, c domain.Category) (domain.Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := domain.ValidateCategory(c); err != nil {
		return c, err
	}
	if c.ID == 0 {
		c.ID = m.nextCategory
		m.nextCategory++
	} else {
		old, ok := m.categories[c.ID]
		if !ok {
			return c, domain.ErrNotFound
		}
		if old.Kind != c.Kind {
			return c, domain.ErrInvalid
		}
	}
	m.categories[c.ID] = c
	return c, nil
}
func (m *Memory) SaveAccount(_ context.Context, a domain.Account) (domain.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := domain.ValidateAccount(a); err != nil {
		return a, err
	}
	if a.ID == 0 {
		a.ID = m.nextAccount
		m.nextAccount++
	} else {
		if _, ok := m.accounts[a.ID]; !ok {
			return a, domain.ErrNotFound
		}
	}
	m.accounts[a.ID] = a
	a.Balance = m.balance(a)
	return a, nil
}
func (m *Memory) balance(a domain.Account) int64 {
	n := a.OpeningBalance
	for _, e := range m.entries {
		if e.AccountID == a.ID {
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
func (m *Memory) Accounts(context.Context) ([]domain.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Account{}
	for _, a := range m.accounts {
		a.Balance = m.balance(a)
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func sameTransfer(a, b domain.Transfer) bool {
	return a.CreatedBy == b.CreatedBy && a.FromAccountID == b.FromAccountID && a.ToAccountID == b.ToAccountID && a.Amount == b.Amount && a.OccurredAt.Equal(b.OccurredAt) && a.Note == b.Note
}
func (m *Memory) SaveTransfer(_ context.Context, t domain.Transfer) (domain.Transfer, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := domain.ValidateTransfer(t); err != nil {
		return t, false, err
	}
	if id, ok := m.transferRequests[t.RequestID]; ok {
		old := m.transfers[id]
		if !sameTransfer(old, t) {
			return t, false, domain.ErrConflict
		}
		return old, false, nil
	}
	for _, id := range []int64{t.FromAccountID, t.ToAccountID} {
		a, ok := m.accounts[id]
		if !ok || a.Disabled {
			return t, false, domain.ErrInvalid
		}
	}
	t.ID = m.nextTransfer
	m.nextTransfer++
	m.transfers[t.ID] = t
	m.transferRequests[t.RequestID] = t.ID
	return t, true, nil
}
func (m *Memory) GetTransfer(_ context.Context, id int64) (domain.Transfer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.transfers[id]
	if !ok {
		return t, domain.ErrNotFound
	}
	return t, nil
}
func (m *Memory) DeleteTransfer(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.transfers[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(m.transferRequests, t.RequestID)
	delete(m.transfers, id)
	return nil
}
func (m *Memory) Transfers(_ context.Context, page, size int) (domain.TransferPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Transfer{}
	for _, t := range m.transfers {
		out = append(out, t)
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
func (m *Memory) Query(_ context.Context, f domain.EntryFilter) (domain.EntryPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.Entry{}
	for _, e := range m.entries {
		if f.Match(e) {
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
func (m *Memory) Authors(context.Context) ([]domain.Author, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := map[int64]bool{}
	for _, e := range m.entries {
		ids[e.CreatedBy] = true
	}
	for _, t := range m.transfers {
		ids[t.CreatedBy] = true
	}
	out := []domain.Author{}
	for id := range ids {
		out = append(out, domain.Author{ID: id})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) PatchCategory(_ context.Context, id int64, p domain.CategoryPatch) (domain.Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.categories[id]
	if !ok {
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
func (m *Memory) PatchAccount(_ context.Context, id int64, p domain.AccountPatch) (domain.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.accounts[id]
	if !ok {
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
