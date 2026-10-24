package auth

import (
	"context"
	"database/sql"
	"errors"
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
	var nickname string
	if err = tx.QueryRowContext(ctx, "SELECT id,nickname FROM t_users WHERE openid=?", openid).Scan(&id, &nickname); err != nil {
		return Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM t_sessions WHERE expires_at<=?", time.Now().UTC()); err != nil {
		return Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_sessions(token_hash,user_id,expires_at) VALUES(?,?,?)", hash, id, expires); err != nil {
		return Principal{}, err
	}
	p := Principal{ID: id, Nickname: nickname}
	err = tx.QueryRowContext(ctx, "SELECT role FROM t_family_members WHERE user_id=?", id).Scan(&p.Role)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Principal{}, err
	}
	return p, tx.Commit()
}
func (m *MySQL) Resolve(ctx context.Context, hash string, now time.Time) (Principal, error) {
	var p Principal
	err := m.DB.QueryRowContext(ctx, "SELECT s.user_id,COALESCE(f.role,''),u.nickname FROM t_sessions s JOIN t_users u ON u.id=s.user_id LEFT JOIN t_family_members f ON f.user_id=s.user_id WHERE s.token_hash=? AND s.expires_at>?", hash, now).Scan(&p.ID, &p.Role, &p.Nickname)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrUnauthorized
	}
	return p, err
}
func (m *MySQL) Logout(ctx context.Context, hash string) error {
	_, err := m.DB.ExecContext(ctx, "DELETE FROM t_sessions WHERE token_hash=?", hash)
	return err
}
func (m *MySQL) Bootstrap(ctx context.Context, id int64) error {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, "UPDATE t_family_setup SET owner_id=? WHERE id=1 AND owner_id IS NULL", id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_family_members(user_id,role) VALUES(?,'admin')", id); err != nil {
		return err
	}
	return tx.Commit()
}
func (m *MySQL) Invite(ctx context.Context, id int64, hash string, expires time.Time) error {
	r, err := m.DB.ExecContext(ctx, "INSERT INTO t_family_invites(token_hash,created_by,expires_at) SELECT ?,user_id,? FROM t_family_members WHERE user_id=? AND role='admin'", hash, expires, id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err == nil && n == 0 {
		return ErrForbidden
	}
	return err
}
func (m *MySQL) Join(ctx context.Context, id int64, hash string, now time.Time) error {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var expires time.Time
	var used sql.NullInt64
	err = tx.QueryRowContext(ctx, "SELECT expires_at,used_by FROM t_family_invites WHERE token_hash=? FOR UPDATE", hash).Scan(&expires, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvite
	}
	if err != nil {
		return err
	}
	if used.Valid || !expires.After(now) {
		return ErrInvite
	}
	var exists int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_family_members WHERE user_id=?", id).Scan(&exists); err != nil {
		return err
	}
	if exists != 0 {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_family_members(user_id,role) VALUES(?,'member')", id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE t_family_invites SET used_by=? WHERE token_hash=?", id, hash); err != nil {
		return err
	}
	return tx.Commit()
}
func (m *MySQL) Members(ctx context.Context) ([]Principal, error) {
	rows, err := m.DB.QueryContext(ctx, "SELECT f.user_id,f.role,u.nickname FROM t_family_members f JOIN t_users u ON u.id=f.user_id ORDER BY f.user_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Principal{}
	for rows.Next() {
		var p Principal
		if err = rows.Scan(&p.ID, &p.Role, &p.Nickname); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (m *MySQL) Remove(ctx context.Context, id int64) error {
	r, err := m.DB.ExecContext(ctx, "DELETE FROM t_family_members WHERE user_id=? AND role='member'", id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err == nil && n == 0 {
		return ErrForbidden
	}
	return err
}
