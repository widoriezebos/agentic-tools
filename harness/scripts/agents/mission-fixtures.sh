#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$root"
fixture_root=$(mktemp -d)
repo=$fixture_root/repo
remote=$fixture_root/origin.git
watcher_pid=
reaper_pid=

wait_for_fixture_pid() { # name, pid, named ceiling seconds
  local name=$1 pid=$2 maximum started deadline elapsed
  maximum=$(harness_fixture_scaled_cap "$3")
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
    sleep 0.05
  done
  wait "$pid" 2>/dev/null || true
}

cleanup() {
  local pid
  for pid in "$watcher_pid" "$reaper_pid"; do
    [[ -n "$pid" ]] || continue
    kill -TERM "$pid" 2>/dev/null || true
    wait_for_fixture_pid supervisor-cleanup "$pid" 5 || true
  done
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

mkdir -p "$repo/scripts/agents" "$repo/scripts" "$repo/truth" "$repo/plans" "$repo/docs"
git init -q -b main "$repo"
git init -q --bare "$remote"
git -C "$repo" config user.name harness
git -C "$repo" config user.email harness@example.invalid

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
git -C "$repo" add scripts truth docs
git -C "$repo" commit -qm instruments
git -C "$repo" tag instruments
git -C "$repo" remote add origin "$remote"
git -C "$repo" push -qu origin main
git -C "$remote" symbolic-ref HEAD refs/heads/main
git -C "$repo" remote set-head origin -a >/dev/null

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
fence.wall-clock-hours=2
fence.cycles=3
fence.jobs=4
fence.concurrency=2
fence.job-cap-min=5
host.runtime=fake
host.model=fake-model
host.turn-cap-min=5
stream.primary=Reach the acceptance score.
envelope.dependencies=jq
exposure=EUR:10
```
CONTRACT

"$root/scripts/assert-mission.sh" --file "$base" >/dev/null

# Every concrete key in the authored grammar is independently required and
# independently type-checked. These fixture commands are synchronous; the
# gate path itself carries fence.job-cap-min as its named execution ceiling.
contract_keys=()
while IFS= read -r key; do contract_keys+=("$key"); done \
  < <(sed -n '/^```mission$/,/^```$/ { /^```/d; s/=.*//p; }' "$base")
for key in "${contract_keys[@]}"; do
  missing="$cases/missing-${key//./-}.md"
  malformed="$cases/malformed-${key//./-}.md"
  python3 - "$base" "$missing" "$malformed" "$key" <<'PY'
import sys
from pathlib import Path
source, missing, malformed, wanted = Path(sys.argv[1]), Path(sys.argv[2]), Path(sys.argv[3]), sys.argv[4]
lines = source.read_text().splitlines()
missing.write_text("\n".join(line for line in lines if not line.startswith(wanted + "=")) + "\n")
bad = {
    "gate.command": " value ",
    "gate.ref": "bad ref",
    "gate.paths": "../outside",
    "truth.paths": "/absolute",
    "truth.certification": "goldish",
    "gate.direction": "up",
    "gate.threshold.score": "=1",
    "gate.noise-floor.score": "-1",
    "guard.audit.command": " value ",
    "guard.audit.floor": "one",
    "guard.audit.noise": "-1",
    "guard.cadence": "0",
    "ledger.cycle-budget": "0",
    "ledger.no-gain-budget": "none",
    "fence.wall-clock-hours": "0",
    "fence.cycles": "0",
    "fence.jobs": "all",
    "fence.concurrency": "0",
    "fence.job-cap-min": "1.5",
    "host.runtime": "bad runtime",
    "host.model": "bad model",
    "host.turn-cap-min": "0",
    "stream.primary": " value ",
    "stream.secondary": " value ",
    "envelope.dependencies": "whatever seems safe",
    "exposure": "10-ish",
}[wanted]
malformed.write_text("\n".join(f"{wanted}={bad}" if line.startswith(wanted + "=") else line for line in lines) + "\n")
PY
  expect_failure "missing-${key//./-}" "" "$root/scripts/assert-mission.sh" --file "$missing"
  expect_failure "malformed-${key//./-}" "" "$root/scripts/assert-mission.sh" --file "$malformed"
done

unbounded=$cases/unbounded.md
sed 's/envelope.dependencies=jq/envelope.dependencies=whatever seems safe/' "$base" >"$unbounded"
expect_failure unbounded-envelope "unbounded or non-literal" "$root/scripts/assert-mission.sh" --file "$unbounded"
unbounded_literal=$cases/unbounded-literal.md
sed 's/envelope.dependencies=jq/envelope.dependencies=all/' "$base" >"$unbounded_literal"
expect_failure unbounded-literal-envelope "unbounded or non-literal" "$root/scripts/assert-mission.sh" --file "$unbounded_literal"
forbidden=$cases/forbidden.md
sed 's/envelope.dependencies=jq/envelope.production-data=fixture/' "$base" >"$forbidden"
expect_failure forbidden-envelope "not marked pre-authorizable" "$root/scripts/assert-mission.sh" --file "$forbidden"

unsealed=$repo/plans/mission-unsealed.contract.md
cp "$base" "$unsealed"
printf '\nApproval: name=Human; date=2026-08-04; contract-sha256=%064d\n' 0 >>"$unsealed"
expect_failure unsealed-preflight "unsealed" "$root/scripts/assert-mission.sh" --preflight --file "$unsealed"

unsigned=$repo/plans/mission-unsigned.contract.md
cp "$base" "$unsigned"
"$root/scripts/assert-mission.sh" --seal --file "$unsigned" >/dev/null
expect_failure unsigned-preflight "unsigned" "$root/scripts/assert-mission.sh" --preflight --file "$unsigned"

mismatched=$repo/plans/mission-mismatched.contract.md
cp "$base" "$mismatched"
"$root/scripts/assert-mission.sh" --seal --file "$mismatched" >/dev/null
printf '\nApproval: name=Human; date=2026-08-04; contract-sha256=%064d\n' 0 >>"$mismatched"
expect_failure mismatched-preflight "approval hash" "$root/scripts/assert-mission.sh" --preflight --file "$mismatched"

stale=$repo/plans/mission-stale.contract.md
cp "$base" "$stale"
"$root/scripts/assert-mission.sh" --seal --file "$stale" >/dev/null
python3 - "$stale" <<'PY'
import sys
from pathlib import Path
path=Path(sys.argv[1]); text=path.read_text(); path.write_text(text.replace("fence.jobs=4", "fence.jobs=5", 1))
PY
expect_failure stale-exposure "exposure is stale" "$root/scripts/assert-mission.sh" --preflight --file "$stale"

contract=$repo/plans/mission-alpha.contract.md
cp "$base" "$contract"
sed -i.bak '/stream.primary=/a\
stream.secondary=Preserve the evidence contract.
' "$contract"
rm "$contract.bak"
contract_sha=$("$root/scripts/assert-mission.sh" --seal --file "$contract")
printf '\nApproval: name=Human; date=2026-08-04; contract-sha256=%s\n' "$contract_sha" >>"$contract"
git -C "$repo" add plans/mission-alpha.contract.md
git -C "$repo" commit -qm 'sign mission contract'
git -C "$repo" push -qu origin main

# Fabricate only the supervisor facts preflight reads. Each process is a real
# live process whose argv carries the recorded tag; cleanup waits are named
# and ceiling-bounded above.
python3 -c 'import time; time.sleep(60)' mission-watcher-tag & watcher_pid=$!
python3 -c 'import time; time.sleep(60)' mission-reaper-tag & reaper_pid=$!
identity_file=$fixture_root/mission-process-identities.json
printf '{"%s":{"pidStartedAt":%s,"command":"fixture mission-watcher-tag"},"%s":{"pidStartedAt":%s,"command":"fixture mission-reaper-tag"}}\n' \
  "$watcher_pid" "$watcher_pid" "$reaper_pid" "$reaper_pid" >"$identity_file"
export HARNESS_MISSION_PROCESS_IDENTITY_FILE=$identity_file
supervision=$repo/artifacts/agents/supervision
mkdir -p "$supervision"
python3 - "$supervision" "$watcher_pid" "$reaper_pid" <<'PY'
import json, sys, time
from pathlib import Path
directory, watcher, reaper = Path(sys.argv[1]), int(sys.argv[2]), int(sys.argv[3])
now = int(time.time())
watcher_hb = directory / "watcher.heartbeat.json"
reaper_hb = directory / "reaper.heartbeat.json"
watcher_hb.write_text(json.dumps({"function":"watcher","pid":watcher,"pidStartedAt":watcher,"observedAtEpoch":now}) + "\n")
reaper_hb.write_text(json.dumps({"function":"reaper","pid":reaper,"pidStartedAt":reaper,"observedAtEpoch":now}) + "\n")
state = {"intervalSec":60,"fingerprint":"fixture-fingerprint","components":{
    "watcher":{"pid":watcher,"pidStartedAt":watcher,"instanceTag":"mission-watcher-tag","heartbeat":str(watcher_hb)},
    "reaper":{"pid":reaper,"pidStartedAt":reaper,"instanceTag":"mission-reaper-tag","heartbeat":str(reaper_hb)},
}}
(directory / "state.json").write_text(json.dumps(state) + "\n")
(directory / "last-census.json").write_text(json.dumps({"verdict":"SUCCESS","completedAtEpoch":now,"fingerprint":"fixture-fingerprint"}) + "\n")
PY

"$root/scripts/assert-mission.sh" --preflight --file "$contract" >/dev/null
mv "$supervision/state.json" "$supervision/state.unarmed"
expect_failure unarmed-preflight "supervisor set is unarmed" "$root/scripts/assert-mission.sh" --preflight --file "$contract"
mv "$supervision/state.unarmed" "$supervision/state.json"
mkdir -p "$repo/artifacts/agents/missions/alpha/lease.d"
expect_failure lease-preflight "lease is not acquirable" "$root/scripts/assert-mission.sh" --preflight --file "$contract"
rmdir "$repo/artifacts/agents/missions/alpha/lease.d"

printf 'bad candidate\n' >"$repo/candidate-bad"
git -C "$repo" add candidate-bad
git -C "$repo" commit -qm 'make candidate unmeasurable'
git -C "$repo" push -qu origin main
expect_failure unrunnable-gate "gate measurement failed" "$root/scripts/assert-mission.sh" --preflight --file "$contract"

# Mission ledger grammar is consumed by assert-stop-loss.sh without any
# adapter or conversion layer.
ledger=$repo/artifacts/agents/missions/alpha/ledger.md
"$root/scripts/agents/mission-ledger.py" init --file "$ledger" --cycle-budget 2 --no-gain-budget 2
current_sha=$(git -C "$repo" rev-parse HEAD)
"$root/scripts/agents/mission-ledger.py" append --file "$ledger" --cycle 1 --classification no-progress --candidate-sha "$current_sha" --observed score=1
"$root/scripts/agents/mission-ledger.py" append --file "$ledger" --cycle 2 --classification unresolved --candidate-sha "$current_sha" --observed score=1
expect_failure mission-stop-loss "stop-loss triggered" "$root/scripts/assert-stop-loss.sh" --file "$ledger"

# State updates prove one reserved stream can park while another remains
# active, and that parked-stop-loss cannot be self-assigned or self-unparked.
state_ledger=$repo/artifacts/agents/missions/alpha/state-ledger.md
"$root/scripts/agents/mission-ledger.py" init --file "$state_ledger" --cycle-budget 3 --no-gain-budget 2
state=$repo/artifacts/agents/missions/alpha/state.json
"$root/scripts/agents/mission-state.py" init --state "$state" --contract "$contract" --ledger "$state_ledger" --branch main
state_hash=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["integrity"]["hash"])' "$state")
proposal=$fixture_root/state-proposal.json
python3 - "$state" "$proposal" <<'PY'
import json, sys
from pathlib import Path
value=json.loads(Path(sys.argv[1]).read_text()); value.pop("integrity")
value["streams"]["primary"].update({"state":"parked-reserved","reason":"reserved-decision","answeredAsk":None})
Path(sys.argv[2]).write_text(json.dumps(value) + "\n")
PY
"$root/scripts/agents/mission-state.py" write --state "$state" --source "$proposal" --expect "$state_hash"
"$root/scripts/agents/mission-state.py" verify --state "$state" >/dev/null
python3 - "$state" <<'PY'
import json,sys
value=json.load(open(sys.argv[1]))
assert value["status"] == "running"
assert value["streams"]["primary"]["state"] == "parked-reserved"
assert value["streams"]["secondary"]["state"] == "active"
PY

