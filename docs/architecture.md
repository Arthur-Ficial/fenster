# fenster — architecture

```
                ┌──────────────────────────────────────┐
                │  fenster (Go daemon)                 │
                │  ┌──────────┐    ┌────────────────┐  │
HTTP client ───▶│  │ HTTP/SSE │◀──▶│ Native msg     │  │
(curl, IDE)     │  │ :11434   │    │ stdio framing  │  │
                │  └──────────┘    └───────┬────────┘  │
                │  ┌──────────┐    ┌───────▼────────┐  │
                │  │ doctor / │    │ Chrome process │  │
                │  │ logs cli │    │ supervisor     │  │
                │  └──────────┘    └───────┬────────┘  │
                └──────────────────────────┼───────────┘
                                           │ spawn
                                           ▼
                ┌──────────────────────────────────────┐
                │  Headless Chrome (--headless=new)    │
                │  --user-data-dir=~/.fenster/profile  │
                │  ┌────────────────┐                  │
                │  │ fenster ext.   │                  │
                │  │  service worker│                  │
                │  │   ↕ stdio      │                  │
                │  │   ↕ native msg │                  │
                │  │  LanguageModel │                  │
                │  └───────┬────────┘                  │
                │          ▼                           │
                │   Gemini Nano (GPU)                  │
                └──────────────────────────────────────┘
```

## Layers

| Layer | Package | Responsibility |
|---|---|---|
| CLI | `cmd/fenster` | cobra subcommands; one entrypoint binary |
| HTTP/SSE | `internal/server` | OpenAI-compatible endpoints; pure, `httptest`-friendly |
| Session pool | `internal/session` | LRU keyed by message-prefix hash; multi-turn cache |
| OpenAI types | `internal/openai` | request/response/stream/error types; matches apfel's wire format |
| Chrome supervisor | `internal/chrome` | locate chrome, spawn `--headless=new`, supervise, restart on crash |
| Native Messaging | `internal/nm` | 4-byte LE length prefix + UTF-8 JSON; bidirectional stdio |
| Extension (embedded) | `internal/extension` + `extension/` | MV3 service worker that bridges NM ↔ Prompt API |
| Manifest installer | `internal/manifest` | per-OS native messaging manifest registration (darwin/linux/windows) |
| MCP host | `internal/mcp` | host-side MCP client + auto-execution loop (mirrors apfel) |
| Doctor | `internal/doctor` | preconditions check (Chrome, GPU, disk, model) |
| Token counter | `internal/tokencount` | usage block; honest estimates with `_estimated: true` |

The dotted line between fenster's binary and the Chrome process is **Native Messaging** — the only legal way to talk to a Chrome extension's service worker from outside the browser.

## Lifecycle of one chat completion

1. CLI client POSTs OpenAI JSON to `localhost:11434/v1/chat/completions`.
2. `internal/server` validates the request; assigns a UUID for tracing.
3. `internal/session` looks up or creates a `LanguageModel` session keyed by `hash(messages[0..n-1])`.
4. The host frames `{id, type:"chat", payload:{...}}` and writes to `internal/nm`'s outbound channel.
5. `internal/nm` writes the framed message to stdout (which is connected to the Chrome extension by Chrome itself — Chrome spawned us).
6. The extension service worker reads from its NM port, calls `session.promptStreaming()`.
7. The service worker iterates the ReadableStream; each chunk goes back through `port.postMessage({id, type:"chunk", delta:"..."})`.
8. `internal/nm` reads frames from stdin, demuxes by `id`, pushes deltas into the per-request channel.
9. `internal/server` formats each delta as OpenAI SSE (`data: {...}\n\n`), flushes, until done. Final frame `{id, type:"done"}` triggers `data: [DONE]\n\n`.

## Why Chrome spawns us, not the other way around

Native Messaging hosts are launched **by Chrome**, never by the user or another process. fenster's lifecycle is therefore:

1. User runs `fenster --serve`.
2. fenster locates Chrome, writes the manifest pointing at its own absolute path (`os.Executable()`), embeds and unpacks the extension.
3. fenster spawns Chrome with `--headless=new --load-extension=...`.
4. The extension's service worker runs `chrome.runtime.connectNative("com.fullstackoptimization.fenster")` on startup.
5. Chrome launches a **second** copy of fenster's binary in NM-host mode (we detect this via stdin-is-pipe + a `--nm-host` flag).
6. The first copy supervises Chrome; the second copy is the NM bridge owned by Chrome.

