#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
fixture=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-return-schema.XXXXXX")
trap 'rm -rf "$fixture"' EXIT

python3 - "$fixture" <<'PY'
import json,sys
from pathlib import Path
root=Path(sys.argv[1])
common={
  "jobId":"fixture-job","round":1,"runtime":"fake","sessionId":None,
  "model":{"requested":"requested-model","effective":None},
  "evidence":[],"gaps":[],"mode":"implement","riskiestPart":"fixture",
  "diffBoundary":[],"whatWasDone":"fixture",
}
(root/"v1.json").write_text(json.dumps(common)+"\n")
record={"effectiveModel":"observed-model"}
(root/"record.json").write_text(json.dumps(record)+"\n")
candidate=dict(common)
candidate.update({"schemaVersion":2,"sessionId":"claimed-session","claimed":{"model":"earlier-claim"}})
candidate["model"]={"requested":"requested-model","effective":"claimed-model"}
(root/"candidate.json").write_text(json.dumps(candidate)+"\n")
missing=dict(candidate); missing.pop("schemaVersion")
(root/"missing-version.json").write_text(json.dumps(missing)+"\n")
extra=dict(candidate); extra["undeclared"]="refuse"
(root/"extra.json").write_text(json.dumps(extra)+"\n")
PY

"$root/scripts/assert-return-complete.sh" --role implementer --file "$fixture/v1.json"

# Exercise the adapter's real normalization owner, not a fixture reimplementation.
source "$root/scripts/agents/adapters/runtime-common.sh"
record=$fixture/record.json
round_dir=$fixture
session_id=observed-session
normalize_return "$fixture/candidate.json"
python3 - "$fixture/return.json" <<'PY'
import json,sys
value=json.load(open(sys.argv[1]))
assert value["schemaVersion"]==2,value
assert value["sessionId"]=="observed-session",value
assert value["model"]["effective"]=="observed-model",value
assert value["claimed"]=={"sessionId":"claimed-session","model":"claimed-model"},value
PY
"$root/scripts/assert-return-complete.sh" --role implementer --file "$fixture/return.json"

if "$root/scripts/assert-return-complete.sh" --role implementer --file "$fixture/missing-version.json" >/dev/null 2>&1; then
  echo "version-2-shaped return without schemaVersion passed the frozen v1 schema" >&2
  exit 1
fi
if "$root/scripts/assert-return-complete.sh" --role implementer --file "$fixture/extra.json" >/dev/null 2>&1; then
  echo "version-2 return with an undeclared property passed" >&2
  exit 1
fi

echo "return schema version 1 compatibility and version 2 normalization fixtures passed"
