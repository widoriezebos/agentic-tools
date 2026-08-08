#!/usr/bin/env bash
# The kit's own gate. The kit measures the metasystem from outside, so its
# checks live outside too: the shipped validation suite must never reference
# the kit (dependencies point one way, kit into equipment). Run this beside
# scripts/validate-metasystem.sh when developing; adopted repositories never
# see it.
set -euo pipefail

kit=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
top=$(cd "$kit/.." && pwd -P)

tmp=$(mktemp -d)

# Same marker as the metasystem suite: this gate is work in flight for the
# sibling checkout whose turn-end report has to know about it.
gate_run_marker=
if [[ -x "$top/metasystem/scripts/agents/gate-run.py" ]]; then
  gate_run_marker=$("$top/metasystem/scripts/agents/gate-run.py" register \
    --root "$top/metasystem" --gate validate-kit.sh --pid $$ 2>/dev/null || true)
fi

# The kit gate is required to run in delegate sandboxes where process-table
# visibility is denied. Provisioning still exercises the real supervision
# bridge; only its OS process source is replaced, and only when even the
# current shell cannot be inspected. An empty enumeration is sufficient here:
# this fixture proves arming/preflight integration, while the metasystem suite
# owns process-census behavior itself.
if ! ps -p "$$" -o lstart= >/dev/null 2>&1 \
    || ! ps -axo pid=,ppid=,pgid=,lstart=,command= >/dev/null 2>&1; then
  restricted_bin=$tmp/restricted-bin
  mkdir -p "$restricted_bin"
  python3 - "$restricted_bin/ps" <<'PY'
import sys
from pathlib import Path

Path(sys.argv[1]).write_text(
    "#!/usr/bin/env python3\n"
    "import json, os, sys\n"
    "from pathlib import Path\n"
    "args = sys.argv[1:]\n"
    "if 'lstart=' in args:\n"
    "    print('Wed Aug  6 12:00:00 2026')\n"
    "elif 'command=' in args:\n"
    "    pid = int(args[args.index('-p') + 1])\n"
    "    root = Path(os.environ['METASYSTEM_RESTRICTED_PS_ROOT'])\n"
    "    def tagged(value):\n"
    "        if isinstance(value, dict):\n"
    "            if value.get('pid') == pid and isinstance(value.get('instanceTag'), str):\n"
    "                return value['instanceTag']\n"
    "            for child in value.values():\n"
    "                found = tagged(child)\n"
    "                if found: return found\n"
    "        elif isinstance(value, list):\n"
    "            for child in value:\n"
    "                found = tagged(child)\n"
    "                if found: return found\n"
    "        return None\n"
    "    for path in root.rglob('*.json'):\n"
    "        try: found = tagged(json.loads(path.read_text()))\n"
    "        except (OSError, ValueError): continue\n"
    "        if found:\n"
    "            print(found)\n"
    "            break\n",
    encoding="utf-8",
)
PY
  chmod +x "$restricted_bin/ps"
  PATH=$restricted_bin:$PATH
  METASYSTEM_RESTRICTED_PS_ROOT=$tmp
  export PATH METASYSTEM_RESTRICTED_PS_ROOT
fi

# KI-13's lesson, inherited with the provisioning block: every repository this
# gate arms gets shut down again, because killing components is futile while
# the owner self-heals, and a leaked owner outlives the run by days.
armed_supervision_repos=()
track_armed_supervision() { # repository
  local repo=$1 known
  [[ -n "$repo" ]] || return 0
  for known in ${armed_supervision_repos[@]+"${armed_supervision_repos[@]}"}; do
    [[ "$known" == "$repo" ]] && return 0
  done
  armed_supervision_repos+=("$repo")
}
kit_cleanup() {
  [[ -z "${gate_run_marker:-}" ]] || rm -f "$gate_run_marker"
  local repo
  for repo in ${armed_supervision_repos[@]+"${armed_supervision_repos[@]}"}; do
    [[ -x "$repo/scripts/agents/arm-supervision.sh" ]] || continue
    "$repo/scripts/agents/arm-supervision.sh" --repo "$repo" --shutdown >/dev/null 2>&1 || true
  done
  rm -rf -- "$tmp"
}
trap kit_cleanup EXIT

# 1. The extractor's own fixtures.
"$kit/extractor-fixtures.sh" >/dev/null
echo "kit: extractor fixtures passed"

