package api

import (
	"context"
	"cost/internal/auth"
	"testing"
)

func TestMemberProfilesMySQL(t *testing.T) {
	db := testMySQL(t, false)
	testMemberProfiles(t, db, &auth.MySQL{DB: db.DB()})
}
func TestMemberNicknameMigration(t *testing.T) {
	db := testMySQL(t, true)
	ctx := context.Background()
	var count int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='t_users' AND column_name='nickname'").Scan(&count); err != nil || count != 1 {
		t.Fatal("nickname migration missing", count, err)
	}
	for _, q := range []string{"DELETE FROM t_schema_migrations WHERE version='005_member_nicknames.sql'", "ALTER TABLE t_users DROP COLUMN nickname", "INSERT INTO t_users(openid) VALUES('before-f05')", "INSERT INTO t_family_members(user_id,role) SELECT id,'member' FROM t_users WHERE openid='before-f05'"} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var nickname string
	if err := db.DB().QueryRow("SELECT nickname FROM t_users WHERE openid='before-f05'").Scan(&nickname); err != nil || nickname != "" {
		t.Fatal(nickname, err)
	}
	for _, q := range []string{"UPDATE t_users SET nickname='旧家人🍀' WHERE openid='before-f05'", "DELETE FROM t_schema_migrations WHERE version='005_member_nicknames.sql'"} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.DB().QueryRow("SELECT nickname FROM t_users WHERE openid='before-f05'").Scan(&nickname); err != nil || nickname != "旧家人🍀" {
		t.Fatal("rerun lost nickname", nickname, err)
	}
	e, err := db.Get(ctx, 1)
	if err != nil || e.Amount != 1234 || e.Note != "迁移前记录" {
		t.Fatal("old entry changed", e, err)
	}
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM t_family_members").Scan(&count); err != nil || count != 1 {
		t.Fatal("old membership changed", count, err)
	}
}
