#!/usr/bin/env python3
"""Process identity, signature, scope, and census mechanics for supervision."""

from __future__ import annotations

import argparse
import datetime as dt
import errno
import json
import os
import re
import shlex
import signal
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass, replace
from hashlib import sha256
from pathlib import Path
from typing import Any, Iterable


LIVE_STATUSES = {"pending", "running", "starting"}
PATH_FLAGS = {
    "-C", "--cwd", "--directory", "--path", "--project-dir", "--repo",
    "--root", "--workspace", "--worktree",
}
METASYSTEM_ROOT = Path(__file__).resolve().parents[2]


class CensusError(RuntimeError):
    pass


@dataclass(frozen=True)
class Process:
    pid: int
    ppid: int
    pgid: int
    started: int
    argv: str
    cwd: str | None
    cwd_error: bool = False
    alive: bool = True


def config_value(root: Path, key: str, default: str = "") -> str:
    try:
        lines = (root / "metasystem.conf").read_text(encoding="utf-8").splitlines()
    except OSError:
        return default
    values = []
    for raw in lines:
        stripped = raw.strip()
        if not stripped or stripped.startswith("#") or "=" not in raw:
            continue
        found, value = (part.strip() for part in raw.split("=", 1))
        if found == key:
            values.append(value)
    return values[-1] if values else default


def linux_started_at(pid: int) -> int:
    stat = Path(f"/proc/{pid}/stat").read_text(encoding="utf-8")
    close = stat.rfind(")")
    if close < 0:
        raise CensusError(f"malformed /proc stat for pid {pid}")
    fields = stat[close + 2 :].split()
    ticks_after_boot = int(fields[19])
    clock_ticks = os.sysconf("SC_CLK_TCK")
    boot = None
    for raw in Path("/proc/stat").read_text(encoding="utf-8").splitlines():
        if raw.startswith("btime "):
            boot = int(raw.split()[1])
            break
    if boot is None:
        raise CensusError("/proc/stat has no btime")
    return boot + ticks_after_boot // clock_ticks


def ps_started_at(pid: int) -> int:
    env = {**os.environ, "LC_ALL": "C"}
    result = subprocess.run(
        ["ps", "-p", str(pid), "-o", "lstart="],
        text=True,
        capture_output=True,
        env=env,
        check=False,
    )
    if result.returncode != 0 or not result.stdout.strip():
        raise ProcessLookupError(pid)
    try:
        value = dt.datetime.strptime(result.stdout.strip(), "%a %b %d %H:%M:%S %Y")
    except ValueError as error:
        raise CensusError(f"unreadable start time for pid {pid}: {error}") from error
    return int(value.replace(tzinfo=dt.datetime.now().astimezone().tzinfo).timestamp())


def ps_identity(pid: int) -> dict[str, Any]:
    """Read authentication start time and command from one process-table row."""
    env = {**os.environ, "LC_ALL": "C"}
    result = subprocess.run(
        ["ps", "-p", str(pid), "-o", "lstart=,command="],
        text=True,
        capture_output=True,
        env=env,
        check=False,
    )
    if result.returncode != 0 or not result.stdout.strip():
        raise ProcessLookupError(pid)
    match = re.fullmatch(
        r"\s*([A-Z][a-z]{2}\s+[A-Z][a-z]{2}\s+[0-9]{1,2}\s+"
        r"[0-9]{2}:[0-9]{2}:[0-9]{2}\s+[0-9]{4})\s+(.+?)\s*",
        result.stdout,
        re.DOTALL,
    )
    if not match:
        raise CensusError(f"unreadable process identity for pid {pid}")
    try:
        started = dt.datetime.strptime(match.group(1), "%a %b %d %H:%M:%S %Y")
    except ValueError as error:
        raise CensusError(f"unreadable start time for pid {pid}: {error}") from error
    command = match.group(2).rstrip("\n")
    if not command:
        raise CensusError(f"unreadable command for pid {pid}")
    return {
        "pid": pid,
        "pidStartedAt": int(
            started.replace(tzinfo=dt.datetime.now().astimezone().tzinfo).timestamp()
        ),
        "command": command,
    }


