#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/hosts/fake.sh start-turn --mission <id> --turn-id <id>
      --prompt <file> --result <file> [--resume-session <sid>]

Reads FAKEHOST:<behavior> markers from the assembled prompt. Behaviors:
return-ok (default), return-malformed, dispatch-ghost, dispatch-terminal,
close-stream, park-request, exit-nonzero, and no-return.
USAGE
}

host_runtime=fake
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/host-common.sh"

host_parse_start_turn "$@"

turn_dir=$(cd "$(dirname "$prompt")" && pwd -P)
turn_record="$turn_dir/turn.json"
[[ -f "$turn_record" ]] || { echo "fake host turn record is missing: $turn_record" >&2; exit 3; }
if [[ ${METASYSTEM_FAKE_HOST_START_UNVERIFIED:-0} == 1 ]]; then
  exit 0
fi
host_require_cli ""

behaviors=$(sed -n 's/.*FAKEHOST:\([a-z-][a-z-]*\).*/\1/p' "$prompt" | sort -u)
behavior_count=$(printf '%s\n' "$behaviors" | sed '/^$/d' | wc -l | tr -d ' ')
if (( behavior_count > 1 )); then
  echo "fake host prompt contains multiple behaviors" >&2
  exit 3
fi
behavior=${behaviors:-return-ok}
case "$behavior" in
  return-ok|return-malformed|dispatch-ghost|dispatch-terminal|close-stream|park-request|exit-nonzero|no-return) ;;
  *) echo "unknown fake host behavior: $behavior" >&2; exit 3 ;;
esac

raw="$turn_dir/raw.out"
return_path="$turn_dir/return.json"
printf 'fake host behavior=%s instance=%s\n' "$behavior" "$instance_tag" >"$raw"
session=${resume_session:-fake-host-session-$mission}

if [[ "$behavior" == exit-nonzero ]]; then
  "$ms" host fake-result --result "$result" --session "$session" --raw "$raw" --outcome failed
  exit 3
fi

if [[ "$behavior" == return-malformed ]]; then
  printf '{malformed\n' >"$return_path"
elif [[ "$behavior" != no-return ]]; then
  "$ms" host fake-return --turn "$turn_record" \
    --state "$root/artifacts/agents/missions/$mission/state.json" \
    --output "$return_path" --behavior "$behavior" --root "$root"
fi

"$ms" host fake-result --result "$result" --session "$session" --raw "$raw" \
  --return-path "$return_path" --outcome completed
