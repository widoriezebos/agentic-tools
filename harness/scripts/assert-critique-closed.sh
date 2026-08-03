#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/assert-critique-closed.sh --findings <return.json> --dispositions <file>

Joins the canonical findings array from a critic return JSON against the
Markdown dispositions table on finding id.

Required dispositions table header:
| Finding id | Disposition | Reasoning and evidence | Amendment |

Exit codes: 0 closed; 1 open or unjoinable; 2 usage.
USAGE
}

findings=
dispositions=

while (($#)); do
  case "$1" in
    --findings)
      [[ $# -ge 2 && -z "$findings" ]] || { usage; exit 2; }
      findings=$2
      shift 2
      ;;
    --dispositions)
      [[ $# -ge 2 && -z "$dispositions" ]] || { usage; exit 2; }
      dispositions=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

[[ -n "$findings" && -n "$dispositions" ]] || { usage; exit 2; }

python3 - "$findings" "$dispositions" <<'PY'
import json
import re
import sys
from pathlib import Path

findings_path = Path(sys.argv[1])
dispositions_path = Path(sys.argv[2])
violations = []


def violation(message):
    violations.append(message)


def read_findings():
    try:
        with findings_path.open(encoding="utf-8") as handle:
            result = json.load(handle)
    except FileNotFoundError:
        violation(f"return JSON is unjoinable: file does not exist: {findings_path}")
        return {}, False
    except json.JSONDecodeError as error:
        violation(
            "return JSON is unjoinable: invalid JSON at "
            f"line {error.lineno}, column {error.colno}: {error.msg}"
        )
        return {}, False
    except OSError as error:
        violation(f"return JSON is unjoinable: could not read {findings_path}: {error}")
        return {}, False

    if not isinstance(result, dict):
        violation("return JSON is unjoinable: root must be an object")
        return {}, False
    if "findings" not in result:
        violation("return JSON is unjoinable: $.findings array is missing")
        return {}, False
    if not isinstance(result["findings"], list):
        violation("return JSON is unjoinable: $.findings must be an array")
        return {}, False

    by_id = {}
    seen = set()
    joinable = True
    for index, finding in enumerate(result["findings"]):
        path = f"$.findings[{index}]"
        if not isinstance(finding, dict):
            violation(f"return JSON is unjoinable: {path} must be an object")
            joinable = False
            continue

        finding_id = finding.get("id")
        if (
            not isinstance(finding_id, str)
            or not finding_id.strip()
            or finding_id != finding_id.strip()
        ):
            violation(
                f"return JSON is unjoinable: {path}.id must be a non-empty string "
                "without surrounding whitespace"
            )
            joinable = False
            continue

        if finding_id in seen:
            violation(f"duplicate finding id: {finding_id!r}")
        else:
            seen.add(finding_id)

        material = finding.get("material")
        if not isinstance(material, bool):
            violation(f"return JSON is unjoinable: {path}.material must be boolean")
            joinable = False
            continue
        by_id.setdefault(finding_id, material)

    return by_id, joinable


def markdown_cells(line):
    stripped = line.strip()
    if "|" not in stripped:
        return None

    cells = []
    cell = []
    escaped = False
    for character in stripped:
        if escaped:
            cell.append(character)
            escaped = False
        elif character == "\\":
            cell.append(character)
            escaped = True
        elif character == "|":
            cells.append("".join(cell).strip())
            cell = []
        else:
            cell.append(character)
    cells.append("".join(cell).strip())

    if cells and cells[0] == "":
        cells.pop(0)
    if cells and cells[-1] == "":
        cells.pop()
    return cells


def lines_outside_fences(lines):
    visible = []
    fence_character = None
    fence_length = 0
    for line in lines:
        marker = re.match(r"^[ ]{0,3}(`{3,}|~{3,})", line)
        if fence_character is None:
            if marker:
                token = marker.group(1)
                fence_character = token[0]
                fence_length = len(token)
                visible.append(False)
            else:
                visible.append(True)
        elif marker and marker.group(1)[0] == fence_character and len(marker.group(1)) >= fence_length:
            visible.append(False)
            fence_character = None
            fence_length = 0
        else:
            visible.append(False)
    return visible


def read_dispositions():
    try:
        lines = dispositions_path.read_text(encoding="utf-8").splitlines()
    except FileNotFoundError:
        violation(f"dispositions file is unjoinable: file does not exist: {dispositions_path}")
        return {}, False
    except OSError as error:
        violation(f"dispositions file is unjoinable: could not read {dispositions_path}: {error}")
        return {}, False

    header = ["Finding id", "Disposition", "Reasoning and evidence", "Amendment"]
    visible = lines_outside_fences(lines)
    header_indexes = [
        index
        for index, line in enumerate(lines)
        if visible[index] and markdown_cells(line) == header
    ]
    if not header_indexes:
        violation(
            "dispositions file is unjoinable: malformed dispositions table: "
            "required header not found"
        )
        return {}, False
    if len(header_indexes) > 1:
        violation(
            "dispositions file is unjoinable: malformed dispositions table: "
            "multiple required headers found"
        )
        return {}, False

    header_index = header_indexes[0]
    separator_index = header_index + 1
    if separator_index >= len(lines) or not visible[separator_index]:
        violation(
            "dispositions file is unjoinable: malformed dispositions table: "
            "separator row is missing"
        )
        return {}, False
    separator = markdown_cells(lines[separator_index])
    if (
        separator is None
        or len(separator) != len(header)
        or not all(re.fullmatch(r":?-{3,}:?", cell) for cell in separator)
    ):
        violation(
            "dispositions file is unjoinable: malformed dispositions table: "
            f"invalid separator row at line {separator_index + 1}"
        )
        return {}, False

    rows = []
    joinable = True
    for index in range(separator_index + 1, len(lines)):
        if not visible[index] or not lines[index].strip():
            break
        cells = markdown_cells(lines[index])
        if cells is None:
            break
        if len(cells) != len(header):
            violation(
                "dispositions file is unjoinable: malformed dispositions table: "
                f"row at line {index + 1} has {len(cells)} columns instead of {len(header)}"
            )
            joinable = False
            continue
        rows.append((index + 1, cells))

    by_id = {}
    seen = set()
    allowed = {"accepted", "refuted", "noted"}
    for line_number, row in rows:
        finding_id, disposition = row[0], row[1]
        if not finding_id:
            violation(
                "dispositions file is unjoinable: malformed dispositions table: "
                f"row at line {line_number} has an empty finding id"
            )
            joinable = False
            continue
        if finding_id in seen:
            violation(f"duplicate disposition id: {finding_id!r}")
        else:
            seen.add(finding_id)
        if disposition not in allowed:
            violation(
                f"disposition for finding id {finding_id!r} has unknown value "
                f"{disposition!r}; allowed values are accepted, refuted, noted"
            )
        by_id.setdefault(finding_id, disposition)

    return by_id, joinable


findings, findings_joinable = read_findings()
dispositions, dispositions_joinable = read_dispositions()

if findings_joinable and dispositions_joinable:
    for finding_id, material in findings.items():
        if finding_id not in dispositions:
            violation(f"finding id {finding_id!r} has no disposition row")
        elif material and dispositions[finding_id] == "noted":
            violation(f"material finding id {finding_id!r} cannot use disposition 'noted'")
    for finding_id in dispositions:
        if finding_id not in findings:
            violation(f"disposition names unknown finding id: {finding_id!r}")

for item in violations:
    print(f"violation: {item}", file=sys.stderr)
sys.exit(1 if violations else 0)
PY
