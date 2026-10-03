#!/usr/bin/env bash
set -euo pipefail
project_dir=$(cd "$(dirname "$0")/.." && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
export RUNNER_TEMP="$test_dir"
export REPORT_PATH="$test_dir/report [timings].md"
export COMMENT_HEADER=benchmark-report PUBLISH=true SUMMARY=true
export EVENT_NAME=pull_request PR_NUMBER=15
export HEAD_REPOSITORY=example/project REPOSITORY=example/project ACTOR=contributor
export GITHUB_OUTPUT="$test_dir/output" GITHUB_STEP_SUMMARY="$test_dir/summary"
printf '# Results\n\n| Benchmark | Time |\n| --- | --- |\n| decode | 1 µs |\n' > "$REPORT_PATH"

run_case() {
  : > "$GITHUB_OUTPUT"
  : > "$GITHUB_STEP_SUMMARY"
  bash "$project_dir/scripts/prepare-comment.sh" > "$test_dir/log" 2>&1
}
expect_status() {
  run_case
  grep -qx "publication=$1" "$GITHUB_OUTPUT"
}
expect_failure() {
  if run_case; then
    printf 'Expected failure: %s\n' "$1" >&2
    exit 1
  fi
  grep -Fq "$1" "$test_dir/log"
  [[ ! -s "$GITHUB_OUTPUT" ]]
}

expect_status completed
grep -qx 'publish=true' "$GITHUB_OUTPUT"
snapshot=$(sed -n 's/^comment-path=//p' "$GITHUB_OUTPUT")
cmp "$REPORT_PATH" "$snapshot"
[[ -s "$GITHUB_STEP_SUMMARY" ]]

HEAD_REPOSITORY=external/fork expect_status fork
grep -qx 'publish=false' "$GITHUB_OUTPUT"
[[ -s "$GITHUB_STEP_SUMMARY" ]]
EVENT_NAME=pull_request_target expect_status unsupported-event
EVENT_NAME=push expect_status unsupported-event
ACTOR='dependabot[bot]' expect_status dependabot
PUBLISH=false expect_status disabled
SUMMARY=false expect_status completed
[[ ! -s "$GITHUB_STEP_SUMMARY" ]]
PR_NUMBER=0 expect_failure 'no valid PR number'
PUBLISH=yes expect_failure 'publish must be'
SUMMARY=yes expect_failure 'summary must be'
COMMENT_HEADER='bad header' expect_failure 'header must be'
REPORT_PATH="$test_dir/missing" expect_failure 'regular file'
ln -s "$REPORT_PATH" "$test_dir/link"
REPORT_PATH="$test_dir/link" expect_failure 'regular file'

printf ' \n\t' > "$REPORT_PATH"
expect_failure 'empty or contains only whitespace'
printf '\377' > "$REPORT_PATH"
expect_failure 'valid UTF-8'
head -c 60000 /dev/zero | tr '\0' a > "$REPORT_PATH"
expect_failure '60000-byte budget'
PUBLISH=false expect_status disabled
[[ -s "$GITHUB_STEP_SUMMARY" ]]

printf 'Publication preparation checks passed. No GitHub requests were made.\n'
