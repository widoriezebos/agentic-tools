#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
bin=${METASYSTEM_BIN:-$root/bin/metasystem}

case ${1:-} in
  __printing)
    progress=$2 suite=$3 section=$4
    printf '{"suite":"%s","section":"%s","event":"start","at":"%s","depth":0}\n' \
      "$suite" "$section" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
    for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
      echo "fixture output is still growing"
      sleep 0.1
    done
    printf '{"suite":"%s","section":"%s","event":"end","at":"%s","depth":0}\n' \
      "$suite" "$section" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
    exit 0
    ;;
  __printing_forever)
    progress=$2 suite=$3 section=$4
    printf '{"suite":"%s","section":"%s","event":"start","at":"%s","depth":0}\n' \
      "$suite" "$section" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
    while :; do echo "fixture remains chatty"; sleep 0.05; done
    ;;
  __detached)
    trap '' TERM
    while :; do sleep 1; done
    ;;
  __stopped)
    bed=$2 progress=$3 suite=$4 section=$5 fixture_bin=$6
    tmp=$bed/tmp
    mkdir -p "$tmp"
    echo $$ >"$tmp/suite.pid"
    sleep 300 &
    echo $! >"$tmp/group-child.pid"
    "$fixture_bin" gate guard-acquire --root "$bed" --owner "stopped suite fixture" \
      --wait-sec 2 --progress-sec 1 >/dev/null
    detached_pid=$("$fixture_bin" supervise launch-detached --cwd "$bed" \
      --execution-guard-root "$bed" --execution-guard-owner "stopped suite detached member" \
      -- bash "$root/scripts/agents/suite-progress-fixtures.sh" __detached)
    echo "$detached_pid" >"$tmp/detached.pid"
    printf 'evidence written before stop\n' >"$tmp/evidence.txt"
    printf '{"suite":"%s","section":"%s","event":"start","at":"%s","depth":0}\n' \
      "$suite" "$section" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
    kill -STOP $$
    exit 99
    ;;
esac

[[ -x "$bin" ]] || { echo "suite-progress fixture: build bin/metasystem first" >&2; exit 1; }
source "$root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$root"
wait_cap=$(harness_fixture_cap suite-watchdog-wait)
reap_cap=$(harness_fixture_cap suite-watchdog-reap)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/suite-progress-fixtures.XXXXXX")
tmp=$(cd "$tmp" && pwd -P)
owned_pids=()
watch_dispatch_record=
cleanup() {
  local pid
  for pid in ${owned_pids[@]+"${owned_pids[@]}"}; do
    [[ "$pid" =~ ^[1-9][0-9]*$ ]] || continue
    kill "$pid" 2>/dev/null || true
  done
  [[ -z "$watch_dispatch_record" ]] || rm -f "$watch_dispatch_record"
  rm -rf "$tmp"
}
trap cleanup EXIT

launch_fixture() { # bed, suite, section, banner, extra launcher flags -- command
  local bed=$1 suite=$2 section=$3 banner=$4
  shift 4
  mkdir -p "$bed/tmp" "$bed/logs"
  "$bin" proof-run launch --suite "$suite" --root "$bed" --conf "$root/metasystem.conf" \
    --progress "$bed/progress.jsonl" --log "$bed/logs/suite.log" --tmp "$bed/tmp" \
    --banner "$banner" --selected "$section" "$@"
}

# Output growth keeps a section alive beyond the shortened silence window.
printing="$tmp/printing"
printing_banner='suite-cost suite=printing witness=armed duration=minutes heartbeat=progress.jsonl logs=logs/suite.log'
launch_fixture "$printing" printing long-printing "$printing_banner" \
  --silence-ms 300 --section-cap-ms 10000 --evidence-timeout-ms 1000 \
  --evidence-max-bytes 1048576 --poll-ms 50 --term-grace-ms 100 --kill-grace-ms 100 -- \
  bash "$root/scripts/agents/suite-progress-fixtures.sh" __printing \
    "$printing/progress.jsonl" printing long-printing
[[ $(grep -c -xF "$printing_banner" "$printing/logs/suite.log") -eq 1 ]] \
  || { echo "suite-progress fixture: cost banner was not logged exactly once" >&2; exit 1; }

