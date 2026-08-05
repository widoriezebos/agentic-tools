#!/usr/bin/env bash
set -euo pipefail

# Finds a rule stated in more than one place whose statements disagree.
#
# Eight of the nine rounds of one design critique found nothing else: a rule
# corrected in the paragraph being edited while the same claim survived in a
# summary table, a condition list, a rationale, or another document. A critique
# round is the wrong instrument for that. It costs minutes and real tokens to
# notice that two sentences in one repository contradict each other, and the
# failure is not specific to any agent: anyone editing a design across several
# documents will do it.
#
# The check is deliberately narrow. It does not understand prose. It knows two
# things a plan can state in several places and get wrong in exactly one way:
#
#   1. A metric named in a table row must not also be named as removed,
#      replaced, or superseded elsewhere without that row saying so.
#   2. A term the plans declare retired, through a `RETIRED:` marker, must not
#      still be prescribed anywhere as though it were current.
#
# Everything else is left to readers, because a checker that guesses at meaning
# produces false alarms, and a check people learn to ignore is worse than none.

usage() {
  cat <<'USAGE' >&2
Usage: scripts/assert-plan-consistency.sh [--plans-dir <directory>]

Fails when a plan prescribes a term another plan has retired.

A plan retires a term by writing, anywhere in its text:

  RETIRED: <term> -- <what replaced it>

Every other plan is then checked for that term. A mention is allowed only on a
line that also carries one of: RETIRED, SUPERSEDED, "replaces", "replaced",
"no longer", or "removed", which is how a document explains a change rather
than prescribing the old rule.

Exit codes: 0 consistent; 1 a retired term is still prescribed; 2 usage.
USAGE
}

