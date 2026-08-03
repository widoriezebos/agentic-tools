#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

scripts/audit-harness.sh .

# Validate every skill present, including project-added and moved optional
# skills, so this script holds in adopted repositories as well as the template.
# A skill directory without a SKILL.md is invisible to the find, so check for
# hollow directories explicitly first.
for dir in skills optional-skills; do
  [[ -d "$dir" ]] || continue
  for d in "$dir"/*/; do
    [[ -d "$d" ]] || continue
    [[ -f "${d}SKILL.md" ]] || { echo "skill directory without SKILL.md: ${d%/}" >&2; exit 1; }
  done
  while IFS= read -r skill_md; do
    scripts/validate-skill.sh "$(dirname "$skill_md")"
  done < <(find "$dir" -name SKILL.md | sort)
done

# Core assets are required everywhere. The full six-skill set with every
# per-runtime profile is required only in the template repository (marked by
# meta/harness-design.md): adopted repositories may prune unused skills, and
# each skill present is still validated by the loop above. Profile files are
# not demanded in adopted mode, because project-added skills never had them
# and core skills may drop them after runtime registration.
template_mode=0
[[ -f meta/harness-design.md ]] && template_mode=1

for link in \
  docs/project-rules.md \
  docs/orchestration.md \
  docs/collaboration.md \
  docs/design/design-principles.md \
  docs/design/design-obligation-gate.md \
  docs/examples/design-obligation-matrix.md \
  docs/examples/step-back-ledger.md \
  .gitattributes \
  plans/instruction-ledger.md \
  scripts/refactor-baseline.sh \
  scripts/frontier.sh \
  scripts/receipt.sh \
  scripts/adopt.sh \
  scripts/enforcement/github-actions-harness.yml \
  scripts/enforcement/claude-code-hooks.json \
  scripts/assert-stop-loss.sh \
  docs/project-adaptation.md \
  docs/harness-reconciliation.md \
  docs/working-modes.md \
  docs/working-with-agents.md \
  plans/README.md; do
  [[ -e "$link" ]] || { echo "missing routed asset: $link" >&2; exit 1; }
done

# The agent protocol is runtime-neutral and ships in template and adopted
# repositories. Keep the five dispatchable roles in lockstep across their
# preamble, return schema, and capability declaration.
for link in \
  scripts/agents/templates/brief.md \
  scripts/agents/templates/follow-up.md \
  scripts/agents/permissions/none.json \
  scripts/agents/permissions/workspace.json \
  harness.conf \
  scripts/harness-config.sh \
  scripts/agents/dispatch.sh \
  scripts/agents/adapters/fake.sh \
  scripts/agents/adapters/runtime-common.sh \
  scripts/agents/adapters/claude-session-signal.py \
  scripts/agents/assert-conformance.sh \
  scripts/assert-critique-closed.sh \
  scripts/assert-return-complete.sh \
  scripts/agents/check-preamble-quotes.sh; do
  [[ -e "$link" ]] || { echo "missing agent protocol asset: $link" >&2; exit 1; }
done

# Real runtime selftests spend model calls and remain manual acceptance steps.
# Validation covers only their static adapter contract.
bash -n scripts/agents/adapters/runtime-common.sh
python3 - scripts/agents/adapters/claude-session-signal.py <<'PY'
import ast, sys
from pathlib import Path
ast.parse(Path(sys.argv[1]).read_text(encoding="utf-8"), filename=sys.argv[1])
PY
for runtime in claude codex devin; do
  adapter="scripts/agents/adapters/$runtime.sh"
  [[ -f "$adapter" ]] || { echo "missing $runtime runtime adapter: $adapter" >&2; exit 1; }
  [[ -x "$adapter" ]] || { echo "$runtime runtime adapter is not executable: $adapter" >&2; exit 1; }
  bash -n "$adapter"
  adapter_usage=$($adapter --help 2>&1)
  for verb in identity probe dispatch follow-up cancel selftest; do
    grep -Fq "adapters/$runtime.sh $verb" <<<"$adapter_usage" \
      || { echo "$runtime adapter usage does not advertise $verb" >&2; exit 1; }
  done
  grep -Fq "adapter_common_init $runtime" "$adapter" \
    || { echo "$runtime adapter does not bind its snapshot runtime identity" >&2; exit 1; }
  grep -Fq "write_capability_snapshot $runtime \"\$version\" \"\$hash\"" "$adapter" \
    || { echo "$runtime adapter does not write its named capability snapshot" >&2; exit 1; }
done
grep -Fq 'path = directory / f"{runtime}-{version}-{config_hash}-{date}-{sequence:03d}.json"' \
  scripts/agents/adapters/runtime-common.sh \
  || { echo "real adapter capability snapshot naming contract drifted" >&2; exit 1; }
for role in design-critic implementer code-critic verifier investigator; do
  for suffix in md requirements.json; do
    [[ -f "scripts/agents/roles/$role.$suffix" ]] \
      || { echo "missing $role role asset: scripts/agents/roles/$role.$suffix" >&2; exit 1; }
  done
  [[ -f "scripts/agents/schemas/$role.schema.json" ]] \
    || { echo "missing $role return schema" >&2; exit 1; }
done

if (( template_mode )); then
  for link in \
    skills/take-a-step-back/SKILL.md \
    skills/take-a-step-back/agents/claude-profile.md \
    skills/take-a-step-back/agents/devin/AGENT.md \
    skills/take-a-step-back/agents/openai.yaml \
    skills/design-critique/SKILL.md \
    skills/design-critique/agents/claude-profile.md \
    skills/design-critique/agents/devin/AGENT.md \
    skills/design-critique/agents/openai.yaml \
    skills/verify/SKILL.md \
    skills/verify/agents/claude-profile.md \
    skills/verify/agents/devin/AGENT.md \
    skills/verify/agents/openai.yaml \
    skills/refactor/SKILL.md \
    skills/refactor/agents/claude-profile.md \
    skills/refactor/agents/devin/AGENT.md \
    skills/refactor/agents/openai.yaml \
    skills/improve/SKILL.md \
    skills/improve/agents/claude-profile.md \
    skills/improve/agents/devin/AGENT.md \
    skills/improve/agents/openai.yaml \
    skills/retro/SKILL.md \
    skills/retro/agents/claude-profile.md \
    skills/retro/agents/devin/AGENT.md \
    skills/retro/agents/openai.yaml; do
    [[ -e "$link" ]] || { echo "missing template skill asset: $link" >&2; exit 1; }
  done
fi

# Registered skills must track their canonical source under skills/: copies
# must not drift, orphaned copies of a pruned skill must not linger, and a
# symlink to a pruned skill is dangling.
for regroot in .claude/skills .agents/skills; do
  [[ -d "$regroot" ]] || continue
  for reg in "$regroot"/*; do
    [[ -e "$reg" || -L "$reg" ]] || continue
    name=$(basename "$reg")
    if [[ -L "$reg" ]]; then
      [[ -e "$reg" ]] || { echo "registered skill link is dangling: $reg" >&2; exit 1; }
      continue
    fi
    [[ -d "$reg" ]] || continue
    [[ -d "skills/$name" ]] || { echo "orphaned registered skill copy: $reg has no skills/$name source" >&2; exit 1; }
    if ! diff -rq "$reg" "skills/$name" >/dev/null 2>&1; then
      echo "registered skill copy has drifted from its source: $reg vs skills/$name" >&2
      exit 1
    fi
  done
done

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The brief contains only orchestrator-authored header fields. Dispatch owns
# job identity, role, runtime, model, and round, so none may appear as a brief
# header before those values exist.
brief=scripts/agents/templates/brief.md
for header in 'Working Mode:' 'Mission Stream:' 'Orchestrator Identity:' 'Date:'; do
  grep -q "^$header" "$brief" || { echo "brief template is missing authored header: $header" >&2; exit 1; }
done
if grep -Eq '^(Job-Id|Role|Runtime|Model|Round):' "$brief"; then
  echo "brief template contains a dispatch-assigned header" >&2
  exit 1
fi
grep -q '^Finding Id:' scripts/agents/templates/follow-up.md \
  || { echo "follow-up template does not restate one finding" >&2; exit 1; }
grep -q '^Disposition:' scripts/agents/templates/follow-up.md \
  || { echo "follow-up template does not restate the disposition" >&2; exit 1; }
grep -q '^# Unchanged Return Contract$' scripts/agents/templates/follow-up.md \
  || { echo "follow-up template can lose the original return contract" >&2; exit 1; }

# Assert the machine-readable shapes item 2 owns. Later dispatcher fixtures
# exercise expansion and capability gating; here the shipped declarations
# must remain minimal and must not grow baseline guarantees into snapshots.
python3 - "$root" <<'PY'
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
common = {"jobId", "round", "runtime", "sessionId", "model", "evidence", "gaps", "mode"}
role_fields = {
    "design-critic": {"findings", "verdictMaterialCount"},
    "implementer": {"riskiestPart", "diffBoundary", "whatWasDone"},
    "code-critic": {"findings", "verdictMaterialCount"},
    "verifier": {"riskiestPart", "whatWasDone"},
    "investigator": {"frozenFrame", "theories", "classifications", "stopLoss"},
}
for role, owned in role_fields.items():
    path = root / "scripts" / "agents" / "schemas" / f"{role}.schema.json"
    schema = json.loads(path.read_text())
    expected = common | owned
    actual = set(schema.get("properties", {}))
    required = set(schema.get("required", []))
    if actual != expected or required != expected or schema.get("additionalProperties") is not False:
        raise SystemExit(f"{role} schema property set drifted from the protocol")

permission_expected = {
    "none": {
        "readRoots": ["."], "writeRoots": [], "network": "deny",
        "approvals": "deny", "tools": "read-only",
    },
    "workspace": {
        "readRoots": ["."], "writeRoots": ["<worktree>"], "network": "deny",
        "approvals": "deny", "tools": "runtime-default",
    },
}
for name, expected in permission_expected.items():
    path = root / "scripts" / "agents" / "permissions" / f"{name}.json"
    if json.loads(path.read_text()) != expected:
        raise SystemExit(f"{name} permission preset drifted from its envelope")

for role in role_fields:
    path = root / "scripts" / "agents" / "roles" / f"{role}.requirements.json"
    requirement = json.loads(path.read_text())
    if not {"required", "optional"}.issubset(requirement) or not set(requirement).issubset({"required", "optional", "waivers"}):
        raise SystemExit(f"{role} capability declaration has unknown top-level fields")
    waivers = requirement.get("waivers", {})
    if not isinstance(waivers, dict) or any(
        not isinstance(field, str) or not isinstance(runtimes, list)
        or not all(isinstance(runtime, str) for runtime in runtimes)
        for field, runtimes in waivers.items()
    ):
        raise SystemExit(f"{role} capability waivers have an invalid shape")
    if requirement["required"] != []:
        raise SystemExit(f"{role} incorrectly repeats adapter-guaranteed baseline capabilities")
    if role == "implementer":
        resume = requirement["optional"].get("resume")
        if set(requirement["optional"]) != {"resume"} or not isinstance(resume, dict) or not resume.get("fallback"):
            raise SystemExit("implementer resume capability lacks its embed fallback")
    elif requirement["optional"] != {}:
        raise SystemExit(f"{role} declares a variable capability it does not need")
PY

# Quote markers name their canonical source. The checker compares the content
# bytes rather than trusting a second prose copy of the binding criterion.
scripts/agents/check-preamble-quotes.sh
cp -R scripts/agents/roles "$tmp/drifted-roles"
sed 's/build something DIFFERENT/build the same thing/' \
  "$tmp/drifted-roles/design-critic.md" >"$tmp/drifted-roles/design-critic.md.new"
mv "$tmp/drifted-roles/design-critic.md.new" "$tmp/drifted-roles/design-critic.md"
set +e
scripts/agents/check-preamble-quotes.sh --roles-dir "$tmp/drifted-roles" >"$tmp/quote-drift.out" 2>&1
quote_status=$?
set -e
if [[ $quote_status -eq 0 ]]; then
  echo "preamble quote checker accepted a drifted binding criterion" >&2
  exit 1
fi
[[ $quote_status -eq 1 ]] \
  || { echo "preamble quote checker used $quote_status instead of exit 1 for drift" >&2; exit 1; }
grep -q 'quote drifted from skills/design-critique/SKILL.md' "$tmp/quote-drift.out" \
  || { echo "preamble quote checker did not name the drifted source" >&2; exit 1; }

# Build canonical positive returns and one role-specific negative per role.
# JSON remains canonical; these fixtures never rely on the Markdown view.
return_fixtures="$tmp/returns"
mkdir -p "$return_fixtures"
python3 - "$return_fixtures" <<'PY'
import copy
import json
import sys
from pathlib import Path

out = Path(sys.argv[1])
common = {
    "jobId": "fixture-job",
    "round": 1,
    "runtime": "fake",
    "sessionId": "session-1",
    "model": {"requested": "fake-model", "effective": "fake-model"},
    "evidence": [{"command": "scripts/validate-harness.sh", "observed": "fixture output", "level": "ran"}],
    "gaps": [],
    "mode": "implement",
}
positive = {
    "design-critic": {
        **common,
        "mode": "design",
        "findings": [{"id": "F-1", "severity": "high", "material": True, "claim": "contract gap", "evidence": "read design"}],
        "verdictMaterialCount": 1,
    },
    "code-critic": {
        **common,
        "findings": [],
        "verdictMaterialCount": 0,
    },
    "implementer": {
        **common,
        "riskiestPart": "schema boundary",
        "diffBoundary": ["scripts/example.sh"],
        "whatWasDone": "implemented the brief",
    },
    "verifier": {
        **common,
        "mode": "verify",
        "riskiestPart": "failure path",
        "whatWasDone": "drove the runnable surface",
    },
    "investigator": {
        **common,
        "mode": "take-a-step-back",
        "frozenFrame": "symptom and boundary frozen",
        "theories": [{"statement": "owner lost state", "evidenceFor": "trace", "evidenceAgainst": "focused check"}],
        "classifications": ["falsified-continue"],
        "stopLoss": {"triggered": False, "trigger": None},
    },
}
for role, value in positive.items():
    (out / f"{role}-positive.json").write_text(json.dumps(value, indent=2) + "\n")

negative = copy.deepcopy(positive)
negative["design-critic"].pop("findings")
negative["design-critic"].pop("verdictMaterialCount")
negative["code-critic"]["whatWasDone"] = "critics do not own this section"
negative["implementer"].pop("diffBoundary")
negative["verifier"]["diffBoundary"] = ["not verifier-owned"]
negative["investigator"].pop("frozenFrame")
negative["investigator"].pop("theories")
for role, value in negative.items():
    (out / f"{role}-negative.json").write_text(json.dumps(value, indent=2) + "\n")

miscount = copy.deepcopy(positive["design-critic"])
miscount["verdictMaterialCount"] = 0
(out / "critic-miscount.json").write_text(json.dumps(miscount, indent=2) + "\n")
missing_verdict = copy.deepcopy(positive["design-critic"])
missing_verdict.pop("verdictMaterialCount")
(out / "critic-missing-verdict.json").write_text(json.dumps(missing_verdict, indent=2) + "\n")
PY

for role in design-critic implementer code-critic verifier investigator; do
  scripts/assert-return-complete.sh --role "$role" --file "$return_fixtures/$role-positive.json"
done

check_bad_return() { # role, file, required diagnostic text
  local role=$1 file=$2 expected=$3 output status
  output="$tmp/${role}-negative.out"
  set +e
  scripts/assert-return-complete.sh --role "$role" --file "$file" >"$output" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "return checker accepted the negative $role fixture" >&2
    exit 1
  fi
  [[ $status -eq 1 ]] \
    || { echo "return checker used $status instead of exit 1 for $role validation" >&2; exit 1; }
  grep -Fq "$expected" "$output" \
    || { echo "return checker did not name the $role violation: $expected" >&2; exit 1; }
}

check_bad_return design-critic "$return_fixtures/design-critic-negative.json" '$.findings is required'
check_bad_return code-critic "$return_fixtures/code-critic-negative.json" '$.whatWasDone is not allowed'
check_bad_return implementer "$return_fixtures/implementer-negative.json" '$.diffBoundary is required'
check_bad_return verifier "$return_fixtures/verifier-negative.json" '$.diffBoundary is not allowed'
check_bad_return investigator "$return_fixtures/investigator-negative.json" '$.frozenFrame is required'
check_bad_return design-critic "$return_fixtures/critic-missing-verdict.json" '$.verdictMaterialCount is required'
check_bad_return design-critic "$return_fixtures/critic-miscount.json" '$.verdictMaterialCount must equal the count of findings with material=true'

set +e
scripts/assert-return-complete.sh >"$tmp/return-usage.out" 2>&1
return_usage_status=$?
set -e
[[ $return_usage_status -eq 2 ]] \
  || { echo "return checker used $return_usage_status instead of exit 2 for usage" >&2; exit 1; }

# Critique closure joins the canonical return JSON against the one Markdown
# dispositions table. Reuse the item-2 return fixture shape and vary only the
# findings and table rows needed to exercise each join invariant.
critique_fixtures="$tmp/critiques"
mkdir -p "$critique_fixtures"
python3 - "$return_fixtures/design-critic-positive.json" "$critique_fixtures" <<'PY'
import copy
import json
import sys
from pathlib import Path

source = json.loads(Path(sys.argv[1]).read_text())
out = Path(sys.argv[2])
material = {
    "id": "F-1", "severity": "high", "material": True,
    "claim": "contract gap", "evidence": "read design",
}
nonmaterial = {
    "id": "F-2", "severity": "low", "material": False,
    "claim": "wording issue", "evidence": "read design",
}
second_material = {
    "id": "F-3", "severity": "medium", "material": True,
    "claim": "incorrect premise", "evidence": "checked implementation",
}


def write_return(name, findings):
    value = copy.deepcopy(source)
    value["findings"] = findings
    value["verdictMaterialCount"] = sum(1 for finding in findings if finding["material"])
    (out / f"{name}.json").write_text(json.dumps(value, indent=2) + "\n")


def write_table(name, rows, separator="| --- | --- | --- | --- |"):
    lines = [
        "| Finding id | Disposition | Reasoning and evidence | Amendment |",
        separator,
        *[f"| {finding_id} | {disposition} | {reasoning} | {amendment} |"
          for finding_id, disposition, reasoning, amendment in rows],
    ]
    (out / f"{name}.md").write_text("\n".join(lines) + "\n")


write_return("joinable", [material, nonmaterial, second_material])
write_table("all-disposed", [
    ("F-1", "accepted", "design amended", "section 3"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("open-material", [
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("noted-on-material", [
    ("F-1", "noted", "incorrect disposition", "none"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("missing-nonmaterial-disposition", [
    ("F-1", "accepted", "design amended", "section 3"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("unknown-disposition", [
    ("F-1", "dismissed", "not a protocol value", "none"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])
write_table("unknown-finding-id", [
    ("F-1", "accepted", "design amended", "section 3"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
    ("F-404", "refuted", "no matching finding", "none"),
])

write_return("duplicate-id", [material, material, nonmaterial, second_material])
write_table("duplicate-id", [
    ("F-1", "accepted", "first row", "section 3"),
    ("F-1", "refuted", "second row", "none"),
    ("F-2", "noted", "does not change implementation", "none"),
    ("F-3", "refuted", "implementation disproves the claim", "none"),
])

missing_findings = copy.deepcopy(source)
missing_findings.pop("findings")
(out / "unjoinable-missing-findings.json").write_text(
    json.dumps(missing_findings, indent=2) + "\n"
)
write_table("unjoinable-malformed-table", [], separator="| --- | --- | --- |")
PY

scripts/assert-critique-closed.sh \
  --findings "$critique_fixtures/joinable.json" \
  --dispositions "$critique_fixtures/all-disposed.md"

check_open_critique() { # fixture name, findings file, dispositions file, diagnostics...
  local name=$1 findings_file=$2 dispositions_file=$3 output status expected
  shift 3
  output="$tmp/critique-$name.out"
  set +e
  scripts/assert-critique-closed.sh \
    --findings "$findings_file" \
    --dispositions "$dispositions_file" >"$output" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "critique checker accepted the negative $name fixture" >&2
    exit 1
  fi
  [[ $status -eq 1 ]] \
    || { echo "critique checker used $status instead of exit 1 for $name" >&2; exit 1; }
  for expected in "$@"; do
    grep -Fq "$expected" "$output" || {
      echo "critique checker did not name the $name violation: $expected" >&2
      cat "$output" >&2
      exit 1
    }
  done
}

check_open_critique open-material \
  "$critique_fixtures/joinable.json" "$critique_fixtures/open-material.md" \
  "finding id 'F-1' has no disposition row"
check_open_critique noted-on-material \
  "$critique_fixtures/joinable.json" "$critique_fixtures/noted-on-material.md" \
  "material finding id 'F-1' cannot use disposition 'noted'"
check_open_critique missing-nonmaterial-disposition \
  "$critique_fixtures/joinable.json" "$critique_fixtures/missing-nonmaterial-disposition.md" \
  "finding id 'F-2' has no disposition row"
check_open_critique duplicate-id \
  "$critique_fixtures/duplicate-id.json" "$critique_fixtures/duplicate-id.md" \
  "duplicate finding id: 'F-1'" "duplicate disposition id: 'F-1'"
check_open_critique unknown-disposition \
  "$critique_fixtures/joinable.json" "$critique_fixtures/unknown-disposition.md" \
  "disposition for finding id 'F-1' has unknown value 'dismissed'"
check_open_critique unknown-finding-id \
  "$critique_fixtures/joinable.json" "$critique_fixtures/unknown-finding-id.md" \
  "disposition names unknown finding id: 'F-404'"
check_open_critique unjoinable-format-missing-findings \
  "$critique_fixtures/unjoinable-missing-findings.json" "$critique_fixtures/all-disposed.md" \
  '$.findings array is missing'
check_open_critique unjoinable-format-malformed-table \
  "$critique_fixtures/joinable.json" "$critique_fixtures/unjoinable-malformed-table.md" \
  "malformed dispositions table: invalid separator row"

set +e
scripts/assert-critique-closed.sh >"$tmp/critique-usage.out" 2>&1
critique_usage_status=$?
set -e
[[ $critique_usage_status -eq 2 ]] \
  || { echo "critique checker used $critique_usage_status instead of exit 2 for usage" >&2; exit 1; }

# Job mode derives the schema and return path from the record and then checks
# all four identity fields, one at a time, against a schema-valid return.
job_fixture="$tmp/job-harness"
mkdir -p "$job_fixture/scripts/agents" \
  "$job_fixture/artifacts/agents/jobs" \
  "$job_fixture/artifacts/agents/fixture-job/rounds/1"
cp scripts/assert-return-complete.sh "$job_fixture/scripts/"
cp -R scripts/agents/schemas "$job_fixture/scripts/agents/"
cat >"$job_fixture/artifacts/agents/jobs/fixture-job.json" <<'EOF'
{
  "jobId": "fixture-job",
  "role": "implementer",
  "round": 1,
  "parentJob": null,
  "runtime": "fake",
  "sessionId": "session-1"
}
EOF
cp "$return_fixtures/implementer-positive.json" \
  "$job_fixture/artifacts/agents/fixture-job/rounds/1/return.json"
(cd "$job_fixture" && scripts/assert-return-complete.sh --job fixture-job)

mkdir -p "$job_fixture/artifacts/agents/fixture-job/rounds/2"
cat >"$job_fixture/artifacts/agents/jobs/fixture-job-r2.json" <<'EOF'
{
  "jobId": "fixture-job-r2",
  "role": "implementer",
  "round": 2,
  "parentJob": "fixture-job",
  "runtime": "fake",
  "sessionId": "session-2"
}
EOF
python3 - "$return_fixtures/implementer-positive.json" \
  "$job_fixture/artifacts/agents/fixture-job/rounds/2/return.json" <<'PY'
import json
import sys
from pathlib import Path

value = json.loads(Path(sys.argv[1]).read_text())
value.update({"jobId": "fixture-job-r2", "round": 2, "sessionId": "session-2"})
Path(sys.argv[2]).write_text(json.dumps(value, indent=2) + "\n")
PY
(cd "$job_fixture" && scripts/assert-return-complete.sh --job fixture-job-r2)

python3 - "$return_fixtures/implementer-positive.json" "$return_fixtures" <<'PY'
import copy
import json
import sys
from pathlib import Path

source = json.loads(Path(sys.argv[1]).read_text())
out = Path(sys.argv[2])
for field, value in {
    "jobId": "other-job",
    "round": 2,
    "runtime": "other-runtime",
    "sessionId": "other-session",
}.items():
    changed = copy.deepcopy(source)
    changed[field] = value
    (out / f"identity-{field}.json").write_text(json.dumps(changed, indent=2) + "\n")
PY
for field in jobId round runtime sessionId; do
  cp "$return_fixtures/identity-$field.json" \
    "$job_fixture/artifacts/agents/fixture-job/rounds/1/return.json"
  set +e
  (cd "$job_fixture" && scripts/assert-return-complete.sh --job fixture-job) >"$tmp/identity-$field.out" 2>&1
  identity_status=$?
  set -e
  if [[ $identity_status -eq 0 ]]; then
    echo "job-aware return checker accepted a mismatched $field" >&2
    exit 1
  fi
  [[ $identity_status -eq 1 ]] \
    || { echo "job-aware return checker used $identity_status instead of exit 1 for $field" >&2; exit 1; }
  grep -Fq "$.${field} identity mismatch" "$tmp/identity-$field.out" \
    || { echo "job-aware return checker did not name the $field mismatch" >&2; exit 1; }
done

# Dispatcher and fake-adapter fixtures run in a minimal adopted-mode Git
# repository. Keeping their artifacts outside this checkout proves that the
# scripts derive every path from the repository they ship into. Nested
# adoption validations inherit the skip after this block, avoiding duplicate
# process-lifecycle runs while a direct adopted-repository validation still
# exercises the full contract.
if [[ -z "${HARNESS_SKIP_AGENT_FIXTURES:-}" ]]; then
  agent_fixture="$tmp/agent-fixture"
  agent_repo="$agent_fixture/repo"
  agent_evidence="$agent_fixture/evidence"
  mkdir -p "$agent_repo/scripts"
  agent_repo=$(cd "$agent_repo" && pwd -P)
  cp -R scripts/agents "$agent_repo/scripts/"
  cp scripts/harness-config.sh scripts/assert-return-complete.sh "$agent_repo/scripts/"
  cp harness.conf "$agent_repo/"
  perl -0pi -e 's/^harness\.runtimes=.*$/harness.runtimes=fake/m; s|^evidence\.root=.*$|evidence.root='"$agent_evidence"'|m; s/^model\.tier\.1=.*$/model.tier.1=fake:fake-model/m; s/^model\.tier\.2=.*$/model.tier.2=/m; s/^model\.tier\.3=.*$/model.tier.3=/m; s/^role\.default\.runtime=.*$/role.default.runtime=fake/m; s/^role\.default\.model\.codex=.*$/role.default.model.fake=fake-model/m; s/^role\.default\.model\.(?:claude|devin)=.*\n//mg; s/\.runtime=(?:codex|devin)$/\.runtime=fake/mg; s/\.model\.(?:codex|devin)=.*$/\.model.fake=fake-model/mg' "$agent_repo/harness.conf"
  git -C "$agent_repo" init -q
  git -C "$agent_repo" add .
  git -C "$agent_repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm base
  agent_dispatch="$agent_repo/scripts/agents/dispatch.sh"
  fake_adapter="$agent_repo/scripts/agents/adapters/fake.sh"
  agent_config="$agent_repo/scripts/harness-config.sh"
  good_agent_conf="$agent_fixture/good-harness.conf"
  cp "$agent_repo/harness.conf" "$good_agent_conf"

  agent_fixture_timeout_sec=${HARNESS_AGENT_FIXTURE_TIMEOUT_SEC:-20}
  [[ "$agent_fixture_timeout_sec" =~ ^[1-9][0-9]*$ && "$agent_fixture_timeout_sec" -le 120 ]] \
    || { echo "HARNESS_AGENT_FIXTURE_TIMEOUT_SEC must be an integer from 1 through 120" >&2; exit 1; }

  agent_fixture_job_from_args() {
    local previous= argument
    for argument in "$@"; do
      if [[ "$previous" == --job || "$previous" == --job-id ]]; then
        printf '%s\n' "$argument"
        return
      fi
      previous=$argument
    done
    printf '%s\n' -
  }

  agent_fixture_diagnostics() { # fixture name, job id or -
    local name=$1 job=$2 path
    echo "agent fixture timed out after ${agent_fixture_timeout_sec}s: $name (job: $job)" >&2
    [[ "$job" != - ]] || return
    for path in \
      "$agent_repo/artifacts/agents/jobs/$job.json" \
      "$agent_repo/artifacts/agents/jobs/$job.log" \
      "$agent_repo/artifacts/agents/hb/$job.start" \
      "$agent_repo/artifacts/agents/hb/$job" \
      "$agent_repo/artifacts/agents/hb/$job.waiting"; do
      if [[ -e "$path" ]]; then
        echo "--- $path" >&2
        sed -n '1,240p' "$path" >&2
      else
        echo "--- missing: $path" >&2
      fi
    done
  }

  stop_timed_out_agent_fixture() { # fixture name, job id or -, driver pid
    local name=$1 job=$2 driver_pid=$3 cleanup_pid cleanup_deadline driver_deadline status=
    agent_fixture_diagnostics "$name" "$job"
    if [[ "$job" != - && -f "$agent_repo/artifacts/agents/jobs/$job.json" ]]; then
      status=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("status", "malformed"))' \
        "$agent_repo/artifacts/agents/jobs/$job.json" 2>/dev/null || true)
      if [[ "$status" == pending || "$status" == running ]]; then
        "$agent_dispatch" cancel --job "$job" >"$agent_fixture/$name-timeout-cancel.out" 2>&1 &
        cleanup_pid=$!
        cleanup_deadline=$(( SECONDS + 7 ))
        while kill -0 "$cleanup_pid" 2>/dev/null && (( SECONDS < cleanup_deadline )); do sleep 0.05; done
        if kill -0 "$cleanup_pid" 2>/dev/null; then
          echo "agent fixture cleanup timed out: $name cancel pid $cleanup_pid" >&2
          kill -TERM "$cleanup_pid" 2>/dev/null || true
          sleep 0.1
          kill -KILL "$cleanup_pid" 2>/dev/null || true
        fi
      fi
    fi
    if kill -0 "$driver_pid" 2>/dev/null; then
      kill -TERM "$driver_pid" 2>/dev/null || true
      driver_deadline=$(( SECONDS + 2 ))
      while kill -0 "$driver_pid" 2>/dev/null && (( SECONDS < driver_deadline )); do sleep 0.05; done
      if kill -0 "$driver_pid" 2>/dev/null; then
        echo "agent fixture driver survived TERM: $name pid $driver_pid; sending KILL" >&2
        kill -KILL "$driver_pid" 2>/dev/null || true
      fi
    fi
    exit 1
  }

  wait_for_agent_fixture_process() { # fixture name, job id or -, exact child pid
    local name=$1 job=$2 child_pid=$3 deadline=$(( SECONDS + agent_fixture_timeout_sec )) result
    echo "agent fixture wait: $name (job: $job; cap: ${agent_fixture_timeout_sec}s)" >&2
    while kill -0 "$child_pid" 2>/dev/null; do
      (( SECONDS < deadline )) || stop_timed_out_agent_fixture "$name" "$job" "$child_pid"
      sleep 0.05
    done
    if wait "$child_pid"; then result=0; else result=$?; fi
    return "$result"
  }

  run_agent_fixture() { # fixture name, job id or -, command...
    local name=$1 job=$2 child_pid
    shift 2
    "$@" &
    child_pid=$!
    wait_for_agent_fixture_process "$name" "$job" "$child_pid"
  }

  run_agent_fixture_captured() { # fixture name, job id or -, output file, command...
    local name=$1 job=$2 output=$3 child_pid
    shift 3
    "$@" >"$output" 2>&1 &
    child_pid=$!
    wait_for_agent_fixture_process "$name" "$job" "$child_pid"
  }

  agent_fails() { # output name, expected text, command...
    local name=$1 expected=$2 result job
    shift 2
    job=$(agent_fixture_job_from_args "$@")
    set +e
    run_agent_fixture_captured "$name" "$job" "$agent_fixture/$name.out" "$@"
    result=$?
    set -e
    if [[ $result -eq 0 ]]; then
      echo "agent fixture unexpectedly passed: $name" >&2
      exit 1
    fi
    [[ -z "$expected" ]] || grep -Fq "$expected" "$agent_fixture/$name.out" || {
      echo "agent fixture $name did not report: $expected" >&2
      cat "$agent_fixture/$name.out" >&2
      exit 1
    }
  }

  make_agent_brief() { # output, mode, optional marker/value lines...
    local output=$1 mode=$2
    shift 2
    sed "s/^Working Mode:.*/Working Mode: $mode/" "$agent_repo/scripts/agents/templates/brief.md" >"$output"
    for line in "$@"; do printf '\n%s\n' "$line" >>"$output"; done
  }

  wait_for_agent_status() { # job, expected
    local job=$1 expected=$2 observed= i
    echo "agent fixture status wait: $job -> $expected (cap: 5s)" >&2
    for ((i=0; i<100; i++)); do
      observed=$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job "$job" 2>/dev/null || true)
      [[ "$observed" == "$expected" ]] && return 0
      sleep 0.05
    done
    echo "agent fixture $job did not reach $expected (last status: ${observed:-missing})" >&2
    return 1
  }

  # Configuration resolution is flag, environment, mode, plain, default.
  config_order="$agent_fixture/config-order"
  mkdir -p "$config_order/scripts"
  cp scripts/harness-config.sh "$config_order/scripts/"
  cat >"$config_order/harness.conf" <<EOF
