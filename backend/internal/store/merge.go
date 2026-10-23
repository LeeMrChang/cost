package store

import (
	"context"
	"fmt"
	"regexp"
)

var schemaIdentifier = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// MergeLedger imports business data from a quiescent, prefixed staging database
// on the same MySQL server. It never imports identities, sessions or privileges.
// The caller must stop application writes and hold a verified backup.
func (m *MySQL) MergeLedger(ctx context.Context, source string) (int64, error) {
	if !schemaIdentifier.MatchString(source) {
		return 0, fmt.Errorf("invalid source schema")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var target string
	if err = tx.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&target); err != nil {
		return 0, err
	}
	if source == target {
		return 0, fmt.Errorf("source and target must differ")
	}
	var id int
	if err = tx.QueryRowContext(ctx, "SELECT id FROM t_ledgers WHERE id=1 FOR UPDATE").Scan(&id); err != nil {
		return 0, err
	}
	src := "`" + source + "`."
	checks := []struct{ label, query string }{
		{"source must contain exactly the single family ledger", "SELECT IF(COUNT(*)=1 AND MIN(id)=1,0,1) FROM " + src + "t_ledgers"},
		{"ledger currency/timezone conflict", "SELECT COUNT(*) FROM " + src + "t_ledgers s JOIN t_ledgers t ON s.id=t.id WHERE NOT(BINARY s.currency <=> BINARY t.currency) OR NOT(BINARY s.timezone <=> BINARY t.timezone)"},
		{"source has records outside ledger 1", "SELECT (SELECT COUNT(*) FROM " + src + "t_entries WHERE ledger_id<>1)+(SELECT COUNT(*) FROM " + src + "t_categories WHERE ledger_id<>1)+(SELECT COUNT(*) FROM " + src + "t_budgets WHERE ledger_id<>1)"},
		{"category ID conflicts", "SELECT COUNT(*) FROM " + src + "t_categories s JOIN t_categories t ON s.id=t.id WHERE s.ledger_id<>t.ledger_id OR NOT(BINARY s.name <=> BINARY t.name) OR NOT(BINARY s.kind <=> BINARY t.kind) OR NOT(BINARY s.icon <=> BINARY t.icon)"},
		{"creator missing cloud identity or membership", "SELECT COUNT(*) FROM " + src + "t_entries e LEFT JOIN " + src + "t_users s ON e.created_by=s.id LEFT JOIN t_users u ON s.openid=u.openid LEFT JOIN t_family_members f ON f.user_id=u.id WHERE e.created_by IS NOT NULL AND (u.id IS NULL OR f.user_id IS NULL)"},
		{"budget month conflicts", "SELECT COUNT(*) FROM " + src + "t_budgets s JOIN t_budgets t ON s.ledger_id=t.ledger_id AND s.month=t.month WHERE s.amount_cent<>t.amount_cent"},
		{"entry request conflicts", "SELECT COUNT(*) FROM " + src + "t_entries s JOIN t_entries t ON s.ledger_id=t.ledger_id AND s.client_request_id=t.client_request_id LEFT JOIN " + src + "t_users su ON s.created_by=su.id LEFT JOIN t_users u ON su.openid=u.openid WHERE s.category_id<>t.category_id OR BINARY s.kind<>BINARY t.kind OR s.amount_cent<>t.amount_cent OR s.occurred_at<>t.occurred_at OR BINARY s.note<>BINARY t.note OR NOT(u.id <=> t.created_by)"},
	}
	for _, check := range checks {
		var count int
		if err = tx.QueryRowContext(ctx, check.query).Scan(&count); err != nil {
			return 0, err
		}
		if count != 0 {
			return 0, fmt.Errorf("ledger merge stopped: %s (%d)", check.label, count)
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_categories(id,ledger_id,name,kind,icon) SELECT s.id,s.ledger_id,s.name,s.kind,s.icon FROM "+src+"t_categories s LEFT JOIN t_categories t ON s.id=t.id WHERE t.id IS NULL"); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_budgets(ledger_id,month,amount_cent) SELECT s.ledger_id,s.month,s.amount_cent FROM "+src+"t_budgets s LEFT JOIN t_budgets t ON s.ledger_id=t.ledger_id AND s.month=t.month WHERE t.ledger_id IS NULL"); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO t_entries(ledger_id,category_id,kind,amount_cent,occurred_at,note,client_request_id,created_at,updated_at,created_by) SELECT s.ledger_id,s.category_id,s.kind,s.amount_cent,s.occurred_at,s.note,s.client_request_id,s.created_at,s.updated_at,u.id FROM "+src+"t_entries s LEFT JOIN "+src+"t_users su ON s.created_by=su.id LEFT JOIN t_users u ON su.openid=u.openid LEFT JOIN t_entries t ON s.ledger_id=t.ledger_id AND s.client_request_id=t.client_request_id WHERE t.id IS NULL ORDER BY s.id")
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, tx.Commit()
}
