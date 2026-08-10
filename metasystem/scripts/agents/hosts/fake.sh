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

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"

wait_for_start_gate() {
  local gate=${METASYSTEM_HOST_START_GATE:-} cap=${METASYSTEM_HOST_START_GATE_TIMEOUT_SEC:-10} started=$SECONDS
  local poll_ms=${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-20} poll_sleep
  [[ -z "$gate" ]] && return 0
  [[ "$cap" =~ ^[1-9][0-9]*$ ]] || { echo "fake host start-gate timeout is invalid" >&2; return 3; }
  [[ "$poll_ms" =~ ^[1-9][0-9]*$ ]] || { echo "fake host handshake poll interval is invalid" >&2; return 3; }
  printf -v poll_sleep '%d.%03d' "$((poll_ms / 1000))" "$((poll_ms % 1000))"
  while [[ ! -e "$gate" ]]; do
    if (( SECONDS - started >= cap )); then
      echo "fake host start gate was not released within ${cap}s" >&2
      return 3
    fi
    sleep "$poll_sleep"
  done
}

command_name=${1:-}
if [[ "$command_name" == -h || "$command_name" == --help ]]; then usage; exit 0; fi
[[ "$command_name" == start-turn ]] || { usage; exit 2; }
shift
mission= turn_id= prompt= result= resume_session= instance_tag=
while (($#)); do
  case "$1" in
    --mission) [[ $# -ge 2 ]] || { usage; exit 2; }; mission=$2; shift 2 ;;
    --turn-id) [[ $# -ge 2 ]] || { usage; exit 2; }; turn_id=$2; shift 2 ;;
    --prompt) [[ $# -ge 2 ]] || { usage; exit 2; }; prompt=$2; shift 2 ;;
    --result) [[ $# -ge 2 ]] || { usage; exit 2; }; result=$2; shift 2 ;;
    --resume-session) [[ $# -ge 2 ]] || { usage; exit 2; }; resume_session=$2; shift 2 ;;
    --instance-tag) [[ $# -ge 2 ]] || { usage; exit 2; }; instance_tag=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done
[[ "$mission" =~ ^[a-z0-9][a-z0-9-]*$ && "$turn_id" =~ ^[a-z0-9][a-z0-9-]*$ ]] || { usage; exit 2; }
[[ -f "$prompt" && -n "$result" && -n "$instance_tag" ]] || { usage; exit 2; }

turn_dir=$(cd "$(dirname "$prompt")" && pwd -P)
turn_record="$turn_dir/turn.json"
[[ -f "$turn_record" ]] || { echo "fake host turn record is missing: $turn_record" >&2; exit 3; }
if [[ ${METASYSTEM_FAKE_HOST_START_UNVERIFIED:-0} == 1 ]]; then
  exit 0
fi
wait_for_start_gate || exit $?

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
