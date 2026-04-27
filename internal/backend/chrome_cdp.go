package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/Arthur-Ficial/fenster/internal/core/tokens"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

// ChromeCDPBackend talks to Chrome's Built-in LanguageModel API directly via
// CDP (no extension, no Native Messaging). The supervisor spawns Canary
// headed with a bootstrapped profile (see internal/chrome) and points it
// at fenster's own HTTP server (which serves a tiny page at /). Chat
// requests are evaluated as JavaScript in that page's context.
//
// Why this works empirically (Chrome 147+):
//   - HeadlessChrome does NOT expose LanguageModel; we run headed.
//   - about:blank does NOT expose LanguageModel; we serve a real http://
//     page at fenster's GET / route.
//   - Stable Chrome 147 does not register the model component; we use Canary.
//   - The model download requires a user gesture; we synthesize a click.
type ChromeCDPBackend struct {
	browserCtx context.Context
	targetURL  string
	mu         sync.Mutex
	navigated  bool
	ready      bool
}

// ensureNavigated lazily navigates the controlled tab to targetURL once.
func (b *ChromeCDPBackend) ensureNavigated() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.navigated || b.targetURL == "" {
		return nil
	}
	// Retry: the supervisor's HTTP server may still be coming up.
	var lastErr error
	for i := 0; i < 20; i++ {
		err := chromedp.Run(b.browserCtx, chromedp.Navigate(b.targetURL))
		if err == nil {
			b.navigated = true
			return nil
		}
		lastErr = err
		select {
		case <-b.browserCtx.Done():
			return b.browserCtx.Err()
		default:
		}
	}
	return fmt.Errorf("chrome cdp: navigate %s: %w", b.targetURL, lastErr)
}

// NewChromeCDPBackend takes an already-prepared browser context and a
// target URL the tab should be navigated to (must be a real http:// origin
// — Built-in AI APIs are not exposed on about:blank). Navigate is LAZY:
// it happens on the first Chat/Health call, not in the constructor, so the
// supervisor's HTTP server has time to bind before Chrome tries to load it.
func NewChromeCDPBackend(browserCtx context.Context, targetURL string) (*ChromeCDPBackend, error) {
	return &ChromeCDPBackend{browserCtx: browserCtx, targetURL: targetURL}, nil
}

