// Package chrome drives a headless Chrome instance via CDP (Chrome DevTools
// Protocol) and exposes the Prompt API → Gemini Nano via JavaScript evaluation.
//
// The architecture deliberately avoids Native Messaging here: we spawn Chrome
// with --enable-features=PromptAPIForGeminiNano,OptimizationGuideOnDeviceModel
// and an isolated --user-data-dir, navigate to a controlled origin
// (about:blank then a data: URL), and call window.LanguageModel from there.
// CDP gives us full bi-directional control without an extension or NM host.
//
// fenster/extension/ remains shipped for users who want to opt out of the
// feature flag and use the extension path; that's a separate Backend (M5).
package chrome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Browser is a running headless Chrome we can evaluate JS in.
type Browser struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	browserCtx  context.Context
	browserCancel context.CancelFunc
	closed      sync.Once
}

// LaunchOptions configures Launch.
type LaunchOptions struct {
	BinaryPath string // optional; auto-detected when empty
	ProfileDir string // optional; defaults to ~/.fenster/profile
	Headless   bool   // default true; pass false for `fenster debug`
	ExtraFlags []string
}

// Launch finds Chrome and spawns a controlled instance.
//
// Empirical Chrome 147+ findings (proven against this host):
//   - HeadlessChrome does NOT expose window.LanguageModel (the Built-in AI
//     APIs are gated for security/perf reasons in headless mode).
//   - Headed Chrome on Stable 147 also does not expose it on a fresh
//     profile, even with --enable-features=...
//   - Chrome Canary 149 + headed + Local State pre-bootstrapped with
//     enabled_labs_experiments + a real http://127.0.0.1 origin DOES
//     expose LanguageModel as a function.
//   - The on-device model component download requires a user gesture
//     (Chrome's security gate). fenster needs to either have the user
//     trigger it once, or bootstrap with a synthetic mouse click via CDP.
//
// Launch therefore:
//   1. Writes Local State with enabled_labs_experiments BEFORE Chrome runs
//   2. Defaults to HEADED unless the caller explicitly asks for headless
//      (which is not useful for actual inference)
//   3. Adds --remote-allow-origins=* so external CDP probes can connect
func Launch(ctx context.Context, opts LaunchOptions) (*Browser, error) {
	binary := opts.BinaryPath
	if binary == "" {
		binary = LocateBinary()
	}
	if binary == "" {
		return nil, errors.New("chrome: could not locate Chrome (set FENSTER_BROWSER or pass --browser)")
	}
	profile := opts.ProfileDir
	if profile == "" {
		profile = DefaultProfileDir()
	}
	if err := os.MkdirAll(profile, 0o755); err != nil {
		return nil, fmt.Errorf("chrome: profile dir: %w", err)
	}
	if err := bootstrapLocalState(profile); err != nil {
		return nil, fmt.Errorf("chrome: bootstrap Local State: %w", err)
	}
	// Singleton lock cleanup (prior Chrome may have left this behind).
	for _, name := range []string{"SingletonLock", "SingletonCookie", "SingletonSocket"} {
		_ = os.Remove(filepath.Join(profile, name))
	}

	flags := buildFlags(profile, opts.Headless, opts.ExtraFlags)
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		append(flags, chromedp.ExecPath(binary))...,
	)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(browserCtx); err != nil {
		browserCancel()
		allocCancel()
		return nil, fmt.Errorf("chrome: start: %w", err)
	}
	return &Browser{
		allocCtx: allocCtx, allocCancel: allocCancel,
		browserCtx: browserCtx, browserCancel: browserCancel,
	}, nil
}

