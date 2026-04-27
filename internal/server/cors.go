package server

import "net/http"

// handleCORS returns a 204 preflight response. When --footgun is set,
// preflight echoes ANY requested headers (the client gets to decide). When
// --cors is set without footgun, preflight echoes Access-Control-Request-
// Headers and lists standard methods. When neither is set, preflight is
// still 204 but with no ACAO/AC-* headers (apfel default).
func handleCORS(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.EnableCORS || cfg.Footgun {
			origin := r.Header.Get("Origin")
			if origin == "" || cfg.Footgun {
				origin = "*"
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if origin != "*" {
				w.Header().Set("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			if reqHdr := r.Header.Get("Access-Control-Request-Headers"); reqHdr != "" {
				w.Header().Set("Access-Control-Allow-Headers", reqHdr)
			} else if cfg.Footgun {
				w.Header().Set("Access-Control-Allow-Headers", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// withCORSResponse decorates non-OPTIONS responses with ACAO/Vary when
// EnableCORS is set. We don't add the headers when CORS is disabled so
// apfel's "CORS-disabled" tests assert correctly.
func withCORSResponse(cfg Config, h http.Handler) http.Handler {
	if !cfg.EnableCORS {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodOptions {
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
		}
		h.ServeHTTP(w, r)
	})
}
