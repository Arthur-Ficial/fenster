# fenster - Project Instructions

**The free AI already in your browser, served as if it were OpenAI.** This is our claim. Every surface (README, landing page, repo description) must reinforce it.

## The Golden Goal

fenster exposes Chrome's on-device Gemini Nano via the Prompt API, behind the same OpenAI-compatible HTTP server that apfel exposes. **Two things are the product. One thing is a byproduct.**

### Core product (this is what fenster IS)

1. **UNIX tool** (`fenster "prompt"`, `echo "text" | fenster`, `fenster --stream`)
   - Pipe-friendly, composable, correct exit codes
   - Works with `jq`, `xargs`, shell scripts
   - `--json` output for machine consumption
   - Respects `NO_COLOR`, `--quiet`, stdin detection

2. **OpenAI API-compatible HTTP server** (`fenster --serve`)
   - Drop-in replacement for `openai.OpenAI(base_url="http://localhost:11434/v1")`
   - `/v1/chat/completions` (streaming + non-streaming)
   - `/v1/models`, `/health`, tool calling, `response_format`
   - Honest 501s for unsupported features (embeddings, legacy completions)
   - CORS for browser clients
   - **Same wire format as apfel.** Verified by running apfel's integration test suite verbatim against fenster (see "The apfel-compat gate" below).

These two modes are what the README.md leads with. Every design decision, test, and release gate is scored against them first.

### Byproducts (useful, but not the pitch)

3. **Interactive mini TUI chat** (`fenster --chat`) — **a byproduct for quick testing, not a main product.**
   - Ships because the pieces are already there (Session, ContextManager, tool calling)
   - Handy for quick testing a prompt or a local MCP server without writing a client
   - Should not dominate README real-estate; a short Quick Start entry is enough

### README.md structure rule

The README.md mirrors this priority — **violating this structure is a bug.**

- Hero + tagline: UNIX tool and OpenAI-compatible server only
- "What it is" table: **two rows** (UNIX tool, OpenAI server). Nothing else.
- Right after the table: a one-command "Try it right away: `fenster --chat`" pointer. Rationale: chat is not the main product, but it is the lowest-friction way for a new user to verify install and see fenster responding.
- Quick Start: UNIX tool first, server second, chat gets a short subsection
- Reference Docs: links to platform matrix and architecture notes

### Non-negotiable principles

- **On-device.** No cloud, no API keys, no network for inference after the initial Gemini Nano download. Ever.
- **Honest about limitations.** Chrome required. ~3B model. No embeddings. Tool calling is faked via `responseConstraint` — say so clearly.
- **Clean code, clean logic.** No hacks. Proper error types. Real token counts (or honest estimates with `_estimated: true`).
- **Modern Go.** Go 1.22+. Stdlib-first. No third-party HTTP router. `log/slog` for logging. `embed.FS` for the extension. `context.Context` everywhere.
- **Usable security.** Secure defaults that don't get in the way. CORS off by default. Bind to `127.0.0.1` by default. No bearer tokens over `http://` external interfaces.
- **TDD always, red-to-green, 100%.** No production code without a failing test first. Write the test, watch it fail for the right reason, write the minimal code to pass, watch it go green. No exceptions, no "I'll add tests after", no "this is too simple to test". Behavior-preserving refactors are covered by existing tests; new behavior gets a new failing test first.

### Documentation style

- **Links in docs and README:** Always use the URL/path as the anchor text, not generic phrases like "full guide" or "click here". Example: `[docs/native-messaging.md](docs/native-messaging.md)` not `[full guide](docs/native-messaging.md)`.

## The apfel-compat gate

fenster's load-bearing acceptance criterion: **the entire apfel integration test suite, vendored verbatim, must pass against fenster's binary.**

- The apfel pytest suite lives at `Tests/integration/`. It was copied from `Arthur-Ficial/apfel` and patched only at one point: `conftest.py` spawns `bin/fenster` instead of `.build/release/apfel`.
- The suite is transport-agnostic — it talks HTTP/SSE/JSON to `localhost:11434` and `localhost:11435`. It does not care which language wrote the server. If fenster's wire format diverges, a test breaks.
- Apfel-specific tests that don't apply (`test_apfelcore_*`, `test_brew_service`, `test_nixpkgs_bump`) are excluded by name in `Tests/integration/conftest.py::collect_ignore`.
- **Skipping a test is a critical error.** `pytest.skip()` calls in the suite are forbidden in green state. The release gate counts skipped tests as failures.
- When the upstream apfel suite changes, we re-vendor (`scripts/port-apfel-tests.sh`) and accept any new red tests as work.

