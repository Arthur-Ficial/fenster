package server

import (
	"net/http"

	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
)

// handleNotImplemented returns the apfel-style 501 envelope for
// /v1/completions and /v1/embeddings.
func handleNotImplemented(feature string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeError(w, feature+" is not implemented by fenster", cerr.NotImplemented)
	}
}
