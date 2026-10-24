package store

import (
	"context"
	"database/sql"
)

// applyMemberNicknames 在迁移锁内执行；DDL 后中断时，重试不覆盖已填写的昵称。
func applyMemberNicknames(ctx context.Context, c *sql.Conn) error {
	const version = "005_member_nicknames.sql"
	var n int
	if err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_schema_migrations WHERE version=?", version).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='t_users' AND column_name='nickname'").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		raw, err := migrations.ReadFile("migrations/" + version)
		if err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, string(raw)); err != nil {
			return err
		}
	}
	_, err := c.ExecContext(ctx, "INSERT INTO t_schema_migrations(version) VALUES(?)", version)
	return err
}
