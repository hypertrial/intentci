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
  TIME_OUTPUT="$TEMPORARY_DIRECTORY/time.txt"
  if [[ "$(uname -s)" == "Darwin" ]]; then
    /usr/bin/time -l "$TEMPORARY_DIRECTORY/intentci" version > /dev/null 2> "$TIME_OUTPUT"
    PEAK_RSS_BYTES="$(awk '/maximum resident set size/ { print $1; exit }' "$TIME_OUTPUT")"
  else
    /usr/bin/time -v "$TEMPORARY_DIRECTORY/intentci" version > /dev/null 2> "$TIME_OUTPUT"
    PEAK_RSS_KIB="$(awk -F: '/Maximum resident set size/ { gsub(/ /, "", $2); print $2; exit }' "$TIME_OUTPUT")"
    PEAK_RSS_BYTES="$((PEAK_RSS_KIB * 1024))"
  fi
  if [[ ! "$PEAK_RSS_BYTES" =~ ^[0-9]+$ ]]; then
    echo "ERROR: unable to measure peak resident memory" >&2
    exit 1
  fi
  {
    echo "IntentCI v1 performance record"
    echo "generated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "platform=$(go env GOOS)/$(go env GOARCH)"
    echo "go_version=$(go version)"
    echo "commit=$(git rev-parse HEAD)"
    echo "binary_bytes=$(wc -c < "$TEMPORARY_DIRECTORY/intentci" | tr -d ' ')"
    echo "version_peak_rss_bytes=$PEAK_RSS_BYTES"
    echo
    INTENTCI_BENCHMARK_BINARY="$TEMPORARY_DIRECTORY/intentci" \
      go test ./internal/compiler ./internal/impact ./internal/verdict ./internal/executor \
        ./tests/performance -run '^$' -bench '^BenchmarkV1' -benchmem -count=5
  } > "$OUTPUT_PATH"
)

cat "$OUTPUT_PATH"
