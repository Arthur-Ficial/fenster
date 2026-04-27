# fenster

**Run Gemini Nano local — on every computer.**

`fenster` is a tiny Go binary that turns Chrome's on-device **Gemini Nano**
model into an OpenAI-compatible HTTP server on `localhost:11434`. No cloud,
no API keys, no per-token costs — your prompts never leave the machine.

```bash
$ fenster --serve
fenster v0.0.1 — listening on http://127.0.0.1:11434/v1

$ curl -sX POST http://127.0.0.1:11434/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model":"gemini-nano","messages":[
          {"role":"user","content":"Capital of France? One word."}]}'
{"id":"chatcmpl-9d888a87ba6f","object":"chat.completion","created":1777293692,
 "model":"gemini-nano",
 "choices":[{"index":0,
   "message":{"content":"Paris\n","refusal":null,"role":"assistant"},
   "finish_reason":"stop","logprobs":null}],
 "usage":{"prompt_tokens":18,"completion_tokens":2,"total_tokens":20}}
```

---

## Why fenster

Apple shipped [`apfel`](https://github.com/Arthur-Ficial/apfel): a UNIX
front-end for the on-device Apple Intelligence model. Beautiful — but Apple
Intelligence runs on Macs only.

Most of the world isn't on a Mac.

Chrome ships Gemini Nano on **Windows, macOS, Linux, and ChromeOS**. fenster
wraps Chrome's Built-in AI Prompt API behind the same OpenAI-compatible
wire protocol apfel speaks. Drop-in replacement, three desktop OSes wider.

| Tool   | Engine                  | OS         | Wire format |
|--------|-------------------------|------------|-------------|
| apfel  | Apple FoundationModels  | macOS only | OpenAI      |
| **fenster** | **Chrome Gemini Nano** | **Windows · macOS · Linux · ChromeOS** | **OpenAI (apfel-identical)** |

If you have apfel's tests passing against apfel today, point them at fenster
and most of them stay green. That's the design contract.

---

## What you get

```
                ┌──────────────────────────────────────┐
                │  fenster (Go daemon)                 │
                │  ┌──────────┐    ┌────────────────┐  │
HTTP client ───▶│  │ HTTP/SSE │◀──▶│ Chrome (CDP)   │  │
(curl, IDE)     │  │ :11434   │    │ headless       │  │
                │  └──────────┘    └───────┬────────┘  │
                └──────────────────────────┼───────────┘
                                           │ spawn
                                           ▼
                ┌──────────────────────────────────────┐
                │  Headless Chrome Canary 149+         │
                │  ┌────────────────┐                  │
                │  │ LanguageModel  │                  │
                │  │  (Prompt API)  │                  │
                │  └───────┬────────┘                  │
                │          ▼                           │
                │   Gemini Nano (~3B)                  │
                └──────────────────────────────────────┘
```

Two modes are the product:

1. **UNIX tool** — `fenster "prompt"`, pipe-friendly, JSON-capable, exit codes.
2. **OpenAI HTTP server** — `fenster --serve`. Drop your client at it:
   ```python
   from openai import OpenAI
   client = OpenAI(api_key="sk-fenster", base_url="http://localhost:11434/v1")
   ```

A small **TUI chat** (`fenster --chat`) is a byproduct.

---

## Honest status

**Today (April 2026):** v0.0.1, **169/233 (72.5%) of the apfel integration
test suite passes against fenster** with the real Gemini Nano model
running headless. All Go unit tests are green and race-clean.

| | passing | gain |
|---|---:|---:|
| baseline (Echo backend) | 84 | — |
| security middleware + debug logs | 96 | +12 |
| real Gemini Nano via CDP | 105 | +9 |
| `-f`/`--file` + flat `-o json` | 128 | +23 |
| `--update`/`--release` + USAGE: + exit codes | 139 | +11 |
| man-page lints | 142 | +3 |
| footgun preflight + /health on loopback | 146 | +4 |
| `--token-auto` + `--no-origin-check` + WWW-Auth | 151 | +5 |
| ANSI under TTY + chat TUI + tool flatten | **169** | **+18** |

Realistic path to 100%:
- chat TUI completeness (arrow keys, JSON-mode, debug logs): ~17 more tests
- MCP host-side execute loop: ~10
- cli_e2e text-matching corners: ~22
- man-page bidirectional coverage: ~3
- openai_client tool calls + refusal: ~3
- the long tail: ~10

`docs/status.md` is the source of truth and gets updated each session.

---

## Architecture decisions you should know

These are empirically-proven, not aspirational:

1. **Chrome Canary 149+** is required. Stable Chrome doesn't expose
   `LanguageModel` even with `--enable-features=PromptAPIForGeminiNano`.
   `fenster doctor` will install/locate Canary for you.
2. **Headless mode works** — `--headless=new` + bootstrapped `Local State` +
   real `http://127.0.0.1` origin + `userGesture:true` CDP `Runtime.evaluate`
   trips the model-download gate. AppKit reports zero windows; you don't
   see Chrome on your desktop.
3. **One shared Chrome per machine** via `~/.fenster/run/chrome.json`
   lockfile — many `fenster --serve` instances all attach to the same
   browser. No dialog floods.
4. **Sentinel session reuse + pre-warm at startup** — first request after
   `fenster --serve` returns in <2s instead of paying the cold
   `LanguageModel.create()` tax. ~3× speedup on subsequent calls.
5. **`fenster --chat`'s Ctrl-C semantics** are ported from apfel: SIGINT
   while waiting at the prompt → clean exit 130 with terminal reset; SIGINT
   mid-stream → cancels the request, returns to the prompt.
6. **Tests are vendored once** from apfel into `Tests/integration/` and are
   now fenster's own. We don't re-vendor; tests evolve as fenster evolves.

Performance principles in [`docs/architecture.md`](docs/architecture.md).

---

## Try it

### One-time setup (~5 minutes, ~2.4 GB download)

```bash
# Install fenster (Go 1.22+)
go install github.com/Arthur-Ficial/fenster/cmd/fenster@latest

# Install Chrome Canary if you don't have it (macOS shown; Linux/Windows similar)
brew install --cask google-chrome@canary

# Verify your environment
fenster doctor
```

`fenster doctor` is honest about what's missing and tells you exactly what
to do. macOS 13+, Chrome 138+, GPU with >4 GB VRAM (or 16 GB RAM CPU
fallback), ≥22 GB free disk.

### One-shot UNIX tool

```bash
fenster "what is 2 + 2?"
fenster --json "summarize this file" -f main.go
echo "translate to french" | fenster
```

### OpenAI-compatible HTTP server

```bash
fenster --serve
# OR run with bearer auth + cors:
fenster --serve --token-auto --cors
```

Point any OpenAI client at `http://localhost:11434/v1`.

### Chat TUI

```bash
fenster --chat
# you› what's the capital of France?
#  ai› Paris.
# you› quit
# Goodbye.
```

---

## Build from source

```bash
git clone https://github.com/Arthur-Ficial/fenster
cd fenster
make build              # release binary to bin/fenster
make test-fast          # Go unit + non-model integration (~30s)
make test               # Full apfel test suite (~5 min, real Gemini Nano)
```

Modern Go (1.22+), stdlib-first. Direct deps: `cobra`, `chromedp`, `term`.
No third-party HTTP router; `net/http` 1.22 pattern routing is enough.

---

## What's open / contributing

Every concrete remaining task ships as a GitHub issue. See
https://github.com/Arthur-Ficial/fenster/issues — issues with the `up-for-grabs`
or `area:cli` / `area:server` / `area:chrome` labels are well-scoped places
to start.

The big ones:
- **MCP host-side execute loop** (`area:mcp`) — when fenster is launched
  with `--mcp <path-or-url>`, intercept tool calls in the model output,
  execute against the MCP server, re-prompt the model with the result.
- **Chat TUI completeness** (`area:cli`) — arrow-key history, multi-line
  paste, JSON-mode token streaming, debug-mode log split.
- **Real streaming SSE from `LanguageModel.promptStreaming()`** (`area:chrome`) —
  currently fenster awaits the full response then chunks on word boundaries.
  Wiring `promptStreaming()` saves time-to-first-byte.

---

## Sister project

`apfel` is fenster's macOS-Apple-Intelligence twin —
[github.com/Arthur-Ficial/apfel](https://github.com/Arthur-Ficial/apfel).
fenster's wire format is byte-for-byte compatible: clients written for
apfel work against fenster too.

---

## License

MIT. See `LICENSE`.

---

*"The free AI already in your browser, served as if it were OpenAI."*
