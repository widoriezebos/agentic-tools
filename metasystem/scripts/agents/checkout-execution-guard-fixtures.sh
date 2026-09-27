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
evidence_started="$tmp/evidence-started"
touch "$evidence_started"
owned_pids=()
fixture_completed=0
wait_evidence_printed=0

cleanup() {
  local status=$? pid
  trap - EXIT
  set +e
  for pid in ${owned_pids[@]+"${owned_pids[@]}"}; do
    if kill -0 "$pid" 2>/dev/null; then
      kill -TERM "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  if (( status == 0 && ! fixture_completed )); then
    status=1
  fi
  if (( status != 0 )); then
    print_wait_evidence
  fi
  rm -rf "$tmp"
  exit "$status"
}
trap cleanup EXIT

print_wait_evidence() {
  local evidence suite_log suite_log_root
  (( wait_evidence_printed == 0 )) || return 0
  wait_evidence_printed=1
  for evidence in "$tmp"/*.out "$tmp"/*.err; do
    [[ -f "$evidence" ]] || continue
    echo "--- $evidence ---" >&2
    tail -n 40 "$evidence" >&2
  done
  suite_log_root="$root/artifacts/agents/supervision/suite-logs"
  [[ -d "$suite_log_root" ]] || return 0
  while IFS= read -r -d '' suite_log; do
    echo "--- $suite_log ---" >&2
    tail -n 40 "$suite_log" >&2
  done < <(find "$suite_log_root" -type f -name 'validate-*.log' \
    -newer "$evidence_started" -print0 2>/dev/null)
}

wait_with_evidence() { # pid, child name
  local pid=$1 name=$2 status
  set +e
  wait "$pid"
  status=$?
  set -e
  if (( status != 0 )); then
    echo "$name child $pid exited with status $status" >&2
    print_wait_evidence
  fi
  return "$status"
}

wait_for_file() { # path, fixture name, optional producer pid
  local path=$1 name=$2 producer_pid=${3:-} deadline=$((SECONDS + fixture_cap)) producer_status
  while [[ ! -e "$path" ]]; do
    if [[ -n "$producer_pid" ]] && ! kill -0 "$producer_pid" 2>/dev/null; then
      [[ -e "$path" ]] && return 0
      set +e
      wait "$producer_pid"
      producer_status=$?
      set -e
      (( producer_status != 0 )) || producer_status=1
      echo "$name producer $producer_pid exited with status $producer_status before writing $path" >&2
      print_wait_evidence
      return "$producer_status"
    fi
    (( SECONDS < deadline )) \
      || { echo "$name timed out after ${fixture_cap}s waiting for $path" >&2
           print_wait_evidence
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

# The validation-suite queueing legs (suite first, dispatch first, and a
# dispatch nested inside the suite's own guard) retired with
# validate-metasystem.sh (verbs-object-action U7b): the suite entrypoint that
# contended for the checkout guard no longer exists.

# Two contenders wait on one holder. The native guard test observes both
# blocked acquisitions and proves the second remains blocked by the first.
# This public flow retains stale cleanup and ordered release through the real
# command boundary without using a quiet interval as evidence.
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

for private_guard_root in "$race_root" "$forged_root"; do
  [[ ! -e "$private_guard_root/artifacts/agents/supervision/gate-runs/checkout-execution.lock.d" ]] \
    || { echo "checkout execution guard fixture left a private guard owned: $private_guard_root" >&2; exit 1; }
done
fixture_completed=1
echo "checkout execution guard fixtures passed"
