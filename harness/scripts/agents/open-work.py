#!/usr/bin/env python3
"""Report plans that name an unblocked next step while nothing is running.

Continuation is the one part of the loop no prompt can guarantee. An agent that
ends its turn with work still named in a plan has not been disobedient, it has
simply stopped, and prose telling it not to is not a mechanism. This reporter is
the mechanism: the end-of-turn hook runs it, so stopping with open work becomes
a visible fact rather than something only a reader would notice.

Its inputs are the structured fields `plans/README.md` already mandates for
every stream, not free prose: the `- Next step:` line and the
`- Waiting on the human:` line. A plan is open work when it names a next step,
is not waiting on the human, and no agent job is in flight.
"""

from __future__ import annotations

import argparse
import json
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
    return count


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
    parser.add_argument("--repo", required=True, help="harness root")
    args = parser.parse_args()
    root = Path(args.repo).resolve()
    if not (root / "plans").is_dir():
        return 0
    for line in open_work(root):
        print(line)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
