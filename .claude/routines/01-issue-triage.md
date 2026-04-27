# 01 — Issue triage

Trigger: `issues.opened` on `Arthur-Ficial/fenster`.

## What you do

1. Read the issue body and any user-supplied environment details.
2. Classify it against the golden goal:
   - Real bug? → `type:bug`, severity-rank with `priority:P0/P1/P2/P3`
   - Feature request aligned with the goal? → `type:feat`, size with `size:XS..XL`
   - Docs gap? → `type:docs`, `area:docs`
   - Off-topic / out-of-scope? → polite comment, recommend the right repo (apfel, ollama, llama.cpp, etc.)
3. Apply `area:*` label by what code path it touches (`area:server`, `area:chrome`, `area:nm`, `area:cli`, etc.).
4. If reproduction info is missing, post a friendly comment listing what's needed (Chrome version, OS, GPU, disk free, exact command, full output). Add `status:needs-design` if it's a feature without a clear behavior spec.
5. If the report violates a non-negotiable (e.g. asks for a cloud fallback) — say so directly, point at CLAUDE.md, close with `wontfix`.

## What you must NOT do

- Close issues that have a real reproducer just because they're inconvenient.
- Apply `epic` to anything other than M0..M5 milestone-closing issues.
- Skip the env check on user reports — most "fenster doesn't work" issues are missing GPU / Chrome version / disk space.

## Tone

Friendly. Specific. Cite filenames. Link to docs/ pages. Never speculate where you can grep or run.
