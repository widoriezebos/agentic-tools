#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
exec python3 - "$root" "$script" "$@" <<'PY'
from __future__ import annotations

import copy
import fcntl
import importlib.util
import json
import math
import os
import re
import secrets
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timezone
from decimal import Decimal
from pathlib import Path
from typing import Any


ROOT = Path(sys.argv[1]).resolve()
SCRIPT = Path(sys.argv[2]).resolve()
ARGV = sys.argv[3:]
AGENTS = ROOT / "artifacts" / "agents"
MISSIONS = AGENTS / "missions"
RUNNERS = MISSIONS / "runners"
ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
TERMINAL_JOBS = {"completed", "failed", "timeout", "cancelled"}
KNOWN_ASK_REASONS = {"reserved-decision", "red-test", "merge-conflict", "host-failure"}
LEGAL_STREAM_TRANSITIONS = {
    "active": {"active", "parked-reserved", "parked-stop-loss", "done"},
    "parked-reserved": {"parked-reserved"},
    "parked-stop-loss": {"parked-stop-loss"},
    "done": {"done"},
}


class RunnerError(RuntimeError):
    def __init__(self, message: str, code: int = 3):
        super().__init__(message)
        self.code = code


def usage() -> None:
    print(
        "Usage:\n"
        "  scripts/agents/mission-runner.sh start --mission <id> [--foreground]\n"
        "  scripts/agents/mission-runner.sh resume --mission <id> [--foreground]\n"
        "  scripts/agents/mission-runner.sh status --mission <id>\n"
        "  scripts/agents/mission-runner.sh answer --mission <id> --ask <ask-id> --answer <text>",
        file=sys.stderr,
    )


def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


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


