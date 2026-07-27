#!/usr/bin/env bash
set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT_DIRECTORY="${1:-$REPOSITORY_ROOT/dist/cross-compile}"
VERSION="${INTENTCI_BUILD_VERSION:-1.1.0-dev}"

mkdir -p "$OUTPUT_DIRECTORY"
for target in linux/amd64 darwin/amd64 darwin/arm64; do
  target_os="${target%/*}"
  target_arch="${target#*/}"
  output="$OUTPUT_DIRECTORY/intentci_${target_os}_${target_arch}"
  (
    cd "$REPOSITORY_ROOT"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
      go build -trimpath \
      -ldflags="-X github.com/hypertrial/intentci/internal/version.Version=$VERSION" \
      -o "$output" ./cmd/intentci
  )
done

echo "Cross-compiled Linux amd64 and macOS amd64/arm64 binaries."
