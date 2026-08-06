#!/usr/bin/env bash
set -euo pipefail

runtime=${1:-}
event=${2:-}
case "$runtime" in claude|codex|devin|fake) ;; *) exit 2 ;; esac
case "$event" in start|stop|end) ;; *) exit 2 ;; esac

payload=$(mktemp "${TMPDIR:-/tmp}/metasystem-supervision-hook.XXXXXX")
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
  # Continuation is the one part of the loop no prompt can guarantee, so it is
  # checked here rather than asked for in prose: a turn ending while a plan
  # still names an unblocked next step and nothing is in flight says so.
  open_work=$(python3 "$script_dir/open-work.py" --repo "$harness_root" 2>/dev/null || true)
  [[ -z "$open_work" ]] || message=$(printf '%s%s%s' "$message" "${message:+$'\n'}" "$open_work")

  # Leave evidence that this ran. Without it there is no telling a hook that
  # fired and found nothing from one that never fired, which is the confusion
  # that let this repository run for days with its hooks uninstalled.
  supervision_dir="$harness_root/artifacts/agents/supervision"
  mkdir -p "$supervision_dir"
  # Hygiene runs itself: every turn end collects what the mirror already
  # holds, so residue never waits for a human to wander in with a flashlight.
  # Best-effort by design; a repo without an evidence root is politely refused
  # by the collector and that is fine.
  "$script_dir/evidence-gc.sh" >>"$supervision_dir/hooks.log" 2>&1 || true

  printf '%s stop open=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    "$(printf '%s' "$message" | grep -c '^OPEN-WORK' || true)" \
    >>"$supervision_dir/hooks.log" 2>/dev/null || true

  # Refuse to end the turn while work is open, once. A report is ignorable and
  # was ignored. An unbounded block is the loop the design forbids, so the same
  # open work never blocks twice: the second attempt passes and the human
  # decides.
  # Keyed by session: with a shared file, a peer session's refusal consumed
  # this session's one block, and the same open work then sailed through.
  blocked_state="$supervision_dir/stop-block-$session.state"
  open_only=$(printf '%s' "$message" | grep '^OPEN-WORK' || true)
  if [[ -n "$open_only" ]]; then
    signature=$(printf '%s' "$open_only" | shasum | cut -d' ' -f1)
    if [[ "$(cat "$blocked_state" 2>/dev/null || true)" != "$signature" ]]; then
      printf '%s' "$signature" >"$blocked_state" 2>/dev/null || true
      python3 "$script_dir/stop-block.py" "$message"
      exit 0
    fi
  else
    rm -f "$blocked_state" 2>/dev/null || true
  fi

  [[ -z "$message" ]] || surface_json "$message"
  exit 0
fi

if [[ -z "$identity" ]]; then
  surface_json "Metasystem supervision could not identify the immediate $runtime agent process; arming was refused."
  exit 0
fi
pid=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["pid"])' "$identity")
started=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["pidStartedAt"])' "$identity")
tag="metasystem-main-$runtime-$(python3 -c 'import re,sys; print(re.sub(r"[^A-Za-z0-9._-]+","-",sys.argv[1]).strip("-.").lower() or "session")' "$session")"

if [[ "$event" == end ]]; then
  METASYSTEM_AGENT_RUNTIME="$runtime" "$arm" --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" --tag "$tag" --retire >/dev/null 2>&1 || true
  exit 0
fi

if output=$(METASYSTEM_AGENT_RUNTIME="$runtime" "$arm" --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" --tag "$tag" 2>&1); then
  exit 0
fi
surface_json "Metasystem supervision arming failed: $(printf '%s' "$output" | tail -1)"
