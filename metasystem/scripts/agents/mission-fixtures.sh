#!/usr/bin/env bash
set -euo pipefail

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
  local pid
  for pid in "$watcher_pid" "$reaper_pid"; do
    [[ -n "$pid" ]] || continue
    kill -TERM "$pid" 2>/dev/null || true
    wait_for_fixture_pid supervisor-cleanup "$pid" mission-process-wait || true
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

mkdir -p "$repo/scripts/agents" "$repo/scripts" "$repo/truth" "$repo/plans" "$repo/docs" "$repo/bin"
# The copied assert scripts resolve their engine as <repo>/bin/metasystem.
cp "$root/bin/metasystem" "$repo/bin/metasystem"
git init -q -b main "$repo"
git init -q --bare "$remote"
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
fence.job-cap-min=FIXTURE_JOB_CAP_MIN
host.runtime=fake
host.model=fake-model
host.turn-cap-min=FIXTURE_TURN_CAP_MIN
stream.primary=Reach the acceptance score.
envelope.dependencies=jq
exposure=EUR:10
```
CONTRACT
perl -0pi -e 's/FIXTURE_JOB_CAP_MIN/'"$mission_job_cap_min"'/g; s/FIXTURE_TURN_CAP_MIN/'"$mission_turn_cap_min"'/g' "$base"

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
tier_move=$cases/tier-move.md
sed 's/envelope.dependencies=jq/envelope.tier-move=3/' "$base" >"$tier_move"
expect_failure tier-move-retired "use envelope.dispatch-allow" "$root/scripts/assert-mission.sh" --file "$tier_move"
malformed_dispatch_allow=$cases/malformed-dispatch-allow.md
sed 's/envelope.dependencies=jq/envelope.dispatch-allow=fake-model/' "$base" >"$malformed_dispatch_allow"
expect_failure malformed-dispatch-allow "exact runtime:model pairs" "$root/scripts/assert-mission.sh" --file "$malformed_dispatch_allow"

dispatch_allow=$repo/plans/mission-dispatch-allow.contract.md
sed 's/envelope.dependencies=jq/envelope.dispatch-allow=fake:fake-model,codex:gpt-5.6-sol/' "$base" >"$dispatch_allow"
"$root/scripts/assert-mission.sh" --seal --file "$dispatch_allow" >/dev/null
# The dispatch-allow value survives sealing byte-exactly and the sealed
# contract still validates. (The parser-internal assertions the python
# module leg made — the values map and pair splitting — are owned by
# internal/mission's contract unit tests under the go gate.)
"$root/bin/metasystem" mission-contract validate --file "$dispatch_allow" >/dev/null
grep -Fq 'envelope.dispatch-allow=fake:fake-model,codex:gpt-5.6-sol' "$dispatch_allow" \
  || { echo "sealing altered the dispatch-allow envelope line" >&2; exit 1; }

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
"$root/bin/metasystem" util hold --tag mission-watcher-tag & watcher_pid=$!
"$root/bin/metasystem" util hold --tag mission-reaper-tag & reaper_pid=$!
# The engine ships in this fixture repo, so preflight demands EXACT-START
# liveness: record the holders' real start times, not synthetic ones.
watcher_start=$("$root/bin/metasystem" identity started-at --pid "$watcher_pid")
reaper_start=$("$root/bin/metasystem" identity started-at --pid "$reaper_pid")
identity_file=$fixture_root/mission-process-identities.json
printf '{"%s":{"pidStartedAt":%s,"command":"fixture mission-watcher-tag"},"%s":{"pidStartedAt":%s,"command":"fixture mission-reaper-tag"}}\n' \
  "$watcher_pid" "$watcher_start" "$reaper_pid" "$reaper_start" >"$identity_file"
export METASYSTEM_MISSION_PROCESS_IDENTITY_FILE=$identity_file
supervision=$repo/artifacts/agents/supervision
mkdir -p "$supervision"
python3 - "$supervision" "$watcher_pid" "$reaper_pid" "$watcher_start" "$reaper_start" <<'PY'
import json, sys, time
from pathlib import Path
directory = Path(sys.argv[1])
watcher, reaper, watcher_start, reaper_start = map(int, sys.argv[2:6])
now = int(time.time())
watcher_hb = directory / "watcher.heartbeat.json"
reaper_hb = directory / "reaper.heartbeat.json"
watcher_hb.write_text(json.dumps({"function":"watcher","pid":watcher,"pidStartedAt":watcher_start,"observedAtEpoch":now}) + "\n")
reaper_hb.write_text(json.dumps({"function":"reaper","pid":reaper,"pidStartedAt":reaper_start,"observedAtEpoch":now}) + "\n")
state = {"intervalSec":60,"fingerprint":"fixture-fingerprint","components":{
    "watcher":{"pid":watcher,"pidStartedAt":watcher_start,"instanceTag":"mission-watcher-tag","heartbeat":str(watcher_hb)},
    "reaper":{"pid":reaper,"pidStartedAt":reaper_start,"instanceTag":"mission-reaper-tag","heartbeat":str(reaper_hb)},
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
"$root/bin/metasystem" mission-ledger init --file "$ledger" --cycle-budget 2 --no-gain-budget 2
current_sha=$(git -C "$repo" rev-parse HEAD)
"$root/bin/metasystem" mission-ledger append --file "$ledger" --cycle 1 --classification no-progress --candidate-sha "$current_sha" --observed score=1
"$root/bin/metasystem" mission-ledger append --file "$ledger" --cycle 2 --classification unresolved --candidate-sha "$current_sha" --observed score=1
expect_failure mission-stop-loss "stop-loss triggered" "$root/scripts/assert-stop-loss.sh" --file "$ledger"

# State updates prove one reserved stream can park while another remains
# active, and that parked-stop-loss cannot be self-assigned or self-unparked.
state_ledger=$repo/artifacts/agents/missions/alpha/state-ledger.md
"$root/bin/metasystem" mission-ledger init --file "$state_ledger" --cycle-budget 3 --no-gain-budget 2
state=$repo/artifacts/agents/missions/alpha/state.json
"$root/bin/metasystem" mission-state init --state "$state" --contract "$contract" --ledger "$state_ledger" --branch main
state_hash=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["integrity"]["hash"])' "$state")
proposal=$fixture_root/state-proposal.json
python3 - "$state" "$proposal" <<'PY'
import json, sys
from pathlib import Path
value=json.loads(Path(sys.argv[1]).read_text()); value.pop("integrity")
value["streams"]["primary"].update({"state":"parked-reserved","reason":"reserved-decision","answeredAsk":None})
Path(sys.argv[2]).write_text(json.dumps(value) + "\n")
PY
"$root/bin/metasystem" mission-state write --state "$state" --source "$proposal" --expect "$state_hash"
"$root/bin/metasystem" mission-state verify --state "$state" >/dev/null
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
expect_failure self-park-stop-loss "reserved for a human answer" "$root/bin/metasystem" mission-state write --state "$state" --source "$stoploss_proposal" --expect "$state_hash"

forked=$fixture_root/forked-state.json
cp "$state" "$forked"
python3 - "$forked" <<'PY'
import json,sys
from pathlib import Path
path=Path(sys.argv[1]); value=json.loads(path.read_text())
value["integrity"]["history"][-1]["previousHash"]="f"*64
path.write_text(json.dumps(value) + "\n")
PY
expect_failure state-chain-fork "fork" "$root/bin/metasystem" mission-state verify --state "$forked"
set +e
"$root/bin/metasystem" mission-state reconcile --state "$forked" --repo "$repo" --ledger "$state_ledger"
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
"$root/bin/metasystem" mission-state verify --state "$forked" >/dev/null

anchor_state=$repo/artifacts/agents/missions/alpha/anchor-state.json
anchor_ledger=$repo/artifacts/agents/missions/alpha/anchor-ledger.md
"$root/bin/metasystem" mission-ledger init --file "$anchor_ledger" --cycle-budget 3 --no-gain-budget 2
"$root/bin/metasystem" mission-state init --state "$anchor_state" --contract "$contract" --ledger "$anchor_ledger" --branch main
"$root/bin/metasystem" mission-state anchor --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger" >/dev/null
"$root/bin/metasystem" mission-state verify --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger" >/dev/null
"$root/bin/metasystem" mission-ledger append --file "$anchor_ledger" --cycle 1 --classification unresolved --candidate-sha "$current_sha" --observed score=1
"$root/bin/metasystem" mission-state reconcile --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger"
python3 - "$anchor_state" <<'PY'
import json,sys
value=json.load(open(sys.argv[1])); assert value["ledger"]["cycles"] == 1 and value["fences"]["cycles"] == 1
PY
"$root/bin/metasystem" mission-state anchor --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger" >/dev/null
"$root/bin/metasystem" mission-state verify --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger" >/dev/null
anchor_hash=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["integrity"]["hash"])' "$anchor_state")
python3 - "$anchor_state" "$proposal" <<'PY'
import json,sys
from pathlib import Path
value=json.loads(Path(sys.argv[1]).read_text()); value.pop("integrity")
value["streams"]["primary"].update({"state":"parked-reserved","reason":"reserved-decision","answeredAsk":None})
Path(sys.argv[2]).write_text(json.dumps(value) + "\n")
PY
"$root/bin/metasystem" mission-state write --state "$anchor_state" --source "$proposal" --expect "$anchor_hash"
expect_failure rewritten-anchor "anchor disagrees" "$root/bin/metasystem" mission-state verify --state "$anchor_state" --repo "$repo" --ledger "$anchor_ledger"

# The reaper uses this same locked refusal operation when a mission-stamped
# job reaches its cap; prove the resulting ask as a world-state fact.
mkdir -p "$repo/plans" "$repo/artifacts/agents/missions/timeout-ask"
cp "$base" "$repo/plans/mission-timeout-ask.contract.md"
"$root/bin/metasystem" mission-fence refuse --repo "$repo" --mission timeout-ask --reason job-cap-min >/dev/null
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
python3 - "$race_contract" "$race_fences" <<'PY'
import hashlib
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

contract_path, fences_path = Path(sys.argv[1]), Path(sys.argv[2])
raw = contract_path.read_bytes()
fences_path.write_text(json.dumps({
    "schemaVersion": 1,
    "missionId": "race",
    "startedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "cycles": 0,
    "reservations": {},
    "approvedContractSha256": hashlib.sha256(raw).hexdigest(),
}) + "\n")
PY
(
  set +e
  "$root/bin/metasystem" mission-fence reserve-job --repo "$repo" --mission race --job race-a --cap-min "$minimum_cap_min" >"$fixture_root/race-a.out" 2>&1
  printf '%s\n' "$?" >"$fixture_root/race-a.status"
) &
race_a_pid=$!
(
  set +e
  "$root/bin/metasystem" mission-fence reserve-job --repo "$repo" --mission race --job race-b --cap-min "$minimum_cap_min" >"$fixture_root/race-b.out" 2>&1
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

# The end-state fixtures run the real runner with the fake host and synthetic
# process identities. They stay independent of the process-owning validator
# fixtures so restricted worktrees can exercise these two terminal outcomes.
cp -R "$root/scripts/agents/." "$repo/scripts/agents/"
cp "$root/scripts/metasystem-config.sh" "$root/scripts/assert-mission.sh" \
  "$root/scripts/assert-return-complete.sh" "$root/scripts/assert-stop-loss.sh" \
  "$root/scripts/assert-turn-prompt.sh" "$repo/scripts/"
cp "$root/metasystem.conf" "$repo/metasystem.conf"
perl -0pi -e 's|^evidence\.root=.*$|evidence.root='"$fixture_root/runner-evidence"'|m; s/^metasystem\.runtimes=.*$/metasystem.runtimes=fake/m' \
  "$repo/metasystem.conf"
cat >"$repo/scripts/agents/arm-supervision.sh" <<'ARM'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == fingerprint && ${2:-} == --repo && $# -eq 3 ]]; then
  printf 'fixture-fingerprint\n'
else
  printf 'ARMED fixture-supervision\n'
fi
ARM
chmod +x "$repo/scripts/agents/arm-supervision.sh"
git -C "$repo" rm -q candidate-bad
git -C "$repo" add scripts metasystem.conf
git -C "$repo" commit -qm 'install mission runner fixtures'
git -C "$repo" push -qu origin main

export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE=$identity_file
python3 - "$supervision" <<'PY'
import json,sys,time
from pathlib import Path
directory=Path(sys.argv[1]); now=int(time.time())
for name in ("watcher.heartbeat.json","reaper.heartbeat.json"):
    path=directory/name; value=json.loads(path.read_text()); value["observedAtEpoch"]=now; path.write_text(json.dumps(value)+"\n")
path=directory/"last-census.json"; value=json.loads(path.read_text()); value["completedAtEpoch"]=now; path.write_text(json.dumps(value)+"\n")
PY

# A mission runner reaps and closes job chains, and both are control-plane
# writes reserved for the checkout's holder. This shell is that main: announce
# it, which also claims the checkout, so the runner it starts authenticates
# through an announced ancestor instead of classifying as a delegate of
# whichever agent happens to be running the suite.
"$root/bin/metasystem" lease announce --root "$repo" \
  --session mission-fixtures --pid $$ \
  --start "$("$root/bin/metasystem" identity started-at --pid $$)" \
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
  contract_sha=$("$repo/scripts/assert-mission.sh" --seal --file "$path")
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
    "$repo/scripts/agents/mission-runner.sh" status --mission "$mission" >/dev/null 2>&1
    result=$?
    set -e
    [[ $result -eq $expected ]] && return 0
    if [[ -f "$repo/artifacts/agents/missions/runners/$mission.json" ]] \
      && grep -Fq '"status": "failed"' "$repo/artifacts/agents/missions/runners/$mission.json"; then
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

make_end_state_contract gate-and-close close-stream
METASYSTEM_AGENT_RUNTIME=fake "$repo/scripts/agents/mission-runner.sh" start \
  --mission gate-and-close --foreground >/dev/null
wait_end_state gate-and-close 10
python3 - "$repo/artifacts/agents/missions/gate-and-close/state.json" <<'PY'
import json,sys
state=json.load(open(sys.argv[1]))
assert state["status"]=="completed" and state["parkReason"] is None and state["gatePassed"] is True, state
assert state["streams"]["primary"]["state"]=="done", state["streams"]
PY

make_end_state_contract runner-closes-chain dispatch-terminal
METASYSTEM_AGENT_RUNTIME=fake "$repo/scripts/agents/mission-runner.sh" start \
  --mission runner-closes-chain --foreground >/dev/null
wait_end_state runner-closes-chain 10
python3 - "$repo" <<'PY'
import json,sys
from pathlib import Path
root=Path(sys.argv[1]); mission=root/"artifacts/agents/missions/runner-closes-chain"
state=json.loads((mission/"state.json").read_text())
record=json.loads((root/"artifacts/agents/jobs/verifier-runner-closes-chain.json").read_text())
assert state["status"]=="completed", state
assert record["chainClosed"] is True and record["runnerClosed"] is True, record
assert record["mirror"] and (Path(record["mirror"]["path"])/"manifest.json").is_file(), record
assert any(item["kind"]=="dispatched" and item["value"]["jobId"]==record["jobId"] for item in state["turnLog"][-1]["accepted"]), state["turnLog"][-1]
PY

# The host adapter is a witness, not a judge (plans/patience-turn-identity.md
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
python3 - "$host_turn/result.json" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
assert value["outcome"] == "completed", value
assert value["sessionId"] == "rotated-session", value
PY

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
python3 - "$host_turn/result-missing.json" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
assert value["outcome"] == "unresumable" and value["sessionId"] is None, value
PY

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

echo "mission contract, state, and runner end-state fixtures passed"
