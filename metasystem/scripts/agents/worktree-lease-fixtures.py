#!/usr/bin/env python3
"""Focused lease and caller-classification proofs that do not enumerate processes."""

from __future__ import annotations

import contextlib
import fcntl
import importlib.util
import io
import json
import re
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path
from types import SimpleNamespace
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location(
    "metasystem_worktree_lease", ROOT / "scripts/agents/worktree-lease.py"
)
assert SPEC and SPEC.loader
LEASE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = LEASE
SPEC.loader.exec_module(LEASE)
PROCESS_SPEC = importlib.util.spec_from_file_location(
    "metasystem_process_census", ROOT / "scripts/agents/process-census.py"
)
assert PROCESS_SPEC and PROCESS_SPEC.loader
PROCESS_CENSUS = importlib.util.module_from_spec(PROCESS_SPEC)
sys.modules[PROCESS_SPEC.name] = PROCESS_CENSUS
PROCESS_SPEC.loader.exec_module(PROCESS_CENSUS)


def announcement(main: str, pid: int, start: int, command: str = "codex main") -> dict[str, object]:
    return {
        "sessionId": main,
        "mainId": main,
        "pid": pid,
        "pidStartedAt": start,
        "pgid": pid,
        "runtime": "codex",
        "instanceTag": main,
        "commandHash": LEASE.command_hash(command),
        "announcedAt": "2026-08-07T00:00:00Z",
    }


def read_json(path: Path) -> dict[str, object]:
    return json.loads(path.read_text(encoding="utf-8"))


def expect_refusal(callable_object, contains: str) -> None:
    error = io.StringIO()
    try:
        with contextlib.redirect_stderr(error):
            callable_object()
    except SystemExit as result:
        assert result.code != 0
    else:
        raise AssertionError(f"expected refusal containing {contains!r}")
    assert contains in error.getvalue(), error.getvalue()


def identity_and_classification_fixture() -> None:
    with tempfile.TemporaryDirectory(prefix="metasystem-identity-fixture.") as raw:
        root = Path(raw)
        args = SimpleNamespace(
            root=root, session="session", pid=42, start=100, tag="tag", runtime="codex"
        )
        claims: list[str] = []
        output = io.StringIO()
        with (
            mock.patch.object(LEASE, "started_at", return_value=100),
            mock.patch.object(LEASE, "process_command", return_value="codex main"),
            mock.patch.object(LEASE.os, "getpgid", return_value=42),
            mock.patch.object(
                LEASE,
                "claim_for_announcement",
                side_effect=lambda _root, value: claims.append(str(value["mainId"])),
            ),
            contextlib.redirect_stdout(output),
        ):
            LEASE.announce(args)
            LEASE.announce(args)
        records = LEASE.announcements(root)
        assert len(records) == 1
        assert len(claims) == 2 and claims[0] == claims[1]
        record = records[0][1]
        assert record["mainId"].startswith("main-100-42-")
        assert output.getvalue().splitlines()[0] == output.getvalue().splitlines()[1]

        with mock.patch.object(
            LEASE,
            "process_identity",
            return_value={"pid": 42, "pidStartedAt": 100, "command": "different command"},
        ):
            assert LEASE.authenticated_announcement(root, 42, records) is None
        with mock.patch.object(
            LEASE,
            "process_identity",
            return_value={"pid": 42, "pidStartedAt": 100, "command": "codex main"},
        ):
            assert LEASE.authenticated_announcement(root, 42, records) == record

        parent = {10: 20, 20: None}
        main_record = announcement("main-200-20-abcdef", 20, 200, "delegate codex")

        def classify_with(
            authenticated,
            command: str | None,
            supervision: set[tuple[int, int]],
            adapters: dict[tuple[int, int], str],
        ) -> dict[str, object]:
            with (
                mock.patch.object(LEASE, "announcements", return_value=[]),
                mock.patch.object(LEASE, "authenticated_announcement", side_effect=authenticated),
                mock.patch.object(
                    LEASE, "adapter_patterns", return_value=[("codex", re.compile("delegate"))]
                ),
                mock.patch.object(LEASE, "custody_identities", return_value=(supervision, adapters)),
                mock.patch.object(LEASE, "parent_pid", side_effect=lambda pid: parent.get(pid)),
                mock.patch.object(LEASE, "process_command", return_value=command),
                mock.patch.object(LEASE, "started_at", return_value=200),
            ):
                return LEASE.classify(root, 10)

        result = classify_with(
            lambda _root, pid, _records: main_record if pid == 20 else None,
            "delegate codex",
            {(20, 200)},
            {(20, 200): "job"},
        )
        assert result["class"] == "MAIN"
        result = classify_with(lambda *_: None, "delegate codex", {(20, 200)}, {(20, 200): "job"})
        assert result["class"] == "DELEGATE"
        result = classify_with(lambda *_: None, "ordinary shell", {(20, 200)}, {(20, 200): "job"})
        assert result["class"] == "SUPERVISION"
        result = classify_with(lambda *_: None, "ordinary shell", set(), {(20, 200): "job"})
        assert result == {"class": "ADAPTER-SUPERVISOR", "pid": 20, "jobId": "job"}
        result = classify_with(lambda *_: None, "ordinary shell", set(), {})
        assert result == {"class": "HUMAN"}