// bootstrapLocalState writes the chrome://flags toggles fenster needs into
// Local State BEFORE Chrome reads it. Idempotent: preserves any other
// keys the file already had.
func bootstrapLocalState(profileDir string) error {
	path := filepath.Join(profileDir, "Local State")
	var state map[string]any
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &state)
	}
	if state == nil {
		state = map[string]any{}
	}
	browser, _ := state["browser"].(map[string]any)
	if browser == nil {
		browser = map[string]any{}
	}
	browser["enabled_labs_experiments"] = []string{
		"prompt-api-for-gemini-nano@1",
		"optimization-guide-on-device-model@2",
		"prompt-api-for-gemini-nano-multimodal-input@1",
	}
	state["browser"] = browser
	out, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func buildFlags(profile string, headless bool, extra []string) []chromedp.ExecAllocatorOption {
	var f []chromedp.ExecAllocatorOption
	// Feature set required for the Prompt API to expose window.LanguageModel.
	// PromptAPIForGeminiNano                       -> registers LanguageModel global
	// PromptAPIForGeminiNanoMultimodalInput        -> image/audio input
	// OptimizationGuideOnDeviceModel               -> on-device model component
	// OptimizationGuideOnDeviceModelBypassPerfRequirement -> allow on machines that
	//   don't pass Chrome's internal performance benchmark (most laptops fail it)
	// AIPromptAPI                                  -> newer-Chrome alias used in 147+
	features := strings.Join([]string{
		"PromptAPIForGeminiNano",
		"PromptAPIForGeminiNanoMultimodalInput",
		"OptimizationGuideOnDeviceModel",
		"OptimizationGuideOnDeviceModelBypassPerfRequirement",
		"AIPromptAPI",
		"AIPromptAPIForExtension",
		"AIRewriterAPI",
		"AISummarizationAPI",
	}, ",")
	f = append(f,
		chromedp.UserDataDir(profile),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("enable-features", features),
		chromedp.Flag("optimization-guide-on-device-model", "Enabled"),
		chromedp.Flag("remote-allow-origins", "*"),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("mute-audio", true),
	)
	// Headless intentionally omitted by default — the Built-in AI APIs
	// are not exposed in HeadlessChrome on Chrome 147+.
	if headless {
		f = append(f, chromedp.Flag("headless", "new"))
	}
	for _, raw := range extra {
		// "--key=value" or "--key"
		s := strings.TrimPrefix(raw, "--")
		if eq := strings.Index(s, "="); eq != -1 {
			f = append(f, chromedp.Flag(s[:eq], s[eq+1:]))
		} else {
			f = append(f, chromedp.Flag(s, true))
		}
	}
	return f
}

// Close shuts the browser down.
func (b *Browser) Close() error {
	b.closed.Do(func() {
		if b.browserCancel != nil {
			b.browserCancel()
		}
		if b.allocCancel != nil {
			b.allocCancel()
		}
	})
	return nil
}

// Eval evaluates a JS snippet in the current page (creating a blank page if
// needed) and unmarshals the result into res.
func (b *Browser) Eval(ctx context.Context, js string, res any) error {
	return chromedp.Run(b.browserCtx,
		chromedp.Navigate("about:blank"),
		chromedp.Evaluate(js, res, awaitPromise),
	)
}

// awaitPromise tells chromedp.Evaluate to await a returned Promise.
func awaitPromise(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
	return p.WithAwaitPromise(true)
}

// Attach connects to an already-running Chrome instance via its remote
// debugging URL (e.g. "http://127.0.0.1:9339"). This is the simplest path
// for users who already have Canary running with the right flags.
func Attach(ctx context.Context, debugURL string) (*Browser, error) {
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, debugURL)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(browserCtx); err != nil {
		browserCancel()
		allocCancel()
		return nil, fmt.Errorf("chrome: attach %s: %w", debugURL, err)
	}
	return &Browser{
		allocCtx: allocCtx, allocCancel: allocCancel,
		browserCtx: browserCtx, browserCancel: browserCancel,
	}, nil
}

// BrowserCtx exposes the underlying chromedp context so callers (e.g.
// internal/backend/chrome_cdp) can run their own evals.
func (b *Browser) BrowserCtx() context.Context { return b.browserCtx }

// LocateBinary returns the absolute path to a Chrome/Chromium binary,
// honouring FENSTER_BROWSER and falling back to OS-default install paths.
func LocateBinary() string {
	if v := os.Getenv("FENSTER_BROWSER"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	for _, p := range osCandidates() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// DefaultProfileDir returns the profile dir for the chosen Chrome binary.
// Canary and Stable have incompatible profile formats; using one profile
// for both produces "Something went wrong opening your profile" errors
// when Chrome detects a profile-version mismatch. fenster keeps a separate
// dir per binary identity.
func DefaultProfileDir() string {
	if v := os.Getenv("FENSTER_PROFILE"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".fenster"
	}
	binary := os.Getenv("FENSTER_BROWSER")
	if binary == "" {
		binary = LocateBinary()
	}
	subdir := "profile"
	switch {
	case strings.Contains(binary, "Canary"):
		subdir = "profile-canary"
	case strings.Contains(binary, "Chromium"):
		subdir = "profile-chromium"
	case strings.Contains(binary, "Brave"):
		subdir = "profile-brave"
	case strings.Contains(binary, "Edge"):
		subdir = "profile-edge"
	}
	return filepath.Join(home, ".fenster", subdir)
}

func osCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		// Canary first — Chrome Stable does not expose the Built-in AI
		// LanguageModel API on this Mac (verified empirically against
		// Stable 147 vs Canary 149); Canary does.
		return []string{
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "linux":
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
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

// Used to avoid unused import warnings.
var _ = strconv.Itoa
