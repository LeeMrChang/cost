package api

import (
	"context"
	"testing"
)

func TestMySQLMergeLedger(t *testing.T) {
	source := legacySchema(t)
	if err := source.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	target := testMySQL(t, false)
	for _, q := range []string{
		"INSERT INTO t_users(id,openid) VALUES(99,'fixture-openid')",
		"INSERT INTO t_family_members(user_id,role) VALUES(99,'admin')",
		"UPDATE t_family_setup SET owner_id=99 WHERE id=1",
		"INSERT INTO t_entries(ledger_id,category_id,kind,amount_cent,occurred_at,note,client_request_id,created_by) VALUES(1,2,'expense',55,'2026-10-02','云端原记录','cloud-existing',99)",
	} {
		if _, err := target.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	var schema string
	source.DB().QueryRow("SELECT DATABASE()").Scan(&schema)
	n, err := target.MergeLedger(context.Background(), schema)
	if err != nil || n != 1 {
		t.Fatalf("first merge %d %v", n, err)
	}
	n, err = target.MergeLedger(context.Background(), schema)
	if err != nil || n != 0 {
		t.Fatalf("repeat merge %d %v", n, err)
	}
	var count, sum, creator int
	target.DB().QueryRow("SELECT COUNT(*),SUM(amount_cent),MAX(created_by) FROM t_entries").Scan(&count, &sum, &creator)
	if count != 2 || sum != 1289 || creator != 99 {
		t.Fatalf("counts %d sum %d creator %d", count, sum, creator)
	}
	var sessions int
	target.DB().QueryRow("SELECT COUNT(*) FROM t_sessions").Scan(&sessions)
	if sessions != 0 {
		t.Fatal("source sessions copied")
	}
	for _, tc := range []struct{ name, mutation string }{
		{"entry", "UPDATE t_entries SET amount_cent=999 WHERE client_request_id='prefix-fixture'"},
		{"budget", "UPDATE t_budgets SET amount_cent=999"},
		{"category", "UPDATE t_categories SET name='冲突' WHERE id=2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A separate target preserves fixtures independently between conflict cases.
			other := testMySQL(t, false)
			other.DB().Exec("INSERT INTO t_users(id,openid) VALUES(99,'fixture-openid')")
			other.DB().Exec("INSERT INTO t_family_members(user_id,role) VALUES(99,'admin')")
			if _, err := other.MergeLedger(context.Background(), schema); err != nil {
				t.Fatal(err)
			}
			if _, err := other.DB().Exec(tc.mutation); err != nil {
				t.Fatal(err)
			}
			if _, err := other.MergeLedger(context.Background(), schema); err == nil {
				t.Fatal("expected conflict")
			}
			var c int
			other.DB().QueryRow("SELECT COUNT(*) FROM t_entries").Scan(&c)
			if c != 1 {
				t.Fatal("partial merge")
			}
		})
	}
	unknown := testMySQL(t, false)
	if _, err := unknown.MergeLedger(context.Background(), schema); err == nil {
		t.Fatal("unknown creator must fail")
	}
	unknown.DB().QueryRow("SELECT COUNT(*) FROM t_entries").Scan(&count)
	if count != 0 {
		t.Fatal("unknown creator partially inserted")
	}
	if _, err := target.MergeLedger(context.Background(), "bad;name"); err == nil {
		t.Fatal("invalid source schema accepted")
	}
	var targetName string
	target.DB().QueryRow("SELECT DATABASE()").Scan(&targetName)
	if _, err := target.MergeLedger(context.Background(), targetName); err == nil {
		t.Fatal("same database accepted")
	}
}
