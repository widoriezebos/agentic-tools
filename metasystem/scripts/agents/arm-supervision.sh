#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/arm-supervision.sh --repo <root> [--session <id>]
      [--pid <pid>] [--start-time <epoch>] [--tag <tag>]
  scripts/agents/arm-supervision.sh fingerprint --repo <root>

Arming order is fixed: announce the session process; acquire or join the
per-repository supervisor lock; start missing functions; wait for a complete
census; verify watcher, reaper, and census; print ARMED.

When session identity options are omitted, --pid is the immediate
agent-signature ancestor, --start-time is read from the census identity source,
--session is METASYSTEM_SESSION_ID or session-<pid>, and --tag is
METASYSTEM_INSTANCE_TAG or metasystem-main-<runtime>-<sanitized-session>. If no
agent-signature ancestor can be proven, arming refuses.
USAGE
}

die() { echo "$2" >&2; exit "$1"; }

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
script_path=$script_dir/$(basename "${BASH_SOURCE[0]}")
harness_root=$(cd "$script_dir/../.." && pwd -P)
helper=$script_dir/process-census.py
config=$harness_root/scripts/metasystem-config.sh
watcher=$harness_root/scripts/watch-background-jobs.sh
dispatch=$harness_root/scripts/agents/dispatch.sh
agents=$harness_root/artifacts/agents

now_iso() { date -u +%Y-%m-%dT%H:%M:%SZ; }

