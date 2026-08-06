#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/dispatch.sh dispatch --role <role> --brief <file>
      [--mode <working-mode>] [--runtime claude|codex|devin|fake]
      [--model <model>] [--job-id <id>]
      [--workspace <dir> | --worktree]
      [--permissions <preset|envelope-file>] [--mission <id>]
      [--wait] [--cap-min N]
  scripts/agents/dispatch.sh --role <role> --brief <file> [dispatch options]
  scripts/agents/dispatch.sh follow-up --job <job-id> --message <file> [--wait]
  scripts/agents/dispatch.sh status --job <job-id>
  scripts/agents/dispatch.sh cancel --job <job-id>
  scripts/agents/dispatch.sh close --job <root-id>
  scripts/agents/dispatch.sh reap [--job <job-id>] [--interval <sec>]

Exit codes: 0 success/completed; 2 usage; 3 failed; 4 timeout;
5 vanished; 6 unknown status job; 7 malformed status record; 8 cancelled.
USAGE
}

die() { echo "$2" >&2; exit "$1"; }

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
repo_scope=$(git -C "$root" rev-parse --show-toplevel 2>/dev/null) \
  || die 1 "metasystem installation is not inside a git repository: $root"
repo_scope=$(cd "$repo_scope" && pwd -P)
config="$root/scripts/metasystem-config.sh"
agents="$root/artifacts/agents"
jobs="$agents/jobs"
heartbeats="$agents/hb"
locks="$agents/locks"
record_locks="$agents/record-locks"
# Self-cleaning: every operation mktemps here and not every path removes its
# file; the comb found 142k orphans holding 557MB. Age-based, so nothing live
# is ever touched.
find "$record_locks" -maxdepth 1 -type f -mmin +60 -delete 2>/dev/null || true
capabilities="$agents/capabilities"
worktrees="$agents/worktrees"
process_instance_tag=
process_census="$root/scripts/agents/process-census.py"
arm_supervision="$root/scripts/agents/arm-supervision.sh"
mission_fence="$root/scripts/agents/mission-fence.py"

valid_id() { [[ "$1" =~ ^[a-z0-9][a-z0-9-]*$ ]]; }
now_iso() { date -u +%Y-%m-%dT%H:%M:%SZ; }
sha256_file() { shasum -a 256 "$1" | awk '{print $1}'; }

