#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
token=$root/artifacts/agents/mains/worktree-commit-token.json

if [[ ${1:-} != __lease-held ]]; then
  result=$("$ms" lease require-holder --root "$root" --caller-pid "$$") || exit $?
  # --default "" collapses an absent or null claimEpoch to empty, so the
  # human-commit branch below is taken when there is no epoch.
  epoch=$("$ms" json get --value "$result" --field claimEpoch --default "")
  if [[ -n "$epoch" ]]; then
    exec "$ms" lease run-held --root "$root" --caller-pid "$$" \
      --expected-epoch "$epoch" -- "$0" __lease-held "$epoch" "$@"
  fi
  exec "$ms" lease run-held --root "$root" --caller-pid "$$" -- "$0" __lease-held human "$@"
fi
shift
expected_epoch=${1:-}
[[ -n "$expected_epoch" ]] || exit 2
shift
if [[ "$expected_epoch" =~ ^[1-9][0-9]*$ ]]; then
  "$ms" lease require-holder --root "$root" --caller-pid "$$" \
    --expected-epoch "$expected_epoch" >/dev/null
else
  [[ "$expected_epoch" == human ]] || exit 2
  "$ms" lease require-holder --root "$root" --caller-pid "$$" >/dev/null
fi

started=$("$ms" identity started-at --pid $$) || {
  echo "agent commit wrapper refused: wrapper process start time is unreadable" >&2
  exit 1
}
nonce=$("$ms" util token-hex --bytes 16)
"$ms" lease commit-token --path "$token" --pid "$$" --start "$started" --nonce "$nonce"
trap 'rm -f -- "$token"' EXIT
git -C "$root" commit "$@"