supervision_wait_cap() { # base seconds; fixture validation may export a scale
  local base=$1 scale_milli=${METASYSTEM_FIXTURE_CAP_SCALE_MILLI:-1000}
  [[ "$base" =~ ^[1-9][0-9]*$ && "$scale_milli" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "supervision wait cap inputs must be positive integers"
  printf '%s\n' "$(( (base * scale_milli + 999) / 1000 ))"
}

resolve_repo() {
  local supplied=$1 top
  top=$(git -C "$supplied" rev-parse --show-toplevel 2>/dev/null) \
    || die 2 "--repo is not inside a git repository: $supplied"
  (cd "$top" && pwd -P)
}

sanitize() {
  python3 - "$1" <<'PY'
import re, sys
value=re.sub(r"[^A-Za-z0-9._-]+", "-", sys.argv[1]).strip("-.").lower()
print(value or "session")
PY
}

json_field() { # file, dotted field
  python3 - "$1" "$2" <<'PY'
import json, sys
try:
    value=json.load(open(sys.argv[1]))
    for part in sys.argv[2].split("."): value=value[part]
except (OSError, ValueError, KeyError, TypeError): raise SystemExit(1)
if isinstance(value, bool): print("true" if value else "false")
elif isinstance(value, (dict, list)): print(json.dumps(value,separators=(",",":")))
elif value is not None: print(value)
PY
}

identity_alive() { # pid, start, optional tag
  local pid=$1 start=$2 tag=${3:-} command
  "$helper" alive --pid "$pid" --start-time "$start" >/dev/null 2>&1 || return 1
  [[ -z "$tag" ]] && return 0
  command=$(ps -p "$pid" -o command= 2>/dev/null || true)
  # Exact pid/start proves the recorded process is still live. If argv is not
  # observable, conservatively treat the owner as live: inability to inspect
  # a tag is never proof that permits takeover or signalling.
  if [[ -z "$command" ]]; then return 0; fi
  [[ "$command" == *"$tag"* ]]
}

atomic_json_identity() { # path, pid, start, tag, acquired-at
  python3 - "$@" <<'PY'
import json, os, sys, tempfile
from pathlib import Path
path=Path(sys.argv[1]); value={"pid":int(sys.argv[2]),"pidStartedAt":int(sys.argv[3]),"instanceTag":sys.argv[4],"acquiredAt":sys.argv[5]}
path.parent.mkdir(parents=True,exist_ok=True)
fd,tmp=tempfile.mkstemp(prefix=path.name+".",suffix=".tmp",dir=path.parent)
with os.fdopen(fd,"w") as h: json.dump(value,h,indent=2,sort_keys=True); h.write("\n"); h.flush(); os.fsync(h.fileno())
os.replace(tmp,path)
PY
}

write_announcement() { # repo, session, pid, start, tag, runtime
  python3 - "$@" <<'PY'
import fcntl, json, os, re, sys, tempfile
from datetime import datetime, timezone
from pathlib import Path
repo, session, pid, started, tag, runtime = Path(sys.argv[1]), sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), sys.argv[5], sys.argv[6]
safe=re.sub(r"[^A-Za-z0-9._-]+","-",session).strip("-.").lower() or "session"
directory=repo/"artifacts"/"agents"/"mains"; directory.mkdir(parents=True,exist_ok=True)
with (directory/".registry.lock").open("a+") as lock:
    fcntl.flock(lock.fileno(),fcntl.LOCK_EX)
    for path in directory.glob("*.json"):
        try: old=json.loads(path.read_text())
        except (OSError,ValueError): continue
        if old.get("pid")==pid and old.get("pidStartedAt")==started and path.name != f"{safe}-{pid}.json":
            path.unlink()
    value={"sessionId":session,"pid":pid,"pidStartedAt":started,"pgid":os.getpgid(pid),"runtime":runtime,"instanceTag":tag,"announcedAt":datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}
    output=directory/f"{safe}-{pid}.json"
    fd,tmp=tempfile.mkstemp(prefix=output.name+".",suffix=".tmp",dir=directory)
    with os.fdopen(fd,"w") as h: json.dump(value,h,indent=2,sort_keys=True); h.write("\n"); h.flush(); os.fsync(h.fileno())
    os.replace(tmp,output)
print(output)
PY
}

retire_announcement() { # repo, session, pid, start
  python3 - "$@" <<'PY'
import json, re, sys
from pathlib import Path
repo,session,pid,started=Path(sys.argv[1]),sys.argv[2],int(sys.argv[3]),int(sys.argv[4])
safe=re.sub(r"[^A-Za-z0-9._-]+","-",session).strip("-.").lower() or "session"
path=repo/"artifacts"/"agents"/"mains"/f"{safe}-{pid}.json"
try: value=json.loads(path.read_text())
except (OSError,ValueError): raise SystemExit(0)
if value.get("pid")==pid and value.get("pidStartedAt")==started: path.unlink()
PY
}

stop_identity() { # name, pid, start, tag
  local name=$1 pid=$2 start=$3 tag=$4 cap started deadline elapsed
  identity_alive "$pid" "$start" "$tag" || return 0
  kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
  cap=$(supervision_wait_cap 5)
  started=$SECONDS
  deadline=$((SECONDS + cap))
  while identity_alive "$pid" "$start" "$tag"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "supervision stop ceiling reached: $name pid=$pid (elapsed: ${elapsed}s; scaled cap: ${cap}s); sending KILL" >&2
      kill -KILL -- "-$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
      return 1
    fi
    sleep 0.05
  done
  wait "$pid" 2>/dev/null || true
}

launch_detached() { # output pid variable name, log path, command...
  local __name=$1 log=$2
  shift 2
  python3 - "$log" "$@" <<'PY' &
import os, sys
log=os.open(sys.argv[1], os.O_WRONLY|os.O_CREAT|os.O_APPEND, 0o600)
null=os.open(os.devnull, os.O_RDONLY)
os.dup2(null, 0); os.dup2(log, 1); os.dup2(log, 2)
os.setsid()
os.execv(sys.argv[2], sys.argv[2:])
PY
  printf -v "$__name" '%s' "$!"
}

wait_for_start_identity() { # name, pid
  local name=$1 pid=$2 cap started deadline elapsed value
  cap=$(supervision_wait_cap 5)
  started=$SECONDS
  deadline=$((SECONDS + cap))
  while (( SECONDS < deadline )); do
    if value=$("$helper" started-at --pid "$pid" 2>/dev/null); then printf '%s\n' "$value"; return 0; fi
    sleep 0.02
  done
  elapsed=$((SECONDS - started))
  echo "supervision start identity ceiling reached: $name pid=$pid (elapsed: ${elapsed}s; scaled cap: ${cap}s)" >&2
  return 1
}

read_component_identity() { # state, component => pid start tag
  python3 - "$1" "$2" <<'PY'
import json, sys
try: v=json.load(open(sys.argv[1]))["components"][sys.argv[2]]; print(v["pid"],v["pidStartedAt"],v["instanceTag"])
except (OSError,ValueError,KeyError,TypeError): raise SystemExit(1)
PY
}

stop_recorded_components() {
  local state=$1/artifacts/agents/supervision/state.json component pid start tag
  [[ -f "$state" ]] || return 0
  for component in watcher reaper; do
    read -r pid start tag < <(read_component_identity "$state" "$component" 2>/dev/null || true)
    [[ -n "${pid:-}" ]] && stop_identity "$component" "$pid" "$start" "$tag" || true
  done
}

run_owner() {
  local repo= gate= owner_tag= interval supervision state lock owner watcher_pid watcher_start reaper_pid reaper_start
  local watcher_tag reaper_tag generation=0 fingerprint component pid start tag stale gate_cap gate_started elapsed
  while (($#)); do
    case "$1" in
      --repo) repo=$2; shift 2 ;; --gate) gate=$2; shift 2 ;; --tag) owner_tag=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  [[ -n "$repo" && -n "$gate" && -n "$owner_tag" ]] || exit 2
  supervision=$agents/supervision
  state=$supervision/state.json
  lock=$supervision/lock.d
  owner=$lock/owner.json
  interval=$("$config" get --key watch.interval-sec --default 60)
  [[ "$interval" =~ ^[1-9][0-9]*$ ]] || exit 1
  gate_cap=$(supervision_wait_cap 10)
  gate_started=$SECONDS
  deadline=$((SECONDS + gate_cap))
  while [[ ! -e "$gate" ]]; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - gate_started))
      echo "supervision owner start-gate ceiling reached (elapsed: ${elapsed}s; scaled cap: ${gate_cap}s)" >&2
      exit 1
    fi
    sleep 0.02
  done
  rm -f "$gate"

  cleanup_owner() {
    stop_recorded_components "$harness_root"
    if [[ -f "$owner" ]] && [[ "$(json_field "$owner" pid 2>/dev/null || true)" == "$$" ]]; then
      rm -f "$owner"
      rmdir "$lock" 2>/dev/null || true
    fi
  }
  trap cleanup_owner EXIT
  trap 'exit 0' TERM INT

  launch_set() {
    local suffix watcher_heartbeat=$supervision/watcher.heartbeat.json reaper_heartbeat=$supervision/reaper.heartbeat.json
    stop_recorded_components "$harness_root"
    generation=$((generation + 1))
    suffix="$generation-$(date +%s)"
    watcher_tag="$owner_tag-watcher-$suffix"
    reaper_tag="$owner_tag-reaper-$suffix"
    launch_detached watcher_pid "$supervision/watcher.log" "$watcher" \
      --dir "$agents/jobs" --scope "$repo" --state "$supervision/jobs.state" \
      --interval "$interval" --census --supervision-dir "$supervision" \
      --heartbeat "$watcher_heartbeat" --instance-tag "$watcher_tag"
    watcher_start=$(wait_for_start_identity watcher "$watcher_pid") || exit 1
    launch_detached reaper_pid "$supervision/reaper.log" "$dispatch" reap --interval "$interval" \
      --heartbeat "$reaper_heartbeat" --instance-tag "$reaper_tag"
    reaper_start=$(wait_for_start_identity reaper "$reaper_pid") || exit 1
    python3 - "$state" "$$" "$("$helper" started-at --pid "$$")" "$owner_tag" \
      "$watcher_pid" "$watcher_start" "$watcher_tag" "$watcher_heartbeat" \
      "$reaper_pid" "$reaper_start" "$reaper_tag" "$reaper_heartbeat" "$interval" "$generation" <<'PY'
import json, os, sys, tempfile
from datetime import datetime, timezone
from pathlib import Path
args=sys.argv[1:]; output=Path(args[0]); owner_pid,owner_start,owner_tag=int(args[1]),int(args[2]),args[3]
w_pid,w_start,w_tag,w_hb=int(args[4]),int(args[5]),args[6],args[7]
r_pid,r_start,r_tag,r_hb=int(args[8]),int(args[9]),args[10],args[11]
interval,generation=int(args[12]),int(args[13])
value={"schemaVersion":1,"owner":{"pid":owner_pid,"pidStartedAt":owner_start,"instanceTag":owner_tag},"components":{"watcher":{"pid":w_pid,"pidStartedAt":w_start,"instanceTag":w_tag,"heartbeat":w_hb},"reaper":{"pid":r_pid,"pidStartedAt":r_start,"instanceTag":r_tag,"heartbeat":r_hb}},"intervalSec":interval,"generation":generation,"fingerprint":None,"startedAt":datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}
output.parent.mkdir(parents=True,exist_ok=True); fd,tmp=tempfile.mkstemp(prefix=output.name+".",suffix=".tmp",dir=output.parent)
with os.fdopen(fd,"w") as h: json.dump(value,h,indent=2,sort_keys=True); h.write("\n"); h.flush(); os.fsync(h.fileno())
os.replace(tmp,output)
PY
    fingerprint=$("$helper" fingerprint --repo "$repo") || exit 1
    python3 - "$state" "$fingerprint" <<'PY'
import json,os,sys,tempfile
from pathlib import Path
p=Path(sys.argv[1]); v=json.loads(p.read_text()); v["fingerprint"]=sys.argv[2]
fd,t=tempfile.mkstemp(prefix=p.name+".",suffix=".tmp",dir=p.parent)
with os.fdopen(fd,"w") as h: json.dump(v,h,indent=2,sort_keys=True); h.write("\n"); h.flush(); os.fsync(h.fileno())
os.replace(t,p)
PY
  }

  launch_set
  while true; do
    stale=
    for component in watcher reaper; do
      read -r pid start tag < <(read_component_identity "$state" "$component") || { stale=$component; break; }
      if ! identity_alive "$pid" "$start" "$tag"; then stale=$component; break; fi
      heartbeat=$(json_field "$state" "components.$component.heartbeat")
      observed=$(json_field "$heartbeat" observedAtEpoch 2>/dev/null || echo 0)
      (( $(date +%s) - observed <= interval * 2 + 2 )) || { stale=$component; break; }
    done
    if [[ -n "$stale" ]]; then
      printf '%s STALE-SUPERVISOR component=%s generation=%s\n' "$(now_iso)" "$stale" "$generation" >>"$supervision/supervisor.log"
      launch_set
    fi
    sleep "$interval"
  done
}

