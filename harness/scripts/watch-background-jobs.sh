#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/watch-background-jobs.sh --dir <job-state-dir> [--dir <more>]...
                                   [--scope <path>] [--scope-field <name>]
                                   [--state <file>] [--stale-min <n>]
                                   [--cap-min <n>] [--interval <sec>]
                                   [--baseline] [--once] [--census]

Emits one line per background job that reaches a REPORTABLE state, then keeps
watching. Written for the arm-once supervision contract in
docs/orchestration.md: arm this ONCE per session, before the first dispatch,
so no per-dispatch watcher can be forgotten.

It trips on all four conditions the contract requires, because a status-only
watcher loops forever on a job that hangs or whose record disappears:

  DONE      terminal status recorded by the runner
  STALE     no state change for --stale-min minutes while still running
  CAPPED    no record or sidecar change for --cap-min minutes (inactivity ceiling)
  VANISHED  a job seen running whose record disappeared (runner lost it)

Job records are files under the given directories. A record's status is read
from a top-level JSON "status" field when the file parses as JSON; otherwise
the file's mtime alone drives STALE/CAPPED and terminal status is unknown.
Directories may be given as globs so a whole tool's per-project layout is
covered by one argument, and they need not exist yet when the watcher starts.

SCOPE. A shared runner records every project's jobs side by side, so watch only
the jobs belonging to THIS repository: reporting a peer session's job is noise
at best and a false claim of ownership at worst. Pass --scope with the
repository root; records whose workspace field falls outside it are ignored.
Scope by recorded workspace, not by directory name — worktrees of one
repository get their own job directories, and a name filter misses them.

STATE ISOLATION. The state file records which jobs were reported. Two projects
sharing one state file suppress each other's reports, which is the exact silent
miss this watcher exists to prevent. The default path is derived from the
--dir/--scope arguments so distinct scopes get distinct state automatically;
only override --state with a path unique to the scope.

Options:
  --dir DIR        directory (or glob) holding job records; repeatable, required
  --scope PATH     only report jobs whose workspace field is PATH or below
                   (default: no filter)
  --scope-field F  JSON field holding the workspace (default workspaceRoot)
  --state FILE     where already-reported jobs are remembered
                   (default: ${TMPDIR:-/tmp}/watch-background-jobs.<hash>.state)
  --stale-min N    minutes without a state change before STALE (default 20)
  --cap-min N      minutes without a state change before CAPPED (default 180)
  --interval SEC   poll interval (default 60)
  --census         inventory agent-signature processes in --scope on every pass
  --supervision-dir DIR  census/heartbeat directory (arming-only option)
  --heartbeat FILE watcher function heartbeat (arming-only option)
  --instance-tag TAG  watcher process identity tag (arming-only option)
  --baseline       record every currently-terminal job as already reported and
                   exit; use once when adopting the watcher on a repository
                   with history, so the first arming does not flood
                   (a MISSING state file now auto-baselines on first run, so
                   this flag is only needed to re-baseline an existing file)
  --start-verify-min N  minutes a job may sit queued/pending before
                   NEVER-STARTED fires (default 5; 0 disables)
  --once           make a single pass and exit (for tests and cron)

Reported lines are stable and greppable:
  <STATE> <job-id> status=<status> age=<minutes>m record=<path>
plus NEVER-STARTED for a dispatch that stays queued/pending past
--start-verify-min, and one ARMED banner on startup carrying the effective
config and the script's own fingerprint — so a transcript always shows WHICH
code and config a long-lived watcher is actually running (armed watchers keep
executing the code they started with; re-arm after editing this script).

Exit codes: 0 normal exit (--once/--baseline); 2 usage error.
USAGE
}

dirs=()
scope=""
scope_field="workspaceRoot"
state_file=""
stale_min=
cap_min=
stale_min_set=0
cap_min_set=0
interval=60
interval_set=0
baseline=0
once=0
start_verify_min=5
census_enabled=0
supervision_dir=
watcher_heartbeat=
instance_tag=
census_writer_owned=0
census_budget_percent=

