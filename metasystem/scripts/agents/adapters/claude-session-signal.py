#!/usr/bin/env python3
"""Turn Claude's SessionStart hook payload into the adapter handshake signal."""

import json
import os
import tempfile
from pathlib import Path
import sys


payload = json.load(sys.stdin)
session_id = payload.get("session_id")
if not isinstance(session_id, str) or not session_id:
    raise SystemExit("SessionStart payload has no session_id")

signal_path = Path(os.environ["METASYSTEM_CLAUDE_SESSION_SIGNAL"])
events_path = Path(os.environ["METASYSTEM_CLAUDE_EVENTS"])
signal_path.parent.mkdir(parents=True, exist_ok=True)

fd, temporary = tempfile.mkstemp(prefix=signal_path.name + ".", suffix=".tmp", dir=signal_path.parent)
try:
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(
            {
                "session_id": session_id,
                "model": payload.get("model"),
                "source": payload.get("source"),
            },
            handle,
            sort_keys=True,
        )
        handle.write("\n")
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, signal_path)
finally:
    try:
        os.unlink(temporary)
    except FileNotFoundError:
        pass

with events_path.open("a", encoding="utf-8") as handle:
    handle.write(
        json.dumps(
            {
                "type": "system",
                "subtype": "init",
                "session_id": session_id,
                "source": "SessionStart-hook",
            },
            sort_keys=True,
        )
        + "\n"
    )

# SessionStart stdout becomes runtime context, allowing the role return to
# carry the transport identity without asking the model to guess it.
print(f"Metasystem runtime session id: {session_id}")
