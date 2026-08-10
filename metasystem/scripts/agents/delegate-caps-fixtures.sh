#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d)
armed_repo=
passed=()
identity_updater=
rearm_race_pid=

cleanup() {
  [[ -z "$rearm_race_pid" ]] || { kill "$rearm_race_pid" 2>/dev/null || true; wait "$rearm_race_pid" 2>/dev/null || true; }
  [[ -z "$identity_updater" ]] || { kill "$identity_updater" 2>/dev/null || true; wait "$identity_updater" 2>/dev/null || true; }
  if [[ -n "$armed_repo" && -x "$armed_repo/scripts/agents/arm-supervision.sh" ]]; then
    "$armed_repo/scripts/agents/arm-supervision.sh" --repo "$armed_repo" --shutdown >/dev/null 2>&1 || true
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

authority_repo=$tmp/authority
mkdir -p "$authority_repo/scripts/agents" "$authority_repo/plans" "$authority_repo/artifacts/agents/missions"

# Install a signed-enough contract: the fence's cap authority reads the raw
# bytes pinned in fences.json (approvedContractSha256), so the fixture pins
# exactly what it writes.
caps_contract() { # pair cap, fallback cap, wall hours
  printf '```mission\nfence.wall-clock-hours=%s\nfence.cycles=20\nfence.jobs=50\nfence.concurrency=50\nfence.job-cap-min=%s\ncap.min.devin.swe-1-7=%s\n```\nApproval: name=Fixture; date=2026-08-09; contract-sha256=%s\n' \
    "$3" "$2" "$1" "$(printf '0%.0s' {1..64})"
}
caps_install() { # mission, contract bytes on stdin
  local mission=$1 contract="$authority_repo/plans/mission-$1.contract.md"
  cat >"$contract"
  mkdir -p "$authority_repo/artifacts/agents/missions/$mission"
  printf '{"schemaVersion":1,"missionId":"%s","startedAt":"%s","cycles":0,"reservations":{},"approvedContractSha256":"%s"}\n' \
    "$mission" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(shasum -a 256 "$contract" | awk '{print $1}')" \
    >"$authority_repo/artifacts/agents/missions/$mission/fences.json"
}
authorize_cli() { # mission, job, [requested]
  local mission=$1 job=$2 requested=${3:-}
  if [[ -n "$requested" ]]; then
    "$ms" mission-fence authorize-cap --repo "$authority_repo" --mission "$mission" \
      --job "$job" --runtime devin --model swe-1-7 --requested "$requested"
  else
    "$ms" mission-fence authorize-cap --repo "$authority_repo" --mission "$mission" \
      --job "$job" --runtime devin --model swe-1-7
  fi
}

# AUTH-R2-001: omitted selects the signed pair cap, a lower argument narrows,
# and the adversarial request above the signed cap is refused.
caps_install interface < <(caps_contract 150 120 10)
selected=$(authorize_cli interface selected)
python3 - "$selected" <<'PY'
import json, sys
value = json.loads(sys.argv[1])
assert set(value) == {"capMin", "capDeadline", "source"}, value
assert value["capMin"] == 150 and value["source"]["rule"] == "contract-pair", value
PY
if raised_err=$(authorize_cli interface raised 200 2>&1); then
  echo "AUTH-R2-001: requested-above-signed bypass was accepted" >&2; exit 1
fi
grep -Fq 'mission fence refused requested cap 200m above signed' <<<"$raised_err" \
  || { echo "AUTH-R2-001: refusal did not name the signed bound: $raised_err" >&2; exit 1; }
narrowed=$(authorize_cli interface narrowed 90)
python3 - "$narrowed" <<'PY'
import json, sys
value = json.loads(sys.argv[1])
assert set(value) == {"capMin", "capDeadline", "source"}, value
assert value["capMin"] == 90 and value["source"]["rule"] == "argument", value
PY
pass_fixture AUTH-R2-001

# AUTH-R2-002: the pinned bytes are the authority. After one successful
# authorization, swapping the live contract for a higher-cap variant must be
# refused on the next call. (The in-transaction single-buffer-read atomicity
# the python leg injected into via after_buffer_read is an engine internal
# now — internal/mission/fence.go reads the contract once into a buffer —
# and cannot be reached from outside the process; the observable drift
# refusal is what this leg keeps.)
caps_install transaction < <(caps_contract 150 120 10)
buffered=$(authorize_cli transaction buffer-wins)
[[ $("$ms" json get --value "$buffered" --field capMin) == 150 ]] \
  || { echo "AUTH-R2-002: pinned contract did not authorize its signed cap" >&2; exit 1; }
caps_contract 999 120 10 >"$authority_repo/plans/mission-transaction.contract.md"
if drift_err=$(authorize_cli transaction drift-refused 2>&1); then
  echo "AUTH-R2-002: changed pinned bytes were accepted on the next call" >&2; exit 1
fi
grep -Fq 'does not match approvedContractSha256' <<<"$drift_err" \
  || { echo "AUTH-R2-002: drift refusal did not name the pin: $drift_err" >&2; exit 1; }
pass_fixture AUTH-R2-002

# AUTH-R2-003: raw-file hashing detects bytes the canonical approval digest
# deliberately ignores, including a trailing-whitespace-only edit.
caps_install raw-hash < <(caps_contract 150 120 10)
printf '  \n' >>"$authority_repo/plans/mission-raw-hash.contract.md"
if ws_err=$(authorize_cli raw-hash whitespace 2>&1); then
  echo "AUTH-R2-003: trailing-whitespace drift was accepted" >&2; exit 1
fi
grep -Fq 'does not match approvedContractSha256' <<<"$ws_err" \
  || { echo "AUTH-R2-003: whitespace refusal did not name the pin: $ws_err" >&2; exit 1; }
pass_fixture AUTH-R2-003

# AUTH-R2-004 RETIRED with the python runner: it executed the python embedded
# in mission-runner.sh and monkeypatched mission-contract.py's preflight to
# prove the preflight-to-pin handoff (start pins, a generic preflight gains no
# pin authority, resume re-pins a resealed amendment, an unsigned amendment is
# refused). That embedded python no longer exists; the handoff lives in the
# engine (internal/missionrunner/launch.go pinVerifiedContract, answer.go's
# amendment preflight), the runner's start/park/resume lifecycle is proven end
# to end by the mission-runner process fixtures in validate-metasystem.sh, and
# the pin's enforcement (drift refusal against approvedContractSha256) is
# proven at the CLI by AUTH-R2-002/003 above.
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
git -C "$harness" init -q
git -C "$harness" add .
git -C "$harness" -c user.name=fixture -c user.email=fixture.invalid commit -qm fixture
armed_repo=$harness

process_fixture=$harness/process-fixture.json
identity_fixture=$harness/process-identities.json
printf '[]\n' >"$process_fixture"
printf '{}\n' >"$identity_fixture"
export METASYSTEM_CENSUS_PROCESS_FILE="$process_fixture"
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$identity_fixture"
fake_bin=$harness/fixture-bin
mkdir -p "$fake_bin"
cat >"$fake_bin/ps" <<'PY'
#!/usr/bin/env python3
import datetime, json, os, sys
from pathlib import Path
args = sys.argv[1:]
try:
    pid = args[args.index("-p") + 1]
    value = json.loads(Path(os.environ["METASYSTEM_FAKE_PROCESS_IDENTITY_FILE"]).read_text())[pid]
except (ValueError, KeyError, OSError, json.JSONDecodeError):
    raise SystemExit(1)
command = value.get("command", "fake-process-identity")
started = int(value.get("pidStartedAt", value.get("started", 1)))
lstart = datetime.datetime.fromtimestamp(started).strftime("%a %b %d %H:%M:%S %Y")
format_arg = args[args.index("-o") + 1] if "-o" in args else ""
if "lstart=" in format_arg and "command=" in format_arg:
    print(f"{lstart} {command}")
elif "lstart=" in format_arg:
    print(lstart)
elif "ppid=" in format_arg:
    print(f"1 {pid} {command}")
else:
    print(command)
PY
chmod +x "$fake_bin/ps"
export PATH="$fake_bin:$PATH"
process_start=$("$harness/bin/metasystem" identity started-at --pid $$)
python3 - "$identity_fixture" "$$" "$process_start" <<'PY'
import json, sys
from pathlib import Path
path, pid, started = Path(sys.argv[1]), sys.argv[2], int(sys.argv[3])
value = json.loads(path.read_text())
# The engine reads start times natively and no longer registers callers in
# the fixture file the way the python helper's restricted-CI fallback did,
# so create the entry rather than updating one.
value.setdefault(pid, {}).update({"started": started, "pidStartedAt": started, "pgid": int(pid), "command": "caps-fixture"})
path.write_text(json.dumps(value) + "\n")
PY
( while true; do
    python3 - "$identity_fixture" "$harness/artifacts/agents/supervision/state.json" <<'PY' || true
import fcntl, json, sys
from pathlib import Path
identities, state_path = map(Path, sys.argv[1:])
if not state_path.is_file(): raise SystemExit(0)
try: state = json.loads(state_path.read_text())
except (OSError, ValueError): raise SystemExit(0)
lock_path = identities.with_suffix(identities.suffix + ".lock")
with lock_path.open("a+") as lock:
    fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
    try: values = json.loads(identities.read_text())
    except (OSError, ValueError): values = {}
    for item in [state.get("owner"), *state.get("components", {}).values()]:
        if not isinstance(item, dict): continue
        pid, started, tag = item.get("pid"), item.get("pidStartedAt"), item.get("instanceTag")
        if type(pid) is int and type(started) is int and isinstance(tag, str):
            values[str(pid)] = {
                "started": started, "pidStartedAt": started,
                "pgid": pid, "command": f"fixture {tag}",
            }
    identities.write_text(json.dumps(values) + "\n")
PY
    sleep 0.02
  done ) &
identity_updater=$!
"$harness/bin/metasystem" lease announce --root "$harness" \
  --session caps-fixture --pid $$ --start "$process_start" --tag caps-fixture --runtime fake >/dev/null

arm=$harness/scripts/agents/arm-supervision.sh
$arm --repo "$harness" --session caps-fixture --pid $$ --start-time "$process_start" --tag caps-fixture >/dev/null

state=$harness/artifacts/agents/supervision/state.json
heartbeat=$harness/artifacts/agents/supervision/watcher.heartbeat.json
python3 - "$state" "$heartbeat" <<'PY'
import json, sys
state = json.load(open(sys.argv[1]))
heartbeat = json.load(open(sys.argv[2]))
assert state["derivedWatcherCapMin"] == 230, state
assert heartbeat["loadedCapMin"] == 230, heartbeat
PY

$arm --repo "$harness" --session caps-fixture --pid $$ --start-time "$process_start" \
  --tag caps-fixture --rearm --max-cap 300 >/dev/null
python3 - "$state" "$heartbeat" <<'PY'
import json, sys
state = json.load(open(sys.argv[1]))
heartbeat = json.load(open(sys.argv[2]))
assert state["derivedWatcherCapMin"] == 330, state
assert heartbeat["loadedCapMin"] == 330, heartbeat
PY
pass_fixture AUTH-R2-007

mkdir -p "$harness/artifacts/agents/jobs" "$harness/artifacts/agents/supervision/cap-authority.lock.d"
(
  set +e
  $arm --repo "$harness" --session caps-fixture --pid $$ --start-time "$process_start" \
    --tag caps-fixture --rearm >"$tmp/downward-rearm.out" 2>&1
  printf '%s\n' "$?" >"$tmp/downward-rearm.status"
) &
rearm_race_pid=$!
sleep 0.2
kill -0 "$rearm_race_pid" 2>/dev/null \
  || { echo "AUTH-R2-006: re-arm did not serialize behind the cap-authority transaction" >&2; exit 1; }
cat >"$harness/artifacts/agents/jobs/blocking-job.json" <<'EOF'
{"jobId":"blocking-job","status":"running","capMin":400}
EOF
rmdir "$harness/artifacts/agents/supervision/cap-authority.lock.d"
deadline=$((SECONDS + 10))
while kill -0 "$rearm_race_pid" 2>/dev/null; do
  (( SECONDS < deadline )) || { echo "AUTH-R2-006: serialized re-arm did not finish" >&2; exit 1; }
  sleep 0.05
done
wait "$rearm_race_pid" || true
rearm_race_pid=
if [[ $(cat "$tmp/downward-rearm.status") -eq 0 ]]; then
  echo "AUTH-R2-006: downward re-arm bypass was accepted" >&2
  exit 1
fi
grep -Fq 'blocking-job' "$tmp/downward-rearm.out" \
  && grep -Fq 'reserved cap 400m' "$tmp/downward-rearm.out" \
  || { cat "$tmp/downward-rearm.out" >&2; exit 1; }
pass_fixture AUTH-R2-006

python3 - "$harness/artifacts/agents/jobs/blocking-job.json" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["status"] = "completed"
path.write_text(json.dumps(value) + "\n")
PY

# Raise only the derived state field, then let the live watcher publish a fresh
# successful census over that state while continuing to attest its actually
# loaded 330-minute ceiling. A transient CENSUS-FAILED verdict can replace the
# successful census before dispatch reads it, so re-attest and retry only that
# pre-reservation refusal under the same deadline.
python3 - "$state" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["derivedWatcherCapMin"] = 999
path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY

brief=$tmp/cap-brief.md
cat >"$brief" <<'EOF'
Working Mode: build

Attempt a cap above the loaded watcher ceiling.
EOF

census=$harness/artifacts/agents/supervision/last-census.json
deadline=$((SECONDS + 10))
while true; do
  until python3 - "$state" "$census" <<'PY'
import hashlib, json, sys
from pathlib import Path
state = Path(sys.argv[1]).read_bytes()
try: census = json.loads(Path(sys.argv[2]).read_text())
except (OSError, ValueError): raise SystemExit(1)
attests_state = census.get("stateDigest") == hashlib.sha256(state).hexdigest()
raise SystemExit(0 if census.get("verdict") == "SUCCESS" and attests_state else 1)
PY
  do
    (( SECONDS < deadline )) \
      || { echo "AUTH-R2-005: watcher did not publish a successful attestation of mutated state" >&2; exit 1; }
    sleep 0.05
  done

  if "$harness/scripts/agents/dispatch.sh" dispatch --role implementer --brief "$brief" \
      --job-id attested-ceiling --cap-min 500 >"$tmp/attested.out" 2>&1; then
    echo "AUTH-R2-005: dispatch trusted raised supervision state over watcher attestation" >&2
    exit 1
  fi
  if grep -Fq "live watcher's attested 330m ceiling" "$tmp/attested.out"; then
    break
  fi
  if grep -Fq 'dispatch refused: last census verdict is CENSUS-FAILED' "$tmp/attested.out"; then
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
$arm --repo "$harness" --shutdown >/dev/null
mkdir "$harness/artifacts/agents/supervision/cap-authority.lock.d"
(
  set +e
  $arm --repo "$harness" --session caps-fixture --pid $$ --start-time "$process_start" \
    --tag caps-fixture >"$tmp/ordinary-establish.out" 2>&1
  printf '%s\n' "$?" >"$tmp/ordinary-establish.status"
) &
rearm_race_pid=$!
sleep 0.2
kill -0 "$rearm_race_pid" 2>/dev/null \
  || { echo "AUTH-R2-006: ordinary establishment did not serialize behind the cap-authority transaction" >&2; exit 1; }
cat >"$harness/artifacts/agents/jobs/ordinary-blocking-job.json" <<'EOF'
{"jobId":"ordinary-blocking-job","status":"running","capMin":400}
EOF
rmdir "$harness/artifacts/agents/supervision/cap-authority.lock.d"
deadline=$((SECONDS + 10))
while kill -0 "$rearm_race_pid" 2>/dev/null; do
  (( SECONDS < deadline )) || { echo "AUTH-R2-006: serialized ordinary establishment did not finish" >&2; exit 1; }
  sleep 0.05
done
wait "$rearm_race_pid" || true
rearm_race_pid=
if [[ $(cat "$tmp/ordinary-establish.status") -eq 0 ]]; then
  echo "AUTH-R2-006: ordinary downward establishment bypass was accepted" >&2
  exit 1
fi
grep -Fq 'ordinary-blocking-job' "$tmp/ordinary-establish.out" \
  && grep -Fq 'reserved cap 400m' "$tmp/ordinary-establish.out" \
  || { cat "$tmp/ordinary-establish.out" >&2; exit 1; }

# AUTH-R2-008 attacks the local override layer with the exact noncanonical key.
perl -0pi -e 's/^metasystem\.runtimes=fake$/metasystem.runtimes=fake,devin/m' "$harness/metasystem.conf"
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
if python3 - AUTH-R2-001 AUTH-R2-002 AUTH-R2-003 AUTH-R2-005 AUTH-R2-006 <<'PY'
import sys
expected = {f"AUTH-R2-{index:03d}" for index in (1, 2, 3, 5, 6, 7, 8)}
actual = set(sys.argv[1:])
raise SystemExit(0 if actual == expected else 1)
PY
then
  echo "AUTH-R2-009: incomplete fixture registry was accepted" >&2
  exit 1
fi
python3 - "${passed[@]}" <<'PY'
import sys
# AUTH-R2-004 retired to the engine (see its comment above).
expected = {f"AUTH-R2-{index:03d}" for index in (1, 2, 3, 5, 6, 7, 8)}
actual = set(sys.argv[1:])
assert actual == expected, (sorted(actual), sorted(expected))
PY
pass_fixture AUTH-R2-009

echo "delegate caps authority fixtures passed" >&2
