#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/arm-supervision.sh --repo <root> [--session <id>]
      [--pid <pid>] [--start-time <epoch>] [--tag <tag>]
      [--rearm] [--max-cap <minutes>]
  scripts/agents/arm-supervision.sh fingerprint --repo <root>

Arming order is fixed: announce the session process; acquire or join the
per-repository supervisor lock; start missing functions; wait for a complete
census; verify watcher, reaper, and census; print ARMED.

An ordinary arm joins a live supervisor and never changes its ceiling.
--rearm replaces the live set after refusing any derived ceiling below a
currently reserved delegate-job cap. --max-cap participates in the config-only
ceiling derivation; the loaded watcher ceiling is the maximum cap plus 30.

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
lease_helper=$script_dir/worktree-lease.py
config=$harness_root/scripts/metasystem-config.sh
watcher=$harness_root/scripts/watch-background-jobs.sh
dispatch=$harness_root/scripts/agents/dispatch.sh
agents=$harness_root/artifacts/agents
cap_authority_lock_held=0

now_iso() { date -u +%Y-%m-%dT%H:%M:%SZ; }

derive_watcher_ceiling() { # optional declared maximum cap
  local declared=${1:-} key value maximum=120
  if [[ -n "$declared" ]]; then
    [[ "$declared" =~ ^[1-9][0-9]*$ ]] || die 2 "--max-cap must be a positive integer"
    (( declared > maximum )) && maximum=$declared
  fi
  value=$("$config" get --key dispatch.cap-min --default 120)
  [[ "$value" =~ ^[1-9][0-9]*$ ]] || die 1 "dispatch.cap-min must be a positive integer"
  (( value > maximum )) && maximum=$value
  value=$("$config" get --key fence.job-cap-min --default '')
  if [[ -n "$value" ]]; then
    [[ "$value" =~ ^[1-9][0-9]*$ ]] || die 1 "fence.job-cap-min must be a positive integer"
    (( value > maximum )) && maximum=$value
  fi
  while IFS= read -r key; do
    [[ "$key" == cap.min.* ]] || continue
    value=$("$config" get --key "$key" --default '')
    [[ "$value" =~ ^[1-9][0-9]*$ ]] || die 1 "$key must be a positive integer"
    (( value > maximum )) && maximum=$value
  done < <("$config" keys --prefix cap.min.)
  while IFS='=' read -r key value; do
    [[ "$key" == METASYSTEM_CAP_MIN_* ]] || continue
    [[ "$value" =~ ^[1-9][0-9]*$ ]] || die 1 "$key must be a positive integer"
    (( value > maximum )) && maximum=$value
  done < <(env)
  printf '%s\n' "$((maximum + 30))"
}

blocking_reserved_cap() { # proposed watcher ceiling; prints job|cap for first blocker
  python3 - "$agents" "$1" <<'PY'
import json, sys
from pathlib import Path
agents, ceiling = Path(sys.argv[1]), int(sys.argv[2])
terminal = {"completed", "failed", "timeout", "cancelled"}
reserved = {}
jobs = agents / "jobs"
for path in sorted(jobs.glob("*.json")) if jobs.exists() else []:
    try: value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError): continue
    cap, status, job = value.get("capMin"), value.get("status"), value.get("jobId", path.stem)
    if type(cap) is int and cap >= ceiling and status not in terminal:
        reserved[str(job)] = cap
missions = agents / "missions"
for path in sorted(missions.glob("*/fences.json")) if missions.exists() else []:
    try: value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError): continue
    for job, reservation in value.get("reservations", {}).items():
        cap = reservation.get("capMin") if isinstance(reservation, dict) else None
        if type(cap) is not int or cap < ceiling:
            continue
        try: status = json.loads((jobs / f"{job}.json").read_text(encoding="utf-8")).get("status")
        except (OSError, ValueError): status = None
        if status not in terminal:
            reserved[str(job)] = max(cap, reserved.get(str(job), 0))
