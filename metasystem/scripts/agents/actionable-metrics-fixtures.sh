#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/actionable-metrics-fixtures.XXXXXX")

cleanup() {
  local status=$?
  rm -rf "$tmp"
  return "$status"
}
trap cleanup EXIT

bash -n "$root/scripts/agents/dispatch.sh" "$root/scripts/agents/critique-round.sh" "$0"

packet="$tmp/packet.md"
printf 'review this\n' >"$packet"

grep -Fq -- '[--stream <mission-stream-id>] [--goal <goal-id>]' "$root/scripts/agents/dispatch.sh"
grep -Fq -- '--goal) [[ $# -ge 2 ]] || { usage; exit 2; }; goal=$2; shift 2 ;;' "$root/scripts/agents/dispatch.sh"
[[ $(grep -Fc -- '--goal "$goal"' "$root/scripts/agents/dispatch.sh") -ge 3 ]]
grep -Fq -- 'goal=$(json_field "$latest" goalId 2>/dev/null || true)' "$root/scripts/agents/dispatch.sh"
if "$root/scripts/agents/dispatch.sh" dispatch --role implementer --brief "$packet" --goal Invalid_goal >"$tmp/dispatch.out" 2>"$tmp/dispatch.err"; then
  echo "O13-dispatch-goal-plumbing failed: invalid goal was accepted" >&2
  exit 1
fi
grep -Fq 'invalid goal id: Invalid_goal' "$tmp/dispatch.err"
echo "O13-dispatch-goal-plumbing passed"