# The evolution-loop tools use only synthetic scorecards and a local fake gate:
# no mission is launched and no network or paid runtime is touched.
"$kit/evolution-fixtures.sh" >/dev/null
echo "kit: evolution fixtures passed"

# 2. Cross-artifact seam checks: manifest against spec against instruments.
cd "$kit/.."

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

# 3. Provisioning, end to end against a committed snapshot of the working
# tree (adopt.sh rightly refuses a dirty template, and the gate must test the
# tree as it is now, not as it was last committed).
srcrepo="$tmp/snapshot"
mkdir -p "$srcrepo"
for part in metasystem benchmark; do
  mkdir -p "$srcrepo/$part"
  # Stage through a temp archive rather than a create|extract pipe. bsdtar
  # reports a spurious "Write error" when the reading tar closes the pipe at
  # end-of-archive, and a larger tree (a second spec) makes it deterministic;
  # the copy is complete regardless, but the nonzero create exit fails the gate.
  # A temp file has no pipe to break, and each tar exits 0.
  part_archive="$tmp/snapshot-$part.tar"
  ( cd "$top/$part" && tar -cf "$part_archive" --exclude './artifacts' --exclude './target' . )
  ( cd "$srcrepo/$part" && tar -xf "$part_archive" )
  rm -f "$part_archive"
done
git -C "$srcrepo" init -q
git -C "$srcrepo" add .
git -C "$srcrepo" -c user.name=kit -c user.email=kit@example.invalid commit -qm snapshot
  # Benchmark provisioning owns the complete bridge from a held-out spec kit
  # to the human seal/sign boundary. Exercise the real BM-1 manifest through a
  # clean source snapshot because adopt.sh correctly refuses a dirty source.
  provision_target="$tmp/provision-bm-1"
  provision_contract="$provision_target/plans/mission-bm-1.contract.md"
  provision_output="$tmp/provision-bm-1.out"
  provision_identity=(
    env
    GIT_AUTHOR_NAME=metasystem-fixture
    GIT_AUTHOR_EMAIL=metasystem-fixture@example.invalid
    GIT_COMMITTER_NAME=metasystem-fixture
    GIT_COMMITTER_EMAIL=metasystem-fixture@example.invalid
  )

  # Defence in depth must fail before target creation if any declared copy
  # source crosses the held-out grader boundary.
  grader_spec="$tmp/provision-grader-spec"
  cp -R "$srcrepo/benchmark/specs/bm-1" "$grader_spec"
  python3 - "$grader_spec/manifest.json" <<'PY'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