def authentication_identity(pid: int) -> dict[str, Any]:
    """Start time and command from ONE source.

    The simulated table takes precedence when installed, exactly as
    started_at treats it. Preferring the real process table here while
    started_at preferred the fixture split one identity across two sources: a
    main announced a real command hash beside a simulated start time and could
    then never recognise its own announcement.
    """
    fixture = os.environ.get("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
    if fixture:
        try:
            values = json.loads(Path(fixture).read_text(encoding="utf-8"))
        except (OSError, ValueError):
            values = {}
        value = values.get(str(pid)) if isinstance(values, dict) else None
        if (
            isinstance(value, dict)
            and type(value.get("started")) is int
            and isinstance(value.get("command"), str)
            and value["command"]
        ):
            return {"pid": pid, "pidStartedAt": value["started"], "command": value["command"]}
    return ps_identity(pid)


def started_at(pid: int) -> int:
    if sys.platform.startswith("linux") and Path(f"/proc/{pid}/stat").exists():
        try:
            return linux_started_at(pid)
        except (OSError, ValueError, CensusError):
            pass
    try:
        return ps_started_at(pid)
    except (OSError, PermissionError, ProcessLookupError):
        fixture = os.environ.get("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
        if not fixture:
            raise
        return fake_identity_started_at(Path(fixture), pid)


def fake_identity_started_at(path: Path, pid: int) -> int:
    """Restricted-CI identity source, enabled explicitly by fake fixtures."""
    if not pid_exists(pid):
        raise ProcessLookupError(pid)
    path.parent.mkdir(parents=True, exist_ok=True)
    lock_path = path.with_suffix(path.suffix + ".lock")
    import fcntl
    with lock_path.open("a+") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        try:
            values = json.loads(path.read_text(encoding="utf-8")) if path.exists() else {}
        except (OSError, ValueError):
            values = {}
        key = str(pid)
        if key not in values:
            values[key] = {
                "pidStartedAt": int(time.time()),
                "pgid": os.getpgid(pid),
                "command": "fake-process-identity",
            }
            atomic_json(path, values)
        return int(values[key]["pidStartedAt"])


def pid_exists(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def identity_alive(pid: int, expected_start: int) -> bool:
    if not pid_exists(pid):
        return False
    fixture = os.environ.get("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
    if fixture:
        try:
            values = json.loads(Path(fixture).read_text(encoding="utf-8"))
            return int(values[str(pid)]["pidStartedAt"]) == expected_start
        except (OSError, ValueError, KeyError, TypeError):
            pass
    try:
        return started_at(pid) == expected_start
    except (OSError, ProcessLookupError, CensusError):
        return False


def process_command(pid: int) -> str:
    result = subprocess.run(
        ["ps", "-p", str(pid), "-o", "command="],
        text=True,
        capture_output=True,
        env={**os.environ, "LC_ALL": "C"},
        check=False,
    )
    if result.returncode != 0:
        raise ProcessLookupError(pid)
    return result.stdout.strip()


def read_supervision_snapshot() -> tuple[dict[str, dict[str, Any]], int, str]:
    state_path = METASYSTEM_ROOT / "artifacts" / "agents" / "supervision" / "state.json"
    try:
        state_bytes = state_path.read_bytes()
        state = json.loads(state_bytes)
    except (OSError, ValueError) as error:
        raise CensusError(f"supervision state is unavailable: {error}") from error
    if not isinstance(state, dict):
        raise CensusError("supervision state is not an object")
    generation = state.get("generation")
    if type(generation) is not int or generation < 1:
        raise CensusError("supervision state has an invalid generation")
    components = state.get("components")
    owner = state.get("owner")
    if not isinstance(components, dict) or set(components) != {"watcher", "reaper"} or not isinstance(owner, dict):
        raise CensusError("supervision state has no complete instance set")
    identities: dict[str, dict[str, Any]] = {}
    for name, value in {"owner": owner, **components}.items():
        if not isinstance(value, dict):
            raise CensusError(f"supervision state has invalid {name} identity")
        pid, started, tag = value.get("pid"), value.get("pidStartedAt"), value.get("instanceTag")
        if type(pid) is not int or pid < 1 or type(started) is not int or started < 1 or not isinstance(tag, str) or not tag:
            raise CensusError(f"supervision state has invalid {name} identity")
        identities[name] = {"pid": pid, "pidStartedAt": started, "instanceTag": tag}
    return identities, generation, sha256(state_bytes).hexdigest()


def verify_supervision_snapshot(identities: dict[str, dict[str, Any]], errors: list[str]) -> None:
    for name in ("owner", "watcher", "reaper"):
        identity = identities[name]
        pid, started, tag = identity["pid"], identity["pidStartedAt"], identity["instanceTag"]
        if not identity_alive(pid, started):
            errors.append(f"supervision-not-live:{name}:pid={pid}")
            continue
        try:
            command = process_command(pid)
        except (OSError, ProcessLookupError):
            errors.append(f"supervision-command-unavailable:{name}:pid={pid}")
            continue
        if tag not in command:
            errors.append(f"supervision-tag-mismatch:{name}:pid={pid}")
        elif not identity_alive(pid, started):
            errors.append(f"supervision-not-live:{name}:pid={pid}")


def resolve_cwd(pid: int) -> tuple[str | None, bool]:
    proc_cwd = Path(f"/proc/{pid}/cwd")
    if proc_cwd.exists() or sys.platform.startswith("linux"):
        try:
            return os.path.realpath(os.readlink(proc_cwd)), False
        except FileNotFoundError:
            if not pid_exists(pid):
                return None, False
        except (OSError, PermissionError):
            pass
    result = subprocess.run(
        ["lsof", "-a", "-p", str(pid), "-d", "cwd", "-Fn"],
        text=True,
        capture_output=True,
        check=False,
    ) if shutil_which("lsof") else None
    if result is not None and result.returncode == 0:
        for raw in result.stdout.splitlines():
            if raw.startswith("n") and len(raw) > 1:
                return os.path.realpath(raw[1:]), False
    if not pid_exists(pid):
        return None, False
    return None, True


def shutil_which(command: str) -> str | None:
    for directory in os.environ.get("PATH", "").split(os.pathsep):
        candidate = Path(directory or ".") / command
        if candidate.is_file() and os.access(candidate, os.X_OK):
            return str(candidate)
    return None


def enumerate_ps() -> list[Process]:
    result = subprocess.run(
        ["ps", "-axo", "pid=,ppid=,pgid=,lstart=,command="],
        text=True,
        capture_output=True,
        env={**os.environ, "LC_ALL": "C"},
        check=False,
    )
    if result.returncode != 0:
        raise CensusError(f"process enumeration failed: ps exited {result.returncode}")
    pattern = re.compile(
        r"^\s*(\d+)\s+(\d+)\s+(\d+)\s+"
        r"([A-Z][a-z]{2}\s+[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\s+\d{4})\s+(.*)$"
    )
    processes = []
    local_zone = dt.datetime.now().astimezone().tzinfo
    for raw in result.stdout.splitlines():
        match = pattern.match(raw)
        if not match:
            continue
        pid, ppid, pgid = (int(match.group(index)) for index in range(1, 4))
        argv = match.group(5)
        if not argv:
            continue
        try:
            parsed = dt.datetime.strptime(match.group(4), "%a %b %d %H:%M:%S %Y")
        except ValueError:
            # A malformed row cannot safely join custody if it is agent-shaped;
            # retain an impossible start so the signature pass can fail it.
            epoch = -1
        else:
            epoch = int(parsed.replace(tzinfo=local_zone).timestamp())
        processes.append(Process(pid, ppid, pgid, epoch, argv, None))
    return processes


def enumerate_fixture(path: Path) -> list[Process]:
    if config_value(METASYSTEM_ROOT, "metasystem.runtimes") != "fake":
        raise CensusError("METASYSTEM_CENSUS_PROCESS_FILE is allowed only when metasystem.runtimes=fake")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise CensusError(f"process enumeration fixture is unreadable: {error}") from error
    if not isinstance(value, list):
        raise CensusError("process enumeration fixture must be an array")
    processes = []
    required = {"pid", "ppid", "pgid", "pidStartedAt", "argv", "cwd", "cwdError", "alive"}
    for item in value:
        if not isinstance(item, dict) or set(item) != required:
            raise CensusError("process enumeration fixture row has invalid shape")
        if not isinstance(item["argv"], str) or not item["argv"]:
            raise CensusError("process enumeration fixture has unreadable argv")
        processes.append(Process(
            int(item["pid"]), int(item["ppid"]), int(item["pgid"]),
            int(item["pidStartedAt"]), item["argv"], item["cwd"],
            bool(item["cwdError"]), bool(item["alive"]),
        ))
    return processes


def enumerate_processes(_repo: Path) -> list[Process]:
    fixture = os.environ.get("METASYSTEM_CENSUS_PROCESS_FILE")
    return enumerate_fixture(Path(fixture)) if fixture else enumerate_ps()


def grep_ere(pattern: str, text: str) -> tuple[bool, bool]:
    result = subprocess.run(
        ["grep", "-E", "-q", "--", pattern], input=text + "\n", text=True,
        stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, check=False,
    )
    if result.returncode == 0:
        return True, True
    if result.returncode == 1:
        return False, True
    return False, False


def adapter_signatures(adapter: Path) -> tuple[list[str], list[str], str]:
    result = subprocess.run([str(adapter), "signature"], text=True, capture_output=True, check=False)
    if result.returncode != 0:
        raise CensusError(f"signature adapter failed: {adapter.name} exit={result.returncode}")
    matches: list[str] = []
    excludes: list[str] = []
    normalized = []
    for raw in result.stdout.splitlines():
        if not raw or raw != raw.strip():
            raise CensusError(f"malformed signature declaration from {adapter.name}")
        verb, separator, pattern = raw.partition(" ")
        if verb not in {"match", "exclude"} or not separator or not pattern:
            raise CensusError(f"malformed signature declaration from {adapter.name}: {raw}")
        _, valid = grep_ere(pattern, "")
        if not valid:
            raise CensusError(f"invalid POSIX ERE from {adapter.name}: {pattern}")
        (matches if verb == "match" else excludes).append(pattern)
        normalized.append(raw)
    if not matches:
        raise CensusError(f"signature adapter returned no match declaration: {adapter.name}")
    return matches, excludes, "\n".join(normalized) + "\n"


_SIGNATURE_CACHE: dict[tuple[str, ...], dict[str, tuple[list[str], list[str], str]]] = {}


def configured_signatures(runtimes: Iterable[str] | None = None) -> dict[str, tuple[list[str], list[str], str]]:
    """Memoized for the life of the process.

    Every census pass asks for signatures twice — once to classify processes
    and once inside the fingerprint — and each ask spawned one subprocess per
    runtime. Adapters cannot change mid-pass (a change to one alters the
    fingerprint, which is itself what forces a re-arm), so the second ask is
    pure overhead. This is the difference between a census that keeps up with
    a fast interval and one that cannot.
    """
    cache_key = tuple(runtimes) if runtimes is not None else ("__configured__",)
    cached = _SIGNATURE_CACHE.get(cache_key)
    if cached is not None:
        return cached
    result = _configured_signatures_uncached(runtimes)
    _SIGNATURE_CACHE[cache_key] = result
    return result


def _configured_signatures_uncached(runtimes: Iterable[str] | None = None) -> dict[str, tuple[list[str], list[str], str]]:
    selected = list(runtimes) if runtimes is not None else [
        item.strip() for item in config_value(METASYSTEM_ROOT, "metasystem.runtimes").split(",") if item.strip()
    ]
    if not selected:
        raise CensusError(
            f"{METASYSTEM_ROOT / 'metasystem.conf'} lists no metasystem.runtimes; "
            f"configure at least one runtime with an executable signature adapter under "
            f"{METASYSTEM_ROOT / 'scripts' / 'agents' / 'adapters'}"
        )
    result = {}
    for runtime in selected:
        adapter = METASYSTEM_ROOT / "scripts" / "agents" / "adapters" / f"{runtime}.sh"
        if not adapter.is_file() or not os.access(adapter, os.X_OK):
            raise CensusError(
                f"metasystem.runtimes names {runtime!r}, but its signature adapter is missing "
                f"or not executable: {adapter}; install or enable that adapter, or remove "
                f"{runtime!r} from {METASYSTEM_ROOT / 'metasystem.conf'}"
            )
        result[runtime] = adapter_signatures(adapter)
    return result


def signature_runtime(argv: str, declarations: dict[str, tuple[list[str], list[str], str]]) -> str | None:
    for runtime, (matches, excludes, _) in declarations.items():
        excluded = any(grep_ere(pattern, argv)[0] for pattern in excludes)
        matched = any(grep_ere(pattern, argv)[0] for pattern in matches)
        if matched and not excluded:
            return runtime
    return None


def grep_ere_lines(pattern: str, lines: list[str]) -> set[int]:
    if not lines:
        return set()
    result = subprocess.run(
        ["grep", "-E", "-n", "--", pattern], input="\n".join(lines) + "\n", text=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False,
    )
    if result.returncode == 1:
        return set()
    if result.returncode != 0:
        raise CensusError(f"configured signature became invalid during census: {pattern}")
    matches = set()
    for raw in result.stdout.splitlines():
        number, separator, _ = raw.partition(":")
        if separator and number.isdigit() and 1 <= int(number) <= len(lines):
            matches.add(int(number) - 1)
    return matches


def signature_processes(
    processes: list[Process],
    declarations: dict[str, tuple[list[str], list[str], str]],
) -> list[tuple[Process, str]]:
    """Match one immutable process snapshot with bounded grep subprocesses."""
    argvs = [process.argv for process in processes]
    assigned: dict[int, str] = {}
    for runtime, (matches, excludes, _) in declarations.items():
        excluded_indexes: set[int] = set()
        matched_indexes: set[int] = set()
        for pattern in excludes:
            excluded_indexes.update(grep_ere_lines(pattern, argvs))
        for pattern in matches:
            matched_indexes.update(grep_ere_lines(pattern, argvs))
        for index in matched_indexes - excluded_indexes:
            assigned.setdefault(index, runtime)
    return [
        (process, assigned[index])
        for index, process in enumerate(processes)
        if index in assigned
    ]


def resolve_cwds_batch(pids: list[int]) -> dict[int, tuple[str | None, bool]]:
    """One lsof for every candidate instead of one lsof each.

    lsof costs about fifty milliseconds per invocation on macOS regardless of
    how many pids it is asked about, so six candidates cost six times as much
    as one for no reason. Profiling a real census put 307ms of a 475ms pass in
    exactly this loop — the true cost the census's interval could not keep up
    with, which is neither the process count nor the signature spawns an
    earlier guess blamed.
    """
    if not pids:
        return {}
    if not shutil_which("lsof"):
        return {pid: resolve_cwd(pid) for pid in pids}
    result = subprocess.run(
        ["lsof", "-a", "-p", ",".join(str(pid) for pid in pids), "-d", "cwd", "-Fpn"],
        text=True,
        capture_output=True,
        check=False,
    )
    found: dict[int, tuple[str | None, bool]] = {}
    current: int | None = None
    for raw in result.stdout.splitlines():
        if raw.startswith("p"):
            try:
                current = int(raw[1:])
            except ValueError:
                current = None
        elif raw.startswith("n") and current is not None:
            found.setdefault(current, (os.path.realpath(raw[1:]), False))
    # A pid lsof said nothing about is resolved singly, preserving the exact
    # alive-versus-denied distinction the single-pid path already encodes.
    return {pid: found.get(pid) or resolve_cwd(pid) for pid in pids}


def resolve_signature_cwds(
    processes: list[tuple[Process, str]],
) -> list[tuple[Process, str]]:
    """Resolve cwd only after argv proves that a process is agent-shaped."""
    if os.environ.get("METASYSTEM_CENSUS_PROCESS_FILE"):
        return processes
    batch = resolve_cwds_batch([process.pid for process, _ in processes])
    resolved = []
    for process, runtime in processes:
        cwd, cwd_error = batch[process.pid]
        alive = not (cwd is None and not cwd_error)
        resolved.append((replace(process, cwd=cwd, cwd_error=cwd_error, alive=alive), runtime))
    return resolved


def path_below(candidate: str | Path, root: Path) -> bool:
    try:
        Path(os.path.realpath(candidate)).relative_to(root)
    except (ValueError, OSError):
        return False
    return True


_WITNESS_SEQ = 0


def _emit_census_witness(repo, verdict, generation, untracked):
    """Flight-recorder witness (plans/flight-recorder.md). Never raises.

    In-process on purpose: the census runs at fixture intervals of 50ms, and
    a per-event interpreter spawn blew that budget the day this was added.
    A direct O_APPEND write costs microseconds.
    """
    global _WITNESS_SEQ
    try:
        import json as _json
        events = [{"event": "census-verdict",
                   "summary": f"census {'FAILED' if verdict != 'SUCCESS' else 'SUCCESS'}",
                   "verdict": "FAILED" if verdict != "SUCCESS" else "SUCCESS",
                   "generation": int(generation or 0)}]
        for item in untracked[:5]:
            events.append({"event": "census-untracked",
                           "summary": "untracked process observed",
                           "observedPid": int(item.get("pid", 0)),
                           "argvSummary": str(item.get("argv", ""))[:200]})
        now = dt.datetime.now(dt.timezone.utc)
        pid = os.getpid()
        try:
            started = int(started_at(pid))
        except BaseException:
            started = 0
        path = os.path.join(str(repo), "artifacts", "agents", "events.jsonl")
        payload = b""
        for body in events:
            _WITNESS_SEQ += 1
            body.update({"schemaVersion": 1,
                         "ts": now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{now.microsecond // 1000:03d}Z",
                         "component": "census", "level": "info", "pid": pid,
                         "pidStartedAt": started, "seq": _WITNESS_SEQ})
            payload += b"\n" + _json.dumps(body, separators=(",", ":"), sort_keys=True).encode("utf-8")
        fd = os.open(path, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o644)
        try:
            os.write(fd, payload)
        finally:
            os.close(fd)
    except BaseException:
        pass


def argv_paths(argv: str, cwd: str | None) -> list[Path]:
    try:
        tokens = shlex.split(argv)
    except ValueError as error:
        raise CensusError(f"argv tokenization failed: {error}") from error
    paths: list[Path] = []
    previous_path_flag = False
    for index, token in enumerate(tokens):
        candidate: str | None = None
        if previous_path_flag:
            candidate = token
            previous_path_flag = False
        elif token in PATH_FLAGS:
            previous_path_flag = True
            continue
        elif token.startswith("-") and "=" in token:
            flag, value = token.split("=", 1)
            if flag in PATH_FLAGS:
                candidate = value
        elif token.startswith("/") or token.startswith("./") or token.startswith("../"):
            candidate = token
        if not candidate or "://" in candidate:
            continue
        path = Path(candidate)
        if not path.is_absolute():
            if cwd is None:
                continue
            path = Path(cwd) / path
        paths.append(Path(os.path.realpath(path)))
    return paths


def atomic_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=path.name + ".", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(value, handle, indent=2, sort_keys=True)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass


def iter_dicts(value: Any) -> Iterable[dict[str, Any]]:
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from iter_dicts(child)
    elif isinstance(value, list):
        for child in value:
            yield from iter_dicts(child)


def live_custody() -> list[dict[str, Any]]:
    agents = METASYSTEM_ROOT / "artifacts" / "agents"
    records: list[tuple[Path, bool]] = []
    records.extend((path, True) for path in (agents / "jobs").glob("*.json"))
    records.extend((path, True) for path in (agents / "missions" / "runners").glob("*.json"))
    records.extend((path, False) for path in (agents / "missions").glob("*/turns/*/*.json"))
    identities = []
    for path, require_status in records:
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            continue
        status = value.get("status") if isinstance(value, dict) else None
        if require_status and status not in LIVE_STATUSES:
            continue
        if not require_status and status not in LIVE_STATUSES and value.get("outcome") != "running":
            continue
        record_tag = value.get("instanceTag") if isinstance(value, dict) else None
        if not isinstance(record_tag, str) or not record_tag:
            continue
        candidates = [value] + (value.get("custodyProcesses", []) if isinstance(value, dict) else [])
        for candidate in candidates:
            if not isinstance(candidate, dict):
                continue
            pid, start, tag = candidate.get("pid"), candidate.get("pidStartedAt"), candidate.get("instanceTag")
            # Child identities belong to the same custody set only when their
            # tag joins the live record's tag. PID/start matches alone would
            # let a malformed or cross-job child entry claim another process.
            if isinstance(pid, int) and isinstance(start, int) and tag == record_tag:
                identities.append({"pid": pid, "pidStartedAt": start, "instanceTag": tag, "registry": str(path)})
    return identities


def process_is_live(process: Process, fixture_by_pid: dict[int, Process]) -> bool:
    if fixture_by_pid:
        candidate = fixture_by_pid.get(process.pid)
        return bool(candidate and candidate.alive and candidate.started == process.started)
    return identity_alive(process.pid, process.started)


def announcements(fixture_by_pid: dict[int, Process], errors: list[str]) -> list[dict[str, Any]]:
    directory = METASYSTEM_ROOT / "artifacts" / "agents" / "mains"
    directory.mkdir(parents=True, exist_ok=True)
    live: list[dict[str, Any]] = []
    by_identity: dict[tuple[int, int], tuple[Path, dict[str, Any]]] = {}
    # mainId and commandHash arrived with the one-writer change. Requiring
    # them made every announcement written before it — and every fixture's —
    # schema-invalid, which dropped those mains from the census entirely: a
    # blind census, not a strict one. They are OPTIONAL for classification;
    # authentication (which is what actually needs them) refuses on its own
    # when they are absent.
    expected = {
        "sessionId", "pid", "pidStartedAt", "pgid", "runtime",
        "instanceTag", "announcedAt",
    }
    for path in sorted(directory.glob("*.json")):
        if path.name in {"worktree-lease.json", "worktree-commit-token.json", "reaped-after-claim.json"} \
                or path.name.endswith(".protocol-cursor.json"):
            continue
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as error:
            errors.append(f"announcement-unreadable:{path.name}:{error}")
            continue
        # Base fields are REQUIRED; the one-writer identity fields (mainId,
        # commandHash) are optional extras. Exact-set equality rejected the
        # new format the moment the base set was relaxed for the old one —
        # both formats must read, or the census goes blind again.
        if not isinstance(value, dict) or not expected <= set(value) \
                or not set(value) <= expected | {"mainId", "commandHash", "ownerLineage"}:
            errors.append(f"announcement-schema:{path.name}")
            continue
        pid, start = value.get("pid"), value.get("pidStartedAt")
        main_id, digest = value.get("mainId"), value.get("commandHash")
        if not isinstance(pid, int) or not isinstance(start, int):
            errors.append(f"announcement-identity:{path.name}")
            continue
        # When the one-writer fields ARE present they must be well formed —
        # a malformed identity is a defect. Absent, the announcement is a
        # pre-change or fixture record: it classifies normally and simply
        # cannot authenticate, which authentication enforces on its own.
        if main_id is not None or digest is not None:
            if (not isinstance(main_id, str)
                    or re.fullmatch(r"main-[1-9][0-9]*-[1-9][0-9]*-[0-9a-f]{6}", main_id) is None
                    or not isinstance(digest, str)
                    or re.fullmatch(r"[0-9a-f]{64}", digest) is None):
                errors.append(f"announcement-identity:{path.name}")
                continue
        synthetic = fixture_by_pid.get(pid)
        # Fixture first, kernel second — the same precedence every other
        # identity reader uses. A simulated process table ADDS processes for a
        # census to inventory; it does not declare the rest of the machine
        # dead. Treating absence from the fixture as death deleted the
        # announcement of every real main that ran while a fixture was
        # installed, and a main whose announcement is gone classifies as a
        # delegate in its own checkout.
        alive = (
            bool(synthetic.alive and synthetic.started == start)
            if synthetic is not None
            else identity_alive(pid, start)
        )
        if not alive:
            try:
                path.unlink()
            except OSError as error:
                errors.append(f"announcement-prune:{path.name}:{error}")
            continue
        key = (pid, start)
        previous = by_identity.get(key)
        if previous is not None:
            keep, discard = (path, previous[0]) if value["announcedAt"] >= previous[1]["announcedAt"] else (previous[0], path)
            try:
                discard.unlink()
            except OSError as error:
                errors.append(f"announcement-dedupe:{discard.name}:{error}")
            if keep == previous[0]:
                continue
        value = {**value, "registry": str(path)}
        by_identity[key] = (path, value)
    live.extend(value for _, value in by_identity.values())
    return live


def run_census(repo: Path, fingerprint: str, interval: int, output: Path) -> int:
    repo = Path(os.path.realpath(repo))
    scan_started = time.monotonic()
    inventory: list[dict[str, Any]] = []
    diagnostics: list[str] = []
    errors: list[str] = []
    counts = {"CUSTODY": 0, "ANNOUNCED": 0, "UNTRACKED": 0}
    generation: int | None = None
    state_digest: str | None = None
    try:
        supervisor_identities, generation, state_digest = read_supervision_snapshot()
        verify_supervision_snapshot(supervisor_identities, errors)
    except CensusError as error:
        errors.append(f"supervision-state:{error}")
    try:
        declarations = configured_signatures()
        processes = enumerate_processes(repo)
        signature_matches = signature_processes(processes, declarations)
        resolved_processes = resolve_signature_cwds(signature_matches)
    except CensusError as error:
        errors.append(f"enumeration:{error}")
        processes = []
        declarations = {}
        resolved_processes = []
    fixture_by_pid = {process.pid: process for process in processes} if os.environ.get("METASYSTEM_CENSUS_PROCESS_FILE") else {}
    custody = live_custody()
    announced = announcements(fixture_by_pid, errors)
    for process, runtime in resolved_processes:
        if not process.alive:
            diagnostics.append(f"RACED-EXIT pid={process.pid}")
            continue
        if process.started < 0:
            errors.append(f"start-time-unreadable:{process.pid}")
            continue
        resolved_cwd = os.path.realpath(process.cwd) if process.cwd else None
        cwd_in_scope = bool(resolved_cwd and path_below(resolved_cwd, repo))
        try:
            named_paths = argv_paths(process.argv, resolved_cwd)
        except CensusError as error:
            errors.append(f"argv-unreadable:{process.pid}:{error}")
            continue
        argv_in_scope = any(path_below(path, repo) for path in named_paths)
        if process.cwd_error:
            diagnostics.append(f"UNRESOLVED-CWD pid={process.pid} argv={process.argv}")
        if not cwd_in_scope and not argv_in_scope:
            if process.cwd_error:
                errors.append(f"scope-unresolved:{process.pid}")
            continue
        custody_match = next((item for item in custody if item["pid"] == process.pid and item["pidStartedAt"] == process.started), None)
        announcement_match = next((item for item in announced if item["pid"] == process.pid and item["pidStartedAt"] == process.started), None)
        if custody_match:
            classification = "CUSTODY"
            registry = custody_match["registry"]
            tag = custody_match["instanceTag"]
        elif announcement_match:
            classification = "ANNOUNCED"
            registry = announcement_match["registry"]
            tag = announcement_match["instanceTag"]
        else:
            classification = "UNTRACKED"
            registry = "none"
            tag = None
        counts[classification] += 1
        inventory.append({
            "key": f"{registry}|{process.pid}|{process.started}",
            "class": classification,
            "registry": registry,
            "pid": process.pid,
            "pidStartedAt": process.started,
            "pgid": process.pgid,
            "runtime": runtime,
            "instanceTag": tag,
            "cwd": resolved_cwd if resolved_cwd else "UNRESOLVED-CWD",
            "scope": "cwd" if cwd_in_scope else "argv",
            "argv": process.argv,
        })
    verdict = "CENSUS-FAILED" if errors else "SUCCESS"
    _emit_census_witness(repo, verdict, generation, [
        item for item in inventory if item.get("class") == "UNTRACKED"
    ])
    completed_at = dt.datetime.now(dt.timezone.utc)
    duration_ms = round((time.monotonic() - scan_started) * 1000)
    value = {
        "schemaVersion": 2,
        "writer": "watch-background-jobs.sh",
        "verdict": verdict,
        "completedAt": completed_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "completedAtEpoch": int(completed_at.timestamp()),
        "durationMs": duration_ms,
        "intervalSec": interval,
        "fingerprint": fingerprint,
        "generation": generation,
        "stateDigest": state_digest,
        "counts": counts,
        "inventory": sorted(inventory, key=lambda item: item["pid"]),
        "diagnostics": diagnostics,
        "errors": errors,
    }
    atomic_json(output, value)
    for item in value["inventory"]:
        print(f'{item["class"]} pid={item["pid"]} start={item["pidStartedAt"]} runtime={item["runtime"]} registry={item["registry"]} argv={item["argv"]}')
    for diagnostic in diagnostics:
        print(diagnostic)
    if errors:
        print("CENSUS-FAILED " + ";".join(errors))
    return 0


def fingerprint(repo: Path) -> str:
    repo = Path(os.path.realpath(repo))
    declarations = configured_signatures()
    files = [
        METASYSTEM_ROOT / "scripts" / "agents" / "arm-supervision.sh",
        METASYSTEM_ROOT / "scripts" / "agents" / "dispatch.sh",
        METASYSTEM_ROOT / "scripts" / "agents" / "process-census.py",
        METASYSTEM_ROOT / "scripts" / "agents" / "adapters" / "runtime-common.sh",
        METASYSTEM_ROOT / "scripts" / "watch-background-jobs.sh",
    ]
    selected = [item.strip() for item in config_value(METASYSTEM_ROOT, "metasystem.runtimes").split(",") if item.strip()]
    files.extend(METASYSTEM_ROOT / "scripts" / "agents" / "adapters" / f"{runtime}.sh" for runtime in selected)
    file_hashes = {}
    for path in files:
        try:
            file_hashes[str(path.relative_to(METASYSTEM_ROOT))] = sha256(path.read_bytes()).hexdigest()
        except OSError as error:
            raise CensusError(f"fingerprint input is unavailable: {path}: {error}") from error
    relevant_config = {
        key: config_value(METASYSTEM_ROOT, key, default)
        for key, default in {
            "metasystem.runtimes": "",
            "watch.interval-sec": "60",
            "watch.stale-min": "20",
            "watch.cap-min": "180",
            "census.log-max-bytes": "1048576",
            "census.max-interval-share-percent": "50",
        }.items()
    }
    payload = {
        "repositoryScope": str(repo),
        "files": file_hashes,
        "signatures": {runtime: value[2] for runtime, value in declarations.items()},
        "config": relevant_config,
    }
    return sha256(json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def find_ancestor(_repo: Path, pid: int, runtime: str | None) -> dict[str, Any]:
    fake_ancestor = os.environ.get("METASYSTEM_FAKE_AGENT_ANCESTOR_PID")
    if fake_ancestor and runtime == "fake":
        candidate = int(fake_ancestor)
        return {"pid": candidate, "pidStartedAt": started_at(candidate), "pgid": os.getpgid(candidate), "runtime": "fake", "argv": "metasystem-fake-agent"}
    declarations = configured_signatures([runtime] if runtime else None)
    current = pid
    seen = set()
    while current > 1 and current not in seen:
        seen.add(current)
        result = subprocess.run(
            ["ps", "-p", str(current), "-o", "ppid=,pgid=,command="],
            text=True, capture_output=True, check=False,
        )
        if result.returncode != 0 or not result.stdout.strip():
            break
        match = re.match(r"\s*(\d+)\s+(\d+)\s+(.*)$", result.stdout.strip())
        if not match:
            break
        ppid, pgid, command = int(match.group(1)), int(match.group(2)), match.group(3)
        found_runtime = signature_runtime(command, declarations)
        if found_runtime:
            return {"pid": current, "pidStartedAt": started_at(current), "pgid": pgid, "runtime": found_runtime, "argv": command}
        current = ppid
    raise CensusError("no immediate agent-signature ancestor was found")


def main() -> int:
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="command", required=True)
    started = commands.add_parser("started-at")
    started.add_argument("--pid", required=True, type=int)
    identity = commands.add_parser("authentication-identity")
    identity.add_argument("--pid", required=True, type=int)
    alive = commands.add_parser("alive")
    alive.add_argument("--pid", required=True, type=int)
    alive.add_argument("--start-time", required=True, type=int)
    sig = commands.add_parser("signature-check")
    sig.add_argument("--adapter", required=True, type=Path)
    sig.add_argument("--positive", required=True)
    sig.add_argument("--lookalike", required=True)
    census = commands.add_parser("census")
    census.add_argument("--repo", required=True, type=Path)
    census.add_argument("--fingerprint", required=True)
    census.add_argument("--interval", required=True, type=int)
    census.add_argument("--output", required=True, type=Path)
    fp = commands.add_parser("fingerprint")
    fp.add_argument("--repo", required=True, type=Path)
    ancestor = commands.add_parser("find-ancestor")
    ancestor.add_argument("--repo", required=True, type=Path)
    ancestor.add_argument("--pid", required=True, type=int)
    ancestor.add_argument("--runtime")
    args = parser.parse_args()
    try:
        if args.command == "started-at":
            print(started_at(args.pid))
        elif args.command == "authentication-identity":
            print(json.dumps(authentication_identity(args.pid), separators=(",", ":")))
        elif args.command == "alive":
            return 0 if identity_alive(args.pid, args.start_time) else 1
        elif args.command == "signature-check":
            matches, excludes, _ = adapter_signatures(args.adapter)
            positive = any(grep_ere(pattern, args.positive)[0] for pattern in matches) and not any(grep_ere(pattern, args.positive)[0] for pattern in excludes)
            lookalike = any(grep_ere(pattern, args.lookalike)[0] for pattern in matches) and not any(grep_ere(pattern, args.lookalike)[0] for pattern in excludes)
            if not positive or lookalike:
                raise CensusError(f"signature positive/lookalike contract failed for {args.adapter.name}")
        elif args.command == "census":
            return run_census(args.repo, args.fingerprint, args.interval, args.output)
        elif args.command == "fingerprint":
            print(fingerprint(args.repo))
        elif args.command == "find-ancestor":
            print(json.dumps(find_ancestor(args.repo, args.pid, args.runtime), separators=(",", ":")))
    except (CensusError, OSError, ProcessLookupError, ValueError) as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
