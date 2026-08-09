#!/usr/bin/env python3
"""Serialized mission lifecycle fences, batched asks, and typed usage."""

from __future__ import annotations

import argparse
import fcntl
import hashlib
import importlib.util
import json
import os
import re
import sys
import tempfile
from datetime import datetime, timedelta, timezone
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any


ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
TERMINAL = {"completed", "failed", "timeout", "cancelled"}


class FenceError(RuntimeError):
    pass


def now() -> datetime:
    return datetime.now(timezone.utc)


def now_iso() -> str:
    return now().strftime("%Y-%m-%dT%H:%M:%SZ")


def atomic_json(path: Path, value: dict[str, Any]) -> None:
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


def canonical_model(name: str, repo: Path) -> str:
    helper = repo / "scripts" / "agents" / "canonical-model.py"
    specification = importlib.util.spec_from_file_location("mission_fence_canonical_model", helper)
    if specification is None or specification.loader is None:
        raise FenceError("canonical model helper is unavailable")
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return str(module.canonical_model(name))


def contract_values_from_bytes(data: bytes, repo: Path) -> dict[str, str]:
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as error:
        raise FenceError(f"mission contract is not UTF-8: {error}") from error
    blocks = re.findall(r"^```mission[ \t]*\n(.*?)^```[ \t]*$", text, re.MULTILINE | re.DOTALL)
    if len(blocks) != 1:
        raise FenceError("mission contract does not have exactly one authored block")
    values: dict[str, str] = {}
    for raw in blocks[0].splitlines():
        if not raw.strip():
            continue
        key, separator, value = raw.partition("=")
        if not separator or key in values:
            raise FenceError("mission contract key/value grammar is invalid")
        values[key] = value
    required = {
        "fence.wall-clock-hours",
        "fence.cycles",
        "fence.jobs",
        "fence.concurrency",
        "fence.job-cap-min",
    }
    if not required.issubset(values):
        raise FenceError("mission contract lacks a universal lifecycle fence")
    try:
        wall = Decimal(values["fence.wall-clock-hours"])
    except InvalidOperation as error:
        raise FenceError("mission wall-clock fence is invalid") from error
    if wall <= 0:
        raise FenceError("mission wall-clock fence is invalid")
    for key in required - {"fence.wall-clock-hours"}:
        if not re.fullmatch(r"[1-9][0-9]*", values[key]):
            raise FenceError(f"mission {key} is invalid")
    for key, value in values.items():
        if not key.startswith("cap.min."):
            continue
        parts = key.split(".", 3)
        if len(parts) != 4 or not ID_RE.fullmatch(parts[2]):
            raise FenceError(f"mission pair-cap key is invalid: {key}")
        encoded = canonical_model(parts[3], repo)
        expected = f"cap.min.{parts[2]}.{encoded}"
        if not encoded or key != expected:
            raise FenceError(f"mission pair-cap key is not canonical: {key}; use {expected}")
        if not re.fullmatch(r"[1-9][0-9]*", value):
            raise FenceError(f"mission {key} is invalid")
    return values


def verified_contract_values(
    repo: Path,
    mission: str,
    fences: dict[str, Any],
    after_buffer_read: Any = None,
) -> dict[str, str]:
    """Read, hash, compare, and parse one raw-file snapshot while the caller holds the fence lock."""
    # approvedContractSha256 is always the SHA-256 digest of the exact raw file
    # bytes, including the Approval line and trailing whitespace. It is not the
    # canonical signed-content digest carried by the Approval line.
    approved = fences.get("approvedContractSha256")
    if not isinstance(approved, str) or not re.fullmatch(r"[0-9a-f]{64}", approved):
        raise FenceError("mission fence refused: approvedContractSha256 is absent or invalid")
    path = repo / "plans" / f"mission-{mission}.contract.md"
    try:
        snapshot = path.read_bytes()
    except OSError as error:
        raise FenceError(f"mission contract is unreadable: {error}") from error
    if after_buffer_read is not None:
        after_buffer_read()
    actual = hashlib.sha256(snapshot).hexdigest()
    if actual != approved:
        raise FenceError(
            "mission fence refused: live contract raw-file sha256 does not match approvedContractSha256"
        )
    return contract_values_from_bytes(snapshot, repo)


def mission_paths(repo: Path, mission: str) -> tuple[Path, Path, Path]:
    directory = repo / "artifacts" / "agents" / "missions" / mission
    return directory, directory / "fences.json", directory / "mission-fence.lock"


