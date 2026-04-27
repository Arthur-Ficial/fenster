package server

import (
	"net/http"
	"strings"

	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
)

// withOriginCheck rejects requests with foreign Origin headers.
//
// - Footgun mode: bypassed entirely.
// - OPTIONS preflight: bypassed (CORS handler decides).
// - No Origin header: allowed (matches apfel; works for curl/SDKs).
// - Origin in allowlist: allowed.
// - Otherwise: 403 with the apfel-shaped error envelope.
func withOriginCheck(cfg Config, next http.Handler) http.Handler {
	allowlist := cfg.AllowedOrigins
	if allowlist == nil {
		allowlist = DefaultOriginAllowlist()
	} else {
		// Custom list ADDS to the default localhost allowlist (matches apfel).
		allowlist = append(append([]string{}, DefaultOriginAllowlist()...), allowlist...)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.Footgun {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if OriginAllowed(r.Header.Get("Origin"), allowlist) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, "Origin not allowed", cerr.Forbidden)
	})
}

// withBearerAuth gates /v1/* (and optionally /health) on a Bearer token
// when cfg.BearerToken is non-empty. Footgun bypasses.
func withBearerAuth(cfg Config, next http.Handler) http.Handler {
	if cfg.BearerToken == "" || cfg.Footgun {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /health is open by default unless --public-health=false; even when
		// guarded, loopback bind doesn't need it. Apfel's behaviour: when
		// token is set AND server bound to non-loopback, /health requires
		// auth unless --public-health is set.
		if r.URL.Path == "/health" && cfg.PublicHealth {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) || strings.TrimPrefix(auth, prefix) != cfg.BearerToken {
			w.Header().Set("WWW-Authenticate", `Bearer realm="fenster"`)
			writeError(w, "missing or invalid bearer token", cerr.Authentication)
			return
		}
		next.ServeHTTP(w, r)
	})
}