role.implementer.runtime=plain
mode.refactor.role.implementer.runtime=mode
plain.knob=plain-value
EOF
  [[ "$("$config_order/scripts/harness-config.sh" get --key role.implementer.runtime --mode refactor --flag flag)" == flag ]] \
    || { echo "harness config did not prefer the flag" >&2; exit 1; }
  [[ "$(HARNESS_ROLE_IMPLEMENTER_RUNTIME=environment "$config_order/scripts/harness-config.sh" get --key role.implementer.runtime --mode refactor)" == environment ]] \
    || { echo "harness config did not prefer the environment" >&2; exit 1; }
  [[ "$(env -u HARNESS_ROLE_IMPLEMENTER_RUNTIME "$config_order/scripts/harness-config.sh" get --key role.implementer.runtime --mode refactor)" == mode ]] \
    || { echo "harness config did not resolve the mode scope" >&2; exit 1; }
  [[ "$("$config_order/scripts/harness-config.sh" get --key plain.knob --mode refactor)" == plain-value ]] \
    || { echo "harness config did not resolve the plain key" >&2; exit 1; }
  [[ "$("$config_order/scripts/harness-config.sh" get --key absent.knob --default built-in)" == built-in ]] \
    || { echo "harness config did not resolve the built-in default" >&2; exit 1; }

  "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's/^role\.design-critic\.runtime=.*$/role.design-critic.runtime=ghost/m' "$agent_repo/harness.conf"
  agent_fails invalid-role-runtime 'outside harness.runtimes' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's/^mode\.refactor\.role\.implementer\.runtime=.*$/mode.refactor.role.implementer.runtime=ghost/m' "$agent_repo/harness.conf"
  agent_fails invalid-mode-runtime 'outside harness.runtimes' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's/^model\.tier\.1=.*$/model.tier.1=/m' "$agent_repo/harness.conf"
  agent_fails unmapped-model 'appears in 0 model tiers' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's/^model\.tier\.2=.*$/model.tier.2=fake:fake-model/m' "$agent_repo/harness.conf"
  agent_fails duplicate-model-tier 'appears in 2 model tiers' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's/^role\.design-critic\.model\.fake=.*\n//m; s/^role\.default\.model\.fake=.*\n//m' "$agent_repo/harness.conf"
  agent_fails missing-runtime-model 'has no model.fake value' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's|^evidence\.root=.*$|evidence.root='"$agent_repo"'/evidence|m' "$agent_repo/harness.conf"
  agent_fails inside-evidence-root 'outside the repository' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"

  first_snapshot=$($fake_adapter probe)
  second_snapshot=$($fake_adapter probe)
  [[ "$first_snapshot" == *-001.json && "$second_snapshot" == *-002.json && "$first_snapshot" != "$second_snapshot" ]] \
    || { echo "fake probe did not create immutable sequence-suffixed snapshots" >&2; exit 1; }

  happy_brief="$agent_fixture/happy.md"
  make_agent_brief "$happy_brief" design
  run_agent_fixture happy happy "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id happy --wait
  [[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job happy)" == completed ]] \
    || { echo "valid fake dispatch did not complete" >&2; exit 1; }
  python3 - "$agent_repo" <<'PY'
import json, re, sys
from pathlib import Path
root = Path(sys.argv[1])
record = json.loads((root / "artifacts/agents/jobs/happy.json").read_text())
required = {
    "jobId", "role", "mission", "runtime", "round", "parentJob", "status", "phase", "error",
    "workspaceRoot", "baseSha", "branch", "permissions", "capMin", "pid", "pgid", "instanceTag",
    "sessionId", "turnId", "requestedModel", "effectiveModel", "overridden", "capabilitySnapshot",
    "input", "startedAt", "endedAt", "usage", "mirror",
}
assert required.issubset(record) and record["status"] == "completed"
assert record["capabilitySnapshot"].endswith("-002.json")
assert record["permissions"]["requested"]["preset"] == "none"
assert record["input"]["delivery"] == "stdin" and record["input"]["bytes"] > 0
prompt = (root / "artifacts/agents/happy/rounds/1/prompt.md").read_text()
assert prompt.startswith("Job-Id: happy\nRole: design-critic\nRuntime: fake\nModel: fake-model\nRound: 1\n")
preamble = (root / "scripts/agents/roles/design-critic.md").read_text().rstrip("\n")
brief = (root / "artifacts/agents/happy/brief.md").read_text().rstrip("\n")
assert prompt.index(preamble) < prompt.index(brief)
PY
  mkdir -p "$agent_repo/artifacts/agents/locks/stale-lock.d"
  printf '{"pid":999999,"instanceTag":"dead-owner","acquiredAt":"2000-01-01T00:00:00Z"}\n' >"$agent_repo/artifacts/agents/locks/stale-lock.d/owner.json"
  run_agent_fixture stale-lock stale-lock "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id stale-lock --wait

  generated=$(run_agent_fixture generated-dispatch - "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief")
  [[ "$generated" =~ ^design-critic-[0-9]{8}t[0-9]{6}z-[a-f0-9]{4}$ ]] \
    || { echo "generated job id does not match the lowercase grammar: $generated" >&2; exit 1; }
  agent_fails collision 'job id collision' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id happy
  agent_fails malformed-job-id 'invalid job id' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id 'Bad_Id'
  agent_fails contradictory-mode "contradicts the brief's Working Mode" "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --mode verify
  agent_fails unregistered-override 'outside harness.runtimes' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --runtime ghost
  agent_fails main-override 'assigned to main' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --runtime main
  agent_fails costlier-unmapped 'requires human approval' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model absent-from-tier

  # The recorded default is a real fallback, while its absence refuses.
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's/^role\.verifier\.runtime=.*\n//m' "$agent_repo/harness.conf"
  verifier_brief="$agent_fixture/verifier.md"
  make_agent_brief "$verifier_brief" verify
  run_agent_fixture default-role default-role "$agent_dispatch" dispatch --role verifier --brief "$verifier_brief" --permissions none --job-id default-role --wait
  cp "$agent_repo/harness.conf" "$agent_fixture/no-role.conf"
  perl -0pi -e 's/^role\.default\.runtime=.*\n//m' "$agent_repo/harness.conf"
  agent_fails no-role-default 'neither a runtime entry nor role.default.runtime' "$agent_dispatch" dispatch --role verifier --brief "$verifier_brief" --permissions none
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  code_brief="$agent_fixture/code.md"
  make_agent_brief "$code_brief" implement
  run_agent_fixture flag-runtime flag-runtime "$agent_dispatch" dispatch --role code-critic --brief "$code_brief" --runtime fake --permissions none --job-id flag-runtime --wait
  python3 - "$agent_repo/artifacts/agents/jobs/flag-runtime.json" <<'PY'
import json, sys
record = json.load(open(sys.argv[1])); assert record["runtime"] == "fake" and record["overridden"] is True
PY
  investigator_brief="$agent_fixture/investigator.md"
  make_agent_brief "$investigator_brief" take-a-step-back
  run_agent_fixture investigator-role investigator-role "$agent_dispatch" dispatch --role investigator --brief "$investigator_brief" --runtime fake --permissions none --job-id investigator-role --wait

  no_signal="$agent_fixture/no-signal.md"
  make_agent_brief "$no_signal" design 'FAKE:no-session-signal'
  agent_fails no-session-signal '' "$agent_dispatch" dispatch --role design-critic --brief "$no_signal" --job-id no-signal --wait
  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["status"])' "$agent_repo/artifacts/agents/jobs/no-signal.json")" == failed ]] \
    || { echo "non-signal handshake did not end failed" >&2; exit 1; }
  grep -Fq 'handshake_timeout' "$agent_repo/artifacts/agents/jobs/no-signal.json" \
    || { echo "non-signal handshake did not retain its error" >&2; exit 1; }
  handshake_failure="$agent_fixture/handshake-failure.md"
  make_agent_brief "$handshake_failure" design 'FAKE:handshake-failure'
  agent_fails handshake-failure '' "$agent_dispatch" dispatch --role design-critic --brief "$handshake_failure" --job-id handshake-failure --wait
  grep -Fq 'authentication_failed' "$agent_repo/artifacts/agents/jobs/handshake-failure.json" \
    || { echo "fake handshake failure lost its named error" >&2; exit 1; }
  missing_session="$agent_fixture/missing-session.md"
  make_agent_brief "$missing_session" design 'FAKE:missing-session-id'
  agent_fails missing-session '' "$agent_dispatch" dispatch --role design-critic --brief "$missing_session" --job-id missing-session --wait
  grep -Fq 'handshake_missing_session_id' "$agent_repo/artifacts/agents/jobs/missing-session.json" \
    || { echo "missing session id did not fail the strong handshake" >&2; exit 1; }

  pending_brief="$agent_fixture/pending.md"
  make_agent_brief "$pending_brief" design 'FAKE:no-session-signal'
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh dispatch --role design-critic --brief "$pending_brief" --job-id pending-chain >/dev/null 2>&1) & pending_driver=$!
  wait_for_agent_status pending-chain pending
  pending_message="$agent_fixture/pending-follow.md"
  cp "$agent_repo/scripts/agents/templates/follow-up.md" "$pending_message"
  agent_fails pending-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job pending-chain --message "$pending_message"
  wait_for_agent_fixture_process pending-chain-driver pending-chain "$pending_driver" || true

  pending_loss_brief="$agent_fixture/pending-loss.md"
  make_agent_brief "$pending_loss_brief" design 'FAKE:pending-process-loss'
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh dispatch --role design-critic --brief "$pending_loss_brief" --job-id pending-loss >/dev/null 2>&1) & pending_loss_driver=$!
  wait_for_agent_status pending-loss pending
  run_agent_fixture pending-loss-reap pending-loss "$agent_dispatch" reap --job pending-loss
  wait_for_agent_fixture_process pending-loss-driver pending-loss "$pending_loss_driver" || true
  grep -Fq 'process-lost' "$agent_repo/artifacts/agents/jobs/pending-loss.json" \
    || { echo "dead pending supervisor did not transition through reap" >&2; exit 1; }

  malformed_brief="$agent_fixture/malformed-return.md"
  make_agent_brief "$malformed_brief" design 'FAKE:malformed-return'
  set +e
  run_agent_fixture malformed-return malformed-return "$agent_dispatch" dispatch --role design-critic --brief "$malformed_brief" --job-id malformed-return --wait
  malformed_status=$?
  set -e
  [[ $malformed_status -eq 3 ]] || { echo "malformed return mapped to $malformed_status instead of 3" >&2; exit 1; }
  grep -Fq 'protocol_error' "$agent_repo/artifacts/agents/jobs/malformed-return.json" \
    || { echo "malformed return did not record protocol_error" >&2; exit 1; }

  interrupted="$agent_fixture/interrupted.md"
  make_agent_brief "$interrupted" design 'FAKE:interrupted-atomic-write'
  run_agent_fixture interrupted interrupted "$agent_dispatch" dispatch --role design-critic --brief "$interrupted" --job-id interrupted --wait
  [[ -f "$agent_repo/artifacts/agents/record-locks/interrupted.interrupted" ]] \
    || { echo "interrupted atomic-write fixture did not leave its staged partial" >&2; exit 1; }
  python3 -m json.tool "$agent_repo/artifacts/agents/jobs/interrupted.json" >/dev/null
  terminal_patch="$agent_fixture/terminal-race.json"
  printf '{"error":"loser"}\n' >"$terminal_patch"
  set +e
  run_agent_fixture_captured terminal-race interrupted /dev/null "$agent_dispatch" __record-cas --job interrupted --expect running --status failed --patch "$terminal_patch"
  terminal_race_status=$?
  set -e
  [[ $terminal_race_status -eq 3 && "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job interrupted)" == completed ]] \
    || { echo "terminal compare-and-set did not preserve the first writer" >&2; exit 1; }
  agent_fails illegal-terminal-transition 'illegal job transition' "$agent_dispatch" __record-cas --job interrupted --expect completed --status failed --patch "$terminal_patch"

  effective_wider="$agent_fixture/effective-wider.md"
  make_agent_brief "$effective_wider" design 'FAKE:effective-wider'
  agent_fails effective-wider '' "$agent_dispatch" dispatch --role design-critic --brief "$effective_wider" --job-id effective-wider --wait
  grep -Fq 'permissions_mismatch:network' "$agent_repo/artifacts/agents/jobs/effective-wider.json" \
    || { echo "wider effective envelope did not record the mismatch" >&2; exit 1; }
  permissive_permissions="$agent_fixture/permissive-permissions.json"
  printf '{"readRoots":["."],"writeRoots":[],"network":"allow","approvals":"deny","tools":"read-only"}\n' >"$permissive_permissions"
  effective_narrower="$agent_fixture/effective-narrower.md"
  make_agent_brief "$effective_narrower" design 'FAKE:effective-narrower'
  run_agent_fixture effective-narrower effective-narrower "$agent_dispatch" dispatch --role design-critic --brief "$effective_narrower" --permissions "$permissive_permissions" --job-id effective-narrower --wait

  agent_fails writable-without-worktree 'writable permissions require --worktree' \
    "$agent_dispatch" dispatch --role implementer --brief "$code_brief" --job-id no-worktree
  custom_permissions="$agent_fixture/custom-permissions.json"
  cat >"$custom_permissions" <<EOF
{"readRoots":["."],"writeRoots":["$agent_fixture/outside"],"network":"deny","approvals":"deny","tools":"runtime-default"}
EOF
  agent_fails escaping-write-root 'escapes the job worktree' "$agent_dispatch" dispatch --role implementer --brief "$code_brief" --job-id escaping-root --worktree --permissions "$custom_permissions"

  oversized="$agent_fixture/oversized.md"
  make_agent_brief "$oversized" design 'FAKE:oversized-input'
  head -c 70000 /dev/zero | tr '\0' x >>"$oversized"
  agent_fails oversized-input 'pass a file reference' "$agent_dispatch" dispatch --role design-critic --brief "$oversized" --job-id oversized

  # Reap owns process loss, absolute caps, group death, and terminal mirroring.
  process_loss="$agent_fixture/process-loss.md"
  make_agent_brief "$process_loss" design 'FAKE:process-loss'
  set +e
  run_agent_fixture process-loss process-loss "$agent_dispatch" dispatch --role design-critic --brief "$process_loss" --job-id process-loss --wait
  process_loss_status=$?
  set -e
  [[ $process_loss_status -eq 3 ]] || { echo "process loss mapped to $process_loss_status instead of 3" >&2; exit 1; }
  grep -Fq 'process-lost' "$agent_repo/artifacts/agents/jobs/process-loss.json" \
    || { echo "reap did not name process-lost" >&2; exit 1; }
  [[ -f "$agent_repo/artifacts/agents/process-loss/rounds/1/child.stopped" ]] \
    || { echo "reap did not TERM the orphaned process-loss child" >&2; exit 1; }
  grep -Fq 'groupDeathProvenAt' "$agent_repo/artifacts/agents/jobs/process-loss.json" \
    || { echo "process-loss terminal record lacks group-death proof" >&2; exit 1; }

  timeout_brief="$agent_fixture/timeout.md"
  make_agent_brief "$timeout_brief" design 'FAKE:timeout'
  timeout_result="$agent_fixture/timeout.status"
  (
    set +e
    cd "$agent_repo"
    scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id timed --cap-min 1 --wait
    printf '%s\n' "$?" >"$timeout_result"
  ) &
  timeout_driver=$!
  wait_for_agent_status timed running
  python3 - "$agent_repo/artifacts/agents/jobs/timed.json" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["startedAt"] = "2000-01-01T00:00:00Z"; path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  run_agent_fixture timed-reap timed "$agent_dispatch" reap --job timed
  wait_for_agent_fixture_process timed-driver timed "$timeout_driver"
  [[ "$(cat "$timeout_result")" == 4 ]] || { echo "timeout did not map to wait exit 4 (got $(cat "$timeout_result"))" >&2; exit 1; }
  grep -Fq 'budget-cap' "$agent_repo/artifacts/agents/jobs/timed.json" \
    || { echo "absolute cap did not record budget-cap" >&2; exit 1; }
  [[ -f "$agent_repo/artifacts/agents/timed/rounds/1/child.stopped" ]] \
    || { echo "timeout did not TERM the whole owned group" >&2; exit 1; }
  grep -Fq 'groupDeathProvenAt' "$agent_repo/artifacts/agents/jobs/timed.json" \
    || { echo "timeout terminal record lacks group-death proof" >&2; exit 1; }

  cancel_result="$agent_fixture/cancel.status"
  (
    set +e
    cd "$agent_repo"
    scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id cancelled --wait
    printf '%s\n' "$?" >"$cancel_result"
  ) &
  cancel_driver=$!
  wait_for_agent_status cancelled running
  run_agent_fixture cancelled-cancel cancelled "$agent_dispatch" cancel --job cancelled
  wait_for_agent_fixture_process cancelled-driver cancelled "$cancel_driver"
  [[ "$(cat "$cancel_result")" == 8 ]] || { echo "cancelled did not map to wait exit 8" >&2; exit 1; }
  [[ -f "$agent_repo/artifacts/agents/cancelled/rounds/1/child.stopped" ]] \
    || { echo "cancel did not TERM the whole owned group" >&2; exit 1; }
  grep -Fq 'groupDeathProvenAt' "$agent_repo/artifacts/agents/jobs/cancelled.json" \
    || { echo "cancelled terminal record lacks group-death proof" >&2; exit 1; }

  vanished_result="$agent_fixture/vanished.status"
  (
    set +e
    cd "$agent_repo"
    scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id vanished --wait
    printf '%s\n' "$?" >"$vanished_result"
  ) &
  vanished_driver=$!
  echo "agent fixture file wait: vanished.waiting (job: vanished; cap: 5s)" >&2
  for ((i=0; i<100; i++)); do
    [[ -f "$agent_repo/artifacts/agents/hb/vanished.waiting" ]] && break
    sleep 0.05
  done
  [[ -f "$agent_repo/artifacts/agents/hb/vanished.waiting" ]] \
    || { echo "vanished wait fixture never entered the wait loop" >&2; exit 1; }
  mv "$agent_repo/artifacts/agents/jobs/vanished.json" "$agent_fixture/vanished.json"
  wait_for_agent_fixture_process vanished-driver vanished "$vanished_driver"
  mv "$agent_fixture/vanished.json" "$agent_repo/artifacts/agents/jobs/vanished.json"
  [[ "$(cat "$vanished_result")" == 5 ]] || { echo "vanished record did not map to wait exit 5 (got $(cat "$vanished_result"))" >&2; exit 1; }
  run_agent_fixture vanished-cancel vanished "$agent_dispatch" cancel --job vanished

  cancel_race="$agent_fixture/cancel-race.md"
  make_agent_brief "$cancel_race" design 'FAKE:cancel-race'
  run_agent_fixture_captured cancel-race-dispatch cancel-race /dev/null "$agent_dispatch" dispatch --role design-critic --brief "$cancel_race" --job-id cancel-race
  run_agent_fixture cancel-race-cancel cancel-race "$agent_dispatch" cancel --job cancel-race
  [[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job cancel-race)" == completed ]] \
    || { echo "completion did not win the scripted cancellation race" >&2; exit 1; }

  mirror_failure="$agent_fixture/mirror-failure.md"
  make_agent_brief "$mirror_failure" design 'FAKE:mirror-failure'
  run_agent_fixture mirror-retry mirror-retry "$agent_dispatch" dispatch --role design-critic --brief "$mirror_failure" --job-id mirror-retry --wait
  [[ -f "$agent_repo/artifacts/agents/mirror-retry/.mirror-failed" ]] \
    || { echo "scripted first mirror failure did not occur" >&2; exit 1; }
  if [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mirror"])' "$agent_repo/artifacts/agents/jobs/mirror-retry.json")" == None ]]; then
    run_agent_fixture mirror-retry-first-reap mirror-retry "$agent_dispatch" reap --job mirror-retry
  fi
  mirror_hash_before=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mirror"]["manifest"])' "$agent_repo/artifacts/agents/jobs/mirror-retry.json")
  run_agent_fixture mirror-retry-second-reap mirror-retry "$agent_dispatch" reap --job mirror-retry
  mirror_hash_after=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mirror"]["manifest"])' "$agent_repo/artifacts/agents/jobs/mirror-retry.json")
  [[ "$mirror_hash_before" == "$mirror_hash_after" && "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job mirror-retry)" == completed ]] \
    || { echo "idempotent mirror retry changed terminal state or durable content" >&2; exit 1; }
  python3 - "$agent_repo/artifacts/agents/jobs/mirror-retry.json" <<'PY'