def atomic_text(path: Path, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=path.name + ".", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(value)
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


def read_json(path: Path, label: str, code: int = 3) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise RunnerError(f"{label} is unreadable: {path}: {error}", code) from error
    if not isinstance(value, dict):
        raise RunnerError(f"{label} must be a JSON object: {path}", code)
    return value


def run_command(command: list[str], *, capture: bool = True, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        command,
        cwd=ROOT,
        check=False,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
        text=True,
        env=env,
    )


def require_command(command: list[str], message: str, code: int = 3) -> str:
    result = run_command(command)
    if result.returncode != 0:
        detail = (result.stderr or result.stdout).strip()
        raise RunnerError(f"{message}: {detail}", code)
    return result.stdout.strip()


def contract_path(mission: str) -> Path:
    return ROOT / "plans" / f"mission-{mission}.contract.md"


def mission_dir(mission: str) -> Path:
    return MISSIONS / mission


def parse_contract(mission: str) -> tuple[str, dict[str, str], dict[str, str]]:
    path = contract_path(mission)
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as error:
        raise RunnerError(f"mission contract is unreadable: {path}: {error}") from error
    authored_blocks = re.findall(r"^```mission[ \t]*\n(.*?)^```[ \t]*$", text, re.MULTILINE | re.DOTALL)
    seal_blocks = re.findall(r"^```mission-seal[ \t]*\n(.*?)^```[ \t]*$", text, re.MULTILINE | re.DOTALL)
    if len(authored_blocks) != 1 or len(seal_blocks) != 1:
        raise RunnerError("mission contract lacks one authored block and one generated seal")

    def values(block: str) -> dict[str, str]:
        result: dict[str, str] = {}
        for raw in block.splitlines():
            if not raw.strip():
                continue
            key, separator, value = raw.partition("=")
            if not separator or key in result:
                raise RunnerError("mission contract key/value grammar is invalid")
            result[key] = value
        return result

    return text, values(authored_blocks[0]), values(seal_blocks[0])


def process_started_at(pid: int) -> int:
    output = require_command(
        [str(ROOT / "scripts" / "agents" / "process-census.py"), "started-at", "--pid", str(pid)],
        f"cannot resolve process start identity for pid {pid}",
    )
    try:
        return int(output)
    except ValueError as error:
        raise RunnerError(f"process start identity is invalid for pid {pid}") from error


def fake_identity(pid: int) -> dict[str, Any] | None:
    path = os.environ.get("HARNESS_FAKE_PROCESS_IDENTITY_FILE")
    if not path:
        return None
    try:
        value = json.loads(Path(path).read_text(encoding="utf-8"))[str(pid)]
    except (OSError, json.JSONDecodeError, KeyError, TypeError):
        return None
    return value if isinstance(value, dict) else None


def publish_fake_identity(pid: int, started: int, pgid: int, tag: str) -> None:
    path_text = os.environ.get("HARNESS_FAKE_PROCESS_IDENTITY_FILE")
    if not path_text:
        return
    path = Path(path_text)
    path.parent.mkdir(parents=True, exist_ok=True)
    lock_path = path.with_name(path.name + ".lock")
    with lock_path.open("a+") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        try:
            value = json.loads(path.read_text(encoding="utf-8")) if path.exists() else {}
        except (OSError, json.JSONDecodeError):
            value = {}
        if not isinstance(value, dict):
            value = {}
        value[str(pid)] = {
            "pidStartedAt": started,
            "pgid": pgid,
            "command": f"fixture {tag}",
        }
        atomic_json(path, value)


def process_command(pid: int, *, allow_fake: bool = False) -> str:
    try:
        result = subprocess.run(
            ["ps", "-p", str(pid), "-o", "command="],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
        )
    except OSError:
        result = None
    if result is not None and result.returncode == 0:
        return result.stdout.strip()
    identity = fake_identity(pid) if allow_fake else None
    command = identity.get("command") if identity else None
    return command if isinstance(command, str) else ""


def pid_exists(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def group_owned(pgid: int, tag: str, *, allow_fake: bool = False) -> bool:
    try:
        result = subprocess.run(
            ["ps", "-axo", "pgid=,command="],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
        )
    except OSError:
        result = None
    if result is not None and result.returncode == 0:
        for raw in result.stdout.splitlines():
            number, separator, command = raw.strip().partition(" ")
            if separator and number.isdigit() and int(number) == pgid and tag in command.strip():
                return True
    if allow_fake:
        identity = fake_identity(pgid)
        return bool(identity and identity.get("pgid") == pgid and tag in str(identity.get("command", "")))
    return False


def group_alive(pgid: int) -> bool:
    try:
        os.killpg(pgid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def scaled_seconds(base: int) -> int:
    raw = os.environ.get("HARNESS_FIXTURE_CAP_SCALE_MILLI", "1000")
    try:
        scale = int(raw)
    except ValueError as error:
        raise RunnerError("HARNESS_FIXTURE_CAP_SCALE_MILLI must be a positive integer") from error
    if scale < 1:
        raise RunnerError("HARNESS_FIXTURE_CAP_SCALE_MILLI must be a positive integer")
    return max(1, math.ceil(base * scale / 1000))


def terminate_group(pgid: int, tag: str, *, allow_fake: bool = False) -> None:
    if not group_alive(pgid):
        return
    if not group_owned(pgid, tag, allow_fake=allow_fake):
        raise RunnerError(f"refusing to signal unowned host process group {pgid}")
    os.killpg(pgid, signal.SIGTERM)
    deadline = time.monotonic() + scaled_seconds(5)
    while group_alive(pgid) and time.monotonic() < deadline:
        time.sleep(0.05)
    if group_alive(pgid):
        if not group_owned(pgid, tag, allow_fake=allow_fake):
            raise RunnerError(f"lost ownership proof for host process group {pgid}")
        os.killpg(pgid, signal.SIGKILL)


def cleanup_stale_lease(mission: str) -> None:
    directory = mission_dir(mission)
    marker = directory / "lease.d"
    lease_path = directory / "lease.json"
    if not marker.exists() and not lease_path.exists():
        return
    lease = read_json(lease_path, "mission lease") if lease_path.exists() else {}
    pid = lease.get("pid")
    tag = lease.get("instanceTag")
    if isinstance(pid, int) and isinstance(tag, str) and pid_exists(pid) and tag in process_command(pid):
        raise RunnerError(f"mission runner is already live for {mission}")
    turns = directory / "turns"
    for path in sorted(turns.glob("*/turn.json")) if turns.exists() else []:
        try:
            turn = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        if turn.get("status") not in {"pending", "running"} and turn.get("outcome") != "running":
            continue
        pgid = turn.get("pgid")
        turn_tag = turn.get("instanceTag")
        if isinstance(pgid, int) and isinstance(turn_tag, str) and group_alive(pgid):
            terminate_group(pgid, turn_tag, allow_fake=turn.get("runtime") == "fake")
            turn.update({"status": "failed", "outcome": "failed", "error": "turn-lost", "detail": "turn-lost", "endedAt": now_iso()})
            atomic_json(path, turn)
    if marker.exists():
        children = list(marker.iterdir())
        if any(path.name != "owner.json" or not path.is_file() for path in children):
            raise RunnerError(f"stale mission lease marker contains unexpected files: {marker}")
        for path in children:
            path.unlink()
        marker.rmdir()
    lease_path.unlink(missing_ok=True)


def arm_and_preflight(mission: str) -> None:
    pid = os.getpid()
    started = process_started_at(pid)
    arm = run_command(
        [
            str(ROOT / "scripts" / "agents" / "arm-supervision.sh"),
            "--repo",
            str(ROOT),
            "--session",
            f"mission-runner-{mission}-{pid}",
            "--pid",
            str(pid),
            "--start-time",
            str(started),
            "--tag",
            "mission-runner.sh",
        ]
    )
    if arm.returncode != 0 or "ARMED" not in arm.stdout:
        detail = (arm.stderr or arm.stdout).strip()
        raise RunnerError(f"mission start refused: supervision did not arm: {detail}")
    preflight = run_command([str(ROOT / "scripts" / "assert-mission.sh"), "--preflight", "--file", str(contract_path(mission))])
    if preflight.returncode != 0:
        raise RunnerError(f"mission start refused by preflight: {(preflight.stderr or preflight.stdout).strip()}")


def verify_state(path: Path, *, anchor: bool = False) -> dict[str, Any]:
    command = [str(ROOT / "scripts" / "agents" / "mission-state.py"), "verify", "--state", str(path)]
    if anchor:
        command += ["--repo", str(ROOT), "--ledger", str(path.with_name("ledger.md"))]
    result = run_command(command)
    if result.returncode != 0:
        raise RunnerError((result.stderr or result.stdout).strip() or "mission state is unreadable", 7)
    return read_json(path, "mission state", 7)


def write_state(path: Path, proposed: dict[str, Any]) -> dict[str, Any]:
    current = read_json(path, "mission state", 7)
    expected = current.get("integrity", {}).get("hash")
    if not isinstance(expected, str):
        raise RunnerError("mission state integrity hash is unreadable", 7)
    with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", suffix=".json", delete=False) as handle:
        json.dump(proposed, handle, indent=2, sort_keys=True)
        handle.write("\n")
        source = Path(handle.name)
    try:
        result = run_command(
            [
                str(ROOT / "scripts" / "agents" / "mission-state.py"),
                "write",
                "--state",
                str(path),
                "--source",
                str(source),
                "--expect",
                expected,
            ]
        )
    finally:
        source.unlink(missing_ok=True)
    if result.returncode != 0:
        raise RunnerError((result.stderr or result.stdout).strip() or "mission state update refused")
    return read_json(path, "mission state", 7)


def anchor_state(state_path: Path, ledger: Path) -> None:
    result = run_command(
        [
            str(ROOT / "scripts" / "agents" / "mission-state.py"),
            "anchor",
            "--state",
            str(state_path),
            "--repo",
            str(ROOT),
            "--ledger",
            str(ledger),
        ]
    )
    if result.returncode != 0:
        raise RunnerError(f"mission anchor refused: {(result.stderr or result.stdout).strip()}")


def open_ask_ids(directory: Path) -> list[str]:
    result = []
    if not directory.exists():
        return result
    for path in directory.glob("*.json"):
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        if isinstance(value, dict) and value.get("answeredAt") is None and isinstance(value.get("askId"), str):
            result.append(value["askId"])
    return sorted(set(result))


def project_fences(mission: str, state: dict[str, Any]) -> None:
    directory = mission_dir(mission)
    fences_path = directory / "fences.json"
    if fences_path.exists():
        fences = read_json(fences_path, "mission fence counters")
        reservations = fences.get("reservations", {})
        if not isinstance(reservations, dict):
            raise RunnerError("mission fence reservations are unreadable")
        active = 0
        for job in reservations:
            path = AGENTS / "jobs" / f"{job}.json"
            try:
                status = json.loads(path.read_text(encoding="utf-8")).get("status")
            except (OSError, json.JSONDecodeError):
                status = None
            if status not in TERMINAL_JOBS:
                active += 1
        state["fences"].update(
            {
                "startedAt": fences.get("startedAt", state["fences"]["startedAt"]),
                "cycles": fences.get("cycles", state["fences"]["cycles"]),
                "jobs": len(reservations),
                "activeJobs": active,
            }
        )
    usage_path = directory / "usage.json"
    if usage_path.exists():
        usage = read_json(usage_path, "mission usage")
        units = usage.get("units")
        if isinstance(units, list):
            state["fences"]["usage"] = units


def initialize_state(mission: str, lease: Path) -> tuple[Path, Path, dict[str, Any]]:
    directory = mission_dir(mission)
    state_path = directory / "state.json"
    ledger = directory / "ledger.md"
    if state_path.exists() or ledger.exists():
        raise RunnerError("mission state already exists; use resume")
    _, values, _ = parse_contract(mission)
    require_command(
        [
            str(ROOT / "scripts" / "agents" / "mission-ledger.py"),
            "init",
            "--file",
            str(ledger),
            "--cycle-budget",
            values["ledger.cycle-budget"],
            "--no-gain-budget",
            values["ledger.no-gain-budget"],
        ],
        "mission ledger initialization refused",
    )
    require_command(
        [
            str(ROOT / "scripts" / "agents" / "mission-state.py"),
            "init",
            "--state",
            str(state_path),
            "--contract",
            str(contract_path(mission)),
            "--ledger",
            str(ledger),
            "--lease",
            str(lease),
        ],
        "mission state initialization refused",
    )
    anchor_state(state_path, ledger)
    return state_path, ledger, verify_state(state_path, anchor=True)


def resume_state(mission: str) -> tuple[Path, Path, dict[str, Any]]:
    directory = mission_dir(mission)
    state_path = directory / "state.json"
    ledger = directory / "ledger.md"
    if not state_path.exists():
        raise RunnerError("mission state does not exist", 7)
    result = run_command(
        [
            str(ROOT / "scripts" / "agents" / "mission-state.py"),
            "reconcile",
            "--state",
            str(state_path),
            "--repo",
            str(ROOT),
            "--ledger",
            str(ledger),
        ]
    )
    if result.returncode != 0:
        raise RunnerError(f"mission state reconciliation parked the mission: {(result.stderr or result.stdout).strip()}")
    state = verify_state(state_path, anchor=True)
    if state["status"] != "running":
        raise RunnerError(f"mission is {state['status']}; answer or amend its park reason before resume")
    return state_path, ledger, state


def runner_paths(mission: str) -> tuple[Path, Path, Path]:
    return RUNNERS / f"{mission}.json", RUNNERS / f"{mission}.heartbeat", RUNNERS / f"{mission}.log"


def runner_record(mission: str, pid: int, pgid: int, started: int, tag: str) -> dict[str, Any]:
    return {
        "missionId": mission,
        "status": "running",
        "error": None,
        "workspaceRoot": str(ROOT),
        "pid": pid,
        "pidStartedAt": started,
        "pgid": pgid,
        "instanceTag": tag,
        "startedAt": now_iso(),
        "endedAt": None,
    }


def finish_runner(mission: str, status: str, error: str | None) -> None:
    record_path, _, _ = runner_paths(mission)
    try:
        record = read_json(record_path, "mission runner record")
    except RunnerError:
        return
    record.update({"status": status, "error": error, "endedAt": now_iso()})
    atomic_json(record_path, record)


def heartbeat(mission: str, turn_id: str | None = None) -> None:
    record_path, heartbeat_path, _ = runner_paths(mission)
    record = read_json(record_path, "mission runner record")
    atomic_json(
        heartbeat_path,
        {
            "function": "mission-runner",
            "missionId": mission,
            "turnId": turn_id,
            "pid": record["pid"],
            "pidStartedAt": record["pidStartedAt"],
            "instanceTag": record["instanceTag"],
            "observedAtEpoch": int(time.time()),
        },
    )


def write_start_signal(path: Path, verified: bool, turn_id: str | None, error: str | None = None) -> None:
    atomic_json(path, {"verified": verified, "turnId": turn_id, "error": error})


def allocate_turn(mission: str, cycle: int) -> tuple[str, Path]:
    turn_id = f"{mission}-t{cycle}-{secrets.token_hex(2)}"
    directory = mission_dir(mission) / "turns" / turn_id
    try:
        directory.mkdir(parents=True)
    except FileExistsError as error:
        raise RunnerError(f"turn id collision refused: {turn_id}") from error
    return turn_id, directory


def prior_context(state: dict[str, Any]) -> tuple[str | None, bool, int]:
    if not state["turnLog"]:
        return None, False, 0
    last = state["turnLog"][-1]
    session_id = last.get("sessionId") if isinstance(last.get("sessionId"), str) else None
    outcome = last.get("outcome")
    reconciliation = outcome not in {"completed", "return-ok"}
    failures = 0
    for item in reversed(state["turnLog"]):
        if item.get("outcome") in {"completed", "return-ok"}:
            break
        if item.get("outcome") == "unresumable":
            continue
        failures += 1
    if outcome == "unresumable":
        session_id = None
    return session_id, reconciliation, failures


def patch_turn(path: Path, **fields: Any) -> dict[str, Any]:
    turn = read_json(path, "turn record")
    turn.update(fields)
    atomic_json(path, turn)
    return turn


def notify_started(start_signal: Path, turn_id: str, notified: list[bool]) -> None:
    if not notified[0]:
        write_start_signal(start_signal, True, turn_id)
        notified[0] = True


def launch_host(
    mission: str,
    turn_id: str,
    turn_dir: Path,
    turn: dict[str, Any],
    lease: Path,
    start_signal: Path,
    notified: list[bool],
) -> tuple[int, dict[str, Any] | None, str]:
    adapter = ROOT / "scripts" / "agents" / "hosts" / f"{turn['runtime']}.sh"
    if not adapter.is_file() or not os.access(adapter, os.X_OK):
        raise RunnerError(f"host adapter is not installed or executable: {adapter}")
    prompt = turn_dir / "prompt.md"
    result_path = turn_dir / "result.json"
    host_gate = turn_dir / "host.start"
    tag = f"harness-host-{turn_id}"
    command = [
        str(adapter),
        "start-turn",
        "--mission",
        mission,
        "--turn-id",
        turn_id,
        "--prompt",
        str(prompt),
        "--result",
        str(result_path),
        "--instance-tag",
        tag,
    ]
    if turn["hostSession"] is not None:
        command += ["--resume-session", turn["hostSession"]]
    environment = os.environ.copy()
    environment.update(
        {
            "HARNESS_MISSION_ID": mission,
            "HARNESS_MISSION_LEASE": str(lease),
            "HARNESS_MISSION_TURN": turn_id,
            "HARNESS_HOST_START_GATE": str(host_gate),
            "HARNESS_HOST_START_GATE_TIMEOUT_SEC": str(scaled_seconds(10)),
        }
    )
    host_log = (turn_dir / "host.log").open("a", encoding="utf-8")
    process = subprocess.Popen(
        command,
        cwd=ROOT,
        env=environment,
        stdout=host_log,
        stderr=subprocess.STDOUT,
        start_new_session=True,
        text=True,
    )
    grace = scaled_seconds(5)
    deadline = time.monotonic() + grace
    started = None
    verified = False
    fake_runtime = turn["runtime"] == "fake"
    force_unverified = fake_runtime and os.environ.get("HARNESS_FAKE_HOST_START_UNVERIFIED") == "1"
    while time.monotonic() <= deadline:
        if process.poll() is not None:
            break
        try:
            started = process_started_at(process.pid)
            if fake_runtime:
                publish_fake_identity(process.pid, started, process.pid, tag)
            command_line = process_command(process.pid, allow_fake=fake_runtime)
            verified = (
                not force_unverified
                and os.getpgid(process.pid) == process.pid
                and tag in command_line
            )
        except (OSError, RunnerError):
            verified = False
        if verified:
            break
        heartbeat(mission, turn_id)
        time.sleep(0.02)
    turn_path = turn_dir / "turn.json"
    if not verified or started is None:
        if process.poll() is None:
            terminate_group(process.pid, tag, allow_fake=fake_runtime)
        process.wait(timeout=scaled_seconds(5))
        host_log.close()
        patch_turn(
            turn_path,
            status="failed",
            outcome="failed",
            error="start-unverified",
            detail="start-unverified",
            endedAt=now_iso(),
        )
        return 3, None, "start-unverified"
    patch_turn(
        turn_path,
        pid=process.pid,
        pidStartedAt=started,
        pgid=process.pid,
        instanceTag=tag,
        status="running",
        outcome="running",
    )
    atomic_text(host_gate, "started\n")
    notify_started(start_signal, turn_id, notified)

    cap_seconds = int(turn["turnCapMin"]) * 60
    capped = False
    deadline = time.monotonic() + cap_seconds
    while process.poll() is None:
        heartbeat(mission, turn_id)
        if time.monotonic() >= deadline:
            terminate_group(process.pid, tag, allow_fake=fake_runtime)
            capped = True
            break
        time.sleep(0.1)
    try:
        exit_code = process.wait(timeout=scaled_seconds(5))
    except subprocess.TimeoutExpired:
        terminate_group(process.pid, tag, allow_fake=fake_runtime)
        exit_code = process.wait(timeout=scaled_seconds(5))
    host_log.close()
    if capped:
        patch_turn(
            turn_path,
            status="failed",
            outcome="capped",
            error="turn-cap",
            detail="host turn reached host.turn-cap-min",
            endedAt=now_iso(),
        )
        return 3, None, "capped"
    result = None
    if result_path.exists():
        try:
            result = read_json(result_path, "host result")
        except RunnerError:
            result = None
    return exit_code, result, "host exited without a usable result" if result is None else "host result received"


def contained_path(turn_dir: Path, raw: Any, label: str, *, required: bool = True) -> Path | None:
    if raw is None and not required:
        return None
    if not isinstance(raw, str) or not raw:
        raise RunnerError(f"host result {label} is missing")
    path = Path(raw).resolve()
    try:
        path.relative_to(turn_dir.resolve())
    except ValueError as error:
        raise RunnerError(f"host result {label} escapes the turn directory") from error
    return path


def validate_return(turn: dict[str, Any], result: dict[str, Any], turn_dir: Path) -> tuple[dict[str, Any], Path, Path]:
    expected_result = {"sessionId", "outcome", "usage", "rawPath", "returnPath"}
    if set(result) != expected_result:
        raise RunnerError("host result has missing or unexpected fields")
    if result["outcome"] != "completed":
        raise RunnerError(f"host result outcome is not completed: {result['outcome']}")
    raw_path = contained_path(turn_dir, result["rawPath"], "rawPath")
    return_path = contained_path(turn_dir, result["returnPath"], "returnPath")
    assert raw_path is not None and return_path is not None
    checker = run_command(
        [
            str(ROOT / "scripts" / "assert-return-complete.sh"),
            "--role",
            "orchestrator",
            "--file",
            str(return_path),
        ]
    )
    if checker.returncode != 0:
        raise RunnerError(f"orchestrator return is invalid: {(checker.stderr or checker.stdout).strip()}")
    returned = read_json(return_path, "orchestrator return")
    identity = returned.get("identity")
    expected_identity = {
        "turnId": turn["turnId"],
        "missionId": turn["missionId"],
        "cycle": turn["cycle"],
    }
    for field, expected in expected_identity.items():
        if returned.get(field) != expected:
            raise RunnerError(f"orchestrator return identity mismatch at {field}")
    if not isinstance(identity, dict):
        raise RunnerError("orchestrator return identity is missing")
    if identity.get("runtime") != turn["runtime"] or identity.get("model") != turn["model"]:
        raise RunnerError("orchestrator return runtime/model identity mismatch")
    if identity.get("sessionId") != result.get("sessionId"):
        raise RunnerError("orchestrator return session identity mismatch")
    return returned, raw_path, return_path


def next_ask_id(directory: Path, prefix: str) -> str:
    base = prefix
    candidate = base
    index = 1
    while (directory / f"{candidate}.json").exists():
        index += 1
        candidate = f"{base}-{index}"
    return candidate


def write_ask(directory: Path, ask_id: str, stream_id: str | None, reason: str, question: str) -> None:
    atomic_json(
        directory / f"{ask_id}.json",
        {
            "askId": ask_id,
            "streamId": stream_id,
            "reasonClass": reason,
            "question": question.replace("\r", " ").replace("\n", " "),
            "createdAt": now_iso(),
            "answeredAt": None,
            "answer": None,
        },
    )


def apply_orchestrator_return(
    mission: str,
    turn: dict[str, Any],
    state: dict[str, Any],
    returned: dict[str, Any],
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    accepted: list[dict[str, Any]] = []
    rejected: list[dict[str, Any]] = []
    asks_dir = mission_dir(mission) / "asks"
    asks_dir.mkdir(parents=True, exist_ok=True)

    for entry in returned["dispatched"]:
        record_path = AGENTS / "jobs" / f"{entry['jobId']}.json"
        reason = None
        try:
            record = json.loads(record_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            reason = "job record does not exist or is unreadable"
        else:
            if record.get("mission") != mission:
                reason = "job record is not stamped for this mission"
            elif record.get("turnId") != turn["turnId"]:
                reason = "job record was not created during this host turn"
        if reason is None:
            accepted.append({"kind": "dispatched", "value": entry})
        else:
            rejected.append({"kind": "dispatched", "value": entry, "reason": reason})

    for entry in returned["streamUpdatesRequested"]:
        stream_id = entry["streamId"]
        stream = state["streams"].get(stream_id)
        reason = None
        if stream is None:
            reason = "stream does not exist"
        elif entry["requestedState"] not in LEGAL_STREAM_TRANSITIONS.get(stream["state"], set()):
            reason = f"illegal stream transition {stream['state']} to {entry['requestedState']}"
        elif entry["requestedState"].startswith("parked-") and not entry["reason"]:
            reason = "parked stream request has no reason"
        if reason is None:
            stream["state"] = entry["requestedState"]
            stream["reason"] = entry["reason"] or None
            accepted.append({"kind": "streamUpdate", "value": entry})
        else:
            rejected.append({"kind": "streamUpdate", "value": entry, "reason": reason})

    for index, entry in enumerate(returned["askCandidates"], 1):
        reason = None
        if entry["streamId"] not in state["streams"]:
            reason = "stream does not exist"
        elif entry["reasonClass"] not in KNOWN_ASK_REASONS:
            reason = "reason class is unknown"
        if reason is None:
            ask_id = next_ask_id(asks_dir, f"ask-{turn['cycle']}-{index}")
            write_ask(asks_dir, ask_id, entry["streamId"], entry["reasonClass"], entry["question"])
            accepted.append({"kind": "askCandidate", "value": entry, "askId": ask_id})
        else:
            rejected.append({"kind": "askCandidate", "value": entry, "reason": reason})

    fallback_stream = next(
        (stream_id for stream_id, value in sorted(state["streams"].items()) if value["state"] == "active"),
        next(iter(sorted(state["streams"]))),
    )
    for index, item in enumerate(rejected, 1):
        value = item.get("value", {})
        stream_id = value.get("stream") or value.get("streamId")
        if stream_id not in state["streams"]:
            stream_id = fallback_stream
        ask_id = next_ask_id(asks_dir, f"rejected-{turn['cycle']}-{index}")
        write_ask(
            asks_dir,
            ask_id,
            stream_id,
            "host-failure",
            f"Runner rejected host return {item['kind']}: {item['reason']}. Review the return before proceeding.",
        )
        item["askId"] = ask_id
    state["waitingList"] = open_ask_ids(asks_dir)
    return accepted, rejected


def mission_jobs(mission: str) -> list[tuple[Path, dict[str, Any]]]:
    records = []
    jobs = AGENTS / "jobs"
    for path in jobs.glob("*.json") if jobs.exists() else []:
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        if value.get("mission") == mission:
            records.append((path, value))
    return records


def drain_jobs(mission: str) -> tuple[bool, bool]:
    while True:
        records = mission_jobs(mission)
        active = [(path, value) for path, value in records if value.get("status") not in TERMINAL_JOBS]
        if not active:
            chains_closed = all(value.get("chainClosed") is True for _, value in records)
            return True, chains_closed
        for path, value in active:
            run_command(
                [str(ROOT / "scripts" / "agents" / "dispatch.sh"), "reap", "--job", value.get("jobId", path.stem)]
            )
        time.sleep(0.1)


def load_contract_module():
    path = ROOT / "scripts" / "agents" / "mission-contract.py"
    specification = importlib.util.spec_from_file_location("harness_mission_contract", path)
    if specification is None or specification.loader is None:
        raise RunnerError("cannot load mission contract measurement library")
    module = importlib.util.module_from_spec(specification)
    sys.modules[specification.name] = module
    specification.loader.exec_module(module)
    return module


def run_guard(module: Any, contract: Any, repo: Path, project_root: Path, name: str, command: str) -> Decimal:
    values = contract.values
    gate_ref = str(module.git(repo, "rev-parse", f"{values['gate.ref']}^{{commit}}")).strip()
    branch = contract.sealed["candidate.branch"]
    candidate_sha = str(module.git(repo, "rev-parse", f"{branch}^{{commit}}")).strip()
    gate_paths = module.expand_paths(repo, project_root, gate_ref, module.validate_globs(values["gate.paths"], "gate.paths"), "gate.paths")
    truth_paths = module.expand_paths(repo, project_root, gate_ref, module.validate_globs(values["truth.paths"], "truth.paths"), "truth.paths")
    scratch = Path(tempfile.mkdtemp(prefix="mission-guard."))
    worktree = scratch / "candidate"
    try:
        module.git(repo, "worktree", "add", "--detach", "--quiet", str(worktree), candidate_sha)
        module.git(worktree, "checkout", "--quiet", gate_ref, "--", *sorted(set(gate_paths + truth_paths)))
        prefix = project_root.resolve().relative_to(repo.resolve())
        result = subprocess.run(
            ["bash", "-lc", command],
            cwd=worktree / prefix,
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=int(values["fence.job-cap-min"]) * 60,
        )
        if result.returncode != 0:
            raise RunnerError(f"guard {name} measurement failed with exit {result.returncode}")
        metrics: dict[str, Decimal] = {}
        for line in result.stdout.splitlines():
            match = module.METRIC_RE.fullmatch(line.strip())
            if match:
                metrics[match.group(1)] = Decimal(match.group(2))
        if name not in metrics:
            raise RunnerError(f"guard {name} output omitted metric={name}=<value>")
        return metrics[name]
    finally:
        if worktree.exists():
            module.git(repo, "worktree", "remove", "--force", str(worktree), check=False)
        shutil.rmtree(scratch, ignore_errors=True)


def previous_metrics(state: dict[str, Any], seal: dict[str, str], names: list[str]) -> dict[str, Decimal]:
    for item in reversed(state["turnLog"]):
        measurement = item.get("measurement")
        if isinstance(measurement, dict) and isinstance(measurement.get("metrics"), dict):
            try:
                return {name: Decimal(str(measurement["metrics"][name])) for name in names}
            except (KeyError, ArithmeticError):
                continue
    return {name: Decimal(seal[f"sealed.baseline.{name}"]) for name in names}


def measure(mission: str, state: dict[str, Any]) -> tuple[str, str, dict[str, Any], bool]:
    module = load_contract_module()
    path = contract_path(mission).resolve()
    contract = module.read_contract(path)
    repo = module.repository_for(path)
    project_root = module.project_root_for(path, repo)
    module.validate_contract(contract, project_root)
    candidate_sha, raw_metrics, failures = module.run_gate(contract, repo, project_root)
    names = sorted(raw_metrics)
    current = {name: Decimal(raw_metrics[name]) for name in names}
    previous = previous_metrics(state, contract.sealed, names)
    direction = contract.values["gate.direction"]
    improved = False
    regressed = False
    within = True
    for name in names:
        noise = Decimal(contract.values[f"gate.noise-floor.{name}"])
        delta = current[name] - previous[name]
        directed = delta if direction == "max" else -delta
        if directed > noise:
            improved = True
            within = False
        elif directed < -noise:
            regressed = True
            within = False
    if improved and not regressed:
        classification = "contract-improved"
    elif within:
        classification = "unresolved"
    else:
        classification = "no-progress"
    guards_passed = True
    guards: dict[str, str] = {}
    for key in sorted(contract.values):
        match = re.fullmatch(r"guard\.([a-z0-9][a-z0-9-]*)\.command", key)
        if not match:
            continue
        name = match.group(1)
        guard_value = run_guard(module, contract, repo, project_root, name, contract.values[key])
        guards[name] = str(guard_value)
        if guard_value < Decimal(contract.values[f"guard.{name}.floor"]):
            guards_passed = False
    observed = ",".join(f"{name}={raw_metrics[name]}" for name in names)
    measurement = {"metrics": raw_metrics, "guards": guards, "candidateSha": candidate_sha}
    return classification, observed, measurement, failures == 0 and guards_passed


def append_ledger(ledger: Path, cycle: int, classification: str, candidate_sha: str, observed: str) -> None:
    result = run_command(
        [
            str(ROOT / "scripts" / "agents" / "mission-ledger.py"),
            "append",
            "--file",
            str(ledger),
            "--cycle",
            str(cycle),
            "--classification",
            classification,
            "--candidate-sha",
            candidate_sha,
            "--observed",
            observed,
        ]
    )
    if result.returncode != 0:
        raise RunnerError(f"mission ledger append refused: {(result.stderr or result.stdout).strip()}")


def park_state(state_path: Path, ledger: Path, state: dict[str, Any], reason: str, mission: str) -> dict[str, Any]:
    proposed = copy.deepcopy(state)
    asks_dir = mission_dir(mission) / "asks"
    if reason in {"host-failure", "stop-loss"}:
        has_reason = False
        for path in asks_dir.glob("*.json") if asks_dir.exists() else []:
            try:
                ask = json.loads(path.read_text(encoding="utf-8"))
            except (OSError, json.JSONDecodeError):
                continue
            if ask.get("answeredAt") is None and ask.get("reasonClass") == reason:
                has_reason = True
                break
        if not has_reason:
            asks_dir.mkdir(parents=True, exist_ok=True)
            stream_id = next(
                (name for name, value in sorted(proposed["streams"].items()) if value["state"] == "active"),
                next(iter(sorted(proposed["streams"]))),
            )
            ask_id = next_ask_id(asks_dir, reason)
            question = (
                "Acknowledge the host failure before resuming the mission."
                if reason == "host-failure"
                else "Amend, price, reseal, and sign the mission budget before requesting stop-loss unpark."
            )
            write_ask(asks_dir, ask_id, stream_id, reason, question)
    proposed["status"] = "parked"
    proposed["parkReason"] = reason
    proposed["gatePassed"] = False
    proposed["waitingList"] = open_ask_ids(mission_dir(mission) / "asks")
    project_fences(mission, proposed)
    updated = write_state(state_path, proposed)
    anchor_state(state_path, ledger)
    return updated


def record_failed_turn(
    state_path: Path,
    ledger: Path,
    state: dict[str, Any],
    turn: dict[str, Any],
    detail: str,
    outcome: str,
    consecutive_failures: int,
    mission: str,
) -> dict[str, Any]:
    candidate_sha = require_command(["git", "-C", str(ROOT), "rev-parse", state["branch"]], "cannot resolve candidate sha")
    append_ledger(ledger, turn["cycle"], "no-progress", candidate_sha, f"unmeasurable:{detail}".replace("\n", " "))
    proposed = copy.deepcopy(state)
    proposed["turnLog"].append(
        {
            "turnId": turn["turnId"],
            "cycle": turn["cycle"],
            "outcome": outcome,
            "detail": detail,
            "sessionId": None,
            "measurement": None,
        }
    )
    proposed["ledger"]["cycles"] = turn["cycle"]
    project_fences(mission, proposed)
    if consecutive_failures >= 2:
        proposed.update({"status": "parked", "parkReason": "host-failure", "gatePassed": False})
    updated = write_state(state_path, proposed)
    anchor_state(state_path, ledger)
    if updated["status"] == "parked" and updated.get("parkReason") == "host-failure":
        return park_state(state_path, ledger, updated, "host-failure", mission)
    if updated["status"] == "running":
        stop = run_command([str(ROOT / "scripts" / "assert-stop-loss.sh"), "--file", str(ledger)])
        if stop.returncode == 1:
            updated = park_state(state_path, ledger, updated, "stop-loss", mission)
    return updated


def one_cycle(
    mission: str,
    state_path: Path,
    ledger: Path,
    state: dict[str, Any],
    lease: Path,
    start_signal: Path,
    notified: list[bool],
) -> dict[str, Any]:
    reserve = run_command(
        [str(ROOT / "scripts" / "agents" / "mission-fence.py"), "reserve-cycle", "--repo", str(ROOT), "--mission", mission]
    )
    if reserve.returncode != 0:
        return park_state(state_path, ledger, state, "fence", mission)
    fences = read_json(mission_dir(mission) / "fences.json", "mission fence counters")
    cycle = fences.get("cycles")
    if not isinstance(cycle, int) or isinstance(cycle, bool) or cycle < 1:
        raise RunnerError("reserved mission cycle number is invalid")
    host_session, reconciliation, prior_failures = prior_context(state)
    _, values, _ = parse_contract(mission)
    turn_id, turn_dir = allocate_turn(mission, cycle)
    turn_path = turn_dir / "turn.json"
    turn = {
        "missionId": mission,
        "turnId": turn_id,
        "cycle": cycle,
        "runtime": values["host.runtime"],
        "model": values["host.model"],
        "hostSession": host_session,
        "reconciliation": reconciliation,
        "startedAt": now_iso(),
        "turnCapMin": int(values["host.turn-cap-min"]),
        "pid": None,
        "pidStartedAt": None,
        "pgid": None,
        "instanceTag": None,
        "status": "pending",
        "outcome": None,
        "error": None,
        "detail": None,
        "resultPath": str(turn_dir / "result.json"),
        "returnPath": str(turn_dir / "return.json"),
        "rawPath": str(turn_dir / "raw.out"),
        "endedAt": None,
    }
    atomic_json(turn_path, turn)
    prompt = run_command(
        [
            str(ROOT / "scripts" / "agents" / "mission-prompt.py"),
            "--mission",
            mission,
            "--turn",
            turn_id,
            "--output",
            str(turn_dir / "prompt.md"),
        ]
    )
    if prompt.returncode != 0:
        detail = (prompt.stderr or prompt.stdout).strip() or "prompt assembly refused"
        patch_turn(turn_path, status="failed", outcome="failed", error="prompt-refused", detail=detail, endedAt=now_iso())
        return record_failed_turn(state_path, ledger, state, turn, detail, "failed", 2, mission)
    checker = run_command(
        [str(ROOT / "scripts" / "assert-turn-prompt.sh"), "--file", str(turn_dir / "prompt.md"), "--turn", str(turn_dir)]
    )
    if checker.returncode != 0:
        detail = (checker.stderr or checker.stdout).strip() or "turn prompt checker refused launch"
        patch_turn(turn_path, status="failed", outcome="failed", error="prompt-refused", detail=detail, endedAt=now_iso())
        return record_failed_turn(state_path, ledger, state, turn, detail, "failed", 2, mission)

    exit_code, result, launch_detail = launch_host(mission, turn_id, turn_dir, turn, lease, start_signal, notified)
    turn = read_json(turn_path, "turn record")
    if launch_detail == "start-unverified":
        return record_failed_turn(state_path, ledger, state, turn, launch_detail, "failed", 2, mission)
    if exit_code == 6:
        patch_turn(turn_path, status="failed", outcome="unresumable", error="unresumable", detail="host session is not resumable", endedAt=now_iso())
        return record_failed_turn(state_path, ledger, state, turn, "host session is not resumable", "unresumable", prior_failures, mission)
    if exit_code != 0 or result is None:
        detail = launch_detail if result is None else f"host exited non-zero ({exit_code})"
        patch_turn(turn_path, status="failed", outcome="failed", error="host-failure", detail=detail, endedAt=now_iso())
        return record_failed_turn(state_path, ledger, state, turn, detail, "failed", prior_failures + 1, mission)
    try:
        returned, raw_path, return_path = validate_return(turn, result, turn_dir)
    except RunnerError as error:
        detail = str(error)
        patch_turn(turn_path, status="failed", outcome="failed", error="protocol-error", detail=detail, endedAt=now_iso(), result=result)
        return record_failed_turn(state_path, ledger, state, turn, detail, "failed", prior_failures + 1, mission)

    proposed = copy.deepcopy(state)
    accepted, rejected = apply_orchestrator_return(mission, turn, proposed, returned)
    drained, chains_closed = drain_jobs(mission)
    try:
        classification, observed, measurement, gate_passed = measure(mission, state) if drained else (
            "no-progress",
            "unmeasurable:mission jobs did not drain",
            None,
            False,
        )
    except Exception as error:
        classification, observed, measurement, gate_passed = (
            "no-progress",
            f"unmeasurable:{str(error).replace(chr(10), ' ')}",
            None,
            False,
        )
    candidate_sha = (
        measurement["candidateSha"]
        if isinstance(measurement, dict)
        else require_command(["git", "-C", str(ROOT), "rev-parse", state["branch"]], "cannot resolve candidate sha")
    )
    append_ledger(ledger, cycle, classification, candidate_sha, observed)
    proposed["turnLog"].append(
        {
            "turnId": turn_id,
            "cycle": cycle,
            "outcome": "completed",
            "detail": "host return accepted",
            "sessionId": result.get("sessionId"),
            "measurement": measurement,
            "accepted": accepted,
            "rejected": rejected,
            "certified": returned["certified"],
            "factsForLedger": returned["factsForLedger"],
            "gaps": returned["gaps"],
        }
    )
    proposed["ledger"]["cycles"] = cycle
    proposed["waitingList"] = open_ask_ids(mission_dir(mission) / "asks")
    project_fences(mission, proposed)
    if gate_passed and chains_closed:
        proposed.update({"status": "completed", "parkReason": None, "gatePassed": True})
    elif not any(value["state"] == "active" for value in proposed["streams"].values()):
        proposed.update({"status": "parked", "parkReason": "all-streams-parked", "gatePassed": False})
    else:
        proposed.update({"status": "running", "parkReason": None, "gatePassed": False})
    updated = write_state(state_path, proposed)
    patch_turn(
        turn_path,
        status="completed",
        outcome="completed",
        error=None,
        detail="host return accepted",
        result=result,
        endedAt=now_iso(),
        rawPath=str(raw_path),
        returnPath=str(return_path),
    )
    anchor_state(state_path, ledger)
    if updated["status"] == "running":
        stop = run_command([str(ROOT / "scripts" / "assert-stop-loss.sh"), "--file", str(ledger)])
        if stop.returncode == 1:
            updated = park_state(state_path, ledger, updated, "stop-loss", mission)
    return updated


def acquire_lease(mission: str, tag: str) -> Path:
    directory = mission_dir(mission)
    marker = directory / "lease.d"
    lease = directory / "lease.json"
    directory.mkdir(parents=True, exist_ok=True)
    try:
        marker.mkdir()
    except FileExistsError as error:
        raise RunnerError("mission lease is busy") from error
    pid = os.getpid()
    started = process_started_at(pid)
    pgid = os.getpgid(pid)
    value = {
        "missionId": mission,
        "pid": pid,
        "pgid": pgid,
        "instanceTag": tag,
        "startedAt": now_iso(),
        "renewedAt": now_iso(),
    }
    atomic_json(marker / "owner.json", value)
    atomic_json(lease, value)
    return lease


def release_lease(mission: str) -> None:
    directory = mission_dir(mission)
    marker = directory / "lease.d"
    lease = directory / "lease.json"
    if marker.exists():
        owner = marker / "owner.json"
        owner.unlink(missing_ok=True)
        try:
            marker.rmdir()
        except OSError:
            pass
    lease.unlink(missing_ok=True)


def internal_run(mission: str, mode: str, tag: str, start_signal: Path) -> int:
    record_path, _, _ = runner_paths(mission)
    pid = os.getpid()
    pgid = os.getpgid(pid)
    started = process_started_at(pid)
    atomic_json(record_path, runner_record(mission, pid, pgid, started, tag))
    heartbeat(mission)
    lease = None
    notified = [False]
    try:
        lease = acquire_lease(mission, tag)
        if mode == "start":
            state_path, ledger, state = initialize_state(mission, lease)
        else:
            state_path, ledger, state = resume_state(mission)
        while state["status"] == "running":
            heartbeat(mission)
            state = one_cycle(mission, state_path, ledger, state, lease, start_signal, notified)
        if not notified[0]:
            write_start_signal(start_signal, False, None, f"mission parked before a host turn started: {state.get('parkReason')}")
        finish_runner(mission, "completed", None)
        return 0
    except RunnerError as error:
        if not notified[0]:
            write_start_signal(start_signal, False, None, str(error))
        finish_runner(mission, "failed", str(error))
        return error.code
    except Exception as error:
        if not notified[0]:
            write_start_signal(start_signal, False, None, str(error))
        finish_runner(mission, "failed", str(error))
        return 3
    finally:
        if lease is not None:
            release_lease(mission)


def parse_simple(command: str, arguments: list[str]) -> tuple[str, bool]:
    mission = ""
    foreground = False
    index = 0
    while index < len(arguments):
        item = arguments[index]
        if item == "--mission" and index + 1 < len(arguments):
            mission = arguments[index + 1]
            index += 2
        elif item == "--foreground" and command in {"start", "resume"}:
            foreground = True
            index += 1
        else:
            raise RunnerError("usage", 2)
    if not ID_RE.fullmatch(mission):
        raise RunnerError("usage", 2)
    return mission, foreground


def launch_runner(command: str, mission: str, foreground: bool) -> int:
    directory = mission_dir(mission)
    state_path = directory / "state.json"
    if command == "start" and state_path.exists():
        raise RunnerError("mission state already exists; use resume")
    if command == "resume":
        if not state_path.exists():
            raise RunnerError("mission state does not exist", 7)
        state = verify_state(state_path)
        if state["status"] != "running":
            raise RunnerError(f"mission is {state['status']}; answer its park reason before resume")
    cleanup_stale_lease(mission)
    arm_and_preflight(mission)
    tag = f"harness-mission-runner-{mission}-{secrets.token_hex(3)}"
    signal_path = directory / f"runner-start-{secrets.token_hex(4)}.json"
    _, _, log_path = runner_paths(mission)
    log_path.parent.mkdir(parents=True, exist_ok=True)
    log = log_path.open("a", encoding="utf-8")
    process = subprocess.Popen(
        [
            str(SCRIPT),
            "__run",
            "--mission",
            mission,
            "--mode",
            command,
            "--instance-tag",
            tag,
            "--start-signal",
            str(signal_path),
        ],
        cwd=ROOT,
        stdout=None if foreground else log,
        stderr=None if foreground else subprocess.STDOUT,
        start_new_session=not foreground,
        text=True,
    )
    deadline = time.monotonic() + scaled_seconds(15)
    while time.monotonic() <= deadline:
        if signal_path.exists():
            signal_value = read_json(signal_path, "runner start signal")
            signal_path.unlink(missing_ok=True)
            if signal_value.get("verified") is True:
                if foreground:
                    process.wait()
                log.close()
                print(f"mission={mission} started=yes turn={signal_value.get('turnId')}")
                return 0
            process.wait(timeout=scaled_seconds(5))
            log.close()
            raise RunnerError(f"mission start refused: {signal_value.get('error')}")
        if process.poll() is not None:
            log.close()
            record_path, _, _ = runner_paths(mission)
            error = "runner exited before verified host start"
            if record_path.exists():
                try:
                    error = read_json(record_path, "mission runner record").get("error") or error
                except RunnerError:
                    pass
            raise RunnerError(error)
        time.sleep(0.05)
    if process.poll() is None:
        command_line = process_command(process.pid)
        if tag in command_line:
            terminate_group(os.getpgid(process.pid), tag)
    log.close()
    raise RunnerError("mission runner start verification timed out")


def status_command(mission: str) -> int:
    path = mission_dir(mission) / "state.json"
    if not path.exists():
        print(f"mission={mission} status=unreadable reason=missing-state")
        return 7
    try:
        state = verify_state(path)
    except RunnerError as error:
        print(f"mission={mission} status=unreadable reason={str(error).replace(' ', '-')}")
        return 7
    reason = state.get("parkReason") or "none"
    print(f"mission={mission} status={state['status']} reason={reason}")
    return {"running": 0, "completed": 10, "parked": 11}.get(state["status"], 7)


def fence_reached(mission: str, values: dict[str, str]) -> bool:
    path = mission_dir(mission) / "fences.json"
    if not path.exists():
        return False
    fences = read_json(path, "mission fence counters")
    started = datetime.fromisoformat(fences["startedAt"].replace("Z", "+00:00"))
    elapsed = Decimal(str((datetime.now(timezone.utc) - started).total_seconds())) / Decimal(3600)
    reservations = fences.get("reservations", {})
    active = 0
    if isinstance(reservations, dict):
        records = {path.stem: value for path, value in mission_jobs(mission)}
        active = sum(
            1
            for job_id in reservations
            if records.get(job_id, {}).get("status") not in TERMINAL_JOBS
        )
    return any(
        (
            elapsed >= Decimal(values["fence.wall-clock-hours"]),
            int(fences["cycles"]) >= int(values["fence.cycles"]),
            len(reservations) >= int(values["fence.jobs"]),
            active >= int(values["fence.concurrency"]),
        )
    )


def answer_command(arguments: list[str]) -> int:
    mission = ask_id = answer = ""
    index = 0
    while index < len(arguments):
        item = arguments[index]
        if item in {"--mission", "--ask", "--answer"} and index + 1 < len(arguments):
            value = arguments[index + 1]
            if item == "--mission":
                mission = value
            elif item == "--ask":
                ask_id = value
            else:
                answer = value
            index += 2
        else:
            return 2
    if not ID_RE.fullmatch(mission) or not ID_RE.fullmatch(ask_id) or not answer or "\x00" in answer:
        return 2
    state_path = mission_dir(mission) / "state.json"
    try:
        state = verify_state(state_path)
    except RunnerError as error:
        print(str(error), file=sys.stderr)
        return 7
    ask_path = mission_dir(mission) / "asks" / f"{ask_id}.json"
    if not ask_path.exists():
        print(f"answer refused: unknown ask {ask_id}", file=sys.stderr)
        return 3
    ask = read_json(ask_path, "mission ask")
    if ask.get("askId") != ask_id or ask.get("answeredAt") is not None:
        print(f"answer refused: unknown or already answered ask {ask_id}", file=sys.stderr)
        return 3
    reason = ask.get("reasonClass")
    if reason == "stop-loss" or state.get("parkReason") == "stop-loss":
        print("answer refused: stop-loss requires a contract amendment before this minimal runner can unpark", file=sys.stderr)
        return 3
    if reason not in KNOWN_ASK_REASONS | {"fence"}:
        print(f"answer refused: unsupported reason class {reason}", file=sys.stderr)
        return 3
    proposed = copy.deepcopy(state)
    stream_id = ask.get("streamId")
    if reason in {"reserved-decision", "red-test", "merge-conflict"}:
        stream = proposed["streams"].get(stream_id)
        if stream is None or stream["state"] != "parked-reserved":
            print("answer refused: reserved ask does not name a parked-reserved stream", file=sys.stderr)
            return 3
        stream.update({"state": "active", "reason": None, "answeredAsk": ask_id})
        proposed.update({"status": "running", "parkReason": None, "gatePassed": False})
    elif reason == "host-failure" and proposed["status"] == "parked":
        if proposed.get("parkReason") != "host-failure":
            print("answer refused: host-failure answer does not match the mission park reason", file=sys.stderr)
            return 3
        if not any(value["state"] == "active" for value in proposed["streams"].values()):
            print("answer refused: host-failure mission has no active stream", file=sys.stderr)
            return 3
        proposed.update({"status": "running", "parkReason": None, "gatePassed": False})
    elif reason == "fence":
        preflight = run_command(
            [str(ROOT / "scripts" / "assert-mission.sh"), "--preflight", "--file", str(contract_path(mission))]
        )
        if preflight.returncode != 0:
            print(f"answer refused: fence contract amendment is not preflight-ready: {(preflight.stderr or preflight.stdout).strip()}", file=sys.stderr)
            return 3
        _, values, _ = parse_contract(mission)
        if not fence_reached(mission, values):
            proposed.update({"status": "running", "parkReason": None, "gatePassed": False})
    proposed["waitingList"] = [value for value in proposed["waitingList"] if value != ask_id]
    original_ask = copy.deepcopy(ask)
    ask.update({"answeredAt": now_iso(), "answer": answer})
    atomic_json(ask_path, ask)
    try:
        updated = write_state(state_path, proposed)
        anchor_state(state_path, mission_dir(mission) / "ledger.md")
    except RunnerError as error:
        atomic_json(ask_path, original_ask)
        print(f"answer refused: {error}", file=sys.stderr)
        return 3
    print(f"mission={mission} ask={ask_id} applied=yes status={updated['status']}")
    return 0


def main() -> int:
    if not ARGV or ARGV[0] in {"-h", "--help"}:
        usage()
        return 0 if ARGV else 2
    command = ARGV[0]
    try:
        if command in {"start", "resume"}:
            mission, foreground = parse_simple(command, ARGV[1:])
            return launch_runner(command, mission, foreground)
        if command == "status":
            mission, foreground = parse_simple(command, ARGV[1:])
            if foreground:
                raise RunnerError("usage", 2)
            return status_command(mission)
        if command == "answer":
            result = answer_command(ARGV[1:])
            if result == 2:
                usage()
            return result
        if command == "__run":
            values: dict[str, str] = {}
            args = ARGV[1:]
            while args:
                if len(args) < 2 or not args[0].startswith("--"):
                    return 2
                values[args[0][2:]] = args[1]
                args = args[2:]
            required = {"mission", "mode", "instance-tag", "start-signal"}
            if set(values) != required or not ID_RE.fullmatch(values["mission"]) or values["mode"] not in {"start", "resume"}:
                return 2
            return internal_run(
                values["mission"], values["mode"], values["instance-tag"], Path(values["start-signal"])
            )
        usage()
        return 2
    except RunnerError as error:
        if error.code == 2:
            usage()
        else:
            print(str(error), file=sys.stderr)
        return error.code


raise SystemExit(main())
PY
