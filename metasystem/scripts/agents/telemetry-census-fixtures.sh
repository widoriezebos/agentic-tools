#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-telemetry-census.XXXXXX")
trap 'rm -rf -- "$tmp"' EXIT

# Load the real adapter extraction and common normalization functions without
# launching the provider command.
source "$root/scripts/agents/adapters/claude.sh" --help >/dev/null 2>&1

fixture_record_cas() {
  [[ $1 == __record-cas && $2 == --job && $4 == --expect && $6 == --status && $8 == --patch ]] || return 2
  python3 - "$record" "$9" "$5" "$7" <<'PY'
import json, sys
from pathlib import Path

record_path, patch_path = Path(sys.argv[1]), Path(sys.argv[2])
record = json.loads(record_path.read_text(encoding="utf-8"))
assert record["status"] == sys.argv[3]
record.update(json.loads(patch_path.read_text(encoding="utf-8")))
record["status"] = sys.argv[4]
record_path.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

run_model_case() { # case name, expected model, modelUsage JSON
  local name=$1 expected=$2 model_usage=$3 case_dir="$tmp/$1"
  mkdir -p "$case_dir/round"
  record="$case_dir/job.json"
  round_dir="$case_dir/round"
  job="fixture-$name"
  session_id=fixture-session
  effective_model=provisional-handshake-model
  dispatch=fixture_record_cas
  python3 - "$record" "$case_dir/result.json" "$job" "$model_usage" <<'PY'
import json, sys
from pathlib import Path

record_path, result_path = Path(sys.argv[1]), Path(sys.argv[2])
job, model_usage = sys.argv[3], json.loads(sys.argv[4])
record = {
    "jobId": job,
    "round": 1,
    "runtime": "claude",
    "sessionId": "fixture-session",
    "requestedModel": "requested-model",
    "effectiveModel": "provisional-handshake-model",
    "status": "running",
}
role_return = {
    "jobId": job,
    "round": 1,
    "runtime": "claude",
    "sessionId": "fixture-session",
    "model": {"requested": "requested-model", "effective": "agent-claimed-model"},
    "evidence": [],
    "gaps": [],
    "mode": "implement",
    "riskiestPart": "fixture",
    "diffBoundary": [],
    "whatWasDone": "fixture",
}
result = {
    "type": "result",
    "session_id": "fixture-session",
    "modelUsage": model_usage,
    "structured_output": role_return,
}
record_path.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8")
result_path.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  result_model=$(claude_result_field "$case_dir/result.json" model)
  [[ "$result_model" == "$expected" ]] || {
    echo "$name modelUsage produced $result_model instead of $expected" >&2
    exit 1
  }
  record_result_effective_model "$result_model"
  normalize_return "$case_dir/result.json"
  python3 - "$record" "$round_dir/return.json" "$expected" <<'PY'
import json, sys
from pathlib import Path

record = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
role_return = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
expected = sys.argv[3]
assert record["effectiveModel"] == expected
assert role_return["model"] == {"requested": "requested-model", "effective": expected}
PY
}

run_model_case one-key actual-model '{"actual-model": {}}'
run_model_case zero-keys unobserved '{}'
run_model_case two-keys multi-model:a-model,z-model '{"z-model": {}, "a-model": {}}'

python3 - "$root/scripts/agents/process-census.py" "$tmp" <<'PY'
import importlib.util
import json
import sys
from pathlib import Path

source, output_dir = Path(sys.argv[1]), Path(sys.argv[2])
spec = importlib.util.spec_from_file_location("process_census_fixture", source)
module = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = module
spec.loader.exec_module(module)

module.verify_supervision_snapshot = lambda identities, errors: None
module.configured_signatures = lambda: {}
module.enumerate_processes = lambda repo: []
module.live_custody = lambda: []
module.announcements = lambda fixture_by_pid, errors: []

success_path = output_dir / "success-census.json"
module.read_supervision_snapshot = lambda: ({}, 7, "a" * 64)
module.run_census(output_dir, "fixture-fingerprint", 10, success_path)
success = json.loads(success_path.read_text(encoding="utf-8"))
assert success["schemaVersion"] == 2
assert success["verdict"] == "SUCCESS"
assert success["generation"] == 7
assert success["stateDigest"] == "a" * 64

failure_path = output_dir / "failure-census.json"
def failed_snapshot():
    raise module.CensusError("fixture failure")
module.read_supervision_snapshot = failed_snapshot
module.run_census(output_dir, "fixture-fingerprint", 10, failure_path)
failure = json.loads(failure_path.read_text(encoding="utf-8"))
assert failure["schemaVersion"] == 2
assert failure["verdict"] == "CENSUS-FAILED"
assert "generation" in failure and failure["generation"] is None
assert "stateDigest" in failure and failure["stateDigest"] is None
PY

echo "adapter telemetry and census schema fixtures passed"