verify_armed() { # repo, owner pid/start/tag
  local repo=$1 owner_pid=$2 owner_start=$3 owner_tag=$4 supervision state last interval cap started deadline elapsed component pid start tag heartbeat observed expected actual verdict completed
  supervision=$agents/supervision; state=$supervision/state.json; last=$supervision/last-census.json
  interval=$("$config" get --key watch.interval-sec --default 60)
  cap=$(supervision_wait_cap "$((interval + 10))")
  started=$SECONDS
  deadline=$((SECONDS + cap))
  while (( SECONDS < deadline )); do
    if identity_alive "$owner_pid" "$owner_start" "$owner_tag" && [[ -f "$state" && -f "$last" ]]; then
      functions_live=1
      for component in watcher reaper; do
        read -r pid start tag < <(read_component_identity "$state" "$component" 2>/dev/null || true)
        if [[ -z "${pid:-}" ]] || ! identity_alive "$pid" "$start" "$tag"; then functions_live=0; break; fi
        heartbeat=$(json_field "$state" "components.$component.heartbeat" 2>/dev/null || true)
        observed=$(json_field "$heartbeat" observedAtEpoch 2>/dev/null || echo 0)
        (( $(date +%s) - observed <= interval * 2 + 2 )) || { functions_live=0; break; }
      done
      if (( functions_live )); then
        verdict=$(json_field "$last" verdict 2>/dev/null || true)
        completed=$(json_field "$last" completedAtEpoch 2>/dev/null || echo 0)
        actual=$(json_field "$last" fingerprint 2>/dev/null || true)
        expected=$(json_field "$state" fingerprint 2>/dev/null || true)
        if [[ "$verdict" == SUCCESS && -n "$expected" && "$actual" == "$expected" ]] && (( $(date +%s) - completed <= interval )); then
          return 0
        fi
      fi
    fi
    sleep 0.05
  done
  elapsed=$((SECONDS - started))
  echo "supervision arming timed out: first complete census (elapsed: ${elapsed}s; scaled cap: ${cap}s): watcher, reaper, and a fresh successful census did not verify" >&2
  return 1
}