import hashlib, json, sys
from pathlib import Path
record = json.load(open(sys.argv[1])); mirror = record["mirror"]; assert mirror
manifest_path = Path(mirror["path"]) / "manifest.json"
assert hashlib.sha256(manifest_path.read_bytes()).hexdigest() == mirror["manifest"]
manifest = json.loads(manifest_path.read_text())
for relative, item in manifest["files"].items():
    assert hashlib.sha256((Path(mirror["path"]) / relative).read_bytes()).hexdigest() == item["sha256"]
PY

  # Follow-ups are child records under one serialized, explicitly closed chain.
  follow_message="$agent_fixture/follow.md"
  cp "$agent_repo/scripts/agents/templates/follow-up.md" "$follow_message"
  run_agent_fixture happy-follow-up happy-r2 "$agent_dispatch" follow-up --job happy --message "$follow_message" --wait
  [[ -d "$agent_repo/artifacts/agents/happy/rounds/1" && -d "$agent_repo/artifacts/agents/happy/rounds/2" ]] \
    || { echo "follow-up did not preserve round 1 and create round 2" >&2; exit 1; }
  python3 - "$agent_repo" <<'PY'
import json, sys
from pathlib import Path
root = Path(sys.argv[1]); parent = json.loads((root / "artifacts/agents/jobs/happy.json").read_text()); child = json.loads((root / "artifacts/agents/jobs/happy-r2.json").read_text())
assert child["parentJob"] == "happy" and child["round"] == 2 and child["sessionId"] == parent["sessionId"]
assert child["startedAt"] >= parent["startedAt"] and child["capMin"] == parent["capMin"]
assert parent["chainUsage"]["providerUnits"]["fake"]["fake-unit"] == 2
assert parent["mirror"]["manifest"] == child["mirror"]["manifest"]
PY
  run_agent_fixture malformed-return-follow-up malformed-return-r2 "$agent_dispatch" follow-up --job malformed-return --message "$follow_message" --wait
  [[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job malformed-return-r2)" == completed ]] \
    || { echo "protocol-error retry did not create a completed child" >&2; exit 1; }
  agent_fails pending-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job cancelled --message "$follow_message"
  agent_fails timeout-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job timed --message "$follow_message"
  agent_fails process-loss-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job process-loss --message "$follow_message"

  python3 - "$agent_repo/artifacts/agents/jobs/default-role.json" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["sessionId"] = None; path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  agent_fails null-session-follow-up 'fresh-context embed fallback' "$agent_dispatch" follow-up --job default-role --message "$follow_message"

  resume_root="$agent_fixture/resume-root.md"
  make_agent_brief "$resume_root" design
  run_agent_fixture resume-root resume-root "$agent_dispatch" dispatch --role design-critic --brief "$resume_root" --job-id resume-root --wait
  resume_collision="$agent_fixture/resume-collision.md"
  cp "$follow_message" "$resume_collision"
  printf '\nFAKE:resume-collision\n' >>"$resume_collision"
  set +e
  run_agent_fixture resume-collision resume-root-r2 "$agent_dispatch" follow-up --job resume-root --message "$resume_collision" --wait
  resume_status=$?
  set -e
  [[ $resume_status -eq 3 ]] || { echo "resume collision did not map to failed" >&2; exit 1; }
  grep -Fq 'resume_collision' "$agent_repo/artifacts/agents/jobs/resume-root-r2.json" \
    || { echo "resume collision did not retain its named error" >&2; exit 1; }

  active_brief="$agent_fixture/active.md"
  make_agent_brief "$active_brief" design 'FAKE:concurrent-turn'
  run_agent_fixture_captured active-turn-dispatch active-turn /dev/null "$agent_dispatch" dispatch --role design-critic --brief "$active_brief" --job-id active-turn
  agent_fails active-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job active-turn --message "$follow_message"
  run_agent_fixture active-turn-cancel active-turn "$agent_dispatch" cancel --job active-turn

  run_agent_fixture happy-close happy "$agent_dispatch" close --job happy
  agent_fails closed-follow-up 'job chain is closed' "$agent_dispatch" follow-up --job happy --message "$follow_message"

  # A close racing a follow-up cannot land between its open check and child creation.
  run_agent_fixture close-race close-race "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id close-race --wait
  close_rc="$agent_fixture/close-race.close"; follow_rc="$agent_fixture/close-race.follow"
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh close --job close-race >/dev/null 2>&1; printf '%s\n' "$?" >"$close_rc") & close_pid=$!
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh follow-up --job close-race --message "$follow_message" >/dev/null 2>&1; printf '%s\n' "$?" >"$follow_rc") & follow_pid=$!
  wait_for_agent_fixture_process close-race-close close-race "$close_pid"
  wait_for_agent_fixture_process close-race-follow close-race-r2 "$follow_pid"
  close_won=$(cat "$close_rc"); follow_won=$(cat "$follow_rc")
  [[ ( "$close_won" == 0 && "$follow_won" != 0 ) || ( "$close_won" != 0 && "$follow_won" == 0 ) ]] \
    || { echo "close/follow-up race did not serialize to one winner" >&2; exit 1; }

  # Conformance uses the actual intent-to-add working-tree diff and protects
  # both plans/ and the ignored agent control plane.
  implement_brief="$agent_fixture/implement.md"
  make_agent_brief "$implement_brief" implement
  run_agent_fixture conformance conformance "$agent_dispatch" dispatch --role implementer --brief "$implement_brief" --job-id conformance --worktree --wait
  agent_fails close-before-diff 'diff.patch is not mirrored' "$agent_dispatch" close --job conformance
  (cd "$agent_repo" && scripts/agents/assert-conformance.sh --job conformance)
  [[ -f "$agent_repo/artifacts/agents/conformance/rounds/1/diff.patch" ]] \
    || { echo "conformance did not persist diff.patch" >&2; exit 1; }
  run_agent_fixture conformance-reap conformance "$agent_dispatch" reap --job conformance
  run_agent_fixture conformance-close conformance "$agent_dispatch" close --job conformance
  conformance_workspace=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["workspaceRoot"])' "$agent_repo/artifacts/agents/jobs/conformance.json")
  case "${conformance_workspace%/}/" in "${agent_repo%/}/"*) ;; *) echo "job worktree is outside the watcher scope" >&2; exit 1 ;; esac
  printf 'untracked change\n' >"$conformance_workspace/source.txt"
  agent_fails diff-boundary-mismatch 'Diff-boundary claim does not match computed diff' "$agent_repo/scripts/agents/assert-conformance.sh" --job conformance
  python3 - "$agent_repo/artifacts/agents/conformance/rounds/1/return.json" source.txt <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["diffBoundary"] = sys.argv[2:]; path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  (cd "$agent_repo" && scripts/agents/assert-conformance.sh --job conformance)
  mkdir -p "$conformance_workspace/plans"
  printf 'delegate plan\n' >"$conformance_workspace/plans/delegate.md"
  python3 - "$agent_repo/artifacts/agents/conformance/rounds/1/return.json" source.txt plans/delegate.md <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["diffBoundary"] = sys.argv[2:]; path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  agent_fails untracked-plan 'trusted plans/ state changed' "$agent_repo/scripts/agents/assert-conformance.sh" --job conformance
  git -C "$conformance_workspace" add source.txt plans/delegate.md
  git -C "$conformance_workspace" -c user.name=harness -c user.email=harness@example.invalid commit -qm delegate-checkpoint
  agent_fails committed-plan 'trusted plans/ state changed' "$agent_repo/scripts/agents/assert-conformance.sh" --job conformance
  printf 'uncommitted change\n' >>"$conformance_workspace/plans/delegate.md"
  agent_fails uncommitted-plan 'trusted plans/ state changed' "$agent_repo/scripts/agents/assert-conformance.sh" --job conformance
  mkdir -p "$conformance_workspace/artifacts/agents"
  printf 'tamper\n' >"$conformance_workspace/artifacts/agents/tamper"
  agent_fails control-plane-change 'agent control plane contains delegate-created files' "$agent_repo/scripts/agents/assert-conformance.sh" --job conformance

  # Snapshot refusal, fallbacks, permission waivers, and raw/event degradations.
  snapshot_dir="$agent_repo/artifacts/agents/capabilities"
  snapshot_save="$agent_fixture/snapshots"
  mkdir -p "$snapshot_save"
  mv "$snapshot_dir"/*.json "$snapshot_save/"
  agent_fails no-snapshot 'no capability snapshot matches' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id no-snapshot
  "$fake_adapter" probe --age-days 31 >/dev/null
  agent_fails stale-snapshot 'capability snapshot is stale' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id stale-snapshot
  mv "$snapshot_dir"/*.json "$agent_fixture/"
  mv "$snapshot_save"/*.json "$snapshot_dir/"

  old_save="$agent_fixture/current-snapshots"
  mkdir -p "$old_save"
  mv "$snapshot_dir"/*.json "$old_save/"
  "$fake_adapter" probe --profile old >/dev/null
  old_brief="$agent_fixture/old-capabilities.md"
  make_agent_brief "$old_brief" implement 'FAKE:old-capability-set' 'FAKE:no-event-stream' 'FAKE:hook-unavailable'
  run_agent_fixture old-capabilities old-capabilities "$agent_dispatch" dispatch --role implementer --brief "$old_brief" --job-id old-capabilities --worktree --wait
  python3 - "$agent_repo/artifacts/agents/jobs/old-capabilities.json" <<'PY'
import json, sys
record = json.load(open(sys.argv[1])); assert record["sessionEstablishedSignal"] is False
assert any(item["capability"] == "resume" for item in record["capabilityFallbacks"])
PY
  [[ ! -e "$agent_repo/artifacts/agents/old-capabilities/rounds/1/events.jsonl" ]] \
    || { echo "no-event-stream fallback emitted a native event file" >&2; exit 1; }
  grep -Fq 'polling fallback used' "$agent_repo/artifacts/agents/jobs/old-capabilities.log" \
    || { echo "hook-unavailable fallback was not observable" >&2; exit 1; }
  run_agent_fixture old-capabilities-follow-up old-capabilities-r2 "$agent_dispatch" follow-up --job old-capabilities --message "$follow_message" --wait
  python3 - "$agent_repo" <<'PY'
import json, sys
from pathlib import Path
root = Path(sys.argv[1]); parent = json.loads((root / "artifacts/agents/jobs/old-capabilities.json").read_text()); child = json.loads((root / "artifacts/agents/jobs/old-capabilities-r2.json").read_text())
assert child["resumeMode"] == "fresh-context" and child["sessionId"] != parent["sessionId"]
prompt = (root / "artifacts/agents/old-capabilities/rounds/2/prompt.md").read_text()
assert "# Prior brief" in prompt and "# Prior return" in prompt and "# Correction" in prompt
PY
  mv "$snapshot_dir"/*.json "$agent_fixture/"
  mv "$old_save"/*.json "$snapshot_dir/"

  requirements="$agent_repo/scripts/agents/roles/design-critic.requirements.json"
  saved_requirements="$agent_fixture/design-critic.requirements.json"
  cp "$requirements" "$saved_requirements"
  "$fake_adapter" probe --profile unverified-network >/dev/null
  agent_fails unverified-deny 'cannot verify restrictive permission field network' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id unverified-deny
  perl -0pi -e 's/"optional": \{\}/"optional": {},\n  "waivers": {"network": ["fake"]}/' "$requirements"
  run_agent_fixture waived-deny waived-deny "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id waived-deny --wait
  cp "$saved_requirements" "$requirements"
  "$fake_adapter" probe >/dev/null

  nested_brief="$agent_fixture/nested.md"
  make_agent_brief "$nested_brief" design 'FAKE:nested-agent-events'
  run_agent_fixture nested-events nested-events "$agent_dispatch" dispatch --role design-critic --brief "$nested_brief" --job-id nested-events --wait
  grep -Fq '"topLevel":false' "$agent_repo/artifacts/agents/nested-events/rounds/1/events.jsonl" \
    || { echo "nested-agent event was not captured" >&2; exit 1; }
  [[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job nested-events)" == completed ]] \
    || { echo "nested completion event ended the wrong lifecycle" >&2; exit 1; }
  malicious_brief="$agent_fixture/malicious.md"
  make_agent_brief "$malicious_brief" design 'Fake-Argument: $(touch should-not-exist)'
  run_agent_fixture malicious-argument malicious-argument "$agent_dispatch" dispatch --role design-critic --brief "$malicious_brief" --job-id malicious-argument --wait
  [[ ! -e "$agent_repo/should-not-exist" ]] || { echo "malicious provider argument was evaluated" >&2; exit 1; }
  grep -Fq '$(touch should-not-exist)' "$agent_repo/artifacts/agents/malicious-argument/rounds/1/raw.out" \
    || { echo "malicious provider argument was not transported verbatim as a value" >&2; exit 1; }

  # Dispatch only reads mission leases. The future mission runner owns their
  # acquisition and renewal; this fixture fabricates the frozen shape around
  # a process whose command line carries the instance tag.
  mkdir -p "$agent_repo/plans" "$agent_repo/artifacts/agents/missions/mission-alpha"
  printf '```text\nfence.job-cap-min=120\n```\n' >"$agent_repo/plans/mission-mission-alpha.contract.md"
  python3 -c 'import time; time.sleep(30)' mission-lease-tag & mission_pid=$!
  mission_pgid=$(python3 -c 'import os,sys; print(os.getpgid(int(sys.argv[1])))' "$mission_pid")
  mission_identity="$agent_fixture/mission-process-identity.json"
  python3 - "$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" "$mission_identity" "$mission_pid" "$mission_pgid" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
pid, pgid = int(sys.argv[3]), int(sys.argv[4])
Path(sys.argv[1]).write_text(json.dumps({"missionId":"mission-alpha","pid":pid,"pgid":pgid,"instanceTag":"mission-lease-tag","startedAt":now,"renewedAt":now}) + "\n")
Path(sys.argv[2]).write_text(json.dumps({str(pid): {"pgid": pgid, "command": "python3 fixture mission-lease-tag"}}) + "\n")
PY
  export HARNESS_FAKE_PROCESS_IDENTITY_FILE="$mission_identity"
  run_agent_fixture mission-explicit mission-explicit "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-explicit --mission mission-alpha --wait
  HARNESS_MISSION_ID=mission-alpha HARNESS_MISSION_LEASE="$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" \
    run_agent_fixture mission-inherited mission-inherited "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-inherited --wait
  agent_fails mission-cap 'exceeds the mission' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-cap --mission mission-alpha --cap-min 121
  agent_fails missing-mission-lease 'does not have a live' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id missing-mission --mission missing
  agent_fails ambiguous-mission 'ambiguous mission context' env HARNESS_MISSION_ID=mission-alpha HARNESS_MISSION_LEASE="$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" \
    "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-ambiguous --mission another
  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mission"])' "$agent_repo/artifacts/agents/jobs/happy.json")" == None ]] \
    || { echo "unstamped interactive dispatch gained mission authority" >&2; exit 1; }
  kill "$mission_pid" 2>/dev/null || true
  wait_for_agent_fixture_process mission-lease-holder - "$mission_pid" 2>/dev/null || true
  unset HARNESS_FAKE_PROCESS_IDENTITY_FILE

  agent_fails unknown-status-job '' "$agent_dispatch" status --job absent
  set +e
  "$agent_dispatch" status --job absent >/dev/null 2>&1
  unknown_status=$?
  set -e
  [[ $unknown_status -eq 6 ]] || { echo "unknown status job mapped to $unknown_status instead of 6" >&2; exit 1; }
  printf '{malformed\n' >"$agent_repo/artifacts/agents/jobs/malformed-status.json"
  set +e
  "$agent_dispatch" status --job malformed-status >/dev/null 2>&1
  malformed_record_status=$?
  set -e
  [[ $malformed_record_status -eq 7 ]] || { echo "malformed status record mapped to $malformed_record_status instead of 7" >&2; exit 1; }
  run_agent_fixture_captured fake-selftest - "$agent_fixture/fake-selftest.out" "$fake_adapter" selftest
  grep -Fq 'full protocol sequence' "$agent_fixture/fake-selftest.out" \
    || { echo "fake adapter selftest did not run its full protocol sequence" >&2; exit 1; }
  python3 - "$agent_repo/artifacts/agents/selftests" <<'PY'
import json, sys
from pathlib import Path
paths = list(Path(sys.argv[1]).glob("fake-selftest-*.json")); assert paths
value = json.loads(max(paths, key=lambda path: path.stat().st_mtime).read_text())
assert "resume-identity" in value["provenBehaviorally"] and "network" in value["constructedOnly"]
PY

  export HARNESS_SKIP_AGENT_FIXTURES=1
fi

# The shipped Stop hook must stay rooted and surface via JSON output: hooks
# run in the session's cwd, receipt.sh resolves its ledger from there, and a
# non-blocking exit code shows only a first-line hook-error notice.
hooks_json=scripts/enforcement/claude-code-hooks.json
grep -Fq 'cd \"$CLAUDE_PROJECT_DIR\"' "$hooks_json" || { echo "stop hook is not rooted at CLAUDE_PROJECT_DIR" >&2; exit 1; }
grep -Fq 'systemMessage' "$hooks_json" || { echo "stop hook does not surface a systemMessage when a retro is due" >&2; exit 1; }
if grep -Fq '|| true' "$hooks_json"; then
  echo "stop hook masks the retro-due exit code with || true" >&2
  exit 1
fi
if command -v python3 >/dev/null; then
  hook_cmd=$(python3 -c "import json; print(json.load(open('$hooks_json'))['hooks']['Stop'][0]['hooks'][0]['command'])")
  hookrepo="$tmp/hookrepo"
  mkdir -p "$hookrepo/scripts" "$hookrepo/plans"
  cp scripts/receipt.sh "$hookrepo/scripts/"
  printf '1|1970-01-01T00:00:01Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|note=aged\n' >"$hookrepo/plans/receipts.log"
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
  grep -q systemMessage <<<"$out" || { echo "stop hook stayed silent on a due retro" >&2; exit 1; }
  printf '%s|%s|RETRO|note=fixture\n' "$(date -u +%s)" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$hookrepo/plans/receipts.log"
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
  [[ -z "$out" ]] || { echo "stop hook emitted output when no retro is due" >&2; exit 1; }
  printf 'garbage\n' >"$hookrepo/plans/receipts.log"
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
  grep -q "errored" <<<"$out" || { echo "stop hook hid a failing receipt check" >&2; exit 1; }
  if grep -q "retro due" <<<"$out"; then
    echo "stop hook misreported a check error as a due retro" >&2
    exit 1
  fi
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$tmp/definitely-missing" bash -c "$hook_cmd")
  grep -q "project directory" <<<"$out" || { echo "stop hook stayed silent on an unresolvable project directory" >&2; exit 1; }
fi

# The debug-java preflight is optional: absent in adopted repositories that
# excluded the skill, moved into skills/ in JVM repositories that enabled it.
for preflight in optional-skills/debug-java/scripts/preflight.sh skills/debug-java/scripts/preflight.sh; do
  if [[ -x "$preflight" ]]; then
    touch "$tmp/source" "$tmp/artifact"
    "$preflight" --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null
    touch -t 202001010000 "$tmp/artifact"
    if "$preflight" --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null 2>&1; then
      echo "debug preflight accepted a stale artifact" >&2
      exit 1
    fi
    break
  fi
done

cat >"$tmp/good.md" <<'EOF'
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-1 | HIGH | Requirement | Behavior | `owner.py` | `owner.py` | `test_owner.py` | Not applicable: pure derivation | DONE | None |
EOF
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/good.md" >/dev/null

# Proof cells on critical/high rows must be concrete: a DONE row whose proof
# is vague prose must fail, or a declared status can outrun its evidence.
sed 's/| `test_owner.py` |/| covered somewhere |/' "$tmp/good.md" >"$tmp/vague.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/vague.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a DONE row with a vague proof cell" >&2
  exit 1
fi
# Keyword-carrying prose is still prose: promises of future proof, and owners
# without a code-shaped token, must fail.
cat >"$tmp/keyword.md" <<'EOF'
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-1 | CRITICAL | Requirement | Behavior | someone will own this | we should test this later | needs testing | manual test pending | DONE | None |
EOF
if scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/keyword.md" >/dev/null 2>&1; then
  echo "obligation gate accepted keyword prose as proof and a prose owner" >&2
  exit 1
fi
sed 's/| Not applicable: pure derivation |/| Not applicable |/' "$tmp/good.md" >"$tmp/bare-na.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/bare-na.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a bare Not applicable without a reason" >&2
  exit 1
fi
sed 's/| Not applicable: pure derivation |/| Not applicable: |/' "$tmp/good.md" >"$tmp/empty-na.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/empty-na.md" >/dev/null 2>&1; then
  echo "obligation gate accepted an empty-delimiter Not applicable" >&2
  exit 1
fi
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | pyproject.toml |/' "$tmp/good.md" >"$tmp/toml.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/toml.md" >/dev/null || {
  echo "obligation gate rejected an unbackticked config-file proof path" >&2
  exit 1
}
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | module.mjs |/' "$tmp/good.md" >"$tmp/mjs.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/mjs.md" >/dev/null || {
  echo "obligation gate rejected an unbackticked filename outside the old whitelist" >&2
  exit 1
}
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | compare e.g. the results |/' "$tmp/good.md" >"$tmp/eg.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/eg.md" >/dev/null 2>&1; then
  echo "obligation gate mistook abbreviation prose for a filename" >&2
  exit 1
fi
# Matrices shown inside fenced code blocks are documentation, not declarations.
{ printf '```markdown\n'; cat "$tmp/good.md"; printf '```\n'; } >"$tmp/fenced.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/fenced.md" >/dev/null 2>&1; then
  echo "obligation gate read a matrix out of a fenced code block" >&2
  exit 1
fi

sed 's/| DONE |/| MISSING |/' "$tmp/good.md" >"$tmp/bad.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/bad.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a missing high obligation" >&2
  exit 1
fi

sed 's/| HIGH |/| MEDIUM |/; s/| DONE |/| PARTIAL |/' "$tmp/good.md" >"$tmp/medium.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/medium.md" >/dev/null || {
  echo "obligation gate rejected a valid medium-only matrix" >&2
  exit 1
}

scripts/assert-design-obligation-gate.sh --runtime-required --file docs/examples/design-obligation-matrix.md >/dev/null 2>&1 && {
  echo "example matrix with READY_FOR_RUNTIME passed --runtime-required; negative fixture broken" >&2
  exit 1
}
scripts/assert-design-obligation-gate.sh --file docs/examples/design-obligation-matrix.md >/dev/null

repo="$tmp/baseline-repo"
git init -q "$repo"
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit --allow-empty -qm base
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null) || {
  echo "refactor baseline check blocked on the baseline file's own dirt right after record" >&2
  exit 1
}
git -C "$repo" add plans/refactor-baseline
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null)
echo dirty >"$repo/dirty.txt"
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null 2>&1); then
  echo "refactor baseline check accepted a dirty worktree" >&2
  exit 1
fi
rm "$repo/dirty.txt"
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" check --max-commits 0 >/dev/null 2>&1); then
  echo "refactor baseline check ignored the commit-count backstop" >&2
  exit 1
fi
# Custom and absolute --file paths normalize to the repository root; paths
# outside the repository are rejected because git cannot see their dirt.
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file plans/custom-baseline >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file plans/custom-baseline >/dev/null) || {
  echo "refactor baseline check blocked a custom relative --file right after record" >&2
  exit 1
}
git -C "$repo" add plans/custom-baseline
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm custom-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "$repo/plans/abs-baseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "$repo/plans/abs-baseline" >/dev/null) || {
  echo "refactor baseline check blocked an in-repository absolute --file right after record" >&2
  exit 1
}
git -C "$repo" add plans/abs-baseline
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm abs-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "plans/bäseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "plans/bäseline" >/dev/null) || {
  echo "refactor baseline check blocked a non-ASCII --file right after record (quotePath)" >&2
  exit 1
}
git -C "$repo" add "plans/bäseline"
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm nonascii-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "plans/my baseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "plans/my baseline" >/dev/null) || {
  echo "refactor baseline check blocked a space-containing --file right after record (C-quoting)" >&2
  exit 1
}
git -C "$repo" add "plans/my baseline"
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm space-baseline
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "$tmp/outside-baseline" >/dev/null 2>&1); then
  echo "refactor baseline accepted a --file outside the repository" >&2
  exit 1
fi

(cd "$repo" && "$root/scripts/frontier.sh" record --score 80 --min-delta 1 --eval "declared eval" >/dev/null)
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 79 >/dev/null 2>&1); then
  echo "frontier challenge accepted a score below the frontier" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 80.5 >/dev/null 2>&1); then
  echo "frontier challenge forgot the stored noise floor" >&2
  exit 1
fi
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 80.5 --min-delta 0 >/dev/null)
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 82 >/dev/null)
git -C "$repo" add plans/frontier
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm frontier
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 75 --eval "declared eval" >/dev/null 2>&1); then
  echo "frontier record accepted a regression without --force" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=60\neval=declared\nartifact=\n' >"$tmp/frontier-old"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-old" >/dev/null 2>&1; then
  echo "frontier challenge compared against an expired frontier" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-nowindow"
scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-nowindow" >/dev/null
scripts/frontier.sh status --file "$tmp/frontier-nowindow" | grep -qx 'direction=max' || {
  echo "frontier status hid the effective direction of a legacy file" >&2
  exit 1
}
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=sideways\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-malformed"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-malformed" >/dev/null 2>&1; then
  echo "frontier challenge accepted a malformed persisted direction" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-emptydir"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-emptydir" >/dev/null 2>&1; then
  echo "frontier challenge accepted an empty persisted direction" >&2
  exit 1
fi

# Lower-is-better frontiers: persisted direction, force-gated changes, and a
# challenge that only ever uses the stored direction.
(cd "$repo" && "$root/scripts/frontier.sh" record --score 80 --min-delta 1 --direction min --eval "declared eval" --file plans/frontier-min >/dev/null)
git -C "$repo" add plans/frontier-min
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm frontier-min
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 79.5 --file plans/frontier-min >/dev/null 2>&1); then
  echo "min-direction challenge accepted a within-noise improvement" >&2
  exit 1
fi
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 78 --file plans/frontier-min >/dev/null)
(cd "$repo" && HARNESS_FRONTIER_DIRECTION=max "$root/scripts/frontier.sh" challenge --score 78 --file plans/frontier-min >/dev/null) || {
  echo "challenge honored an environment direction instead of the persisted one" >&2
  exit 1
}
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 85 --eval "declared eval" --file plans/frontier-min >/dev/null 2>&1); then
  echo "min-direction record accepted a regression without --force" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 99 --direction max --eval "declared eval" --file plans/frontier-min >/dev/null 2>&1); then
  echo "frontier record accepted a direction change without --force" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 1 --direction min --file plans/frontier-min >/dev/null 2>&1); then
  echo "frontier challenge accepted a direction flag" >&2
  exit 1
fi

scripts/assert-stop-loss.sh --file docs/examples/step-back-ledger.md >/dev/null
printf '### Cycle C1\n- Classification: no-progress\n### Cycle C2\n- Classification: no-progress\n' >"$tmp/stuck.md"
if scripts/assert-stop-loss.sh --file "$tmp/stuck.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed a third cycle after two no-progress results" >&2
  exit 1
fi
printf -- '- Cycle budget: 2\n### Cycle C1\n- Classification: contract-improved\n### Cycle C2\n- Classification: falsified-continue\n' >"$tmp/spent.md"
if scripts/assert-stop-loss.sh --file "$tmp/spent.md" >/dev/null 2>&1; then
  echo "stop-loss check ignored an exhausted cycle budget" >&2
  exit 1
fi
printf '### Cycle C1\n- Classification: falsified-dead-end\n' >"$tmp/deadend.md"
if scripts/assert-stop-loss.sh --file "$tmp/deadend.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed cycles after a dead end" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: falsified-continue\n### Cycle E3\n- Classification: unresolved\n' >"$tmp/nogain.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain.md" >/dev/null 2>&1; then
  echo "stop-loss check ignored an exhausted no-gain budget over a mixed trailing sequence" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: contract-improved\n### Cycle E3\n- Classification: unresolved\n### Cycle E4\n- Classification: falsified-continue\n' >"$tmp/nogain-reset.md"
scripts/assert-stop-loss.sh --file "$tmp/nogain-reset.md" >/dev/null || {
  echo "stop-loss check failed to reset the no-gain count on a contract-improved cycle" >&2
  exit 1
}
printf '### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: unresolved\n### Cycle E3\n- Classification: unresolved\n' >"$tmp/nogain-optout.md"
scripts/assert-stop-loss.sh --file "$tmp/nogain-optout.md" >/dev/null || {
  echo "stop-loss check blocked unresolved cycles without a declared no-gain budget" >&2
  exit 1
}
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n### Cycle E3\n- Classification: falsified-continue\n' >"$tmp/nogain-unclassified.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain-unclassified.md" >/dev/null 2>&1; then
  echo "stop-loss no-gain count let an unclassified cycle vanish from the tail" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: not-contract-improved\n### Cycle E3\n- Classification: falsified-continue\n' >"$tmp/nogain-fakegain.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain-fakegain.md" >/dev/null 2>&1; then
  echo "stop-loss no-gain count reset on a classification merely containing contract-improved" >&2
  exit 1
fi

rfile="$tmp/receipts.log"
scripts/receipt.sh add --type implement --outcome shipped --file "$rfile" >/dev/null
scripts/receipt.sh check --file "$rfile" >/dev/null
scripts/receipt.sh add --type review --outcome reworked --corrections 1 --file "$rfile" >/dev/null
if scripts/receipt.sh check --max-receipts 1 --file "$rfile" >/dev/null 2>&1; then
  echo "receipt check ignored the receipt-count backstop" >&2
  exit 1
fi
scripts/receipt.sh retro "fixture retro" --file "$rfile" >/dev/null
scripts/receipt.sh check --max-receipts 1 --file "$rfile" >/dev/null
if scripts/receipt.sh add --type bogus --outcome shipped --file "$rfile" >/dev/null 2>&1; then
  echo "receipt add accepted an invalid type" >&2
  exit 1
fi
printf '1|1970-01-01T00:00:01Z|RETRO|note=aged\n' >"$tmp/receipts-aged.log"
scripts/receipt.sh check --max-age-days 0 --file "$tmp/receipts-aged.log" >/dev/null || {
  echo "receipt check demanded a retro over an empty period" >&2
  exit 1
}
scripts/receipt.sh add --type improve --outcome shipped --verify caught --file "$rfile" >/dev/null
scripts/receipt.sh stats --file "$rfile" | grep -q '^receipts=1$' || { echo "receipt stats miscounted the post-retro period" >&2; exit 1; }
scripts/receipt.sh stats --file "$rfile" | grep -q '^type_improve=1$' || { echo "receipt stats missed the improve type" >&2; exit 1; }
scripts/receipt.sh stats --all --file "$rfile" | grep -q '^receipts=3$' || { echo "receipt stats --all miscounted" >&2; exit 1; }

# Every free-text field is sanitized by one shared path: CRLF through the
# note, the skills list, and the retro summary must each stay one log line.
crlf_fixture=$(printf 'a\r\nb')
rfile_crlf="$tmp/receipts-crlf.log"
scripts/receipt.sh add --type implement --outcome shipped --note "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 1 ]] || { echo "a CRLF note corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh add --type implement --outcome shipped --skills "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 2 ]] || { echo "a CRLF skills list corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh retro "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 3 ]] || { echo "a CRLF retro summary corrupted the receipt log" >&2; exit 1; }

# Adopted-mode contract: a copy without the template marker validates with a
# skill pruned, and a present-but-broken skill still fails. Template mode
# only, so the nested run (which lacks meta/) cannot recurse.
if (( template_mode )); then
  adopted="$tmp/adopted"
  mkdir -p "$adopted"
  cp -R "$root/." "$adopted"
  rm -rf "$adopted/meta" "$adopted/skills/improve" "$adopted/plans/receipts.log" "$adopted/.claude"
  sed 's/<[^>]*>/filled/g' "$adopted/docs/project-rules.md" >"$adopted/docs/project-rules.md.new"
  mv "$adopted/docs/project-rules.md.new" "$adopted/docs/project-rules.md"
  sed 's/<[^>]*>/filled/g' "$adopted/harness.conf" >"$adopted/harness.conf.new"
  mv "$adopted/harness.conf.new" "$adopted/harness.conf"
  bash "$adopted/scripts/validate-harness.sh" >/dev/null 2>&1 || {
    echo "adopted-mode validation failed for a copy with one skill pruned" >&2
    exit 1
  }
  mkdir "$adopted/skills/hollow"
  if bash "$adopted/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopted-mode validation accepted a skill directory without SKILL.md" >&2
    exit 1
  fi
  rmdir "$adopted/skills/hollow"
  grep -v '^name:' "$adopted/skills/verify/SKILL.md" >"$adopted/skills/verify/SKILL.md.new"
  mv "$adopted/skills/verify/SKILL.md.new" "$adopted/skills/verify/SKILL.md"
  if bash "$adopted/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopted-mode validation accepted a present skill with broken frontmatter" >&2
    exit 1
  fi
fi

# adopt.sh self-test, template mode only. The source is a committed snapshot
# of the current working tree: a git clone would exercise committed HEAD, not
# the implementation under review.
if (( template_mode )); then
  srcrepo="$tmp/adopt-src"
  mkdir -p "$srcrepo"
  cp -R "$root/." "$srcrepo"
  echo 'ignored-fixture.txt' >>"$srcrepo/.gitignore"
  echo junk >"$srcrepo/ignored-fixture.txt"
  git init -q "$srcrepo"
  git -C "$srcrepo" add -A
  git -C "$srcrepo" -c user.name=harness -c user.email=harness@example.invalid commit -qm snapshot
  adopt="$srcrepo/scripts/adopt.sh"
  src_sha=$(git -C "$srcrepo" rev-parse HEAD)

  tgt="$tmp/adopt-default"
  mkdir -p "$tgt"
  printf 'project readme\n' >"$tgt/README.md"
  bash "$adopt" "$tgt" >/dev/null
  [[ -f "$tgt/.github/workflows/harness.yml" ]] || { echo "adopt: CI workflow not installed" >&2; exit 1; }
  [[ -L "$tgt/.claude/skills/verify" ]] || { echo "adopt: claude skill symlink missing" >&2; exit 1; }
  [[ -f "$tgt/.claude/agents/verify.md" ]] || { echo "adopt: claude agent profile missing" >&2; exit 1; }
  grep -q systemMessage "$tgt/.claude/settings.json" || { echo "adopt: settings.json lacks the shipped hook" >&2; exit 1; }
  [[ ! -e "$tgt/optional-skills" ]] || { echo "adopt: unselected optional skills were copied" >&2; exit 1; }
  [[ "$(cat "$tgt/README.md")" == "project readme" ]] || { echo "adopt: the project's own README was touched" >&2; exit 1; }
  [[ ! -e "$tgt/ignored-fixture.txt" ]] || { echo "adopt: ignored source content entered the payload" >&2; exit 1; }
  [[ "$(ls "$tgt/plans" | sort | tr '\n' ' ')" == "README.md instruction-ledger.md known-issues.md " ]] \
    || { echo "adopt: plans/ payload carries more than the standing ledgers" >&2; exit 1; }
  [[ -d "$tgt/artifacts" ]] || { echo "adopt: artifacts directory missing" >&2; exit 1; }
  grep -qxF 'artifacts/' "$tgt/.gitignore" || { echo "adopt: artifacts/ not gitignored" >&2; exit 1; }
  grep -q "$src_sha" "$tgt/docs/project-rules.md" || { echo "adopt: template SHA not recorded" >&2; exit 1; }
  if grep -q '<template sha>' "$tgt/docs/project-rules.md"; then
    echo "adopt: template SHA placeholder left unreplaced" >&2
    exit 1
  fi
  snap="$tmp/adopt-snap"
  mkdir -p "$snap"
  cp -R "$tgt/." "$snap"
  bash "$adopt" "$tgt" >/dev/null
  diff -r "$snap" "$tgt" >/dev/null || { echo "adopt: second run changed an adopted target" >&2; exit 1; }
  if bash "$tgt/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopt: target validated with unreplaced placeholders" >&2
    exit 1
  fi
  sed 's/<[^>]*>/filled/g' "$tgt/docs/project-rules.md" >"$tgt/docs/project-rules.md.new"
  mv "$tgt/docs/project-rules.md.new" "$tgt/docs/project-rules.md"
  sed 's/<[^>]*>/filled/g' "$tgt/harness.conf" >"$tgt/harness.conf.new"
  mv "$tgt/harness.conf.new" "$tgt/harness.conf"
  bash "$tgt/scripts/validate-harness.sh" >/dev/null 2>&1 || { echo "adopt: filled target failed validation" >&2; exit 1; }

  bash "$adopt" "$tmp/adopt-devin" --runtimes devin >/dev/null
  [[ -f "$tmp/adopt-devin/.devin/agents/verify/AGENT.md" ]] || { echo "adopt: devin profile missing" >&2; exit 1; }
  [[ ! -e "$tmp/adopt-devin/.claude" ]] || { echo "adopt: devin-only target got .claude state" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-codex" --runtimes codex >/dev/null
  [[ -L "$tmp/adopt-codex/.agents/skills/verify" ]] || { echo "adopt: codex skill registration missing" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-none" --runtimes none >/dev/null
  [[ ! -e "$tmp/adopt-none/.claude" && ! -e "$tmp/adopt-none/.devin" && ! -e "$tmp/adopt-none/.agents" ]] \
    || { echo "adopt: --runtimes none still registered a runtime" >&2; exit 1; }
  [[ -f "$tmp/adopt-none/.github/workflows/harness.yml" ]] || { echo "adopt: CI workflow skipped for --runtimes none" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-java" --enable debug-java >/dev/null
  [[ -f "$tmp/adopt-java/skills/debug-java/SKILL.md" ]] || { echo "adopt: --enable did not move the optional skill" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-copy" --runtimes claude,codex --copy-skills >/dev/null
  [[ -d "$tmp/adopt-copy/.claude/skills/verify" && ! -L "$tmp/adopt-copy/.claude/skills/verify" ]] \
    || { echo "adopt: --copy-skills did not copy" >&2; exit 1; }
  [[ -d "$tmp/adopt-copy/.agents/skills/verify" && ! -L "$tmp/adopt-copy/.agents/skills/verify" ]] \
    || { echo "adopt: --copy-skills did not copy the codex registration" >&2; exit 1; }
  sed 's/<[^>]*>/filled/g' "$tmp/adopt-copy/docs/project-rules.md" >"$tmp/adopt-copy/docs/project-rules.md.new"
  mv "$tmp/adopt-copy/docs/project-rules.md.new" "$tmp/adopt-copy/docs/project-rules.md"
  sed 's/<[^>]*>/filled/g' "$tmp/adopt-copy/harness.conf" >"$tmp/adopt-copy/harness.conf.new"
  mv "$tmp/adopt-copy/harness.conf.new" "$tmp/adopt-copy/harness.conf"
  bash "$tmp/adopt-copy/scripts/validate-harness.sh" >/dev/null 2>&1 || { echo "adopt: copied-skills target failed validation" >&2; exit 1; }
  echo drift >>"$tmp/adopt-copy/.claude/skills/verify/SKILL.md"
  if bash "$tmp/adopt-copy/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopt: validation missed a drifted claude skill copy" >&2
    exit 1
  fi
  cp "$tmp/adopt-copy/skills/verify/SKILL.md" "$tmp/adopt-copy/.claude/skills/verify/SKILL.md"
  echo drift >>"$tmp/adopt-copy/.agents/skills/verify/SKILL.md"
  if bash "$tmp/adopt-copy/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopt: validation missed a drifted codex skill copy" >&2
    exit 1
  fi
  cp "$tmp/adopt-copy/skills/verify/SKILL.md" "$tmp/adopt-copy/.agents/skills/verify/SKILL.md"
  rm -rf "$tmp/adopt-copy/skills/verify"
  if bash "$tmp/adopt-copy/scripts/validate-harness.sh" >"$tmp/orphan.out" 2>&1; then
    echo "adopt: validation missed an orphaned copy of a pruned skill" >&2
    exit 1
  fi
  grep -q "orphaned" "$tmp/orphan.out" || {
    echo "adopt: pruned-skill failure did not name the orphaned copy" >&2
    exit 1
  }

  mkdir -p "$tmp/adopt-foreign"
  touch "$tmp/adopt-foreign/.cursorrules"
  if bash "$adopt" "$tmp/adopt-foreign" >/dev/null 2>&1; then
    echo "adopt: accepted a target with a foreign instruction asset" >&2
    exit 1
  fi
  mkdir -p "$tmp/adopt-collide/docs"
  echo different >"$tmp/adopt-collide/docs/collaboration.md"
  if bash "$adopt" "$tmp/adopt-collide" >/dev/null 2>"$tmp/collide.err"; then
    echo "adopt: overwrote or skipped a colliding payload path" >&2
    exit 1
  fi
  grep -q 'docs/collaboration.md' "$tmp/collide.err" || {
    echo "adopt: collision refusal did not name the colliding path" >&2
    exit 1
  }
  if bash "$adopt" "$tmp/adopt-bogus" --runtimes codez >/dev/null 2>&1; then
    echo "adopt: accepted an unknown runtime name" >&2
    exit 1
  fi
  [[ ! -e "$tmp/adopt-bogus/wow.md" ]] || {
    echo "adopt: a rejected runtime name still mutated the target" >&2
    exit 1
  }
  if bash "$adopt" "$tmp/adopt-nonemix" --runtimes none,claude >/dev/null 2>&1; then
    echo "adopt: accepted the contradictory none-plus-runtime form" >&2
    exit 1
  fi
  [[ ! -e "$tmp/adopt-nonemix/wow.md" ]] || {
    echo "adopt: a rejected runtime combination still mutated the target" >&2
    exit 1
  }
  bash "$adopt" "$tmp/adopt-partial" >/dev/null
  rm "$tmp/adopt-partial/.github/workflows/harness.yml"
  if bash "$adopt" "$tmp/adopt-partial" >/dev/null 2>&1; then
    echo "adopt: rerun over an incomplete installation reported success" >&2
    exit 1
  fi
  bash "$adopt" "$tmp/adopt-partial2" >/dev/null
  rm "$tmp/adopt-partial2/AGENTS.md"
  if bash "$adopt" "$tmp/adopt-partial2" >/dev/null 2>&1; then
    echo "adopt: rerun over a structurally broken installation reported success" >&2
    exit 1
  fi
  echo dirty >>"$srcrepo/wow.md"
  if bash "$adopt" "$tmp/adopt-dirty" >/dev/null 2>&1; then
    echo "adopt: ran from a dirty template worktree" >&2
    exit 1
  fi
  git -C "$srcrepo" checkout -q -- wow.md
  rm -rf "$tgt/skills/take-a-step-back"
  if bash "$tgt/scripts/validate-harness.sh" >"$tmp/dangling.out" 2>&1; then
    echo "adopt: validation missed a dangling registered skill link" >&2
    exit 1
  fi
  grep -q "dangling" "$tmp/dangling.out" || {
    echo "adopt: pruned-skill failure did not name the dangling link" >&2
    exit 1
  }
fi

# watch-background-jobs: all four reportable states plus baseline suppression.
# The state file is pre-created because a MISSING state file auto-baselines on
# first run (the 2026-08-03 hardening); an existing empty state means armed.
wbj="$tmp/wbj"; mkdir -p "$wbj/jobs"
printf '{"status":"completed"}' >"$wbj/jobs/done.json"
printf '{"status":"running"}'   >"$wbj/jobs/live.json"
touch "$wbj/s1" "$wbj/s2" "$wbj/s3" "$wbj/s3b" "$wbj/s3c" "$wbj/s6" "$wbj/s7" "$wbj/s8"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s1" --once >"$wbj/o1" 2>&1
grep -q "^DONE done status=completed" "$wbj/o1" || {
  echo "watch-background-jobs: terminal job not reported" >&2; exit 1; }
grep -q "live" "$wbj/o1" && {
  echo "watch-background-jobs: running job reported as terminal" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s1" --once >"$wbj/o2" 2>&1
grep -v "^ARMED " "$wbj/o2" | grep -q . && {
  echo "watch-background-jobs: re-reported an already-reported job" >&2; exit 1; }
# age the record by a controlled 10 minutes so stale and cap are separable
python3 - "$wbj/jobs/live.json" <<'AGE'
import os, sys, time
t = time.time() - 600
os.utime(sys.argv[1], (t, t))
AGE
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s2" --stale-min 5 --cap-min 600 --once >"$wbj/o3" 2>&1
grep -q "^STALE live" "$wbj/o3" || {
  echo "watch-background-jobs: stale job not reported" >&2; exit 1; }
grep -q "^CAPPED live" "$wbj/o3" && {
  echo "watch-background-jobs: hard cap fired inside its own window" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s3" --stale-min 5 --cap-min 5 --once >"$wbj/o4" 2>&1
grep -q "^CAPPED live" "$wbj/o4" || {
  echo "watch-background-jobs: hard cap not reported" >&2; exit 1; }
# A job whose RECORD is old but whose sibling log is fresh is WORKING, not stale.
# Runners write the record once at dispatch and stream progress to the log, so a
# record-only liveness check cries wolf on every long phase.
mkdir -p "$wbj/live-log/jobs"
printf '{"status":"running"}' >"$wbj/live-log/jobs/busy.json"
printf 'building\n' >"$wbj/live-log/jobs/busy.log"
python3 - "$wbj/live-log/jobs/busy.json" <<'AGE'
import os, sys, time
t = time.time() - 3600
os.utime(sys.argv[1], (t, t))
AGE
scripts/watch-background-jobs.sh --dir "$wbj/live-log/jobs" --state "$wbj/s3b" --stale-min 5 --once >"$wbj/o4b" 2>&1
grep -q "^STALE busy" "$wbj/o4b" && {
  echo "watch-background-jobs: reported STALE for a job whose log is advancing" >&2; exit 1; }
# ...but when BOTH files go quiet it is genuinely stale and must still report.
python3 - "$wbj/live-log/jobs/busy.log" <<'AGE'
import os, sys, time
t = time.time() - 3600
os.utime(sys.argv[1], (t, t))
AGE
scripts/watch-background-jobs.sh --dir "$wbj/live-log/jobs" --state "$wbj/s3c" --stale-min 5 --once >"$wbj/o4c" 2>&1
grep -q "^STALE busy" "$wbj/o4c" || {
  echo "watch-background-jobs: missed a genuinely stale job (all files quiet)" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s4" --baseline >/dev/null 2>&1
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --state "$wbj/s4" --once >"$wbj/o5" 2>&1
grep -q "^DONE done" "$wbj/o5" && {
  echo "watch-background-jobs: baseline did not suppress pre-existing jobs" >&2; exit 1; }
if scripts/watch-background-jobs.sh --state "$wbj/s5" --once >/dev/null 2>&1; then
  echo "watch-background-jobs: accepted a call with no --dir" >&2; exit 1
fi
# sidecar records must not double-report or bypass scope
printf '{"status":"completed","workspaceRoot":"/r/other"}' >"$wbj/jobs/side.json"
printf 'log text, not json\n'                             >"$wbj/jobs/side.log"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s7" --once >"$wbj/o7" 2>&1
grep -q "side" "$wbj/o7" && {
  echo "watch-background-jobs: sidecar record bypassed the scope filter" >&2; exit 1; }
printf '{"status":"completed","workspaceRoot":"/r/mine"}' >"$wbj/jobs/dual.json"
printf 'log text, not json\n'                            >"$wbj/jobs/dual.log"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s8" --once >"$wbj/o8" 2>&1
[ "$(grep -c '^DONE dual' "$wbj/o8")" -eq 1 ] || {
  echo "watch-background-jobs: job with a sidecar did not report exactly once" >&2; exit 1; }
printf '{"jobId":"chain-r2","parentJob":"chain","round":2,"status":"completed","workspaceRoot":"/r/mine"}' >"$wbj/jobs/chain-r2.json"
touch "$wbj/s8b"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s8b" --once >"$wbj/o8b" 2>&1
grep -q '^DONE chain-r2 status=completed' "$wbj/o8b" || {
  echo "watch-background-jobs: follow-up child was not tracked under its own id" >&2; exit 1; }
# scope: own repo and its worktrees in, peer repo and prefix-collision out
printf '{"status":"completed","workspaceRoot":"/r/mine"}'                >"$wbj/jobs/sc-mine.json"
printf '{"status":"completed","workspaceRoot":"/r/mine/.worktrees/w"}'   >"$wbj/jobs/sc-wt.json"
printf '{"status":"completed","workspaceRoot":"/r/other"}'               >"$wbj/jobs/sc-peer.json"
printf '{"status":"completed","workspaceRoot":"/r/mine-other"}'          >"$wbj/jobs/sc-prefix.json"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine --state "$wbj/s6" --once >"$wbj/o6" 2>&1
grep -q "^DONE sc-mine" "$wbj/o6" || {
  echo "watch-background-jobs: in-scope job not reported" >&2; exit 1; }
grep -q "^DONE sc-wt" "$wbj/o6" || {
  echo "watch-background-jobs: worktree job dropped by scope filter" >&2; exit 1; }
grep -q "sc-peer" "$wbj/o6" && {
  echo "watch-background-jobs: peer repository job reported" >&2; exit 1; }
grep -q "sc-prefix" "$wbj/o6" && {
  echo "watch-background-jobs: scope matched on a path prefix" >&2; exit 1; }
# distinct scopes must not share default state. Auto-baseline swallows the
# first pass per fresh default state, so warm each scope, then prove each
# reports a job that arrives after its own arming.
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine  --once >/dev/null 2>&1
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/other --once >/dev/null 2>&1
printf '{"status":"completed","workspaceRoot":"/r/mine"}'  >"$wbj/jobs/nu-mine.json"
printf '{"status":"completed","workspaceRoot":"/r/other"}' >"$wbj/jobs/nu-other.json"
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/mine  --once 2>/dev/null | grep -q "^DONE nu-mine" || {
  echo "watch-background-jobs: post-arming job not reported under its scope's default state" >&2; exit 1; }
scripts/watch-background-jobs.sh --dir "$wbj/jobs" --scope /r/other --once 2>/dev/null | grep -q "^DONE nu-other" || {
  echo "watch-background-jobs: distinct scopes shared a default state file" >&2; exit 1; }

echo "harness validation passed"
