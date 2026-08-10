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

# The census verdict schema, at the CLI boundary. The old leg imported
# process-census.py and monkeypatched its internals; that module is gone and
# the schema itself is owned by internal/census and internal/supervise unit
# tests under the go gate. What stays here is the black-box contract: a
# checkout whose supervision state is unavailable still publishes a
# well-formed schema-2 CENSUS-FAILED verdict with a null generation and
# stateDigest. (The SUCCESS shape is proven end to end by the armed
# supervision fixtures, whose census freshness gate reads it.)
census_checkout="$tmp/census-checkout"
mkdir -p "$census_checkout/scripts/agents/adapters"
printf 'metasystem.runtimes=fake\n' >"$census_checkout/metasystem.conf"
printf '#!/usr/bin/env bash\ncase "$1" in signature) echo "match metasystem-fake-runtime-marker" ;; esac\n' \
  >"$census_checkout/scripts/agents/adapters/fake.sh"
chmod +x "$census_checkout/scripts/agents/adapters/fake.sh"
printf '[]\n' >"$census_checkout/processes.json"
METASYSTEM_CENSUS_PROCESS_FILE="$census_checkout/processes.json" \
  "$root/bin/metasystem" census run --root "$census_checkout" --repo "$census_checkout" \
  --fingerprint fixture-fingerprint --interval 10 --output "$tmp/failure-census.json"
python3 - "$tmp/failure-census.json" <<'PY'
import json, sys
failure = json.load(open(sys.argv[1]))
assert failure["schemaVersion"] == 2
assert failure["verdict"] == "CENSUS-FAILED"
assert "generation" in failure and failure["generation"] is None
assert "stateDigest" in failure and failure["stateDigest"] is None
PY

echo "adapter telemetry and census schema fixtures passed"
