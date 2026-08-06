#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  benchmark/attest.sh
  benchmark/attest.sh --from-ci <prefetched-ci-record.json>

The default form runs metasystem/scripts/validate-metasystem.sh locally and
writes an attestation only after it passes. The CI form performs no network
access; it validates and records the supplied, already-fetched green record.
USAGE
}

die() { echo "$2" >&2; exit "$1"; }

kit=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
top=$(cd "$kit/.." && pwd -P)
ci_record=
if (($#)); then
  [[ $# -eq 2 && "$1" == --from-ci ]] || { usage; exit 2; }
  ci_record=$2
fi

candidate_sha=$(git -C "$top" log -1 --format=%H HEAD -- . ':(exclude)benchmark/results/**')
[[ "$candidate_sha" =~ ^[0-9a-f]{40}$ ]] || die 1 "attestation refused: candidate sha is unavailable"
output=$kit/results/attestations/$candidate_sha.json
[[ ! -e "$output" ]] || die 1 "attestation refused: output already exists: $output"
mkdir -p "$(dirname "$output")"

if [[ -z "$ci_record" ]]; then
  command=metasystem/scripts/validate-metasystem.sh
  [[ -x "$top/$command" ]] || die 1 "attestation refused: local gate is not executable: $command"
  if ! (cd "$top" && "$command"); then
    die 1 "attestation refused: local gate was red"
  fi
  machine=$($kit/system-fingerprint.py)
  python3 - "$output" "$candidate_sha" "$command" "$machine" <<'PY'
import datetime as dt
import json
import os
import sys
import tempfile
from pathlib import Path

output = Path(sys.argv[1])
value = {
    "schemaVersion": 1,
    "source": "local",
    "command": sys.argv[3],
    "candidateSha": sys.argv[2],
    "conclusion": "success",
    "timestamp": dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "machineFingerprint": json.loads(sys.argv[4]),
}
descriptor, temporary = tempfile.mkstemp(prefix=f".{output.name}.", dir=output.parent)
with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
    json.dump(value, handle, indent=2, sort_keys=True)
    handle.write("\n")
    handle.flush()
    os.fsync(handle.fileno())
os.replace(temporary, output)
PY
else
  [[ -f "$ci_record" ]] || die 2 "attestation refused: CI record does not exist: $ci_record"
  python3 - "$ci_record" "$output" "$candidate_sha" <<'PY'
import json
import os
import sys
import tempfile
from datetime import datetime
from pathlib import Path

source_path = Path(sys.argv[1])
output = Path(sys.argv[2])
expected_sha = sys.argv[3]
try:
    record = json.loads(source_path.read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError) as error:
    raise SystemExit(f"attestation refused: CI record is unreadable: {error}")
if not isinstance(record, dict):
    raise SystemExit("attestation refused: CI record root is not an object")
sha = record.get("candidateSha", record.get("sha"))
if sha != expected_sha:
    raise SystemExit("attestation refused: CI record sha does not match the candidate")
if record.get("conclusion") != "success":
    raise SystemExit("attestation refused: CI record is not green")
timestamp = record.get("timestamp", record.get("retrievedAt"))
try:
    if not isinstance(timestamp, str):
        raise ValueError("missing")
    datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
except ValueError:
    raise SystemExit("attestation refused: CI record has no valid timestamp")
machine = record.get("machineFingerprint")
if not isinstance(machine, dict) or set(machine) != {"os", "cpuModel", "coreCount"}:
    raise SystemExit("attestation refused: CI record has no complete machine fingerprint")
if not isinstance(machine.get("os"), str) or not machine["os"]:
    raise SystemExit("attestation refused: CI record OS is invalid")
if not isinstance(machine.get("cpuModel"), str) or not machine["cpuModel"]:
    raise SystemExit("attestation refused: CI record CPU model is invalid")
if not isinstance(machine.get("coreCount"), int) or isinstance(machine["coreCount"], bool) or machine["coreCount"] < 1:
    raise SystemExit("attestation refused: CI record core count is invalid")
value = {
    "schemaVersion": 1,
    "source": "ci",
    "command": record.get("command", record.get("workflowPath")),
    "candidateSha": sha,
    "conclusion": "success",
    "timestamp": timestamp,
    "machineFingerprint": machine,
    "ciRecord": record,
}
if not isinstance(value["command"], str) or not value["command"]:
    raise SystemExit("attestation refused: CI record has no command or workflowPath")
descriptor, temporary = tempfile.mkstemp(prefix=f".{output.name}.", dir=output.parent)
with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
    json.dump(value, handle, indent=2, sort_keys=True)
    handle.write("\n")
    handle.flush()
    os.fsync(handle.fileno())
os.replace(temporary, output)
PY
fi

printf '%s\n' "$output"
