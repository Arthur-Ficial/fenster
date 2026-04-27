// Package main is the fenster CLI entry point.
//
// Modes (in apfel-priority order):
//
//  1. UNIX tool — `fenster "prompt"` or `echo prompt | fenster` — one-shot
//     completion to stdout. Pipes-friendly. --stream, --json, --quiet.
//  2. OpenAI HTTP server — `fenster --serve` — listens on :11434 by default.
//  3. Chat TUI byproduct — `fenster --chat`.
//  4. `fenster doctor` — preconditions check.
//  5. `fenster --version` — version + build info.
//
// The CLI talks to the model only through internal/backend.Backend so the
// same code paths work against EchoBackend (tests), NullBackend (no-Chrome
// fallback), and the real ChromeBackend.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Arthur-Ficial/fenster/internal/backend"
	"github.com/Arthur-Ficial/fenster/internal/buildinfo"
	"github.com/Arthur-Ficial/fenster/internal/doctor"
	"github.com/Arthur-Ficial/fenster/internal/oneshot"
	"github.com/Arthur-Ficial/fenster/internal/server"
	"github.com/spf13/cobra"
)

// Exit codes — keep stable; integration tests assert on them.
const (
	exitOK           = 0
	exitGenericError = 1
	exitNotImpl      = 2
	exitDoctorFail   = 3
	exitInvalidArgs  = 64 // sysexits EX_USAGE
)

// envFlag returns the first non-empty value among the given env names.
// DRY: APFEL_* and FENSTER_* live side-by-side without each flag rewriting
// this lookup.
func envFlag(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		var exitErr *exitError
		if errors.As(err, &exitErr) {
			if exitErr.msg != "" && !exitErr.silent {
				fmt.Fprintln(os.Stderr, "fenster:", exitErr.msg)
			}
			os.Exit(exitErr.code)
		}
		fmt.Fprintln(os.Stderr, "fenster:", err)
		os.Exit(exitGenericError)
	}
}

type exitError struct {
	code   int
	msg    string
	silent bool
}

func (e *exitError) Error() string { return e.msg }

