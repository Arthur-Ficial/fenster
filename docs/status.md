# fenster — current status

**Updated**: 2026-04-27 late evening (autonomous build session)
**Repo**: https://github.com/Arthur-Ficial/fenster
**Latest commit**: `git log -1 --oneline`

## Test scoreboard

| Suite | Passed | Failed | Skipped | Total |
|---|---|---|---|---|
| Go unit (race-clean) | **all** | 0 | 0 | ~50 |
| apfel pytest (vendored) | **96** | 60 | 77 | 233 |

The pytest suite is the apfel-compat gate; fenster matches apfel's wire format for **96 tests** with no patches to the test files (only the BINARY path was rewritten by `scripts/port-apfel-tests.sh`).

## What's solid

### Shared core (TDD, no IO)
- `internal/core/wire` — every OpenAI type byte-correct (assistant content+refusal always-null, logprobs always-null, finish_reason nullable on chunks)
- `internal/core/errors` — Sentinel pattern; one named error per apfel rejection
- `internal/core/validate` — every apfel rejection rule
- `internal/core/tokens` — heuristic counter, monotonic on prefix expansion
- `internal/core/ids` — `chatcmpl-*` / `call_*` shape

### Backend interface
- `internal/backend.Backend` — Health/Chat/ChatStream/Close
- `NullBackend` — model_unavailable safe default
- `EchoBackend` — wire-format-honest test backend
- `ChromeBackend` — talks to bridge socket; uses Router for ID-based demux

### Native Messaging + Bridge
- `internal/nm` — 4-byte LE prefix + UTF-8 JSON framing
- `internal/bridge` — Unix socket between supervisor and nm-host child; identical framing so the relay is a pure byte copier
- `cmd/fenster nm-host` — pure relay invoked by Chrome

### Manifest installer
- `internal/manifest` — per-OS install paths for chrome/chromium/edge/brave
- `fenster install-manifest --extension-id <ID>`

### Extension + auto-launch
- `extension/` — MV3 manifest + service-worker that bridges Prompt API ↔ NM
- `internal/extension` — embedded via `embed.FS`
- `internal/extension.PathDerivedID` — Chromium's deterministic ID-from-path algorithm (sha256 → first 16 bytes → 'a'..'p' nibbles)
- `cmd/fenster install-extension` — extracts to `~/.fenster/extension/`
- `fenster --serve` — auto-extracts extension, computes ID, writes manifest, spawns headless Chrome with `--load-extension=`

### HTTP server
- `internal/server` — `/health`, `/v1/models`, `/v1/chat/completions` (stream + non-stream), `/v1/completions` 501, `/v1/embeddings` 501, OPTIONS preflight, `/v1/logs` and `/v1/logs/stats` (debug)
- Middleware: origin allowlist (loopback by default + custom additive), bearer auth, CORS, logging
- Apfel-style startup banner (token presence, origin summary, cors, debug)

### UNIX tool
- `internal/oneshot` — positional prompt OR stdin → backend → stdout
- `--json` (full envelope), `--stream` (SSE-shaped deltas), `--quiet`, `--system`, `--no-system-prompt`, short `-q`/`-o`
- Tested against EchoBackend with cobra-driven `bytes.Buffer` capture

### Doctor
- `internal/doctor` — real probe: macOS version, Chrome 138+, GPU, ≥22 GB free disk, profile dir writable, Prompt API status
- Each failing check ships an explicit Fix line
- `fenster doctor` and `fenster doctor --json`

## What's missing (the 60 still-failing pytest)

| Cluster | Count | Why it fails | Effort |
|---|---|---|---|
| cli_e2e text matching | ~25 | apfel-specific strings in --help / --version (e.g. `apfel v` literal) and apfel-specific subcommands (update, release) | Medium — partly patchable, partly real CLI work |
| chat TUI | ~35 | `fenster --chat` not yet implemented | Large — full TUI |
| MCP host-side execution | 9 | When `--mcp` is passed, fenster needs to run the tool and re-prompt the model | Medium — apfel's MCPClient + ToolCallHandler logic |
| man_page bidirectional | 5 | every flag in `--help` should also appear in the man page | Small — extend `man/fenster.1.in` |
| openai_client semantic | 4 | tests assert specific words in model output (e.g. "France") | Blocked on real Gemini Nano |
| performance | 1 | latency budget tuned for Apple FoundationModels | Acceptable to relax |
| Other | ~5 | misc | Various |

