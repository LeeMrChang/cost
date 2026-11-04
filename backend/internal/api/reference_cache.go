package api

import (
	"bytes"
	"context"
	"cost/internal/domain"
	"cost/internal/store"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// ReferenceCache is an optional disposable cache, never an authority for auth or money.
// A missing key returns nil bytes and no error.
type ReferenceCache interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
}
type Option func(*API)

func WithReferenceCache(c ReferenceCache) Option { return func(a *API) { a.cache = c } }

const referenceTTL = 5 * time.Minute
const referenceTimeout = 80 * time.Millisecond

type referenceGeneration struct {
	prefix string
	value  atomic.Uint64
}

func (a *API) initReferenceCache() {
	if a.cache == nil {
		return
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		a.cache = nil
		slog.Warn("reference cache disabled: random namespace unavailable")
		return
	}
	a.generation.prefix = "cost:reference:v1:" + hex.EncodeToString(nonce[:])
}

// Unknown write outcomes may already be committed (for example a failed readback).
func (a *API) invalidateReferences(err error) {
	if a.cache != nil && !errors.Is(err, domain.ErrInvalid) && !errors.Is(err, domain.ErrConflict) {
		a.generation.value.Add(1)
	}
}

func referenceRead[T any](a *API, w http.ResponseWriter, r *http.Request, resource string, load func() (T, error)) {
	// Snapshot BEFORE the database read: old in-flight results only fill old keys.
	var key string
	state := "BYPASS"
	writable := false
	if a.cache != nil {
		key = fmt.Sprintf("%s:%d:ledger:%d:%s", a.generation.prefix, a.generation.value.Load(), store.LedgerID(r.Context()), resource)
		ctx, cancel := context.WithTimeout(r.Context(), referenceTimeout)
		raw, err := a.cache.Get(ctx, key)
		cancel()
		if err == nil {
			state = "MISS"
			writable = true
			if len(raw) > 0 {
				var cached T
				if json.Unmarshal(raw, &cached) == nil && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
					w.Header().Set("X-Cost-Cache", "HIT")
					respond(w, 200, cached)
					return
				}
			}
		}
	}
	out, err := load()
	if err != nil {
		fail(w, err)
		return
	}
	if writable {
		raw, marshalErr := json.Marshal(out)
		if marshalErr == nil {
			ctx, cancel := context.WithTimeout(r.Context(), referenceTimeout)
			if err := a.cache.Set(ctx, key, raw, referenceTTL); err != nil {
				state = "BYPASS"
			}
			cancel()
		}
	}
	w.Header().Set("X-Cost-Cache", state)
	respond(w, 200, out)
}
