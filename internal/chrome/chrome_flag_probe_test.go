//go:build chrome

package chrome

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestLive_FlagProbe lists the actual flag names this Chrome ships for the
// Prompt API. Run once to learn the right --enable-features set for this
// specific Chrome version.
func TestLive_FlagProbe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	b, err := Launch(ctx, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()
	var dom string
	err = chromedp.Run(b.browserCtx,
		chromedp.Navigate("chrome://flags/"),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`document.documentElement.outerHTML`, &dom),
	)
	if err != nil {
		t.Fatalf("chrome://flags eval: %v", err)
	}
	for _, kw := range []string{"prompt-api", "gemini-nano", "language-model", "on-device-model", "optimization-guide", "AIPromptAPI", "BuiltinAI"} {
		if strings.Contains(strings.ToLower(dom), strings.ToLower(kw)) {
			// Find the surrounding ~120 chars for context
			low := strings.ToLower(dom)
			start := strings.Index(low, strings.ToLower(kw))
			end := start + len(kw)
			ctxStart := start - 80
			if ctxStart < 0 {
				ctxStart = 0
			}
			ctxEnd := end + 80
			if ctxEnd > len(dom) {
				ctxEnd = len(dom)
			}
			t.Logf("flag-keyword %q context: %q", kw, dom[ctxStart:ctxEnd])
		} else {
			t.Logf("flag-keyword %q NOT present", kw)
		}
	}
}

// TestLive_NavigateOnDeviceInternals shows what chrome://on-device-internals
// has in this Chrome.
func TestLive_NavigateOnDeviceInternals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	b, err := Launch(ctx, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()
	var body string
	err = chromedp.Run(b.browserCtx,
		chromedp.Navigate("chrome://on-device-internals/"),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`document.body.innerText`, &body),
	)
	if err != nil {
		t.Logf("on-device-internals navigate err: %v", err)
		return
	}
	if len(body) > 800 {
		body = body[:800] + "..."
	}
	t.Logf("on-device-internals body: %s", body)
}
