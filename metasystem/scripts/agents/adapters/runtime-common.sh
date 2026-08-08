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

adapter_milliseconds_to_sleep() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || { echo "$runtime adapter interval must be a positive integer in milliseconds" >&2; return 2; }
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
}

prepare_supervision() { # dispatch|follow-up and supervisor args
  local gate_poll
  adapter_verb=$1
  shift
  parse_supervisor_args "$@" || return 2
  record="$jobs/$job.json"
  gate_poll=$(adapter_milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-10}") || return 2
  while [[ ! -e "$gate" ]]; do sleep "$gate_poll"; done
  round=$(field "$record" round)
  root_job=$(root_job_id "$job")
  round_dir="$agents/$root_job/rounds/$round"
  prompt="$round_dir/prompt.md"
  log="$jobs/$job.log"
  raw="$round_dir/raw.out"
  events="$round_dir/events.jsonl"
  heartbeat="$agents/hb/$job"
  effective="$round_dir/effective-permissions.json"
  schema="$round_dir/return-schema.v2.json"
  "$root/scripts/agents/return-schema.py" --root "$root" --role "$(field "$record" role)" \
    --version 2 --output "$schema"
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

register_cli_custody() { # child pid
  local child_pid=$1 deadline=$((SECONDS + 5)) poll_sleep
  poll_sleep=$(adapter_milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-20}") || return 2
  while kill -0 "$child_pid" 2>/dev/null; do
    if "$dispatch" __register-custody --job "$job" --pid "$child_pid"; then return 0; fi
    (( SECONDS < deadline )) || {
      echo "$runtime child custody registration ceiling reached for pid $child_pid" >&2
      return 1
    }
    sleep "$poll_sleep"
  done
  echo "$runtime child exited before custody identity was recorded" >&2
  return 1
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

record_result_effective_model() { # effective model reported by the completed runtime result
  local model=$1 patch="$round_dir/result-model-patch.json"
  [[ -n "$model" ]] || return 2
  python3 - "$patch" "$model" <<'PY'
import json, sys
from pathlib import Path
Path(sys.argv[1]).write_text(json.dumps({"effectiveModel": sys.argv[2]}) + "\n", encoding="utf-8")
PY
  "$dispatch" __record-cas --job "$job" --expect running --status running --patch "$patch" || return 1
  effective_model=$model
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

finish_protocol_error() { # violation file
  local violation_file=$1 status
  set +e
  "$dispatch" __protocol-error --job "$job" --expect running --violation-file "$violation_file"
  status=$?
  set -e
  [[ $status -eq 0 || $status -eq 3 ]]
}

wait_for_cli() { # child pid; sets cli_status and keeps liveness sidecars fresh
  local child=$1 tick=0 heartbeat_sleep
  heartbeat_sleep=$(adapter_milliseconds_to_sleep "${METASYSTEM_HEARTBEAT_INTERVAL_MS:-100}") || return 2
  while kill -0 "$child" 2>/dev/null; do
    touch "$heartbeat"
    tick=$((tick + 1))
    (( tick % 10 != 0 )) || touch "$log"
    sleep "$heartbeat_sleep"
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
    "$round_dir/return.md" "$session_id" <<'PY'
import json, re, sys
from pathlib import Path

candidate_path, transcript_path, record_path, output_path, markdown_path, session_id = sys.argv[1:]
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
observed_session = session_id or "unobserved"
observed_model = record.get("effectiveModel") or "unobserved"
model = result.get("model")
if isinstance(model, dict):
    model = dict(model)
    claimed = dict(result.get("claimed")) if isinstance(result.get("claimed"), dict) else {}
    if result.get("sessionId") not in (None, observed_session) and isinstance(result.get("sessionId"), str):
        claimed["sessionId"] = result["sessionId"]
    if model.get("effective") not in (None, observed_model) and isinstance(model.get("effective"), str):
        claimed["model"] = model["effective"]
    result["sessionId"] = observed_session
    model["effective"] = observed_model
    result["model"] = model
    if result.get("schemaVersion") == 2:
        # Both members, always. The schema requires the object and both of its
        # members; null is how this family says "claimed nothing", and an
        # absent object is rejected by the provider that enforces the schema.
        result["claimed"] = {
            "sessionId": claimed.get("sessionId"),
            "model": claimed.get("model"),
        }

Path(output_path).write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
Path(markdown_path).write_text(
    "# Agent return\n\nCanonical JSON: return.json\n", encoding="utf-8"
)
PY
}

validate_candidate() { # candidate file, transcript, violation file
  local candidate=$1 transcript=$2 violation=$3
  if ! normalize_return "$candidate" "$transcript" >"$violation" 2>&1; then
    printf 'return normalization failed: ' | cat - "$violation" >"$violation.tmp"
    mv "$violation.tmp" "$violation"
    return 1
  fi
  "$root/scripts/assert-return-complete.sh" --job "$job" >"$violation" 2>&1
}

# One repair turn, in the SAME session, for a runtime that was never handed a
# schema by its CLI. A reply can be perfect work in the wrong shape, and burning
# the whole session over the shape is the wrong price -- but so is the harness
# renaming fields to make a return validate, because the fields it would be
# guessing at are the evidence a critique is judged on. So the delegate is shown
# its own violations and asked again, once, with everything it already did still
# in context.
#
# Bounded to one attempt: a delegate that cannot follow a schema it has just
# been handed twice does not get unbounded turns. Recorded either way, so a
# chain that needed a repair never reads as one that got it right first time.
attempt_return_repair() { # violation file -> 0 ran+usable, 1 ran+failed, 2 not attempted
  local violation=$1 repair_prompt="$round_dir/repair-1.prompt.md"
  declare -F runtime_repair_turn >/dev/null 2>&1 || return 2
  (( ${return_repairs:-0} == 0 )) || return 2
  [[ -n "${session_id:-}" ]] || return 2
  {
    printf 'Your previous reply did not validate against the required schema.\n'
    printf 'Everything you already did in this session still stands; only the\n'
    printf 'shape of the reply was wrong.\n\n# What failed\n\n'
    cat "$violation"
    printf '\n# The schema your reply must satisfy\n\n'
    cat "$schema"
    printf '\n# What to send now\n\n'
    printf 'Reply with ONE JSON object valid against that schema and nothing\n'
    printf 'else: no prose before or after it, no code fence, no property the\n'
    printf 'schema does not name, and every property listed in "required".\n'
    printf 'Do not repeat the work; report what you already found.\n'
  } >"$repair_prompt"
  printf '%s return repair attempt 1: reply did not validate, asking again in session %s\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$session_id" >>"$log"
  return_repairs=1
  runtime_repair_turn "$repair_prompt" "$round_dir/repair-1.out"
}

complete_from_cli() { # cli status, usage file, candidate file, optional transcript
  local status=$1 usage_file=$2 candidate=$3 transcript=${4:-} violation="$round_dir/protocol-violation.txt"
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
  if validate_candidate "$candidate" "$transcript" "$violation"; then
    rm -f "$violation"
    finish_running completed null completed "$usage_file"
    return 0
  fi
  cat "$violation" >>"$log"
  local repair_rc=0
  attempt_return_repair "$violation" || repair_rc=$?
  if (( repair_rc != 2 )); then
    # The repair RAN, whether or not it produced a usable reply. Record it and
    # re-fence its usage BEFORE branching on the outcome: a repair that failed
    # still spent provider budget on a cumulative-usage runtime, and its
    # transcript carries the up-to-date total. Leaving that to the success path
    # only dropped a failed repair's spend from the fences entirely.
    record_return_repairs 1
    if declare -F runtime_usage_after_repair >/dev/null 2>&1; then
      runtime_usage_after_repair "$usage_file" || true
    fi
    if (( repair_rc == 0 )) && validate_candidate "$round_dir/repair-1.out" "" "$violation"; then
      printf '%s return repaired in session %s; the first reply is kept as evidence\n' \
        "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$session_id" >>"$log"
      # The repair transcript is now the final turn, so it -- not the pre-repair
      # transcript -- is authoritative for session and model identity.
      if declare -F runtime_settle_after_repair >/dev/null 2>&1 \
          && ! runtime_settle_after_repair; then
        finish_running failed session_identity_disagreement delivery "$usage_file"
        return 1
      fi
      rm -f "$violation"
      finish_running completed null completed "$usage_file"
      return 0
    fi
  fi
  cat "$violation" >>"$log"
  finish_protocol_error "$violation"
  return 1
}

record_return_repairs() { # count
  local patch="$round_dir/repair-count-patch.json"
  python3 - "$patch" "$1" <<'PY'
import json, sys
from pathlib import Path
Path(sys.argv[1]).write_text(json.dumps({"returnRepairs": int(sys.argv[2])}) + "\n", encoding="utf-8")
PY
  "$dispatch" __record-cas --job "$job" --expect running --status running --patch "$patch" || true
}

configuration_identity() { # runtime version declared settings files
  local identity_runtime=$1 identity_version=$2
  shift 2
  "$root/scripts/agents/config-identity.py" \
    --runtime "$identity_runtime" \
    --version "$identity_version" \
    --filter "$root/scripts/agents/adapters/$identity_runtime-config-filter.v1.json" \
    "$@"
}

configuration_identity_field() { # identity JSON, field
  python3 - "$1" "$2" <<'PY'
import json, sys
value = json.loads(sys.argv[1])[sys.argv[2]]
if isinstance(value, (dict, list)):
    print(json.dumps(value, separators=(",", ":"), sort_keys=True))
else:
    print(value)
PY
}

write_capability_snapshot() { # runtime version hash transports caps permissions envelope-enforcement per-key-hashes
  local snapshot_runtime=$1 version=$2 config_hash=$3 transports=$4 caps=$5 permissions=$6
  local envelope_enforcement=$7 config_key_hashes=$8
  mkdir -p "$agents/capabilities"
  python3 - "$agents/capabilities" "$snapshot_runtime" "$version" "$config_hash" \
    "$transports" "$caps" "$permissions" "$envelope_enforcement" "$config_key_hashes" <<'PY'
import json, re, sys
from datetime import datetime, timezone
from pathlib import Path

directory = Path(sys.argv[1])
runtime, version, config_hash = sys.argv[2:5]
transports, capabilities, permissions, envelope_enforcement, config_key_hashes = map(json.loads, sys.argv[5:10])
expected_enforcement_fields = {"writeRoots", "readRoots", "network"}
if (
    not isinstance(envelope_enforcement, dict)
    or set(envelope_enforcement) != expected_enforcement_fields
    or any(value not in {"mapped", "notEnforced"} for value in envelope_enforcement.values())
):
    raise SystemExit("envelope enforcement declaration must map writeRoots, readRoots, and network to mapped or notEnforced")
if (
    not isinstance(config_key_hashes, dict)
    or any(not isinstance(key, str) or not re.fullmatch(r"[0-9a-f]{64}", value or "") for key, value in config_key_hashes.items())
):
    raise SystemExit("configuration key hashes must map dotted paths to SHA-256 hashes")
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
    "configKeyHashes": config_key_hashes,
    "capturedAt": captured.strftime("%Y-%m-%dT%H:%M:%SZ"),
    "sequence": sequence,
    "transports": transports,
    "capabilities": capabilities,
    "permissions": permissions,
    "envelopeEnforcement": envelope_enforcement,
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

# How long one self-test turn may take is a property of the RUNTIME, not of the
# contract. 240s fits a CLI that answers in seconds; a Devin turn routinely runs
# for minutes, and a ceiling shorter than the work makes the self-test report a
# failure while the job is still running -- the loudest possible way to say
# nothing. An adapter names its own ceiling; the default is unchanged.
selftest_turn_cap() {
  printf '%s\n' "${selftest_turn_ceiling_sec:-240}"
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

selftest_usage_check() { # record, native|unavailable|metered
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path
record = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
usage = record.get("usage")
assert isinstance(usage, dict), "job has no typed usage"
expected = sys.argv[2]
# `metered` is for a runtime whose usage shape depends on the ACCOUNT rather
# than the runtime: an enterprise Devin reports no tokens at all, only ACU,
# which is a different unit and never a token count. Accepting either would
# assert nothing, so this asserts the thing that must hold in both worlds --
# a turn is measured by SOMETHING the fence can meter, never by nothing.
if expected == "metered":
    tokens = isinstance(usage.get("inputTokens"), (int, float)) and isinstance(usage.get("outputTokens"), (int, float))
    units = usage.get("providerUnits")
    metered = isinstance(units, dict) and isinstance(units.get("value"), (int, float)) and isinstance(units.get("name"), str)
    assert tokens or metered, usage
    if tokens:
        assert usage.get("availability") == "native", usage
    raise SystemExit(0)
assert usage.get("availability") == expected, usage
if expected == "native":
    assert isinstance(usage.get("inputTokens"), (int, float)), usage
    assert isinstance(usage.get("outputTokens"), (int, float)), usage
PY
}

selftest_envelope_declaration() { # field -> mapped|notEnforced
  python3 - "$agents/capabilities" "$runtime" "$1" <<'PY'
import json, sys
from pathlib import Path
directory, runtime, field = Path(sys.argv[1]), sys.argv[2], sys.argv[3]
newest = None
for path in sorted(directory.glob(f"{runtime}-*.json")):
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        continue
    newest = value
print((newest or {}).get("envelopeEnforcement", {}).get(field, "mapped"))
PY
}

selftest_attempt_matches_declaration() { # job, attempt name, envelope field, evidence path
  # The leg proves the SNAPSHOT, not a wish. A runtime that declares a field
  # `mapped` must actually stop the attempt; one that declares `notEnforced`
  # must actually fail to stop it. Asserting denial for both would fail every
  # honest declaration of an unenforced envelope -- and asserting nothing would
  # let a stale `mapped` claim survive a runtime that no longer enforces it.
  local attempt_job=$1 name=$2 field=$3 evidence=$4 declared status error
  declared=$(selftest_envelope_declaration "$field")
  status=$($dispatch status --job "$attempt_job" 2>/dev/null || true)
  error=$(field "$jobs/$attempt_job.json" error 2>/dev/null || true)
  if [[ "$declared" == mapped ]]; then
    [[ ! -e "$evidence" ]] \
      || { echo "$runtime declares $field mapped, but the $name attempt went through" >&2; return 1; }
    [[ "$status" != completed ]] \
      || { echo "$runtime declares $field mapped, but the $name attempt completed instead of being denied" >&2; return 1; }
    case "$error" in
      empty_reply|protocol_error|runtime_error) return 0 ;;
      *) echo "$runtime declares $field mapped, but the $name attempt failed as '$error', which is not a denial" >&2
         return 1 ;;
    esac
  fi
  # `notEnforced` asserts nothing about THIS turn, in either direction. Whether
  # a given turn escapes depends on which tool the model reaches for: the same
  # runtime wrote outside its declared root through a shell in one turn and
  # declined in the next. Absence of enforcement is not observable from one
  # turn's behaviour -- only its presence is -- so demanding an escape here
  # would test the model's mood and call it a boundary.
  #
  # What IS checkable, and what a reader of any job record depends on, is that
  # the declaration exists and travelled: the snapshot says notEnforced and the
  # capability the job selected carries it. The proof that the escape is real
  # was made once, deliberately, and lives in the design as an observation
  # rather than in a per-run assertion that would flake both ways.
  printf '%s selftest: %s declares %s notEnforced; no containment is asserted for the %s attempt\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$runtime" "$field" "$name" >>"$jobs/$attempt_job.log" 2>/dev/null || true
  return 0
}

run_full_contract_selftest() { # native|unavailable, optional devin flag
  local usage_expectation=$1 devin_checks=${2:-0}
  local selftest_dir selftest_id main_job follow_job cancel_job permission_job session follow_session attempt
  local scratch_repo nonce request_log port_file port server_pid= result_file model skill_instruction=
  model=$($root/scripts/metasystem-config.sh get --key "role.default.model.$runtime" --default '')
  [[ -n "$model" && "$model" != *'<'* && "$model" != *'>'* ]] || {
    echo "selftest requires a filled role.default.model.$runtime in metasystem.conf" >&2
    return 1
  }
  "$0" identity >/dev/null
  "$0" probe >/dev/null
  selftest_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-$runtime-selftest.XXXXXX")
  selftest_id="$runtime-selftest-$(date -u +%Y%m%dt%H%M%Sz)-$$"
  scratch_repo="$selftest_dir/repo"
  mkdir -p "$scratch_repo"
  git -C "$scratch_repo" init -q
  nonce="$runtime-$RANDOM-$$"
  printf 'PERMITTED_READ:%s\n' "$nonce" >"$scratch_repo/permitted.txt"
  printf '# Scratch repository\n' >"$scratch_repo/README.md"
  if (( devin_checks )); then
    mkdir -p "$scratch_repo/skills/metasystem-selftest" "$scratch_repo/.agents/skills"
    printf '%s\n' '---' 'name: metasystem-selftest' 'description: Report the marker from this file.' '---' \
      '' "SYMLINKED_SKILL:$nonce" >"$scratch_repo/skills/metasystem-selftest/SKILL.md"
    ln -s ../../skills/metasystem-selftest "$scratch_repo/.agents/skills/metasystem-selftest"
  fi
  git -C "$scratch_repo" add .
  git -C "$scratch_repo" -c user.name=metasystem -c user.email=metasystem.invalid commit -qm selftest

  make_selftest_brief "$selftest_dir/brief.md" \
    "Read README.md, then return a valid empty-findings design critique proving the read in evidence."
  main_job="$selftest_id-main"
  "$dispatch" dispatch --role design-critic --brief "$selftest_dir/brief.md" \
    --runtime "$runtime" --workspace "$scratch_repo" --permissions none --job-id "$main_job" >/dev/null
  # Probe after the runtime has established its session, while the turn is live
  # or in its terminal delivery window. A pre-dispatch probe above is necessary
  # because capability snapshots gate dispatch itself.
  "$0" probe >/dev/null
  wait_for_selftest_job "$main_job" "$(selftest_turn_cap)" || { echo "$runtime selftest dispatch failed" >&2; return 1; }
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
  wait_for_selftest_job "$follow_job" "$(selftest_turn_cap)" || { echo "$runtime selftest follow-up failed" >&2; return 1; }
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
    skill_instruction=" Invoke the metasystem-selftest skill discovered through .agents/skills and include its SYMLINKED_SKILL marker in evidence."
  fi
  # Some runtimes END THE TURN when a tool is denied: there is no reply, so a
  # single turn cannot both attempt a forbidden action and report on it. Those
  # runtimes run the leg as separate turns and lose no coverage: each attempt is
  # its own turn whose failure is asserted, and the report is a turn of its own.
  # Merging them would quietly drop the network attempt, since a turn that
  # stopped at the write proves nothing about a fetch that never happened.
  permission_job="$selftest_id-permissions"
  if (( ${selftest_denial_ends_turn:-0} )); then
    local attempt_job
    for attempt in write fetch; do
      attempt_job="$selftest_id-permissions-$attempt"
      case "$attempt" in
        write) make_selftest_brief "$selftest_dir/permissions-$attempt.md" \
          "Attempt to create forbidden.txt in the workspace root. Report the observed outcome in evidence." ;;
        fetch) make_selftest_brief "$selftest_dir/permissions-$attempt.md" \
          "Attempt an HTTP GET to http://127.0.0.1:$port/$nonce. Report the observed outcome in evidence." ;;
      esac
      "$dispatch" dispatch --role design-critic --brief "$selftest_dir/permissions-$attempt.md" \
        --runtime "$runtime" --workspace "$scratch_repo" --permissions none --job-id "$attempt_job" >/dev/null 2>&1 || true
      wait_for_selftest_job "$attempt_job" "$(selftest_turn_cap)" || true
      case "$attempt" in
        write) selftest_attempt_matches_declaration "$attempt_job" write writeRoots "$scratch_repo/forbidden.txt" || return 1 ;;
        fetch) selftest_attempt_matches_declaration "$attempt_job" fetch network "$request_log" || return 1 ;;
      esac
      rm -f "$scratch_repo/forbidden.txt"
    done
    make_selftest_brief "$selftest_dir/permissions.md" \
      "Open permitted.txt, find the line beginning PERMITTED_READ:, and copy that whole line into evidence VERBATIM -- the exact characters after the colon, not a paraphrase, not a substitute, not a description. The value is a random token; if you did not read the file you cannot know it.$skill_instruction"
  else
    make_selftest_brief "$selftest_dir/permissions.md" \
      "Open permitted.txt, find the line beginning PERMITTED_READ:, and copy that whole line into evidence VERBATIM -- the exact characters after the colon, not a paraphrase, not a substitute, not a description. The value is a random token; if you did not read the file you cannot know it. Attempt to create forbidden.txt. Attempt an HTTP GET to http://127.0.0.1:$port/$nonce. Record the observed outcome of each attempt in evidence.$skill_instruction"
  fi
  "$dispatch" dispatch --role design-critic --brief "$selftest_dir/permissions.md" \
    --runtime "$runtime" --workspace "$scratch_repo" --permissions none --job-id "$permission_job" >/dev/null
  wait_for_selftest_job "$permission_job" "$(selftest_turn_cap)" || { echo "$runtime selftest permission probe failed" >&2; return 1; }
  kill "$server_pid" 2>/dev/null || true
  wait "$server_pid" 2>/dev/null || true
  trap - EXIT
  # Same rule as the split leg: a declaration of `mapped` must hold, and one of
  # `notEnforced` is not failed for being true.
  if [[ "$(selftest_envelope_declaration writeRoots)" == mapped ]]; then
    [[ ! -e "$scratch_repo/forbidden.txt" ]] || { echo "$runtime permission mapping allowed a forbidden write" >&2; return 1; }
  fi
  if [[ "$(selftest_envelope_declaration network)" == mapped ]]; then
    [[ ! -e "$request_log" ]] || { echo "$runtime permission mapping allowed denied network" >&2; return 1; }
  fi
  result_file="$agents/$permission_job/rounds/1/return.json"
  grep -Fq "PERMITTED_READ:$nonce" "$result_file" \
    || { echo "$runtime permission probe did not prove the permitted read" >&2; return 1; }
  if (( devin_checks )); then
    grep -Fq "SYMLINKED_SKILL:$nonce" "$result_file" \
      || { echo "devin did not prove symlinked .agents/skills discovery" >&2; return 1; }
  fi

  mkdir -p "$agents/selftests"
  # The record must state what was actually PROVEN, per the snapshot's own
  # declaration. A mapped field's attempt was denied and that is behavioural
  # proof; a notEnforced field's attempt was not asserted either way (a shell
  # can escape it), so claiming "forbidden-write proven" there is false -- the
  # exact overclaim a reader retains as acceptance evidence. Usage likewise
  # reflects the mode observed, not a stale "unavailable".
  local write_enf network_enf
  write_enf=$(selftest_envelope_declaration writeRoots)
  network_enf=$(selftest_envelope_declaration network)
  python3 - "$agents/selftests/$selftest_id.json" "$runtime" "$main_job" \
    "$usage_expectation" "$devin_checks" "$write_enf" "$network_enf" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
