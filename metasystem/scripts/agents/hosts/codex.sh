#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
# The runtime adapter owns Codex command construction, permission mapping,
# event identity, and typed usage. It sources runtime-common.sh and is guarded
# so hosts can reuse those contracts without entering its dispatch verb loop.
source "$root/scripts/agents/adapters/codex.sh"

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/hosts/codex.sh start-turn --mission <id> --turn-id <id>
      --prompt <file> --result <file> [--resume-session <sid>]
USAGE
}

atomic_result() { # result path, session, outcome, usage JSON path, raw, return or empty
  python3 - "$@" <<'PY'
import json,os,sys,tempfile
from pathlib import Path
path,session,outcome,usage_path,raw,return_path=sys.argv[1:]
try: usage=json.loads(Path(usage_path).read_text())
except (OSError,ValueError): usage={"availability":"unavailable"}
value={"sessionId":session or None,"outcome":outcome,"usage":usage,"rawPath":raw,"returnPath":return_path or None}
path=Path(path); path.parent.mkdir(parents=True,exist_ok=True); fd,temp=tempfile.mkstemp(prefix=path.name+".",suffix=".tmp",dir=path.parent)
try:
    with os.fdopen(fd,"w",encoding="utf-8") as handle:
        json.dump(value,handle,indent=2,sort_keys=True); handle.write("\n"); handle.flush(); os.fsync(handle.fileno())
    os.replace(temp,path)
    directory=os.open(path.parent,os.O_RDONLY)
    try: os.fsync(directory)
    finally: os.close(directory)
finally:
    try: os.unlink(temp)
    except FileNotFoundError: pass
PY
}

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
command -v codex >/dev/null 2>&1 || { echo "codex CLI is not installed" >&2; exit 3; }
wait_for_start_gate || { echo "codex host start gate was not released" >&2; exit 3; }

turn_dir=$(cd "$(dirname "$prompt")" && pwd -P)
turn_record="$turn_dir/turn.json"
raw="$turn_dir/raw.out"
return_path="$turn_dir/return.json"
events="$turn_dir/events.jsonl"
usage_path="$turn_dir/usage.json"
log="$turn_dir/host.log"
schema="$root/scripts/agents/schemas/orchestrator.schema.json"
permissions="$root/scripts/agents/permissions/workspace.json"
model=$(field "$turn_record" model)
codex_permission_settings "$permissions"
if [[ -z "$resume_session" ]]; then
  adapter_verb=dispatch
else
  adapter_verb=follow-up
fi
build_codex_command "$adapter_verb" "$model" "$root" "$schema" "$raw" \
  "$codex_sandbox_mode" "$codex_network_access" "$resume_session"

set +e
(
  # `codex exec resume` takes no -C. Entering the workspace before invocation
  # keeps resumed turns inside the same boundary as first turns.
  cd "$root"
  "${codex_cli_command[@]}" <"$prompt" >"$events" 2>>"$log"
)
cli_status=$?
set -e
[[ -f "$raw" ]] || : >"$raw"
codex_usage "$events" "$usage_path"
if [[ -s "$raw" ]]; then cp "$raw" "$return_path"; fi
session=$(codex_event_field "$events" session 2>/dev/null || true)
if (( cli_status != 0 )); then
  atomic_result "$result" "$session" failed "$usage_path" "$raw" ""
  exit 3
fi
if [[ -z "$session" || ( -n "$resume_session" && "$session" != "$resume_session" ) ]]; then
  atomic_result "$result" "$session" unresumable "$usage_path" "$raw" "$return_path"
  exit 6
fi
atomic_result "$result" "$session" completed "$usage_path" "$raw" "$return_path"