# A chatty section still dies on the independent absolute section ceiling.
chatty="$tmp/chatty"
chatty_out="$tmp/chatty.out"
if launch_fixture "$chatty" chatty over-cap \
    'suite-cost suite=chatty witness=armed duration=minutes heartbeat=progress.jsonl logs=logs/suite.log' \
    --silence-ms 2000 --section-cap-ms 400 --evidence-timeout-ms 1000 \
    --evidence-max-bytes 1048576 --poll-ms 50 --term-grace-ms 100 --kill-grace-ms 100 -- \
    bash "$root/scripts/agents/suite-progress-fixtures.sh" __printing_forever \
      "$chatty/progress.jsonl" chatty over-cap >"$chatty_out" 2>&1; then
  echo "suite-progress fixture: a printing section exceeded its cap without being killed" >&2
  exit 1
fi
grep -Fq 'section exceeded its 400ms cap' "$chatty_out" \
  || { echo "suite-progress fixture: absolute-cap failure did not name the stalled section and cap" >&2; exit 1; }

# A missing selector section is structural red even when the command succeeds.
silent="$tmp/silent"
silent_out="$tmp/silent.out"
if launch_fixture "$silent" silent missing-section \
    'suite-cost suite=silent witness=unarmed duration=full-gate heartbeat=progress.jsonl logs=logs/suite.log' \
    --silence-ms 2000 --section-cap-ms 2000 --evidence-timeout-ms 1000 \
    --evidence-max-bytes 1048576 --poll-ms 50 --term-grace-ms 100 --kill-grace-ms 100 -- \
    bash -c ':' >"$silent_out" 2>&1; then
  echo "suite-progress fixture: a silent selector section passed" >&2
  exit 1
fi
grep -Fq 'missing-section has 0 starts and 0 ends' "$silent_out" \
  || { echo "suite-progress fixture: structural red did not name the silent section" >&2; exit 1; }

# Both runner-facing watchers relay the deepest open section from the same
# read-only journal view. The background watcher prefixes a reportable job;
# the command-level printer is covered at its Go-owned boundary.
watch_root="$tmp/watch-root"
watch_jobs="$tmp/watch-jobs"
watch_state="$tmp/watch.state"
watch_out="$tmp/watch.out"
dispatch_watch_out="$tmp/dispatch-watch.out"
mkdir -p "$watch_root/artifacts/agents/supervision" "$watch_jobs"
watch_now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '{"tmpPaths":[],"logPaths":["suite.log"]}\n' >"$watch_root/artifacts/agents/supervision/suite-progress.jsonl"
printf '{"suite":"outer","section":"parent","event":"start","at":"%s","depth":0}\n' "$watch_now" \
  >>"$watch_root/artifacts/agents/supervision/suite-progress.jsonl"
printf '{"suite":"inner","section":"child","event":"start","at":"%s","depth":1}\n' "$watch_now" \
  >>"$watch_root/artifacts/agents/supervision/suite-progress.jsonl"
[[ "$("$bin" proof-run heartbeat --root "$watch_root")" == 'inner:child since 0min' ]] \
  || { echo "suite-progress fixture: deepest live heartbeat was not selected" >&2; exit 1; }
printf '{"status":"completed","workspaceRoot":"%s"}\n' "$watch_root" >"$watch_jobs/prefix-job.json"
: >"$watch_state"
METASYSTEM_BIN="$bin" "$root/scripts/watch-background-jobs.sh" \
  --dir "$watch_jobs" --scope "$watch_root" --state "$watch_state" --once >"$watch_out" 2>&1
grep -Fq 'inner:child since 0min DONE prefix-job status=completed' "$watch_out" \
  || { echo "suite-progress fixture: background watcher did not prefix its job note with the deepest heartbeat" >&2; cat "$watch_out" >&2; exit 1; }
dispatch_watch_job="suite-prefix-$$"
watch_dispatch_record="$root/artifacts/agents/jobs/$dispatch_watch_job.json"
mkdir -p "$(dirname "$watch_dispatch_record")"
printf '{"jobId":"%s","status":"completed","startedAt":"%s","workspaceRoot":"%s"}\n' \
  "$dispatch_watch_job" "$watch_now" "$watch_root" >"$watch_dispatch_record"
METASYSTEM_BIN="$bin" "$root/scripts/agents/dispatch.sh" watch \
  --job "$dispatch_watch_job" >"$dispatch_watch_out" 2>&1
grep -Fq 'inner:child since 0min' "$dispatch_watch_out" \
  || { echo "suite-progress fixture: dispatch watch did not print the deepest heartbeat" >&2; cat "$dispatch_watch_out" >&2; exit 1; }

