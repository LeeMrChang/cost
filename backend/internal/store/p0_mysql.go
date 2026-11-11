package store

import (
	"context"
	"cost/internal/domain"
	"database/sql"
	"errors"
	"strings"
)

// Every reference-changing mutation locks the ledger first. This serializes writes
// against category/account disable operations without blocking snapshot reads.
func (m *MySQL) ledgerTx(ctx context.Context) (*sql.Tx, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM t_ledgers WHERE id=? FOR UPDATE", LedgerID(ctx)).Scan(&id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}
func applyLedgerFeatures(ctx context.Context, c *sql.Conn) error {
	const version = "004_ledger_features.sql"
	var n int
	if err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_schema_migrations WHERE version=?", version).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	raw, err := migrations.ReadFile("migrations/" + version)
	if err != nil {
		return err
	}
	for _, q := range strings.Split(string(raw), ";") {
		if strings.TrimSpace(q) != "" {
			if _, err = c.ExecContext(ctx, q); err != nil {
				return err
			}
		}
	}
	for _, v := range []struct{ table, column, ddl string }{
		{"t_categories", "sort_order", "ALTER TABLE t_categories ADD COLUMN sort_order INT NOT NULL DEFAULT 0 COMMENT '显示顺序'"},
		{"t_categories", "disabled", "ALTER TABLE t_categories ADD COLUMN disabled BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否停用'"},
		{"t_entries", "account_id", "ALTER TABLE t_entries ADD COLUMN account_id BIGINT NULL COMMENT '支付账户编号，空表示未指定'"},
	} {
		if err = c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", v.table, v.column).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err = c.ExecContext(ctx, v.ddl); err != nil {
				return err
			}
		}
	}
	// Constraint and index are independently restartable after an interrupted ALTER.
	if err = c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name='t_entries' AND constraint_name='fk_entry_account'").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err = c.ExecContext(ctx, "ALTER TABLE t_entries ADD CONSTRAINT fk_entry_account FOREIGN KEY(account_id) REFERENCES t_accounts(id)"); err != nil {
			return err
		}
	}
	if err = c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='t_entries' AND index_name='idx_entry_account_time'").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err = c.ExecContext(ctx, "ALTER TABLE t_entries ADD INDEX idx_entry_account_time (ledger_id,account_id,occurred_at)"); err != nil {
			return err
		}
	}
	_, err = c.ExecContext(ctx, "INSERT INTO t_schema_migrations(version) VALUES(?)", version)
	return err
}
func (m *MySQL) SaveCategory(ctx context.Context, c domain.Category) (domain.Category, error) {
	if err := domain.ValidateCategory(c); err != nil {
		return c, err
	}
	tx, err := m.ledgerTx(ctx)
	if err != nil {
		return c, err
	}
	defer tx.Rollback()
	ledgerID := LedgerID(ctx)
	if c.ID == 0 {
		if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0)+1 FROM t_categories").Scan(&c.ID); err != nil {
			return c, err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO t_categories(id,ledger_id,name,kind,icon,sort_order,disabled,icon_key) VALUES(?,?,?,?,?,?,?,NULLIF(?,''))", c.ID, ledgerID, c.Name, c.Kind, c.Icon, c.SortOrder, c.Disabled, c.IconKey)
	} else {
		var kind string
		err = tx.QueryRowContext(ctx, "SELECT kind FROM t_categories WHERE ledger_id=? AND id=?", ledgerID, c.ID).Scan(&kind)
		if errors.Is(err, sql.ErrNoRows) {
			return c, domain.ErrNotFound
		}
		if err != nil {
			return c, err
		}
		if kind != c.Kind {
			return c, domain.ErrInvalid
		}
		_, err = tx.ExecContext(ctx, "UPDATE t_categories SET name=?,icon=?,sort_order=?,disabled=?,icon_key=NULLIF(?,'') WHERE ledger_id=? AND id=?", c.Name, c.Icon, c.SortOrder, c.Disabled, c.IconKey, ledgerID, c.ID)
	}
	if err != nil {
		return c, err
	}
	return c, tx.Commit()
}
func (m *MySQL) SaveAccount(ctx context.Context, a domain.Account) (domain.Account, error) {
	if err := domain.ValidateAccount(a); err != nil {
		return a, err
	}
	tx, err := m.ledgerTx(ctx)
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	ledgerID := LedgerID(ctx)
	if a.ID == 0 {
		r, e := tx.ExecContext(ctx, "INSERT INTO t_accounts(ledger_id,name,type,opening_balance_cent,disabled) VALUES(?,?,?,?,?)", ledgerID, a.Name, a.Type, a.OpeningBalance, a.Disabled)
		err = e
		if err == nil {
			a.ID, err = r.LastInsertId()
		}
	} else {
		var id int64
		err = tx.QueryRowContext(ctx, "SELECT id FROM t_accounts WHERE ledger_id=? AND id=?", ledgerID, a.ID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return a, domain.ErrNotFound
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE t_accounts SET name=?,type=?,opening_balance_cent=?,disabled=? WHERE ledger_id=? AND id=?", a.Name, a.Type, a.OpeningBalance, a.Disabled, ledgerID, a.ID)
		}
	}
	if err != nil {
		return a, err
	}
	if err = tx.Commit(); err != nil {
		return a, err
	}
	accounts, err := m.Accounts(ctx)
	for _, v := range accounts {
		if v.ID == a.ID {
			return v, err
		}
	}
	return a, err
}
func (m *MySQL) Accounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT a.id,a.name,a.type,a.opening_balance_cent,a.disabled,
 a.opening_balance_cent+COALESCE((SELECT SUM(CASE e.kind WHEN 'income' THEN e.amount_cent ELSE -e.amount_cent END) FROM t_entries e WHERE e.ledger_id=a.ledger_id AND e.account_id=a.id AND e.deleted_at IS NULL),0)
 +COALESCE((SELECT SUM(t.amount_cent) FROM t_transfers t WHERE t.ledger_id=a.ledger_id AND t.to_account_id=a.id),0)
 -COALESCE((SELECT SUM(t.amount_cent) FROM t_transfers t WHERE t.ledger_id=a.ledger_id AND t.from_account_id=a.id),0)
 FROM t_accounts a WHERE a.ledger_id=? ORDER BY a.id`, LedgerID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Account{}
	for rows.Next() {
		var a domain.Account
		if err = rows.Scan(&a.ID, &a.Name, &a.Type, &a.OpeningBalance, &a.Disabled, &a.Balance); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func validateSQLReferences(ctx context.Context, tx *sql.Tx, e, old domain.Entry) error {
	ledgerID := LedgerID(ctx)
	var c domain.Category
	err := tx.QueryRowContext(ctx, "SELECT id,kind,disabled FROM t_categories WHERE ledger_id=? AND id=?", ledgerID, e.CategoryID).Scan(&c.ID, &c.Kind, &c.Disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrInvalid
	}
	if err != nil {
		return err
	}
	if c.Disabled && old.CategoryID != c.ID {
		return domain.ErrInvalid
	}
	if err = domain.Validate(e, []domain.Category{c}); err != nil {
		return err
	}
	if e.AccountID < 0 {
		return domain.ErrInvalid
	}
	if e.AccountID != 0 {
		var disabled bool
		err = tx.QueryRowContext(ctx, "SELECT disabled FROM t_accounts WHERE ledger_id=? AND id=?", ledgerID, e.AccountID).Scan(&disabled)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrInvalid
		}
		if err != nil {
			return err
		}
		if disabled && old.AccountID != e.AccountID {
			return domain.ErrInvalid
		}
	}
	return nil
}

const transferFields = "id,from_account_id,to_account_id,amount_cent,occurred_at,note,client_request_id,COALESCE(created_by,0)"

func scanTransfer(s interface{ Scan(...any) error }) (domain.Transfer, error) {
	var t domain.Transfer
	err := s.Scan(&t.ID, &t.FromAccountID, &t.ToAccountID, &t.Amount, &t.OccurredAt, &t.Note, &t.RequestID, &t.CreatedBy)
	return t, err
}
func (m *MySQL) SaveTransfer(ctx context.Context, t domain.Transfer) (domain.Transfer, bool, error) {
	if err := domain.ValidateTransfer(t); err != nil {
		return t, false, err
	}
	tx, err := m.ledgerTx(ctx)
	if err != nil {
		return t, false, err
	}
	defer tx.Rollback()
	ledgerID := LedgerID(ctx)
	old, err := scanTransfer(tx.QueryRowContext(ctx, "SELECT "+transferFields+" FROM t_transfers WHERE ledger_id=? AND client_request_id=?", ledgerID, t.RequestID))
	if err == nil {
		if !sameTransfer(old, t) {
			return t, false, domain.ErrConflict
		}
		return old, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return t, false, err
	}
	for _, id := range []int64{t.FromAccountID, t.ToAccountID} {
		var disabled bool
		err = tx.QueryRowContext(ctx, "SELECT disabled FROM t_accounts WHERE ledger_id=? AND id=?", ledgerID, id).Scan(&disabled)
		if errors.Is(err, sql.ErrNoRows) || disabled {
			return t, false, domain.ErrInvalid
		}
		if err != nil {
			return t, false, err
		}
	}
	r, err := tx.ExecContext(ctx, "INSERT INTO t_transfers(ledger_id,from_account_id,to_account_id,amount_cent,occurred_at,note,client_request_id,created_by) VALUES(?,?,?,?,?,?,?,NULLIF(?,0))", ledgerID, t.FromAccountID, t.ToAccountID, t.Amount, t.OccurredAt.UTC(), t.Note, t.RequestID, t.CreatedBy)
	if err != nil {
		return t, false, err
	}
	t.ID, err = r.LastInsertId()
	if err != nil {
		return t, false, err
	}
	return t, true, tx.Commit()
}
func (m *MySQL) GetTransfer(ctx context.Context, id int64) (domain.Transfer, error) {
	t, err := scanTransfer(m.db.QueryRowContext(ctx, "SELECT "+transferFields+" FROM t_transfers WHERE ledger_id=? AND id=?", LedgerID(ctx), id))
	if errors.Is(err, sql.ErrNoRows) {
		err = domain.ErrNotFound
	}
	return t, err
}
func (m *MySQL) DeleteTransfer(ctx context.Context, id int64) error {
	r, err := m.db.ExecContext(ctx, "DELETE FROM t_transfers WHERE ledger_id=? AND id=?", LedgerID(ctx), id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}
func (m *MySQL) Transfers(ctx context.Context, page, size int) (domain.TransferPage, error) {
	out := domain.TransferPage{Items: []domain.Transfer{}, Page: page, PageSize: size}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	ledgerID := LedgerID(ctx)
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_transfers WHERE ledger_id=?", ledgerID).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+transferFields+" FROM t_transfers WHERE ledger_id=? ORDER BY occurred_at DESC,id DESC LIMIT ? OFFSET ?", ledgerID, size, (page-1)*size)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		t, e := scanTransfer(rows)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, t)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	return out, tx.Commit()
}
func (m *MySQL) Query(ctx context.Context, f domain.EntryFilter) (domain.EntryPage, error) {
	out := domain.EntryPage{Items: []domain.Entry{}, Page: f.Page, PageSize: f.PageSize}
	where := "ledger_id=? AND deleted_at IS NULL"
	args := []any{LedgerID(ctx)}
	add := func(q string, v ...any) { where += " AND " + q; args = append(args, v...) }
	if !f.Start.IsZero() {
		add("occurred_at>=?", f.Start.UTC())
	}
	if !f.End.IsZero() {
		add("occurred_at<?", f.End.UTC())
	}
	if f.Query != "" {
		add("(LOCATE(BINARY ?,BINARY note)>0 OR category_id IN (SELECT id FROM t_categories WHERE ledger_id=? AND LOCATE(BINARY ?,BINARY name)>0))", f.Query, LedgerID(ctx), f.Query)
	}
	if f.Kind != "" {
		add("kind=?", f.Kind)
	}
	for _, v := range []struct {
		q string
		n *int64
	}{{"category_id=?", f.CategoryID}, {"COALESCE(created_by,0)=?", f.CreatedBy}, {"COALESCE(account_id,0)=?", f.AccountID}, {"amount_cent>=?", f.MinAmount}, {"amount_cent<=?", f.MaxAmount}} {
		if v.n != nil {
			add(v.q, *v.n)
		}
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM(CASE kind WHEN 'income' THEN amount_cent ELSE 0 END),0),COALESCE(SUM(CASE kind WHEN 'expense' THEN amount_cent ELSE 0 END),0) FROM t_entries WHERE "+where, args...).Scan(&out.Total, &out.Summary.Income, &out.Summary.Expense)
	if err != nil {
		return out, err
	}
	out.Summary.Balance = out.Summary.Income - out.Summary.Expense
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := tx.QueryContext(ctx, "SELECT "+fields+" FROM t_entries WHERE "+where+" ORDER BY occurred_at DESC,id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		e, eErr := scan(rows)
		if eErr != nil {
			return out, eErr
		}
		out.Items = append(out.Items, e)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	return out, tx.Commit()
}
func (m *MySQL) Authors(ctx context.Context) ([]domain.Author, error) {
	rows, err := m.db.QueryContext(ctx, "SELECT DISTINCT COALESCE(created_by,0) AS id FROM t_entries WHERE ledger_id=? AND deleted_at IS NULL ORDER BY id", LedgerID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Author{}
	for rows.Next() {
		var a domain.Author
		if err = rows.Scan(&a.ID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (m *MySQL) PatchCategory(ctx context.Context, id int64, p domain.CategoryPatch) (domain.Category, error) {
	var c domain.Category
	tx, err := m.ledgerTx(ctx)
	if err != nil {
		return c, err
	}
	defer tx.Rollback()
	ledgerID := LedgerID(ctx)
	err = tx.QueryRowContext(ctx, "SELECT id,name,kind,icon,sort_order,disabled,COALESCE(icon_key,''),COALESCE(system_icon_key,'') FROM t_categories WHERE ledger_id=? AND id=?", ledgerID, id).Scan(&c.ID, &c.Name, &c.Kind, &c.Icon, &c.SortOrder, &c.Disabled, &c.IconKey, &c.SystemIconKey)
	if errors.Is(err, sql.ErrNoRows) {
		return c, domain.ErrNotFound
	}
	if err != nil {
		return c, err
	}
	oldKind := c.Kind
	c = p.Apply(c)
	if c.Kind != oldKind {
		return c, domain.ErrInvalid
	}
	if err = domain.ValidateCategory(c); err != nil {
		return c, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE t_categories SET name=?,icon=?,sort_order=?,disabled=?,icon_key=NULLIF(?,'') WHERE ledger_id=? AND id=?", c.Name, c.Icon, c.SortOrder, c.Disabled, c.IconKey, ledgerID, c.ID)
	if err != nil {
		return c, err
	}
	return c, tx.Commit()
}
func (m *MySQL) PatchAccount(ctx context.Context, id int64, p domain.AccountPatch) (domain.Account, error) {
	var a domain.Account
	tx, err := m.ledgerTx(ctx)
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	ledgerID := LedgerID(ctx)
	err = tx.QueryRowContext(ctx, "SELECT id,name,type,opening_balance_cent,disabled FROM t_accounts WHERE ledger_id=? AND id=?", ledgerID, id).Scan(&a.ID, &a.Name, &a.Type, &a.OpeningBalance, &a.Disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return a, domain.ErrNotFound
	}
	if err != nil {
		return a, err
	}
	a = p.Apply(a)
	if err = domain.ValidateAccount(a); err != nil {
		return a, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE t_accounts SET name=?,type=?,opening_balance_cent=?,disabled=? WHERE ledger_id=? AND id=?", a.Name, a.Type, a.OpeningBalance, a.Disabled, ledgerID, a.ID)
	if err != nil {
		return a, err
	}
	if err = tx.Commit(); err != nil {
		return a, err
	}
	accounts, err := m.Accounts(ctx)
	for _, v := range accounts {
		if v.ID == id {
			return v, err
		}
	}
	return a, err
}
