package store

import (
	"context"
	"cost/internal/domain"
)

func (m *Memory) PatchLedger(ctx context.Context, patch domain.LedgerPatch) (domain.Ledger, error) {
	p, err := patch.Normalize()
	if err != nil {
		return domain.Ledger{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id := LedgerID(ctx)
	l, ok := m.ledgers[id]
	if !ok {
		return l, domain.ErrNotFound
	}
	if p.Name != nil {
		l.Name = *p.Name
	}
	if p.Icon != nil {
		l.Icon = *p.Icon
	}
	m.ledgers[id] = l
	return l, nil
}

func (m *MySQL) PatchLedger(ctx context.Context, patch domain.LedgerPatch) (domain.Ledger, error) {
	p, err := patch.Normalize()
	if err != nil {
		return domain.Ledger{}, err
	}
	id := LedgerID(ctx)
	_, err = m.db.ExecContext(ctx, "UPDATE t_ledgers SET name=COALESCE(?,name),icon=COALESCE(?,icon) WHERE id=?", p.Name, p.Icon, id)
	if err != nil {
		return domain.Ledger{}, err
	}
	return m.Ledger(ctx, id)
}
