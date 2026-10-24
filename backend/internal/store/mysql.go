package store

import (
	"context"
	"cost/internal/domain"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

//go:embed migrations/*.sql
var migrations embed.FS

type MySQL struct{ db *sql.DB }

func NewMySQL(ctx context.Context, dsn string) (*MySQL, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &MySQL{db}, nil
}

// Migrate applies restartable DDL while holding a database-scoped migration lock.
// MySQL DDL commits implicitly; the version is recorded only after all steps succeed.
func (m *MySQL) Migrate(ctx context.Context) error {
	conn, err := m.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT('cost_migrate_',DATABASE()),10)").Scan(&locked); err != nil {
		return err
	}
	if locked != 1 {
		return errors.New("migration lock unavailable")
	}
	defer func() {
		release, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(release, "DO RELEASE_LOCK(CONCAT('cost_migrate_',DATABASE()))")
	}()
	prefixed, err := prefixState(ctx, conn)
	if err != nil {
		return err
	}
	if !prefixed {
		if _, err = conn.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version VARCHAR(64) PRIMARY KEY,applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6))"); err != nil {
			return err
		}
		for _, name := range []string{"001_init.sql", "002_auth.sql"} {
			var applied int
			if err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version=?", name).Scan(&applied); err != nil {
				return err
			}
			if applied > 0 {
				continue
			}
			raw, err := migrations.ReadFile("migrations/" + name)
			if err != nil {
				return err
			}
			for _, q := range strings.Split(string(raw), ";") {
				if strings.TrimSpace(q) == "" {
					continue
				}
				if _, err = conn.ExecContext(ctx, q); err != nil {
					return err
				}
			}
			if name == "001_init.sql" {
				if _, err = conn.ExecContext(ctx, "INSERT IGNORE INTO ledgers(id,name) VALUES(1,'家庭账本')"); err != nil {
					return err
				}
				for _, c := range domain.DefaultCategories() {
					if _, err = conn.ExecContext(ctx, "INSERT IGNORE INTO categories(id,ledger_id,name,kind,icon) VALUES(?,1,?,?,?)", c.ID, c.Name, c.Kind, c.Icon); err != nil {
						return err
					}
				}
			}
			if name == "002_auth.sql" {
				var columns int
				if err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='entries' AND column_name='created_by'").Scan(&columns); err != nil {
					return err
				}
				if columns == 0 {
					if _, err = conn.ExecContext(ctx, "ALTER TABLE entries ADD COLUMN created_by BIGINT NULL, ADD CONSTRAINT fk_entry_creator FOREIGN KEY(created_by) REFERENCES users(id)"); err != nil {
						return err
					}
				}
			}
			if _, err = conn.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES(?)", name); err != nil {
				return err
			}
		}
	}
	if !prefixed {
		if err := renameTables(ctx, conn); err != nil {
			return err
		}
	}
	if err := applyTableComments(ctx, conn); err != nil {
		return err
	}
	return applyLedgerFeatures(ctx, conn)
}
func (m *MySQL) DB() *sql.DB { return m.db }

