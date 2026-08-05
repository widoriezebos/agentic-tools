#!/usr/bin/env python3
"""Check that this repository runs under the metasystem it ships.

`adopt.sh` installs the lifecycle hooks into an adopted repository. The template
never adopts itself, so for the whole of this metasystem's development its own
session-start arming, untracked-process report, stale-supervisor warning and
open-work check were inert: every one of them was covered by fixtures that call
the hook script directly, and not one had ever fired in a live session. Turns
were lost to exactly the condition the hooks exist to surface.

A metasystem whose own repository does not run under it is testing a claim it never
makes true of itself, so this failure is a suite failure.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: check-own-hooks.py <live settings> <shipped hooks>", file=sys.stderr)
        return 2
    live_path, shipped_path = Path(sys.argv[1]), Path(sys.argv[2])
    try:
        live = json.loads(live_path.read_text(encoding="utf-8"))
        shipped = json.loads(shipped_path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        print(f"cannot read hook configuration: {error}", file=sys.stderr)
        return 1

    missing = sorted(set(shipped.get("hooks", {})) - set(live.get("hooks", {})))
    if missing:
        print(f"this repository is missing its own lifecycle hooks: {missing}", file=sys.stderr)
        return 1

    flat = json.dumps(live)
    if "supervision-hook.sh" not in flat:
        print("this repository's hooks do not invoke the supervision hook", file=sys.stderr)
        return 1
    # The shipped configuration assumes the metasystem is the project root, which
    # holds in an adopted repository. Here it is vendored one level down, so the
    # commands must enter it or every hook silently no-ops.
    if "$CLAUDE_PROJECT_DIR/metasystem" not in flat:
        print(
            "this repository's hooks do not enter the vendored metasystem directory",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
