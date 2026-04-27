// Package backend defines the Backend interface — the seam between fenster's
// core (wire types, validation, server, CLI) and the underlying model engine.
//
// fenster ships three implementations:
//   - NullBackend: always reports "model unavailable"; safe default when
//     Chrome isn't ready (used by doctor before the bridge stands up).
//   - EchoBackend: echoes the last user message; useful for tests that
//     check the wire format end-to-end without needing a real model.
//   - ChromeBackend: real implementation (Native Messaging → Chrome
//     extension → Prompt API → Gemini Nano). Lives in internal/backend/chrome.
//
// All three speak the same Backend contract.
package backend

import (
	"context"
	"strings"

	"github.com/Arthur-Ficial/fenster/internal/core/tokens"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

// Backend is the interface any model engine implements.
type Backend interface {
	// Health reports whether the engine can answer prompts right now.
	Health(ctx context.Context) (Health, error)
	// Chat produces a single complete response for the request.
	Chat(ctx context.Context, req *wire.ChatCompletionRequest) (Result, error)
	// ChatStream streams chunks; the channel closes when generation ends.
	// Each Chunk carries either a content delta, a tool-call delta, or an Err.
	ChatStream(ctx context.Context, req *wire.ChatCompletionRequest) (<-chan Chunk, error)
	// Close releases any resources (Chrome processes, sockets, etc).
	Close() error
}

// Health is the engine readiness report.
type Health struct {
	Available          bool
	Detail             string   // human-readable explanation when not available
	SupportedLanguages []string // optional; engine may leave empty
	ContextWindow      int      // optional; 0 means "use wire.ContextWindow"
}

// Result is a single non-streaming completion.
type Result struct {
	Content      string
	Refusal      string // non-empty -> assistant message gets refusal field, finish=content_filter
	ToolCalls    []wire.ToolCall
	FinishReason string
	Usage        tokens.Usage
}

// Chunk is one streaming delta.
type Chunk struct {
	ContentDelta string
	ToolCalls    []wire.ToolCall // emit a tool-call chunk
	FinishReason string          // non-empty on the terminal chunk
	Usage        *tokens.Usage   // emitted right before [DONE] when include_usage
	Err          error
}

// ----- NullBackend -----

// NullBackend reports "unavailable". Used as a safe default when no engine is
// wired yet (e.g. before `fenster doctor` confirms the bridge).
type NullBackend struct{}

// Health returns unavailable.
func (NullBackend) Health(_ context.Context) (Health, error) {
	return Health{Available: false, Detail: "no backend wired (run `fenster doctor`)"}, nil
}

// Chat refuses to produce.
func (NullBackend) Chat(_ context.Context, _ *wire.ChatCompletionRequest) (Result, error) {
	return Result{}, errBackendUnavailable
}

// ChatStream refuses to produce.
func (NullBackend) ChatStream(_ context.Context, _ *wire.ChatCompletionRequest) (<-chan Chunk, error) {
	return nil, errBackendUnavailable
}

// Close is a no-op.
func (NullBackend) Close() error { return nil }

// ----- EchoBackend -----

// EchoBackend echoes the last user message back as the assistant. Useful for
// host-side tests that don't need a real model but DO need wire-format honesty.
// It computes honest token counts and produces correct stream/non-stream output.
type EchoBackend struct{}

// Health is always available.
func (EchoBackend) Health(_ context.Context) (Health, error) {
	return Health{Available: true, Detail: "echo backend (test-only)", ContextWindow: wire.ContextWindow, SupportedLanguages: []string{"en", "ja", "es"}}, nil
}

// Chat returns "Echo: " + last user content.
func (e EchoBackend) Chat(_ context.Context, req *wire.ChatCompletionRequest) (Result, error) {
	out := echoFor(req)
	prompt := echoPromptText(req)
	return Result{
		Content:      out,
		FinishReason: wire.FinishStop,
		Usage:        tokens.Usage{Prompt: tokens.Estimate(prompt), Completion: tokens.Estimate(out)},
	}, nil
}

// ChatStream streams a few content chunks, then a finish chunk.
func (e EchoBackend) ChatStream(_ context.Context, req *wire.ChatCompletionRequest) (<-chan Chunk, error) {
	out := make(chan Chunk, 4)
	go func() {
		defer close(out)
		text := echoFor(req)
		// Split on space so streaming sends multiple chunks.
		parts := strings.SplitAfter(text, " ")
		for _, p := range parts {
			if p == "" {
				continue
			}
			out <- Chunk{ContentDelta: p}
		}
		usage := &tokens.Usage{Prompt: tokens.Estimate(echoPromptText(req)), Completion: tokens.Estimate(text)}
		out <- Chunk{FinishReason: wire.FinishStop, Usage: usage}
	}()
	return out, nil
}

// Close is a no-op.
func (EchoBackend) Close() error { return nil }

func echoFor(req *wire.ChatCompletionRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return "Echo: " + req.Messages[i].Content.AsString()
		}
	}
	return "Echo: (no user message)"
}

func echoPromptText(req *wire.ChatCompletionRequest) string {
	var b strings.Builder
	for _, m := range req.Messages {
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content.AsString())
		b.WriteString("\n")
	}
	return b.String()
}