plans_dir=""
while (($#)); do
  case "$1" in
    --plans-dir) [[ $# -ge 2 ]] || { usage; exit 2; }; plans_dir=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
[[ -n "$plans_dir" ]] || plans_dir="$root/plans"
[[ -d "$plans_dir" ]] || { echo "no such plans directory: $plans_dir" >&2; exit 2; }

python3 - "$plans_dir" <<'PY'
import re
import sys
from pathlib import Path

plans = Path(sys.argv[1])
# A line that mentions a retired term while explaining the change, rather than
# prescribing it. Kept small on purpose: every word here is a way to say "this
# used to be true", and a longer list would start excusing real drift.
EXPLAINS = re.compile(
    r"RETIRED|SUPERSEDED|superseded|replaces|replaced|no longer|removed|"
    r"used to|earlier|first draft|was vacuous",
)
RETIRE = re.compile(r"^RETIRED:\s*(?P<term>.+?)\s*--\s*(?P<by>.+?)\s*$", re.MULTILINE)

retired = {}
for path in sorted(plans.glob("*.md")):
    try:
        text = path.read_text(encoding="utf-8")
    except OSError:
        continue
    for match in RETIRE.finditer(text):
        term = match.group("term").strip()
        if term:
            retired[term] = (path.name, match.group("by").strip())

violations = []
for term, (declared_in, replacement) in sorted(retired.items()):
    pattern = re.compile(re.escape(term), re.IGNORECASE)
    for path in sorted(plans.glob("*.md")):
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        for number, line in enumerate(lines, start=1):
            if not pattern.search(line):
                continue
            if line.lstrip().startswith("RETIRED:"):
                continue
            if EXPLAINS.search(line):
                continue
            violations.append(
                f"{path.name}:{number}: prescribes '{term}', retired in "
                f"{declared_in} in favour of {replacement}"
            )

if violations:
    print("plan consistency: a retired term is still prescribed", file=sys.stderr)
    for item in violations:
        print(f"  {item}", file=sys.stderr)
    print(
        "  Either state the change on that line (say it was replaced, or mark it "
        "SUPERSEDED) or bring the line up to date.",
        file=sys.stderr,
    )
    raise SystemExit(1)

print(f"plan consistency: {len(retired)} retired term(s), none prescribed")
PY

# IL-15: the same drift class the plan check covers, but across the benchmark
# artifacts, where round 4 of the artifact critique found three seam findings
# by hand that a mechanical read would have caught: a claim landing in one
# file while its contradiction survives in the other.
python3 - <<'PY'
import json
import re
import sys
from pathlib import Path

violations = []
for manifest_path in sorted(Path("benchmark/specs").glob("*/manifest.json")):
    spec_dir = manifest_path.parent
    try:
        manifest = json.loads(manifest_path.read_text())
    except ValueError as error:
        violations.append(f"{manifest_path}: unparseable: {error}")
        continue

    metrics = set(manifest.get("metrics", {}))
    deferred = set(manifest.get("deferredMetrics", {}))
    for name in sorted(metrics & deferred):
        violations.append(f"{manifest_path}: {name} is both emitted and deferred")
    for name, spec in sorted(manifest.get("metrics", {}).items()):
        if not str(spec.get("formula", "")).strip():
            violations.append(f"{manifest_path}: metric {name} has no formula")

    vectors = (manifest.get("grader", {}).get("calibration", {}).get("probeVectors", {}))
    for probe, vector in sorted(vectors.items()):
        if not isinstance(vector, dict) or "target" not in vector:
            continue
        target = vector.get("target")
        disturb = set(vector.get("mustNotDisturb", []))
        if target is not None and target not in metrics:
            violations.append(f"{manifest_path}: probe {probe} targets unknown metric {target}")
        for name in sorted(disturb - metrics):
            violations.append(f"{manifest_path}: probe {probe} protects unknown metric {name}")
        if target in disturb:
            violations.append(f"{manifest_path}: probe {probe} protects its own target")

    # The contract grammar has no guard-to-metric mapping: a guard named X must
    # emit metric=X, and each instrument must emit the metric its contract
    # names. The first draft violated both and only a delegate noticed.
    contract = manifest.get("missionContract", {})
    for kind in ("gate", "guard"):
        block = contract.get(kind, {})
        metric = block.get("metric")
        if not metric:
            continue
        if kind == "guard" and block.get("name") != metric:
            violations.append(
                f"{manifest_path}: guard named {block.get('name')} emits {metric}; "
                f"the runner requires the name to equal the metric"
            )
        command = str(block.get("command", ""))
        scripts = re.findall(r"[\w./-]+\.sh", command)
        for script in scripts:
            instrument = spec_dir / Path(script).name
            if not instrument.exists():
                violations.append(f"{manifest_path}: {kind} instrument {script} not shipped in {spec_dir}")
            elif f"metric={metric}=" not in instrument.read_text():
                violations.append(
                    f"{manifest_path}: {kind} instrument {instrument.name} never emits metric={metric}="
                )

    spec_md = spec_dir / "spec.md"
    if spec_md.exists():
        requirement_count = len(re.findall(r"^\d+\. ", spec_md.read_text(), re.M))
        formula = str(manifest.get("metrics", {}).get("requirement_coverage", {}).get("formula", ""))
        denominator = re.search(r"/\s*(\d+)", formula)
        if denominator and int(denominator.group(1)) != requirement_count:
            violations.append(
                f"{manifest_path}: requirement_coverage divides by {denominator.group(1)} "
                f"but {spec_md.name} numbers {requirement_count} requirements"
            )
        seed_spec = spec_dir / "seed" / "spec.md"
        if seed_spec.exists() and seed_spec.read_text() != spec_md.read_text():
            violations.append(f"{manifest_path.parent}: seed/spec.md has drifted from spec.md")

if violations:
    print("benchmark consistency: the artifacts contradict each other", file=sys.stderr)
    for item in violations:
        print(f"  {item}", file=sys.stderr)
    raise SystemExit(1)

count = len(list(Path("benchmark/specs").glob("*/manifest.json")))
print(f"benchmark consistency: {count} spec(s), no seams")
PY