dispatch_fixture_wait_cap() { # base seconds; normal dispatch remains 1x
  local base=$1 scale_milli=${METASYSTEM_FIXTURE_CAP_SCALE_MILLI:-1000}
  [[ "$base" =~ ^[1-9][0-9]*$ && "$scale_milli" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "dispatch wait cap inputs must be positive integers"
  printf '%s\n' "$(( (base * scale_milli + 999) / 1000 ))"
}

report_plan_drift() {
  # Surfaced here as well as at end of turn, because the end-of-turn hook needs
  # a runtime that fires hooks and only one of the three has ever been observed
  # doing so. Every agent that delegates passes through this function whatever
  # runtime it is, so a plan contradicting the job records cannot stay invisible
  # on Codex or Devin merely because their hooks are unproven. Reporting only:
  # a stale plan is never a reason to refuse work.
  local reporter="$root/scripts/agents/open-work.py" output
  [[ -f "$reporter" ]] || return 0
  output=$(python3 "$reporter" --repo "$root" 2>/dev/null | grep '^STALE-PLAN' || true)
  [[ -z "$output" ]] || printf '%s\n' "$output" >&2
}

require_fresh_census() {
  local verdict="$agents/supervision/last-census.json" expected
  [[ -f "$verdict" ]] || die 1 "dispatch refused: census verdict is absent; run $arm_supervision --repo $repo_scope"
  python3 - "$verdict" <<'PY' || exit $?
import json, sys, time
from pathlib import Path
try: value=json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
except (OSError,ValueError) as error: raise SystemExit(f"dispatch refused: census verdict is unreadable: {error}")
required={"schemaVersion","writer","verdict","completedAtEpoch","intervalSec","fingerprint","counts","inventory","diagnostics","errors"}
if not required.issubset(value) or value.get("schemaVersion") != 1 or value.get("writer") != "watch-background-jobs.sh":
    raise SystemExit("dispatch refused: census verdict schema or writer is invalid")
if value.get("verdict") == "CENSUS-FAILED":
    raise SystemExit("dispatch refused: last census verdict is CENSUS-FAILED")
if value.get("verdict") != "SUCCESS":
    raise SystemExit("dispatch refused: census verdict is not successful")
completed, interval = value.get("completedAtEpoch"), value.get("intervalSec")
if not isinstance(completed,int) or not isinstance(interval,int) or interval < 1:
    raise SystemExit("dispatch refused: census freshness fields are invalid")
age=int(time.time())-completed
if age < -5 or age > interval:
    raise SystemExit(f"dispatch refused: census verdict is stale (age={age}s interval={interval}s)")
PY
  expected=$("$arm_supervision" fingerprint --repo "$repo_scope" 2>&1) \
    || die 1 "dispatch refused: census fingerprint cannot be computed: $expected"
  [[ "$(json_field "$verdict" fingerprint 2>/dev/null || true)" == "$expected" ]] \
    || die 1 "dispatch refused: census fingerprint does not match the armed code, signatures, configuration, and supervisor instances"
}

json_field() { # file, dotted field
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path
try:
    value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
    for part in sys.argv[2].split("."):
        value = value[part]
except (OSError, ValueError, KeyError, TypeError):
    raise SystemExit(1)
if value is None:
    print("null")
elif isinstance(value, bool):
    print("true" if value else "false")
elif isinstance(value, (dict, list)):
    print(json.dumps(value, separators=(",", ":")))
else:
    print(value)
PY
}

record_cas() { # job, expected status, target status, patch file
  local cas_rc=0
  "$0" __record-cas --job "$1" --expect "$2" --status "$3" --patch "$4" || cas_rc=$?
  # One-shot by contract: twenty call sites mktemp a patch and none cleaned it,
  # which is how record-locks reached 142k files. The wrapper is the one point
  # that always runs.
  rm -f -- "$4" 2>/dev/null || true
  return "$cas_rc"
}

record_create() { # job, source json
  "$0" __record-create --job "$1" --source "$2"
}

atomic_record_python() {
  python3 - "$root" "$@" <<'PY'
import fcntl
import json
import os
import re
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path

root = Path(sys.argv[1])
operation = sys.argv[2]
args = sys.argv[3:]
values = {}
while args:
    if len(args) < 2 or not args[0].startswith("--"):
        raise SystemExit(2)
    values[args[0][2:]] = args[1]
    args = args[2:]
job = values.get("job", "")
if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", job):
    raise SystemExit(2)

jobs = root / "artifacts" / "agents" / "jobs"
lock_dir = root / "artifacts" / "agents" / "record-locks"
jobs.mkdir(parents=True, exist_ok=True)
lock_dir.mkdir(parents=True, exist_ok=True)
record_path = jobs / f"{job}.json"

with (lock_dir / f"{job}.lock").open("a+") as lock:
    fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
    if operation == "create":
        if record_path.exists():
            print(f"job id collision: {job}", file=sys.stderr)
            raise SystemExit(1)
        try:
            record = json.loads(Path(values["source"]).read_text(encoding="utf-8"))
        except (OSError, ValueError) as error:
            print(f"invalid initial record for {job}: {error}", file=sys.stderr)
            raise SystemExit(1)
        if not isinstance(record, dict) or record.get("jobId") != job or record.get("status") != "pending":
            print(f"invalid initial record identity or status for {job}", file=sys.stderr)
            raise SystemExit(1)
    elif operation == "cas":
        try:
            record = json.loads(record_path.read_text(encoding="utf-8"))
            patch = json.loads(Path(values["patch"]).read_text(encoding="utf-8"))
        except (OSError, ValueError) as error:
            print(f"cannot update job record {job}: {error}", file=sys.stderr)
            raise SystemExit(1)
        expected = values.get("expect")
        target = values.get("status")
        current = record.get("status")
        if current != expected:
            raise SystemExit(3)
        transitions = {
            "pending": {"running", "failed", "cancelled"},
            "running": {"completed", "failed", "cancelled", "timeout"},
        }
        metadata_update = current == target
        if not metadata_update and target not in transitions.get(current, set()):
            print(f"illegal job transition: {current} to {target}", file=sys.stderr)
            raise SystemExit(1)
        if not isinstance(patch, dict) or "status" in patch:
            print("record patch must be an object and cannot contain status", file=sys.stderr)
            raise SystemExit(1)
        immutable = {"jobId", "role", "runtime", "round", "parentJob", "workspaceRoot", "baseSha", "branch", "startedAt"}
        if immutable.intersection(patch):
            print("record patch attempts to change immutable identity", file=sys.stderr)
            raise SystemExit(1)
        terminal = {"completed", "failed", "cancelled", "timeout"}
        if current in terminal and metadata_update and not set(patch).issubset({"mirror", "chainClosed", "chainUsage"}):
            print("terminal record metadata is final except mirror, closure, and aggregate usage", file=sys.stderr)
            raise SystemExit(1)
        record.update(patch)
        record["status"] = target
        if target in terminal and not record.get("endedAt"):
            record["endedAt"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    else:
        raise SystemExit(2)

    fd, temp_name = tempfile.mkstemp(prefix=f"{job}.", suffix=".tmp", dir=lock_dir)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(record, handle, indent=2, sort_keys=True)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temp_name, record_path)
        dir_fd = os.open(jobs, os.O_RDONLY)
        try:
            os.fsync(dir_fd)
        finally:
            os.close(dir_fd)
    finally:
        try:
            os.unlink(temp_name)
        except FileNotFoundError:
            pass
PY
}

process_matches() { # pid, tag
  local pid=$1 tag=$2 command
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  command=$(ps -p "$pid" -o command= 2>/dev/null || true)
  [[ -n "$tag" && "$command" == *"$tag"* ]]
}

process_exists() { # pid; permission denied still proves the pid exists
  python3 - "$1" <<'PY'
import os, sys
try:
    os.kill(int(sys.argv[1]), 0)
except ProcessLookupError:
    raise SystemExit(1)
except (PermissionError, ValueError):
    pass
PY
}

lock_owner_state() { # pid, tag -> live, dead, stale, or unknown
  local pid=$1 tag=$2 command status
  if ! process_exists "$pid"; then printf 'dead\n'; return; fi
  set +e
  command=$(ps -p "$pid" -o command= 2>/dev/null)
  status=$?
  set -e
  if [[ $status -ne 0 ]]; then printf 'unknown\n'; return; fi
  if [[ -n "$tag" && "$command" == *"$tag"* ]]; then printf 'live\n'; else printf 'stale\n'; fi
}

job_supervisor_matches() { # record
  local record=$1 pid tag runtime proof heartbeat
  pid=$(json_field "$record" pid 2>/dev/null || true)
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  process_matches "$pid" "$tag" && return 0
  runtime=$(json_field "$record" runtime 2>/dev/null || true)
  [[ "$runtime" == fake ]] || return 1
  process_exists "$pid" || return 1
  proof=$(json_field "$record" ownershipProof 2>/dev/null || true)
  heartbeat="$heartbeats/$(json_field "$record" jobId 2>/dev/null || true)"
  [[ "$proof" == *"\"pid\":$pid"* && "$proof" == *"\"instanceTag\":\"$tag\""* && -f "$heartbeat" ]] || return 1
  python3 - "$heartbeat" "$pid" "$tag" <<'PY'
import json, sys
from pathlib import Path
try: value = json.loads(Path(sys.argv[1]).read_text())
except (OSError, ValueError): raise SystemExit(1)
raise SystemExit(0 if value.get("pid") == int(sys.argv[2]) and value.get("instanceTag") == sys.argv[3] else 1)
PY
}

group_alive() { # pgid
  local pgid=$1
  [[ "$pgid" =~ ^[1-9][0-9]*$ ]] || return 1
  python3 - "$pgid" <<'PY'
import os, sys
try:
    os.killpg(int(sys.argv[1]), 0)
except ProcessLookupError:
    raise SystemExit(1)
except PermissionError:
    pass
PY
}

group_owned() { # record
  local record=$1 pgid tag runtime proof
  pgid=$(json_field "$record" pgid 2>/dev/null || true)
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  [[ "$pgid" =~ ^[1-9][0-9]*$ && "$pgid" -gt 1 ]] || return 1
  if ps -axo pgid=,command= 2>/dev/null | awk -v wanted="$pgid" -v tag="$tag" '
    $1 == wanted { $1=""; sub(/^ +/, ""); if (index($0, tag)) found=1 }
    END { exit !found }
  '; then
    return 0
  fi
  # The fake runtime runs under restricted CI sandboxes that may deny process
  # table reads for detached sessions. Its trusted launcher records the exact
  # pgid/tag pair before releasing the start gate, so fake-only fixtures can
  # still exercise real group signals without weakening a real adapter.
  runtime=$(json_field "$record" runtime 2>/dev/null || true)
  proof=$(json_field "$record" ownershipProof 2>/dev/null || true)
  [[ "$runtime" == fake && "$proof" == *"\"pgid\":$pgid"* && "$proof" == *"\"instanceTag\":\"$tag\""* ]]
}

wind_down_group() { # record
  local record=$1 pgid tag until
  pgid=$(json_field "$record" pgid 2>/dev/null || true)
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  group_alive "$pgid" || return 0
  group_owned "$record" || { echo "refusing to signal unowned process group $pgid" >&2; return 1; }
  kill -TERM -- "-$pgid" 2>/dev/null || true
  until=$(( $(date +%s) + 2 ))
  while group_alive "$pgid" && (( $(date +%s) < until )); do sleep 0.05; done
  if group_alive "$pgid"; then
    group_owned "$record" || { echo "lost ownership proof for process group $pgid" >&2; return 1; }
    kill -KILL -- "-$pgid" 2>/dev/null || true
  fi
  # When this shell launched the supervisor directly, reap its terminal wait
  # status now so a zombie group leader cannot masquerade as a live writer.
  wait "$(json_field "$record" pid 2>/dev/null || true)" 2>/dev/null || true
  until=$(( $(date +%s) + 2 ))
  while group_alive "$pgid" && (( $(date +%s) < until )); do sleep 0.05; done
  group_alive "$pgid" && { echo "process group $pgid survived KILL" >&2; return 1; }
  return 0
}

acquire_chain_lock() { # root id
  local chain=$1 dir="$locks/$1.d" owner pid tag owner_state
  mkdir -p "$locks"
  while ! mkdir "$dir" 2>/dev/null; do
    owner="$dir/owner.json"
    [[ -f "$owner" ]] || die 1 "chain lock has no owner lease: $dir"
    pid=$(json_field "$owner" pid 2>/dev/null || true)
    tag=$(json_field "$owner" instanceTag 2>/dev/null || true)
    owner_state=$(lock_owner_state "$pid" "$tag")
    [[ "$owner_state" != live ]] || die 1 "chain is busy: $chain"
    [[ "$owner_state" != unknown ]] || die 1 "chain lock owner liveness cannot be verified: $chain"
    [[ "$(find "$dir" -mindepth 1 -maxdepth 1 -type f -print | wc -l | tr -d ' ')" == 1 && -f "$owner" ]] \
      || die 1 "stale chain lock contains unexpected files: $dir"
    rm "$owner"
    rmdir "$dir"
  done
  python3 - "$dir/owner.json" "$$" "$process_instance_tag" <<'PY'
import json, os, sys, tempfile
from datetime import datetime, timezone
from pathlib import Path
path = Path(sys.argv[1])
value = {"pid": int(sys.argv[2]), "instanceTag": sys.argv[3], "acquiredAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}
fd, temp = tempfile.mkstemp(prefix="owner.", suffix=".tmp", dir=path.parent)
with os.fdopen(fd, "w", encoding="utf-8") as handle:
    json.dump(value, handle, sort_keys=True)
    handle.write("\n")
    handle.flush()
    os.fsync(handle.fileno())
os.replace(temp, path)
PY
}

release_chain_lock() { # root id
  local dir="$locks/$1.d" owner="$locks/$1.d/owner.json" pid tag
  [[ -f "$owner" ]] || return 0
  pid=$(json_field "$owner" pid 2>/dev/null || true)
  tag=$(json_field "$owner" instanceTag 2>/dev/null || true)
  [[ "$pid" == "$$" && "$tag" == "$process_instance_tag" ]] || die 1 "refusing to release another owner's chain lock"
  rm "$owner"
  rmdir "$dir"
}

acquire_lifecycle_lock() { # job id; nonzero means a live owner has it
  local job=$1 dir="$record_locks/$1.lifecycle.d" owner pid tag owner_state
  mkdir -p "$record_locks"
  while ! mkdir "$dir" 2>/dev/null; do
    owner="$dir/owner.json"
    [[ -f "$owner" ]] || return 1
    pid=$(json_field "$owner" pid 2>/dev/null || true)
    tag=$(json_field "$owner" instanceTag 2>/dev/null || true)
    owner_state=$(lock_owner_state "$pid" "$tag")
    [[ "$owner_state" != live && "$owner_state" != unknown ]] || return 1
    [[ "$(find "$dir" -mindepth 1 -maxdepth 1 -type f -print | wc -l | tr -d ' ')" == 1 && -f "$owner" ]] || return 1
    rm "$owner"
    rmdir "$dir"
  done
  python3 - "$dir/owner.json" "$$" "$process_instance_tag" <<'PY'
import json, os, sys, tempfile
from datetime import datetime, timezone
from pathlib import Path
path = Path(sys.argv[1])
value = {"pid": int(sys.argv[2]), "instanceTag": sys.argv[3], "acquiredAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}
fd, temp = tempfile.mkstemp(prefix="owner.", suffix=".tmp", dir=path.parent)
with os.fdopen(fd, "w", encoding="utf-8") as handle:
    json.dump(value, handle, sort_keys=True); handle.write("\n"); handle.flush(); os.fsync(handle.fileno())
os.replace(temp, path)
PY
}

acquire_lifecycle_lock_until() { # job id, maximum wait seconds
  local job=$1 base=$2 maximum started deadline elapsed
  maximum=$(dispatch_fixture_wait_cap "$base")
  started=$SECONDS
  deadline=$(( SECONDS + maximum ))
  while ! acquire_lifecycle_lock "$job"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "timed out acquiring lifecycle lock for $job (elapsed: ${elapsed}s; scaled cap: ${maximum}s)" >&2
      return 1
    fi
    sleep 0.05
  done
}

release_lifecycle_lock() { # job id
  local dir="$record_locks/$1.lifecycle.d" owner="$record_locks/$1.lifecycle.d/owner.json"
  [[ -f "$owner" ]] || return 0
  rm "$owner"
  rmdir "$dir"
}

config_get() { "$config" get "$@"; }

registered_runtime() { # runtime
  local configured
  configured=$(config_get --key metasystem.runtimes --default '')
  python3 - "$configured" "$1" <<'PY'
import sys
items = [item.strip() for item in sys.argv[1].split(",") if item.strip()]
raise SystemExit(0 if sys.argv[2] in items else 1)
PY
}

brief_mode() { # brief
  python3 - "$1" <<'PY'
import sys
from pathlib import Path
values = []
for line in Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    if line.startswith("Working Mode:"):
        values.append(line.split(":", 1)[1].strip())
if len(values) != 1 or not values[0] or values[0].startswith("<"):
    raise SystemExit(1)
print(values[0])
PY
}

model_tier() { # runtime, model; prints 999999 if absent
  python3 - "$root/metasystem.conf" "$1:$2" <<'PY'
import re, sys
from pathlib import Path
wanted = sys.argv[2]
tiers = []
for raw in Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    if "=" not in raw:
        continue
    key, value = (part.strip() for part in raw.split("=", 1))
    match = re.fullmatch(r"model\.tier\.([1-9][0-9]*)", key)
    if match and wanted in [item.strip() for item in value.split(",")]:
        tiers.append(int(match.group(1)))
print(tiers[0] if len(tiers) == 1 else 999999)
PY
}

validate_mission() { # mission id, lease path
  python3 - "$root" "$1" "$2" <<'PY'
import json, os, subprocess, sys
from pathlib import Path
root = Path(sys.argv[1]).resolve()
mission, supplied = sys.argv[2:]
if not mission or not all(c.islower() or c.isdigit() or c == "-" for c in mission) or not mission[0].isalnum():
    raise SystemExit("invalid mission id")
expected = (root / "artifacts" / "agents" / "missions" / mission / "lease.json").resolve()
lease_path = Path(supplied).resolve()
if lease_path != expected:
    raise SystemExit("mission lease path is ambiguous or non-canonical")
try:
    lease = json.loads(lease_path.read_text(encoding="utf-8"))
except (OSError, ValueError) as error:
    raise SystemExit(f"mission has no readable live lease: {error}")
required = {"missionId", "pid", "pgid", "instanceTag", "startedAt", "renewedAt"}
if set(lease) != required or lease.get("missionId") != mission:
    raise SystemExit("mission lease has an invalid shape or identity")
pid, pgid, tag = lease.get("pid"), lease.get("pgid"), lease.get("instanceTag")
if not isinstance(pid, int) or not isinstance(pgid, int) or not isinstance(tag, str) or not tag:
    raise SystemExit("mission lease has invalid ownership fields")
try:
    os.kill(pid, 0)
    actual_pgid = os.getpgid(pid)
except OSError:
    raise SystemExit("mission lease holder is not alive")
try:
    command = subprocess.check_output(["ps", "-p", str(pid), "-o", "command="], text=True).strip()
except (OSError, subprocess.SubprocessError):
    fixture_path = os.environ.get("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
    configured = ""
    for raw in (root / "metasystem.conf").read_text().splitlines():
        if raw.startswith("metasystem.runtimes="):
            configured = raw.split("=", 1)[1].strip()
    if configured != "fake" or not fixture_path:
        raise SystemExit("mission process command line could not be verified")
    try:
        fixture = json.loads(Path(fixture_path).read_text())
        identity = fixture[str(pid)]
        command = identity["command"]
        if identity["pgid"] != actual_pgid:
            raise ValueError("pgid mismatch")
    except (OSError, ValueError, KeyError, TypeError):
        raise SystemExit("fake mission process identity fixture is invalid")
if tag not in command or actual_pgid != pgid:
    raise SystemExit("mission lease holder failed process identity proof")
PY
}

resolve_mission() { # explicit id; prints mission|lease|turn or ||
  local explicit=$1 env_id=${METASYSTEM_MISSION_ID:-} env_lease=${METASYSTEM_MISSION_LEASE:-}
  local env_turn=${METASYSTEM_MISSION_TURN:-} mission lease
  if [[ -n "$env_id" || -n "$env_lease" ]]; then
    [[ -n "$env_id" && -n "$env_lease" ]] || die 1 "ambiguous inherited mission context: both METASYSTEM_MISSION_ID and METASYSTEM_MISSION_LEASE are required"
  fi
  [[ -z "$env_turn" || -n "$env_id" ]] \
    || die 1 "ambiguous inherited mission context: METASYSTEM_MISSION_TURN requires METASYSTEM_MISSION_ID and METASYSTEM_MISSION_LEASE"
  [[ -z "$env_turn" ]] || valid_id "$env_turn" || die 1 "invalid inherited mission turn id"
  if [[ -n "$explicit" && -n "$env_id" && "$explicit" != "$env_id" ]]; then
    die 1 "ambiguous mission context: --mission and METASYSTEM_MISSION_ID disagree"
  fi
  mission=${explicit:-$env_id}
  if [[ -z "$mission" ]]; then printf '||\n'; return; fi
  lease=${env_lease:-$agents/missions/$mission/lease.json}
  validate_mission "$mission" "$lease" || die 1 "mission $mission does not have a live, matching lease"
  printf '%s|%s|%s\n' "$mission" "$lease" "$env_turn"
}

expand_permissions() { # requested value, workspace root, worktree flag, output
  local requested=$1 workspace=$2 is_worktree=$3 output=$4 source preset network_floor
  if [[ -f "$requested" ]]; then source=$requested; preset=custom; else source="$root/scripts/agents/permissions/$requested.json"; preset=$requested; fi
  [[ -f "$source" ]] || die 1 "unknown permissions preset or envelope file: $requested"
  # A repository may deny network to every delegate regardless of preset. A
  # benchmark target sets this, because an agent that can reach the internet can
  # download a solution and the measurement stops meaning anything. It only ever
  # narrows: a repository cannot grant access a preset withholds.
  network_floor=$(config_get --key dispatch.permissions.network --default '')
  case "$network_floor" in ''|deny|allow) ;; *) die 1 "dispatch.permissions.network must be deny or allow" ;; esac
  python3 - "$source" "$repo_scope" "$workspace" "$is_worktree" "$preset" "$output" "$network_floor" <<'PY'
import json, os, sys
from pathlib import Path
source, repo, workspace, is_worktree, preset, output, network_floor = sys.argv[1:]
repo = Path(repo).resolve()
workspace = Path(workspace).resolve()
try:
    envelope = json.loads(Path(source).read_text(encoding="utf-8"))
except (OSError, ValueError) as error:
    raise SystemExit(f"invalid permissions envelope: {error}")
expected = {"readRoots", "writeRoots", "network", "approvals", "tools"}
if not isinstance(envelope, dict) or set(envelope) != expected:
    raise SystemExit("permissions envelope must contain exactly readRoots, writeRoots, network, approvals, and tools")
if not isinstance(envelope["readRoots"], list) or not isinstance(envelope["writeRoots"], list):
    raise SystemExit("permission roots must be arrays")
def expand(value):
    if value == ".":
        return str(repo)
    if value == "<worktree>":
        return str(workspace)
    path = Path(value)
    return str((repo / path).resolve()) if not path.is_absolute() else str(path.resolve())
envelope["readRoots"] = [expand(item) for item in envelope["readRoots"] if isinstance(item, str)]
envelope["writeRoots"] = [expand(item) for item in envelope["writeRoots"] if isinstance(item, str)]
if envelope["writeRoots"] and is_worktree != "1":
    raise SystemExit("writable permissions require --worktree")
for item in envelope["writeRoots"]:
    try:
        Path(item).resolve().relative_to(workspace)
    except ValueError:
        raise SystemExit(f"permission write root escapes the job worktree: {item}")
if network_floor == "deny":
    envelope["network"] = "deny"
envelope = {"preset": preset, **envelope}
Path(output).write_text(json.dumps(envelope, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

select_snapshot() { # runtime, role, requested envelope, output json
  local runtime=$1 role=$2 envelope=$3 output=$4 adapter="$root/scripts/agents/adapters/$1.sh" identity version hash max_age
  [[ -x "$adapter" ]] || die 1 "runtime adapter is not installed: $runtime"
  identity=$($adapter identity) || die 1 "could not read $runtime adapter identity"
  read -r version hash extra <<<"$identity"
  [[ -n "$version" && -n "$hash" && -z "${extra:-}" && "$version" =~ ^[A-Za-z0-9._-]+$ && "$hash" =~ ^[A-Za-z0-9._-]+$ ]] \
    || die 1 "$runtime adapter returned a malformed identity"
  max_age=$(config_get --key capability.snapshot-max-age-days --default 30)
  [[ "$max_age" =~ ^[0-9]+$ ]] || die 1 "capability.snapshot-max-age-days must be a non-negative integer"
  python3 - "$root" "$runtime" "$version" "$hash" "$max_age" "$role" "$envelope" "$output" <<'PY'
import json, re, sys
from datetime import datetime, timezone
from pathlib import Path
root = Path(sys.argv[1])
runtime, version, config_hash, max_age, role, envelope_path, output = sys.argv[2:]
max_age = int(max_age)
directory = root / "artifacts" / "agents" / "capabilities"
pattern = re.compile(rf"{re.escape(runtime)}-{re.escape(version)}-{re.escape(config_hash)}-(\d{{8}})-(\d{{3}})\.json$")
candidates = []
for path in directory.glob(f"{runtime}-{version}-{config_hash}-*.json") if directory.exists() else []:
    match = pattern.fullmatch(path.name)
    if not match:
        continue
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
        captured = datetime.fromisoformat(value["capturedAt"].replace("Z", "+00:00"))
    except (OSError, ValueError, KeyError, TypeError):
        continue
    if value.get("runtime") != runtime or value.get("cliVersion") != version or value.get("configHash") != config_hash:
        continue
    candidates.append((match.group(1), int(match.group(2)), captured, path, value))
if not candidates:
    raise SystemExit(f"no capability snapshot matches {runtime} {version} {config_hash}; run {runtime} adapter probe")
date, sequence, captured, path, snapshot = max(candidates, key=lambda item: (item[0], item[1]))
age_days = (datetime.now(timezone.utc) - captured.astimezone(timezone.utc)).total_seconds() / 86400
if age_days > max_age:
    raise SystemExit(f"capability snapshot is stale ({age_days:.1f} days); re-run {runtime} adapter probe")
requirements_path = root / "scripts" / "agents" / "roles" / f"{role}.requirements.json"
try:
    requirements = json.loads(requirements_path.read_text(encoding="utf-8"))
    envelope = json.loads(Path(envelope_path).read_text(encoding="utf-8"))
except (OSError, ValueError) as error:
    raise SystemExit(f"cannot evaluate capabilities: {error}")
caps = snapshot.get("capabilities", {})
handshake_timeout = caps.get("sessionEstablishedTimeoutSec", 2)
if (
    isinstance(handshake_timeout, bool)
    or not isinstance(handshake_timeout, int)
    or not 1 <= handshake_timeout <= 60
):
    raise SystemExit("capability snapshot has an invalid session-established timeout")
missing = [name for name in requirements.get("required", []) if caps.get(name) is not True]
if missing:
    raise SystemExit("required runtime capabilities are absent: " + ", ".join(sorted(missing)))
fallbacks = []
for name, declaration in requirements.get("optional", {}).items():
    if caps.get(name) is not True:
        fallbacks.append({"capability": name, "fallback": declaration.get("fallback")})
waivers = requirements.get("waivers", {})
unverified = snapshot.get("permissions", {}).get("unverified", [])
for field in unverified:
    if envelope.get(field) == "deny" and runtime not in waivers.get(field, []):
        raise SystemExit(f"runtime cannot verify restrictive permission field {field}; add an explicit role waiver or choose another runtime")
result = {
    "path": str(path.relative_to(root)), "fallbacks": fallbacks,
    "sessionEstablishedSignal": caps.get("sessionEstablishedSignal") is True,
    "sessionEstablishedTimeoutSec": handshake_timeout,
    "resume": caps.get("resume") is True,
}
Path(output).write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

root_job_id() { # job record
  python3 - "$jobs" "$1" <<'PY'
import json, sys
from pathlib import Path
jobs = Path(sys.argv[1])
job = sys.argv[2]
seen = set()
while True:
    if job in seen:
        raise SystemExit(1)
    seen.add(job)
    value = json.loads((jobs / f"{job}.json").read_text(encoding="utf-8"))
    parent = value.get("parentJob")
    if parent is None:
        print(job)
        break
    job = parent
PY
}

latest_chain_record() { # root job
  python3 - "$jobs" "$1" <<'PY'
import json, sys
from pathlib import Path
jobs = Path(sys.argv[1]); root = sys.argv[2]
records = []
for path in jobs.glob("*.json"):
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        continue
    current = value
    seen = set()
    while current.get("parentJob") is not None:
        parent = current.get("parentJob")
        if parent in seen:
            break
        seen.add(parent)
        try:
            current = json.loads((jobs / f"{parent}.json").read_text(encoding="utf-8"))
        except (OSError, ValueError):
            break
    if current.get("jobId") == root:
        records.append((value.get("round", 0), path))
if not records:
    raise SystemExit(1)
print(max(records, key=lambda item: item[0])[1])
PY
}

write_prompt() { # path job role runtime model round mission content
  local path=$1 job=$2 role=$3 runtime=$4 model=$5 round=$6 mission=${7:-none} content=$8
  {
    printf 'Job-Id: %s\nRole: %s\nRuntime: %s\nModel: %s\nRound: %s\nMission: %s\n\n' "$job" "$role" "$runtime" "$model" "$round" "${mission:-none}"
    cat "$root/scripts/agents/roles/$role.md"
    printf '\n\n'
    cat "$content"
  } >"$path"
}

launch_adapter() { # runtime verb job tag
  local runtime=$1 verb=$2 job=$3 tag=$4 gate="$heartbeats/$job.start" adapter="$root/scripts/agents/adapters/$runtime.sh" pid pid_started patch cap started deadline elapsed
  mkdir -p "$heartbeats"
  python3 - "$root" "$adapter" "$verb" "$job" "$gate" "$tag" >/dev/null 2>&1 <<'PY' &
import os, sys
root, adapter, verb, job, gate, tag = sys.argv[1:]
os.chdir(root)
os.setsid()
os.environ["GIT_AUTHOR_NAME"] = job
os.environ["GIT_AUTHOR_EMAIL"] = f"{job}@metasystem.invalid"
os.execv(adapter, [adapter, verb, "--job", job, "--start-gate", gate, "--instance-tag", tag])
PY
  pid=$!
  cap=$(dispatch_fixture_wait_cap 5)
  started=$SECONDS
  deadline=$((SECONDS + cap))
  until pid_started=$("$process_census" started-at --pid "$pid" 2>/dev/null); do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "adapter start identity ceiling reached for $job (elapsed: ${elapsed}s; scaled cap: ${cap}s)" >&2
      return 1
    fi
    sleep 0.02
  done
  patch=$(mktemp "$record_locks/launch.XXXXXX")
  python3 - "$patch" "$pid" "$pid_started" "$tag" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
pid = int(sys.argv[2])
pid_started = int(sys.argv[3])
Path(sys.argv[1]).write_text(json.dumps({
  "pid": pid, "pidStartedAt": pid_started, "pgid": pid,
  "ownershipProof": {"pid": pid, "pidStartedAt": pid_started, "pgid": pid, "instanceTag": sys.argv[4], "provenAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"), "source": "trusted-launcher"},
}) + "\n")
PY
  record_cas "$job" pending pending "$patch" || return 1
  touch "$gate"
}

await_handshake() { # job, maximum session-established seconds
  local job=$1 timeout=$2 record="$jobs/$1.json" deadline status session patch
  [[ "$timeout" =~ ^[1-9][0-9]*$ && "$timeout" -le 60 ]] || return 1
  deadline=$(( $(date +%s) + timeout ))
  while (( $(date +%s) <= deadline )); do
    if [[ -f "$record" ]]; then
      status=$(json_field "$record" status 2>/dev/null || true)
      session=$(json_field "$record" sessionId 2>/dev/null || true)
      case "$status" in
        running|completed)
          [[ -f "$jobs/$job.log" && "$session" != null && -n "$session" ]] && return 0
          ;;
        failed|cancelled|timeout) return 1 ;;
      esac
    fi
    sleep 0.05
  done
  wind_down_group "$record" || return 1
  patch=$(mktemp "$record_locks/handshake.XXXXXX")
  printf '{"error":"handshake_timeout","phase":"handshake"}\n' >"$patch"
  record_cas "$job" pending failed "$patch" || true
  return 1
}

wait_for_job() { # job
  local job=$1 record="$jobs/$1.json" status
  touch "$heartbeats/$job.waiting"
  while true; do
    [[ -f "$record" ]] || return 5
    status=$(json_field "$record" status 2>/dev/null || true)
    case "$status" in
      completed|failed|timeout|cancelled)
        reap_one "$job"
        case "$status" in completed) return 0 ;; failed) return 3 ;; timeout) return 4 ;; cancelled) return 8 ;; esac
        ;;
      pending|running)
        if ! reap_one "$job"; then
          [[ -f "$record" ]] || return 5
          return 3
        fi
        [[ -f "$record" ]] || return 5
        sleep 0.1
        ;;
      *) return 5 ;;
    esac
  done
}

