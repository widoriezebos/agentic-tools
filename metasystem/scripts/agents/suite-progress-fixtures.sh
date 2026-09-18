#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
bin=${METASYSTEM_BIN:-$root/bin/metasystem}

if [ -n "${METASYSTEM_FIXTURE_OWNER-}" ]; then
  tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"
  attempt_tag=
  [ -z "${METASYSTEM_FIXTURE_ATTEMPT-}" ] || attempt_tag="METASYSTEM_FIXTURE_ATTEMPT=$METASYSTEM_FIXTURE_ATTEMPT"
  if [ "${1-}" != "$tag" ]; then
    [ -z "$attempt_tag" ] || exec /bin/sh "$0" "$tag" "$attempt_tag" "$@"
    exec /bin/sh "$0" "$tag" "$@"
  fi
  if [ -n "$attempt_tag" ] && [ "${2-}" != "$attempt_tag" ]; then
    shift
    exec /bin/sh "$0" "$tag" "$attempt_tag" "$@"
  fi
  shift
  [ -z "$attempt_tag" ] || shift
fi

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
  __printing_until)
    release=$2 progress=$3 suite=$4 section=$5
    printf '{"suite":"%s","section":"%s","event":"start","at":"%s","depth":0}\n' \
      "$suite" "$section" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
    while [[ ! -e "$release" ]]; do
      echo "fixture output is still growing"
      sleep 0.05
    done
    printf '{"suite":"%s","section":"%s","event":"end","at":"%s","depth":0}\n' \
      "$suite" "$section" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
    exit 0
    ;;
  __detached)
    trap '' TERM
    if [ -n "${METASYSTEM_DETACHED_READY-}" ]; then
      : >"$METASYSTEM_DETACHED_READY"
    fi
    if [ -n "${METASYSTEM_FIXTURE_LEASH-}" ]; then
      exec 3<"$METASYSTEM_FIXTURE_LEASH"
      read -r _ <&3
    else
      while :; do sleep 1; done
    fi
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
      --env "METASYSTEM_DETACHED_READY=$tmp/detached.ready" \
      -- bash "$root/scripts/agents/suite-progress-fixtures.sh" "$harness_fixture_tag" __detached)
    echo "$detached_pid" >"$tmp/detached.pid"
    printf 'evidence written before stop\n' >"$tmp/evidence.txt"
    printf '{"suite":"%s","section":"%s","event":"start","at":"%s","depth":0}\n' \
      "$suite" "$section" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
    # The watchdog ends a suite for the supervisor's dead or runaway verdict
    # and for a cancellation intent, never for silence. An inspection release lets
    # the parent inspect the stopped suite before a sibling publishes that same
    # verdict; the watchdog still owns the CONT/TERM/KILL ladder.
    if [[ -n ${METASYSTEM_STOPPED_WITNESS_RELEASE:-} ]]; then
      (
        kill -STOP $$
        while [[ ! -e "$METASYSTEM_STOPPED_WITNESS_RELEASE" ]]; do sleep 0.05; done
        printf '{"suite":"%s","section":"%s","event":"verdict","at":"%s","depth":0,"verdict":"dead"}\n' \
          "$suite" "$section" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
      ) &
      wait
    else
      printf '{"suite":"%s","section":"%s","event":"verdict","at":"%s","depth":0,"verdict":"dead"}\n' \
        "$suite" "$section" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$progress"
      kill -STOP $$
    fi
    exit 99
    ;;
esac

[[ -x "$bin" ]] || { echo "suite-progress fixture: build bin/metasystem first" >&2; exit 1; }
source "$root/scripts/agents/fixture-budget.sh"
harness_fixture_owner "$root"
harness_fixture_budget_init "$root"
wait_cap=$(harness_fixture_cap suite-watchdog-wait)
reap_cap=$(harness_fixture_cap suite-watchdog-reap)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/suite-progress-fixtures.XXXXXX")
tmp=$(cd "$tmp" && pwd -P)
owned_pids=()
cleanup() {
  local status=$?
  trap - EXIT
  harness_fixture_reap || status=1
  rm -rf "$tmp"
  return "$status"
}
trap cleanup EXIT

