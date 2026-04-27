// Package main is the fenster CLI entry point.
//
// fenster is the cross-platform sister of apfel: it exposes Chrome's on-device
// Gemini Nano (via the Prompt API, headless Chrome, and Native Messaging) as
// an OpenAI-compatible HTTP server on localhost.
//
// At M0 the CLI is a stub: --version prints the version, --serve and --chat
// are not yet wired (M3, M4). This is intentional — the apfel-compat pytest
// suite is meant to start RED.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Arthur-Ficial/fenster/internal/buildinfo"
	"github.com/spf13/cobra"
)

// Exit codes — keep stable; integration tests assert on them.
const (
	exitOK            = 0
	exitGenericError  = 1
	exitNotImpl       = 2
	exitInvalidArgs   = 64 // matches sysexits EX_USAGE
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		var exitErr *exitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.code)
		}
		os.Exit(exitGenericError)
	}
}

type exitError struct {
	code int
	msg  string
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
	)

	cmd := &cobra.Command{
		Use:   "fenster [prompt]",
		Short: "Chrome's on-device Gemini Nano, served as if it were OpenAI.",
		Long: `fenster wraps Chrome's Prompt API (Gemini Nano) and exposes it as an
OpenAI-compatible HTTP server on localhost:11434.

At M0 (current milestone) the CLI is a stub. --version is the only working flag.
The apfel-compat integration suite (Tests/integration/) is intentionally RED.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVer {
				printVersion(jsonOut)
				return nil
			}
			if serve {
				fmt.Fprintln(os.Stderr, "fenster --serve: not implemented yet (M3 — see https://github.com/Arthur-Ficial/fenster/milestones)")
				return &exitError{code: exitNotImpl, msg: "serve mode not implemented"}
			}
			if chat {
				fmt.Fprintln(os.Stderr, "fenster --chat: not implemented yet (M4 — see https://github.com/Arthur-Ficial/fenster/milestones)")
				return &exitError{code: exitNotImpl, msg: "chat mode not implemented"}
			}
			if len(args) == 0 {
				_ = cmd.Help()
				return &exitError{code: exitInvalidArgs, msg: "no prompt provided"}
			}
			fmt.Fprintln(os.Stderr, "fenster (one-shot prompt): not implemented yet (M3 requires the full server stack)")
			return &exitError{code: exitNotImpl, msg: "one-shot mode not implemented"}
		},
	}

	cmd.Flags().BoolVar(&showVer, "version", false, "print version and exit")
	cmd.Flags().BoolVar(&serve, "serve", false, "run the OpenAI-compatible HTTP server (M3)")
	cmd.Flags().BoolVar(&chat, "chat", false, "interactive TUI chat (M4)")
	cmd.Flags().IntVar(&port, "port", 11434, "port for --serve mode")
	cmd.Flags().StringVar(&mcp, "mcp", "", "path to an MCP server script (used with --serve)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON envelope output")
	cmd.Flags().BoolVar(&stream, "stream", false, "stream tokens to stdout")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
	cmd.Flags().BoolVar(&debug, "debug", false, "verbose debug logging")
	cmd.Flags().StringVar(&system, "system", "", "system prompt")
	cmd.Flags().BoolVar(&noSystem, "no-system-prompt", false, "disable the default system prompt")

	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
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
	return &cobra.Command{
		Use:   "doctor",
		Short: "check preconditions (Chrome, GPU, disk, model) — M2",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "fenster doctor: not implemented yet (M2)")
			return &exitError{code: exitNotImpl, msg: "doctor not implemented"}
		},
	}
}

func printVersion(jsonOut bool) {
	if jsonOut {
		fmt.Printf(`{"version":"%s","commit":"%s","branch":"%s","date":"%s","go":"%s","os":"%s"}`+"\n",
			buildinfo.Version, buildinfo.Commit, buildinfo.Branch, buildinfo.Date, buildinfo.GoVersion, buildinfo.OS)
		return
	}
	fmt.Printf("fenster %s (%s, %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
}
