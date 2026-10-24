package auth

import (
	"context"
	"strings"
)

// SetNickname 在同一事务中锁定操作者和目标的成员关系，防止移除与改名并发绕过权限。
func (m *MySQL) SetNickname(ctx context.Context, actor, id int64, nickname string) error {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT user_id,role FROM t_family_members WHERE user_id IN (?,?) ORDER BY user_id FOR UPDATE", actor, id)
	if err != nil {
		return err
	}
	roles := map[int64]string{}
	for rows.Next() {
		var uid int64
		var role string
		if err = rows.Scan(&uid, &role); err != nil {
			rows.Close()
			return err
		}
		roles[uid] = role
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if roles[actor] == "" || (roles[actor] != "admin" && actor != id) {
		return ErrForbidden
	}
	if roles[id] == "" {
		return ErrMemberNotFound
	}
	if _, err = tx.ExecContext(ctx, "UPDATE t_users SET nickname=? WHERE id=?", nickname, id); err != nil {
		return err
	}
	return tx.Commit()
}
func (m *MySQL) Profiles(ctx context.Context, ids []int64) ([]Principal, error) {
	query := "SELECT u.id,COALESCE(f.role,''),u.nickname FROM t_users u LEFT JOIN t_family_members f ON f.user_id=u.id WHERE f.user_id IS NOT NULL"
	args := []any{}
	for _, id := range ids {
		if id > 0 {
			args = append(args, id)
		}
	}
	if len(args) > 0 {
		query += " OR u.id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(args)), ",") + ")"
	}
	rows, err := m.DB.QueryContext(ctx, query+" ORDER BY u.id", args...)
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
