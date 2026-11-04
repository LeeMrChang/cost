package api

import (
	"bytes"
	"cost/internal/domain"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"
)

func (a *API) updateLedger(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if err != nil || !utf8.Valid(raw) {
		respond(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var p domain.LedgerPatch
	if !decode(w, r, &p) {
		return
	}
	out, err := a.store.PatchLedger(r.Context(), p)
	a.invalidateReferences(err)
	if errors.Is(err, domain.ErrInvalid) {
		respond(w, 400, map[string]string{"error": "invalid_ledger"})
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, out)
}