launch_fixture() { # bed, suite, section, banner, extra launcher flags -- command
  local bed=$1 suite=$2 section=$3 banner=$4
  shift 4
  mkdir -p "$bed/tmp" "$bed/logs"
  env -u METASYSTEM_PROOF_CONTROL_ROOT -u METASYSTEM_PROOF_ATTEMPT \
    -u METASYSTEM_PROOF_RUN_ROOT -u METASYSTEM_PROOF_RUN_ID \
    -u METASYSTEM_HOOK_DELEGATE_STATE_ROOT -u METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT \
    -u METASYSTEM_HOOK_DELEGATE_JOB -u METASYSTEM_PROOF_RECORD_KEY \
    -u METASYSTEM_PROOF_CREATION_CLAIM -u METASYSTEM_PROOF_AUTH_BIN \
    METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" \
    "$bin" proof-run launch --suite "$suite" --root "$bed" --control-root "$bed" --conf "$root/metasystem.conf" \
    --progress "$bed/progress.jsonl" --log "$bed/logs/suite.log" --tmp "$bed/tmp" \
    --banner "$banner" --selected "$section" "$@" 9>&-
}

wait_for_exact_death() { # name, pid, exact ref
  local name=$1 pid=$2 ref=$3 current deadline
  deadline=$((SECONDS + reap_cap))
  while current=$(harness_fixture_engine_call proc ref --pid "$pid" 2>/dev/null) \
      && [[ "$current" == "$ref" ]] && (( SECONDS < deadline )); do
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  if current=$(harness_fixture_engine_call proc ref --pid "$pid" 2>/dev/null) && [[ "$current" == "$ref" ]]; then
    echo "suite-progress fixture: $name remained alive at $ref" >&2
    return 1
  fi
}

# Output growth keeps a section alive beyond the shortened silence window.
printing="$tmp/printing"
printing_banner='suite-cost suite=printing witness=armed duration=minutes heartbeat=progress.jsonl logs=logs/suite.log'
launch_fixture "$printing" printing long-printing "$printing_banner" \
  --silence-ms 300 --section-cap-ms 10000 --evidence-timeout-ms 1000 \
  --evidence-max-bytes 1048576 --poll-ms 50 --term-grace-ms 100 --kill-grace-ms 100 -- \
  bash "$root/scripts/agents/suite-progress-fixtures.sh" "$harness_fixture_tag" __printing \
    "$printing/progress.jsonl" printing long-printing
[[ $(grep -c -xF "$printing_banner" "$printing/logs/suite.log") -eq 1 ]] \
  || { echo "suite-progress fixture: cost banner was not logged exactly once" >&2; exit 1; }

# A chatty section past its cap runs on: the clock ends nothing (decision 3
# of the hang-detection design). The watchdog notes the cap once, the
# section finishes its own work, and the launch succeeds.
chatty="$tmp/chatty"
chatty_out="$tmp/chatty.out"
chatty_release="$tmp/chatty.release"
chatty_note='section over-cap passed its 400ms cap while still producing output'
launch_fixture "$chatty" chatty over-cap \
  'suite-cost suite=chatty witness=armed duration=minutes heartbeat=progress.jsonl logs=logs/suite.log' \
  --silence-ms 2000 --section-cap-ms 400 --evidence-timeout-ms 1000 \
  --evidence-max-bytes 1048576 --poll-ms 50 --term-grace-ms 100 --kill-grace-ms 100 -- \
  bash "$root/scripts/agents/suite-progress-fixtures.sh" "$harness_fixture_tag" __printing_until \
    "$chatty_release" "$chatty/progress.jsonl" chatty over-cap >"$chatty_out" 2>&1 &
chatty_pid=$!
owned_pids+=("$chatty_pid")
harness_fixture_hold_pid "$chatty_pid"
chatty_deadline=$((SECONDS + wait_cap))
chatty_noted=0
while (( SECONDS < chatty_deadline )); do
  if grep -Fq "$chatty_note" "$chatty_out"; then
    chatty_noted=1
    break
  fi
  kill -0 "$chatty_pid" 2>/dev/null || break
  sleep 0.05
done
if (( chatty_noted == 0 )); then
  kill "$chatty_pid" 2>/dev/null || true
  wait "$chatty_pid" 2>/dev/null || true
  owned_pids=()
  echo "suite-progress fixture: the watchdog did not note the section past its cap within ${wait_cap}s" >&2
  sed 's/^/  launcher: /' "$chatty_out" >&2
  exit 1
fi
: >"$chatty_release"
chatty_status=0
wait "$chatty_pid" || chatty_status=$?
owned_pids=()
if (( chatty_status != 0 )); then
  echo "suite-progress fixture: a printing section past its cap ended with status $chatty_status" >&2
  sed 's/^/  launcher: /' "$chatty_out" >&2
  exit 1
