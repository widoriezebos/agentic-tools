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

atomic_json() { # output, JSON text
  python3 - "$1" "$2" <<'PY'
import json, os, sys, tempfile
from pathlib import Path
path=Path(sys.argv[1]); value=json.loads(sys.argv[2]); path.parent.mkdir(parents=True,exist_ok=True)
fd,temp=tempfile.mkstemp(prefix=path.name+".",suffix=".tmp",dir=path.parent)
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
  [[ "$cap" =~ ^[1-9][0-9]*$ ]] || { echo "fake host start-gate timeout is invalid" >&2; return 3; }
  while [[ ! -e "$gate" ]]; do
    if (( SECONDS - started >= cap )); then
      echo "fake host start gate was not released within ${cap}s" >&2
      return 3
    fi
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
  result_value=$(python3 - "$session" "$raw" <<'PY'
import json,sys
print(json.dumps({
  "sessionId":sys.argv[1],"outcome":"failed",
  "usage":{"availability":"native","inputTokens":1,"cachedInputTokens":0,"outputTokens":0,"reasoningTokens":None,"cost":None,"providerUnits":{"name":"fake-host-turn","value":1}},
  "rawPath":sys.argv[2],"returnPath":None,
},separators=(",",":")))
PY
  )
  atomic_json "$result" "$result_value"
  exit 3
fi

if [[ "$behavior" == return-malformed ]]; then
  printf '{malformed\n' >"$return_path"
elif [[ "$behavior" != no-return ]]; then
  python3 - "$turn_record" "$root/artifacts/agents/missions/$mission/state.json" "$return_path" \
    "$behavior" "$session" "$root" <<'PY'
import json,sys
from pathlib import Path
turn=json.loads(Path(sys.argv[1]).read_text()); state=json.loads(Path(sys.argv[2]).read_text())
out,behavior,session,root=Path(sys.argv[3]),sys.argv[4],sys.argv[5],Path(sys.argv[6])
active=next((name for name,value in sorted(state["streams"].items()) if value["state"]=="active"),next(iter(sorted(state["streams"]))))
value={
  "turnId":turn["turnId"],"missionId":turn["missionId"],"cycle":turn["cycle"],
  "dispatched":[],"certified":[],"streamUpdatesRequested":[],"askCandidates":[],
  "factsForLedger":[],"gaps":[],
  # The return attests what the prompt declared (Host-Session header, null on a
  # first or unresumable turn), never the session the adapter minted: that is
  # 6.2c's contract, and the live first turn of Mission Zero proved it.
  "identity":{"runtime":"fake","model":turn["model"],"sessionId":turn.get("hostSession")},
}
if behavior=="dispatch-ghost":
    value["dispatched"]=[{"jobId":f"ghost-{turn['cycle']}","role":"implementer","stream":active}]
elif behavior=="dispatch-terminal":
    job_id=f"verifier-{turn['missionId']}"
    jobs=root/"artifacts"/"agents"/"jobs"; jobs.mkdir(parents=True,exist_ok=True)
    record={
      "jobId":job_id,"role":"verifier","mission":turn["missionId"],"turnId":turn["turnId"],
      "runtime":"fake","round":1,"parentJob":None,"status":"completed","endedAt":turn["startedAt"],
      "capabilitySnapshot":"artifacts/agents/capabilities/fake.json","usage":None,"mirror":None,
      "chainClosed":False,"runnerClosed":False,
    }
    (jobs/f"{job_id}.json").write_text(json.dumps(record,indent=2,sort_keys=True)+"\n",encoding="utf-8")
    value["dispatched"]=[{"jobId":job_id,"role":"verifier","stream":active}]
elif behavior=="close-stream":
    value["streamUpdatesRequested"]=[{"streamId":active,"requestedState":"done","reason":"done"}]
elif behavior=="park-request":
    value["streamUpdatesRequested"]=[{"streamId":active,"requestedState":"parked-reserved","reason":"fake-host-request"}]
out.write_text(json.dumps(value,indent=2,sort_keys=True)+"\n",encoding="utf-8")
PY
fi

result_value=$(python3 - "$session" "$raw" "$return_path" <<'PY'
import json,sys
print(json.dumps({
  "sessionId":sys.argv[1],"outcome":"completed",
  "usage":{"availability":"native","inputTokens":11,"cachedInputTokens":2,"outputTokens":7,"reasoningTokens":None,"cost":None,"providerUnits":{"name":"fake-host-turn","value":1}},
  "rawPath":sys.argv[2],"returnPath":sys.argv[3],
},separators=(",",":")))
PY
)
atomic_json "$result" "$result_value"
