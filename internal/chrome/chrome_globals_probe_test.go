//go:build chrome

package chrome

import (
	"context"
	"testing"
	"time"
)

// TestLive_ProbeWindowGlobals lists which AI-ish globals exist on window.
func TestLive_ProbeWindowGlobals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := Launch(ctx, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()
	js := `(() => {
		const globals = ['LanguageModel','ai','translation','Translator','LanguageDetector','Summarizer','Writer','Rewriter'];
		const out = {};
		for (const g of globals) out[g] = (typeof self[g]) + (self[g] ? ' present' : '');
		// Also dump all top-level keys whose name matches /ai|model|translate|prompt|nano|gemini/i
		const matching = [];
		for (const k of Object.getOwnPropertyNames(self)) {
			if (/ai|model|translate|prompt|nano|gemini|language/i.test(k)) matching.push(k);
		}
		out._matching_keys = matching;
		out._chrome = (typeof chrome !== 'undefined') ? Object.keys(chrome) : null;
		return JSON.stringify(out);
	})()`
	var raw string
	if err := b.Eval(ctx, js, &raw); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	t.Logf("globals probe: %s", raw)
}

// TestLive_ProbeUserAgent dumps the UA and Chrome version reported in JS.
func TestLive_ProbeUserAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	b, err := Launch(ctx, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()
	var ua string
	if err := b.Eval(ctx, `navigator.userAgent`, &ua); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	t.Logf("UA: %s", ua)
}