## Architecture

```
CLI (single/stream/chat) ──┐
                           ├─→ HTTP server (/v1/*)  ──┐
                                                       ├─→ NM stdio (4-byte LE prefix + JSON)
                                                       ├─→ Chrome extension service worker
                                                       ├─→ LanguageModel.create() / promptStreaming()
                                                       └─→ Gemini Nano (GPU) — inside headless Chrome
```

- `internal/server/` — HTTP/SSE; pure, testable in-process via `httptest`
- `internal/chrome/` — supervisor (locate, spawn, watch, restart)
- `internal/nm/` — Native Messaging framing (4-byte little-endian length prefix + UTF-8 JSON)
- `internal/extension/` — `embed.FS` for the JS extension shipped inside the binary
- `internal/manifest/` — per-OS native messaging manifest installer
- Tests: `Tests/integration/` (apfel pytest, vendored) + `Tests/go/` (Go-native, fenster-specific)
- No Chrome on the build host? Build still works. `make test` requires Chrome.

## Current Status

- Version: `0.0.1` (source of truth: `.version`)
- Tests: TBD Go unit + ~220 integration (vendored from apfel; RED at M0)
- Distribution: planned — Arthur-Ficial/homebrew-tap (M5), Scoop (M5), apt (M5)
- Stability policy: planned for v1.0
- Security policy: planned for v0.5

## Build & Test

```bash
make test                      # BUILD + ALL TESTS (Go unit + apfel-compat integration) — the one command
make install                   # build release + install to /usr/local/bin (NO version bump)
make build                     # build release only (NO version bump)
make version                   # print current version
go build ./...                 # debug build
go test ./...                  # Go unit tests only
make preflight                 # full release qualification (unit + integration + policy checks)
```

`make test` builds the release binary, runs all Go unit tests, starts test servers (`fenster --serve --port 11434` plain, `fenster --serve --port 11435 --mcp mcp/calculator/server.py`), runs the entire `Tests/integration/` pytest suite, and cleans up. This is the single command for development.

`make install` auto-unlinks any Homebrew fenster so the dev binary takes PATH priority. `make uninstall` restores the Homebrew link.

**Version is in `.version` file** (single source of truth). Local builds (`make build`, `make install`) do NOT change the version. Only the release workflow (`make release`) bumps versions. **Never manually edit `.version`, `internal/buildinfo/buildinfo.go`, or the README badge** — these are updated atomically by the release workflow.

## Key Files

| Area | Files |
|------|-------|
| Entry point | `cmd/fenster/main.go` |
| CLI commands | `cmd/fenster/*.go` (cobra subcommands) |
| HTTP server | `internal/server/server.go`, `internal/server/chat.go`, `internal/server/chat_stream.go` |
| Chrome supervisor | `internal/chrome/supervisor.go`, `internal/chrome/locate.go`, `internal/chrome/cdp.go` |
| Native Messaging | `internal/nm/framing.go`, `internal/nm/port.go` |
| Extension (embedded) | `internal/extension/embed.go`, `extension/manifest.json`, `extension/service-worker.js` |
| Manifest installer | `internal/manifest/install.go` (+ `_darwin.go`, `_linux.go`, `_windows.go`) |
| Session pool | `internal/session/pool.go`, `internal/session/prefix.go` |
| OpenAI types | `internal/openai/chat.go`, `internal/openai/tools.go`, `internal/openai/stream.go` |
| Tool calling shim | `internal/openai/tools.go` (responseConstraint mapping) |
| Token counting | `internal/tokencount/count.go` |
| Error types | `internal/openai/error.go` |
| Build info | `internal/buildinfo/buildinfo.go` (auto-generated by `make`) |
| Security | `internal/server/cors.go`, `internal/server/validate.go` |
| MCP client | `internal/mcp/client.go`, `internal/mcp/protocol.go`, `internal/mcp/auto_exec.go` |
| MCP calculator | `mcp/calculator/server.py` (vendored from apfel) |
| Tests | `Tests/integration/` (apfel-compat pytest, vendored), `Tests/go/` (fenster-specific Go) |
| Docs | `docs/` (architecture, chrome-flags, native-messaging, platforms, release, tool-calling-guide) |
| Scripts | `scripts/publish-release.sh`, `scripts/release-preflight.sh`, `scripts/post-release-verify.sh`, `scripts/port-apfel-tests.sh`, `scripts/generate-build-info.sh` |

## Handling GitHub Issues

When a new issue comes in, follow this process:

