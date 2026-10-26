package api

import (
	"context"
	"cost/internal/auth"
	"cost/internal/store"
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"os"
	"strings"
	"testing"
	"time"
)

// The optional admin DSN is used only to create and drop randomly named test databases.
// The configured application database is never selected or modified.
func rawTestMySQL(t *testing.T) *store.MySQL {
	t.Helper()
	dsn := os.Getenv("COST_TEST_MYSQL_ADMIN_DSN")
	if dsn == "" {
		t.Skip("set COST_TEST_MYSQL_ADMIN_DSN to run isolated MySQL integration tests")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("cost_test_%d", time.Now().UnixNano())
	ctx := context.Background()
	if _, err = admin.ExecContext(ctx, "CREATE DATABASE "+name+" CHARACTER SET utf8mb4"); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := admin.ExecContext(context.Background(), "DROP DATABASE "+name)
		if err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	cfg.DBName = name
	db, err := store.NewMySQL(ctx, cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testMySQL(t *testing.T, legacy bool) *store.MySQL {
	t.Helper()
	db := rawTestMySQL(t)
	ctx := context.Background()
	if legacy {
		raw, err := os.ReadFile("../store/migrations/001_init.sql")
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range strings.Split(string(raw), ";") {
			if strings.TrimSpace(q) != "" {
				if _, err = db.DB().ExecContext(ctx, q); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, q := range []string{"INSERT INTO ledgers(id,name) VALUES(1,'原有账本')", "INSERT INTO categories(id,ledger_id,name,kind,icon) VALUES(2,1,'午餐','expense','🍜')", "INSERT INTO entries(ledger_id,category_id,kind,amount_cent,occurred_at,note,client_request_id) VALUES(1,2,'expense',1234,'2026-10-01 00:00:00','迁移前记录','legacy-record')"} {
			if _, err = db.DB().ExecContext(ctx, q); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migration not idempotent: %v", err)
	}
	return db
}
func TestMySQLLoginWorkflow(t *testing.T) {
	db := testMySQL(t, false)
	testWorkflow(t, db, &auth.MySQL{DB: db.DB()})
}
func TestMySQLLegacyUpgrade(t *testing.T) {
	db := testMySQL(t, true)
	e, err := db.Get(context.Background(), 1)
	if err != nil || e.Amount != 1234 || e.Note != "迁移前记录" || e.CreatedBy != 0 {
		t.Fatalf("legacy entry changed: %+v %v", e, err)
	}
	var versions int
	if err = db.DB().QueryRow("SELECT COUNT(*) FROM t_schema_migrations").Scan(&versions); err != nil || versions != 5 {
		t.Fatal("migration versions missing", err)
	}
}