# The bounded copier reports the exact truncated source in its loud result,
# while retaining the bytes that fit and its durable copy note.
bounded="$tmp/bounded"
mkdir -p "$bounded/source"
printf 'eight-bytes' >"$bounded/source/evidence"
"$bin" proof-run preserve --destination "$bounded/result" --max-bytes 4 \
  --source "$bounded/source" >"$bounded/out"
grep -Fq "DROPPED $bounded/source/evidence" "$bounded/out" \
  || { echo "suite-progress fixture: bounded evidence result did not name dropped content" >&2; cat "$bounded/out" >&2; exit 1; }
grep -Fq "DROPPED $bounded/source/evidence" "$bounded/result/copy-note.txt" \
  || { echo "suite-progress fixture: bounded evidence note did not name dropped content" >&2; exit 1; }

# A stopped suite cannot run cleanup. The sibling preserves evidence, resumes
# and kills the suite group, then sweeps the separately detached guard member.
stopped="$tmp/stopped"
stopped_out="$tmp/stopped.out"
if launch_fixture "$stopped" stopped stopped-section \
    'suite-cost suite=stopped witness=frozen duration=minutes heartbeat=progress.jsonl logs=logs/suite.log' \
    --silence-ms 300 --section-cap-ms 5000 --evidence-timeout-ms 1000 \
    --evidence-max-bytes 1048576 --poll-ms 50 --term-grace-ms 100 --kill-grace-ms 100 -- \
    bash "$root/scripts/agents/suite-progress-fixtures.sh" __stopped \
      "$stopped" "$stopped/progress.jsonl" stopped stopped-section "$bin" \
      >"$stopped_out" 2>&1; then
  echo "suite-progress fixture: a stopped suite passed" >&2
  exit 1
fi
grep -Fq 'suite stalled in section stopped-section' "$stopped_out" \
  || { echo "suite-progress fixture: stopped-suite failure did not name its section" >&2; cat "$stopped_out" >&2; exit 1; }
evidence_dir=$(find "$stopped/artifacts/agents/suite-failures" -mindepth 1 -maxdepth 1 -type d | head -1)
[[ -n "$evidence_dir" && -f "$evidence_dir/copy-note.txt" ]]
find "$evidence_dir" -type f -name evidence.txt -print -quit | grep -q . \
  || { echo "suite-progress fixture: external watchdog did not preserve in-suite evidence" >&2; exit 1; }
for pid_file in suite.pid group-child.pid detached.pid; do
  pid=$(cat "$stopped/tmp/$pid_file")
  deadline=$((SECONDS + reap_cap))
  while kill -0 "$pid" 2>/dev/null && (( SECONDS < deadline )); do sleep 0.05; done
  if kill -0 "$pid" 2>/dev/null; then
    echo "suite-progress fixture: watchdog left $pid_file process $pid alive" >&2
    exit 1
  fi
done

# A mismatched start identity authorizes neither supervision shutdown nor a
# signal. The live process remains until this fixture, which spawned it, reaps.
recycle="$tmp/recycle"
mkdir -p "$recycle"
printf '{"tmpPaths":[],"logPaths":["%s"]}\n' "$recycle/log" >"$recycle/progress.jsonl"
: >"$recycle/log"
sleep "$wait_cap" &
recycle_pid=$!
owned_pids+=("$recycle_pid")
if "$bin" proof-run watchdog --suite recycle --root "$recycle" \
    --conf "$root/metasystem.conf" \
    --progress "$recycle/progress.jsonl" --done "$recycle/done" --log "$recycle/log" \
    --suite-pid "$recycle_pid" --suite-started-at 1 --suite-start-ticks 0 --suite-boot-id '' \
    --silence-ms 100 --section-cap-ms 1000 --evidence-timeout-ms 1000 \
    --evidence-max-bytes 1048576 --poll-ms 25 --term-grace-ms 50 --kill-grace-ms 50 \
    >"$recycle/out" 2>&1; then
  echo "suite-progress fixture: recycled-identity watchdog passed" >&2
  exit 1
fi
kill -0 "$recycle_pid" 2>/dev/null \
  || { echo "suite-progress fixture: recycled-identity guard killed the wrong process" >&2; exit 1; }
grep -Fq 'kill refused because suite pid' "$recycle/out" \
  || { echo "suite-progress fixture: recycled-identity refusal was not loud" >&2; exit 1; }
kill "$recycle_pid" 2>/dev/null || true
wait "$recycle_pid" 2>/dev/null || true
owned_pids=()

echo "suite progress fixtures passed"
