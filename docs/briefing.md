# fenster — briefing

Source: email from Franz (2026-04-27 12:30) → "create project /dev/fenster/ and save this as /docs/briefing.md"

This is the verbatim Q&A transcript that triggered the project. Future-you: read this end-to-end before touching code. The architectural decisions and tradeoffs below are the spec.

---

## Q1 — Franz

> what is this — what plattform does it work?
> https://developer.chrome.com/docs/ai/prompt-api
> explain in bulletpoints and so on?

## A1 — Claude

### What it is

Chrome's **Prompt API** — a browser-built-in JavaScript API that lets web pages and Chrome Extensions send natural-language prompts to **Gemini Nano**, Google's small on-device model bundled with Chrome. No server calls, no API keys, no data leaves the device after the initial model download.

### Key facts

- Exposed as a global `LanguageModel` object (e.g. `LanguageModel.availability()`, `LanguageModel.create()`, `session.prompt()`, `session.promptStreaming()`).
- **Status:** Origin Trial for the web (Chrome 138+); a second Origin Trial for sampling parameters (`topK`, `temperature`) lands in Chrome 148. Not stable/shipped yet.
- For Chrome Extensions, the API is directly available (no Origin Trial token needed).
- Multimodal input: text, image, audio. Output: text only.
- Languages currently supported: English, Japanese, Spanish.
- Supports streaming, JSON Schema structured output, session cloning, abort signals, and an `initialPrompts` array for system prompt + chat history.

### Where it works (platforms)

- **Desktop only:** Windows 10/11, macOS 13+ (Ventura+), Linux, ChromeOS on Chromebook Plus.
- **Not supported:** Chrome on Android, iOS, or non-Plus ChromeOS devices.
- **Hardware floor:** GPU with >4 GB VRAM, or CPU with ≥16 GB RAM and ≥4 cores. Audio input requires a GPU.
- **Storage:** ≥22 GB free on the Chrome profile volume (model is several GB; auto-removed if free space drops under 10 GB).
- **Network:** needed only for the initial Gemini Nano download; after that fully offline.
- Available in top-level windows and same-origin iframes; cross-origin iframes need `allow="language-model"`. Not available in Web Workers.

### Critical caveats

- This is **Chrome-only**. Not a web standard yet — Safari, Firefox, Edge (despite Chromium base) do not expose it. Building production features on it means single-browser dependency.
- Origin Trial = time-limited; tokens expire and the API shape can still change.
- Gemini Nano is a small model — quality is meaningfully below cloud Gemini/GPT/Claude. Suitable for classification, extraction, short rewrites, summarization. Not suitable for complex reasoning.
- The hardware/storage gate excludes a large portion of real-world users, so any deployed feature needs a server-side fallback.

Given your GEO/AI-first work, the relevant angle: this is one piece of the broader "browser-native AI" shift (alongside WebMCP, also documented on the same site) — worth tracking, not yet worth betting on for client deliverables.

---

## Q2 — Franz

> can we create something like apfel — and on what devices — but using this in the backend?!

## A2 — Claude

**Short answer: conceptually yes, practically no — not as a drop-in apfel equivalent.**

### What apfel does

Swift CLI → wraps Apple FoundationModels → exposes an OpenAI-compatible HTTP server on localhost. Any tool that speaks OpenAI API can point at it.

### What a Chrome-Prompt-API version would look like

You can't build a CLI. The Prompt API only exists inside a Chrome browser tab or Chrome Extension — there is no Node binding, no standalone runtime, no way to call Gemini Nano from outside Chrome. The model weights are locked inside Chrome's process.

So the closest architecture is:

- A Chrome Extension (or a pinned tab running a local page) that hosts the Prompt API.
- An HTTP bridge — but here's the wall: extensions and pages cannot open a listening socket. Browsers don't expose server sockets to JS.
- Workarounds: WebSocket client connecting outward to a tiny native helper, or **Native Messaging** (extension ↔ local native binary over stdio), and that native binary exposes the OpenAI-compatible HTTP endpoint.

