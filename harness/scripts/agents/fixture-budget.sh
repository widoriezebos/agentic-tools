#!/usr/bin/env bash

# Owns bounded, load-scaled ceilings for process-backed harness fixtures.

harness_fixture_budget_init() { # harness root
  local harness_root=$1 resolved
  if [[ -n "${HARNESS_FIXTURE_CAP_SCALE:-}" ]]; then
    resolved=$(python3 - "$HARNESS_FIXTURE_CAP_SCALE" <<'PY'
import decimal
import sys

try:
    scale = decimal.Decimal(sys.argv[1])
except decimal.InvalidOperation:
    raise SystemExit("HARNESS_FIXTURE_CAP_SCALE must be a decimal from 0.01 through 20")
if not scale.is_finite() or not decimal.Decimal("0.01") <= scale <= decimal.Decimal("20"):
    raise SystemExit("HARNESS_FIXTURE_CAP_SCALE must be a decimal from 0.01 through 20")
millis = int((scale * 1000).to_integral_value(rounding=decimal.ROUND_CEILING))
print(f"{scale.normalize():f} {millis}")
PY
    ) || return 1
  else
    resolved=$(python3 - "$harness_root" <<'PY'
import math
import subprocess
import sys
import tempfile
import time
from pathlib import Path

root = Path(sys.argv[1]).resolve()
started = time.monotonic()
try:
    with tempfile.TemporaryDirectory(prefix="harness-fixture-probe.") as directory:
        subprocess.run(
            [
                sys.executable,
                str(root / "scripts/agents/process-census.py"),
                "census",
                "--repo", str(root),
                "--fingerprint", "fixture-budget-probe",
                "--interval", "60",
                "--output", str(Path(directory) / "census.json"),
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
            timeout=30,
        )
except subprocess.TimeoutExpired:
    elapsed = math.ceil(time.monotonic() - started)
    raise SystemExit(
        f"fixture calibration timed out: census probe "
        f"(elapsed: {elapsed}s; scaled cap: 30s)"
    )

elapsed_ms = max(1, math.ceil((time.monotonic() - started) * 1000))
scale = min(12, max(3, math.ceil(elapsed_ms / 250)))
print(f"{scale} {scale * 1000}")
PY
    ) || return 1
  fi

  read -r HARNESS_FIXTURE_CAP_SCALE HARNESS_FIXTURE_CAP_SCALE_MILLI <<EOF
$resolved
EOF
  export HARNESS_FIXTURE_CAP_SCALE HARNESS_FIXTURE_CAP_SCALE_MILLI
}

harness_fixture_scaled_cap() { # positive integer base seconds
  local base=$1
  [[ "$base" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fixture base cap must be a positive integer: $base" >&2; return 1; }
  [[ "${HARNESS_FIXTURE_CAP_SCALE_MILLI:-}" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fixture cap scale is not initialized" >&2; return 1; }
  printf '%s\n' "$(( (base * HARNESS_FIXTURE_CAP_SCALE_MILLI + 999) / 1000 ))"
}
