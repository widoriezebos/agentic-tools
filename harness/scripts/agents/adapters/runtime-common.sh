#!/usr/bin/env bash

# Shared lifecycle plumbing for the real CLI adapters. Runtime command lines,
# event parsing, permission construction, and identity stay in each adapter.

adapter_common_init() { # runtime
  runtime=$1
  root=$(cd "$(dirname "${BASH_SOURCE[1]}")/../../.." && pwd -P)
  dispatch="$root/scripts/agents/dispatch.sh"
  agents="$root/artifacts/agents"
  jobs="$agents/jobs"
}

field() { # json file, dotted field
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path
value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
for part in sys.argv[2].split("."):
    value = value[part]
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

parse_supervisor_args() {
  job= gate= instance_tag=
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || return 2; job=$2; shift 2 ;;
      --start-gate) [[ $# -ge 2 ]] || return 2; gate=$2; shift 2 ;;
      --instance-tag) [[ $# -ge 2 ]] || return 2; instance_tag=$2; shift 2 ;;
      *) return 2 ;;
    esac
  done
  [[ -n "$job" && -n "$gate" && -n "$instance_tag" ]]
}

root_job_id() { # job id
  python3 - "$jobs" "$1" <<'PY'
import json, sys
from pathlib import Path
jobs, job = Path(sys.argv[1]), sys.argv[2]
seen = set()
while True:
    if job in seen:
        raise SystemExit("cyclic job chain")
    seen.add(job)
    value = json.loads((jobs / f"{job}.json").read_text(encoding="utf-8"))
    parent = value.get("parentJob")
    if parent is None:
        print(job)
        break
    job = parent
PY
}

prepare_supervision() { # dispatch|follow-up and supervisor args
  adapter_verb=$1
  shift
  parse_supervisor_args "$@" || return 2
  record="$jobs/$job.json"
  while [[ ! -e "$gate" ]]; do sleep 0.01; done
  round=$(field "$record" round)
  root_job=$(root_job_id "$job")
  round_dir="$agents/$root_job/rounds/$round"
  prompt="$round_dir/prompt.md"
  log="$jobs/$job.log"
  raw="$round_dir/raw.out"
  events="$round_dir/events.jsonl"
  heartbeat="$agents/hb/$job"
  effective="$round_dir/effective-permissions.json"
  schema="$root/scripts/agents/schemas/$(field "$record" role).schema.json"
  workspace=$(field "$record" workspaceRoot)
  requested_model=$(field "$record" requestedModel)
  requested_session=$(field "$record" sessionId 2>/dev/null || true)
  [[ "$requested_session" != null ]] || requested_session=
  handshake_done=0
  mkdir -p "$round_dir" "$(dirname "$heartbeat")"
  printf '%s adapter supervisor started value=%s\n' "$runtime" "$instance_tag" >"$log"
  printf '{"pid":%s,"pgid":%s,"instanceTag":"%s"}\n' "$$" "$$" "$instance_tag" >"$heartbeat"
  python3 - "$record" "$effective" <<'PY'
import json, sys
from pathlib import Path
record = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
Path(sys.argv[2]).write_text(
    json.dumps(record["permissions"]["requested"], indent=2, sort_keys=True) + "\n",
    encoding="utf-8",
)
PY
}

record_actual_workspace_write_scope() {
  python3 - "$effective" "$workspace" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1])
value = json.loads(path.read_text(encoding="utf-8"))
if value.get("writeRoots"):
    # All three baseline CLIs make their cwd/workspace the OS-sandbox write
    # boundary. A custom subdirectory-only request therefore fails the shared
    # effective-wider handshake instead of being recorded as falsely exact.
    value["writeRoots"] = [str(Path(sys.argv[2]).resolve())]
path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

fail_if_effective_wider_before_launch() {
  local mismatch
  mismatch=$(python3 - "$record" "$effective" <<'PY'
import json, sys
from pathlib import Path
record = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
effective = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
requested = record["permissions"]["requested"]
errors = []
for field in ("readRoots", "writeRoots"):
    if not set(effective.get(field, [])).issubset(set(requested[field])):
        errors.append(field)
orders = {
    "network": {"deny": 0, "ask": 1, "allow": 2},
    "approvals": {"deny": 0, "ask": 1, "allow": 2},
    "tools": {"read-only": 0, "runtime-default": 1},
}
for field, order in orders.items():
    if field in effective and order.get(effective[field], 999) > order.get(requested[field], -1):
        errors.append(field)
print(",".join(errors))
PY
  )
  [[ -z "$mismatch" ]] || {
    fail_pending "permissions_mismatch:$mismatch" handshake
    return 1
  }
}

record_handshake() { # session, turn, effective model
  local session=$1 turn=${2:-} model=${3:-$requested_model} signal
  [[ -n "$session" ]] || return 1
  if [[ -n "$requested_session" && "$session" != "$requested_session" ]]; then
    fail_pending resume_collision resume
    return 1
  fi
  signal=$(field "$record" sessionEstablishedSignal)
  "$dispatch" __handshake --job "$job" --session "$session" --turn "$turn" \
    --model "$model" --effective "$effective" --signal "$signal" || return 1
  session_id=$session
  effective_model=$model
  handshake_done=1
}

write_patch() { # output, error|null, phase, usage file
  python3 - "$1" "$2" "$3" "$4" <<'PY'
import json, sys
from pathlib import Path
output, error, phase, usage_path = sys.argv[1:]
usage = None
if usage_path and Path(usage_path).is_file():
    usage = json.loads(Path(usage_path).read_text(encoding="utf-8"))
Path(output).write_text(json.dumps({
    "error": None if error == "null" else error,
    "phase": phase,
    "usage": usage,
}) + "\n", encoding="utf-8")
PY
}

fail_pending() { # error, phase, optional usage file
  local error=$1 phase=$2 usage_file=${3:-} patch status
  patch="$round_dir/pending-failure.json"
  write_patch "$patch" "$error" "$phase" "$usage_file"
  set +e
  "$dispatch" __record-cas --job "$job" --expect pending --status failed --patch "$patch"
  status=$?
  set -e
  [[ $status -eq 0 || $status -eq 3 ]]
}

finish_running() { # completed|failed, error|null, phase, usage file
  local target=$1 error=$2 phase=$3 usage_file=$4 patch status
  patch="$round_dir/terminal-patch.json"
  write_patch "$patch" "$error" "$phase" "$usage_file"
  set +e
  "$dispatch" __record-cas --job "$job" --expect running --status "$target" --patch "$patch"
  status=$?
  set -e
  [[ $status -eq 0 || $status -eq 3 ]]
}

wait_for_cli() { # child pid; sets cli_status and keeps liveness sidecars fresh
  local child=$1 tick=0
  while kill -0 "$child" 2>/dev/null; do
    touch "$heartbeat"
    tick=$((tick + 1))
    (( tick % 10 != 0 )) || touch "$log"
    sleep 0.1
  done
  set +e
  wait "$child"
  cli_status=$?
  set -e
}

terminate_cli_child() { # exact child pid owned by this adapter
  local child=$1 deadline
  kill -TERM "$child" 2>/dev/null || true
  deadline=$(( $(date +%s) + 2 ))
  while kill -0 "$child" 2>/dev/null && (( $(date +%s) < deadline )); do sleep 0.05; done
  kill -KILL "$child" 2>/dev/null || true
  wait "$child" 2>/dev/null || true
}

normalize_return() { # candidate file, optional transcript file
  local candidate=$1 transcript=${2:-}
  python3 - "$candidate" "$transcript" "$record" "$round_dir/return.json" \
    "$round_dir/return.md" "$session_id" "$effective_model" <<'PY'
import json, re, sys
from pathlib import Path

candidate_path, transcript_path, record_path, output_path, markdown_path, session_id, effective_model = sys.argv[1:]
record = json.loads(Path(record_path).read_text(encoding="utf-8"))
required = {
    "jobId", "round", "runtime", "sessionId", "model", "evidence", "gaps", "mode"
}

def parse_text(text):
    values = []
    stripped = text.strip()
    if stripped:
        try:
            values.append(json.loads(stripped))
        except json.JSONDecodeError:
            pass
    for match in re.finditer(r"```(?:json)?\s*(.*?)```", text, re.I | re.S):
        try:
            values.append(json.loads(match.group(1).strip()))
        except json.JSONDecodeError:
            pass
    decoder = json.JSONDecoder()
    for index, char in enumerate(text):
        if char != "{":
            continue
        try:
            value, _ = decoder.raw_decode(text[index:])
            values.append(value)
        except json.JSONDecodeError:
            pass
    return values

def nested_values(value):
    yield value
    if isinstance(value, dict):
        if isinstance(value.get("structured_output"), dict):
            yield value["structured_output"]
        if isinstance(value.get("result"), str):
            yield from parse_text(value["result"])
        for child in value.values():
            if isinstance(child, (dict, list)):
                yield from nested_values(child)
            elif isinstance(child, str) and "{" in child:
                yield from parse_text(child)
    elif isinstance(value, list):
        for child in value:
            yield from nested_values(child)

sources = []
for path_text in (candidate_path, transcript_path):
    if not path_text:
        continue
    path = Path(path_text)
    if path.is_file():
        sources.extend(parse_text(path.read_text(encoding="utf-8", errors="replace")))

candidates = []
for source in sources:
    for value in nested_values(source):
        if isinstance(value, dict):
            score = len(required.intersection(value))
            if score:
                candidates.append((score, value))
if not candidates:
    raise SystemExit("no JSON return object found in runtime output")

result = dict(max(candidates, key=lambda item: item[0])[1])
if result.get("sessionId") is None:
    result["sessionId"] = session_id
model = result.get("model")
if isinstance(model, dict) and model.get("effective") is None:
    model = dict(model)
    model["effective"] = effective_model or None
    result["model"] = model

Path(output_path).write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
Path(markdown_path).write_text(
    "# Agent return\n\nCanonical JSON: return.json\n", encoding="utf-8"
)
PY
}

complete_from_cli() { # cli status, usage file, candidate file, optional transcript
  local status=$1 usage_file=$2 candidate=$3 transcript=${4:-}
  if (( status != 0 )); then
    if (( handshake_done )); then
      finish_running failed runtime_error runtime "$usage_file"
    else
      fail_pending runtime_error handshake "$usage_file"
    fi
    return 1
  fi
  if (( ! handshake_done )); then
    fail_pending handshake_missing_session_id handshake "$usage_file"
    return 1
  fi
  if ! normalize_return "$candidate" "$transcript" >>"$log" 2>&1; then
    finish_running failed protocol_error validation "$usage_file"
    return 1
  fi
  if "$root/scripts/assert-return-complete.sh" --job "$job" >>"$log" 2>&1; then
    finish_running completed null completed "$usage_file"
  else
    finish_running failed protocol_error validation "$usage_file"
    return 1
  fi
}

configuration_hash() { # declared settings files
  python3 - "$@" <<'PY'
import hashlib, sys
from pathlib import Path
digest = hashlib.sha256()
for raw in sys.argv[1:]:
    path = Path(raw).expanduser().resolve(strict=False)
    digest.update(str(path).encode())
    digest.update(b"\0")
    try:
        digest.update(path.read_bytes())
    except OSError:
        digest.update(b"<missing>")
    digest.update(b"\0")
print(digest.hexdigest()[:24])
PY
}

write_capability_snapshot() { # runtime version hash transports caps permissions
  local snapshot_runtime=$1 version=$2 config_hash=$3 transports=$4 caps=$5 permissions=$6
  mkdir -p "$agents/capabilities"
  python3 - "$agents/capabilities" "$snapshot_runtime" "$version" "$config_hash" \
    "$transports" "$caps" "$permissions" <<'PY'
import json, re, sys
from datetime import datetime, timezone
from pathlib import Path

directory = Path(sys.argv[1])
runtime, version, config_hash = sys.argv[2:5]
transports, capabilities, permissions = map(json.loads, sys.argv[5:8])
captured = datetime.now(timezone.utc)
date = captured.strftime("%Y%m%d")
prefix = f"{runtime}-{version}-{config_hash}-{date}-"
sequences = []
for path in directory.glob(prefix + "*.json"):
    match = re.fullmatch(re.escape(prefix) + r"(\d{3})\.json", path.name)
    if match:
        sequences.append(int(match.group(1)))
sequence = max(sequences, default=0) + 1
path = directory / f"{runtime}-{version}-{config_hash}-{date}-{sequence:03d}.json"
value = {
    "runtime": runtime,
    "cliVersion": version,
    "configHash": config_hash,
    "capturedAt": captured.strftime("%Y-%m-%dT%H:%M:%SZ"),
    "sequence": sequence,
    "transports": transports,
    "capabilities": capabilities,
    "permissions": permissions,
}
with path.open("x", encoding="utf-8") as handle:
    json.dump(value, handle, indent=2, sort_keys=True)
    handle.write("\n")
print(path)
PY
}

make_selftest_brief() { # output, goal text
  local output=$1 goal=$2
  cat >"$output" <<EOF
Working Mode: design
Orchestrator Identity: adapter-selftest
Date: $(date -u +%Y-%m-%d)

# Goal

$goal

# Workspace

Use only the current scratch workspace. Do not modify plans.

# Inputs

The runtime role preamble and this brief are complete.

# Constraints

Keep the response short. Perform every explicitly requested probe.

# Expected Return

Return only schema-valid JSON for the design-critic role. Use no findings unless a requested probe fails.

# Acceptance Criteria

The requested behavior is visible in the evidence observations.

# Gap Rule

stop and report a gap; never fill it silently.
EOF
}

wait_for_selftest_job() { # job, maximum seconds
  local selftest_job=$1 limit=$2 start status
  start=$(date +%s)
  while true; do
    status=$($dispatch status --job "$selftest_job" 2>/dev/null || true)
    case "$status" in
      completed) return 0 ;;
      failed|timeout|cancelled) return 1 ;;
    esac
    "$dispatch" reap --job "$selftest_job" >/dev/null 2>&1 || true
    (( $(date +%s) - start < limit )) || return 1
    sleep 0.2
  done
}

