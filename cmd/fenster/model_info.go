package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Arthur-Ficial/fenster/internal/backend"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

// runModelInfo prints model availability + capabilities. The format is
// chosen so apfel's pytest helpers (`require_model()`) recognize "available:
// yes" in stdout when the engine is ready.
func runModelInfo(ctx context.Context, asJSON bool) error {
	be, err := chooseBackend(ctx, false)
	if err != nil {
		return err
	}
	defer be.Close()
	h, err := be.Health(ctx)
	if err != nil {
		return err
	}
	if asJSON {
		out, _ := json.Marshal(map[string]any{
			"model":               wire.ModelID,
			"available":           h.Available,
			"context_window":      orInt(h.ContextWindow, wire.ContextWindow),
			"supported_languages": orStrings(h.SupportedLanguages, wire.SupportedLanguagesFallback()),
			"detail":              h.Detail,
		})
		fmt.Println(string(out))
		return nil
	}
	avail := "no"
	if h.Available {
		avail = "yes"
	}
	fmt.Printf("model:      %s\n", wire.ModelID)
	fmt.Printf("available:  %s\n", avail)
	fmt.Printf("context:    %d tokens\n", orInt(h.ContextWindow, wire.ContextWindow))
	langs := orStrings(h.SupportedLanguages, wire.SupportedLanguagesFallback())
	fmt.Printf("languages:  %v\n", langs)
	if h.Detail != "" {
		fmt.Printf("detail:     %s\n", h.Detail)
	}
	return nil
}

// chooseBackend's interface allows a no-op test helper to honour the same
// FENSTER_BACKEND switch the rest of the CLI uses.
var _ = (backend.Backend)(nil)

func orInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}
func orStrings(v, fallback []string) []string {
	if len(v) == 0 {
		return fallback
	}
	return v
}
