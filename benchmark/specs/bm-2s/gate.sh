#!/usr/bin/env bash
# BM-2s mission completion gate: the builders' own self-assessment.
#
# The real grader is held out, so the mission cannot iterate against it. This
# gate answers a narrower question the builders can honestly answer themselves:
# does the thing build, do their own tests pass, and did they map every
# requirement to a test? The run stops when that is true. How right that belief
# was is exactly what the held-out grading measures afterwards, and the distance
# between the two is among the most interesting numbers this benchmark produces.
#
# v0.2 (human ruling, 2026-08-11): the metric is a COUNT, not a boolean. While
# the scaffolding proofs hold (package, jar, tests, the pinned test-command
# line), the score is the number of requirements validly mapped in
# requirements-map.json — partial maps included — so the ledger's no-gain
# accounting registers gain per landed requirement instead of all-or-nothing:
# the count gate paired with the engine's jurisdiction-aware gain, which
# covers the design phase. While any scaffolding proof fails, the count is
# unproven and the score is 0; a mission parks on red tests rather than
# debiting no-gain, so the zero cannot punish honest red phases. Completion is
# score >= 26: identical in substance to v0.1's all-checks-pass, since a full
# valid map plus green scaffolding is exactly what "done" claimed before.
#
# Gate grammar: emit the metric line, exit 0 whenever measurement ran. A
# non-zero exit means the gate itself could not measure, never that the work
# is unfinished.
set -uo pipefail
cd "$(dirname "$0")"

scaffold_ok=1
scaffold_fail() { scaffold_ok=0; printf 'self-assessment: %s\n' "$1" >&2; }

[[ -x ./mvnw ]] || scaffold_fail "no ./mvnw"
./mvnw -o -q package >/dev/null 2>&1 || scaffold_fail "./mvnw -o -q package failed"
[[ -f target/taskrun.jar ]] || scaffold_fail "target/taskrun.jar missing after package"

# Requirement 26 pins the test command exactly, so the gate need not guess it.
./mvnw -o test >/dev/null 2>&1 || scaffold_fail "./mvnw -o test failed"
grep -qxF 'Test command: ./mvnw -o test' README.md 2>/dev/null \
  || scaffold_fail "README.md lacks the exact test-command line"

# Requirement 26: every requirement mapped to at least one test identifier.
# The count of validly mapped requirements IS the metric; entries that fail
# the grammar (not a list, empty, identifiers without '#') do not count.
mapped=$(python3 - <<'PY'
import json
from pathlib import Path
count = 0
try:
    m = json.loads(Path("requirements-map.json").read_text())
except Exception:
    m = {}
if isinstance(m, dict):
    for n in range(1, 27):
        v = m.get(str(n))
        if isinstance(v, list) and v and all(isinstance(x, str) and "#" in x for x in v):
            count += 1
print(count)
PY
) || mapped=0

if (( scaffold_ok )); then
  score=$mapped
  (( mapped == 26 )) \
    || printf 'self-assessment: requirements-map.json covers %s/26 requirements\n' "$mapped" >&2
else
  score=0
  (( mapped == 0 )) \
    || printf 'self-assessment: %s requirements mapped but unproven while a scaffolding proof fails\n' "$mapped" >&2
fi

printf 'metric=self-assessment=%s\n' "$score"
exit 0
