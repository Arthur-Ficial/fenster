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
	"path/filepath"
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

			// -o json maps to --json
			if out, _ := cmd.Flags().GetString("output"); strings.EqualFold(out, "json") {
				jsonOut = true
			}

			if showVer {
				printVersion(jsonOut)
				return nil
			}
			if runDoctor {
				return runDoctorMode(jsonOut)
			}
			if mi, _ := cmd.Flags().GetBool("model-info"); mi {
				return runModelInfo(ctx, jsonOut)
			}
			if serve {
				host, _ := cmd.Flags().GetString("host")
				token, _ := cmd.Flags().GetString("token")
				origins, _ := cmd.Flags().GetStringSlice("allowed-origins")
				corsOn, _ := cmd.Flags().GetBool("cors")
				publicHealth, _ := cmd.Flags().GetBool("public-health")
				footgun, _ := cmd.Flags().GetBool("footgun")
				return runServeModeFull(ctx, serveFlags{
					Port:           port,
					Host:           host,
					MCP:            mcp,
					Debug:          debug,
					Token:          token,
					AllowedOrigins: origins,
					EnableCORS:     corsOn,
					PublicHealth:   publicHealth,
					Footgun:        footgun,
				})
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
	cmd.Flags().Bool("model-info", false, "print model info (availability, languages, context window)")
	cmd.Flags().BoolVar(&serve, "serve", false, "run the OpenAI-compatible HTTP server")
	cmd.Flags().BoolVar(&chat, "chat", false, "interactive TUI chat")
	cmd.Flags().IntVar(&port, "port", defaultPort(), "port for --serve mode (env: APFEL_PORT/FENSTER_PORT)")
	cmd.Flags().String("host", envFlag("FENSTER_HOST", "APFEL_HOST"), "bind address for --serve (default 127.0.0.1)")
	cmd.Flags().StringVar(&mcp, "mcp", envFlag("FENSTER_MCP", "APFEL_MCP"), "path to an MCP server script (used with --serve)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON envelope output")
	cmd.Flags().StringP("output", "o", "", "output format (text|json)") // -o json alias for --json
	cmd.Flags().BoolVar(&stream, "stream", false, "stream tokens to stdout")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	cmd.Flags().BoolVar(&debug, "debug", false, "verbose debug logging")
	cmd.Flags().StringVar(&system, "system", envFlag("FENSTER_SYSTEM_PROMPT", "APFEL_SYSTEM_PROMPT"), "system prompt")
	cmd.Flags().BoolVar(&noSystem, "no-system-prompt", false, "disable the default system prompt")
	// security flags
	cmd.Flags().String("token", envFlag("FENSTER_TOKEN", "APFEL_TOKEN"), "bearer token (or 'auto' to generate one)")
	cmd.Flags().StringSlice("allowed-origins", nil, "additional CORS/origin allowlist entries (repeatable)")
	cmd.Flags().Bool("cors", envFlag("FENSTER_CORS", "APFEL_CORS") == "1", "enable CORS preflight responses")
	cmd.Flags().Bool("public-health", false, "allow /health without bearer token (--token mode)")
	cmd.Flags().Bool("footgun", false, "DANGER: disable origin and bearer checks")

	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newNMHostCmd())
	cmd.AddCommand(newInstallExtensionCmd())
	cmd.AddCommand(newInstallManifestCmd())

	return cmd
}

func newNMHostCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "nm-host",
		Short:  "internal: Native Messaging host invoked by Chrome",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return runNMHost(ctx)
		},
	}
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
	return runServeModeFull(ctx, serveFlags{Port: port, MCP: mcp, Debug: debug})
}

type serveFlags struct {
	Port           int
	Host           string
	MCP            string
	Debug          bool
	Token          string
	AllowedOrigins []string
	EnableCORS     bool
	PublicHealth   bool
	Footgun        bool
}

