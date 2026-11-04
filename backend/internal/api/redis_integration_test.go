package api

import (
	"cost/internal/auth"
	"cost/internal/cache"
	"cost/internal/store"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRedisIntegrationReferenceRoutes(t *testing.T) {
	addr, password := os.Getenv("COST_TEST_REDIS_ADDR"), os.Getenv("COST_TEST_REDIS_PASSWORD")
	if addr == "" || password == "" {
		t.Skip("requires isolated test Redis")
	}
	client := cache.NewRedis(addr, password)
	defer client.Close()
	data := &referenceDataFixture{Store: store.NewMemory()}
	h := NewWithAuth(data, auth.New(auth.NewMemory(), fakeWeChat{}), WithReferenceCache(client))
	// WeChat test provider and auth repository are real HTTP/session plumbing.
	token := referenceCall(h, "POST", "/api/v1/auth/wechat", `{"code":"redis-integration","nickname":"测试"}`, "")
	raw := token.Body.String()
	var session struct{ Token string }
	if token.Code != 200 || json.Unmarshal([]byte(raw), &session) != nil {
		t.Fatal("test login failed")
	}
	expectReference(t, referenceCall(h, "GET", "/api/v1/ledgers/me", "", session.Token), 200, "MISS")
	expectReference(t, referenceCall(h, "GET", "/api/v1/ledgers/me", "", session.Token), 200, "HIT")
	if data.ledgers.Load() != 1 {
		t.Fatal("Redis hit still queried store")
	}
	expectReference(t, referenceCall(h, "PATCH", "/api/v1/ledgers/me", `{"name":"Redis真实缓存"}`, session.Token), 200, "")
	fresh := referenceCall(h, "GET", "/api/v1/ledgers/me", "", session.Token)
	expectReference(t, fresh, 200, "MISS")
	if !strings.Contains(fresh.Body.String(), "Redis真实缓存") {
		t.Fatal("write did not invalidate Redis result")
	}
	expectReference(t, referenceCall(h, "GET", "/api/v1/ledgers/me", "", session.Token), 200, "HIT")
	expectReference(t, referenceCall(h, "GET", "/api/v1/ledgers/me", "", ""), 401, "")
}
