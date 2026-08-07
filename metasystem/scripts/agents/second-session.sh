#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/../.." && pwd -P)
checkout=$(git -C "$harness_root" rev-parse --show-toplevel)
checkout=$(cd "$checkout" && pwd -P)
parent=${checkout%/*}
name=${1:-$(basename "$checkout")-session-$(date -u +%Y%m%dt%H%M%Sz)-$(python3 -c 'import secrets; print(secrets.token_hex(2))')}
[[ "$name" =~ ^[A-Za-z0-9._-]+$ ]] || {
  echo "second-session name must contain only letters, numbers, dot, underscore, and hyphen" >&2
  exit 2
}
destination=$parent/$name
[[ ! -e "$destination" ]] || {
  echo "second-session destination already exists: $destination" >&2
  exit 1
}

branch="session/$name"
git -C "$checkout" worktree add -q -b "$branch" "$destination" HEAD

paths=$(mktemp "${TMPDIR:-/tmp}/metasystem-local-config-paths.XXXXXX")
trap 'rm -f "$paths"' EXIT
for adapter in "$script_dir"/adapters/*.sh; do
  [[ ${adapter##*/} != runtime-common.sh ]] || continue
  "$adapter" local-config-paths >>"$paths"
done
sort -u -o "$paths" "$paths"

new_harness=$("$script_dir/second-session-isolation.py" \
  --source-root "$checkout" --destination-root "$destination" \
  --manifest "$paths" --harness-root "$harness_root")
bootstrap_start=$("$new_harness/scripts/agents/process-census.py" started-at --pid "$$")
"$new_harness/scripts/agents/arm-supervision.sh" --repo "$destination" \
  --session "second-session-bootstrap-$name-$$" --pid "$$" \
  --start-time "$bootstrap_start" --tag "metasystem-main-bootstrap-$name-$$" >/dev/null
printf "cd '%s'\n" "$destination"