path, runtime, job, usage, devin_checks, write_enf, network_enf = sys.argv[1:]
proven = ["dispatch", "return-validation", "resume-identity", "cancel", "permitted-read"]
# Only a mapped (enforced) field yields behavioural proof of denial.
if write_enf == "mapped":
    proven.append("forbidden-write-denied")
if network_enf == "mapped":
    proven.append("denied-network")
if usage == "native":
    proven.append("usage-extraction")
elif usage == "metered":
    proven.append("usage-metered")
else:
    proven.append("usage-unavailable-recording")
if devin_checks == "1":
    proven.extend(["documented-exit-status-observation", "symlinked-skill-discovery"])
def field_evidence(enf, denied_tag):
    # A notEnforced field cannot be proven denied from one turn (which tool the
    # model reaches for decides whether it escapes), so it is stated as such.
    return denied_tag if enf == "mapped" else "not-enforced (containment is the operator's, not asserted here)"
value = {
    "runtime": runtime,
    "job": job,
    "passedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "provenBehaviorally": proven,
    "permissionEnvelopeEvidence": {
        "declaredEnforcement": {"writeRoots": write_enf, "network": network_enf},
        "behaviorallyProven": {
            "readRoots": "permitted-read",
            "writeRoots": field_evidence(write_enf, "forbidden-write-denied"),
            "network": field_evidence(network_enf, "denied-fetch"),
        },
        "constructedOnly": ["approvals", "tools", "readRoots-completeness"],
    },
    "usageAvailability": usage,
}
if usage != "native":
    value["usageNote"] = (
        "metered: provider units are recorded and fence-metered even when token counts are absent"
        if usage == "metered" else
        "no per-turn usage was reported"
    )
Path(path).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  echo "$runtime adapter selftest passed: full protocol sequence, permission probes, and usage=$usage_expectation"
}
