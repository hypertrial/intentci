#!/usr/bin/env bash
set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?usage: package_release.sh VERSION [OUTPUT_DIRECTORY]}"
OUTPUT_DIRECTORY="${2:-$REPOSITORY_ROOT/dist/release}"
TEMPORARY_DIRECTORY="$(mktemp -d "${TMPDIR:-/tmp}/intentci-package.XXXXXX")"
trap 'rm -rf "$TEMPORARY_DIRECTORY"' EXIT

mkdir -p "$OUTPUT_DIRECTORY"
for target in linux/amd64 darwin/amd64 darwin/arm64; do
  target_os="${target%/*}"
  target_arch="${target#*/}"
  stage="$TEMPORARY_DIRECTORY/${target_os}_${target_arch}"
  mkdir -p "$stage"
  (
    cd "$REPOSITORY_ROOT"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
      go build -trimpath \
      -ldflags="-s -w -X github.com/hypertrial/intentci/internal/version.Version=$VERSION" \
      -o "$stage/intentci" ./cmd/intentci
  )
  archive="$OUTPUT_DIRECTORY/intentci_${VERSION}_${target_os}_${target_arch}.tar.gz"
  tar --sort=name --owner=0 --group=0 --numeric-owner \
    --mtime='UTC 1970-01-01' -cf - -C "$stage" intentci | gzip -n > "$archive"
done

(
  cd "$OUTPUT_DIRECTORY"
  sha256sum intentci_"$VERSION"_*.tar.gz > checksums.txt
)

echo "Created deterministic v$VERSION release archives and checksums."
