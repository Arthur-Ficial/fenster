//go:build chrome

package chrome

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestLive_TriggerComponentUpdate visits chrome://components/ and tries to
// force-update the Optimization Guide On Device Model. This is the
// documented manual step every Built-in AI tutorial omits.
func TestLive_TriggerComponentUpdate(t *testing.T) {
	const canary = "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"
	if _, err := os.Stat(canary); err != nil {
		t.Skip("Canary not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	profile, _ := os.UserHomeDir()
	profile = filepath.Join(profile, ".fenster", "canary-probe")
	b, err := Launch(ctx, LaunchOptions{
		BinaryPath: canary,
		ProfileDir: profile,
		Headless:   false,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()

	// Navigate to chrome://components/.
	var componentsHTML string
	err = chromedp.Run(b.browserCtx,
		chromedp.Navigate("chrome://components/"),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`document.body.innerText`, &componentsHTML),
	)
	if err != nil {
		t.Fatalf("nav components: %v", err)
	}
	t.Logf("chrome://components/ first 600 chars: %s", componentsHTML[:min(600, len(componentsHTML))])
	if strings.Contains(strings.ToLower(componentsHTML), "optimization guide on device") {
		t.Log("✓ on-device model component is listed; need to click Check-for-update")
	} else {
		t.Log("✗ on-device model component NOT listed — Chrome doesn't know about it on this profile")
	}
}