// initOnce ensures LanguageModel is "available". When the model is in a
// "downloadable"/"downloading" state, fenster triggers the download via a
// synthetic user-gesture click + LanguageModel.create(), then polls until
// the model is ready. The first call may take ~5 minutes on a fresh
// profile (Component Updater pulls ~2.4 GB).
func (b *ChromeCDPBackend) initOnce(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ready {
		return nil
	}
	// Wire a button into the page that the synthetic click will fire from.
	// LanguageModel.create() inside its handler passes Chrome's user-gesture
	// gate and triggers the download.
	if err := chromedp.Run(b.browserCtx, chromedp.Evaluate(`(async()=>{
		if (typeof LanguageModel === 'undefined') return 'no-api';
		const avail = await LanguageModel.availability();
		if (avail === 'available') return 'available';
		if (avail === 'unavailable' || avail === 'no') return 'unavailable';
		// Wire the download trigger.
		if (!document.getElementById('__fenster_dl')) {
			const b = document.createElement('button');
			b.id = '__fenster_dl';
			b.textContent = 'fenster-init';
			document.body.appendChild(b);
			window.__fensterCreated = false;
			b.addEventListener('click', async () => {
				try {
					const session = await LanguageModel.create({
						monitor(m){m.addEventListener('downloadprogress', e => { window.__fensterProgress = e.loaded; });},
					});
					try { session.destroy?.(); } catch(_) {}
					window.__fensterCreated = true;
				} catch (e) {
					window.__fensterCreateErr = String(e);
				}
			});
		}
		return 'wired';
	})()`, nil, withAwait)); err != nil {
		return err
	}
	// Synthesize a trusted click via CDP.
	if err := chromedp.Run(b.browserCtx,
		chromedp.MouseClickXY(50, 50),
	); err != nil {
		return fmt.Errorf("chrome cdp: synth click: %w", err)
	}
	// Poll availability for up to 15 minutes.
	deadline := time.Now().Add(15 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return errors.New("chrome cdp: timed out waiting for model download")
		}
		var raw string
		if err := chromedp.Run(b.browserCtx, chromedp.Evaluate(`(async()=>JSON.stringify({
			avail: await LanguageModel.availability(),
			progress: window.__fensterProgress || 0,
			err: window.__fensterCreateErr || ''
		}))()`, &raw, withAwait)); err != nil {
			return err
		}
		if strings.Contains(raw, `"available"`) {
			b.ready = true
			return nil
		}
		if strings.Contains(raw, `"unavailable"`) || strings.Contains(raw, `"no"`) {
			return errors.New("chrome cdp: model unavailable on this device: " + raw)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func withAwait(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
	return p.WithAwaitPromise(true)
}

// Health reports availability.
func (b *ChromeCDPBackend) Health(ctx context.Context) (Health, error) {
	if err := b.ensureNavigated(); err != nil {
		return Health{Available: false, Detail: err.Error()}, nil
	}
	var raw string
	err := chromedp.Run(b.browserCtx,
		chromedp.Evaluate(`(async()=>{
			if (typeof LanguageModel === 'undefined') return JSON.stringify({avail:'no-api'});
			return JSON.stringify({avail: await LanguageModel.availability()});
		})()`, &raw, withAwait),
	)
	if err != nil {
		return Health{Available: false, Detail: "cdp eval failed: " + err.Error()}, nil
	}
	var probe struct{ Avail string }
	_ = json.Unmarshal([]byte(raw), &probe)
	return Health{
		Available:          probe.Avail == "available",
		Detail:             "Gemini Nano availability=" + probe.Avail,
		ContextWindow:      wire.ContextWindow,
		SupportedLanguages: wire.SupportedLanguagesFallback(),
	}, nil
}

// Chat runs a prompt and returns the full result.
func (b *ChromeCDPBackend) Chat(ctx context.Context, req *wire.ChatCompletionRequest) (Result, error) {
	if err := b.ensureNavigated(); err != nil {
		return Result{}, err
	}
	if err := b.initOnce(ctx); err != nil {
		return Result{}, err
	}
	system, last, history := splitMessages(req.Messages)
	js := buildPromptJS(system, history, last, false)
	var raw string
	if err := chromedp.Run(b.browserCtx, chromedp.Evaluate(js, &raw, withAwait)); err != nil {
		return Result{}, err
	}
	var probe struct {
		Out string `json:"out"`
		Err string `json:"err"`
	}
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return Result{}, fmt.Errorf("chrome cdp: bad response %q: %w", raw, err)
	}
	if probe.Err != "" {
		return Result{}, fmt.Errorf("chrome cdp: %s", probe.Err)
	}
	out := probe.Out
	// JSON mode: strip markdown code fences when the client requested one.
	// Apfel does the same host-side; the model often wraps JSON in ```json … ```.
	if req.ResponseFormat != nil && req.ResponseFormat.IsJSONObject() {
		out = stripJSONFence(out)
	}
	return Result{
		Content:      out,
		FinishReason: wire.FinishStop,
		Usage:        tokens.Usage{Prompt: tokens.Estimate(joinForUsage(req.Messages)), Completion: tokens.Estimate(out)},
	}, nil
}

// ChatStream streams chunks. Implemented as a single full call broken into
// chunks at word boundaries — the Prompt API has promptStreaming() which we
// could use, but cross-CDP streaming requires more wiring; this is the
// pragmatic v1.
func (b *ChromeCDPBackend) ChatStream(ctx context.Context, req *wire.ChatCompletionRequest) (<-chan Chunk, error) {
	out := make(chan Chunk, 8)
	go func() {
		defer close(out)
		res, err := b.Chat(ctx, req)
		if err != nil {
			out <- Chunk{Err: err}
			return
		}
		// Word-boundary chunks for stream-ish output.
		parts := strings.SplitAfter(res.Content, " ")
		for _, p := range parts {
			if p == "" {
				continue
			}
			out <- Chunk{ContentDelta: p}
		}
		usage := res.Usage
		out <- Chunk{FinishReason: wire.FinishStop, Usage: &usage}
	}()
	return out, nil
}

// Close is a no-op (Chrome lifecycle is owned by the supervisor).
func (b *ChromeCDPBackend) Close() error { return nil }

// ----- helpers -----

func splitMessages(msgs []wire.Message) (system, last string, history []map[string]string) {
	for _, m := range msgs {
		c := m.Content.AsString()
		switch m.Role {
		case "system":
			if system != "" {
				system += "\n"
			}
			system += c
		case "user", "assistant":
			history = append(history, map[string]string{"role": m.Role, "content": c})
		}
	}
	if len(history) > 0 && history[len(history)-1]["role"] == "user" {
		last = history[len(history)-1]["content"]
		history = history[:len(history)-1]
	}
	return
}

func joinForUsage(msgs []wire.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content.AsString())
		b.WriteString("\n")
	}
	return b.String()
}

// buildPromptJS returns a single async-IIFE that creates a session, runs
// the prompt, and returns JSON {out:string,err?:string}.
func buildPromptJS(system string, history []map[string]string, lastUser string, _ bool) string {
	opts := map[string]any{}
	if system != "" {
		opts["systemPrompt"] = system
	}
	if len(history) > 0 {
		opts["initialPrompts"] = history
	}
	optsJSON, _ := json.Marshal(opts)
	prompt, _ := json.Marshal(lastUser)
	return fmt.Sprintf(`(async () => {
		try {
			const opts = %s;
			const session = await LanguageModel.create(opts);
			const out = await session.prompt(%s);
			session.destroy?.();
			return JSON.stringify({out: String(out)});
		} catch (e) {
			return JSON.stringify({err: String(e)});
		}
	})()`, string(optsJSON), string(prompt))
}

// Compile-time interface check.
var _ Backend = (*ChromeCDPBackend)(nil)
