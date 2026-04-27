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

// withBearerAuth gates /v1/* on a Bearer token when cfg.BearerToken is set.
// Footgun bypasses entirely. /health is open on loopback connections (and
// always when --public-health is set); on non-loopback binds with --token
// set, /health requires auth unless --public-health is set.
func withBearerAuth(cfg Config, next http.Handler) http.Handler {
	if cfg.BearerToken == "" || cfg.Footgun {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/health" {
			if cfg.PublicHealth || isLoopbackAddr(r.RemoteAddr) {
				next.ServeHTTP(w, r)
				return
			}
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

// isLoopbackAddr reports whether the RemoteAddr is a loopback connection.
// r.RemoteAddr is "ip:port" — strip the port and parse.
func isLoopbackAddr(s string) bool {
	if i := strings.LastIndex(s, ":"); i > 0 {
		s = s[:i]
	}
	s = strings.TrimPrefix(strings.TrimSuffix(s, "]"), "[")
	switch s {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return strings.HasPrefix(s, "127.")
}