func (m *MySQL) Categories(ctx context.Context) ([]domain.Category, error) {
	rows, err := m.db.QueryContext(ctx, "SELECT id,name,kind,icon,sort_order,disabled FROM t_categories WHERE ledger_id=1 ORDER BY sort_order,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Category{}
	for rows.Next() {
		var c domain.Category
		if err = rows.Scan(&c.ID, &c.Name, &c.Kind, &c.Icon, &c.SortOrder, &c.Disabled); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const fields = "id,category_id,kind,amount_cent,occurred_at,note,client_request_id,COALESCE(created_by,0),COALESCE(account_id,0)"

func scan(s interface{ Scan(...any) error }) (domain.Entry, error) {
	var e domain.Entry
	err := s.Scan(&e.ID, &e.CategoryID, &e.Kind, &e.Amount, &e.OccurredAt, &e.Note, &e.RequestID, &e.CreatedBy, &e.AccountID)
	return e, err
}
func (m *MySQL) List(ctx context.Context, start, end time.Time) ([]domain.Entry, error) {
	rows, err := m.db.QueryContext(ctx, "SELECT "+fields+" FROM t_entries WHERE ledger_id=1 AND occurred_at>=? AND occurred_at<? ORDER BY occurred_at DESC,id DESC", start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Entry{}
	for rows.Next() {
		e, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (m *MySQL) Save(ctx context.Context, e domain.Entry) (domain.Entry, bool, error) {
	tx, err := m.ledgerTx(ctx)
	if err != nil {
		return e, false, err
	}
	defer tx.Rollback()
	old, err := scan(tx.QueryRowContext(ctx, "SELECT "+fields+" FROM t_entries WHERE ledger_id=1 AND client_request_id=?", e.RequestID))
	if err == nil {
		if !same(old, e) {
			return e, false, domain.ErrConflict
		}
		return old, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return e, false, err
	}
	if err = validateSQLReferences(ctx, tx, e, domain.Entry{}); err != nil {
		return e, false, err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO t_entries(ledger_id,category_id,kind,amount_cent,occurred_at,note,client_request_id,created_by,account_id) VALUES(1,?,?,?,?,?,?,NULLIF(?,0),NULLIF(?,0))", e.CategoryID, e.Kind, e.Amount, e.OccurredAt.UTC(), e.Note, e.RequestID, e.CreatedBy, e.AccountID)
	if err != nil {
		return e, false, err
	}
	e.ID, err = result.LastInsertId()
	if err != nil {
		return e, false, err
	}
	return e, true, tx.Commit()
}
func (m *MySQL) Update(ctx context.Context, e domain.Entry) (domain.Entry, error) {
	tx, err := m.ledgerTx(ctx)
	if err != nil {
		return e, err
	}
	defer tx.Rollback()
	old, err := scan(tx.QueryRowContext(ctx, "SELECT "+fields+" FROM t_entries WHERE ledger_id=1 AND id=? FOR UPDATE", e.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return e, domain.ErrNotFound
	}
	if err != nil {
		return e, err
	}
	if err = validateSQLReferences(ctx, tx, e, old); err != nil {
		return e, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE t_entries SET category_id=?,kind=?,amount_cent=?,occurred_at=?,note=?,account_id=NULLIF(?,0) WHERE ledger_id=1 AND id=?", e.CategoryID, e.Kind, e.Amount, e.OccurredAt.UTC(), e.Note, e.AccountID, e.ID)
	if err != nil {
		return e, err
	}
	e.RequestID = old.RequestID
	e.CreatedBy = old.CreatedBy
	return e, tx.Commit()
}
func (m *MySQL) Delete(ctx context.Context, id int64) error {
	r, err := m.db.ExecContext(ctx, "DELETE FROM t_entries WHERE ledger_id=1 AND id=?", id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
func (m *MySQL) Budget(ctx context.Context, month string) (*int64, error) {
	var n int64
	err := m.db.QueryRowContext(ctx, "SELECT amount_cent FROM t_budgets WHERE ledger_id=1 AND month=?", month).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &n, err
}
func (m *MySQL) SetBudget(ctx context.Context, month string, n int64) error {
	_, err := m.db.ExecContext(ctx, "INSERT INTO t_budgets(ledger_id,month,amount_cent) VALUES(1,?,?) ON DUPLICATE KEY UPDATE amount_cent=?", month, n, n)
	return err
}
func (m *MySQL) Close() error { return m.db.Close() }
func (m *MySQL) String() string {
	return fmt.Sprintf("MySQL(max_open=%d)", m.db.Stats().MaxOpenConnections)
}

func (m *MySQL) Get(ctx context.Context, id int64) (domain.Entry, error) {
	e, err := scan(m.db.QueryRowContext(ctx, "SELECT "+fields+" FROM t_entries WHERE ledger_id=1 AND id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		err = domain.ErrNotFound
	}
	return e, err
}