stoploss_proposal=$fixture_root/state-stoploss-proposal.json
python3 - "$state" "$stoploss_proposal" <<'PY'
import json, sys
from pathlib import Path
value=json.loads(Path(sys.argv[1]).read_text()); value.pop("integrity")
value["streams"]["secondary"].update({"state":"parked-stop-loss","reason":"stop-loss","answeredAsk":None})
Path(sys.argv[2]).write_text(json.dumps(value) + "\n")
PY
state_hash=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["integrity"]["hash"])' "$state")
expect_failure self-park-stop-loss "reserved for a human answer" "$root/scripts/agents/mission-state.py" write --state "$state" --source "$stoploss_proposal" --expect "$state_hash"

forked=$fixture_root/forked-state.json
cp "$state" "$forked"
python3 - "$forked" <<'PY'
import json,sys
from pathlib import Path
path=Path(sys.argv[1]); value=json.loads(path.read_text())
value["integrity"]["history"][-1]["previousHash"]="f"*64
path.write_text(json.dumps(value) + "\n")
PY
expect_failure state-chain-fork "fork" "$root/scripts/agents/mission-state.py" verify --state "$forked"
set +e
"$root/scripts/agents/mission-state.py" reconcile --state "$forked" --repo "$repo" --ledger "$state_ledger"
fork_reconcile_status=$?
set -e
[[ $fork_reconcile_status -eq 3 ]] || { echo "forked state reconciliation did not park with exit 3" >&2; exit 1; }
python3 - "$forked" <<'PY'
import json,sys
from pathlib import Path
path=Path(sys.argv[1]); value=json.loads(path.read_text())
assert value["status"] == "parked" and value["parkReason"] == "state-integrity"
assert value["integrity"]["recoveryOf"] and list(path.parent.glob("state.corrupt.*.json"))
PY
"$root/scripts/agents/mission-state.py" verify --state "$forked" >/dev/null

