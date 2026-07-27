#!/usr/bin/env bash
set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT_PATH="${1:-$REPOSITORY_ROOT/dist/performance.txt}"
TEMPORARY_DIRECTORY="$(mktemp -d "${TMPDIR:-/tmp}/intentci-performance.XXXXXX")"
trap 'rm -rf "$TEMPORARY_DIRECTORY"' EXIT

mkdir -p "$(dirname "$OUTPUT_PATH")"
(
  cd "$REPOSITORY_ROOT"
  go build -trimpath -o "$TEMPORARY_DIRECTORY/intentci" ./cmd/intentci
  {
    echo "IntentCI v1 performance record"
    echo "generated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "platform=$(go env GOOS)/$(go env GOARCH)"
    echo "go_version=$(go version)"
    echo "commit=$(git rev-parse HEAD)"
    echo "binary_bytes=$(wc -c < "$TEMPORARY_DIRECTORY/intentci" | tr -d ' ')"
    echo
    INTENTCI_BENCHMARK_BINARY="$TEMPORARY_DIRECTORY/intentci" \
      go test ./internal/compiler ./internal/impact ./internal/verdict ./internal/executor \
        ./tests/performance -run '^$' -bench '^BenchmarkV1' -benchmem -count=5
  } > "$OUTPUT_PATH"
)

cat "$OUTPUT_PATH"
