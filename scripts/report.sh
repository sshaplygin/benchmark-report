#!/usr/bin/env bash
set -euo pipefail
: "${REPORT_BINARY:?REPORT_BINARY is required}"
: "${REPORT_PARSER:?parser is required}"
: "${BASE_MANIFEST:?base-manifest is required}"
: "${HEAD_MANIFEST:?head-manifest is required}"
: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is required}"
case "${ALLOW_ENVIRONMENT_MISMATCH-false}" in true|false) ;; *) printf 'allow-environment-mismatch must be true or false.\n' >&2; exit 1 ;; esac
header=${COMMENT_HEADER-benchmark-report}
[[ "$header" =~ ^[A-Za-z0-9._:-]{1,100}$ ]] || { printf 'Invalid comment-header.\n' >&2; exit 1; }
output_dir=${REPORT_OUTPUT_DIR:-}
if [[ -z "$output_dir" ]]; then output_dir=$(mktemp -d "$RUNNER_TEMP/benchreport-report.XXXXXX"); fi
args=(report --parser "$REPORT_PARSER" --base-manifest "$BASE_MANIFEST" --head-manifest "$HEAD_MANIFEST" --output-dir "$output_dir" --comment-header "$header")
if [[ -n "${REPORT_CONFIG:-}" ]]; then args+=(--config "$REPORT_CONFIG"); fi
if [[ -n "${ARTIFACT_URL:-}" ]]; then args+=(--artifact-url "$ARTIFACT_URL"); fi
if [[ "${ALLOW_ENVIRONMENT_MISMATCH-false}" == true ]]; then args+=(--allow-environment-mismatch); fi
result=$(mktemp "$RUNNER_TEMP/benchreport-result.XXXXXX")
trap 'rm -f "$result"' EXIT
"$REPORT_BINARY" "${args[@]}" > "$result"
jq -e '.schema_version == 1 and (.gate == "disabled" or .gate == "passed" or .gate == "failed") and (.files.comparison | type == "string") and (.files.reproduction | type == "string")' "$result" >/dev/null
emit() {
  local name=$1 value=$2 delimiter
  delimiter="benchreport_${RANDOM}_${RANDOM}_${RANDOM}"
  while [[ "$value" == *"$delimiter"* ]]; do delimiter="benchreport_${RANDOM}_${RANDOM}_${RANDOM}"; done
  printf '%s<<%s\n%s\n%s\n' "$name" "$delimiter" "$value" "$delimiter" >> "$GITHUB_OUTPUT"
}
scalar() {
  # The sentinel preserves trailing newlines in valid filenames.
  json_value=$(jq -j "$1" "$result" && printf '.')
  json_value=${json_value%.}
}
scalar '.gate'
emit gate "$json_value"
scalar '.files.reproduction'
artifact_dir=${json_value%/*}
emit artifact-path "${artifact_dir:-/}"
for mapping in 'markdown:report-path' 'presentation:json-path' 'comparison:comparison-path' 'reproduction:reproduction-path' 'comment:comment-path' 'history_smaller:history-smaller-path' 'history_bigger:history-bigger-path'; do
  key=${mapping%%:*}
  name=${mapping#*:}
  scalar ".files.$key // \"\""
  emit "$name" "$json_value"
done