def load_fences(repo: Path, mission: str) -> dict[str, Any]:
    directory, path, _ = mission_paths(repo, mission)
    if path.exists():
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as error:
            raise FenceError(f"mission fence counters are unreadable: {error}") from error
    else:
        started = now_iso()
        lease = directory / "lease.json"
        if lease.exists():
            try:
                lease_value = json.loads(lease.read_text(encoding="utf-8"))
                datetime.fromisoformat(lease_value["startedAt"].replace("Z", "+00:00"))
                started = lease_value["startedAt"]
            except (OSError, ValueError, KeyError, TypeError):
                raise FenceError("mission lease start time is invalid")
        value = {"schemaVersion": 1, "missionId": mission, "startedAt": started, "cycles": 0, "reservations": {}}
    if (
        not isinstance(value, dict)
        or value.get("schemaVersion") != 1
        or value.get("missionId") != mission
        or not isinstance(value.get("cycles"), int)
        or isinstance(value.get("cycles"), bool)
        or value["cycles"] < 0
        or not isinstance(value.get("reservations"), dict)
    ):
        raise FenceError("mission fence counters have an invalid shape")
    try:
        started = datetime.fromisoformat(value["startedAt"].replace("Z", "+00:00"))
    except (AttributeError, ValueError):
        raise FenceError("mission fence start time is invalid")
    if (started.astimezone(timezone.utc) - now()).total_seconds() > 5:
        raise FenceError("mission fence start time is in the future")
    return value


def job_status(repo: Path, job: str) -> str | None:
    path = repo / "artifacts" / "agents" / "jobs" / f"{job}.json"
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    return value.get("status") if isinstance(value, dict) else None


def active_reservations(repo: Path, fences: dict[str, Any]) -> list[str]:
    return sorted(job for job in fences["reservations"] if job_status(repo, job) not in TERMINAL)


def violations(repo: Path, values: dict[str, str], fences: dict[str, Any], cap_min: int | None, reserve: str) -> list[str]:
    started = datetime.fromisoformat(fences["startedAt"].replace("Z", "+00:00"))
    elapsed_hours = Decimal(str((now() - started.astimezone(timezone.utc)).total_seconds())) / Decimal(3600)
    result = []
    if elapsed_hours >= Decimal(values["fence.wall-clock-hours"]):
        result.append("wall-clock-hours")
    if fences["cycles"] >= int(values["fence.cycles"]):
        result.append("cycles")
    if reserve in {"job", "authorized-job"}:
        if len(fences["reservations"]) >= int(values["fence.jobs"]):
            result.append("jobs")
        if len(active_reservations(repo, fences)) >= int(values["fence.concurrency"]):
            result.append("concurrency")
    if reserve == "job":
        if cap_min is None or cap_min > int(values["fence.job-cap-min"]):
            result.append("job-cap-min")
    return result


def write_batched_ask(repo: Path, mission: str, reasons: list[str]) -> Path:
    asks = repo / "artifacts" / "agents" / "missions" / mission / "asks"
    asks.mkdir(parents=True, exist_ok=True)
    existing: list[tuple[int, Path, dict[str, Any]]] = []
    for path in asks.glob("fence-bound*.json"):
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            continue
        match = re.fullmatch(r"fence-bound(?:-([1-9][0-9]*))?\.json", path.name)
        if match and value.get("answeredAt") is None:
            existing.append((int(match.group(1) or 1), path, value))
    if existing:
        _, path, value = max(existing)
        previous = set(re.findall(r"`([a-z-]+)`", value.get("question", "")))
        combined = sorted(previous | set(reasons))
    else:
        indices = []
        for path in asks.glob("fence-bound*.json"):
            match = re.fullmatch(r"fence-bound(?:-([1-9][0-9]*))?\.json", path.name)
            if match:
                indices.append(int(match.group(1) or 1))
        index = max(indices, default=0) + 1
        ask_id = "fence-bound" if index == 1 else f"fence-bound-{index}"
        path = asks / f"{ask_id}.json"
        value = {
            "askId": ask_id,
            "streamId": None,
            "reasonClass": "fence",
            "question": "",
            "createdAt": now_iso(),
            "answeredAt": None,
            "answer": None,
        }
        combined = sorted(set(reasons))
    named = ", ".join(f"`{reason}`" for reason in combined)
    value["question"] = (
        f"Mission {mission} reached lifecycle fence(s) {named}. "
        "Choose whether to amend, price, reseal, and sign the contract or leave the mission parked."
    )
    atomic_json(path, value)
    return path


