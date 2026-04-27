// oneshot is the UNIX tool path: take a prompt, hand it to a Backend, write
// the result to stdout. Tests assert behaviour against the deterministic
// EchoBackend so apfel-shaped CLI semantics can be checked without a model.
package oneshot

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Arthur-Ficial/fenster/internal/backend"
)

func runShot(t *testing.T, opts Options) (stdout, stderr string) {
	t.Helper()
	var so, se bytes.Buffer
	opts.Stdout = &so
	opts.Stderr = &se
	if opts.Backend == nil {
		opts.Backend = backend.EchoBackend{}
	}
	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run err: %v", err)
	}
	return so.String(), se.String()
}

func TestOneShot_Prompt_PrintsContent(t *testing.T) {
	out, _ := runShot(t, Options{Prompt: "hello world"})
	if !strings.Contains(out, "Echo: hello world") {
		t.Fatalf("expected echoed content, got %q", out)
	}
}

func TestOneShot_PromptEndsWithNewline(t *testing.T) {
	out, _ := runShot(t, Options{Prompt: "x"})
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected trailing newline, got %q", out)
	}
}

func TestOneShot_StdinFallback(t *testing.T) {
	out, _ := runShot(t, Options{Prompt: "", Stdin: strings.NewReader("from stdin\n")})
	if !strings.Contains(out, "from stdin") {
		t.Fatalf("expected stdin to feed prompt, got %q", out)
	}
}

func TestOneShot_RequiresPromptOrStdin(t *testing.T) {
	var so, se bytes.Buffer
	err := Run(context.Background(), Options{Stdout: &so, Stderr: &se, Backend: backend.EchoBackend{}})
	if err == nil {
		t.Fatal("expected error when no prompt and no stdin")
	}
}

func TestOneShot_JSON_EmitsEnvelope(t *testing.T) {
	out, _ := runShot(t, Options{Prompt: "hi", JSON: true})
	for _, want := range []string{`"object":"chat.completion"`, `"model":"gemini-nano"`, `"finish_reason":"stop"`, `"usage"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in JSON output: %s", want, out)
		}
	}
}

func TestOneShot_Stream_EmitsTokensProgressively(t *testing.T) {
	out, _ := runShot(t, Options{Prompt: "the quick brown fox", Stream: true})
	// EchoBackend chunks on space, we should see the full echo string.
	if !strings.Contains(out, "Echo:") || !strings.Contains(out, "fox") {
		t.Fatalf("expected streamed echo, got %q", out)
	}
}

func TestOneShot_Quiet_SuppressesStderr(t *testing.T) {
	_, errOut := runShot(t, Options{Prompt: "hi", Quiet: true})
	if errOut != "" {
		t.Fatalf("quiet should suppress stderr, got %q", errOut)
	}
}

func TestOneShot_System_PrependsSystemMessage(t *testing.T) {
	// The system prompt is just metadata; we check it's accepted without crashing
	// (echoing only echoes the user message).
	out, _ := runShot(t, Options{Prompt: "hello", System: "you are a parrot"})
	if !strings.Contains(out, "Echo: hello") {
		t.Fatalf("expected echo, got %q", out)
	}
}

func TestOneShot_BackendUnavailable_NonZero(t *testing.T) {
	var so, se bytes.Buffer
	err := Run(context.Background(), Options{
		Prompt:  "hi",
		Stdout:  &so,
		Stderr:  &se,
		Backend: backend.NullBackend{},
	})
	if err == nil {
		t.Fatal("expected error from NullBackend")
	}
}
