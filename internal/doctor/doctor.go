// Package doctor probes the host and reports whether fenster can serve
// Gemini Nano right now. The output is intentionally explicit: every missing
// dependency comes with a Fix line telling the user exactly what to do.
//
// `fenster doctor` runs this and prints either a plain human-readable report
// (default) or JSON (with --json). The CLI also runs a quick subset of
// these checks at startup so a bare `fenster --serve` can refuse with a
// helpful pointer instead of a cryptic error.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Status values for both the overall report and individual checks.
const (
	StatusOK       = "ok"
	StatusDegraded = "degraded"
	StatusFailing  = "failing"
)

// Check IDs (stable; doctor tests assert on these).
const (
	CheckOS        = "os"
	CheckChrome    = "chrome"
	CheckGPU       = "gpu"
	CheckDisk      = "disk"
	CheckProfile   = "profile"
	CheckPromptAPI = "prompt_api"
)

// Report is the full doctor output.
type Report struct {
	Status   string    `json:"status"`
	Time     string    `json:"time"`
	Hostname string    `json:"hostname"`
	OS       string    `json:"os"`
	Arch     string    `json:"arch"`
	Checks   []Check   `json:"checks"`
	Summary  string    `json:"summary"`
}

// Check is one diagnostic line.
type Check struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // StatusOK | StatusDegraded | StatusFailing
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// Run executes every check and returns the report. Safe to call concurrently;
// it spawns short subprocess probes (chrome --version, df, etc).
func Run() Report {
	hostname, _ := os.Hostname()
	r := Report{
		Time:     time.Now().UTC().Format(time.RFC3339),
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}
	r.Checks = []Check{
		checkOS(),
		checkChrome(),
		checkGPU(),
		checkDisk(),
		checkProfile(),
		checkPromptAPI(),
	}
	r.Status, r.Summary = aggregate(r.Checks)
	return r
}

// RenderPlain produces a human-readable report.
func (r Report) RenderPlain() string {
	var b strings.Builder
	fmt.Fprintf(&b, "fenster doctor — %s/%s on %s\n", r.OS, r.Arch, r.Hostname)
	fmt.Fprintf(&b, "Overall: %s — %s\n\n", strings.ToUpper(r.Status), r.Summary)
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "%s %-12s %s\n", icon(c.Status), c.ID, c.Title)
		if c.Detail != "" {
			fmt.Fprintf(&b, "             %s\n", c.Detail)
		}
		if c.Fix != "" {
			fmt.Fprintf(&b, "  → fix:    %s\n", c.Fix)
		}
	}
	return b.String()
}

// RenderJSON produces a machine-readable JSON envelope.
func (r Report) RenderJSON() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

// CanInfer is the quick yes/no the CLI uses at startup before --serve.
func (r Report) CanInfer() bool {
	for _, c := range r.Checks {
		if c.Status == StatusFailing {
			return false
		}
	}
	return true
}

// ----- individual checks -----

func checkOS() Check {
	c := Check{ID: CheckOS, Title: "Operating system"}
	switch runtime.GOOS {
	case "darwin":
		ver := sw_versString()
		c.Status = StatusOK
		c.Detail = "macOS " + ver + " (darwin/" + runtime.GOARCH + ")"
		// macOS 13+ for Chrome's Prompt API.
		if compareSemver(ver, "13.0.0") < 0 {
			c.Status = StatusFailing
			c.Fix = "Upgrade macOS to 13 (Ventura) or newer"
		}
	case "linux":
		c.Status = StatusOK
		c.Detail = "linux/" + runtime.GOARCH
	case "windows":
		c.Status = StatusOK
		c.Detail = "windows/" + runtime.GOARCH
	default:
		c.Status = StatusFailing
		c.Detail = runtime.GOOS + "/" + runtime.GOARCH
		c.Fix = "Run on macOS 13+, Linux desktop, or Windows 10/11"
	}
	return c
}

func checkChrome() Check {
	c := Check{ID: CheckChrome, Title: "Google Chrome 138+"}
	path := chromePath()
	if path == "" {
		c.Status = StatusFailing
		c.Detail = "Chrome not found in default install paths"
		c.Fix = "Install Google Chrome from https://www.google.com/chrome (or set FENSTER_BROWSER to your Chromium binary)"
		return c
	}
	ver := chromeVersion(path)
	c.Detail = path + " (" + ver + ")"
	if !chromeAtLeast(ver, 138) {
		c.Status = StatusFailing
		c.Fix = "Upgrade Chrome to 138+ (chrome://settings/help)"
		return c
	}
	c.Status = StatusOK
	return c
}