func newRootCmd() *cobra.Command {
	var (
		serve    bool
		chat     bool
		port     int
		mcp      string
		showVer  bool
		jsonOut  bool
		stream   bool
		quiet    bool
		debug    bool
		system   string
		noSystem bool
		runDoctor bool
	)

	cmd := &cobra.Command{
		Use:   "fenster [prompt]",
		Short: "Chrome's on-device Gemini Nano, served as if it were OpenAI.",
		Long: `fenster wraps Chrome's Prompt API (Gemini Nano) and exposes it as a
UNIX tool, an OpenAI-compatible HTTP server on localhost:11434, and a small
chat TUI. Cross-platform sister of apfel.

Run 'fenster doctor' to verify your environment.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			if showVer {
				printVersion(jsonOut)
				return nil
			}
			if runDoctor {
				return runDoctorMode(jsonOut)
			}
			if serve {
				return runServeMode(ctx, port, mcp, debug)
			}
			if chat {
				return runChatMode(ctx)
			}
			// One-shot UNIX tool path.
			prompt := strings.Join(args, " ")
			be, err := chooseBackend(ctx, debug)
			if err != nil {
				return err
			}
			defer be.Close()
			return oneshot.Run(ctx, oneshot.Options{
				Prompt:  prompt,
				System:  resolveSystem(system, noSystem),
				JSON:    jsonOut,
				Stream:  stream,
				Quiet:   quiet,
				Backend: be,
			})
		},
	}

	cmd.Flags().BoolVar(&showVer, "version", false, "print version and exit")
	cmd.Flags().BoolVar(&runDoctor, "doctor", false, "run preconditions check")
	cmd.Flags().BoolVar(&serve, "serve", false, "run the OpenAI-compatible HTTP server")
	cmd.Flags().BoolVar(&chat, "chat", false, "interactive TUI chat")
	cmd.Flags().IntVar(&port, "port", defaultPort(), "port for --serve mode (env: APFEL_PORT/FENSTER_PORT)")
	cmd.Flags().StringVar(&mcp, "mcp", envFlag("FENSTER_MCP", "APFEL_MCP"), "path to an MCP server script (used with --serve)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON envelope output")
	cmd.Flags().BoolVar(&stream, "stream", false, "stream tokens to stdout")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
	cmd.Flags().BoolVar(&debug, "debug", false, "verbose debug logging")
	cmd.Flags().StringVar(&system, "system", envFlag("FENSTER_SYSTEM_PROMPT", "APFEL_SYSTEM_PROMPT"), "system prompt")
	cmd.Flags().BoolVar(&noSystem, "no-system-prompt", false, "disable the default system prompt")

	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func defaultPort() int {
	if v := envFlag("FENSTER_PORT", "APFEL_PORT"); v != "" {
		var p int
		_, _ = fmt.Sscanf(v, "%d", &p)
		if p > 0 && p < 65536 {
			return p
		}
	}
	return 11434
}

func resolveSystem(s string, noSystem bool) string {
	if noSystem {
		return ""
	}
	return s
}

func newVersionCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "print version and build info",
		Run: func(cmd *cobra.Command, args []string) {
			printVersion(jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON envelope")
	return cmd
}

func newDoctorCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "verify the environment for fenster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorMode(jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON output")
	return cmd
}

func runDoctorMode(jsonOut bool) error {
	r := doctor.Run()
	if jsonOut {
		fmt.Println(r.RenderJSON())
	} else {
		fmt.Println(r.RenderPlain())
	}
	if r.Status == doctor.StatusFailing {
		return &exitError{code: exitDoctorFail, msg: "", silent: true}
	}
	return nil
}

func runServeMode(ctx context.Context, port int, mcp string, debug bool) error {
	be, err := chooseBackend(ctx, debug)
	if err != nil {
		return err
	}
	defer be.Close()
	addr := "127.0.0.1:" + strconv.Itoa(port)
	if v := envFlag("FENSTER_HOST", "APFEL_HOST"); v != "" {
		addr = v + ":" + strconv.Itoa(port)
	}
	cfg := server.Config{
		Backend:    be,
		EnableCORS: envFlag("FENSTER_CORS", "APFEL_CORS") == "1",
		Debug:      debug,
	}
	if mcp != "" {
		fmt.Fprintln(os.Stderr, "fenster: --mcp registered:", mcp, "(MCP host-side wiring is M4)")
	}
	mux := server.NewMux(cfg)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	if !cfg.Debug {
		fmt.Fprintf(os.Stderr, "fenster %s — listening on http://%s/v1\n", buildinfo.Version, addr)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func runChatMode(ctx context.Context) error {
	_ = ctx
	fmt.Fprintln(os.Stderr, "fenster --chat: chat TUI is M4")
	return &exitError{code: exitNotImpl, msg: "chat mode not implemented"}
}

// chooseBackend picks the right Backend for non-server modes. Until the
// Chrome bridge lands we use EchoBackend so the UNIX tool is testable
// today; the Chrome backend will be wired here next.
func chooseBackend(ctx context.Context, debug bool) (backend.Backend, error) {
	_ = ctx
	if debug {
		fmt.Fprintln(os.Stderr, "fenster: using EchoBackend (Chrome bridge wires next)")
	}
	return backend.EchoBackend{}, nil
}

func printVersion(jsonOut bool) {
	if jsonOut {
		fmt.Printf(`{"version":"%s","commit":"%s","branch":"%s","date":"%s","go":"%s","os":"%s"}`+"\n",
			buildinfo.Version, buildinfo.Commit, buildinfo.Branch, buildinfo.Date, buildinfo.GoVersion, buildinfo.OS)
		return
	}
	fmt.Printf("fenster %s (%s, %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
}
