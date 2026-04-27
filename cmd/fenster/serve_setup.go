package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Arthur-Ficial/fenster/internal/chrome"
	"github.com/Arthur-Ficial/fenster/internal/extension"
	"github.com/Arthur-Ficial/fenster/internal/manifest"
)

// ensureExtensionAndManifest extracts the bundled extension on disk (if not
// already present) and writes the NM manifest pointing at this binary.
// The extension ID is computed deterministically from the install path so
// the manifest's allowed_origins matches what Chrome will assign.
func ensureExtensionAndManifest() (extDir, extID string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	extDir = filepath.Join(home, ".fenster", "extension")
	if err := writeExtensionFromEmbed(extDir); err != nil {
		return "", "", fmt.Errorf("ensure extension: %w", err)
	}
	extID = extension.PathDerivedID(extDir)
	binary, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	// Write the manifest for every supported browser. Failures per browser
	// are non-fatal — most users only have one Chromium-derived browser.
	results, _ := manifest.InstallAll(binary, extID)
	wroteAny := false
	for _, r := range results {
		if r.Err == nil {
			wroteAny = true
		}
	}
	if !wroteAny {
		return "", "", fmt.Errorf("ensure extension: no NM manifest could be written")
	}
	return extDir, extID, nil
}

// autoLaunchChrome spawns a controlled Chrome with our extension preloaded.
// The user doesn't need to click anything in chrome://extensions/. Returns a
// browser handle the supervisor can close on shutdown.
func autoLaunchChrome(ctx context.Context, extDir string, debug bool) (*chrome.Browser, error) {
	opts := chrome.LaunchOptions{
		Headless: !debug, // headed when --debug so user can see Chrome
		ExtraFlags: []string{
			"--load-extension=" + extDir,
			"--disable-extensions-except=" + extDir,
		},
	}
	return chrome.Launch(ctx, opts)
}

// writeExtensionFromEmbed extracts the embedded assets to a directory.
// Idempotent: overwrites every file each call so updates ride along with
// fenster releases.
func writeExtensionFromEmbed(out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(extension.Assets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(extension.Assets, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(out, filepath.Base(path))
		return os.WriteFile(dst, body, 0o644)
	})
}
