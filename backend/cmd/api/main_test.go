package main

import (
	"strings"
	"testing"
)

func lookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadConfig(t *testing.T) {
	production := map[string]string{
		"COST_ENV":          "production",
		"COST_STORE":        "mysql",
		"COST_MYSQL_DSN":    "cost:secret@tcp(127.0.0.1:3306)/cost",
		"COST_AUTH_MODE":    "wechat",
		"WECHAT_APP_ID":     "wx-app",
		"WECHAT_APP_SECRET": "wx-secret",
	}
	tests := []struct {
		name      string
		env       map[string]string
		migrate   bool
		wantErr   string
		wantStore string
		wantAuth  string
		wantAddr  string
	}{
		{
			name:      "local defaults are usable",
			env:       map[string]string{"COST_AUTH_MODE": "development"},
			wantStore: "memory",
			wantAuth:  "development",
			wantAddr:  "0.0.0.0:8080",
		},
		{
			name:      "production wechat mysql",
			env:       production,
			wantStore: "mysql",
			wantAuth:  "wechat",
			wantAddr:  "0.0.0.0:8080",
		},
		{
			name:    "unknown environment",
			env:     map[string]string{"COST_ENV": "staging", "COST_AUTH_MODE": "development"},
			wantErr: "COST_ENV",
		},
		{
			name: "production requires mysql",
			env: map[string]string{
				"COST_ENV":          "production",
				"COST_STORE":        "memory",
				"COST_AUTH_MODE":    "wechat",
				"WECHAT_APP_ID":     "wx-app",
				"WECHAT_APP_SECRET": "wx-secret",
			},
			wantErr: "production requires MySQL",
		},
		{
			name:    "mysql requires dsn",
			env:     map[string]string{"COST_STORE": "mysql", "COST_AUTH_MODE": "development"},
			wantErr: "COST_MYSQL_DSN",
		},
		{
			name:    "unknown store",
			env:     map[string]string{"COST_STORE": "sqlite", "COST_AUTH_MODE": "development"},
			wantErr: "COST_STORE",
		},
		{
			name:    "unknown auth mode",
			env:     map[string]string{"COST_AUTH_MODE": "password"},
			wantErr: "COST_AUTH_MODE",
		},
		{
			name: "wechat requires credentials",
			env: map[string]string{
				"COST_STORE":     "mysql",
				"COST_MYSQL_DSN": "dsn",
				"COST_AUTH_MODE": "wechat",
			},
			wantErr: "WECHAT_APP_ID",
		},
		{
			name: "development auth is forbidden in production",
			env: map[string]string{
				"COST_ENV":       "production",
				"COST_STORE":     "mysql",
				"COST_MYSQL_DSN": "dsn",
				"COST_AUTH_MODE": "development",
			},
			wantErr: "forbidden in production",
		},
		{
			name:      "migration only needs store configuration",
			env:       map[string]string{"COST_STORE": "mysql", "COST_MYSQL_DSN": "dsn"},
			migrate:   true,
			wantStore: "mysql",
			wantAuth:  "wechat",
			wantAddr:  "0.0.0.0:8080",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfig(lookup(tt.env), tt.migrate)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("loadConfig() = nil error, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("loadConfig() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig() unexpected error: %v", err)
			}
			if cfg.store != tt.wantStore || cfg.authMode != tt.wantAuth || cfg.addr != tt.wantAddr {
				t.Fatalf("loadConfig() = %+v, want store=%q auth=%q addr=%q", cfg, tt.wantStore, tt.wantAuth, tt.wantAddr)
			}
		})
	}
}
