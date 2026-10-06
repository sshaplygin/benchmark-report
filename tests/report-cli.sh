#!/usr/bin/env bash
set -euo pipefail
repo_dir=$(cd "$(dirname "$0")/.." && pwd)
test_dir=$(mktemp -d "${TMPDIR:-/tmp}/benchreport-report.XXXXXX")
trap 'rm -rf "$test_dir"' EXIT
cd "$repo_dir"
if [[ -n "${BENCHREPORT_BINARY:-}" ]]; then binary="$BENCHREPORT_BINARY"; else binary="$test_dir/benchreport"; go build -o "$binary" ./cmd/benchreport; fi
"$binary" --version
for parser in go criterion; do
 case "$parser" in go) fixture=go-pr15;; criterion) fixture=criterion-four-suites;; esac
 "$binary" report --parser "$parser" --base-manifest "testdata/captured/$fixture/base-manifest.json" --head-manifest "testdata/captured/$fixture/head-manifest.json" --output-dir "$test_dir/$parser" > "$test_dir/result.json"
 mv "$test_dir/$parser" "$test_dir/$parser-moved"
 "$binary" replay --reproduction "$test_dir/$parser-moved/reproduction.json" --output-dir "$test_dir/$parser-replay" > "$test_dir/replay-result.json"
 for name in report.md report.json comment.md comparison.json reproduction.json; do cmp "$test_dir/$parser-moved/$name" "$test_dir/$parser-replay/$name"; done
 "$binary" render --input "$test_dir/$parser-moved/comparison.json" --config "$test_dir/$parser-moved/replay-inputs/configuration.json" --reproduction "$test_dir/$parser-moved/reproduction.json" --output-dir "$test_dir/$parser-render"
 cmp "$test_dir/$parser-moved/report.md" "$test_dir/$parser-render/report.md"
 printf '\n' >> "$test_dir/$parser-moved/raw/base/000/000.log"
 if "$binary" replay --reproduction "$test_dir/$parser-moved/reproduction.json" --output-dir "$test_dir/$parser-bad"; then exit 1; fi
 test ! -e "$test_dir/$parser-bad"
done
printf '%s\n' '{"schema_version":1,"comparison":{"fail_on_regression":true,"regression_percent":0.000001}}' > "$test_dir/config.json"
"$binary" report --parser go --base-manifest testdata/captured/go-pr15/base-manifest.json --head-manifest testdata/captured/go-pr15/head-manifest.json --config "$test_dir/config.json" --output-dir "$test_dir/gate" > "$test_dir/gate-result.json"
jq -e '.gate == "failed"' "$test_dir/gate-result.json" >/dev/null
jq -j '.files[] | ., "\u0000"' "$test_dir/gate-result.json" > "$test_dir/gate-paths"
while IFS= read -r -d '' result_path; do
 test -f "$result_path"
done < "$test_dir/gate-paths"
if [[ -n "${BENCHREPORT_TEST_BENCHSTAT:-}" ]]; then
 printf '%s\n' '{"schema_version":1,"comparison":{"statistics":"benchstat"}}' > "$test_dir/stats.json"
 "$binary" report --parser go --base-manifest testdata/captured/go-pr15/base-manifest.json --head-manifest testdata/captured/go-pr15/head-manifest.json --config "$test_dir/stats.json" --benchstat-path "$BENCHREPORT_TEST_BENCHSTAT" --output-dir "$test_dir/stats" >/dev/null
 "$binary" replay --reproduction "$test_dir/stats/reproduction.json" --benchstat-path "$BENCHREPORT_TEST_BENCHSTAT" --output-dir "$test_dir/stats-replay" >/dev/null
fi
