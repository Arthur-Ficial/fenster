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
	"github.com/Arthur-Ficial/fenster/internal/chat"
	"github.com/Arthur-Ficial/fenster/internal/chrome"
	"github.com/Arthur-Ficial/fenster/internal/doctor"
	"github.com/Arthur-Ficial/fenster/internal/oneshot"
	"github.com/Arthur-Ficial/fenster/internal/server"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Exit codes — keep stable; apfel pytest asserts on them.
const (
	exitOK           = 0
	exitGenericError = 1
	exitInvalidArgs  = 2 // apfel convention; pytest asserts returncode == 2
	exitDoctorFail   = 3
	exitNotImpl      = 64
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
	// Pre-flag-parse: detect a bare trailing -f or --file with no value.
	// cobra would otherwise emit "flag needs an argument" with exit 1.
	if isBareFileFlag(os.Args[1:]) {
		fmt.Fprintln(os.Stderr, "fenster: -f/--file requires a file path")
		os.Exit(exitInvalidArgs)
	}
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
		// apfel-shaped help: "USAGE:" header (uppercase) so pytest's
		// `assert "USAGE:" in result.stdout` passes.
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
			if up, _ := cmd.Flags().GetBool("update"); up {
				return runUpdate(quiet)
			}
			if rl, _ := cmd.Flags().GetBool("release"); rl {
				return runRelease(quiet)
			}
			if serve {
				host, _ := cmd.Flags().GetString("host")
				token, _ := cmd.Flags().GetString("token")
				origins, _ := cmd.Flags().GetStringSlice("allowed-origins")
				corsOn, _ := cmd.Flags().GetBool("cors")
				publicHealth, _ := cmd.Flags().GetBool("public-health")
				footgun, _ := cmd.Flags().GetBool("footgun")
				noOrigin, _ := cmd.Flags().GetBool("no-origin-check")
				tokenAuto, _ := cmd.Flags().GetBool("token-auto")
				if tokenAuto {
					token = "auto"
				}
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
					NoOriginCheck:  noOrigin,
				})
			}
			if chat {
				return runChatMode(ctx, resolveSystem(system, noSystem), jsonOut, quiet, debug)
			}
			// One-shot UNIX tool path.
			prompt := strings.Join(args, " ")
			files, _ := cmd.Flags().GetStringSlice("file")
			fileBody, ferr := readFileFlags(files)
			if ferr != nil {
				return ferr
			}
			finalPrompt, perr := combinePromptAndFiles(fileBody, prompt)
			if perr != nil && fileBody == "" && prompt == "" {
				// Allow stdin-fed prompt.
				finalPrompt = ""
			}
			be, err := chooseBackend(ctx, debug)
			if err != nil {
				return err
			}
			defer be.Close()
			return oneshot.Run(ctx, oneshot.Options{
				Prompt:  finalPrompt,
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
	cmd.Flags().StringVarP(&system, "system", "s", envFlag("FENSTER_SYSTEM_PROMPT", "APFEL_SYSTEM_PROMPT"), "system prompt")
	cmd.Flags().BoolVar(&noSystem, "no-system-prompt", false, "disable the default system prompt")
	cmd.Flags().StringSliceP("file", "f", nil, "include file content in the prompt (repeatable)")
	cmd.Flags().Bool("update", false, "check for updates and self-upgrade if available")
	cmd.Flags().Bool("release", false, "print release notes for the running version")
	// security flags
	cmd.Flags().String("token", envFlag("FENSTER_TOKEN", "APFEL_TOKEN"), "bearer token (or 'auto' to generate one)")
	cmd.Flags().StringSlice("allowed-origins", nil, "additional CORS/origin allowlist entries (repeatable)")
	cmd.Flags().Bool("cors", envFlag("FENSTER_CORS", "APFEL_CORS") == "1", "enable CORS preflight responses")
	cmd.Flags().Bool("public-health", false, "allow /health without bearer token (--token mode)")
	cmd.Flags().Bool("footgun", false, "DANGER: disable origin and bearer checks")
	cmd.Flags().Bool("no-origin-check", false, "disable origin allowlist (preserves bearer auth)")
	cmd.Flags().Bool("token-auto", false, "auto-generate a bearer token and print to stderr")

	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newNMHostCmd())
	cmd.AddCommand(newInstallExtensionCmd())
	cmd.AddCommand(newInstallManifestCmd())

	// Customize help so it includes "USAGE:" (apfel convention) and
	// preserves cobra's tree under it.
	cmd.SetUsageTemplate(usageTemplateFor())
	// Map cobra's flag-parse errors (exit 1 default) to apfel's exit 2
	// + "unknown option" wording.
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		msg := err.Error()
		// Cobra: "unknown flag: --foo"  -> apfel: "unknown option: --foo"
		msg = strings.Replace(msg, "unknown flag:", "unknown option:", 1)
		fmt.Fprintln(os.Stderr, "fenster:", msg)
		return &exitError{code: exitInvalidArgs, msg: "", silent: true}
	})

	return cmd
}

