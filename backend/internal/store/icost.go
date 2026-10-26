package store

import (
	"context"
	"cost/internal/domain"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// applyICostRealignment 应用 005 迁移：预算表重构、账单扩展列、账本归属与分类补种、DROP 家庭协作三表。
// 所有步骤按 information_schema 状态幂等，中断后可重跑；执行前须已备份真实库。
func applyICostRealignment(ctx context.Context, c *sql.Conn) error {
	const version = "005_icost_realignment.sql"
	var n int
	if err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_schema_migrations WHERE version=?", version).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	// 1. 扩展列（幂等）。
	for _, v := range []struct{ table, column, ddl string }{
		{"t_ledgers", "icon", "ALTER TABLE t_ledgers ADD COLUMN icon VARCHAR(32) NOT NULL DEFAULT '📒' COMMENT '账本图标'"},
		{"t_ledgers", "owner_id", "ALTER TABLE t_ledgers ADD COLUMN owner_id BIGINT NULL COMMENT '账本所有者用户编号'"},
		{"t_categories", "parent_id", "ALTER TABLE t_categories ADD COLUMN parent_id BIGINT NULL COMMENT '父分类编号，NULL或0为一级分类'"},
		{"t_entries", "refund_of_id", "ALTER TABLE t_entries ADD COLUMN refund_of_id BIGINT NULL COMMENT '退款关联的原账单编号'"},
		{"t_entries", "reimbursable", "ALTER TABLE t_entries ADD COLUMN reimbursable BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否标记可报销'"},
		{"t_entries", "exclude_stats", "ALTER TABLE t_entries ADD COLUMN exclude_stats BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否不计收支统计'"},
		{"t_entries", "exclude_budget", "ALTER TABLE t_entries ADD COLUMN exclude_budget BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否不计预算消耗'"},
		{"t_entries", "currency", "ALTER TABLE t_entries ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'CNY' COMMENT '币种代码，默认CNY'"},
	} {
		if err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", v.table, v.column).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err := c.ExecContext(ctx, v.ddl); err != nil {
				return err
			}
		}
	}
	if err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name='t_ledgers' AND constraint_name='fk_ledger_owner'").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := c.ExecContext(ctx, "ALTER TABLE t_ledgers ADD CONSTRAINT fk_ledger_owner FOREIGN KEY(owner_id) REFERENCES t_users(id)"); err != nil {
			return err
		}
	}
	if err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='t_ledgers' AND index_name='uk_ledger_owner'").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := c.ExecContext(ctx, "ALTER TABLE t_ledgers ADD UNIQUE INDEX uk_ledger_owner (owner_id)"); err != nil {
			return err
		}
	}

	// 2. 预算表重构：旧 (ledger_id,month,amount_cent) → 新 (ledger_id,scope,period_key,category_id,amount_cent)。
	if err := convertBudgets(ctx, c); err != nil {
		return err
	}

	// 3. 数据归属与改名（家庭管理员即账本所有者）。
	owner, err := icostOwner(ctx, c)
	if err != nil {
		return err
	}
	if owner != 0 {
		if _, err = c.ExecContext(ctx, "UPDATE t_ledgers SET name='我的账本', owner_id=? WHERE id=1 AND owner_id IS NULL", owner); err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, "UPDATE t_entries SET created_by=? WHERE ledger_id=1 AND COALESCE(created_by,0)=0", owner); err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, "UPDATE t_categories SET name='其他' WHERE ledger_id=1 AND kind='expense' AND name='其他支出'"); err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, "UPDATE t_categories SET name='其他' WHERE ledger_id=1 AND kind='income' AND name='其他收入'"); err != nil {
			return err
		}
		for _, cat := range domain.DefaultCategories() {
			// 从派生表取 MAX+1：直接 FROM t_categories 时，NOT EXISTS 为假会把所有行滤掉，
			// 聚合在空输入上仍输出一行 NULL，导致误插 id=1 与已有主键冲突。
			_, err = c.ExecContext(ctx, "INSERT INTO t_categories(id,ledger_id,name,kind,icon,sort_order,disabled) SELECT v.id,1,?,?,?,0,FALSE FROM (SELECT COALESCE(MAX(id),0)+1 AS id FROM t_categories) v WHERE NOT EXISTS(SELECT 1 FROM t_categories WHERE ledger_id=1 AND name=? AND kind=?)", cat.Name, cat.Kind, cat.Icon, cat.Name, cat.Kind)
			if err != nil {
				return err
			}
		}
	}

	// 4. DROP 家庭协作三表（新旧前缀都清，兼容未改名前的老库快照）。
	for _, name := range []string{"t_family_invites", "t_family_members", "t_family_setup", "family_invites", "family_members", "family_setup"} {
		if _, err = c.ExecContext(ctx, "DROP TABLE IF EXISTS "+name); err != nil {
			return err
		}
	}

	_, err = c.ExecContext(ctx, "INSERT INTO t_schema_migrations(version) VALUES(?)", version)
	return err
}

