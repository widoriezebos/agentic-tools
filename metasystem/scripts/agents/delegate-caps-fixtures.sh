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

authority_repo=$tmp/authority
mkdir -p "$authority_repo/scripts/agents" "$authority_repo/plans" "$authority_repo/artifacts/agents/missions"
cp "$source_root/scripts/agents/canonical-model.py" "$authority_repo/scripts/agents/"

python3 - "$source_root/scripts/agents/mission-fence.py" "$authority_repo" <<'PY'
import hashlib
import importlib.util
import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from types import SimpleNamespace

module_path, repo = Path(sys.argv[1]), Path(sys.argv[2])
specification = importlib.util.spec_from_file_location("delegate_caps_mission_fence", module_path)
module = importlib.util.module_from_spec(specification)
sys.modules[specification.name] = module
specification.loader.exec_module(module)

def contract(pair=150, fallback=120, wall="10"):
    return (
        "```mission\n"
        f"fence.wall-clock-hours={wall}\n"
        "fence.cycles=20\n"
        "fence.jobs=50\n"
        "fence.concurrency=50\n"
        f"fence.job-cap-min={fallback}\n"
        f"cap.min.devin.swe-1-7={pair}\n"
        "```\n"
        "Approval: name=Fixture; date=2026-08-09; contract-sha256=" + "0" * 64 + "\n"
    ).encode()

def install(mission, raw):
    contract_path = repo / "plans" / f"mission-{mission}.contract.md"
    contract_path.write_bytes(raw)
    directory = repo / "artifacts" / "agents" / "missions" / mission
    directory.mkdir(parents=True, exist_ok=True)
    (directory / "fences.json").write_text(json.dumps({
        "schemaVersion": 1,
        "missionId": mission,
        "startedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "cycles": 0,
        "reservations": {},
        "approvedContractSha256": hashlib.sha256(raw).hexdigest(),
    }) + "\n")
    return contract_path

def args(mission, job, requested=None):
    return SimpleNamespace(
        repo=repo, mission=mission, job=job, runtime="devin", model="swe-1-7", requested=requested
    )

def authorize_cli(mission, job, requested=None):
    command = [
        sys.executable, str(module_path), "authorize-cap", "--repo", str(repo),
        "--mission", mission, "--job", job, "--runtime", "devin", "--model", "swe-1-7",
    ]
    if requested is not None:
        command += ["--requested", str(requested)]
    return subprocess.run(command, text=True, capture_output=True, check=False)

# AUTH-R2-001: omitted selects the signed pair cap, a lower argument narrows,
# and the adversarial request above the signed cap is refused.
install("interface", contract())
selected_result = authorize_cli("interface", "selected")
assert selected_result.returncode == 0, selected_result.stderr
selected = json.loads(selected_result.stdout)
assert set(selected) == {"capMin", "capDeadline", "source"}, selected
assert selected["capMin"] == 150 and selected["source"]["rule"] == "contract-pair", selected
raised = authorize_cli("interface", "raised", 200)
assert raised.returncode != 0, "requested-above-signed bypass was accepted"
assert "mission fence refused requested cap 200m above signed" in raised.stderr, raised.stderr
narrowed_result = authorize_cli("interface", "narrowed", 90)
assert narrowed_result.returncode == 0, narrowed_result.stderr
narrowed = json.loads(narrowed_result.stdout)
assert set(narrowed) == {"capMin", "capDeadline", "source"}, narrowed
assert narrowed["capMin"] == 90 and narrowed["source"]["rule"] == "argument", narrowed

# AUTH-R2-002: swap the path after the one buffer read. The first reservation
# must use the buffered 150-minute contract; the next call must reject drift.
original = contract(150)
swapped = contract(999)
path = install("transaction", original)
result = module.authorize_cap_transaction(
    args("transaction", "buffer-wins"), after_buffer_read=lambda: path.write_bytes(swapped)
)
assert result["capMin"] == 150, result
try:
    module.authorize_cap_transaction(args("transaction", "drift-refused"))
except module.FenceError as error:
    assert "does not match approvedContractSha256" in str(error), error
else:
    raise AssertionError("changed pinned bytes were accepted on the next call")

# AUTH-R2-003: raw-file hashing detects bytes the canonical approval digest
# deliberately ignores, including a trailing-whitespace-only edit.
path = install("raw-hash", contract())
path.write_bytes(path.read_bytes() + b"  \n")
try:
    module.authorize_cap_transaction(args("raw-hash", "whitespace"))
except module.FenceError as error:
    assert "does not match approvedContractSha256" in str(error), error
else:
    raise AssertionError("trailing-whitespace drift was accepted")
PY
pass_fixture AUTH-R2-001
pass_fixture AUTH-R2-002
pass_fixture AUTH-R2-003

