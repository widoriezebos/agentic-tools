#!/usr/bin/env python3
"""Run and record the BM-1 v0.1 calibration contract."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parent
MANIFEST = ROOT.parent / "manifest.json"
OUTPUT = ROOT / "calibration.md"


def parse_metrics(stdout: str) -> dict[str, float]:
    metrics: dict[str, float] = {}
    for line in stdout.splitlines():
        if not line.startswith("metric="):
            continue
        name, separator, value = line[len("metric="):].rpartition("=")
        if not separator or not name:
            raise RuntimeError(f"malformed metric line: {line}")
        metrics[name] = float(value)
    return metrics


def main() -> int:
    manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))
    calibration = manifest["grader"]["calibration"]
    repetitions = calibration["repetitions"]
    vectors = {name: value for name, value in calibration["probeVectors"].items() if name != "note"}
    document = [
        "# BM-1 v0.1 calibration",
        "",
        "Calibration passed. Each targeted probe failed its declared target in all "
        f"{repetitions} repetitions, every declared must-not-disturb metric remained 1, "
        "and the incomplete probe's acceptance remained strictly between 0 and 1.",
        "",
        "The probes are Java because the held-out surface is `java -jar` and the four "
        "near-correct probes must exercise process, cache, and reporting behavior directly. "
        "They share source text only to keep their identical correct behavior reviewable; "
        "every packaged probe selects a mandatory flaw and there is no correct or reference mode. "
        "The incomplete probe selects a deliberately partial mode.",
        "",
    ]
    failures: list[str] = []
    for name, vector in vectors.items():
        probe = ROOT / "probes" / name
        declaration = json.loads((probe / "probe.json").read_text(encoding="utf-8"))
        if declaration.get("name") != name or declaration.get("target") != vector.get("target"):
            failures.append(f"{name}: declaration target does not match manifest")
        if vector.get("target") is not None and declaration.get("mustNotDisturb") != vector.get("mustNotDisturb"):
            failures.append(f"{name}: declaration mustNotDisturb does not match manifest")
        document.extend([f"## {name}", "", f"Declaration: `{json.dumps(declaration, sort_keys=True)}`", ""])
        for repetition in range(1, repetitions + 1):
            completed = subprocess.run(
                [str(ROOT / "grade.sh"), str(probe)],
                cwd=ROOT,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                timeout=240,
                check=False,
            )
            if completed.returncode != 0:
                failures.append(f"{name} repetition {repetition}: measurement exit {completed.returncode}: {completed.stderr.strip()}")
                metrics: dict[str, float] = {}
            else:
                try:
                    metrics = parse_metrics(completed.stdout)
                except (RuntimeError, ValueError) as error:
                    failures.append(f"{name} repetition {repetition}: {error}")
                    metrics = {}
            target = vector.get("target")
            if target is None:
                acceptance = metrics.get("acceptance")
                if acceptance is None or not 0 < acceptance < 1:
                    failures.append(f"{name} repetition {repetition}: acceptance is not strictly between 0 and 1: {acceptance}")
            else:
                target_value = metrics.get(target)
                if target_value is None or target_value >= 1:
                    failures.append(f"{name} repetition {repetition}: target {target} did not fail: {target_value}")
                for preserved in vector["mustNotDisturb"]:
                    preserved_value = metrics.get(preserved)
                    if preserved_value != 1:
                        failures.append(f"{name} repetition {repetition}: must-not-disturb {preserved} was {preserved_value}")
            document.extend([f"### Repetition {repetition}", "", "```text", completed.stdout.rstrip(), "```", ""])
    if failures:
        for failure in failures:
            print(f"calibration failed: {failure}", file=sys.stderr)
        return 1
    OUTPUT.write_text("\n".join(document).rstrip() + "\n", encoding="utf-8")
    print(f"calibration passed: {len(vectors)} probes x {repetitions} repetitions")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