fi

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

# Delivery validation uses the adopted section set even when its files live in
# the template checkout. The same selector view drives the completion check:
# absent inactive sections are valid, while every active section remains owed.
context_selector="$root/scripts/agents/validate-section-selector.sh"
context_bed="$tmp/context-active"
context_progress="$context_bed/progress.jsonl"
context_missing_progress="$context_bed/missing-progress.jsonl"
context_sections="$context_bed/sections"
mkdir -p "$context_bed"
METASYSTEM_DELIVERY_CONTRACT=1 bash "$context_selector" list \
  | cut -f1 >"$context_sections"
for inactive_section in adoption-fixtures \
    witness-gate-fixtures suite-progress-fixtures land-fixtures \
    fixture-bed-scenarios-fixtures; do
  grep -qxF "$inactive_section" "$context_sections" && {
    echo "suite-progress fixture: delivery context retained inactive section $inactive_section" >&2
    exit 1
  }
done
printf '{"tmpPaths":[],"logPaths":["suite.log"]}\n' >"$context_progress"
printf '{"tmpPaths":[],"logPaths":["suite.log"]}\n' >"$context_missing_progress"
context_now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
while IFS= read -r active_section; do
  printf '{"suite":"context-active","section":"%s","event":"start","at":"%s","depth":0}\n' \
    "$active_section" "$context_now" >>"$context_progress"
  printf '{"suite":"context-active","section":"%s","event":"end","at":"%s","depth":0}\n' \
    "$active_section" "$context_now" >>"$context_progress"
  if [[ "$active_section" != engine-delivery-contract ]]; then
    printf '{"suite":"context-active","section":"%s","event":"start","at":"%s","depth":0}\n' \
      "$active_section" "$context_now" >>"$context_missing_progress"
    printf '{"suite":"context-active","section":"%s","event":"end","at":"%s","depth":0}\n' \
      "$active_section" "$context_now" >>"$context_missing_progress"
  fi
done <"$context_sections"
METASYSTEM_DELIVERY_CONTRACT=1 "$bin" proof-run assert \
  --progress "$context_progress" --suite context-active --selector "$context_selector"
context_missing_out="$context_bed/missing.out"
if METASYSTEM_DELIVERY_CONTRACT=1 "$bin" proof-run assert \
    --progress "$context_missing_progress" --suite context-active --selector "$context_selector" \
    >"$context_missing_out" 2>&1; then
  echo "suite-progress fixture: an active delivery section was not required" >&2
  exit 1
fi
grep -Fq 'engine-delivery-contract has 0 starts and 0 ends' "$context_missing_out" \
  || { echo "suite-progress fixture: missing active section was not named" >&2; cat "$context_missing_out" >&2; exit 1; }

# Both runner-facing watchers relay the deepest open section from the same
# read-only journal view. The background watcher prefixes a reportable job;
# the command-level printer is covered at its Go-owned boundary.
watch_repository="$tmp/watch-repository"
watch_root="$watch_repository/metasystem"
watch_workspace="$tmp/watch-workspace"
watch_jobs="$tmp/watch-jobs"
watch_state="$tmp/watch.state"
watch_out="$tmp/watch.out"
dispatch_watch_out="$tmp/dispatch-watch.out"
mkdir -p "$watch_workspace/artifacts/agents/supervision" \
  "$watch_root/artifacts/agents/jobs" "$watch_root/scripts/agents/adapters" \
  "$watch_root/bin" "$watch_jobs"
git -C "$watch_repository" init -q -b main
# dispatch.sh derives jobs and waiter directories from its own location, so its
# private installation needs a copied binary rather than a symlink.
cp "$root/scripts/agents/dispatch.sh" "$watch_root/scripts/agents/dispatch.sh"
cp "$root/scripts/agents/checkout-execution-guard.sh" "$watch_root/scripts/agents/checkout-execution-guard.sh"
cp "$root/scripts/agents/adapters/fake.sh" "$watch_root/scripts/agents/adapters/fake.sh"
cp "$bin" "$watch_root/bin/metasystem"
printf 'metasystem.runtimes=fake\nrole.default.model.fake=fake-model\n' >"$watch_root/metasystem.conf"
watch_run_id=$(printf '%s' "${tmp##*/}" | tr '[:upper:]' '[:lower:]' | tr . -)
watch_session="$watch_run_id"
"$watch_root/bin/metasystem" lease announce --root "$watch_root" \
  --session "$watch_session" --pid $$ \
  --start "$("$watch_root/bin/metasystem" proc started-at --pid $$)" \
  --tag "$watch_session" --runtime fake --owner-lineage "$watch_session" >/dev/null
