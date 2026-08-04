#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage: scripts/assert-turn-prompt.sh --file <prompt> --turn <turn-directory>

Validates an assembled unattended host-turn prompt against its canonical turn
record and the shipped orchestrator preamble.

Exit codes: 0 pass; 1 validation failure; 2 usage.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
file=
turn=

while (($#)); do
  case "$1" in
    --file) [[ $# -ge 2 ]] || { usage; exit 2; }; file=$2; shift 2 ;;
    --turn) [[ $# -ge 2 ]] || { usage; exit 2; }; turn=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

[[ -n "$file" && -n "$turn" ]] || { usage; exit 2; }

python3 - "$root" "$file" "$turn" <<'PY'
import json
import re
import sys
from pathlib import Path

root = Path(sys.argv[1])
prompt_path = Path(sys.argv[2])
turn_dir = Path(sys.argv[3])
preamble_path = root / "scripts" / "agents" / "roles" / "orchestrator.md"
turn_record_path = turn_dir / "turn.json"

HEADER_KEYS = [
    "Mission-Id", "Turn-Id", "Cycle", "Host-Session", "Runtime", "Model", "Reconciliation",
]
HEADINGS = [
    "## Mission Contract", "## Ledger Tail", "## Open Asks",
    "## Streams", "## Reconciliation", "## This Turn",
]
ID_RE = re.compile(r"[a-z0-9][a-z0-9-]*\Z")
SHA_RE = re.compile(r"[0-9a-f]{40,64}\Z")
CLASSIFICATIONS = {
    "contract-improved", "falsified-continue", "falsified-dead-end",
    "no-progress", "unresolved", "invalid-run",
}
REASON_CLASSES = {"reserved-decision", "red-test", "merge-conflict", "host-failure"}
STREAM_STATES = {"active", "parked-reserved", "parked-stop-loss", "done"}


def fail(check, message):
    print(f"turn prompt violation [{check}]: {message}", file=sys.stderr)
    raise SystemExit(1)


def read_bytes(path, label, check):
    try:
        return path.read_bytes()
    except OSError as error:
        fail(check, f"{label} could not be read: {path}: {error}")


prompt = read_bytes(prompt_path, "prompt", "framing")
preamble = read_bytes(preamble_path, "shipped preamble", "preamble")
if b"\r" in prompt:
    fail("framing", "prompt must use LF line endings")
if not prompt.endswith(b"\n"):
    fail("framing", "prompt must end with an LF")

header_end = prompt.find(b"\n\n")
if header_end < 0:
    fail("headers", "machine header is not followed by one blank line")
try:
    header_lines = prompt[:header_end].decode("utf-8").split("\n")
except UnicodeDecodeError as error:
    fail("headers", f"machine header is not UTF-8: {error}")

header = {}
header_order = []
for line in header_lines:
    key, separator, value = line.partition(": ")
    if not separator:
        fail("headers", f"malformed machine header line: {line!r}")
    if key in header:
        fail("headers", f"machine header repeats {key}")
    header[key] = value
    header_order.append(key)
for key in HEADER_KEYS:
    if not header.get(key):
        fail("headers", f"header key {key} is missing or empty")
if header_order != HEADER_KEYS:
    fail("headers", "machine header keys are not in their declared order")

try:
    record = json.loads(turn_record_path.read_text(encoding="utf-8"))
except FileNotFoundError:
    fail("turn-record", f"turn record does not exist: {turn_record_path}")
except (OSError, json.JSONDecodeError) as error:
    fail("turn-record", f"turn record is unreadable: {turn_record_path}: {error}")
if not isinstance(record, dict):
    fail("turn-record", "turn.json must contain a JSON object")
for field in ("missionId", "turnId"):
    if not isinstance(record.get(field), str) or not record[field]:
        fail("turn-record", f"turn.json field {field} must be a non-empty string")
if header["Mission-Id"] != record["missionId"]:
    fail("identity", "Mission-Id does not equal turn.json missionId")
if header["Turn-Id"] != record["turnId"]:
    fail("identity", "Turn-Id does not equal turn.json turnId")

preamble_start = header_end + 2
preamble_end = preamble_start + len(preamble)
if prompt[preamble_start:preamble_end] != preamble:
    fail("preamble", "assembled preamble bytes differ from scripts/agents/roles/orchestrator.md")
if prompt[preamble_end:preamble_end + 1] != b"\n":
    fail("preamble", "shipped preamble is not followed by exactly one blank line")

try:
    section_text = prompt[preamble_end + 1:].decode("utf-8")
except UnicodeDecodeError as error:
    fail("framing", f"prompt sections are not UTF-8: {error}")
lines = section_text.split("\n")
if lines[-1] != "":
    fail("framing", "prompt sections must end with an LF")

positions = []
inside_data = False
for index, line in enumerate(lines[:-1]):
    if line == "<<<DATA>>>":
        if inside_data:
            fail("fencing", "data fences may not nest")
        inside_data = True
        continue
    if line == "<<<END>>>":
        if not inside_data:
            fail("fencing", "data end marker has no matching start marker")
        inside_data = False
        continue
    if not inside_data and line in HEADINGS:
        positions.append((index, line))
if inside_data:
    fail("fencing", "data start marker has no matching end marker")
if [heading for _, heading in positions] != HEADINGS:
    fail("headings", "the six required headings are missing, duplicated, or out of order")

sections = {}
for position, (start, heading) in enumerate(positions):
    end = positions[position + 1][0] if position + 1 < len(positions) else len(lines) - 1
    body = lines[start + 1:end]
    if position + 1 < len(positions):
        if not body or body[-1] != "" or (len(body) > 1 and body[-2] == ""):
            fail("framing", f"{heading} is not separated from the next block by exactly one blank line")
        body = body[:-1]
    sections[heading] = body


def data_records(heading, field_count):
    body = sections[heading]
    if len(body) < 3 or body[0] != "<<<DATA>>>" or body[-1] != "<<<END>>>":
        fail("fencing", f"{heading} is not fenced with the fixed data markers")
    content = body[1:-1]
    if not content:
        fail("fencing", f"{heading} has an empty data fence; use (none)")
    if content == ["(none)"]:
        return []
    if "(none)" in content:
        fail("records", f"{heading} mixes (none) with records")
    records = []
    for number, line in enumerate(content, 1):
        fields = line.split("\t")
        if len(fields) != field_count or any(not field for field in fields):
            fail(
                "records",
                f"{heading} record {number} must contain {field_count} non-empty tab-separated fields",
            )
        records.append(fields)
    return records


ledger = data_records("## Ledger Tail", 4)
ledger_cycles = []
for number, (cycle, classification, candidate_sha, observed) in enumerate(ledger, 1):
    if not re.fullmatch(r"[1-9][0-9]*", cycle):
        fail("records", f"Ledger Tail record {number} cycle must be a positive integer")
    if classification not in CLASSIFICATIONS:
        fail("records", f"Ledger Tail record {number} classification is unknown")
    if candidate_sha != "none" and not SHA_RE.fullmatch(candidate_sha):
        fail("records", f"Ledger Tail record {number} candidateSha must be a resolved git SHA or none")
    if observed == "(none)":
        fail("records", f"Ledger Tail record {number} uses (none) instead of literal none")
    ledger_cycles.append(int(cycle))
if ledger_cycles != sorted(set(ledger_cycles)):
    fail("records", "Ledger Tail records must be unique and ordered oldest to newest")

asks = data_records("## Open Asks", 4)
ask_ids = []
for number, (ask_id, stream_id, reason_class, question) in enumerate(asks, 1):
    if not ID_RE.fullmatch(ask_id):
        fail("records", f"Open Asks record {number} askId is invalid")
    if stream_id != "none" and not ID_RE.fullmatch(stream_id):
        fail("records", f"Open Asks record {number} streamId must be an id or none")
    if reason_class not in REASON_CLASSES:
        fail("records", f"Open Asks record {number} reasonClass is unknown")
    if question == "(none)":
        fail("records", f"Open Asks record {number} uses (none) instead of literal none")
    ask_ids.append(ask_id)
if ask_ids != sorted(set(ask_ids)):
    fail("records", "Open Asks records must have unique ask ids in sorted order")

streams = data_records("## Streams", 4)
stream_ids = []
for number, (stream_id, state, goal, reason) in enumerate(streams, 1):
    if not ID_RE.fullmatch(stream_id):
        fail("records", f"Streams record {number} streamId is invalid")
    if state not in STREAM_STATES:
        fail("records", f"Streams record {number} state is unknown")
    if goal == "(none)" or reason == "(none)":
        fail("records", f"Streams record {number} uses (none) instead of literal none")
    stream_ids.append(stream_id)
if stream_ids != sorted(set(stream_ids)):
    fail("records", "Streams records must have unique stream ids in sorted order")

reconciliation = data_records("## Reconciliation", 3)
for number, (turn_id, outcome, detail) in enumerate(reconciliation, 1):
    if not ID_RE.fullmatch(turn_id):
        fail("records", f"Reconciliation record {number} turnId is invalid")
    if outcome == "(none)" or detail == "(none)":
        fail("records", f"Reconciliation record {number} uses (none) instead of literal none")
PY
