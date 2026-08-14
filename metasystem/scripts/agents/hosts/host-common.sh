# Shared host boilerplate (review script-adapters-13), sourced by the four
# host scripts. This is plumbing — invocation sequencing over verbs — so a
# shell library is the doctrine-conformant home. Each host keeps only its
# runtime-specific launch. The gate-wait messages carry the host's name and
# the timeout detail (fake.sh's formerly drifted variant is now everyone's:
# more diagnostic, same taxonomy).
#
# Expects from the sourcing host: usage() defined, host_runtime set to the
# runtime name. Provides: root, ms, wait_for_start_gate,
# host_parse_start_turn (sets mission turn_id prompt result resume_session
# instance_tag), host_require_cli.

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"

wait_for_start_gate() {
  local gate=${METASYSTEM_HOST_START_GATE:-} cap=${METASYSTEM_HOST_START_GATE_TIMEOUT_SEC:-10} started=$SECONDS
  local poll_ms=${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-20} poll_sleep
  [[ -z "$gate" ]] && return 0
  [[ "$cap" =~ ^[1-9][0-9]*$ ]] || { echo "$host_runtime host start-gate timeout is invalid" >&2; return 3; }
  [[ "$poll_ms" =~ ^[1-9][0-9]*$ ]] || { echo "$host_runtime host handshake poll interval is invalid" >&2; return 3; }
  printf -v poll_sleep '%d.%03d' "$((poll_ms / 1000))" "$((poll_ms % 1000))"
  while [[ ! -e "$gate" ]]; do
    if (( SECONDS - started >= cap )); then
      echo "$host_runtime host start gate was not released within ${cap}s" >&2
      return 3
    fi
    sleep "$poll_sleep"
  done
}

host_parse_start_turn() { # "$@" — sets the six start-turn variables
  local command_name=${1:-}
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
}

host_require_cli() { # CLI binary that must exist (skipped when empty — the fake host)
  if [[ -n "$1" ]] && ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 CLI is not installed" >&2
    exit 3
  fi
  wait_for_start_gate || exit 3
}
