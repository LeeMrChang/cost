package cache

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

func TestRedisTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback unavailable: %v", err)
	}
	defer listener.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); <-done }()
		}
	}()
	client := NewRedis(listener.Addr().String(), "test-only")
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := client.Get(ctx, "cost:test:timeout"); err == nil {
		t.Fatal("expected unreachable response to fall back")
	}
	if time.Since(start) > time.Second {
		t.Fatal("optional Redis operation blocked beyond timeout")
	}
}
func TestRedisIntegration(t *testing.T) {
	addr, password := os.Getenv("COST_TEST_REDIS_ADDR"), os.Getenv("COST_TEST_REDIS_PASSWORD")
	if addr == "" || password == "" {
		t.Skip("requires isolated test Redis")
	}
	ctx := context.Background()
	client := NewRedis(addr, password)
	defer client.Close()
	key := "cost:integration:reference" // Disposable CI Redis only; never configure this test against production.
	raw, err := client.Get(ctx, key)
	if err != nil || raw != nil {
		t.Fatal("empty authenticated cache must miss")
	}
	if err := client.Set(ctx, key, []byte(`{"name":"测试"}`), 100*time.Millisecond); err != nil {
		t.Fatal("authenticated cache set failed")
	}
	raw, err = client.Get(ctx, key)
	if err != nil || string(raw) != `{"name":"测试"}` {
		t.Fatal("authenticated cache round trip failed")
	}
	limit := time.Now().Add(2 * time.Second)
	for {
		raw, err = client.Get(ctx, key)
		if err != nil {
			t.Fatal("cache expiration read failed")
		}
		if raw == nil {
			break
		}
		if time.Now().After(limit) {
			t.Fatal("cache TTL not applied")
		}
		time.Sleep(10 * time.Millisecond)
	}
	wrong := NewRedis(addr, "incorrect-test-only")
	defer wrong.Close()
	if _, err := wrong.Get(ctx, key); err == nil {
		t.Fatal("cache accepted incorrect credentials")
	}
}