arm_repository() {
  local repo= session= pid= start= tag= runtime=${METASYSTEM_AGENT_RUNTIME:-} retire=0 shutdown=0 ancestor safe announcement
  local owner_cap owner_started owner_deadline elapsed expected_owner_prefix
  while (($#)); do
    case "$1" in
      --repo) [[ $# -ge 2 ]] || { usage; exit 2; }; repo=$2; shift 2 ;;
      --session) [[ $# -ge 2 ]] || { usage; exit 2; }; session=$2; shift 2 ;;
      --pid) [[ $# -ge 2 ]] || { usage; exit 2; }; pid=$2; shift 2 ;;
      --start-time) [[ $# -ge 2 ]] || { usage; exit 2; }; start=$2; shift 2 ;;
      --tag) [[ $# -ge 2 ]] || { usage; exit 2; }; tag=$2; shift 2 ;;
      --retire) retire=1; shift ;; --shutdown) shutdown=1; shift ;;
      -h|--help) usage; exit 0 ;; *) usage; exit 2 ;;
    esac
  done
  [[ -n "$repo" ]] || { usage; exit 2; }
  repo=$(resolve_repo "$repo")
  [[ -x "$helper" ]] || die 1 "process census helper is not executable"
  if (( shutdown )); then
    lock=$agents/supervision/lock.d/owner.json
    [[ -f "$lock" ]] || exit 0
    owner_pid=$(json_field "$lock" pid); owner_start=$(json_field "$lock" pidStartedAt); owner_tag=$(json_field "$lock" instanceTag)
    # The tag carries the repository the owner was armed for. A record copied
    # from another checkout names a live process this repository does not own,
    # so shutting it down would kill a stranger's supervisor: refuse instead.
    expected_owner_prefix="metasystem-supervision-owner-$(sanitize "$(git -C "$harness_root" rev-parse --show-toplevel 2>/dev/null || true)")-"
    [[ "$owner_tag" == "$expected_owner_prefix"* ]] \
      || die 1 "supervision lock names an owner armed for another repository ($owner_tag); refusing to stop a process this repository does not own"
    stop_identity owner "$owner_pid" "$owner_start" "$owner_tag"
    exit 0
  fi
  if [[ -z "$pid" ]]; then
    if ! ancestor=$("$helper" find-ancestor --repo "$repo" --pid "$PPID" ${runtime:+--runtime "$runtime"} 2>&1); then
      die 1 "cannot infer arming identity: no agent-signature ancestor was proven. Pass --pid <agent-pid> and --start-time <epoch-seconds>, or run from a session whose ancestor matches a configured runtime signature. Detail: $ancestor"
    fi
    pid=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["pid"])' "$ancestor")
    [[ -n "$runtime" ]] || runtime=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["runtime"])' "$ancestor")
  fi
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || die 2 "--pid must be a positive integer"
  start=${start:-$("$helper" started-at --pid "$pid")} || die 1 "cannot read pid start time"
  [[ "$start" =~ ^[1-9][0-9]*$ ]] || die 2 "--start-time must be epoch seconds"
  identity_alive "$pid" "$start" || die 1 "announcement pid identity is not live"
  session=${session:-${METASYSTEM_SESSION_ID:-session-$pid}}
  runtime=${runtime:-unknown}
  safe=$(sanitize "$session")
  tag=${tag:-${METASYSTEM_INSTANCE_TAG:-metasystem-main-$runtime-$safe}}
  [[ -n "$tag" ]] || die 2 "--tag cannot be empty"
  if (( retire )); then retire_announcement "$harness_root" "$session" "$pid" "$start"; exit 0; fi

  # Fixed arming step 1: registry write precedes lock acquisition and census.
  announcement=$(write_announcement "$harness_root" "$session" "$pid" "$start" "$tag" "$runtime")
  supervision=$agents/supervision
  mkdir -p "$supervision"
  printf '%s announcement-written registry=%s pid=%s start=%s\n' "$(now_iso)" "$announcement" "$pid" "$start" >>"$supervision/arming.log"

  lock_dir=$supervision/lock.d; owner_file=$lock_dir/owner.json
  owner_tag="metasystem-supervision-owner-$(sanitize "$(git -C "$repo" rev-parse --show-toplevel)")-$(date +%s)-$$"
  if mkdir "$lock_dir" 2>/dev/null; then
    # The process is launched only after this arming call owns the repository
    # lock. This preserves the fixed order and avoids speculative supervisors.
    gate=$supervision/owner-gate.$$.$RANDOM
    launch_detached candidate_pid "$supervision/owner.log" "$script_path" __owner --repo "$repo" --gate "$gate" --tag "$owner_tag"
    candidate_start=$(wait_for_start_identity owner-candidate "$candidate_pid") || {
      rmdir "$lock_dir" 2>/dev/null || true
      die 1 "could not start supervision owner"
    }
    atomic_json_identity "$owner_file" "$candidate_pid" "$candidate_start" "$owner_tag" "$(now_iso)"
    touch "$gate"
    owner_pid=$candidate_pid; owner_start=$candidate_start
  else
    # mkdir and owner.json cannot be one filesystem operation. Give the lock
    # winner a bounded publication window; an ownerless lock still refuses
    # because no process identity exists to prove dead.
    owner_cap=$(supervision_wait_cap 5)
    owner_started=$SECONDS
    owner_deadline=$((SECONDS + owner_cap))
    while [[ ! -f "$owner_file" && $SECONDS -lt $owner_deadline ]]; do sleep 0.02; done
    if [[ ! -f "$owner_file" ]]; then
      elapsed=$((SECONDS - owner_started))
      die 1 "supervision lock join timed out: owner identity (elapsed: ${elapsed}s; scaled cap: ${owner_cap}s); refusing unproven takeover"
    fi
    owner_pid=$(json_field "$owner_file" pid) || die 1 "supervision lock owner is malformed"
    owner_start=$(json_field "$owner_file" pidStartedAt) || die 1 "supervision lock owner is malformed"
    existing_tag=$(json_field "$owner_file" instanceTag) || die 1 "supervision lock owner is malformed"
    if identity_alive "$owner_pid" "$owner_start" "$existing_tag"; then
      owner_tag=$existing_tag
    else
      # Takeover is legal only after exact pid+start identity is proven dead.
      stop_recorded_components "$harness_root"
      rm "$owner_file"
      rmdir "$lock_dir" || die 1 "supervision lock takeover lost a race"
      mkdir "$lock_dir" || die 1 "supervision lock takeover lost a race"
      gate=$supervision/owner-gate.$$.$RANDOM
      launch_detached candidate_pid "$supervision/owner.log" "$script_path" __owner --repo "$repo" --gate "$gate" --tag "$owner_tag"
      candidate_start=$(wait_for_start_identity takeover-owner "$candidate_pid") || {
        rmdir "$lock_dir" 2>/dev/null || true
        die 1 "could not start takeover owner"
      }
      atomic_json_identity "$owner_file" "$candidate_pid" "$candidate_start" "$owner_tag" "$(now_iso)"
      touch "$gate"
      owner_pid=$candidate_pid; owner_start=$candidate_start
    fi
  fi
  verify_armed "$repo" "$owner_pid" "$owner_start" "$owner_tag" || exit 1
  printf '%s ARMED repo=%s owner=%s start=%s tag=%s announcement=%s\n' "$(now_iso)" "$repo" "$owner_pid" "$owner_start" "$owner_tag" "$announcement"
}

case ${1:-} in
  __owner) shift; run_owner "$@" ;;
  fingerprint)
    shift
    [[ ${1:-} == --repo && $# -eq 2 ]] || { usage; exit 2; }
    repo=$(resolve_repo "$2")
    "$helper" fingerprint --repo "$repo"
    ;;
  *) arm_repository "$@" ;;
esac
