#!/usr/bin/env bash
# One-command release. Stub at M0 — full implementation in M5 (FEN-132).
#
# When fully implemented this script will:
#   1. Run release-preflight.sh
#   2. Bump .version (patch/minor/major from $1)
#   3. Cross-compile binaries for darwin/arm64, darwin/amd64, linux/amd64,
#      linux/arm64, windows/amd64
#   4. Run all Go unit tests with -race
#   5. Run the FULL Tests/integration/ pytest suite
#   6. Commit .version + buildinfo.go + README.md and push to main
#   7. Create git tag vX.Y.Z and push
#   8. Package per-OS tarballs
#   9. gh release create with all tarballs and changelog
#  10. Update Arthur-Ficial/homebrew-tap formula
set -euo pipefail

TYPE="${1:-patch}"
echo "publish-release.sh: stub — full implementation in M5 (FEN-132)"
echo "Requested release type: ${TYPE}"
echo "Manual release path is not supported — wait for M5."
exit 1
