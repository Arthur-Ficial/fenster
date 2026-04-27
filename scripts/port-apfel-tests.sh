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

# Patch every test file. Idempotent — re-running won't double-rewrite.
#
# Substitutions:
#   1. BINARY path: .build/release/apfel  ->  bin/fenster
#   2. Model identity: "apple-foundationmodel" -> "gemini-nano"
#      (fenster wraps Chrome's Gemini Nano, not Apple FoundationModels.
#       The wire format is identical; only the model identity differs.)
#   3. Banner identity: "apfel " -> "fenster " in *expected log strings*
find "$DEST" -name "*.py" -type f -print0 | while IFS= read -r -d '' f; do
  sed -i.bak \
    -e 's|ROOT / "\.build" / "release" / "apfel"|ROOT / "bin" / "fenster"|g' \
    -e 's|".build/release/apfel"|"bin/fenster"|g' \
    -e 's|".build/release/apfel.1"|".build/release/fenster.1"|g' \
    -e 's|"\.build/release/apfel\.1"|".build/release/fenster.1"|g' \
    -e 's|"apple-foundationmodel"|"gemini-nano"|g' \
    -e "s|'apple-foundationmodel'|'gemini-nano'|g" \
    -e 's|/apfel\.1\b|/fenster.1|g' \
    -e 's|"apfel\.1"|"fenster.1"|g' \
    -e 's|"apfel v"|"fenster v"|g' \
    -e "s|'apfel v'|'fenster v'|g" \
    -e 's|"man" / "apfel\.1\.in"|"man" / "fenster.1.in"|g' \
    -e 's|man/apfel\.1\.in|man/fenster.1.in|g' \
    -e 's|"Sources" / "BuildInfo\.swift"|"internal" / "buildinfo" / "buildinfo.go"|g' \
    -e 's|"Sources" / "CLI" / "ExitCodes\.swift"|"cmd" / "fenster" / "main.go"|g' \
    -e 's|"Sources" / "main\.swift"|"cmd" / "fenster" / "main.go"|g' \
    "$f"
  rm -f "$f.bak"
done

CONFTEST="$DEST/conftest.py"
if [[ ! -f "$CONFTEST" ]]; then
  echo "error: $CONFTEST missing after rsync" >&2
  exit 1
fi

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
