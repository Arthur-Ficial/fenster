#!/usr/bin/env bash
# Post-release sanity check. Stub at M0 — full implementation in M5 (FEN-133).
#
# When fully implemented this script will:
#   1. gh release view v$(cat .version) — exists, tarballs attached
#   2. git tag exists and matches origin
#   3. .version matches latest tag
#   4. installed binary --version matches .version
#   5. Homebrew tap formula updated and synthesizes the right SHA
set -euo pipefail

echo "post-release-verify.sh: stub — full implementation in M5 (FEN-133)"
exit 0
