package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Arthur-Ficial/fenster/internal/core/ids"
	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
	"github.com/Arthur-Ficial/fenster/internal/core/validate"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

// handleChat dispatches /v1/chat/completions to the streaming or non-streaming
// path after running validation. DRY: validation lives in core/validate, not here.
func handleChat(cfg Config, state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		done := state.IncRequest()
		defer done()

		var req wire.ChatCompletionRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields() // apfel rejects strict unknown keys
		if err := dec.Decode(&req); err != nil {
			// Apfel doesn't strictly fail on unknown keys; relax this to a
			// regular decode so the pytest suite passes when clients send
			// extras. Keep the disallow-unknown for our own internal probes.
			if reqRetry := decodeLenient(r); reqRetry != nil {
				req = *reqRetry
			} else {
				writeSentinel(w, withMsg(cerr.ErrInvalidJSON, "Invalid JSON: "+err.Error()))
				return
			}
		}

		if err := validate.Request(&req); err != nil {
			var s *cerr.Sentinel
			if errors.As(err, &s) {
				writeSentinel(w, s)
				return
			}
			writeError(w, err.Error(), cerr.InvalidRequest)
			return
		}

		// Route to streaming or non-streaming.
		if req.IsStream() {
			runStreaming(w, r, cfg, &req)
			return
		}
		runNonStreaming(w, r, cfg, &req)
	}
}

func runNonStreaming(w http.ResponseWriter, r *http.Request, cfg Config, req *wire.ChatCompletionRequest) {
	res, err := cfg.Backend.Chat(r.Context(), req)
	if err != nil {
		var s *cerr.Sentinel
		if errors.As(err, &s) {
			writeSentinel(w, s)
			return
		}
		writeError(w, err.Error(), cerr.ServerError)
		return
	}

	msg := wire.Message{Role: "assistant"}
	if res.Refusal != "" {
		ref := res.Refusal
		msg.Refusal = &ref
		msg.Content = nil
	} else if len(res.ToolCalls) > 0 {
		msg.ToolCalls = res.ToolCalls
	} else {
		msg.Content = wire.TextContent(res.Content)
	}

	finish := res.FinishReason
	if finish == "" {
		if res.Refusal != "" {
			finish = wire.FinishContentFilter
		} else if len(res.ToolCalls) > 0 {
			finish = wire.FinishToolCalls
		} else {
			finish = wire.FinishStop
		}
	}

	resp := wire.NewChatResponse(
		ids.ChatCompletion(),
		time.Now().Unix(),
		wire.ModelID,
		[]wire.Choice{{Index: 0, Message: msg, FinishReason: finish}},
		wire.Usage{
			PromptTokens:     res.Usage.Prompt,
			CompletionTokens: res.Usage.Completion,
			TotalTokens:      res.Usage.Total(),
		},
	)
	writeJSON(w, http.StatusOK, resp)
}

// withMsg returns a Sentinel with an overridden message but the same Type/Status.
// DRY: many JSON-decode failures want to keep the type but specialise the message.
func withMsg(s *cerr.Sentinel, msg string) *cerr.Sentinel {
	return &cerr.Sentinel{Type: s.Type, Status: s.Status, Message: msg}
}

// decodeLenient is a fallback that doesn't reject unknown JSON fields.
func decodeLenient(r *http.Request) *wire.ChatCompletionRequest {
	// http.Request.Body has already been partially consumed; we read again
	// from the original raw bytes captured by middleware. Until we add that,
	// we just give up. Apfel's own tests don't rely on lenient decoding.
	return nil
}
