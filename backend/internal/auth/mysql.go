package auth

import (
	"context"
	"database/sql"
	"time"
)

type MySQL struct{ DB *sql.DB }

func (m *MySQL) Login(ctx context.Context, openid, nickname, hash string, expires time.Time) (Principal, error) {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return Principal{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_users(openid,nickname) VALUES(?,?) ON DUPLICATE KEY UPDATE nickname=IF(VALUES(nickname)='',nickname,VALUES(nickname))", openid, nickname); err != nil {
		return Principal{}, err
	}
	var p Principal
	if err = tx.QueryRowContext(ctx, "SELECT id,nickname FROM t_users WHERE openid=?", openid).Scan(&p.ID, &p.Nickname); err != nil {
		return Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM t_sessions WHERE expires_at<=?", time.Now().UTC()); err != nil {
		return Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_sessions(token_hash,user_id,expires_at) VALUES(?,?,?)", hash, p.ID, expires); err != nil {
		return Principal{}, err
	}
	return p, tx.Commit()
}
func (m *MySQL) Resolve(ctx context.Context, hash string, now time.Time) (Principal, error) {
	var p Principal
	err := m.DB.QueryRowContext(ctx, "SELECT u.id,u.nickname FROM t_sessions s JOIN t_users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?", hash, now).Scan(&p.ID, &p.Nickname)
	if err == sql.ErrNoRows {
		return p, ErrUnauthorized
	}
	return p, err
}
func (m *MySQL) UpdateNickname(ctx context.Context, id int64, nickname string) (Principal, error) {
	_, err := m.DB.ExecContext(ctx, "UPDATE t_users SET nickname=? WHERE id=?", nickname, id)
	return Principal{ID: id, Nickname: nickname}, err
}
func (m *MySQL) Logout(ctx context.Context, hash string) error {
	_, err := m.DB.ExecContext(ctx, "DELETE FROM t_sessions WHERE token_hash=?", hash)
	return err
}
