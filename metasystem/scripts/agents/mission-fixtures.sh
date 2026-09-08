#!/usr/bin/env bash
set -euo pipefail

# The wall preflight demands a CLEAN initial baseline at start:
# close everything the bed laid down in tracked space, exactly as a real
# mission repository begins.
close_bed_baseline() { # repo
  git -C "$1" add -A . >/dev/null 2>&1 || true
  if ! git -C "$1" diff --cached --quiet 2>/dev/null; then
    git -C "$1" -c user.name=fixture -c user.email=fixture@example.invalid \
      commit -qm 'bed baseline' >/dev/null
  fi
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$root"
mission_job_cap_min=$(harness_fixture_semantic_cap mission-job-minutes)
mission_turn_cap_min=$(harness_fixture_semantic_cap mission-turn-minutes)
minimum_cap_min=$(harness_fixture_semantic_cap minimum-minutes)
fixture_root=$(mktemp -d)
repo=$fixture_root/repo
remote=$fixture_root/origin.git
watcher_pid=
reaper_pid=
owned_process_file=$fixture_root/owned-processes

wait_for_fixture_pid() { # name, pid, named cap
  local name=$1 pid=$2 maximum started deadline elapsed
  maximum=$(harness_fixture_cap "$3")
  started=$SECONDS
  deadline=$((SECONDS + maximum))
  while kill -0 "$pid" 2>/dev/null; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "mission fixture wait ceiling reached: $name pid=$pid (elapsed: ${elapsed}s; scaled cap: ${maximum}s)" >&2
      kill -KILL "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  wait "$pid" 2>/dev/null || true
}

cleanup() {
  local pid started label command maximum deadline
  for pid in "$watcher_pid" "$reaper_pid"; do
    [[ -n "$pid" ]] || continue
    kill -TERM "$pid" 2>/dev/null || true
    wait_for_fixture_pid supervisor-cleanup "$pid" mission-process-wait || true
  done
  if [[ -f "$owned_process_file" && -x "$repo/bin/metasystem" ]]; then
    while read -r pid started label; do
      [[ "$pid" =~ ^[0-9]+$ && "$started" =~ ^[0-9]+$ ]] || continue
      "$repo/bin/metasystem" proc alive --pid "$pid" --start-time "$started" --root "$repo" >/dev/null 2>&1 || continue
      command=$(ps -p "$pid" -o command= 2>/dev/null || true)
      if [[ "$command" != *"$repo"* ]]; then
        echo "mission fixture cleanup refused unproven $label process $pid: $command" >&2
        continue
      fi
      kill -TERM "$pid" 2>/dev/null || true
      maximum=$(harness_fixture_cap mission-process-wait)
      deadline=$((SECONDS + maximum))
      while (( SECONDS < deadline )) \
        && "$repo/bin/metasystem" proc alive --pid "$pid" --start-time "$started" --root "$repo" >/dev/null 2>&1; do
        sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
      done
      if "$repo/bin/metasystem" proc alive --pid "$pid" --start-time "$started" --root "$repo" >/dev/null 2>&1; then
        command=$(ps -p "$pid" -o command= 2>/dev/null || true)
        if [[ "$command" == *"$repo"* ]]; then
          kill -KILL "$pid" 2>/dev/null || true
        else
          echo "mission fixture cleanup refused unproven $label process $pid after TERM: $command" >&2
        fi
      fi
    done <"$owned_process_file"
  fi
  rm -rf "$fixture_root"
}
trap cleanup EXIT

expect_failure() { # name, expected text, command...
  local name=$1 expected=$2 status
  shift 2
  set +e
  "$@" >"$fixture_root/$name.out" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "mission fixture unexpectedly passed: $name" >&2
    exit 1
  fi
  if [[ -n "$expected" ]] && ! grep -Fq "$expected" "$fixture_root/$name.out"; then
    echo "mission fixture $name did not report: $expected" >&2
    sed -n '1,240p' "$fixture_root/$name.out" >&2
    exit 1
  fi
}

mkdir -p "$repo/scripts/agents" "$repo/scripts" "$repo/truth" "$repo/plans" "$repo/docs" "$repo/bin"
# The fixture repo receives the engine at <repo>/bin/metasystem.
cp "$root/bin/metasystem" "$repo/bin/metasystem"
git init -q -b main "$repo"
git init -q -b main --bare "$remote"
git -C "$repo" config user.name metasystem
git -C "$repo" config user.email metasystem@example.invalid

cat >"$repo/scripts/gate.sh" <<'GATE'
#!/usr/bin/env bash
set -euo pipefail
[[ ! -e candidate-bad ]] || exit 4
printf 'metric=score=1\n'
GATE
chmod +x "$repo/scripts/gate.sh"
printf 'certified truth\n' >"$repo/truth/reference.txt"
cp "$root/docs/project-rules.md" "$repo/docs/project-rules.md"
cat >"$repo/scripts/agents/arm-supervision.sh" <<'ARM'
#!/usr/bin/env bash
set -euo pipefail
[[ ${1:-} == fingerprint && ${2:-} == --repo && $# -eq 3 ]] || exit 2
printf 'fixture-fingerprint\n'
ARM
chmod +x "$repo/scripts/agents/arm-supervision.sh"
# The fixture mirrors the deployment's projection boundary (HIW-O3): the
# wall's shippable snapshot must exclude runtime state exactly as the real
# repository's .gitignore does.
printf 'artifacts/\nbin/\nmetasystem.conf\n' >"$repo/.gitignore"
git -C "$repo" add .gitignore scripts truth docs
git -C "$repo" commit -qm instruments
git -C "$repo" tag instruments
git -C "$repo" remote add origin "$remote"
git -C "$repo" push -qu origin main
git -C "$remote" symbolic-ref HEAD refs/heads/main
git -C "$repo" remote set-head origin -a >/dev/null

source "$root/scripts/agents/fixture-bed-scenarios.sh"
fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario mission "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO
if (( ! fixture_bed_child )); then
  fixture_bed_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios mission \
    "mission contract, state, and runner end-state fixtures passed" \
    "$fixture_bed_script" contract-and-state runner-end-state mission-stop
fi

# Fabricate only the supervisor facts preflight reads; both scenarios
# need their own live holders and facts, so this is a function.
fabricate_supervisor_facts() {
  # Fabricate only the supervisor facts preflight reads. Each process is a real
  # live process whose argv carries the recorded tag; cleanup waits are named
  # and ceiling-bounded above.
  watcher_pid=$("$root/bin/metasystem" supervise launch-detached --cwd "$repo" \
    "$root/bin/metasystem" util hold --tag mission-watcher-tag)
  reaper_pid=$("$root/bin/metasystem" supervise launch-detached --cwd "$repo" \
    "$root/bin/metasystem" util hold --tag mission-reaper-tag)
  # The engine ships in this fixture repo, so preflight demands EXACT-START
  # liveness: record the holders' real start times, not synthetic ones.
  watcher_start=$("$root/bin/metasystem" proc started-at --pid "$watcher_pid")
  reaper_start=$("$root/bin/metasystem" proc started-at --pid "$reaper_pid")
  identity_file=$fixture_root/mission-process-identities.json
  printf '{"%s":{"pidStartedAt":%s,"command":"fixture mission-watcher-tag"},"%s":{"pidStartedAt":%s,"command":"fixture mission-reaper-tag"},"%s":{"terminal":true}}\n' \
    "$watcher_pid" "$watcher_start" "$reaper_pid" "$reaper_start" "$$" >"$identity_file"
  export METASYSTEM_MISSION_PROCESS_IDENTITY_FILE=$identity_file
  supervision=$repo/artifacts/agents/supervision
  mkdir -p "$supervision"
  supervision_now=$(date +%s)
  watcher_hb=$supervision/watcher.heartbeat.json
  reaper_hb=$supervision/reaper.heartbeat.json
  printf '{"function":"watcher","pid":%s,"pidStartedAt":%s,"observedAtEpoch":%s}\n' \
    "$watcher_pid" "$watcher_start" "$supervision_now" >"$watcher_hb"
  printf '{"function":"reaper","pid":%s,"pidStartedAt":%s,"observedAtEpoch":%s}\n' \
    "$reaper_pid" "$reaper_start" "$supervision_now" >"$reaper_hb"
  printf '{"intervalSec":60,"fingerprint":"fixture-fingerprint","components":{"watcher":{"pid":%s,"pidStartedAt":%s,"instanceTag":"mission-watcher-tag","heartbeat":"%s"},"reaper":{"pid":%s,"pidStartedAt":%s,"instanceTag":"mission-reaper-tag","heartbeat":"%s"}}}\n' \
    "$watcher_pid" "$watcher_start" "$watcher_hb" \
    "$reaper_pid" "$reaper_start" "$reaper_hb" >"$supervision/state.json"
  printf '{"verdict":"SUCCESS","completedAtEpoch":%s,"fingerprint":"fixture-fingerprint"}\n' \
    "$supervision_now" >"$supervision/last-census.json"
}


if [[ "$fixture_scenario" == contract-and-state ]]; then

base=$repo/base.contract.md
cases=$repo/cases
mkdir -p "$cases"
cat >"$base" <<'CONTRACT'
# Intent

Reach the declared score with frozen instruments.

# Non-goals

Do not publish or deploy.

# Initial streams

Keep one stream active when the other needs a reserved decision.

```mission
gate.command=scripts/gate.sh
gate.ref=instruments
gate.paths=scripts/gate.sh
truth.paths=truth/*.txt
truth.certification=certified
gate.direction=max
gate.threshold.score=>=1
gate.noise-floor.score=0
guard.audit.command=scripts/gate.sh
guard.audit.floor=1
guard.audit.noise=0
guard.cadence=1
ledger.cycle-budget=3
ledger.no-gain-budget=2
ledger.accept-binary-gate-fuse=true
fence.wall-clock-hours=2
fence.cycles=3
fence.jobs=4
fence.concurrency=2
fence.job-cap-min=FIXTURE_JOB_CAP_MIN
host.runtime=fake
host.model=fake-model
host.turn-cap-min=FIXTURE_TURN_CAP_MIN
stream.primary=Reach the acceptance score.
envelope.dependencies=jq
exposure=EUR:10
```
CONTRACT
conf_edit "$base" replace-literal FIXTURE_JOB_CAP_MIN "$mission_job_cap_min"
conf_edit "$base" replace-literal FIXTURE_TURN_CAP_MIN "$mission_turn_cap_min"

"$root/bin/metasystem" mission contract-validate --file "$base" >/dev/null

# Every concrete key in the authored grammar is independently required and
# independently type-checked. These fixture commands are synchronous; the
# gate path itself carries fence.job-cap-min as its named execution ceiling.
# The per-key grammar matrix (52 subprocess legs) and the envelope
# rejection variants moved in-process: TestContractValidateRejects and
# TestContractValidateRejectsPerKeyMatrix (internal/contract, under the
# go gate) carry the exact missing/malformed table this loop drove
# through the contract validator (script-fixtures-003/D38). The seal-sign-
# preflight smokes below stay: they prove the engine verbs.

dispatch_allow=$repo/plans/mission-dispatch-allow.contract.md
sed 's/envelope.dependencies=jq/envelope.dispatch-allow=fake:fake-model,codex:gpt-5.6-sol/' "$base" >"$dispatch_allow"
"$root/bin/metasystem" mission contract-seal --file "$dispatch_allow" >/dev/null
# The dispatch-allow value survives sealing byte-exactly and the sealed
# contract still validates. (The parser-internal assertions the python
# module leg made — the values map and pair splitting — are owned by
# internal/mission's contract unit tests under the go gate.)
"$root/bin/metasystem" mission contract-validate --file "$dispatch_allow" >/dev/null
grep -Fq 'envelope.dispatch-allow=fake:fake-model,codex:gpt-5.6-sol' "$dispatch_allow" \
  || { echo "sealing altered the dispatch-allow envelope line" >&2; exit 1; }

# The unsealed/unsigned/mismatched-hash/stale-exposure preflight
# rejections are TestContractPreflightUnsealed/Unsigned/
# ApprovalHashMismatch/StaleExposure in internal/contract — verified 1:1
# by the review's verifier before this retirement (D38).

contract=$repo/plans/mission-alpha.contract.md
cp "$base" "$contract"
conf_edit "$contract" insert-after '^stream[.]primary=' \
  'stream.secondary=Preserve the evidence contract.'
contract_sha=$("$root/bin/metasystem" mission contract-seal --file "$contract")
printf '\nApproval: name=Human; date=2026-08-04; contract-sha256=%s\n' "$contract_sha" >>"$contract"
git -C "$repo" add plans/mission-alpha.contract.md
git -C "$repo" commit -qm 'sign mission contract'
git -C "$repo" push -qu origin main

fabricate_supervisor_facts

"$root/bin/metasystem" mission contract-preflight --file "$contract" >/dev/null
mv "$supervision/state.json" "$supervision/state.unarmed"
expect_failure unarmed-preflight "supervisor set is unarmed" "$root/bin/metasystem" mission contract-preflight --file "$contract"
mv "$supervision/state.unarmed" "$supervision/state.json"
mkdir -p "$repo/artifacts/agents/missions/alpha/lease.d"
expect_failure lease-preflight "lease is not acquirable" "$root/bin/metasystem" mission contract-preflight --file "$contract"
rmdir "$repo/artifacts/agents/missions/alpha/lease.d"

printf 'bad candidate\n' >"$repo/candidate-bad"
git -C "$repo" add candidate-bad
git -C "$repo" commit -qm 'make candidate unmeasurable'
git -C "$repo" push -qu origin main
expect_failure unrunnable-gate "gate measurement failed" "$root/bin/metasystem" mission contract-preflight --file "$contract"

# The ledger grammar, state chain, fork detection, reconcile parks, and
# anchor round-trips are internal/mission's unit tests under the go gate
# (ledger_test, state_test — TestChainDetectsTamper,
# TestWriteRefusesIllegalTransition — and anchor_test's five
# TestReconcile* rows including divergence and stop-loss parks); the
# review's verifier and this retirement confirmed the equivalents before
# the shell copies were cut (script-fixtures-004/D39). CLI arg-forwarding
# stays proven by the runner legs below.

current_sha=$(git -C "$repo" rev-parse HEAD)

# The reaper uses this same locked refusal operation when a mission-stamped
# job reaches its cap; prove the resulting ask as a world-state fact.
mkdir -p "$repo/plans" "$repo/artifacts/agents/missions/timeout-ask"
cp "$base" "$repo/plans/mission-timeout-ask.contract.md"
"$root/bin/metasystem" mission fence-refuse --repo "$repo" --mission timeout-ask --reason job-cap-min >/dev/null
timeout_ask=$repo/artifacts/agents/missions/timeout-ask/asks/fence-bound.json
[[ -f "$timeout_ask" ]] || { echo "mission timeout refusal did not write its ask" >&2; exit 1; }
grep -Fq '`job-cap-min`' "$timeout_ask" || { echo "mission timeout ask omitted its reached fence" >&2; exit 1; }

# Two simultaneous reservations cannot both cross a concurrency fence. The
# exact subprocess waits are named and ceiling-bounded (IL-1).
race_contract=$repo/plans/mission-race.contract.md
sed 's/fence.concurrency=2/fence.concurrency=1/' "$base" >"$race_contract"
# This fixture enters below the runner lifecycle to isolate the fence lock, so
# seed the runner-owned contract pin explicitly. The delegate-caps fixtures
# separately prove that production start and resume pin only preflight-verified
# bytes; here the exact raw bytes must merely stay fixed across both contenders.
race_fences=$repo/artifacts/agents/missions/race/fences.json
mkdir -p "$(dirname "$race_fences")"
printf '{"schemaVersion":1,"missionId":"race","startedAt":"%s","cycles":0,"reservations":{},"approvedContractSha256":"%s"}\n' \
  "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  "$("$root/bin/metasystem" util sha256 --file "$race_contract")" >"$race_fences"
(
  set +e
  "$root/bin/metasystem" mission fence-reserve-job --repo "$repo" --mission race --job race-a --cap-min "$minimum_cap_min" >"$fixture_root/race-a.out" 2>&1
  printf '%s\n' "$?" >"$fixture_root/race-a.status"
) &
race_a_pid=$!
(
  set +e
  "$root/bin/metasystem" mission fence-reserve-job --repo "$repo" --mission race --job race-b --cap-min "$minimum_cap_min" >"$fixture_root/race-b.out" 2>&1
  printf '%s\n' "$?" >"$fixture_root/race-b.status"
) &
race_b_pid=$!
wait_for_fixture_pid concurrency-reservation-a "$race_a_pid" mission-process-wait
wait_for_fixture_pid concurrency-reservation-b "$race_b_pid" mission-process-wait
race_total=$(( $(cat "$fixture_root/race-a.status") + $(cat "$fixture_root/race-b.status") ))
[[ $race_total -eq 1 ]] || { echo "mission concurrency lock admitted zero or two reservations" >&2; exit 1; }
race_ask=$repo/artifacts/agents/missions/race/asks/fence-bound.json
[[ -f "$race_ask" ]] && grep -Fq '`concurrency`' "$race_ask" \
  || { echo "concurrent mission refusal did not write its batched ask" >&2; exit 1; }

fi

if [[ "$fixture_scenario" != runner-end-state && "$fixture_scenario" != mission-stop ]]; then exit 0; fi

# The end-state fixtures run the real runner with the fake host and synthetic
# process identities. They stay independent of the process-owning validator
# fixtures so restricted worktrees can exercise these two terminal outcomes.
cp -R "$root/scripts/agents/." "$repo/scripts/agents/"
cp "$root/scripts/metasystem-config.sh" \
  "$root/scripts/assert-return-complete.sh" "$repo/scripts/"
cp "$root/metasystem.conf" "$repo/metasystem.conf"
conf_edit "$repo/metasystem.conf" replace-line-first '^evidence[.]root=.*$' \
  "evidence.root=$fixture_root/runner-evidence"
conf_edit "$repo/metasystem.conf" replace-line-first '^metasystem[.]runtimes=.*$' \
  'metasystem.runtimes=fake'
printf 'role.default.model.fake=fake-model\n' >>"$repo/metasystem.conf"
cat >"$repo/scripts/agents/arm-supervision.sh" <<'ARM'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == fingerprint && ${2:-} == --repo && $# -eq 3 ]]; then
  printf 'fixture-fingerprint\n'
else
  printf 'up outcome=armed authority=writer\n'
fi
ARM
chmod +x "$repo/scripts/agents/arm-supervision.sh"
# candidate-bad is the contract scenario's leftover; under scenario
# isolation this child may never have created it.
git -C "$repo" rm -q --ignore-unmatch candidate-bad
# -f: the fixture historically tracks its conf for origin durability; the
# projection boundary above ignores it exactly as production does.
git -C "$repo" add -f scripts metasystem.conf
git -C "$repo" commit -qm 'install mission runner fixtures'
git -C "$repo" push -qu origin main

fabricate_supervisor_facts
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE=$identity_file
# A process fixture that drives human-reserved control-plane verbs cannot
# borrow the ambient session's class. Keep only the configured fake runtime's
# signature before mission-stop reaches its staged terminal fact, matching the
# supervision bed's agent-free fixture ancestry.
if [[ "$fixture_scenario" == mission-stop ]]; then
  for adapter in "$repo"/scripts/agents/adapters/*.sh; do
    case "${adapter##*/}" in fake.sh | runtime-common.sh) ;;
      *) rm -f "$adapter" ;;
    esac
  done
fi
# Refresh the supervision facts to now; json set stages beside each file
# and renames, so no reader can observe a torn record.
supervision_now=$(date +%s)
"$root/bin/metasystem" json set --file "$supervision/watcher.heartbeat.json" --int "observedAtEpoch=$supervision_now"
"$root/bin/metasystem" json set --file "$supervision/reaper.heartbeat.json" --int "observedAtEpoch=$supervision_now"
"$root/bin/metasystem" json set --file "$supervision/last-census.json" --int "completedAtEpoch=$supervision_now"

# A mission runner reaps and closes job chains, and both are control-plane
# writes reserved for the checkout's holder. This shell is that main: announce
# it, which also claims the checkout, so the runner it starts authenticates
# through an announced ancestor instead of classifying as a delegate of
# whichever agent happens to be running the suite.
"$root/bin/metasystem" lease announce --root "$repo" \
  --session mission-fixtures --pid $$ \
  --start "$("$root/bin/metasystem" proc started-at --pid $$)" \
  --tag fixture-mission-main --runtime fake >/dev/null

make_end_state_contract() { # mission, fake-host behavior
  local mission=$1 behavior=$2 path="$repo/plans/mission-$1.contract.md" contract_sha
  cat >"$path" <<EOF
# Intent

Reach the gate in one fake-host turn.

# Non-goals

Do not publish or deploy.

# Initial streams

Close the primary stream when the work succeeds.

\`\`\`mission
gate.command=scripts/gate.sh
gate.ref=instruments
gate.paths=scripts/gate.sh
truth.paths=truth/reference.txt
truth.certification=certified
gate.direction=max
gate.threshold.score=>=1
gate.noise-floor.score=0
guard.score.command=scripts/gate.sh
guard.score.floor=1
guard.score.noise=0
guard.cadence=1
ledger.cycle-budget=3
ledger.no-gain-budget=2
ledger.accept-binary-gate-fuse=true
fence.wall-clock-hours=2
fence.cycles=3
fence.jobs=4
fence.concurrency=1
fence.job-cap-min=$mission_job_cap_min
host.runtime=fake
host.model=fake-model
host.turn-cap-min=$minimum_cap_min
stream.primary=FAKEHOST:$behavior complete the primary stream.
envelope.dependencies=jq
exposure=EUR:1
\`\`\`
EOF
  contract_sha=$("$repo/bin/metasystem" mission contract-seal --file "$path")
  printf '\nApproval: name=Fixture-Human; date=2026-08-06; contract-sha256=%s\n' "$contract_sha" >>"$path"
  git -C "$repo" add "plans/mission-$mission.contract.md"
  git -C "$repo" commit -qm "sign mission $mission"
  git -C "$repo" push -qu origin main
}

wait_end_state() { # mission, expected status exit
  local mission=$1 expected=$2 result=7 maximum deadline
  maximum=$(harness_fixture_cap mission-end-state)
  deadline=$((SECONDS + maximum))
  while (( SECONDS < deadline )); do
    set +e
    "$repo/bin/metasystem" mission status --root "$repo" --mission "$mission" >/dev/null 2>&1
    result=$?
    set -e
    [[ $result -eq $expected ]] && return 0
    if [[ -f "$repo/artifacts/agents/missions/runners/$mission.json" ]] \
      && [[ "$("$repo/bin/metasystem" json get --file "$repo/artifacts/agents/missions/runners/$mission.json" --field status --default '')" == failed ]]; then
      echo "mission end-state fixture runner failed: $mission" >&2
      sed -n '1,240p' "$repo/artifacts/agents/missions/runners/$mission.json" >&2
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  echo "mission end-state fixture timed out: $mission -> $expected (last exit: $result; scaled cap: ${maximum}s)" >&2
  sed -n '1,240p' "$repo/artifacts/agents/missions/runners/$mission.json" >&2 2>/dev/null || true
  sed -n '1,240p' "$repo/artifacts/agents/missions/$mission/state.json" >&2 2>/dev/null || true
  return 1
}

wait_for_mission_path() { # name, path
  local name=$1 path=$2 maximum deadline
  maximum=$(harness_fixture_cap mission-process-wait)
  deadline=$((SECONDS + maximum))
  while (( SECONDS < deadline )); do
    [[ -e "$path" ]] && return 0
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  echo "mission fixture wait ceiling reached: $name ($path; scaled cap: ${maximum}s)" >&2
  return 1
}

if [[ "$fixture_scenario" == mission-stop ]]; then
  resolved_repo=$(cd "$repo" && pwd -P)
  # Both fake-host hold controls are fixture-only. Prove the rejection before
  # entering the fake checkout, where the scenario is authorized to use them.
  outside_turn=$fixture_root/outside-turn
  mkdir -p "$outside_turn"
  printf '{"missionId":"outside","turnId":"outside-t1","cycle":1}\n' >"$outside_turn/turn.json"
  printf 'outside fixture control\n' >"$outside_turn/prompt.md"
  for control in METASYSTEM_FAKE_HOST_HOLD METASYSTEM_FAKE_HOST_IGNORE_TERM; do
    set +e
    env "$control=1" "$root/scripts/agents/hosts/fake.sh" start-turn \
      --mission outside --turn-id outside-t1 --prompt "$outside_turn/prompt.md" \
      --result "$outside_turn/result.json" --instance-tag outside-fixture-control \
      >"$outside_turn/$control.out" 2>&1
    outside_rc=$?
    set -e
    (( outside_rc != 0 )) && grep -Fq 'available only in a fixture-mode root' "$outside_turn/$control.out" \
      || { echo "$control was not refused outside fixture mode" >&2; cat "$outside_turn/$control.out" >&2; exit 1; }
  done

  launch_held_mission() { # mission, ignore TERM (0|1), start|resume
    local mission=$1 ignore_term=$2 mode=$3 output turn runner_record turn_record
    local runner_pid runner_started host_pid host_started status
    output=$fixture_root/$mission-$mode.out
    set +e
    if [[ "$ignore_term" == 1 ]]; then
      METASYSTEM_AGENT_RUNTIME=fake METASYSTEM_FAKE_HOST_HOLD=1 METASYSTEM_FAKE_HOST_IGNORE_TERM=1 \
        "$repo/bin/metasystem" mission "$mode" --root "$repo" --mission "$mission" >"$output"
    else
      METASYSTEM_AGENT_RUNTIME=fake METASYSTEM_FAKE_HOST_HOLD=1 \
        "$repo/bin/metasystem" mission "$mode" --root "$repo" --mission "$mission" >"$output"
    fi
    status=$?
    set -e
    if [[ $status -ne 0 ]]; then
      echo "held mission $mission $mode failed with exit $status" >&2
      cat "$output" >&2
      return 1
    fi
    turn=$(sed -n "s/^mission=$mission started=yes turn=//p" "$output" | tail -n 1)
    [[ -n "$turn" ]] || { echo "held mission did not report its started turn" >&2; cat "$output" >&2; return 1; }
    wait_for_mission_path "$mission host readiness" \
      "$repo/artifacts/agents/missions/$mission/turns/$turn/host-ready"
    runner_record=$repo/artifacts/agents/missions/runners/$mission.json
    turn_record=$repo/artifacts/agents/missions/$mission/turns/$turn/turn.json
    runner_pid=$("$repo/bin/metasystem" json get --file "$runner_record" --field pid)
    runner_started=$("$repo/bin/metasystem" json get --file "$runner_record" --field pidStartedAt)
    host_pid=$("$repo/bin/metasystem" json get --file "$turn_record" --field pid)
    host_started=$("$repo/bin/metasystem" json get --file "$turn_record" --field pidStartedAt)
    printf '%s %s %s\n%s %s %s\n' \
      "$runner_pid" "$runner_started" "$mission-runner" \
      "$host_pid" "$host_started" "$mission-host" >>"$owned_process_file"
    printf '%s\n' "$turn"
  }

  run_complete_stop() { # case name, output file
    local name=$1 output=$2 status closing
    set +e
    "$repo/bin/metasystem" stop --repo "$repo" >"$output" 2>&1
    status=$?
    set -e
    closing="stopped $resolved_repo; start again: metasystem arm --repo $resolved_repo"
    if [[ $status -ne 0 ]] || grep -q '^NOT STOPPED ' "$output" \
      || [[ "$(tail -n 1 "$output")" != "$closing" ]]; then
      echo "$name did not stop every component with the complete-stop grammar (exit $status)" >&2
      cat "$output" >&2
      return 1
    fi
  }

  make_end_state_contract cooperating-host no-return
  close_bed_baseline "$repo"
  cooperating_turn=$(launch_held_mission cooperating-host 0 start)
  cooperating_state=$repo/artifacts/agents/missions/cooperating-host/state.json
  cp "$cooperating_state" "$fixture_root/cooperating-state.before-stop"

  # The first launch needed the checkout's holder. Retire it now so stop and
  # the closed-fence resume below classify this shell through terminal facts.
  "$repo/bin/metasystem" lease retire --root "$repo" --session mission-fixtures \
    --pid $$ --start "$("$root/bin/metasystem" proc started-at --pid $$)"
  rm -f "$repo/artifacts/agents/mains/worktree-lease.json" \
    "$repo/artifacts/agents/mains/reaped-after-claim.json"

  run_complete_stop cooperating-host "$fixture_root/cooperating-host.stop"
  grep -Fq "repo-watcher pid $watcher_pid: stopped (TERM)" "$fixture_root/cooperating-host.stop" \
    && grep -Fq "job-reaper pid $reaper_pid: stopped (TERM)" "$fixture_root/cooperating-host.stop" \
    || { echo "cooperating stop omitted its ownerless supervision outcomes" >&2; cat "$fixture_root/cooperating-host.stop" >&2; exit 1; }
  if "$repo/bin/metasystem" proc group-exists --pgid "$watcher_pid" \
    || "$repo/bin/metasystem" proc group-exists --pgid "$reaper_pid"; then
    echo "cooperating stop left the ownerless watcher or reaper process group live" >&2
    cat "$fixture_root/cooperating-host.stop" >&2
    exit 1
  fi
  cooperating_runner=$repo/artifacts/agents/missions/runners/cooperating-host.json
  cooperating_turn_record=$repo/artifacts/agents/missions/cooperating-host/turns/$cooperating_turn/turn.json
  cooperating_runner_pgid=$("$repo/bin/metasystem" json get --file "$cooperating_runner" --field pgid)
  cooperating_host_pgid=$("$repo/bin/metasystem" json get --file "$cooperating_turn_record" --field pgid)
  [[ "$("$repo/bin/metasystem" json get --file "$cooperating_runner" --field status)" == stopped \
    && "$("$repo/bin/metasystem" json get --file "$cooperating_turn_record" --field status)" == failed \
    && "$("$repo/bin/metasystem" json get --file "$cooperating_turn_record" --field error)" == turn-lost \
    && "$("$repo/bin/metasystem" json get --file "$cooperating_turn_record" --field detail)" == 'stopped by metasystem stop' \
    && "$("$repo/bin/metasystem" json get --file "$cooperating_turn_record" --field hostTermination)" == term ]] \
    || { echo "cooperating host did not conclude as a stopped mission turn" >&2; cat "$fixture_root/cooperating-host.stop" >&2; cat "$cooperating_runner" >&2; cat "$cooperating_turn_record" >&2; exit 1; }
  [[ ! -e "$repo/artifacts/agents/missions/cooperating-host/lease.d" \
    && ! -e "$repo/artifacts/agents/missions/cooperating-host/lease.json" ]] \
    || { echo "mission stop left the cooperating runner lease" >&2; cat "$fixture_root/cooperating-host.stop" >&2; exit 1; }
  if "$repo/bin/metasystem" proc group-exists --pgid "$cooperating_runner_pgid" \
    || "$repo/bin/metasystem" proc group-exists --pgid "$cooperating_host_pgid"; then
    echo "mission stop left the cooperating runner or host process group live" >&2
    cat "$fixture_root/cooperating-host.stop" >&2
    exit 1
  fi
  cmp -s "$fixture_root/cooperating-state.before-stop" "$cooperating_state" \
    || { echo "mission stop changed runner-owned mission state" >&2; cat "$fixture_root/cooperating-host.stop" >&2; exit 1; }
  set +e
  "$repo/bin/metasystem" mission status --root "$repo" --mission cooperating-host \
    >"$fixture_root/cooperating-host.status" 2>&1
  cooperating_status_rc=$?
  set -e
  [[ $cooperating_status_rc -eq 13 \
    && "$(cat "$fixture_root/cooperating-host.status")" == 'mission=cooperating-host status=stopped reason=metasystem-stop' ]] \
    || { echo "stopped mission status did not name resume" >&2; cat "$fixture_root/cooperating-host.status" >&2; exit 1; }

  fabricate_supervisor_facts
  export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE=$identity_file
  resumed_turn=$(launch_held_mission cooperating-host 0 resume)
  [[ "$resumed_turn" != "$cooperating_turn" ]] \
    || { echo "mission resume did not start a successor turn" >&2; cat "$fixture_root/cooperating-host-resume.out" >&2; exit 1; }
  [[ "$("$repo/bin/metasystem" json get --file "$repo/artifacts/agents/supervision/transition.json" --field state)" == open \
    && "$("$repo/bin/metasystem" json get --file "$repo/artifacts/agents/supervision/transition.json" --field by.verb)" == mission-resume ]] \
    || { echo "fixture-human resume did not open the closed process fence" >&2; cat "$fixture_root/cooperating-host-resume.out" >&2; exit 1; }
  run_complete_stop cooperating-host-resumed "$fixture_root/cooperating-host-resumed.stop"

  fabricate_supervisor_facts
  export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE=$identity_file
  make_end_state_contract ignores-term no-return
  close_bed_baseline "$repo"
  ignored_turn=$(launch_held_mission ignores-term 1 start)
  run_complete_stop ignores-term "$fixture_root/ignores-term.stop"
  grep -Fq 'mission ignores-term runner ' "$fixture_root/ignores-term.stop" \
    && grep -Fq ': stopped (TERM, runner concluded)' "$fixture_root/ignores-term.stop" \
    && grep -Fq "mission ignores-term turn $ignored_turn host " "$fixture_root/ignores-term.stop" \
    && grep -Fq ': killed (by the runner, TERM ignored)' "$fixture_root/ignores-term.stop" \
    || { echo "ignore-TERM host did not exercise the runner's kill ladder" >&2; cat "$fixture_root/ignores-term.stop" >&2; exit 1; }
  ignored_turn_record=$repo/artifacts/agents/missions/ignores-term/turns/$ignored_turn/turn.json
  [[ "$("$repo/bin/metasystem" json get --file "$ignored_turn_record" --field hostTermination)" == kill ]] \
    || { echo "ignore-TERM host turn did not record kill termination" >&2; cat "$fixture_root/ignores-term.stop" >&2; cat "$ignored_turn_record" >&2; exit 1; }
  ignored_runner_record=$repo/artifacts/agents/missions/runners/ignores-term.json
  ignored_runner_pgid=$("$repo/bin/metasystem" json get --file "$ignored_runner_record" --field pgid)
  ignored_host_pgid=$("$repo/bin/metasystem" json get --file "$ignored_turn_record" --field pgid)
  if "$repo/bin/metasystem" proc group-exists --pgid "$ignored_runner_pgid" \
    || "$repo/bin/metasystem" proc group-exists --pgid "$ignored_host_pgid"; then
    echo "mission stop left the ignore-TERM runner or host process group live" >&2
    cat "$fixture_root/ignores-term.stop" >&2
    exit 1
  fi
  exit 0
fi

make_end_state_contract gate-and-close close-stream
# A landed return no host ever acted on (records/patience/patience-orphan-usage.md):
# the chain is already runner-closed — closure never hides a landed round —
# so the completed mission must list it in every turn prompt and deliver it
# into the final ledger block as a Landed unconsumed annotation.
mkdir -p "$repo/artifacts/agents/jobs" "$repo/artifacts/agents/landed-orphan/rounds/1"
cat >"$repo/artifacts/agents/jobs/landed-orphan.json" <<'EOF'
{
  "jobId": "landed-orphan",
  "mission": "gate-and-close",
  "status": "completed",
  "round": 1,
  "parentJob": null,
  "chainClosed": true,
  "runnerClosed": true
}
EOF
printf '{"jobId":"landed-orphan"}\n' >"$repo/artifacts/agents/landed-orphan/rounds/1/return.json"
close_bed_baseline "$repo"
METASYSTEM_AGENT_RUNTIME=fake "$repo/bin/metasystem" mission start --root "$repo" \
  --mission gate-and-close --foreground >/dev/null
wait_end_state gate-and-close 10
# The end-state details this leg used to re-assert — completed state,
# park reason, landed-orphan prompt/ledger/usage annotations — are
# TestInternalRunCloseStreamCycle and
# TestDeliverLandedUnconsumedWritesFinalBlock (internal/missionrunner,
# under the go gate; script-fixtures-005/D40). The runner launch and the
# status exit-10 wait above remain the process-level proof.

make_end_state_contract runner-closes-chain dispatch-terminal
close_bed_baseline "$repo"
METASYSTEM_AGENT_RUNTIME=fake "$repo/bin/metasystem" mission start --root "$repo" \
  --mission runner-closes-chain --foreground >/dev/null
wait_end_state runner-closes-chain 10
# The runner-closed chain, mirror manifest, and turn-log acceptance are
# TestInternalRunDispatchTerminalCycle and TestArmAndPreflightFullPass
# (internal/missionrunner; script-fixtures-005/D40).

# The host adapter is a witness, not a judge (records/patience/patience-turn-identity.md
# T3): a rotated session is reported in the result envelope with outcome
# completed for the runner's adjudication; only a MISSING session keeps the
# adapter's own exit-6 fault; and a start gate that never releases still
# fails the launch. Driven against the real claude host adapter with only the
# paid CLI call replaced.
host_fixture=$fixture_root/host-adapter
host_bin=$host_fixture/bin
host_turn=$host_fixture/turns/host-session-t1-aaaa
mkdir -p "$host_bin" "$host_turn"
cat >"$host_bin/claude" <<'CLAUDE'
#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
if [[ ${FAKE_CLAUDE_SESSION:-} == none ]]; then
  printf '{"result":"{}","usage":{"input_tokens":1,"output_tokens":1}}\n'
else
  printf '{"session_id":"%s","result":"{}","usage":{"input_tokens":1,"output_tokens":1}}\n' \
    "${FAKE_CLAUDE_SESSION:?}"
fi
CLAUDE
chmod +x "$host_bin/claude"
printf '{"missionId":"host-session","turnId":"host-session-t1-aaaa","cycle":1,"model":"fixture-model","hostSession":"announced-session"}\n' \
  >"$host_turn/turn.json"
printf 'host adapter session fixture prompt\n' >"$host_turn/prompt.md"

FAKE_CLAUDE_SESSION=rotated-session PATH="$host_bin:$PATH" \
  "$root/scripts/agents/hosts/claude.sh" start-turn --mission host-session \
  --turn-id host-session-t1-aaaa --prompt "$host_turn/prompt.md" \
  --result "$host_turn/result.json" --instance-tag fixture-host-session-tag \
  --resume-session announced-session
[[ "$("$root/bin/metasystem" json get --file "$host_turn/result.json" --field outcome)" == completed ]] \
  || { echo "a rotated session did not stay outcome completed" >&2; cat "$host_turn/result.json" >&2; exit 1; }
[[ "$("$root/bin/metasystem" json get --file "$host_turn/result.json" --field sessionId)" == rotated-session ]] \
  || { echo "the result envelope did not report the rotated session" >&2; cat "$host_turn/result.json" >&2; exit 1; }

set +e
FAKE_CLAUDE_SESSION=none PATH="$host_bin:$PATH" \
  "$root/scripts/agents/hosts/claude.sh" start-turn --mission host-session \
  --turn-id host-session-t1-aaaa --prompt "$host_turn/prompt.md" \
  --result "$host_turn/result-missing.json" --instance-tag fixture-host-session-tag \
  --resume-session announced-session >"$fixture_root/host-missing-session.out" 2>&1
missing_status=$?
set -e
[[ $missing_status -eq 6 ]] \
  || { echo "missing host session did not keep exit 6 (got $missing_status)" >&2; exit 1; }
[[ "$("$root/bin/metasystem" json get --file "$host_turn/result-missing.json" --field outcome)" == unresumable \
   && "$("$root/bin/metasystem" json get --file "$host_turn/result-missing.json" --field sessionId)" == null ]] \
  || { echo "a missing session did not report unresumable with a null session" >&2; cat "$host_turn/result-missing.json" >&2; exit 1; }

set +e
FAKE_CLAUDE_SESSION=rotated-session PATH="$host_bin:$PATH" \
  METASYSTEM_HOST_START_GATE="$host_fixture/never-released" \
  METASYSTEM_HOST_START_GATE_TIMEOUT_SEC=1 \
  "$root/scripts/agents/hosts/claude.sh" start-turn --mission host-session \
  --turn-id host-session-t1-aaaa --prompt "$host_turn/prompt.md" \
  --result "$host_turn/result-gate.json" --instance-tag fixture-host-session-tag \
  >"$fixture_root/host-gate-timeout.out" 2>&1
gate_status=$?
set -e
[[ $gate_status -eq 3 ]] \
  || { echo "unreleased start gate did not fail the launch (got $gate_status)" >&2; exit 1; }
grep -Fq 'start gate was not released' "$fixture_root/host-gate-timeout.out" \
  || { echo "start-gate timeout did not name its refusal" >&2; exit 1; }
[[ ! -e "$host_turn/result-gate.json" ]] \
  || { echo "a launch that never passed the gate must not write a result envelope" >&2; exit 1; }
