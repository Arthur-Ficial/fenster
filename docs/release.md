# fenster — release process

Stub — full content lands with M5 (FEN-131..FEN-150). For now, see CLAUDE.md "Publishing a Release" for the policy.

Until M5 ships:

1. Do not run `make release` — `scripts/publish-release.sh` is a guard stub that exits 1.
2. `make build` produces `bin/fenster` for local dev.
3. `.version` stays at the M0 value (`0.0.1`) until the first real release.
