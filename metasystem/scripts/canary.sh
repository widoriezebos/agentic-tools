#!/usr/bin/env bash
# The canary: a cheap first verdict before the full suite
# (plans/canary-validation.md). It composes checks that already exist as
# standalone invocables — the go gate and the per-domain fixture drivers —
# so a change gets its likely-failure answer in seconds-to-minutes instead
# of the full suite's price. THE CANARY IS NOT THE VERDICT: commits still
# require the gate, and acceptance still requires the full suite; a green
# canary only says the expensive run is worth starting.
#
# BATCHING (2026-08-14): the full suite runs per BATCH, not per commit.
# Each commit gets its canary class and the gate; the full suite (on
# every host the project validates on, if more than one) closes a batch
# of 3-5 low-risk commits, or immediately after any high-risk one (kill
# paths, lock protocol, adapter turn machinery). A batch failure bisects
# with the per-commit history — every commit in it was canary- and
# gate-green, so replaying the suite at an intermediate commit isolates
# the culprit in one extra run.
#
# Usage: scripts/canary.sh <change-class> [more classes...]
# Classes:
#   go            the Go gate alone (unit+race tests, ratchet, cross-builds)
#   supervision   gate + supervision fixtures
#   dispatch      gate + conformance and delegate-caps fixtures
#   mission       gate + mission fixtures
#   lease         gate + lease-succession fixtures
#   records       gate + record-protocol and flight-recorder fixtures
#   shell         syntax sweep of every tracked script + repository audit
#   docs          repository audit alone
# Unknown classes are refused so a typo cannot masquerade as a green canary.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
cd "$root"

(($#)) || { echo "canary: name at least one change class (see the header)" >&2; exit 2; }

run() { # label, command...
  local label=$1; shift
  echo "canary[$label]: $*"
  "$@" || { echo "CANARY RED at $label — fix before spending the full suite" >&2; exit 1; }
}

gate_done=0
gate() {
  ((gate_done)) && return 0
  run go-gate bash scripts/agents/go-gate.sh
  gate_done=1
}

for class in "$@"; do
  case "$class" in
    go) gate ;;
    supervision) gate; run supervision bash scripts/agents/supervision-fixtures.sh ;;
    dispatch)
      gate
      run conformance bash scripts/agents/conformance-fixtures.sh
      run delegate-caps bash scripts/agents/delegate-caps-fixtures.sh ;;
    mission) gate; run mission bash scripts/agents/mission-fixtures.sh ;;
    lease) gate; run lease bash scripts/agents/lease-succession-fixtures.sh ;;
    records)
      gate
      run record-protocol bash scripts/agents/record-protocol-fixtures.sh
      run flight-recorder bash scripts/agents/flight-recorder-fixtures.sh ;;
    shell)
      while IFS= read -r script; do
        bash -n "$script" || { echo "CANARY RED at shell-syntax: $script" >&2; exit 1; }
      done < <(git ls-files 'scripts/*.sh' 'scripts/**/*.sh')
      echo "canary[shell]: syntax sweep clean"
      run audit bash scripts/audit-metasystem.sh . ;;
    docs) run audit bash scripts/audit-metasystem.sh . ;;
    *) echo "canary: unknown change class '$class' — refusing rather than passing vacuously" >&2; exit 2 ;;
  esac
done

echo "CANARY GREEN — not a verdict: the commit gate and the full suite remain the authority"