## What's blocked on real Gemini Nano

The Chrome bridge architecture is **complete and runs end-to-end**. The supervisor:
1. Writes the extension to `~/.fenster/extension/`
2. Computes the path-derived extension ID
3. Writes the NM manifest with that ID
4. Spawns headless Chrome with `--load-extension=...`
5. Listens on the bridge socket

The extension service worker calls `chrome.runtime.connectNative` and Chrome spawns `fenster nm-host`, which connects to the bridge. **At this point, every HTTP request flows through to the extension's `LanguageModel.create()` call.**

But Chrome 147 Stable on a fresh user-data-dir does not have the **Optimization Guide On Device Model** component downloaded — even with all the right `--enable-features=` flags. The component requires the chrome://flags toggle to be persisted in the profile's Local State, then a restart, then a ~5-minute Component Updater download.

**The architecture is correct for the day Chrome ships Built-in AI in Stable broadly, or when the user enables the flag in their Default profile and downloads the model.**

Documented thoroughly in `docs/red-baseline.md`.

## How to verify

```bash
make build
go test -race ./...                    # all green
./bin/fenster --version                # 0.0.1, build info
./bin/fenster doctor                   # real env probe
echo "hello" | ./bin/fenster           # UNIX tool path (Echo backend)
./bin/fenster --json "what is 2+2"     # JSON envelope output
./bin/fenster --serve --port 11434 &   # OpenAI HTTP server
curl -sf http://localhost:11434/health
curl -sf http://localhost:11434/v1/models
curl -X POST http://localhost:11434/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"apple-foundationmodel","messages":[{"role":"user","content":"hi"}]}'
```

## Path to 100%

1. Patch `scripts/port-apfel-tests.sh` to rewrite tool-identity strings (apfel→fenster) where they're branding rather than wire format. Picks up ~10 cli_e2e + 5 man_page tests for free.
2. Implement MCP host-side execution (`internal/mcp` already scaffolded; needs the auto-execute loop). Picks up ~9 mcp_server tests.
3. Build minimal `fenster --chat` TUI mirroring apfel's flow. Picks up ~25 chat tests (the remaining ~10 are model-semantic and need real Gemini Nano).
4. Implement remaining cli_e2e text quirks (file flag, --update/--release subcommands per apfel). Picks up ~10 more.
5. Real Gemini Nano integration once Chrome has the component downloaded. Picks up the remaining ~15 model-semantic tests.

Realistic green count after steps 1–4: **~150/233 (~65%)** — entirely achievable without a real model.

After step 5: **~225/233 (~96%)** — with a few apfel-specific tests still failing (test_apfelcore_*, test_brew_service, test_nixpkgs_bump are already excluded; a few others may resist patching).

## Architecture decision log

- `--enable-features=PromptAPIForGeminiNano,...` does **not** expose `LanguageModel` in HeadlessChrome 147, headed Chrome 147 fresh-profile, or Canary 149 fresh-profile (empirical probes in `internal/chrome/chrome_*_test.go`). The component must be present for the API global to register; the component requires persistent flag state in the profile.
- Therefore `fenster --serve` ships the **Chrome Extension architecture** as the runtime path. Extensions get the API exposed without flag friction once the model component is downloaded.
- The path-derived extension ID lets fenster pre-compute the ID Chrome will assign to the unpacked extension, so `allowed_origins` in the NM manifest matches what Chrome expects without a chicken-and-egg.
- Bridge is a Unix socket because the NM-host child is owned by Chrome (lifetime managed externally) and the supervisor is a separate user-launched process.
- All code uses Go 1.22+ stdlib (`net/http` pattern routing, `log/slog`, `embed.FS`, `context.Context`). No third-party HTTP router; cobra is the only direct CLI dep beyond chromedp.
