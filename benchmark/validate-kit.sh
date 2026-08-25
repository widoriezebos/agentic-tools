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
if [[ -x "$top/metasystem/bin/metasystem" ]]; then
  gate_run_marker=$("$top/metasystem/bin/metasystem" gate register \
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

# Kit-vs-engine drift guard: the kit scores evidence the CURRENT engine
# writes. Schemas shared with the engine must match its shipped copies
# byte-for-byte, and every engine-owned evidence filename the extractor
# requires must still appear in the engine's sources. Both halves have
# bitten: the 'proc started-at' verb rename was hand-fixed with no guard,
# and cohort bm-1-20260813t055303z lost three validity gates to a stale
# mission-state schema plus a check for the retired watcher.log.
# Delegate-role returns are STORED in the engine's derived v2 envelope
# (schemaVersion + claimed), so the kit pins the v2 form the engine's own
# materializer emits; everything else shared with the engine is a direct
# byte copy of the shipped schema.
derived_roles=" code-critic design-critic implementer investigator verifier "
for schema in "$kit"/schemas/evidence/*.schema.json; do
  name=$(basename "$schema")
  role=${name%.schema.json}
  if [[ "$derived_roles" == *" $role "* ]]; then
    "$top/metasystem/bin/metasystem" schema materialize --root "$top/metasystem"       --role "$role" --version 2 --output "$tmp/derived-$name"
    cmp -s "$schema" "$tmp/derived-$name" || {
      echo "kit drift: schemas/evidence/$name differs from the engine's materialized v2 schema; regenerate it with 'metasystem schema materialize --version 2'" >&2
      exit 1
    }
    continue
  fi
  engine_schema=$top/metasystem/scripts/agents/schemas/$name
  [[ -f "$engine_schema" ]] || continue
  cmp -s "$schema" "$engine_schema" || {
    echo "kit drift: schemas/evidence/$name differs from the engine's shipped copy; sync it from scripts/agents/schemas/" >&2
    exit 1
  }
done
for evidence_name in owner.ndjson last-census.json state.json ledger.md; do
  grep -rqF "$evidence_name" "$top/metasystem/internal" "$top/metasystem/scripts" || {
    echo "kit drift: the extractor requires $evidence_name but the engine's sources never mention it" >&2
    exit 1
  }
done
echo "kit: engine drift guard passed"

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

# The evidence schemas must track the engine (KI-40's weekend, third
# drift): a fresh state from the CURRENT engine validates under the
# kit's own ruler, or nothing else here is measuring anything.
"$kit/evidence-drift-fixtures.sh" >/dev/null
echo "kit: evidence drift fixtures passed"

# 2. Cross-artifact seam checks: case against spec against instruments, plus
# the object model itself — every shipped case version, configuration version
# and the alias table validate against their schemas; the version registry
# matches HEAD and is append-only across history; every alias pair is
# compatible (design §6); each case's stream text names the gate it runs.
cd "$kit/.."
python3 - "$kit" <<'PY'
import json
import subprocess
import sys
from pathlib import Path
kit = Path(sys.argv[1])
sys.path.insert(0, str(kit))
from extractor import schema_violations, read_schema  # the kit's own checker
problems = []
def check(path, schema):
    try:
        doc = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        problems.append(f"{path}: unreadable: {error}"); return
    for violation in schema_violations(doc, read_schema(schema)):
        problems.append(f"{path}: {violation}")
cases = sorted(kit.glob("cases/*/*/case.json"))
configs = sorted(kit.glob("configurations/*/*.json"))
if not cases: problems.append("no benchmark cases under benchmark/cases")
if not configs: problems.append("no benchmark configurations under benchmark/configurations")
for c in cases:
    check(c, "case.schema.json")
    doc = json.loads(c.read_text(encoding="utf-8"))
    if (doc.get("id"), doc.get("version")) != (c.parent.parent.name, c.parent.name):
        problems.append(f"{c}: names {doc.get('id')}@{doc.get('version')} but lives at cases/{c.parent.parent.name}/{c.parent.name}")
    for name in ("spec.md", doc.get("seed", {}).get("path", "seed/"), doc.get("grader", {}).get("path", "grader/")):
        if not (c.parent / name).exists(): problems.append(f"{c.parent}: missing {name}")
    for instrument in doc.get("mission", {}).get("instruments", []):
        if not (c.parent / instrument).is_file(): problems.append(f"{c.parent}: instrument {instrument} is not shipped")
    gate = doc.get("mission", {}).get("gate", {})
    metric, threshold = gate.get("metric", ""), str(gate.get("threshold", "")).replace(" ", "")
    for stream, text in doc.get("mission", {}).get("streams", {}).items():
        wanted = f"{metric}{threshold}" if threshold.startswith((">=", "<=", "=")) else f"{metric}={threshold}"
        if wanted.replace(">=1", "=1") not in text.replace(" ", "") and wanted not in text.replace(" ", ""):
            problems.append(f"{c}: stream {stream!r} does not name the gate it runs ({metric} {threshold}); builders would be told the wrong target")
for k in configs:
    check(k, "configuration.schema.json")
    doc = json.loads(k.read_text(encoding="utf-8"))
    if (doc.get("id"), doc.get("version")) != (k.parent.name, k.stem):
        problems.append(f"{k}: names {doc.get('id')}@{doc.get('version')} but lives at configurations/{k.parent.name}/{k.stem}.json")
check(kit / "aliases.json", "aliases.schema.json")
check(kit / "versions.lock", "versions-lock.schema.json")
lock = json.loads((kit / "versions.lock").read_text(encoding="utf-8")).get("entries", {})
for c in cases:
    if f"case:{c.parent.parent.name}@{c.parent.name}" not in lock: problems.append(f"{c.parent}: not registered in versions.lock (benchmark/pairs.py register)")
for k in configs:
    if f"configuration:{k.parent.name}@{k.stem}" not in lock: problems.append(f"{k}: not registered in versions.lock (benchmark/pairs.py register)")
reg = subprocess.run([sys.executable, str(kit / "pairs.py"), "registry-check", "--kit", str(kit), "--history"], capture_output=True, text=True)
if reg.returncode != 0:
    problems.append("version registry: " + reg.stderr.strip().replace("\n", " | "))
aliases = json.loads((kit / "aliases.json").read_text(encoding="utf-8")).get("aliases", {})
for legacy in sorted(aliases):
    res = subprocess.run([sys.executable, str(kit / "pairs.py"), "resolve", "--kit", str(kit), "--spec", legacy], capture_output=True, text=True)
    if res.returncode != 0:
        problems.append(f"alias {legacy}: " + res.stderr.strip().replace("\n", " | "))
    for line in res.stderr.splitlines():
        if line.startswith("warning:"):
            print(f"benchmark consistency: {line}", file=sys.stderr)
if problems:
    print("benchmark consistency: the object model is inconsistent", file=sys.stderr)
    for item in problems: print(f"  {item}", file=sys.stderr)
    raise SystemExit(1)
print(f"benchmark consistency: {len(cases)} case version(s), {len(configs)} configuration version(s), {len(aliases)} alias(es): schemas, registry, compatibility, stream/gate agreement OK")
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
for manifest_path in sorted(Path("benchmark/cases").glob("*/*/case.json")):
    spec_dir = manifest_path.parent
    try:
        manifest = json.loads(manifest_path.read_text())
        # The seam checks below were written against the old manifest shape;
        # a case version carries the same task policy under `mission`.
        manifest["missionContract"] = manifest.get("mission", {})
    except ValueError as error:
        violations.append(f"{manifest_path}: unparseable: {error}")
        continue

    # A spec's id IS its directory name: the id keys cohort naming, the
    # recorded benchmarkSpecId, and --resume's spec resolution, so a copied
    # id silently files one spec's cohorts under another and resumes them
    # against the wrong spec. bm-2d and bm-2dc shipped carrying bm-2's id
    # and the first acceptance cohort provisioned under the wrong identity
    # before anything refused (2026-08-17).
    spec_id = manifest.get("id")
    if manifest.get("id") != spec_dir.parent.name or manifest.get("version") != spec_dir.name:
        violations.append(f"{manifest_path}: {manifest.get('id')}@{manifest.get('version')} must live at cases/{manifest.get('id')}/{manifest.get('version')} (found cases/{spec_dir.parent.name}/{spec_dir.name})")

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
    copy_dir = spec_dir
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
            instrument = copy_dir / Path(script).name
            if not instrument.exists():
                violations.append(f"{manifest_path}: {kind} instrument {script} not shipped in {copy_dir}")
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

count = len(list(Path("benchmark/cases").glob("*/*/case.json")))
print(f"benchmark consistency: {count} case version(s), no seams")
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
# Machine-pinning enrollment (2026-08-24): adoption refuses an
# unenrolled source, and this scratch snapshot is its own machine —
# a fixture machine publishes a fixture nickname through the same
# front door every real machine uses.
git -C "$srcrepo" config metasystem.goal.machine kit-fixture
git -C "$srcrepo" add .
git -C "$srcrepo" -c user.name=kit -c user.email=kit@example.invalid commit -qm snapshot
  # Benchmark provisioning owns the complete bridge from a held-out spec kit
  # to the human seal/sign boundary. Exercise the real BM-1 manifest through a
  # clean source snapshot because adopt.sh correctly refuses a dirty source.
  provision_target="$tmp/provision-bm-1"
  provision_contract="$provision_target/plans/mission-taskrun.contract.md"
  provision_output="$tmp/provision-bm-1.out"
  provision_identity=(
    env
    GIT_AUTHOR_NAME=metasystem-fixture
    GIT_AUTHOR_EMAIL=metasystem-fixture@example.invalid
    GIT_COMMITTER_NAME=metasystem-fixture
    GIT_COMMITTER_EMAIL=metasystem-fixture@example.invalid
  )

  # Defence in depth must fail before target creation if any declared copy
  # source crosses the held-out grader boundary. The bad case version is
  # committed and registered in the snapshot so provisioning reaches it
  # through the real pinned path.
  cp -R "$srcrepo/benchmark/cases/taskrun/0.1" "$srcrepo/benchmark/cases/taskrun/9.9"
  python3 - "$srcrepo/benchmark/cases/taskrun/9.9/case.json" <<'PY'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
value = json.loads(path.read_text(encoding="utf-8"))
value["version"] = "9.9"
value["seed"]["path"] = value["grader"]["path"]
path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")
PY
  git -C "$srcrepo" add benchmark/cases/taskrun/9.9
  python3 "$srcrepo/benchmark/pairs.py" register --kit "$srcrepo/benchmark" --case taskrun@9.9 >/dev/null
  git -C "$srcrepo" add benchmark/versions.lock
  git -C "$srcrepo" -c user.name=kit -c user.email=kit@example.invalid commit -qm "bad case version for the grader-boundary refusal"
  if "${provision_identity[@]}" "$srcrepo/benchmark/provision.sh" \
      --case taskrun@9.9 --config cheap@1 --target "$tmp/provision-must-refuse" \
      >"$tmp/provision-grader.out" 2>"$tmp/provision-grader.err"; then
    echo "benchmark provision: accepted a grader path as a copy source" >&2
    exit 1
  fi
  grep -q 'held-out grader path would be copied' "$tmp/provision-grader.err" \
    || { echo "benchmark provision: grader refusal was not loud and specific" >&2; exit 1; }
  [[ ! -e "$tmp/provision-must-refuse" ]] \
    || { echo "benchmark provision: grader refusal created the target" >&2; exit 1; }

  if ! "${provision_identity[@]}" "$srcrepo/benchmark/provision.sh" \
      --case taskrun@0.1 --config cheap@1 --target "$provision_target" \
      >"$provision_output" 2>"$tmp/provision-bm-1.err"; then
    echo "benchmark provision: taskrun@0.1 under cheap@1 provisioning failed" >&2
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
  validate_warnings="$tmp/provision-validate-warnings"
  "$provision_target/scripts/assert-mission.sh" --file "$provision_contract" >/dev/null 2>"$validate_warnings" \
    || { cat "$validate_warnings" >&2; echo "benchmark provision: unsigned contract is structurally invalid" >&2; exit 1; }
  # Calibration warnings come from VALIDATION, not the seal (issues #8/#7
  # round 2: the first gate captured the seal's stderr, which carries only
  # the hash or an error — the warning check was inert). A provisioned
  # contract must validate silently, and the sealed cap keys must equal
  # the manifest's byte for byte, so removing or drifting either cap can
  # never pass the kit.
  if grep -q '^warning:' "$validate_warnings"; then
    cat "$validate_warnings" >&2
    echo "benchmark provision: provisioned contract validates with warnings; the manifest must carry the policy" >&2
    exit 1
  fi
  manifest_turns=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["hostCaps"]["maxTurns"])' "$srcrepo/benchmark/configurations/cheap/1.json")
  manifest_budget=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["hostCaps"]["maxBudgetUsd"])' "$srcrepo/benchmark/configurations/cheap/1.json")
  # The caps are asserted INSIDE the fenced mission block — the only bytes
  # the runtime parses (round 2: a whole-file grep false-passed on an
  # expected line sitting in prose while the effective block disagreed).
  # The fence grammar mirrors grammar.go's authoredBlockRe exactly: both
  # fences tolerate trailing blanks, and extraction EXITS at the block's
  # close so a spaced closing fence can never spill into later prose
  # (round 3's witness).
  contract_mission_block=$(awk '/^\`\`\`mission[[:blank:]]*$/{f=1;next} f && /^\`\`\`[[:blank:]]*$/{exit} f' "$provision_contract")
  grep -qxF "host.max-turns=$manifest_turns" <<<"$contract_mission_block" \
    || { echo "benchmark provision: mission block host.max-turns disagrees with the manifest" >&2; exit 1; }
  grep -qxF "host.max-budget-usd=$manifest_budget" <<<"$contract_mission_block" \
    || { echo "benchmark provision: mission block host.max-budget-usd disagrees with the manifest" >&2; exit 1; }

  # PI-R1-006 (plans/provisioning-identity.md Proof): the wrapper route is
  # PROVEN, not hoped for. With an announced main in the caller's ancestry —
  # deterministic in every outer invocation shape — the target's guard must
  # REFUSE a raw commit and CARRY a wrapped one. The provisioner retired, so
  # the gate announces itself for the control, then retires.
  control_start=$("$provision_target/bin/metasystem" proc started-at --pid $$)
  "$provision_target/bin/metasystem" lease announce --root "$provision_target" \
    --session kit-gate-control --pid $$ --start "$control_start" \
    --tag metasystem-kit-gate-control --runtime fake >/dev/null
  echo control >"$provision_target/control-file.txt"
  git -C "$provision_target" add control-file.txt
  if git -C "$provision_target" -c user.name=m -c user.email=m@example.invalid \
      commit -qm "raw control commit" >"$tmp/raw-control.out" 2>&1; then
    echo "benchmark provision: a RAW agent commit passed the target's guard" >&2
    exit 1
  fi
  grep -q "requires scripts/agents/commit.sh" "$tmp/raw-control.out" \
    || { echo "benchmark provision: raw-commit refusal lost its message" >&2; cat "$tmp/raw-control.out" >&2; exit 1; }
  (cd "$provision_target" && GIT_AUTHOR_NAME=m GIT_AUTHOR_EMAIL=m@example.invalid \
      GIT_COMMITTER_NAME=m GIT_COMMITTER_EMAIL=m@example.invalid \
      scripts/agents/commit.sh -qm "wrapped control commit") \
    || { echo "benchmark provision: the wrapper route failed to carry a commit" >&2; exit 1; }
  git -C "$provision_target" reset -q --hard HEAD~1
  "$provision_target/bin/metasystem" lease retire --root "$provision_target" \
    --session kit-gate-control --pid $$ --start "$control_start" >/dev/null

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
  python3 "$srcrepo/benchmark/pairs.py" resolve --kit "$srcrepo/benchmark" --case taskrun@0.1 --config cheap@1 --out "$tmp/merged-manifest.json" >/dev/null 2>&1 \
    || { echo "benchmark provision: pair resolution failed in the snapshot" >&2; exit 1; }
  python3 - "$tmp/merged-manifest.json" \
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
roster = manifest["roster"]
delegates = roster["delegates"]
# The per-runtime projection is what provisioning writes for every spec:
# runtimes host-first, one model slot per runtime, model.tier.1 host-first.
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
tier_pairs = [(host["runtime"], host["model"])]
for runtime, model in delegates.items():
    if (runtime, model) not in tier_pairs:
        tier_pairs.append((runtime, model))
