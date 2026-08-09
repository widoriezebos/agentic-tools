#!/usr/bin/env python3
"""Own main-process identity, caller classification, and the checkout lease."""

from __future__ import annotations

import argparse
import datetime as dt
import errno
import fcntl
import hashlib
import json
import os
import re
import secrets
import signal
import subprocess
import time
import sys
import tempfile
from pathlib import Path
from typing import Any


TERMINAL = {"completed", "failed", "timeout", "cancelled"}
# What every announcement carries, in any generation.
ANNOUNCEMENT_BASE_FIELDS = {
    "sessionId",
    "pid",
    "pidStartedAt",
    "pgid",
    "runtime",
    "instanceTag",
    "announcedAt",
}
# What an announcement needs before it can authenticate anybody. An older
# record without them is not corrupt; it simply identifies no main.
ANNOUNCEMENT_IDENTITY_FIELDS = {"mainId", "commandHash"}
ANNOUNCEMENT_FIELDS = ANNOUNCEMENT_BASE_FIELDS | ANNOUNCEMENT_IDENTITY_FIELDS


def now() -> str:
    return dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def fail(message: str, status: int = 1) -> None:
    print(message, file=sys.stderr)
    raise SystemExit(status)


def atomic_json(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=path.name + ".", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
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


def load_object(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        fail(f"{path} is unreadable: {error}")
    if not isinstance(value, dict):
        fail(f"{path} is not a JSON object")
    return value


# The census helper is this script's own sibling. Resolving it against the
# TARGET root broke every identity read in any repository that does not carry
# the harness scripts — every fixture sandbox and every freshly provisioned
# target — so announcements were refused as "not a live process" and no lease
# could ever be claimed.
CENSUS_HELPER = Path(__file__).resolve().parent / "process-census.py"


def started_at(root: Path, pid: int) -> int | None:
    try:
        value = subprocess.check_output(
            [str(CENSUS_HELPER), "started-at", "--pid", str(pid)],
            text=True,
            stderr=subprocess.DEVNULL,
            timeout=2,
        ).strip()
        return int(value)
    except (OSError, ValueError, subprocess.SubprocessError):
        return None


def process_command(pid: int) -> str | None:
    # One reader for identity, always the census helper: it honours the
    # simulated process table fixtures install, while a raw ps call does not.
    # Two readers meant a main could not recognise its own announcement — it
    # classified as DELEGATE through its ancestor instead.
    identity = process_identity(None, pid)
    if identity is not None:
        return identity["command"]
    try:
        result = subprocess.run(
            ["ps", "-p", str(pid), "-o", "command="],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            timeout=2,
            check=False,
        )
    except OSError:
        return None
    command = result.stdout.rstrip("\n")
    return command if result.returncode == 0 and command else None


def process_identity(root: Path, pid: int) -> dict[str, Any] | None:
    try:
        raw = subprocess.check_output(
            [
                str(CENSUS_HELPER),
                "authentication-identity",
                "--pid",
                str(pid),
            ],
            text=True,
            stderr=subprocess.DEVNULL,
            timeout=2,
        )
        value = json.loads(raw)
    except (OSError, ValueError, subprocess.SubprocessError):
        return None
    if (
        not isinstance(value, dict)
        or value.get("pid") != pid
        or type(value.get("pidStartedAt")) is not int
        or not isinstance(value.get("command"), str)
        or not value["command"]
    ):
        return None
    return value


def parent_pid(pid: int) -> int | None:
    try:
        value = subprocess.check_output(
            ["ps", "-p", str(pid), "-o", "ppid="],
            text=True,
            stderr=subprocess.DEVNULL,
            timeout=2,
        ).strip()
        parent = int(value)
        return parent if parent > 0 and parent != pid else None
    except (OSError, ValueError, subprocess.SubprocessError):
        return None


def command_hash(command: str) -> str:
    return hashlib.sha256(command.encode()).hexdigest()


def live(root: Path, pid: Any, start: Any) -> bool:
    if type(pid) is not int or type(start) is not int or pid < 1 or start < 1:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    actual_start = started_at(root, pid)
    # The zero signal proved that some process currently owns the pid. An
    # unreadable start time cannot prove that the recorded holder died, so the
    # only safe result is alive. A readable mismatch does prove pid reuse.
    return True if actual_start is None else actual_start == start


def announcements(root: Path, strict: bool = True) -> list[tuple[Path, dict[str, Any]]]:
    result: list[tuple[Path, dict[str, Any]]] = []
    directory = root / "artifacts/agents/mains"
    for path in sorted(directory.glob("*.json")):
        if path.name.endswith(".protocol-cursor.json") or path.name in {
            "worktree-lease.json", "worktree-commit-token.json", "reaped-after-claim.json",
        }:
            continue
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            if strict:
                fail(f"caller classification refused: unreadable announcement {path.name}")
            continue
        # Same rule as the census: the base fields are required and the
        # one-writer identity fields are the only permitted extras. Exact-set
        # equality discarded every announcement of the other generation, and a
        # discarded announcement makes its own main classify as a delegate.
        if not isinstance(value, dict) or not ANNOUNCEMENT_BASE_FIELDS <= set(value):
            if strict:
                fail(f"caller classification refused: invalid announcement schema {path.name}")
            continue
        # An announcement written before the one-writer fields existed can
        # authenticate nobody, which is all it should cost: skip it. Failing
        # the whole walk on it meant ONE pre-change file in a checkout refused
        # every control-plane write in that checkout, naming the file and no
        # remedy. Malformed identity fields are a different matter -- those are
        # tampering-shaped, and they still refuse.
        if not ANNOUNCEMENT_IDENTITY_FIELDS & set(value):
            continue
        if not isinstance(value.get("mainId"), str) or not re.fullmatch(
            r"main-[1-9][0-9]*-[1-9][0-9]*-[0-9a-f]{6}", value["mainId"]
        ):
            if strict:
                fail(f"caller classification refused: invalid main identity {path.name}")
            continue
        if not isinstance(value.get("commandHash"), str) or not re.fullmatch(
            r"[0-9a-f]{64}", value["commandHash"]
        ):
            if strict:
                fail(f"caller classification refused: invalid command hash {path.name}")
            continue
        result.append((path, value))
    return result


def authenticated_announcement(
    root: Path, pid: int, records: list[tuple[Path, dict[str, Any]]]
) -> dict[str, Any] | None:
    identity = process_identity(root, pid)
    if identity is None:
        return None
    digest = command_hash(identity["command"])
    return next(
        (
            value
            for _, value in records
            if value.get("pid") == pid
            and value.get("pidStartedAt") == identity["pidStartedAt"]
            and value.get("commandHash") == digest
        ),
        None,
    )


def adapter_patterns(root: Path) -> list[tuple[str, re.Pattern[str]]]:
    result: list[tuple[str, re.Pattern[str]]] = []
    adapters = root / "scripts/agents/adapters"
    for path in sorted(adapters.glob("*.sh")):
        if path.name == "runtime-common.sh":
            continue
        try:
            lines = subprocess.check_output(
                [str(path), "signature"], text=True, stderr=subprocess.DEVNULL, timeout=3
            ).splitlines()
        except (OSError, subprocess.SubprocessError):
            fail(f"caller classification refused: signature registry failed for {path.name}")
        matches: list[str] = []
        excludes: list[str] = []
        try:
            for line in lines:
                kind, pattern = line.split(" ", 1)
                normalized = pattern.replace("[:space:]", r"\s")
                re.compile(normalized)
                (matches if kind == "match" else excludes if kind == "exclude" else []).append(normalized)
        except (ValueError, re.error):
            fail(f"caller classification refused: invalid signature registry for {path.name}")
        # Exclusion is represented as a negative lookahead around the complete argv.
        for match in matches:
            result.append(
                (
                    path.stem,
                    re.compile(
                        "^(?!.*(?:" + "|".join(excludes) + ")).*(?:"
                        + match
                        + ")"
                        if excludes
                        else ".*(?:" + match + ")"
                    ),
                )
            )
    return result


def custody_identities(root: Path) -> tuple[set[tuple[int, int]], dict[tuple[int, int], str]]:
    supervision: set[tuple[int, int]] = set()
    adapters: dict[tuple[int, int], str] = {}
    state = root / "artifacts/agents/supervision/state.json"
    if state.exists():
        try:
            value = json.loads(state.read_text(encoding="utf-8"))
            candidates = [value.get("owner")] + list((value.get("components") or {}).values())
            for item in candidates:
                if isinstance(item, dict) and type(item.get("pid")) is int and type(item.get("pidStartedAt")) is int:
                    supervision.add((item["pid"], item["pidStartedAt"]))
        except (OSError, ValueError, AttributeError):
            fail("caller classification refused: supervision state is unreadable")
    for path in (root / "artifacts/agents/jobs").glob("*.json"):
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            continue
        candidates = [value] + (value.get("custodyProcesses") or []) if isinstance(value, dict) else []
        for item in candidates:
            if isinstance(item, dict) and type(item.get("pid")) is int and type(item.get("pidStartedAt")) is int:
                job_id = value.get("jobId")
                if isinstance(job_id, str):
                    adapters[(item["pid"], item["pidStartedAt"])] = job_id
    return supervision, adapters


def classify(root: Path, caller: int) -> dict[str, Any]:
    records = announcements(root)
    own = authenticated_announcement(root, caller, records)
    if own is not None:
        return {"class": "MAIN", "mainId": own["mainId"], "announcement": own}
    patterns = adapter_patterns(root)
    supervision, adapters = custody_identities(root)
    seen: set[int] = {caller}
    current = parent_pid(caller)
    while current is not None and current not in seen:
        seen.add(current)
        announcement = authenticated_announcement(root, current, records)
        if announcement is not None:
            return {"class": "MAIN", "mainId": announcement["mainId"], "announcement": announcement}
        command = process_command(current)
        if command is not None and any(pattern.search(command) for _, pattern in patterns):
            return {"class": "DELEGATE", "pid": current}
        start = started_at(root, current)
        identity = (current, start) if start is not None else None
        if identity in supervision:
            return {"class": "SUPERVISION", "pid": current}
        if identity in adapters:
            return {"class": "ADAPTER-SUPERVISOR", "pid": current, "jobId": adapters[identity]}
        current = parent_pid(current)
    return {"class": "HUMAN"}


def lease_paths(root: Path) -> tuple[Path, Path, Path, Path]:
    mains = root / "artifacts/agents/mains"
    return (
        mains / "worktree-lease.json",
        mains / "worktree-lease.lock",
        mains / "worktree-commit-token.json",
        mains / "reaped-after-claim.json",
    )


def load_lease(root: Path, required: bool = True) -> dict[str, Any] | None:
    path = lease_paths(root)[0]
    if not path.exists():
        if required:
            fail("checkout lease is absent; start or arm an agent main first")
        return None
    value = load_object(path)
    # A lease written before ownership lineages existed is READ, never refused:
    # its holder is one process, so its lineage is its own mainId. Refusing
    # instead would make every already-claimed checkout unable to arm or
    # authorise its holder the moment this change landed. The default persists
    # on the next lease write, so the migration needs no separate step.
    if isinstance(value, dict) and "ownerLineage" not in value and isinstance(
        value.get("holderMainId"), str
    ):
        value = dict(value)
        value["ownerLineage"] = value["holderMainId"]
    required_fields = {
        "holderMainId",
        "pid",
        "pidStartedAt",
        "commandHash",
        "claimedAt",
        "renewedAt",
        "takeovers",
        "revision",
        "claimEpoch",
        "ownerLineage",
    }
    if set(value) != required_fields or type(value.get("revision")) is not int or type(value.get("claimEpoch")) is not int:
        fail("checkout lease schema is invalid")
    return value


def lock_probe_held(lock_path: Path) -> None:
    code = """import errno,fcntl,sys
f=open(sys.argv[1],'a+')
try: fcntl.flock(f.fileno(),fcntl.LOCK_EX|fcntl.LOCK_NB)
except OSError as e: raise SystemExit(0 if e.errno in (errno.EACCES,errno.EAGAIN) else 2)
raise SystemExit(1)
"""
    result = subprocess.run([sys.executable, "-c", code, str(lock_path)], timeout=3, check=False)
    if result.returncode != 0:
        fail("checkout lease claim refused: held-lock self-probe did not report would-block")


def lock_probe_released(lock_path: Path) -> None:
    code = """import fcntl,sys
f=open(sys.argv[1],'a+')
fcntl.flock(f.fileno(),fcntl.LOCK_EX|fcntl.LOCK_NB)
fcntl.flock(f.fileno(),fcntl.LOCK_UN)
"""
    result = subprocess.run([sys.executable, "-c", code, str(lock_path)], timeout=3, check=False)
    if result.returncode != 0:
        fail("checkout lease claim refused: released-lock self-probe could not acquire")


def verify_revision(current: dict[str, Any] | None, expected: int | None) -> None:
    if expected is not None and (current is None or current.get("revision") != expected):
        fail("checkout lease changed before the compare-and-swap write")


def protocol_counts(root: Path) -> dict[str, int]:
    jobs = root / "artifacts/agents/jobs"
    records: dict[str, dict[str, Any]] = {}
    for path in jobs.glob("*.json"):
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            continue
        if isinstance(value, dict) and value.get("jobId") == path.stem:
            records[path.stem] = value

    def chain(job: str) -> str | None:
        seen: set[str] = set()
        while job in records and job not in seen:
            seen.add(job)
            parent = records[job].get("parentJob")
            if parent is None:
                return job
            if not isinstance(parent, str):
                return None
            job = parent
        return None

    keys: dict[str, set[str]] = {}
    for job, value in records.items():
        error = value.get("protocolError")
        root_job = chain(job)
        if root_job and isinstance(error, dict) and isinstance(error.get("key"), str):
            keys.setdefault(root_job, set()).add(error["key"])
    return {job: len(items) for job, items in keys.items()}


def initialize_cursor(root: Path, main_id: str) -> None:
    atomic_json(
        root / "artifacts/agents/mains" / f"{main_id}.protocol-cursor.json",
        {"mainId": main_id, "counts": protocol_counts(root), "updatedAt": now()},
    )


SUPERVISION_TAG_PREFIXES = ("metasystem-supervision-",)


def is_supervision_tag(tag: str) -> bool:
    return any(tag.startswith(prefix) for prefix in SUPERVISION_TAG_PREFIXES)


def announce(args: argparse.Namespace) -> None:
    root = args.root.resolve()
    if getattr(args, "owner_lineage", None) and not valid_lineage(args.owner_lineage):
        fail("owner lineage must match [A-Za-z0-9._-]{1,128}")
    actual_start = started_at(root, args.pid)
    command = process_command(args.pid)
    if actual_start != args.start or command is None:
        fail("announcement identity is not a live, readable process")
    directory = root / "artifacts/agents/mains"
    directory.mkdir(parents=True, exist_ok=True)
    registry_lock = directory / ".registry.lock"
    with registry_lock.open("a+") as lock:
        acquire_bounded(lock, "lease")
        for path, value in announcements(root, strict=False):
            if value.get("pid") == args.pid and value.get("pidStartedAt") == args.start:
                supplied = getattr(args, "owner_lineage", None)
                stored = value.get("ownerLineage")
                if supplied and stored is not None and stored != supplied:
                    # A process does not change its logical owner mid-life.
                    # Silently preferring either value would hide a caller bug
                    # behind a guess.
                    fail(
                        f"announcement already carries owner lineage {stored}; "
                        f"refusing to replace it with {supplied}"
                    )
                if supplied and stored is None:
                    # Absent-to-present fill, the one permitted transition.
                    # The LEASE is written first (claim_for_announcement), then
                    # the announcement: the lease is what a claim reads, so
                    # writing it first leaves no window in which a sibling
                    # process is judged foreign and sweeps.
                    value = dict(value)
                    value["ownerLineage"] = supplied
                    claim_for_announcement(root, value)
                    atomic_json(path, value)
                    print(path)
                    return
                print(path)
                claim_for_announcement(root, value)
                return
        safe = re.sub(r"[^A-Za-z0-9._-]+", "-", args.session).strip("-.").lower() or "session"
        main_id = f"main-{args.start}-{args.pid}-{secrets.token_hex(3)}"
        value = {
            "sessionId": args.session,
            "mainId": main_id,
            "pid": args.pid,
            "pidStartedAt": args.start,
            "pgid": os.getpgid(args.pid),
            "runtime": args.runtime,
            "instanceTag": args.tag,
            "commandHash": command_hash(command),
            "announcedAt": now(),
        }
        if getattr(args, "owner_lineage", None):
            value["ownerLineage"] = args.owner_lineage
        path = directory / f"{safe}-{args.pid}.json"
        atomic_json(path, value)
        initialize_cursor(root, main_id)
    claim_for_announcement(root, value)
    print(path)


def inherited_protocol_total(root: Path, predecessor: str) -> int:
    counts = protocol_counts(root)
    total = sum(counts.values())
    if total:
        print(
            f"INHERITED-PROTOCOL-ERRORS predecessor={predecessor} total={total}",
            file=sys.stderr,
        )
    return total


def cleanup_stale_jobs(root: Path, epoch: int) -> None:
    jobs = root / "artifacts/agents/jobs"
    locks = root / "artifacts/agents/record-locks"
    locks.mkdir(parents=True, exist_ok=True)
    swept = 0
    for path in sorted(jobs.glob("*.json")):
        with (locks / f"{path.stem}.lock").open("a+") as record_lock:
            fcntl.flock(record_lock.fileno(), fcntl.LOCK_EX)
            try:
                value = json.loads(path.read_text(encoding="utf-8"))
            except (OSError, ValueError):
                continue
            record_epoch = value.get("claimEpoch")
            if (
                type(record_epoch) is not int
                or record_epoch >= epoch
                or value.get("status") in TERMINAL
            ):
                continue
            status = value.get("status")
            if status not in {"pending-setup", "pending", "running"}:
                continue
            pgid = value.get("pgid")
            tag = value.get("instanceTag")
            if type(pgid) is int and pgid > 1 and isinstance(tag, str):
                try:
                    listing = subprocess.check_output(
                        ["ps", "-axo", "pgid=,command="],
                        text=True,
                        stderr=subprocess.DEVNULL,
                        timeout=3,
                    )
                except (OSError, subprocess.SubprocessError):
                    fail(f"claim sweep cannot prove ownership of stale job {path.stem}")
                owned = any(
                    fields and fields[0] == str(pgid) and tag in line
                    for line in listing.splitlines()
                    if (fields := line.strip().split(None, 1))
                )
                if owned:
                    try:
                        os.killpg(pgid, signal.SIGTERM)
                    except ProcessLookupError:
                        pass
                    except PermissionError:
                        fail(f"claim sweep cannot stop stale job {path.stem}")
            value.update(
                {
                    "status": "failed",
                    "phase": "claim-sweep",
                    "error": "stale-claim-epoch",
                    "endedAt": value.get("endedAt") or now(),
                }
            )
            atomic_json(path, value)
            swept += 1
            # Post-write, under the record lock: the verdict is committed
            # state before the witness mentions it (flight-recorder D-3a).
            emit_event(root, "job-verdict", "stale-claim-epoch sweep",
                       jobId=path.stem, verdict="failed",
                       reason="stale-claim-epoch",
                       missionId=value.get("mission"))
    emit_event(root, "sweep-completed", f"epoch {epoch}", epoch=epoch,
               sweptCount=swept)


def prove_lock(lock: Any, lock_path: Path) -> None:
    acquire_bounded(lock, "lease")
    lock_probe_held(lock_path)
    fcntl.flock(lock.fileno(), fcntl.LOCK_UN)
    lock_probe_released(lock_path)
    acquire_bounded(lock, "lease")
    lock_probe_held(lock_path)


_EVENT_SEQ = 0


def emit_event(root: Path, event: str, summary: str, **fields: object) -> None:
    """Flight-recorder witness (plans/flight-recorder.md). Never raises."""
    global _EVENT_SEQ
    try:
        _EVENT_SEQ += 1
        helper = Path(__file__).resolve().parent / "emit-event.py"
        try:
            started = started_at(root, os.getpid())
        except BaseException:
            started = 0
        args = [sys.executable, str(helper), f"root={root}", "component=lease",
                f"event={event}", f"summary={summary}", f"pid={os.getpid()}",
                f"pidStartedAt={started}", f"seq={_EVENT_SEQ}"]
        args += [f"{k}={v}" for k, v in fields.items() if v is not None]
        subprocess.run(args, check=False, stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL, timeout=10)
    except BaseException:
        pass


LINEAGE_RE = re.compile(r"[A-Za-z0-9._-]{1,128}")


def valid_lineage(value: Any) -> bool:
    return isinstance(value, str) and bool(LINEAGE_RE.fullmatch(value))


def announcement_lineage(announcement: dict[str, Any]) -> str:
    """The logical writer this announcement belongs to.

    Absent means one process, one lineage: the announcement's own mainId. That
    default is what keeps an ad-hoc agent session behaving exactly as before --
    a different session succeeding it is still a foreign takeover.
    """
    value = announcement.get("ownerLineage")
    return value if valid_lineage(value) else str(announcement.get("mainId"))


def lease_lineage(lease: dict[str, Any]) -> str:
    value = lease.get("ownerLineage")
    return value if valid_lineage(value) else str(lease.get("holderMainId"))


def claim_for_announcement(root: Path, announcement: dict[str, Any]) -> None:
    lease_path, lock_path, _, stamp_path = lease_paths(root)
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    takeover = False
    predecessor = "none"
    report_inherited = False
    renewal_completed = False
    succeeds_predecessor = False
    with lock_path.open("a+") as lock:
        # Both lock-behavior probes finish before any lease state is persisted.
        # Reacquiring and re-reading after the released probe closes the race
        # with another claimant during the probe window.
        prove_lock(lock, lock_path)
        current = load_lease(root, required=False)
        if current is not None and current.get("holderMainId") != announcement["mainId"]:
            if live(root, current.get("pid"), current.get("pidStartedAt")):
                # A LIVE holder never loses the lease, whatever the lineage.
                # Letting a same-lineage claimant displace a live holder would
                # let an accidental duplicate launcher steal the checkout and
                # let siblings alternate custody.
                return
            if lease_lineage(current) == announcement_lineage(announcement):
                # The same logical writer continued in a new process -- a
                # mission's staging, resume, or re-arm succeeding its own
                # predecessor. Preserve the epoch so the predecessor's
                # in-flight jobs stay valid instead of being swept.
                succeeds_predecessor = True
            else:
                takeover = True
            predecessor = str(current.get("holderMainId"))
        elif current is not None:
            stamp = load_object(stamp_path) if stamp_path.exists() else {}
            stamp_complete = (
                stamp.get("holderMainId") == current["holderMainId"]
                and stamp.get("claimEpoch") == current["claimEpoch"]
            )
            if not stamp_complete:
                # A takeover writes the lease before sweeping so every stale
                # record can be judged against the new generation. If cleanup
                # crashed, the same holder's next announcement resumes it.
                cleanup_stale_jobs(root, current["claimEpoch"])
                atomic_json(
                    stamp_path,
                    {
                        "holderMainId": current["holderMainId"],
                        "claimEpoch": current["claimEpoch"],
                        "reapedAt": now(),
                    },
                )
                history = current.get("takeovers") or []
                if (
                    history
                    and history[-1].get("toMainId") == current["holderMainId"]
                    and history[-1].get("claimEpoch") == current["claimEpoch"]
                ):
                    predecessor = str(history[-1].get("fromMainId", "none"))
                    report_inherited = True
            renewed = dict(current)
            # D-1c: every announcement reconciles the lease it holds, so an
            # interrupted migration heals here instead of persisting.
            renewed["ownerLineage"] = announcement_lineage(announcement)
            renewed["renewedAt"] = now()
            renewed["revision"] += 1
            verify_revision(load_lease(root), current["revision"])
            atomic_json(lease_path, renewed)
            renewal_completed = True
            emit_event(root, "lease-renewed", "same-process renewal", epoch=renewed["claimEpoch"])
        if is_supervision_tag(str(announcement.get("instanceTag", ""))):
            # A supervision component is not a writer. It announces so the
            # census can see it, but claiming the checkout would steal the
            # lease from the very main that launched it — which is exactly
            # what happened: the detached owner claimed and its parent's
            # arming was then refused as OWNED-ELSEWHERE.
            renewal_completed = True
        if succeeds_predecessor and not renewal_completed:
            # Finish a predecessor's interrupted sweep BEFORE renewing. A
            # foreign takeover writes its epoch before sweeping, so a crash
            # between the two leaves a stamp naming the prior epoch. Completing
            # it here never certifies unreaped jobs, and cannot touch this
            # epoch's jobs: cleanup_stale_jobs fails only records BELOW the
            # lease epoch, and a renewal does not raise it.
            stamp = load_object(stamp_path) if stamp_path.exists() else {}
            if not (
                stamp.get("holderMainId") == current["holderMainId"]
                and stamp.get("claimEpoch") == current["claimEpoch"]
            ):
                cleanup_stale_jobs(root, int(current["claimEpoch"]))
            successor = dict(current)
            # The WHOLE holder identity moves: liveness is the pair
            # (pid, pidStartedAt), so a new pid beside the predecessor's start
            # time would make the live successor test as dead and invite the
            # very takeover this prevents.
            successor["holderMainId"] = announcement["mainId"]
            successor["pid"] = announcement["pid"]
            successor["pidStartedAt"] = announcement["pidStartedAt"]
            successor["commandHash"] = announcement["commandHash"]
            successor["ownerLineage"] = announcement_lineage(announcement)
            successor["renewedAt"] = now()
            successor["revision"] = int(current["revision"]) + 1
            # claimEpoch is preserved -- that is the whole point -- and
            # takeovers is not appended: this is not a seizure.
            verify_revision(load_lease(root), current["revision"])
            atomic_json(lease_path, successor)
            emit_event(root, "lease-renewed",
                       f"same-lineage succession from {predecessor}",
                       epoch=successor["claimEpoch"])
            atomic_json(
                stamp_path,
                {
                    "holderMainId": announcement["mainId"],
                    "claimEpoch": int(current["claimEpoch"]),
                    "reapedAt": now(),
                },
            )
            renewal_completed = True
        if not renewal_completed:
            claimed_at = now()
            history = list(current.get("takeovers", [])) if current else []
            epoch = int(current.get("claimEpoch", 0)) + 1 if current else 1
            revision = int(current.get("revision", 0)) + 1 if current else 1
            if takeover:
                history.append(
                    {
                        "fromMainId": predecessor,
                        "toMainId": announcement["mainId"],
                        "claimEpoch": epoch,
                        "takenAt": claimed_at,
                        "reason": "holder-death",
                    }
                )
            value = {
                "holderMainId": announcement["mainId"],
                "ownerLineage": announcement_lineage(announcement),
                "pid": announcement["pid"],
                "pidStartedAt": announcement["pidStartedAt"],
                "commandHash": announcement["commandHash"],
                "claimedAt": claimed_at,
                "renewedAt": claimed_at,
                "takeovers": history,
                "revision": revision,
                "claimEpoch": epoch,
            }
            verify_revision(
                load_lease(root, required=False), current.get("revision") if current else None
            )
            atomic_json(lease_path, value)
            emit_event(root, "lease-takeover" if takeover else "lease-claimed",
                       f"predecessor {predecessor}" if takeover else "fresh claim",
                       epoch=epoch,
                       **({"reason": "holder-death", "predecessor": predecessor} if takeover else {}))
            if takeover:
                cleanup_stale_jobs(root, epoch)
            atomic_json(
                stamp_path,
                {"holderMainId": announcement["mainId"], "claimEpoch": epoch, "reapedAt": now()},
            )
            report_inherited = takeover
    if report_inherited:
        inherited_protocol_total(root, predecessor)


def retire(args: argparse.Namespace) -> None:
    root = args.root.resolve()
    directory = root / "artifacts/agents/mains"
    with (directory / ".registry.lock").open("a+") as lock:
        acquire_bounded(lock, "lease")
        for path, value in announcements(root, strict=False):
            if (
                value.get("pid") == args.pid
                and value.get("pidStartedAt") == args.start
                and value.get("sessionId") == args.session
            ):
                path.unlink(missing_ok=True)


def authorize(args: argparse.Namespace) -> None:
    root = args.root.resolve()
    identity = classify(root, args.caller_pid)
    lease = load_lease(root, required=False)
    # An UNCLAIMED checkout has no holder to be elsewhere: an authenticated
    # main is its writer until someone claims, and the first gated write does
    # claim. Reporting holder=false for an unclaimed checkout refused every
    # first write in a fresh repository, which is the state every provisioned
    # target and every fixture sandbox starts in.
    identity["holder"] = bool(
        identity.get("class") == "MAIN"
        and (lease is None or identity.get("mainId") == lease.get("holderMainId"))
    )
    if lease is not None:
        identity["claimEpoch"] = lease["claimEpoch"]
        identity["revision"] = lease["revision"]
    print(json.dumps(identity, sort_keys=True))


def require_holder(args: argparse.Namespace) -> None:
    root = args.root.resolve()
    identity = classify(root, args.caller_pid)
    if identity.get("class") == "HUMAN":
        print(json.dumps({"class": "HUMAN", "holder": True, "claimEpoch": None, "mainId": None}))
        return
    if identity.get("class") in {"DELEGATE", "ADAPTER-SUPERVISOR", "SUPERVISION"}:
        # Same rule as holder_identity: internal helpers of an authorized
        # operation are not writers and are not re-gated here.
        print(json.dumps({"class": identity["class"], "holder": False,
                          "claimEpoch": None, "mainId": None}, sort_keys=True))
        return
    lease = load_lease(root, required=False)
    if lease is None:
        # An unclaimed checkout is claimable by an authenticated main: one
        # writer means first-come-first-served, not "nobody may write until
        # some other command claims first". Refusing here made every fresh
        # repository, and every fixture that dispatches before arming,
        # permanently unable to act.
        if identity.get("class") != "MAIN":
                        fail(
                f"checkout lease is absent and caller pid {args.caller_pid} is "
                f"{identity.get('class')}, not an authenticated main"
            )
        claim_for_announcement(root, identity["announcement"])
        lease = load_lease(root)
    if identity.get("class") != "MAIN" or identity.get("mainId") != lease.get("holderMainId"):
        fail(
            f"OWNED-ELSEWHERE: this checkout is held by {lease.get('holderMainId')} "
            f"(caller is {identity.get('class')} {identity.get('mainId')}); "
            "use scripts/agents/second-session.sh for an isolated writer"
        )
    stamp = load_object(lease_paths(root)[3]) if lease_paths(root)[3].exists() else {}
    if stamp.get("claimEpoch") != lease.get("claimEpoch"):
        fail("checkout lease claim sweep is incomplete for the current claim epoch")
    if args.expected_epoch is not None and lease.get("claimEpoch") != args.expected_epoch:
        fail("checkout lease claim epoch changed before the final mutation")
    print(
        json.dumps(
            {
                "class": "HOLDER",
                "holder": True,
                "claimEpoch": lease["claimEpoch"],
                "revision": lease["revision"],
                "mainId": lease["holderMainId"],
            },
            sort_keys=True,
        )
    )


def holder_identity(root: Path, caller_pid: int, expected_epoch: int | None) -> dict[str, Any]:
    identity = classify(root, caller_pid)
    if identity.get("class") == "HUMAN":
        return {"class": "HUMAN", "claimEpoch": None, "mainId": None}
    # A DELEGATE reaching this point is an internal helper of an operation the
    # holder already authorized at entry (dispatch's own __lock-owner and the
    # adapter supervisors it launches). Re-gating them would refuse work the
    # holder is mid-way through, and they cannot claim or hold anything: the
    # authority matrix keeps them out of every gated verb at ENTRY, which is
    # where the decision belongs.
    if identity.get("class") in {"DELEGATE", "ADAPTER-SUPERVISOR", "SUPERVISION"}:
        return {"class": identity["class"], "claimEpoch": expected_epoch, "mainId": None}
    lease = load_lease(root)
    if identity.get("class") != "MAIN" or identity.get("mainId") != lease.get("holderMainId"):
        fail(
            f"OWNED-ELSEWHERE: this checkout is held by {lease.get('holderMainId')}; "
            "use scripts/agents/second-session.sh for an isolated writer"
        )
    if expected_epoch is not None and lease.get("claimEpoch") != expected_epoch:
        fail("checkout lease claim epoch changed before the final mutation")
    return {"class": "HOLDER", "claimEpoch": lease["claimEpoch"], "mainId": lease["holderMainId"]}


LOCK_WAIT_SEC = float(os.environ.get("METASYSTEM_LEASE_LOCK_WAIT_SEC", "10"))


def acquire_bounded(handle, what: str) -> None:
    """Take the lease lock without ever blocking forever.

    A blocking LOCK_EX deadlocks the legitimate nested case — an arming that
    runs inside an operation already holding the lease — and an operator sees
    only a timeout with no cause. Bounded acquisition turns that into a plain
    refusal naming what to do.
    """
    deadline = time.monotonic() + LOCK_WAIT_SEC
    while True:
        try:
            fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            return
        except OSError:
            if time.monotonic() >= deadline:
                fail(
                    f"checkout lease lock is busy for {what} after {LOCK_WAIT_SEC:g}s; "
                    "another lease-gated operation holds it. If this call is nested "
                    "inside one, pass --lease-held; otherwise retry in a moment."
                )
            time.sleep(0.05)


def run_held(args: argparse.Namespace) -> None:
    root = args.root.resolve()
    initial = classify(root, args.caller_pid)
    if initial.get("class") == "HUMAN":
        result = subprocess.run(args.argv, check=False)
        raise SystemExit(result.returncode)
    lock_path = lease_paths(root)[1]
    try:
        # Create on first use: the lock file is an implementation detail, and
        # demanding that something else create it first made every first
        # lease-gated operation in a fresh checkout fail.
        lock_path.parent.mkdir(parents=True, exist_ok=True)
        lock_path.touch(exist_ok=True)
        lock = lock_path.open("r+")
    except OSError as error:
        fail(f"checkout lease lock cannot be opened: {error}")
    with lock:
        acquire_bounded(lock, "run-held")
        holder_identity(root, args.caller_pid, args.expected_epoch)
        result = subprocess.run(args.argv, check=False)
    raise SystemExit(result.returncode)


def renew(args: argparse.Namespace) -> None:
    root = args.root.resolve()
    lease_path, lock_path, _, _ = lease_paths(root)
    with lock_path.open("a+") as lock:
        acquire_bounded(lock, "renew")
        identity = classify(root, args.caller_pid)
        lease = load_lease(root)
        if identity.get("class") != "MAIN" or identity.get("mainId") != lease.get("holderMainId"):
            fail("checkout lease renewal refused: caller is not the authenticated holder")
        expected = lease["revision"]
        lease["renewedAt"] = now()
        lease["revision"] += 1
        verify_revision(load_lease(root), expected)
        atomic_json(lease_path, lease)
    print(json.dumps({"claimEpoch": lease["claimEpoch"], "revision": lease["revision"]}))


def protocol_growth(args: argparse.Namespace) -> None:
    root = args.root.resolve()
    cursor_path = root / "artifacts/agents/mains" / f"{args.main_id}.protocol-cursor.json"
    cursor = load_object(cursor_path)
    if cursor.get("mainId") != args.main_id or not isinstance(cursor.get("counts"), dict):
        fail("protocol-error cursor schema is invalid")
    counts = protocol_counts(root)
    growth = {
        chain: count - int(cursor["counts"].get(chain, 0))
        for chain, count in counts.items()
        if count > int(cursor["counts"].get(chain, 0))
    }
    total = sum(growth.values())
    message = ""
    if total:
        details = ", ".join(f"{chain}=+{count}" for chain, count in sorted(growth.items()))
        message = f"PROTOCOL-ERRORS: {total} new validation error(s) since this main's last report ({details})."
    print(json.dumps({"message": message, "counts": counts}, sort_keys=True))


def protocol_advance(args: argparse.Namespace) -> None:
    root = args.root.resolve()
    identity = classify(root, args.caller_pid)
    if identity.get("class") != "HUMAN" and not (
        identity.get("class") == "MAIN" and identity.get("mainId") == args.main_id
    ):
        fail("a main may advance only its own protocol-error cursor")
    try:
        counts = json.loads(args.counts)
    except ValueError:
        fail("protocol-error cursor counts are not JSON")
    if not isinstance(counts, dict) or any(not isinstance(key, str) or type(value) is not int for key, value in counts.items()):
        fail("protocol-error cursor counts are invalid")
    cursor_path = root / "artifacts/agents/mains" / f"{args.main_id}.protocol-cursor.json"
    lock_path = cursor_path.with_suffix(cursor_path.suffix + ".lock")
    with lock_path.open("a+") as lock:
        acquire_bounded(lock, "lease")
        current = load_object(cursor_path)
        if current.get("mainId") != args.main_id:
            fail("protocol-error cursor belongs to another main")
        merged = dict(current.get("counts", {}))
        for chain, count in counts.items():
            merged[chain] = max(int(merged.get(chain, 0)), count)
        atomic_json(cursor_path, {"mainId": args.main_id, "counts": merged, "updatedAt": now()})


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser()
    result.add_argument("--root", type=Path, required=True)
    commands = result.add_subparsers(dest="command", required=True)
    announce_parser = commands.add_parser("announce")
    announce_parser.add_argument("--session", required=True)
    announce_parser.add_argument("--pid", type=int, required=True)
    announce_parser.add_argument("--start", type=int, required=True)
    announce_parser.add_argument("--tag", required=True)
    announce_parser.add_argument("--runtime", required=True)
    announce_parser.add_argument("--owner-lineage", default=None)
    announce_parser.set_defaults(function=announce)
    retire_parser = commands.add_parser("retire")
    retire_parser.add_argument("--session", required=True)
    retire_parser.add_argument("--pid", type=int, required=True)
    retire_parser.add_argument("--start", type=int, required=True)
    retire_parser.set_defaults(function=retire)
    classify_parser = commands.add_parser("classify")
    classify_parser.add_argument("--caller-pid", type=int, required=True)
    classify_parser.set_defaults(function=authorize)
    holder_parser = commands.add_parser("require-holder")
    holder_parser.add_argument("--caller-pid", type=int, required=True)
    holder_parser.add_argument("--expected-epoch", type=int)
    holder_parser.set_defaults(function=require_holder)
    renew_parser = commands.add_parser("renew")
    renew_parser.add_argument("--caller-pid", type=int, required=True)
    renew_parser.set_defaults(function=renew)
    run_parser = commands.add_parser("run-held")
    run_parser.add_argument("--caller-pid", type=int, required=True)
    run_parser.add_argument("--expected-epoch", type=int)
    run_parser.add_argument("argv", nargs=argparse.REMAINDER)
    run_parser.set_defaults(function=run_held)
    growth_parser = commands.add_parser("protocol-growth")
    growth_parser.add_argument("--main-id", required=True)
    growth_parser.set_defaults(function=protocol_growth)
    advance_parser = commands.add_parser("protocol-advance")
    advance_parser.add_argument("--main-id", required=True)
    advance_parser.add_argument("--caller-pid", type=int, required=True)
    advance_parser.add_argument("--counts", required=True)
    advance_parser.set_defaults(function=protocol_advance)
    return result


if __name__ == "__main__":
    arguments = parser().parse_args()
    if getattr(arguments, "command", None) == "run-held":
        if arguments.argv[:1] == ["--"]:
            arguments.argv = arguments.argv[1:]
        if not arguments.argv:
            fail("run-held requires a command", 2)
    arguments.function(arguments)
