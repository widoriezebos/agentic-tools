#!/usr/bin/env python3
"""Canonical model-key encoding shared by adapters and cap authorities."""

from __future__ import annotations

import argparse
import re


def canonical_model(name: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", name.strip().lower()).strip("-")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("name")
    args = parser.parse_args()
    print(canonical_model(args.name))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
