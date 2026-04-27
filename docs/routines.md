# fenster — Claude Code routines

This document mirrors `apfel/docs/routines.md`. fenster's routines live in `.claude/routines/` and are operationalized via [claude.ai/code/routines](https://claude.ai/code/routines).

## Active routines

| File | Trigger | What it does |
|---|---|---|
| `.claude/routines/_golden-goal.md` | (preamble, included by every routine) | Repo identity, non-negotiables, "what fenster is" — concatenated by hand into each routine's live prompt. |
| `.claude/routines/01-issue-triage.md` | `issues.opened` | Reads the issue, applies labels (area:/type:/priority:/size:), posts a clarifying comment if needed. Cannot close issues. |
| `.claude/routines/02-pr-auto-review.md` | `pull_request.opened`, `pull_request.synchronize` | Runs the 12-step PR review from CLAUDE.md, posts a `COMMENTED` review. Cannot `--approve`, cannot merge, cannot release. |

## Sync workflow

Routines do **not** auto-sync from the repo to claude.ai. When `_golden-goal.md` or any routine changes:

1. `cat .claude/routines/_golden-goal.md .claude/routines/<routine>.md` → the full live prompt
2. Paste into [claude.ai/code/routines](https://claude.ai/code/routines)
3. Save

This is the same flow apfel uses. We accept it as a small manual cost.