watch_now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '{"tmpPaths":[],"logPaths":["suite.log"]}\n' >"$watch_workspace/artifacts/agents/supervision/suite-progress.jsonl"
printf '{"suite":"outer","section":"parent","event":"start","at":"%s","depth":0}\n' "$watch_now" \
  >>"$watch_workspace/artifacts/agents/supervision/suite-progress.jsonl"
printf '{"suite":"inner","section":"child","event":"start","at":"%s","depth":1}\n' "$watch_now" \
  >>"$watch_workspace/artifacts/agents/supervision/suite-progress.jsonl"
[[ "$("$bin" proof-run heartbeat --root "$watch_workspace")" == 'inner:child since 0min' ]] \
  || { echo "suite-progress fixture: deepest live heartbeat was not selected" >&2; exit 1; }
printf '{"status":"completed","workspaceRoot":"%s"}\n' "$watch_workspace" >"$watch_jobs/prefix-job.json"
: >"$watch_state"
METASYSTEM_BIN="$bin" "$root/scripts/watch-background-jobs.sh" \
  --dir "$watch_jobs" --scope "$watch_workspace" --state "$watch_state" --once >"$watch_out" 2>&1
grep -Fq 'inner:child since 0min DONE prefix-job status=completed' "$watch_out" \
  || { echo "suite-progress fixture: background watcher did not prefix its job note with the deepest heartbeat" >&2; cat "$watch_out" >&2; exit 1; }
dispatch_watch_job="suite-prefix-$watch_run_id"
watch_dispatch_record="$watch_root/artifacts/agents/jobs/$dispatch_watch_job.json"
printf '{"jobId":"%s","operationId":"reserve-%s","round":1,"status":"completed","startedAt":"%s","endedAt":"%s","workspaceRoot":"%s"}\n' \
  "$dispatch_watch_job" "$dispatch_watch_job" "$watch_now" "$watch_now" "$watch_workspace" >"$watch_dispatch_record"
METASYSTEM_BIN="$watch_root/bin/metasystem" "$watch_root/scripts/agents/dispatch.sh" watch \
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

# A stopped suite cannot run cleanup. Judged dead (the fixture writes the
# supervisor's verdict), the sibling preserves evidence, resumes and kills
# the suite group, then sweeps the separately detached guard member.
stopped="$tmp/stopped"
stopped_out="$tmp/stopped.out"
launch_fixture "$stopped" stopped stopped-section \
    'suite-cost suite=stopped witness=frozen duration=minutes heartbeat=progress.jsonl logs=logs/suite.log' \
    --silence-ms 300 --section-cap-ms 5000 --evidence-timeout-ms 1000 \
    --evidence-max-bytes 1048576 --poll-ms 50 --term-grace-ms 100 --kill-grace-ms 100 -- \
    env METASYSTEM_STOPPED_WITNESS_RELEASE="$stopped/tmp/assertions.done" \
      bash "$root/scripts/agents/suite-progress-fixtures.sh" "$harness_fixture_tag" __stopped \
      "$stopped" "$stopped/progress.jsonl" stopped stopped-section "$bin" \
      >"$stopped_out" 2>&1 &
stopped_launcher_pid=$!
harness_fixture_hold_pid "$stopped_launcher_pid"
stopped_deadline=$((SECONDS + wait_cap))
while [[ ! -s "$stopped/tmp/suite.pid" || ! -s "$stopped/tmp/group-child.pid" \
    || ! -s "$stopped/tmp/detached.pid" || ! -e "$stopped/tmp/detached.ready" ]] \
    && kill -0 "$stopped_launcher_pid" 2>/dev/null && (( SECONDS < stopped_deadline )); do
  sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
done
if [[ ! -s "$stopped/tmp/suite.pid" || ! -s "$stopped/tmp/group-child.pid" \
    || ! -s "$stopped/tmp/detached.pid" || ! -e "$stopped/tmp/detached.ready" ]]; then
  echo "suite-progress fixture: stopped suite did not publish its three pids and detached readiness" >&2
  wait "$stopped_launcher_pid" 2>/dev/null || true
  cat "$stopped_out" >&2
  exit 1