value = json.loads(path.read_text(encoding="utf-8"))
value["seed"]["path"] = value["grader"]["path"]
path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")
PY
  if "${provision_identity[@]}" "$srcrepo/benchmark/provision.sh" \
      --spec "$grader_spec" --target "$tmp/provision-must-refuse" \
      >"$tmp/provision-grader.out" 2>"$tmp/provision-grader.err"; then
    echo "benchmark provision: accepted a grader path as a copy source" >&2
    exit 1
  fi
  grep -q 'held-out grader path would be copied' "$tmp/provision-grader.err" \
    || { echo "benchmark provision: grader refusal was not loud and specific" >&2; exit 1; }
  [[ ! -e "$tmp/provision-must-refuse" ]] \
    || { echo "benchmark provision: grader refusal created the target" >&2; exit 1; }

  if ! "${provision_identity[@]}" "$srcrepo/benchmark/provision.sh" \
      --spec "$srcrepo/benchmark/specs/bm-1" --target "$provision_target" \
      >"$provision_output" 2>"$tmp/provision-bm-1.err"; then
    echo "benchmark provision: BM-1 provisioning failed" >&2
    cat "$tmp/provision-bm-1.err" >&2
    exit 1
  fi
  track_armed_supervision "$provision_target"

  # Count STEPS, not lines: informational notices (F-4's tier note, for one)
  # legitimately share the stream, and a human step is what the human must do.
  grep -v '^INFO: ' "$provision_output" >"$provision_output.steps"
  [[ $(wc -l <"$provision_output.steps" | tr -d ' ') == 3 ]] \
    || { echo "benchmark provision: output was not exactly three human steps" >&2; cat "$provision_output" >&2; exit 1; }
  provision_output=$provision_output.steps
  sed -n '1p' "$provision_output" | grep -q '^Review ' \
    || { echo "benchmark provision: first human step is not contract review" >&2; exit 1; }
  sed -n '2p' "$provision_output" | grep -q '^Seal it: ' \
    || { echo "benchmark provision: second human step is not sealing" >&2; exit 1; }
  sed -n '3p' "$provision_output" | grep -q '^Sign it: ' \
    || { echo "benchmark provision: third human step is not signing" >&2; exit 1; }

  [[ -f "$provision_contract" ]] \
    || { echo "benchmark provision: mission contract is missing" >&2; exit 1; }
  if grep -qE '^```mission-seal|^Approval:' "$provision_contract"; then
    echo "benchmark provision: provisioner crossed the human seal/sign boundary" >&2
    exit 1
  fi
  "$provision_target/scripts/assert-mission.sh" --file "$provision_contract" >/dev/null \
    || { echo "benchmark provision: unsigned contract is structurally invalid" >&2; exit 1; }

  # The grader must be absent by world state, not merely by provisioner claim.
  [[ ! -e "$provision_target/benchmark" && ! -e "$provision_target/grader" ]] \
    || { echo "benchmark provision: held-out grader or benchmark kit reached the target" >&2; exit 1; }
  [[ -z "$(find "$provision_target" -path "$provision_target/artifacts" -prune \
      -o -path "$provision_target/.git" -prune -o -type d -name grader -print -quit)" ]] \
    || { echo "benchmark provision: a held-out grader directory exists in the target" >&2; exit 1; }
  [[ ! -e "$provision_target/calibrate.py" && ! -e "$provision_target/grade.sh" ]] \
    || { echo "benchmark provision: held-out grader files reached the target" >&2; exit 1; }

  # Configuration is manifest-derived: compare the live target to the manifest
  # instead of restating either model identifier in the fixture.
  python3 - "$srcrepo/benchmark/specs/bm-1/manifest.json" \
    "$provision_target/metasystem.conf" "$provision_contract" <<'PY'
import json
import re
import sys
from pathlib import Path

manifest = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
config = {}
for raw in Path(sys.argv[2]).read_text(encoding="utf-8").splitlines():
    if raw and not raw.startswith("#") and "=" in raw:
        key, value = raw.split("=", 1)
        config[key] = value
contract = {}
inside = False
for raw in Path(sys.argv[3]).read_text(encoding="utf-8").splitlines():
    if raw == "```mission":
        inside = True
    elif raw == "```" and inside:
        inside = False
    elif inside and "=" in raw:
        key, value = raw.split("=", 1)
        contract[key] = value

host = manifest["roster"]["host"]
delegates = manifest["roster"]["delegates"]
models = {host["runtime"]: host["model"], **delegates}
if config.get("metasystem.runtimes", "").split(",") != list(models):
    raise SystemExit("benchmark provision: configured runtimes differ from manifest roster")
for runtime, model in models.items():
    configured = [
        value for key, value in config.items()
        if re.fullmatch(rf"(?:role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+)\.model\.{re.escape(runtime)}", key)
    ]
    if not configured or set(configured) != {model}:
        raise SystemExit(f"benchmark provision: configured {runtime} models differ from manifest")
if contract.get("host.runtime") != host["runtime"] or contract.get("host.model") != host["model"]:
    raise SystemExit("benchmark provision: contract host differs from manifest roster")
fences = manifest["fences"]
expected = {
    "ledger.cycle-budget": fences["ledgerCycleBudget"],
    "ledger.no-gain-budget": fences["ledgerNoGainBudget"],
    "fence.wall-clock-hours": fences["wallClockHours"],
    "fence.cycles": fences["cycles"],
    "fence.jobs": fences["jobs"],
    "fence.concurrency": fences["concurrency"],
    "fence.job-cap-min": fences["jobCapMin"],
    "host.turn-cap-min": fences["hostTurnCapMin"],
}
for key, value in expected.items():
    if contract.get(key) != str(value):
        raise SystemExit(f"benchmark provision: contract {key} differs from manifest")
for stream, goal in manifest["missionContract"]["streams"].items():
    if contract.get(f"stream.{stream}") != goal:
        raise SystemExit(f"benchmark provision: stream {stream} differs from manifest")
