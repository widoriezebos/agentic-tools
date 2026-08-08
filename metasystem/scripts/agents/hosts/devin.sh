#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/hosts/devin.sh start-turn --mission <id> --turn-id <id>
      --prompt <file> --result <file> [--resume-session <sid>]
      --instance-tag <tag>
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
command -v devin >/dev/null 2>&1 || { echo "devin CLI is not installed" >&2; exit 3; }
wait_for_start_gate || { echo "devin host start gate was not released" >&2; exit 3; }

turn_dir=$(cd "$(dirname "$prompt")" && pwd -P)
turn_record="$turn_dir/turn.json"
raw="$turn_dir/raw.out"
return_path="$turn_dir/return.json"
transcript="$turn_dir/transcript.atif.json"
cumulative="$turn_dir/session-usage.json"
config_file="$turn_dir/devin-config.json"
usage_path="$turn_dir/usage.json"
log="$turn_dir/host.log"
schema="$root/scripts/agents/schemas/orchestrator.schema.json"
permissions="$root/scripts/agents/permissions/workspace.json"
model=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["model"])' "$turn_record")

# The host edits the repository it is advancing, so it runs write-capable:
# accept-edits with the workspace permission preset. A host confined to `auto`
# with edit and exec denied could not move a mission at all.
#
# The boundary those roots describe is NOT enforced on this runtime: --sandbox
# is refused by this organisation's policy, and a shell command writes outside
# the declared write root. The human accepted that residual globally on
# 2026-08-08; the capability snapshot declares it, and this host does not
# pretend otherwise.
python3 - "$permissions" "$config_file" "$root" <<'PY'
import json, os, sys
from pathlib import Path

