#!/usr/bin/env python3
"""Append one flight-recorder event. This process must NEVER fail its caller.

The stream is a witness, not an authority (plans/flight-recorder.md D-5):
verdicts come from records, liveness from the kernel, custody from the lease.
So every failure here — bad arguments, missing registry, unwritable stream,
full disk — ends in exit 0 with at most a line on stderr. The shell wrapper
adds its own `|| true`; this exit-0 rule is belt to that suspender.

Framing (D-2): one write of `\\n` + JSON, no trailing newline, capped at 4096
bytes. A torn fragment from a short write is terminated into its own
unparseable line by the NEXT writer's leading newline, so it can never make
another writer's event unparseable. No retry — a retry could interleave.
"""

from __future__ import annotations

import json
import os
import sys

CAP_BYTES = 4096
CAPS = {"component": 16, "event": 40, "id": 160, "level": 8, "payload": 256}
ID_FIELDS = ("missionId", "jobId", "turnId", "cohortId", "executionId")


def clip(value: str, cap: int) -> str:
    raw = value.encode("utf-8")
    if len(raw) <= cap:
        return value
    # Truncate on a UTF-8 boundary and mark it visibly.
    cut = raw[: cap - 1]
    while cut and (cut[-1] & 0xC0) == 0x80:
        cut = cut[:-1]
    return cut.decode("utf-8", "ignore") + "~"


def main() -> int:
    try:
        emit(dict(pair.split("=", 1) for pair in sys.argv[1:] if "=" in pair))
    except BaseException as error:  # noqa: BLE001 -- the contract IS catch-everything
        try:
            print(f"emit-event: dropped: {error}", file=sys.stderr)
        except BaseException:
            pass
    return 0


def emit(args: dict[str, str]) -> None:
    root = args.pop("root", "") or os.environ.get("METASYSTEM_HARNESS_ROOT", "")
    if not root:
        return
    now_ms = __import__("datetime").datetime.now(__import__("datetime").timezone.utc)
    event = {
        "schemaVersion": 1,
        "ts": now_ms.strftime("%Y-%m-%dT%H:%M:%S.") + f"{now_ms.microsecond // 1000:03d}Z",
        "component": clip(args.pop("component", "unknown"), CAPS["component"]),
        "event": clip(args.pop("event", "unknown"), CAPS["event"]),
        "level": clip(args.pop("level", "info"), CAPS["level"]),
        "pid": int(args.pop("pid", 0) or 0),
        "pidStartedAt": int(args.pop("pidStartedAt", 0) or 0),
        "seq": int(args.pop("seq", 1) or 1),
    }
    summary = args.pop("summary", "")
    execution = os.environ.get("METASYSTEM_EXECUTION_ID", "")
    if execution and "executionId" not in args:
        args["executionId"] = execution
    for name in ID_FIELDS:
        if args.get(name):
            event[name] = clip(args.pop(name), CAPS["id"])
    if args.get("ref"):
        event["ref"] = clip(args.pop("ref"), CAPS["id"])
    for name, value in list(args.items()):
        event[name] = clip(value, CAPS["payload"])

    # Summary gets what fits; it is required but may be empty (D-1a). Drop
    # bytes from summary first, never from required fields.
    event["summary"] = summary
    line = "\n" + json.dumps(event, separators=(",", ":"), sort_keys=True)
    overshoot = len(line.encode("utf-8")) - CAP_BYTES
    if overshoot > 0:
        event["summary"] = clip(summary, max(0, len(summary.encode("utf-8")) - overshoot - 8))
        line = "\n" + json.dumps(event, separators=(",", ":"), sort_keys=True)

    path = os.path.join(root, "artifacts", "agents", "events.jsonl")
    os.makedirs(os.path.dirname(path), exist_ok=True)
    fd = os.open(path, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o644)
    try:
        os.write(fd, line.encode("utf-8"))  # one write; no retry (D-2)
    finally:
        os.close(fd)


if __name__ == "__main__":
    sys.exit(main())
