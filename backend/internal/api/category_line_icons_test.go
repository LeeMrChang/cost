package api

import (
	"context"
	"cost/internal/store"
	"encoding/json"
	"fmt"
	"testing"
)

func categoryJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func testCategoryLineIcons(t *testing.T, s store.Store) {
	h := New(s, "test")
	var cats []map[string]any
	json.Unmarshal(p0Call(t, h, "GET", "/api/v1/categories", nil, 200).Body.Bytes(), &cats)
	if cats[0]["icon_key"] != "dining" || cats[0]["system_icon_key"] != "dining" {
		t.Fatalf("default keys missing: %+v", cats[0])
	}
	patch := func(id any, body any, status int) map[string]any {
		return categoryJSON(t, p0Call(t, h, "PATCH", fmt.Sprintf("/api/v1/categories/%.0f", id), body, status).Body.Bytes())
	}
	id := cats[0]["id"]
	got := patch(id, map[string]any{"name": "我的餐饮"}, 200)
	if got["icon_key"] != "dining" {
		t.Fatal("name patch cleared key")
	}
	got = patch(id, map[string]any{"icon": "🍕"}, 200)
	if got["icon_key"] != "" || got["system_icon_key"] != "dining" {
		t.Fatalf("legacy emoji patch: %+v", got)
	}
	got = patch(id, map[string]any{"icon_key": "coffee", "icon": "☕"}, 200)
	if got["icon_key"] != "coffee" {
		t.Fatal(got)
	}
	got = patch(id, map[string]any{"icon_key": nil}, 200)
	if got["icon_key"] != "" {
		t.Fatal("null did not clear")
	}
	patch(id, map[string]any{"icon_key": "missing-key"}, 400)
	patch(id, map[string]any{"icon_key": 12}, 400)
	custom := categoryJSON(t, p0Call(t, h, "POST", "/api/v1/categories", map[string]any{"name": "餐饮", "kind": "expense", "icon": "🐱", "icon_key": "pet-paw", "system_icon_key": "dining"}, 201).Body.Bytes())
	if custom["system_icon_key"] != "" || custom["icon_key"] != "pet-paw" {
		t.Fatalf("user forged builtin: %+v", custom)
	}
	p0Call(t, h, "POST", "/api/v1/categories/system-icons", nil, 200)
	json.Unmarshal(p0Call(t, h, "GET", "/api/v1/categories", nil, 200).Body.Bytes(), &cats)
	for _, c := range cats {
		if c["id"] == id && (c["icon_key"] != "dining" || c["icon"] != "☕") {
			t.Fatal("unify should restore system key only", c)
		}
		if c["id"] == custom["id"] && c["icon_key"] != "pet-paw" {
			t.Fatal("unify altered user category", c)
		}
	}
	p0Call(t, h, "POST", "/api/v1/categories/system-icons", nil, 200)
}
func TestCategoryLineIconsMemory(t *testing.T) { testCategoryLineIcons(t, store.NewMemory()) }
func TestCategoryLineIconsMySQL(t *testing.T)  { testCategoryLineIcons(t, testMySQL(t, false)) }

func TestMySQLLineIconsMigration(t *testing.T) {
	db := testMySQL(t, false)
	ctx := context.Background()
	for _, q := range []string{
		"DELETE FROM t_schema_migrations WHERE version='010_category_line_icons.sql'",
		"UPDATE t_categories SET icon_key=NULL,system_icon_key=NULL WHERE ledger_id=1",
		"UPDATE t_categories SET name='宵夜',icon='🍗' WHERE id=3",
		"UPDATE t_categories SET icon='⭐' WHERE id=16",
		"INSERT INTO t_categories(id,ledger_id,name,kind,icon) VALUES(301,1,'早餐','expense','🥞')",
		"INSERT INTO t_ledgers(id,name) VALUES(2,'隔离账本')",
		"INSERT INTO t_categories(id,ledger_id,name,kind,icon) VALUES(302,2,'早餐','expense','🍳')",
		"INSERT INTO t_entries(ledger_id,category_id,kind,amount_cent,occurred_at,note,client_request_id) VALUES(1,3,'expense',1234,'2026-11-01 08:00:00','保留内容','line-icon-migrate')",
	} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[int64][2]string{3: {"late-snack", "late-snack"}, 16: {"", "benefits"}, 301: {"", ""}, 302: {"", ""}} {
		var key, system string
		if err := db.DB().QueryRow("SELECT COALESCE(icon_key,''),COALESCE(system_icon_key,'') FROM t_categories WHERE id=?", id).Scan(&key, &system); err != nil || key != want[0] || system != want[1] {
			t.Fatalf("%d: keys=%q,%q want=%v err=%v", id, key, system, want, err)
		}
	}
	if _, err := db.DB().Exec("UPDATE t_categories SET icon_key=NULL,icon='💎' WHERE id=3"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var key string
	db.DB().QueryRow("SELECT COALESCE(icon_key,'') FROM t_categories WHERE id=3").Scan(&key)
	if key != "" {
		t.Fatal("rerun overwrote user emoji")
	}
	var n int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM t_entries WHERE category_id=3 AND amount_cent=1234 AND note='保留内容'").Scan(&n); err != nil || n != 1 {
		t.Fatal("historical entry changed", err)
	}
}

