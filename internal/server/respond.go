package server

import (
	"encoding/json"
	"net/http"

	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
)

// writeJSON marshals v as JSON and writes it with the given status. DRY:
// all 200 / 4xx JSON responses go through this so we never forget the
// content-type or trailing newline.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError emits an OpenAI-compatible error envelope at the right HTTP status.
func writeError(w http.ResponseWriter, msg, errType string) {
	writeJSON(w, cerr.HTTPStatus(errType), cerr.New(msg, errType))
}

// writeSentinel is the DRY shortcut when the source is a *cerr.Sentinel.
func writeSentinel(w http.ResponseWriter, s *cerr.Sentinel) {
	writeJSON(w, s.Status, s.Envelope())
}