// usageTemplateFor returns the colored or plain template depending on the
// caller's environment. ANSI on when stdout is a TTY and NO_COLOR is unset.
func usageTemplateFor() string {
	if term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == "" {
		return apfelUsageTemplateColor
	}
	return apfelUsageTemplate
}

// apfelUsageTemplateColor is the same as apfelUsageTemplate but the section
// headers (USAGE:, COMMANDS:, OPTIONS:, ...) are emitted bold via ANSI.
// pytest test_help_uses_ansi_under_tty checks for any ANSI escape; bold is
// the most readable choice that still satisfies the assertion.
const apfelUsageTemplateColor = "\x1b[1mUSAGE:\x1b[0m\n" + `  {{.UseLine}}{{if .HasAvailableSubCommands}} [command]{{end}}{{if gt (len .Aliases) 0}}

` + "\x1b[1mALIASES:\x1b[0m\n" + `  {{.NameAndAliases}}{{end}}{{if .HasExample}}

` + "\x1b[1mEXAMPLES:\x1b[0m\n" + `{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}

` + "\x1b[1mCOMMANDS:\x1b[0m" + `{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

` + "\x1b[1mOPTIONS:\x1b[0m\n" + `{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

` + "\x1b[1mINHERITED OPTIONS:\x1b[0m\n" + `{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

` + "\x1b[1mADDITIONAL HELP TOPICS:\x1b[0m" + `{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// apfelUsageTemplate makes cobra emit "USAGE:" (uppercase) and roughly
// matches apfel's --help layout while keeping cobra's command tree.
const apfelUsageTemplate = `USAGE:
  {{.UseLine}}{{if .HasAvailableSubCommands}} [command]{{end}}{{if gt (len .Aliases) 0}}

ALIASES:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

EXAMPLES:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}

COMMANDS:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

OPTIONS:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

INHERITED OPTIONS:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

ADDITIONAL HELP TOPICS:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

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
	NoOriginCheck  bool
}

