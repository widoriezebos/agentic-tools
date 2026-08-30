#!/usr/bin/env bash
# Retired by R-21-m1 and ordered dead by R-22-m1 (Wido, 2026-08-30).
# The orchestration produced twenty-four runs and zero greens and was
# judged noise wearing a uniform; the full record lives in
# plans/battery-postmortem-and-way-out.md. The scripts as they were
# are restorable at the git tag battery-restoration-point.
echo "REFUSED-RETIRED: the milestone battery is retired (R-21-m1)." >&2
echo "remedy: run the retained direct validation instead:" >&2
echo "  bash scripts/validate-metasystem.sh" >&2
exit 3
