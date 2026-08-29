#!/usr/bin/env bash
set -euo pipefail

fixture_mode=${1:-}
if [[ "$fixture_mode" == __wait-only ]]; then
  ready=$2 release=$3 cap=$4 deadline=$((SECONDS + cap))
  touch "$ready"
  while [[ ! -e "$release" ]]; do
    (( SECONDS < deadline )) \
      || { echo "checkout execution guard detached member timed out after ${cap}s" >&2; exit 1; }
    sleep 0.05
  done
  exit 0
fi
if [[ "$fixture_mode" == __holder || "$fixture_mode" == __contender ]]; then
  engine=$2 fixture_root=$3 owner=$4 ready=$5 release=$6 cap=$7
  result_file=$(mktemp "${TMPDIR:-/tmp}/metasystem-checkout-result.XXXXXX")
  "$engine" gate guard-acquire --root "$fixture_root" --owner "$owner" \
    --wait-sec "$cap" --progress-sec 1 >"$result_file"
  IFS= read -r result <"$result_file"
  rm -f "$result_file"
  printf '%s\n' "$result"
  [[ "$result" == acquired ]]
  touch "$ready"
  deadline=$((SECONDS + cap))
  while [[ ! -e "$release" ]]; do
    (( SECONDS < deadline )) \
      || { echo "checkout execution guard fixture process timed out after ${cap}s" >&2; exit 1; }
    sleep 0.05
  done
  "$engine" gate guard-release --root "$fixture_root"
  exit 0
fi

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"
source scripts/agents/fixture-budget.sh
harness_fixture_budget_init "$root"
fixture_cap=$(harness_fixture_cap checkout-execution-guard)
engine=${METASYSTEM_BIN:-$root/bin/metasystem}
script="$root/scripts/agents/checkout-execution-guard-fixtures.sh"
tmp=$(mktemp -d)
owned_pids=()

cleanup() {
  local pid
  for pid in ${owned_pids[@]+"${owned_pids[@]}"}; do
    if kill -0 "$pid" 2>/dev/null; then
      kill -TERM "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  rm -rf "$tmp"
}
trap cleanup EXIT

