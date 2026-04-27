package server

import (
	"net/http"
	"strings"

	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
)

// withOriginCheck rejects requests with foreign Origin headers.
//
// - Footgun mode: bypassed entirely.
// - --no-origin-check: bypassed (caller takes responsibility).
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
		if cfg.Footgun || cfg.NoOriginCheck {
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
			// Two paths to bypass /health auth:
			//   1. --public-health flag explicitly opts in
			//   2. server is bound to loopback (the typical local dev setup)
			// When the supervisor binds to 0.0.0.0 or a routable IP, /health
			// requires auth — apfel parity (test_health_requires_auth_on_non_loopback).
			if cfg.PublicHealth || isLoopbackBind(cfg.BindHost) {
				next.ServeHTTP(w, r)
				return
			}
		}
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) || strings.TrimPrefix(auth, prefix) != cfg.BearerToken {
			// apfel emits exactly "Bearer" (no realm) so the wire shape is
			// minimal and predictable for SDKs.
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, "missing or invalid bearer token", cerr.Authentication)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLoopbackBind reports whether the supervisor's BindHost is loopback.
// Empty / "127.0.0.1" / "localhost" / "::1" are loopback. "0.0.0.0" or
// a routable IP are not.
func isLoopbackBind(host string) bool {
	if host == "" {
		return true // default bind is loopback
	}
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return strings.HasPrefix(host, "127.")
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
