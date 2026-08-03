#!/usr/bin/env bash
set -euo pipefail

runtime=${1:-}
event=${2:-}
case "$runtime" in claude|codex|devin|fake) ;; *) exit 2 ;; esac
case "$event" in start|stop|end) ;; *) exit 2 ;; esac

payload=$(mktemp "${TMPDIR:-/tmp}/harness-supervision-hook.XXXXXX")
trap 'rm -f "$payload"' EXIT
cat >"$payload"

read_payload() {
  python3 - "$payload" "$1" <<'PY'
import json,sys
try: value=json.load(open(sys.argv[1])).get(sys.argv[2])
except (OSError,ValueError,AttributeError): value=None
if value is not None: print(value)
PY
}

cwd=$(read_payload cwd)
[[ -n "$cwd" ]] || cwd=${CLAUDE_PROJECT_DIR:-${DEVIN_PROJECT_DIR:-$PWD}}
repo=$(git -C "$cwd" rev-parse --show-toplevel 2>/dev/null) || exit 0
repo=$(cd "$repo" && pwd -P)
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/../.." && pwd -P)
helper=$script_dir/process-census.py
arm=$script_dir/arm-supervision.sh
[[ -x "$helper" && -x "$arm" ]] || exit 0
session=$(read_payload session_id)
[[ -n "$session" ]] || session="session-$PPID"

# Hook frameworks commonly insert `/bin/sh -c` between the agent and this
# script. Start above that shell so the runtime name in the hook command itself
# cannot become a false signature match.
search_pid=$(ps -p "$PPID" -o ppid= 2>/dev/null | tr -d ' ' || true)
[[ "$search_pid" =~ ^[1-9][0-9]*$ ]] || search_pid=$PPID
identity=$("$helper" find-ancestor --repo "$repo" --pid "$search_pid" --runtime "$runtime" 2>/dev/null || true)

surface_json() { # message
  python3 - "$1" <<'PY'
import json,sys
print(json.dumps({"systemMessage":sys.argv[1]},separators=(",",":")))
PY
}

if [[ "$event" == stop ]]; then
  last="$harness_root/artifacts/agents/supervision/last-census.json"
  state="$harness_root/artifacts/agents/supervision/state.json"
  message=$(python3 - "$last" "$state" "$helper" <<'PY'
import json,subprocess,sys,time
from pathlib import Path
last_path,state_path,helper=Path(sys.argv[1]),Path(sys.argv[2]),sys.argv[3]
lines=[]
try: last=json.loads(last_path.read_text())
except (OSError,ValueError): last=None
if last is None:
    lines.append("STALE-SUPERVISOR census verdict is absent")
else:
    age=int(time.time())-int(last.get("completedAtEpoch",0)); interval=int(last.get("intervalSec",0) or 0)
    if last.get("verdict") != "SUCCESS" or interval < 1 or age > interval:
        lines.append(f"STALE-SUPERVISOR census={last.get('verdict','UNREADABLE')} age={age}s")
    for item in last.get("inventory",[]):
        if item.get("class")=="UNTRACKED": lines.append(f"UNTRACKED pid={item.get('pid')} runtime={item.get('runtime')} argv={item.get('argv')}")
try: state=json.loads(state_path.read_text())
except (OSError,ValueError): state=None
if not isinstance(state,dict) or not isinstance(state.get("owner"),dict) or not isinstance(state.get("components"),dict):
    lines.append("STALE-SUPERVISOR state is absent or unreadable")
    state={}
elif last is not None and state.get("fingerprint") != last.get("fingerprint"):
    lines.append("STALE-SUPERVISOR census fingerprint does not match the active supervisor set")
identities={}
if isinstance(state.get("owner"),dict): identities["owner"]=state["owner"]
identities.update(state.get("components",{}))
for name,item in identities.items():
    try:
        ok=subprocess.run([helper,"alive","--pid",str(item["pid"]),"--start-time",str(item["pidStartedAt"])],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0
    except (KeyError,TypeError,OSError): ok=False
    if not ok: lines.append(f"STALE-SUPERVISOR component={name}")
print("\n".join(lines[:20]))
PY
  )
  [[ -z "$message" ]] || surface_json "$message"
  exit 0
fi

if [[ -z "$identity" ]]; then
  surface_json "Harness supervision could not identify the immediate $runtime agent process; arming was refused."
  exit 0
fi
pid=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["pid"])' "$identity")
started=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["pidStartedAt"])' "$identity")
tag="harness-main-$runtime-$(python3 -c 'import re,sys; print(re.sub(r"[^A-Za-z0-9._-]+","-",sys.argv[1]).strip("-.").lower() or "session")' "$session")"

if [[ "$event" == end ]]; then
  HARNESS_AGENT_RUNTIME="$runtime" "$arm" --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" --tag "$tag" --retire >/dev/null 2>&1 || true
  exit 0
fi

if output=$(HARNESS_AGENT_RUNTIME="$runtime" "$arm" --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" --tag "$tag" 2>&1); then
  exit 0
fi
surface_json "Harness supervision arming failed: $(printf '%s' "$output" | tail -1)"
