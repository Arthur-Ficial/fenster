// Package server is fenster's OpenAI-compatible HTTP layer. It is pure: it
// takes a Backend interface and exposes apfel's wire format byte-for-byte.
//
// DRY: all error responses go through writeError; all chat-success responses
// go through writeChatResponse; all SSE chunks go through writeSSE.
//
// The Mux uses Go 1.22's stdlib pattern routing (METHOD /path) so we don't
// pull in a third-party router.
package server

import (
	"net/http"
	"sync/atomic"

	"github.com/Arthur-Ficial/fenster/internal/backend"
)

// Config configures NewMux.
type Config struct {
	Backend    backend.Backend
	EnableCORS bool   // when false, OPTIONS still resolves but no ACAO is emitted
	BearerToken string // optional; when non-empty, /v1/* requires Authorization: Bearer <token>
	Debug      bool
}

// State is the shared, request-spanning state (active request gauge, etc.).
type State struct {
	activeRequests int64
}

// IncRequest atomically increments the active-request counter; returns a
// decrement function for defer.
func (s *State) IncRequest() func() {
	atomic.AddInt64(&s.activeRequests, 1)
	return func() { atomic.AddInt64(&s.activeRequests, -1) }
}

// ActiveRequests returns the current count.
func (s *State) ActiveRequests() int64 { return atomic.LoadInt64(&s.activeRequests) }

// NewMux returns the configured http.Handler.
func NewMux(cfg Config) http.Handler {
	if cfg.Backend == nil {
		cfg.Backend = backend.NullBackend{}
	}
	state := &State{}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handleHealth(cfg, state))
	mux.HandleFunc("GET /v1/models", handleModels())
	mux.HandleFunc("POST /v1/chat/completions", handleChat(cfg, state))
	mux.HandleFunc("POST /v1/completions", handleNotImplemented("legacy completions"))
	mux.HandleFunc("POST /v1/embeddings", handleNotImplemented("embeddings"))

	// CORS preflight on every relevant route.
	mux.HandleFunc("OPTIONS /v1/chat/completions", handleCORS(cfg))
	mux.HandleFunc("OPTIONS /v1/completions", handleCORS(cfg))
	mux.HandleFunc("OPTIONS /v1/embeddings", handleCORS(cfg))
	mux.HandleFunc("OPTIONS /v1/models", handleCORS(cfg))
	mux.HandleFunc("OPTIONS /health", handleCORS(cfg))

	// Wrap in CORS-response middleware (adds ACAO/Vary on real responses
	// when EnableCORS is set).
	return withCORSResponse(cfg, mux)
}
