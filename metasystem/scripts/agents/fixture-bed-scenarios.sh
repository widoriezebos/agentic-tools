#!/usr/bin/env bash
# The ONE owner of the continue-and-collect scenario harness for fixture
# beds (Ruling P): a bed runs every scenario as its own child process,
# records each red, continues past it, and fails once at the end with
# every failure's tail. Every bed sources this file (the four early
# conversions that carried private serial copies of this block joined it
# on 2026-09-11, so their scenarios run side by side like every other
# bed's). Requires fixture-budget.sh sourced first (child detection and
# capability minting live there).
#
# Usage, at the top of a bed after sourcing fixture-budget.sh:
#   source "$root/scripts/agents/fixture-bed-scenarios.sh"
# then the detection idiom at the bottom of this file, then
#   run_fixture_bed_scenarios <bed> "<success line>" <script> <scenario>...
# in the parent branch; the child's body gates its sections on
# $fixture_scenario.
#
# A bed with a private need names a function in one of these variables
# before calling the runner (each is optional):
#   fixture_bed_prepare_hook        run once with the private log root before
#                                   the first scenario launches (a shared
#                                   engine build, say)
#   fixture_bed_mint_capability     mints a child's capability file; called
#                                   with the log root, the scenario index and
#                                   the scenario name; defaults to
#                                   harness_fixture_bed_mint_capability
#   fixture_bed_scenario_verdict    judges one finished scenario; called with
#                                   the scenario name, its exit status and
#                                   its log path; returns 0 for a pass (the
#                                   default passes exactly exit status 0)
#   fixture_bed_parent_extra_cleanup runs inside the parent's exit cleanup

fixture_bed_harness_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
fixture_bed_parent_log_root=
fixture_bed_parent_child_pid=
fixture_bed_parent_bed=
fixture_bed_parent_scenario=
fixture_bed_prepare_hook=${fixture_bed_prepare_hook:-}
fixture_bed_mint_capability=${fixture_bed_mint_capability:-harness_fixture_bed_mint_capability}
fixture_bed_scenario_verdict=${fixture_bed_scenario_verdict:-}
fixture_bed_parent_extra_cleanup=${fixture_bed_parent_extra_cleanup:-}

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
  local status=$? live
  trap - EXIT HUP INT QUIT TERM
  if [[ -n "$fixture_bed_parent_child_pid" ]]; then
    fixture_bed_reap_group "$fixture_bed_parent_child_pid" || true
  fi
  for live in ${fixture_bed_parent_child_pids:-}; do
    fixture_bed_reap_group "$live" || true
  done
  [[ -z "$fixture_bed_parent_log_root" ]] \
    || rm -rf "$fixture_bed_parent_log_root" 2>/dev/null || true
  if [[ -n "$fixture_bed_parent_extra_cleanup" ]]; then
    "$fixture_bed_parent_extra_cleanup" || true
  fi
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
  if [[ -n "$fixture_bed_prepare_hook" ]]; then
    "$fixture_bed_prepare_hook" "$log_root"
  fi
  # Scenarios are independent children with their own temp roots, so a bed
  # runs up to METASYSTEM_FIXTURE_SCENARIO_CONCURRENCY of them side by side
  # (default: the cores divided by six, at least one, at most three; a bed's
  # scenarios spawn stewards, runners and fake adapters, so the cap is about
  # process trees, not cores). Each child keeps its own ceiling; its log is
  # printed whole when it ends, so the section log stays one scenario at a
  # time even though the work overlapped.
  local scenario_slots cores
  scenario_slots=${METASYSTEM_FIXTURE_SCENARIO_CONCURRENCY:-}
  if [[ ! "$scenario_slots" =~ ^[1-9][0-9]*$ ]]; then
    cores=$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 1)
    [[ "$cores" =~ ^[1-9][0-9]*$ ]] || cores=1
    scenario_slots=$((cores / 6))
    (( scenario_slots >= 1 )) || scenario_slots=1
    (( scenario_slots <= 3 )) || scenario_slots=3
  fi
  local -a queued=("$@")
  local queued_at=0 total=$#
  local -a live_pids=() live_names=() live_logs=() live_started=() live_deadlines=()
  local slot live_count collected
  fixture_bed_parent_child_pids=
  while (( queued_at < total || ${#live_pids[@]} > 0 )); do
    while (( queued_at < total && ${#live_pids[@]} < scenario_slots )); do
      scenario=${queued[$queued_at]}
      log=$log_root/$queued_at.log
      capability=$("$fixture_bed_mint_capability" "$log_root" "$queued_at" "$scenario")
      echo "$bed fixture scenario started: $scenario" >&2
      set -m
      "$script" --fixture-bed-child "$scenario" "$capability" </dev/null >"$log" 2>&1 &
      live_pids+=("$!")
      set +m
      live_names+=("$scenario")
      live_logs+=("$log")
      live_started+=("$SECONDS")
      live_deadlines+=("$((SECONDS + scenario_cap))")
      fixture_bed_parent_child_pids="${live_pids[*]}"
      queued_at=$((queued_at + 1))
    done
    collected=0
    for ((slot = 0; slot < ${#live_pids[@]}; slot++)); do
      if kill -0 "${live_pids[$slot]}" 2>/dev/null && (( SECONDS < live_deadlines[slot] )); then
        continue
      fi
      scenario=${live_names[$slot]}
      log=${live_logs[$slot]}
      if kill -0 "${live_pids[$slot]}" 2>/dev/null; then
        scenario_elapsed=$((SECONDS - live_started[slot]))
        echo "$bed fixture scenario exceeded its ceiling: $scenario (elapsed ${scenario_elapsed}s, scaled cap ${scenario_cap}s)" >&2
        fixture_bed_reap_group "${live_pids[$slot]}" || true
        rc=124
      else
        set +e
        wait "${live_pids[$slot]}"
        rc=$?
        set -e
      fi
      cat "$log"
      scenario_elapsed=$((SECONDS - live_started[slot]))
      if [[ -n "$fixture_bed_scenario_verdict" ]]; then
        if "$fixture_bed_scenario_verdict" "$scenario" "$rc" "$log"; then
          rc=0
        elif [[ $rc -eq 0 ]]; then
          rc=1
        fi
      fi
      if [[ $rc -eq 0 ]]; then
        echo "$bed fixture scenario passed: $scenario (${scenario_elapsed}s)" >&2
      else
        failed_names+=("$scenario")
        failed_rcs+=("$rc")
        failed_logs+=("$log")
        echo "$bed fixture scenario failed: $scenario (rc=$rc, ${scenario_elapsed}s); continuing" >&2
      fi
      unset "live_pids[$slot]" "live_names[$slot]" "live_logs[$slot]" "live_started[$slot]" "live_deadlines[$slot]"
      live_pids=("${live_pids[@]+"${live_pids[@]}"}")
      live_names=("${live_names[@]+"${live_names[@]}"}")
      live_logs=("${live_logs[@]+"${live_logs[@]}"}")
      live_started=("${live_started[@]+"${live_started[@]}"}")
      live_deadlines=("${live_deadlines[@]+"${live_deadlines[@]}"}")
      fixture_bed_parent_child_pids="${live_pids[*]+"${live_pids[*]}"}"
      collected=1
      break
    done
    (( collected )) || sleep 0.25
  done
  fixture_bed_parent_child_pid=
  fixture_bed_parent_scenario=
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