func runServeModeFull(ctx context.Context, sf serveFlags) error {
	// Zero-touch setup: extract extension, write NM manifest, spawn Chrome.
	autoChrome := os.Getenv("FENSTER_BACKEND") != "echo" && os.Getenv("FENSTER_BACKEND") != "null" && os.Getenv("FENSTER_NO_CHROME") == ""
	if autoChrome {
		_, extID, err := ensureExtensionAndManifest()
		if err != nil {
			fmt.Fprintln(os.Stderr, "fenster: extension/manifest setup failed:", err)
		} else if sf.Debug {
			fmt.Fprintln(os.Stderr, "fenster: extension ID =", extID)
		}
	}

	be, err := chooseServeBackend(ctx, sf.Debug)
	if err != nil {
		return err
	}
	defer be.Close()

	if autoChrome {
		home, _ := os.UserHomeDir()
		extDir := filepath.Join(home, ".fenster", "extension")
		if br, err := autoLaunchChrome(ctx, extDir, sf.Debug); err != nil {
			fmt.Fprintln(os.Stderr, "fenster: could not launch Chrome:", err)
			fmt.Fprintln(os.Stderr, "fenster: server still up; install extension manually to enable real model")
		} else {
			defer br.Close()
		}
	}
	host := sf.Host
	if host == "" {
		host = "127.0.0.1"
	}
	addr := host + ":" + strconv.Itoa(sf.Port)
	token := sf.Token
	if token == "auto" {
		token = autoToken()
		if !sf.Debug {
			fmt.Fprintf(os.Stderr, "fenster: auto-generated bearer token: %s\n", token)
		}
	}
	cfg := server.Config{
		Backend:        be,
		EnableCORS:     sf.EnableCORS,
		BearerToken:    token,
		AllowedOrigins: sf.AllowedOrigins,
		PublicHealth:   sf.PublicHealth,
		Footgun:        sf.Footgun,
		Debug:          sf.Debug,
	}
	if sf.MCP != "" {
		fmt.Fprintln(os.Stderr, "fenster: --mcp registered:", sf.MCP, "(MCP host-side wiring is M4)")
	}
	_ = sf.Port
	_ = sf.MCP
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
	// Apfel-style startup banner. The token field reports presence, not the
	// secret, so logs/test_explicit_token_not_echoed_in_startup_banner passes.
	tokenStatus := "open"
	if cfg.BearerToken != "" {
		tokenStatus = "required"
	}
	fmt.Fprintf(os.Stderr, "fenster %s — listening on http://%s/v1\n", buildinfo.Version, addr)
	fmt.Fprintf(os.Stderr, "  token:    %s\n", tokenStatus)
	fmt.Fprintf(os.Stderr, "  origin:   %s\n", originSummary(cfg.AllowedOrigins, cfg.Footgun))
	fmt.Fprintf(os.Stderr, "  cors:     %t\n", cfg.EnableCORS)
	if cfg.Debug {
		fmt.Fprintln(os.Stderr, "  debug:    on (logs at /v1/logs, /v1/logs/stats)")
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

// chooseBackend picks the Backend for the runtime.
//
// Selection (in order):
//
//	FENSTER_BACKEND=chrome     -> ChromeBackend (listens on bridge socket)
//	FENSTER_BACKEND=null       -> NullBackend
//	FENSTER_BACKEND=echo or "" -> EchoBackend (default, deterministic, no Chrome)
//
// The default is EchoBackend so the UNIX tool path works out of the box.
// `--serve` enables ChromeBackend automatically (see runServeModeFull).
func chooseBackend(ctx context.Context, debug bool) (backend.Backend, error) {
	_ = ctx
	switch os.Getenv("FENSTER_BACKEND") {
	case "chrome":
		cb, err := backend.NewChromeBackend(defaultBridgeSock())
		if err != nil {
			fmt.Fprintln(os.Stderr, "fenster: cannot start ChromeBackend:", err)
			return backend.EchoBackend{}, nil
		}
		if debug {
			fmt.Fprintln(os.Stderr, "fenster: ChromeBackend listening at", defaultBridgeSock())
		}
		return cb, nil
	case "null":
		return backend.NullBackend{}, nil
	default:
		if debug {
			fmt.Fprintln(os.Stderr, "fenster: using EchoBackend (set FENSTER_BACKEND=chrome for real Gemini Nano)")
		}
		return backend.EchoBackend{}, nil
	}
}

// chooseServeBackend is the serve-mode variant: prefer Chrome, fall back to Echo.
func chooseServeBackend(ctx context.Context, debug bool) (backend.Backend, error) {
	_ = ctx
	if os.Getenv("FENSTER_BACKEND") == "echo" {
		return backend.EchoBackend{}, nil
	}
	if os.Getenv("FENSTER_BACKEND") == "null" {
		return backend.NullBackend{}, nil
	}
	cb, err := backend.NewChromeBackend(defaultBridgeSock())
	if err != nil {
		fmt.Fprintln(os.Stderr, "fenster: ChromeBackend failed:", err)
		fmt.Fprintln(os.Stderr, "fenster: falling back to EchoBackend (set FENSTER_BACKEND=echo to silence)")
		return backend.EchoBackend{}, nil
	}
	if debug {
		fmt.Fprintln(os.Stderr, "fenster: ChromeBackend listening at", defaultBridgeSock())
	}
	return cb, nil
}

func printVersion(jsonOut bool) {
	if jsonOut {
		fmt.Printf(`{"version":"%s","commit":"%s","branch":"%s","date":"%s","go":"%s","os":"%s"}`+"\n",
			buildinfo.Version, buildinfo.Commit, buildinfo.Branch, buildinfo.Date, buildinfo.GoVersion, buildinfo.OS)
		return
	}
	fmt.Printf("fenster %s (%s, %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
}
