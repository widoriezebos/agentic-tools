#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$source_root/scripts/agents/fixture-budget.sh"
tmp=$(mktemp -d)
tmp=$(cd "$tmp" && pwd -P)
armed_repo=
passed=()
identity_updater=
rearm_race_pid=
export METASYSTEM_SUPERVISION_REGISTRY_HOME="$tmp/registry-home"
mkdir -p "$METASYSTEM_SUPERVISION_REGISTRY_HOME"

cleanup() {
  local cleanup_engine
  [[ -z "$rearm_race_pid" ]] || { kill "$rearm_race_pid" 2>/dev/null || true; wait "$rearm_race_pid" 2>/dev/null || true; }
  [[ -z "$identity_updater" ]] || { kill "$identity_updater" 2>/dev/null || true; wait "$identity_updater" 2>/dev/null || true; }
  if [[ -n "$armed_repo" && -x "$armed_repo/scripts/agents/arm-supervision.sh" ]]; then
    cleanup_engine=${enrolled_engine:-$ms}
    # The config-refusal leg broadens the runtime list, where fake process
    # tables are no longer authorized. Cleanup uses the real process table.
    METASYSTEM_CENSUS_PROCESS_FILE= METASYSTEM_FAKE_PROCESS_IDENTITY_FILE= \
      METASYSTEM_BIN="$cleanup_engine" \
      "$armed_repo/scripts/agents/arm-supervision.sh" --repo "$armed_repo" --shutdown >&2 \
      || echo "delegate caps fixture cleanup shutdown failed" >&2
  fi
  # Backstop: kill any process still rooted under this run's unique temp
  # dir so a failed assertion cannot leak a child that flakes later runs.
  if [[ -n "$tmp" && "$tmp" == /*/tmp.* ]]; then
    strays=$(pgrep -f "$tmp" 2>/dev/null || true)
    if [[ -n "$strays" ]]; then
      kill -TERM $strays 2>/dev/null || true
      sleep 1
      kill -KILL $(pgrep -f "$tmp" 2>/dev/null || true) 2>/dev/null || true
    fi
  fi
  if [[ -n "${METASYSTEM_KEEP_DELEGATE_CAPS_FIXTURE:-}" ]]; then
    echo "kept delegate-caps fixture: $tmp" >&2
  else
    rm -rf "$tmp"
  fi
}
trap cleanup EXIT

pass_fixture() {
  passed+=("$1")
  echo "$1 passed" >&2
}

ms="${METASYSTEM_BIN:-$source_root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "delegate caps fixtures: binary absent; run the go gate first" >&2; exit 1; }

# AUTH-R2-001..003 RETIRED to internal/mission/fence_test.go
# (script-fixtures-006/D42): the pair-cap selection, narrowing, and
# above-signed refusal are TestAuthorizeCapUsesPairCap and
# TestAuthorizeCapRefusesAboveSigned; the pinned-bytes and
# whitespace-only drift refusals are TestAuthorizeCapRefusesPinned-
# ContractDrift and TestAuthorizeCapRefusesWhitespaceOnlyDrift, ported
# green before this retirement. Pure file-in/refusal-out logic needs no
# processes; the supervision legs below are what this file is for.
for retired in AUTH-R2-001 AUTH-R2-002 AUTH-R2-003; do
  echo "$retired retired: cap authority proven in internal/mission fence tests" >&2
done

# AUTH-R2-004 RETIRED with the python runner: it executed embedded Python and
# monkeypatched mission-contract.py's preflight to
# prove the preflight-to-pin handoff (start pins, a generic preflight gains no
# pin authority, resume re-pins a resealed amendment, an unsigned amendment is
# refused). That embedded python no longer exists; the handoff lives in the
# engine (internal/missionrunner/launch.go pinVerifiedContract, answer.go's
# amendment preflight), the runner's start/park/resume lifecycle is proven end
# to end by the mission-runner process fixtures in validate-metasystem.sh, and
# the pin's enforcement (drift refusal against approvedContractSha256) is
# proven by the fence tests named in the retirement note above.
echo "AUTH-R2-004 retired: preflight-to-pin handoff moved into the engine; see comment" >&2

# AUTH-R2-005..007 run through a real isolated supervision set. The config has
# a 200-minute pair cap and an irrelevant 900-minute mission contract: arming
# must derive 230 from config only, then 330 when --max-cap 300 is declared.
harness=$tmp/supervision
mkdir -p "$harness"
cp -R "$source_root/scripts" "$harness/"
mkdir -p "$harness/development" "$harness/plans" "$harness/bin"
cp "$source_root/bin/metasystem" "$harness/bin/metasystem"
printf 'fixture\n' >"$harness/development/metasystem-design.md"
cat >"$harness/metasystem.conf" <<EOF
metasystem.version=1
metasystem.runtimes=fake
watch.stale-min=20
watch.cap-min=180
watch.interval-sec=1
census.log-max-bytes=1048576
census.max-interval-share-percent=50
dispatch.cap-min=120
dispatch.max-inline-input-kb=64
capability.snapshot-max-age-days=30
cap.min.fake.fake-model=200
evidence.root=$tmp/evidence
role.default.runtime=fake
role.default.model.fake=fake-model
role.implementer.runtime=fake
role.implementer.model.fake=fake-model
dispatch.permissions.implementer=workspace
EOF
cat >"$harness/plans/mission-ignored.contract.md" <<'EOF'
```mission
fence.wall-clock-hours=10
fence.cycles=5
fence.jobs=5
fence.concurrency=2
fence.job-cap-min=120
cap.min.fake.fake-model=900
```
EOF
git -C "$harness" init -q -b main
git -C "$harness" add .
git -C "$harness" -c user.name=fixture -c user.email=fixture.invalid commit -qm fixture
armed_repo=$harness

# Fake-runtime fixtures authorize their disposable enrollment by staging the
# accepted engine identity directly. Keep that engine under the scratch root
# so neither the source checkout nor a person's installation is enrolled.
fixture_install=$tmp/fixture-install
mkdir -p "$fixture_install/bin" "$fixture_install/scripts/agents"
printf 'metasystem.runtimes=fake\n' >"$fixture_install/metasystem.conf"
cp "$ms" "$fixture_install/bin/metasystem"
cp -R "$source_root/scripts/agents/adapters" "$fixture_install/scripts/agents/"
enrolled_engine=$fixture_install/bin/metasystem
enrollment_digest=$("$enrolled_engine" util sha256 --file "$enrolled_engine")
mkdir -p "$harness/artifacts/agents/steward"
printf '{"repoIdentity":"%s","generation":1,"installPath":"%s","installDigest":"sha256:%s","mintedAt":"1970-01-01T00:00:00Z"}\n' \
  "$harness" "$enrolled_engine" "$enrollment_digest" \
  >"$harness/artifacts/agents/steward/identity.json"
chmod 0600 "$harness/artifacts/agents/steward/identity.json"

process_fixture=$harness/process-fixture.json
identity_fixture=$harness/process-identities.json
printf '[]\n' >"$process_fixture"
printf '{}\n' >"$identity_fixture"
export METASYSTEM_CENSUS_PROCESS_FILE="$process_fixture"
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$identity_fixture"
# The fake ps shim retired (script-fixtures-007/D47): every identity
# reader — including the armer since this same change — goes through the
# engine, which honors METASYSTEM_FAKE_PROCESS_IDENTITY_FILE directly.
process_start=$("$harness/bin/metasystem" proc started-at --pid $$)
# The table still holds its initial {} here, so the merged result is this
# shell's one entry. Rename into place, never truncate-in-place: readers
# take no lock, and a reader that catches a torn write sees an empty table.
# The engine treats an unparseable table as "no entry" and falls back to
# the kernel — which un-authenticates this shell's announcement
# mid-classify and lets the ancestry walk escape into the real process
# tree (where a claude/codex CLI ancestor classifies the caller DELEGATE).
identity_staged=$(mktemp "$harness/.identities.XXXXXX")
printf '{"%s":{"pidStartedAt":%s,"pgid":%s,"command":"caps-fixture"}}\n' \
  "$$" "$process_start" "$$" >"$identity_staged"
mv "$identity_staged" "$identity_fixture"
# The 20ms mirror daemon retired (D47): supervision pids change only at
# arm/re-arm, so each arming is followed by ONE explicit registration of
# the published identities — same atomic rename discipline as the tear
# fix, no standing writer racing every read.
register_supervision_identities() {
  local state_file=$harness/artifacts/agents/supervision/state.json
  local entries="" role pid started tag staged
  [[ -f "$state_file" ]] || return 0
  "$ms" util json-validate --file "$state_file" >/dev/null 2>&1 || return 0
  # The table's whole population is this shell plus the identities the
  # current state publishes. Entries a past registration wrote for
  # components a later arming replaced are dead pids the kernel vetoes
  # anyway, so rebuilding from the current state is observably the same as
  # the retired merge — and with the retired mirror daemon gone nothing
  # writes this table concurrently, so its flock retired with it.
  for role in owner components.watcher components.reaper; do
    pid=$("$ms" json get --file "$state_file" --field "$role.pid" 2>/dev/null) || continue
    started=$("$ms" json get --file "$state_file" --field "$role.pidStartedAt" 2>/dev/null) || continue
    tag=$("$ms" json get --file "$state_file" --field "$role.instanceTag" 2>/dev/null) || continue
    [[ "$pid" =~ ^-?[0-9]+$ && "$started" =~ ^-?[0-9]+$ ]] || continue
    case "$tag" in *[\"\\]*) continue ;; esac
    entries="$entries,\"$pid\":{\"pidStartedAt\":$started,\"pgid\":$pid,\"command\":\"fixture $tag\"}"
  done
  # Same atomic rename discipline as the tear fix above.
  staged=$(mktemp "$(dirname "$identity_fixture")/.identities.XXXXXX") || return 0
  printf '{"%s":{"pidStartedAt":%s,"pgid":%s,"command":"caps-fixture"}%s}\n' \
    "$$" "$process_start" "$$" "$entries" >"$staged"
  mv "$staged" "$identity_fixture"
}
"$harness/bin/metasystem" lease announce --root "$harness" \
  --session caps-fixture --pid $$ --start "$process_start" --tag caps-fixture --runtime fake >/dev/null

arm=$harness/scripts/agents/arm-supervision.sh
run_arm() { # description, arm arguments...
  local description=$1 arm_rc
  shift
  set +e
  METASYSTEM_BIN="$enrolled_engine" "$arm" "$@" >&2
  arm_rc=$?
  set -e
  if (( arm_rc != 0 )); then
    echo "delegate caps fixture $description failed (exit status $arm_rc)" >&2
    return "$arm_rc"
  fi
}

report_captured_arm() { # description, output file, status file
  local description=$1 output_file=$2 status_file=$3 arm_status=missing
  [[ ! -f "$status_file" ]] || arm_status=$(cat "$status_file")
  echo "delegate caps fixture $description result (exit status $arm_status)" >&2
  [[ ! -f "$output_file" ]] || cat "$output_file" >&2
}

run_arm "initial arm" --repo "$harness" --session caps-fixture --pid $$ \
  --start-time "$process_start" --tag caps-fixture
register_supervision_identities

state=$harness/artifacts/agents/supervision/state.json
heartbeat=$harness/artifacts/agents/supervision/watcher.heartbeat.json
assert_loaded_cap() { # expected minutes
  [[ "$("$ms" json get --file "$state" --field derivedWatcherCapMin)" == "$1" ]] \
    || { echo "derivedWatcherCapMin is not $1" >&2; cat "$state" >&2; exit 1; }
  [[ "$("$ms" json get --file "$heartbeat" --field loadedCapMin)" == "$1" ]] \
    || { echo "loadedCapMin is not $1" >&2; cat "$heartbeat" >&2; exit 1; }
}
assert_loaded_cap 230

run_arm "raised-cap re-arm" --repo "$harness" --session caps-fixture --pid $$ \
  --start-time "$process_start" --tag caps-fixture --rearm --max-cap 300
assert_loaded_cap 330
pass_fixture AUTH-R2-007

# Hold the cap-authority transaction with a REAL live identity: under the
# owner-lock protocol (script-orchestration-01/D18) a bare ownerless
# directory is garbage by construction and the armer would heal it
# immediately instead of waiting. This fixture process is alive and its
# script name is in its argv, so the armer classifies the holder Alive.
mkdir -p "$harness/artifacts/agents/jobs" "$harness/artifacts/agents/supervision"
"$ms" job owner-lock --command claim --dir "$harness/artifacts/agents/supervision/cap-authority.lock.d" \
  --pid $$ --tag delegate-caps-fixtures.sh
(
  set +e
  METASYSTEM_BIN="$enrolled_engine" "$arm" --repo "$harness" --session caps-fixture --pid $$ --start-time "$process_start" \
    --tag caps-fixture --rearm >"$tmp/downward-rearm.out" 2>&1
  printf '%s\n' "$?" >"$tmp/downward-rearm.status"
) &
rearm_race_pid=$!
sleep 0.2
kill -0 "$rearm_race_pid" 2>/dev/null \
  || { report_captured_arm "downward re-arm" "$tmp/downward-rearm.out" "$tmp/downward-rearm.status"; echo "AUTH-R2-006: re-arm did not serialize behind the cap-authority transaction" >&2; exit 1; }
cat >"$harness/artifacts/agents/jobs/blocking-job.json" <<'EOF'
{"jobId":"blocking-job","status":"running","capMin":400}
EOF
"$ms" job owner-lock --command release --dir "$harness/artifacts/agents/supervision/cap-authority.lock.d" \
  --pid $$ --tag delegate-caps-fixtures.sh
deadline=$((SECONDS + 10))
while kill -0 "$rearm_race_pid" 2>/dev/null; do
  (( SECONDS < deadline )) || { report_captured_arm "downward re-arm" "$tmp/downward-rearm.out" "$tmp/downward-rearm.status"; echo "AUTH-R2-006: serialized re-arm did not finish" >&2; exit 1; }
  sleep 0.05
done
wait "$rearm_race_pid" || true
rearm_race_pid=
report_captured_arm "downward re-arm" "$tmp/downward-rearm.out" "$tmp/downward-rearm.status"
if [[ $(cat "$tmp/downward-rearm.status") -eq 0 ]]; then
  echo "AUTH-R2-006: downward re-arm bypass was accepted" >&2
  exit 1
fi
grep -Fq 'blocking-job' "$tmp/downward-rearm.out" \
  && grep -Fq 'reserved cap 400m' "$tmp/downward-rearm.out" \
  || { cat "$tmp/downward-rearm.out" >&2; exit 1; }
pass_fixture AUTH-R2-006

"$ms" json set --file "$harness/artifacts/agents/jobs/blocking-job.json" --field status=completed

# Raise only the derived state field, then let the live watcher publish a fresh
# successful census over that state while continuing to attest its actually
# loaded 330-minute ceiling. A transient CENSUS-FAILED verdict can replace the
# successful census before dispatch reads it, so re-attest and retry only that
# pre-reservation refusal under the same deadline.
"$ms" json set --file "$state" --int derivedWatcherCapMin=999

brief=$tmp/cap-brief.md
cat >"$brief" <<'EOF'
Working Mode: build

Attempt a cap above the loaded watcher ceiling.
EOF

census=$harness/artifacts/agents/supervision/last-census.json
# The live watcher republishes the census by atomic rename, so one cp gives
# a consistent snapshot and the verdict and digest below bind to the same
# publication the way the retired one-shot reader did.
attested_census_over_state() { # census file, state file
  local digest verdict attested snap="$tmp/attested-census.snapshot.json"
  digest=$("$ms" util sha256 --file "$2" 2>/dev/null) || return 1
  cp "$1" "$snap" 2>/dev/null || return 1
  verdict=$("$ms" json get --file "$snap" --field verdict 2>/dev/null) || return 1
  attested=$("$ms" json get --file "$snap" --field stateDigest 2>/dev/null) || return 1
  [[ "$verdict" == SUCCESS && "$attested" == "$digest" ]]
}
deadline=$((SECONDS + 10))
while true; do
  until attested_census_over_state "$census" "$state"
  do
    (( SECONDS < deadline )) \
      || { echo "AUTH-R2-005: watcher did not publish a successful attestation of mutated state" >&2; exit 1; }
    sleep 0.05
  done

  if {
    mkdir -p "$harness/docs"
    cp "$source_root/docs/orchestration.md" "$harness/docs/orchestration.md"
    METASYSTEM_DELEGATE_ROOT="$harness" METASYSTEM_BIN="$enrolled_engine" \
        METASYSTEM_CAP_MIN_IMPLEMENTER_FAKE_FAKE_MODEL=500 \
        METASYSTEM_DISPATCH_PERMISSIONS_IMPLEMENTER=none \
        "$enrolled_engine" delegate --role implementer --brief "$brief" \
          --goal none-explicit --destructive-reach MECHANICAL --op attested-ceiling \
          >"$tmp/attested.out" 2>&1
  }; then
    echo "AUTH-R2-005: dispatch trusted raised supervision state over watcher attestation" >&2
    exit 1
  fi
  if grep -Fq '"outcome":"REFUSED-INTERNAL"' "$tmp/attested.out" \
      && grep -Fq "\"detail\":\"dispatch cap 500m must stay below the live watcher's attested 330m ceiling" "$tmp/attested.out"; then
    break
  fi
  if grep -Fq 'dispatch refused: last census verdict is CENSUS-FAILED' "$tmp/attested.out" \
      && grep -Fq '"outcome":"REFUSED-INTERNAL"' "$tmp/attested.out"; then
    (( SECONDS < deadline )) \
      || { echo "AUTH-R2-005: transient CENSUS-FAILED refusal did not recover before the deadline" >&2; cat "$tmp/attested.out" >&2; exit 1; }
    sleep 0.05
    continue
  fi
  cat "$tmp/attested.out" >&2
  exit 1
done
pass_fixture AUTH-R2-005

# An ordinary establishing arm is also a replacement authority. Race a new
# reservation into its lock wait after the prior watcher is stopped; it must
# refuse the lower config-derived ceiling instead of silently taking over.
run_arm "shutdown before ordinary establishment" --repo "$harness" --shutdown
register_supervision_identities
# Same live-identity hold as the re-arm leg above (D18).
"$ms" job owner-lock --command claim --dir "$harness/artifacts/agents/supervision/cap-authority.lock.d" \
  --pid $$ --tag delegate-caps-fixtures.sh
(
  set +e
  METASYSTEM_BIN="$enrolled_engine" "$arm" --repo "$harness" --session caps-fixture --pid $$ --start-time "$process_start" \
    --tag caps-fixture >"$tmp/ordinary-establish.out" 2>&1
  printf '%s\n' "$?" >"$tmp/ordinary-establish.status"
) &
rearm_race_pid=$!
sleep 0.2
kill -0 "$rearm_race_pid" 2>/dev/null \
  || { report_captured_arm "ordinary establishment" "$tmp/ordinary-establish.out" "$tmp/ordinary-establish.status"; echo "AUTH-R2-006: ordinary establishment did not serialize behind the cap-authority transaction" >&2; exit 1; }
cat >"$harness/artifacts/agents/jobs/ordinary-blocking-job.json" <<'EOF'
{"jobId":"ordinary-blocking-job","status":"running","capMin":400}
EOF
"$ms" job owner-lock --command release --dir "$harness/artifacts/agents/supervision/cap-authority.lock.d" \
  --pid $$ --tag delegate-caps-fixtures.sh
deadline=$((SECONDS + 10))
while kill -0 "$rearm_race_pid" 2>/dev/null; do
  (( SECONDS < deadline )) || { report_captured_arm "ordinary establishment" "$tmp/ordinary-establish.out" "$tmp/ordinary-establish.status"; echo "AUTH-R2-006: serialized ordinary establishment did not finish" >&2; exit 1; }
  sleep 0.05
done
wait "$rearm_race_pid" || true
rearm_race_pid=
report_captured_arm "ordinary establishment" "$tmp/ordinary-establish.out" "$tmp/ordinary-establish.status"
if [[ $(cat "$tmp/ordinary-establish.status") -eq 0 ]]; then
  echo "AUTH-R2-006: ordinary downward establishment bypass was accepted" >&2
  exit 1
fi
grep -Fq 'ordinary-blocking-job' "$tmp/ordinary-establish.out" \
  && grep -Fq 'reserved cap 400m' "$tmp/ordinary-establish.out" \
  || { cat "$tmp/ordinary-establish.out" >&2; exit 1; }

# AUTH-R2-008 attacks the local override layer with the exact noncanonical key.
conf_edit "$harness/metasystem.conf" replace-line-first '^metasystem[.]runtimes=fake$' 'metasystem.runtimes=fake,devin'
printf 'cap.min.devin.swe-1.7=250\n' >"$harness/metasystem.conf.local"
if "$harness/scripts/metasystem-config.sh" validate >"$tmp/noncanonical.out" 2>&1; then
  echo "AUTH-R2-008: noncanonical local cap key was accepted" >&2
  exit 1
fi
grep -Fq 'cap.min.devin.swe-1.7' "$tmp/noncanonical.out" \
  && grep -Fq 'cap.min.devin.swe-1-7' "$tmp/noncanonical.out" \
  || { cat "$tmp/noncanonical.out" >&2; exit 1; }
pass_fixture AUTH-R2-008

# AUTH-R2-009 attacks proof completeness itself: removing one named fixture
# must fail the registry check, while the actually executed registry is exact.
# AUTH-R2-004 retired to the engine (see its comment above).
registry_is_complete() { # executed fixture ids, order-free and deduplicated
  [[ "$(printf '%s\n' "$@" | sort -u)" == "$(printf '%s\n' AUTH-R2-005 AUTH-R2-006 AUTH-R2-007 AUTH-R2-008)" ]]
}
if registry_is_complete AUTH-R2-005 AUTH-R2-006; then
  echo "AUTH-R2-009: incomplete fixture registry was accepted" >&2
  exit 1
fi
registry_is_complete "${passed[@]}" \
  || { echo "AUTH-R2-009: executed registry is not exact:" "${passed[@]}" >&2; exit 1; }
pass_fixture AUTH-R2-009

echo "delegate caps authority fixtures passed" >&2