wait_for_file() { # path, fixture name
  local path=$1 name=$2 deadline=$((SECONDS + fixture_cap)) err
  while [[ ! -e "$path" ]]; do
    (( SECONDS < deadline )) \
      || { echo "$name timed out after ${fixture_cap}s waiting for $path" >&2
           for err in "$tmp"/*.err; do
             [[ -s "$err" ]] || continue
             echo "--- $err ---" >&2
             tail -5 "$err" >&2
           done
           return 1; }
    sleep 0.05
  done
}

write_control() { # control ready release [child control] [child brief]
  local control=$1 ready=$2 release=$3 child_control=${4:-} child_brief=${5:-}
  if [[ -n "$child_control" ]]; then
    printf '{"attempted":"%s","ready":"%s","release":"%s","capSec":%s,"childControl":"%s","childBrief":"%s"}\n' \
      "${ready%.ready}.attempted" "$ready" "$release" "$fixture_cap" "$child_control" "$child_brief" >"$control"
  else
    printf '{"attempted":"%s","ready":"%s","release":"%s","capSec":%s}\n' \
      "${ready%.ready}.attempted" "$ready" "$release" "$fixture_cap" >"$control"
  fi
}

brief="$tmp/brief.md"
printf 'Working Mode: implement\n\n# Goal\n\nCheckout execution guard fixture.\n' >"$brief"

# Bootstrap precedent: an executable engine that predates guard-acquire may
# return usage status two. The entrypoint proceeds with one explicit note so
# its later Go gate can rebuild that engine.
stale_engine="$tmp/stale-engine"
printf '%s\n' '#!/usr/bin/env bash' \
  'if [[ ${1:-} == config && ${2:-} == get ]]; then' \
  '  for ((i=1; i<=$#; i++)); do if [[ ${!i} == --default ]]; then j=$((i+1)); printf "%s\n" "${!j}"; exit 0; fi; done' \
  'fi' \
  'echo "unknown verb guard-acquire" >&2' \
  'exit 2' >"$stale_engine"
chmod +x "$stale_engine"
mkdir -p "$tmp/bootstrap"
cp metasystem.conf "$tmp/bootstrap/metasystem.conf"
(
  root="$tmp/bootstrap"
  ms="$stale_engine"
  source "$PWD/scripts/agents/checkout-execution-guard.sh"
  checkout_execution_guard_acquire "bootstrap fixture"
  (( checkout_execution_guard_held == 0 ))
) 2>"$tmp/bootstrap.err"
grep -Fq 'existing engine does not know gate guard-acquire; proceeding until this run rebuilds it' "$tmp/bootstrap.err"

# The queueing legs drive scripts/agents/dispatch.sh as a real process, and
# dispatch refuses any root outside a git repository. Adoption supports such
# targets (the hook installs when git init happens), and no dispatch can run
# there — so there is no suite-vs-dispatch contention to certify. Skip those
# legs loudly; the holder races and token refusals still run everywhere.
borrowed_validation_progress_env=(
  env
  METASYSTEM_SUITE_PROGRESS_ACTIVE=1
  METASYSTEM_SUITE_PROGRESS_ROOT="$root"
  METASYSTEM_SUITE_PROGRESS_TMP="$tmp"
  METASYSTEM_SUITE_PROGRESS_TMP_OWNER=
)
if git -C "$root" rev-parse --show-toplevel >/dev/null 2>&1; then
# Suite first: launch the actual validation entrypoint, then the actual
# dispatch verb. Dispatch must stay queued until validation releases; both
# entrypoints then finish their explicit guard fixture with status zero.
suite_first_guard_root="$tmp/guard-state/suite-first"
suite_control="$tmp/suite-first-suite.json"
dispatch_control="$tmp/suite-first-dispatch.json"
write_control "$suite_control" "$tmp/suite-first-suite.ready" "$tmp/suite-first-suite.release"
write_control "$dispatch_control" "$tmp/suite-first-dispatch.ready" "$tmp/suite-first-dispatch.release"
"${borrowed_validation_progress_env[@]}" METASYSTEM_BIN="$engine" \
  METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT="$suite_first_guard_root" \
  METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE="$suite_control" \
  "$root/scripts/validate-metasystem.sh" >"$tmp/suite-first-suite.out" 2>"$tmp/suite-first-suite.err" &
suite_pid=$!; owned_pids+=("$suite_pid")
wait_for_file "$tmp/suite-first-suite.ready" "suite-first validation"
METASYSTEM_BIN="$engine" METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT="$suite_first_guard_root" \
  METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE="$dispatch_control" \
  "$root/scripts/agents/dispatch.sh" dispatch --role implementer --brief "$brief" \
  >"$tmp/suite-first-dispatch.out" 2>"$tmp/suite-first-dispatch.err" &
dispatch_pid=$!; owned_pids+=("$dispatch_pid")
wait_for_file "$tmp/suite-first-dispatch.attempted" "suite-first dispatch acquisition"
[[ ! -e "$tmp/suite-first-dispatch.ready" ]] \
  || { echo "suite-first dispatch entered before validation released" >&2; exit 1; }
touch "$tmp/suite-first-suite.release"
wait_for_file "$tmp/suite-first-dispatch.ready" "suite-first dispatch"
touch "$tmp/suite-first-dispatch.release"
wait "$suite_pid"
wait "$dispatch_pid"

# Dispatch first: the same production entrypoints in reverse order prove the
# validation suite queues behind an active dispatch.
dispatch_first_guard_root="$tmp/guard-state/dispatch-first"
dispatch_control="$tmp/dispatch-first-dispatch.json"
suite_control="$tmp/dispatch-first-suite.json"
write_control "$dispatch_control" "$tmp/dispatch-first-dispatch.ready" "$tmp/dispatch-first-dispatch.release"
write_control "$suite_control" "$tmp/dispatch-first-suite.ready" "$tmp/dispatch-first-suite.release"
METASYSTEM_BIN="$engine" METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT="$dispatch_first_guard_root" \
  METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE="$dispatch_control" \
  "$root/scripts/agents/dispatch.sh" dispatch --role implementer --brief "$brief" \
  >"$tmp/dispatch-first-dispatch.out" 2>"$tmp/dispatch-first-dispatch.err" &
dispatch_pid=$!; owned_pids+=("$dispatch_pid")
wait_for_file "$tmp/dispatch-first-dispatch.ready" "dispatch-first dispatch"
"${borrowed_validation_progress_env[@]}" METASYSTEM_BIN="$engine" \
  METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT="$dispatch_first_guard_root" \
  METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE="$suite_control" \
  "$root/scripts/validate-metasystem.sh" >"$tmp/dispatch-first-suite.out" 2>"$tmp/dispatch-first-suite.err" &
suite_pid=$!; owned_pids+=("$suite_pid")
wait_for_file "$tmp/dispatch-first-suite.attempted" "dispatch-first validation acquisition"
[[ ! -e "$tmp/dispatch-first-suite.ready" ]] \
  || { echo "dispatch-first validation entered before dispatch released" >&2; exit 1; }
touch "$tmp/dispatch-first-dispatch.release"
wait_for_file "$tmp/dispatch-first-suite.ready" "dispatch-first validation"
touch "$tmp/dispatch-first-suite.release"
wait "$dispatch_pid"
wait "$suite_pid"

# A validation fixture spawns the actual dispatch verb from inside its own
# process chain against a guard the fixture itself holds. Exact ancestry lets
# the nested dispatch register immediately, explicitly preserving the join
# exemption while the foreign queueing legs use separate private roots.
nested_guard_root="$tmp/guard-state/nested-own-ancestry"
nested_dispatch_control="$tmp/nested-dispatch.json"
nested_suite_control="$tmp/nested-suite.json"
printf '{"attempted":"%s","ready":"%s","release":"%s","capSec":%s,"detachReady":"%s","detachRelease":"%s"}\n' \
  "$tmp/nested-dispatch.attempted" "$tmp/nested-dispatch.ready" "$tmp/nested-dispatch.release" "$fixture_cap" \
  "$tmp/nested-detached.ready" "$tmp/nested-detached.release" >"$nested_dispatch_control"
write_control "$nested_suite_control" "$tmp/nested-suite.ready" "$tmp/nested-suite.release" \
  "$nested_dispatch_control" "$brief"
"${borrowed_validation_progress_env[@]}" METASYSTEM_BIN="$engine" \
  METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT="$nested_guard_root" \
  METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE="$nested_suite_control" \
  "$root/scripts/validate-metasystem.sh" >"$tmp/nested-suite.out" 2>"$tmp/nested-suite.err" &
suite_pid=$!; owned_pids+=("$suite_pid")
wait_for_file "$tmp/nested-suite.ready" "nested validation"
wait_for_file "$tmp/nested-dispatch.ready" "nested dispatch ancestry join"
wait_for_file "$tmp/nested-detached.ready" "nested detached dispatch member"
touch "$tmp/nested-suite.release"
wait "$suite_pid"

# The suite and dispatch entry processes are now gone, but the registered
# detached wrapper still owns execution. A foreign contender remains queued
# until that member exits and runs its release path.
"$script" __contender "$engine" "$nested_guard_root" "post-suite contender" \
  "$tmp/nested-contender.ready" "$tmp/nested-contender.release" "$fixture_cap" \
  >"$tmp/nested-contender.out" 2>"$tmp/nested-contender.err" &
nested_contender_pid=$!; owned_pids+=("$nested_contender_pid")
sleep 0.2
[[ ! -e "$tmp/nested-contender.ready" ]] \
  || { echo "nested detached dispatch lost the guard when its suite exited" >&2; exit 1; }
touch "$tmp/nested-detached.release"
wait_for_file "$tmp/nested-contender.ready" "post-suite contender"
touch "$tmp/nested-contender.release"
wait "$nested_contender_pid"

fi
if ! git -C "$root" rev-parse --show-toplevel >/dev/null 2>&1; then
  echo "checkout execution guard fixtures: root is not a git repository; dispatch-entrypoint legs skipped"
fi

# Two contenders wait on one holder. Killing it permits exactly one fenced
# stale cleanup and one first entrant; the other remains queued until release.
race_root="$tmp/dead-holder-race"
"$script" __holder "$engine" "$race_root" "dead suite holder" \
  "$tmp/race-holder.ready" "$tmp/race-holder.release" "$fixture_cap" \
  >"$tmp/race-holder.out" 2>"$tmp/race-holder.err" &
holder_pid=$!; owned_pids+=("$holder_pid")
wait_for_file "$tmp/race-holder.ready" "dead-holder race holder" \
  || { cat "$tmp/race-holder.err" >&2; wait "$holder_pid" 2>/dev/null || true; exit 1; }
for contender in one two; do
  "$script" __contender "$engine" "$race_root" "race contender $contender" \
    "$tmp/race-$contender.ready" "$tmp/race-$contender.release" "$fixture_cap" \
    >"$tmp/race-$contender.out" 2>"$tmp/race-$contender.err" &
  contender_pid=$!; owned_pids+=("$contender_pid")
  if [[ "$contender" == one ]]; then contender_one_pid=$contender_pid; else contender_two_pid=$contender_pid; fi
done
kill -KILL "$holder_pid"
wait "$holder_pid" 2>/dev/null || true
deadline=$((SECONDS + fixture_cap))
while [[ ! -e "$tmp/race-one.ready" && ! -e "$tmp/race-two.ready" ]]; do
  (( SECONDS < deadline )) || { echo "dead-holder race had no winner" >&2; exit 1; }
  sleep 0.05
done
if [[ -e "$tmp/race-one.ready" ]]; then first=one; second=two; else first=two; second=one; fi
sleep 0.2
[[ ! -e "$tmp/race-$second.ready" ]] \
  || { echo "dead-holder race admitted two contenders" >&2; exit 1; }
touch "$tmp/race-$first.release"
wait_for_file "$tmp/race-$second.ready" "dead-holder race second contender"
touch "$tmp/race-$second.release"
wait "$contender_one_pid"
wait "$contender_two_pid"
[[ $(grep -hF 'removed stale holder dead suite holder' "$tmp/race-one.err" "$tmp/race-two.err" | wc -l | tr -d ' ') == 1 ]] \
  || { echo "dead-holder race did not emit exactly one stale cleanup note" >&2; exit 1; }

# An inherited bearer-shaped string has no authority. A process outside the
# holder's ancestry reaches its bound and loudly names the live holder.
forged_root="$tmp/forged-token"
"$script" __holder "$engine" "$forged_root" "forged-token holder" \
  "$tmp/forged-holder.ready" "$tmp/forged-holder.release" "$fixture_cap" \
  >"$tmp/forged-holder.out" 2>"$tmp/forged-holder.err" &
holder_pid=$!; owned_pids+=("$holder_pid")
wait_for_file "$tmp/forged-holder.ready" "forged-token holder"
set +e
METASYSTEM_CHECKOUT_EXECUTION_TOKEN=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  "$engine" gate guard-acquire --root "$forged_root" --owner "forged contender" \
  --wait-sec 1 --progress-sec 1 >"$tmp/forged.out" 2>"$tmp/forged.err"
forged_rc=$?
set -e
[[ "$forged_rc" == 1 ]]
grep -Fq 'expired after 1s waiting for forged-token holder' "$tmp/forged.err"
touch "$tmp/forged-holder.release"
wait "$holder_pid"

for private_guard_root in "$suite_first_guard_root" "$dispatch_first_guard_root" \
    "$nested_guard_root" "$race_root" "$forged_root"; do
  [[ ! -e "$private_guard_root/artifacts/agents/supervision/gate-runs/checkout-execution.lock.d" ]] \
    || { echo "checkout execution guard fixture left a private guard owned: $private_guard_root" >&2; exit 1; }
done
echo "checkout execution guard fixtures passed"
