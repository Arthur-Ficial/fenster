# fenster — current status

**Updated**: 2026-04-27 16:14 (end-of-day snapshot, autonomous session)
**Repo**: https://github.com/Arthur-Ficial/fenster
**Latest commit**: `cf170b3`

## Headline

**REAL Gemini Nano answers prompts through fenster end-to-end, in headless Chrome, byte-correct OpenAI wire format.** Live proof:

```
$ ./bin/fenster --serve --port 11434
fenster: launched shared Chrome at http://127.0.0.1:62030
fenster v0.0.1 — listening on http://127.0.0.1:11434/v1

$ curl -X POST http://127.0.0.1:11434/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model":"gemini-nano","messages":[{"role":"user","content":"What is the capital of France? Answer with one word."}]}'

{"id":"chatcmpl-9d888a87ba6f","object":"chat.completion","created":1777293692,
 "model":"gemini-nano",
 "choices":[{"index":0,
   "message":{"content":"Paris\n","refusal":null,"role":"assistant"},
   "finish_reason":"stop","logprobs":null}],
 "usage":{"prompt_tokens":18,"completion_tokens":2,"total_tokens":20}}
```

AppKit window count for Chrome: **0**. fenster's Chrome is fully invisible.

## Test scoreboard

| Suite | Passed | Failed | Skipped | Total |
|---|---|---|---|---|
| Go unit (race-clean) | **all** | 0 | 0 | ~60 |
| apfel pytest (vendored) | **146** | 85 | 2 | 233 |

Pytest progression today (each row is a commit, 14 commits total):

| Commit | passed | gain |
|---|---|---|
| start | 0 | — |
| `188e2ee` M0 scaffold | 0 | — |
| `f71c357` shared core + UNIX tool + server (echo) | 84 | +84 |
| `2cc2eee` security middleware + debug logs | 96 | +12 |
| `cda1d1a` real Gemini Nano via Chrome bridge | 105 | +9 |
| `e44d3e0`–`deee492` headless + Canary + bootstrap | 105 | 0 (architecture) |
| `3a2bf04` -f/--file + flat JSON | 128 | +23 |
| `f8c9f11` --update/--release + USAGE: + exit codes | 139 | +11 |
| `1232031` man-page lint fixes | 142 | +3 |
| `ba176d6` footgun preflight + /health on loopback | 146 | +4 |
| `cf170b3` Sources/main.swift -> cmd/fenster/main.go | 146 | 0 |

**+146 tests today**, real model serving headlessly, full architecture in place.

## What's solid

### Shared core (TDD, no IO)
- `internal/core/wire` — OpenAI types, byte-correct
- `internal/core/errors` — Sentinel pattern, every apfel rejection rule
- `internal/core/validate` — order-preserving rejection pass
- `internal/core/tokens` — heuristic counter (prompt+completion sum invariant)
- `internal/core/ids` — `chatcmpl-*` / `call_*` shapes

### Backends
- `EchoBackend` — wire-format-honest deterministic
- `NullBackend` — model_unavailable safe default
- `ChromeBackend` (NM bridge) — extension architecture (legacy path)
- **`ChromeCDPBackend` — direct CDP control of headless Canary 149**

### Chrome bridge
- `internal/chrome/shared.go` — lockfile + state file → all fensters share ONE Chrome
- `internal/chrome/chrome.go` — bootstrapLocalState, Canary preferred over Stable, **headless by default** (`FENSTER_CHROME_HEADED=1` to surface)
- `internal/backend/chrome_cdp.go` — userGesture:true `Runtime.evaluate` triggers model download; polls availability for up to 15 min on first run

### Native Messaging
- `internal/nm` — 4-byte LE prefix + UTF-8 JSON framing
- `internal/bridge` — Unix socket between supervisor and nm-host child
- `internal/manifest` — per-OS installer (chrome/chromium/edge/brave)
- `internal/extension` — embedded MV3 extension + path-derived ID

