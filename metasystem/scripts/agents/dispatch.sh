#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/dispatch.sh dispatch --role <role> --brief <file>
      [--mode <working-mode>] [--runtime claude|codex|devin|fake]
      [--model <model>] [--job-id <id>] [--reviews <implementer-job-id>]
      [--workspace <dir> | --worktree]
      [--permissions <preset|envelope-file>] [--mission <id>]
      [--approve-escalation] [--wait] [--cap-min N]
  scripts/agents/dispatch.sh --role <role> --brief <file> [dispatch options]
  scripts/agents/dispatch.sh follow-up --job <job-id> --message <file> [--wait]
  scripts/agents/dispatch.sh status --job <job-id>
  scripts/agents/dispatch.sh cancel --job <job-id>
  scripts/agents/dispatch.sh close --job <root-id> [--runner-closed]
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
capabilities="$agents/capabilities"
worktrees="$agents/worktrees"
process_instance_tag=
standing_reaper=0
# How long the reaper waits past a record's handshake budget before calling a
# unfinished-handshake job process-lost. See the handshake branch in reap_one_locked.
handshake_backstop_grace_sec=2
process_census="$root/scripts/agents/process-census.py"
arm_supervision="$root/scripts/agents/arm-supervision.sh"
mission_fence="$root/scripts/agents/mission-fence.py"
lease_helper="$root/scripts/agents/worktree-lease.py"
entry_caller_pid=$$
current_claim_epoch=
current_main_id=
current_caller_class=
lease_reentry=0

valid_id() { [[ "$1" =~ ^[a-z0-9][a-z0-9-]*$ ]]; }
now_iso() { date -u +%Y-%m-%dT%H:%M:%SZ; }
sha256_file() { shasum -a 256 "$1" | awk '{print $1}'; }

