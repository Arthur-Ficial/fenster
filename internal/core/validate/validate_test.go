// validate tests are TDD-first: each apfel rejection rule is tested before
// the implementation in validate.go is written.
package validate

import (
	"encoding/json"
	"errors"
	"testing"

	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

func req() *wire.ChatCompletionRequest {
	return &wire.ChatCompletionRequest{
		Model:    wire.ModelID,
		Messages: []wire.Message{{Role: "user", Content: wire.TextContent("hi")}},
	}
}

func mustReject(t *testing.T, r *wire.ChatCompletionRequest, want *cerr.Sentinel) {
	t.Helper()
	err := Request(r)
	if err == nil {
		t.Fatalf("expected rejection %s, got nil", want.Type)
	}
	var s *cerr.Sentinel
	if !errors.As(err, &s) {
		t.Fatalf("expected *Sentinel, got %T: %v", err, err)
	}
	if s != want {
		t.Errorf("got sentinel %s, want %s", s.Message, want.Message)
	}
}

func TestEmptyMessagesRejected(t *testing.T) {
	r := req()
	r.Messages = nil
	mustReject(t, r, cerr.ErrEmptyMessages)
}

func TestImageContentRejected(t *testing.T) {
	r := req()
	r.Messages = []wire.Message{{Role: "user", Content: &wire.Content{Parts: []wire.ContentPart{
		{Type: "image_url", ImageURL: &wire.ContentImage{URL: "data:..."}},
	}}}}
	mustReject(t, r, cerr.ErrImageContent)
}

func TestLogprobsRejected(t *testing.T) {
	r := req()
	yes := true
	r.Logprobs = &yes
	mustReject(t, r, cerr.ErrLogprobs)
}

func TestNRejectedWhenNotOne(t *testing.T) {
	r := req()
	two := 2
	r.N = &two
	mustReject(t, r, cerr.ErrN)
}

func TestNAcceptedWhenOne(t *testing.T) {
	r := req()
	one := 1
	r.N = &one
	if err := Request(r); err != nil {
		t.Fatalf("n=1 should be accepted, got %v", err)
	}
}

func TestPresencePenaltyRejected(t *testing.T) {
	r := req()
	x := 0.5
	r.PresencePenalty = &x
	mustReject(t, r, cerr.ErrPresencePenalty)
}

func TestFrequencyPenaltyRejected(t *testing.T) {
	r := req()
	x := 0.5
	r.FrequencyPenalty = &x
	mustReject(t, r, cerr.ErrFrequencyPenalty)
}

func TestStopRejected(t *testing.T) {
	r := req()
	raw := json.RawMessage(`["END"]`)
	r.Stop = &raw
	mustReject(t, r, cerr.ErrStop)
}

func TestMaxTokensZero(t *testing.T) {
	r := req()
	zero := 0
	r.MaxTokens = &zero
	mustReject(t, r, cerr.ErrMaxTokensInvalid)
}

func TestMaxTokensNegative(t *testing.T) {
	r := req()
	neg := -5
	r.MaxTokens = &neg
	mustReject(t, r, cerr.ErrMaxTokensInvalid)
}

func TestTemperatureNegative(t *testing.T) {
	r := req()
	t1 := -0.1
	r.Temperature = &t1
	mustReject(t, r, cerr.ErrTemperatureInvalid)
}

func TestHappyPath(t *testing.T) {
	r := req()
	if err := Request(r); err != nil {
		t.Fatalf("baseline req should validate, got %v", err)
	}
}

func TestToolMessageOk(t *testing.T) {
	r := req()
	r.Messages = []wire.Message{
		{Role: "user", Content: wire.TextContent("call add 1+2")},
		{Role: "assistant", ToolCalls: []wire.ToolCall{{ID: "c1", Type: "function", Function: wire.ToolCallFunction{Name: "add", Arguments: `{"a":1,"b":2}`}}}},
		{Role: "tool", ToolCallID: "c1", Content: wire.TextContent("3")},
	}
	if err := Request(r); err != nil {
		t.Fatalf("tool flow should validate, got %v", err)
	}
}