# AUTH-R2-004 exercises the production runner's preflight-to-pin handoff. The
# fixture replaces only preflight's unrelated gate/origin/supervision legs; its
# approval check, raw-byte output, runner pin writer, and resume replacement are
# the production implementations.
python3 - "$source_root/scripts/agents/mission-runner.sh" "$source_root/scripts/agents/mission-contract.py" "$tmp/repin" <<'PY'
import contextlib
import hashlib
import importlib.util
import io
import json
import os
import subprocess
import sys
from pathlib import Path
from types import SimpleNamespace

runner_path, contract_module_path, root = map(Path, sys.argv[1:])
(root / "plans").mkdir(parents=True)
(root / "scripts" / "agents").mkdir(parents=True)
(root / "scripts").mkdir(exist_ok=True)
(root / "truth").mkdir()
(root / "docs").mkdir()
for source in (contract_module_path, contract_module_path.with_name("canonical-model.py")):
    (root / "scripts" / "agents" / source.name).write_bytes(source.read_bytes())
(root / "docs" / "project-rules.md").write_bytes(
    runner_path.parents[2].joinpath("docs/project-rules.md").read_bytes()
)
(root / "scripts" / "gate.sh").write_text("#!/usr/bin/env bash\nprintf 'metric=score=1\\n'\n")
os.chmod(root / "scripts" / "gate.sh", 0o755)
(root / "truth" / "reference.txt").write_text("certified truth\n")
subprocess.run(["git", "init", "-q", "-b", "main", str(root)], check=True)
subprocess.run(["git", "-C", str(root), "config", "user.name", "fixture"], check=True)
subprocess.run(["git", "-C", str(root), "config", "user.email", "fixture.invalid"], check=True)
subprocess.run(["git", "-C", str(root), "add", "docs", "scripts", "truth"], check=True)
subprocess.run(["git", "-C", str(root), "commit", "-qm", "fixture instruments"], check=True)

runner_source = runner_path.read_text(encoding="utf-8")
embedded = runner_source.split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
embedded = embedded.rsplit("raise SystemExit(main())", 1)[0]
namespace = {"__name__": "delegate_caps_runner_fixture"}
saved_argv = sys.argv
sys.argv = ["mission-runner-fixture", str(root), str(runner_path)]
try:
    exec(compile(embedded, str(runner_path), "exec"), namespace)
finally:
    sys.argv = saved_argv

specification = importlib.util.spec_from_file_location("delegate_caps_contract", contract_module_path)
contract_module = importlib.util.module_from_spec(specification)
sys.modules[specification.name] = contract_module
specification.loader.exec_module(contract_module)
contract_module.verify_origin = lambda _contract, _repo: None
contract_module.verify_supervision = lambda _project_root: None

mission = "repin"
contract_path = root / "plans" / "mission-repin.contract.md"

def signed(cap):
    authored = (
        "# Intent\n\nExercise the production preflight-to-pin handoff.\n\n"
        "# Non-goals\n\nDo not publish anything.\n\n"
        "# Initial streams\n\nComplete one fixture stream.\n\n"
        "```mission\n"
        "gate.command=scripts/gate.sh\n"
        "gate.ref=HEAD\n"
        "gate.paths=scripts/gate.sh\n"
        "truth.paths=truth/reference.txt\n"
        "truth.certification=certified\n"
        "gate.direction=max\n"
        "gate.threshold.score=>=1\n"
        "gate.noise-floor.score=0\n"
        "guard.score.command=scripts/gate.sh\n"
        "guard.score.floor=1\n"
        "guard.score.noise=0\n"
        "guard.cadence=1\n"
        "ledger.cycle-budget=5\n"
        "ledger.no-gain-budget=2\n"
        "fence.wall-clock-hours=4\n"
        "fence.cycles=5\n"
        "fence.jobs=5\n"
        "fence.concurrency=2\n"
        "fence.job-cap-min=120\n"
        "host.runtime=devin\n"
        "host.model=swe-1-7\n"
        "host.turn-cap-min=120\n"
        "stream.primary=Complete the fixture.\n"
        "envelope.dependencies=jq\n"
        "exposure=EUR:1\n"
        f"cap.min.devin.swe-1-7={cap}\n"
        "```\n"
    )
    contract_path.write_text(authored)
    contract = contract_module.read_contract(contract_path)
    contract_module.validate_contract(contract, root)
    digest = contract_module.seal_contract(contract, root, root)
    with contract_path.open("a", encoding="utf-8") as handle:
        handle.write(f"\nApproval: name=Fixture; date=2026-08-09; contract-sha256={digest}\n")
    return contract_path.read_bytes()

real_run_command = namespace["run_command"]

