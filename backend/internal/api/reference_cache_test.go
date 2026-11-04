package api

import (
	"bytes"
	"context"
	"cost/internal/auth"
	"cost/internal/domain"
	"cost/internal/store"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Only the external Redis boundary is replaced; routing, auth and mutations stay real.
type referenceCacheFixture struct {
	mu         sync.Mutex
	rows       map[string][]byte
	expires    map[string]time.Time
	now        time.Time
	down, slow bool
}

func newReferenceFixture() *referenceCacheFixture {
	return &referenceCacheFixture{rows: map[string][]byte{}, expires: map[string]time.Time{}, now: time.Now()}
}
func (c *referenceCacheFixture) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	down, slow := c.down, c.slow
	b, ok := c.rows[key]
	expired := !c.now.Before(c.expires[key])
	c.mu.Unlock()
	if slow {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if down {
		return nil, errors.New("redis unavailable")
	}
	if !ok || expired {
		return nil, nil
	}
	return bytes.Clone(b), nil
}
func (c *referenceCacheFixture) Set(_ context.Context, key string, b []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.down {
		return errors.New("redis unavailable")
	}
	c.rows[key] = bytes.Clone(b)
	c.expires[key] = c.now.Add(ttl)
	return nil
}

type referenceDataFixture struct {
	store.Store
	cats, ledgers    atomic.Int64
	fail             atomic.Bool
	started, release chan struct{}
	once             sync.Once
}

func (s *referenceDataFixture) Categories(ctx context.Context) ([]domain.Category, error) {
	s.cats.Add(1)
	if s.fail.Load() {
		return nil, errors.New("database unavailable")
	}
	rows, err := s.Store.Categories(ctx)
	if s.started != nil {
		s.once.Do(func() { close(s.started); <-s.release })
	}
	return rows, err
}
func (s *referenceDataFixture) Ledger(ctx context.Context, id int64) (domain.Ledger, error) {
	s.ledgers.Add(1)
	return s.Store.Ledger(ctx, id)
}
func referenceCall(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("X-Dev-Token", "dev")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func expectReference(t *testing.T, w *httptest.ResponseRecorder, status int, cache string) {
	t.Helper()
	if w.Code != status || w.Header().Get("X-Cost-Cache") != cache {
		t.Fatalf("status=%d cache=%q body=%s; want %d %q", w.Code, w.Header().Get("X-Cost-Cache"), w.Body.String(), status, cache)
	}
}
func TestReferenceCacheHitAndExpiration(t *testing.T) {
	data := &referenceDataFixture{Store: store.NewMemory()}
	c := newReferenceFixture()
	h := New(data, "dev", WithReferenceCache(c))
	for _, path := range []string{"/api/v1/categories", "/api/v1/ledgers/me"} {
		first := referenceCall(h, "GET", path, "", "")
		expectReference(t, first, 200, "MISS")
		second := referenceCall(h, "GET", path, "", "")
		expectReference(t, second, 200, "HIT")
		if first.Body.String() != second.Body.String() {
			t.Fatal("cache changed response")
		}
	}
	if data.cats.Load() != 1 || data.ledgers.Load() != 1 {
		t.Fatal("hit still read store")
	}
	c.mu.Lock()
	c.now = c.now.Add(301 * time.Second)
	c.mu.Unlock()
	expectReference(t, referenceCall(h, "GET", "/api/v1/categories", "", ""), 200, "MISS")
	if data.cats.Load() != 2 {
		t.Fatal("expired cache did not reload")
	}
}
func TestReferenceCacheSuccessfulMutationsInvalidate(t *testing.T) {
	h := New(store.NewMemory(), "dev", WithReferenceCache(newReferenceFixture()))
	for _, change := range []struct {
		method, path, body, read, want string
		status                         int
	}{
		{"POST", "/api/v1/categories", `{"name":"新分类","kind":"expense","icon":"🍚"}`, "/api/v1/categories", "新分类", 201},
		{"PATCH", "/api/v1/categories/1", `{"name":"修改分类","sort_order":7,"disabled":true}`, "/api/v1/categories", "修改分类", 200},
		{"PATCH", "/api/v1/ledgers/me", `{"name":"新账本","icon":"✈️"}`, "/api/v1/ledgers/me", "新账本", 200},
	} {
		referenceCall(h, "GET", change.read, "", "")
		expectReference(t, referenceCall(h, "GET", change.read, "", ""), 200, "HIT")
		expectReference(t, referenceCall(h, change.method, change.path, change.body, ""), change.status, "")
		out := referenceCall(h, "GET", change.read, "", "")
		expectReference(t, out, 200, "MISS")
		if !strings.Contains(out.Body.String(), change.want) {
			t.Fatal("stale response", out.Body.String())
		}
	}
}
func TestReferenceCacheFailedMutationKeepsValidCache(t *testing.T) {
	h := New(store.NewMemory(), "dev", WithReferenceCache(newReferenceFixture()))
	referenceCall(h, "GET", "/api/v1/ledgers/me", "", "")
	expectReference(t, referenceCall(h, "PATCH", "/api/v1/ledgers/me", `{"name":""}`, ""), 400, "")
	expectReference(t, referenceCall(h, "GET", "/api/v1/ledgers/me", "", ""), 200, "HIT")
}
func TestReferenceCacheFallbacks(t *testing.T) {
	for _, mode := range []string{"down", "slow", "corrupt", "null"} {
		t.Run(mode, func(t *testing.T) {
			data := &referenceDataFixture{Store: store.NewMemory()}
			c := newReferenceFixture()
			h := New(data, "dev", WithReferenceCache(c))
			referenceCall(h, "GET", "/api/v1/categories", "", "")
			c.mu.Lock()
			c.down = mode == "down"
			c.slow = mode == "slow"
			if mode == "corrupt" || mode == "null" {
				for k := range c.rows {
					c.rows[k] = []byte(`{`)
					if mode == "null" {
						c.rows[k] = []byte(" null\n")
					}
				}
			}
			c.mu.Unlock()
			want := "BYPASS"
			if mode == "corrupt" || mode == "null" {
				want = "MISS"
			}
			start := time.Now()
			out := referenceCall(h, "GET", "/api/v1/categories", "", "")
			expectReference(t, out, 200, want)
			if time.Since(start) > time.Second || data.cats.Load() != 2 {
				t.Fatal("fallback blocked or did not reload")
			}
			if mode == "corrupt" {
				expectReference(t, referenceCall(h, "GET", "/api/v1/categories", "", ""), 200, "HIT")
			}
		})
	}
}
func TestReferenceCacheDoesNotCacheStoreErrors(t *testing.T) {
	data := &referenceDataFixture{Store: store.NewMemory()}
	data.fail.Store(true)
	h := New(data, "dev", WithReferenceCache(newReferenceFixture()))
	if out := referenceCall(h, "GET", "/api/v1/categories", "", ""); out.Code != 500 {
		t.Fatal(out.Code)
	}
	data.fail.Store(false)
	expectReference(t, referenceCall(h, "GET", "/api/v1/categories", "", ""), 200, "MISS")
}
func TestReferenceCacheOldReadCannotRefillNewGeneration(t *testing.T) {
	data := &referenceDataFixture{Store: store.NewMemory(), started: make(chan struct{}), release: make(chan struct{})}
	h := New(data, "dev", WithReferenceCache(newReferenceFixture()))
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- referenceCall(h, "GET", "/api/v1/categories", "", "") }()
	<-data.started
	expectReference(t, referenceCall(h, "PATCH", "/api/v1/categories/1", `{"name":"新名字"}`, ""), 200, "")
	close(data.release)
	<-done
	out := referenceCall(h, "GET", "/api/v1/categories", "", "")
	expectReference(t, out, 200, "MISS")
	if !strings.Contains(out.Body.String(), "新名字") {
		t.Fatal("old read poisoned new cache")
	}
}
func TestReferenceCacheWriteDuringOutageDoesNotReviveOldValue(t *testing.T) {
	c := newReferenceFixture()
	h := New(store.NewMemory(), "dev", WithReferenceCache(c))
	referenceCall(h, "GET", "/api/v1/ledgers/me", "", "")
	c.mu.Lock()
	c.down = true
	c.mu.Unlock()
	expectReference(t, referenceCall(h, "PATCH", "/api/v1/ledgers/me", `{"name":"断线期间修改"}`, ""), 200, "")
	c.mu.Lock()
	c.down = false
	c.mu.Unlock()
	out := referenceCall(h, "GET", "/api/v1/ledgers/me", "", "")
	expectReference(t, out, 200, "MISS")
	if !strings.Contains(out.Body.String(), "断线期间修改") {
		t.Fatal("reconnected cache revived stale value")
	}
}
func TestReferenceCacheInstancesDoNotReusePreviousValues(t *testing.T) {
	c := newReferenceFixture()
	s := store.NewMemory()
	h := New(s, "dev", WithReferenceCache(c))
	referenceCall(h, "GET", "/api/v1/ledgers/me", "", "")
	name := "离线修改"
	_, err := s.PatchLedger(context.Background(), domain.LedgerPatch{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	out := referenceCall(New(s, "dev", WithReferenceCache(c)), "GET", "/api/v1/ledgers/me", "", "")
	expectReference(t, out, 200, "MISS")
	if !strings.Contains(out.Body.String(), "离线修改") {
		t.Fatal("new instance reused old cache")
	}
}
func TestReferenceCacheAuthAndLedgerIsolation(t *testing.T) {
	h := NewWithAuth(store.NewMemory(), auth.New(auth.NewMemory(), fakeWeChat{}), WithReferenceCache(newReferenceFixture()))
	login := func(code string) string {
		out := referenceCall(h, "POST", "/api/v1/auth/wechat", `{"code":"`+code+`","nickname":"测试"}`, "")
		var v struct{ Token string }
		if out.Code != 200 || json.Unmarshal(out.Body.Bytes(), &v) != nil {
			t.Fatal(out.Body.String())
		}
		return v.Token
	}
	a, b := login("owner-cache"), login("other-cache")
	referenceCall(h, "PATCH", "/api/v1/ledgers/me", `{"name":"仅用户A"}`, a)
	expectReference(t, referenceCall(h, "GET", "/api/v1/ledgers/me", "", a), 200, "MISS")
	expectReference(t, referenceCall(h, "GET", "/api/v1/ledgers/me", "", a), 200, "HIT")
	other := referenceCall(h, "GET", "/api/v1/ledgers/me", "", b)
	expectReference(t, other, 200, "MISS")
	if strings.Contains(other.Body.String(), "仅用户A") {
		t.Fatal("cross-ledger leak")
	}
	expectReference(t, referenceCall(h, "GET", "/api/v1/ledgers/me", "", ""), 401, "")
	expectReference(t, referenceCall(h, "POST", "/api/v1/auth/logout", "", a), 204, "")
	expectReference(t, referenceCall(h, "GET", "/api/v1/ledgers/me", "", a), 401, "")
}

// Model a committed DB write followed by a failed result read/commit acknowledgement.
type uncertainReferenceStore struct{ store.Store }

func (s uncertainReferenceStore) PatchLedger(ctx context.Context, p domain.LedgerPatch) (domain.Ledger, error) {
	out, err := s.Store.PatchLedger(ctx, p)
	if err == nil {
		err = errors.New("write result unavailable")
	}
	return out, err
}
func (s uncertainReferenceStore) PatchCategory(ctx context.Context, id int64, p domain.CategoryPatch) (domain.Category, error) {
	out, err := s.Store.PatchCategory(ctx, id, p)
	if err == nil {
		err = errors.New("commit acknowledgement unavailable")
	}
	return out, err
}
func (s uncertainReferenceStore) SaveCategory(ctx context.Context, c domain.Category) (domain.Category, error) {
	out, err := s.Store.SaveCategory(ctx, c)
	if err == nil {
		err = errors.New("commit acknowledgement unavailable")
	}
	return out, err
}
func TestReferenceCacheUncertainWriteOutcomeInvalidates(t *testing.T) {
	for _, tt := range []struct{ method, path, body, read, want string }{
		{"PATCH", "/api/v1/ledgers/me", `{"name":"已提交的名称"}`, "/api/v1/ledgers/me", "已提交的名称"},
		{"PATCH", "/api/v1/categories/1", `{"name":"已提交的分类"}`, "/api/v1/categories", "已提交的分类"},
		{"POST", "/api/v1/categories", `{"name":"已新增的分类","kind":"expense","icon":"🍚"}`, "/api/v1/categories", "已新增的分类"},
	} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			h := New(uncertainReferenceStore{store.NewMemory()}, "dev", WithReferenceCache(newReferenceFixture()))
			referenceCall(h, "GET", tt.read, "", "")
			expectReference(t, referenceCall(h, "GET", tt.read, "", ""), 200, "HIT")
			expectReference(t, referenceCall(h, tt.method, tt.path, tt.body, ""), 500, "")
			out := referenceCall(h, "GET", tt.read, "", "")
			expectReference(t, out, 200, "MISS")
			if !strings.Contains(out.Body.String(), tt.want) {
				t.Fatal("committed write hidden by old cache")
			}
		})
	}
}