aggregate_chain_usage() { # root id
  local chain=$1 record="$jobs/$1.json" status patch
  [[ -f "$record" ]] || return 0
  status=$(json_field "$record" status 2>/dev/null || true)
  case "$status" in completed|failed|timeout|cancelled) ;; *) return 0 ;; esac
  patch=$(mktemp "$record_locks/usage.XXXXXX")
  python3 - "$jobs" "$chain" "$patch" <<'PY'
import json, sys
from pathlib import Path
jobs, root, output = Path(sys.argv[1]), sys.argv[2], Path(sys.argv[3])
records = []
for path in jobs.glob("*.json"):
    try: value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError): continue
    current = value; seen = set()
    while current.get("parentJob") is not None:
        parent = current.get("parentJob")
        if parent in seen: break
        seen.add(parent)
        try: current = json.loads((jobs / f"{parent}.json").read_text(encoding="utf-8"))
        except (OSError, ValueError): break
    if current.get("jobId") == root: records.append(value)
tokens = {}; costs = {}; units = {}
for record in records:
    usage = record.get("usage")
    if not isinstance(usage, dict): continue
    runtime = record.get("runtime", "unknown")
    target = tokens.setdefault(runtime, {name: None for name in ("inputTokens", "cachedInputTokens", "outputTokens", "reasoningTokens")})
    for name in target:
        value = usage.get(name)
        if isinstance(value, (int, float)) and not isinstance(value, bool): target[name] = (target[name] or 0) + value
    cost = usage.get("cost")
    if isinstance(cost, dict) and isinstance(cost.get("amount"), (int, float)) and isinstance(cost.get("currency"), str):
        costs[cost["currency"]] = costs.get(cost["currency"], 0) + cost["amount"]
    unit = usage.get("providerUnits")
    if isinstance(unit, dict) and isinstance(unit.get("name"), str) and isinstance(unit.get("value"), (int, float)):
        units.setdefault(runtime, {})[unit["name"]] = units.setdefault(runtime, {}).get(unit["name"], 0) + unit["value"]