So the realistic stack is:

```
your CLI client → local Node/Go/Rust HTTP server (OpenAI-compatible)
              → Native Messaging → Chrome Extension → Prompt API → Gemini Nano
```

Functional, but architecturally absurd compared to apfel — Chrome must be running, the extension installed, a tab/service worker alive, and you've added two IPC hops.

### Devices it would run on

- Windows 10/11, macOS 13+, Linux desktop, ChromeOS on Chromebook Plus
- GPU >4 GB VRAM, or CPU with ≥16 GB RAM + 4 cores
- ≥22 GB free disk (Gemini Nano gets evicted under 10 GB free)
- **Not** Android, iOS, mobile in general

Notable: this gives you Linux and Windows coverage, which apfel does not. **That is the only real reason to do this.**

### Honest assessment

Gemini Nano is smaller and weaker than Apple's FoundationModels (~3B). The Prompt API is still in Origin Trial — shape can change. You'd ship a fragile multi-hop bridge to wrap a weaker model on platforms apfel doesn't target. If the goal is "apfel for non-Mac," a cleaner path is wrapping llama.cpp or ollama — both already expose OpenAI-compatible endpoints natively and run on the same hardware without Chrome in the loop.

The Chrome route only makes sense if you specifically want to demonstrate browser-native AI as part of a GEO/workshop narrative — which, given your work, might actually be the point.

---

## Q3 — Franz

> but is it possible?!?

## A3 — Claude

**Yes, technically possible.** Here's the minimum viable architecture:

### The chain

1. Chrome Extension with `"permissions": ["nativeMessaging"]` + Prompt API access
2. Native host binary (Go/Rust/Node) registered with Chrome via a manifest JSON in the OS-specific location
3. Native host exposes `http://localhost:PORT/v1/chat/completions` (OpenAI-compatible)
4. Native host ↔ extension communicate over stdio (Native Messaging protocol: 4-byte length prefix + JSON)
5. Extension's service worker calls `LanguageModel.create()` / `session.prompt()` and pipes results back

### Flow for one request

1. Client (Cursor, Continue.dev, your CLI) → POST to localhost HTTP server
2. Native host serializes request → writes to stdout (Chrome reads it)
3. Extension service worker receives message → calls Prompt API
4. Streams chunks back via `port.postMessage()` → native host stdin
5. Native host reformats as OpenAI SSE → streams to HTTP client

### Known constraints to plan for

- Chrome must be running with the extension installed and enabled
- Native Messaging hosts are launched **by Chrome**, not the other way around — so the extension boots the bridge, not your CLI. Means: open Chrome first, then your tools work.
- Service workers in MV3 die after ~30s idle. You need to keep it alive (offscreen document, or a persistent connected port — a long-lived Native Messaging port keeps the worker alive).
- Prompt API is not available in Web Workers, but is available in extension service workers and offscreen documents.
- Token streaming works (`promptStreaming()` returns a `ReadableStream`), so OpenAI SSE compatibility is doable.
- One session per conversation — you'll want to map OpenAI's stateless `messages[]` to either a fresh session each call (slow, recreates context) or a session pool keyed by conversation hash.
- JSON mode / tool calling: Prompt API has `responseConstraint` (JSON Schema) — you can map OpenAI's `response_format: json_schema` to it. OpenAI-style function calling has no direct equivalent; you'd have to fake it with structured output + prompt engineering.

### Realistic scope

