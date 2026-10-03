#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'benchreport: %s\n' "$1" >&2
  exit 1
}

case "$PUBLISH" in true|false) ;; *) fail 'publish must be true or false' ;; esac
case "$SUMMARY" in true|false) ;; *) fail 'summary must be true or false' ;; esac
[[ "$COMMENT_HEADER" =~ ^[a-zA-Z0-9._:-]{1,100}$ ]] || fail 'header must be 1 to 100 ASCII letters, digits, dots, underscores, colons, or hyphens'
[[ -f "$REPORT_PATH" && ! -L "$REPORT_PATH" ]] || fail 'report-path must identify a regular file, not a symlink'

# Snapshot to a generated filename: the upstream path input accepts glob patterns.
# A report filename containing brackets must still select exactly one file.
comment_path=$(mktemp "$RUNNER_TEMP/benchreport.XXXXXX")
cp -- "$REPORT_PATH" "$comment_path"
LC_ALL=C grep -q '[^[:space:]]' "$comment_path" || fail 'report is empty or contains only whitespace'
iconv -f UTF-8 -t UTF-8 "$comment_path" > /dev/null || fail 'report must contain valid UTF-8'

publication=completed
publish=true
if [[ "$PUBLISH" == false ]]; then
  publication=disabled
elif [[ "$EVENT_NAME" != pull_request ]]; then
  publication=unsupported-event
elif [[ "$ACTOR" == 'dependabot[bot]' ]]; then
  publication=dependabot
elif [[ -z "$HEAD_REPOSITORY" || "$HEAD_REPOSITORY" != "$REPOSITORY" ]]; then
  publication=fork
elif [[ ! "$PR_NUMBER" =~ ^[1-9][0-9]*$ ]]; then
  fail 'pull_request event has no valid PR number'
fi
if [[ "$publication" != completed ]]; then
  publish=false
fi

if [[ "$publish" == true ]]; then
  body_bytes=$(wc -c < "$comment_path" | tr -d '[:space:]')
  marker=$(printf '\n<!-- Sticky Pull Request Comment%s -->' "$COMMENT_HEADER")
  (( body_bytes + ${#marker} <= 60000 )) || fail 'comment exceeds the 60000-byte budget; provide a shorter report linked to the full artifact'
fi

if [[ "$SUMMARY" == true ]]; then
  cat "$comment_path" >> "$GITHUB_STEP_SUMMARY"
  printf '\n' >> "$GITHUB_STEP_SUMMARY"
fi
printf 'publish=%s\npublication=%s\ncomment-path=%s\n' "$publish" "$publication" "$comment_path" >> "$GITHUB_OUTPUT"
printf 'Benchmark comment publication: %s\n' "$publication"