dispatch_fixture_wait_cap() { # base seconds; normal dispatch remains 1x
  local base=$1 scale_milli=${METASYSTEM_FIXTURE_CAP_SCALE_MILLI:-1000}
  [[ "$base" =~ ^[1-9][0-9]*$ && "$scale_milli" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "dispatch wait cap inputs must be positive integers"
  printf '%s\n' "$(( (base * scale_milli + 999) / 1000 ))"
}

milliseconds_to_sleep() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "poll interval must be a positive integer in milliseconds"
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
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
  local verdict="$agents/supervision/last-census.json" state="$agents/supervision/state.json" expected
  [[ -f "$verdict" ]] || die 1 "dispatch refused: census verdict is absent; run $arm_supervision --repo $repo_scope"
  python3 - "$verdict" "$state" "$arm_supervision" "$repo_scope" <<'PY' || exit $?
import json, re, sys, time
from pathlib import Path
try: value=json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
except (OSError,ValueError) as error: raise SystemExit(f"dispatch refused: census verdict is unreadable: {error}")
required={"schemaVersion","writer","verdict","completedAtEpoch","intervalSec","fingerprint","counts","inventory","diagnostics","errors"}
if not required.issubset(value) or value.get("schemaVersion") != 2 or value.get("writer") != "watch-background-jobs.sh":
    raise SystemExit("dispatch refused: census verdict schema or writer is invalid")
if value.get("verdict") == "CENSUS-FAILED":
    raise SystemExit("dispatch refused: last census verdict is CENSUS-FAILED")
if value.get("verdict") != "SUCCESS":
    raise SystemExit("dispatch refused: census verdict is not successful")
completed, interval = value.get("completedAtEpoch"), value.get("intervalSec")
if type(completed) is not int or type(interval) is not int or interval < 1:
    raise SystemExit("dispatch refused: census freshness fields are invalid")
age=int(time.time())-completed
window=min(2 * interval, 180)
census_generation, state_digest = value.get("generation"), value.get("stateDigest")
if type(census_generation) is not int or census_generation < 1 or not isinstance(state_digest,str) or not re.fullmatch(r"[0-9a-f]{64}",state_digest):
    raise SystemExit("dispatch refused: census generation fields are invalid")
try: armed=json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
except (OSError,ValueError) as error: raise SystemExit(f"dispatch refused: arming record is unreadable: {error}")
armed_generation=armed.get("generation") if isinstance(armed,dict) else None
if type(armed_generation) is not int or armed_generation < 1:
    raise SystemExit("dispatch refused: arming record generation is invalid")
if census_generation != armed_generation:
    raise SystemExit(
        f"dispatch refused: census verdict is stale (age={age}s window={window}s "
        f"censusGeneration={census_generation} armedGeneration={armed_generation}); "
        f"retry in a moment; re-arm with {sys.argv[3]} --repo {sys.argv[4]} if supervision is dead"
    )
if age >= window:
    raise SystemExit(
        f"dispatch refused: census verdict is stale (age={age}s window={window}s); "
        f"retry in a moment; re-arm with {sys.argv[3]} --repo {sys.argv[4]} if supervision is dead"
    )
PY
  expected=$("$arm_supervision" fingerprint --repo "$repo_scope" 2>&1) \
    || die 1 "dispatch refused: census fingerprint cannot be computed: $expected"
  [[ "$(json_field "$verdict" fingerprint 2>/dev/null || true)" == "$expected" ]] \
    || die 1 "dispatch refused: census fingerprint does not match the armed code, signatures, and configuration"
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

record_setup() { # job, complete source json
  "$0" __record-setup --job "$1" --source "$2"
}

lease_entry_check() {
  local result
  result=$("$lease_helper" --root "$root" require-holder --caller-pid "$entry_caller_pid") \
    || exit $?
  current_claim_epoch=$(python3 -c 'import json,sys; v=json.loads(sys.argv[1]); print("" if v.get("claimEpoch") is None else v["claimEpoch"])' "$result")
  current_main_id=$(python3 -c 'import json,sys; v=json.loads(sys.argv[1]); print("" if v.get("mainId") is None else v["mainId"])' "$result")
  current_caller_class=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["class"])' "$result")
}

lease_run_held() { # expected epoch (empty for human), command...
  local expected=$1
  shift
  if [[ -n "$expected" ]]; then
    "$lease_helper" --root "$root" run-held --caller-pid "$entry_caller_pid" \
      --expected-epoch "$expected" -- "$@"
  else
    "$lease_helper" --root "$root" run-held --caller-pid "$entry_caller_pid" -- "$@"
  fi
}

internal_authority() { # holder-only|record-writer|adapter-writer|supervision-only, optional job id
  local mode=$1 job=${2:-} result
  result=$("$lease_helper" --root "$root" classify --caller-pid "$entry_caller_pid") \
    || die 1 "control-plane write refused: caller classification failed"
  if [[ -n "$job" ]]; then
    "$root/scripts/agents/control-plane-authority.py" \
      --mode "$mode" --classification "$result" --job "$job"
  else
    "$root/scripts/agents/control-plane-authority.py" \
      --mode "$mode" --classification "$result"
  fi
}

atomic_record_python() {
  python3 - "$root" "$@" <<'PY'
import fcntl
import hashlib
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
        if not isinstance(record, dict) or record.get("jobId") != job or record.get("status") != "pending-setup":
            print(f"invalid initial record identity or status for {job}", file=sys.stderr)
            raise SystemExit(1)
    elif operation == "setup":
        try:
            current = json.loads(record_path.read_text(encoding="utf-8"))
            record = json.loads(Path(values["source"]).read_text(encoding="utf-8"))
        except (OSError, ValueError) as error:
            print(f"cannot complete setup for job record {job}: {error}", file=sys.stderr)
            raise SystemExit(1)
        if (not isinstance(current, dict) or current.get("status") != "pending-setup"
                or not isinstance(record, dict) or record.get("jobId") != job
                or record.get("status") != "pending"
                or record.get("claimEpoch") != current.get("claimEpoch")
                or record.get("mainId") != current.get("mainId")):
            print(f"invalid setup transition for {job}", file=sys.stderr)
            raise SystemExit(1)
    elif operation == "protocol-error":
        try:
            record = json.loads(record_path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as error:
            print(f"cannot record protocol error for {job}: {error}", file=sys.stderr)
            raise SystemExit(1)
        expected = values.get("expect")
        violation = values.get("violation", "")
        if not violation and values.get("violation-file"):
            try:
                violation = Path(values["violation-file"]).read_text(encoding="utf-8").strip()
            except OSError as error:
                print(f"cannot read protocol violation for {job}: {error}", file=sys.stderr)
                raise SystemExit(1)
        if not violation:
            print(f"protocol violation text is empty for {job}", file=sys.stderr)
            raise SystemExit(1)
        key = hashlib.sha256(f"{job}{record.get('round')}{violation}".encode()).hexdigest()[:16]
        existing = record.get("protocolError")
        if record.get("status") == "failed" and record.get("error") == "protocol_error" \
                and isinstance(existing, dict) and existing.get("key") == key:
            raise SystemExit(0)
        if record.get("status") != expected or expected not in {"pending", "running"}:
            raise SystemExit(3)
        record.update({
            "status": "failed", "error": "protocol_error", "phase": "validation",
            "protocolError": {"key": key, "violation": violation, "detectedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")},
            "endedAt": record.get("endedAt") or datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        })
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
            "pending-setup": {"failed"},
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
        immutable = {"jobId", "role", "runtime", "round", "parentJob", "reviews", "workspaceRoot", "baseSha", "branch", "startedAt", "claimEpoch", "mainId"}
        if immutable.intersection(patch):
            print("record patch attempts to change immutable identity", file=sys.stderr)
            raise SystemExit(1)
        terminal = {"completed", "failed", "cancelled", "timeout"}
        if current in terminal and metadata_update and not set(patch).issubset({"mirror", "chainClosed", "chainUsage", "runnerClosed", "critiqueExhaustions"}):
            print("terminal record metadata is final except mirror, closure, aggregate usage, and critique exhaustion", file=sys.stderr)
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

# One primitive for both directory locks, because both got the same two rules
# wrong. A claim publishes the directory and its owner in ONE step: a directory
# rename replaces only an EMPTY directory, so it claims an absent lock, heals an
# ownerless husk left by an older crash, and refuses an owned one. Creating the
# directory first and writing the owner second left a window in which a
# contender read an ownerless lock and refused. A release frees only a lock this
# process still owns, and never fails when it no longer does -- a release that
# deletes whatever it finds hands a live owner's lock to a third writer.
owner_lock() { # claim|release, directory, pid, tag -> 0 done, 3 busy, 4 not-owner
  python3 - "$@" <<'PY'
import json, os, shutil, subprocess, sys, tempfile
from datetime import datetime, timezone
from pathlib import Path

command, directory, pid, tag = sys.argv[1], Path(sys.argv[2]), int(sys.argv[3]), sys.argv[4]
owner = directory / "owner.json"

def identity():
    try:
        value = json.loads(owner.read_text(encoding="utf-8"))
        return int(value["pid"]), str(value.get("instanceTag", ""))
    except (OSError, ValueError, KeyError, TypeError):
        return None

def holder_state(other_pid, other_tag):
    try:
        os.kill(other_pid, 0)
    except ProcessLookupError:
        return "dead"
    except PermissionError:
        return "live"
    result = subprocess.run(
        ["ps", "-p", str(other_pid), "-o", "command="],
        text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=False,
    )
    if result.returncode != 0:
        return "unknown"
    return "live" if other_tag and other_tag in result.stdout else "stale"

def owner_payload():
    return json.dumps(
        {"pid": pid, "instanceTag": tag,
         "acquiredAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")},
        sort_keys=True,
    ) + "\n"

if command == "release":
    current = identity()
    if current != (pid, tag):
        raise SystemExit(0 if current is None else 4)
    retiring = directory.parent / f"{directory.name}.retiring.{pid}"
    try:
        os.rename(directory, retiring)
    except OSError:
        raise SystemExit(0)
    shutil.rmtree(retiring, ignore_errors=True)
    raise SystemExit(0)

directory.parent.mkdir(parents=True, exist_ok=True)
for attempt in range(3):
    staging = Path(tempfile.mkdtemp(prefix=f"{directory.name}.claim.", dir=directory.parent))
    (staging / "owner.json").write_text(owner_payload(), encoding="utf-8")
    try:
        os.rename(staging, directory)
        raise SystemExit(0)
    except OSError:
        shutil.rmtree(staging, ignore_errors=True)
    current = identity()
    if current is None or holder_state(*current) in {"live", "unknown"}:
        raise SystemExit(3)
    husk = directory.parent / f"{directory.name}.dead.{pid}.{attempt}"
    try:
        os.rename(directory, husk)
    except OSError:
        continue
    shutil.rmtree(husk, ignore_errors=True)
raise SystemExit(3)
PY
}

acquire_chain_lock() { # root id
  local chain=$1 dir="$locks/$1.d" status=0 pid tag owner_state
  mkdir -p "$locks"
  owner_lock claim "$dir" "$$" "$process_instance_tag" || status=$?
  (( status == 0 )) && return 0
  pid=$(json_field "$dir/owner.json" pid 2>/dev/null || true)
  tag=$(json_field "$dir/owner.json" instanceTag 2>/dev/null || true)
  [[ -n "$pid" ]] || die 1 "chain lock has no owner lease: $dir"
  owner_state=$(lock_owner_state "$pid" "$tag")
  [[ "$owner_state" != unknown ]] || die 1 "chain lock owner liveness cannot be verified: $chain"
  die 1 "chain is busy: $chain"
}

release_chain_lock() { # root id
  local status=0
  owner_lock release "$locks/$1.d" "$$" "$process_instance_tag" || status=$?
  (( status == 4 )) && die 1 "refusing to release another owner's chain lock"
  return 0
}

acquire_lifecycle_lock() { # job id; nonzero means a live owner has it
  mkdir -p "$record_locks"
  owner_lock claim "$record_locks/$1.lifecycle.d" "$$" "$process_instance_tag"
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
  owner_lock release "$record_locks/$1.lifecycle.d" "$$" "$process_instance_tag" || true
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

# Tiers are read through the configuration resolver, not by parsing the
# template. Reading metasystem.conf directly made every tier in
# metasystem.conf.local invisible -- the file the system itself tells you to put
# local settings in -- so a dispatch refused as "unrankable" pointed the caller
# at a key they had already set, in the place they were told to set it.
# Tiers are contiguous from 1 -- config validation refuses a gap -- so scanning
# stops at the first missing index rather than at a fixed cap. A fixed cap
# silently ignored model.tier.<cap+1> and beyond; there is no cap now, and a
# non-contiguous config never reaches dispatch because validation rejects it.
model_tier() { # runtime, model; prints 999999 if absent or ambiguous
  local wanted="$1:$2" index=1 value found=0 rank=999999
  while value=$(config_get --key "model.tier.$index" --default ''); [[ -n "$value" ]]; do
    while IFS= read -r entry; do
      entry="${entry#"${entry%%[![:space:]]*}"}"
      entry="${entry%"${entry##*[![:space:]]}"}"
      [[ "$entry" == "$wanted" ]] || continue
      found=$((found + 1))
      rank=$index
    done < <(printf '%s\n' "${value//,/$'\n'}")
    index=$((index + 1))
  done
  (( found == 1 )) && printf '%s\n' "$rank" || printf '999999\n'
}

# The configured tier indices, enumerated from the MERGED config the resolver
# actually reads (base + .local), not probed over a fixed range. A fixed bound
# was itself a cap: a tier above it would be silently dropped even though config
# validation accepts any positive index. Enumerating the real keys removes the
# cap entirely.
configured_tier_indices() {
  "$config" keys --prefix model.tier. 2>/dev/null \
    | sed -n 's/^model\.tier\.\([1-9][0-9]*\)$/\1/p' | sort -n
}

model_tiers_configured() {
  [[ -n "$(configured_tier_indices)" ]]
}

# A gap is a config error, not a truncation: dispatch stops ranking at the first
# missing index, so a gap in the merged config would silently drop every tier
# above it. The set of present indices must be exactly 1..n.
assert_tiers_contiguous() {
  local indices expected=1 index
  indices=$(configured_tier_indices)
  for index in $indices; do
    if (( index != expected )); then
      die 1 "model tiers must be contiguous from 1: found index $index where $expected was expected (a gap would be silently ignored during ranking)"
    fi
    expected=$((expected + 1))
  done
}

signed_dispatch_envelope_allows() { # mission id, exact runtime:model pair
  python3 - "$root" "$1" "$2" <<'PY'
import importlib.util
import sys
from pathlib import Path

root, mission, requested = Path(sys.argv[1]), sys.argv[2], sys.argv[3]
module_path = root / "scripts" / "agents" / "mission-contract.py"
specification = importlib.util.spec_from_file_location("dispatch_mission_contract", module_path)
if specification is None or specification.loader is None:
    raise SystemExit(1)
module = importlib.util.module_from_spec(specification)
sys.modules[specification.name] = module
specification.loader.exec_module(module)
try:
    contract = module.read_contract(root / "plans" / f"mission-{mission}.contract.md")
    if not contract.sealed:
        module.fail("dispatch-allow envelope is not sealed")
    module.verify_approval(contract)
    module.verify_origin(contract, module.repository_for(contract.path))
    value = contract.values.get("envelope.dispatch-allow")
    allowed = module.validate_dispatch_allow(value) if value is not None else []
except module.ContractError as error:
    print(f"signed dispatch envelope unavailable: {error}", file=sys.stderr)
    raise SystemExit(1)
raise SystemExit(0 if requested in allowed else 1)
PY
}

confirm_escalation() { # roster pair, requested pair, displayed cost direction
  local roster_pair=$1 requested_pair=$2 cost_direction=$3 confirmation name
  printf 'Roster resolution: %s\n' "$roster_pair" >&2
  printf 'Requested pair: %s\n' "$requested_pair" >&2
  printf 'Cost direction: %s\n' "$cost_direction" >&2
  printf 'Type APPROVE <name> to confirm: ' >&2
  IFS= read -r confirmation || confirmation=
  if [[ "$confirmation" != "APPROVE "* ]]; then
    die 1 "escalation approval declined; re-run without the override, or repeat from an interactive TTY with --approve-escalation and type APPROVE <name>"
  fi
  name=${confirmation#APPROVE }
  python3 - "$name" <<'PY' || die 1 "escalation approval declined; type APPROVE followed by a non-empty name without leading, trailing, or control characters"
import sys
name = sys.argv[1]
raise SystemExit(0 if name and name == name.strip() and not any(ord(char) < 32 for char in name) else 1)
PY
  printf '%s\n' "$name"
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
  local runtime=$1 role=$2 envelope=$3 output=$4 adapter="$root/scripts/agents/adapters/$1.sh" identity max_age
  [[ -x "$adapter" ]] || die 1 "runtime adapter is not installed: $runtime"
  identity=$($adapter config-identity) || die 1 "could not read $runtime adapter configuration identity"
  max_age=$(config_get --key capability.snapshot-max-age-days --default 30)
  [[ "$max_age" =~ ^[0-9]+$ ]] || die 1 "capability.snapshot-max-age-days must be a non-negative integer"
  "$root/scripts/agents/select-capability-snapshot.py" \
    --root "$root" --runtime "$runtime" --role "$role" --identity "$identity" \
    --max-age "$max_age" --envelope "$envelope" --output "$output"
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
  local runtime=$1 verb=$2 job=$3 tag=$4 gate="$heartbeats/$job.start" adapter="$root/scripts/agents/adapters/$runtime.sh" pid pid_started patch cap started deadline elapsed poll_sleep
  poll_sleep=$(milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-20}")
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
    sleep "$poll_sleep"
  done
  patch=$(mktemp "$record_locks/launch.XXXXXX")
  # The handshake deadline is stamped HERE, at launch, because that is when the
  # dispatcher starts waiting. Derived from the record's creation time instead,
  # the reaper's copy of the same deadline ran early by however long setup took,
  # and the backstop then overwrote the dispatcher's own verdict.
  handshake_budget=$(json_field "$jobs/$job.json" sessionEstablishedTimeoutSec 2>/dev/null || echo 0)
  python3 - "$patch" "$pid" "$pid_started" "$tag" "$handshake_budget" <<'PY'
import json, sys, time
from datetime import datetime, timezone
from pathlib import Path
pid = int(sys.argv[2])
pid_started = int(sys.argv[3])
budget = int(sys.argv[5]) if sys.argv[5].isdigit() else 0
value = {
  "pid": pid, "pidStartedAt": pid_started, "pgid": pid,
  "ownershipProof": {"pid": pid, "pidStartedAt": pid_started, "pgid": pid, "instanceTag": sys.argv[4], "provenAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"), "source": "trusted-launcher"},
}
if budget > 0:
    value["handshakeDeadline"] = int(time.time()) + budget
Path(sys.argv[1]).write_text(json.dumps(value) + "\n")
PY
  record_cas "$job" pending pending "$patch" || return 1
  touch "$gate"
}

await_handshake() { # job, maximum session-established seconds, dispatch claim epoch
  local job=$1 timeout=$2 claim_epoch=${3:-} record="$jobs/$1.json" deadline status session poll_sleep
  [[ "$timeout" =~ ^[1-9][0-9]*$ && "$timeout" -le 60 ]] || return 1
  poll_sleep=$(milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-50}")
  # The deadline stamped at launch, so the waiter and the reaper's backstop
  # work from ONE number. Computing a fresh one here started the clock again
  # after setup had already run, which put this verdict later than the backstop
  # that is supposed to defer to it -- and the backstop won every time.
  deadline=$(json_field "$record" handshakeDeadline 2>/dev/null || true)
  [[ "$deadline" =~ ^[1-9][0-9]*$ ]] || deadline=$(( $(date +%s) + timeout ))
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
    sleep "$poll_sleep"
  done
  lease_run_held "$claim_epoch" "$0" __handshake-timeout --job "$job" || true
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
        lease_run_held "$current_claim_epoch" "$0" __reap-held --job "$job" \
          || return 3
        case "$status" in completed) return 0 ;; failed) return 3 ;; timeout) return 4 ;; cancelled) return 8 ;; esac
        ;;
      pending|running)
        if ! lease_run_held "$current_claim_epoch" "$0" __reap-held --job "$job"; then
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
  local aggregate_rc=0
  python3 - "$jobs" "$chain" "$patch" <<'PY' || aggregate_rc=$?
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
value = {"chainUsage": {"tokens": tokens, "cost": costs, "providerUnits": units}}
try:
    current = json.loads((jobs / f"{root}.json").read_text()).get("chainUsage")
