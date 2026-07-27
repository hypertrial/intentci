#!/usr/bin/env bash
set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT_DIRECTORY="${1:-$REPOSITORY_ROOT/dist/release-evidence}"
if [[ "$OUTPUT_DIRECTORY" != /* ]]; then
  OUTPUT_DIRECTORY="$REPOSITORY_ROOT/$OUTPUT_DIRECTORY"
fi
TEMPORARY_DIRECTORY="$(mktemp -d "${TMPDIR:-/tmp}/intentci-release.XXXXXX")"
trap 'rm -rf "$TEMPORARY_DIRECTORY"' EXIT

mkdir -p "$OUTPUT_DIRECTORY"
export GOCACHE="${GOCACHE:-$TEMPORARY_DIRECTORY/go-build-cache}"

cd "$REPOSITORY_ROOT"
go vet ./...
go test -race ./...
./scripts/check-coverage.sh
./scripts/check_schemas.sh
./scripts/check_examples.sh
INTENTCI_ACCEPTANCE_OUTPUT="$OUTPUT_DIRECTORY/acceptance-v1.json" \
  go test ./tests/acceptance -run '^TestV1Acceptance$' -v
INTENTCI_BUILD_VERSION="${INTENTCI_BUILD_VERSION:-1.1.0-dev}" \
  ./scripts/cross_compile.sh "$TEMPORARY_DIRECTORY/cross-compile"
./scripts/record_performance.sh "$OUTPUT_DIRECTORY/performance.txt"

if ! grep -q '"all_passed": true' "$OUTPUT_DIRECTORY/acceptance-v1.json"; then
  echo "ERROR: v1 acceptance matrix is not fully green" >&2
  exit 1
fi

if [[ "${INTENTCI_RUN_MUTATION:-0}" == "1" ]]; then
  ./scripts/check_mutation.sh "$OUTPUT_DIRECTORY/mutation"
fi

echo "IntentCI v1 release validation passed."