PY
  "$provision_target/scripts/metasystem-config.sh" validate \
    || { echo "benchmark provision: filled metasystem.conf is invalid" >&2; exit 1; }

  provision_ref=$(sed -n 's/^gate\.ref=//p' "$provision_contract")
  [[ -n "$provision_ref" ]] \
    || { echo "benchmark provision: contract has no gate.ref" >&2; exit 1; }
  git -C "$provision_target" rev-parse --verify -q "refs/tags/$provision_ref^{commit}" >/dev/null \
    || { echo "benchmark provision: gate.ref is not a tag" >&2; exit 1; }
  if git -C "$provision_target" show-ref --verify -q "refs/heads/$provision_ref"; then
    echo "benchmark provision: gate.ref also names a branch" >&2
    exit 1
  fi
  cmp "$srcrepo/benchmark/specs/bm-1/gate.sh" "$provision_target/gate.sh" >/dev/null \
    && cmp "$srcrepo/benchmark/specs/bm-1/guard-deps.sh" "$provision_target/guard-deps.sh" >/dev/null \
    || { echo "benchmark provision: copied instruments differ from the spec" >&2; exit 1; }
  git -C "$provision_target" cat-file -e "$provision_ref:gate.sh" \
    && git -C "$provision_target" cat-file -e "$provision_ref:guard-deps.sh" \
    || { echo "benchmark provision: instrument tag does not contain both instruments" >&2; exit 1; }

  provision_origin=$(git -C "$provision_target" remote get-url origin)
  # Compare resolved to resolved: provision canonicalises its target, and
  # macOS mktemp hands out the symlinked spelling of the temp directory, so a
  # textual comparison fails on any normal Mac while passing in a sandbox
  # whose TMPDIR is already canonical.
  [[ "$(python3 -c 'import os,sys;print(os.path.realpath(sys.argv[1]))' "$provision_origin")" \
      == "$(python3 -c 'import os,sys;print(os.path.realpath(sys.argv[1]))' "$provision_target.origin.git")" ]] \
    || { echo "benchmark provision: origin is not the sibling bare repository" >&2; exit 1; }
  [[ "$(git -C "$provision_origin" rev-parse --is-bare-repository)" == true ]] \
    || { echo "benchmark provision: local origin is not bare" >&2; exit 1; }
  [[ "$(git -C "$provision_target" symbolic-ref refs/remotes/origin/HEAD)" == refs/remotes/origin/main ]] \
    || { echo "benchmark provision: origin default branch is not declared" >&2; exit 1; }

  before_rerun=$(git -C "$provision_target" status --porcelain=v1)
  if "${provision_identity[@]}" "$srcrepo/benchmark/provision.sh" \
      --spec "$srcrepo/benchmark/specs/bm-1" --target "$provision_target" \
      >"$tmp/provision-rerun.out" 2>"$tmp/provision-rerun.err"; then
    echo "benchmark provision: rerun accepted an existing target" >&2
    exit 1
  fi
  grep -q 'target already exists' "$tmp/provision-rerun.err" \
    || { echo "benchmark provision: rerun refusal did not name the existing target" >&2; exit 1; }
  [[ "$(git -C "$provision_target" status --porcelain=v1)" == "$before_rerun" ]] \
    || { echo "benchmark provision: refused rerun changed the target" >&2; exit 1; }

  # The cohort driver owns the next layer: one persistent cohort record, a
  # fresh target, the extractor-visible identity stamp, and a hard pause at
  # the unsigned human boundary. Do not resume past that boundary here.
  cohort_output=$tmp/cohort.out
  # The gate runs its cohort in its OWN trials root. The snapshot carries the
  # operator's trials-root.local, so without this the gate provisions a fake
  # cohort into the directory where real benchmark runs live, and then looks
  # for it somewhere else entirely.
  cohort_trials_root=$tmp/trials
  if ! "${provision_identity[@]}" METASYSTEM_TRIALS_ROOT="$cohort_trials_root" \
      "$srcrepo/benchmark/run-cohort.sh" \
      --spec bm-1 --repetitions 2 >"$cohort_output" 2>"$tmp/cohort.err"; then
    echo "benchmark cohort: initial staging failed" >&2
    cat "$tmp/cohort.err" >&2
    exit 1
  fi
  [[ $(wc -l <"$cohort_output" | tr -d ' ') == 4 ]] \
    || { echo "benchmark cohort: human boundary was not exactly four steps" >&2; exit 1; }
  sed -n '1p' "$cohort_output" | grep -q '^Review ' \
    && sed -n '2p' "$cohort_output" | grep -q '^Seal it: ' \
    && sed -n '3p' "$cohort_output" | grep -q '^Sign it: ' \
    && sed -n '4p' "$cohort_output" | grep -q '^Resume it: ' \
    || { echo "benchmark cohort: printed human boundary is incomplete" >&2; exit 1; }
  # Take the id from the cohort THIS gate just staged, not from whatever file
  # happens to sort first: any run record left in the source tree would
  # otherwise be picked and its target looked for in this gate's temp trials
  # root, where it does not exist. The Resume line is already validated above.
  cohort_id=$(sed -n '4p' "$cohort_output" | awk '{print $NF}')
  [[ -n "$cohort_id" ]] \
    || { echo "benchmark cohort: could not read the staged cohort id" >&2; exit 1; }
  cohort_record="$srcrepo/benchmark/results/cohorts/$cohort_id.json"
  [[ -f "$cohort_record" ]] \
    || { echo "benchmark cohort: cohort record is missing for $cohort_id" >&2; exit 1; }
  cohort_target=$cohort_trials_root/cohorts/$cohort_id/targets/1
  track_armed_supervision "$cohort_target"
  python3 - "$cohort_record" "$cohort_target/artifacts/agents/benchmark-identity.json" \
    "$srcrepo/benchmark/kit-version" <<'PY'
