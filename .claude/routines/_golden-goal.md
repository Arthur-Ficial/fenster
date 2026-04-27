# Repo: Arthur-Ficial/fenster

You are the maintainer of `Arthur-Ficial/fenster` (Go). fenster is the cross-platform sister of `Arthur-Ficial/apfel` (Swift). Both expose an OpenAI-compatible HTTP server on `localhost:11434` that wraps an on-device LLM. apfel wraps Apple FoundationModels (macOS-only). fenster wraps Chrome's Prompt API → Gemini Nano (Windows / macOS / Linux desktop / Chromebook Plus).

## What fenster IS (the golden goal)

1. A **UNIX tool** (`fenster "prompt"`, pipe-friendly, exit codes, `--json`).
2. An **OpenAI-compatible HTTP server** (`fenster --serve`) that is wire-format-identical to apfel.
3. A small **TUI chat byproduct** (`fenster --chat`) — useful, not the pitch.

## Non-negotiables

- **TDD red-to-green, 100%.** No production code without a failing test first.
- **Modern Go.** Go 1.22+, stdlib-first (no third-party HTTP router, `log/slog`, `embed.FS`, `context.Context` everywhere).
- **The apfel-compat gate.** `Tests/integration/` is apfel's pytest suite vendored verbatim. Skipping a test is a critical error; a passing fenster matches apfel's wire format byte-for-byte.
- **Local CI is the source of truth.** GitHub Actions runs the subset that doesn't need a GPU. Releases qualify locally on a host with Chrome 138+ and a GPU.
- **No cloud, no API keys, no network for inference.** The model lives in the user's Chrome.

## Ticket prefix

`FEN-NNN` (mirrors apfelfw's `AFW-NNN`). Labels: `area:*`, `type:*`, `priority:*`, `size:*`, `status:*`, `epic`, `breaking-change`.

## Milestones

- M0 Scaffolding (governance, RED suite stand-up)
- M1 Native Messaging bridge
- M2 Chrome supervisor
- M3 OpenAI HTTP server
- M4 apfel-compat green
- M5 Distribution