if reserved:
    job = sorted(reserved, key=lambda item: (-reserved[item], item))[0]
    print(f"{job}|{reserved[job]}")
PY
}

supervision_wait_cap() { # base seconds; fixture validation may export a scale
  local base=$1 scale_milli=${METASYSTEM_FIXTURE_CAP_SCALE_MILLI:-1000}
  [[ "$base" =~ ^[1-9][0-9]*$ && "$scale_milli" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "supervision wait cap inputs must be positive integers"
  printf '%s\n' "$(( (base * scale_milli + 999) / 1000 ))"
}

acquire_cap_authority_lock() {
  local directory="$agents/supervision/cap-authority.lock.d" maximum started deadline elapsed
  mkdir -p "${directory%/*}"
  maximum=$(supervision_wait_cap 10)
  started=$SECONDS
  deadline=$((SECONDS + maximum))
  while ! mkdir "$directory" 2>/dev/null; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      die 1 "timed out acquiring repository cap-authority lock (elapsed: ${elapsed}s; scaled cap: ${maximum}s)"
    fi
    sleep 0.05
  done
  cap_authority_lock_held=1
}

release_cap_authority_lock() {
  (( cap_authority_lock_held )) || return 0
  rmdir "$agents/supervision/cap-authority.lock.d" 2>/dev/null \
    || die 1 "repository cap-authority lock disappeared or is not empty"
  cap_authority_lock_held=0
}

milliseconds_to_sleep() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "supervision interval must be a positive integer in milliseconds"
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
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

# An adopted or fixture copy may lack the emitter; a failed `source` under
# set -e kills the script before || can catch it, so test first.
if [[ -f "$(dirname "${BASH_SOURCE[0]}")/emit-event.sh" ]]; then
  source "$(dirname "${BASH_SOURCE[0]}")/emit-event.sh"
else
  emit_event() { :; }
fi

rotate_event_stream() { # harness root -- only on the ESTABLISHING path (D-4)
  local stream="$1/artifacts/agents/events.jsonl" archive_dir="$1/artifacts/agents/events-archive"
  [[ -s "$stream" ]] || return 0
  mkdir -p "$archive_dir"
  local stamp name n=1
  stamp=$(date -u +%Y%m%dT%H%M%SZ)
  name="$archive_dir/events-$stamp-$$.jsonl"
  while [[ -e "$name" ]]; do n=$((n+1)); name="$archive_dir/events-$stamp-$$-$n.jsonl"; done
  mv "$stream" "$name" 2>/dev/null || return 0
  METASYSTEM_HARNESS_ROOT="$1" emit_event arming stream-rotated "previousPath=${name#"$1"/}" "summary=rotated at arming"
}

write_announcement() { # repo, session, pid, start, tag, runtime, optional lineage
  if [[ -n "${7:-}" ]]; then
    "$lease_helper" --root "$1" announce --session "$2" --pid "$3" \
      --start "$4" --tag "$5" --runtime "$6" --owner-lineage "$7"
  else
    "$lease_helper" --root "$1" announce --session "$2" --pid "$3" \
      --start "$4" --tag "$5" --runtime "$6"
  fi
}

retire_announcement() { # repo, session, pid, start
  "$lease_helper" --root "$1" retire --session "$2" --pid "$3" --start "$4"
}

stop_identity() { # name, pid, start, tag
  local name=$1 pid=$2 start=$3 tag=$4 cap started deadline elapsed kill_cap
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
      kill_cap=$(supervision_wait_cap 1)
      deadline=$((SECONDS + kill_cap))
      while identity_alive "$pid" "$start" "$tag" && (( SECONDS < deadline )); do sleep 0.05; done
      identity_alive "$pid" "$start" "$tag" && return 1
      wait "$pid" 2>/dev/null || true
      return 0
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

# A launched component is not yet a supervising one. The staleness rule reads
# heartbeats, and a component that has not written its first heartbeat looks
# exactly like one that stopped writing them -- so a launch that returns at
# spawn lets the supervisor declare its own newborn component stale and replace
# it. Every replacement stops the running set, and each of those gaps is a
# moment with no census writer at all. A launch therefore completes only once
# both components have heartbeat under the identity it just started.
#
# Waiting for a heartbeat is not waiting for a live process, so this also stops
# the moment the component dies: a process killed before its first heartbeat
# never produces one, and blocking for the full ceiling on it would strand the
# supervisor exactly when it is needed to replace the set.
wait_for_first_heartbeat() { # name, heartbeat file, instance tag, pid, start
  local name=$1 file=$2 tag=$3 pid=$4 start=$5 cap started deadline elapsed
  cap=$(supervision_wait_cap 5)
  started=$SECONDS
  deadline=$((SECONDS + cap))
  while (( SECONDS < deadline )); do
    [[ "$(json_field "$file" instanceTag 2>/dev/null || true)" == "$tag" ]] && return 0
    if ! identity_alive "$pid" "$start" "$tag"; then
      echo "supervision component died before its first heartbeat: $name pid=$pid" >&2
      return 1
    fi
    sleep 0.02
  done
  elapsed=$((SECONDS - started))
  echo "supervision first-heartbeat ceiling reached: $name (elapsed: ${elapsed}s; scaled cap: ${cap}s)" >&2
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
  local repo= gate= owner_tag= watcher_cap= interval interval_ms interval_sleep supervision state lock owner watcher_pid watcher_start reaper_pid reaper_start
  local watcher_tag reaper_tag generation=0 fingerprint component pid start tag stale gate_cap gate_started elapsed
  while (($#)); do
    case "$1" in
      --repo) repo=$2; shift 2 ;; --gate) gate=$2; shift 2 ;; --tag) owner_tag=$2; shift 2 ;;
      --watcher-cap) watcher_cap=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  [[ -n "$repo" && -n "$gate" && -n "$owner_tag" && "$watcher_cap" =~ ^[1-9][0-9]*$ ]] || exit 2
  supervision=$agents/supervision
  state=$supervision/state.json
  lock=$supervision/lock.d
  owner=$lock/owner.json
  interval=$("$config" get --key watch.interval-sec --default 60)
  [[ "$interval" =~ ^[1-9][0-9]*$ ]] || exit 1
  interval_ms=${METASYSTEM_WATCH_POLL_INTERVAL_MS:-$((interval * 1000))}
  interval_sleep=$(milliseconds_to_sleep "$interval_ms")
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

  if [[ -f "$state" ]]; then
    generation=$(json_field "$state" generation 2>/dev/null) \
      || { echo "supervision state has no readable generation" >&2; exit 1; }
    [[ "$generation" =~ ^[0-9]+$ ]] \
      || { echo "supervision state generation is not a non-negative integer" >&2; exit 1; }
  fi

  launch_set() {
    local suffix watcher_heartbeat=$supervision/watcher.heartbeat.json reaper_heartbeat=$supervision/reaper.heartbeat.json reaper_gate
    # (scrub moved to arm_repository entry -- FRCC-006: lease events emitted
    # during arming must not carry the id either)
    if [[ "${1:-}" == --establishing ]]; then
      METASYSTEM_HARNESS_ROOT="$harness_root" emit_event arming arming-started "summary=establishing launch_set"
      rotate_event_stream "$harness_root"
    fi
    stop_recorded_components "$harness_root"
    generation=$((generation + 1))
    suffix="$generation-$(date +%s)"
    watcher_tag="$owner_tag-watcher-$suffix"
    reaper_tag="$owner_tag-reaper-$suffix"
    reaper_gate="$supervision/reaper-start-gate-$suffix"
    launch_detached watcher_pid "$supervision/watcher.log" "$watcher" \
      --dir "$agents/jobs" --scope "$repo" --state "$supervision/jobs.state" \
      --interval "$interval" --census --supervision-dir "$supervision" \
      --heartbeat "$watcher_heartbeat" --ceiling-state "$state" --expected-cap "$watcher_cap" \
      --instance-tag "$watcher_tag"
    watcher_start=$(wait_for_start_identity watcher "$watcher_pid") || exit 1
    launch_detached reaper_pid "$supervision/reaper.log" "$dispatch" reap --interval "$interval" \
      --heartbeat "$reaper_heartbeat" --instance-tag "$reaper_tag" --start-gate "$reaper_gate"
    reaper_start=$(wait_for_start_identity reaper "$reaper_pid") || exit 1
    fingerprint=$("$helper" fingerprint --repo "$repo") || exit 1
    python3 - "$state" "$$" "$("$helper" started-at --pid "$$")" "$owner_tag" \
      "$watcher_pid" "$watcher_start" "$watcher_tag" "$watcher_heartbeat" \
      "$reaper_pid" "$reaper_start" "$reaper_tag" "$reaper_heartbeat" "$interval" "$generation" "$fingerprint" "$watcher_cap" <<'PY'
import json, os, sys, tempfile
from datetime import datetime, timezone
from pathlib import Path
args=sys.argv[1:]; output=Path(args[0]); owner_pid,owner_start,owner_tag=int(args[1]),int(args[2]),args[3]
w_pid,w_start,w_tag,w_hb=int(args[4]),int(args[5]),args[6],args[7]
r_pid,r_start,r_tag,r_hb=int(args[8]),int(args[9]),args[10],args[11]
interval,generation,fingerprint,watcher_cap=int(args[12]),int(args[13]),args[14],int(args[15])
value={"schemaVersion":1,"owner":{"pid":owner_pid,"pidStartedAt":owner_start,"instanceTag":owner_tag},"components":{"watcher":{"pid":w_pid,"pidStartedAt":w_start,"instanceTag":w_tag,"heartbeat":w_hb},"reaper":{"pid":r_pid,"pidStartedAt":r_start,"instanceTag":r_tag,"heartbeat":r_hb}},"intervalSec":interval,"generation":generation,"fingerprint":fingerprint,"derivedWatcherCapMin":watcher_cap,"startedAt":datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}
output.parent.mkdir(parents=True,exist_ok=True); fd,tmp=tempfile.mkstemp(prefix=output.name+".",suffix=".tmp",dir=output.parent)
with os.fdopen(fd,"w") as h: json.dump(value,h,indent=2,sort_keys=True); h.write("\n"); h.flush(); os.fsync(h.fileno())
os.replace(tmp,output)
PY
    touch "$reaper_gate"
    # An incomplete launch is not fatal: the supervision loop below detects the
    # component that never reported and replaces the set on its next tick.
    wait_for_first_heartbeat watcher "$watcher_heartbeat" "$watcher_tag" "$watcher_pid" "$watcher_start" || return 1
    wait_for_first_heartbeat reaper "$reaper_heartbeat" "$reaper_tag" "$reaper_pid" "$reaper_start" || return 1
  }

  launch_set --establishing && METASYSTEM_HARNESS_ROOT="$harness_root" emit_event arming arming-complete "summary=components launched" || true
  [[ -z "${METASYSTEM_WATCH_POLL_INTERVAL_MS:-}" ]] || sleep "$interval_sleep"
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
      launch_set || true
    fi
    sleep "$interval_sleep"
  done
}

verify_armed() { # repo, owner pid/start/tag
  local repo=$1 owner_pid=$2 owner_start=$3 owner_tag=$4 supervision state last interval cap started deadline elapsed component pid start tag heartbeat observed expected actual verdict completed expected_generation actual_generation derived_cap loaded_cap
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
        if [[ "$component" == watcher ]]; then
          derived_cap=$(json_field "$state" derivedWatcherCapMin 2>/dev/null || true)
          loaded_cap=$(json_field "$heartbeat" loadedCapMin 2>/dev/null || true)
          [[ "$derived_cap" =~ ^[1-9][0-9]*$ && "$loaded_cap" == "$derived_cap" ]] \
            || { functions_live=0; break; }
        fi
      done
      if (( functions_live )); then
        verdict=$(json_field "$last" verdict 2>/dev/null || true)
        completed=$(json_field "$last" completedAtEpoch 2>/dev/null || echo 0)
        actual=$(json_field "$last" fingerprint 2>/dev/null || true)
        expected=$(json_field "$state" fingerprint 2>/dev/null || true)
        actual_generation=$(json_field "$last" generation 2>/dev/null || true)
        expected_generation=$(json_field "$state" generation 2>/dev/null || true)
        if [[ "$verdict" == SUCCESS && -n "$expected" && "$actual" == "$expected" \
          && -n "$expected_generation" && "$actual_generation" == "$expected_generation" ]] \
          && (( $(date +%s) - completed <= interval )); then
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
  # Supervision and everything arming does NEVER carry the driver's execution
  # id: a joined watcher cannot receive one, so none may depend on timing --
  # and the lease events emitted DURING arming must not leak it (FRCC-006).
  unset METASYSTEM_EXECUTION_ID
  local repo= session= pid= start= tag= runtime=${METASYSTEM_AGENT_RUNTIME:-} retire=0 shutdown=0 lease_held=0 rearm=0 max_cap= watcher_cap= blocker= ancestor safe announcement
  local owner_lineage=${METASYSTEM_OWNER_LINEAGE:-}
  local owner_cap owner_started owner_deadline elapsed expected_owner_prefix
  while (($#)); do
    case "$1" in
      --repo) [[ $# -ge 2 ]] || { usage; exit 2; }; repo=$2; shift 2 ;;
      --session) [[ $# -ge 2 ]] || { usage; exit 2; }; session=$2; shift 2 ;;
      --pid) [[ $# -ge 2 ]] || { usage; exit 2; }; pid=$2; shift 2 ;;
      --start-time) [[ $# -ge 2 ]] || { usage; exit 2; }; start=$2; shift 2 ;;
      --tag) [[ $# -ge 2 ]] || { usage; exit 2; }; tag=$2; shift 2 ;;
      --retire) retire=1; shift ;; --shutdown) shutdown=1; shift ;;
      --lease-held) lease_held=1; shift ;;
      --owner-lineage) [[ $# -ge 2 ]] || { usage; exit 2; }; owner_lineage=$2; shift 2 ;;
      --rearm) rearm=1; shift ;;
      --max-cap) [[ $# -ge 2 ]] || { usage; exit 2; }; max_cap=$2; shift 2 ;;
      -h|--help) usage; exit 0 ;; *) usage; exit 2 ;;
    esac
  done
  [[ -n "$repo" ]] || { usage; exit 2; }
  repo=$(resolve_repo "$repo")
  [[ -x "$helper" && -x "$lease_helper" ]] \
    || die 1 "process census or checkout lease helper is not executable"
  if (( shutdown )); then
    if (( ! lease_held )); then
      lease_result=$("$lease_helper" --root "$harness_root" require-holder --caller-pid "$$") || exit $?
      lease_epoch=$(python3 -c 'import json,sys; v=json.loads(sys.argv[1]); print("" if v.get("claimEpoch") is None else v["claimEpoch"])' "$lease_result")
      if [[ -n "$lease_epoch" ]]; then
        exec "$lease_helper" --root "$harness_root" run-held --caller-pid "$$" \
          --expected-epoch "$lease_epoch" -- "$script_path" --repo "$repo" --shutdown --lease-held
      fi
      exec "$lease_helper" --root "$harness_root" run-held --caller-pid "$$" -- \
        "$script_path" --repo "$repo" --shutdown --lease-held
    fi
    "$lease_helper" --root "$harness_root" require-holder --caller-pid "$$" >/dev/null
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
  watcher_cap=$(derive_watcher_ceiling "$max_cap")

  # Fixed arming step 1: registry write precedes lock acquisition and census.
  announcement=$(write_announcement "$harness_root" "$session" "$pid" "$start" "$tag" "$runtime" "$owner_lineage")
  "$lease_helper" --root "$harness_root" require-holder --caller-pid "$pid" >/dev/null
  supervision=$agents/supervision
  mkdir -p "$supervision"
  printf '%s announcement-written registry=%s pid=%s start=%s\n' "$(now_iso)" "$announcement" "$pid" "$start" >>"$supervision/arming.log"

  lock_dir=$supervision/lock.d; owner_file=$lock_dir/owner.json
  owner_tag="metasystem-supervision-owner-$(sanitize "$(git -C "$repo" rev-parse --show-toplevel)")-$(date +%s)-$$"
  trap 'release_cap_authority_lock' EXIT
  acquire_cap_authority_lock
  if (( rearm )); then
    blocker=$(blocking_reserved_cap "$watcher_cap")
    if [[ -n "$blocker" ]]; then
      IFS='|' read -r blocking_job blocking_cap <<<"$blocker"
      die 1 "supervision re-arm refused: derived ${watcher_cap}m ceiling is not strictly above reserved cap ${blocking_cap}m for job $blocking_job"
    fi
  fi
  if (( rearm )) && [[ -f "$owner_file" ]]; then
    owner_pid=$(json_field "$owner_file" pid) || die 1 "supervision lock owner is malformed"
    owner_start=$(json_field "$owner_file" pidStartedAt) || die 1 "supervision lock owner is malformed"
    existing_tag=$(json_field "$owner_file" instanceTag) || die 1 "supervision lock owner is malformed"
    identity_alive "$owner_pid" "$owner_start" "$existing_tag" \
      || die 1 "supervision re-arm refused: existing owner identity is not live"
    stop_identity owner "$owner_pid" "$owner_start" "$existing_tag" \
      || die 1 "supervision re-arm refused: existing owner did not stop; replacement was not established"
  fi
  if mkdir "$lock_dir" 2>/dev/null; then
    blocker=$(blocking_reserved_cap "$watcher_cap")
    if [[ -n "$blocker" ]]; then
      IFS='|' read -r blocking_job blocking_cap <<<"$blocker"
      rmdir "$lock_dir" 2>/dev/null || true
      die 1 "supervision establishment refused: derived ${watcher_cap}m ceiling is not strictly above reserved cap ${blocking_cap}m for job $blocking_job"
    fi
    # The process is launched only after this arming call owns the repository
    # lock. This preserves the fixed order and avoids speculative supervisors.
    gate=$supervision/owner-gate.$$.$RANDOM
    launch_detached candidate_pid "$supervision/owner.log" "$script_path" __owner --repo "$repo" --gate "$gate" --tag "$owner_tag" --watcher-cap "$watcher_cap"
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
      (( ! rearm )) \
        || die 1 "supervision re-arm refused: another live owner won replacement; refusing to join it"
      owner_tag=$existing_tag
    else
      # Takeover is legal only after exact pid+start identity is proven dead.
      blocker=$(blocking_reserved_cap "$watcher_cap")
      if [[ -n "$blocker" ]]; then
        IFS='|' read -r blocking_job blocking_cap <<<"$blocker"
        die 1 "supervision takeover refused: derived ${watcher_cap}m ceiling is not strictly above reserved cap ${blocking_cap}m for job $blocking_job"
      fi
      stop_recorded_components "$harness_root"
      rm "$owner_file"
      rmdir "$lock_dir" || die 1 "supervision lock takeover lost a race"
      mkdir "$lock_dir" || die 1 "supervision lock takeover lost a race"
      gate=$supervision/owner-gate.$$.$RANDOM
      launch_detached candidate_pid "$supervision/owner.log" "$script_path" __owner --repo "$repo" --gate "$gate" --tag "$owner_tag" --watcher-cap "$watcher_cap"
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
  release_cap_authority_lock
  trap - EXIT
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
