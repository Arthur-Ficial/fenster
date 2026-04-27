//go:build chrome

package chrome

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLive_BootstrapLocalState writes enabled_labs_experiments to a fresh
// profile's Local State BEFORE launching Chrome. If this works, Chrome
// reads the persisted flag at startup and registers the on-device model
// component (and exposes LanguageModel after a download).
func TestLive_BootstrapLocalState(t *testing.T) {
	const canary = "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"
	if _, err := os.Stat(canary); err != nil {
		t.Skip("Canary not installed")
	}
	home, _ := os.UserHomeDir()
	profile := filepath.Join(home, ".fenster", "canary-bootstrap")
	_ = os.RemoveAll(profile)
	if err := os.MkdirAll(profile, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write Local State with enabled_labs_experiments BEFORE Chrome ever
	// touches the dir.
	state := map[string]any{
		"browser": map[string]any{
			"enabled_labs_experiments": []string{
				"prompt-api-for-gemini-nano@1",
				"optimization-guide-on-device-model@2",
				"prompt-api-for-gemini-nano-multimodal-input@1",
			},
		},
	}
	b, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(profile, "Local State"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote Local State at %s/Local State", profile)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	br, err := Launch(ctx, LaunchOptions{
		BinaryPath: canary,
		ProfileDir: profile,
		Headless:   true,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer br.Close()

	js := `(async () => {
		const out = {ua: navigator.userAgent, LanguageModel: typeof LanguageModel};
		if (typeof LanguageModel !== 'undefined') {
			try { out.availability = await LanguageModel.availability(); }
			catch (e) { out.err = String(e); }
		}
		return JSON.stringify(out);
	})()`
	var raw string
	if err := br.Eval(ctx, js, &raw); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	t.Logf("bootstrap probe: %s", raw)
}
