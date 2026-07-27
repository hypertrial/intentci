#!/usr/bin/env bash
set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPOSITORY_ROOT"

go test ./pkg/schema

TEMPORARY_DIRECTORY="$(mktemp -d "${TMPDIR:-/tmp}/intentci-schemas.XXXXXX")"
trap 'rm -rf "$TEMPORARY_DIRECTORY"' EXIT

go build -trimpath -o "$TEMPORARY_DIRECTORY/intentci" ./cmd/intentci
for schema in requirement evidence verdict repair ir plan report; do
  "$TEMPORARY_DIRECTORY/intentci" schema "$schema" > "$TEMPORARY_DIRECTORY/$schema.json"
done

echo "Validated 7 embedded v1 JSON schemas."
