package store

import (
	"context"
	"database/sql"
	"strings"
)

// applyCategoryIcons updates existing default category icons once, atomically with its version marker.
func applyCategoryIcons(ctx context.Context, c *sql.Conn) error {
	const version = "009_category_icons.sql"
	tx, err := c.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var applied int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_schema_migrations WHERE version=?", version).Scan(&applied); err != nil {
		return err
	}
	if applied > 0 {
		return tx.Commit()
	}
	raw, err := migrations.ReadFile("migrations/" + version)
	if err != nil {
		return err
	}
	for _, q := range strings.Split(string(raw), ";") {
		if strings.TrimSpace(q) == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_schema_migrations(version) VALUES(?)", version); err != nil {
		return err
	}
	return tx.Commit()
}
