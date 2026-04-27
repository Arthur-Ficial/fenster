package server

import (
	"net/http"

	"github.com/Arthur-Ficial/fenster/internal/buildinfo"
	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

func handleHealth(cfg Config, state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h, err := cfg.Backend.Health(r.Context())
		if err != nil {
			writeError(w, err.Error(), cerr.ServerError)
			return
		}
		ctxWindow := h.ContextWindow
		if ctxWindow == 0 {
			ctxWindow = wire.ContextWindow
		}
		langs := h.SupportedLanguages
		if langs == nil {
			langs = wire.SupportedLanguagesFallback()
		}
		status := "ok"
		if !h.Available {
			status = "model_unavailable"
		}
		writeJSON(w, http.StatusOK, wire.HealthInfo{
			Status:             status,
			Model:              wire.ModelID,
			Version:            buildinfo.Version,
			ActiveRequests:     int(state.ActiveRequests()),
			ContextWindow:      ctxWindow,
			ModelAvailable:     h.Available,
			SupportedLanguages: langs,
		})
	}
}
