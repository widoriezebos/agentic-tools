#!/usr/bin/env python3
"""Record that a gate is running, and answer whether one still is.

A gate run is work in flight that no job record describes, so the turn-end
report has to know about it: a plan that says "waiting for the gates" while
the gates are running is accurate, not stale, and an idle report during a gate
run is wrong.

Knowing it by matching process command lines does not work. `pgrep -f` matches
any process that MENTIONS the script -- a wait-loop, a grep, an editor, this
session's own shell -- and it answers for the whole machine rather than for
this checkout. Both failure directions are real: a false yes silences the
open-work check, and a gate running in someone else's checkout is not this
checkout's work.

So a gate says so itself. It writes a marker naming its process by the two
kernel facts that identify it, and the reader believes a marker only while
that exact process is alive. A marker left behind by a killed gate is pruned
by the next reader rather than believed forever.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


def marker_dir(root: Path) -> Path:
    return root / "artifacts" / "agents" / "supervision" / "gate-runs"


def started_at(pid: int) -> int | None:
    helper = Path(__file__).resolve().parent / "process-census.py"
    try:
        value = subprocess.check_output(
            [str(helper), "started-at", "--pid", str(pid)],
            text=True,
            stderr=subprocess.DEVNULL,
            timeout=5,
        ).strip()
        return int(value)
    except (OSError, ValueError, subprocess.SubprocessError):
        return None


def alive(pid: int, start: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    actual = started_at(pid)
    # An unreadable start time cannot prove the recorded process died, and a
    # readable mismatch proves the pid was reused.
    return True if actual is None else actual == start


def register(args: argparse.Namespace) -> int:
    start = started_at(args.pid)
    if start is None:
        # Without a start time the marker cannot be verified later, so it would
        # be believed forever. Better to record nothing than something unfalsifiable.
        return 0
    directory = marker_dir(args.root.resolve())
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / f"{args.pid}.json"
    path.write_text(
        json.dumps(
            {
                "pid": args.pid,
                "pidStartedAt": start,
                "gate": args.gate,
                "startedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            },
            sort_keys=True,
        )
        + "\n",
        encoding="utf-8",
    )
    print(path)
    return 0


def running(root: Path) -> bool:
    found = False
    for path in sorted(marker_dir(root).glob("*.json")):
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
            pid, start = int(value["pid"]), int(value["pidStartedAt"])
        except (OSError, ValueError, KeyError, TypeError):
            path.unlink(missing_ok=True)
            continue
        if alive(pid, start):
            found = True
        else:
            path.unlink(missing_ok=True)
    return found


def check(args: argparse.Namespace) -> int:
    print("1" if running(args.root.resolve()) else "0")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    add = commands.add_parser("register", help="record that this process is a gate run")
    add.add_argument("--root", required=True, type=Path)
    add.add_argument("--gate", required=True)
    add.add_argument("--pid", required=True, type=int)
    ask = commands.add_parser("check", help="print 1 when a gate is running here, else 0")
    ask.add_argument("--root", required=True, type=Path)
    args = parser.parse_args()
    return register(args) if args.command == "register" else check(args)


if __name__ == "__main__":
    raise SystemExit(main())
