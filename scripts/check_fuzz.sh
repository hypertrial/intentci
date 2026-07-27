#!/usr/bin/env bash
set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FUZZ_TIME="${INTENTCI_FUZZ_TIME:-2s}"

run_fuzz() {
  local package="$1"
  local target="$2"
  go test "$package" -run '^$' -fuzz "^${target}$" -fuzztime "$FUZZ_TIME"
}

cd "$REPOSITORY_ROOT"
run_fuzz ./internal/impact FuzzV1PathMatching
run_fuzz ./internal/compiler FuzzV1DependencyGraphs
run_fuzz ./internal/verdict FuzzV1VerdictAggregation
run_fuzz ./internal/evidence FuzzV1ManifestHashing
run_fuzz ./internal/security FuzzV1Redaction
run_fuzz ./internal/evidence FuzzV1RunIDOrdering
run_fuzz ./internal/ir FuzzV1LogicalExpressionNormalization

echo "IntentCI v1 property fuzz checks passed."
