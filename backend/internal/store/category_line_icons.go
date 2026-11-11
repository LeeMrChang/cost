package store

import (
	"context"
	"cost/internal/domain"
	"database/sql"
	"strings"
)

func applyCategoryLineIcons(ctx context.Context, c *sql.Conn) error {
	const version = "010_category_line_icons.sql"
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
	for i, column := range []string{"icon_key", "system_icon_key"} {
		exists, err := columnExists(ctx, c, "t_categories", column)
		if err != nil {
			return err
		}
		if !exists {
			if _, err = c.ExecContext(ctx, steps[i]); err != nil {
				return err
			}
		}
	}
	tx, err := c.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT id,name,kind,icon,COALESCE(icon_key,''),COALESCE(system_icon_key,'') FROM t_categories WHERE ledger_id=1 ORDER BY id")
	if err != nil {
		return err
	}
	var cats []domain.Category
	for rows.Next() {
		var cat domain.Category
		if err = rows.Scan(&cat.ID, &cat.Name, &cat.Kind, &cat.Icon, &cat.IconKey, &cat.SystemIconKey); err != nil {
			rows.Close()
			return err
		}
		cats = append(cats, cat)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	used := map[int64]bool{}
	for _, icon := range domain.CategoryIcons() {
		if icon.SystemID == 0 {
			continue
		}
		index := -1
		// Fixed seed IDs identify renamed defaults; historical MAX+1 seeds fall back to one unambiguous name.
		for i, cat := range cats {
			if cat.ID == icon.SystemID && cat.Kind == icon.Kind {
				// Pre-005 databases may already use this ID for a different named default.
				// Do not misclassify a known lunch as transport merely because it has ID 2.
				conflict := false
				for _, other := range domain.CategoryIcons() {
					if other.SystemID != 0 && other.Key != icon.Key && other.Kind == cat.Kind && other.Label == cat.Name {
						conflict = true
						break
					}
				}
				if !conflict {
					index = i
					break
				}
			}
		}
		if index < 0 {
			count := 0
			for i, cat := range cats {
				if !used[cat.ID] && cat.Name == icon.Label && cat.Kind == icon.Kind {
					index = i
					count++
				}
			}
			if count != 1 {
				continue
			}
		}
		cat := cats[index]
		if used[cat.ID] {
			continue
		}
		used[cat.ID] = true
		key := cat.IconKey
		if key == "" && icon.IsDefaultEmoji(cat.Icon) {
			key = icon.Key
		}
		if _, err = tx.ExecContext(ctx, "UPDATE t_categories SET system_icon_key=?,icon_key=NULLIF(?,'') WHERE ledger_id=1 AND id=?", icon.Key, key, cat.ID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO t_schema_migrations(version) VALUES(?)", version); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *MySQL) UnifySystemIcons(ctx context.Context) (int, error) {
	tx, err := m.ledgerTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	n := 0
	for _, icon := range domain.CategoryIcons() {
		if icon.SystemID == 0 {
			continue
		}
		result, err := tx.ExecContext(ctx, "UPDATE t_categories SET icon_key=system_icon_key WHERE ledger_id=? AND system_icon_key=? AND NOT(icon_key <=> system_icon_key)", LedgerID(ctx), icon.Key)
		if err != nil {
			return 0, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		n += int(count)
	}
	return n, tx.Commit()
}