A working prototype is a weekend. Chat completions endpoint, streaming, basic JSON mode — yes. Full OpenAI API surface (embeddings, vision in OpenAI's exact format, function calling parity) — significant work, and some parts (embeddings) the Prompt API simply doesn't offer.

### Naming, since you have a theme going

German Unix-philosophy CLI family: `apfel` (Apple), so the Chrome one wants something equally on-the-nose. **`chrom`** is the obvious one. **`fenster`** (window) also fits. `nano` is taken by the model itself.

---

## Q4 — Franz

> i want you to go deep into the documentation and figure out a real way how to use this on all linux windows and so on mac platforms — and also do we go rust or go and so on!!

## A4 — Claude

### Building apfel-for-Chrome — full architecture

#### The verified facts

**Service worker stays alive on a native port.** Per Chrome docs: `chrome.runtime.connectNative()` keeps a service worker alive as long as the port is open. If the host crashes, the port closes and the worker dies — so reconnect inside `port.onDisconnect`. This kills the usual MV3 30-second/5-minute timeout problem dead. **No offscreen document hack needed.**

**One process per port.** When Chrome opens a Native Messaging port, it spawns the host binary as a child process and pipes JSON over stdin/stdout (4-byte little-endian length prefix + UTF-8 JSON). Max message size: 1 MB host→Chrome, 4 GB Chrome→host. The host's lifetime equals the port's lifetime — Chrome owns the process.

**Critical implication for HTTP-server design.** The host binary is spawned by Chrome, not by the user. So the HTTP server has to live inside the host binary, started on first message. Order of operations: user opens Chrome → extension auto-connects on startup → host binary launches → host opens `localhost:PORT` → CLI clients can connect. If Chrome isn't running, nothing answers.

#### Per-OS install matrix

The native host binary is OS-agnostic, but **manifest registration differs sharply per OS**.

- **macOS** — drop a JSON file at `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.fullstackoptimization.chrom.json`. Per-browser path: Chromium, Edge, Brave each have their own directory. Path inside the manifest must be absolute.
- **Linux** — `~/.config/google-chrome/NativeMessagingHosts/com.fullstackoptimization.chrom.json` for user-level, `/etc/opt/chrome/native-messaging-hosts/` for system-wide. Brave/Chromium/Edge have parallel paths. Absolute paths required.
- **Windows** — manifest can sit anywhere; the location is announced via a registry key: `HKEY_CURRENT_USER\Software\Google\Chrome\NativeMessagingHosts\com.fullstackoptimization.chrom` with the default value pointing to the JSON. Two extra Windows footguns: (1) set stdin/stdout to binary mode (`_setmode(_fileno(stdin), _O_BINARY)`) or CRLF in payloads will corrupt the length prefix; (2) registry has 32-bit and 64-bit views, Chrome checks 32-bit first.

The manifest itself is identical across OSes:

```json
{
  "name": "com.fullstackoptimization.chrom",
  "description": "Chrom: OpenAI-compatible bridge to Gemini Nano",
  "path": "/absolute/path/to/chrom-host",
  "type": "stdio",
  "allowed_origins": ["chrome-extension://YOUR_EXTENSION_ID/"]
}
```

#### Rust vs Go — the actual decision

Both work. Both have battle-tested ecosystems for this exact job. The honest tradeoff:

- **Go** — `github.com/rickypc/native-messaging-host` and `sashahilton00/native-messaging-host` are mature, handle the framing, manifest install, even auto-update. `net/http` in stdlib is excellent. Cross-compile to Win/Mac/Linux/arm64 is one `GOOS=... GOARCH=... go build` command, single static binary, no runtime. SSE streaming is trivial (`http.Flusher`). Build time minutes, not hours. Binary size ~10 MB.
- **Rust** — `native_messaging` crate (recent, includes the cross-platform manifest installer, async via Tokio). Tokio + axum or hyper for HTTP/SSE. Binary size ~5 MB with size optimizations. Cross-compilation works but is fiddlier (need `cross` or per-target toolchains, especially for Windows from Mac). Build times longer.

**Recommendation: Go.** Reasoning, with the apfel context:

- apfel is Swift because it has to be — Apple FoundationModels only has a Swift binding. There's no analogous constraint here.
- The host's job is 90% I/O plumbing (stdin framing, HTTP server, SSE). Go's stdlib does exactly this without dependencies.
- Cross-compilation matters more than runtime perf. You'll ship binaries for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64. Go does this in one Makefile.
- The "real" inference happens inside Chrome's process running Gemini Nano on the GPU. The host is a thin pipe. Rust's perf advantages are wasted here.
- Go's concurrency model (goroutine per HTTP request, channel back to the single stdin/stdout pump) maps the architecture directly.

Pick Rust only if you (a) want a single tree shared with future native-inference work, or (b) want the smaller binary for Homebrew-core eligibility. Otherwise Go.

#### The full architecture, concretely

```
┌─────────────┐   HTTP+SSE     ┌──────────────┐  stdio    ┌─────────────────┐
│  CLI client │ ──────────────▶│  chrom-host  │ ◀────────▶│ Chrome ext SW   │
│ (curl, IDE) │ ◀──────────────│  (Go binary) │           │  (background)   │
└─────────────┘  localhost:    └──────────────┘           └────────┬────────┘
                  11434                                             │
                                                                    ▼
                                                          ┌─────────────────┐
                                                          │  LanguageModel  │
                                                          │  (Gemini Nano)  │
                                                          └─────────────────┘
```

Request lifecycle for `POST /v1/chat/completions` with `stream: true`:

1. CLI POSTs OpenAI-format JSON to host's HTTP server.
2. Host assigns request ID, frames `{id, type:"chat", payload:{...}}` to stdout.
3. Chrome SW reads from port, looks up or creates a `LanguageModel` session keyed by request shape, calls `session.promptStreaming()`.
4. SW iterates the ReadableStream, sending each chunk as `{id, type:"chunk", delta:"..."}` back via `port.postMessage()`.
5. Host receives chunks on stdin, transforms each into OpenAI SSE format (`data: {...}\n\n`), flushes to HTTP response.
6. SW sends `{id, type:"done"}`. Host writes `data: [DONE]\n\n` and closes the stream.

**Session strategy.** OpenAI's API is stateless — every request sends the full `messages[]`. Two options:
(a) cheap path: create a fresh `LanguageModel` session per request, replay everything via `initialPrompts`, destroy after. Slow but simple.
(b) fast path: hash the `messages[0..n-1]` prefix, cache sessions by hash, only `prompt()` the new last message. Big win for multi-turn IDE assistants. Apply LRU eviction since each session holds context-window state.

**JSON mode mapping.** OpenAI `response_format: {type: "json_schema", json_schema: {...}}` → Prompt API's `responseConstraint`. Direct map. OpenAI `response_format: {type: "json_object"}` → pass `{type: "object"}` as schema.

**Tool/function calling.** No native equivalent in Prompt API. Fake it: inject a system prompt describing tools, use `responseConstraint` to force a `{tool_call: {...}} | {content: "..."}` schema, parse on the host side. Works imperfectly with a 3B model — manage user expectations.

**Embeddings.** Prompt API doesn't expose them. Return 501 from `/v1/embeddings` or proxy to a separate WASM model. Be honest in docs.

#### Cross-platform reality check

| Platform | apfel | chrom |
|---|---|---|
| macOS 26+ | yes | macOS 13+ |
| Windows | no | Windows 10/11 |
| Linux | no | yes |
| ChromeOS | no | Chromebook Plus only |
| iOS/Android | no | no |

The hardware floor (>4 GB VRAM or 16 GB RAM + 4 cores, 22 GB free disk for the model) excludes a real chunk of users. Plan a graceful "model not available" response from the host.

#### Naming

`chrom` is the obvious German/Chrome pun and fits the apfel/ohr/auge family. It also reads as "chromium" — accurate, since this works on any Chromium browser (Edge, Brave, Opera) by adding their manifest paths and extension IDs.

#### What I'd ship in v0.1

- Single Go binary `chrom-host` that auto-installs its own manifest on first run (`chrom install`), uninstalls on `chrom uninstall`, supports `--browser=chrome|brave|edge|chromium`.
- One Chrome extension with no UI, just the service worker auto-connecting on startup.
- HTTP endpoints: `/v1/chat/completions` (stream + non-stream), `/v1/models` (returns `gemini-nano`), `/health`.
- Session cache with prefix hashing.
- Structured output via `responseConstraint`.
- Honest 501s for embeddings and audio/image until v0.2.

---

## Q5 — Franz

> how about make this more user friendly. start a headless chrome and so on… serious debugging and so on!

## A5 — Claude

### Making it actually user-friendly: the headless approach

#### What changed in my thinking

After research, the architecture flips. The earlier "user must keep Chrome open" model is awful UX. There's a much better path.

#### The key facts that reshape the design

**1. `--headless=new` (Chrome 112+) supports extensions and service workers fully.** Not the old `--headless` — that's the legacy mode that never had extension support and was the source of years of "extensions don't work in headless" complaints. The new mode launches the real Chrome browser engine, just without painting a window. Extensions, service workers, native messaging, GPU acceleration — all work.

**2. Gemini Nano lives in Chrome's user-data-dir.** It's a Component Updater payload (~2.4 GB) installed alongside the user's profile. Two consequences: (a) if you launch Chrome with `--user-data-dir=/some/fresh/path`, you redownload 2.4 GB; (b) if you use the user's existing profile while their normal Chrome is running, you get profile-locking conflicts.

**3. The model needs a GPU.** That's the constraint that makes "true headless on a Linux server" hard — `--disable-gpu` (the standard headless flag) kills Gemini Nano. You need a GPU-capable headless setup, or to run on the user's actual desktop with GPU passthrough working.

#### The actual user experience to ship

```bash
$ brew install chrom    # or scoop, or apt
$ chrom start
✓ Found Chrome at /Applications/Google Chrome.app
✓ Created dedicated profile at ~/.chrom/profile
✓ Downloading Gemini Nano (2.4 GB)... done
✓ Listening on http://localhost:11434
✓ OpenAI-compatible API ready

$ curl http://localhost:11434/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-nano","messages":[{"role":"user","content":"hi"}]}'
```

`chrom start` runs in foreground (or `chrom start -d` for daemon). `chrom stop` kills everything. User never sees a Chrome window. User never opens Chrome manually.

#### How `chrom start` actually works

The Go binary does this on launch:

1. **Locate Chrome.** Look in OS-default install paths: `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, `C:\Program Files\Google\Chrome\Application\chrome.exe`, `/usr/bin/google-chrome` etc. Fall back to `$CHROM_BROWSER` env var. Refuse to start if not found, with install instructions.
2. **Use a dedicated profile dir** at `~/.chrom/profile` (or `%LOCALAPPDATA%\chrom\profile` on Windows). This isolates from the user's main Chrome, avoids profile-lock conflicts when both run, and contains the model download. Yes, this means the user pays the 2.4 GB download once for chrom separately from their normal browsing — that's the price of not stomping on their actual profile.
3. **Write the extension** (bundled inside the Go binary as embedded files via `embed.FS`) to `~/.chrom/extension/` on first run. No Chrome Web Store dependency.
4. **Write the native messaging manifest** to the right OS location (paths from Q4), pointing `path` at the running Go binary's own absolute path (`os.Executable()`).
5. **Spawn Chrome:**

   ```
   chrome \
     --headless=new \
     --user-data-dir=$HOME/.chrom/profile \
     --load-extension=$HOME/.chrom/extension \
     --disable-extensions-except=$HOME/.chrom/extension \
     --enable-features=PromptAPIForGeminiNano,OptimizationGuideOnDeviceModel \
     --remote-debugging-port=9222 \
     --no-first-run \
     --no-default-browser-check \
     about:blank
   ```

   Notes: don't add `--disable-gpu` (kills Nano). The feature flags are the headless equivalent of the manual `chrome://flags` toggles.
6. **Use CDP** (Chrome DevTools Protocol) on `:9222` to verify the extension loaded and trigger the first model availability check. This is the "press the button to wake the service worker." Library: `chromedp` for Go.
7. **Wait for native messaging connection** from the extension's service worker → that opens the stdio pipe → the Go process now has bidirectional comms with Gemini Nano running inside the headless Chrome.
8. **Open the HTTP server** on `localhost:11434`. From here it's the same OpenAI-compatible bridge as Q4.

#### Serious debugging — also user-friendly

This is where most "headless wrapper" tools fail. Build it in from day one:

- **`chrom logs`** — tails a structured log file at `~/.chrom/logs/chrom.log`. Both Go-side and Chrome-side events. Use a subprocess log scraper that captures Chrome's stderr (passed `--enable-logging --v=1`).
- **`chrom doctor`** — runs every precondition check and prints a status report:
  - Chrome found? Version ≥ 138?
  - GPU detected? VRAM > 4 GB? (parse `chrome://gpu` via CDP)
  - Disk space ≥ 22 GB free?
  - Model downloaded? (visit `chrome://on-device-internals` via CDP, scrape status)
  - Extension loaded? Service worker registered?
  - Native messaging port open?
  - HTTP server reachable?

  **This is the single most important UX feature.** Every failure mode in this stack is invisible by default.
- **`chrom debug`** — launches with `--headless=new` off so the user sees the Chrome window, plus auto-opens DevTools for the service worker (CDP can do this). When something is broken and `doctor` doesn't find it, this is the escape hatch.
- **`chrom inspect`** — exposes `chrome://inspect`-equivalent functionality. Print URLs the user can paste into their real Chrome's address bar to attach DevTools to the headless instance via the remote debugging port. This is a known trick: visit `chrome://inspect/#devices`, configure target discovery to include `localhost:9222`, then attach to the service worker remotely.
- **Structured request tracing.** Every HTTP request gets a UUID. Log: HTTP receive → stdio frame sent → SW received → session created/reused → `prompt()` called → first chunk → last chunk → HTTP closed. Each step with a timestamp. `chrom trace <request-id>` prints the timeline.
- **Crash recovery.** If Chrome dies (it will), the Go process detects port closure, logs the cause (parse Chrome's exit code and last stderr lines), and either restarts Chrome automatically (default) or surfaces the error. Backoff with limits — don't fork-bomb on a model-load failure.

#### The trade-offs you accept with this approach

- **Disk:** ~2.5 GB profile + ~2.4 GB model = ~5 GB on first run. Separate from the user's normal Chrome. You can give them an opt-in flag `--use-system-profile` for the brave, with a warning.
- **RAM:** significant. A headless Chrome with the model loaded is 1–3 GB resident. Not a daemon you forget about — show this in `chrom status`.
- **Startup time:** ~2–5 seconds cold. Chrome boot + extension load + native messaging handshake + first session creation. Acceptable for a daemon, terrible for one-shot CLI use. Default to daemon mode.
- **Linux server use:** requires GPU. Gemini Nano needs >4 GB VRAM or a 16 GB+ RAM CPU fallback. On a typical headless Linux server with no GPU, this either fails or runs the CPU path very slowly. Honest answer: this is a desktop tool, not a server tool.
- **Chrome version coupling.** Chrome auto-updates and the Prompt API is still in Origin Trial. Pin a known-good version range in `doctor`. Warn loudly when Chrome silently updates past your tested range.

#### The full story end-to-end

```
                ┌──────────────────────────────────────┐
                │  chrom (Go daemon)                   │
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
                │  --user-data-dir=~/.chrom/profile    │
                │  ┌────────────────┐                  │
                │  │ chrom extension│                  │
                │  │  service worker│                  │
                │  │   ↕ stdio      │                  │
                │  │   ↕ native msg │                  │
                │  │  LanguageModel │                  │
                │  └───────┬────────┘                  │
                │          ▼                           │
                │   Gemini Nano (GPU)                  │
                └──────────────────────────────────────┘
```

A real consumer-grade tool. `brew install`, `chrom start`, point any OpenAI-compatible client at `localhost:11434`. For your GEO/AI-first workshop angle, this is also a very compelling demo: "browser-native AI, no Chrome window, OpenAI API on Linux/Windows."

---

## Q6 — Franz

> is there another way to use headless nano local?

## A6 — Claude

### Other ways to run Gemini Nano locally — and the honest verdict

#### Short answer

**For literally Gemini Nano (the same Google weights):** Chrome is essentially the only sanctioned desktop path. Everything else is either gone, never worked, or is on Android.

**For the goal (a small local model behind an OpenAI-compatible HTTP server, headless, cross-platform):** there are dramatically better paths than Chrome. You'd be wrapping the wrong model for the wrong reason.

#### Paths to actual Gemini Nano

1. **Chrome / Chromium browsers + Prompt API.** What we've been discussing. The only first-party desktop path. Same weights. Locked behind the browser process.
2. **MediaPipe `tasks-genai` (browser/JS).** A Hugging Face user (`oongaboongahacker`) extracted the Nano weights from Chrome and packaged them as a `.bin` file consumable by Google's MediaPipe LLM Inference SDK. Technically runs the same model outside Chrome — in JS via `@mediapipe/tasks-genai`. But: the weights upload is over a year old, of unclear license status (Google has not released Nano weights — these were pulled from a Chrome installation), and the API is browser-side WebGPU/WASM, not a server runtime. So you're still in a browser-like environment, just without Chrome's Origin Trial gating. **Not a real "headless server" path.**
3. **MediaPipe LLM Inference (native Android/iOS).** Google's official SDK. But it ships with **Gemma** weights, not Nano. On Android you can call AICore which dispatches to Nano on the device's NPU — that's mobile-only.
4. **Android AICore + AI Edge SDK.** The official on-device Gemini Nano path. Runs only on Pixel 9+, certain Samsung Galaxy S24/S25, and a handful of MediaTek/Qualcomm flagship-equipped Androids. There is no desktop AICore. No Linux, no macOS, no Windows.
5. **llama.cpp / Ollama support for Nano.** Confirmed: **no.** Issue is open since 2024 and won't close — Google has not released the weights or architecture details needed for a GGUF conversion. The model uses a custom "low-rank adaptation" structure that llama.cpp can't represent without source weights.

That's the complete set. Outside Chrome, your real options for actual Nano are: Hugging Face's reverse-engineered MediaPipe bundle (not really server-grade), or buy a Pixel 9.

#### The honest reframe

The question worth asking is **why Gemini Nano specifically** rather than "a small on-device LLM." The Nano-specific reasons:

- It's the same model the browser exposes (matters if you're building dual-mode apps or testing browser AI features).
- It's tuned for short, focused tasks and benchmarks reasonably for its ~3.25B size.
- Google's brand attached to it.

**None of those reasons survive the practical constraint that Nano is locked in a browser.** If you're doing GEO/AI-first work where the story is "browser-native AI" — yes, Nano via Chrome matters. If the goal is "a local model my CLI can hit" — Nano is one of the worse choices available.

#### What actually works for "OpenAI-compatible local LLM, headless, cross-platform"

These all run as standalone daemons, no browser involved, on Linux/Mac/Windows, with OpenAI-compatible HTTP servers built-in:

- **Ollama.** Single binary, install via Homebrew/winget/script. Runs any GGUF model. Exposes `http://localhost:11434/v1/chat/completions` natively (same port you'd want for chrom, ironically). Works on CPU, CUDA, Metal, Vulkan, ROCm. Production-quality. **This is the obvious answer.**
- **llama.cpp's `llama-server`.** Lower level than Ollama, more tuning. OpenAI-compatible endpoint. Same model coverage.
- **LM Studio.** GUI-first but ships an OpenAI-compatible server. Easier for non-developers.
- **MLX (Apple-only).** Apple's native ML framework for Apple Silicon. Faster than llama.cpp on M-series for many models. Has Python bindings; OpenAI server via `mlx-lm`.

#### What model would replace Nano on these runtimes

If your real ask is "small, fast, local, decent quality":

- **Gemma 3 270M / 1B / 4B** — Google's open small models, Apache 2.0 licensed, GGUF-ready. Direct philosophical replacement for Nano: same lineage, same Google research, fully open. The 4B is ~Nano size. 1B for laptops, 270M for embedded.
- **Gemma 3n E2B / E4B** — Google's edge-optimized variant, the actual "successor lineage" to Nano architecturally. Multimodal (text/image/audio), runs on phones and laptops.
- **Qwen 3 4B / 8B** — Often outperforms Gemma at the same size. Apache-ish licenses.
- **Phi-4 mini** — Microsoft's small model, strong reasoning for size.
- **SmolLM3** — when you want smaller than 1B.

Any of these in Ollama gives you literally the architecture you've been spec'ing for chrom, minus Chrome, minus the headless complexity, minus the 22 GB disk requirement, minus the Origin Trial fragility.

#### So the honest decision tree

- **If the project is an apfel sibling for non-Mac platforms and you want it to work:** Don't wrap Chrome. Wrap Ollama, or just tell users to install Ollama directly. Your CLI family (`ohr`, `auge`, `nähe`, etc.) can target any OpenAI-compatible endpoint — Ollama, apfel, OpenRouter, Anthropic — uniformly. That's the right abstraction.
- **If the project is "demonstrate browser-native AI" for your GEO/workshop work:** The Chrome headless approach is the right answer **because** the point is that it's Chrome. Ship `chrom` (or `fenster`) as a demo / educational tool, not as a production daemon. Make it explicit in the README: "this exists to show what Chrome's Built-in AI can do, not to replace Ollama."
- **If the project is "I want the same model the browser uses, hit from CLI, for parity testing":** Then yes, the headless Chrome bridge is unavoidable. There is no other supported path.

My read of your actual situation: you're an SEO/GEO consultant whose value-add is showing clients what AI in the browser actually means. `chrom`/`fenster` as a demo/teaching artifact has real value to your workshop business — "look, here's the literal Chrome model, served as if it were OpenAI." The technical absurdity is the demo. As a serious tool for daily use, Ollama wins on every dimension.

---

## TL;DR for fenster v0.1

1. **Name:** `fenster` (Franz picked the directory name; `chrom` was the alternate).
2. **Language:** Go. Rationale in Q4. Single static binary, trivial cross-compile.
3. **Architecture:** Headless Chrome supervisor + embedded extension + Native Messaging stdio bridge + OpenAI-compatible HTTP server on `localhost:11434`.
4. **Distribution:** `brew install fenster` / `scoop install fenster` / `apt install fenster`. Single binary embeds the extension via `embed.FS`. Auto-installs native messaging manifest on first run.
5. **CLI surface (apfel-tier, not MVP):** `start`, `stop`, `status`, `restart`, `doctor`, `logs`, `trace`, `debug`, `inspect`, `install`, `uninstall`, `models`, `version`, plus a `serve` daemon mode. Stable exit codes. JSON envelope output (`--json`). Config cascade (`~/.fenster/config.toml` + env + flags).
6. **HTTP endpoints:** `/v1/chat/completions` (stream + non-stream), `/v1/models`, `/health`. 501 on `/v1/embeddings` until proven otherwise.
7. **Browser support:** `--browser=chrome|chromium|brave|edge` — manifest paths and feature flags differ but the host binary is shared.
8. **Positioning:** GEO/workshop demo artifact, not Ollama replacement. README says so plainly.
9. **TDD discipline:** mirror apfel — 100% real tests, no fakes, no canned-response JSON, no mocks of Chrome. Tests spawn real headless Chrome with a known-good extension.
10. **Local CI only:** `make test` + pre-push hook. No `.github/workflows`.

## Open questions for Franz before scaffolding

1. **Confirm `fenster` over `chrom`** — directory says `fenster` but most of the briefing reads `chrom`. Pick one.
2. **Demo vs daily-driver positioning** — README tone changes accordingly.
3. **Origin Trial handling** — register a token under `fullstackoptimization.com` for the bundled extension, or rely on the no-token-needed extension path? (Extensions don't need OT tokens; this is probably moot.)
4. **`fenster` family role** — does this slot into the apfel/ohr/auge family as a peer (its own CLI), or as a backend that the existing CLIs target via `OPENAI_BASE_URL=http://localhost:11434`?
