#!/usr/bin/env python3
"""Print the benchmark's local machine comparability fingerprint."""

from __future__ import annotations

import json
import os
import platform
import re
import subprocess
from pathlib import Path


def cpu_model() -> str:
    if platform.system() == "Darwin":
        # Guarded like every other source: a restricted PATH without
        # sysctl must fall through to the later sources, not crash the
        # fingerprint before its own fallbacks (verification round 2).
        try:
            result = subprocess.run(
                ["sysctl", "-n", "machdep.cpu.brand_string"],
                check=False,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.DEVNULL,
            )
        except FileNotFoundError:
            result = None
        if result is not None and result.returncode == 0 and result.stdout.strip():
            return result.stdout.strip()
    cpuinfo = Path("/proc/cpuinfo")
    if cpuinfo.is_file():
        for raw in cpuinfo.read_text(encoding="utf-8", errors="replace").splitlines():
            match = re.match(r"^(?:model name|Hardware)\s*:\s*(.+)$", raw)
            if match and match.group(1).strip():
                return match.group(1).strip()
    # aarch64 Linux guests (the acceptance VM among them) expose neither
    # "model name" nor "Hardware": /proc/cpuinfo carries only hex
    # implementer/part codes and lscpu prints a literal "-" for the model.
    # The fingerprint exists to name the machine class a cohort ran on, so
    # synthesize a stable identity from what the platform does publish
    # rather than refusing the whole benchmark on a missing marketing name.
    # lscpu may be absent (Darwin without it reaches here when sysctl
    # yields nothing) and its labels are localized — force the C locale
    # and treat a missing binary as one more absent source rather than
    # a crash (the verification round ran the unguarded version into
    # FileNotFoundError, cutting off the platform.processor() fallback).
    try:
        result = subprocess.run(
            ["lscpu"],
            check=False,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            env={**os.environ, "LC_ALL": "C"},
        )
    except FileNotFoundError:
        result = None
    vendor = ""
    if result is not None and result.returncode == 0:
        for raw in result.stdout.splitlines():
            match = re.match(r"^Model name:\s*(.+)$", raw)
            if match:
                value = match.group(1).strip()
                if value and value != "-":
                    return value
            match = re.match(r"^Vendor ID:\s*(.+)$", raw)
            if match and match.group(1).strip():
                vendor = match.group(1).strip()
    if cpuinfo.is_file():
        implementer = part = ""
        for raw in cpuinfo.read_text(encoding="utf-8", errors="replace").splitlines():
            match = re.match(r"^CPU implementer\s*:\s*(.+)$", raw)
            if match:
                implementer = match.group(1).strip()
            match = re.match(r"^CPU part\s*:\s*(.+)$", raw)
            if match:
                part = match.group(1).strip()
        if implementer:
            prefix = vendor or "unknown-vendor"
            machine = platform.machine().strip() or "unknown-arch"
            synthesized = f"{prefix} {machine} (implementer {implementer}"
            if part:
                synthesized += f", part {part}"
            return synthesized + ")"
    value = platform.processor().strip()
    if value:
        return value
    raise SystemExit("machine fingerprint refused: CPU model is unavailable")


cores = os.cpu_count()
if not isinstance(cores, int) or cores < 1:
    raise SystemExit("machine fingerprint refused: core count is unavailable")
system = platform.system().strip()
release = platform.release().strip()
if not system or not release:
    raise SystemExit("machine fingerprint refused: OS identity is unavailable")

print(json.dumps({"os": f"{system} {release}", "cpuModel": cpu_model(), "coreCount": cores}, sort_keys=True))