fi
suite_pid=$(<"$stopped/tmp/suite.pid")
group_child_pid=$(<"$stopped/tmp/group-child.pid")
detached_pid=$(<"$stopped/tmp/detached.pid")
harness_fixture_hold_pid "$suite_pid"
harness_fixture_hold_pid "$group_child_pid"
harness_fixture_hold_pid "$detached_pid"
stopped_state=
stopped_deadline=$((SECONDS + wait_cap))
while (( SECONDS < stopped_deadline )); do
  stopped_state=$(ps -o state= -p "$suite_pid" 2>/dev/null | tr -d '[:space:]')
  [[ "$stopped_state" == T* ]] && break
  kill -0 "$stopped_launcher_pid" 2>/dev/null || break
  sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
done
[[ "$stopped_state" == T* ]] || {
  echo "suite-progress fixture: suite.pid $suite_pid state=${stopped_state:-absent}, want T while the bed lives" >&2
  exit 1
}
detached_ref=$(harness_fixture_engine_call proc ref --pid "$detached_pid")
kill -TERM "$detached_pid"
detached_after_term=$(harness_fixture_engine_call proc ref --pid "$detached_pid" 2>/dev/null || true)
[[ "$detached_after_term" == "$detached_ref" ]] || {
  echo "suite-progress fixture: detached.pid $detached_pid did not remain alive at its exact identity after TERM (before=$detached_ref after=${detached_after_term:-dead})" >&2
  exit 1
}
: >"$stopped/tmp/assertions.done"
stopped_status=0
wait "$stopped_launcher_pid" || stopped_status=$?
if (( stopped_status == 0 )); then
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

# Killing a bed owner leaves no watchdog to help its stopped runner or its
# detached member. The custodian owns both through the bed's fixture key.
stopped_owner="$tmp/stopped-owner-killed"
mkdir -p "$stopped_owner/tmp" "$stopped_owner/logs"
: >"$stopped_owner/progress.jsonl"
METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" bash -c '
  set -euo pipefail
  root=$1 bin=$2 bed=$3
  source "$root/scripts/agents/fixture-budget.sh"
  harness_fixture_owner "$root"
  harness_fixture_key stopped-owner-killed
  METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" \
    bash "$root/scripts/agents/suite-progress-fixtures.sh" "$harness_fixture_tag" __stopped \
    "$bed" "$bed/progress.jsonl" stopped-owner stopped-owner-section "$bin" 9>&- &
  runner=$!
  harness_fixture_hold_pid "$runner"
  printf "%s\n" "$runner" >"$bed/tmp/owner-runner.pid"
  wait "$runner"
' bash "$root" "$bin" "$stopped_owner" 9>&- >"$stopped_owner/owner.out" 2>&1 &
stopped_owner_pid=$!
harness_fixture_hold_pid "$stopped_owner_pid"
stopped_owner_deadline=$((SECONDS + wait_cap))
while [[ ! -s "$stopped_owner/tmp/owner-runner.pid" || ! -s "$stopped_owner/tmp/detached.pid" \
    || ! -e "$stopped_owner/tmp/detached.ready" ]] \
    && kill -0 "$stopped_owner_pid" 2>/dev/null && (( SECONDS < stopped_owner_deadline )); do
  sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
done
if [[ ! -s "$stopped_owner/tmp/owner-runner.pid" || ! -s "$stopped_owner/tmp/detached.pid" \
    || ! -e "$stopped_owner/tmp/detached.ready" ]]; then
  echo "suite-progress fixture: stopped-owner-killed did not publish its runner and detached member" >&2
  cat "$stopped_owner/owner.out" >&2
  exit 1
fi
stopped_owner_runner=$(<"$stopped_owner/tmp/owner-runner.pid")
stopped_owner_detached=$(<"$stopped_owner/tmp/detached.pid")
harness_fixture_hold_pid "$stopped_owner_runner"
harness_fixture_hold_pid "$stopped_owner_detached"
stopped_owner_runner_ref=$(harness_fixture_engine_call proc ref --pid "$stopped_owner_runner")
stopped_owner_detached_ref=$(harness_fixture_engine_call proc ref --pid "$stopped_owner_detached")
kill -KILL "$stopped_owner_pid"
wait "$stopped_owner_pid" 2>/dev/null || true
wait_for_exact_death "stopped-owner-killed runner" "$stopped_owner_runner" "$stopped_owner_runner_ref"
wait_for_exact_death "stopped-owner-killed detached member" "$stopped_owner_detached" "$stopped_owner_detached_ref"

