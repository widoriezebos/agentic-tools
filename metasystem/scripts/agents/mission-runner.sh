#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
exec python3 - "$root" "$script" "$ms" "$@" <<'PY'
from __future__ import annotations

import copy
import fcntl
import hashlib
import json
import math
import os
import re
import secrets
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
MS = sys.argv[3]
ARGV = sys.argv[4:]
AGENTS = ROOT / "artifacts" / "agents"
MISSIONS = AGENTS / "missions"
RUNNERS = MISSIONS / "runners"
ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
TERMINAL_JOBS = {"completed", "failed", "timeout", "cancelled"}
KNOWN_ASK_REASONS = {"reserved-decision", "red-test", "merge-conflict", "host-failure"}


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


def atomic_bytes(path: Path, value: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=path.name + ".", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(descriptor, "wb") as handle:
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


def ms_json(arguments: list[str], label: str) -> dict[str, Any]:
    """Run a metasystem verb that prints a JSON object and parse it."""
    result = run_command([MS] + arguments)
    if result.returncode != 0:
        raise RunnerError((result.stderr or result.stdout).strip() or f"{label} refused")
    try:
        value = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        raise RunnerError(f"{label} produced unreadable JSON: {error}") from error
    if not isinstance(value, dict):
        raise RunnerError(f"{label} produced unreadable JSON")
    return value


def contract_path(mission: str) -> Path:
    return ROOT / "plans" / f"mission-{mission}.contract.md"


def mission_dir(mission: str) -> Path:
    return MISSIONS / mission


def approved_contract_path(mission: str) -> Path:
    return mission_dir(mission) / f"mission-{mission}.contract.md"


def parse_contract(mission: str, *, approved: bool = True) -> tuple[str, dict[str, str], dict[str, str]]:
    path = approved_contract_path(mission) if approved else contract_path(mission)
    try:
        raw_bytes = path.read_bytes()
        text = raw_bytes.decode("utf-8")
    except OSError as error:
        raise RunnerError(f"mission contract is unreadable: {path}: {error}") from error
    except UnicodeDecodeError as error:
        raise RunnerError(f"mission contract is not UTF-8: {path}: {error}") from error
    if approved:
        fences = read_json(mission_dir(mission) / "fences.json", "mission fence counters")
        expected = fences.get("approvedContractSha256")
        if not isinstance(expected, str) or hashlib.sha256(raw_bytes).hexdigest() != expected:
            raise RunnerError("approved mission contract snapshot does not match approvedContractSha256")
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
        [MS, "identity", "started-at", "--pid", str(pid)],
        f"cannot resolve process start identity for pid {pid}",
    )
    try:
        return int(output)
    except ValueError as error:
        raise RunnerError(f"process start identity is invalid for pid {pid}") from error