This is the same trick apfel uses to keep the protocol sane: one process, two roles, selected by argv at startup.

## Performance — fast AI is a non-negotiable

fenster sits in front of a real on-device model. Every millisecond between
"client POST" and "first byte of response" matters. The architecture is built
around the principles below — adding code that violates any of them must come
with a benchmark showing the overall path got faster.

### 1. Sentinel session reuse (`window.__fensterSentinel`)

`LanguageModel.create()` is the cold-init path inside Chrome — it allocates
GPU resources for the model. Empirically ~1-2 seconds. fenster's
`internal/backend/chrome_cdp.go` keeps **one** sentinel session alive in the
controlled tab and reuses it across requests:

- One-shot prompts (no `system`, no history): use the sentinel directly,
  skip `create()` entirely. **Single-digit-millisecond CDP overhead** before
  the model starts generating.
- Multi-turn / system-prompt requests: clone the sentinel (`session.clone()`
  is a cheap history copy) or build a per-request session, but the sentinel
  remains warm for the next caller.

Measured: 23s → 8s for two consecutive curl prompts on the same backend.
Cumulative: pytest's ~190 model-hitting tests save ~2.5 minutes per run.

### 2. Single shared Chrome (`~/.fenster/run/chrome.json`)

Every `fenster --serve` reuses one Chrome instance. The first launches it;
subsequent fensters attach via the saved CDP URL. This:

- Eliminates 20+ Chrome cold-starts in a `pytest` session.
- Keeps Chrome's V8 + GPU compiles warm across processes.
- Avoids Chrome's profile-lock dialog flood.

Implementation: `internal/chrome/shared.go` — flock + atomic state file.

### 3. CDP webSocket kept alive

`chromedp.NewRemoteAllocator` reuses a single webSocket to Chrome's debug
port. We never tear down between prompts. `Eval` round-trip is
sub-millisecond on localhost.

### 4. Streaming SSE first (`/v1/chat/completions stream:true`)

For interactive clients (IDE assistants, chat UIs), we emit content tokens
as they arrive rather than waiting for the full response. Time-to-first-byte
is ~50-200 ms after the model produces its first token, vs whole-response
latency.

Implementation: `internal/server/chat_stream.go` — `http.Flusher` per chunk.

### 5. Pre-encoded NM framing path (current code)

The bridge between fenster supervisor and the NM-host child is byte-copy.
No JSON parse on the relay (fenster's host parses once at the supervisor).
4-byte LE length + UTF-8 — no allocations in the hot copy path.

### 6. Production checklist (deploy targets)

When fenster ships in a real workflow (IDE plugin, daily UNIX use, server):

- **Pre-warm the sentinel on startup**: `--serve` triggers a tiny "echo"
  prompt at boot so `__fensterSentinel` is ready before the first user
  request. (Follow-up ticket; the cold first-request cost is currently the
  user's first turn.)
- **Bind to `127.0.0.1` only** unless explicitly opened. Loopback is
  always the fastest route.
- **HTTP keep-alive on**: clients should reuse connections. fenster's
  stdlib `net/http.Server` does this by default.
- **No CORS preflight unless the client needs it**: each `OPTIONS` is one
  more round-trip. fenster's default is CORS-off.
- **Single bridge socket**: don't multiplex through multiple CDP tabs;
  the sentinel pattern keeps one tab hot.
- **Lower test timeouts in CI to match real model latency** (15-30s) so
  failures surface quickly instead of waiting on 60s hangs.

## TODO (M3+)

- Cross-process protocol detail: how `--serve` host talks to `--nm-host` child (Unix socket at `~/.fenster/run/bridge.sock`)
- Session-pool eviction policy + memory budget for cloned sessions
- Restart backoff thresholds for Chrome supervisor
- Extension manifest version pinning
- Pre-warm sentinel session on `--serve` startup
- Streaming bridge: deltas pulled from `session.promptStreaming()` ReadableStream and forwarded as SSE chunks (currently we await full result then chunk on word boundaries)