def lock_and_claim_fixture() -> None:
    with tempfile.TemporaryDirectory(prefix="metasystem-lease-fixture.") as raw:
        root = Path(raw)
        lock_path = root / "probe.lock"
        with lock_path.open("a+") as lock:
            fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
            LEASE.lock_probe_held(lock_path)
        LEASE.lock_probe_released(lock_path)

        first = announcement("main-101-11-aaaaaa", 11, 101)
        second = announcement("main-202-22-bbbbbb", 22, 202)
        with mock.patch.object(LEASE, "live", return_value=False):
            LEASE.claim_for_announcement(root, first)
        initial = LEASE.load_lease(root)
        assert initial["claimEpoch"] == 1 and initial["revision"] == 1

        with mock.patch.object(LEASE, "live", return_value=True):
            LEASE.claim_for_announcement(root, second)
        assert LEASE.load_lease(root)["holderMainId"] == first["mainId"]

        jobs = root / "artifacts/agents/jobs"
        jobs.mkdir(parents=True, exist_ok=True)
        LEASE.atomic_json(
            jobs / "pending-setup.json",
            {"jobId": "pending-setup", "status": "pending-setup", "claimEpoch": 1},
        )
        LEASE.atomic_json(
            jobs / "protocol-chain.json",
            {
                "jobId": "protocol-chain",
                "parentJob": None,
                "status": "failed",
                "protocolError": {"key": "inherited"},
            },
        )
        inherited = io.StringIO()
        with (
            mock.patch.object(LEASE, "live", return_value=False),
            contextlib.redirect_stderr(inherited),
        ):
            LEASE.claim_for_announcement(root, second)
        assert inherited.getvalue().count("INHERITED-PROTOCOL-ERRORS") == 1
        assert "total=1" in inherited.getvalue()
        claimed = LEASE.load_lease(root)
        assert claimed["holderMainId"] == second["mainId"]
        assert claimed["claimEpoch"] == 2 and claimed["revision"] == 2
        assert claimed["takeovers"] == [
            {
                "fromMainId": first["mainId"],
                "toMainId": second["mainId"],
                "claimEpoch": 2,
                "takenAt": claimed["takeovers"][0]["takenAt"],
                "reason": "holder-death",
            }
        ]
        swept = read_json(jobs / "pending-setup.json")
        assert swept["status"] == "failed" and swept["error"] == "stale-claim-epoch"
        stamp = read_json(root / "artifacts/agents/mains/reaped-after-claim.json")
        assert stamp["holderMainId"] == second["mainId"] and stamp["claimEpoch"] == 2

        previous_revision = claimed["revision"]
        repeated = io.StringIO()
        with (
            mock.patch.object(LEASE, "live", return_value=True),
            contextlib.redirect_stderr(repeated),
        ):
            LEASE.claim_for_announcement(root, second)
        assert "INHERITED-PROTOCOL-ERRORS" not in repeated.getvalue()
        renewed = LEASE.load_lease(root)
        assert renewed["claimEpoch"] == 2 and renewed["revision"] == previous_revision + 1

        with mock.patch.object(
            LEASE, "classify", return_value={"class": "MAIN", "mainId": first["mainId"]}
        ):
            expect_refusal(
                lambda: LEASE.renew(SimpleNamespace(root=root, caller_pid=11)),
                "caller is not the authenticated holder",
            )
        with mock.patch.object(
            LEASE, "classify", return_value={"class": "MAIN", "mainId": second["mainId"]}
        ):
            expect_refusal(
                lambda: LEASE.holder_identity(root, 22, 1),
                "claim epoch changed before the final mutation",
            )


