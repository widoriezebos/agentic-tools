#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "return schema fixtures: binary absent; run the go gate first" >&2; exit 1; }
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

# A schema is only proven by the provider that enforces it, and this family is
# handed straight to structured output. Two defects shipped because nothing
# checked the shape: an object without `required` and a bare `const` without a
# `type`, each of which failed EVERY codex delegate dispatch before the model
# produced a token. The rules are cheap to state, so they are checked here
# instead of being rediscovered live.
for role in behavior-judge code-critic design-critic implementer investigator verifier; do
  "$ms" schema materialize --root "$root" --role "$role" --version 2 \
    --output "$fixture/strict-$role.json"
done
python3 - "$fixture" <<'PY'
import json, sys
from pathlib import Path

problems = []

def declares_a_type(node):
    return any(key in node for key in ("type", "enum", "anyOf", "oneOf", "allOf", "$ref"))

def walk(node, where):
    if not isinstance(node, dict):
        return
    properties = node.get("properties")
    if isinstance(properties, dict):
        if node.get("type") != "object":
            problems.append(f"{where}: has properties but is not typed object")
        required = node.get("required")
        if not isinstance(required, list):
            problems.append(f"{where}: object without a required list")
        else:
            absent = sorted(set(properties) - set(required))
            if absent:
                problems.append(f"{where}: properties absent from required: {absent}")
        if node.get("additionalProperties") is not False:
            problems.append(f"{where}: object without additionalProperties false")
        for name, child in properties.items():
            if isinstance(child, dict) and not declares_a_type(child):
                problems.append(f"{where}/{name}: declares neither type nor enum")
            walk(child, f"{where}/{name}")
    if isinstance(node.get("items"), dict):
        walk(node["items"], f"{where}[]")
    for key in ("anyOf", "oneOf", "allOf"):
        for index, child in enumerate(node.get(key) or []):
            walk(child, f"{where}/{key}[{index}]")

for path in sorted(Path(sys.argv[1]).glob("strict-*.json")):
    walk(json.loads(path.read_text()), path.stem)
if problems:
    print("\n".join(problems), file=sys.stderr)
    raise SystemExit("version-2 schemas violate the structured-output rules above")
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

# A claim on ONE member and agreement on the other still carries both keys.
# OpenAI structured output rejects an object schema that leaves any property
# out of `required`, and that rejection failed every codex delegate dispatch
# before the model produced a token, so the shape is checked here rather than
# discovered live again.
python3 - "$fixture/candidate.json" "$fixture/one-claim.json" <<'PY'
import json,sys
from pathlib import Path
value=json.loads(Path(sys.argv[1]).read_text())
value["sessionId"]="observed-session"
value["model"]={"requested":"requested-model","effective":"claimed-model"}
value.pop("claimed",None)
Path(sys.argv[2]).write_text(json.dumps(value,indent=2,sort_keys=True)+"\n")
PY
normalize_return "$fixture/one-claim.json"
python3 - "$fixture/return.json" <<'PY'
import json,sys
value=json.load(open(sys.argv[1]))
assert value["claimed"]=={"sessionId":None,"model":"claimed-model"},value
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
