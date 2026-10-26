package main

import (
	"context"
	"cost/internal/api"
	"cost/internal/auth"
	"cost/internal/store"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"
)

func env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}
func main() {
	if err := run(); err != nil {
		slog.Error("exit", "error", err)
		os.Exit(1)
	}
}
func run() error {
	migrate := flag.Bool("migrate", false, "initialize MySQL schema and exit")
	flag.Parse()
	timeout := 15 * time.Second
	if *migrate {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var s store.Store
	var identities auth.Repository
	mode := env("COST_STORE", "memory")
	switch mode {
	case "memory":
		if *migrate {
			return errors.New("-migrate requires COST_STORE=mysql")
		}
		s = store.NewMemory()
		identities = auth.NewMemory()
	case "mysql":
		dsn := os.Getenv("COST_MYSQL_DSN")
		if dsn == "" {
			return errors.New("COST_MYSQL_DSN is required")
		}
		db, err := store.NewMySQL(ctx, dsn)
		if err != nil {
			return err
		}
		s = db
		identities = &auth.MySQL{DB: db.DB()}
		if *migrate {
			defer db.Close()
			return db.Migrate(ctx)
		}
	default:
		return errors.New("COST_STORE must be memory or mysql")
	}
	defer s.Close()
	authMode := env("COST_AUTH_MODE", "wechat")
	var handler http.Handler
	switch authMode {
	case "wechat":
		handler = api.NewWithAuth(s, auth.New(identities, auth.WeChat{AppID: os.Getenv("WECHAT_APP_ID"), Secret: os.Getenv("WECHAT_APP_SECRET")}))
	case "development":
		if env("COST_ENV", "development") == "production" {
			return errors.New("development authentication is forbidden in production")
		}
		handler = api.New(s, env("COST_DEV_TOKEN", "cost-local-dev"))
	default:
		return fmt.Errorf("unknown COST_AUTH_MODE %q", authMode)
	}
	if env("COST_ENV", "development") == "production" && mode != "mysql" {
		return errors.New("production requires MySQL")
	}
	server := &http.Server{Addr: env("COST_ADDR", "0.0.0.0:8080"), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	slog.Info("starting cost", "store", mode, "auth_mode", authMode, "addr", server.Addr)
	if authMode == "development" {
		slog.Warn("development token authentication: local use only")
	}
	if mode == "memory" {
		slog.Warn("memory mode: data is lost when the process exits")
	}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-stop.Done():
	}
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return err
	}
	return nil
}
