#!/usr/bin/env python3
"""Atomic owner for the mission-wide stop-loss ledger grammar."""

from __future__ import annotations

import argparse
import fcntl
import os
import re
import sys
import tempfile
from pathlib import Path


CLASSIFICATIONS = {
    "contract-improved",
    "falsified-continue",
    "falsified-dead-end",
    "no-progress",
    "unresolved",
    "invalid-run",
}


def atomic_write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=path.name + ".", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(text)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass


def parse(path: Path) -> tuple[int, int, list[tuple[int, str]]]:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as error:
        raise ValueError(f"cannot read mission ledger: {error}") from error
    cycle_budget = re.findall(r"^- Cycle budget:[ \t]*([1-9][0-9]*)[ \t]*$", text, re.MULTILINE)
    no_gain_budget = re.findall(r"^- No-gain budget:[ \t]*([1-9][0-9]*)[ \t]*$", text, re.MULTILINE)
    if len(cycle_budget) != 1 or len(no_gain_budget) != 1:
        raise ValueError("mission ledger must have exactly one positive Cycle budget and No-gain budget")
    headings = list(re.finditer(r"^### Cycle ([1-9][0-9]*)[ \t]*$", text, re.MULTILINE))
    cycles: list[tuple[int, str]] = []
    for index, heading in enumerate(headings):
        number = int(heading.group(1))
        if number != index + 1:
            raise ValueError("mission ledger cycle headings must be contiguous from 1")
        end = headings[index + 1].start() if index + 1 < len(headings) else len(text)
        block = text[heading.end() : end]
        matches = re.findall(r"^- Classification:[ \t]*([^\n]+)$", block, re.MULTILINE)
        if len(matches) != 1:
            raise ValueError(f"Cycle {number} must have exactly one Classification line")
        classification = re.match(r"([a-z-]+)", matches[0])
        if classification is None or classification.group(1) not in CLASSIFICATIONS:
            raise ValueError(f"Cycle {number} has an unknown classification")
        cycles.append((number, matches[0]))
    return int(cycle_budget[0]), int(no_gain_budget[0]), cycles


def locked(path: Path):
    lock_path = path.with_name(path.name + ".lock")
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    return lock_path.open("a+")


def command_init(args: argparse.Namespace) -> None:
    with locked(args.file) as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        if args.file.exists():
            raise ValueError("mission ledger already exists")
        text = (
            "# Mission Ledger\n\n"
            f"- Cycle budget: {args.cycle_budget}\n"
            f"- No-gain budget: {args.no_gain_budget}\n"
        )
        atomic_write(args.file, text)


def one_line(value: str, label: str) -> str:
    if not value or "\n" in value or "\r" in value:
        raise ValueError(f"{label} must be one non-empty line")
    return value


def command_append(args: argparse.Namespace) -> None:
    with locked(args.file) as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        _, _, cycles = parse(args.file)
        expected = len(cycles) + 1
        if args.cycle != expected:
            raise ValueError(f"next mission ledger cycle must be {expected}")
        if args.classification not in CLASSIFICATIONS:
            raise ValueError("unknown mission classification")
        sha = one_line(args.candidate_sha, "candidate sha")
        if not re.fullmatch(r"[0-9a-f]{40,64}", sha):
            raise ValueError("candidate sha must be a resolved git sha")
        observed = one_line(args.observed, "observed measurement")
        existing = args.file.read_text(encoding="utf-8").rstrip()
        entry = (
            f"\n\n### Cycle {args.cycle}\n"
            f"- Classification: {args.classification}; candidate-sha={sha}; observed={observed}\n"
        )
        atomic_write(args.file, existing + entry)


def main() -> int:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)
    init = subparsers.add_parser("init")
    init.add_argument("--file", required=True, type=Path)
    init.add_argument("--cycle-budget", required=True, type=int)
    init.add_argument("--no-gain-budget", required=True, type=int)
    append = subparsers.add_parser("append")
    append.add_argument("--file", required=True, type=Path)
    append.add_argument("--cycle", required=True, type=int)
    append.add_argument("--classification", required=True)
    append.add_argument("--candidate-sha", required=True)
    append.add_argument("--observed", required=True)
    verify = subparsers.add_parser("verify")
    verify.add_argument("--file", required=True, type=Path)
    count = subparsers.add_parser("count")
    count.add_argument("--file", required=True, type=Path)
    args = parser.parse_args()
    try:
        if args.command == "init":
            if args.cycle_budget < 1 or args.no_gain_budget < 1:
                raise ValueError("mission ledger budgets must be positive integers")
            command_init(args)
        elif args.command == "append":
            command_append(args)
        else:
            budgets = parse(args.file)
            if args.command == "count":
                print(len(budgets[2]))
            else:
                print(f"mission ledger valid: {len(budgets[2])} cycles")
    except ValueError as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
