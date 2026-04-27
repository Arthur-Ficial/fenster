// Package wire holds the OpenAI-compatible JSON types fenster speaks.
// These tests are the FIRST line of TDD — they describe the exact JSON
// shape apfel emits, and any divergence is a bug.
package wire

import (
	"encoding/json"
	"strings"
	"testing"
)

// helper: marshal x to compact JSON and return string.
func mustMarshal(t *testing.T, x any) string {
	t.Helper()
	b, err := json.Marshal(x)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return string(b)
}

func TestMessage_AssistantContentAndRefusalAlwaysPresent(t *testing.T) {
	// apfel always emits "content": null and "refusal": null on assistant
	// messages even when both are nil. The pytest suite checks for the keys.
	m := Message{Role: "assistant"}
	got := mustMarshal(t, m)
	for _, key := range []string{`"content":null`, `"refusal":null`} {
		if !strings.Contains(got, key) {
			t.Errorf("missing key %s in %s", key, got)
		}
	}
}

func TestMessage_StringContentMarshalsAsString(t *testing.T) {
	m := Message{Role: "user", Content: TextContent("hello")}
	got := mustMarshal(t, m)
	if !strings.Contains(got, `"content":"hello"`) {
		t.Errorf("expected string content, got %s", got)
	}
}

func TestMessage_ToolCallRoundTrip(t *testing.T) {
	in := []byte(`{"role":"assistant","content":null,"refusal":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"add","arguments":"{\"a\":1}"}}]}`)
	var m Message
	if err := json.Unmarshal(in, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(m.ToolCalls) != 1 || m.ToolCalls[0].Function.Name != "add" {
		t.Errorf("tool call lost: %+v", m.ToolCalls)
	}
	out := mustMarshal(t, m)
	if !strings.Contains(out, `"name":"add"`) || !strings.Contains(out, `"refusal":null`) {
		t.Errorf("round-trip lost data: %s", out)
	}
}

func TestContent_PartsArrayUnmarshal(t *testing.T) {
	in := []byte(`[{"type":"text","text":"hi"},{"type":"image_url","image_url":{"url":"data:..."}}]`)
	var c Content
	if err := json.Unmarshal(in, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(c.Parts) != 2 || c.Parts[0].Type != "text" || c.Parts[1].Type != "image_url" {
		t.Errorf("parts lost: %+v", c)
	}
	if !c.HasImage() {
		t.Errorf("HasImage should detect image_url")
	}
}

func TestChoice_LogprobsAlwaysNull(t *testing.T) {
	c := Choice{Index: 0, Message: Message{Role: "assistant"}, FinishReason: "stop"}
	got := mustMarshal(t, c)
	if !strings.Contains(got, `"logprobs":null`) {
		t.Errorf("expected logprobs:null, got %s", got)
	}
}

func TestChunkChoice_FinishReasonNullable(t *testing.T) {
	c := ChunkChoice{Index: 0}
	got := mustMarshal(t, c)
	if !strings.Contains(got, `"finish_reason":null`) {
		t.Errorf("nil finish_reason should marshal as null, got %s", got)
	}
	stop := "stop"
	c.FinishReason = &stop
	got = mustMarshal(t, c)
	if !strings.Contains(got, `"finish_reason":"stop"`) {
		t.Errorf("string finish_reason lost, got %s", got)
	}
}

func TestChatCompletionResponse_Object(t *testing.T) {
	r := NewChatResponse("chatcmpl-x", 1700000000, "apple-foundationmodel", []Choice{
		{Index: 0, Message: Message{Role: "assistant", Content: TextContent("ok")}, FinishReason: "stop"},
	}, Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2})
	got := mustMarshal(t, r)
	for _, want := range []string{`"object":"chat.completion"`, `"id":"chatcmpl-x"`, `"model":"apple-foundationmodel"`, `"prompt_tokens":1`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %s in %s", want, got)
		}
	}
}

func TestChatCompletionChunk_Object(t *testing.T) {
	role := "assistant"
	c := NewChunk("chatcmpl-x", 1700000000, "apple-foundationmodel", []ChunkChoice{
		{Index: 0, Delta: Delta{Role: role}},
	})
	got := mustMarshal(t, c)
	if !strings.Contains(got, `"object":"chat.completion.chunk"`) {
		t.Errorf("object should be chat.completion.chunk, got %s", got)
	}
}
