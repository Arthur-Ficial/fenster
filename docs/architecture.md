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

## TODO (M1+)

- Cross-process protocol detail: how `--serve` host talks to `--nm-host` child (Unix socket at `~/.fenster/run/bridge.sock`)
- Session-pool eviction policy + memory budget
- Restart backoff thresholds
- Extension manifest version pinning