func checkGPU() Check {
	c := Check{ID: CheckGPU, Title: "GPU acceleration"}
	if runtime.GOOS == "darwin" {
		// Apple Silicon: integrated GPU on every M-series Mac.
		c.Status = StatusOK
		c.Detail = "Apple Silicon integrated GPU assumed available"
		return c
	}
	// On Linux/Windows we trust Chrome's own probe (chrome://gpu) at runtime.
	c.Status = StatusDegraded
	c.Detail = "GPU presence is verified at Chrome startup; not probed by doctor"
	c.Fix = "If `fenster --serve` reports model_unavailable, check chrome://gpu in your normal Chrome"
	return c
}

func checkDisk() Check {
	c := Check{ID: CheckDisk, Title: "≥22 GB free in profile dir"}
	dir := profileDir()
	free, err := freeBytesAt(dir)
	if err != nil {
		c.Status = StatusDegraded
		c.Detail = "could not statfs " + dir + ": " + err.Error()
		c.Fix = "Free up disk space; Chrome needs ~22 GB for Gemini Nano"
		return c
	}
	gb := free / (1024 * 1024 * 1024)
	c.Detail = fmt.Sprintf("%d GB free at %s", gb, dir)
	if gb < 22 {
		c.Status = StatusFailing
		c.Fix = "Free up disk space (Gemini Nano model is ~2.4 GB but Chrome evicts under 10 GB free)"
		return c
	}
	c.Status = StatusOK
	return c
}

func checkProfile() Check {
	c := Check{ID: CheckProfile, Title: "fenster profile dir writable"}
	dir := profileDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.Status = StatusFailing
		c.Detail = err.Error()
		c.Fix = "Ensure " + dir + " is writable (fenster creates this on first run)"
		return c
	}
	test := filepath.Join(dir, ".write-test")
	if err := os.WriteFile(test, []byte("ok"), 0o644); err != nil {
		c.Status = StatusFailing
		c.Detail = err.Error()
		c.Fix = "Ensure " + dir + " is writable"
		return c
	}
	_ = os.Remove(test)
	c.Status = StatusOK
	c.Detail = dir
	return c
}

func checkPromptAPI() Check {
	c := Check{ID: CheckPromptAPI, Title: "Chrome Prompt API + Gemini Nano model"}
	c.Status = StatusDegraded
	c.Detail = "model availability is reported at runtime via chrome://on-device-internals"
	c.Fix = "Run `fenster --serve` once; the supervisor will trigger the on-demand model download (~2.4 GB)"
	return c
}

// ----- helpers -----

func aggregate(checks []Check) (string, string) {
	failing, degraded := 0, 0
	for _, c := range checks {
		switch c.Status {
		case StatusFailing:
			failing++
		case StatusDegraded:
			degraded++
		}
	}
	switch {
	case failing > 0:
		return StatusFailing, fmt.Sprintf("%d blocking issue(s); fenster cannot serve", failing)
	case degraded > 0:
		return StatusDegraded, fmt.Sprintf("%d non-blocking item(s); fenster should work, verify at runtime", degraded)
	default:
		return StatusOK, "All checks passed; fenster is ready to serve"
	}
}

func icon(s string) string {
	switch s {
	case StatusOK:
		return "✓"
	case StatusDegraded:
		return "·"
	case StatusFailing:
		return "✗"
	}
	return "?"
}

func chromePath() string {
	if v := os.Getenv("FENSTER_BROWSER"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	candidates := chromeCandidates()
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if which, _ := exec.LookPath("google-chrome"); which != "" {
		return which
	}
	if which, _ := exec.LookPath("chromium"); which != "" {
		return which
	}
	return ""
}

func chromeCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "linux":
		return []string{
			"/usr/bin/google-chrome", "/usr/bin/google-chrome-stable",
			"/usr/bin/chromium", "/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}
	case "windows":
		return []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		}
	}
	return nil
}

func chromeVersion(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func chromeAtLeast(version string, minMajor int) bool {
	// "Google Chrome 147.0.7727.102" -> 147
	for _, tok := range strings.Fields(version) {
		if n, err := strconv.Atoi(strings.SplitN(tok, ".", 2)[0]); err == nil {
			return n >= minMajor
		}
	}
	return false
}

func sw_versString() string {
	out, err := exec.Command("/usr/bin/sw_vers", "-productVersion").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func compareSemver(a, b string) int {
	for i := 0; i < 3; i++ {
		ai := semverPart(a, i)
		bi := semverPart(b, i)
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}

func semverPart(s string, i int) int {
	parts := strings.Split(s, ".")
	if i >= len(parts) {
		return 0
	}
	n, _ := strconv.Atoi(parts[i])
	return n
}

func profileDir() string {
	if v := os.Getenv("FENSTER_PROFILE"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".fenster"
	}
	return filepath.Join(home, ".fenster", "profile")
}

func freeBytesAt(dir string) (uint64, error) {
	// Make sure the parent exists so statfs works on a fresh setup.
	parent := dir
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(parent); err == nil {
			break
		}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
		parent = next
	}
	var s syscall.Statfs_t
	if err := syscall.Statfs(parent, &s); err != nil {
		return 0, err
	}
	return uint64(s.Bavail) * uint64(s.Bsize), nil
}