while [ $# -gt 0 ]; do
  case "$1" in
    --dir) [ $# -ge 2 ] || { usage; exit 2; }; dirs+=("$2"); shift 2 ;;
    --scope) [ $# -ge 2 ] || { usage; exit 2; }; scope="${2%/}"; shift 2 ;;
    --scope-field) [ $# -ge 2 ] || { usage; exit 2; }; scope_field="$2"; shift 2 ;;
    --state) [ $# -ge 2 ] || { usage; exit 2; }; state_file="$2"; shift 2 ;;
    --stale-min) [ $# -ge 2 ] || { usage; exit 2; }; stale_min="$2"; stale_min_set=1; shift 2 ;;
    --cap-min) [ $# -ge 2 ] || { usage; exit 2; }; cap_min="$2"; cap_min_set=1; shift 2 ;;
    --interval) [ $# -ge 2 ] || { usage; exit 2; }; interval="$2"; interval_set=1; shift 2 ;;
    --census) census_enabled=1; shift ;;
    --supervision-dir) [ $# -ge 2 ] || { usage; exit 2; }; supervision_dir=$2; shift 2 ;;
    --heartbeat) [ $# -ge 2 ] || { usage; exit 2; }; watcher_heartbeat=$2; shift 2 ;;
    --instance-tag) [ $# -ge 2 ] || { usage; exit 2; }; instance_tag=$2; shift 2 ;;
    --baseline) baseline=1; shift ;;
    --start-verify-min) [ $# -ge 2 ] || { usage; exit 2; }; start_verify_min="$2"; shift 2 ;;
    --once) once=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/.." && pwd -P)
config=$script_dir/harness-config.sh
stale_args=(get --key watch.stale-min --default 20)
cap_args=(get --key watch.cap-min --default 180)
interval_args=(get --key watch.interval-sec --default 60)
(( stale_min_set )) && stale_args+=(--flag "$stale_min")
(( cap_min_set )) && cap_args+=(--flag "$cap_min")
(( interval_set )) && interval_args+=(--flag "$interval")
stale_min=$("$config" "${stale_args[@]}")
cap_min=$("$config" "${cap_args[@]}")
interval=$("$config" "${interval_args[@]}")
census_budget_percent=$("$config" get --key census.max-interval-share-percent --default 50)