### Server
- `/`, `/health`, `/v1/models`, `/v1/chat/completions` (stream + non-stream), `/v1/completions` 501, `/v1/embeddings` 501, `/v1/logs`, `/v1/logs/stats`, OPTIONS preflight
- Middleware: origin allowlist, bearer auth (loopback bypass for /health), CORS, footgun, request logging
- Apfel-style startup banner (token presence, never the secret)

### CLI
- Apfel-shape `--version`, `--help`, exit codes (0/1/2/3/64)
- `--serve` / `--chat` (M3) / `--update` / `--release` / `--model-info` / `doctor`
- `-f` / `--file` (StringSlice; image/binary/UTF-8 validation)
- `-o json` (flat shape) and `--json` (alias)
- `--system` / `--no-system-prompt` / `--stream` / `--quiet` / `--debug`
- `--token` / `--allowed-origins` / `--cors` / `--public-health` / `--footgun` / `--host`
- `install-extension` / `install-manifest` / `nm-host` (hidden)

### Doctor
- Real env probe: macOS ≥13, Chrome 138+, GPU, ≥22 GB free, profile dir writable
- Each check carries a Fix line

### Architecture decisions (empirically proven)
- **Chrome Canary 149+ required** (Stable 147 doesn't expose `LanguageModel`)
- **Headless works** with `--headless=new` + bootstrapped Local State + real http:// origin
- **`about:blank` does NOT expose the API** — fenster serves GET / that Chrome navigates to
- **userGesture:true via CDP `Runtime.evaluate`** trips the download gate (synthetic mouse clicks were unreliable)
- **One shared Chrome per machine** via lockfile + state file
- **Per-binary profile dirs** so Canary's profile format never collides with Stable's

## What still fails (85 tests)

| Cluster | Count | Need |
|---|---|---|
| chat TUI (`test_chat.py`) | 31 | Build `fenster --chat` interactive TUI mode |
| cli_e2e text-matching | ~25 | TTY ANSI in --help, system-prompt+stream combos, stdin+file+stream multi-source |
| mcp_server (host execution) | 9 | Implement MCP auto-execute loop |
| security details | 4 | --token=auto banner echoes secret, WWW-Authenticate under CORS, log body capture shape, non-footgun preflight |
| man_page bidirectional | 3 | Match every --flag and FENSTER_/APFEL_ env in both --help and man source |
| openai_client | 3 | tool_calls shim, refusal trigger, build-info pattern |
| openapi_spec / conformance | 3 | Specific spec-level checks |
| performance | 1 | Latency budget tuned for FoundationModels; relax for Gemini Nano |
| test_build_info | 1 | Apfel-specific BuildInfo.swift assertions |
| test_chat orphan | 4 | test_chat tests that aren't TUI-dependent |
| mcp_remote | 1 | One leftover (HTTP MCP server is now vendored) |

**The biggest remaining slice is the chat TUI** (31 tests). Next concentrated push.

## Path to 100%

Realistic forecast for the remaining 85:

1. Chat TUI (`fenster --chat`): ~25 tests achievable. ~4-6 hours.
2. CLI text-matching remaining (~25): small fixes. ~2 hours.
3. MCP host execution (9 tests): auto-exec loop. ~3 hours.
4. Security/openai/spec/perf details (~12): individual fixes. ~2 hours.
5. man-page bidirectional (3): Go-aware parser. ~1 hour.

**Realistic 100% path: ~12 more focused hours of work** (one more autonomous session).

## How to verify

```bash
./bin/fenster --serve --port 11434                    # spawns headless Canary
curl -sf http://127.0.0.1:11434/health
curl -X POST http://127.0.0.1:11434/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-nano","messages":[{"role":"user","content":"hello"}]}'

FENSTER_BACKEND=echo ./bin/fenster -q -o json "what is 2+2?"
./bin/fenster doctor
make test
```
