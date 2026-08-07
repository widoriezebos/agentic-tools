#!/usr/bin/env python3
"""Apply the control-plane authority matrix to one classified caller."""

from __future__ import annotations

import argparse
import json
from typing import Any


MODES = {"holder-only", "record-writer", "adapter-writer", "supervision-only"}


def authorize(mode: str, classification: dict[str, Any], job: str | None) -> None:
    caller_class = classification.get("class")
    if caller_class == "HUMAN":
        return
    if caller_class == "MAIN" and classification.get("holder") is True:
        if mode == "supervision-only":
            raise PermissionError("standing reap requires authenticated supervision custody")
        return
    if mode == "holder-only":
        raise PermissionError("control-plane write requires the authenticated lease holder")
    if caller_class == "SUPERVISION":
        if mode not in {"record-writer", "supervision-only"}:
            raise PermissionError(
                "supervision may write only its state, census, and standing-reaper transitions"
            )
        return
    if caller_class == "ADAPTER-SUPERVISOR":
        if mode not in {"record-writer", "adapter-writer"}:
            raise PermissionError("adapter supervisor is outside this control-plane authority")
        if not job or classification.get("jobId") != job:
            raise PermissionError(
                "adapter supervisor may mutate only the job named by its custody record"
            )
        return
    raise PermissionError(f"control-plane write refused for caller class {caller_class}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", required=True, choices=sorted(MODES))
    parser.add_argument("--classification", required=True)
    parser.add_argument("--job")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        classification = json.loads(args.classification)
    except ValueError as error:
        raise SystemExit(f"caller classification is not JSON: {error}")
    if not isinstance(classification, dict):
        raise SystemExit("caller classification is not an object")
    try:
        authorize(args.mode, classification, args.job)
    except PermissionError as error:
        raise SystemExit(str(error))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
