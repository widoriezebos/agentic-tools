#!/usr/bin/env bash
set -euo pipefail

kit=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
fixture=$(mktemp -d)

cleanup() {
  case "$fixture" in
    /tmp/*|/private/tmp/*|/var/folders/*|/private/var/folders/*) rm -rf -- "$fixture" ;;
    *) echo "refusing to remove unexpected fixture path: $fixture" >&2; return 1 ;;
  esac
}
trap cleanup EXIT

repo=$fixture/repo
mkdir -p "$repo/benchmark/cases/fixture/1.0" "$repo/benchmark/cases/fixture/0.1" "$repo/benchmark/configurations/fixturecfg" "$repo/benchmark/configurations/probe" "$repo/benchmark/schemas" "$repo/metasystem/scripts"
cp "$kit/compare.sh" "$kit/compare.py" "$kit/extractor.py" "$kit/attest.sh" "$kit/pairs.py" \
  "$kit/system-fingerprint.py" "$repo/benchmark/"
cp -R "$kit/schemas/." "$repo/benchmark/schemas/"
chmod +x "$repo/benchmark/compare.sh" "$repo/benchmark/attest.sh" \
  "$repo/benchmark/system-fingerprint.py"

python3 - "$repo" <<'PY'
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
# The pair-shaped mini kit: one case in two versions (1.0 eligible, 0.1 not),
# two configurations (a capability measurement and an orchestration-health
# probe), and an alias table so a schema-1 record can still be read.
def dump(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
dump(root / "benchmark" / "cases" / "fixture" / "1.0" / "case.json",
     {"id": "fixture", "version": "1.0", "comparisonEligible": True})
dump(root / "benchmark" / "cases" / "fixture" / "0.1" / "case.json",
     {"id": "fixture", "version": "0.1", "comparisonEligible": False, "comparisonEligibleNote": "fixture 0.x is descriptive only"})
dump(root / "benchmark" / "configurations" / "fixturecfg" / "1.json",
     {"id": "fixturecfg", "version": "1", "purpose": "capability"})
dump(root / "benchmark" / "configurations" / "probe" / "1.json",
     {"id": "probe", "version": "1", "purpose": "orchestration-health"})
dump(root / "benchmark" / "aliases.json",
     {"schemaVersion": 1, "aliases": {"legacy": {"case": "fixture", "caseVersion": "1.0", "config": "fixturecfg", "configVersion": "1", "legacyVersionLabel": "1.0"}}})
(root / "metasystem" / "scripts" / "validate-metasystem.sh").write_text(
    "#!/usr/bin/env bash\nexit 0\n",
    encoding="utf-8",
)
(root / "candidate.txt").write_text("baseline\n", encoding="utf-8")
PY
chmod +x "$repo/metasystem/scripts/validate-metasystem.sh"
git -C "$repo" init -q -b main
git -C "$repo" add .
env GIT_AUTHOR_DATE=2026-08-01T00:00:00Z GIT_COMMITTER_DATE=2026-08-01T00:00:00Z \
  git -C "$repo" -c user.name=fixture -c user.email=fixture@example.invalid commit -qm baseline
baseline_sha=$(git -C "$repo" rev-parse HEAD)
# The pins every record carries: the case version's tree, the configuration's blob.
case_tree=$(git -C "$repo" rev-parse HEAD:benchmark/cases/fixture/1.0)
case_tree_01=$(git -C "$repo" rev-parse HEAD:benchmark/cases/fixture/0.1)
config_blob=$(git -C "$repo" rev-parse HEAD:benchmark/configurations/fixturecfg/1.json)
probe_blob=$(git -C "$repo" rev-parse HEAD:benchmark/configurations/probe/1.json)

mkdir -p "$repo/benchmark/results/proposals"
python3 - "$repo/benchmark/results/proposals/proposal-1.json" <<'PY'
import json
import sys
from pathlib import Path

Path(sys.argv[1]).write_text(json.dumps({
    "schemaVersion": 2,
    "id": "proposal-1",
    "proposalId": "proposal-1",
    "targetMetric": "acceptance",
    "direction": "max",
    "benchmarks": [
        {"case": "fixture", "caseVersion": "1.0", "config": "fixturecfg", "configVersion": "1"},
        {"case": "fixture", "caseVersion": "0.1", "config": "fixturecfg", "configVersion": "1"},
        {"case": "fixture", "caseVersion": "1.0", "config": "probe", "configVersion": "1"},
    ],
    "candidateBranch": "main",
    "author": "fixture",
    "date": "2026-08-02",
}, indent=2) + "\n", encoding="utf-8")
PY
git -C "$repo" add benchmark/results/proposals/proposal-1.json
env GIT_AUTHOR_DATE=2026-08-02T00:00:00Z GIT_COMMITTER_DATE=2026-08-02T00:00:00Z \
  git -C "$repo" -c user.name=fixture -c user.email=fixture@example.invalid commit -qm proposal
python3 - "$repo/candidate.txt" <<'PY'
import sys
from pathlib import Path
Path(sys.argv[1]).write_text("candidate\n", encoding="utf-8")
PY
git -C "$repo" add candidate.txt
env GIT_AUTHOR_DATE=2026-08-03T00:00:00Z GIT_COMMITTER_DATE=2026-08-03T00:00:00Z \
  git -C "$repo" -c user.name=fixture -c user.email=fixture@example.invalid commit -qm candidate
candidate_sha=$(git -C "$repo" rev-parse HEAD)

python3 - "$repo" "$baseline_sha" "$candidate_sha" "$case_tree" "$config_blob" "$case_tree_01" "$probe_blob" <<'PY'
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
baseline_sha = sys.argv[2]
candidate_sha = sys.argv[3]
case_tree, config_blob, case_tree_01, probe_blob = sys.argv[4], sys.argv[5], sys.argv[6], sys.argv[7]
results = root / "benchmark" / "results"
machine = {"os": "fixture-os", "cpuModel": "fixture-cpu", "coreCount": 4}
roster = {"host": {"runtime": "fake", "model": "host"}, "delegates": []}
fences = {
    "cycles": 2,
    "jobs": 2,
    "concurrency": 1,
    "jobCapMinutes": 10,
    "wallClockHours": 1,
    "hostTurnCapMinutes": 10,
}

def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")

def record(cohort, sha, proposal, case_version="1.0", tree=case_tree, config="fixturecfg", blob=config_blob):
    return {
        "schemaVersion": 2,
        "cohortId": cohort,
        "caseId": "fixture",
        "caseVersion": case_version,
        "caseTree": tree,
        "configId": config,
        "configVersion": "1",
        "configTree": blob,
        "measuringKitVersion": "0.1.0",
        "proposalId": proposal,
        "repetitionCount": 3,
        "machineFingerprint": machine,
        "roster": roster,
        "fences": fences,
        "candidateSha": sha,
        "measuringMetasystemSha": candidate_sha,
        "createdAt": "2026-08-06T00:00:00Z",
    }

def card(cohort, sha, index, acceptance, latency, case_version="1.0", tree=case_tree, config="fixturecfg", blob=config_blob):
    return {
        "schemaVersion": 1,
        "identity": {
            "missionId": "fixture",
            "caseId": "fixture",
            "caseVersion": case_version,
            "caseTree": tree,
            "configId": config,
            "configVersion": "1",
            "configTree": blob,
            "legacyId": None,
            "legacyVersionLabel": None,
            "measuringKitVersion": "0.1.0",
            "candidateSha": sha,
            "cohortId": cohort,
            "repetitionIndex": index,
            "repetitionCount": 3,
            "roster": roster,
            "fences": fences,
            "measuringMetasystemSha": candidate_sha,
        },
        "runValidity": {"valid": True, "reasons": [], "gates": []},
        "productMetrics": [
            {
                "name": "acceptance",
                "rawName": "acceptance",
                "measurementClass": "constraint",
                "sourceOwner": "kit",
                "value": acceptance,
                "unit": "ratio",
                "direction": "max",
                "floor": 0.75,
                "ceiling": None,
                "noiseFloor": 0.01,
                "verdict": "pass",
            },
            {
                "name": "latencySeconds",
                "rawName": "latency_seconds",
                "measurementClass": "constraint",
                "sourceOwner": "kit",
                "value": latency,
                "unit": "seconds",
                "direction": "min",
                "floor": None,
                "ceiling": 10,
                "noiseFloor": 0.5,
                "verdict": "pass",
            },
        ],
        "mechanicalBehaviorMetrics": [],
        "judgedScores": [],
        "judge": None,
        "costPerProvider": [],
        "machineFingerprint": machine,
        "watches": [],
        "gaps": [],
    }

write(results / "cohorts" / "baseline.json", record("baseline", baseline_sha, None))
write(results / "cohorts" / "candidate.json", record("candidate", candidate_sha, "proposal-1"))
for index, value in enumerate((0.80, 0.82, 0.81), 1):
    write(results / baseline_sha / "baseline" / f"{index}.json", card("baseline", baseline_sha, index, value, (8, 9, 7)[index - 1]))
for index, value in enumerate((0.90, 0.92, 0.91), 1):
    write(results / candidate_sha / "candidate" / f"{index}.json", card("candidate", candidate_sha, index, value, (7, 8, 6)[index - 1]))
# A 0.x case version (comparison-ineligible by maturity) and a health-probe
# configuration (never verdict-eligible), each as its own cohort pair.
write(results / "cohorts" / "baseline0.json", record("baseline0", baseline_sha, None, "0.1", case_tree_01))
write(results / "cohorts" / "candidate0.json", record("candidate0", candidate_sha, "proposal-1", "0.1", case_tree_01))
write(results / "cohorts" / "baselineh.json", record("baselineh", baseline_sha, None, "1.0", case_tree, "probe", probe_blob))
write(results / "cohorts" / "candidateh.json", record("candidateh", candidate_sha, "proposal-1", "1.0", case_tree, "probe", probe_blob))
for index, value in enumerate((0.80, 0.82, 0.81), 1):
    write(results / baseline_sha / "baseline0" / f"{index}.json", card("baseline0", baseline_sha, index, value, (8, 9, 7)[index - 1], "0.1", case_tree_01))
    write(results / baseline_sha / "baselineh" / f"{index}.json", card("baselineh", baseline_sha, index, value, (8, 9, 7)[index - 1], "1.0", case_tree, "probe", probe_blob))
for index, value in enumerate((0.90, 0.92, 0.91), 1):
    write(results / candidate_sha / "candidate0" / f"{index}.json", card("candidate0", candidate_sha, index, value, (7, 8, 6)[index - 1], "0.1", case_tree_01))
    write(results / candidate_sha / "candidateh" / f"{index}.json", card("candidateh", candidate_sha, index, value, (7, 8, 6)[index - 1], "1.0", case_tree, "probe", probe_blob))
# A schema-1 (legacy) record pair naming the retired spec id "legacy".
def legacy_record(cohort, sha, proposal):
    value = record(cohort, sha, proposal)
    for key in ("caseId", "caseVersion", "caseTree", "configId", "configVersion", "configTree"):
        value.pop(key)
    value.update({"schemaVersion": 1, "benchmarkSpecId": "legacy", "benchmarkSpecVersion": "1.0"})
    return value
def legacy_card(cohort, sha, index, acceptance, latency):
    value = card(cohort, sha, index, acceptance, latency)
    value["identity"].update({"caseTree": None, "configTree": None, "legacyId": "legacy", "legacyVersionLabel": "1.0"})
    return value
write(results / "cohorts" / "baselinel.json", legacy_record("baselinel", baseline_sha, None))
write(results / "cohorts" / "candidatel.json", legacy_record("candidatel", candidate_sha, "proposal-1"))
for index, value in enumerate((0.80, 0.82, 0.81), 1):
    write(results / baseline_sha / "baselinel" / f"{index}.json", legacy_card("baselinel", baseline_sha, index, value, (8, 9, 7)[index - 1]))
for index, value in enumerate((0.90, 0.92, 0.91), 1):
    write(results / candidate_sha / "candidatel" / f"{index}.json", legacy_card("candidatel", candidate_sha, index, value, (7, 8, 6)[index - 1]))
write(results / "attestations" / f"{candidate_sha}.json", {
    "schemaVersion": 1,
    "source": "local",
    "command": "metasystem/scripts/validate-metasystem.sh",
    "candidateSha": candidate_sha,
    "conclusion": "success",
    "timestamp": "2026-08-05T00:00:00Z",
    "machineFingerprint": machine,
})
PY

compare=$repo/benchmark/compare.sh
"$compare" baseline candidate >/dev/null
verdict=$repo/benchmark/results/compares/baseline-vs-candidate.json
python3 - "$verdict" <<'PY'
import json
import sys
value = json.load(open(sys.argv[1], encoding="utf-8"))
assert value["verdict"] == "improved", value
assert value["comparisonEligible"] is True
assert value["judgedScoresUsed"] is False
metrics = {item["metric"]: item for item in value["metrics"]}
assert metrics["product.acceptance"]["baselineMedian"] == 0.81
assert metrics["product.acceptance"]["candidateMedian"] == 0.91
assert round(metrics["product.acceptance"]["delta"], 2) == 0.10
assert metrics["product.acceptance"]["verdict"] == "improved"
assert metrics["product.latency_seconds"]["verdict"] == "improved"
PY
cp "$verdict" "$fixture/full-verdict.json"

# Equal within each cohort, unequal across cohorts: the tuple mismatch must be
# refused before arithmetic.
python3 - "$repo/benchmark/results" "$candidate_sha" <<'PY'
import json
import sys
from pathlib import Path
root = Path(sys.argv[1])
sha = sys.argv[2]
for path in [root / "cohorts" / "candidate.json", *sorted((root / sha / "candidate").glob("*.json"))]:
    value = json.loads(path.read_text(encoding="utf-8"))
    machine = value["machineFingerprint"]
    machine["coreCount"] = 8
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
if "$compare" baseline candidate >"$fixture/tuple.out" 2>"$fixture/tuple.err"; then
  echo "compare fixture: tuple mismatch was accepted" >&2
  exit 1
fi
grep -q 'comparability tuple mismatch' "$fixture/tuple.err" \
  || { echo "compare fixture: tuple refusal was not specific" >&2; exit 1; }
python3 - "$repo/benchmark/results" "$candidate_sha" <<'PY'
import json
import sys
from pathlib import Path
root = Path(sys.argv[1])
sha = sys.argv[2]
for path in [root / "cohorts" / "candidate.json", *sorted((root / sha / "candidate").glob("*.json"))]:
    value = json.loads(path.read_text(encoding="utf-8"))
    value["machineFingerprint"]["coreCount"] = 4
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bad_card=$repo/benchmark/results/$candidate_sha/candidate/1.json
cp "$bad_card" "$fixture/card.json"
python3 - "$bad_card" <<'PY'
import json
import sys
from pathlib import Path
path = Path(sys.argv[1])
value = json.loads(path.read_text(encoding="utf-8"))
del value["judgedScores"]
path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
if "$compare" baseline candidate >"$fixture/schema.out" 2>"$fixture/schema.err"; then
  echo "compare fixture: invalid scorecard was accepted" >&2
  exit 1
fi
grep -q 'violates scorecard.schema.json' "$fixture/schema.err" \
  || { echo "compare fixture: schema refusal was not specific" >&2; exit 1; }
cp "$fixture/card.json" "$bad_card"

attestation=$repo/benchmark/results/attestations/$candidate_sha.json
mv "$attestation" "$fixture/attestation.json"
if "$compare" baseline candidate >"$fixture/attestation.out" 2>"$fixture/attestation.err"; then
  echo "compare fixture: missing attestation was accepted" >&2
  exit 1
fi
grep -q 'candidate attestation is missing' "$fixture/attestation.err" \
  || { echo "compare fixture: attestation refusal was not specific" >&2; exit 1; }
mv "$fixture/attestation.json" "$attestation"

# Repoint only the candidate cohort to the pre-proposal baseline commit. Its
# scorecards remain well formed, but the proposal is no longer an ancestor.
candidate_dir=$repo/benchmark/results/$candidate_sha/candidate
baseline_candidate_dir=$repo/benchmark/results/$baseline_sha/candidate
mkdir -p "$(dirname "$baseline_candidate_dir")"
mv "$candidate_dir" "$baseline_candidate_dir"
python3 - "$repo/benchmark/results/cohorts/candidate.json" "$baseline_candidate_dir" "$baseline_sha" <<'PY'
import json
import sys
from pathlib import Path
record = Path(sys.argv[1])
directory = Path(sys.argv[2])
sha = sys.argv[3]
value = json.loads(record.read_text(encoding="utf-8"))
value["candidateSha"] = sha
record.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
for path in directory.glob("*.json"):
    card = json.loads(path.read_text(encoding="utf-8"))
    card["identity"]["candidateSha"] = sha
    path.write_text(json.dumps(card, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
python3 - "$attestation" "$repo/benchmark/results/attestations/$baseline_sha.json" "$baseline_sha" <<'PY'
import json
import sys
from pathlib import Path
value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
value["candidateSha"] = sys.argv[3]
Path(sys.argv[2]).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
if "$compare" baseline candidate >"$fixture/ancestor.out" 2>"$fixture/ancestor.err"; then
  echo "compare fixture: non-ancestor proposal was accepted" >&2
  exit 1
fi
grep -q 'not an ancestor' "$fixture/ancestor.err" \
  || { echo "compare fixture: ancestry refusal was not specific" >&2; exit 1; }

mv "$baseline_candidate_dir" "$candidate_dir"
python3 - "$repo/benchmark/results/cohorts/candidate.json" "$candidate_dir" "$candidate_sha" <<'PY'
import json
import sys
from pathlib import Path
record = Path(sys.argv[1])
directory = Path(sys.argv[2])
sha = sys.argv[3]
value = json.loads(record.read_text(encoding="utf-8"))
value["candidateSha"] = sha
record.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
for path in directory.glob("*.json"):
    card = json.loads(path.read_text(encoding="utf-8"))
    card["identity"]["candidateSha"] = sha
    path.write_text(json.dumps(card, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

# A 0.x case version remains descriptive even with complete scalar data and
# floors: no verdict, the case's own note as the reason.
"$compare" baseline0 candidate0 >/dev/null
python3 - "$repo/benchmark/results/compares/baseline0-vs-candidate0.json" <<'PY'
import json
import sys
value = json.load(open(sys.argv[1], encoding="utf-8"))
assert value["comparisonEligible"] is False
assert value["verdict"] == "no-verdict"
assert value["eligibilityReasons"] == ["fixture 0.x is descriptive only"], value["eligibilityReasons"]
assert all(item["verdict"] == "no-verdict" for item in value["metrics"])
assert value["benchmark"] == {"caseId": "fixture", "caseVersion": "0.1", "configId": "fixturecfg", "configVersion": "1"}
PY

# An orchestration-health configuration is never verdict-eligible, whatever
# the case's maturity (design §3).
"$compare" baselineh candidateh >/dev/null
python3 - "$repo/benchmark/results/compares/baselineh-vs-candidateh.json" <<'PY'
import json
import sys
value = json.load(open(sys.argv[1], encoding="utf-8"))
assert value["comparisonEligible"] is False
assert value["verdict"] == "no-verdict"
assert any("orchestration-health" in reason for reason in value["eligibilityReasons"]), value["eligibilityReasons"]
PY

# A schema-1 (legacy) record pair still compares: the retired id resolves
# through aliases.json, the tuple carries no pins, and the report says so.
"$compare" baselinel candidatel >/dev/null
python3 - "$repo/benchmark/results/compares/baselinel-vs-candidatel.json" <<'PY'
import json
import sys
value = json.load(open(sys.argv[1], encoding="utf-8"))
assert value["verdict"] == "improved", value["verdict"]
assert value["cohorts"]["baseline"]["record"]["legacyId"] == "legacy"
assert value["comparabilityTuple"]["caseTree"] is None
assert any("legacy cohort" in reason for reason in value["eligibilityReasons"]), value["eligibilityReasons"]
PY

# A legacy record can never be verdict-compared against a pinned one: the
# tuple's pins differ.
if "$compare" baseline candidatel >"$fixture/mixed.out" 2>"$fixture/mixed.err"; then
  echo "compare fixture: legacy-vs-pinned comparison was accepted" >&2
  exit 1
fi
grep -q 'comparability tuple mismatch' "$fixture/mixed.err" \
  || { echo "compare fixture: legacy-vs-pinned refusal was not specific" >&2; exit 1; }

# The configuration report: same case, different configurations, no verdict.
"$compare" --configurations baseline baselineh >"$fixture/configurations.out" 2>"$fixture/configurations.err" \
  || { cat "$fixture/configurations.err" >&2; echo "compare fixture: configuration report failed" >&2; exit 1; }
grep -q 'No verdict' "$fixture/configurations.out" \
  || { echo "compare fixture: configuration report has no no-verdict line" >&2; exit 1; }
if "$compare" --configurations baseline baseline0 >/dev/null 2>"$fixture/configurations-mismatch.err"; then
  echo "compare fixture: configuration report accepted differing case versions" >&2
  exit 1
fi
grep -q 'do not share caseVersion' "$fixture/configurations-mismatch.err" \
  || { echo "compare fixture: configuration report refusal was not specific" >&2; exit 1; }

red_repo=$fixture/red-repo
mkdir -p "$red_repo/benchmark" "$red_repo/metasystem/scripts"
cp "$kit/attest.sh" "$kit/system-fingerprint.py" "$red_repo/benchmark/"
python3 - "$red_repo" <<'PY'
import sys
from pathlib import Path
root = Path(sys.argv[1])
(root / "metasystem" / "scripts" / "validate-metasystem.sh").write_text(
    "#!/usr/bin/env bash\nexit 9\n", encoding="utf-8"
)
(root / "candidate.txt").write_text("candidate\n", encoding="utf-8")
PY
chmod +x "$red_repo/benchmark/attest.sh" "$red_repo/benchmark/system-fingerprint.py" \
  "$red_repo/metasystem/scripts/validate-metasystem.sh"
git -C "$red_repo" init -q -b main
git -C "$red_repo" add .
git -C "$red_repo" -c user.name=fixture -c user.email=fixture@example.invalid commit -qm red
if "$red_repo/benchmark/attest.sh" >"$fixture/red.out" 2>"$fixture/red.err"; then
  echo "attest fixture: red gate was attested" >&2
  exit 1
fi
grep -q 'local gate was red' "$fixture/red.err" \
  || { echo "attest fixture: red refusal was not specific" >&2; exit 1; }
[[ ! -d "$red_repo/benchmark/results/attestations" \
    || -z "$(find "$red_repo/benchmark/results/attestations" -type f -print -quit)" ]] \
  || { echo "attest fixture: red gate wrote an attestation" >&2; exit 1; }

if [[ ${BENCHMARK_COMPARE_FIXTURE_PRINT:-0} == 1 ]]; then
  cat "$fixture/full-verdict.json"
fi
echo "benchmark evolution fixtures: PASS"
