package api

import (
	"context"
	"cost/internal/domain"
	"testing"
	"time"
)

func TestMySQLMergeLedger(t *testing.T) {
	source := legacySchema(t)
	// Align source categories with the current default set: the merge requires
	// identical categories on shared IDs. legacySchema seeds id 2 as the legacy
	// 午餐; the post-005 defaults put 交通 there.
	for _, c := range domain.DefaultCategories() {
		if c.ID == 2 {
			if _, err := source.DB().Exec("UPDATE categories SET name=?, icon=? WHERE id=2", c.Name, c.Icon); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if _, err := source.DB().Exec("INSERT INTO categories(id,ledger_id,name,kind,icon) VALUES(?,1,?,?,?)", c.ID, c.Name, c.Kind, c.Icon); err != nil {
			t.Fatal(err)
		}
	}
	if err := source.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	target := testMySQL(t, false)
	for _, q := range []string{
		"INSERT INTO t_users(id,openid) VALUES(99,'fixture-openid')",
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
	t.Run("divergent legacy categories must stop", func(t *testing.T) {
		legacy := legacySchema(t)
		if err := legacy.Migrate(context.Background()); err != nil {
			t.Fatal(err)
		}
		var name string
		legacy.DB().QueryRow("SELECT DATABASE()").Scan(&name)
		other := testMySQL(t, false)
		other.DB().Exec("INSERT INTO t_users(id,openid) VALUES(99,'fixture-openid')")
		if _, err := other.MergeLedger(context.Background(), name); err == nil {
			t.Fatal("expected category conflict")
		}
		var c int
		other.DB().QueryRow("SELECT COUNT(*) FROM t_entries").Scan(&c)
		if c != 0 {
			t.Fatal("partial merge")
		}
	})
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

func TestMySQLMergeRejectsSoftDeletedSource(t *testing.T) {
	source := testMySQL(t, false)
	target := testMySQL(t, false)
	ctx := context.Background()
	e, _, err := source.Save(ctx, domain.Entry{CategoryID: 1, Kind: "expense", Amount: 100, OccurredAt: time.Now(), RequestID: "deleted-source"})
	if err != nil {
		t.Fatal(err)
	}
	if err = source.Delete(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	var name string
	if err = source.DB().QueryRow("SELECT DATABASE()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if _, err = target.MergeLedger(ctx, name); err == nil {
		t.Fatal("merge must not resurrect a soft-deleted entry")
	}
}
