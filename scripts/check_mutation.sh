#!/usr/bin/env bash
set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT_DIRECTORY="${1:-$REPOSITORY_ROOT/dist/mutation}"
GREMLINS_BINARY="${GREMLINS_BINARY:-gremlins}"

if ! command -v "$GREMLINS_BINARY" >/dev/null 2>&1; then
  echo "ERROR: Gremlins v0.6.0 must be installed before running mutation checks." >&2
  echo "Install with: go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIRECTORY"
cd "$REPOSITORY_ROOT"

run_mutation() {
  name="$1"
  package="$2"
  shift 2
  "$GREMLINS_BINARY" unleash "$package" \
    --workers 2 \
    --timeout-coefficient 100 \
    --threshold-efficacy 100 \
    --threshold-mcover 100 \
    --output "$OUTPUT_DIRECTORY/$name.json" \
    --output-statuses lc \
    "$@"
  jq -e '
    .mutants_total > 0 and
    .mutants_lived == 0 and
    .mutants_not_covered == 0 and
    ([.files[].mutations[] |
      select(.status != "KILLED" and .status != "NOT_VIABLE")] | length) == 0
  ' "$OUTPUT_DIRECTORY/$name.json" >/dev/null
}

run_mutation verdict ./internal/verdict
run_mutation compiler ./internal/compiler

provider_exclusions=()
while IFS= read -r source; do
  base="$(basename "$source")"
  provider_exclusions+=(--exclude-files "^${base//./\\.}$")
done < <(find internal/provider -maxdepth 1 -name '*.go' ! -name '*_test.go' ! -name 'boundary.go' -print | sort)
run_mutation boundary ./internal/provider "${provider_exclusions[@]}"

run_mutation repair ./internal/repair

echo "Mutation checks completed with no live covered mutants."