[ ${#dirs[@]} -gt 0 ] || { usage; exit 2; }
case "$stale_min$cap_min$interval$start_verify_min" in *[!0-9]*) usage; exit 2 ;; esac
[[ "$census_budget_percent" =~ ^[1-9][0-9]*$ ]] \
  && (( census_budget_percent <= 100 )) \
  || { echo "census.max-interval-share-percent must be 1..100" >&2; exit 2; }

process_census=$script_dir/agents/process-census.py
if (( census_enabled )); then
  [[ -n "$scope" && -x "$process_census" ]] || { echo "--census requires --scope and process-census.py" >&2; exit 2; }
  scope=$(git -C "$scope" rev-parse --show-toplevel 2>/dev/null) \
    || { echo "--census scope is not a git repository" >&2; exit 2; }
  scope=$(cd "$scope" && pwd -P)
  supervision_dir=${supervision_dir:-$harness_root/artifacts/agents/supervision}
  watcher_heartbeat=${watcher_heartbeat:-$supervision_dir/watcher.heartbeat.json}
  mkdir -p "$supervision_dir"
fi

if [ -z "$state_file" ]; then
  key=$(printf '%s\n' "${dirs[@]}" "scope=$scope" | cksum | tr -d ' \t')
  state_file="${TMPDIR:-/tmp}/watch-background-jobs.${key}.state"
fi
mkdir -p "$(dirname "$state_file")"
auto_baseline=0
[ -f "$state_file" ] || auto_baseline=1
touch "$state_file"

script_fp=$(cksum "$0" 2>/dev/null | tr -d ' \t' | cut -c1-12)
printf 'ARMED watcher fp=%s dirs=%s scope=%s state=%s stale=%sm cap=%sm start-verify=%sm auto-baseline=%s\n' \
  "$script_fp" "${dirs[*]}" "${scope:-none}" "$state_file" "$stale_min" "$cap_min" "$start_verify_min" "$auto_baseline"

now_epoch() { date +%s; }

atomic_identity_json() { # output, function
  python3 - "$1" "$2" "$$" "$instance_tag" "$process_census" <<'PY'
import json, os, subprocess, sys, tempfile, time
from pathlib import Path
output, function, pid, tag, helper = Path(sys.argv[1]), sys.argv[2], int(sys.argv[3]), sys.argv[4], sys.argv[5]
started = int(subprocess.check_output([helper, "started-at", "--pid", str(pid)], text=True).strip())
value = {"function": function, "pid": pid, "pidStartedAt": started, "instanceTag": tag, "observedAtEpoch": int(time.time())}
output.parent.mkdir(parents=True, exist_ok=True)
fd, temporary = tempfile.mkstemp(prefix=output.name + ".", suffix=".tmp", dir=output.parent)
with os.fdopen(fd, "w") as handle:
    json.dump(value, handle, sort_keys=True); handle.write("\n"); handle.flush(); os.fsync(handle.fileno())
os.replace(temporary, output)
PY
}

acquire_census_writer() {
  local lock="$supervision_dir/census-writer.d" owner="$supervision_dir/census-writer.d/owner.json" pid start
  if mkdir "$lock" 2>/dev/null; then
    atomic_identity_json "$owner" census-writer
    census_writer_owned=1
    return 0
  fi
  [[ -f "$owner" ]] || { echo "census writer lock has no provable owner" >&2; return 1; }
  read -r pid start < <(python3 - "$owner" <<'PY'
import json, sys
try: v=json.load(open(sys.argv[1])); print(v["pid"], v["pidStartedAt"])
except (OSError, ValueError, KeyError, TypeError): raise SystemExit(1)
PY
  ) || { echo "census writer lock has malformed owner identity" >&2; return 1; }
  if "$process_census" alive --pid "$pid" --start-time "$start" >/dev/null 2>&1; then
    echo "live census writer already owns $supervision_dir" >&2
    return 1
  fi
  rm "$owner"
  rmdir "$lock"
  mkdir "$lock" || { echo "census writer takeover lost a race" >&2; return 1; }
  atomic_identity_json "$owner" census-writer
  census_writer_owned=1
}

release_census_writer() {
  (( census_writer_owned )) || return 0
  local lock="$supervision_dir/census-writer.d" owner="$supervision_dir/census-writer.d/owner.json"
  rm -f "$owner"
  rmdir "$lock" 2>/dev/null || true
}

append_census_log() { # captured scan output
  local captured=$1 log="$supervision_dir/census.log" max_bytes current=0 incoming
  max_bytes=$("$config" get --key census.log-max-bytes --default 1048576)
  [[ "$max_bytes" =~ ^[1-9][0-9]*$ ]] || max_bytes=1048576
  [[ -f "$log" ]] && current=$(wc -c <"$log" | tr -d ' ')
  incoming=$(wc -c <"$captured" | tr -d ' ')
  if (( current + incoming > max_bytes )) && [[ -s "$log" ]]; then
    mv "$log" "$log.1"
  fi
  cat "$captured" >>"$log"
}

monitor_census_duration() { # start-ns, share-marker, interval-marker
  local started_ns=$1 share_marker=$2 interval_marker=$3 period=5 elapsed_ms sleeper=
  local interval_ms=$((interval * 1000)) budget_ms=$((interval * 1000 * census_budget_percent / 100))
  (( interval < period )) && period=$interval
  trap 'kill "${sleeper:-}" 2>/dev/null || true; exit 0' TERM INT
  while true; do
    sleep "$period" &
    sleeper=$!
    wait "$sleeper"
    sleeper=
    atomic_identity_json "$watcher_heartbeat" watcher
    elapsed_ms=$(python3 - "$started_ns" <<'PY'
import sys, time
print(round((time.time_ns() - int(sys.argv[1])) / 1_000_000))
PY
    )
    if (( elapsed_ms > interval_ms )) && [[ ! -e "$interval_marker" ]]; then
      touch "$share_marker" "$interval_marker"
      printf 'WARNING CENSUS-SLOW durationMs=%s intervalMs=%s budgetPercent=%s budgetMs=%s defect=scan-exceeds-interval\n' \
        "$elapsed_ms" "$interval_ms" "$census_budget_percent" "$budget_ms" >&2
    elif (( elapsed_ms > budget_ms )) && [[ ! -e "$share_marker" ]]; then
      touch "$share_marker"
      printf 'WARNING CENSUS-SLOW durationMs=%s intervalMs=%s budgetPercent=%s budgetMs=%s defect=none\n' \
        "$elapsed_ms" "$interval_ms" "$census_budget_percent" "$budget_ms" >&2
    fi
  done
}

run_process_census() {
  local fingerprint captured started_ns census_pid monitor_pid scan_status=0
  local share_marker interval_marker last="$supervision_dir/last-census.json"
  captured=$(mktemp "${TMPDIR:-/tmp}/harness-census-pass.XXXXXX")
  share_marker=$captured.warned-share
  interval_marker=$captured.warned-interval
  started_ns=$(python3 -c 'import time; print(time.time_ns())')
  # A scan is active work, not a stale watcher. Publish liveness before the
  # scan as well as after it so the owner does not mistake startup for death.
  atomic_identity_json "$watcher_heartbeat" watcher
  if fingerprint=$("$process_census" fingerprint --repo "$scope" 2>"$captured.error"); then
    started_ns=$(python3 -c 'import time; print(time.time_ns())')
    "$process_census" census --repo "$scope" --fingerprint "$fingerprint" --interval "$interval" --output "$last" >"$captured" &
    census_pid=$!
    monitor_census_duration "$started_ns" "$share_marker" "$interval_marker" &
    monitor_pid=$!
    wait "$census_pid" || scan_status=$?
    kill "$monitor_pid" 2>/dev/null || true
    wait "$monitor_pid" 2>/dev/null || true
    (( scan_status == 0 )) || return "$scan_status"
  else
    python3 - "$last" "$interval" "$captured.error" "$started_ns" <<'PY'
import json, os, sys, tempfile, time
from datetime import datetime, timezone
from pathlib import Path
output, interval, error_path, started_ns = Path(sys.argv[1]), int(sys.argv[2]), Path(sys.argv[3]), int(sys.argv[4])
error = error_path.read_text(errors="replace").strip()
value={"schemaVersion":1,"writer":"watch-background-jobs.sh","verdict":"CENSUS-FAILED","completedAt":datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),"completedAtEpoch":int(time.time()),"durationMs":round((time.time_ns()-started_ns)/1_000_000),"intervalSec":interval,"fingerprint":"FINGERPRINT-FAILED","counts":{"CUSTODY":0,"ANNOUNCED":0,"UNTRACKED":0},"inventory":[],"diagnostics":[],"errors":["fingerprint:"+error]}
fd,tmp=tempfile.mkstemp(prefix=output.name+".",suffix=".tmp",dir=output.parent)
with os.fdopen(fd,"w") as h: json.dump(value,h,indent=2,sort_keys=True); h.write("\n"); h.flush(); os.fsync(h.fileno())
os.replace(tmp,output)
PY
    printf 'CENSUS-FAILED fingerprint=%s\n' "$(tr '\n' ' ' <"$captured.error")" >"$captured"
  fi
  append_census_log "$captured"
  python3 - "$last" "$interval" "$census_budget_percent" "$share_marker" "$interval_marker" <<'PY'
import json, sys
from pathlib import Path

last, interval_ms, budget_percent = Path(sys.argv[1]), int(sys.argv[2]) * 1000, int(sys.argv[3])
share_warned, interval_warned = Path(sys.argv[4]).exists(), Path(sys.argv[5]).exists()
try:
    duration_ms = int(json.loads(last.read_text(encoding="utf-8"))["durationMs"])
except (OSError, ValueError, KeyError, TypeError):
    print("WARNING CENSUS-DURATION-UNAVAILABLE last-census has no valid durationMs", file=sys.stderr)
    raise SystemExit(0)
budget_ms = interval_ms * budget_percent // 100
if duration_ms > interval_ms and not interval_warned:
    print(
        f"WARNING CENSUS-SLOW durationMs={duration_ms} intervalMs={interval_ms} "
        f"budgetPercent={budget_percent} budgetMs={budget_ms} defect=scan-exceeds-interval",
        file=sys.stderr,
    )
elif duration_ms > budget_ms and not share_warned:
    print(
        f"WARNING CENSUS-SLOW durationMs={duration_ms} intervalMs={interval_ms} "
        f"budgetPercent={budget_percent} budgetMs={budget_ms} defect=none",
        file=sys.stderr,
    )
PY
  atomic_identity_json "$watcher_heartbeat" watcher
  if [[ -f "$supervision_dir/arming.log" ]]; then
    printf '%s first-census-complete verdict=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(job_field "$last" verdict)" >>"$supervision_dir/arming.log"
  fi
  rm -f "$captured" "$captured.error" "$share_marker" "$interval_marker"
}

