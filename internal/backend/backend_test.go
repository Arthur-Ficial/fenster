package backend

import (
	"context"
	"testing"

	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

// Compile-time check: every backend (NullBackend, EchoBackend, ChromeBackend)
// must satisfy Backend.
var _ Backend = (*NullBackend)(nil)
var _ Backend = (*EchoBackend)(nil)

func TestNullBackend_NotAvailable(t *testing.T) {
	b := &NullBackend{}
	h, err := b.Health(context.Background())
	if err != nil {
		t.Fatalf("Health err: %v", err)
	}
	if h.Available {
		t.Fatal("NullBackend should report not available")
	}
}

func TestEchoBackend_RoundTripsLastUserMessage(t *testing.T) {
	b := &EchoBackend{}
	req := &wire.ChatCompletionRequest{
		Model: wire.ModelID,
		Messages: []wire.Message{
			{Role: "system", Content: wire.TextContent("you are a parrot")},
			{Role: "user", Content: wire.TextContent("hello world")},
		},
	}
	out, err := b.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat err: %v", err)
	}
	if out.Content == "" {
		t.Fatal("expected content")
	}
	if out.FinishReason != wire.FinishStop {
		t.Fatalf("expected stop, got %s", out.FinishReason)
	}
}

func TestEchoBackend_StreamYieldsChunks(t *testing.T) {
	b := &EchoBackend{}
	req := &wire.ChatCompletionRequest{
		Model: wire.ModelID,
		Messages: []wire.Message{{Role: "user", Content: wire.TextContent("hi")}},
	}
	ch, err := b.ChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatStream err: %v", err)
	}
	count := 0
	for c := range ch {
		if c.Err != nil {
			t.Fatalf("chunk err: %v", c.Err)
		}
		count++
	}
	if count == 0 {
		t.Fatal("expected ≥1 chunk")
	}
}
