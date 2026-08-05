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
  [[ -z "$gate" ]] && return 0
  [[ "$cap" =~ ^[1-9][0-9]*$ ]] || return 3
  while [[ ! -e "$gate" ]]; do
    (( SECONDS - started < cap )) || return 3
    sleep 0.02
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
model=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["model"])' "$turn_record")
schema_json=$(python3 -c 'import json,sys; print(json.dumps(json.load(open(sys.argv[1])),separators=(",",":")))' "$schema")
max_budget=${METASYSTEM_CLAUDE_MAX_BUDGET_USD:-5.00}
max_turns=${METASYSTEM_CLAUDE_MAX_TURNS:-50}
[[ "$max_budget" =~ ^[0-9]+([.][0-9]+)?$ && "$max_turns" =~ ^[1-9][0-9]*$ ]] || {
  echo "claude host native budget configuration is invalid" >&2
  exit 3
}

claude_command=(
  claude -p --output-format json --model "$model" --json-schema "$schema_json"
  --permission-mode acceptEdits
  --tools Bash,Edit,Write,Read,Glob,Grep,NotebookEdit
  --allowedTools Bash,Edit,Write,Read,Glob,Grep,NotebookEdit
  --max-budget-usd "$max_budget" --max-turns "$max_turns"
)
[[ -z "$resume_session" ]] || claude_command+=(--resume "$resume_session")
set +e
(
  cd "$root"
  "${claude_command[@]}" <"$prompt" >"$provider_result" 2>"$log"
)
cli_status=$?
set -e
cp "$provider_result" "$raw" 2>/dev/null || : >"$raw"

python3 - "$provider_result" "$return_path" "$usage_path" <<'PY'
import json,sys
from pathlib import Path
source,out,usage_out=Path(sys.argv[1]),Path(sys.argv[2]),Path(sys.argv[3])
try: value=json.loads(source.read_text(encoding="utf-8"))
except (OSError,json.JSONDecodeError): value={}
candidate=value.get("structured_output")
if not isinstance(candidate,dict):
    result=value.get("result")
    if isinstance(result,str):
        try: candidate=json.loads(result)
        except json.JSONDecodeError: candidate=None
if isinstance(candidate,dict): out.write_text(json.dumps(candidate,indent=2,sort_keys=True)+"\n",encoding="utf-8")
native=value.get("usage") if isinstance(value.get("usage"),dict) else {}
cost=value.get("total_cost_usd")
usage={
  "availability":"native","inputTokens":native.get("input_tokens"),
  "cachedInputTokens":native.get("cache_read_input_tokens"),"outputTokens":native.get("output_tokens"),
  "reasoningTokens":native.get("reasoning_tokens"),
  "cost":{"amount":cost,"currency":"USD"} if isinstance(cost,(int,float)) else None,"providerUnits":None,
}
usage_out.write_text(json.dumps(usage,sort_keys=True)+"\n",encoding="utf-8")
PY
session=$(python3 - "$provider_result" <<'PY'
import json,sys
try: value=json.load(open(sys.argv[1]))
except (OSError,ValueError): value={}
print(value.get("session_id") or "")
PY
)
if (( cli_status != 0 )); then
  atomic_result "$result" "$session" failed "$usage_path" "$raw" ""
  exit 3
fi
if [[ -z "$session" || ( -n "$resume_session" && "$session" != "$resume_session" ) ]]; then
  atomic_result "$result" "$session" unresumable "$usage_path" "$raw" "${return_path:-}"
  exit 6
fi
atomic_result "$result" "$session" completed "$usage_path" "$raw" "$return_path"