file_mtime() {
  # A failed GNU `stat -f` still prints filesystem details for valid operands,
  # so a plain command chain leaks that output into the fallback result.
  local mtime
  if mtime=$(stat -c %Y "$1" 2>/dev/null); then
    printf '%s\n' "$mtime"
  elif mtime=$(stat -f %m "$1" 2>/dev/null); then
    printf '%s\n' "$mtime"
  else
    echo 0
  fi
}

job_field() { # record field -> value ("" when absent/unparseable)
  python3 - "$1" "$2" 2>/dev/null <<'JSONQ' || true
import json, sys
try:
    with open(sys.argv[1]) as fh:
        d = json.load(fh)
    v = d.get(sys.argv[2]) if isinstance(d, dict) else None
    if isinstance(v, str):
        print(v)
except Exception:
    pass
JSONQ
}

in_scope() { # record -> 0 when the job belongs to this scope
  [ -z "$scope" ] && return 0
  local ws; ws=$(job_field "$1" "$scope_field")
  # No workspace field: report it. Unknown ownership beats an unobserved job.
  [ -z "$ws" ] && return 0
  case "${ws%/}/" in "$scope"/*) return 0 ;; *) return 1 ;; esac
}

job_status() {
  # Top-level JSON "status" if the record parses as JSON, else empty.
  python3 - "$1" <<'PY' 2>/dev/null || true
import json, sys
try:
    with open(sys.argv[1]) as fh:
        d = json.load(fh)
    if isinstance(d, dict) and isinstance(d.get("status"), str):
        print(d["status"])
except Exception:
    pass
PY
}

is_terminal() {
  case "$1" in
    completed|complete|success|succeeded|failed|failure|error|errored|cancelled|canceled|killed|timeout|timed_out) return 0 ;;
    *) return 1 ;;
  esac
}

seen() { grep -qxF "$1" "$state_file" 2>/dev/null; }
mark() { printf '%s\n' "$1" >> "$state_file"; }

# Records observed running, so a disappearance is distinguishable from a job
# that was never seen at all.
declare -a running_ids=()
declare -a running_paths=()
running_index() {
  local i=0
  while [ $i -lt ${#running_ids[@]} ]; do
    [ "${running_ids[$i]}" = "$1" ] && { printf '%s' "$i"; return 0; }
    i=$((i + 1))
  done
  return 1
}
remember_running() {
  running_index "$1" >/dev/null && return 0
  running_ids+=("$1"); running_paths+=("$2")
}
forget_running() {
  local idx; idx=$(running_index "$1") || return 0
  unset 'running_ids[idx]' 'running_paths[idx]'
  running_ids=(${running_ids[@]+"${running_ids[@]}"})
  running_paths=(${running_paths[@]+"${running_paths[@]}"})
}

report() { # state id status age record
  printf '%s %s status=%s age=%sm record=%s\n' "$1" "$2" "${3:-unknown}" "$4" "$5"
}

scan_once() {
  local now; now=$(now_epoch)
  local pattern path id status mtime age_min primary sib

  (( census_enabled )) && run_process_census

  for pattern in "${dirs[@]}"; do
    # word-splitting is intended: patterns may be globs
    for path in $pattern/*; do
      [ -f "$path" ] || continue
      id=$(basename "$path"); id="${id%.*}"
      seen "$id" && continue
      # Prefer the record that actually carries fields; skip sidecars of an id
      # whose primary record exists, so one job reports once and scope holds.
      primary="$path"
      for sib in "$(dirname "$path")/$id".*; do
        [ -f "$sib" ] || continue
        if [ -n "$(job_status "$sib")" ] || [ -n "$(job_field "$sib" "$scope_field")" ]; then
          primary="$sib"; break
        fi
      done
      [ "$primary" = "$path" ] || continue
      in_scope "$path" || continue

      status=$(job_status "$path")
      # Liveness is the NEWEST mtime across every file the runner keeps for this
      # job, not the record alone. Runners commonly write the status record once
      # at dispatch and then stream progress to a sibling log, so a record-only
      # check reports STALE for a job that is demonstrably working — observed on
      # a healthy 25-minute build whose .log was updating continuously.
      # Scope and status still come from the primary record; only age widens.
      mtime=$(file_mtime "$path")
      for sib in "$(dirname "$path")/$id".*; do
        [ -f "$sib" ] || continue
        sib_mtime=$(file_mtime "$sib")
        if [ "${sib_mtime:-0}" -gt "${mtime:-0}" ] 2>/dev/null; then mtime=$sib_mtime; fi
      done
      age_min=$(( (now - mtime) / 60 ))

      if [ -n "$status" ] && is_terminal "$status"; then
        if [ "$baseline" -eq 1 ]; then mark "$id"; continue; fi
        report DONE "$id" "$status" "$age_min" "$path"; mark "$id"; forget_running "$id"
      elif [ "$age_min" -ge "$cap_min" ]; then
        [ "$baseline" -eq 1 ] && { mark "$id"; continue; }
        report CAPPED "$id" "${status:-running}" "$age_min" "$path"; mark "$id"; forget_running "$id"
      elif [ "$start_verify_min" -gt 0 ] && [ "$age_min" -ge "$start_verify_min" ] && \
           { [ -z "$status" ] || case "$status" in queued|pending|starting|created) true ;; *) false ;; esac; }; then
        # A dispatch that never left the queue is a silent failure long before
        # STALE would fire — observed 2026-08-03: a resume queued, died, and
        # cost 2.3 idle hours. Report it early and by its real name.
        [ "$baseline" -eq 1 ] && { mark "$id"; continue; }
        report NEVER-STARTED "$id" "${status:-absent}" "$age_min" "$path"; mark "$id"; forget_running "$id"
      elif [ "$age_min" -ge "$stale_min" ]; then
        [ "$baseline" -eq 1 ] && continue
        report STALE "$id" "${status:-running}" "$age_min" "$path"; mark "$id"; forget_running "$id"
      else
        remember_running "$id" "$path"
      fi
    done
  done

  # Records that were running and are now gone: the runner lost the job.
  local i=0
  while [ $i -lt ${#running_ids[@]} ]; do
    if [ ! -f "${running_paths[$i]}" ]; then
      [ "$baseline" -eq 1 ] || { report VANISHED "${running_ids[$i]}" running 0 "${running_paths[$i]}"; mark "${running_ids[$i]}"; }
      unset 'running_ids[i]' 'running_paths[i]'
      running_ids=(${running_ids[@]+"${running_ids[@]}"})
      running_paths=(${running_paths[@]+"${running_paths[@]}"})
      continue
    fi
    i=$((i + 1))
  done
}

if (( census_enabled )); then
  acquire_census_writer
  trap release_census_writer EXIT
fi

if [ "$baseline" -eq 1 ]; then
  scan_once
  echo "baseline recorded in $state_file" >&2
  exit 0
fi

if [ "$auto_baseline" -eq 1 ]; then
  # First run against a fresh state file: adopt history instead of flooding.
  # The arm-once contract arms BEFORE the first dispatch, so nothing current
  # is suppressed; anything already terminal predates this session's work.
  baseline=1
  scan_once
  baseline=0
  echo "auto-baselined historical jobs into $state_file" >&2
fi

if [ "$once" -eq 1 ]; then
  scan_once
  exit 0
fi

while true; do
  scan_once
  sleep "$interval"
done
