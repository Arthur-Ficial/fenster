#!/usr/bin/env bash
# Vendors apfel's integration test suite into Tests/integration/, patching
# only what's strictly required for fenster compatibility:
#
#   - conftest.py BINARY = ROOT / "bin" / "fenster"  (apfel uses .build/release/apfel)
#   - collect_ignore for apfel-only tests (test_apfelcore_*, test_brew_service,
#     test_nixpkgs_bump) so pytest doesn't even try to import them
#
# The test files themselves are NOT modified. fenster's job is to match apfel's
# wire format byte-for-byte; any test divergence we want to know about.
#
# Re-runnable, idempotent. Run after every apfel release that updates Tests/integration/.

set -euo pipefail

cd "$(dirname "$0")/.."

APFEL_DIR="${APFEL_DIR:-/Users/arthurficial/dev/apfel}"
SRC="$APFEL_DIR/Tests/integration"
DEST="Tests/integration"

if [[ ! -d "$SRC" ]]; then
  echo "error: apfel test source not found at $SRC" >&2
  echo "       set APFEL_DIR or clone apfel next to fenster." >&2
  exit 1
fi

mkdir -p "$DEST"

echo "Vendoring apfel integration tests from $SRC -> $DEST"

# Copy everything except __pycache__ and any apfel-build artefacts.
rsync -a --delete \
  --exclude='__pycache__' \
  --exclude='*.pyc' \
  --exclude='.pytest_cache' \
  "$SRC/" "$DEST/"

# Patch conftest.py for fenster.
CONFTEST="$DEST/conftest.py"
if [[ ! -f "$CONFTEST" ]]; then
  echo "error: $CONFTEST missing after rsync" >&2
  exit 1
fi

# 1. Point BINARY at fenster's bin/ output.
sed -i.bak \
  -e 's|BINARY = ROOT / ".build" / "release" / "apfel"|BINARY = ROOT / "bin" / "fenster"|' \
  "$CONFTEST"
rm -f "$CONFTEST.bak"

# 2. Add collect_ignore for apfel-specific tests if not already present.
if ! grep -q '^collect_ignore' "$CONFTEST"; then
  cat >> "$CONFTEST" <<'PY'

# fenster: apfel-specific tests that don't apply (added by scripts/port-apfel-tests.sh).
collect_ignore = [
    "test_apfelcore_examples.py",
    "test_apfelcore_package.py",
    "test_brew_service.py",
    "test_nixpkgs_bump.py",
]
PY
fi

# Drop the ignored files outright so they don't show up in `find`.
for f in test_apfelcore_examples.py test_apfelcore_package.py test_brew_service.py test_nixpkgs_bump.py; do
  rm -f "$DEST/$f"
done

echo "Vendor complete."
echo "Patched: $CONFTEST"
echo "Ignored: test_apfelcore_*, test_brew_service, test_nixpkgs_bump"

if command -v python3 >/dev/null 2>&1; then
  echo ""
  echo "Test collection:"
  python3 -m pytest "$DEST" --collect-only -q 2>/dev/null | tail -5 || true
fi