def contender_fixture() -> None:
    with tempfile.TemporaryDirectory(prefix="metasystem-contender-fixture.") as raw:
        root = Path(raw)
        dead = announcement("main-101-11-aaaaaa", 11, 101)
        contenders = [
            announcement("main-202-22-bbbbbb", 22, 202),
            announcement("main-303-33-cccccc", 33, 303),
        ]
        with (
            mock.patch.object(LEASE, "live", return_value=False),
            mock.patch.object(LEASE, "lock_probe_held"),
            mock.patch.object(LEASE, "lock_probe_released"),
        ):
            LEASE.claim_for_announcement(root, dead)

        failures: list[BaseException] = []

        def contend(value: dict[str, object]) -> None:
            try:
                LEASE.claim_for_announcement(root, value)
            except BaseException as error:  # preserve thread failures for the main assertion
                failures.append(error)

        with (
            mock.patch.object(LEASE, "live", side_effect=lambda _root, pid, _start: pid != 11),
            mock.patch.object(LEASE, "lock_probe_held"),
            mock.patch.object(LEASE, "lock_probe_released"),
        ):
            threads = [threading.Thread(target=contend, args=(value,)) for value in contenders]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join()
        assert not failures, failures
        lease = LEASE.load_lease(root)
        assert lease["holderMainId"] in {value["mainId"] for value in contenders}
        assert lease["claimEpoch"] == 2
        assert len(lease["takeovers"]) == 1


