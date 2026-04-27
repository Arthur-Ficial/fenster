# fenster

### Run Chrome's local Gemini Nano through a Go bridge.

[![Version 0.0.1](https://img.shields.io/badge/version-0.0.1-blue)](https://github.com/Arthur-Ficial/fenster)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![macOS / Linux / Windows](https://img.shields.io/badge/desktop-cross--platform-000000?logo=apple&logoColor=white)](https://github.com/Arthur-Ficial/fenster)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![100% On-Device](https://img.shields.io/badge/inference-100%25%20on--device-green)](https://developer.chrome.com/docs/ai/prompt-api)
[![Apfel-compat](https://img.shields.io/badge/wire--format-apfel--compatible-orange)](https://github.com/Arthur-Ficial/apfel)
[![#agentswelcome](https://img.shields.io/badge/%23agentswelcome-PRs%20welcome-0066cc?style=for-the-badge&labelColor=0d1117&logo=probot&logoColor=white)](#contributing)

Chrome ships a built-in LLM ([Gemini Nano](https://developer.chrome.com/docs/ai/prompt-api), about 3B parameters, GPU-accelerated). It is gated behind an Origin Trial and a `window.LanguageModel` JS API exposed to pages. `fenster` starts headless Chrome Canary, drives it over CDP, sets `userGesture:true` for the model-download gate, and exposes the result as a UNIX tool and an OpenAI-compatible HTTP server on `localhost:11434`. Inference stays on-device and does not require an API key.

| Mode | Command | What you get |
|------|---------|--------------|
| UNIX tool | `fenster "prompt"` / `echo "text" \| fenster` | Pipe-friendly answers, file attachments, JSON output, exit codes |
| OpenAI-compatible server | `fenster --serve` | Drop-in local `http://localhost:11434/v1` backend for OpenAI SDKs |

`fenster --chat` is an interactive REPL for testing prompts.

Cross-platform sister of [apfel](https://github.com/Arthur-Ficial/apfel) (macOS-only, Apple Intelligence). Same wire format. Clients written for one work against the other.

## How it works

```
                ┌─────────────────────────────────────────────────────────┐
                │  fenster (Go binary, single static executable)          │
                │  ┌──────────────────┐    ┌───────────────────────────┐  │
HTTP client ──> │  │ HTTP/SSE :11434  │    │ Chrome supervisor (CDP)   │  │
(curl, IDE,     │  │ stdlib net/http  │<──>│ chromedp + flock lockfile │  │
 openai SDK)    │  │ /v1/* + /health  │    │ ~/.fenster/run/chrome.json│  │
                │  └──────────────────┘    └────────────┬──────────────┘  │
CLI (UNIX) ───> │  ┌──────────────────┐                 │                  │
                │  │ oneshot / chat   │                 │ spawn (one shared)
                │  │ stdin / -f files │                 │ across N processes
                │  └──────────────────┘                 v                  │
                └─────────────────────────────────────────────────────────┘
                                                        │
                                                        v
                ┌─────────────────────────────────────────────────────────┐
                │  Headless Chrome Canary 149+ (--headless=new)           │
                │  ┌─────────────────────────────────────────────────┐    │
                │  │ Profile: ~/.fenster/profile-canary              │    │
                │  │   Local State pre-bootstrapped with             │    │
                │  │   enabled_labs_experiments to flip on Built-in  │    │
                │  │   AI APIs without --enable-features churn       │    │
                │  └─────────────────────────────────────────────────┘    │
                │  ┌─────────────────────────────────────────────────┐    │
                │  │ Page served from http://127.0.0.1:11434/        │    │
                │  │ (about:blank does NOT expose LanguageModel,     │    │
                │  │  must be a real http origin - Chrome's rule)    │    │
                │  └────────────────────────┬────────────────────────┘    │
                │                           v                             │
                │  ┌─────────────────────────────────────────────────┐    │
                │  │ window.LanguageModel  (Chrome's Prompt API)     │    │
                │  │   Runtime.evaluate { userGesture: true } over   │    │
                │  │   CDP synthesizes a user click, so              │    │
                │  │   LanguageModel.create() is allowed to download │    │
                │  │   the model and run prompts                     │    │
                │  └────────────────────────┬────────────────────────┘    │
                │                           v                             │
                │  ┌─────────────────────────────────────────────────┐    │
                │  │ Gemini Nano (~3B params, ~2.4 GB on disk)       │    │
                │  │   GPU inference (Metal / DirectML / Vulkan)     │    │
                │  │   16 GB RAM CPU fallback if no GPU              │    │
                │  └─────────────────────────────────────────────────┘    │
                └─────────────────────────────────────────────────────────┘

  Also shipped (alternative bridge, MV3-blessed path):
    extension/  ── Chrome MV3 service worker, nativeMessaging permission
    internal/nm ── 4-byte LE length prefix + UTF-8 JSON Native Messaging host
  Currently the CDP path is the default; the extension is wired and ready
  for cases where pure CDP is not enough (locked-down enterprise builds, etc.).
```

The Chrome that fenster spawns is invisible. AppKit reports zero windows. No Dock icon. `FENSTER_CHROME_HEADED=1` surfaces it for debugging. Many `fenster --serve` instances on the same machine attach to one shared Chrome via a flock lockfile - no dialog floods, no twenty Chrome icons.

Tech stack: **Go 1.22+ stdlib first** (`net/http` 1.22 patterns, `log/slog`, `embed.FS`, `context.Context` everywhere), **`chromedp`** for CDP, **`cobra`** for CLI, **`golang.org/x/term`** for TTY detection. No third-party HTTP router. No mocks of Chrome. Single static binary.

## Requirements & Install

Chrome Canary 149+ (Stable does not expose `LanguageModel` even with `--enable-features=PromptAPIForGeminiNano` - empirically tested), GPU with >4 GB VRAM (or 16 GB RAM CPU fallback), 22 GB free disk. Building from source needs Go 1.22+.

```bash
go install github.com/Arthur-Ficial/fenster/cmd/fenster@latest
brew install --cask google-chrome@canary    # the Built-in AI Origin Trial requires Canary today
fenster doctor                              # verify your environment, tells you exactly what is missing
```

`fenster doctor` reports what is missing and what to do about it. It checks Chrome channel, GPU, disk, profile bootstrap state, and whether the model is downloaded.

## Quick Start

### UNIX tool

Quote prompts with `!` in single quotes (zsh/bash history expansion): `fenster 'Hello, Chrome!'`.

```bash
# Single prompt
fenster "What is the capital of Austria?"

# Stream output
fenster --stream "Write a haiku about code"

# Pipe input
echo "Summarize: $(cat README.md)" | fenster

# Attach file content to prompt
fenster -f README.md "Summarize this project"

# Attach multiple files
fenster -f old.go -f new.go "What changed between these two files?"

# Combine files with piped input
git diff HEAD~1 | fenster -f CONVENTIONS.md "Review this diff against our conventions"

# JSON output for scripting
fenster -o json "Translate to German: hello" | jq .content

# System prompt
fenster -s "You are a pirate" "What is recursion?"

# Quiet mode for shell scripts
result=$(fenster -q "Capital of France? One word.")
```

### OpenAI-compatible server

```bash
fenster --serve                              # foreground; spawns headless Chrome
FENSTER_TOKEN=$(uuidgen) fenster --serve     # bearer-protected
fenster --serve --token-auto                 # auto-generate token, print to stderr
```

```bash
curl http://localhost:11434/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-nano","messages":[{"role":"user","content":"Hello"}]}'
```

```python
from openai import OpenAI
client = OpenAI(base_url="http://localhost:11434/v1", api_key="unused")
resp = client.chat.completions.create(
    model="gemini-nano",
    messages=[{"role": "user", "content": "What is 1+1?"}],
)
print(resp.choices[0].message.content)
```

`fenster --serve` shares one Chrome instance across every server you start. The first one launches Chrome (about 5 minutes on first ever boot to download the 2.4 GB Gemini Nano model). Subsequent starts attach to the same browser via the lockfile.

### Chat REPL

`fenster --chat` is a small REPL for testing prompts or MCP servers.

```bash
fenster --chat
fenster --chat -s "You are a helpful coding assistant"
fenster --chat --mcp ./mcp/calculator/server.py      # chat with MCP tools
fenster --chat --debug                                # debug output to stderr
```

Ctrl-C exits. Type `quit` or hit Ctrl-D to exit cleanly.

## Architecture (longer version)

The design priority order is **UNIX tool first, HTTP server second, chat third.** Everything else hangs off these.

### 1. UNIX tool path (`fenster "prompt"`, pipes, `-f`, `-o json`)

```
stdin / argv / -f files
        │
        v
cmd/fenster/main.go (cobra)
        │
        v
internal/oneshot  ── builds a single ChatCompletionRequest
        │
        v
internal/backend.Backend  ── interface: EchoBackend (no Chrome) | ChromeCDPBackend (real)
        │
        v
stdout (text)  /  stdout (JSON envelope)  /  stderr (errors)
exit code: 0 success, 1 generic, 2 invalid args, 3 doctor fail, 64 not implemented
```

Pure pipe behavior, no daemon required for one-shots. The Backend interface lets the same code path run against `EchoBackend` (deterministic, no Chrome - used for tests and `FENSTER_BACKEND=echo` smoke checks) or `ChromeCDPBackend` (real model). When `--serve` is on, the CLI process keeps Chrome supervised; when it is a one-shot, fenster connects to an existing shared Chrome via the lockfile or starts one and keeps it for the next call.

### 2. HTTP server path (`fenster --serve`)

```
HTTP request (curl, openai-python, IDE)
        │
        v
internal/server (stdlib net/http 1.22 patterns, no router dep)
  ├─ /health                    ── liveness (loopback by default; --public-health to flip)
  ├─ /v1/models                 ── { "gemini-nano": ... }
  ├─ /v1/chat/completions       ── stream + non-stream, OpenAI envelope
  ├─ /v1/completions            ── 501
  ├─ /v1/embeddings             ── 501
  └─ middleware: bearer auth, origin check, CORS, request validation
        │
        v
internal/backend.ChromeCDPBackend
  ├─ initOnce()  ── Runtime.evaluate {userGesture:true} → LanguageModel.create()
  ├─ sentinel session in window.__fensterSentinel  ── created once, .clone() per request
  ├─ PreWarm()  ── pays the cold-start tax in the background at server boot
  └─ splitMessages()  ── flattens OpenAI tool_calls/tool messages into text history
        │
        v
chromedp (CDP client)
        │
        v
Headless Chrome Canary, page = http://127.0.0.1:11434/  (must be a real http origin)
        │
        v
window.LanguageModel.promptStreaming(history) → Gemini Nano on GPU
        │
        v
SSE chunks back up the stack, OpenAI-shaped, byte-for-byte apfel-compatible
```

Single shared Chrome per machine: `~/.fenster/run/chrome.json` holds the PID and CDP URL, protected by `flock(2)`. Every `fenster --serve` instance attaches to the same browser. First one launches it, last one to leave optionally cleans up. Sentinel session reuse drops first-byte latency from ~23s to ~2s on warm starts.

### 3. The Chrome extension bridge (alternative path, shipped)

fenster also ships a real MV3 Chrome extension and a Native Messaging host. The extension's only job is to call `LanguageModel.create()` from inside Chrome's extension context (where it is also exposed) and stream chunks back to a Native Messaging host process over Chrome's stdio framing protocol (4-byte little-endian length prefix + UTF-8 JSON).

```
extension/service-worker.js  ── connectNative("com.fullstackoptimization.fenster")
        │
        │   Chrome stdio (4-byte LE prefix + JSON)
        v
internal/nm    ── Native Messaging framing
internal/bridge ── Unix-socket multiplex to fenster supervisor
        │
        v
internal/backend.ChromeBackend (alternative to ChromeCDPBackend)
```

This path is wired and tested but is not the default. The CDP path has fewer deployment requirements: no per-OS Native Messaging manifest install step, no extension ID drift, and no Chrome Web Store packaging. The extension remains available for environments where `Runtime.evaluate {userGesture:true}` is blocked but an installed extension is permitted.

## Pros and cons of this architecture

### Pros

- **Free, on-device, no API key.** Gemini Nano runs locally on hardware that already shipped with Chrome. No tokens metered, no rate limits, no telemetry to a vendor.
- **OpenAI wire-format compatible.** Drop-in for `openai-python`, `openai-node`, LangChain, anything that takes a `base_url`. Same envelope as apfel; clients written for one work against the other unmodified.
- **Single static Go binary.** `go install` and the binary runs. No Python venv, no Node, no Docker. Cross-compiles to darwin-arm64, darwin-amd64, linux-amd64, linux-arm64, windows-amd64.
- **Invisible by default.** `--headless=new` plus no AppKit window plus no Dock icon. `FENSTER_CHROME_HEADED=1` surfaces Chrome for debugging.
- **One shared Chrome per machine.** Lockfile-based supervisor means N `fenster --serve` processes attach to one Chrome; first one launches, others reuse.
- **Fast warm path.** Sentinel session reuse plus pre-warm at server boot brings first-token latency from ~23s (cold `LanguageModel.create()`) to ~2s on subsequent prompts.
- **GPU acceleration via Chrome.** Metal on macOS, DirectML on Windows, Vulkan on Linux. No CUDA, no ROCm, no per-OS driver setup.
- **Explicit limits.** `fenster doctor` reports environment state. 501 responses for `/v1/embeddings` and legacy completions, not silent stubs.
- **Stdlib-first Go.** ~10k LOC. No third-party HTTP router. No testify. No DI framework. Direct deps: `cobra`, `chromedp`, `golang.org/x/term`.

### Cons

- **Chrome Canary 149+ required.** Stable Chrome does not expose `LanguageModel` even with `--enable-features=PromptAPIForGeminiNano` (empirically tested). The Built-in AI APIs are on a public Origin Trial; the gate can change between Chrome versions.
- **First boot is heavy.** ~2.4 GB Gemini Nano model download on first launch. Minutes on a fast connection. The model lives inside Chrome's profile; nothing fenster can do to skip it.
- **GPU floor.** ~4 GB VRAM minimum. CPU fallback needs ~16 GB RAM and is slow. Realistic target is ChromeOS Plus, Windows 10/11, macOS 13+, modern Linux desktops.
- **~3B parameter model.** Gemini Nano is small. Reasoning, math, and long-context tasks are not its strength. Output quality is well below GPT-4-class models.
- **Tool calling is shimmed.** Chrome's Prompt API does not expose OpenAI-shape tool calls. fenster maps them to `responseConstraint` JSON-schema constraints and parses host-side.
- **No embeddings, no fine-tune, no `logit_bias`.** What Chrome exposes is the entire surface.
- **Origin-trial fragility.** Future Chrome versions may change the API surface or pull the gate. fenster tracks upstream as it shifts.
- **CDP `userGesture:true` is the trigger mechanism, not a guarantee.** Chrome accepts a synthesized user gesture from CDP for the download trigger today. The MV3 extension bridge is the documented fallback if that path closes.
- **Multi-step agentic loops drift.** For agent work that needs stronger reasoning, a hosted frontier model is a better fit. fenster's fit is local privacy-sensitive single-turn Q&A and structured-output tasks.
- **A whole browser to run a 3B model.** Headless Chrome can crash, hang, or fail to download the model. fenster supervises and restarts, but the process footprint is browser-sized.

## Status (today, April 2026)

v0.0.1, **172 of 233 apfel integration tests pass** against fenster with the real Gemini Nano model running headless. All Go unit tests are green and race-clean.

| Wave | passing | gain |
|------|--------:|-----:|
| baseline (Echo backend, no model) | 84 | |
| security middleware + debug logs | 96 | +12 |
| real Gemini Nano via CDP | 105 | +9 |
| `-f`/`--file` + flat `-o json` | 128 | +23 |
| `--update`/`--release` + USAGE: + exit codes | 139 | +11 |
| man-page lints | 142 | +3 |
| footgun preflight + /health on loopback | 146 | +4 |
| `--token-auto` + `--no-origin-check` + WWW-Auth | 151 | +5 |
| ANSI under TTY + chat TUI + tool flatten | 169 | +18 |
| chat ` ai› ` + tool messages + --stream/--json | **172** | +3 |

Path to 100% lives in [docs/status.md](docs/status.md). Every remaining task is a [GitHub issue](https://github.com/Arthur-Ficial/fenster/issues).

## Implementation notes

What is actually in the code, file by file:

1. **Profile Local State bootstrap** ([internal/chrome/chrome.go](internal/chrome/chrome.go)). fenster writes Chrome's `Local State` JSON with `enabled_labs_experiments` set before Chrome launches. On boot Chrome reads it and the Built-in AI flags are enabled. No `--enable-features` flag string, no manual `chrome://flags` toggling.
2. **Real `http://127.0.0.1` origin** ([internal/backend/chrome_cdp.go](internal/backend/chrome_cdp.go)). `about:blank` does not expose `LanguageModel`; the API requires a real http origin. fenster's HTTP server doubles as the page Chrome navigates to.
3. **Synthesized user gesture over CDP**. `Runtime.evaluate {userGesture: true}` causes Chrome to treat the call as user-initiated. `LanguageModel.create()` requires a user gesture for the model-download trigger.
4. **Sentinel session reuse**. One `LanguageModel` session is created at startup and stashed on `window.__fensterSentinel`; every request uses `.clone()`. `LanguageModel.create()` costs ~5-8s, `.clone()` costs ~50ms.
5. **Pre-warm goroutine** at server boot. `PreWarm()` calls `initOnce()` on a background goroutine so the cold-start cost is paid before the first client request.
6. **Single shared Chrome via lockfile** ([internal/chrome/shared.go](internal/chrome/shared.go)). `~/.fenster/run/chrome.json` plus `flock(2)`. Multiple `fenster --serve` instances attach to the same Chrome; first one launches, the rest reuse.
7. **MV3 extension and Native Messaging host** ([extension/](extension/), [internal/nm/](internal/nm/)). Service worker, manifest with `nativeMessaging` permission, 4-byte LE length-prefix framing, per-OS manifest installer. Wired and tested. Not the default path; documented fallback for environments where the CDP `userGesture` path is blocked.
8. **apfel pytest suite vendored** ([Tests/integration/](Tests/integration/)). 233 transport-agnostic tests written for apfel's Swift server, talking HTTP to `localhost:11434`. fenster's Go server passes 172 of them today.

## Architecture decisions you should know

These behavior notes come from the current implementation and test matrix:

1. **Chrome Canary 149+ is required.** Stable Chrome does not expose `LanguageModel` even with `--enable-features=PromptAPIForGeminiNano`. `fenster doctor` will guide you.
2. **Headless mode works.** `--headless=new` plus a bootstrapped `Local State` plus a real `http://127.0.0.1` origin plus `userGesture:true` CDP `Runtime.evaluate` trips the model-download gate.
3. **One shared Chrome per machine** via `~/.fenster/run/chrome.json` lockfile. Many `fenster --serve` instances attach to the same browser.
4. **Sentinel session reuse plus pre-warm at startup.** First request after `fenster --serve` returns in under 2 seconds because the cold `LanguageModel.create()` tax is paid in the background.
5. **Ctrl-C semantics ported from apfel.** SIGINT at the chat prompt exits 130 with a terminal reset. SIGINT mid-response cancels the request and returns to the prompt.

Performance principles in [docs/architecture.md](docs/architecture.md). Chrome flags, why each one is needed, in [docs/chrome-flags.md](docs/chrome-flags.md). Native Messaging framing details in [docs/native-messaging.md](docs/native-messaging.md).

## Build from source

```bash
git clone https://github.com/Arthur-Ficial/fenster
cd fenster
make build              # release binary to bin/fenster
make test-fast          # Go unit + non-model integration in 30 seconds
make test               # full apfel-compat suite, real Gemini Nano, about 5 minutes
```

Modern Go (1.22+), stdlib first. Direct deps: `cobra`, `chromedp`, `term`. No third-party HTTP router. No mocks of Chrome.

## Sister project

[apfel](https://github.com/Arthur-Ficial/apfel) is fenster's macOS Apple-Intelligence twin. Wire format is byte-for-byte compatible. Clients written for apfel work against fenster too.

## Contributing

See [the open issues](https://github.com/Arthur-Ficial/fenster/issues). Issues with `up-for-grabs` are well-scoped places to start. The big ones:

* `FEN-201` MCP host-side execute loop (auto-tool dispatch)
* `FEN-202` Chat TUI completeness (arrow keys, JSON-mode, MCP integration)
* `FEN-203` cli_e2e text-matching corners
* `FEN-205` Tool-calling shim (responseConstraint)
* `FEN-206` Real streaming SSE from `LanguageModel.promptStreaming()`
* `FEN-207` HTTP MCP client (Streamable HTTP)
* `FEN-208` Distribution: Homebrew tap, Scoop bucket, apt deb

## License

MIT. See `LICENSE`.