def check_or_reserve(args: argparse.Namespace, reserve: bool) -> None:
    directory, path, lock_path = mission_paths(args.repo, args.mission)
    directory.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a+") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        fences = load_fences(args.repo, args.mission)
        values = verified_contract_values(args.repo, args.mission, fences)
        found = violations(args.repo, values, fences, args.cap_min, "job")
        if found:
            ask = write_batched_ask(args.repo, args.mission, found)
            raise FenceError(f"mission fence refused job ({', '.join(found)}); batched ask written: {ask}")
        if reserve:
            if args.job in fences["reservations"]:
                raise FenceError(f"mission fence reservation already exists for job: {args.job}")
            fences["reservations"][args.job] = {"reservedAt": now_iso(), "capMin": args.cap_min}
            atomic_json(path, fences)


def reserve_cycle(args: argparse.Namespace) -> None:
    directory, path, lock_path = mission_paths(args.repo, args.mission)
    directory.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a+") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        fences = load_fences(args.repo, args.mission)
        values = verified_contract_values(args.repo, args.mission, fences)
        found = violations(args.repo, values, fences, None, "cycle")
        if found:
            ask = write_batched_ask(args.repo, args.mission, found)
            raise FenceError(f"mission fence refused cycle ({', '.join(found)}); batched ask written: {ask}")
        fences["cycles"] += 1
        atomic_json(path, fences)


def authorize_cap_transaction(args: argparse.Namespace, after_buffer_read: Any = None) -> dict[str, Any]:
    directory, path, lock_path = mission_paths(args.repo, args.mission)
    directory.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a+") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        fences = load_fences(args.repo, args.mission)
        values = verified_contract_values(args.repo, args.mission, fences, after_buffer_read)
        pair_key = f"cap.min.{args.runtime}.{args.model}"
        if pair_key in values:
            authorized = int(values[pair_key])
            signed_rule = "contract-pair"
        else:
            authorized = int(values["fence.job-cap-min"])
            signed_rule = "fence-default"
        if args.requested is not None and args.requested > authorized:
            raise FenceError(
                f"mission fence refused requested cap {args.requested}m above signed {pair_key if pair_key in values else 'fence.job-cap-min'}={authorized}m"
            )
        cap_min = args.requested if args.requested is not None else authorized
        found = violations(args.repo, values, fences, None, "authorized-job")
        if found:
            ask = write_batched_ask(args.repo, args.mission, found)
            raise FenceError(f"mission fence refused job ({', '.join(found)}); batched ask written: {ask}")

        launch_time = now().replace(microsecond=0)
        mission_started = datetime.fromisoformat(fences["startedAt"].replace("Z", "+00:00")).astimezone(timezone.utc)
        mission_end = mission_started + timedelta(seconds=float(Decimal(values["fence.wall-clock-hours"]) * 3600))
        remaining_seconds = int((mission_end - launch_time).total_seconds())
        if remaining_seconds < 120:
            raise FenceError(
                f"mission has {remaining_seconds} seconds of wall clock; refusing to start a job that cannot run"
            )
        requested_deadline = launch_time + timedelta(minutes=cap_min)
        truncated = requested_deadline > mission_end
        deadline = min(requested_deadline, mission_end).strftime("%Y-%m-%dT%H:%M:%SZ")
        source = {
            "rule": "argument" if args.requested is not None else signed_rule,
            "origin": "argument" if args.requested is not None else "contract",
            "truncatedBy": "wall-clock" if truncated else None,
        }
        if args.job in fences["reservations"]:
            raise FenceError(f"mission fence reservation already exists for job: {args.job}")
        fences["reservations"][args.job] = {
            "reservedAt": now_iso(),
            "capMin": cap_min,
            "capDeadline": deadline,
            "runtime": args.runtime,
            "model": args.model,
            "source": source,
        }
        atomic_json(path, fences)
        return {"capMin": cap_min, "capDeadline": deadline, "source": source}


def authorize_cap(args: argparse.Namespace) -> None:
    print(json.dumps(authorize_cap_transaction(args), separators=(",", ":"), sort_keys=True))


