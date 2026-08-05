#!/usr/bin/env python3
"""Refuse to end a turn while work named in a plan is unblocked and idle.

A report is ignorable, and was ignored: this session dropped work four times
with the open-work check already written, because a systemMessage is advice and
advice competes with an agent's sense that it has finished.

The refusal is bounded, because an unbounded one is the blocking stop-hook loop
the cross-runtime report rules out. The caller blocks only the first time a
given set of open work is seen, so a second attempt always succeeds and the
human decides what happens next.
"""

import json
import sys


def main() -> int:
    detail = sys.argv[1] if len(sys.argv) > 1 else ""
    print(json.dumps({
        "decision": "block",
        "reason": (
            "Work named in a plan is unblocked and nothing is in flight. Do it now, "
            "or record in the plan why it is blocked or waiting on the human. "
            "This refusal does not repeat for the same work.\n\n" + detail
        ),
    }, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