1. **Fetch** the full issue with `gh issue view <n> --repo Arthur-Ficial/fenster --json body,comments,title,author,labels`
2. **Vet** — is it a real bug, valid feature request, or noise?
   - Does it align with the golden goal and non-negotiable principles?
   - Can you reproduce it?
   - Check comments for additional context and links
   - Verify the user's environment against known gotchas: Chrome 138+ required, GPU with >4 GB VRAM (or CPU with ≥16 GB RAM and ≥4 cores), ≥22 GB free disk for Gemini Nano model, GPU-capable host (no `--disable-gpu`), correct OS (Windows 10/11, macOS 13+, Linux desktop, ChromeOS Plus).
3. **Fix** if valid:
   - Write tests first (TDD) for bugs
   - Keep changes minimal and KISS
   - `make install` + run all tests (`go test ./...` + `python3 -m pytest Tests/integration/ -v`)
4. **Release** if code changed — see "Publishing a Release" below
5. **Close** the issue with a friendly, short, truthful comment:
   - What was the problem
   - What was fixed (or why it was closed without a fix)
   - How to update (`brew upgrade fenster` / `scoop update fenster`)
6. **Landing page** (fenster.franzai.com — planned) is a separate Cloudflare Pages project, not in this repo

## Handling Pull Requests

When a PR is opened, follow this process. Scale the rigor to the PR type — docs-only PRs skip the security audit and test coverage steps, code PRs get the full treatment.

**Automated first-responder:** `Arthur-Ficial/fenster` has a Claude Code routine (`.claude/routines/02-pr-auto-review.md`) that runs this entire process on `pull_request.opened` / `pull_request.synchronize` and posts a `COMMENTED` review. The routine cannot `--approve`, cannot merge, cannot run `make test` (no GPU/Chrome on cloud runners), and cannot cut releases. It is a first-pass safety net, not a replacement for human judgement. Franz still merges, Franz still releases — always.

### 1. Fetch everything

```bash
gh pr view <n> --repo Arthur-Ficial/fenster --json title,author,body,state,mergeable,mergeStateStatus,reviews,comments,commits,statusCheckRollup,files,headRefName,headRepositoryOwner
gh pr diff <n> --repo Arthur-Ficial/fenster                             # full diff
gh api repos/Arthur-Ficial/fenster/pulls/<n>/comments                   # inline review comments
git fetch origin pull/<n>/head:pr-<n>-head && git checkout pr-<n>-head # actual tree
```

### 2. Vet the author

- First-time contributor to fenster? (`gh pr list --repo Arthur-Ficial/fenster --state all --author <login>`)
- Legitimate GitHub profile? Check `gh api users/<login>` for public_repos, followers, blog, creation date
- Commit author email matches the GitHub account (spot typo-squatting)
- Any red flags in prior public work

### 3. Classify the PR type

| Type | What it touches | Process depth |
|------|-----------------|---------------|
| **Docs-only** | `docs/**`, `README.md`, `CLAUDE.md` | Factual accuracy, link validity, alignment with golden goal, tone |
| **Test-only** | `Tests/**` | Test quality, no false positives/negatives, actually exercises new behavior |
| **Code: non-network** | `internal/**` (no `net.Listen`, `os/exec`, file I/O outside sandbox) | Full architecture + test coverage + build + tests |
| **Code: network/parsing/auth** | server, NM stdin parsing, manifest installer, OpenAI handlers, auth, URL parsing | **Full security audit** on top of the code-PR process |
| **Build/CI** | `go.mod`, `.github/workflows/**`, `Makefile`, `scripts/**` | Reproducibility check, supply chain (pinned versions), runner safety |

### 4. Read every changed file

No skimming. Use `git show pr-<n>-head:<path>` or read from the checked-out tree. For large PRs, map the changes before diving in: list files, group by concern, read in dependency order.

### 5. Security audit (code PRs, especially network/parsing/auth)

