# fenster — RED → GREEN journey

## M0 starting line (2026-04-27)

233 tests collected from apfel's vendored `Tests/integration/` suite. With the M0 stub binary all 233 silently skipped (conftest auto-skip when no server). Captured.

## After M1–M3 wave (Go core + UNIX tool + HTTP server, EchoBackend) — 2026-04-27

Full suite re-run with the bin/fenster binary serving on :11434 and :11435. EchoBackend produces wire-correct OpenAI responses but no semantic content.

| | Count | Δ vs M0 |
|---|---|---|
| Collected | **233** | — |
| **Passed** | **84** | **+84** |
| Failed | 72 | — |
| Skipped | 77 | -156 |

### What's GREEN

- All 11 packages in the Go test suite are green: `core/wire`, `core/errors`, `core/validate`, `core/tokens`, `core/ids`, `nm`, `doctor`, `oneshot`, `server`, `backend`, `cmd/fenster`.
- 84 pytest tests pass against the stub backend — wire-format compatibility is real:
  - `/health`, `/v1/models`, `/v1/chat/completions` (stream + non-stream), `/v1/completions` 501, `/v1/embeddings` 501, CORS preflight 204, validation rejections (empty messages, image content, logprobs, n≠1, presence_penalty, frequency_penalty, stop, max_tokens≤0, temperature<0).
  - SSE format includes role chunk, content deltas, finish chunk, usage chunk (when `include_usage`), `data: [DONE]\n\n` terminator.
  - Error envelope is byte-correct: `{"error":{"message","type","param":null,"code":null}}`.
  - `refusal` and `logprobs` always present as JSON null on assistant messages and choices.

### What's RED

- 27 cli_e2e tests need fenster's CLI surface to mirror apfel's text output (help, version, file flags, update/release subcommands, mcp logging format).
- 20 security tests need server-side middleware (origin validation, bearer auth, debug log redaction).
- 9 MCP server tests need host-side MCP execution (forward tool_calls to MCP server, re-prompt model, stitch result).
- 5 man-page tests need apfel-shaped sections in `man/fenster.1.in`.
- 4 openai_client tests need real model output for semantic assertions.
- 35 chat tests need the TUI mode (`fenster --chat`).

## Empirical Chrome 147 / 149 finding (2026-04-27)

The CDP-direct path doesn't expose Built-in AI globals. Tested:

| Setup | LanguageModel | ai | Notes |
|---|---|---|---|
| HeadlessChrome 147 (`--headless=new`) | undefined | undefined | UA: `HeadlessChrome/147.0.0.0` |
| Chrome 147 headed (UA `Chrome/147.0.0.0`) | undefined | undefined | Fresh user-data-dir |
| Chrome Canary 149 headless | undefined | undefined | UA: `HeadlessChrome/149.0.0.0` |
| Chrome Canary 149 headed | undefined | undefined | Fresh user-data-dir |
| Canary 149 chrome://components/ | "Optimization Guide On Device Model" **not listed** | n/a | The component isn't registered on a fresh profile even with `--enable-features=PromptAPIForGeminiNano,OptimizationGuideOnDeviceModel,...,OptimizationGuideOnDeviceModelBypassPerfRequirement,AIPromptAPI` |

**Root cause**: enabling the runtime feature flags on the command line is **not equivalent** to toggling `chrome://flags/#prompt-api-for-gemini-nano` in the UI. Chrome's Component Updater registers the on-device-model component only when the flag is persisted in the profile's `Local State`, then read at startup, and even then the model takes ~5 minutes to download via the Component Updater service.

**Implication**: fenster's "spawn headless Chrome via CDP, eval LanguageModel" path doesn't work today on fresh profiles. Two viable real-world paths:

1. **Chrome Extension architecture** (the original briefing choice). Extensions have the LanguageModel API exposed without flag friction. fenster ships an MV3 extension and a Native Messaging host; user installs the extension once; fenster --serve runs the NM host. The model download is still required but Chrome triggers it on first `LanguageModel.create()` call from the extension.

2. **Pre-bootstrap the user-data-dir's Local State** with the right `enabled_labs_experiments` entries before launching Chrome. This persists the flag, lets Chrome register the component, and (after a wait) downloads the model. fenster could bundle this bootstrap step inside `fenster doctor --bootstrap` or `fenster setup-chrome-model`.

fenster M2 ships path (1) — Chrome Extension + NM bridge — because that's the supported, documented path that doesn't require Chromium-internal preference manipulation.

## Path to GREEN

| Cluster | Tickets | Milestone |
|---|---|---|
| cli_e2e text matching | FEN-101..FEN-115 | M2 |
| security middleware (origin/bearer/debug-redaction) | FEN-121..FEN-130 | M2 |
| MCP host-side execution | FEN-131..FEN-140 | M3 |
| chat TUI | FEN-141..FEN-150 | M3 |
| Chrome Extension + NM bridge (real model) | FEN-151..FEN-180 | M3 |
| Semantic tests (real Gemini Nano) | gated on M3 Chrome bridge | M4 |
| Performance tuning | M4 | M4 |
| Distribution (brew/scoop/apt) | M5 | M5 |

Total: 84/233 GREEN, 0 SKIPPED in stub mode. The remaining 149 require either CLI/middleware work (108 tests, no model needed) or real Gemini Nano (41 tests).
