# fenster — RED baseline (M0)

**Captured:** 2026-04-27
**Binary:** `bin/fenster` (M0 stub — `--serve` exits 2 with "not implemented")
**Apfel suite vendor commit:** initial vendor from `/Users/arthurficial/dev/apfel/Tests/integration/` on 2026-04-27

## Summary

| | Count |
|---|---|
| Collected | **233** |
| Passed | 0 |
| Failed | 0 |
| Skipped | **233** |
| Errors | 0 |

## Why everything skipped (and not failed)

Apfel's `conftest.py` defensively skips tests when the server can't be started (`pytest.skip("Could not start apfel server on port 11434")`). At M0, fenster's `--serve` flag exits 2 immediately, so the fixture's health-poll loop times out and triggers `pytest.skip` for the whole session.

**This is the antipattern called out in CLAUDE.md "Integration test rules":**

> "If servers aren't running, tests skip silently — this is NOT acceptable. Always start them."

We accept this as the M0 baseline because we don't have a server yet. From M3 onwards, the conftest skip path will be **removed** so the suite fails loudly when the server isn't healthy.

## Per-file collection counts

```
Tests/integration/cli_e2e_test.py
Tests/integration/mcp_remote_test.py
Tests/integration/mcp_server_test.py
Tests/integration/openai_client_test.py
Tests/integration/openapi_conformance_test.py
Tests/integration/openapi_spec_test.py
Tests/integration/performance_test.py
Tests/integration/security_test.py
Tests/integration/test_build_info.py
Tests/integration/test_chat.py
Tests/integration/test_man_page.py
```

(Files removed by `collect_ignore` in conftest.py: `test_apfelcore_examples.py`, `test_apfelcore_package.py`, `test_brew_service.py`, `test_nixpkgs_bump.py` — apfel-only Swift library / distribution channels that don't apply to fenster.)

## Reproduce

```bash
make build
python3 -m pytest Tests/integration/ --tb=no -q --no-header --timeout=20
```

## Path to GREEN

| Cluster | First green expected | Tickets |
|---|---|---|
| `/health` (smoke) | M3 | (file in FEN-011) |
| `/v1/models` (trivial) | M3 | |
| `cli_e2e` (no model needed for most) | M2 | |
| `chat completions non-streaming` | M3 | |
| `chat completions streaming SSE` | M3 | |
| validation 400/501 errors | M3 | |
| usage block | M3 | |
| refusal field | M3 | |
| JSON mode (responseConstraint) | M3 | |
| tool calling (faked via responseConstraint) | M3-M4 | |
| MCP server mode (host-side execution) | M4 | |
| MCP remote | M4 | |
| security (CORS, origin, bearer) | M3-M4 | |
| chat TUI (`fenster --chat`) | M4 | |
| performance (latency budgets) | M4 | |
| `test_build_info.py` (version flag, env) | M2 | |
| `test_man_page.py` (man page coverage) | M2 | |

The detailed sub-tickets are filed under FEN-011 once we read each test cluster carefully and break it apart.

## Discipline

This is the start line. Every commit from M1 onwards either:

1. Drives a test from skipped → passed (good), or
2. Drives a test from skipped → failed-loudly (also good — exposes wire-format gaps), or
3. Doesn't touch the test count (refactor, tooling, docs) and explains why in the PR.

Skipping more tests is never OK.
