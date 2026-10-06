#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "$0")/.." && pwd)
fixture_dir="$repo_dir/testdata/captured"
test_dir=$(mktemp -d "${TMPDIR:-/tmp}/benchreport-normalize.XXXXXX")
trap 'rm -rf "$test_dir"' EXIT

cd "$repo_dir"
go build -o "$test_dir/benchreport" ./cmd/benchreport
mkdir "$test_dir/out"

# Resolve inputs from their manifest and outputs from their own directory, even
# when the binary is invoked outside the checkout.
cd "$test_dir"
"$test_dir/benchreport" normalize --parser go \
  --manifest "$fixture_dir/go-pr15/base-manifest.json" --out out/go.json
"$test_dir/benchreport" normalize --parser criterion \
  --manifest "$fixture_dir/criterion-four-suites/head-manifest.json" --out out/criterion.json
cp out/go.json out/first-go.json
"$test_dir/benchreport" normalize --parser go \
  --manifest "$fixture_dir/go-pr15/base-manifest.json" --out out/go.json
cmp out/first-go.json out/go.json

expect_input_failure() {
  local result
  if "$test_dir/benchreport" "$@" >stdout.txt 2>stderr.txt; then
    printf 'Expected input failure: %s\n' "$*" >&2
    exit 1
  else
    result=$?
  fi
  test "$result" -eq 1
  test -s stderr.txt
  test ! -e out/invalid.json
}

expect_input_failure normalize --parser go --manifest absent.json --out out/invalid.json
expect_input_failure normalize --parser benchmarkdotnet \
  --manifest "$fixture_dir/go-pr15/base-manifest.json" --out out/invalid.json
expect_input_failure normalize --parser go \
  --manifest "$repo_dir/testdata/handwritten/invalid/missing-suite-manifest.json" --out out/invalid.json

cd "$repo_dir"
go run ./cmd/contractcheck schemas/normalized-run.schema.json \
  "$test_dir/out/go.json" "$test_dir/out/criterion.json"
printf 'Normalization CLI checks passed for both captured consumers.\n'
