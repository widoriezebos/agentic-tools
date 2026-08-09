#!/usr/bin/env bash

# Fixture timing policy:
# - expected time is configured through the fixture interval overrides below;
# - every harness cap is at least 10x the expected duration it guards;
# - this file is the only owner of harness cap values; and
# - load scaling is extra headroom, never the reason an idle run passes.

harness_fixture_base_cap() { # named harness cap
  local name=$1 base
  case "$name" in
    calibration-census) base=30 ;;
    supervision-wait)
      base=${METASYSTEM_SUPERVISION_FIXTURE_TIMEOUT_SEC:-12}
      [[ "$base" =~ ^[1-9][0-9]*$ && "$base" -ge 12 && "$base" -le 60 ]] \
        || { echo "METASYSTEM_SUPERVISION_FIXTURE_TIMEOUT_SEC must be 12..60" >&2; return 1; }
      ;;
    mission-process-wait) base=5 ;;
    mission-end-state) base=10 ;;
    agent-command)
      base=${METASYSTEM_AGENT_FIXTURE_TIMEOUT_SEC:-20}
      [[ "$base" =~ ^[1-9][0-9]*$ && "$base" -ge 20 && "$base" -le 120 ]] \
        || { echo "METASYSTEM_AGENT_FIXTURE_TIMEOUT_SEC must be an integer from 20 through 120" >&2; return 1; }
      ;;
    agent-status) base=5 ;;
    agent-cleanup) base=7 ;;
    agent-driver-stop) base=2 ;;
    adapter-handshake) base=2 ;;
    runner-git-lock) base=8 ;;
    *) echo "unknown fixture cap: $name" >&2; return 1 ;;
  esac
  printf '%s\n' "$base"
}

harness_fixture_semantic_cap() { # named product cap used as fixture input
  case "$1" in
    # Dormancy comes from the fixture job's FUTURE startedAt (2099), not
    # from an absurd cap: the cap must stay below any derivable watcher
    # ceiling or the caps interlock rightly refuses to arm over it.
    dormant-job-minutes) printf '120\n' ;;
    mission-job-minutes) printf '5\n' ;;
    mission-turn-minutes) printf '5\n' ;;
    minimum-minutes) printf '1\n' ;;
    dispatch-envelope-minutes) printf '120\n' ;;
    dispatch-over-envelope-minutes) printf '121\n' ;;
    watcher-config-minutes) printf '9\n' ;;
    watcher-nonfiring-minutes) printf '600\n' ;;
    watcher-firing-minutes) printf '5\n' ;;
    *) echo "unknown fixture semantic cap: $1" >&2; return 1 ;;
  esac
}

harness_fixture_milliseconds_to_seconds() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fixture interval must be a positive integer in milliseconds: $milliseconds" >&2; return 1; }
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
}

harness_fixture_budget_init() { # metasystem root
  local harness_root=$1 resolved calibration_cap interval_name interval_value
  calibration_cap=$(harness_fixture_base_cap calibration-census) || return 1
  if [[ -n "${METASYSTEM_FIXTURE_CAP_SCALE:-}" ]]; then
    resolved=$(python3 - "$METASYSTEM_FIXTURE_CAP_SCALE" <<'PY'
import decimal
import sys

try:
    scale = decimal.Decimal(sys.argv[1])
except decimal.InvalidOperation:
    raise SystemExit("METASYSTEM_FIXTURE_CAP_SCALE must be a decimal from 1 through 20")
if not scale.is_finite() or not decimal.Decimal("1") <= scale <= decimal.Decimal("20"):
    raise SystemExit("METASYSTEM_FIXTURE_CAP_SCALE must be a decimal from 1 through 20")
millis = int((scale * 1000).to_integral_value(rounding=decimal.ROUND_CEILING))
print(f"{scale.normalize():f} {millis}")
PY
    ) || return 1
  else
    resolved=$(python3 - "$harness_root" "$calibration_cap" <<'PY'
import math
import subprocess
import sys
import tempfile
import time
from pathlib import Path

root = Path(sys.argv[1]).resolve()
cap = int(sys.argv[2])
started = time.monotonic()
try:
    with tempfile.TemporaryDirectory(prefix="metasystem-fixture-probe.") as directory:
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
            timeout=cap,
        )
except subprocess.TimeoutExpired:
    elapsed = math.ceil(time.monotonic() - started)
    raise SystemExit(
        f"fixture calibration timed out: census probe "
        f"(elapsed: {elapsed}s; scaled cap: {cap}s)"
    )

elapsed_ms = max(1, math.ceil((time.monotonic() - started) * 1000))
scale = min(12, max(3, math.ceil(elapsed_ms / 250)))
print(f"{scale} {scale * 1000}")
PY
    ) || return 1
  fi

  read -r METASYSTEM_FIXTURE_CAP_SCALE METASYSTEM_FIXTURE_CAP_SCALE_MILLI <<EOF
$resolved
EOF
  export METASYSTEM_FIXTURE_CAP_SCALE METASYSTEM_FIXTURE_CAP_SCALE_MILLI

  : "${METASYSTEM_FIXTURE_POLL_INTERVAL_MS:=20}"
  : "${METASYSTEM_CENSUS_INTERVAL_MS:=50}"
  : "${METASYSTEM_WATCH_POLL_INTERVAL_MS:=500}"
  : "${METASYSTEM_HEARTBEAT_INTERVAL_MS:=20}"
  : "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:=20}"
  for interval_name in \
      METASYSTEM_FIXTURE_POLL_INTERVAL_MS \
      METASYSTEM_CENSUS_INTERVAL_MS \
      METASYSTEM_WATCH_POLL_INTERVAL_MS \
      METASYSTEM_HEARTBEAT_INTERVAL_MS \
      METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS; do
    interval_value=${!interval_name}
    [[ "$interval_value" =~ ^[1-9][0-9]*$ ]] \
      || { echo "$interval_name must be a positive integer in milliseconds" >&2; return 1; }
  done
  METASYSTEM_FIXTURE_POLL_INTERVAL_SEC=$(
    harness_fixture_milliseconds_to_seconds "$METASYSTEM_FIXTURE_POLL_INTERVAL_MS"
  ) || return 1
  export METASYSTEM_FIXTURE_POLL_INTERVAL_MS METASYSTEM_FIXTURE_POLL_INTERVAL_SEC \
    METASYSTEM_CENSUS_INTERVAL_MS METASYSTEM_WATCH_POLL_INTERVAL_MS \
    METASYSTEM_HEARTBEAT_INTERVAL_MS \
    METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS
}

harness_fixture_scaled_cap() { # positive integer base seconds
  local base=$1
  [[ "$base" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fixture base cap must be a positive integer: $base" >&2; return 1; }
  [[ "${METASYSTEM_FIXTURE_CAP_SCALE_MILLI:-}" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fixture cap scale is not initialized" >&2; return 1; }
  printf '%s\n' "$(( (base * METASYSTEM_FIXTURE_CAP_SCALE_MILLI + 999) / 1000 ))"
}

harness_fixture_cap() { # named harness cap
  local base
  base=$(harness_fixture_base_cap "$1") || return 1
  harness_fixture_scaled_cap "$base"
}