import json
import sys
record = json.load(open(sys.argv[1], encoding="utf-8"))
identity = json.load(open(sys.argv[2], encoding="utf-8"))
kit_version = open(sys.argv[3], encoding="utf-8").read().strip()
assert record["benchmarkSpecId"] == "bm-1"
assert record["benchmarkSpecVersion"] == "0.1"
assert record["measuringKitVersion"] == kit_version == "0.1.0"
assert record["proposalId"] is None and record["repetitionCount"] == 2
assert set(record["machineFingerprint"]) == {"os", "cpuModel", "coreCount"}
assert isinstance(record["roster"], dict) and record["roster"]
assert identity["cohortId"] == record["cohortId"]
assert identity["candidateSha"] == record["candidateSha"]
assert identity["repetitionIndex"] == 1 and identity["repetitionCount"] == 2
assert identity["measuringKitVersion"] == kit_version
PY
  if "${provision_identity[@]}" METASYSTEM_TRIALS_ROOT="$cohort_trials_root" \
      "$srcrepo/benchmark/run-cohort.sh" --resume "$cohort_id" \
      >"$tmp/cohort-resume.out" 2>"$tmp/cohort-resume.err"; then
    echo "benchmark cohort: unsigned repetition resumed" >&2
    exit 1
  fi
  grep -q 'contract has no Approval line' "$tmp/cohort-resume.err" \
    || { echo "benchmark cohort: unsigned refusal was not specific" >&2; exit 1; }
  [[ ! -e "$cohort_target/artifacts/agents/missions/bm-1/state.json" ]] \
    || { echo "benchmark cohort: unsigned refusal started the mission" >&2; exit 1; }

  # Fixture-only human actions: seal, add the byte-attesting approval, commit,
  # and push it so origin provenance can pass. Provisioning itself did none.
  seal_hash=$("${provision_identity[@]}" "$provision_target/scripts/assert-mission.sh" \
    --seal --file "$provision_contract")
  [[ "$seal_hash" =~ ^[0-9a-f]{64}$ ]] \
    || { echo "benchmark provision: fixture seal did not return a contract hash" >&2; exit 1; }
  printf '\nApproval: name=Metasystem Fixture; date=2026-08-05; contract-sha256=%s\n' \
    "$seal_hash" >>"$provision_contract"
  git -C "$provision_target" add plans/mission-bm-1.contract.md
  "${provision_identity[@]}" git -C "$provision_target" commit -qm "Seal and sign fixture mission"
  git -C "$provision_target" push -q origin main
  preflight_output=$("$provision_target/scripts/assert-mission.sh" --preflight --file "$provision_contract") \
    || { echo "benchmark provision: provisioned BM-1 failed preflight" >&2; exit 1; }
  [[ "$preflight_output" == "mission preflight passed: bm-1" ]] \
    || { echo "benchmark provision: preflight did not report BM-1 success" >&2; exit 1; }

echo "kit: provisioning bridge passed"
echo "kit validation passed"
