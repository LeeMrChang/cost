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
	if err = tx.QueryRowContext(ctx, "SELECT id,nickname,COALESCE(avatar_url,'') FROM t_users WHERE openid=?", openid).Scan(&p.ID, &p.Nickname, &p.AvatarURL); err != nil {
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
	err := m.DB.QueryRowContext(ctx, "SELECT u.id,u.nickname,COALESCE(u.avatar_url,'') FROM t_sessions s JOIN t_users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?", hash, now).Scan(&p.ID, &p.Nickname, &p.AvatarURL)
	if err == sql.ErrNoRows {
		return p, ErrUnauthorized
	}
	return p, err
}
func (m *MySQL) UpdateNickname(ctx context.Context, id int64, nickname string) (Principal, error) {
	_, err := m.DB.ExecContext(ctx, "UPDATE t_users SET nickname=? WHERE id=?", nickname, id)
	if err != nil {
		return Principal{}, err
	}
	return m.profile(ctx, id)
}
func (m *MySQL) profile(ctx context.Context, id int64) (Principal, error) {
	var p Principal
	err := m.DB.QueryRowContext(ctx, "SELECT id,nickname,COALESCE(avatar_url,'') FROM t_users WHERE id=?", id).Scan(&p.ID, &p.Nickname, &p.AvatarURL)
	return p, err
}
func (m *MySQL) UpdateAvatar(ctx context.Context, id int64, url string, data []byte) (Principal, error) {
	_, err := m.DB.ExecContext(ctx, "UPDATE t_users SET avatar_url=?,avatar_data=? WHERE id=?", url, data, id)
	if err != nil {
		return Principal{}, err
	}
	return m.profile(ctx, id)
}
func (m *MySQL) Avatar(ctx context.Context, url string) ([]byte, error) {
	var data []byte
	err := m.DB.QueryRowContext(ctx, "SELECT avatar_data FROM t_users WHERE avatar_url=?", url).Scan(&data)
	if err == sql.ErrNoRows || (err == nil && len(data) == 0) {
		return nil, ErrAvatarNotFound
	}
	return data, err
}
func (m *MySQL) Logout(ctx context.Context, hash string) error {
	_, err := m.DB.ExecContext(ctx, "DELETE FROM t_sessions WHERE token_hash=?", hash)
	return err
}
