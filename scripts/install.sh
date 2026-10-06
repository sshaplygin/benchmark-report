#!/usr/bin/env bash
# Install the binary version mapped by this action commit. Never install a toolchain.
set -euo pipefail
script_dir=$(cd "$(dirname "$0")" && pwd)
version=$(cat "$script_dir/../report/VERSION")
benchstat_version=v0.0.0-20251023143056-3684bd442cc8
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]] || { printf 'Invalid action version mapping.\n' >&2; exit 1; }
case "$(uname -s)" in Linux) target_os=linux ;; Darwin) target_os=darwin ;; *) printf 'Unsupported operating system.\n' >&2; exit 1 ;; esac
case "$(uname -m)" in x86_64|amd64) target_arch=amd64 ;; arm64|aarch64) target_arch=arm64 ;; *) printf 'Unsupported architecture.\n' >&2; exit 1 ;; esac
: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is required}"
work_dir=$(mktemp -d "$RUNNER_TEMP/benchreport-install.XXXXXX")
installed=false
trap 'if [[ "$installed" != true ]]; then rm -rf "$work_dir"; fi' EXIT
asset="benchreport_${version}_${target_os}_${target_arch}.tar.gz"
base_url="https://github.com/sshaplygin/benchmark-report/releases/download/v${version}"
curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --retry 3 \
  "$base_url/$asset.sha256" --output "$work_dir/checksum"
curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --retry 3 \
  "$base_url/$asset" --output "$work_dir/archive.tar.gz"
expected=$(awk -v asset="$asset" '$2 == asset && NF == 2 {print $1}' "$work_dir/checksum")
[[ "$expected" =~ ^[0-9a-f]{64}$ ]] || { printf 'Missing, duplicate, or invalid archive checksum.\n' >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$work_dir/archive.tar.gz" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$work_dir/archive.tar.gz" | awk '{print $1}')
fi
[[ "$actual" == "$expected" ]] || { printf 'Archive checksum mismatch.\n' >&2; exit 1; }
# Only regular files with the packager's known namespace may be extracted.
tar -tzf "$work_dir/archive.tar.gz" > "$work_dir/members"
[[ ! -s "$work_dir/members" ]] && { printf 'Empty archive.\n' >&2; exit 1; }
[[ -z "$(sort "$work_dir/members" | uniq -d)" ]] || { printf 'Duplicate archive members.\n' >&2; exit 1; }
while IFS= read -r member; do
  case "$member" in benchreport|benchstat|VERSION|manifest.json|LICENSE|PROJECT-LICENSE-STATUS.txt) ;;
    LICENSES/*) [[ "$member" =~ ^LICENSES/[A-Za-z0-9._-]+$ ]] || { printf 'Invalid license archive path.\n' >&2; exit 1; } ;;
    *) printf 'Unexpected archive member: %s\n' "$member" >&2; exit 1 ;;
  esac
done < "$work_dir/members"
tar -tvzf "$work_dir/archive.tar.gz" | awk 'substr($0,1,1) != "-" {bad=1} END {exit bad}' || { printf 'Archive contains links or nonregular entries.\n' >&2; exit 1; }
mkdir "$work_dir/bin"
tar -xzf "$work_dir/archive.tar.gz" -C "$work_dir/bin"
[[ -f "$work_dir/bin/benchreport" && -f "$work_dir/bin/benchstat" && -f "$work_dir/bin/VERSION" && -f "$work_dir/bin/manifest.json" ]] || { printf 'Incomplete platform archive.\n' >&2; exit 1; }
[[ "$(cat "$work_dir/bin/VERSION")" == "$version" ]] || { printf 'Archive version mapping mismatch.\n' >&2; exit 1; }
jq -e --arg version "$version" --arg os "$target_os" --arg arch "$target_arch" --arg benchstat "$benchstat_version" \
  '.schema_version == 1 and .version == $version and .os == $os and .arch == $arch and .benchstat == $benchstat' \
  "$work_dir/bin/manifest.json" >/dev/null || { printf 'Archive platform or version mismatch.\n' >&2; exit 1; }
[[ -x "$work_dir/bin/benchreport" && -x "$work_dir/bin/benchstat" ]] || { printf 'Archive tools are not executable.\n' >&2; exit 1; }
[[ "$("$work_dir/bin/benchreport" --version)" == "benchreport $version" ]] || { printf 'Installed binary version mismatch.\n' >&2; exit 1; }
# RUNNER_TEMP may contain newlines; use the output protocol's multiline form.
delimiter="benchreport_${RANDOM}_${RANDOM}_${RANDOM}"
while [[ "$work_dir" == *"$delimiter"* ]]; do delimiter="benchreport_${RANDOM}_${RANDOM}_${RANDOM}"; done
printf 'bin-dir<<%s\n%s/bin\n%s\n' "$delimiter" "$work_dir" "$delimiter" >> "$GITHUB_OUTPUT"
installed=true