- **Input validation** — URL schemes (reject `file://`, `javascript://`), paths (no directory traversal), JSON (malformed + deeply nested), env vars (empty handling)
- **Native Messaging stdin parsing** — the 4-byte length prefix is untrusted input from a child process. Bound-check (≤ 1 MB host→Chrome / ≤ 4 GB Chrome→host per the protocol), reject lengths that would exceed remaining stdin, never `make([]byte, n)` without a cap.
- **Chrome supervisor flags** — no shell injection in spawned args. Use `os/exec.Command` with `[]string` arg slice, never string concatenation. Validate `--browser=` against an allowlist.
- **Manifest installer** — path traversal in `--browser` value when computing manifest dir; symlink-attack-safe writes (`os.OpenFile` with `O_EXCL|O_CREATE|O_NOFOLLOW`).
- **Authentication** — bearer tokens over HTTPS only, no token echo in logs, no token in `ps aux` (prefer env vars), per-server token scoping
- **TLS** — no cert skipping, no insecure fallback
- **Resource limits** — response size cap (no OOM from malicious server), timeouts (`context.WithTimeout`), concurrent request caps
- **Injection risks** — shell (unquoted `$(...)`), HTTP header (CRLF), JSON-in-string, path
- **Secrets leakage** — `--debug` logs, error messages, crash dumps, test fixtures
- **Secure by default** — opt-in for dangerous features, loud warnings, conservative defaults; `127.0.0.1` bind by default; CORS off by default
- **Concurrency** — no goroutine leaks (every spawn cancellable via context), `sync.Mutex` correctly scoped, race-detector clean (`go test -race ./...`)
- **Supply chain** — new dependencies pinned, scope justified, no `replace` directives, `go.sum` matches

Priority-rank findings:
- **P0** blocks merge (security, data loss, credential leak, regression to previous fix)
- **P1** should fix before merge (correctness, test coverage, architectural consistency)
- **P2** nice to have (code quality, follow-up PR acceptable)

### 6. Architecture review

- Does it fit the golden goal (UNIX tool + OpenAI server + chat)?
- Does it respect the non-negotiable principles (on-device, honest limits, clean code, modern Go, usable security)?
- Does it introduce cross-target dependencies that violate the `internal/server` (pure HTTP) / `internal/chrome` (supervisor) / `internal/nm` (framing) / `cmd/fenster` (CLI wiring) layering?
- Are the existing patterns followed (test harness, error types, context propagation, supervisor lifecycle)?

### 7. Test coverage check (code PRs)

- New flag? Must have happy-path + every validation error test in `cmd/fenster/*_test.go`
- New public API on a pure `internal/openai` or `internal/nm` type? Unit test in the corresponding `*_test.go`
- New network or subprocess surface? Integration test wired into `Tests/integration/` using the existing conftest pattern, or `Tests/go/` with `//go:build integration` — **standalone manual scripts in `mcp/`, `scripts/`, etc. do not count**
- Error tests must check the wrapped error: `if !errors.Is(err, openai.ErrInvalidRequest) { t.Fatalf(...) }` — not just `if err == nil`.

### 8. Build + run tests on the PR branch

```bash
git checkout pr-<n>-head
go vet ./...                                              # must be clean
go test -race ./...                                       # existing unit tests must still pass with race detector
# For code PRs, also:
make install && fenster --serve --port 11434 &
fenster --serve --port 11435 --mcp mcp/calculator/server.py &
sleep 4
python3 -m pytest Tests/integration/ -v                   # must pass, 0 skipped
pkill -f "fenster --serve"
```

### 9. Verify CI on the PR

- `gh pr view <n> --repo Arthur-Ficial/fenster --json statusCheckRollup`
- First-time contributors trigger `action_required` on Actions — the CI run needs manual approval before it executes. Approve it before reviewing so the PR has real CI results to reference.

### 10. Review

Post a structured review via `gh pr review <n> --repo Arthur-Ficial/fenster --request-changes|--approve|--comment --body "..."`:

- **Open with genuine praise** for what works. Reviews that lead with negatives make contributors defensive.
- **Summary table** of findings (P0/P1/P2, severity, area, one-line summary)
- **Each finding** gets its own subsection: exact file:line reference, reproducer where possible, concrete fix with code sample
- **What I verified** section listing what's clean (shows the contributor you actually read everything)
- **Suggested path forward** ranked by minimum-viable-merge vs full fix
- **Credit co-authors** — when landing, use `Co-Authored-By: <Name> <email>` in the merge commit

Do not approve code PRs with P0 findings. For docs-only PRs, a request-changes on a broken link is appropriate. For first-time contributors, err on the side of gentler tone.

### 11. Merge decision