output.write_text(json.dumps({"chainUsage": {"tokens": tokens, "cost": costs, "providerUnits": units}}, sort_keys=True) + "\n")
PY
  record_cas "$chain" "$status" "$status" "$patch" || true
}

aggregate_mission_usage() { # job record
  local record=$1 mission
  mission=$(json_field "$record" mission 2>/dev/null || true)
  [[ -n "$mission" && "$mission" != null ]] || return 0
  "$mission_fence" aggregate-usage --repo "$root" --mission "$mission"
}

mirror_fail() { # job, reason — durable trace beside the jobs it failed for
  printf '%s %s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" \
    >>"${jobs%/jobs}/mirror-failures.log" 2>/dev/null || true
  echo "cannot mirror $1: $2" >&2
}

mirror_record() { # job
  local job=$1 record="$jobs/$1.json" status evidence result patch root_id
  [[ -f "$record" ]] || return 0
  status=$(json_field "$record" status 2>/dev/null || true)
  case "$status" in completed|failed|timeout|cancelled) ;; *) return 0 ;; esac
  evidence=$(config_get --key evidence.root --default '')
  [[ "$evidence" == /* ]] || { mirror_fail "$job" "evidence.root must be absolute"; return 1; }
  evidence=$(python3 - "$evidence" <<'PY'
import sys
from pathlib import Path
print(Path(sys.argv[1]).resolve(strict=False))
PY
  )
  case "${evidence%/}/" in "${repo_scope%/}/"*) mirror_fail "$job" "evidence.root is inside the repository"; return 1 ;; esac
  root_id=$(root_job_id "$job") || return 1
  result=$(mktemp "$record_locks/mirror-result.XXXXXX")
  if ! python3 - "$root" "$evidence" "$root_id" "$job" "$result" <<'PY'
import hashlib, json, os, shutil, sys, tempfile
from datetime import datetime, timezone
from pathlib import Path
repo, evidence, root_job, job, result = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3], sys.argv[4], Path(sys.argv[5])
agents = repo / "artifacts" / "agents"
record_path = agents / "jobs" / f"{job}.json"
record = json.loads(record_path.read_text(encoding="utf-8"))
round_number = record["round"]
payload = agents / root_job
if (payload / ".mirror-fail-once").exists():
    (payload / ".mirror-fail-once").unlink()
    (payload / ".mirror-failed").write_text("scripted interruption\n")
    raise SystemExit("scripted mirror interruption")
destination = evidence / "agents" / root_job
destination.mkdir(parents=True, exist_ok=True)
manifest_path = destination / "manifest.json"

def digest(path):
    value = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()

def semantic_record_hash(path):
    value = json.loads(path.read_text(encoding="utf-8"))
    value["mirror"] = None
    return hashlib.sha256((json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()).hexdigest()

sources = [(record_path, Path("jobs") / record_path.name)]
log = agents / "jobs" / f"{job}.log"
if log.exists(): sources.append((log, Path("jobs") / log.name))
if round_number == 1 and (payload / "brief.md").exists(): sources.append((payload / "brief.md", Path("brief.md")))
round_dir = payload / "rounds" / str(round_number)
if round_dir.exists():
    for source in sorted(path for path in round_dir.rglob("*") if path.is_file()):
        sources.append((source, Path("rounds") / str(round_number) / source.relative_to(round_dir)))
snapshot = repo / record["capabilitySnapshot"]
if snapshot.exists(): sources.append((snapshot, Path("capabilities") / snapshot.name))

old = {}
if manifest_path.exists():
    try: old = json.loads(manifest_path.read_text(encoding="utf-8")).get("files", {})
    except (OSError, ValueError): old = {}
unchanged = True
for source, relative in sources:
    item = old.get(str(relative))
    target = destination / relative
    if not isinstance(item, dict) or not target.exists() or digest(target) != item.get("sha256"):
        unchanged = False; break
    if source == record_path:
        if item.get("sourceStateHash") != semantic_record_hash(source): unchanged = False; break
    elif digest(source) != item.get("sha256"):
        unchanged = False; break
if unchanged and manifest_path.exists():
    manifest_hash = digest(manifest_path)
else:
    files = dict(old)
    for source, relative in sources:
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        fd, temp_name = tempfile.mkstemp(prefix=target.name + ".", suffix=".tmp", dir=target.parent)
        os.close(fd)
        try:
            shutil.copyfile(source, temp_name)
            os.replace(temp_name, target)
        finally:
            try: os.unlink(temp_name)
            except FileNotFoundError: pass
        if digest(source) != digest(target): raise SystemExit(f"mirror verification failed for {relative}")
        item = {"sha256": digest(target), "bytes": target.stat().st_size}
        if source == record_path: item["sourceStateHash"] = semantic_record_hash(source)
        files[str(relative)] = item
    manifest = {"rootJob": root_job, "files": files, "updatedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}
    fd, temp_name = tempfile.mkstemp(prefix="manifest.", suffix=".tmp", dir=destination)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(manifest, handle, indent=2, sort_keys=True); handle.write("\n"); handle.flush(); os.fsync(handle.fileno())
    os.replace(temp_name, manifest_path)
    manifest_hash = digest(manifest_path)
Path(result).write_text(json.dumps({"path": str(destination), "manifest": manifest_hash, "mirroredAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}) + "\n")
PY
  then
    mirror_fail "$job" "copy or verification failed (see stderr above)"
    return 1
  fi
  patch=$(mktemp "$record_locks/mirror-patch.XXXXXX")
  python3 - "$result" "$patch" <<'PY'
import json, sys
from pathlib import Path
Path(sys.argv[2]).write_text(json.dumps({"mirror": json.loads(Path(sys.argv[1]).read_text())}) + "\n")
PY
  python3 - "$jobs" "$root_id" <<'PY' | while IFS='|' read -r chain_job chain_status; do
import json, sys
from pathlib import Path
jobs, root = Path(sys.argv[1]), sys.argv[2]
for path in jobs.glob("*.json"):
    try: value = json.loads(path.read_text())
    except (OSError, ValueError): continue
    current = value; seen = set()
    while current.get("parentJob") is not None:
        parent = current.get("parentJob")
        if parent in seen: break
        seen.add(parent)
        try: current = json.loads((jobs / f"{parent}.json").read_text())
        except (OSError, ValueError): break
    if current.get("jobId") == root and value.get("status") in {"completed", "failed", "timeout", "cancelled"}:
        print(f"{value['jobId']}|{value['status']}")
PY
    # record_cas consumes its patch (one-shot); this loop applies the same
    # content to every job in the chain, so each call gets its own copy.
    patch_copy=$(mktemp "$record_locks/mirror-patch.XXXXXX")
    cp "$patch" "$patch_copy"
    record_cas "$chain_job" "$chain_status" "$chain_status" "$patch_copy" || return 1
  done
  rm -f -- "$result" "$patch" 2>/dev/null || true
}

reap_one_locked() { # job
  local job=$1 record="$jobs/$1.json" status pid tag started cap elapsed patch root_id mission
  [[ -f "$record" ]] || return 0
  status=$(json_field "$record" status 2>/dev/null || true)
  case "$status" in
    completed|failed|timeout|cancelled)
      settled_mirror=$(json_field "$record" mirror 2>/dev/null || true)
      if [[ -n "$settled_mirror" && "$settled_mirror" != None && "$settled_mirror" != null ]]; then
        # Settled once: mirrored terminal records are done. Re-walking them
        # every sweep rewrote every record every interval, all day. A null
        # mirror is a FAILED mirror and must keep retrying: the first guard
        # treated the string None as settled and the mirror-retry fixture
        # caught it within one gate run.
        return
      fi
      root_id=$(root_job_id "$job" 2>/dev/null || true)
      [[ -n "$root_id" ]] && aggregate_chain_usage "$root_id"
      aggregate_mission_usage "$record" || true
      mirror_record "$job" || true
      return
      ;;
    pending|running) ;;
    *) return ;;
  esac
  pid=$(json_field "$record" pid 2>/dev/null || true)
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  started=$(json_field "$record" startedAt 2>/dev/null || true)
  cap=$(json_field "$record" capMin 2>/dev/null || true)
  if ! job_supervisor_matches "$record"; then
    wind_down_group "$record" || return 1
    patch=$(mktemp "$record_locks/lost.XXXXXX")
    printf '{"error":"process-lost","phase":"supervision","groupDeathProvenAt":"%s"}\n' "$(now_iso)" >"$patch"
    record_cas "$job" "$status" failed "$patch" || true
    mirror_record "$job" || true
    return
  fi
  elapsed=$(python3 - "$started" <<'PY'
from datetime import datetime, timezone
import sys
try: started = datetime.fromisoformat(sys.argv[1].replace("Z", "+00:00"))
except ValueError: raise SystemExit(1)
print(int((datetime.now(timezone.utc) - started).total_seconds()))
PY
  ) || return 1
  if [[ "$cap" =~ ^[1-9][0-9]*$ ]] && (( elapsed >= cap * 60 )); then
    wind_down_group "$record" || return 1
    patch=$(mktemp "$record_locks/timeout.XXXXXX")
    printf '{"error":"budget-cap","phase":"supervision","groupDeathProvenAt":"%s"}\n' "$(now_iso)" >"$patch"
    record_cas "$job" "$status" timeout "$patch" || true
    mission=$(json_field "$record" mission 2>/dev/null || true)
    if [[ -n "$mission" && "$mission" != null ]]; then
      "$mission_fence" refuse --repo "$root" --mission "$mission" --reason job-cap-min >/dev/null || true
      aggregate_mission_usage "$record" || true
    fi
    mirror_record "$job" || true
  fi
}

reap_one() { # job
  local job=$1 result
  acquire_lifecycle_lock "$job" || return 0
  set +e
  reap_one_locked "$job"
  result=$?
  set -e
  release_lifecycle_lock "$job"
  return "$result"
}

dispatch_job() {
  local role= brief= mode_override= runtime_override= model_override= job= workspace= permissions_override= mission_override= cap_override=
  local use_worktree=0 wait=0 mode runtime model default_model configured_runtime overridden=false mission_data mission lease mission_turn cap watch_cap
  local permission_name permission_json snapshot_json snapshot_path fallbacks signal input_bytes input_hash max_kb payload round_dir record_json
  while (($#)); do
    case "$1" in
      --role) [[ $# -ge 2 ]] || { usage; exit 2; }; role=$2; shift 2 ;;
      --brief) [[ $# -ge 2 ]] || { usage; exit 2; }; brief=$2; shift 2 ;;
      --mode) [[ $# -ge 2 ]] || { usage; exit 2; }; mode_override=$2; shift 2 ;;
      --runtime) [[ $# -ge 2 ]] || { usage; exit 2; }; runtime_override=$2; shift 2 ;;
      --model) [[ $# -ge 2 ]] || { usage; exit 2; }; model_override=$2; shift 2 ;;
      --job-id) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --workspace) [[ $# -ge 2 ]] || { usage; exit 2; }; workspace=$2; shift 2 ;;
      --worktree) use_worktree=1; shift ;;
      --permissions) [[ $# -ge 2 ]] || { usage; exit 2; }; permissions_override=$2; shift 2 ;;
      --mission) [[ $# -ge 2 ]] || { usage; exit 2; }; mission_override=$2; shift 2 ;;
      --cap-min) [[ $# -ge 2 ]] || { usage; exit 2; }; cap_override=$2; shift 2 ;;
      --wait) wait=1; shift ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ -n "$role" && -f "$brief" ]] || { usage; exit 2; }
  [[ -f "$root/scripts/agents/roles/$role.md" && -f "$root/scripts/agents/roles/$role.requirements.json" ]] || die 1 "unknown dispatch role: $role"
  [[ ! ( $use_worktree -eq 1 && -n "$workspace" ) ]] || die 2 "--workspace and --worktree are mutually exclusive"
  mode=$(brief_mode "$brief") || die 1 "brief must contain exactly one filled Working Mode header"
  [[ -z "$mode_override" || "$mode_override" == "$mode" ]] || die 1 "--mode contradicts the brief's Working Mode header"
  require_fresh_census
  report_plan_drift

  configured_runtime=$(config_get --key "role.$role.runtime" --mode "$mode" --default __missing__)
  [[ "$configured_runtime" != __missing__ ]] || configured_runtime=$(config_get --key role.default.runtime --mode "$mode" --default __missing__)
  runtime=${runtime_override:-$configured_runtime}
  [[ "$runtime" != __missing__ && -n "$runtime" ]] || die 1 "role $role has neither a runtime entry nor role.default.runtime"
  [[ "$runtime" != main ]] || die 1 "role $role is assigned to main and cannot be dispatched"
  registered_runtime "$runtime" || die 1 "runtime $runtime is outside metasystem.runtimes"
  default_model=$(config_get --key "role.$role.model.$runtime" --mode "$mode" --default __missing__)
  [[ "$default_model" != __missing__ ]] || default_model=$(config_get --key "role.default.model.$runtime" --mode "$mode" --default __missing__)
  [[ "$default_model" != __missing__ ]] || die 1 "role $role resolves to $runtime but has no model.$runtime value"
  model=${model_override:-$default_model}
  if [[ -n "$model_override" && $(model_tier "$runtime" "$model") -gt $(model_tier "$runtime" "$default_model") ]]; then
    die 1 "model override is costlier than the recorded default and requires human approval"
  fi
  [[ -z "$runtime_override" && -z "$model_override" ]] || overridden=true

  if [[ -z "$job" ]]; then
    job=$(python3 - "$role" <<'PY'
import secrets, sys
from datetime import datetime, timezone
stamp = datetime.now(timezone.utc).strftime("%Y%m%dt%H%M%Sz").lower()
print(f"{sys.argv[1]}-{stamp}-{secrets.token_hex(2)}")
PY
    )
  fi
  valid_id "$job" || die 2 "invalid job id: $job"
  mkdir -p "$jobs" "$record_locks" "$capabilities" "$worktrees"
  acquire_chain_lock "$job"
  trap 'release_chain_lock "$job"' EXIT
  [[ ! -e "$jobs/$job.json" && ! -e "$agents/$job" ]] || die 1 "job id collision: $job"

  mission_data=$(resolve_mission "$mission_override")
  IFS='|' read -r mission lease mission_turn <<<"$mission_data"
  cap=$(config_get --key dispatch.cap-min ${cap_override:+--flag "$cap_override"} --default 120)
  [[ "$cap" =~ ^[1-9][0-9]*$ ]] || die 1 "dispatch cap must be a positive integer"
  if [[ -n "$mission" ]]; then
    "$mission_fence" check-job --repo "$root" --mission "$mission" --job "$job" --cap-min "$cap" \
      || die 1 "mission dispatch refused by a lifecycle fence"
  fi
  watch_cap=$(config_get --key watch.cap-min --default 180)
  [[ "$watch_cap" =~ ^[1-9][0-9]*$ && $cap -lt $watch_cap ]] || die 1 "dispatch cap must stay below watch.cap-min"

  if (( use_worktree )); then
    workspace="$worktrees/$job"
    [[ ! -e "$workspace" ]] || die 1 "job worktree already exists: $workspace"
    git -C "$repo_scope" worktree add -q -b "agent/$job" "$workspace" HEAD || die 1 "could not create job worktree"
  else
    workspace=${workspace:-$repo_scope}
    workspace=$(cd "$workspace" && pwd -P) || die 1 "workspace does not exist: $workspace"
  fi
  permission_name=${permissions_override:-$(config_get --key "dispatch.permissions.$role" --default none)}
  permission_json=$(mktemp "$record_locks/permissions.XXXXXX")
  expand_permissions "$permission_name" "$workspace" "$use_worktree" "$permission_json"
  snapshot_json=$(mktemp "$record_locks/snapshot.XXXXXX")
  select_snapshot "$runtime" "$role" "$permission_json" "$snapshot_json"
  snapshot_path=$(json_field "$snapshot_json" path)
  fallbacks=$(json_field "$snapshot_json" fallbacks)
  signal=$(json_field "$snapshot_json" sessionEstablishedSignal)

  input_bytes=$(wc -c <"$brief" | tr -d ' ')
  max_kb=$(config_get --key dispatch.max-inline-input-kb --default 64)
  [[ "$max_kb" =~ ^[1-9][0-9]*$ ]] || die 1 "dispatch.max-inline-input-kb must be a positive integer"
  (( input_bytes <= max_kb * 1024 )) || die 1 "inline input exceeds dispatch.max-inline-input-kb; pass a file reference in the brief"
  input_hash=$(sha256_file "$brief")
  payload="$agents/$job"; round_dir="$payload/rounds/1"
  mkdir -p "$round_dir"
  cp "$brief" "$payload/brief.md"
  write_prompt "$round_dir/prompt.md" "$job" "$role" "$runtime" "$model" 1 "${mission:-none}" "$brief"

  record_json=$(mktemp "$record_locks/record.XXXXXX")
  python3 - "$record_json" "$job" "$role" "$mission" "$mission_turn" "$runtime" "$workspace" "$cap" "$model" "$overridden" "$snapshot_path" "$input_bytes" "$input_hash" "$permission_json" "$fallbacks" "$signal" <<'PY'
import json, subprocess, sys
from datetime import datetime, timezone
from pathlib import Path
out, job, role, mission, mission_turn, runtime, workspace, cap, model, overridden, snapshot, size, digest, permissions, fallbacks, signal = sys.argv[1:]
try: base = subprocess.check_output(["git", "-C", workspace, "rev-parse", "HEAD"], text=True).strip()
except subprocess.SubprocessError: raise SystemExit("workspace is not a git worktree")
branch = subprocess.check_output(["git", "-C", workspace, "branch", "--show-current"], text=True).strip()
record = {
  "jobId": job, "role": role, "mission": mission or None, "runtime": runtime,
  "round": 1, "parentJob": None, "status": "pending", "phase": "handshake", "error": None,
  "workspaceRoot": str(Path(workspace).resolve()), "baseSha": base, "branch": branch,
  "permissions": {"requested": json.loads(Path(permissions).read_text()), "effective": None},
  "capMin": int(cap), "pid": None, "pidStartedAt": None, "pgid": None, "instanceTag": f"metasystem-job-{job}",
  "custodyProcesses": [],
  "sessionId": None, "turnId": mission_turn or None, "requestedModel": model, "effectiveModel": None,
  "overridden": overridden == "true", "capabilitySnapshot": snapshot,
  "capabilityFallbacks": json.loads(fallbacks), "sessionEstablishedSignal": signal == "true",
  "input": {"bytes": int(size), "hash": digest, "delivery": "stdin"},
  "startedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"), "endedAt": None,
  "usage": None, "mirror": None, "chainClosed": False,
}
Path(out).write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
PY
  if [[ -n "$mission" ]]; then
    "$mission_fence" reserve-job --repo "$root" --mission "$mission" --job "$job" --cap-min "$cap" \
      || die 1 "mission dispatch refused by a lifecycle fence"
  fi
  record_create "$job" "$record_json"
  release_chain_lock "$job"; trap - EXIT
  launch_adapter "$runtime" dispatch "$job" "metasystem-job-$job" || {
    patch=$(mktemp "$record_locks/launch-failed.XXXXXX"); printf '{"error":"launch_failed"}\n' >"$patch"; record_cas "$job" pending failed "$patch" || true; return 3; }
  await_handshake "$job" "$(json_field "$snapshot_json" sessionEstablishedTimeoutSec)" || return 3
  if (( wait )); then wait_for_job "$job"; return $?; fi
  printf '%s\n' "$job"
}

follow_up() {
  local job= message= wait=0 root_id latest status error session role runtime model round child payload round_dir cap permission_json snapshot_json snapshot_path fallbacks signal resume_cap record_json mission mission_data lease mission_turn
  local resume_mode=resumed adapter_verb=follow-up delivery_content parent_round
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --message) [[ $# -ge 2 ]] || { usage; exit 2; }; message=$2; shift 2 ;;
      --wait) wait=1; shift ;;
      *) usage; exit 2 ;;
    esac
  done
  valid_id "$job" && [[ -f "$message" && -f "$jobs/$job.json" ]] || { usage; exit 2; }
  require_fresh_census
  report_plan_drift
  root_id=$(root_job_id "$job") || die 1 "cannot resolve the job chain"
  acquire_chain_lock "$root_id"; trap 'release_chain_lock "$root_id"' EXIT
  [[ "$(json_field "$jobs/$root_id.json" chainClosed 2>/dev/null || true)" != true ]] || die 1 "job chain is closed"
  latest=$(latest_chain_record "$root_id") || die 1 "cannot find the newest chain record"
  status=$(json_field "$latest" status); error=$(json_field "$latest" error 2>/dev/null || true)
  if [[ "$status" == completed || ( "$status" == failed && "$error" == protocol_error ) ]]; then :; else
    die 1 "follow-up requires the newest record to be completed or failed with protocol_error; use a fresh dispatch after pending, running, timeout, or process-lost"
  fi
  session=$(json_field "$latest" sessionId 2>/dev/null || true)
  [[ -n "$session" && "$session" != null ]] || die 1 "follow-up has no resumable session id; use the fresh-context embed fallback"
  role=$(json_field "$latest" role); runtime=$(json_field "$latest" runtime); model=$(json_field "$latest" requestedModel)
  round=$(( $(json_field "$latest" round) + 1 )); child="$root_id-r$round"
  [[ ! -e "$jobs/$child.json" ]] || die 1 "follow-up job id collision: $child"
  mission=$(json_field "$latest" mission 2>/dev/null || true); [[ "$mission" == null ]] && mission=
  mission_turn=
  if [[ -n "$mission" ]]; then
    mission_data=$(resolve_mission "$mission")
    IFS='|' read -r mission lease mission_turn <<<"$mission_data"
  fi
  cap=$(json_field "$latest" capMin)
  permission_json=$(mktemp "$record_locks/follow-permissions.XXXXXX")
  json_field "$latest" permissions.requested >"$permission_json"
  snapshot_json=$(mktemp "$record_locks/follow-snapshot.XXXXXX")
  select_snapshot "$runtime" "$role" "$permission_json" "$snapshot_json"
  snapshot_path=$(json_field "$snapshot_json" path); fallbacks=$(json_field "$snapshot_json" fallbacks); signal=$(json_field "$snapshot_json" sessionEstablishedSignal); resume_cap=$(json_field "$snapshot_json" resume)
  payload="$agents/$root_id"; round_dir="$payload/rounds/$round"; mkdir -p "$round_dir"
  delivery_content=$message
  if [[ "$resume_cap" != true ]]; then
    resume_mode=fresh-context
    adapter_verb=dispatch
    parent_round=$(json_field "$latest" round)
    delivery_content="$round_dir/fresh-context.md"
    {
      printf '# Prior brief\n\n'; cat "$payload/brief.md"
      printf '\n\n# Prior return\n\n'; cat "$payload/rounds/$parent_round/return.json"
      printf '\n\n# Correction\n\n'; cat "$message"
    } >"$delivery_content"
  fi
  max_kb=$(config_get --key dispatch.max-inline-input-kb --default 64); input_bytes=$(wc -c <"$delivery_content" | tr -d ' ')
  (( input_bytes <= max_kb * 1024 )) || die 1 "inline input exceeds dispatch.max-inline-input-kb; pass a file reference in the message"
  input_hash=$(sha256_file "$delivery_content")
  write_prompt "$round_dir/prompt.md" "$child" "$role" "$runtime" "$model" "$round" "${mission:-none}" "$delivery_content"
  record_json=$(mktemp "$record_locks/follow-record.XXXXXX")
  python3 - "$latest" "$record_json" "$child" "$round" "$(basename "${latest%.json}")" "$snapshot_path" "$fallbacks" "$signal" "$resume_mode" "$input_bytes" "$input_hash" "$mission_turn" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
parent = json.loads(Path(sys.argv[1]).read_text()); out = Path(sys.argv[2])
job, round_number, parent_job, snapshot, fallbacks, signal, resume_mode, size, digest, mission_turn = sys.argv[3:]
record = {key: parent[key] for key in ("role", "mission", "runtime", "workspaceRoot", "baseSha", "branch", "permissions", "capMin", "requestedModel")}
record.update({
  "jobId": job, "round": int(round_number), "parentJob": parent_job, "status": "pending", "phase": "handshake", "error": None,
  "permissions": {"requested": parent["permissions"]["requested"], "effective": None}, "pid": None, "pidStartedAt": None, "pgid": None,
  "custodyProcesses": [],
  "instanceTag": f"metasystem-job-{job}", "sessionId": parent["sessionId"] if resume_mode == "resumed" else None,
  "turnId": mission_turn or None,
  "effectiveModel": None, "overridden": False, "capabilitySnapshot": snapshot,
  "capabilityFallbacks": json.loads(fallbacks), "sessionEstablishedSignal": signal == "true",
  "resumeMode": resume_mode,
  "input": {"bytes": int(size), "hash": digest, "delivery": "stdin"},
  "startedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"), "endedAt": None,
  "usage": None, "mirror": None,
})
out.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
PY
  if [[ -n "$mission" ]]; then
    "$mission_fence" check-job --repo "$root" --mission "$mission" --job "$child" --cap-min "$cap" \
      || die 1 "mission follow-up refused by a lifecycle fence"
    "$mission_fence" reserve-job --repo "$root" --mission "$mission" --job "$child" --cap-min "$cap" \
      || die 1 "mission follow-up refused by a lifecycle fence"
  fi
  record_create "$child" "$record_json"
  release_chain_lock "$root_id"; trap - EXIT
  launch_adapter "$runtime" "$adapter_verb" "$child" "metasystem-job-$child" || {
    patch=$(mktemp "$record_locks/follow-launch.XXXXXX"); printf '{"error":"launch_failed"}\n' >"$patch"; record_cas "$child" pending failed "$patch" || true; return 3; }
  await_handshake "$child" "$(json_field "$snapshot_json" sessionEstablishedTimeoutSec)" || return 3
  if (( wait )); then wait_for_job "$child"; return $?; fi
  printf '%s\n' "$child"
}

status_job() {
  local job= status
  [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }; job=$2
  valid_id "$job" || { usage; exit 2; }
  [[ -e "$jobs/$job.json" ]] || return 6
  status=$(json_field "$jobs/$job.json" status 2>/dev/null || true)
  case "$status" in
    pending|running|completed|failed|timeout|cancelled)
      printf '%s\n' "$status"
      surface_census_verdict >&2
      ;;
    *) return 7 ;;
  esac
}

surface_census_verdict() {
  local verdict="$agents/supervision/last-census.json" value completed age
  if [[ ! -f "$verdict" ]]; then echo "CENSUS verdict=ABSENT"; return; fi
  value=$(json_field "$verdict" verdict 2>/dev/null || echo UNREADABLE)
  completed=$(json_field "$verdict" completedAtEpoch 2>/dev/null || echo 0)
  age=$(( $(date +%s) - completed ))
  printf 'CENSUS verdict=%s age=%ss fingerprint=%s\n' "$value" "$age" "$(json_field "$verdict" fingerprint 2>/dev/null || echo unavailable)"
}

cancel_job() {
  local job=
  [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }; job=$2
  valid_id "$job" && [[ -f "$jobs/$job.json" ]] || die 1 "unknown job: $job"
  "$root/scripts/agents/adapters/$(json_field "$jobs/$job.json" runtime).sh" cancel --job "$job"
}

close_chain() {
  local job= root_id root_record status patch
  [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }; job=$2
  valid_id "$job" && [[ -f "$jobs/$job.json" ]] || die 1 "unknown job: $job"
  root_id=$(root_job_id "$job") || die 1 "cannot resolve job chain"
  [[ "$root_id" == "$job" ]] || die 1 "close requires the root job id: $root_id"
  acquire_chain_lock "$root_id"; trap 'release_chain_lock "$root_id"' EXIT
  root_record="$jobs/$root_id.json"
  python3 - "$root" "$root_id" <<'PY'
import hashlib, json, sys
from pathlib import Path
repo, root = Path(sys.argv[1]), sys.argv[2]
jobs = repo / "artifacts" / "agents" / "jobs"
records = []
for path in jobs.glob("*.json"):
    try: value = json.loads(path.read_text())
    except (OSError, ValueError): continue
    current = value; seen = set()
    while current.get("parentJob") is not None:
        parent = current.get("parentJob")
        if parent in seen: break
        seen.add(parent)
        try: current = json.loads((jobs / f"{parent}.json").read_text())
        except (OSError, ValueError): break
    if current.get("jobId") == root: records.append(value)
if not records or any(value.get("status") not in {"completed", "failed", "timeout", "cancelled"} for value in records):
    raise SystemExit("cannot close a chain with a non-terminal record")
mirror = json.loads((jobs / f"{root}.json").read_text()).get("mirror")
if not isinstance(mirror, dict): raise SystemExit("cannot close an unmirrored chain")
manifest_path = Path(mirror.get("path", "")) / "manifest.json"
try: manifest = json.loads(manifest_path.read_text())
except (OSError, ValueError): raise SystemExit("cannot close a chain without its durable manifest")
files = manifest.get("files", {})
for value in records:
    job = value["jobId"]; round_number = value["round"]
    if f"jobs/{job}.json" not in files: raise SystemExit(f"manifest does not cover job record {job}")
    if value.get("role") == "implementer":
        relative = f"rounds/{round_number}/diff.patch"
        source = repo / "artifacts" / "agents" / root / relative
        if relative not in files or not source.exists(): raise SystemExit(f"implementer diff.patch is not mirrored for {job}")
        digest = hashlib.sha256(source.read_bytes()).hexdigest()
        if files[relative].get("sha256") != digest: raise SystemExit(f"manifest has a stale implementer diff.patch for {job}")
PY
  status=$(json_field "$root_record" status)
  patch=$(mktemp "$record_locks/close.XXXXXX"); printf '{"chainClosed":true}\n' >"$patch"
  record_cas "$root_id" "$status" "$status" "$patch"
  release_chain_lock "$root_id"; trap - EXIT
}

reap_jobs() {
  local job= interval= supervision_heartbeat= supervision_tag=
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --interval) [[ $# -ge 2 ]] || { usage; exit 2; }; interval=$2; shift 2 ;;
      --heartbeat) [[ $# -ge 2 ]] || { usage; exit 2; }; supervision_heartbeat=$2; shift 2 ;;
      --instance-tag) [[ $# -ge 2 ]] || { usage; exit 2; }; supervision_tag=$2; shift 2 ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ -z "$job" ]] || valid_id "$job" || { usage; exit 2; }
  [[ -z "$interval" || "$interval" =~ ^[1-9][0-9]*$ ]] || { usage; exit 2; }
  [[ -z "$supervision_heartbeat" || ( -n "$interval" && -n "$supervision_tag" ) ]] || { usage; exit 2; }
  while true; do
    if [[ -n "$job" ]]; then reap_one "$job"; else
      mkdir -p "$jobs"
      for record in "$jobs"/*.json; do [[ -f "$record" ]] && reap_one "$(basename "${record%.json}")"; done
    fi
    if [[ -n "$supervision_heartbeat" ]]; then
      python3 - "$supervision_heartbeat" "$$" "$supervision_tag" "$process_census" <<'PY'
import json,os,subprocess,sys,tempfile,time
from pathlib import Path
p=Path(sys.argv[1]); pid=int(sys.argv[2]); tag=sys.argv[3]; helper=sys.argv[4]
started=int(subprocess.check_output([helper,"started-at","--pid",str(pid)],text=True).strip())
v={"function":"reaper","pid":pid,"pidStartedAt":started,"instanceTag":tag,"observedAtEpoch":int(time.time())}
p.parent.mkdir(parents=True,exist_ok=True); fd,t=tempfile.mkstemp(prefix=p.name+".",suffix=".tmp",dir=p.parent)
with os.fdopen(fd,"w") as h: json.dump(v,h,sort_keys=True); h.write("\n"); h.flush(); os.fsync(h.fileno())
os.replace(t,p)
PY
    fi
    [[ -n "$interval" ]] || break
    sleep "$interval"
  done
}

internal_register_custody() {
  local job= pid= record status tag started patch
  while (($#)); do
    case "$1" in --job) job=$2; shift 2 ;; --pid) pid=$2; shift 2 ;; *) exit 2 ;; esac
  done
  valid_id "$job" && [[ "$pid" =~ ^[1-9][0-9]*$ && -f "$jobs/$job.json" ]] || exit 2
  record="$jobs/$job.json"; status=$(json_field "$record" status); tag=$(json_field "$record" instanceTag)
  case "$status" in pending|running) ;; *) exit 1 ;; esac
  started=$("$process_census" started-at --pid "$pid") || exit 1
  patch=$(mktemp "$record_locks/custody.XXXXXX")
  python3 - "$record" "$patch" "$pid" "$started" "$tag" <<'PY'
import json,sys
from pathlib import Path
record=json.loads(Path(sys.argv[1]).read_text()); output=Path(sys.argv[2]); pid,start,tag=int(sys.argv[3]),int(sys.argv[4]),sys.argv[5]
items=[item for item in record.get("custodyProcesses",[]) if item.get("pid") != pid or item.get("pidStartedAt") != start]
items.append({"pid":pid,"pidStartedAt":start,"instanceTag":tag})
output.write_text(json.dumps({"custodyProcesses":items})+"\n")
PY
  record_cas "$job" "$status" "$status" "$patch"
}

internal_handshake() {
  local job= session= turn= model= effective= signal=
  while (($#)); do
    case "$1" in
      --job) job=$2; shift 2 ;; --session) session=$2; shift 2 ;; --turn) turn=$2; shift 2 ;;
      --model) model=$2; shift 2 ;; --effective) effective=$2; shift 2 ;; --signal) signal=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  valid_id "$job" && [[ -f "$effective" && -f "$jobs/$job.json" ]] || exit 2
  patch=$(mktemp "$record_locks/handshake-patch.XXXXXX")
  set +e
  python3 - "$jobs/$job.json" "$effective" "$session" "$turn" "$model" "$signal" "$patch" <<'PY'
import json, sys
from pathlib import Path
record = json.loads(Path(sys.argv[1]).read_text()); effective = json.loads(Path(sys.argv[2]).read_text())
session, turn, model, signal, output = sys.argv[3:]
requested = record["permissions"]["requested"]
errors = []
orders = {
    "network": {"deny": 0, "ask": 1, "allow": 2},
    "approvals": {"deny": 0, "ask": 1, "allow": 2},
    "tools": {"read-only": 0, "runtime-default": 1},
}
for field, order in orders.items():
    if field not in effective:
        continue
    requested_value, effective_value = requested[field], effective[field]
    if requested_value in order and effective_value in order:
        if order[effective_value] > order[requested_value]: errors.append(field)
    elif effective_value != requested_value:
        errors.append(field)
for field in ("readRoots", "writeRoots"):
    if field in effective and not set(effective[field]).issubset(set(requested[field])): errors.append(field)
if signal == "true" and not session: errors.append("sessionId")
patch = {
    "permissions": {"requested": requested, "effective": effective},
    "effectiveModel": model,
    "turnId": record.get("turnId") or turn or None,
}
if session: patch["sessionId"] = session
if errors:
    patch.update({"error": "permissions_mismatch:" + ",".join(errors) if errors != ["sessionId"] else "handshake_missing_session_id", "phase": "handshake"})
    target = "failed"
else:
    patch.update({"error": None, "phase": "running"}); target = "running"
Path(output).write_text(json.dumps({"target": target, "patch": patch}) + "\n")
PY
  py_status=$?
  set -e
  [[ $py_status -eq 0 ]] || exit 1
  target=$(json_field "$patch" target)
  body=$(mktemp "$record_locks/handshake-body.XXXXXX")
  json_field "$patch" patch >"$body"
  record_cas "$job" pending "$target" "$body"
  [[ "$target" == running ]]
}

internal_cancel() {
  local job=$1 record="$jobs/$1.json" status patch
  [[ -f "$record" ]] || exit 1
  process_instance_tag=${process_instance_tag:-$job}
  acquire_lifecycle_lock_until "$job" 5 || exit 1
  status=$(json_field "$record" status)
  case "$status" in pending|running) ;; *) release_lifecycle_lock "$job"; exit 0 ;; esac
  wind_down_group "$record" || { release_lifecycle_lock "$job"; exit 1; }
  patch=$(mktemp "$record_locks/cancel.XXXXXX"); printf '{"error":null,"phase":"cancelled","groupDeathProvenAt":"%s"}\n' "$(now_iso)" >"$patch"
  record_cas "$job" "$status" cancelled "$patch" || true
  mirror_record "$job" || true
  release_lifecycle_lock "$job"
}

# Lock-owning public commands re-exec once so their lease tag is part of the
# process command line and a contender can distinguish this process from PID
# reuse. Internal adapter callbacks never acquire a chain lock.
if [[ ${1:-} == __lock-owner ]]; then
  [[ $# -ge 3 ]] || exit 2
  process_instance_tag=$2; shift 2
elif [[ ${1:-} != __* ]]; then
  public=${1:-dispatch}
  [[ "$public" == dispatch || "$public" == follow-up || "$public" == close || "$public" == reap || "$public" == --* ]] && {
    tag="metasystem-lock-$$-$(date +%s)"
    exec "$0" __lock-owner "$tag" "$@"
  }
fi

command=${1:-}
if [[ "$command" == --* ]]; then command=dispatch; else shift || true; fi
case "$command" in
  dispatch) dispatch_job "$@" ;;
  follow-up) follow_up "$@" ;;
  status) status_job "$@" ;;
  cancel) cancel_job "$@" ;;
  close) close_chain "$@" ;;
  reap) reap_jobs "$@" ;;
  __record-create) atomic_record_python create "$@" ;;
  __record-cas) atomic_record_python cas "$@" ;;
  __handshake) internal_handshake "$@" ;;
  __cancel-owned) [[ ${1:-} == --job && $# -eq 2 ]] || exit 2; internal_cancel "$2" ;;
  __register-custody) internal_register_custody "$@" ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
