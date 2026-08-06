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
        result = subprocess.run(
            ["sysctl", "-n", "machdep.cpu.brand_string"],
            check=False,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
        )
        if result.returncode == 0 and result.stdout.strip():
            return result.stdout.strip()
    cpuinfo = Path("/proc/cpuinfo")
    if cpuinfo.is_file():
        for raw in cpuinfo.read_text(encoding="utf-8", errors="replace").splitlines():
            match = re.match(r"^(?:model name|Hardware)\s*:\s*(.+)$", raw)
            if match and match.group(1).strip():
                return match.group(1).strip()
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