def fake_identity(pid: int) -> dict[str, Any] | None:
    path = os.environ.get("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
    if not path:
        return None
    try:
        value = json.loads(Path(path).read_text(encoding="utf-8"))[str(pid)]
    except (OSError, json.JSONDecodeError, KeyError, TypeError):
        return None
    return value if isinstance(value, dict) else None


def publish_fake_identity(pid: int, started: int, pgid: int, tag: str) -> None:
    path_text = os.environ.get("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
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
    # Twin of internal/missionrunner ScaledSeconds; keep the rounding identical.
    raw = os.environ.get("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "1000")
    try:
        scale = int(raw)
    except ValueError as error:
        raise RunnerError("METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer") from error
    if scale < 1:
        raise RunnerError("METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer")
    return max(1, math.ceil(base * scale / 1000))


def interval_seconds(name: str, default_ms: int) -> float:
    # Twin of internal/missionrunner Interval; keep the validation identical.
    raw = os.environ.get(name, str(default_ms))
    try:
        milliseconds = int(raw)
    except ValueError as error:
        raise RunnerError(f"{name} must be a positive integer in milliseconds") from error
    if milliseconds < 1:
        raise RunnerError(f"{name} must be a positive integer in milliseconds")
    return milliseconds / 1000


def terminate_group(pgid: int, tag: str, *, allow_fake: bool = False) -> None:
    """Best-effort wind-down of a host group this runner launched.

    Ownership is proven by the tag on a live member. When the proof is gone --
    the tagged host exited and only untagged children linger, or the pgid was
    recycled -- the group is NOT ours to signal, and that is a normal way for
    a turn to end, not a mission-fatal error: the thing we launched is no
    longer running. Raising here killed a whole mission at the moment its
    host finished (the runner died with "lost ownership proof" seconds after
    the turn result landed, and the driver then polled a dead mission for
    hours). We never signal without proof; we also never die over a group
    that already stopped being ours. Anything genuinely left behind is
    UNTRACKED to the census, which is the safety net designed to catch it.
    """
    if not group_alive(pgid):
        return
    if not group_owned(pgid, tag, allow_fake=allow_fake):
        print(
            f"host process group {pgid} is no longer provably ours; "
            "leaving it to the census rather than signaling an unowned group",
            file=sys.stderr,
        )
        emit_event("wind-down", f"group {pgid} unowned; skipped",
                   missionId=CURRENT_MISSION, action="skipped-unowned",
                   reason="ownership-proof-absent")
        return
    emit_event("wind-down", f"group {pgid}", missionId=CURRENT_MISSION,
               action="sigterm")
    os.killpg(pgid, signal.SIGTERM)
    deadline = time.monotonic() + scaled_seconds(5)
    poll_interval = interval_seconds("METASYSTEM_HEARTBEAT_INTERVAL_MS", 50)
    while group_alive(pgid) and time.monotonic() < deadline:
        time.sleep(poll_interval)
    if group_alive(pgid):
        if not group_owned(pgid, tag, allow_fake=allow_fake):
            print(
                f"ownership proof for host process group {pgid} disappeared "
                "during wind-down; skipping the kill of an unowned group",
                file=sys.stderr,
            )
            return
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


_EVENT_SEQ = 0


def emit_event(event: str, summary: str, **fields: object) -> None:
    """Flight-recorder witness (plans/flight-recorder.md). Never raises."""
    global _EVENT_SEQ
    try:
        _EVENT_SEQ += 1
        args = [MS, "event", "emit", f"root={ROOT}", "component=runner",
                f"event={event}", f"summary={summary}", f"pid={os.getpid()}",
                f"pidStartedAt={RUNNER_STARTED_AT or 0}", f"seq={_EVENT_SEQ}"]
        args += [f"{k}={v}" for k, v in fields.items() if v is not None]
        subprocess.run(args, check=False, stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL, timeout=10)
    except BaseException:
        pass


RUNNER_STARTED_AT: int | None = None


def mission_lineage(mission: str) -> str:
    """The ownership lineage every process of one mission shares.

    Derived, not concatenated: a mission id has no length bound, so
    "mission-runner-<id>" would overflow the 128-character lineage bound for a
    long id and the mission could not arm at all. Truncating instead would let
    two missions sharing a prefix share a lineage, which would misread a foreign
    takeover as a renewal and suppress the epoch bump and sweep. A hash is
    fixed-length for any id and stays recomputable from the mission id.

    Twin of internal/missionrunner MissionLineage; the derivations must match
    or a successor process stops renewing its own mission's lease.
    """
    return "mission-" + hashlib.sha256(mission.encode("utf-8")).hexdigest()[:32]


def arming_identity(mission: str) -> tuple[str, int, int, str, str | None]:
    """Who this runner arms supervision as: session, pid, start, tag, lineage.

    The lineage is the mission's own ONLY when this runner is the main. Beneath
    a live holder the runner is part of that main's work, so it announces
    nothing new and must not rewrite that holder's lineage -- see D-3a in
    plans/lease-succession.md, which scopes this fix to the unattended branch.

    A runner started BY the main that holds this checkout is part of that
    main's work, not a second writer competing for the same checkout. Arming
    under a fresh identity there announces a second main and is correctly
    refused as OWNED-ELSEWHERE. Unattended -- a benchmark target, a scratch
    checkout, anything with no live holder -- the runner IS the main, and
    announces itself.
    """
    pid = os.getpid()
    view = run_command(
        [
            MS,
            "lease",
            "classify",
            "--root",
            str(ROOT),
            "--caller-pid",
            str(pid),
        ]
    )
    if view.returncode == 0:
        try:
            value = json.loads(view.stdout)
        except json.JSONDecodeError:
            value = {}
        announcement = value.get("announcement")
        if value.get("holder") is True and isinstance(announcement, dict):
            try:
                return (
                    str(announcement["sessionId"]),
                    int(announcement["pid"]),
                    int(announcement["pidStartedAt"]),
                    str(announcement["instanceTag"]),
                    None,
                )
            except (KeyError, TypeError, ValueError):
                pass
    return (
        f"mission-runner-{mission}-{pid}",
        pid,
        process_started_at(pid),
        "mission-runner.sh",
        mission_lineage(mission),
    )


def pin_verified_contract(mission: str, mode: str, snapshot: bytes, approved_sha256: str) -> None:
    directory = mission_dir(mission)
    fences_path = directory / "fences.json"
    lock_path = directory / "mission-fence.lock"
    directory.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a+") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        if fences_path.exists():
            fences = read_json(fences_path, "mission fence counters")
        else:
            if mode != "start":
                raise RunnerError("mission resume refused: fence state is absent")
            fences = {
                "schemaVersion": 1,
                "missionId": mission,
                "startedAt": now_iso(),
                "cycles": 0,
                "reservations": {},
            }
        if fences.get("schemaVersion") != 1 or fences.get("missionId") != mission:
            raise RunnerError("mission fence counters have an invalid identity")
        if mode == "start" and fences.get("approvedContractSha256") is not None:
            raise RunnerError("mission start refused: approved contract is already pinned; use resume")
        if hashlib.sha256(snapshot).hexdigest() != approved_sha256:
            raise RunnerError("mission preflight snapshot does not match its verified raw-file sha256")
        # This snapshot and digest come from one preflight invocation. The pin
        # is the raw-file SHA-256, including Approval and trailing whitespace.
        atomic_bytes(approved_contract_path(mission), snapshot)
        fences["approvedContractSha256"] = approved_sha256
        atomic_json(fences_path, fences)


def arm_and_preflight(mission: str, mode: str) -> None:
    session, pid, started, tag, lineage = arming_identity(mission)
    command = [
        str(ROOT / "scripts" / "agents" / "arm-supervision.sh"),
        "--repo",
        str(ROOT),
        "--session",
        session,
        "--pid",
        str(pid),
        "--start-time",
        str(started),
        "--tag",
        tag,
    ]
    if lineage is not None:
        # Every process of this mission derives the same lineage, so a
        # successor renews the lease instead of taking it over and sweeping
        # the predecessor's in-flight delegates.
        command += ["--owner-lineage", lineage]
    arm = run_command(command)
    if arm.returncode != 0 or "ARMED" not in arm.stdout:
        detail = (arm.stderr or arm.stdout).strip()
        raise RunnerError(f"mission start refused: supervision did not arm: {detail}")
    descriptor, verified_name = tempfile.mkstemp(prefix=f"mission-{mission}-verified.", suffix=".contract.md")
    os.close(descriptor)
    verified_path = Path(verified_name)
    try:
        preflight = run_command(
            [
                MS,
                "mission-contract",
                "preflight",
                "--file",
                str(contract_path(mission)),
                "--verified-bytes-output",
                str(verified_path),
            ]
        )
        if preflight.returncode != 0:
            raise RunnerError(f"mission start refused by preflight: {(preflight.stderr or preflight.stdout).strip()}")
        match = re.search(r"approvedContractSha256=([0-9a-f]{64})", preflight.stdout)
        if match is None:
            raise RunnerError("mission start refused: preflight omitted the verified raw-file sha256")
        try:
            verified_bytes = verified_path.read_bytes()
        except OSError as error:
            raise RunnerError(f"mission start refused: verified contract snapshot is unreadable: {error}") from error
        pin_verified_contract(mission, mode, verified_bytes, match.group(1))
    finally:
        verified_path.unlink(missing_ok=True)


def verify_state(path: Path, *, anchor: bool = False) -> dict[str, Any]:
    command = [MS, "mission-state", "verify", "--state", str(path)]
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
                MS,
                "mission-state",
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


def git_author_environment(identity: str) -> dict[str, str]:
    environment = os.environ.copy()
    environment.update(
        {
            "GIT_AUTHOR_NAME": identity,
            "GIT_AUTHOR_EMAIL": f"{identity}@metasystem.invalid",
        }
    )
    return environment


def anchor_state(state_path: Path, ledger: Path, identity: str) -> None:
    result = run_command(
        [
            MS,
            "mission-state",
            "anchor",
            "--state",
            str(state_path),
            "--repo",
            str(ROOT),
            "--ledger",
            str(ledger),
        ],
        env=git_author_environment(identity),
    )
    if result.returncode != 0:
        raise RunnerError(f"mission anchor refused: {(result.stderr or result.stdout).strip()}")


def initialize_state(mission: str, lease: Path) -> tuple[Path, Path, dict[str, Any]]:
    directory = mission_dir(mission)
    state_path = directory / "state.json"
    ledger = directory / "ledger.md"
    if state_path.exists() or ledger.exists():
        raise RunnerError("mission state already exists; use resume")
    _, values, _ = parse_contract(mission)
    require_command(
        [
            MS,
            "mission-ledger",
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
            MS,
            "mission-state",
            "init",
            "--state",
            str(state_path),
            "--contract",
            str(approved_contract_path(mission)),
            "--ledger",
            str(ledger),
            "--lease",
            str(lease),
        ],
        "mission state initialization refused",
    )
    anchor_state(state_path, ledger, mission)
    return state_path, ledger, verify_state(state_path, anchor=True)


def resume_state(mission: str) -> tuple[Path, Path, dict[str, Any]]:
    directory = mission_dir(mission)
    state_path = directory / "state.json"
    ledger = directory / "ledger.md"
    if not state_path.exists():
        raise RunnerError("mission state does not exist", 7)
    result = run_command(
        [
            MS,
            "mission-state",
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


CURRENT_MISSION: str | None = None


def finish_runner(mission: str, status: str, error: str | None) -> None:
    if status == "failed":
        emit_event("runner-failed", str(error or "unknown")[:200],
                   missionId=mission, error=str(error or "unknown"))
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
    tag = f"metasystem-host-{turn_id}"
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
    environment = git_author_environment(turn_id)
    environment.update(
        {
            "METASYSTEM_MISSION_ID": mission,
            # Every TURN launches a fresh host process, which arms in the target
            # and becomes the lease holder under its own per-process mainId.
            # Without a shared lineage the next turn's host takes the lease from
            # its own dead predecessor and sweeps whatever delegates that turn
            # left in flight -- the loop that cost bm-2 two of three delegates.
            # The host's arming inherits this, so every turn of one mission is
            # the same logical writer and succession renews instead.
            "METASYSTEM_OWNER_LINEAGE": mission_lineage(mission),
            "METASYSTEM_MISSION_LEASE": str(lease),
            "METASYSTEM_MISSION_TURN": turn_id,
            "METASYSTEM_HOST_START_GATE": str(host_gate),
            "METASYSTEM_HOST_START_GATE_TIMEOUT_SEC": str(scaled_seconds(10)),
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
    handshake_poll_interval = interval_seconds("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS", 20)
    started = None
    verified = False
    fake_runtime = turn["runtime"] == "fake"
    force_unverified = fake_runtime and os.environ.get("METASYSTEM_FAKE_HOST_START_UNVERIFIED") == "1"
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
        time.sleep(handshake_poll_interval)
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
    heartbeat_interval = interval_seconds("METASYSTEM_HEARTBEAT_INTERVAL_MS", 100)
    while process.poll() is None:
        heartbeat(mission, turn_id)
        if time.monotonic() >= deadline:
            terminate_group(process.pid, tag, allow_fake=fake_runtime)
            capped = True
            break
        time.sleep(heartbeat_interval)
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


def drain_jobs(mission: str) -> None:
    # Which jobs still need reaping is the binary's judgment; the reap itself
    # stays with dispatch.sh, the owner of job lifecycles.
    poll_interval = interval_seconds("METASYSTEM_HEARTBEAT_INTERVAL_MS", 100)
    while True:
        listing = ms_json(
            ["mission-jobs", "drain", "--root", str(ROOT), "--mission", mission],
            "mission job drain",
        )
        active = listing.get("activeJobs") or []
        if not active:
            return
        for job_id in active:
            run_command([str(ROOT / "scripts" / "agents" / "dispatch.sh"), "reap", "--job", str(job_id)])
        time.sleep(poll_interval)


def close_terminal_chains(mission: str) -> None:
    listing = ms_json(
        ["mission-jobs", "close-chains", "--root", str(ROOT), "--mission", mission],
        "mission chain close",
    )
    for root_id in listing.get("chains") or []:
        run_command([str(ROOT / "scripts" / "agents" / "dispatch.sh"), "reap", "--job", str(root_id)])
        closed = run_command(
            [
                str(ROOT / "scripts" / "agents" / "dispatch.sh"),
                "close",
                "--job",
                str(root_id),
                "--runner-closed",
            ]
        )
        if closed.returncode != 0:
            detail = (closed.stderr or closed.stdout).strip()
            raise RunnerError(f"runner could not close terminal job chain {root_id}: {detail}")


def previous_metrics(state: dict[str, Any], names: list[str]) -> str | None:
    """The prior per-metric values a regression is judged against.

    The most recent turn whose measurement carries every declared metric wins;
    when none does, None lets the gate measure against the sealed baseline.
    """
    for item in reversed(state["turnLog"]):
        measurement = item.get("measurement")
        if not isinstance(measurement, dict):
            continue
        metrics = measurement.get("metrics")
        if isinstance(metrics, dict) and all(name in metrics for name in names):
            return ",".join(f"{name}={metrics[name]}" for name in names)
    return None


def measure(mission: str, state: dict[str, Any]) -> tuple[str, str, dict[str, Any], bool]:
    _, values, _ = parse_contract(mission, approved=False)
    names = sorted(
        key.removeprefix("gate.threshold.") for key in values if key.startswith("gate.threshold.")
    )
    command = [MS, "mission-contract", "measure", "--file", str(contract_path(mission))]
    previous = previous_metrics(state, names)
    if previous is not None:
        command += ["--previous", previous]
    result = run_command(command)
    if result.returncode != 0:
        raise RunnerError((result.stderr or result.stdout).strip() or "mission measurement refused")
    payload = json.loads(result.stdout)
    measurement = {
        "metrics": payload["metrics"],
        "guards": payload["guards"],
        "candidateSha": payload["candidateSha"],
    }
    return payload["classification"], payload["observed"], measurement, bool(payload["gatePassed"])


def append_ledger(ledger: Path, cycle: int, classification: str, candidate_sha: str, observed: str) -> None:
    result = run_command(
        [
            MS,
            "mission-ledger",
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


def write_proposed_asks(mission: str, asks: list[dict[str, Any]]) -> None:
    """Write the ask records a verb proposed, exactly as proposed.

    The proposal's waiting list assumes these asks land; writing anything
    else would make the state lie about what can be answered.
    """
    asks_dir = mission_dir(mission) / "asks"
    for ask in asks:
        atomic_json(asks_dir / f"{ask['askId']}.json", ask)


def park_state(
    state_path: Path,
    ledger: Path,
    reason: str,
    mission: str,
    identity: str,
) -> dict[str, Any]:
    emit_event("mission-parked", str(reason)[:200], missionId=mission, parkReason=str(reason))
    proposal = ms_json(
        [
            "mission-turn",
            "park",
            "--root",
            str(ROOT),
            "--mission",
            mission,
            "--state",
            str(state_path),
            "--reason",
            reason,
        ],
        "mission park proposal",
    )
    write_proposed_asks(mission, proposal.get("asks") or [])
    updated = write_state(state_path, proposal["state"])
    anchor_state(state_path, ledger, identity)
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
    proposed = ms_json(
        [
            "mission-turn",
            "record-failure",
            "--root",
            str(ROOT),
            "--mission",
            mission,
            "--state",
            str(state_path),
            "--turn",
            str(mission_dir(mission) / "turns" / turn["turnId"] / "turn.json"),
            "--detail",
            detail,
            "--outcome",
            outcome,
            "--consecutive-failures",
            str(consecutive_failures),
        ],
        "mission failed-turn proposal",
    )
    updated = write_state(state_path, proposed)
    anchor_state(state_path, ledger, turn["turnId"])
    if updated["status"] == "parked" and updated.get("parkReason") == "host-failure":
        return park_state(state_path, ledger, "host-failure", mission, turn["turnId"])
    if updated["status"] == "running":
        stop = run_command([str(ROOT / "scripts" / "assert-stop-loss.sh"), "--file", str(ledger)])
        if stop.returncode == 1:
            updated = park_state(state_path, ledger, "stop-loss", mission, turn["turnId"])
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
        [MS, "mission-fence", "reserve-cycle", "--repo", str(ROOT), "--mission", mission]
    )
    if reserve.returncode != 0:
        return park_state(state_path, ledger, "fence", mission, mission)
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
            MS,
            "mission-prompt",
            "assemble",
            "--repo",
            str(ROOT),
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

    emit_event("turn-launched", f"cycle {turn.get('cycle')}",
               missionId=mission, turnId=turn_id)
    exit_code, result, launch_detail = launch_host(mission, turn_id, turn_dir, turn, lease, start_signal, notified)
    turn = read_json(turn_path, "turn record")
    emit_event("turn-result",
               f"exit {exit_code}" + (f" ({launch_detail})" if launch_detail else ""),
               missionId=mission, turnId=turn_id,
               outcome=launch_detail or ("ok" if exit_code == 0 else f"exit-{exit_code}"))
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
        verdict = ms_json(
            [
                "mission-turn",
                "adjudicate",
                "--root",
                str(ROOT),
                "--mission",
                mission,
                "--state",
                str(state_path),
                "--turn",
                str(turn_path),
                "--result",
                str(turn_dir / "result.json"),
                "--turn-dir",
                str(turn_dir),
            ],
            "orchestrator return adjudication",
        )
    except RunnerError as error:
        detail = str(error)
        patch_turn(turn_path, status="failed", outcome="failed", error="protocol-error", detail=detail, endedAt=now_iso(), result=result)
        return record_failed_turn(state_path, ledger, state, turn, detail, "failed", prior_failures + 1, mission)

    # The verdict is the audit record of what this turn's return claimed and
    # what the runner made of it; conclude reads it back below.
    atomic_json(turn_dir / "adjudication.json", verdict)
    (mission_dir(mission) / "asks").mkdir(parents=True, exist_ok=True)
    write_proposed_asks(mission, verdict.get("asks") or [])
    drain_jobs(mission)
    try:
        classification, observed, measurement, gate_passed = measure(mission, state)
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
    atomic_json(turn_dir / "measurement.json", {"measurement": measurement, "gatePassed": gate_passed})
    proposed = ms_json(
        [
            "mission-turn",
            "conclude",
            "--root",
            str(ROOT),
            "--mission",
            mission,
            "--state",
            str(state_path),
            "--turn",
            str(turn_path),
            "--verdict",
            str(turn_dir / "adjudication.json"),
            "--return",
            verdict["returnPath"],
            "--result",
            str(turn_dir / "result.json"),
            "--measurement",
            str(turn_dir / "measurement.json"),
        ],
        "mission turn conclusion",
    )
    updated = write_state(state_path, proposed)
    patch_turn(
        turn_path,
        status="completed",
        outcome="completed",
        error=None,
        detail="host return accepted",
        result=result,
        endedAt=now_iso(),
        rawPath=verdict["rawPath"],
        returnPath=verdict["returnPath"],
    )
    anchor_state(state_path, ledger, turn_id)
    if updated["status"] == "running":
        stop = run_command([str(ROOT / "scripts" / "assert-stop-loss.sh"), "--file", str(ledger)])
        if stop.returncode == 1:
            updated = park_state(state_path, ledger, "stop-loss", mission, turn_id)
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
        close_terminal_chains(mission)
        if not notified[0]:
            write_start_signal(start_signal, False, None, f"mission parked before a host turn started: {state.get('parkReason')}")
        if lease is not None:
            release_lease(mission)
            lease = None
        finish_runner(mission, "completed", None)
        return 0
    except RunnerError as error:
        if not notified[0]:
            write_start_signal(start_signal, False, None, str(error))
        if lease is not None:
            release_lease(mission)
            lease = None
        finish_runner(mission, "failed", str(error))
        return error.code
    except Exception as error:
        if not notified[0]:
            write_start_signal(start_signal, False, None, str(error))
        if lease is not None:
            release_lease(mission)
            lease = None
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
    arm_and_preflight(mission, command)
    tag = f"metasystem-mission-runner-{mission}-{secrets.token_hex(3)}"
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
    handshake_poll_interval = interval_seconds("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS", 50)
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
        time.sleep(handshake_poll_interval)
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
    if state["status"] == "running":
        # "Running" is a claim about a PROCESS, and the state file cannot make
        # it alone: when the runner has died, the mission advances nothing,
        # enforces no fence, and a driver trusting this status polls forever
        # (four and a half hours, the night this was written). The runner
        # record and the kernel decide whether anyone is actually driving.
        record_path, _, _ = runner_paths(mission)
        record = read_json(record_path, "mission runner record") if record_path.exists() else None
        if record is None:
            print(f"mission={mission} status=abandoned reason=no-runner-record")
            return 13
        if record.get("status") == "failed":
            failure = str(record.get("error") or "unknown").replace(" ", "-")
            print(f"mission={mission} status=runner-failed reason={failure}")
            return 13
        if record.get("status") == "completed":
            # The previous runner CONCLUDED -- it parked or finished and
            # finalized its record -- and a human's answer has reopened the
            # mission. "Running with no live runner" is the legitimate
            # awaiting-resume resting state here, not abandonment: the next
            # step is `resume`, by whoever answered. Only a runner that died
            # without concluding is a defect worth stopping a driver for.
            pass
        else:
            pid, started = record.get("pid"), record.get("pidStartedAt")
            try:
                alive = (
                    isinstance(pid, int)
                    and isinstance(started, int)
                    and process_started_at(pid) == started
                )
            except RunnerError:
                # A pid that cannot be resolved is a pid that is gone.
                alive = False
            if not alive:
                print(f"mission={mission} status=abandoned reason=runner-process-gone")
                return 13
    print(f"mission={mission} status={state['status']} reason={reason}")
    return {"running": 0, "completed": 10, "parked": 11}.get(state["status"], 7)


# TODO(go-wiring): fence_reached repeats the threshold math that lives in the
# mission-fence family; it needs a mission-fence verb that reports whether a
# contract's fences are reached, then the answer path can call that instead.
def fence_reached(mission: str, values: dict[str, str]) -> bool:
    result = _fence_reached_inner(mission, values)
    emit_event("fence-check", f"reached={result}", missionId=mission,
               fence="mission-fences")
    return result


def _fence_reached_inner(mission: str, values: dict[str, str]) -> bool:
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
        _, values, _ = parse_contract(mission, approved=False)
        if not fence_reached(mission, values):
            proposed.update({"status": "running", "parkReason": None, "gatePassed": False})
    proposed["waitingList"] = [value for value in proposed["waitingList"] if value != ask_id]
    original_ask = copy.deepcopy(ask)
    ask.update({"answeredAt": now_iso(), "answer": answer})
    atomic_json(ask_path, ask)
    try:
        updated = write_state(state_path, proposed)
        anchor_state(state_path, mission_dir(mission) / "ledger.md", mission)
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
            globals()["CURRENT_MISSION"] = values["mission"]
            try:
                globals()["RUNNER_STARTED_AT"] = process_started_at(os.getpid())
            except BaseException:
                pass
            emit_event("runner-started", f"mode={values['mode']}",
                       missionId=values["mission"])
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
