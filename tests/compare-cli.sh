#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "$0")/.." && pwd)
test_dir=$(mktemp -d "${TMPDIR:-/tmp}/benchreport-compare.XXXXXX")
trap 'rm -rf "$test_dir"' EXIT
cd "$repo_dir"
go build -o "$test_dir/benchreport" ./cmd/benchreport
cd "$test_dir"

printf 'pkg: example.test/p\nBenchmarkA-4 1 100 ns/op 0 B/op 0 allocs/op\n' > base.txt
printf 'pkg: example.test/p\nBenchmarkA-4 1 130 ns/op 0 B/op 0 allocs/op\n' > head.txt
for side in base head; do
  cat > "$side-manifest.json" <<EOF
{
  "schema_version": 1,
  "revision": "1111111111111111111111111111111111111111",
  "environment": {"toolchain":"go1.25.0","os":"linux","arch":"amd64","runner":"synthetic"},
  "expected_suites": ["go"],
  "suites": [{"id":"go","parser":{"name":"go","version":"1"},"command":"synthetic fixture","files":["$side.txt"]}]
}
EOF
  "$test_dir/benchreport" normalize --parser go --manifest "$side-manifest.json" --out "$side.json"
done

# A calculation from saved normalized runs needs neither raw logs nor benchstat.
rm base.txt head.txt
"$test_dir/benchreport" compare --base base.json --head head.json --out default-comparison.json
cat > gate.json <<'EOF'
{"schema_version":1,"comparison":{"fail_on_regression":true},"report":{"metrics":["allocations"],"include":["no-displayed-rows"]}}
EOF
"$test_dir/benchreport" config validate --config gate.json
if "$test_dir/benchreport" compare --base base.json --head head.json --config gate.json --out gated-comparison.json; then
  printf 'An undisplayed timing regression must fail the enabled gate.\n' >&2
  exit 1
else
  test "$?" -eq 2
fi
test -s gated-comparison.json

printf '{"schema_version":1,"comparison":{"fail_on_regression":false,"fail_on_regression":true}}\n' > invalid-config.json
if "$test_dir/benchreport" config validate --config invalid-config.json > stdout.txt 2> stderr.txt; then
  printf 'Duplicate policy keys must fail validation.\n' >&2
  exit 1
else
  test "$?" -eq 1
fi
test -s stderr.txt

# Output paths must not destroy either program used to produce the artifact.
cp "$test_dir/benchreport" initial-benchreport
if "$test_dir/benchreport" compare --base base.json --head head.json --out "$test_dir/benchreport" > stdout.txt 2> stderr.txt; then
  printf 'Comparison output must not overwrite its own executable.\n' >&2
  exit 1
else
  test "$?" -eq 1
fi
cmp initial-benchreport "$test_dir/benchreport"

if [[ -n "${BENCHREPORT_TEST_BENCHSTAT:-}" ]]; then
  cp "$BENCHREPORT_TEST_BENCHSTAT" protected-benchstat
  printf 'pkg: example.test/p\nBenchmarkA-4 1 100 ns/op 0 B/op 0 allocs/op\n' > base.txt
  printf 'pkg: example.test/p\nBenchmarkA-4 1 130 ns/op 0 B/op 0 allocs/op\n' > head.txt
  printf '{"schema_version":1,"comparison":{"statistics":"benchstat"}}\n' > statistics.json
  if "$test_dir/benchreport" compare --base base.json --head head.json --config statistics.json \
    --benchstat-path "$test_dir/protected-benchstat" --out "$test_dir/protected-benchstat" > stdout.txt 2> stderr.txt; then
    printf 'Comparison output must not overwrite benchstat.\n' >&2
    exit 1
  else
    test "$?" -eq 1
  fi
  cmp "$BENCHREPORT_TEST_BENCHSTAT" protected-benchstat
fi

cd "$repo_dir"
go run ./cmd/contractcheck schemas/comparison.schema.json \
  "$test_dir/default-comparison.json" "$test_dir/gated-comparison.json"
printf 'Comparison CLI checks passed, including report-independent gating.\n'