func TestMySQLLineIconLegacyIdentity(t *testing.T) {
	db := testMySQL(t, true)
	var key string
	if err := db.DB().QueryRow("SELECT COALESCE(system_icon_key,'') FROM t_categories WHERE id=2").Scan(&key); err != nil {
		t.Fatal(err)
	}
	// The legacy fixture uses id=2 for 午餐, not 交通. An ID collision must not assign the car role.
	if key == "transport" {
		t.Fatal("legacy lunch was misidentified as transport")
	}
}

func TestMySQLLineIconMigrationRollback(t *testing.T) {
	db := testMySQL(t, false)
	ctx := context.Background()
	for _, q := range []string{
		"DELETE FROM t_schema_migrations WHERE version='010_category_line_icons.sql'",
		"UPDATE t_categories SET icon_key=NULL,system_icon_key=NULL",
		"CREATE TRIGGER reject_line_icon_version BEFORE INSERT ON t_schema_migrations FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test rollback'",
	} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx); err == nil {
		t.Fatal("expected version-write failure")
	}
	var n int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM t_categories WHERE icon_key IS NOT NULL OR system_icon_key IS NOT NULL").Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial migration persisted: %d %v", n, err)
	}
	if _, err := db.DB().Exec("DROP TRIGGER reject_line_icon_version"); err != nil {
		t.Fatal(err)
	}
	// Simulate interruption between the two ALTER steps; migration must safely finish DDL and retry.
	if _, err := db.DB().Exec("ALTER TABLE t_categories DROP COLUMN system_icon_key"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM t_categories WHERE icon_key IS NOT NULL AND system_icon_key IS NOT NULL").Scan(&n); err != nil || n != 24 {
		t.Fatalf("retry failed %d %v", n, err)
	}
}

func TestMySQLUnifySystemIconsScopeAndRollback(t *testing.T) {
	db := testMySQL(t, false)
	ctx := context.Background()
	for _, q := range []string{
		"UPDATE t_categories SET icon_key=NULL WHERE ledger_id=1",
		"INSERT INTO t_ledgers(id,name) VALUES(2,'隔离账本')",
		"INSERT INTO t_categories(id,ledger_id,name,kind,icon,system_icon_key) VALUES(601,2,'餐饮','expense','🍜','dining'),(602,1,'餐饮','expense','🐱',NULL)",
		"CREATE TRIGGER reject_system_icon_update BEFORE UPDATE ON t_categories FOR EACH ROW BEGIN IF OLD.id=24 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test atomic update'; END IF; END",
	} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.UnifySystemIcons(ctx); err == nil {
		t.Fatal("expected atomic failure")
	}
	var n int
	db.DB().QueryRow("SELECT COUNT(*) FROM t_categories WHERE icon_key IS NOT NULL").Scan(&n)
	if n != 0 {
		t.Fatalf("partial unify persisted: %d", n)
	}
	if _, err := db.DB().Exec("DROP TRIGGER reject_system_icon_update"); err != nil {
		t.Fatal(err)
	}
	if n, err := db.UnifySystemIcons(ctx); err != nil || n != 24 {
		t.Fatalf("unify %d %v", n, err)
	}
	db.DB().QueryRow("SELECT COUNT(*) FROM t_categories WHERE id IN(601,602) AND icon_key IS NOT NULL").Scan(&n)
	if n != 0 {
		t.Fatal("unify changed other ledger or custom category")
	}
}
