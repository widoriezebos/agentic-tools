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
lease_helper=$script_dir/worktree-lease.py
[[ -x "$helper" && -x "$arm" && -x "$lease_helper" ]] || exit 0
session=$(read_payload session_id)
[[ -n "$session" ]] || session="session-$PPID"

# Hook frameworks commonly insert `/bin/sh -c` between the agent and this
# script. Start above that shell so the runtime name in the hook command itself
# cannot become a false signature match.
search_pid=$(ps -p "$PPID" -o ppid= 2>/dev/null | tr -d ' ' || true)
[[ "$search_pid" =~ ^[1-9][0-9]*$ ]] || search_pid=$PPID
identity=$("$helper" find-ancestor --repo "$repo" --pid "$search_pid" --runtime "$runtime" 2>/dev/null || true)
main_id=
main_holder=false
if [[ -n "$identity" ]]; then
  identity_pid=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["pid"])' "$identity")
  lease_view=$("$lease_helper" --root "$harness_root" classify --caller-pid "$identity_pid" 2>/dev/null || true)
  if [[ -n "$lease_view" ]]; then
    main_id=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("mainId",""))' "$lease_view")
    main_holder=$(python3 -c 'import json,sys; print("true" if json.loads(sys.argv[1]).get("holder") else "false")' "$lease_view")
  fi
fi

surface_json() { # message
  python3 - "$1" <<'PY'
import json,sys
print(json.dumps({"systemMessage":sys.argv[1]},separators=(",",":")))
PY
}

count_running_work() { # sets: running, elsewhere
  running=0
  elsewhere=""
  for record in "$harness_root/artifacts/agents/jobs"/*.json; do
    [[ -e "$record" ]] || break
    if grep -q '"status": *"\(pending\|running\)"' "$record" 2>/dev/null; then
      running=$((running + 1))
    fi
  done
  # A mission started from this repository usually runs in ANOTHER repository
  # (a benchmark target, a scratch repo). Reporting only this directory's
  # jobs told the human "nothing is running" while their benchmark was in
  # full flight, which is worse than saying nothing at all.
  local other
  while IFS= read -r other; do
    [[ -n "$other" ]] || continue
    elsewhere="$elsewhere $other"
  done < <(pgrep -f 'mission-runner.sh __run' 2>/dev/null \
    | while read -r pid; do
        ps -p "$pid" -o command= 2>/dev/null \
          | sed -n 's|.*/\([^/ ]*\)/scripts/agents/mission-runner.sh.*|\1|p'
      done | sort -u)
}

work_sentence() {
  count_running_work
  local missions="" active=""
  [[ -n "$elsewhere" ]] && missions=$(printf '%s' "$elsewhere" | sed 's/^ //; s/ /, /g')
  (( running )) && active="$running helper agent(s)"
  [[ -n "$missions" ]] && active="${active:+$active, and }a mission still going in $missions"
  # The orchestrator's own gate runs are neither delegate jobs nor missions,
  # and they are exactly what a human sees mid-flight and asks about.
  if pgrep -f 'validate-metasystem\.sh|validate-kit\.sh' >/dev/null 2>&1; then
    active="${active:+$active, and }the test gates"
  fi
  if [[ -n "$active" ]]; then
    printf 'STILL WORKING: %s.' "$active"
  elif [[ -n "${open_work:-}" ]]; then
    # Never assert an idle the open-work checker just contradicted.
    printf 'NO AGENTS RUNNING — but a plan still names open work, listed below.'
  else
    printf 'NOTHING LEFT TO WORK ON: no helper agents, no missions, no gate runs, and no plan names open work.'
  fi
}

if [[ "$event" == stop ]]; then
  protocol_message=
  protocol_counts='{}'
  if [[ -n "$main_id" ]]; then
    protocol_growth=$("$lease_helper" --root "$harness_root" protocol-growth --main-id "$main_id" 2>/dev/null || true)
    if [[ -n "$protocol_growth" ]]; then
      protocol_message=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("message",""))' "$protocol_growth")
      protocol_counts=$(python3 -c 'import json,sys; print(json.dumps(json.loads(sys.argv[1]).get("counts",{}),separators=(",",":")))' "$protocol_growth")
    fi
  fi
  if [[ "$main_holder" != true ]]; then
    advisor_message="OWNED-ELSEWHERE: this main is a read-only advisor in this checkout. To write independently, run scripts/agents/second-session.sh."
    [[ -z "$protocol_message" ]] || advisor_message="$advisor_message
