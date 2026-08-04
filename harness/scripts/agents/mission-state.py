#!/usr/bin/env python3
"""Atomic mission state, hash-chain, anchor, and reconciliation owner."""

from __future__ import annotations

import argparse
import copy
import fcntl
import hashlib
import json
import math
import os
import re
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
HASH_RE = re.compile(r"^[0-9a-f]{64}$")
STREAM_STATES = {"active", "parked-reserved", "parked-stop-loss", "done"}
PARK_REASONS = {
    "all-streams-parked",
    "stop-loss",
    "fence",
    "gate-integrity",
    "state-integrity",
    "contract-changed",
    "host-failure",
}
LEGAL_STREAM_TRANSITIONS = {
    "active": {"active", "parked-reserved", "parked-stop-loss", "done"},
    "parked-reserved": {"parked-reserved", "active"},
    "parked-stop-loss": {"parked-stop-loss", "active"},
    "done": {"done"},
}


class StateError(RuntimeError):
    pass


def canonical(value: Any) -> bytes:
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")


def state_hash(state: dict[str, Any]) -> str:
    body = copy.deepcopy(state)
    body["integrity"]["hash"] = ""
    body["integrity"]["history"] = []
    return hashlib.sha256(canonical(body)).hexdigest()


def atomic_write(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=path.name + ".", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            json.dump(value, handle, indent=2, sort_keys=True)
            handle.write("\n")
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


def read_json(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise StateError(f"cannot read mission state: {error}") from error
    if not isinstance(value, dict):
        raise StateError("mission state must be a JSON object")
    return value


def is_nonnegative_int(value: Any) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value >= 0


def validate_shape(state: dict[str, Any]) -> None:
    required = {
        "schemaVersion",
        "missionId",
        "branch",
        "status",
        "parkReason",
        "gatePassed",
        "streams",
        "fences",
        "turnLog",
        "waitingList",
        "runnerLease",
        "ledger",
        "integrity",
    }
    if set(state) != required:
        raise StateError("mission state has missing or unexpected top-level fields")
    if state["schemaVersion"] != 1 or not isinstance(state["missionId"], str) or not ID_RE.fullmatch(state["missionId"]):
        raise StateError("mission state schema version or mission id is invalid")
    if not isinstance(state["branch"], str) or not state["branch"]:
        raise StateError("mission state branch is invalid")
    if state["status"] not in {"running", "completed", "parked"}:
        raise StateError("mission status is invalid")
    if state["parkReason"] is not None and state["parkReason"] not in PARK_REASONS:
        raise StateError("mission park reason is invalid")
    if not isinstance(state["gatePassed"], bool):
        raise StateError("mission gatePassed must be boolean")
    if not isinstance(state["streams"], dict) or not state["streams"]:
        raise StateError("mission streams must be a non-empty object")
    for stream_id, stream in state["streams"].items():
        if not isinstance(stream_id, str) or not ID_RE.fullmatch(stream_id) or not isinstance(stream, dict):
            raise StateError("mission stream identity is invalid")
        if set(stream) != {"goal", "state", "reason", "answeredAsk"}:
            raise StateError(f"mission stream {stream_id} has missing or unexpected fields")
        if not isinstance(stream["goal"], str) or not stream["goal"] or stream["state"] not in STREAM_STATES:
            raise StateError(f"mission stream {stream_id} goal or state is invalid")
        for field in ("reason", "answeredAsk"):
            if stream[field] is not None and not isinstance(stream[field], str):
                raise StateError(f"mission stream {stream_id} {field} is invalid")
        if stream["state"].startswith("parked-") and not stream["reason"]:
            raise StateError(f"mission stream {stream_id} is parked without a reason")
        if stream["answeredAsk"] is not None and not ID_RE.fullmatch(stream["answeredAsk"]):
            raise StateError(f"mission stream {stream_id} answered ask id is invalid")
    fences = state["fences"]
    if not isinstance(fences, dict) or set(fences) != {"startedAt", "cycles", "jobs", "activeJobs", "usage"}:
        raise StateError("mission fence counters have an invalid shape")
    try:
        datetime.fromisoformat(fences["startedAt"].replace("Z", "+00:00"))
    except (AttributeError, ValueError):
        raise StateError("mission fence start time is invalid")
    for field in ("cycles", "jobs", "activeJobs"):
        if not is_nonnegative_int(fences[field]):
            raise StateError(f"mission fence counter {field} is invalid")
    if fences["activeJobs"] > fences["jobs"]:
        raise StateError("mission active job count exceeds total jobs")
    if not isinstance(fences["usage"], list):
        raise StateError("mission usage must be an array")
    seen_units: set[tuple[str, str]] = set()
    for item in fences["usage"]:
        if not isinstance(item, dict) or set(item) != {"provider", "unit", "value"}:
            raise StateError("mission usage entry has an invalid shape")
        key = (item["provider"], item["unit"])
        if not all(isinstance(part, str) and part for part in key):
            raise StateError("mission usage provider/unit is invalid")
        if key in seen_units:
            raise StateError("mission usage repeats a provider/unit tuple")
        seen_units.add(key)
        if (
            isinstance(item["value"], bool)
            or not isinstance(item["value"], (int, float))
            or not math.isfinite(item["value"])
            or item["value"] < 0
        ):
            raise StateError("mission usage value is invalid")
    if not isinstance(state["turnLog"], list) or not all(isinstance(item, dict) for item in state["turnLog"]):
        raise StateError("mission turn log must be an array of objects")
    if not isinstance(state["waitingList"], list) or len(set(state["waitingList"])) != len(state["waitingList"]):
        raise StateError("mission waiting list must contain unique ask ids")
    if not all(isinstance(item, str) and ID_RE.fullmatch(item) for item in state["waitingList"]):
        raise StateError("mission waiting list has an invalid ask id")
    if state["runnerLease"] is not None and not isinstance(state["runnerLease"], str):
        raise StateError("mission runner lease reference is invalid")
    ledger = state["ledger"]
    if not isinstance(ledger, dict) or set(ledger) != {"path", "cycles"}:
        raise StateError("mission ledger reference is invalid")
    if not isinstance(ledger["path"], str) or not ledger["path"] or not is_nonnegative_int(ledger["cycles"]):
        raise StateError("mission ledger path or cycle count is invalid")
    integrity = state["integrity"]
    if not isinstance(integrity, dict) or set(integrity) != {"sequence", "previousHash", "hash", "history", "recoveryOf"}:
        raise StateError("mission integrity block has an invalid shape")
    if not is_nonnegative_int(integrity["sequence"]):
        raise StateError("mission integrity sequence is invalid")
    for field in ("previousHash", "recoveryOf"):
        if integrity[field] is not None and (not isinstance(integrity[field], str) or not HASH_RE.fullmatch(integrity[field])):
            raise StateError(f"mission integrity {field} is invalid")
    if not isinstance(integrity["hash"], str) or not HASH_RE.fullmatch(integrity["hash"]):
        raise StateError("mission integrity hash is invalid")
    if not isinstance(integrity["history"], list):
        raise StateError("mission integrity history is invalid")


def validate_aggregation(state: dict[str, Any]) -> None:
    active = any(stream["state"] == "active" for stream in state["streams"].values())
    if state["status"] == "completed":
        if not state["gatePassed"] or state["parkReason"] is not None:
            raise StateError("completed mission state requires a passed gate and no park reason")
    elif state["status"] == "running":
        if state["gatePassed"] or state["parkReason"] is not None or not active:
            raise StateError("running mission state requires an active stream, no park reason, and an unpassed gate")
    else:
        if state["gatePassed"] or state["parkReason"] is None:
            raise StateError("parked mission state requires an unpassed gate and a reason")
        if state["parkReason"] == "all-streams-parked" and active:
            raise StateError("all-streams-parked cannot retain an active stream")


def validate_chain(state: dict[str, Any]) -> None:
    history = state["integrity"]["history"]
    sequence = state["integrity"]["sequence"]
    if len(history) != sequence + 1:
        raise StateError("mission state hash chain has a missing or forked sequence")
    previous = None
    seen_hashes: set[str] = set()
    for expected_sequence, entry in enumerate(history):
        if not isinstance(entry, dict) or set(entry) != {"sequence", "previousHash", "hash"}:
            raise StateError("mission state hash-chain entry has an invalid shape")
        if entry["sequence"] != expected_sequence or entry["previousHash"] != previous:
            raise StateError("mission state hash chain has a fork")
        value_hash = entry["hash"]
        if not isinstance(value_hash, str) or not HASH_RE.fullmatch(value_hash) or value_hash in seen_hashes:
            raise StateError("mission state hash chain repeats or corrupts a hash")
        seen_hashes.add(value_hash)
        previous = value_hash
    current = state["integrity"]
    if current["previousHash"] != (history[-2]["hash"] if sequence else None):
        raise StateError("mission state previous hash disagrees with history")
    if current["hash"] != history[-1]["hash"] or current["hash"] != state_hash(state):
        raise StateError("mission state hash does not match its bytes")


def validate(state: dict[str, Any]) -> None:
    validate_shape(state)
    validate_aggregation(state)
    validate_chain(state)


def lock_for(path: Path):
    lock = path.with_name(path.name + ".lock")
    lock.parent.mkdir(parents=True, exist_ok=True)
    return lock.open("a+")


def finalize_next(next_state: dict[str, Any], previous: dict[str, Any] | None, recovery_of: str | None = None) -> dict[str, Any]:
    value = copy.deepcopy(next_state)
    if previous is None:
        sequence = 0
        previous_hash = None
        history: list[dict[str, Any]] = []
    else:
        sequence = previous["integrity"]["sequence"] + 1
        previous_hash = previous["integrity"]["hash"]
        history = copy.deepcopy(previous["integrity"]["history"])
    value["integrity"] = {
        "sequence": sequence,
        "previousHash": previous_hash,
        "hash": "0" * 64,
        "history": history,
        "recoveryOf": recovery_of,
    }
    digest = state_hash(value)
    value["integrity"]["hash"] = digest
    value["integrity"]["history"].append({"sequence": sequence, "previousHash": previous_hash, "hash": digest})
    validate(value)
    return value


def authored_contract_values(contract: Path) -> dict[str, str]:
    text = contract.read_text(encoding="utf-8")
    blocks = re.findall(r"^```mission[ \t]*\n(.*?)^```[ \t]*$", text, re.MULTILINE | re.DOTALL)
    if len(blocks) != 1:
        raise StateError("mission contract does not have exactly one authored block")
    values = {}
    for raw in blocks[0].splitlines():
        if not raw.strip():
            continue
        key, separator, value = raw.partition("=")
        if not separator or key in values:
            raise StateError("mission contract key/value grammar is invalid")
        values[key] = value
    return values


def command_init(args: argparse.Namespace) -> None:
    values = authored_contract_values(args.contract)
    match = re.fullmatch(r"mission-([a-z0-9][a-z0-9-]*)\.contract\.md", args.contract.name)
    if match is None:
        raise StateError("mission contract filename is invalid")
    streams = {
        key.removeprefix("stream."): {"goal": value, "state": "active", "reason": None, "answeredAsk": None}
        for key, value in values.items()
        if key.startswith("stream.")
    }
    branch = args.branch or values.get("candidate.branch")
    if branch is None:
        seal = re.findall(r"^```mission-seal[ \t]*\n(.*?)^```[ \t]*$", args.contract.read_text(), re.MULTILINE | re.DOTALL)
        for raw in seal[0].splitlines() if seal else []:
            if raw.startswith("candidate.branch="):
                branch = raw.split("=", 1)[1]
    if not branch:
        raise StateError("sealed candidate branch is absent")
    now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    body = {
        "schemaVersion": 1,
        "missionId": match.group(1),
        "branch": branch,
        "status": "running",
        "parkReason": None,
        "gatePassed": False,
        "streams": streams,
        "fences": {"startedAt": now, "cycles": 0, "jobs": 0, "activeJobs": 0, "usage": []},
        "turnLog": [],
        "waitingList": [],
        "runnerLease": args.lease,
        "ledger": {"path": str(args.ledger), "cycles": 0},
        "integrity": {},
    }
    with lock_for(args.state) as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        if args.state.exists():
            raise StateError("mission state already exists")
        atomic_write(args.state, finalize_next(body, None))


def validate_transition(previous: dict[str, Any], next_state: dict[str, Any]) -> None:
    immutable = ("schemaVersion", "missionId", "branch")
    if any(previous[key] != next_state.get(key) for key in immutable):
        raise StateError("mission state update changes immutable identity")
    if set(previous["streams"]) != set(next_state.get("streams", {})):
        raise StateError("mission state update changes the declared stream set")
    for stream_id, old in previous["streams"].items():
        new = next_state["streams"][stream_id]
        if old["goal"] != new.get("goal"):
            raise StateError(f"mission stream {stream_id} goal is immutable")
        if new.get("state") not in LEGAL_STREAM_TRANSITIONS[old["state"]]:
            raise StateError(f"illegal mission stream transition: {old['state']} to {new.get('state')}")
        answered_changed = new.get("answeredAsk") != old.get("answeredAsk")
        human_answer_transition = (
            (old["state"] in {"parked-reserved", "parked-stop-loss"} and new.get("state") == "active")
            or (old["state"] != "parked-stop-loss" and new.get("state") == "parked-stop-loss")
        )
        if answered_changed and not human_answer_transition:
            raise StateError("mission stream answered ask changes only on a human-answer transition")
        if (
            old["state"] == "parked-reserved"
            and new.get("state") == "active"
            and (not new.get("answeredAsk") or not answered_changed)
        ):
            raise StateError("parked-reserved can reactivate only with an answered ask")
        if (
            old["state"] == "parked-stop-loss"
            and new.get("state") == "active"
            and (not new.get("answeredAsk") or not answered_changed)
        ):
            raise StateError("parked-stop-loss can reactivate only with a human budget answer")
        if (
            old["state"] != "parked-stop-loss"
            and new.get("state") == "parked-stop-loss"
            and (not new.get("answeredAsk") or not answered_changed)
        ):
            raise StateError("parked-stop-loss is reserved for a human answer")
    if previous["status"] == "completed" and next_state.get("status") != "completed":
        raise StateError("completed mission state is terminal")
    for field in ("cycles", "jobs"):
        if next_state["fences"][field] < previous["fences"][field]:
            raise StateError(f"mission fence counter {field} cannot decrease")
    if next_state["ledger"]["cycles"] < previous["ledger"]["cycles"]:
        raise StateError("mission ledger cycle count cannot decrease")


def command_write(args: argparse.Namespace) -> None:
    proposed = read_json(args.source)
    proposed.pop("integrity", None)
    with lock_for(args.state) as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        previous = read_json(args.state)
        validate(previous)
        if previous["integrity"]["hash"] != args.expect:
            raise StateError("mission state compare-and-write hash mismatch")
        validate_transition(previous, proposed)
        atomic_write(args.state, finalize_next(proposed, previous))


def git(repo: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        ["git", "-C", str(repo), *args],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if check and result.returncode != 0:
        raise StateError(f"git {' '.join(args)} failed: {result.stderr.strip()}")
    return result


def ledger_cycle_count(path: Path) -> int:
    text = path.read_text(encoding="utf-8")
    headings = [int(value) for value in re.findall(r"^### Cycle ([1-9][0-9]*)[ \t]*$", text, re.MULTILINE)]
    if headings != list(range(1, len(headings) + 1)):
        raise StateError("mission ledger cycle headings are not contiguous")
    for number, block in enumerate(re.split(r"^### Cycle [1-9][0-9]*[ \t]*$", text, flags=re.MULTILINE)[1:], 1):
        if len(re.findall(r"^- Classification:[ \t]*", block, re.MULTILINE)) != 1:
            raise StateError(f"mission ledger Cycle {number} lacks exactly one classification")
    return len(headings)


def ledger_hash(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def command_anchor(args: argparse.Namespace) -> None:
    state = read_json(args.state)
    validate(state)
    cycles = ledger_cycle_count(args.ledger)
    if cycles != state["ledger"]["cycles"]:
        raise StateError("anchor refused: ledger is truth and its cycle count disagrees with state")
    branch = git(args.repo, "branch", "--show-current").stdout.strip()
    if branch != state["branch"]:
        raise StateError("anchor refused: current branch is not the mission branch")
    if git(args.repo, "diff", "--cached", "--quiet", check=False).returncode != 0:
        raise StateError("anchor refused: staged changes would be swept into the local anchor commit")
    try:
        ledger_relative = args.ledger.resolve().relative_to(args.repo.resolve()).as_posix()
    except ValueError as error:
        raise StateError("anchor refused: mission ledger is outside the repository") from error
    git(args.repo, "add", "-f", "--", ledger_relative)
    subject = f"mission({state['missionId']}): anchor cycle {cycles}"
    body = (
        f"Mission-Id: {state['missionId']}\n"
        f"Mission-State-Hash: {state['integrity']['hash']}\n"
        f"Mission-Ledger-SHA256: {ledger_hash(args.ledger)}\n"
        f"Mission-Ledger-Path: {ledger_relative}\n"
        f"Mission-Cycle: {cycles}"
    )
    git(args.repo, "commit", "--allow-empty", "-m", subject, "-m", body)
    print(git(args.repo, "rev-parse", "HEAD").stdout.strip())


def latest_anchor(repo: Path, mission: str) -> dict[str, str]:
    output = git(repo, "log", "--format=%H%x1f%B%x1e").stdout
    for raw in output.split("\x1e"):
        if not raw.strip():
            continue
        commit, _, message = raw.partition("\x1f")
        trailers = dict(re.findall(r"^(Mission-[A-Za-z0-9-]+): (.+)$", message, re.MULTILINE))
        if trailers.get("Mission-Id") == mission:
            trailers["commit"] = commit.strip()
            return trailers
    raise StateError("mission state has no local anchor commit")


def verify_anchor(repo: Path, state: dict[str, Any], ledger: Path) -> None:
    anchor = latest_anchor(repo, state["missionId"])
    try:
        ledger_relative = ledger.resolve().relative_to(repo.resolve()).as_posix()
    except ValueError as error:
        raise StateError("mission ledger is outside the repository") from error
    expected = {
        "Mission-State-Hash": state["integrity"]["hash"],
        "Mission-Ledger-SHA256": ledger_hash(ledger),
        "Mission-Ledger-Path": ledger_relative,
        "Mission-Cycle": str(state["ledger"]["cycles"]),
    }
    for key, value in expected.items():
        if anchor.get(key) != value:
            raise StateError(f"mission anchor disagrees at {key}")
    if git(repo, "merge-base", "--is-ancestor", anchor["commit"], state["branch"], check=False).returncode != 0:
        raise StateError("mission anchor commit is not on the mission branch")
    anchored = git(repo, "show", f"{anchor['commit']}:{ledger_relative}", check=False)
    if anchored.returncode != 0 or hashlib.sha256(anchored.stdout.encode()).hexdigest() != anchor["Mission-Ledger-SHA256"]:
        raise StateError("mission anchor commit does not contain the declared ledger bytes")


def verify_state_anchor(repo: Path, state: dict[str, Any], ledger: Path) -> None:
    anchor = latest_anchor(repo, state["missionId"])
    if anchor.get("Mission-State-Hash") != state["integrity"]["hash"]:
        raise StateError("mission anchor disagrees at Mission-State-Hash")
    if anchor.get("Mission-Cycle") != str(state["ledger"]["cycles"]):
        raise StateError("mission anchor disagrees at Mission-Cycle")
    try:
        ledger_relative = ledger.resolve().relative_to(repo.resolve()).as_posix()
    except ValueError as error:
        raise StateError("mission ledger is outside the repository") from error
    if anchor.get("Mission-Ledger-Path") != ledger_relative:
        raise StateError("mission anchor disagrees at Mission-Ledger-Path")
    anchored = git(repo, "show", f"{anchor['commit']}:{ledger_relative}", check=False)
    if anchored.returncode != 0:
        raise StateError("mission anchor commit does not contain the prior ledger")
    anchored_bytes = anchored.stdout.encode()
    if hashlib.sha256(anchored_bytes).hexdigest() != anchor.get("Mission-Ledger-SHA256"):
        raise StateError("mission anchor prior ledger hash is invalid")
    if not ledger.read_bytes().startswith(anchored_bytes):
        raise StateError("mission ledger does not extend the anchored ledger truth")
    if git(repo, "merge-base", "--is-ancestor", anchor["commit"], state["branch"], check=False).returncode != 0:
        raise StateError("mission anchor commit is not on the mission branch")


def park_integrity(path: Path, state: dict[str, Any], recovery_of: str | None = None) -> None:
    parked = copy.deepcopy(state)
    parked["status"] = "parked"
    parked["parkReason"] = "state-integrity"
    parked["gatePassed"] = False
    atomic_write(path, finalize_next(parked, state, recovery_of=recovery_of))


def command_reconcile(args: argparse.Namespace) -> int:
    with lock_for(args.state) as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        raw = read_json(args.state)
        try:
            validate(raw)
        except StateError:
            corrupt_hash = hashlib.sha256(args.state.read_bytes()).hexdigest()
            evidence = args.state.with_name(f"state.corrupt.{corrupt_hash}.json")
            if not evidence.exists():
                evidence.write_bytes(args.state.read_bytes())
            # The last advertised hash remains the predecessor; recovery is
            # explicit and parked, never presented as an intact old chain.
            raw["integrity"]["hash"] = raw["integrity"].get("hash", "0" * 64)
            if not HASH_RE.fullmatch(raw["integrity"]["hash"]):
                raw["integrity"]["hash"] = "0" * 64
            history = raw["integrity"].get("history")
            if not isinstance(history, list):
                history = []
            raw["integrity"]["history"] = history
            raw["integrity"]["sequence"] = len(history) - 1
            raw["integrity"]["previousHash"] = history[-2]["hash"] if len(history) > 1 else None
            raw["status"] = "parked"
            raw["parkReason"] = "state-integrity"
            raw["gatePassed"] = False
            # An invalid predecessor cannot honestly head a continued chain.
            # Preserve its exact bytes, start a visibly recovered chain, and
            # leave the mission parked for human reconciliation.
            atomic_write(args.state, finalize_next(raw, None, recovery_of=corrupt_hash))
            return 3
        try:
            cycles = ledger_cycle_count(args.ledger)
        except (OSError, StateError):
            park_integrity(args.state, raw)
            return 3
        if cycles < raw["ledger"]["cycles"]:
            park_integrity(args.state, raw)
            return 3
        if cycles > raw["ledger"]["cycles"]:
            try:
                verify_state_anchor(args.repo, raw, args.ledger)
            except StateError:
                park_integrity(args.state, raw)
                return 3
            proposed = copy.deepcopy(raw)
            proposed["ledger"]["cycles"] = cycles
            proposed["fences"]["cycles"] = max(proposed["fences"]["cycles"], cycles)
            raw = finalize_next(proposed, raw)
            atomic_write(args.state, raw)
        else:
            try:
                verify_anchor(args.repo, raw, args.ledger)
            except StateError:
                park_integrity(args.state, raw)
                return 3
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)
    init = subparsers.add_parser("init")
    init.add_argument("--state", required=True, type=Path)
    init.add_argument("--contract", required=True, type=Path)
    init.add_argument("--ledger", required=True, type=Path)
    init.add_argument("--lease")
    init.add_argument("--branch")
    write = subparsers.add_parser("write")
    write.add_argument("--state", required=True, type=Path)
    write.add_argument("--source", required=True, type=Path)
    write.add_argument("--expect", required=True)
    verify_parser = subparsers.add_parser("verify")
    verify_parser.add_argument("--state", required=True, type=Path)
    verify_parser.add_argument("--repo", type=Path)
    verify_parser.add_argument("--ledger", type=Path)
    anchor = subparsers.add_parser("anchor")
    anchor.add_argument("--state", required=True, type=Path)
    anchor.add_argument("--repo", required=True, type=Path)
    anchor.add_argument("--ledger", required=True, type=Path)
    reconcile = subparsers.add_parser("reconcile")
    reconcile.add_argument("--state", required=True, type=Path)
    reconcile.add_argument("--repo", required=True, type=Path)
    reconcile.add_argument("--ledger", required=True, type=Path)
    args = parser.parse_args()
    try:
        if args.command == "init":
            command_init(args)
        elif args.command == "write":
            if not HASH_RE.fullmatch(args.expect):
                raise StateError("--expect must be a state hash")
            command_write(args)
        elif args.command == "anchor":
            command_anchor(args)
        elif args.command == "reconcile":
            return command_reconcile(args)
        else:
            state = read_json(args.state)
            validate(state)
            if (args.repo is None) != (args.ledger is None):
                raise StateError("--repo and --ledger are required together for anchor verification")
            if args.repo is not None:
                verify_anchor(args.repo, state, args.ledger)
            print(f"mission state valid: sequence={state['integrity']['sequence']} hash={state['integrity']['hash']}")
    except StateError as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
