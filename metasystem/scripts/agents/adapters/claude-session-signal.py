#!/usr/bin/env python3
"""Forward Claude's SessionStart hook payload to the metasystem session-signal verb.

The signal logic lives in the metasystem binary (`adapter claude-session-signal`),
which the SessionStart hook runs directly. This shim stays so any caller that
still points at the script path reaches the same verb, inheriting stdin and the
METASYSTEM_CLAUDE_SESSION_SIGNAL / METASYSTEM_CLAUDE_EVENTS environment.
"""

import os

root = os.path.dirname(
    os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
)
binary = os.environ.get("METASYSTEM_BIN") or os.path.join(root, "bin", "metasystem")
os.execv(binary, [binary, "adapter", "claude-session-signal"])
