//go:build chrome

package chrome

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const canaryPath = "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"

func TestLive_Canary_Globals(t *testing.T) {
	if _, err := os.Stat(canaryPath); err != nil {
		t.Skip("Chrome Canary not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	profile, _ := os.UserHomeDir()
	profile = filepath.Join(profile, ".fenster", "canary-probe")
	b, err := Launch(ctx, LaunchOptions{
		BinaryPath: canaryPath,
		ProfileDir: profile,
		Headless:   true,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()
	js := `(() => JSON.stringify({
		ua: navigator.userAgent,
		LanguageModel: typeof LanguageModel,
		ai: typeof ai,
	}))()`
	var raw string
	if err := b.Eval(ctx, js, &raw); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	t.Logf("Canary headless probe: %s", raw)
	if !strings.Contains(raw, "function") && !strings.Contains(raw, "object") {
		t.Logf("Canary headless: API still not exposed; trying headed next test")
	}
}

func TestLive_Canary_HeadedAvailability(t *testing.T) {
	if _, err := os.Stat(canaryPath); err != nil {
		t.Skip("Chrome Canary not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	profile, _ := os.UserHomeDir()
	profile = filepath.Join(profile, ".fenster", "canary-probe")
	b, err := Launch(ctx, LaunchOptions{
		BinaryPath: canaryPath,
		ProfileDir: profile,
		Headless:   false,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()
	js := `(async () => {
		const out = {ua: navigator.userAgent, LanguageModel: typeof LanguageModel, ai: typeof ai};
		if (typeof LanguageModel !== 'undefined') {
			try { out.availability = await LanguageModel.availability(); } catch (e) { out.availability_err = e.message; }
		}
		return JSON.stringify(out);
	})()`
	var raw string
	if err := b.Eval(ctx, js, &raw); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	t.Logf("Canary headed probe: %s", raw)
}
