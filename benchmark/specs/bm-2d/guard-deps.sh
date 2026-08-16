#!/usr/bin/env bash
# BM-1 guard: requirement 24's dependency floor, checked every cycle.
#
# Orthogonal to the completion gate on purpose. A build can stay green and the
# tests can stay passing while someone quietly adds a dependency that does the
# work; this is the cheap invariant that catches it, and a guard floor stops the
# run rather than merely scoring it later.
set -uo pipefail
cd "$(dirname "$0")"
clean=1
python3 - <<'PY' || clean=0
import re, sys
from pathlib import Path
p = Path("pom.xml")
if not p.exists():
    sys.exit(1)
xml = re.sub(r"<!--.*?-->", "", p.read_text(), flags=re.S)
for dep in re.findall(r"<dependency>(.*?)</dependency>", xml, flags=re.S):
    scope = re.search(r"<scope>\s*([^<\s]+)\s*</scope>", dep)
    if scope is None or scope.group(1) not in ("test", "provided"):
        sys.exit(1)
PY
printf 'metric=no-runtime-deps=%s\n' "$clean"
exit 0