func icostOwner(ctx context.Context, c *sql.Conn) (int64, error) {
	var owner sql.NullInt64
	for _, name := range []string{"t_family_members", "family_members"} {
		exists, err := tableExists(ctx, c, name)
		if err != nil {
			return 0, err
		}
		if !exists {
			continue
		}
		err = c.QueryRowContext(ctx, "SELECT user_id FROM "+name+" WHERE role='admin' ORDER BY user_id LIMIT 1").Scan(&owner)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		if owner.Valid {
			return owner.Int64, nil
		}
		break
	}
	if err := c.QueryRowContext(ctx, "SELECT MIN(id) FROM t_users").Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	if !owner.Valid {
		return 0, nil
	}
	return owner.Int64, nil
}

func tableExists(ctx context.Context, c *sql.Conn, name string) (bool, error) {
	var n int
	err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?", name).Scan(&n)
	return n > 0, err
}

func columnExists(ctx context.Context, c *sql.Conn, table, column string) (bool, error) {
	var n int
	err := c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, column).Scan(&n)
	return n > 0, err
}

// convertBudgets 把旧月预算表迁移为新三粒度结构；RENAME 中断后可重跑恢复。
func convertBudgets(ctx context.Context, c *sql.Conn) error {
	oldExists, err := tableExists(ctx, c, "t_budgets")
	if err != nil {
		return err
	}
	newExists, err := tableExists(ctx, c, "t_budgets_new")
	if err != nil {
		return err
	}
	renamedExists, err := tableExists(ctx, c, "t_budgets_old_005")
	if err != nil {
		return err
	}
	if !oldExists {
		// 中断恢复：第一次 RENAME 已执行但第二次未执行。
		if newExists {
			if _, err = c.ExecContext(ctx, "RENAME TABLE t_budgets_new TO t_budgets"); err != nil {
				return err
			}
			newExists, err = tableExists(ctx, c, "t_budgets_new")
			if err != nil {
				return err
			}
			if _, err = c.ExecContext(ctx, "DROP TABLE IF EXISTS t_budgets_old_005"); err != nil {
				return err
			}
		}
		return nil
	}
	hasMonth, err := columnExists(ctx, c, "t_budgets", "month")
	if err != nil {
		return err
	}
	if !hasMonth {
		// 已转换完成；清理可能的旧表残留。
		if renamedExists {
			if _, err = c.ExecContext(ctx, "DROP TABLE t_budgets_old_005"); err != nil {
				return err
			}
		}
		return nil
	}
	// 旧结构 → 转换。
	if !newExists {
		raw, err := migrations.ReadFile("migrations/005_icost_realignment.sql")
		if err != nil {
			return err
		}
		for _, q := range strings.Split(string(raw), ";") {
			if strings.TrimSpace(q) != "" {
				if _, err = c.ExecContext(ctx, q); err != nil {
					return err
				}
			}
		}
	}
	if _, err = c.ExecContext(ctx, "INSERT IGNORE INTO t_budgets_new(ledger_id,scope,period_key,category_id,amount_cent) SELECT ledger_id,'month',month,0,amount_cent FROM t_budgets"); err != nil {
		return err
	}
	var oldCount, newCount int
	if err = c.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_budgets").Scan(&oldCount); err != nil {
		return err
	}
	if err = c.QueryRowContext(ctx, "SELECT COUNT(*) FROM t_budgets_new").Scan(&newCount); err != nil {
		return err
	}
	if oldCount != newCount {
		return fmt.Errorf("budget conversion count mismatch: old=%d new=%d; migration stopped for manual check", oldCount, newCount)
	}
	if _, err = c.ExecContext(ctx, "RENAME TABLE t_budgets TO t_budgets_old_005"); err != nil {
		return err
	}
	if _, err = c.ExecContext(ctx, "RENAME TABLE t_budgets_new TO t_budgets"); err != nil {
		return err
	}
	_, err = c.ExecContext(ctx, "DROP TABLE IF EXISTS t_budgets_old_005")
	return err
}