# The held fake host ignores TERM but still belongs to the shell that started
# it, so killing that owner leaves no host behind.
fake_owner="$tmp/fake-host-owner-killed"
fake_root="$fake_owner/root"
fake_turn="$fake_root/turn"
mkdir -p "$fake_root/bin" "$fake_root/scripts/agents/hosts" "$fake_turn"
cp "$bin" "$fake_root/bin/metasystem"
cp "$root/scripts/agents/hosts/fake.sh" "$root/scripts/agents/hosts/host-common.sh" \
  "$fake_root/scripts/agents/hosts/"
cp "$root/metasystem.conf" "$fake_root/metasystem.conf"
conf_edit "$fake_root/metasystem.conf" replace-line-first '^metasystem[.]runtimes=.*$' \
  'metasystem.runtimes=fake'
printf '{"missionId":"fixture-host","turnId":"fixture-turn","cycle":1}\n' >"$fake_turn/turn.json"
printf 'FAKEHOST:no-return\n' >"$fake_turn/prompt.md"
METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" bash -c '
  set -euo pipefail
  source_root=$1 fixture_root=$2 turn=$3
  METASYSTEM_BIN="$fixture_root/bin/metasystem"
  export METASYSTEM_BIN
  source "$source_root/scripts/agents/fixture-budget.sh"
  harness_fixture_owner "$fixture_root"
  harness_fixture_key fake-host-owner-killed
  METASYSTEM_FAKE_HOST_HOLD=1 METASYSTEM_FAKE_HOST_IGNORE_TERM=1 \
    METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" \
    "$fixture_root/scripts/agents/hosts/fake.sh" "$harness_fixture_tag" start-turn \
      --mission fixture-host --turn-id fixture-turn --prompt "$turn/prompt.md" \
      --result "$turn/result.json" --instance-tag fixture-host 9>&- &
  host=$!
  harness_fixture_hold_pid "$host"
  printf "%s\n" "$host" >"$turn/host.pid"
  wait "$host"
' bash "$root" "$fake_root" "$fake_turn" 9>&- >"$fake_owner/owner.out" 2>&1 &
fake_owner_pid=$!
harness_fixture_hold_pid "$fake_owner_pid"
fake_deadline=$((SECONDS + wait_cap))
while [[ ! -s "$fake_turn/host.pid" || ! -e "$fake_turn/host-ready" ]] \
    && kill -0 "$fake_owner_pid" 2>/dev/null && (( SECONDS < fake_deadline )); do
  sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
done
if [[ ! -s "$fake_turn/host.pid" || ! -e "$fake_turn/host-ready" ]]; then
  echo "suite-progress fixture: fake-host-owner-killed did not reach host-ready" >&2
  cat "$fake_owner/owner.out" >&2
  exit 1
fi
fake_host_pid=$(<"$fake_turn/host.pid")
harness_fixture_hold_pid "$fake_host_pid"
fake_host_ref=$(harness_fixture_engine_call proc ref --pid "$fake_host_pid")
kill -TERM "$fake_host_pid"
fake_host_after_term=$(harness_fixture_engine_call proc ref --pid "$fake_host_pid" 2>/dev/null || true)
[[ "$fake_host_after_term" == "$fake_host_ref" ]] || {
  echo "suite-progress fixture: fake host $fake_host_pid did not remain alive at its exact identity after TERM" >&2
  exit 1
}
kill -KILL "$fake_owner_pid"
wait "$fake_owner_pid" 2>/dev/null || true
wait_for_exact_death "fake-host-owner-killed host" "$fake_host_pid" "$fake_host_ref"

# A mismatched start identity authorizes neither supervision shutdown nor a
# signal. The live process remains until this fixture, which spawned it, reaps.
# The watchdog attempts a stop only on the supervisor's verdict (decision 3
# of the hang-detection design), so the progress file carries a dead verdict
# for the section, written here in the supervisor's place.
recycle="$tmp/recycle"
mkdir -p "$recycle"
{
  printf '{"tmpPaths":[],"logPaths":["%s"]}\n' "$recycle/log"
  printf '{"suite":"recycle","section":"guard","event":"start","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '{"suite":"recycle","section":"guard","event":"verdict","at":"%s","depth":0,"verdict":"dead"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >"$recycle/progress.jsonl"
: >"$recycle/log"
METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" sleep "$wait_cap" 9>&- &
recycle_pid=$!
owned_pids+=("$recycle_pid")
harness_fixture_hold_pid "$recycle_pid"
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
