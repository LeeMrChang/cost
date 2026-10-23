package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

var tableNames = []string{"ledgers", "categories", "entries", "budgets", "users", "sessions", "family_setup", "family_members", "family_invites", "schema_migrations"}

// prefixState rejects ambiguous layouts before the first DDL statement.
func prefixState(ctx context.Context, conn *sql.Conn) (bool, error) {
	old, newCount := 0, 0
	for _, name := range tableNames {
		var a, b int
		err := conn.QueryRowContext(ctx, "SELECT COALESCE(SUM(table_name=?),0),COALESCE(SUM(table_name=?),0) FROM information_schema.tables WHERE table_schema=DATABASE()", name, "t_"+name).Scan(&a, &b)
		if err != nil {
			return false, err
		}
		old += a
		newCount += b
	}
	if newCount > 0 {
		if old > 0 || newCount != len(tableNames) {
			return false, fmt.Errorf("inconsistent Cost schema: %d legacy and %d prefixed tables; migration stopped", old, newCount)
		}
		var versions int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_schema_migrations WHERE version IN ('001_init.sql','002_auth.sql')").Scan(&versions); err != nil {
			return false, err
		}
		if versions != 2 {
			return false, fmt.Errorf("prefixed schema missing legacy migration history")
		}
		return true, nil
	}
	return false, nil
}
func renameTables(ctx context.Context, conn *sql.Conn) error {
	parts := make([]string, 0, len(tableNames))
	for _, name := range tableNames {
		parts = append(parts, "`"+name+"` TO `t_"+name+"`")
	}
	_, err := conn.ExecContext(ctx, "RENAME TABLE "+strings.Join(parts, ","))
	return err
}
func applyTableComments(ctx context.Context, conn *sql.Conn) error {
	const version = "003_table_comments.sql"
	var applied int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_schema_migrations WHERE version=?", version).Scan(&applied); err != nil {
		return err
	}
	if applied > 0 {
		return nil
	}
	raw, err := migrations.ReadFile("migrations/" + version)
	if err != nil {
		return err
	}
	for _, q := range strings.Split(string(raw), ";") {
		if strings.TrimSpace(q) == "" {
			continue
		}
		if _, err = conn.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("table comments: %w", err)
		}
	}
	_, err = conn.ExecContext(ctx, "INSERT INTO t_schema_migrations(version) VALUES(?)", version)
	return err
}