selftest_usage_check() { # record, native|unavailable
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path
record = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
usage = record.get("usage")
assert isinstance(usage, dict), "job has no typed usage"
assert usage.get("availability") == sys.argv[2], usage
if sys.argv[2] == "native":
    assert isinstance(usage.get("inputTokens"), (int, float)), usage
    assert isinstance(usage.get("outputTokens"), (int, float)), usage
PY
}

run_full_contract_selftest() { # native|unavailable, optional devin flag
  local usage_expectation=$1 devin_checks=${2:-0}
  local selftest_dir selftest_id main_job follow_job cancel_job permission_job session follow_session
  local scratch_repo nonce request_log port_file port server_pid= result_file model skill_instruction=
  model=$($root/scripts/harness-config.sh get --key "role.default.model.$runtime" --default '')
  [[ -n "$model" && "$model" != *'<'* && "$model" != *'>'* ]] || {
    echo "selftest requires a filled role.default.model.$runtime in harness.conf" >&2
    return 1
  }
  "$0" identity >/dev/null
  "$0" probe >/dev/null
  selftest_dir=$(mktemp -d "${TMPDIR:-/tmp}/harness-$runtime-selftest.XXXXXX")
  selftest_id="$runtime-selftest-$(date -u +%Y%m%dt%H%M%Sz)-$$"
  scratch_repo="$selftest_dir/repo"
  mkdir -p "$scratch_repo"
  git -C "$scratch_repo" init -q
  nonce="$runtime-$RANDOM-$$"
  printf 'PERMITTED_READ:%s\n' "$nonce" >"$scratch_repo/permitted.txt"
  printf '# Scratch repository\n' >"$scratch_repo/README.md"
  if (( devin_checks )); then
    mkdir -p "$scratch_repo/skills/harness-selftest" "$scratch_repo/.agents/skills"
    printf '%s\n' '---' 'name: harness-selftest' 'description: Report the marker from this file.' '---' \
      '' "SYMLINKED_SKILL:$nonce" >"$scratch_repo/skills/harness-selftest/SKILL.md"
    ln -s ../../skills/harness-selftest "$scratch_repo/.agents/skills/harness-selftest"
  fi
  git -C "$scratch_repo" add .
  git -C "$scratch_repo" -c user.name=harness -c user.email=harness.invalid commit -qm selftest

  make_selftest_brief "$selftest_dir/brief.md" \
    "Read README.md, then return a valid empty-findings design critique proving the read in evidence."
  main_job="$selftest_id-main"
  "$dispatch" dispatch --role design-critic --brief "$selftest_dir/brief.md" \
    --runtime "$runtime" --workspace "$scratch_repo" --permissions none --job-id "$main_job" >/dev/null
  # Probe after the runtime has established its session, while the turn is live
  # or in its terminal delivery window. A pre-dispatch probe above is necessary
  # because capability snapshots gate dispatch itself.
  "$0" probe >/dev/null
  wait_for_selftest_job "$main_job" 240 || { echo "$runtime selftest dispatch failed" >&2; return 1; }
  "$root/scripts/assert-return-complete.sh" --job "$main_job"
  session=$(field "$jobs/$main_job.json" sessionId)
  selftest_usage_check "$jobs/$main_job.json" "$usage_expectation"

  cat >"$selftest_dir/follow.md" <<EOF
Finding Id: adapter-selftest-resume
Disposition: noted

# Finding Being Corrected

Return another empty-findings result and state in evidence that the existing runtime session was resumed.

# Disposition Reasoning and Evidence

Keep the original session identity and role return contract.

# Unchanged Return Contract

The original design-critic schema remains binding without additions, removals, or relaxations.

Schema: scripts/agents/schemas/design-critic.schema.json
EOF
  "$dispatch" follow-up --job "$main_job" --message "$selftest_dir/follow.md" >/dev/null
  follow_job="$main_job-r2"
  wait_for_selftest_job "$follow_job" 240 || { echo "$runtime selftest follow-up failed" >&2; return 1; }
  "$root/scripts/assert-return-complete.sh" --job "$follow_job"
  follow_session=$(field "$jobs/$follow_job.json" sessionId)
  [[ "$follow_session" == "$session" ]] || { echo "$runtime resumed a different session" >&2; return 1; }

  make_selftest_brief "$selftest_dir/cancel.md" \
    "Inspect repository files one at a time and continue until the orchestrator cancels this scratch turn."
  cancel_job="$selftest_id-cancel"
  "$dispatch" dispatch --role design-critic --brief "$selftest_dir/cancel.md" \
    --runtime "$runtime" --workspace "$scratch_repo" --permissions none --job-id "$cancel_job" >/dev/null
  "$dispatch" cancel --job "$cancel_job"
  [[ $($dispatch status --job "$cancel_job") == cancelled ]] \
    || { echo "$runtime selftest cancellation was not recorded" >&2; return 1; }

  request_log="$selftest_dir/network-requested"
  port_file="$selftest_dir/network-port"
  python3 - "$request_log" "$port_file" <<'PY' &
import socket, sys
from pathlib import Path
request_log, port_file = Path(sys.argv[1]), Path(sys.argv[2])
server = socket.socket()
server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
server.bind(("127.0.0.1", 0))
server.listen(1)
server.settimeout(180)
port_file.write_text(str(server.getsockname()[1]))
try:
    connection, _ = server.accept()
    request_log.write_text(connection.recv(4096).decode(errors="replace"))
    connection.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK")
    connection.close()
except TimeoutError:
    pass
finally:
    server.close()
PY
  server_pid=$!
  trap 'kill '"$server_pid"' 2>/dev/null || true; wait '"$server_pid"' 2>/dev/null || true' EXIT
  for _ in {1..100}; do [[ -s "$port_file" ]] && break; sleep 0.02; done
  [[ -s "$port_file" ]] || { echo "selftest network listener did not start" >&2; return 1; }
  port=$(<"$port_file")
  if (( devin_checks )); then
    skill_instruction=" Invoke the harness-selftest skill discovered through .agents/skills and include its SYMLINKED_SKILL marker in evidence."
  fi
  make_selftest_brief "$selftest_dir/permissions.md" \
    "Read permitted.txt and include its exact PERMITTED_READ marker in evidence. Attempt to create forbidden.txt. Attempt an HTTP GET to http://127.0.0.1:$port/$nonce. Record the observed outcome of each attempt in evidence.$skill_instruction"
  permission_job="$selftest_id-permissions"
  "$dispatch" dispatch --role design-critic --brief "$selftest_dir/permissions.md" \
    --runtime "$runtime" --workspace "$scratch_repo" --permissions none --job-id "$permission_job" >/dev/null
  wait_for_selftest_job "$permission_job" 240 || { echo "$runtime selftest permission probe failed" >&2; return 1; }
  kill "$server_pid" 2>/dev/null || true
  wait "$server_pid" 2>/dev/null || true
  trap - EXIT
  [[ ! -e "$scratch_repo/forbidden.txt" ]] || { echo "$runtime permission mapping allowed a forbidden write" >&2; return 1; }
  [[ ! -e "$request_log" ]] || { echo "$runtime permission mapping allowed denied network" >&2; return 1; }
  result_file="$agents/$permission_job/rounds/1/return.json"
  grep -Fq "PERMITTED_READ:$nonce" "$result_file" \
    || { echo "$runtime permission probe did not prove the permitted read" >&2; return 1; }
  if (( devin_checks )); then
    grep -Fq "SYMLINKED_SKILL:$nonce" "$result_file" \
      || { echo "devin did not prove symlinked .agents/skills discovery" >&2; return 1; }
  fi

  mkdir -p "$agents/selftests"
  python3 - "$agents/selftests/$selftest_id.json" "$runtime" "$main_job" \
    "$usage_expectation" "$devin_checks" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
path, runtime, job, usage, devin_checks = sys.argv[1:]
proven = [
    "dispatch", "return-validation", "resume-identity", "cancel",
    "permitted-read", "forbidden-write", "denied-network",
]
if usage == "native":
    proven.append("usage-extraction")
else:
    proven.append("usage-unavailable-recording")
if devin_checks == "1":
    proven.extend(["documented-exit-status-observation", "symlinked-skill-discovery"])
value = {
    "runtime": runtime,
    "job": job,
    "passedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "provenBehaviorally": proven,
    "permissionEnvelopeEvidence": {
        "behaviorallyProven": {
            "readRoots": "permitted-read",
            "writeRoots": "forbidden-write",
            "network": "denied-fetch",
        },
        "constructedOnly": ["approvals", "tools", "readRoots-completeness"],
    },
    "usageAvailability": usage,
}
if devin_checks == "1":
    value["residuals"] = ["local CLI usage remains unavailable by documented contract"]
Path(path).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  echo "$runtime adapter selftest passed: full protocol sequence, permission probes, and usage=$usage_expectation"
}
