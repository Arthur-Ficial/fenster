package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Arthur-Ficial/fenster/internal/core/ids"
	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

func runStreaming(w http.ResponseWriter, r *http.Request, cfg Config, req *wire.ChatCompletionRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, "streaming not supported by ResponseWriter", cerr.ServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	id := ids.ChatCompletion()
	created := time.Now().Unix()

	// First chunk: role announcement.
	writeChunk(w, flusher, wire.NewChunk(id, created, wire.ModelID, []wire.ChunkChoice{
		{Index: 0, Delta: wire.Delta{Role: "assistant"}},
	}))

	ch, err := cfg.Backend.ChatStream(r.Context(), req)
	if err != nil {
		var s *cerr.Sentinel
		if errors.As(err, &s) {
			writeSentinel(w, s) // before stream starts? at this point we wrote 200 already; emit error frame instead
			return
		}
		writeError(w, err.Error(), cerr.ServerError)
		return
	}

	var lastFinish string
	var lastUsage *wire.Usage

	for c := range ch {
		if c.Err != nil {
			// Best-effort: surface as a comment frame; apfel just ends the stream.
			fmt.Fprintf(w, ": error %s\n\n", c.Err.Error())
			flusher.Flush()
			break
		}
		if c.ContentDelta != "" {
			delta := c.ContentDelta
			writeChunk(w, flusher, wire.NewChunk(id, created, wire.ModelID, []wire.ChunkChoice{
				{Index: 0, Delta: wire.Delta{Content: &delta}},
			}))
		}
		if len(c.ToolCalls) > 0 {
			writeChunk(w, flusher, wire.NewChunk(id, created, wire.ModelID, []wire.ChunkChoice{
				{Index: 0, Delta: wire.Delta{ToolCalls: c.ToolCalls}},
			}))
		}
		if c.FinishReason != "" {
			lastFinish = c.FinishReason
		}
		if c.Usage != nil {
			lastUsage = &wire.Usage{
				PromptTokens:     c.Usage.Prompt,
				CompletionTokens: c.Usage.Completion,
				TotalTokens:      c.Usage.Total(),
			}
		}
	}

	// Finish chunk.
	finish := lastFinish
	if finish == "" {
		finish = wire.FinishStop
	}
	writeChunk(w, flusher, wire.NewChunk(id, created, wire.ModelID, []wire.ChunkChoice{
		{Index: 0, Delta: wire.Delta{}, FinishReason: &finish},
	}))

	// Optional usage chunk.
	if req.IncludeUsageInStream() && lastUsage != nil {
		usageChunk := wire.NewChunk(id, created, wire.ModelID, []wire.ChunkChoice{})
		usageChunk.Usage = lastUsage
		writeChunk(w, flusher, usageChunk)
	}

	// Done.
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func writeChunk(w http.ResponseWriter, flusher http.Flusher, chunk wire.ChatCompletionChunk) {
	b, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", b)
	flusher.Flush()
}