- **Approve + merge** only after: all P0/P1 resolved (or explicitly punted with user's OK), CI green, tests green on the branch locally
- **Squash-merge** by default for clean history. Preserve the contributor's commit messages in the squash body so attribution is intact.
- **Do not release** just because you merged. A merge and a release are separate user decisions — ask first.
- **Close linked issues** via `Closes #N` in the PR body or commit message, otherwise do it manually after merge.

### 12. After merge

- Verify main locally: `git checkout main && git pull --rebase origin main`
- Run the full test suite on the merged commit as a sanity check
- If any follow-up is needed (P2 items punted, new issues surfaced), file them as GitHub issues before moving on
- **Clean up the local PR branch**: `git branch -D pr-<n>-head`

### PR anti-patterns to reject

- No tests for new flags or new behavior
- Standalone test scripts that require manual terminal orchestration (not wired into CI)
- Goroutines spawned without a cancellation path (context plumbing missing)
- `http.DefaultClient` for new network code (shared transport, no timeout)
- Bearer tokens sent over `http://`
- New `os.Exit()` calls in pure parsing functions
- Manual edits to `.version`, `README.md` version badge, or `internal/buildinfo/buildinfo.go` (these are release workflow outputs)
- Merge commits in the PR branch history (prefer rebase and squash)
- Contributor working from their fork's `main` branch instead of a feature branch (cosmetic, but harder to land cleanly)

## Publishing a Release

**MANDATORY: always use the automated workflow.** No manual releases. No exceptions.

### Before releasing

```bash
make preflight
```

This runs the full qualification locally: clean git state, on main, Go unit tests, full `Tests/integration/` pytest suite, policy file checks, version sanity. **Do not release if preflight fails.**

### Release

```bash
make release                    # patch (0.0.1 -> 0.0.2)
make release TYPE=minor         # minor (0.0.x -> 0.1.0)
make release TYPE=major         # major (0.x.y -> 1.0.0)
```

This runs locally (not on GitHub Actions — Linux runners often lack a usable GPU and the headless-Chrome integration tests need one). The script (`scripts/publish-release.sh`) does:

1. Preflight checks (clean tree, on main, up to date with origin)
2. Bumps `.version` (patch/minor/major)
3. Cross-compiles release binaries (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64)
4. Runs ALL Go unit tests with `-race`
5. Runs the FULL apfel-compat suite (`Tests/integration/`)
6. Commits `.version`, `README.md`, `internal/buildinfo/buildinfo.go` and pushes to `main`
7. Creates git tag (`v<version>`) and pushes it
8. Packages tarballs (one per OS/arch) and publishes GitHub Release with changelog
9. Updates the Homebrew tap formula

### After releasing

```bash
./scripts/post-release-verify.sh
```

Verifies: GitHub Release exists with tarballs, git tag exists, `.version` matches, installed binary matches, Homebrew tap formula updated.

### Distribution channels (planned)

fenster will ship through three channels. All pull the same signed tarballs from each GitHub Release.

- **Arthur-Ficial/homebrew-tap** — `brew install Arthur-Ficial/tap/fenster`. Synchronous, pushed as part of `make release`.
- **Scoop** — `scoop install fenster` (Windows). Manifest in a future `Arthur-Ficial/scoop-bucket`.
- **apt** — `.deb` artifact attached to GitHub Release; PPA in a future iteration.
- Emergency Homebrew bump: `brew bump-formula-pr fenster --url=<tarball-url> --sha256=<hash>`

### Do NOT manually

- Run `bump-patch`, `bump-minor`, `bump-major` directly
- Edit `.version`, `internal/buildinfo/buildinfo.go`, or README badge
- Create git tags or run `gh release create`
- Push to the Homebrew tap manually (the workflow handles it)

### Integration test rules

- **Never skip tests.** A skipped test is a critical error.
- Integration tests require two running servers: port 11434 (plain) and port 11435 (with MCP calculator).
- If servers aren't running, tests skip silently — this is NOT acceptable. Always start them.

### Post-release checklist

- [ ] `make preflight` passed before release
- [ ] Publish Release workflow completed green
- [ ] `./scripts/post-release-verify.sh` passed
- [ ] CLAUDE.md version and test counts updated (if changed)

## CI / GitHub Actions

**IMPORTANT: GitHub CI runs only a SUBSET of tests.** GitHub-hosted runners may lack a GPU; Gemini Nano needs GPU acceleration; integration tests that exercise inference cannot run there.

**What GitHub CI runs (automatic, every push/PR):**
- Build (release binary on darwin-arm64, linux-amd64, windows-amd64)
- Go unit tests with `-race`
- Static analysis (`go vet`, `staticcheck`)
- Model-free integration tests (CLI flags, help, version, file handling, manifest install, NM framing roundtrip)

**What GitHub CI CANNOT run (no GPU / no Chrome with model):**
- The bulk of `Tests/integration/` (real model calls)
- MCP tool execution against a live model
- Security tests that send real requests
- Chat mode tests
- Performance / latency-budget tests

**What runs the full suite (local, before every release):**
- `make preflight` or `make release` on a host with Chrome 138+ and a GPU (or 16 GB RAM CPU fallback)
- This is the REAL qualification gate. GitHub CI is a safety net, not the source of truth.

Go 1.22+ required. Release docs: [docs/release.md](docs/release.md)
