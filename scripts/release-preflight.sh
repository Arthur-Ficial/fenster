#!/usr/bin/env bash
# Pre-release qualification gate. Stub at M0 — full implementation in M5 (FEN-131).
#
# When fully implemented this script will:
#   1. Verify clean git working tree
#   2. Verify on `main` branch and up to date with origin/main
#   3. Run `go vet ./...` clean
#   4. Run `go test -race ./...` (all Go unit tests pass)
#   5. Run `python3 -m pytest Tests/integration/ -v` (apfel-compat suite green, 0 skipped)
#   6. Verify CLAUDE.md present and >300 lines
#   7. Verify .version sane and matches buildinfo.go
#   8. Verify man page generates clean (mandoc lint)
#   9. Verify GitHub repo reachable
set -euo pipefail

echo "release-preflight.sh: stub — full implementation in M5 (FEN-131)"
echo "Today: please run 'go test -race ./...' and 'make test' manually."
exit 0