anchor_state=$repo/artifacts/agents/missions/alpha/anchor-state.json
anchor_ledger=$repo/artifacts/agents/missions/alpha/anchor-ledger.md
"$root/scripts/agents/mission-ledger.py" init --file "$anchor_ledger" --cycle-budget 3 --no-gain-budget 2
"$root/scripts/agents/mission-state.py" init --state "$anchor_state" --contract "$contract" --ledger "$anchor_ledger" --branch main
"$root/scripts/agents/mission-state.py" anchor --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger" >/dev/null
"$root/scripts/agents/mission-state.py" verify --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger" >/dev/null
"$root/scripts/agents/mission-ledger.py" append --file "$anchor_ledger" --cycle 1 --classification unresolved --candidate-sha "$current_sha" --observed score=1
"$root/scripts/agents/mission-state.py" reconcile --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger"
python3 - "$anchor_state" <<'PY'
import json,sys
value=json.load(open(sys.argv[1])); assert value["ledger"]["cycles"] == 1 and value["fences"]["cycles"] == 1
PY
"$root/scripts/agents/mission-state.py" anchor --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger" >/dev/null
"$root/scripts/agents/mission-state.py" verify --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger" >/dev/null
anchor_hash=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["integrity"]["hash"])' "$anchor_state")
python3 - "$anchor_state" "$proposal" <<'PY'
import json,sys
from pathlib import Path
value=json.loads(Path(sys.argv[1]).read_text()); value.pop("integrity")
value["streams"]["primary"].update({"state":"parked-reserved","reason":"reserved-decision","answeredAsk":None})
Path(sys.argv[2]).write_text(json.dumps(value) + "\n")
PY
"$root/scripts/agents/mission-state.py" write --state "$anchor_state" --source "$proposal" --expect "$anchor_hash"
expect_failure rewritten-anchor "anchor disagrees" "$root/scripts/agents/mission-state.py" verify --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger"