def fixture_run(command, **kwargs):
    if command[0].endswith("arm-supervision.sh"):
        return subprocess.CompletedProcess(command, 0, "ARMED\n", "")
    if command[0].endswith("mission-contract.py") and command[1] == "preflight":
        stdout, stderr = io.StringIO(), io.StringIO()
        saved = sys.argv
        sys.argv = [str(contract_module_path), *command[1:]]
        try:
            with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                status = contract_module.main()
        finally:
            sys.argv = saved
        return subprocess.CompletedProcess(command, status, stdout.getvalue(), stderr.getvalue())
    return real_run_command(command, **kwargs)

namespace["arming_identity"] = lambda _mission: ("fixture", 1, 1, "fixture", None)
namespace["run_command"] = fixture_run

first = signed(150)
contract_path.write_bytes(first)
namespace["arm_and_preflight"] = namespace["arm_and_preflight"]
namespace["arm_and_preflight"](mission, "start")
fences_path = root / "artifacts" / "agents" / "missions" / mission / "fences.json"
fences = json.loads(fences_path.read_text())
first_pin = fences["approvedContractSha256"]
assert first_pin == hashlib.sha256(first).hexdigest()

amended = signed(180)
contract_path.write_bytes(amended)
generic_snapshot = root / "generic-preflight.contract.md"
generic = fixture_run([
    str(contract_module_path), "preflight", "--file", str(contract_path),
    "--verified-bytes-output", str(generic_snapshot),
])
assert generic.returncode == 0, generic.stderr
assert generic_snapshot.read_bytes() == amended
assert json.loads(fences_path.read_text())["approvedContractSha256"] == first_pin, (
    "generic preflight acquired re-pin authority"
)
namespace["arm_and_preflight"](mission, "resume")
fences = json.loads(fences_path.read_text())
assert fences["approvedContractSha256"] == hashlib.sha256(amended).hexdigest()
assert fences["approvedContractSha256"] != first_pin
assert "```mission-seal\n" in amended.decode(), "amendment was not resealed"

fence_path = runner_path.with_name("mission-fence.py")
fence_specification = importlib.util.spec_from_file_location("delegate_caps_repin_fence", fence_path)
fence_module = importlib.util.module_from_spec(fence_specification)
sys.modules[fence_specification.name] = fence_module
fence_specification.loader.exec_module(fence_module)
authorized = fence_module.authorize_cap_transaction(SimpleNamespace(
    repo=root, mission=mission, job="repinned-cap", runtime="devin", model="swe-1-7", requested=None,
))
assert authorized["capMin"] == 180, authorized

unsigned = signed(240).rsplit(b"\nApproval:", 1)[0] + b"\n"
contract_path.write_bytes(unsigned)
try:
    namespace["arm_and_preflight"](mission, "resume")
except namespace["RunnerError"] as error:
    assert "contract is unsigned" in str(error), error
else:
    raise AssertionError("unsigned amended contract re-pinned on resume")
assert json.loads(fences_path.read_text())["approvedContractSha256"] == hashlib.sha256(amended).hexdigest()
PY
pass_fixture AUTH-R2-004

# AUTH-R2-005..007 run through a real isolated supervision set. The config has
# a 200-minute pair cap and an irrelevant 900-minute mission contract: arming
# must derive 230 from config only, then 330 when --max-cap 300 is declared.
harness=$tmp/supervision
mkdir -p "$harness"
cp -R "$source_root/scripts" "$harness/"
mkdir -p "$harness/development" "$harness/plans"
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
process_start=$($harness/scripts/agents/process-census.py started-at --pid $$)
python3 - "$identity_fixture" "$$" "$process_start" <<'PY'
import json, sys
from pathlib import Path
path, pid, started = Path(sys.argv[1]), sys.argv[2], int(sys.argv[3])
value = json.loads(path.read_text())
value[pid].update({"started": started, "pidStartedAt": started, "command": "caps-fixture"})
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
$harness/scripts/agents/worktree-lease.py --root "$harness" announce \
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
if python3 - AUTH-R2-001 AUTH-R2-002 AUTH-R2-003 AUTH-R2-004 AUTH-R2-005 AUTH-R2-006 AUTH-R2-007 <<'PY'
import sys
expected = {f"AUTH-R2-{index:03d}" for index in range(1, 9)}
actual = set(sys.argv[1:])
raise SystemExit(0 if actual == expected else 1)
PY
then
  echo "AUTH-R2-009: incomplete fixture registry was accepted" >&2
  exit 1
fi
python3 - "${passed[@]}" <<'PY'
import sys
expected = {f"AUTH-R2-{index:03d}" for index in range(1, 9)}
actual = set(sys.argv[1:])
assert actual == expected, (sorted(actual), sorted(expected))
PY
pass_fixture AUTH-R2-009

echo "delegate caps authority fixtures passed" >&2
