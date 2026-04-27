// Package manifest writes Chrome Native Messaging manifests for fenster's
// host binary. One manifest per Chromium-derived browser the user has
// installed (Chrome, Chromium, Edge, Brave, Opera).
//
// The manifest tells Chrome:
//   - which native binary to spawn when an extension calls connectNative
//   - which extension IDs are allowed to talk to it (allowed_origins)
//
// Per-OS install paths (HOME-rooted on darwin/linux; registry on Windows).
// DRY: a single Manifest struct serializes to all of them; PathForBrowser
// resolves the per-OS+per-browser destination.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// HostName is the canonical NM host identifier. Extensions reference this
// in their `nativeMessaging` calls.
const HostName = "com.fullstackoptimization.fenster"

// Browser is one of the Chromium-derived browsers we support.
type Browser string

// Known browser identifiers.
const (
	BrowserChrome   Browser = "chrome"
	BrowserChromium Browser = "chromium"
	BrowserEdge     Browser = "edge"
	BrowserBrave    Browser = "brave"
)

// AllBrowsers returns every browser we know how to install for.
func AllBrowsers() []Browser {
	return []Browser{BrowserChrome, BrowserChromium, BrowserEdge, BrowserBrave}
}

// Manifest is the JSON shape Chrome reads.
type Manifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

// Build returns a manifest for fenster.
func Build(binaryPath, extensionID string) Manifest {
	origin := fmt.Sprintf("chrome-extension://%s/", extensionID)
	return Manifest{
		Name:           HostName,
		Description:    "fenster: OpenAI-compatible bridge to Chrome's Gemini Nano (Prompt API)",
		Path:           binaryPath,
		Type:           "stdio",
		AllowedOrigins: []string{origin},
	}
}

// PathForBrowser returns the per-OS NM directory path for a given browser.
// The returned path is the JSON file location (NOT just the directory).
func PathForBrowser(b Browser) string {
	dir := nmDirForBrowser(b)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, HostName+".json")
}

func nmDirForBrowser(b Browser) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		switch b {
		case BrowserChrome:
			return filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts")
		case BrowserChromium:
			return filepath.Join(home, "Library", "Application Support", "Chromium", "NativeMessagingHosts")
		case BrowserEdge:
			return filepath.Join(home, "Library", "Application Support", "Microsoft Edge", "NativeMessagingHosts")
		case BrowserBrave:
			return filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser", "NativeMessagingHosts")
		}
	case "linux":
		switch b {
		case BrowserChrome:
			return filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts")
		case BrowserChromium:
			return filepath.Join(home, ".config", "chromium", "NativeMessagingHosts")
		case BrowserEdge:
			return filepath.Join(home, ".config", "microsoft-edge", "NativeMessagingHosts")
		case BrowserBrave:
			return filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "NativeMessagingHosts")
		}
	case "windows":
		// Windows NM uses registry; we still write a JSON beside the binary
		// and the registry write happens elsewhere. Use AppData\Local\fenster.
		appdata := os.Getenv("LOCALAPPDATA")
		if appdata == "" {
			appdata = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(appdata, "fenster", "manifests", string(b))
	}
	return ""
}

// Install writes the manifest for `b`. Returns the absolute path written.
func Install(b Browser, binaryPath, extensionID string) (string, error) {
	dst := PathForBrowser(b)
	if dst == "" {
		return "", fmt.Errorf("manifest: no install path for browser %q on %s", b, runtime.GOOS)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("manifest: mkdir: %w", err)
	}
	m := Build(binaryPath, extensionID)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", fmt.Errorf("manifest: write: %w", err)
	}
	return dst, nil
}

// Uninstall removes the manifest for `b`. No-op if not present.
func Uninstall(b Browser) error {
	dst := PathForBrowser(b)
	if dst == "" {
		return nil
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// InstallResult is one entry of InstallAll's report.
type InstallResult struct {
	Browser Browser
	Path    string
	Err     error
}

// InstallAll installs the manifest for every supported browser. Errors per
// browser don't stop the loop — they're reported per-result so the caller
// can show a per-browser status.
func InstallAll(binaryPath, extensionID string) ([]InstallResult, error) {
	var out []InstallResult
	for _, b := range AllBrowsers() {
		path, err := Install(b, binaryPath, extensionID)
		out = append(out, InstallResult{Browser: b, Path: path, Err: err})
	}
	return out, nil
}