expected_tier = ",".join(f"{runtime}:{model}" for runtime, model in tier_pairs)
# Adoption emits no tier row by default; provisioning only rewrites one that
# exists. When one exists it must be the roster's pairs, host first.
if "model.tier.1" in config and config["model.tier.1"] != expected_tier:
    raise SystemExit("benchmark provision: model.tier.1 differs from the roster (host first, then delegates)")
# The per-role projection, when the manifest declares one, must be written
# explicitly and agree with the runtime slots -- and role.default.* must be
# untouched by it (adoption's tailoring owns those).
for role, resolution in (roster.get("delegateRoles") or {}).items():
    if config.get(f"role.{role}.runtime") != resolution["runtime"] \
       or config.get(f"role.{role}.model.{resolution['runtime']}") != resolution["model"]:
        raise SystemExit(f"benchmark provision: configured {role} resolution differs from manifest delegateRoles")
independence = roster.get("independence")
if (config.get("independence") or None) != independence:
    raise SystemExit("benchmark provision: independence declaration differs from manifest roster")
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
  cmp "$srcrepo/benchmark/cases/taskrun/0.1/gate.sh" "$provision_target/gate.sh" >/dev/null \
    && cmp "$srcrepo/benchmark/cases/taskrun/0.1/guard-deps.sh" "$provision_target/guard-deps.sh" >/dev/null \
    || { echo "benchmark provision: copied instruments differ from the case version" >&2; exit 1; }
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
      --case taskrun@0.1 --config cheap@1 --target "$provision_target" \
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
# Alias mode (--spec bm-1): the pair is recorded and pinned; the legacy id
# and label ride along; the cohort id and mission keep the legacy naming.
assert record["schemaVersion"] == 2
assert (record["caseId"], record["caseVersion"], record["configId"], record["configVersion"]) == ("taskrun", "0.1", "cheap", "1")
assert record["legacyId"] == "bm-1" and record["legacyVersionLabel"] == "0.1"
assert len(record["caseTree"]) == 40 and len(record["configTree"]) == 40
assert identity["schemaVersion"] == 2 and identity["caseTree"] == record["caseTree"] and identity["configTree"] == record["configTree"]
assert identity["legacyId"] == "bm-1"
assert record["measuringKitVersion"] == kit_version == "0.2.0"
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
  seal_warnings="$tmp/provision-seal-warnings"
  seal_hash=$("${provision_identity[@]}" "$provision_target/scripts/assert-mission.sh" \
    --seal --file "$provision_contract" 2>"$seal_warnings")
  [[ "$seal_hash" =~ ^[0-9a-f]{64}$ ]] \
    || { cat "$seal_warnings" >&2; echo "benchmark provision: fixture seal did not return a contract hash" >&2; exit 1; }
  # Manifest/validator drift fails HERE, not at the human's seal (issue
  # #8): a freshly provisioned contract must seal clean — a warning means
  # the manifest stopped being the single authority for contract policy.
  if grep -q '^warning:' "$seal_warnings"; then
    cat "$seal_warnings" >&2
    echo "benchmark provision: provisioned contract sealed with warnings; the manifest must carry the policy" >&2
    exit 1
  fi
  printf '\nApproval: name=Metasystem Fixture; date=2026-08-05; contract-sha256=%s\n' \
    "$seal_hash" >>"$provision_contract"
  git -C "$provision_target" add plans/mission-taskrun.contract.md
  # The gate SIMULATES THE HUMAN here: sealing and signing are the human's
  # own acts by design, and a human commit is sovereign under the guard. The
  # gate cannot be a human, so this one simulated-human commit skips hooks
  # explicitly — the guard's bite on agent commits is proven by the
  # refuse-raw control above, and provisioning's own commits stay
  # wrapper-carried under every invocation shape.
  "${provision_identity[@]}" git -C "$provision_target" commit -q --no-verify -m "Seal and sign fixture mission"
  git -C "$provision_target" push -q origin main
  preflight_output=$("$provision_target/scripts/assert-mission.sh" --preflight --file "$provision_contract") \
    || { echo "benchmark provision: provisioned taskrun@0.1 under cheap@1 failed preflight" >&2; exit 1; }
  # The caps work extended the success line with the contract pin; assert
  # the pin AGAINST THE BYTES rather than pattern-matching the sentence.
  expected_pin=$(python3 -c 'import hashlib,sys; print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "$provision_contract")
  [[ "$preflight_output" == "mission preflight passed: taskrun approvedContractSha256=$expected_pin" ]] \
    || { echo "benchmark provision: preflight did not report taskrun success with the contract's own pin; it said:" >&2; printf '%s\n' "$preflight_output" >&2; exit 1; }

echo "kit: provisioning bridge passed"
echo "kit validation passed"