except (OSError, ValueError):
    current = None
if current == value["chainUsage"]:
    raise SystemExit(7)
output.write_text(json.dumps(value, sort_keys=True) + "\n")
PY
  if (( aggregate_rc == 7 )); then
    rm -f -- "$patch" 2>/dev/null || true
    return 0
  fi
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
  if ! python3 - "$root" "$repo_scope" "$evidence" "$root_id" "$job" "$result" <<'PY'
import hashlib, json, os, shutil, sys, tempfile
from datetime import datetime, timezone
from pathlib import Path
repo, checkout, evidence, root_job, job, result = Path(sys.argv[1]), Path(sys.argv[2]), Path(sys.argv[3]), sys.argv[4], sys.argv[5], Path(sys.argv[6])
agents = repo / "artifacts" / "agents"
record_path = agents / "jobs" / f"{job}.json"
record = json.loads(record_path.read_text(encoding="utf-8"))
round_number = record["round"]
payload = agents / root_job
if (payload / ".mirror-fail-once").exists():
    (payload / ".mirror-fail-once").unlink()
    (payload / ".mirror-failed").write_text("scripted interruption\n")
    raise SystemExit("scripted mirror interruption")
checkout_segment = hashlib.sha256(str(checkout.resolve()).encode()).hexdigest()[:12]
destination = evidence / "agents" / checkout_segment / root_job
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
Path(result).write_text(json.dumps({"path": str(destination), "manifest": manifest_hash, "unchanged": bool(unchanged and manifest_path.exists()), "mirroredAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}) + "\n")
PY
  then
    mirror_fail "$job" "copy or verification failed (see stderr above)"
    return 1
  fi
  if [[ "$(python3 -c '
import json, sys
result = json.load(open(sys.argv[1]))
record = json.load(open(sys.argv[2]))
mirror = record.get("mirror") or {}
print(int(bool(result.get("unchanged")) and mirror.get("manifest") == result.get("manifest")))' "$result" "$record")" == 1 ]]; then
    rm -f -- "$result" 2>/dev/null || true
    return 0
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
  local job=$1 record="$jobs/$1.json" status pid tag started cap handshake_budget handshake_deadline session pending_age elapsed patch root_id mission record_epoch lease_epoch
  [[ -f "$record" ]] || return 0
  status=$(json_field "$record" status 2>/dev/null || true)
  case "$status" in
    completed|failed|timeout|cancelled)
      (( standing_reaper == 0 )) || return 0
      root_id=$(root_job_id "$job" 2>/dev/null || true)
      [[ -n "$root_id" ]] && aggregate_chain_usage "$root_id"
      aggregate_mission_usage "$record" || true
      mirror_record "$job" || true
      return
      ;;
    pending-setup|pending|running) ;;
    *) return ;;
  esac
  record_epoch=$(json_field "$record" claimEpoch 2>/dev/null || true)
  lease_epoch=$(json_field "$agents/mains/worktree-lease.json" claimEpoch 2>/dev/null || true)
  if (( standing_reaper )) && [[ "$record_epoch" =~ ^[0-9]+$ && "$lease_epoch" =~ ^[1-9][0-9]*$ ]] \
      && (( record_epoch < lease_epoch )); then
    if [[ "$status" != pending-setup ]]; then
      wind_down_group "$record" || return 1
    fi
    patch=$(mktemp "$record_locks/stale-epoch.XXXXXX")
    printf '{"error":"stale-claim-epoch","phase":"claim-sweep","groupDeathProvenAt":"%s"}\n' "$(now_iso)" >"$patch"
    record_cas "$job" "$status" failed "$patch" || true
    return 0
  fi
  if [[ "$status" == pending-setup ]]; then
    # No process has been launched for a pending-setup record, so reaping one
    # can orphan nothing. Its creating dispatcher finishes setup in seconds --
    # a record still here after ten minutes belongs to a dispatcher that died
    # between create and setup, and skipping it unconditionally left such
    # debris pending forever (one sat untouched for seven hours in a live
    # mission). The generous age keeps a slow live dispatcher unraced.
    created=$(json_field "$record" createdAt 2>/dev/null || true)
    if (( standing_reaper )) && [[ -n "$created" && "$created" != null ]] && python3 - "$created" <<'PY'
import sys
from datetime import datetime, timezone
try:
    created = datetime.fromisoformat(sys.argv[1].replace("Z", "+00:00"))
except ValueError:
    raise SystemExit(1)
age = (datetime.now(timezone.utc) - created).total_seconds()
raise SystemExit(0 if age > 600 else 1)
PY
    then
      patch=$(mktemp "$record_locks/abandoned-setup.XXXXXX")
      printf '{"error":"abandoned-setup","phase":"claim-sweep"}\n' >"$patch"
      record_cas "$job" pending-setup failed "$patch" || true
    fi
    return 0
  fi
  pid=$(json_field "$record" pid 2>/dev/null || true)
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  started=$(json_field "$record" startedAt 2>/dev/null || true)
  cap=$(json_field "$record" capMin 2>/dev/null || true)
  handshake_budget=$(json_field "$record" sessionEstablishedTimeoutSec 2>/dev/null || true)
  handshake_deadline=$(json_field "$record" handshakeDeadline 2>/dev/null || true)
  session=$(json_field "$record" sessionId 2>/dev/null || true)
  # A job is inside its handshake while it has no session, whether its record
  # still says pending or an adapter has already moved it to running. Reading
  # only pending left the running-without-a-session window unprotected, which
  # is precisely where a runtime that never signals ends up.
  if [[ ( "$status" == pending || ( "$status" == running && ( -z "$session" || "$session" == null ) ) ) \
      && "$handshake_budget" =~ ^[1-9][0-9]*$ ]]; then
    # The dispatcher owns the handshake verdict: it is the process that was
    # waiting, and it names the failure handshake_timeout. The reaper is the
    # backstop for a dispatcher that is no longer there, so it waits out the
    # dispatcher's own deadline -- the one stamped at launch -- rather than
    # recomputing a different one from the record's creation time.
    # The window defers to a dispatcher that is still waiting. A supervisor that
    # is provably gone will never complete a handshake, so there is nothing left
    # to defer to and waiting out the budget only delays the true diagnosis.
    #
    # "Provably gone" needs the record to name a supervisor first. Between
    # creating the record and the adapter publishing its identity there is no
    # pid to match, and treating that absence as death reaped every job in its
    # own launch window -- the supervisor had not died, it had not arrived.
    if [[ -z "$pid" || "$pid" == null ]] || job_supervisor_matches "$record"; then
    if [[ "$handshake_deadline" =~ ^[1-9][0-9]*$ ]]; then
      if (( $(date +%s) < handshake_deadline + handshake_backstop_grace_sec )); then
        return
      fi
    else
      pending_age=$(python3 - "$started" <<'PY'
from datetime import datetime, timezone
import sys
try: started = datetime.fromisoformat(sys.argv[1].replace("Z", "+00:00"))
except ValueError: raise SystemExit(1)
print(int((datetime.now(timezone.utc) - started).total_seconds()))
PY
      ) || true
      if [[ "$pending_age" =~ ^-?[0-9]+$ ]] \
        && (( pending_age < handshake_budget + handshake_backstop_grace_sec )); then
        return
      fi
    fi
    fi
  fi
  # The cap is judged BEFORE process liveness. An expired budget is a fact of
  # the record alone (startedAt + capMin); whether the job's process happens to
  # be dead by the time a reaper looks is scheduling noise. Judging liveness
  # first made the verdict a race: the same expired job read timeout from the
  # waiting dispatcher but process-lost from the standing reaper whenever its
  # process had already exited -- two different verdicts for one fact, and the
  # fence's job-cap-min refusal was skipped on the losing side.
  elapsed=$(python3 - "$started" <<'PY'
from datetime import datetime, timezone
import sys
try: started = datetime.fromisoformat(sys.argv[1].replace("Z", "+00:00"))
except ValueError: raise SystemExit(1)
print(int((datetime.now(timezone.utc) - started).total_seconds()))
PY
  ) || return 1
  # The priority applies only to a job that actually RAN: a pending job's
  # budget never started burning, its legal failure is process-lost or
  # handshake_timeout, and pending->timeout is not a lawful transition.
  if [[ "$status" != running ]] || ! [[ "$cap" =~ ^[1-9][0-9]*$ ]] || (( elapsed < cap * 60 )); then
    if ! job_supervisor_matches "$record"; then
      wind_down_group "$record" || return 1
      patch=$(mktemp "$record_locks/lost.XXXXXX")
      printf '{"error":"process-lost","phase":"supervision","groupDeathProvenAt":"%s"}\n' "$(now_iso)" >"$patch"
      record_cas "$job" "$status" failed "$patch" || true
      mirror_record "$job" || true
      return
    fi
  fi
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
  # A standing reaper that finds the lock busy simply comes back on its next
  # tick. An explicit `reap --job` has no next tick: skipping silently returns
  # success to a caller whose job was never looked at, which is how a reap that
  # raced the standing reaper reported nothing and changed nothing.
  if (( standing_reaper )); then
    acquire_lifecycle_lock "$job" || return 0
  else
    acquire_lifecycle_lock_until "$job" 5 || return 1
  fi
  set +e
  reap_one_locked "$job"
  result=$?
  set -e
  release_lifecycle_lock "$job"
  return "$result"
}

dispatch_job() {
  local role= brief= mode_override= runtime_override= model_override= job= reviews= workspace= permissions_override= mission_override= cap_override=
  local use_worktree=0 wait=0 approve_escalation=0 mode runtime model requested_model roster_runtime roster_model roster_pair requested_pair
  local overridden=false mission_data mission lease mission_turn cap watch_cap tiers_present=false roster_tier requested_tier escalation_required=0
  local cost_direction= approval_name= approved_at=
  local permission_name permission_json snapshot_json snapshot_path fallbacks signal handshake_budget input_bytes input_hash max_kb payload round_dir record_json setup_json
  while (($#)); do
    case "$1" in
      --role) [[ $# -ge 2 ]] || { usage; exit 2; }; role=$2; shift 2 ;;
      --brief) [[ $# -ge 2 ]] || { usage; exit 2; }; brief=$2; shift 2 ;;
      --mode) [[ $# -ge 2 ]] || { usage; exit 2; }; mode_override=$2; shift 2 ;;
      --runtime) [[ $# -ge 2 ]] || { usage; exit 2; }; runtime_override=$2; shift 2 ;;
      --model) [[ $# -ge 2 ]] || { usage; exit 2; }; model_override=$2; shift 2 ;;
      --job-id) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --reviews) [[ $# -ge 2 ]] || { usage; exit 2; }; reviews=$2; shift 2 ;;
      --workspace) [[ $# -ge 2 ]] || { usage; exit 2; }; workspace=$2; shift 2 ;;
      --worktree) use_worktree=1; shift ;;
      --permissions) [[ $# -ge 2 ]] || { usage; exit 2; }; permissions_override=$2; shift 2 ;;
      --mission) [[ $# -ge 2 ]] || { usage; exit 2; }; mission_override=$2; shift 2 ;;
      --cap-min) [[ $# -ge 2 ]] || { usage; exit 2; }; cap_override=$2; shift 2 ;;
      --approve-escalation) approve_escalation=1; shift ;;
      --wait) wait=1; shift ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ -n "$role" && -f "$brief" ]] || { usage; exit 2; }
  [[ -f "$root/scripts/agents/roles/$role.md" && -f "$root/scripts/agents/roles/$role.requirements.json" ]] || die 1 "unknown dispatch role: $role"
  if [[ "$role" == code-critic ]]; then
    [[ -n "$reviews" ]] || die 2 "code-critic dispatch requires --reviews <implementer-job-id>"
    valid_id "$reviews" || die 2 "invalid implementer job id for --reviews: $reviews"
    [[ -f "$jobs/$reviews.json" ]] || die 1 "code-critic dispatch cannot review unknown implementer job: $reviews"
    [[ "$(json_field "$jobs/$reviews.json" role 2>/dev/null || true)" == implementer ]] \
      || die 1 "code-critic dispatch --reviews must name an implementer job: $reviews"
  elif [[ -n "$reviews" ]]; then
    die 2 "--reviews is only valid for the code-critic role"
  fi
  [[ ! ( $use_worktree -eq 1 && -n "$workspace" ) ]] || die 2 "--workspace and --worktree are mutually exclusive"
  if (( approve_escalation )) && { [[ ! -t 0 ]] || [[ ! -t 2 ]]; }; then
    die 1 "--approve-escalation requires an interactive TTY; remove the flag or re-run the same dispatch from a TTY"
  fi
  mode=$(brief_mode "$brief") || die 1 "brief must contain exactly one filled Working Mode header"
  [[ -z "$mode_override" || "$mode_override" == "$mode" ]] || die 1 "--mode contradicts the brief's Working Mode header"
  lease_entry_check

  roster_runtime=$(config_get --key "role.$role.runtime" --mode "$mode" --default __missing__)
  [[ "$roster_runtime" != __missing__ ]] || roster_runtime=$(config_get --key role.default.runtime --mode "$mode" --default __missing__)
  runtime=${runtime_override:-$roster_runtime}
  [[ "$runtime" != __missing__ && -n "$runtime" ]] || die 1 "role $role has neither a runtime entry nor role.default.runtime"
  [[ "$runtime" != main ]] || die 1 "role $role is assigned to main and cannot be dispatched"
  registered_runtime "$runtime" || die 1 "runtime $runtime is outside metasystem.runtimes"
  if [[ "$roster_runtime" == main ]]; then
    roster_model='<current-session>'
  else
    roster_model=$(config_get --key "role.$role.model.$roster_runtime" --mode "$mode" --default __missing__)
    [[ "$roster_model" != __missing__ ]] || roster_model=$(config_get --key "role.default.model.$roster_runtime" --mode "$mode" --default __missing__)
    [[ "$roster_model" != __missing__ ]] || die 1 "role $role resolves to $roster_runtime but has no model.$roster_runtime value"
  fi
  roster_pair="$roster_runtime:$roster_model"
  requested_model=$(config_get --key "role.$role.model.$runtime" --mode "$mode" --default __missing__)
  [[ "$requested_model" != __missing__ ]] || requested_model=$(config_get --key "role.default.model.$runtime" --mode "$mode" --default __missing__)
  [[ "$requested_model" != __missing__ ]] || die 1 "role $role resolves to $runtime but has no model.$runtime value"
  model=${model_override:-$requested_model}
  requested_pair="$runtime:$model"
  [[ -z "$runtime_override" && -z "$model_override" ]] || overridden=true

  mission_data=$(resolve_mission "$mission_override")
  IFS='|' read -r mission lease mission_turn <<<"$mission_data"
  if [[ "$overridden" == true && "$requested_pair" != "$roster_pair" ]]; then
    if model_tiers_configured; then
      assert_tiers_contiguous
      tiers_present=true
      roster_tier=$(model_tier "$roster_runtime" "$roster_model")
      requested_tier=$(model_tier "$runtime" "$model")
      if [[ "$roster_tier" == 999999 || "$requested_tier" == 999999 ]]; then
        escalation_required=1
        cost_direction='unranked (one or both resolved pairs are absent from model.tier.*)'
      elif (( requested_tier > roster_tier )); then
        escalation_required=1
        cost_direction="higher (tier $roster_tier -> tier $requested_tier)"
      fi
    else
      escalation_required=1
      cost_direction='unranked (model tiers absent; overrides always escalate)'
    fi
  fi
  if (( escalation_required )); then
    if (( approve_escalation )); then
      approval_name=$(confirm_escalation "$roster_pair" "$requested_pair" "$cost_direction")
      approved_at=$(now_iso)
    elif [[ -n "$mission" ]] && signed_dispatch_envelope_allows "$mission" "$requested_pair"; then
      :
    elif [[ "$tiers_present" == false ]]; then
      die 1 "dispatch escalation refused: roster resolves to $roster_pair, requested pair is $requested_pair, and model tiers are absent. Configure model.tier.* to rank both pairs, add $requested_pair to a signed envelope.dispatch-allow mission contract, or re-run from a TTY with --approve-escalation."
    else
      die 1 "dispatch escalation refused: roster resolves to $roster_pair, requested pair is $requested_pair, cost direction is $cost_direction. Remove the override to use $roster_pair, add $requested_pair to a signed envelope.dispatch-allow mission contract, or re-run from a TTY with --approve-escalation."
    fi
  elif (( approve_escalation )); then
    die 1 "--approve-escalation is unnecessary because the requested pair does not require escalation approval; remove the flag"
  fi

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
  # Preconditions BEFORE the id is reserved. The reservation record exists to
  # stop two mains racing one job id; a refusal after it left a pending-setup
  # husk that burned that id permanently, so a stale census cost the caller
  # their chosen name as well as their dispatch.
  require_fresh_census
  report_plan_drift
  setup_json=$(mktemp "${TMPDIR:-/tmp}/metasystem-pending-setup.XXXXXX")
  python3 - "$setup_json" "$job" "$role" "$current_main_id" "$current_claim_epoch" <<'PY'
import json,sys
from datetime import datetime,timezone
from pathlib import Path
path,job,role,main_id,epoch=sys.argv[1:]
Path(path).write_text(json.dumps({
  "jobId":job,"role":role,"status":"pending-setup","phase":"setup",
  "error":None,"mainId":main_id or None,"claimEpoch":int(epoch) if epoch else None,
  "createdAt":datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
},indent=2,sort_keys=True)+"\n")
PY
  lease_run_held "$current_claim_epoch" "$0" __record-create --job "$job" --source "$setup_json"
  rm -f "$setup_json"
  mkdir -p "$jobs" "$record_locks" "$capabilities" "$worktrees"
  acquire_chain_lock "$job"
  trap 'release_chain_lock "$job"' EXIT
  [[ ! -e "$agents/$job" ]] || die 1 "job payload collision: $job"

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
  handshake_budget=$(json_field "$snapshot_json" sessionEstablishedTimeoutSec)

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
  python3 - "$record_json" "$job" "$role" "$mission" "$mission_turn" "$runtime" "$workspace" "$cap" "$model" "$overridden" "$snapshot_path" "$input_bytes" "$input_hash" "$permission_json" "$fallbacks" "$signal" "$handshake_budget" "$approval_name" "$approved_at" "$roster_pair" "$requested_pair" "$cost_direction" "$reviews" "$current_main_id" "$current_claim_epoch" <<'PY'
import json, subprocess, sys
from datetime import datetime, timezone
from pathlib import Path
out, job, role, mission, mission_turn, runtime, workspace, cap, model, overridden, snapshot, size, digest, permissions, fallbacks, signal, handshake_budget, approval_name, approved_at, roster_pair, requested_pair, cost_direction, reviews, main_id, claim_epoch = sys.argv[1:]
try: base = subprocess.check_output(["git", "-C", workspace, "rev-parse", "HEAD"], text=True).strip()
except subprocess.SubprocessError: raise SystemExit("workspace is not a git worktree")
branch = subprocess.check_output(["git", "-C", workspace, "branch", "--show-current"], text=True).strip()
escalation_approval = None
if approval_name:
    escalation_approval = {
        "name": approval_name,
        "approvedAt": approved_at,
        "rosterResolution": roster_pair,
        "requestedPair": requested_pair,
        "costDirection": cost_direction,
    }
record = {
  "jobId": job, "role": role, "mission": mission or None, "runtime": runtime,
  "round": 1, "parentJob": None, "reviews": reviews or None,
  "status": "pending", "phase": "handshake", "error": None,
  "mainId": main_id or None, "claimEpoch": int(claim_epoch) if claim_epoch else None,
  "workspaceRoot": str(Path(workspace).resolve()), "baseSha": base, "branch": branch,
  "permissions": {
    "requested": json.loads(Path(permissions).read_text()),
    "effective": None,
    "enforcementSnapshot": snapshot,
  },
  "capMin": int(cap), "pid": None, "pidStartedAt": None, "pgid": None, "instanceTag": f"metasystem-job-{job}",
  "custodyProcesses": [],
  "sessionId": None, "turnId": mission_turn or None, "requestedModel": model, "effectiveModel": None,
  "overridden": overridden == "true", "capabilitySnapshot": snapshot,
  "escalationApproval": escalation_approval,
  "capabilityFallbacks": json.loads(fallbacks), "sessionEstablishedSignal": signal == "true",
  "sessionEstablishedTimeoutSec": int(handshake_budget),
  "input": {"bytes": int(size), "hash": digest, "delivery": "stdin"},
  "startedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"), "endedAt": None,
  "usage": None, "mirror": None, "chainClosed": False, "runnerClosed": False,
  "critiqueExhaustions": [],
}
Path(out).write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
PY
  if [[ -n "$mission" ]]; then
    "$mission_fence" reserve-job --repo "$root" --mission "$mission" --job "$job" --cap-min "$cap" \
      || die 1 "mission dispatch refused by a lifecycle fence"
  fi
  lease_run_held "$current_claim_epoch" "$0" __record-setup --job "$job" --source "$record_json"
  release_chain_lock "$job"; trap - EXIT
  lease_run_held "$current_claim_epoch" "$0" __launch --runtime "$runtime" --verb dispatch \
    --job "$job" --tag "metasystem-job-$job" || {
    patch=$(mktemp "$record_locks/launch-failed.XXXXXX"); printf '{"error":"launch_failed"}\n' >"$patch"
    lease_run_held "$current_claim_epoch" "$0" __record-cas --job "$job" --expect pending --status failed --patch "$patch" || true
    rm -f "$patch"; return 3; }
  await_handshake "$job" "$handshake_budget" "$current_claim_epoch" || return 3
  if (( wait )); then wait_for_job "$job"; return $?; fi
  printf '%s\n' "$job"
}

critique_exhaustion_action() { # root job, role, latest record, message, successor id, output manifest
  python3 - "$root" "$1" "$2" "$3" "$4" "$5" "$6" <<'PY'
import json
import re
import sys
from pathlib import Path

repository, root_job, role, latest_path, message_path, successor, output_path = sys.argv[1:]
repository, latest_path, message_path, output_path = map(
    Path, (repository, latest_path, message_path, output_path)
)
jobs = repository / "artifacts" / "agents" / "jobs"
agents = repository / "artifacts" / "agents"


def fail(message):
    print(message, file=sys.stderr)
    raise SystemExit(1)


def load(path, description):
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        fail(f"{description} is unreadable: {error}")
    if not isinstance(value, dict):
        fail(f"{description} is not a JSON object")
    return value


records = {}
for path in jobs.glob("*.json"):
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        continue
    if isinstance(value, dict) and value.get("jobId") == path.stem:
        records[path.stem] = value


def chain_root(job_id):
    current = job_id
    seen = set()
    while current in records and current not in seen:
        seen.add(current)
        parent = records[current].get("parentJob")
        if parent is None:
            return current
        if not isinstance(parent, str):
            return None
        current = parent
    return None


def chain_members(chain_id):
    return [
        value for job_id, value in records.items() if chain_root(job_id) == chain_id
    ]


def latest_member(chain_id):
    members = [
        value
        for value in chain_members(chain_id)
        if isinstance(value.get("round"), int)
        and not isinstance(value.get("round"), bool)
    ]
    return max(members, key=lambda value: value["round"]) if members else None


def open_material_ids(record, chain_id):
    # The job record owns round identity. A delegate-returned round is data and
    # cannot decide whether a three-round budget has elapsed.
    round_number = record.get("round")
    if not isinstance(round_number, int) or isinstance(round_number, bool) or round_number < 1:
        fail(f"job record {record.get('jobId')!r} has an invalid round number")
    if record.get("status") == "failed" and record.get("error") == "protocol_error":
        return None
    if record.get("status") != "completed":
        return None
    return_path = agents / chain_id / "rounds" / str(round_number) / "return.json"
    result = load(return_path, f"critique return for job {record.get('jobId')!r}")
    findings = result.get("findings")
    if not isinstance(findings, list):
        fail(f"critique return for job {record.get('jobId')!r} has no findings array")
    found = []
    for item in findings:
        if not isinstance(item, dict) or item.get("material") is not True:
            continue
        finding_id = item.get("id")
        if isinstance(finding_id, str) and finding_id and finding_id not in found:
            found.append(finding_id)
    return found


def exhaustions(record):
    value = record.get("critiqueExhaustions", [])
    if not isinstance(value, list):
        fail("critiqueExhaustions is malformed; waiting on the human is the only remedy")
    if len(value) > 1:
        fail("a second critique exhaustion is refused outright; waiting on the human is the only remedy")
    return value


try:
    message = message_path.read_text(encoding="utf-8")
except OSError as error:
    fail(f"critique exhaustion successor message is unreadable: {error}")


def require_enumeration(open_ids):
    missing = [
        finding_id
        for finding_id in open_ids
        if re.search(
            rf"(?<![A-Za-z0-9_-]){re.escape(finding_id)}(?![A-Za-z0-9_-])",
            message,
        )
        is None
    ]
    if missing:
        fail(
            "critique budget exhausted; the implementer or design successor follow-up "
            "must enumerate every open finding identifier: " + ", ".join(missing)
        )


def entry(round_number, open_ids):
    return {
        "round": round_number,
        "openFindingIds": open_ids,
        "successorJobId": successor,
    }


latest = load(latest_path, "latest follow-up job record")
if latest.get("status") == "failed" and latest.get("error") == "protocol_error":
    # Protocol recovery deliberately does not read the missing or malformed
    # return that caused the protocol error.
    print("none")
    raise SystemExit(0)

actions = []
if role == "design-critic":
    round_number = latest.get("round")
    open_ids = open_material_ids(latest, root_job)
    if not open_ids or round_number % 3:
        print("none")
        raise SystemExit(0)
    current = records.get(root_job) or load(jobs / f"{root_job}.json", "critique root record")
    previous = exhaustions(current)
    if previous:
        if previous[0].get("round") == round_number and previous[0].get("successorJobId") == successor:
            print("none")
            raise SystemExit(0)
        fail("a second critique exhaustion is refused outright; waiting on the human is the only remedy")
    require_enumeration(open_ids)
    actions.append({"jobId": root_job, "critiqueExhaustions": [entry(round_number, open_ids)]})

elif role == "code-critic":
    round_number = latest.get("round")
    open_ids = open_material_ids(latest, root_job)
    if not open_ids or round_number % 3:
        print("none")
        raise SystemExit(0)
    current = records.get(root_job) or load(jobs / f"{root_job}.json", "critique root record")
    previous = exhaustions(current)
    if not previous:
        fail(
            "code critique budget exhausted; dispatch an implementer follow-up that enumerates "
            "every open finding identifier before continuing the code-critic chain: "
            + ", ".join(open_ids)
        )
    if previous[0].get("round") != round_number:
        fail("a second critique exhaustion is refused outright; waiting on the human is the only remedy")

elif role == "implementer":
    implementation_ids = {
        job_id for job_id in records if chain_root(job_id) == root_job
    }
    critic_roots = [
        value
        for value in records.values()
        if value.get("role") == "code-critic"
        and value.get("parentJob") is None
        and value.get("reviews") in implementation_ids
    ]
    for critic_root in critic_roots:
        critic_id = critic_root["jobId"]
        critic_latest = latest_member(critic_id)
        if critic_latest is None:
            continue
        round_number = critic_latest["round"]
        open_ids = open_material_ids(critic_latest, critic_id)
        if not open_ids or round_number % 3:
            continue
        previous = exhaustions(critic_root)
        if previous:
            if previous[0].get("round") == round_number:
                continue
            fail("a second critique exhaustion is refused outright; waiting on the human is the only remedy")
        require_enumeration(open_ids)
        actions.append({"jobId": critic_id, "critiqueExhaustions": [entry(round_number, open_ids)]})

if not actions:
    print("none")
    raise SystemExit(0)
output_path.write_text(json.dumps({"records": actions}, sort_keys=True) + "\n", encoding="utf-8")
print("record")
PY
}

record_critique_exhaustions() { # manifest
  local manifest=$1 index target patch target_status
  while IFS=$'\t' read -r index target; do
    patch=$(mktemp "$record_locks/exhaustion-record.XXXXXX")
    python3 - "$manifest" "$index" "$patch" <<'PY'
import json, sys
from pathlib import Path
manifest = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
item = manifest["records"][int(sys.argv[2])]
Path(sys.argv[3]).write_text(json.dumps({"critiqueExhaustions": item["critiqueExhaustions"]}) + "\n")
PY
    target_status=$(json_field "$jobs/$target.json" status)
    lease_run_held "$current_claim_epoch" "$0" __record-cas --job "$target" \
      --expect "$target_status" --status "$target_status" --patch "$patch" \
      || die 1 "could not record the critique exhaustion successor on code-critic chain $target"
    rm -f "$patch"
  done < <(python3 - "$manifest" <<'PY'
import json, sys
from pathlib import Path
value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
for index, item in enumerate(value["records"]):
    print(f"{index}\t{item['jobId']}")
PY
)
}

follow_up() {
  local job= message= wait=0 root_id latest status error session role runtime model workspace reviewed_commit round child payload round_dir cap permission_json snapshot_json snapshot_path fallbacks signal handshake_budget resume_cap record_json mission mission_data lease mission_turn
  local resume_mode=resumed adapter_verb=follow-up delivery_content parent_round setup_json
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --message) [[ $# -ge 2 ]] || { usage; exit 2; }; message=$2; shift 2 ;;
      --wait) wait=1; shift ;;
      *) usage; exit 2 ;;
    esac
  done
  valid_id "$job" && [[ -f "$message" && -f "$jobs/$job.json" ]] || { usage; exit 2; }
  lease_entry_check
  require_fresh_census
  report_plan_drift
  root_id=$(root_job_id "$job") || die 1 "cannot resolve the job chain"
  acquire_chain_lock "$root_id"; trap 'release_chain_lock "$root_id"' EXIT
  # A worktree chain reads its own branch, not main: a follow-up citing files
  # amended on main after the branch point describes files the delegate does
  # not have. This lesson (KI-9's complement) was violated three times as
  # prose before becoming this check.
  if worktree_path=$(json_field "$jobs/$root_id.json" workspaceRoot 2>/dev/null) \
      && [[ -n "$worktree_path" && "$worktree_path" != null && -d "$worktree_path" ]]; then
    trunk=$(git -C "$root" branch --show-current 2>/dev/null || true)
    behind=0
    [[ -n "$trunk" ]] && behind=$(git -C "$worktree_path" rev-list --count "HEAD..$trunk" 2>/dev/null || echo 0)
    if (( behind > 0 )); then
      echo "WORKTREE-BEHIND: the chain worktree is $behind commit(s) behind main; if this follow-up cites amended files, merge main into $worktree_path first" >&2
    fi
  fi
  [[ "$(json_field "$jobs/$root_id.json" chainClosed 2>/dev/null || true)" != true ]] || die 1 "job chain is closed"
  latest=$(latest_chain_record "$root_id") || die 1 "cannot find the newest chain record"
  status=$(json_field "$latest" status); error=$(json_field "$latest" error 2>/dev/null || true)
  if [[ "$status" == completed || ( "$status" == failed && "$error" == protocol_error ) ]]; then :; else
    die 1 "follow-up requires the newest record to be completed or failed with protocol_error; use a fresh dispatch after pending, running, timeout, or process-lost"
  fi
  session=$(json_field "$latest" sessionId 2>/dev/null || true)
  [[ -n "$session" && "$session" != null ]] || die 1 "follow-up has no resumable session id; use the fresh-context embed fallback"
  role=$(json_field "$latest" role); runtime=$(json_field "$latest" runtime); model=$(json_field "$latest" requestedModel)
  workspace=$(json_field "$latest" workspaceRoot)
  round=$(( $(json_field "$latest" round) + 1 )); child="$root_id-r$round"
  [[ ! -e "$jobs/$child.json" ]] || die 1 "follow-up job id collision: $child"
  setup_json=$(mktemp "${TMPDIR:-/tmp}/metasystem-follow-pending-setup.XXXXXX")
  python3 - "$setup_json" "$child" "$role" "$root_id" "$current_main_id" "$current_claim_epoch" <<'PY'
import json,sys
from datetime import datetime,timezone
from pathlib import Path
path,job,role,parent,main_id,epoch=sys.argv[1:]
Path(path).write_text(json.dumps({
  "jobId":job,"role":role,"parentJob":parent,"status":"pending-setup","phase":"setup",
  "error":None,"mainId":main_id or None,"claimEpoch":int(epoch) if epoch else None,
  "createdAt":datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
},indent=2,sort_keys=True)+"\n")
PY
  lease_run_held "$current_claim_epoch" "$0" __record-create --job "$child" --source "$setup_json"
  rm -f "$setup_json"
  if [[ "$role" == design-critic ]]; then
    # A critic's workspace is one of two different things, and treating them
    # alike broke the second. A WORKTREE of this repository is synchronised to
    # this repository's HEAD, because the design under review moved. A workspace
    # that is its OWN repository -- a benchmark target, a scratch checkout -- has
    # its own history, and this repository's HEAD is not a commit it has ever
    # heard of: merging it produced "not something we can merge". The test that
    # told them apart was "is the path different", which every separate
    # repository also satisfies. The test is now shared history.
    local workspace_git harness_git
    workspace_git=$( (cd "$workspace" && git rev-parse --git-common-dir 2>/dev/null) || true)
    harness_git=$( (cd "$repo_scope" && git rev-parse --git-common-dir 2>/dev/null) || true)
    [[ -n "$workspace_git" ]] && workspace_git=$( (cd "$workspace" && cd "$workspace_git" && pwd -P) 2>/dev/null || true)
    [[ -n "$harness_git" ]] && harness_git=$( (cd "$repo_scope" && cd "$harness_git" && pwd -P) 2>/dev/null || true)
    if [[ -n "$workspace_git" && "$workspace_git" == "$harness_git" ]]; then
      reviewed_commit=$(git -C "$repo_scope" rev-parse HEAD) \
        || die 1 "design-critic follow-up cannot resolve the current commit"
      if [[ "$(cd "$workspace" && pwd -P)" != "$repo_scope" ]]; then
        [[ -z "$(git -C "$workspace" status --porcelain)" ]] \
          || die 1 "design-critic follow-up cannot synchronize a dirty critic worktree"
        git -C "$workspace" merge --ff-only -q "$reviewed_commit" \
          || die 1 "design-critic follow-up cannot fast-forward its worktree to current commit $reviewed_commit"
      fi
    else
      # An independent repository reviews its own head, and nothing is merged
      # into it: this dispatcher does not own its history.
      reviewed_commit=$(git -C "$workspace" rev-parse HEAD) \
        || die 1 "design-critic follow-up cannot resolve the workspace commit"
    fi
  fi
  if [[ "$role" == implementer || "$role" == design-critic || "$role" == code-critic ]]; then
    exhaustion_patch=$(mktemp "$record_locks/exhaustion.XXXXXX")
    if ! exhaustion_action=$(critique_exhaustion_action \
      "$root_id" "$role" "$latest" "$message" "$child" "$exhaustion_patch" 2>&1); then
      die 1 "$exhaustion_action"
    fi
    if [[ "$exhaustion_action" == record ]]; then
      record_critique_exhaustions "$exhaustion_patch"
    fi
  fi
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
  snapshot_path=$(json_field "$snapshot_json" path); fallbacks=$(json_field "$snapshot_json" fallbacks); signal=$(json_field "$snapshot_json" sessionEstablishedSignal); handshake_budget=$(json_field "$snapshot_json" sessionEstablishedTimeoutSec); resume_cap=$(json_field "$snapshot_json" resume)
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
  python3 - "$latest" "$record_json" "$child" "$round" "$(basename "${latest%.json}")" "$snapshot_path" "$fallbacks" "$signal" "$handshake_budget" "$resume_mode" "$input_bytes" "$input_hash" "$mission_turn" "$current_main_id" "$current_claim_epoch" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
parent = json.loads(Path(sys.argv[1]).read_text()); out = Path(sys.argv[2])
job, round_number, parent_job, snapshot, fallbacks, signal, handshake_budget, resume_mode, size, digest, mission_turn, main_id, claim_epoch = sys.argv[3:]
record = {key: parent[key] for key in ("role", "mission", "runtime", "reviews", "workspaceRoot", "baseSha", "branch", "permissions", "capMin", "requestedModel")}
record.update({
  "jobId": job, "round": int(round_number), "parentJob": parent_job, "status": "pending", "phase": "handshake", "error": None,
  "mainId": main_id or None, "claimEpoch": int(claim_epoch) if claim_epoch else None,
  "permissions": {
    "requested": parent["permissions"]["requested"],
    "effective": None,
    "enforcementSnapshot": snapshot,
  }, "pid": None, "pidStartedAt": None, "pgid": None,
  "custodyProcesses": [],
  "instanceTag": f"metasystem-job-{job}", "sessionId": parent["sessionId"] if resume_mode == "resumed" else None,
  "turnId": mission_turn or None,
  "effectiveModel": None, "overridden": False, "capabilitySnapshot": snapshot,
  "capabilityFallbacks": json.loads(fallbacks), "sessionEstablishedSignal": signal == "true",
  "sessionEstablishedTimeoutSec": int(handshake_budget),
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
  lease_run_held "$current_claim_epoch" "$0" __record-setup --job "$child" --source "$record_json"
  release_chain_lock "$root_id"; trap - EXIT
  lease_run_held "$current_claim_epoch" "$0" __launch --runtime "$runtime" --verb "$adapter_verb" \
    --job "$child" --tag "metasystem-job-$child" || {
    patch=$(mktemp "$record_locks/follow-launch.XXXXXX"); printf '{"error":"launch_failed"}\n' >"$patch"
    lease_run_held "$current_claim_epoch" "$0" __record-cas --job "$child" --expect pending --status failed --patch "$patch" || true
    rm -f "$patch"; return 3; }
  await_handshake "$child" "$handshake_budget" "$current_claim_epoch" || return 3
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
  lease_entry_check
  lease_run_held "$current_claim_epoch" \
    "$root/scripts/agents/adapters/$(json_field "$jobs/$job.json" runtime).sh" cancel --job "$job"
}

close_chain() {
  local job= root_id root_record status patch runner_closed=false
  [[ ${1:-} == --job && $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2
  if [[ ${1:-} == --runner-closed && $# -eq 1 ]]; then
    runner_closed=true
    shift
  fi
  (($# == 0)) || { usage; exit 2; }
  lease_entry_check
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
  patch=$(mktemp "$record_locks/close.XXXXXX")
  if [[ "$runner_closed" == true ]]; then
    printf '{"chainClosed":true,"runnerClosed":true}\n' >"$patch"
  else
    printf '{"chainClosed":true}\n' >"$patch"
  fi
  lease_run_held "$current_claim_epoch" "$0" __record-cas --job "$root_id" \
    --expect "$status" --status "$status" --patch "$patch"
  rm -f "$patch"
  release_chain_lock "$root_id"; trap - EXIT
}

reap_jobs() {
  local job= interval= supervision_heartbeat= supervision_tag= start_gate= interval_ms interval_sleep gate_cap gate_started
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --interval) [[ $# -ge 2 ]] || { usage; exit 2; }; interval=$2; shift 2 ;;
      --heartbeat) [[ $# -ge 2 ]] || { usage; exit 2; }; supervision_heartbeat=$2; shift 2 ;;
      --instance-tag) [[ $# -ge 2 ]] || { usage; exit 2; }; supervision_tag=$2; shift 2 ;;
      --start-gate) [[ $# -ge 2 ]] || { usage; exit 2; }; start_gate=$2; shift 2 ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ -z "$job" ]] || valid_id "$job" || { usage; exit 2; }
  [[ -z "$interval" || "$interval" =~ ^[1-9][0-9]*$ ]] || { usage; exit 2; }
  [[ -z "$supervision_heartbeat" || ( -n "$interval" && -n "$supervision_tag" ) ]] || { usage; exit 2; }
  if [[ -z "$interval" && $lease_reentry -eq 0 ]]; then
    lease_entry_check
    if [[ -n "$job" ]]; then
      lease_run_held "$current_claim_epoch" "$0" __reap-held --job "$job"
    else
      lease_run_held "$current_claim_epoch" "$0" __reap-held
    fi
    return
  fi
  if [[ -n "$interval" ]]; then
    if [[ -n "$start_gate" ]]; then
      gate_cap=$(dispatch_fixture_wait_cap 10)
      gate_started=$SECONDS
      while [[ ! -e "$start_gate" ]]; do
        (( SECONDS - gate_started < gate_cap )) \
          || die 1 "standing reap start gate timed out before supervision custody was published"
        sleep 0.02
      done
      rm -f "$start_gate"
    fi
    internal_authority supervision-only
    standing_reaper=1
    interval_ms=${METASYSTEM_CENSUS_INTERVAL_MS:-$((interval * 1000))}
    interval_sleep=$(milliseconds_to_sleep "$interval_ms")
  fi
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
    sleep "$interval_sleep"
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
    "permissions": {
        "requested": requested,
        "effective": effective,
        "enforcementSnapshot": record["permissions"]["enforcementSnapshot"],
    },
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

internal_critique_exhaustion() {
  local root_job= role= latest= message= successor= output=
  while (($#)); do
    case "$1" in
      --root-job) root_job=$2; shift 2 ;;
      --role) role=$2; shift 2 ;;
      --latest) latest=$2; shift 2 ;;
      --message) message=$2; shift 2 ;;
      --successor) successor=$2; shift 2 ;;
      --output) output=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  valid_id "$root_job" && valid_id "$successor" \
    && [[ "$role" == implementer || "$role" == design-critic || "$role" == code-critic ]] \
    && [[ -f "$latest" && -f "$message" && -n "$output" ]] || exit 2
  critique_exhaustion_action "$root_job" "$role" "$latest" "$message" "$successor" "$output"
}

internal_reap_held() {
  internal_authority holder-only
  lease_reentry=1
  reap_jobs "$@"
}

internal_launch() {
  local runtime= verb= job= tag=
  while (($#)); do
    case "$1" in
      --runtime) runtime=$2; shift 2 ;;
      --verb) verb=$2; shift 2 ;;
      --job) job=$2; shift 2 ;;
      --tag) tag=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  [[ -n "$runtime" && ( "$verb" == dispatch || "$verb" == follow-up ) \
    && -n "$job" && -n "$tag" ]] || exit 2
  internal_authority holder-only "$job"
  launch_adapter "$runtime" "$verb" "$job" "$tag"
}

internal_handshake_timeout() {
  local job=
  [[ ${1:-} == --job && $# -eq 2 ]] || exit 2
  job=$2
  internal_authority holder-only "$job"
  local record="$jobs/$job.json" patch status session
  # An adapter that starts and then never signals a session leaves the record
  # in running, not pending. Writing the verdict only from pending meant this
  # -- the exact case the handshake timeout exists for -- wrote nothing at all,
  # and the reaper's backstop later called it process-lost instead.
  local attempt
  status=$(json_field "$record" status 2>/dev/null || true)
  # This write is the dispatcher's verdict on its own wait. Every step of it
  # used to fail silently, so a job that ended up diagnosed by the reaper's
  # backstop instead gave no clue which step dropped it. The job log says.
  printf '%s handshake-timeout entered status=%s\n' "$(now_iso)" "$status" >>"$jobs/$job.log"
  case "$status" in pending|running) ;; *) return 0 ;; esac
  # Stand down BEFORE killing anything if a session already landed. The waiter
  # gave up at the deadline and the adapter can record a session a moment later;
  # a session in the record means the wait was won, just late. Winding down the
  # group first killed that live, successful turn before this check could see it.
  session=$(json_field "$record" sessionId 2>/dev/null || true)
  if [[ -n "$session" && "$session" != null ]]; then
    printf '%s handshake-timeout stood down; session %s landed before wind-down\n' \
      "$(now_iso)" "$session" >>"$jobs/$job.log"
    return 0
  fi
  # Record the verdict BEFORE killing the group, not after. Winding down first
  # left a gap in which the reaper swept, saw the freshly-killed supervisor as
  # process-lost, and wrote that before this verdict landed -- the dispatcher
  # owns the handshake verdict, so it claims the record first and kills the
  # now-condemned group second. Compare-and-swap, so retry on a losing compare:
  # an adapter moves the record pending->running in exactly this window. The
  # record wrapper deletes the patch after each call, so each attempt makes its
  # own.
  local recorded=0
  for attempt in 1 2 3; do
    status=$(json_field "$record" status 2>/dev/null || true)
    case "$status" in
      pending|running) ;;
      *)
        printf '%s handshake-timeout stood down; record is already %s\n' "$(now_iso)" "$status" >>"$jobs/$job.log"
        return 0
        ;;
    esac
    session=$(json_field "$record" sessionId 2>/dev/null || true)
    if [[ -n "$session" && "$session" != null ]]; then
      printf '%s handshake-timeout stood down; session %s landed while it was being written\n' \
        "$(now_iso)" "$session" >>"$jobs/$job.log"
      return 0
    fi
    patch=$(mktemp "$record_locks/handshake.XXXXXX")
    printf '{"error":"handshake_timeout","phase":"handshake"}\n' >"$patch"
    if record_cas "$job" "$status" failed "$patch"; then
      printf '%s handshake-timeout recorded from %s\n' "$(now_iso)" "$status" >>"$jobs/$job.log"
      recorded=1
      break
    fi
  done
  if (( ! recorded )); then
    printf '%s handshake-timeout lost three compares; the record kept changing\n' "$(now_iso)" >>"$jobs/$job.log"
    return 1
  fi
  # The verdict stands; cleaning up the stalled group is best-effort now.
  wind_down_group "$record" \
    || printf '%s handshake-timeout recorded, but the group did not wind down cleanly\n' "$(now_iso)" >>"$jobs/$job.log"
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
  __record-create) internal_authority holder-only; atomic_record_python create "$@" ;;
  __record-setup) internal_authority holder-only; atomic_record_python setup "$@" ;;
  __record-cas)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority record-writer "$2"
    atomic_record_python cas "$@"
    ;;
  __protocol-error)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority adapter-writer "$2"
    atomic_record_python protocol-error "$@"
    ;;
  __launch) internal_launch "$@" ;;
  __handshake-timeout) internal_handshake_timeout "$@" ;;
  __reap-held) internal_reap_held "$@" ;;
  __handshake)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority adapter-writer "$2"
    internal_handshake "$@"
    ;;
  __cancel-owned)
    [[ ${1:-} == --job && $# -eq 2 ]] || exit 2
    internal_authority holder-only "$2"
    internal_cancel "$2"
    ;;
  __register-custody)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority adapter-writer "$2"
    internal_register_custody "$@"
    ;;
  __critique-exhaustion) internal_critique_exhaustion "$@" ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
