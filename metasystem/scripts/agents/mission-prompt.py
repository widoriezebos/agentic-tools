#!/usr/bin/env python3
"""Assemble one byte-stable unattended mission host-turn prompt."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
CLASSIFICATION_RE = re.compile(
    r"^- Classification:[ \t]*([a-z-]+); candidate-sha=([^;\n]+); observed=([^\n]+)$",
    re.MULTILINE,
)
HEADING_RE = re.compile(r"^### Cycle ([1-9][0-9]*)[ \t]*$", re.MULTILINE)
DATA_MARKERS = (("<<<DATA>>>", "< < <DATA> > >"), ("<<<END>>>", "< < <END> > >"))


class PromptError(RuntimeError):
    pass


def read_json(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as error:
        raise PromptError(f"{label} is missing: {path}") from error
    except (OSError, json.JSONDecodeError) as error:
        raise PromptError(f"{label} is unreadable: {path}: {error}") from error
    if not isinstance(value, dict):
        raise PromptError(f"{label} must be a JSON object: {path}")
    return value


def config_value(key: str, default: str) -> str:
    result = subprocess.run(
        [str(ROOT / "scripts" / "metasystem-config.sh"), "get", "--key", key, "--default", default],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if result.returncode != 0:
        raise PromptError(result.stderr.strip() or f"cannot resolve {key}")
    return result.stdout.strip()


def one_line(value: Any) -> str:
    if value is None:
        return "none"
    if isinstance(value, bool):
        text = "yes" if value else "no"
    else:
        text = str(value)
    text = text.replace("\r", " ").replace("\n", " ").replace("\t", " ")
    for marker, replacement in DATA_MARKERS:
        text = text.replace(marker, replacement)
    return text or "none"


def authored_values(contract_text: str) -> dict[str, str]:
    blocks = re.findall(r"^```mission[ \t]*\n(.*?)^```[ \t]*$", contract_text, re.MULTILINE | re.DOTALL)
    if len(blocks) != 1:
        raise PromptError("mission contract does not contain exactly one authored mission block")
    values: dict[str, str] = {}
    for raw in blocks[0].splitlines():
        if not raw.strip():
            continue
        key, separator, value = raw.partition("=")
        if not separator or key in values:
            raise PromptError("mission contract key/value grammar is invalid")
        values[key] = value
    return values


def ledger_records(ledger: Path, maximum: int) -> list[list[str]]:
    try:
        text = ledger.read_text(encoding="utf-8")
    except OSError as error:
        raise PromptError(f"mission ledger is unreadable: {ledger}: {error}") from error
    headings = list(HEADING_RE.finditer(text))
    records: list[list[str]] = []
    for index, heading in enumerate(headings):
        end = headings[index + 1].start() if index + 1 < len(headings) else len(text)
        block = text[heading.end() : end]
        matches = CLASSIFICATION_RE.findall(block)
        if len(matches) != 1:
            raise PromptError(f"mission ledger cycle {heading.group(1)} lacks one parseable classification")
        classification, candidate_sha, observed = matches[0]
        records.append(
            [heading.group(1), classification, one_line(candidate_sha.strip()), one_line(observed.strip())]
        )
    return records[-maximum:]


def ask_records(directory: Path) -> list[list[str]]:
    records: list[list[str]] = []
    if not directory.exists():
        return records
    for path in directory.glob("*.json"):
        ask = read_json(path, "mission ask")
        if ask.get("answeredAt") is not None:
            continue
        ask_id = ask.get("askId")
        if not isinstance(ask_id, str):
            raise PromptError(f"mission ask has no askId: {path}")
        records.append(
            [
                one_line(ask_id),
                one_line(ask.get("streamId")),
                one_line(ask.get("reasonClass")),
                one_line(ask.get("question")),
            ]
        )
    records.sort(key=lambda item: item[0])
    return records


def stream_records(state: dict[str, Any]) -> list[list[str]]:
    streams = state.get("streams")
    if not isinstance(streams, dict):
        raise PromptError("mission state streams are unreadable")
    records = []
    for stream_id, stream in sorted(streams.items()):
        if not isinstance(stream, dict):
            raise PromptError(f"mission stream is unreadable: {stream_id}")
        records.append(
            [
                one_line(stream_id),
                one_line(stream.get("state")),
                one_line(stream.get("goal")),
                one_line(stream.get("reason")),
            ]
        )
    return records


def reconciliation_records(state: dict[str, Any], turn_id: str, required: bool) -> list[list[str]]:
    if not required:
        return []
    turn_log = state.get("turnLog")
    if not isinstance(turn_log, list):
        raise PromptError("mission state turn log is unreadable")
    for item in reversed(turn_log):
        if not isinstance(item, dict) or item.get("turnId") == turn_id:
            continue
        outcome = item.get("outcome")
        if outcome in {None, "completed", "return-ok"}:
            continue
        return [[one_line(item.get("turnId")), one_line(outcome), one_line(item.get("detail"))]]
    raise PromptError("turn record requires reconciliation but no prior non-zero outcome exists")


def data_section(heading: str, records: list[list[str]]) -> str:
    content = ["\t".join(one_line(field) for field in record) for record in records] or ["(none)"]
    return "\n".join([heading, "<<<DATA>>>", *content, "<<<END>>>"])


def fence_headroom(values: dict[str, str], mission_dir: Path) -> str:
    try:
        cycle_limit = int(values["fence.cycles"])
        job_limit = int(values["fence.jobs"])
    except (KeyError, ValueError) as error:
        raise PromptError("mission contract fence limits are unreadable") from error
    fences_path = mission_dir / "fences.json"
    if fences_path.exists():
        fences = read_json(fences_path, "mission fence counters")
        cycles = fences.get("cycles", 0)
        reservations = fences.get("reservations", {})
        if not isinstance(cycles, int) or isinstance(cycles, bool) or not isinstance(reservations, dict):
            raise PromptError("mission fence counters have an invalid shape")
        jobs = len(reservations)
    else:
        cycles = 0
        jobs = 0
    return f"cycles={max(0, cycle_limit - cycles)},jobs={max(0, job_limit - jobs)}"


def atomic_write(path: Path, content: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=path.name + ".", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass


def assemble(mission: str, turn_id: str, output: Path) -> None:
    if not ID_RE.fullmatch(mission) or not ID_RE.fullmatch(turn_id):
        raise PromptError("mission and turn ids must match the lowercase metasystem id grammar")
    mission_dir = ROOT / "artifacts" / "agents" / "missions" / mission
    turn_dir = mission_dir / "turns" / turn_id
    turn_path = turn_dir / "turn.json"
    if not turn_path.exists():
        raise PromptError(f"missing turn record: {turn_path}")
    turn = read_json(turn_path, "turn record")
    state = read_json(mission_dir / "state.json", "mission state")
    if turn.get("missionId") != mission or turn.get("turnId") != turn_id:
        raise PromptError("turn record identity does not match --mission and --turn")
    if state.get("missionId") != mission:
        raise PromptError("mission state identity does not match --mission")

    required = {
        "cycle": int,
        "runtime": str,
        "model": str,
        "reconciliation": bool,
    }
    for field, expected in required.items():
        value = turn.get(field)
        if not isinstance(value, expected) or (expected is int and isinstance(value, bool)):
            raise PromptError(f"turn record field is invalid: {field}")
    if turn["cycle"] < 1:
        raise PromptError("turn record cycle must be positive")
    host_session = turn.get("hostSession")
    if host_session is not None and (not isinstance(host_session, str) or not host_session):
        raise PromptError("turn record hostSession must be a non-empty string or null")

    contract_path = ROOT / "plans" / f"mission-{mission}.contract.md"
    try:
        contract_text = contract_path.read_text(encoding="utf-8")
        preamble = (ROOT / "scripts" / "agents" / "roles" / "orchestrator.md").read_text(encoding="utf-8")
        instruction = (ROOT / "scripts" / "agents" / "templates" / "host-turn-instruction.md").read_text(
            encoding="utf-8"
        )
    except OSError as error:
        raise PromptError(f"prompt authority artifact is unreadable: {error}") from error
    values = authored_values(contract_text)

    tail_text = config_value("mission.ledger-tail-cycles", "5")
    maximum_text = config_value("mission.max-prompt-kb", "256")
    if not re.fullmatch(r"[1-9][0-9]*", tail_text) or not 1 <= int(tail_text) <= 50:
        raise PromptError("mission.ledger-tail-cycles must be an integer from 1 through 50")
    if not re.fullmatch(r"[1-9][0-9]*", maximum_text):
        raise PromptError("mission.max-prompt-kb must be a positive integer")

    reconciliation = bool(turn["reconciliation"])
    headers = "\n".join(
        [
            f"Mission-Id: {mission}",
            f"Turn-Id: {turn_id}",
            f"Cycle: {turn['cycle']}",
            f"Host-Session: {one_line(host_session)}",
            f"Runtime: {turn['runtime']}",
            f"Model: {turn['model']}",
            f"Reconciliation: {'yes' if reconciliation else 'no'}",
        ]
    )
    this_turn = (
        instruction.replace("<cycle-number>", str(turn["cycle"]))
        .replace("<fence-headroom>", fence_headroom(values, mission_dir))
        .replace("<yes | no>", "yes" if reconciliation else "no")
        .rstrip("\n")
    )
    blocks = [
        ("machine header", headers),
        ("orchestrator preamble", preamble.rstrip("\n")),
        ("## Mission Contract", "## Mission Contract\n" + contract_text.rstrip("\n")),
        (
            "## Ledger Tail",
            data_section("## Ledger Tail", ledger_records(mission_dir / "ledger.md", int(tail_text))),
        ),
        ("## Open Asks", data_section("## Open Asks", ask_records(mission_dir / "asks"))),
        ("## Streams", data_section("## Streams", stream_records(state))),
        (
            "## Reconciliation",
            data_section(
                "## Reconciliation",
                reconciliation_records(state, turn_id, reconciliation),
            ),
        ),
        ("## This Turn", "## This Turn\n" + this_turn),
    ]
    prompt = ("\n\n".join(content for _, content in blocks) + "\n").encode("utf-8")
    maximum = int(maximum_text) * 1024
    if len(prompt) > maximum:
        name, content = max(blocks, key=lambda item: len(item[1].encode("utf-8")))
        raise PromptError(
            f"assembled prompt exceeds mission.max-prompt-kb ({maximum_text} KiB); oversized block: {name}"
        )
    atomic_write(output, prompt)


def main() -> int:
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("--mission")
    parser.add_argument("--turn")
    parser.add_argument("--output", type=Path)
    parser.add_argument("-h", "--help", action="store_true")
    args, extras = parser.parse_known_args()
    if args.help:
        print("Usage: mission-prompt.py --mission <id> --turn <turn-id> --output <file>")
        return 0
    if extras or not args.mission or not args.turn or args.output is None:
        print("Usage: mission-prompt.py --mission <id> --turn <turn-id> --output <file>", file=sys.stderr)
        return 2
    try:
        assemble(args.mission, args.turn, args.output)
    except PromptError as error:
        print(f"mission prompt refused: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
