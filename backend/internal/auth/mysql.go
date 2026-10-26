package auth

import (
	"context"
	"database/sql"
	"time"
)

type MySQL struct{ DB *sql.DB }

func (m *MySQL) Login(ctx context.Context, openid, hash string, expires time.Time) (Principal, error) {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return Principal{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_users(openid) VALUES(?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)", openid); err != nil {
		return Principal{}, err
	}
	var id int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM t_users WHERE openid=?", openid).Scan(&id); err != nil {
		return Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM t_sessions WHERE expires_at<=?", time.Now().UTC()); err != nil {
		return Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_sessions(token_hash,user_id,expires_at) VALUES(?,?,?)", hash, id, expires); err != nil {
		return Principal{}, err
	}
	return Principal{ID: id}, tx.Commit()
}
func (m *MySQL) Resolve(ctx context.Context, hash string, now time.Time) (Principal, error) {
	var p Principal
	err := m.DB.QueryRowContext(ctx, "SELECT s.user_id FROM t_sessions s WHERE s.token_hash=? AND s.expires_at>?", hash, now).Scan(&p.ID)
	if err == sql.ErrNoRows {
		return p, ErrUnauthorized
	}
	return p, err
}
func (m *MySQL) Logout(ctx context.Context, hash string) error {
	_, err := m.DB.ExecContext(ctx, "DELETE FROM t_sessions WHERE token_hash=?", hash)
	return err
}
