#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$root"
health_state_cap_sec=$(harness_fixture_cap health-state)
health_process_cap_sec=$(harness_fixture_cap health-process-wait)

ms=${METASYSTEM_BIN:-$root/bin/metasystem}
[[ -x "$ms" ]] || { echo "health fixtures: binary absent; build the engine first" >&2; exit 1; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-health.XXXXXX")
runner_pid=
owner_pid=
watcher_pid=
main_pid=
cleanup_started=0

continue_recorded_pids() {
  local pid
  for pid in "$runner_pid" "$owner_pid" "$watcher_pid" "$main_pid"; do
    [[ -n "$pid" ]] || continue
    kill -CONT "$pid" 2>/dev/null || true
  done
}

recorded_pids_alive() {
  local pid
  for pid in "$runner_pid" "$owner_pid" "$watcher_pid" "$main_pid"; do
    [[ -n "$pid" ]] || continue
    kill -0 "$pid" 2>/dev/null && return 0
  done
  return 1
}

refresh_recorded_runner_pid() {
  local record=$tmp/repo/artifacts/agents/steward/runner.json recorded
  [[ -f "$record" ]] || return 0
  recorded=$(sed -n 's/^[[:space:]]*"pid":[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$record")
  [[ -n "$recorded" ]] && runner_pid=$recorded
}

wait_for_recorded_pids_to_exit() { # signal stage
  local stage=$1 started=$SECONDS deadline=$((SECONDS + health_process_cap_sec)) elapsed
  while recorded_pids_alive; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "health fixture cleanup ceiling reached: $stage (elapsed: ${elapsed}s; scaled cap: ${health_process_cap_sec}s)" >&2
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

cleanup() {
  (( cleanup_started )) && return 0
  cleanup_started=1
  local pid

  # Arm and restart launch the detached runner before their command returns.
  # Read its durable record so a signal in that small handoff window cannot
  # leave a runner that the shell has not assigned to runner_pid yet.
  refresh_recorded_runner_pid
  # A stopped process cannot act on TERM until it is continued. Repeat CONT
  # before KILL too, so every cleanup signal has the same safe ordering.
  continue_recorded_pids
  for pid in "$runner_pid" "$owner_pid" "$watcher_pid" "$main_pid"; do
    [[ -n "$pid" ]] || continue
    kill -TERM "$pid" 2>/dev/null || true
  done
  if ! wait_for_recorded_pids_to_exit TERM; then
    continue_recorded_pids
    for pid in "$runner_pid" "$owner_pid" "$watcher_pid" "$main_pid"; do
      [[ -n "$pid" ]] || continue
      kill -KILL "$pid" 2>/dev/null || true
    done
    wait_for_recorded_pids_to_exit KILL || true
  fi
  if ! rm -rf "$tmp"; then
    echo "health fixture cleanup could not remove $tmp" >&2
  fi
}
on_signal() {
  local status=$1
  trap - HUP INT QUIT TERM
  exit "$status"
}
trap cleanup EXIT
trap 'on_signal 129' HUP
trap 'on_signal 130' INT
trap 'on_signal 131' QUIT
trap 'on_signal 143' TERM

fail() { echo "health fixture failed: $1" >&2; exit 1; }

repo=$tmp/repo
mkdir -p "$repo/plans" "$repo/artifacts/agents/supervision/lock.d" \
  "$repo/artifacts/agents/mains" "$repo/artifacts/agents/jobs"
git -C "$repo" init -q -b main
git -C "$repo" config user.name fixture
git -C "$repo" config user.email fixture@example.invalid
git -C "$repo" config metasystem.steward.tick-seconds 1

printf '%s\n' \
  'metasystem.runtimes=none' \
  'watch.stale-min=20' \
  'watch.interval-sec=60' \
  'capability.snapshot-max-age-days=30' >"$repo/metasystem.conf"
printf '# Goals\n\n## Goal-free: declared 2026-08-28T00:00:00Z by human over fixture\n' >"$repo/plans/goals.md"
git -C "$repo" add metasystem.conf plans/goals.md
git -C "$repo" commit -qm fixture

cat >"$tmp/notifier.sh" <<'NOTIFIER'
#!/usr/bin/env bash
printf '%s\n' "$STEWARD_MESSAGE" >>"$HEALTH_FIXTURE_ALERTS"
NOTIFIER
chmod +x "$tmp/notifier.sh"
export HEALTH_FIXTURE_ALERTS=$tmp/alerts.log
git -C "$repo" config metasystem.steward.notify-command "$tmp/notifier.sh"

"$ms" util hold --tag health-owner-$$ >/dev/null 2>&1 & owner_pid=$!
"$ms" util hold --tag health-watcher-$$ >/dev/null 2>&1 & watcher_pid=$!
"$ms" util hold --tag health-main-$$ >/dev/null 2>&1 & main_pid=$!

owner_start=$("$ms" proc started-at --pid "$owner_pid") || fail "owner start identity unreadable"
watcher_start=$("$ms" proc started-at --pid "$watcher_pid") || fail "watcher start identity unreadable"
main_start=$("$ms" proc started-at --pid "$main_pid") || fail "main start identity unreadable"
now_epoch=$(date -u +%s)
now_iso=$(date -u +%Y-%m-%dT%H:%M:%SZ)

printf '{"generation":1,"intervalSec":60,"owner":{"pid":%s,"pidStartedAt":%s,"instanceTag":"health-owner-%s"},"components":{"watcher":{"pid":%s,"pidStartedAt":%s,"instanceTag":"health-watcher-%s"}}}\n' \
  "$owner_pid" "$owner_start" "$$" "$watcher_pid" "$watcher_start" "$$" \
  >"$repo/artifacts/agents/supervision/state.json"
printf '{"pid":%s,"pidStartedAt":%s,"instanceTag":"health-owner-%s"}\n' \
  "$owner_pid" "$owner_start" "$$" >"$repo/artifacts/agents/supervision/lock.d/owner.json"
printf '{"schemaVersion":2,"writer":"watch-background-jobs.sh","verdict":"SUCCESS","completedAtEpoch":%s,"intervalSec":60,"generation":1,"fingerprint":"fixture","counts":{},"inventory":[],"diagnostics":[],"errors":[]}\n' \
  "$now_epoch" >"$repo/artifacts/agents/supervision/last-census.json"
printf '{"sessionId":"fixture","mainId":"main-fixture","pid":%s,"pidStartedAt":%s,"runtime":"fake","instanceTag":"health-main-%s"}\n' \
  "$main_pid" "$main_start" "$$" >"$repo/artifacts/agents/mains/fixture.json"
mkdir -p "$repo/artifacts/agents/steward/components"
watcher_digest=$(printf '%s' 'fixture watcher pass' | "$ms" util sha256)
printf '{"component":"repo-watcher","generation":1,"pid":%s,"pidStartedAt":%s,"successPid":%s,"successPidStartedAt":%s,"successAttemptSeq":1,"attemptSeq":1,"lastAttempt":"%s","lastCompletion":"%s","lastSuccess":"%s","result":"OK","outcome":"PASS_COMPLETE","evidenceDigest":"%s"}\n' \
  "$watcher_pid" "$watcher_start" "$watcher_pid" "$watcher_start" "$now_iso" "$now_iso" "$now_iso" "$watcher_digest" \
  >"$repo/artifacts/agents/steward/components/repo-watcher.json"

hook_attempt=$("$ms" steward hook-attempt --repo "$repo" --pid "$$" --turn-key health-fixture) || fail "hook attempt could not be recorded"
hook_generation=$("$ms" json get --value "$hook_attempt" --field generation) || fail "hook generation unreadable"
hook_attempt_seq=$("$ms" json get --value "$hook_attempt" --field attemptSeq) || fail "hook attempt sequence unreadable"
printf '%s\n' '{"systemMessage":"HEALTH fixture"}' >"$tmp/hook-response.json"
"$ms" steward hook-complete --repo "$repo" --generation "$hook_generation" --attempt "$hook_attempt_seq" \
  --result OK --outcome EMITTED --health-line 'HEALTH fixture' --payload-file "$tmp/hook-response.json" \
  || fail "hook completion could not be recorded"

run_health() {
  local name=$1
  set +e
  "$ms" health --repo "$repo" >"$tmp/$name.out" 2>"$tmp/$name.err"
  health_rc=$?
  set -e
}

read_runner_pid() {
  sed -n 's/^[[:space:]]*"pid":[[:space:]]*\([0-9][0-9]*\).*/\1/p' \
    "$repo/artifacts/agents/steward/runner.json" | head -1
}

wait_for_healthy() {
  local label=$1 started=$SECONDS deadline=$((SECONDS + health_state_cap_sec)) elapsed
  while :; do
    run_health "$label"
    if [[ "$health_rc" -eq 0 ]]; then
      return 0
    fi
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      cat "$tmp/$label.out" >&2 2>/dev/null || true
      cat "$tmp/$label.err" >&2 2>/dev/null || true
      echo "health fixture wait ceiling reached: $label healthy (elapsed: ${elapsed}s; scaled cap: ${health_state_cap_sec}s)" >&2
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

wait_for_pid_exit() { # name, pid
  local name=$1 pid=$2 started=$SECONDS deadline=$((SECONDS + health_process_cap_sec)) elapsed
  while kill -0 "$pid" 2>/dev/null; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "health fixture wait ceiling reached: $name pid=$pid (elapsed: ${elapsed}s; scaled cap: ${health_process_cap_sec}s)" >&2
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

"$ms" steward arm --repo "$repo" >"$tmp/arm.out" 2>"$tmp/arm.err" || {
  cat "$tmp/arm.err" >&2
  fail "initial arm failed"
}
runner_pid=$(read_runner_pid)
[[ -n "$runner_pid" ]] || fail "initial arm recorded no runner pid"
wait_for_healthy healthy || fail "armed repository did not become healthy"
for role in steward-runner supervision-owner repo-watcher census-freshness narrator-freshness session-main hook-freshness claimed-goal-appetite nonterminal-jobs capability-snapshots; do
  grep -Fq "$role=alive" "$tmp/healthy.out" || fail "healthy line omitted $role"
done

# Exit 2 uses a malformed job record; runner and narrator failures below use
# their actual processes rather than edited completion evidence.
printf '{"jobId":"unknown-job"}\n' >"$repo/artifacts/agents/jobs/unknown-job.json"
run_health unknown
unknown_rc=$health_rc
[[ "$unknown_rc" -eq 2 ]] || { cat "$tmp/unknown.err" >&2; fail "unknown bed returned $unknown_rc"; }
grep -Fq 'nonterminal-jobs=unknown' "$tmp/unknown.out" || fail "unknown verdict did not name the malformed job"
rm "$repo/artifacts/agents/jobs/unknown-job.json"
wait_for_healthy unknown-recovered || fail "unknown recovery did not become healthy"

# Stopping the resident loop stalls narrator production without altering its
# evidence. Equality at two producer intervals must make it stale.
kill -STOP "$runner_pid"
narrator_stalled=
narrator_started=$SECONDS
narrator_deadline=$((SECONDS + health_state_cap_sec))
while :; do
  run_health narrator-stalled
  if grep -Fq 'narrator-freshness=dead' "$tmp/narrator-stalled.out"; then
    narrator_stalled=yes
    break
  fi
  if (( SECONDS >= narrator_deadline )); then
    narrator_elapsed=$((SECONDS - narrator_started))
    cat "$tmp/narrator-stalled.out" >&2 2>/dev/null || true
    cat "$tmp/narrator-stalled.err" >&2 2>/dev/null || true
    echo "health fixture wait ceiling reached: stopped narrator stale (elapsed: ${narrator_elapsed}s; scaled cap: ${health_state_cap_sec}s)" >&2
    break
  fi
  sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
done
[[ "$narrator_stalled" == yes ]] || fail "stopped ticks did not make narrator evidence stale"
grep -Fq 'steward restart --repo' "$tmp/narrator-stalled.out" || fail "stale narrator omitted the restart remedy"
continue_recorded_pids
"$ms" steward restart --repo "$repo" >"$tmp/narrator-restart.out" 2>"$tmp/narrator-restart.err" || {
  cat "$tmp/narrator-restart.err" >&2
  fail "the stale narrator's printed restart remedy failed"
}
runner_pid=$(read_runner_pid)
wait_for_healthy narrator-recovered || fail "the stale narrator's printed restart remedy did not heal health"

# Killing the actual resident runner proves the acceptance path and starts the
# five-observation breaker from a healthy reset.
kill "$runner_pid"
wait_for_pid_exit "killed runner exit" "$runner_pid" || fail "killed runner did not exit"
# This bed isolates the consecutive-failure breaker from the earlier
# stop-and-recover exercise, which deliberately contributes a flap episode.
rm -f "$repo/artifacts/agents/steward/health.json" "$tmp/alerts.log"
run_health runner-dead
dead_rc=$health_rc
[[ "$dead_rc" -eq 1 ]] || { cat "$tmp/runner-dead.err" >&2; fail "dead bed returned $dead_rc"; }
grep -Fq 'steward-runner=dead' "$tmp/runner-dead.out" || fail "dead verdict did not name the killed runner"
grep -Fq 'steward restart --repo' "$tmp/runner-dead.out" || fail "dead verdict omitted the restart remedy"
if [[ -f "$tmp/alerts.log" ]] && grep -Fq 'HEALTH unhealthy' "$tmp/alerts.log"; then
  fail "a recoverable first failure notified the human before escalation"
fi
silent_episode=
for candidate in "$repo/artifacts/agents/steward/alerts"/*.json; do
  [[ -f "$candidate" ]] || continue
  grep -Fq '"cleared": false' "$candidate" || continue
  silent_episode=$candidate
done
[[ -n "$silent_episode" ]] || fail "the first failure opened no silent history episode"
grep -Fq '"attempts": []' "$silent_episode" || fail "the silent history episode attempted notification before escalation"

for observation in 2 3 4 5; do
  "$ms" steward tick --repo "$repo" >"$tmp/tick-$observation.out" 2>"$tmp/tick-$observation.err" || {
    cat "$tmp/tick-$observation.err" >&2
    fail "failure observation $observation tick failed"
  }
done
grep -Fq '"consecutiveFailures": 5' "$tmp/tick-5.out" || fail "failure five was not projected"
grep -Fq '"failureEscalation": "AUTO_HEAL_ENDED"' "$tmp/tick-5.out" || fail "failure five did not end auto-heal"

episode_count=0
episode_file=
for candidate in "$repo/artifacts/agents/steward/alerts"/*.json; do
  [[ -f "$candidate" ]] || continue
  grep -Fq '"cleared": false' "$candidate" || continue
  episode_count=$((episode_count + 1))
  episode_file=$candidate
done
[[ "$episode_count" -eq 1 ]] || fail "failure five must open one digest-keyed episode, found $episode_count"
grep -Fq '"transportResult": "TRANSPORT_SUBMITTED"' "$episode_file" || fail "notifier exit zero was not recorded as transport submitted"
episode_id=$("$ms" json get --file "$episode_file" --field episodeId) || fail "episode id unreadable"
[[ $(grep -c '^HEALTH unhealthy' "$tmp/alerts.log" 2>/dev/null || true) -eq 1 ]] || fail "one episode must submit one desktop notification"

"$ms" steward tick --repo "$repo" >"$tmp/tick-dedup.out" 2>"$tmp/tick-dedup.err" || {
  cat "$tmp/tick-dedup.err" >&2
  fail "same-digest dedup tick failed"
}
[[ $(grep -c '^HEALTH unhealthy' "$tmp/alerts.log" 2>/dev/null || true) -eq 1 ]] || fail "same digest submitted a second desktop notification"
active_episodes=0
for candidate in "$repo/artifacts/agents/steward/alerts"/*.json; do
  [[ -f "$candidate" ]] || continue
  grep -Fq '"cleared": false' "$candidate" || continue
  active_episodes=$((active_episodes + 1))
done
[[ "$active_episodes" -eq 1 ]] || fail "same digest opened a second active episode"

"$ms" health acknowledge-alert --repo "$repo" --episode "$episode_id" >"$tmp/ack.out" 2>"$tmp/ack.err" || {
  cat "$tmp/ack.err" >&2
  fail "episode acknowledgment failed"
}
grep -Fq '"acknowledged": true' "$episode_file" || fail "episode acknowledgment was not recorded"
grep -Fq '"acknowledgedBy"' "$episode_file" || fail "episode acknowledgment omitted the observed invoker"

"$ms" steward restart --repo "$repo" >"$tmp/runner-restart.out" 2>"$tmp/runner-restart.err" || {
  cat "$tmp/runner-restart.err" >&2
  fail "the killed runner's printed restart remedy failed"
}
runner_pid=$(read_runner_pid)
wait_for_healthy runner-recovered || fail "the killed runner's printed restart remedy did not heal health"
grep -Fq '"resolved": true' "$episode_file" || fail "healthy verdict did not resolve the alert episode"
grep -Fq '"cleared": true' "$episode_file" || fail "healthy verdict did not clear the alert episode"

"$ms" steward disarm --repo "$repo" >/dev/null 2>&1 || true
wait_for_pid_exit "disarmed runner exit" "$runner_pid" || fail "disarmed runner did not exit"
runner_pid=

echo "health fixtures: direct rc 0/1/2, ten roles, silent first-failure history, escalated episode dedup, acknowledgment, and healthy clear PASSED"