def protocol_cursor_fixture() -> None:
    with tempfile.TemporaryDirectory(prefix="metasystem-protocol-cursor.") as raw:
        root = Path(raw)
        jobs = root / "artifacts/agents/jobs"
        jobs.mkdir(parents=True)
        LEASE.atomic_json(
            jobs / "chain.json",
            {"jobId": "chain", "parentJob": None, "protocolError": {"key": "first"}},
        )
        LEASE.atomic_json(
            jobs / "chain-r2.json",
            {"jobId": "chain-r2", "parentJob": "chain", "protocolError": {"key": "second"}},
        )
        assert LEASE.protocol_counts(root) == {"chain": 2}
        LEASE.initialize_cursor(root, "main-cursor")
        LEASE.atomic_json(
            jobs / "chain-r3.json",
            {"jobId": "chain-r3", "parentJob": "chain-r2", "protocolError": {"key": "third"}},
        )
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            LEASE.protocol_growth(SimpleNamespace(root=root, main_id="main-cursor"))
        growth = json.loads(output.getvalue())
        assert growth["counts"] == {"chain": 3}
        assert "1 new validation error" in growth["message"] and "chain=+1" in growth["message"]

        with (
            mock.patch.object(LEASE, "classify", return_value={"class": "HUMAN"}),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            LEASE.protocol_advance(
                SimpleNamespace(
                    root=root,
                    main_id="main-cursor",
                    caller_pid=1,
                    counts=json.dumps(growth["counts"]),
                )
            )
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            LEASE.protocol_growth(SimpleNamespace(root=root, main_id="main-cursor"))
        assert json.loads(output.getvalue())["message"] == ""


def critique_regression_fixture() -> None:
    # WC-2: an existing pid with an unreadable start time is conservatively
    # alive; only a readable mismatch proves that the recorded lifetime died.
    with (
        mock.patch.object(LEASE.os, "kill"),
        mock.patch.object(LEASE, "started_at", return_value=None),
    ):
        assert LEASE.live(Path("/unused"), 44, 123) is True
    with (
        mock.patch.object(LEASE.os, "kill"),
        mock.patch.object(LEASE, "started_at", return_value=124),
    ):
        assert LEASE.live(Path("/unused"), 44, 123) is False

    # WC-10: caller authentication consumes one combined process-table
    # identity and never performs separate start-time and command reads.
    record = announcement("main-123-44-abcdef", 44, 123, "codex main")
    combined = mock.Mock(
        return_value={"pid": 44, "pidStartedAt": 123, "command": "codex main"}
    )
    with (
        mock.patch.object(LEASE, "process_identity", combined),
        mock.patch.object(LEASE, "started_at", side_effect=AssertionError("separate start read")),
        mock.patch.object(
            LEASE, "process_command", side_effect=AssertionError("separate command read")
        ),
    ):
        assert LEASE.authenticated_announcement(Path("/unused"), 44, [(Path("a"), record)]) == record
    combined.assert_called_once_with(Path("/unused"), 44)
    process_read = mock.Mock(
        return_value=SimpleNamespace(
            returncode=0,
            stdout="Fri Aug  7 12:34:56 2026 codex main --fixture\n",
        )
    )
    with mock.patch.object(PROCESS_CENSUS.subprocess, "run", process_read):
        identity = PROCESS_CENSUS.ps_identity(44)
    assert identity["pid"] == 44 and identity["command"] == "codex main --fixture"
    process_read.assert_called_once()
    assert process_read.call_args.args[0] == [
        "ps", "-p", "44", "-o", "lstart=,command="
    ]

    # WC-5: a failed released-lock probe refuses before persisting any claim.
    with tempfile.TemporaryDirectory(prefix="metasystem-probe-refusal.") as raw:
        root = Path(raw)
        with (
            mock.patch.object(LEASE, "lock_probe_held"),
            mock.patch.object(LEASE, "lock_probe_released", side_effect=SystemExit(1)),
        ):
            try:
                LEASE.claim_for_announcement(root, announcement("main-123-44-abcdef", 44, 123))
            except SystemExit:
                pass
            else:
                raise AssertionError("released-lock probe failure did not refuse")
        assert not LEASE.lease_paths(root)[0].exists()

    # WC-4: a sweep failure after takeover leaves the new epoch unstamped, and
    # the same holder's next announce re-runs cleanup and repairs the stamp.
    with tempfile.TemporaryDirectory(prefix="metasystem-sweep-retry.") as raw:
        root = Path(raw)
        first = announcement("main-101-11-aaaaaa", 11, 101)
        second = announcement("main-202-22-bbbbbb", 22, 202)
        with mock.patch.object(LEASE, "live", return_value=False):
            LEASE.claim_for_announcement(root, first)
        failed_cleanup = mock.Mock(side_effect=SystemExit(1))
        with (
            mock.patch.object(LEASE, "live", return_value=False),
            mock.patch.object(LEASE, "cleanup_stale_jobs", failed_cleanup),
        ):
            try:
                LEASE.claim_for_announcement(root, second)
            except SystemExit:
                pass
            else:
                raise AssertionError("scripted sweep failure did not interrupt takeover")
        assert LEASE.load_lease(root)["holderMainId"] == second["mainId"]
        assert read_json(LEASE.lease_paths(root)[3])["claimEpoch"] == 1
        resumed_cleanup = mock.Mock()
        with mock.patch.object(LEASE, "cleanup_stale_jobs", resumed_cleanup):
            LEASE.claim_for_announcement(root, second)
        resumed_cleanup.assert_called_once_with(root, 2)
        repaired = read_json(LEASE.lease_paths(root)[3])
        assert repaired["holderMainId"] == second["mainId"] and repaired["claimEpoch"] == 2

    # WC-6: the sweep waits on the same per-record lock as terminal writers,
    # then re-reads and preserves a completion that won the lock first.
    with tempfile.TemporaryDirectory(prefix="metasystem-sweep-record-lock.") as raw:
        root = Path(raw)
        jobs = root / "artifacts/agents/jobs"
        locks = root / "artifacts/agents/record-locks"
        jobs.mkdir(parents=True)
        locks.mkdir(parents=True)
        record_path = jobs / "race.json"
        LEASE.atomic_json(
            record_path,
            {"jobId": "race", "status": "running", "claimEpoch": 1, "result": None},
        )
        lock_path = locks / "race.lock"
        with lock_path.open("a+") as held:
            fcntl.flock(held.fileno(), fcntl.LOCK_EX)
            process = subprocess.Popen(
                [
                    sys.executable,
                    "-c",
                    """import importlib.util,sys
from pathlib import Path
spec=importlib.util.spec_from_file_location('lease_child',sys.argv[1])
module=importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
module.cleanup_stale_jobs(Path(sys.argv[2]),2)
""",
                    str(ROOT / "scripts/agents/worktree-lease.py"),
                    str(root),
                ],
                close_fds=True,
            )
            time.sleep(0.05)
            assert process.poll() is None
            LEASE.atomic_json(
                record_path,
                {"jobId": "race", "status": "completed", "claimEpoch": 1, "result": "kept"},
            )
        assert process.wait(timeout=3) == 0
        assert read_json(record_path)["status"] == "completed"
        assert read_json(record_path)["result"] == "kept"

        LEASE.initialize_cursor(root, "main-fresh")
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            LEASE.protocol_growth(SimpleNamespace(root=root, main_id="main-fresh"))
        assert json.loads(output.getvalue())["message"] == ""


def main() -> int:
    identity_and_classification_fixture()
    lock_and_claim_fixture()
    contender_fixture()
    protocol_cursor_fixture()
    critique_regression_fixture()
    print("worktree lease fixtures: PASSED")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