func runServeModeFull(ctx context.Context, sf serveFlags) error {
	// fenster shares ONE Chrome instance across all --serve invocations.
	// First fenster launches Chrome; subsequent fensters attach via the
	// state file at ~/.fenster/run/chrome.json. pytest can spawn 20+
	// servers without launching 20+ Chromes.
	//
	// Disable with FENSTER_NO_CHROME=1 or FENSTER_BACKEND=echo|null.
	// Override discovery with FENSTER_CDP_URL=http://127.0.0.1:NNNN.
	wantChrome := os.Getenv("FENSTER_BACKEND") != "echo" &&
		os.Getenv("FENSTER_BACKEND") != "null" &&
		os.Getenv("FENSTER_NO_CHROME") == "" &&
		os.Getenv("FENSTER_CDP_URL") == ""
	if wantChrome {
		cdp, launched, err := chrome.EnsureSharedChrome(ctx, chrome.LaunchOptions{})
		if err != nil {
			fmt.Fprintln(os.Stderr, "fenster: shared Chrome unavailable:", err)
			fmt.Fprintln(os.Stderr, "fenster: continuing with EchoBackend; set FENSTER_NO_CHROME=1 to silence")
		} else {
			if launched {
				fmt.Fprintln(os.Stderr, "fenster: launched shared Chrome at", cdp)
			} else if sf.Debug {
				fmt.Fprintln(os.Stderr, "fenster: attaching to existing shared Chrome at", cdp)
			}
			_ = os.Setenv("FENSTER_CDP_URL", cdp)
		}
	}

	be, err := chooseServeBackend(ctx, sf.Debug)
	if err != nil {
		return err
	}
	defer be.Close()
	host := sf.Host
	if host == "" {
		host = "127.0.0.1"
	}
	addr := host + ":" + strconv.Itoa(sf.Port)
	token := sf.Token
	autoTokenUsed := false
	if token == "auto" {
		token = autoTokenUUID() // UUID-shaped so security_test's regex `[0-9A-Fa-f-]{36}` matches
		autoTokenUsed = true
	}
	cfg := server.Config{
		Backend:        be,
		EnableCORS:     sf.EnableCORS,
		BearerToken:    token,
		AllowedOrigins: sf.AllowedOrigins,
		NoOriginCheck:  sf.NoOriginCheck,
		PublicHealth:   sf.PublicHealth,
		Footgun:        sf.Footgun,
		BindHost:       host,
		Debug:          sf.Debug,
	}
	_ = autoTokenUsed
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
	if autoTokenUsed {
		// Operator opted into --token-auto; surface the generated secret to
		// stderr so they can use it (security_test_token_auto_prints_generated_secret).
		fmt.Fprintf(os.Stderr, "  token: %s\n", token)
	}
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

func runChatMode(ctx context.Context, system string, jsonOut, quiet, debug bool) error {
	be, err := chooseBackend(ctx, debug)
	if err != nil {
		return err
	}
	defer be.Close()
	return chat.Run(ctx, chat.Options{
		Backend: be,
		System:  system,
		JSON:    jsonOut,
		Quiet:   quiet,
		Debug:   debug,
	})
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

// chooseServeBackend is the serve-mode variant. Selection (in order):
//
//	FENSTER_BACKEND=echo                        -> EchoBackend
//	FENSTER_BACKEND=null                        -> NullBackend
//	FENSTER_CDP_URL=http://127.0.0.1:NNNN       -> ChromeCDPBackend (attach)
//	default                                     -> EchoBackend (safe)
//
// FENSTER_CDP_URL is the operator's hook to point fenster at an already-
// running Chrome (with the Prompt API flags enabled and the model
// downloaded). This is the production path while Canary auto-launch is
// being polished.
func chooseServeBackend(ctx context.Context, debug bool) (backend.Backend, error) {
	switch os.Getenv("FENSTER_BACKEND") {
	case "echo":
		return backend.EchoBackend{}, nil
	case "null":
		return backend.NullBackend{}, nil
	}
	if cdp := os.Getenv("FENSTER_CDP_URL"); cdp != "" {
		br, err := chrome.Attach(ctx, cdp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fenster: cannot attach to CDP at", cdp, ":", err)
			return backend.EchoBackend{}, nil
		}
		// Default the CDP target to fenster's own HTTP server's GET /
		// page — Built-in AI APIs are only exposed on real http:// origins.
		target := os.Getenv("FENSTER_CDP_TARGET")
		if target == "" {
			target = fmt.Sprintf("http://127.0.0.1:%d/", defaultPort())
		}
		cb, err := backend.NewChromeCDPBackend(br.BrowserCtx(), target)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fenster: ChromeCDPBackend init:", err)
			return backend.EchoBackend{}, nil
		}
		// Pre-warm the sentinel session in the background so the first
		// user request isn't cold. Fast AI is non-negotiable.
		cb.PreWarm()
		if debug {
			fmt.Fprintln(os.Stderr, "fenster: ChromeCDPBackend attached to", cdp, "(pre-warming sentinel)")
		}
		return cb, nil
	}
	if debug {
		fmt.Fprintln(os.Stderr, "fenster: defaulting to EchoBackend (set FENSTER_CDP_URL to attach to Chrome)")
	}
	return backend.EchoBackend{}, nil
}

func printVersion(jsonOut bool) {
	if jsonOut {
		fmt.Printf(`{"version":"%s","commit":"%s","branch":"%s","date":"%s","go":"%s","os":"%s"}`+"\n",
			buildinfo.Version, buildinfo.Commit, buildinfo.Branch, buildinfo.Date, buildinfo.GoVersion, buildinfo.OS)
		return
	}
	// apfel convention: stdout starts with "<binary> v<semver>" so pytest's
	// `assert result.stdout.startswith("fenster v")` passes.
	fmt.Printf("fenster v%s (%s, %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
}
