package store

import (
	"context"
	"database/sql"
	"strings"
)

func applyUserAvatar(ctx context.Context, c *sql.Conn) error {
	const version = "008_user_avatar.sql"
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
	steps := strings.Split(string(raw), ";")
	for i, column := range []string{"avatar_url", "avatar_data"} {
		exists, err := columnExists(ctx, c, "t_users", column)
		if err != nil {
			return err
		}
		if !exists {
			if _, err = c.ExecContext(ctx, steps[i]); err != nil {
				return err
			}
		}
	}
	if err = c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='t_users' AND index_name='uk_user_avatar_url'").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err = c.ExecContext(ctx, steps[2]); err != nil {
			return err
		}
	}
	_, err = c.ExecContext(ctx, "INSERT INTO t_schema_migrations(version) VALUES(?)", version)
	return err
}
