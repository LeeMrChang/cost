package store

import (
	"context"
	"database/sql"
)

func applyUserNickname(ctx context.Context, c *sql.Conn) error {
	const version = "007_user_nickname.sql"
	var n int
	if err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_schema_migrations WHERE version=?", version).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	exists, err := columnExists(ctx, c, "t_users", "nickname")
	if err != nil {
		return err
	}
	if !exists {
		raw, err := migrations.ReadFile("migrations/" + version)
		if err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, string(raw)); err != nil {
			return err
		}
	}
	_, err = c.ExecContext(ctx, "INSERT INTO t_schema_migrations(version) VALUES(?)", version)
	return err
}
