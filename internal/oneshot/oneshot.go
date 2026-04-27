// Package oneshot is the UNIX-tool entrypoint: accept a prompt (positional
// arg or stdin), hand it to a Backend, write the result to stdout. The CLI
// command in cmd/fenster/main.go composes this with backend selection and
// flag parsing.
//
// Modes:
//   - default: print Content followed by "\n"
//   - --stream: print content deltas as they arrive (no final newline if we
//     already wrote one)
//   - --json: emit a single OpenAI-compatible chat.completion JSON object
//
// The DRY rule: this module never speaks HTTP. It uses Backend directly.
// The HTTP server in internal/server is a separate consumer of Backend.
package oneshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Arthur-Ficial/fenster/internal/backend"
	"github.com/Arthur-Ficial/fenster/internal/core/ids"
	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

// Options configures a one-shot invocation.
type Options struct {
	Prompt  string
	System  string
	JSON    bool
	Stream  bool
	Quiet   bool
	Backend backend.Backend
	Stdin   io.Reader // optional; defaults to os.Stdin when nil
	Stdout  io.Writer // defaults to os.Stdout when nil
	Stderr  io.Writer // defaults to os.Stderr when nil
}

// Run executes the one-shot flow.
func Run(ctx context.Context, opts Options) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Backend == nil {
		return errors.New("oneshot: nil Backend")
	}

	prompt, err := resolvePrompt(opts)
	if err != nil {
		return err
	}
	req := buildRequest(opts.System, prompt, opts.Stream)

	// --json overrides --stream: collect the full response and emit one
	// JSON envelope. Apfel pytest's test_stdin_with_stream_flag and
	// test_stdin_only_with_stream_flag use `-o json --stream` and
	// json.loads() the entire stdout as a single object.
	if opts.Stream && !opts.JSON {
		return runStream(ctx, opts, req)
	}
	return runOnce(ctx, opts, req)
}

func resolvePrompt(opts Options) (string, error) {
	if opts.Prompt != "" {
		return opts.Prompt, nil
	}
	r := opts.Stdin
	if r == nil {
		r = os.Stdin
	}
	if r == os.Stdin {
		// Don't block forever on a TTY with no piped input.
		if isStdinTTY() {
			return "", errors.New("no prompt provided (and stdin is a terminal); pass a prompt argument or pipe input")
		}
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "", errors.New("no prompt provided (stdin was empty)")
	}
	return s, nil
}

func buildRequest(system, prompt string, stream bool) *wire.ChatCompletionRequest {
	msgs := []wire.Message{}
	if system != "" {
		msgs = append(msgs, wire.Message{Role: "system", Content: wire.TextContent(system)})
	}
	msgs = append(msgs, wire.Message{Role: "user", Content: wire.TextContent(prompt)})
	r := &wire.ChatCompletionRequest{Model: wire.ModelID, Messages: msgs}
	if stream {
		t := true
		r.Stream = &t
	}
	return r
}

func runOnce(ctx context.Context, opts Options, req *wire.ChatCompletionRequest) error {
	res, err := opts.Backend.Chat(ctx, req)
	if err != nil {
		return mapBackendErr(err)
	}
	if opts.JSON {
		// CLI -o json shape is flat {content, model, finish_reason, usage}
		// (apfel convention; pytest cli_e2e tests assert payload["content"]).
		// The HTTP /v1/chat/completions endpoint uses the OpenAI envelope.
		flat := map[string]any{
			"content":       res.Content,
			"model":         wire.ModelID,
			"finish_reason": orStop(res.FinishReason),
			"usage": map[string]int{
				"prompt_tokens":     res.Usage.Prompt,
				"completion_tokens": res.Usage.Completion,
				"total_tokens":      res.Usage.Total(),
			},
		}
		if res.Refusal != "" {
			flat["refusal"] = res.Refusal
		}
		return writeJSONNoNewline(opts.Stdout, flat)
	}
	if !opts.Quiet && res.Refusal != "" {
		fmt.Fprintln(opts.Stderr, "fenster: refusal —", res.Refusal)
	}
	if _, err := opts.Stdout.Write([]byte(res.Content)); err != nil {
		return err
	}
	if !strings.HasSuffix(res.Content, "\n") {
		_, _ = opts.Stdout.Write([]byte("\n"))
	}
	return nil
}

func runStream(ctx context.Context, opts Options, req *wire.ChatCompletionRequest) error {
	ch, err := opts.Backend.ChatStream(ctx, req)
	if err != nil {
		return mapBackendErr(err)
	}
	wroteAny := false
	for c := range ch {
		if c.Err != nil {
			return mapBackendErr(c.Err)
		}
		if c.ContentDelta != "" {
			if _, werr := opts.Stdout.Write([]byte(c.ContentDelta)); werr != nil {
				return werr
			}
			wroteAny = true
		}
	}
	if wroteAny {
		_, _ = opts.Stdout.Write([]byte("\n"))
	}
	return nil
}

func buildResponseEnvelope(res backend.Result) wire.ChatCompletionResponse {
	msg := wire.Message{Role: "assistant"}
	if res.Refusal != "" {
		r := res.Refusal
		msg.Refusal = &r
	} else {
		msg.Content = wire.TextContent(res.Content)
	}
	if len(res.ToolCalls) > 0 {
		msg.ToolCalls = res.ToolCalls
	}
	return wire.NewChatResponse(
		ids.ChatCompletion(),
		time.Now().Unix(),
		wire.ModelID,
		[]wire.Choice{{Index: 0, Message: msg, FinishReason: orStop(res.FinishReason)}},
		wire.Usage{
			PromptTokens:     res.Usage.Prompt,
			CompletionTokens: res.Usage.Completion,
			TotalTokens:      res.Usage.Total(),
		},
	)
}

func orStop(s string) string {
	if s == "" {
		return wire.FinishStop
	}
	return s
}

func writeJSON(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

// writeJSONNoNewline emits the JSON payload without a trailing newline.
// apfel's pytest test_json_output_no_trailing_newline asserts on this.
func writeJSONNoNewline(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func mapBackendErr(err error) error {
	if err == nil {
		return nil
	}
	var s *cerr.Sentinel
	if errors.As(err, &s) {
		return fmt.Errorf("%s: %s", s.Type, s.Message)
	}
	return err
}
