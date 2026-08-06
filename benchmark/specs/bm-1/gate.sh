#!/usr/bin/env bash
# BM-1 mission completion gate: the builders' own self-assessment.
#
# The real grader is held out, so the mission cannot iterate against it. This
# gate answers a narrower question the builders can honestly answer themselves:
# does the thing build, do their own tests pass, and did they map every
# requirement to a test? The run stops when that is true. How right that belief
# was is exactly what the held-out grading measures afterwards, and the distance
# between the two is among the most interesting numbers this benchmark produces.
#
# Gate grammar: emit the metric line, exit 0 whenever measurement ran. A
# non-zero exit means the gate itself could not measure, never that the work
# is unfinished.
set -uo pipefail
cd "$(dirname "$0")"

ok=1
fail() { ok=0; printf 'self-assessment: %s\n' "$1" >&2; }

[[ -x ./mvnw ]] || fail "no ./mvnw"
./mvnw -o -q package >/dev/null 2>&1 || fail "./mvnw -o -q package failed"
[[ -f target/taskrun.jar ]] || fail "target/taskrun.jar missing after package"

# Requirement 26: every requirement mapped to at least one test identifier.
python3 - <<'PY' || fail "requirements-map.json missing or incomplete"
import json, sys
from pathlib import Path
p = Path("requirements-map.json")
if not p.exists():
    sys.exit(1)
try:
    m = json.loads(p.read_text())
except Exception:
    sys.exit(1)
if not isinstance(m, dict):
    sys.exit(1)
for n in range(1, 27):
    v = m.get(str(n))
    if not isinstance(v, list) or not v or not all(isinstance(x, str) and "#" in x for x in v):
        sys.exit(1)
PY

# Requirement 26 pins the test command exactly, so the gate need not guess it.
./mvnw -o test >/dev/null 2>&1 || fail "./mvnw -o test failed"
grep -qxF 'Test command: ./mvnw -o test' README.md 2>/dev/null \
  || fail "README.md lacks the exact test-command line"

printf 'metric=self-assessment=%s\n' "$ok"
exit 0
