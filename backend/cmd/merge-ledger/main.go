// merge-ledger is an operator-only command, never run by the HTTP service.
package main

import (
	"context"
	"cost/internal/store"
	"fmt"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	dsn, source := os.Getenv("COST_MYSQL_DSN"), os.Getenv("COST_IMPORT_SOURCE_SCHEMA")
	if dsn == "" || source == "" {
		return fmt.Errorf("COST_MYSQL_DSN and COST_IMPORT_SOURCE_SCHEMA are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := store.NewMySQL(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	n, err := db.MergeLedger(ctx, source)
	if err != nil {
		return err
	}
	fmt.Printf("Imported %d new entries; identities and permissions unchanged\n", n)
	return nil
}
