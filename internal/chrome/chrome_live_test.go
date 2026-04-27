//go:build chrome

// Live Chrome tests — tagged so `go test ./...` skips them. To run:
//
//	go test -tags=chrome ./internal/chrome/... -v -timeout=120s
//
// These tests spawn real headless Chrome on the host. They're the
// proof that fenster's Chrome bridge actually works.
package chrome

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLive_Eval_OnePlusOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := Launch(ctx, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()
	var got int
	if err := b.Eval(ctx, "1+1", &got); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
}

func TestLive_PromptAPI_Availability(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := Launch(ctx, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()
	var status string
	js := `(async () => {
		if (typeof LanguageModel === 'undefined') return 'undefined';
		try { return await LanguageModel.availability(); } catch (e) { return 'error: ' + e.message; }
	})()`
	if err := b.Eval(ctx, js, &status); err != nil {
		t.Fatalf("Eval availability: %v", err)
	}
	t.Logf("LanguageModel.availability() = %q", status)
	// Acceptable: "available" | "downloadable" | "downloading" | "after-download" | "no"
	// "undefined" means the feature flag didn't enable it on this Chrome.
	if status == "" {
		t.Fatal("expected non-empty availability status")
	}
}

func TestLive_PromptAPI_HelloWorld(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	b, err := Launch(ctx, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer b.Close()

	js := `(async () => {
		if (typeof LanguageModel === 'undefined') return JSON.stringify({skip: 'no LanguageModel'});
		const avail = await LanguageModel.availability();
		if (avail === 'no' || avail === 'unavailable') return JSON.stringify({skip: 'avail=' + avail});
		const session = await LanguageModel.create();
		const out = await session.prompt('Reply with the single word: ready');
		return JSON.stringify({ok: out});
	})()`
	var raw string
	if err := b.Eval(ctx, js, &raw); err != nil {
		t.Fatalf("Eval prompt: %v", err)
	}
	t.Logf("Gemini Nano said: %s", raw)
	if strings.Contains(raw, `"skip"`) {
		t.Skipf("model not available on this Chrome: %s", raw)
	}
	if !strings.Contains(raw, `"ok"`) {
		t.Fatalf("unexpected response: %s", raw)
	}
}