$protocol_message"
    surface_json "$advisor_message"
    [[ -z "$main_id" || -z "$protocol_message" ]] || \
      "$lease_helper" --root "$harness_root" protocol-advance --main-id "$main_id" \
        --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
    exit 0
  fi
  "$lease_helper" --root "$harness_root" renew --caller-pid "$identity_pid" >/dev/null 2>&1 || true
  last="$harness_root/artifacts/agents/supervision/last-census.json"
  state="$harness_root/artifacts/agents/supervision/state.json"
  message=$(python3 - "$last" "$state" "$helper" <<'PY'
import json,subprocess,sys,time
from pathlib import Path
last_path,state_path,helper=Path(sys.argv[1]),Path(sys.argv[2]),sys.argv[3]
arm_cmd="scripts/agents/arm-supervision.sh --repo ."
lines=[]
try: last=json.loads(last_path.read_text())
except (OSError,ValueError): last=None
if last is None:
    lines.append("WATCHDOG: it has not reported at all yet. Re-arm it with " + arm_cmd + " if this persists.")
else:
    age=int(time.time())-int(last.get("completedAtEpoch",0)); interval=int(last.get("intervalSec",0) or 0)
    window=min(2*interval,180) if interval>=1 else 0
    if last.get("verdict") != "SUCCESS" or interval < 1 or age >= window:
        lines.append(f"WATCHDOG: its last report is {age}s old or unsuccessful, so it may not be watching. Re-arm it with {arm_cmd}.")
    for item in last.get("inventory",[]):
        if item.get("class")=="UNTRACKED": lines.append(f"UNTRACKED pid={item.get('pid')} runtime={item.get('runtime')} argv={item.get('argv')}")
try: state=json.loads(state_path.read_text())
except (OSError,ValueError): state=None
if not isinstance(state,dict) or not isinstance(state.get("owner"),dict) or not isinstance(state.get("components"),dict):
    lines.append("WATCHDOG: its record of what it is watching is missing or unreadable. Re-arm it with " + arm_cmd + ".")
    state={}
elif last is not None and state.get("fingerprint") != last.get("fingerprint"):
    lines.append("WATCHDOG: it was started against an older version of this code and is now watching something that has changed. Re-arm it with " + arm_cmd + ".")
identities={}
if isinstance(state.get("owner"),dict): identities["owner"]=state["owner"]
identities.update(state.get("components",{}))
for name,item in identities.items():
    try:
        ok=subprocess.run([helper,"alive","--pid",str(item["pid"]),"--start-time",str(item["pidStartedAt"])],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0
    except (KeyError,TypeError,OSError): ok=False
    if not ok: lines.append(f"WATCHDOG: its {name} part is not running. Re-arm it with {arm_cmd}.")
print("\n".join(lines[:20]))
PY
  )
  # Continuation is the one part of the loop no prompt can guarantee, so it is
  # checked here rather than asked for in prose: a turn ending while a plan
  # still names an unblocked next step and nothing is in flight says so.
  open_work=$(python3 "$script_dir/open-work.py" --repo "$harness_root" 2>/dev/null || true)
  [[ -z "$open_work" ]] || message=$(printf '%s%s%s' "$message" "${message:+$'\n'}" "$open_work")
  [[ -z "$protocol_message" ]] || message=$(printf '%s%s%s' "$message" "${message:+$'\n'}" "$protocol_message")

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

  # Busy-or-not is answered on every single turn end, warnings or not. A
  # warning used to REPLACE the answer, so a human asking "is it still
  # working?" got a sentence about watchdog internals instead. Warnings are
  # added to the answer now, never substituted for it.
  if [[ -n "$message" ]]; then
    surface_json "$(work_sentence)
$message"
  else
    # Say so when there is nothing to say. Silence is ambiguous to a human:
    # it reads the same whether work is finished, still running, or the hook
    # never fired at all. An explicit idle line distinguishes all three, and
    # it costs one sentence at the end of a turn.
    # No pipelines here: under pipefail a grep that matches nothing fails the
    # whole assignment, and set -e then kills the hook before it can report —
    # the silent-exit failure this very check exists to make visible.
    surface_json "$(work_sentence)"
  fi
  [[ -z "$main_id" || -z "$protocol_message" ]] || \
    "$lease_helper" --root "$harness_root" protocol-advance --main-id "$main_id" \
      --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
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
