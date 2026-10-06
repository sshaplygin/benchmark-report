#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "$0")/.." && pwd)
test_dir=$(mktemp -d "${TMPDIR:-/tmp}/benchreport-render.XXXXXX")
trap 'rm -rf "$test_dir"' EXIT
cd "$repo_dir"
go build -o "$test_dir/benchreport" ./cmd/benchreport

for parser in go criterion; do
  case "$parser" in
    go) fixture=go-pr15 ;;
    criterion) fixture=criterion-four-suites ;;
  esac
  run_dir="$test_dir/$parser"
  mkdir "$run_dir"
  for side in base head; do
    "$test_dir/benchreport" normalize --parser "$parser" \
      --manifest "$repo_dir/testdata/captured/$fixture/$side-manifest.json" --out "$run_dir/$side.json"
  done
  "$test_dir/benchreport" compare --base "$run_dir/base.json" --head "$run_dir/head.json" --out "$run_dir/comparison.json"
  "$test_dir/benchreport" render --input "$run_dir/comparison.json" --output-dir "$run_dir/report"

  # Move the bundle and remove the calculation files. Rendering from its verified
  # inventory must work offline and produce the same report bytes.
  mv "$run_dir/report" "$test_dir/$parser-bundle"
  rm -r "$run_dir"
  cd "$test_dir/$parser-bundle"
  "$test_dir/benchreport" render --input replay-inputs/comparison.json \
    --config replay-inputs/configuration.json --reproduction reproduction.json --output-dir replayed
  cmp report.md replayed/report.md
  cmp report.json replayed/report.json

  # A syntactically valid modification still fails the recorded checksum.
  printf '\n' >> replay-inputs/comparison.json
  if "$test_dir/benchreport" render --input replay-inputs/comparison.json \
    --config replay-inputs/configuration.json --reproduction reproduction.json --output-dir rejected \
    > "$test_dir/stdout.txt" 2> "$test_dir/stderr.txt"; then
    printf 'Replay must reject changed comparison bytes.\n' >&2
    exit 1
  else
    test "$?" -eq 1
  fi
  test -s "$test_dir/stderr.txt"
  test ! -e rejected/report.md
  test ! -e rejected/report.json

  cd "$repo_dir"
  go run ./cmd/contractcheck schemas/presentation.schema.json "$test_dir/$parser-bundle/report.json"
  go run ./cmd/contractcheck schemas/reproduction.schema.json "$test_dir/$parser-bundle/reproduction.json"
done
printf 'Rendering and verified offline replay checks passed for both consumers.\n'
