#!/usr/bin/env python3
"""Report plans that name an unblocked next step while nothing is running.

Continuation is the one part of the loop no prompt can guarantee. An agent that
ends its turn with work still named in a plan has not been disobedient, it has
simply stopped, and prose telling it not to is not a mechanism. This reporter is
the mechanism: the end-of-turn hook runs it, so stopping with open work becomes
a visible fact rather than something only a reader would notice.

Its inputs are the structured fields `plans/README.md` already mandates for
every stream, not free prose: the `- Next step:` line, the
`- Waiting on the human:` line, and the `- In flight right now:` line. A plan is
open work when it names a next step, is not waiting on the human, and no agent
job is in flight.

It also reports a plan whose own account of itself contradicts the job records,
because a check that reads a stale plan reports the wrong work as confidently as
the right work, which is worse than staying quiet.

Nothing here is specific to a runtime. It reads files written by the dispatcher
and by humans, so it produces the same answer under Claude, Codex, Devin, or no
agent at all. How the answer reaches an agent differs by runtime; what the
answer is does not.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import os
import sys
import time
import re
from pathlib import Path

# A next step that says nothing is left. Anything else is real work.
SETTLED = re.compile(
    r"^(none|nothing|n/?a|done|complete[d]?|-|tbd)\b[.\s]*$", re.IGNORECASE
)
# A waiting line that names no blocker. Anything else blocks continuation.
UNBLOCKED = re.compile(r"^(none|nothing)\b", re.IGNORECASE)
IN_FLIGHT = {"pending", "running"}


def field(text: str, label: str) -> str | None:
    """Return the value of a mandated `- <label>:` line, if present."""
    match = re.search(rf"^-\s*{re.escape(label)}\s*:\s*(.*)$", text, re.MULTILINE)
    if match is None:
        return None
    return match.group(1).strip()


def jobs_in_flight(harness_root: Path) -> int:
    count = 0
    for path in (harness_root / "artifacts" / "agents" / "jobs").glob("*.json"):
        try:
            record = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            # An unreadable record is the census's problem, not this reporter's.
            continue
        if record.get("status") in IN_FLIGHT:
            count += 1
    if count == 0 and gates_running(harness_root):
        count += 1
    return count


def gates_running(harness_root: Path) -> bool:
    """The orchestrator's own gate runs are work in flight that no job record
    describes: they run as background commands of the host session. A plan
    that says "waiting for the gates" while the gates are running is accurate,
    and calling it stale teaches people to stop writing the truth into plans.

    A gate says so itself, in a marker naming its process. Asking `pgrep -f`
    instead matched anything that merely MENTIONED the gate script -- a
    wait-loop, a grep, this session's own shell -- and answered for the whole
    machine rather than for this checkout. It reported "STILL WORKING: the test
    gates" with no gate running anywhere, and, because a running gate counts as
    work in flight, a false yes silences the open-work report entirely.
    """
    # Fixtures state the answer rather than racing whatever else is on the
    # machine; the suite that runs them is itself a gate run.
    declared = os.environ.get("METASYSTEM_GATES_RUNNING")
    if declared in {"0", "1"}:
        return declared == "1"
    helper = Path(__file__).resolve().parent / "gate-run.py"
    if not helper.is_file():
        return False
    probe = subprocess.run(
        [sys.executable, str(helper), "check", "--root", str(harness_root)],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        check=False,
    )
    return probe.returncode == 0 and probe.stdout.strip() == "1"


def stale_plans(harness_root: Path) -> list[str]:
    """Report plans whose own state contradicts the job records.

    A plan that misdescribes reality makes every check reading it worse than
    useless: it reports the wrong work with the same confidence as the right
    work. This session lost a turn to exactly that, so the accuracy of the
    signal is checked alongside the signal.
    """
    live = set()
    chains: dict[str, dict] = {}
    now = time.time()
    grace = float(os.environ.get("METASYSTEM_CHAIN_GRACE_SECONDS", "5400"))
    for path in (harness_root / "artifacts" / "agents" / "jobs").glob("*.json"):
        try:
            record = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            continue
        job_id = record.get("jobId", "") or path.stem
        if record.get("status") in IN_FLIGHT:
            live.add(job_id)
        root = re.sub(r"-r[0-9]+$", "", job_id)
        entry = chains.setdefault(root, {"ids": set(), "closed": False, "newest": None})
        entry["ids"].add(job_id)
        if job_id == root and record.get("chainClosed") is True:
            entry["closed"] = True
        match = re.search(r"-r([0-9]+)$", job_id)
        rank = int(match.group(1)) if match else 1
        newest = entry["newest"]
        if newest is None or rank > newest[0]:
            try:
                mtime = path.stat().st_mtime
            except OSError:
                mtime = 0.0
            entry["newest"] = (rank, record.get("status"), mtime)

    # IL-16: an open chain is work in flight for a PLAN's purposes even while no
    # round is running. Between rounds the orchestrator is adjudicating, and the
    # false stale-plan reports that gap produced taught the operator to ignore a
    # true one. The window is bounded so an abandoned chain still goes stale,
    # and jobs_in_flight() deliberately keeps the strict view: the stop hook
    # must still refuse a turn that walks away from an open chain.
    current = set(live) | {re.sub(r"-r[0-9]+$", "", job) for job in live}
    for root, entry in chains.items():
        if entry["closed"] or entry["newest"] is None:
            continue
        rank, status, mtime = entry["newest"]
        if status == "completed" and now - mtime <= grace:
            current.add(root)
            current |= entry["ids"]

    lines = []
    for plan in sorted((harness_root / "plans").glob("*.md")):
        if plan.name == "README.md":
            continue
        try:
            text = plan.read_text(encoding="utf-8")
        except OSError:
            continue
        claim = field(text, "In flight right now")
        if claim is None or not claim or UNBLOCKED.match(claim):
            # A stream saying it has nothing running is not contradicted by some
            # other stream's job. Only a claim of its own in-flight work can be
            # wrong, and flagging every idle plan whenever anything runs would
            # make this noise rather than signal.
            continue
        name = plan.relative_to(harness_root)
        if "gate" in claim.lower() and gates_running(harness_root):
            # The claim names the gates and the gates are running: accurate.
            continue
        if not current:
            lines.append(
                f"STALE-PLAN {name}: claims work in flight while no job is running"
            )
            continue
        if not any(job and job in claim for job in current):
            lines.append(
                f"STALE-PLAN {name}: names in-flight work that is not among the "
                f"running jobs or open chains"
            )
    return lines


def open_work(harness_root: Path) -> list[str]:
    if jobs_in_flight(harness_root):
        # Work is in flight; the turn is not idle and nothing is being dropped.
        return []
    lines = []
    for plan in sorted((harness_root / "plans").glob("*.md")):
        # README.md documents the stream format and carries the field names
        # inside a fenced example; it is not itself a stream.
        if plan.name == "README.md":
            continue
        try:
            text = plan.read_text(encoding="utf-8")
        except OSError:
            continue
        step = field(text, "Next step")
        if not step or SETTLED.match(step):
            continue
        waiting = field(text, "Waiting on the human")
        if waiting and not UNBLOCKED.match(waiting):
            continue
        name = plan.relative_to(harness_root)
        lines.append(f"OPEN-WORK {name}: {step}")
    return lines


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True, help="metasystem root")
    args = parser.parse_args()
    root = Path(args.repo).resolve()
    if not (root / "plans").is_dir():
        return 0
    for line in stale_plans(root):
        print(line)
    for line in open_work(root):
        print(line)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
