#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/watch-background-jobs.sh --dir <job-state-dir> [--dir <more>]...
                                   [--scope <path>] [--scope-field <name>]
                                   [--state <file>] [--stale-min <n>]
                                   [--cap-min <n>] [--interval <sec>]
                                   [--baseline] [--once]

Emits one line per background job that reaches a REPORTABLE state, then keeps
watching. Written for the arm-once supervision contract in
docs/orchestration.md: arm this ONCE per session, before the first dispatch,
so no per-dispatch watcher can be forgotten.

It trips on all four conditions the contract requires, because a status-only
watcher loops forever on a job that hangs or whose record disappears:

  DONE      terminal status recorded by the runner
  STALE     no state change for --stale-min minutes while still running
  CAPPED    running longer than --cap-min minutes (absolute ceiling)
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
  --cap-min N      minutes of total runtime before CAPPED (default 180)
  --interval SEC   poll interval (default 60)
  --baseline       record every currently-terminal job as already reported and
                   exit; use once when adopting the watcher on a repository
                   with history, so the first arming does not flood
  --once           make a single pass and exit (for tests and cron)

Reported lines are stable and greppable:
  <STATE> <job-id> status=<status> age=<minutes>m record=<path>

Exit codes: 0 normal exit (--once/--baseline); 2 usage error.
USAGE
}

dirs=()
scope=""
scope_field="workspaceRoot"
state_file=""
stale_min=20
cap_min=180
interval=60
baseline=0
once=0

while [ $# -gt 0 ]; do
  case "$1" in
    --dir) [ $# -ge 2 ] || { usage; exit 2; }; dirs+=("$2"); shift 2 ;;
    --scope) [ $# -ge 2 ] || { usage; exit 2; }; scope="${2%/}"; shift 2 ;;
    --scope-field) [ $# -ge 2 ] || { usage; exit 2; }; scope_field="$2"; shift 2 ;;
    --state) [ $# -ge 2 ] || { usage; exit 2; }; state_file="$2"; shift 2 ;;
    --stale-min) [ $# -ge 2 ] || { usage; exit 2; }; stale_min="$2"; shift 2 ;;
    --cap-min) [ $# -ge 2 ] || { usage; exit 2; }; cap_min="$2"; shift 2 ;;
    --interval) [ $# -ge 2 ] || { usage; exit 2; }; interval="$2"; shift 2 ;;
    --baseline) baseline=1; shift ;;
    --once) once=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

[ ${#dirs[@]} -gt 0 ] || { usage; exit 2; }
case "$stale_min$cap_min$interval" in *[!0-9]*) usage; exit 2 ;; esac

if [ -z "$state_file" ]; then
  key=$(printf '%s\n' "${dirs[@]}" "scope=$scope" | cksum | tr -d ' \t')
  state_file="${TMPDIR:-/tmp}/watch-background-jobs.${key}.state"
fi
mkdir -p "$(dirname "$state_file")"
touch "$state_file"

now_epoch() { date +%s; }

file_mtime() {
  # portable: BSD stat then GNU stat
  stat -f %m "$1" 2>/dev/null || stat -c %Y "$1" 2>/dev/null || echo 0
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
  while [ $i -lt ${#running_ids[@]:-0} ]; do
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
  local pattern path id status mtime age_min

  for pattern in "${dirs[@]}"; do
    # word-splitting is intended: patterns may be globs
    for path in $pattern/*; do
      [ -f "$path" ] || continue
      id=$(basename "$path"); id="${id%.*}"
      seen "$id" && continue
      in_scope "$path" || continue

      status=$(job_status "$path")
      mtime=$(file_mtime "$path")
      age_min=$(( (now - mtime) / 60 ))

      if [ -n "$status" ] && is_terminal "$status"; then
        if [ "$baseline" -eq 1 ]; then mark "$id"; continue; fi
        report DONE "$id" "$status" "$age_min" "$path"; mark "$id"; forget_running "$id"
      elif [ "$age_min" -ge "$cap_min" ]; then
        [ "$baseline" -eq 1 ] && { mark "$id"; continue; }
        report CAPPED "$id" "${status:-running}" "$age_min" "$path"; mark "$id"; forget_running "$id"
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
  while [ $i -lt ${#running_ids[@]:-0} ]; do
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

if [ "$baseline" -eq 1 ]; then
  scan_once
  echo "baseline recorded in $state_file" >&2
  exit 0
fi

if [ "$once" -eq 1 ]; then
  scan_once
  exit 0
fi

while true; do
  scan_once
  sleep "$interval"
done