def aggregate_usage(args: argparse.Namespace) -> None:
    directory, _, lock_path = mission_paths(args.repo, args.mission)
    directory.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a+") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        units: dict[tuple[str, str], float] = {}
        unavailable: list[str] = []
        jobs = args.repo / "artifacts" / "agents" / "jobs"
        for path in sorted(jobs.glob("*.json")) if jobs.exists() else []:
            try:
                record = json.loads(path.read_text(encoding="utf-8"))
            except (OSError, ValueError):
                continue
            if record.get("mission") != args.mission or record.get("status") not in TERMINAL:
                continue
            usage = record.get("usage")
            if not isinstance(usage, dict):
                unavailable.append(record.get("jobId", path.stem))
                continue
            provider = record.get("runtime", "unknown")
            # `availability: unavailable` means no TOKEN counts, not "no usage".
            # A runtime that reports provider units instead of tokens -- an
            # enterprise Devin reporting ACU -- is measured, and gating the
            # whole record on availability discarded that measurement before it
            # was read, leaving the fence unable to stop a runaway mission on
            # exactly the account that reports no tokens. Tokens depend on
            # availability; provider units and cost are metered whenever present.
            measured = False
            if usage.get("availability") != "unavailable":
                for field in ("inputTokens", "cachedInputTokens", "outputTokens", "reasoningTokens"):
                    value = usage.get(field)
                    if isinstance(value, (int, float)) and not isinstance(value, bool) and value >= 0:
                        key = (provider, f"tokens.{field}")
                        units[key] = units.get(key, 0) + value
                        measured = True
            cost = usage.get("cost")
            if (
                isinstance(cost, dict)
                and isinstance(cost.get("currency"), str)
                and isinstance(cost.get("amount"), (int, float))
                and not isinstance(cost.get("amount"), bool)
                and cost["amount"] >= 0
            ):
                key = (provider, f"cost.{cost['currency']}")
                units[key] = units.get(key, 0) + cost["amount"]
                measured = True
            native = usage.get("providerUnits")
            if (
                isinstance(native, dict)
                and isinstance(native.get("name"), str)
                and isinstance(native.get("value"), (int, float))
                and not isinstance(native.get("value"), bool)
                and native["value"] >= 0
            ):
                key = (provider, f"provider.{native['name']}")
                units[key] = units.get(key, 0) + native["value"]
                measured = True
            # A record measured by nothing at all is the one the fence cannot
            # bound; that, not "reported no tokens", is what belongs on the
            # unavailable list.
            if not measured:
                unavailable.append(record.get("jobId", path.stem))
        value = {
            "schemaVersion": 1,
            "missionId": args.mission,
            "units": [
                {"provider": provider, "unit": unit, "value": value}
                for (provider, unit), value in sorted(units.items())
            ],
            "unavailableJobs": sorted(unavailable),
            "updatedAt": now_iso(),
        }
        atomic_json(directory / "usage.json", value)


def refuse(args: argparse.Namespace) -> None:
    allowed = {"wall-clock-hours", "cycles", "jobs", "concurrency", "job-cap-min"}
    if args.reason not in allowed:
        raise FenceError("unknown mission fence refusal reason")
    directory, _, lock_path = mission_paths(args.repo, args.mission)
    directory.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a+") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        ask = write_batched_ask(args.repo, args.mission, [args.reason])
    print(ask)


def main() -> int:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)
    for name in ("check-job", "reserve-job"):
        command = subparsers.add_parser(name)
        command.add_argument("--repo", required=True, type=Path)
        command.add_argument("--mission", required=True)
        command.add_argument("--job", required=True)
        command.add_argument("--cap-min", required=True, type=int)
    cycle = subparsers.add_parser("reserve-cycle")
    cycle.add_argument("--repo", required=True, type=Path)
    cycle.add_argument("--mission", required=True)
    authorize = subparsers.add_parser("authorize-cap")
    authorize.add_argument("--repo", required=True, type=Path)
    authorize.add_argument("--mission", required=True)
    authorize.add_argument("--job", required=True)
    authorize.add_argument("--runtime", required=True)
    authorize.add_argument("--model", required=True)
    authorize.add_argument("--requested", type=int)
    usage = subparsers.add_parser("aggregate-usage")
    usage.add_argument("--repo", required=True, type=Path)
    usage.add_argument("--mission", required=True)
    refusal = subparsers.add_parser("refuse")
    refusal.add_argument("--repo", required=True, type=Path)
    refusal.add_argument("--mission", required=True)
    refusal.add_argument("--reason", required=True)
    args = parser.parse_args()
    try:
        if not ID_RE.fullmatch(args.mission):
            raise FenceError("invalid mission id")
        args.repo = args.repo.resolve()
        if args.command in {"check-job", "reserve-job"}:
            if not ID_RE.fullmatch(args.job) or args.cap_min < 1:
                raise FenceError("invalid mission job reservation")
            check_or_reserve(args, reserve=args.command == "reserve-job")
        elif args.command == "reserve-cycle":
            reserve_cycle(args)
        elif args.command == "authorize-cap":
            if (
                not ID_RE.fullmatch(args.job)
                or not ID_RE.fullmatch(args.runtime)
                or args.model != canonical_model(args.model, args.repo)
                or not args.model
                or (args.requested is not None and args.requested < 1)
            ):
                raise FenceError("invalid mission cap authorization request")
            authorize_cap(args)
        elif args.command == "aggregate-usage":
            aggregate_usage(args)
        else:
            refuse(args)
    except FenceError as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
