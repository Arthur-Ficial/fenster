//go:build chrome

package chrome

import (
	"context"
	"testing"
	"time"
)

// TestLive_HeadedAvailability tries headed Chrome (no --headless) to see if
// the Prompt API is actually exposed in real Chrome (vs HeadlessChrome which
// per empirical probe does NOT expose it on Chrome 147).
func TestLive_HeadedAvailability(t *testing.T) {
	if testing.Short() {
		t.Skip("headed launch is interactive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	b, err := Launch(ctx, LaunchOptions{Headless: false})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()

	js := `(() => {
		return JSON.stringify({
			ua: navigator.userAgent,
			has_LanguageModel: (typeof LanguageModel),
			has_ai: (typeof ai),
			has_translation: (typeof translation),
		});
	})()`
	var raw string
	if err := b.Eval(ctx, js, &raw); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	t.Logf("headed probe: %s", raw)
}
