package server

import (
	"net/http"

	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

func handleModels() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, wire.ModelsList{
			Object: wire.ObjectList,
			Data: []wire.ModelObject{{
				ID:                    wire.ModelID,
				Object:                wire.ObjectModel,
				Created:               1719792000,
				OwnedBy:               "apple",
				ContextWindow:         wire.ContextWindow,
				SupportedParameters:   []string{"temperature", "max_tokens", "seed", "stream", "tools", "tool_choice", "response_format", "x_context_strategy", "x_context_max_turns", "x_context_output_reserve"},
				UnsupportedParameters: []string{"logprobs", "n", "stop", "presence_penalty", "frequency_penalty"},
				Notes:                 "Apple-compatible on-device model identity. fenster routes inference to Chrome's Prompt API (Gemini Nano) under the hood.",
			}},
		})
	}
}
