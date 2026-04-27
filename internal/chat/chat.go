// Package chat is the interactive TUI byproduct: `fenster --chat`.
// It's a small readline loop that forwards each user line to the Backend
// and prints the response. Multi-turn context is preserved across lines.
//
// Ctrl-C semantics (ported from apfel/Sources/CReadline/shim.h):
//   - SIGINT during the readline call exits 130 (128 + SIGINT signal number).
//   - On exit we write a "\n" to stderr and a terminal reset ("\x1b[0m") to
//     stdout when stdout is a TTY, so the shell prompt comes back clean
//     even if the user hit Ctrl-C mid-streamed-response.
//   - apfel uses a C-level sigaction so the handler is signal-async-safe;
//     in Go signal.Notify drives a goroutine that writes the same bytes
//     and calls os.Exit(130). The result is the same observable behaviour.
//   - During multi-turn loops, SIGINT outside the readline call (e.g. mid-
//     prompt to the model) cancels the request and returns to the prompt.
//     We implement this with context.WithCancel keyed off the SIGINT channel.
package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/Arthur-Ficial/fenster/internal/backend"
	"github.com/Arthur-Ficial/fenster/internal/buildinfo"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
	"golang.org/x/term"
)

// exitOnSigint is the apfel-shape Ctrl-C handler: reset terminal styling,
// newline, and exit(130). Safe to call from a signal-handler-style goroutine.
func exitOnSigint() {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		_, _ = os.Stdout.Write([]byte("\x1b[0m"))
	}
	_, _ = os.Stderr.Write([]byte("\n"))
	os.Exit(130)
}

// Options configures a chat session.
type Options struct {
	Backend backend.Backend
	System  string
	JSON    bool // emit one JSON object per turn (apfel chat-json mode)
	Quiet   bool
	Debug   bool
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

// Run blocks until the user types `quit`, `/quit`, `exit`, `/exit`, or EOF.
func Run(ctx context.Context, opts Options) error {
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	// SIGINT handler — apfel parity: exit 130 with terminal reset.
	// First Ctrl-C while a request is in flight cancels it; second Ctrl-C
	// while waiting at the prompt exits.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT)
	defer signal.Stop(sigCh)
	var inFlight atomic.Bool
	var requestCancel atomic.Pointer[context.CancelFunc]
	go func() {
		for range sigCh {
			if inFlight.Load() {
				if cancel := requestCancel.Load(); cancel != nil {
					(*cancel)()
				}
				continue
			}
			exitOnSigint()
		}
	}()
	// Header — fenster's own identity. The port-apfel-tests.sh script
	// rewrites "Apple Intelligence" → "Fenster Intelligence" in the
	// vendored test files so apfel's pytest assertions still trip.
	if !opts.Quiet && !opts.JSON {
		fmt.Fprintf(opts.Stdout, "fenster v%s — Fenster Intelligence (Gemini Nano via Chrome)\n", buildinfo.Version)
		fmt.Fprintln(opts.Stdout, "Type 'quit' to exit, Ctrl-D for EOF.")
		if opts.System != "" {
			fmt.Fprintf(opts.Stdout, "System: %s\n", opts.System)
		}
		fmt.Fprintln(opts.Stdout)
	}

	// Multi-turn context: every accepted line appends to history.
	history := make([]wire.Message, 0, 16)
	if opts.System != "" {
		history = append(history, wire.Message{Role: "system", Content: wire.TextContent(opts.System)})
	}

	scanner := bufio.NewScanner(opts.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(opts.Stdout, "Goodbye.")
			return nil
		default:
		}
		if !opts.Quiet && !opts.JSON {
			fmt.Fprint(opts.Stdout, "> ")
		}
		if !scanner.Scan() {
			// EOF (Ctrl-D) or read error → graceful exit.
			fmt.Fprintln(opts.Stdout)
			fmt.Fprintln(opts.Stdout, "Goodbye.")
			return nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		switch line {
		case "quit", "/quit", "exit", "/exit", ":q", "/q":
			fmt.Fprintln(opts.Stdout, "Goodbye.")
			return nil
		}

		history = append(history, wire.Message{Role: "user", Content: wire.TextContent(line)})
		req := &wire.ChatCompletionRequest{
			Model:    wire.ModelID,
			Messages: history,
		}
		// Wrap the request in a cancellable context so a mid-flight
		// Ctrl-C (first press) aborts THIS turn and returns to the prompt.
		reqCtx, cancel := context.WithCancel(ctx)
		requestCancel.Store(&cancel)
		inFlight.Store(true)
		res, err := opts.Backend.Chat(reqCtx, req)
		inFlight.Store(false)
		requestCancel.Store(nil)
		cancel()
		if err != nil {
			if opts.JSON {
				_ = json.NewEncoder(opts.Stdout).Encode(map[string]any{"role": "error", "content": err.Error()})
			} else {
				fmt.Fprintln(opts.Stderr, "fenster: chat error:", err)
			}
			continue
		}
		// Append assistant turn to history (so the next user turn sees context).
		history = append(history, wire.Message{Role: "assistant", Content: wire.TextContent(res.Content)})
		if opts.JSON {
			// JSONL — one object per turn (apfel test_chat_json_emits_jsonl).
			_ = json.NewEncoder(opts.Stdout).Encode(map[string]any{
				"role": "user", "content": line,
			})
			_ = json.NewEncoder(opts.Stdout).Encode(map[string]any{
				"role": "assistant", "content": res.Content,
				"finish_reason": orStop(res.FinishReason),
			})
			continue
		}
		// Plain output: "AI: <content>" prefix matches apfel test_chat_plain_shows_ai_prefix.
		fmt.Fprintf(opts.Stdout, "AI: %s\n\n", strings.TrimRight(res.Content, "\n"))
	}
}

func orStop(s string) string {
	if s == "" {
		return wire.FinishStop
	}
	return s
}
