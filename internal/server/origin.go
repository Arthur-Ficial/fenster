package server

import (
	"net/url"
	"strings"
)

// DefaultOriginAllowlist returns the origins fenster (and apfel) allow by
// default: localhost variants only. Apfel matches on the host portion of
// the Origin header, accepting common loopback names regardless of port
// or scheme.
func DefaultOriginAllowlist() []string {
	return []string{
		"http://localhost",
		"http://127.0.0.1",
		"http://[::1]",
		"https://localhost",
		"https://127.0.0.1",
		"https://[::1]",
	}
}

// OriginAllowed reports whether the given Origin header value is in
// the allowlist. Empty origin → allowed (curl, SDK use, no CORS context).
//
// "*" in the allowlist matches any origin (footgun mode).
//
// For non-wildcard entries, we compare scheme + host (port-agnostic) so
// http://localhost:3000 matches an allowlist entry of http://localhost.
func OriginAllowed(origin string, allowlist []string) bool {
	if origin == "" {
		return true
	}
	if origin == "null" {
		// Browsers send Origin: null for some sandboxed contexts. Apfel
		// accepts these by default; we match.
		return true
	}
	for _, a := range allowlist {
		if a == "*" {
			return true
		}
		if originsMatch(origin, a) {
			return true
		}
	}
	return false
}

func originsMatch(origin, pattern string) bool {
	op, err := url.Parse(origin)
	if err != nil {
		return false
	}
	pp, err := url.Parse(pattern)
	if err != nil {
		return false
	}
	if op.Scheme != pp.Scheme {
		return false
	}
	oHost := strings.TrimSuffix(op.Hostname(), ".")
	pHost := strings.TrimSuffix(pp.Hostname(), ".")
	if oHost != pHost {
		return false
	}
	// Port: if pattern has a port, require an exact match; if not, accept any.
	if pp.Port() != "" && op.Port() != pp.Port() {
		return false
	}
	return true
}
