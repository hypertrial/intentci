#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

OUT="${COVERPROFILE:-coverage.out}"
go test ./... -covermode=atomic -coverprofile="$OUT"
total="$(go tool cover -func="$OUT" | awk '/^total:/{print $3}')"
echo "total coverage: ${total}"
if [[ "$total" != "100.0%" ]]; then
  echo "ERROR: expected 100.0% statement coverage, got ${total}" >&2
  go tool cover -func="$OUT" | awk '$3 != "100.0%" && $1 != "total:"'
  exit 1
fi
