package ids

import (
	"strings"
	"testing"
)

func TestChatID_Prefix(t *testing.T) {
	id := ChatCompletion()
	if !strings.HasPrefix(id, "chatcmpl-") {
		t.Fatalf("expected chatcmpl- prefix, got %s", id)
	}
}

func TestChatID_Length(t *testing.T) {
	// apfel uses ~12 random chars after the prefix; we match.
	id := ChatCompletion()
	suffix := strings.TrimPrefix(id, "chatcmpl-")
	if len(suffix) < 8 {
		t.Fatalf("suffix should be ≥8 chars, got %q", suffix)
	}
}

func TestChatID_UniquePerCall(t *testing.T) {
	a := ChatCompletion()
	b := ChatCompletion()
	if a == b {
		t.Fatalf("two calls should produce distinct ids, both %s", a)
	}
}

func TestToolCallID_Prefix(t *testing.T) {
	id := ToolCall()
	if !strings.HasPrefix(id, "call_") {
		t.Fatalf("expected call_ prefix, got %s", id)
	}
}
