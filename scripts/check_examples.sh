#!/usr/bin/env bash
set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEMPORARY_DIRECTORY="$(mktemp -d "${TMPDIR:-/tmp}/intentci-examples.XXXXXX")"
trap 'rm -rf "$TEMPORARY_DIRECTORY"' EXIT

export GOCACHE="${GOCACHE:-$TEMPORARY_DIRECTORY/go-build-cache}"

for command_name in python3 npm cargo javac java; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "ERROR: $command_name is required to validate the language examples" >&2
    exit 1
  fi
done

INTENTCI_BINARY="${INTENTCI_BINARY:-$TEMPORARY_DIRECTORY/intentci}"
if [[ ! -x "$INTENTCI_BINARY" ]]; then
  (cd "$REPOSITORY_ROOT" && go build -trimpath -o "$INTENTCI_BINARY" ./cmd/intentci)
fi

(cd "$REPOSITORY_ROOT/examples/typescript" && npm ci --ignore-scripts)

for language in go python typescript rust java; do
  example="$REPOSITORY_ROOT/examples/$language"
  echo "Validating $language example"
  (
    cd "$example"
    "$INTENTCI_BINARY" compile --strict --output "$TEMPORARY_DIRECTORY/$language-ir.json"
    "$INTENTCI_BINARY" verify --all --no-git --no-cache --format json \
      --output "$TEMPORARY_DIRECTORY/$language-report.json"
  )
done

echo "Validated Go, Python, TypeScript, Rust, and Java examples."
