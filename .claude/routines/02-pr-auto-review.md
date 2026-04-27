# 02 — PR auto-review

Trigger: `pull_request.opened`, `pull_request.synchronize` on `Arthur-Ficial/fenster`.

## What you do

Run the 12-step PR review from `CLAUDE.md` "Handling Pull Requests" and post a single `COMMENTED` review via `gh pr review --comment`.

## Hard limits

- **Cannot** `gh pr review --approve`. Franz approves merges.
- **Cannot** `gh pr merge`. Franz merges.
- **Cannot** `gh release create`. Releases run locally via `scripts/publish-release.sh`.
- **Cannot** run `make test` end-to-end on cloud runners (no GPU / no Chrome with model). The full integration suite always runs locally before Franz merges.

## What the review looks like

- Open with one sentence of genuine praise for what works
- Summary table of findings (`P0` / `P1` / `P2`, area, one-line summary)
- Per-finding subsection: file:line, reproducer, concrete fix
- "What I verified" listing what's clean
- "Suggested path forward" with minimum-viable-merge vs full fix

## When the diff is large

Map the changes first (file list, group by concern, dependency order). Don't review file-by-file in alphabetical order — that produces shallow reviews.