requested = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
workspace = Path(sys.argv[3]).resolve()
allow = ["read", "grep", "glob", "edit", "exec", f"Read({workspace}/**)", f"Write({workspace}/**)"]
deny = ["mcp__*"]
config_home = os.environ.get("XDG_CONFIG_HOME") or str(Path.home() / ".config")
try:
    value = json.loads((Path(config_home) / "devin" / "config.json").read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        value = {}
except (OSError, ValueError):
    value = {}
# The user's file is the base so the organisation id and onboarding marker
# survive; a config without them makes the CLI print a welcome banner into the
# turn's stdout, which is where this host reads the return from.
value.pop("sandbox", None)
value["permissions"] = {"allow": allow, "ask": [], "deny": deny}
Path(sys.argv[2]).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

# This runtime has no schema flag, so the schema goes in the prompt or the model
# invents field names -- the exact failure seen on the delegate side. The
# dispatcher's prompt file is left untouched as evidence; the CLI reads the
# augmented copy.
devin_prompt="$turn_dir/prompt.devin.md"
{
  cat "$prompt"
  printf '\n\n# Return schema, exact\n\n'
  printf 'Your reply must be ONE JSON object valid against this schema and nothing\n'
  printf 'else: no prose before or after it, no code fence, and no property this\n'
  printf 'schema does not name. Every property listed in "required" must be present.\n\n'
  cat "$schema"
} >"$devin_prompt"

devin_command=(
  devin -p
  --prompt-file "$devin_prompt"
  --respect-workspace-trust false
  --model "$model"
  --permission-mode accept-edits
  --config "$config_file"
  --export "$transcript"
)
[[ -z "$resume_session" ]] || devin_command+=(-r "$resume_session")

set +e
(
  cd "$root"
  "${devin_command[@]}" >"$raw" 2>"$log"
)
cli_status=$?
set -e
[[ -f "$raw" ]] || : >"$raw"

# Devin has no native structured output, so the return is whatever the turn
# printed, kept only when it parses as an object. Anything else leaves the
# return absent for the runner's own validation to report, rather than being
# passed on as a return that is not one.
python3 - "$raw" "$return_path" <<'PY'
import json, sys
from pathlib import Path
source, out = Path(sys.argv[1]), Path(sys.argv[2])
try:
    text = source.read_text(encoding="utf-8")
except OSError:
    raise SystemExit(0)
start, end = text.find("{"), text.rfind("}")
if start < 0 or end <= start:
    raise SystemExit(0)
try:
    value = json.loads(text[start:end + 1])
except ValueError:
    raise SystemExit(0)
if isinstance(value, dict):
    out.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

# final_metrics is CUMULATIVE for a session (a resumed turn reports the session
# total, not its own), and mission and benchmark consumers ADD turn records, so
# each turn publishes the delta against its predecessor's stored totals. A turn
# whose predecessor artifact is missing publishes unavailable rather than a
# number that would double-count every earlier turn in every aggregate.
# The predecessor is keyed by SESSION, not by whichever session-usage.json is
# newest. Picking the most recent across all turns subtracted an unrelated
# turn's totals when the immediate predecessor's artifact was missing; a
# per-session store makes the delta deterministic. The store sits in the turns
# parent so it survives across a mission's turns.
session_store="$(cd "$turn_dir/.." && pwd -P)/.session-usage"
mkdir -p "$session_store"
session_key=$(python3 -c 'import re,sys; print(re.sub(r"[^A-Za-z0-9._-]+","-",sys.argv[1]).strip("-.") or "session")' "${resume_session:-}")
previous_cumulative=
if [[ -n "$resume_session" && -f "$session_store/$session_key.json" ]]; then
  previous_cumulative="$session_store/$session_key.json"
fi
expect_previous=0
[[ -n "$resume_session" ]] && expect_previous=1
python3 - "$usage_path" "$transcript" "$cumulative" "${previous_cumulative:-}" "$expect_previous" <<'PY'
import json, sys
from pathlib import Path

def load(path):
    try:
        return json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None

FIELDS = ("total_prompt_tokens", "total_completion_tokens", "total_cached_tokens", "total_steps")
metrics = (load(sys.argv[2]) or {}).get("final_metrics")
totals = {}
if isinstance(metrics, dict):
    totals = {name: metrics.get(name) for name in FIELDS if isinstance(metrics.get(name), int)}

# Same rule as the delegate adapter: an enterprise account reports ACU and no
# tokens; ACU rides in providerUnits (metered by the fence), never as a token
# or a cost, and nothing is invented when no acu-named field exists.
def provider_unit(source):
    if not isinstance(source, dict):
        return None
    for name, value in sorted(source.items()):
        if "acu" in name.lower() and isinstance(value, (int, float)) and not isinstance(value, bool):
            return {"name": "acu", "value": value, "sourceKey": name}
    return None

acu = provider_unit(metrics)
previous_path = sys.argv[4]
expect_previous = sys.argv[5] == "1"
previous = load(previous_path) if previous_path else None
# A resumed turn that cannot find its predecessor's totals cannot compute a
# delta; publishing session totals would double-count earlier turns.
predecessor_missing = expect_previous and not isinstance(previous, dict)
unavailable = {
    "availability": "unavailable", "inputTokens": None, "cachedInputTokens": None,
    "outputTokens": None, "reasoningTokens": None, "cost": None,
    "providerUnits": {"name": acu["name"], "value": acu["value"]} if acu else None,
}
if len(totals) != len(FIELDS):
    if acu:
        Path(sys.argv[3]).write_text(json.dumps({acu["sourceKey"]: acu["value"]}, sort_keys=True) + "\n", encoding="utf-8")
        base = previous if isinstance(previous, dict) else {}
        earlier = base.get(acu["sourceKey"])
        if predecessor_missing:
            unavailable["providerUnits"] = None
        elif isinstance(earlier, (int, float)) and not isinstance(earlier, bool):
            unavailable["providerUnits"] = {"name": "acu", "value": acu["value"] - earlier}
    Path(sys.argv[1]).write_text(json.dumps(unavailable, sort_keys=True) + "\n", encoding="utf-8")
    raise SystemExit(0)
Path(sys.argv[3]).write_text(json.dumps(totals, sort_keys=True) + "\n", encoding="utf-8")
if predecessor_missing:
    Path(sys.argv[1]).write_text(json.dumps(unavailable, sort_keys=True) + "\n", encoding="utf-8")
    raise SystemExit(0)
base = previous if isinstance(previous, dict) else {}
def delta(name):
    earlier = base.get(name)
    return totals[name] - earlier if isinstance(earlier, int) else totals[name]
Path(sys.argv[1]).write_text(json.dumps({
    "availability": "native",
    "inputTokens": delta("total_prompt_tokens"),
    "cachedInputTokens": delta("total_cached_tokens"),
    "outputTokens": delta("total_completion_tokens"),
    "reasoningTokens": None,
    "cost": None,
    "providerUnits": {"name": "devin-steps", "value": delta("total_steps")},
}, sort_keys=True) + "\n", encoding="utf-8")
PY

session=$(python3 - "$transcript" <<'PY'
import json,sys
from pathlib import Path
try: value=json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
except (OSError,ValueError): value={}
print(value.get("session_id") or "")
PY
)
if (( cli_status != 0 )); then
  atomic_result "$result" "$session" failed "$usage_path" "$raw" ""
  exit 3
fi
# Exit 0 with no reply is this runtime's shape for "could not do it": the turn
# ends the moment a tool is denied. Treating it as success would hand the runner
# an empty return and blame the wrong thing.
if [[ ! -s "$raw" ]]; then
  atomic_result "$result" "$session" failed "$usage_path" "$raw" ""
  exit 3
fi
if [[ -z "$session" || ( -n "$resume_session" && "$session" != "$resume_session" ) ]]; then
  atomic_result "$result" "$session" unresumable "$usage_path" "$raw" "$return_path"
  exit 6
fi
# Publish this turn's cumulative totals into the per-session store so the next
# turn of THIS session subtracts the right predecessor. Keyed by the confirmed
# session, written only on a clean, resumable completion.
if [[ -s "$cumulative" ]]; then
  completed_key=$(python3 -c 'import re,sys; print(re.sub(r"[^A-Za-z0-9._-]+","-",sys.argv[1]).strip("-.") or "session")' "$session")
  cp "$cumulative" "$session_store/$completed_key.json" 2>/dev/null || true
fi
atomic_result "$result" "$session" completed "$usage_path" "$raw" "$return_path"
