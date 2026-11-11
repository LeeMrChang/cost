package api

import (
	"context"
	"cost/internal/domain"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMySQLCategoryIconsMigration 验证 009 的升级语义：
//  1. 仍是旧默认图标的分类升级为新图标（含用户改过名的行，靠 id 锚定）；
//  2. 用户自定义过图标的行保持不变，绝不被覆盖；
//  3. 用户自建的、与内置分类同名的分类不受影响；
//  4. 其他账本不受影响；
//  5. 历史账单关联不变；
//  6. 版本记录后重跑不会覆盖用户后续的修改。
func TestMySQLCategoryIconsMigration(t *testing.T) {
	db := testMySQL(t, false)
	ctx := context.Background()
	// 回到 009 执行前的状态：把 10 个默认分类重置为旧图标，并制造用户已改名 / 已自定义的情况。
	if _, err := db.DB().Exec("DELETE FROM t_schema_migrations WHERE version='009_category_icons.sql'"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"UPDATE t_categories SET icon='🍢' WHERE ledger_id=1 AND id=3",
		"UPDATE t_categories SET icon='🍳' WHERE ledger_id=1 AND id=6",
		"UPDATE t_categories SET icon='🍱' WHERE ledger_id=1 AND id=7",
		"UPDATE t_categories SET icon='🍚' WHERE ledger_id=1 AND id=8",
		"UPDATE t_categories SET icon='🛍️' WHERE ledger_id=1 AND id=9",
		"UPDATE t_categories SET icon='🧾' WHERE ledger_id=1 AND id=12",
		"UPDATE t_categories SET icon='🎁' WHERE ledger_id=1 AND id=14",
		"UPDATE t_categories SET icon='🌙' WHERE ledger_id=1 AND id=15",
		"UPDATE t_categories SET icon='🧧' WHERE ledger_id=1 AND id=16",
		"UPDATE t_categories SET icon='⭐' WHERE ledger_id=1 AND id=20", // 用户改过图标 → 必须保留
		"UPDATE t_categories SET name='宵夜' WHERE ledger_id=1 AND id=3", // 用户改过名 → 仍应按 id 升级
		"INSERT INTO t_categories(id,ledger_id,name,kind,icon,sort_order,disabled) VALUES(301,1,'早餐','expense','🥞',37,TRUE)",
		"INSERT INTO t_categories(id,ledger_id,name,kind,icon) VALUES(305,1,'福利','income','⭐'),(307,1,'消费','expense','🛍'),(308,1,'早餐','expense','🍳')",
		"INSERT INTO t_ledgers(id,name) VALUES(2,'隔离账本')",
		"INSERT INTO t_categories(id,ledger_id,name,kind,icon) VALUES(306,2,'早餐','expense','🍳')",
		"INSERT INTO t_entries(ledger_id,category_id,kind,amount_cent,occurred_at,note,client_request_id) VALUES(1,301,'expense',1234,'2026-11-01 08:00:00','历史账单','icon-migration')",
	} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	before, err := db.Categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := db.Categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatal("category count changed")
	}
	// 期望：未改过图标的升级；改过图标的（20 副业、305 福利 ⭐）保留原值。
	want := map[int64]string{3: "🍗", 6: "🥐", 7: "🍛", 8: "🍲", 9: "🛒", 12: "📦", 14: "🏆", 15: "⏰", 16: "🎫", 20: "⭐", 305: "⭐", 307: "🛒", 308: "🥐"}
	for i, c := range after {
		expected := before[i]
		if icon, ok := want[c.ID]; ok {
			expected.Icon = icon
		}
		if c != expected {
			t.Errorf("category changed unexpectedly: got %+v want %+v", c, expected)
		}
	}
	// 用户自建的同名分类不被波及。
	var custom string
	if err = db.DB().QueryRow("SELECT icon FROM t_categories WHERE id=301").Scan(&custom); err != nil || custom != "🥞" {
		t.Fatalf("user category changed: %q %v", custom, err)
	}
	// 其他账本不被波及。
	var other string
	if err = db.DB().QueryRow("SELECT icon FROM t_categories WHERE id=306").Scan(&other); err != nil || other != "🍳" {
		t.Fatalf("other ledger changed: %q %v", other, err)
	}
	// 历史账单的关联与内容不变。
	var category, amount int64
	var note string
	if err = db.DB().QueryRow("SELECT category_id,amount_cent,note FROM t_entries WHERE client_request_id='icon-migration'").Scan(&category, &amount, &note); err != nil || category != 301 || amount != 1234 || note != "历史账单" {
		t.Fatalf("entry changed %d %d %q %v", category, amount, note, err)
	}
	// 版本记录后重跑：用户后续的修改不会被再次覆盖（执行器层面的保护）。
	if _, err = db.DB().Exec("UPDATE t_categories SET icon='💎' WHERE id=6"); err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var icon string
	if err = db.DB().QueryRow("SELECT icon FROM t_categories WHERE id=6").Scan(&icon); err != nil || icon != "💎" {
		t.Fatalf("rerun overwrote later edit %q %v", icon, err)
	}
	// 即使绕过版本执行器手工重跑原 SQL，旧值条件仍保留后续自定义。
	raw, err := os.ReadFile("../store/migrations/009_category_icons.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range strings.Split(string(raw), ";") {
		if strings.TrimSpace(q) != "" {
			if _, err = db.DB().Exec(q); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = db.DB().QueryRow("SELECT icon FROM t_categories WHERE id=6").Scan(&icon); err != nil || icon != "💎" {
		t.Fatal("manual SQL replay overwrote custom icon", err)
	}
	// 历史分类与账单在分类停用后仍可用。
	entries, err := db.Query(ctx, domain.EntryFilter{Start: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Page: 1, PageSize: 50})
	if err != nil || entries.Total != 1 {
		t.Fatalf("historical entry lost: %+v %v", entries, err)
	}
}
