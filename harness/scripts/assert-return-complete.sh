#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/assert-return-complete.sh --role <role> --file <return.json>
  scripts/assert-return-complete.sh --job <job-id>

Validates a canonical agent return against the shipped schema for its role.
The job form reads artifacts/agents/jobs/<job-id>.json, finds that chain's
round return, and also checks jobId, round, runtime, and sessionId identity.

Exit codes: 0 pass; 1 validation failure; 2 usage.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
role=
file=
job=

while (($#)); do
  case "$1" in
    --role) [[ $# -ge 2 ]] || { usage; exit 2; }; role=$2; shift 2 ;;
    --file) [[ $# -ge 2 ]] || { usage; exit 2; }; file=$2; shift 2 ;;
    --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

if [[ -n "$job" ]]; then
  [[ -z "$role" && -z "$file" && "$job" =~ ^[a-z0-9][a-z0-9-]*$ ]] || { usage; exit 2; }
  mode=job
else
  [[ -n "$role" && -n "$file" ]] || { usage; exit 2; }
  mode=role
fi

case "$role" in
  ""|design-critic|implementer|code-critic|verifier|investigator) ;;
  *) echo "violation: unknown role: $role" >&2; exit 1 ;;
esac

python3 - "$root" "$mode" "$role" "$file" "$job" <<'PY'
import json
import re
import sys
from pathlib import Path

root = Path(sys.argv[1])
mode, requested_role, requested_file, requested_job = sys.argv[2:]
violations = []
job_id_pattern = re.compile(r"[a-z0-9][a-z0-9-]*\Z")


def violation(message):
    violations.append(message)


def load_json(path, label):
    try:
        with path.open(encoding="utf-8") as handle:
            return json.load(handle)
    except FileNotFoundError:
        violation(f"{label} does not exist: {path}")
    except json.JSONDecodeError as error:
        violation(f"{label} is not valid JSON at line {error.lineno}, column {error.colno}: {error.msg}")
    except OSError as error:
        violation(f"{label} could not be read: {path}: {error}")
    return None


def type_matches(value, expected):
    if expected == "object":
        return isinstance(value, dict)
    if expected == "array":
        return isinstance(value, list)
    if expected == "string":
        return isinstance(value, str)
    if expected == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    if expected == "number":
        return isinstance(value, (int, float)) and not isinstance(value, bool)
    if expected == "boolean":
        return isinstance(value, bool)
    if expected == "null":
        return value is None
    return False


def describe_types(expected):
    if isinstance(expected, list):
        return " or ".join(expected)
    return str(expected)


def validate_schema_shape(schema, path="$schema"):
    if not isinstance(schema, dict):
        violation(f"{path} must be a JSON object")
        return
    supported = {
        "$schema", "$comment", "title", "description", "type", "enum",
        "properties", "required", "additionalProperties", "items",
    }
    for keyword in schema:
        if keyword not in supported:
            violation(f"{path} uses unsupported schema keyword {keyword!r}")
    if "properties" in schema:
        if not isinstance(schema["properties"], dict):
            violation(f"{path}.properties must be an object")
        else:
            for name, child in schema["properties"].items():
                validate_schema_shape(child, f"{path}.properties.{name}")
    if "items" in schema:
        validate_schema_shape(schema["items"], f"{path}.items")
    if "required" in schema and (
        not isinstance(schema["required"], list)
        or not all(isinstance(name, str) for name in schema["required"])
    ):
        violation(f"{path}.required must be an array of strings")
    if "additionalProperties" in schema and not isinstance(schema["additionalProperties"], bool):
        violation(f"{path}.additionalProperties must be boolean")
    if "enum" in schema and not isinstance(schema["enum"], list):
        violation(f"{path}.enum must be an array")


def validate(value, schema, path="$"):
    expected = schema.get("type")
    if expected is not None:
        choices = expected if isinstance(expected, list) else [expected]
        if not any(type_matches(value, choice) for choice in choices):
            violation(f"{path} must be {describe_types(expected)}")
            return

    if "enum" in schema and value not in schema["enum"]:
        allowed = ", ".join(repr(item) for item in schema["enum"])
        violation(f"{path} must be one of: {allowed}")

    if isinstance(value, dict):
        properties = schema.get("properties", {})
        for name in schema.get("required", []):
            if name not in value:
                violation(f"{path}.{name} is required")
        if schema.get("additionalProperties") is False:
            for name in value:
                if name not in properties:
                    violation(f"{path}.{name} is not allowed by this role schema")
        for name, child in value.items():
            if name in properties:
                validate(child, properties[name], f"{path}.{name}")

    if isinstance(value, list) and "items" in schema:
        for index, item in enumerate(value):
            validate(item, schema["items"], f"{path}[{index}]")


record = None
if mode == "job":
    record_path = root / "artifacts" / "agents" / "jobs" / f"{requested_job}.json"
    record = load_json(record_path, "job record")
    if record is None:
        for item in violations:
            print(f"violation: {item}", file=sys.stderr)
        sys.exit(1)
    if not isinstance(record, dict):
        violation("job record must be a JSON object")
        role = None
        return_path = None
    else:
        role = record.get("role")
        round_number = record.get("round")
        if record.get("jobId") != requested_job:
            violation(f"job record jobId must equal requested job id {requested_job!r}")

        current = record
        seen = {requested_job}
        root_job_id = requested_job
        while isinstance(current, dict) and current.get("parentJob") is not None:
            parent = current.get("parentJob")
            if not isinstance(parent, str) or not parent:
                violation("job record parentJob must be a non-empty string or null")
                break
            if not job_id_pattern.fullmatch(parent):
                violation(f"job record parentJob is not a valid job id: {parent!r}")
                break
            if parent in seen:
                violation(f"job record parentJob chain contains a cycle at {parent!r}")
                break
            seen.add(parent)
            parent_path = root / "artifacts" / "agents" / "jobs" / f"{parent}.json"
            current = load_json(parent_path, f"parent job record {parent!r}")
            if current is None:
                break
            if not isinstance(current, dict):
                violation(f"parent job record {parent!r} must be a JSON object")
                break
            if current.get("jobId") != parent:
                violation(f"parent job record {parent!r} has a different jobId")
                break
            root_job_id = parent

        if not isinstance(round_number, int) or isinstance(round_number, bool):
            violation("job record round must be an integer")
            return_path = None
        else:
            return_path = root / "artifacts" / "agents" / str(root_job_id) / "rounds" / str(round_number) / "return.json"
else:
    role = requested_role
    return_path = Path(requested_file)

allowed_roles = {"design-critic", "implementer", "code-critic", "verifier", "investigator"}
if role not in allowed_roles:
    violation(f"job record role is not dispatchable: {role!r}" if mode == "job" else f"unknown role: {role!r}")

result = load_json(return_path, "return file") if return_path is not None else None
schema = load_json(root / "scripts" / "agents" / "schemas" / f"{role}.schema.json", "role schema") if role in allowed_roles else None

if result is not None and schema is not None:
    before_schema_check = len(violations)
    validate_schema_shape(schema)
    if len(violations) == before_schema_check:
        validate(result, schema)

    if role in {"design-critic", "code-critic"} and isinstance(result, dict):
        findings = result.get("findings")
        verdict = result.get("verdictMaterialCount")
        if isinstance(findings, list) and isinstance(verdict, int) and not isinstance(verdict, bool):
            material_count = sum(
                1 for finding in findings
                if isinstance(finding, dict) and finding.get("material") is True
            )
            if verdict != material_count:
                violation(
                    "$.verdictMaterialCount must equal the count of findings with material=true "
                    f"(expected {material_count}, got {verdict})"
                )

    if mode == "job" and isinstance(result, dict) and isinstance(record, dict):
        for name in ("jobId", "round", "runtime", "sessionId"):
            if result.get(name) != record.get(name):
                violation(
                    f"$.{name} identity mismatch: return has {result.get(name)!r}, "
                    f"job record has {record.get(name)!r}"
                )

for item in violations:
    print(f"violation: {item}", file=sys.stderr)
sys.exit(1 if violations else 0)
PY
