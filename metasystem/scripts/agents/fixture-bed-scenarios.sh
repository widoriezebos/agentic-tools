#!/usr/bin/env bash
# The ONE owner of the continue-and-collect scenario harness for fixture
# beds (Ruling P): a bed runs every scenario as its own child process,
# records each red, continues past it, and fails once at the end with
# every failure's tail. Four early conversions carry private copies of
# this block (dispatch, health, goal-cli, supervision); new conversions
# source this file instead so the harness has one home. Requires
# fixture-budget.sh sourced first (child detection and capability
# minting live there).
#
# Usage, at the top of a bed after sourcing fixture-budget.sh:
#   source "$root/scripts/agents/fixture-bed-scenarios.sh"
# then the detection idiom at the bottom of this file, then
#   run_fixture_bed_scenarios <bed> "<success line>" <script> <scenario>...
# in the parent branch; the child's body gates its sections on
# $fixture_scenario.

fixture_bed_harness_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
fixture_bed_parent_log_root=
fixture_bed_parent_child_pid=
fixture_bed_parent_bed=
fixture_bed_parent_scenario=

fixture_bed_reap_group() { # process-group leader pid
  local pgid=$1 deadline
  kill -TERM -- "-$pgid" 2>/dev/null || true
  echo "group $pgid: TERM sent" >&2
  deadline=$((SECONDS + 5))
  while kill -0 -- "-$pgid" 2>/dev/null && (( SECONDS < deadline )); do
    sleep 0.25
  done
  if kill -0 -- "-$pgid" 2>/dev/null; then
    kill -KILL -- "-$pgid" 2>/dev/null || true
    echo "group $pgid: alive after 5s grace; KILL sent" >&2
  fi
  wait "$pgid" 2>/dev/null || true
  deadline=$((SECONDS + 5))
  while kill -0 -- "-$pgid" 2>/dev/null && (( SECONDS < deadline )); do
    sleep 0.25
  done
  if kill -0 -- "-$pgid" 2>/dev/null; then
    echo "$fixture_bed_parent_bed fixture scenario left process group $pgid alive after KILL: $fixture_bed_parent_scenario" >&2
    return 1
  fi
  echo "group $pgid: empty" >&2
}

fixture_bed_parent_cleanup() {
  local status=$?
  trap - EXIT HUP INT QUIT TERM
  if [[ -n "$fixture_bed_parent_child_pid" ]]; then
    fixture_bed_reap_group "$fixture_bed_parent_child_pid" || true
  fi
  [[ -z "$fixture_bed_parent_log_root" ]] \
    || rm -rf "$fixture_bed_parent_log_root" 2>/dev/null || true
  return "$status"
}

run_fixture_bed_scenarios() { # bed name, success line, script, scenario names...
  local bed=$1 success_line=$2 script=$3 log_root scenario capability log rc index=0
  local scenario_cap scenario_started scenario_deadline scenario_elapsed
  local failed_names=() failed_rcs=() failed_logs=()
  shift 3
  if [[ ! "${METASYSTEM_FIXTURE_CAP_SCALE_MILLI:-}" =~ ^[1-9][0-9]*$ ]]; then
    harness_fixture_budget_init "$fixture_bed_harness_root"
  fi
  scenario_cap=$(harness_fixture_cap bed-scenario)
  log_root=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-${bed}-scenarios.XXXXXX")
  fixture_bed_parent_log_root=$log_root
  fixture_bed_parent_bed=$bed
  trap fixture_bed_parent_cleanup EXIT
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 131' QUIT
  trap 'exit 143' TERM
  for scenario in "$@"; do
    log=$log_root/$index.log
    capability=$(harness_fixture_bed_mint_capability "$log_root" "$index" "$scenario")
    fixture_bed_parent_scenario=$scenario
    echo "$bed fixture scenario started: $scenario" >&2
    set -m
    "$script" --fixture-bed-child "$scenario" "$capability" </dev/null >"$log" 2>&1 &
    fixture_bed_parent_child_pid=$!
    set +m
    scenario_started=$SECONDS
    scenario_deadline=$((SECONDS + scenario_cap))
    while kill -0 "$fixture_bed_parent_child_pid" 2>/dev/null \
        && (( SECONDS < scenario_deadline )); do
      sleep 0.25
    done
    if kill -0 "$fixture_bed_parent_child_pid" 2>/dev/null; then
      scenario_elapsed=$((SECONDS - scenario_started))
      echo "$bed fixture scenario exceeded its ceiling: $scenario (elapsed ${scenario_elapsed}s, scaled cap ${scenario_cap}s)" >&2
      fixture_bed_reap_group "$fixture_bed_parent_child_pid" || true
      rc=124
    else
      set +e
      wait "$fixture_bed_parent_child_pid"
      rc=$?
      set -e
    fi
    fixture_bed_parent_child_pid=
    fixture_bed_parent_scenario=
    cat "$log"
    if [[ $rc -eq 0 ]]; then
      echo "$bed fixture scenario passed: $scenario" >&2
    else
      failed_names+=("$scenario")
      failed_rcs+=("$rc")
      failed_logs+=("$log")
      echo "$bed fixture scenario failed: $scenario (rc=$rc); continuing" >&2
    fi
    index=$((index + 1))
  done
  if (( ${#failed_names[@]} )); then
    echo "=== $bed failed scenarios ===" >&2
    for ((index = 0; index < ${#failed_names[@]}; index++)); do
      echo "- ${failed_names[$index]} (rc=${failed_rcs[$index]})" >&2
      echo "  output tail:" >&2
      tail -n 40 "${failed_logs[$index]}" | sed 's/^/    /' >&2
    done
    echo "=== end $bed failed scenarios ===" >&2
    rm -rf "$log_root"
    exit 1
  fi
  rm -rf "$log_root"
  echo "$success_line"
  exit 0
}

# Each bed keeps the small detection idiom (it needs the bed's own "$@"):
#   fixture_bed_child=0; fixture_scenario=
#   if fixture_scenario=$(harness_fixture_bed_child_scenario <bed> "$@"); then
#     fixture_bed_child=1
#   else rc=$?; [[ $rc -eq 1 ]] || exit "$rc"; fi
#   unset METASYSTEM_FIXTURE_SCENARIO
