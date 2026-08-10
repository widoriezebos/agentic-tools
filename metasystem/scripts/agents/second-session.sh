#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$harness_root/bin/metasystem}"
checkout=$(git -C "$harness_root" rev-parse --show-toplevel)
checkout=$(cd "$checkout" && pwd -P)
parent=${checkout%/*}
name=${1:-$(basename "$checkout")-session-$(date -u +%Y%m%dt%H%M%Sz)-$("$ms" util token-hex --bytes 2)}
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

# Copy the adapter-declared local configuration into the new worktree and
# audit its isolation; the printed path is the new checkout's harness root.
new_harness=$("$ms" validate session-isolation \
  --source-root "$checkout" --destination-root "$destination" \
  --manifest "$paths" --harness-root "$harness_root")
# started-at is an OS-level start-time query independent of which checkout runs
# it, so the current harness binary is equivalent to the new harness's helper.
bootstrap_start=$("$ms" identity started-at --pid "$$")
"$new_harness/scripts/agents/arm-supervision.sh" --repo "$destination" \
  --session "second-session-bootstrap-$name-$$" --pid "$$" \
  --start-time "$bootstrap_start" --tag "metasystem-main-bootstrap-$name-$$" >/dev/null
printf "cd '%s'\n" "$destination"
