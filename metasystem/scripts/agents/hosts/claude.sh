#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/hosts/claude.sh start-turn --mission <id> --turn-id <id>
      --prompt <file> --result <file> [--resume-session <sid>]
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"


wait_for_start_gate() {
  local gate=${METASYSTEM_HOST_START_GATE:-} cap=${METASYSTEM_HOST_START_GATE_TIMEOUT_SEC:-10} started=$SECONDS
  local poll_ms=${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-20} poll_sleep
  [[ -z "$gate" ]] && return 0
  [[ "$cap" =~ ^[1-9][0-9]*$ ]] || return 3
  [[ "$poll_ms" =~ ^[1-9][0-9]*$ ]] || return 3
  printf -v poll_sleep '%d.%03d' "$((poll_ms / 1000))" "$((poll_ms % 1000))"
  while [[ ! -e "$gate" ]]; do
    (( SECONDS - started < cap )) || return 3
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
command -v claude >/dev/null 2>&1 || { echo "claude CLI is not installed" >&2; exit 3; }
wait_for_start_gate || { echo "claude host start gate was not released" >&2; exit 3; }

turn_dir=$(cd "$(dirname "$prompt")" && pwd -P)
turn_record="$turn_dir/turn.json"
raw="$turn_dir/raw.out"
return_path="$turn_dir/return.json"
provider_result="$turn_dir/claude-result.json"
usage_path="$turn_dir/usage.json"
log="$turn_dir/host.log"
schema="$root/scripts/agents/schemas/orchestrator.schema.json"
model=$("$ms" json get --file "$turn_record" --field model)
# One home for the argv and budget policy (`adapter claude-command`,
# script-adapters-02/D25): host mode is the record-less call — acceptEdits
# with the full tools, exactly the policy this file used to hardcode.
claude_command_args=(--model "$model" --schema "$schema")
[[ -z "$resume_session" ]] || claude_command_args+=(--session "$resume_session")
command_file="$turn_dir/claude-command.nul"
if ! "$ms" adapter claude-command "${claude_command_args[@]}" >"$command_file" 2>>"$log"; then
  echo "claude host native budget configuration is invalid" >&2
  exit 3
fi
claude_command=()
while IFS= read -r -d '' token; do claude_command+=("$token"); done <"$command_file"
(( ${#claude_command[@]} > 0 )) || { echo "claude host argv assembly failed" >&2; exit 3; }
set +e
(
  cd "$root"
  "${claude_command[@]}" <"$prompt" >"$provider_result" 2>"$log"
)
cli_status=$?
set -e
cp "$provider_result" "$raw" 2>/dev/null || : >"$raw"

"$ms" host claude-result --provider "$provider_result" --return "$return_path" --usage "$usage_path"
session=$("$ms" json get --file "$provider_result" --field session_id --default "" 2>/dev/null || true)
# The turn outcome is the engine's one adjudication (`host finish`,
# script-adapters-10/D26); this script propagates its exit taxonomy.
"$ms" host finish --result "$result" --session "$session" --usage-file "$usage_path" \
  --raw "$raw" --return-path "${return_path:-}" --cli-status "$cli_status"
exit $?
