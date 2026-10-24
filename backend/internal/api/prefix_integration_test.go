package api

import (
	"context"
	"cost/internal/auth"
	"cost/internal/store"
	"os"
	"strings"
	"testing"
	"time"
)

var legacyNames = []string{"ledgers", "categories", "entries", "budgets", "users", "sessions", "family_setup", "family_members", "family_invites", "schema_migrations"}

func legacySchema(t *testing.T) *store.MySQL {
	t.Helper()
	db := rawTestMySQL(t)
	for _, name := range []string{"001_init.sql", "002_auth.sql"} {
		raw, err := os.ReadFile("../store/migrations/" + name)
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
	}
	for _, q := range []string{
		"CREATE TABLE schema_migrations(version VARCHAR(64) PRIMARY KEY,applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6))",
		"INSERT INTO schema_migrations(version) VALUES('001_init.sql'),('002_auth.sql')",
		"ALTER TABLE entries ADD COLUMN created_by BIGINT NULL, ADD CONSTRAINT fk_entry_creator FOREIGN KEY(created_by) REFERENCES users(id)",
		"INSERT INTO ledgers(id,name) VALUES(1,'原账本')",
		"INSERT INTO categories(id,ledger_id,name,kind,icon) VALUES(2,1,'午餐','expense','🍜')",
		"INSERT INTO users(id,openid) VALUES(7,'fixture-openid')",
		"UPDATE family_setup SET owner_id=7 WHERE id=1",
		"INSERT INTO family_members(user_id,role) VALUES(7,'admin')",
		"INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(REPEAT('a',64),7,'2099-01-01')",
		"INSERT INTO family_invites(token_hash,created_by,expires_at) VALUES(REPEAT('b',64),7,'2099-01-01')",
		"INSERT INTO budgets(ledger_id,month,amount_cent) VALUES(1,'2026-10',300000)",
		"INSERT INTO entries(id,ledger_id,category_id,kind,amount_cent,occurred_at,note,client_request_id,created_by) VALUES(42,1,2,'expense',1234,'2026-10-01','原账单','prefix-fixture',7)",
	} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return db
}
func assertPrefixed(t *testing.T, db *store.MySQL) {
	t.Helper()
	var tables, columns, uncommented, old int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND LEFT(table_name,2)='t_' AND table_comment<>''").Scan(&tables); err != nil || tables != 12 {
		t.Fatalf("want 12 commented prefixed tables, got %d: %v", tables, err)
	}
	if err := db.DB().QueryRow("SELECT COUNT(*),SUM(column_comment='') FROM information_schema.columns WHERE table_schema=DATABASE() AND LEFT(table_name,2)='t_'").Scan(&columns, &uncommented); err != nil || columns != 57 || uncommented != 0 {
		t.Fatalf("columns=%d uncommented=%d err=%v", columns, uncommented, err)
	}
	for _, name := range legacyNames {
		db.DB().QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?", name).Scan(&old)
		if old != 0 {
			t.Fatalf("old table remains: %s", name)
		}
	}
}
func TestMySQLPrefixedFresh(t *testing.T) { db := testMySQL(t, false); assertPrefixed(t, db) }
func TestMySQLPrefixPreservesLegacy(t *testing.T) {
	db := legacySchema(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	assertPrefixed(t, db)
	e, err := db.Get(ctx, 42)
	if err != nil || e.Amount != 1234 || e.CreatedBy != 7 || e.Note != "原账单" {
		t.Fatalf("lost entry: %+v %v", e, err)
	}
	a := &auth.MySQL{DB: db.DB()}
	p, err := a.Resolve(ctx, strings.Repeat("a", 64), time.Now())
	if err != nil || p.ID != 7 || p.Role != "admin" {
		t.Fatalf("lost session %+v %v", p, err)
	}
	if err = a.Join(ctx, 7, strings.Repeat("b", 64), time.Now()); err != auth.ErrConflict {
		t.Fatalf("invite/membership lost: %v", err)
	}
	n, err := db.Budget(ctx, "2026-10")
	if err != nil || n == nil || *n != 300000 {
		t.Fatalf("lost budget %v %v", n, err)
	}
	var auto string
	db.DB().QueryRow("SELECT extra FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='t_entries' AND column_name='id'").Scan(&auto)
	if auto != "auto_increment" {
		t.Fatal(auto)
	}
	if _, err = db.DB().Exec("INSERT INTO t_entries(ledger_id,category_id,kind,amount_cent,occurred_at,client_request_id) VALUES(1,999,'expense',1,NOW(),'bad-fk')"); err == nil {
		t.Fatal("foreign key lost")
	}
}
func TestMySQLPrefixResumeAfterRename(t *testing.T) {
	db := legacySchema(t)
	parts := []string{}
	for _, name := range legacyNames {
		parts = append(parts, name+" TO t_"+name)
	}
	if _, err := db.DB().Exec("RENAME TABLE " + strings.Join(parts, ",")); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertPrefixed(t, db)
}
func TestMySQLPrefixRejectsConflict(t *testing.T) {
	db := legacySchema(t)
	if _, err := db.DB().Exec("CREATE TABLE t_entries(id BIGINT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(context.Background()); err == nil {
		t.Fatal("must reject conflicting old/new tables")
	}
	var n int
	db.DB().QueryRow("SELECT COUNT(*) FROM entries WHERE id=42 AND amount_cent=1234").Scan(&n)
	if n != 1 {
		t.Fatal("data changed")
	}
}
