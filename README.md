# fenster

### Run Gemini Nano local on every computer.

[![Version 0.0.1](https://img.shields.io/badge/version-0.0.1-blue)](https://github.com/Arthur-Ficial/fenster)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![macOS / Linux / Windows](https://img.shields.io/badge/desktop-cross--platform-000000?logo=apple&logoColor=white)](https://github.com/Arthur-Ficial/fenster)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![100% On-Device](https://img.shields.io/badge/inference-100%25%20on--device-green)](https://developer.chrome.com/docs/ai/prompt-api)
[![Apfel-compat](https://img.shields.io/badge/wire--format-apfel--compatible-orange)](https://github.com/Arthur-Ficial/apfel)
[![#agentswelcome](https://img.shields.io/badge/%23agentswelcome-PRs%20welcome-0066cc?style=for-the-badge&labelColor=0d1117&logo=probot&logoColor=white)](#contributing)

Chrome ships a built-in LLM via [Gemini Nano](https://developer.chrome.com/docs/ai/prompt-api). `fenster` exposes it as a UNIX tool and a local OpenAI-compatible server. 100% on-device. No API keys, no cloud.

| Mode | Command | What you get |
|------|---------|--------------|
| UNIX tool | `fenster "prompt"` / `echo "text" \| fenster` | Pipe-friendly answers, file attachments, JSON output, exit codes |
| OpenAI-compatible server | `fenster --serve` | Drop-in local `http://localhost:11434/v1` backend for OpenAI SDKs |

`fenster --chat` is an interactive REPL.

Cross-platform sister of [apfel](https://github.com/Arthur-Ficial/apfel). Wire-format compatible. Works on Windows, macOS, Linux, ChromeOS.

## How it works

```
                ┌──────────────────────────────────────┐
                │  fenster (Go daemon)                 │
                │  ┌──────────┐    ┌────────────────┐  │
HTTP client ───>│  │ HTTP/SSE │<──>│ Chrome (CDP)   │  │
(curl, IDE)     │  │ :11434   │    │ headless       │  │
                │  └──────────┘    └───────┬────────┘  │
                └──────────────────────────┼───────────┘
                                           │ spawn (one shared)
                                           v
                ┌──────────────────────────────────────┐
                │  Headless Chrome Canary 149+         │
                │  ┌────────────────┐                  │
                │  │ LanguageModel  │                  │
                │  │  (Prompt API)  │                  │
                │  └───────┬────────┘                  │
                │          v                           │
                │   Gemini Nano (~3B)                  │
                └──────────────────────────────────────┘
```

The Chrome that fenster spawns is invisible. AppKit reports zero windows. `FENSTER_CHROME_HEADED=1` surfaces it for debugging.

## Requirements & Install

Chrome 138+, GPU with >4 GB VRAM (or 16 GB RAM CPU fallback), 22 GB free disk. Building from source needs Go 1.22+.

```bash
go install github.com/Arthur-Ficial/fenster/cmd/fenster@latest
brew install --cask google-chrome@canary    # the Built-in AI Origin Trial requires Canary today
fenster doctor                              # verify your environment
```

`fenster doctor` is honest about what is missing and tells you exactly what to do.

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

`fenster --serve` shares one Chrome instance across every server you start. The first one launches Chrome (about 5 minutes on first ever boot to download the 2.4 GB Gemini Nano model). Subsequent starts attach to the same browser.

### Quick testing chat

`fenster --chat` is a small REPL for testing prompts or MCP servers.

```bash
fenster --chat
fenster --chat -s "You are a helpful coding assistant"
fenster --chat --mcp ./mcp/calculator/server.py      # chat with MCP tools
fenster --chat --debug                                # debug output to stderr
```

Ctrl-C exits. Type `quit` or hit Ctrl-D to exit cleanly.

## Honest status (today, April 2026)

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

## Architecture decisions you should know

These are empirically proven, not aspirational:

1. **Chrome Canary 149+ is required.** Stable Chrome does not expose `LanguageModel` even with `--enable-features=PromptAPIForGeminiNano`. `fenster doctor` will guide you.
2. **Headless mode works.** `--headless=new` plus a bootstrapped `Local State` plus a real `http://127.0.0.1` origin plus `userGesture:true` CDP `Runtime.evaluate` trips the model-download gate.
3. **One shared Chrome per machine** via `~/.fenster/run/chrome.json` lockfile. Many `fenster --serve` instances attach to the same browser. No dialog floods.
4. **Sentinel session reuse plus pre-warm at startup.** First request after `fenster --serve` returns in under 2 seconds because the cold `LanguageModel.create()` tax is paid in the background.
5. **Ctrl-C semantics ported from apfel.** SIGINT at the chat prompt exits 130 with a terminal reset. SIGINT mid-response cancels the request and returns to the prompt.

Performance principles in [docs/architecture.md](docs/architecture.md).

## Build from source

```bash
git clone https://github.com/Arthur-Ficial/fenster
cd fenster
make build              # release binary to bin/fenster
make test-fast          # Go unit + non-model integration in 30 seconds
make test               # full apfel-compat suite, real Gemini Nano, about 5 minutes
```

Modern Go (1.22+), stdlib first. Direct deps: `cobra`, `chromedp`, `term`. No third-party HTTP router.

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

> *"The free AI already in your browser, served as if it were OpenAI."*