# The reaper uses this same locked refusal operation when a mission-stamped
# job reaches its cap; prove the resulting ask as a world-state fact.
mkdir -p "$repo/plans" "$repo/artifacts/agents/missions/timeout-ask"
cp "$base" "$repo/plans/mission-timeout-ask.contract.md"
"$root/scripts/agents/mission-fence.py" refuse --repo "$repo" --mission timeout-ask --reason job-cap-min >/dev/null
timeout_ask=$repo/artifacts/agents/missions/timeout-ask/asks/fence-bound.json
[[ -f "$timeout_ask" ]] || { echo "mission timeout refusal did not write its ask" >&2; exit 1; }
grep -Fq '`job-cap-min`' "$timeout_ask" || { echo "mission timeout ask omitted its reached fence" >&2; exit 1; }

# Two simultaneous reservations cannot both cross a concurrency fence. The
# exact subprocess waits are named and ceiling-bounded (IL-1).
race_contract=$repo/plans/mission-race.contract.md
sed 's/fence.concurrency=2/fence.concurrency=1/' "$base" >"$race_contract"
(
  set +e
  "$root/scripts/agents/mission-fence.py" reserve-job --repo "$repo" --mission race --job race-a --cap-min 1 >"$fixture_root/race-a.out" 2>&1
  printf '%s\n' "$?" >"$fixture_root/race-a.status"
) &
race_a_pid=$!
(
  set +e
  "$root/scripts/agents/mission-fence.py" reserve-job --repo "$repo" --mission race --job race-b --cap-min 1 >"$fixture_root/race-b.out" 2>&1
  printf '%s\n' "$?" >"$fixture_root/race-b.status"
) &
race_b_pid=$!
wait_for_fixture_pid concurrency-reservation-a "$race_a_pid" 5
wait_for_fixture_pid concurrency-reservation-b "$race_b_pid" 5
race_total=$(( $(cat "$fixture_root/race-a.status") + $(cat "$fixture_root/race-b.status") ))
[[ $race_total -eq 1 ]] || { echo "mission concurrency lock admitted zero or two reservations" >&2; exit 1; }
race_ask=$repo/artifacts/agents/missions/race/asks/fence-bound.json
[[ -f "$race_ask" ]] && grep -Fq '`concurrency`' "$race_ask" \
  || { echo "concurrent mission refusal did not write its batched ask" >&2; exit 1; }

echo "mission contract and state fixtures passed"
