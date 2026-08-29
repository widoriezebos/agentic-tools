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
  --ceiling-state FILE  supervision state carrying derivedWatcherCapMin
  --expected-cap N  wait for ceiling state to publish this derived value
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
ceiling_state=
expected_cap=
instance_tag=

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
    --ceiling-state) [ $# -ge 2 ] || { usage; exit 2; }; ceiling_state=$2; shift 2 ;;
    --expected-cap) [ $# -ge 2 ] || { usage; exit 2; }; expected_cap=$2; shift 2 ;;
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
ms="${METASYSTEM_BIN:-$harness_root/bin/metasystem}"
config=$script_dir/metasystem-config.sh
stale_args=(get --key watch.stale-min --default 20)
cap_args=(get --key watch.cap-min --default 180)
interval_args=(get --key watch.interval-sec --default 60)
(( stale_min_set )) && stale_args+=(--flag "$stale_min")
(( cap_min_set )) && cap_args+=(--flag "$cap_min")
(( interval_set )) && interval_args+=(--flag "$interval")
stale_min=$("$config" "${stale_args[@]}")
cap_min=$("$config" "${cap_args[@]}")
interval=$("$config" "${interval_args[@]}")

if [[ -n "$expected_cap" && ! "$expected_cap" =~ ^[1-9][0-9]*$ ]]; then
  echo "--expected-cap must be a positive integer" >&2
  exit 2
fi
if [[ -n "$ceiling_state" ]]; then
  ceiling_deadline=$((SECONDS + 10))
  # The ceiling is accepted only as a positive integer: a non-integer or
  # sub-1 value re-polls rather than being trusted.
  read_watcher_cap() {
    local value
    value=$("$ms" json get --file "$ceiling_state" --field derivedWatcherCapMin 2>/dev/null) || return 1
    [[ "$value" =~ ^[1-9][0-9]*$ ]] || return 1
    printf '%s\n' "$value"
  }
  while ! cap_min=$(read_watcher_cap) || [[ -n "$expected_cap" && "$cap_min" != "$expected_cap" ]]; do
    (( SECONDS < ceiling_deadline )) \
      || { echo "watcher startup refused: supervision state did not publish derivedWatcherCapMin" >&2; exit 1; }
    sleep 0.02
  done
fi

[ ${#dirs[@]} -gt 0 ] || { usage; exit 2; }
case "$stale_min$cap_min$interval$start_verify_min" in *[!0-9]*) usage; exit 2 ;; esac
supervision_interval_ms=${METASYSTEM_CENSUS_INTERVAL_MS:-$((interval * 1000))}
[[ "$supervision_interval_ms" =~ ^[1-9][0-9]*$ ]] \
  || { echo "METASYSTEM_CENSUS_INTERVAL_MS must be a positive integer" >&2; exit 2; }
printf -v supervision_interval_sleep '%d.%03d' \
  "$((supervision_interval_ms / 1000))" "$((supervision_interval_ms % 1000))"

if (( census_enabled )); then
  [[ -n "$scope" && -x "$ms" ]] || { echo "--census requires --scope and the metasystem binary" >&2; exit 2; }
  scope=$(git -C "$scope" rev-parse --show-toplevel 2>/dev/null) \
    || { echo "--census scope is not a git repository" >&2; exit 2; }
  scope=$(cd "$scope" && pwd -P)
  supervision_dir=${supervision_dir:-$harness_root/artifacts/agents/supervision}
  watcher_heartbeat=${watcher_heartbeat:-$supervision_dir/watcher.heartbeat.json}
  mkdir -p "$supervision_dir"
fi

if [ -z "$state_file" ]; then
  # cksum prints "sum size"; joined WITHOUT a separator, ("12","345") and
  # ("123","45") collide into one state file (script-misc-10).
  key=$(printf '%s\n' "${dirs[@]}" "scope=$scope" | cksum | awk '{print $1 "-" $2}')
  state_file="${TMPDIR:-/tmp}/watch-background-jobs.${key}.state"
fi
mkdir -p "$(dirname "$state_file")"
auto_baseline=0
[ -f "$state_file" ] || auto_baseline=1
touch "$state_file"

script_fp=$(cksum "$0" 2>/dev/null | tr -d ' \t' | cut -c1-12)
printf 'ARMED watcher fp=%s dirs=%s scope=%s state=%s stale=%sm cap=%sm start-verify=%sm auto-baseline=%s\n' \
  "$script_fp" "${dirs[*]}" "${scope:-none}" "$state_file" "$stale_min" "$cap_min" "$start_verify_min" "$auto_baseline"

# The running set rides in a scratch file so one classification pass can
# hand it to the next; mktemp per watcher keeps the restart-resets-tracking
# behavior the in-process arrays had.
running_file=$(mktemp "${TMPDIR:-/tmp}/watch-running.XXXXXX")
trap 'rm -f "$running_file"' EXIT


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

# One census pass through the engine: it claims the census-writer lock
# (refusing a live writer), heartbeats, scans, publishes the verdict, and
# emits any CENSUS-SLOW warning. Output lands in the census log; a refusal
# surfaces on stderr and ends the run.
run_process_census() {
  local captured
  captured=$(mktemp "${TMPDIR:-/tmp}/metasystem-census-pass.XXXXXX")
  if ! "$ms" supervise watcher-pass --root "$harness_root" --scope "$scope" \
      --supervision-dir "$supervision_dir" --heartbeat "$watcher_heartbeat" \
      --tag "${instance_tag:-watch-background-jobs-$$}" --interval "$interval" \
      >"$captured" 2>&1; then
    cat "$captured" >&2
    append_census_log "$captured"
    rm -f "$captured"
    exit 1
  fi
  grep -E 'WARNING CENSUS-SLOW' "$captured" >&2 || true
  append_census_log "$captured"
  rm -f "$captured"
}



# The classification engine — sidecar selection, sibling-mtime liveness,
# the DONE/CAPPED/NEVER-STARTED/STALE/VANISHED precedence, seen-state and
# baseline policy — lives in `report scan-jobs` (script-orchestration-06 =
# script-misc-3, D23; the REPORT family per r3/KS-R3-009). Report lines and
# the seen-state format are unchanged wire. The engine also refuses empty
# thresholds loudly: the shell's concatenated digit check let an empty
# --stale-min silently disable STALE.
scan_once() {
  (( census_enabled )) && run_process_census
  local scan_args=(--state "$state_file" --running "$running_file" \
    --scope-field "$scope_field" --stale-min "$stale_min" --cap-min "$cap_min" \
    --start-verify-min "$start_verify_min")
  local report heartbeat= line
  [ -z "$scope" ] || scan_args+=(--scope "$scope")
  [ "$baseline" -eq 1 ] && scan_args+=(--baseline)
  local pattern
  for pattern in "${dirs[@]}"; do scan_args+=(--dir "$pattern"); done
  report=$("$ms" report scan-jobs "${scan_args[@]}")
  if [[ -n "$scope" ]]; then
    heartbeat=$("$ms" proof-run heartbeat --root "$scope" 2>/dev/null || true)
  fi
  if [[ -n "$report" ]]; then
    while IFS= read -r line; do
      if [[ -n "$heartbeat" ]]; then
        printf '%s %s\n' "$heartbeat" "$line"
      else
        printf '%s\n' "$line"
      fi
    done <<<"$report"
  elif [[ -n "$heartbeat" ]]; then
    printf '%s\n' "$heartbeat"
  fi
}


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
  sleep "$supervision_interval_sleep"
done
