#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

source scripts/agents/fixture-budget.sh
harness_fixture_budget_init "$root"

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

# Core assets are required everywhere. The full seven-skill set with every
# per-runtime profile is required only in the template repository (marked by
# meta/harness-design.md): adopted repositories may prune unused skills, and
# each skill present is still validated by the loop above. In adopted mode,
# any profile a remaining skill does provide must be registered without drift;
# project-added skills are not required to invent profiles they never shipped.
template_mode=0
[[ -f meta/harness-design.md ]] && template_mode=1
if (( ! template_mode )); then
  scripts/harness-config.sh validate
fi

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
  scripts/enforcement/codex-hooks.json \
  scripts/enforcement/devin-hooks.json \
  scripts/assert-stop-loss.sh \
  scripts/assert-mission.sh \
  docs/examples/mission-contract.md \
  docs/examples/mission-cron.example \
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
  scripts/agents/templates/host-turn-instruction.md \
  scripts/agents/roles/orchestrator.md \
  scripts/agents/schemas/orchestrator.schema.json \
  scripts/agents/permissions/none.json \
  scripts/agents/permissions/workspace.json \
  harness.conf \
  scripts/harness-config.sh \
  scripts/agents/dispatch.sh \
  scripts/agents/arm-supervision.sh \
  scripts/agents/process-census.py \
  scripts/agents/fixture-budget.sh \
  scripts/agents/supervision-hook.sh \
  scripts/agents/supervision-fixtures.sh \
  scripts/agents/mission-fixtures.sh \
  scripts/agents/mission-contract.py \
  scripts/agents/mission-fence.py \
  scripts/agents/mission-ledger.py \
  scripts/agents/mission-state.py \
  scripts/agents/mission-prompt.py \
  scripts/agents/mission-runner.sh \
  scripts/agents/hosts/claude.sh \
  scripts/agents/hosts/fake.sh \
  scripts/agents/schemas/mission-state.schema.json \
  scripts/agents/adapters/fake.sh \
  scripts/agents/adapters/runtime-common.sh \
  scripts/agents/adapters/claude-session-signal.py \
  scripts/agents/assert-conformance.sh \
  scripts/assert-critique-closed.sh \
  scripts/assert-return-complete.sh \
  scripts/assert-turn-prompt.sh \
  scripts/agents/check-preamble-quotes.sh; do
  [[ -e "$link" ]] || { echo "missing agent protocol asset: $link" >&2; exit 1; }
done

# Section 3.11 and retained watch-list round S4 have one bounded fixture suite.
# Process-owning groups run serially and use separate temporary repositories,
# so their supervisors and dispatch jobs cannot share lifecycle state. They
# name S4-1 through S4-10 at their owning checks and contain no uncapped
# process wait (IL-1).
if [[ -z "${HARNESS_SKIP_AGENT_FIXTURES:-}" ]]; then
  scripts/agents/supervision-fixtures.sh
  scripts/agents/mission-fixtures.sh
fi

# Real runtime selftests spend model calls and remain manual acceptance steps.
# Validation covers only their static adapter contract.
bash -n scripts/agents/arm-supervision.sh
bash -n scripts/agents/fixture-budget.sh
bash -n scripts/agents/supervision-hook.sh
bash -n scripts/agents/supervision-fixtures.sh
bash -n scripts/agents/mission-fixtures.sh
bash -n scripts/agents/mission-runner.sh
bash -n scripts/agents/hosts/claude.sh
bash -n scripts/agents/hosts/fake.sh
bash -n scripts/assert-mission.sh
bash -n scripts/assert-return-complete.sh
bash -n scripts/assert-turn-prompt.sh
bash -n scripts/watch-background-jobs.sh
bash -n scripts/agents/dispatch.sh
bash -n scripts/agents/adapters/runtime-common.sh
python3 - scripts/agents/adapters/claude-session-signal.py scripts/agents/process-census.py \
  scripts/agents/mission-contract.py scripts/agents/mission-fence.py \
  scripts/agents/mission-ledger.py scripts/agents/mission-state.py \
  scripts/agents/mission-prompt.py <<'PY'
import ast, sys
from pathlib import Path
for source in sys.argv[1:]:
    ast.parse(Path(source).read_text(encoding="utf-8"), filename=source)
PY
python3 - scripts/enforcement/claude-code-hooks.json scripts/enforcement/codex-hooks.json scripts/enforcement/devin-hooks.json <<'PY'
import json, sys
for source in sys.argv[1:]:
    value=json.load(open(source))
    hooks=value["hooks"]
    assert set(hooks) >= {"SessionStart", "Stop", "SessionEnd"}
    flattened=json.dumps(hooks)
    assert "supervision-hook.sh" in flattened
PY
for runtime in claude codex devin; do
  adapter="scripts/agents/adapters/$runtime.sh"
  [[ -f "$adapter" ]] || { echo "missing $runtime runtime adapter: $adapter" >&2; exit 1; }
  [[ -x "$adapter" ]] || { echo "$runtime runtime adapter is not executable: $adapter" >&2; exit 1; }
  bash -n "$adapter"
  adapter_usage=$($adapter --help 2>&1)
  for verb in identity signature probe dispatch follow-up cancel selftest; do
    grep -Fq "adapters/$runtime.sh $verb" <<<"$adapter_usage" \
      || { echo "$runtime adapter usage does not advertise $verb" >&2; exit 1; }
  done
  grep -Fq "adapter_common_init $runtime" "$adapter" \
    || { echo "$runtime adapter does not bind its snapshot runtime identity" >&2; exit 1; }
  grep -Fq "write_capability_snapshot $runtime \"\$version\" \"\$hash\"" "$adapter" \
    || { echo "$runtime adapter does not write its named capability snapshot" >&2; exit 1; }
done
for runtime in claude fake; do
  host="scripts/agents/hosts/$runtime.sh"
  [[ -x "$host" ]] || { echo "$runtime host adapter is missing or not executable: $host" >&2; exit 1; }
  grep -Fq 'start-turn' <<<"$($host --help 2>&1)" \
    || { echo "$runtime host adapter does not advertise start-turn" >&2; exit 1; }
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
    skills/code-critique/SKILL.md \
    skills/code-critique/agents/claude-profile.md \
    skills/code-critique/agents/devin/AGENT.md \
    skills/code-critique/agents/openai.yaml \
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
for regroot in .claude/skills .agents/skills .devin/skills; do
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

# In adopted mode harness.runtimes is live truth for registrations. Every
# remaining skill must be discoverable by each selected runtime, and copied
# launcher profiles must remain byte-identical to their canonical source.
if (( ! template_mode )); then
  configured_runtimes=$(sed -n 's/^harness\.runtimes=//p' harness.conf)
  runtime_selected() { [[ ",$configured_runtimes," == *",$1,"* ]]; }
  for skill_dir in skills/*/; do
    [[ -d "$skill_dir" ]] || continue
    name=$(basename "$skill_dir")
    if runtime_selected claude; then
      [[ -e ".claude/skills/$name" ]] \
        || { echo "claude registration missing for skill: $name" >&2; exit 1; }
      if [[ -f "$skill_dir/agents/claude-profile.md" ]]; then
        profile=".claude/agents/$name.md"
        [[ -f "$profile" ]] || { echo "claude profile registration missing: $profile" >&2; exit 1; }
        cmp -s "$skill_dir/agents/claude-profile.md" "$profile" \
          || { echo "claude profile drifted from $skill_dir/agents/claude-profile.md: $profile" >&2; exit 1; }
      fi
    fi
    if runtime_selected codex || runtime_selected devin; then
      [[ -e ".agents/skills/$name" ]] \
        || { echo "shared .agents skill registration missing: $name" >&2; exit 1; }
    fi
    if runtime_selected devin; then
      [[ -e ".devin/skills/$name" ]] \
        || { echo "devin skill registration missing: $name" >&2; exit 1; }
      if [[ -f "$skill_dir/agents/devin/AGENT.md" ]]; then
        profile=".devin/agents/$name/AGENT.md"
        [[ -f "$profile" ]] || { echo "devin profile registration missing: $profile" >&2; exit 1; }
        cmp -s "$skill_dir/agents/devin/AGENT.md" "$profile" \
          || { echo "devin profile drifted from $skill_dir/agents/devin/AGENT.md: $profile" >&2; exit 1; }
      fi
    fi
  done
fi

tmp=$(mktemp -d)
agent_supervision_repo=
validation_cleanup() {
  if [[ -n "${agent_supervision_repo:-}" && -x "$agent_supervision_repo/scripts/agents/arm-supervision.sh" ]]; then
    if [[ "$agent_supervision_repo" == "${runner_repo:-}" ]] \
        && declare -p runner_process_env >/dev/null 2>&1; then
      "${runner_process_env[@]}" "$agent_supervision_repo/scripts/agents/arm-supervision.sh" \
        --repo "$agent_supervision_repo" --shutdown >/dev/null 2>&1 || true
    else
      "$agent_supervision_repo/scripts/agents/arm-supervision.sh" \
        --repo "$agent_supervision_repo" --shutdown >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$tmp"
}
trap validation_cleanup EXIT

# IL-3: prove the audit's fallback with a PATH that contains its ordinary POSIX
# tools but deliberately contains no rg binary.
no_rg_bin="$tmp/no-rg-bin"
mkdir -p "$no_rg_bin"
for command_name in cat find grep sort tr wc; do
  ln -s "$(command -v "$command_name")" "$no_rg_bin/$command_name"
done
env PATH="$no_rg_bin" /bin/bash scripts/audit-harness.sh . >"$tmp/audit-no-rg.out"

fill_harness_conf() { # config path, absolute evidence root
  python3 - "$1" "$2" <<'PY'
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
evidence = sys.argv[2]
model_key = re.compile(
    r"^(?:role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+)\.model\.([a-z0-9-]+)$"
)
lines = []
models = set()
for raw in path.read_text(encoding="utf-8").splitlines():
    if "=" not in raw:
        lines.append(raw)
        continue
    key, value = raw.split("=", 1)
    match = model_key.fullmatch(key)
    if key == "evidence.root":
        value = evidence
    elif match:
        runtime = match.group(1)
        value = f"fixture-{runtime}-model"
        models.add(f"{runtime}:{value}")
    elif key == "model.tier.1":
        value = "__MODELS__"
    elif key.startswith("model.tier."):
        value = ""
    lines.append(f"{key}={value}")
joined = "\n".join(lines).replace("model.tier.1=__MODELS__", f"model.tier.1={','.join(sorted(models))}")
path.write_text(joined + "\n", encoding="utf-8")
PY
}

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
python3 - scripts/agents/templates/host-turn-instruction.md <<'PY'
import re
import sys
from pathlib import Path

body = Path(sys.argv[1]).read_text(encoding="utf-8")
parameters = re.findall(r"<([^<>]+)>", body)
if parameters != ["cycle-number", "fence-headroom", "yes | no"]:
    raise SystemExit("host-turn instruction parameters drifted from cycle, fence headroom, reconciliation")
if "Runtime:" in body:
    raise SystemExit("host-turn instruction is parameterized by runtime")
PY

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

orchestrator_path = root / "scripts" / "agents" / "schemas" / "orchestrator.schema.json"
orchestrator = json.loads(orchestrator_path.read_text())
orchestrator_fields = {
    "turnId", "missionId", "cycle", "dispatched", "certified",
    "streamUpdatesRequested", "askCandidates", "factsForLedger", "gaps", "identity",
}
if (
    set(orchestrator.get("properties", {})) != orchestrator_fields
    or set(orchestrator.get("required", [])) != orchestrator_fields
    or orchestrator.get("additionalProperties") is not False
):
    raise SystemExit("orchestrator schema property set drifted from the host-turn protocol")


def assert_closed_schema(node, path):
    if node.get("type") == "object":
        properties = node.get("properties", {})
        if node.get("additionalProperties") is not False or set(node.get("required", [])) != set(properties):
            raise SystemExit(f"orchestrator schema object is not fully enumerated: {path}")
        for name, child in properties.items():
            assert_closed_schema(child, f"{path}.{name}")
    if node.get("type") == "array":
        assert_closed_schema(node.get("items", {}), f"{path}[]")


assert_closed_schema(orchestrator, "$")

permission_expected = {
    # Network is granted by default: the container or VM is the isolation
    # boundary, and a delegate that cannot resolve a dependency or read
    # documentation cannot do real work. A repository narrows it for every role
    # with dispatch.permissions.network=deny.
    "none": {
        "readRoots": ["."], "writeRoots": [], "network": "allow",
        "approvals": "deny", "tools": "read-only",
    },
    "workspace": {
        "readRoots": ["."], "writeRoots": ["<worktree>"], "network": "allow",
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
python3 - scripts/agents/roles/orchestrator.md <<'PY'
import re
import sys
from pathlib import Path

body = Path(sys.argv[1]).read_bytes()
pattern = re.compile(
    br'^<!-- quote source="([^"\r\n]+)" -->\n(.*?)^<!-- /quote -->$',
    re.MULTILINE | re.DOTALL,
)
required = [
    (b"AGENTS.md", b"## Completion\n"),
    (b"docs/orchestration.md", b"## Delegation Contract\n"),
    (b"docs/orchestration.md", b"### Working without the human\n"),
    (b"docs/collaboration.md", b"## Review Guide in Reports\n"),
    (b"docs/collaboration.md", b"## Escalation Shape\n"),
    (b"docs/project-rules.md", b"These require explicit in-task approval"),
]


def matches(value):
    return list(pattern.finditer(value))


def missing(value):
    blocks = matches(value)
    return [
        (source, marker)
        for source, marker in required
        if not any(match.group(1) == source and marker in match.group(2) for match in blocks)
    ]


absent = missing(body)
if absent:
    raise SystemExit(f"orchestrator preamble lacks mandated quote blocks: {absent!r}")
for source, marker in required:
    match = next(
        item for item in matches(body)
        if item.group(1) == source and marker in item.group(2)
    )
    deleted = body[:match.start()] + body[match.end():]
    if (source, marker) not in missing(deleted):
        raise SystemExit(
            f"orchestrator required-block fixture did not detect deletion: {source!r} {marker!r}"
        )
PY
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
cp -R scripts/agents/roles "$tmp/drifted-code-roles"
sed 's/ship a defect/ship no defect/' \
  "$tmp/drifted-code-roles/code-critic.md" >"$tmp/drifted-code-roles/code-critic.md.new"
mv "$tmp/drifted-code-roles/code-critic.md.new" "$tmp/drifted-code-roles/code-critic.md"
set +e
scripts/agents/check-preamble-quotes.sh --roles-dir "$tmp/drifted-code-roles" >"$tmp/code-quote-drift.out" 2>&1
code_quote_status=$?
set -e
[[ $code_quote_status -eq 1 ]] \
  || { echo "preamble quote checker accepted a drifted code-critique criterion" >&2; exit 1; }
grep -q 'quote drifted from skills/code-critique/SKILL.md' "$tmp/code-quote-drift.out" \
  || { echo "preamble quote checker did not name the code-critique source" >&2; exit 1; }
cp -R scripts/agents/roles "$tmp/drifted-orchestrator-roles"
sed "s/The human's absence narrows/The human's presence narrows/" \
  "$tmp/drifted-orchestrator-roles/orchestrator.md" >"$tmp/drifted-orchestrator-roles/orchestrator.md.new"
mv "$tmp/drifted-orchestrator-roles/orchestrator.md.new" \
  "$tmp/drifted-orchestrator-roles/orchestrator.md"
set +e
scripts/agents/check-preamble-quotes.sh \
  --roles-dir "$tmp/drifted-orchestrator-roles" >"$tmp/orchestrator-quote-drift.out" 2>&1
orchestrator_quote_status=$?
set -e
[[ $orchestrator_quote_status -eq 1 ]] \
  || { echo "preamble quote checker accepted a drifted orchestrator quote" >&2; exit 1; }
grep -q 'quote drifted from docs/orchestration.md' "$tmp/orchestrator-quote-drift.out" \
  || { echo "preamble quote checker did not name the drifted orchestrator source" >&2; exit 1; }

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
    "orchestrator": {
        "turnId": "turn-3",
        "missionId": "fixture-mission",
        "cycle": 3,
        "dispatched": [{"jobId": "fixture-job", "role": "implementer", "stream": "stream-a"}],
        "certified": [{"jobId": "prior-job", "verdict": "accepted", "evidence": "focused checks passed"}],
        "streamUpdatesRequested": [{"streamId": "stream-a", "requestedState": "active", "reason": "work remains"}],
        "askCandidates": [{"streamId": "stream-b", "reasonClass": "reserved-decision", "question": "Approve the contract change?"}],
        "factsForLedger": ["focused check exposed one new fact"],
        "gaps": [],
        "identity": {"runtime": "fake", "model": "fake-model", "sessionId": None},
    },
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
negative["orchestrator"].pop("factsForLedger")
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

for role in orchestrator design-critic implementer code-critic verifier investigator; do
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

check_bad_return orchestrator "$return_fixtures/orchestrator-negative.json" '$.factsForLedger is required'
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

# Host-turn prompts are checked against the canonical turn record and shipped
# preamble before a host process may start. The positive prompt is deliberately
# hand-authored so this fixture does not share assembly logic with the checker.
turn_fixture="$tmp/turn-prompt"
turn_dir="$turn_fixture/turn-3"
mkdir -p "$turn_dir"
cat >"$turn_dir/turn.json" <<'EOF'
{
  "missionId": "fixture-mission",
  "turnId": "turn-3",
  "cycle": 3,
  "runtime": "fake",
  "model": "fake-model",
  "hostSession": null,
  "reconciliation": false,
  "startedAt": "2026-08-04T12:00:00Z",
  "pid": 1234,
  "outcome": null
}
EOF
good_turn_prompt="$turn_fixture/good.md"
{
  printf '%s\n' \
    'Mission-Id: fixture-mission' \
    'Turn-Id: turn-3' \
    'Cycle: 3' \
    'Host-Session: none' \
    'Runtime: fake' \
    'Model: fake-model' \
    'Reconciliation: no'
  printf '\n'
  cat scripts/agents/roles/orchestrator.md
  printf '\n'
  cat <<'EOF'
## Mission Contract
Signed fixture mission contract.

## Ledger Tail
<<<DATA>>>
1	contract-improved	aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa	metric=1
2	unresolved	bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb	metric=1
<<<END>>>

## Open Asks
<<<DATA>>>
ask-1	stream-a	reserved-decision	Approve the named API contract?
<<<END>>>

## Streams
<<<DATA>>>
stream-a	active	Make the fixture gate pass	none
stream-b	parked-reserved	Publish the fixture	Awaiting approval
<<<END>>>

## Reconciliation
<<<DATA>>>
(none)
<<<END>>>

## This Turn
Cycle: 3
Fence headroom: cycles=2,jobs=3
Reconciliation: no

Advance active streams by designing, dispatching, reviewing, and certifying. When Reconciliation is `yes`, reconcile the prior turn before starting new work. End this turn when work is dispatched and reviewed; never wait inside the turn.
EOF
} >"$good_turn_prompt"

scripts/assert-turn-prompt.sh --file "$good_turn_prompt" --turn "$turn_dir"

python3 - "$good_turn_prompt" "$turn_fixture" <<'PY'
import sys
from pathlib import Path

source = Path(sys.argv[1]).read_text(encoding="utf-8")
out = Path(sys.argv[2])
mutations = {
    "missing-header": source.replace("Model: fake-model\n", "", 1),
    "turn-mismatch": source.replace("Turn-Id: turn-3\n", "Turn-Id: turn-other\n", 1),
    "mission-mismatch": source.replace("Mission-Id: fixture-mission\n", "Mission-Id: other-mission\n", 1),
    "altered-preamble": source.replace(
        "You are the orchestrator for an unattended mission.",
        "You are an orchestrator for an unattended mission.",
        1,
    ),
    "headings-out-of-order": source.replace("## Open Asks", "## TEMP", 1)
        .replace("## Streams", "## Open Asks", 1)
        .replace("## TEMP", "## Streams", 1),
    "unfenced-data": source.replace(
        "## Open Asks\n<<<DATA>>>\nask-1\tstream-a\treserved-decision\tApprove the named API contract?\n<<<END>>>",
        "## Open Asks\nask-1\tstream-a\treserved-decision\tApprove the named API contract?",
        1,
    ),
    "malformed-record": source.replace(
        "ask-1\tstream-a\treserved-decision\tApprove the named API contract?",
        "ask-1\tstream-a\treserved-decision",
        1,
    ),
}
for name, value in mutations.items():
    if value == source:
        raise SystemExit(f"turn-prompt mutation did not change fixture: {name}")
    (out / f"{name}.md").write_text(value, encoding="utf-8")
PY

check_bad_turn_prompt() { # fixture name, failing check
  local name=$1 expected=$2 output status
  output="$turn_fixture/$name.out"
  set +e
  scripts/assert-turn-prompt.sh \
    --file "$turn_fixture/$name.md" --turn "$turn_dir" >"$output" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "turn prompt checker accepted the negative $name fixture" >&2
    exit 1
  fi
  [[ $status -eq 1 ]] \
    || { echo "turn prompt checker used $status instead of exit 1 for $name" >&2; exit 1; }
  grep -Fq "[$expected]" "$output" \
    || { echo "turn prompt checker did not name the $expected check for $name" >&2; exit 1; }
}

check_bad_turn_prompt missing-header headers
check_bad_turn_prompt turn-mismatch identity
check_bad_turn_prompt mission-mismatch identity
check_bad_turn_prompt altered-preamble preamble
check_bad_turn_prompt headings-out-of-order headings
check_bad_turn_prompt unfenced-data fencing
check_bad_turn_prompt malformed-record records

set +e
scripts/assert-turn-prompt.sh >"$turn_fixture/usage.out" 2>&1
turn_prompt_usage_status=$?
set -e
[[ $turn_prompt_usage_status -eq 2 ]] \
  || { echo "turn prompt checker used $turn_prompt_usage_status instead of exit 2 for usage" >&2; exit 1; }

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

# Plan consistency: a rule stated in several places must not disagree with
# itself. Eight of nine rounds of one design critique found nothing else, and a
# paid round is the wrong instrument for drift a script finds instantly.
scripts/assert-plan-consistency.sh >"$tmp/plan-consistency.out"
grep -q 'retired term' "$tmp/plan-consistency.out" \
  || { echo "plan consistency check did not report its retired terms" >&2; exit 1; }

plan_fixture=$tmp/plan-consistency
mkdir -p "$plan_fixture"
cat >"$plan_fixture/owner.md" <<'FIXTURE'
RETIRED: widget check -- the gadget check
FIXTURE
cat >"$plan_fixture/clean.md" <<'FIXTURE'
The widget check was replaced by the gadget check.
FIXTURE
scripts/assert-plan-consistency.sh --plans-dir "$plan_fixture" >/dev/null \
  || { echo "plan consistency rejected a line that explains the change" >&2; exit 1; }
cat >"$plan_fixture/stale.md" <<'FIXTURE'
Test quality is measured by the widget check.
FIXTURE
set +e
scripts/assert-plan-consistency.sh --plans-dir "$plan_fixture" >"$tmp/plan-stale.out" 2>&1
plan_status=$?
set -e
(( plan_status == 1 )) \
  || { echo "plan consistency did not fail on a prescribed retired term" >&2; exit 1; }
grep -q "stale.md:1: prescribes 'widget check'" "$tmp/plan-stale.out" \
  || { echo "plan consistency did not name the file, line, and term" >&2; cat "$tmp/plan-stale.out" >&2; exit 1; }

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
  mkdir -p "$agent_repo/scripts" "$agent_repo/docs"
  agent_repo=$(cd "$agent_repo" && pwd -P)
  cp -R scripts/agents "$agent_repo/scripts/"
  cp scripts/harness-config.sh scripts/assert-mission.sh scripts/assert-stop-loss.sh \
    scripts/assert-return-complete.sh scripts/assert-turn-prompt.sh \
    scripts/watch-background-jobs.sh "$agent_repo/scripts/"
  cp docs/project-rules.md "$agent_repo/docs/"
  cp harness.conf "$agent_repo/"
  perl -0pi -e 's/^harness\.runtimes=.*$/harness.runtimes=fake/m; s|^evidence\.root=.*$|evidence.root='"$agent_evidence"'|m; s/^watch\.interval-sec=.*$/watch.interval-sec=5/m; s/^census\.log-max-bytes=.*$/census.log-max-bytes=4096/m; s/^model\.tier\.1=.*$/model.tier.1=fake:fake-model/m; s/^model\.tier\.2=.*$/model.tier.2=/m; s/^model\.tier\.3=.*$/model.tier.3=/m; s/^role\.default\.runtime=.*$/role.default.runtime=fake/m; s/^role\.default\.model\.codex=.*$/role.default.model.fake=fake-model/m; s/^role\.default\.model\.(?:claude|devin)=.*\n//mg; s/\.runtime=(?:codex|devin)$/\.runtime=fake/mg; s/\.model\.(?:codex|devin)=.*$/\.model.fake=fake-model/mg' "$agent_repo/harness.conf"
  grep -q '^watch\.interval-sec=' "$agent_repo/harness.conf" || printf 'watch.interval-sec=5\n' >>"$agent_repo/harness.conf"
  grep -q '^census\.log-max-bytes=' "$agent_repo/harness.conf" || printf 'census.log-max-bytes=4096\n' >>"$agent_repo/harness.conf"
  git -C "$agent_repo" init -q
  git -C "$agent_repo" add .
  git -C "$agent_repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm base
  agent_dispatch="$agent_repo/scripts/agents/dispatch.sh"
  fake_adapter="$agent_repo/scripts/agents/adapters/fake.sh"
  agent_config="$agent_repo/scripts/harness-config.sh"
  good_agent_conf="$agent_fixture/good-harness.conf"
  cp "$agent_repo/harness.conf" "$good_agent_conf"

  # The mission runner and compound adapter selftest each get a pristine
  # repository and supervisor. They run only after the main dispatch fixture
  # set has shut down, so neither can queue behind its fixture state or reuse
  # its synthetic-process supervision set.
  runner_repo="$agent_fixture/runner-repo"
  runner_evidence="$agent_fixture/runner-evidence"
  cp -R "$agent_repo" "$runner_repo"
  runner_repo=$(cd "$runner_repo" && pwd -P)
  perl -0pi -e 's|^evidence\.root=.*$|evidence.root='"$runner_evidence"'|m' \
    "$runner_repo/harness.conf"
  agent_selftest_repo="$agent_fixture/selftest-repo"
  agent_selftest_evidence="$agent_fixture/selftest-evidence"
  cp -R "$agent_repo" "$agent_selftest_repo"
  agent_selftest_repo=$(cd "$agent_selftest_repo" && pwd -P)
  perl -0pi -e 's|^evidence\.root=.*$|evidence.root='"$agent_selftest_evidence"'|m' \
    "$agent_selftest_repo/harness.conf"

  agent_fixture_base_cap_sec=${HARNESS_AGENT_FIXTURE_TIMEOUT_SEC:-20}
  [[ "$agent_fixture_base_cap_sec" =~ ^[1-9][0-9]*$ && "$agent_fixture_base_cap_sec" -le 120 ]] \
    || { echo "HARNESS_AGENT_FIXTURE_TIMEOUT_SEC must be an integer from 1 through 120" >&2; exit 1; }
  agent_fixture_cap_sec=$(harness_fixture_scaled_cap "$agent_fixture_base_cap_sec")
  agent_status_cap_sec=$(harness_fixture_scaled_cap 5)
  agent_cleanup_cap_sec=$(harness_fixture_scaled_cap 7)
  agent_driver_stop_cap_sec=$(harness_fixture_scaled_cap 2)

  wait_for_agent_census_fresh() { # fixture name
    local name=$1 started=$SECONDS deadline=$((SECONDS + agent_fixture_cap_sec)) expected elapsed
    [[ -n "${agent_supervision_repo:-}" ]] || return 0
    while (( SECONDS < deadline )); do
      expected=$("$agent_supervision_repo/scripts/agents/arm-supervision.sh" \
        fingerprint --repo "$agent_supervision_repo" 2>/dev/null || true)
      if [[ -n "$expected" ]] && python3 - \
          "$agent_supervision_repo/artifacts/agents/supervision/last-census.json" \
          "$expected" <<'PY'
import json,sys,time
try: value=json.load(open(sys.argv[1]))
except (OSError,ValueError): raise SystemExit(1)
age=int(time.time())-int(value.get("completedAtEpoch",0)); interval=int(value.get("intervalSec",0) or 0)
raise SystemExit(0 if value.get("verdict")=="SUCCESS" and value.get("fingerprint")==sys.argv[2] and 0 <= age <= max(1,interval//2) else 1)
PY
      then return 0; fi
      sleep 0.05
    done
    elapsed=$((SECONDS - started))
    echo "agent fixture timed out waiting for a fresh census: $name (elapsed: ${elapsed}s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
    return 1
  }

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

  agent_fixture_diagnostics() { # fixture name, job id or -, elapsed seconds
    local name=$1 job=$2 elapsed=$3 path
    echo "agent fixture timed out: $name (job: $job; elapsed: ${elapsed}s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
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

  stop_timed_out_agent_fixture() { # fixture name, job id or -, driver pid, wait start
    local name=$1 job=$2 driver_pid=$3 wait_started=$4 cleanup_pid cleanup_started cleanup_deadline driver_started driver_deadline elapsed status=
    elapsed=$((SECONDS - wait_started))
    agent_fixture_diagnostics "$name" "$job" "$elapsed"
    if [[ "$job" != - && -f "$agent_repo/artifacts/agents/jobs/$job.json" ]]; then
      status=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("status", "malformed"))' \
        "$agent_repo/artifacts/agents/jobs/$job.json" 2>/dev/null || true)
      if [[ "$status" == pending || "$status" == running ]]; then
        "$agent_dispatch" cancel --job "$job" >"$agent_fixture/$name-timeout-cancel.out" 2>&1 &
        cleanup_pid=$!
        cleanup_started=$SECONDS
        cleanup_deadline=$(( SECONDS + agent_cleanup_cap_sec ))
        while kill -0 "$cleanup_pid" 2>/dev/null && (( SECONDS < cleanup_deadline )); do sleep 0.05; done
        if kill -0 "$cleanup_pid" 2>/dev/null; then
          elapsed=$((SECONDS - cleanup_started))
          echo "agent fixture cleanup timed out: $name cancel pid $cleanup_pid (elapsed: ${elapsed}s; scaled cap: ${agent_cleanup_cap_sec}s)" >&2
          kill -TERM "$cleanup_pid" 2>/dev/null || true
          sleep 0.1
          kill -KILL "$cleanup_pid" 2>/dev/null || true
        fi
      fi
    fi
    if kill -0 "$driver_pid" 2>/dev/null; then
      kill -TERM "$driver_pid" 2>/dev/null || true
      driver_started=$SECONDS
      driver_deadline=$(( SECONDS + agent_driver_stop_cap_sec ))
      while kill -0 "$driver_pid" 2>/dev/null && (( SECONDS < driver_deadline )); do sleep 0.05; done
      if kill -0 "$driver_pid" 2>/dev/null; then
        elapsed=$((SECONDS - driver_started))
        echo "agent fixture driver stop timed out: $name pid $driver_pid (elapsed: ${elapsed}s; scaled cap: ${agent_driver_stop_cap_sec}s); sending KILL" >&2
        kill -KILL "$driver_pid" 2>/dev/null || true
      fi
    fi
    exit 1
  }

  wait_for_agent_fixture_process() { # fixture name, job id or -, exact child pid
    local name=$1 job=$2 child_pid=$3 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec )) result
    while kill -0 "$child_pid" 2>/dev/null; do
      (( SECONDS < deadline )) || stop_timed_out_agent_fixture "$name" "$job" "$child_pid" "$started"
      sleep 0.05
    done
    if wait "$child_pid"; then result=0; else result=$?; fi
    return "$result"
  }

  run_agent_fixture() { # fixture name, job id or -, command...
    local name=$1 job=$2 child_pid
    shift 2
    case ${2:-} in dispatch|follow-up) wait_for_agent_census_fresh "$name" ;; esac
    "$@" &
    child_pid=$!
    wait_for_agent_fixture_process "$name" "$job" "$child_pid"
  }

  run_agent_fixture_captured() { # fixture name, job id or -, output file, command...
    local name=$1 job=$2 output=$3 child_pid
    shift 3
    case ${2:-} in dispatch|follow-up) wait_for_agent_census_fresh "$name" ;; esac
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
    local job=$1 expected=$2 observed= started=$SECONDS deadline=$((SECONDS + agent_status_cap_sec)) elapsed
    while (( SECONDS < deadline )); do
      observed=$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job "$job" 2>/dev/null || true)
      [[ "$observed" == "$expected" ]] && return 0
      sleep 0.05
    done
    elapsed=$((SECONDS - started))
    echo "agent fixture status timed out: $job -> $expected (last status: ${observed:-missing}; elapsed: ${elapsed}s; scaled cap: ${agent_status_cap_sec}s)" >&2
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
  # An uncommitted local file carries values that must not ship to adopting
  # projects. It outranks the committed conf and yields to the environment.
  cat >"$config_order/harness.conf.local" <<'EOF'
plain.knob=local-value
EOF
  [[ "$(env -u HARNESS_PLAIN_KNOB "$config_order/scripts/harness-config.sh" get --key plain.knob)" == local-value ]] \
    || { echo "harness config did not prefer the local override" >&2; exit 1; }
  [[ "$(HARNESS_PLAIN_KNOB=environment "$config_order/scripts/harness-config.sh" get --key plain.knob)" == environment ]] \
    || { echo "local override outranked the environment" >&2; exit 1; }
  [[ "$(env -u HARNESS_ROLE_IMPLEMENTER_RUNTIME "$config_order/scripts/harness-config.sh" get --key role.implementer.runtime --mode refactor)" == mode ]] \
    || { echo "local override disturbed a key it does not carry" >&2; exit 1; }
  rm -f "$config_order/harness.conf.local"

  "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's/^role\.design-critic\.runtime=.*$/role.design-critic.runtime=ghost/m' "$agent_repo/harness.conf"
  agent_fails invalid-role-runtime 'outside harness.runtimes' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's/^mode\.refactor\.role\.implementer\.runtime=.*$/mode.refactor.role.implementer.runtime=ghost/m' "$agent_repo/harness.conf"
  agent_fails invalid-mode-runtime 'outside harness.runtimes' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  printf 'role.default.model.ghost=ghost-model\n' >>"$agent_repo/harness.conf"
  agent_fails invalid-model-runtime 'outside harness.runtimes' "$agent_config" validate
  cp "$good_agent_conf" "$agent_repo/harness.conf"
  perl -0pi -e 's/^harness\.runtimes=.*$/harness.runtimes=ghost/m' "$agent_repo/harness.conf"
  agent_fails unsupported-runtime 'unsupported runtime' "$agent_config" validate
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

  # All remaining dispatch fixtures run behind a real armed fake-runtime set.
  # The explicit synthetic process table is fixture-only and keeps this test
  # deterministic in restricted environments where ps enumeration is denied.
  agent_process_fixture="$agent_fixture/processes.json"
  agent_identity_fixture="$agent_fixture/process-identities.json"
  printf '[]\n' >"$agent_process_fixture"
  printf '{}\n' >"$agent_identity_fixture"
  export HARNESS_CENSUS_PROCESS_FILE="$agent_process_fixture"
  export HARNESS_FAKE_PROCESS_IDENTITY_FILE="$agent_identity_fixture"
  agent_supervision_repo=$agent_repo
  agent_main_start=$("$agent_repo/scripts/agents/process-census.py" started-at --pid "$$")
  HARNESS_AGENT_RUNTIME=fake "$agent_repo/scripts/agents/arm-supervision.sh" \
    --repo "$agent_repo" --session validator --pid "$$" \
    --start-time "$agent_main_start" --tag harness-main-fake-validator \
    >"$agent_fixture/arming.out"

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
    "workspaceRoot", "baseSha", "branch", "permissions", "capMin", "pid", "pidStartedAt", "pgid", "instanceTag", "custodyProcesses",
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
  wait_for_agent_census_fresh pending-chain
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh dispatch --role design-critic --brief "$pending_brief" --job-id pending-chain >/dev/null 2>&1) & pending_driver=$!
  wait_for_agent_status pending-chain pending
  pending_message="$agent_fixture/pending-follow.md"
  cp "$agent_repo/scripts/agents/templates/follow-up.md" "$pending_message"
  agent_fails pending-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job pending-chain --message "$pending_message"
  wait_for_agent_fixture_process pending-chain-driver pending-chain "$pending_driver" || true

  pending_loss_brief="$agent_fixture/pending-loss.md"
  make_agent_brief "$pending_loss_brief" design 'FAKE:pending-process-loss'
  wait_for_agent_census_fresh pending-loss
  (set +e; cd "$agent_repo"; scripts/agents/dispatch.sh dispatch --role design-critic --brief "$pending_loss_brief" --job-id pending-loss >/dev/null 2>&1) & pending_loss_driver=$!
  wait_for_agent_status pending-loss pending
  # The supervisor is dying while this runs, so a single sweep can legitimately
  # land before the kill and observe a live match. The standing reaper sweeps on
  # an interval, so sweeping until the transition or a scaled ceiling is the
  # faithful check; the assertion itself is unchanged.
  pending_loss_started=$SECONDS
  pending_loss_deadline=$((SECONDS + agent_status_cap_sec))
  until grep -Fq 'process-lost' "$agent_repo/artifacts/agents/jobs/pending-loss.json"; do
    if (( SECONDS >= pending_loss_deadline )); then
      echo "dead pending supervisor did not transition through reap (elapsed: $((SECONDS - pending_loss_started))s; scaled cap: ${agent_status_cap_sec}s)" >&2
      exit 1
    fi
    run_agent_fixture pending-loss-reap pending-loss "$agent_dispatch" reap --job pending-loss
    sleep 0.1
  done
  wait_for_agent_fixture_process pending-loss-driver pending-loss "$pending_loss_driver" || true

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

  # The fake reports network access it was not granted. Now that the presets
  # grant it by default, the request has to withhold it explicitly for the
  # report to be wider than the request at all.
  restrictive_permissions="$agent_fixture/restrictive-permissions.json"
  printf '{"readRoots":["."],"writeRoots":[],"network":"deny","approvals":"deny","tools":"read-only"}\n' >"$restrictive_permissions"
  effective_wider="$agent_fixture/effective-wider.md"
  make_agent_brief "$effective_wider" design 'FAKE:effective-wider'
  agent_fails effective-wider '' "$agent_dispatch" dispatch --role design-critic --brief "$effective_wider" --permissions "$restrictive_permissions" --job-id effective-wider --wait
  grep -Fq 'permissions_mismatch:network' "$agent_repo/artifacts/agents/jobs/effective-wider.json" \
    || { echo "wider effective envelope did not record the mismatch" >&2; exit 1; }
  permissive_permissions="$agent_fixture/permissive-permissions.json"
  printf '{"readRoots":["."],"writeRoots":[],"network":"allow","approvals":"deny","tools":"read-only"}\n' >"$permissive_permissions"
  effective_narrower="$agent_fixture/effective-narrower.md"
  make_agent_brief "$effective_narrower" design 'FAKE:effective-narrower'
  run_agent_fixture effective-narrower effective-narrower "$agent_dispatch" dispatch --role design-critic --brief "$effective_narrower" --permissions "$permissive_permissions" --job-id effective-narrower --wait

  # The shipped presets grant network, and a repository may narrow it for every
  # role at once. Until 2026-08-05 the adapters hard-coded network off and never
  # read the field, so a job could be recorded as networked and still be cut off
  # (KI-12); these fixtures exist so that cannot recur silently.
  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["network"])' scripts/agents/permissions/workspace.json)" == allow ]] \
    || { echo "the workspace preset no longer grants network" >&2; exit 1; }
  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["network"])' scripts/agents/permissions/none.json)" == allow ]] \
    || { echo "the none preset no longer grants network" >&2; exit 1; }
  net_default="$agent_fixture/net-default.md"
  make_agent_brief "$net_default" design
  run_agent_fixture net-default net-default "$agent_dispatch" dispatch --role design-critic --brief "$net_default" --job-id net-default --wait
  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["permissions"]["requested"]["network"])' "$agent_repo/artifacts/agents/jobs/net-default.json")" == allow ]] \
    || { echo "a delegate did not receive network by default" >&2; exit 1; }
  printf 'dispatch.permissions.network=deny\n' >>"$agent_repo/harness.conf"
  net_floor="$agent_fixture/net-floor.md"
  make_agent_brief "$net_floor" design
  run_agent_fixture net-floor net-floor "$agent_dispatch" dispatch --role design-critic --brief "$net_floor" --job-id net-floor --wait
  [[ "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["permissions"]["requested"]["network"])' "$agent_repo/artifacts/agents/jobs/net-floor.json")" == deny ]] \
    || { echo "the repository network floor did not narrow the preset" >&2; exit 1; }
  perl -0pi -e 's/^dispatch\.permissions\.network=deny\n//m' "$agent_repo/harness.conf"
  agent_fails invalid-network-floor 'must be deny or allow' \
    env HARNESS_DISPATCH_PERMISSIONS_NETWORK=sometimes "$agent_dispatch" dispatch --role design-critic --brief "$net_default" --job-id bad-floor

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
  wait_for_agent_census_fresh timed
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
  wait_for_agent_census_fresh cancelled
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
  wait_for_agent_census_fresh vanished
  (
    set +e
    cd "$agent_repo"
    scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id vanished --wait
    printf '%s\n' "$?" >"$vanished_result"
  ) &
  vanished_driver=$!
  vanished_wait_started=$SECONDS
  vanished_wait_deadline=$((SECONDS + agent_status_cap_sec))
  while (( SECONDS < vanished_wait_deadline )); do
    [[ -f "$agent_repo/artifacts/agents/hb/vanished.waiting" ]] && break
    sleep 0.05
  done
  [[ -f "$agent_repo/artifacts/agents/hb/vanished.waiting" ]] \
    || { vanished_wait_elapsed=$((SECONDS - vanished_wait_started)); echo "agent fixture file wait timed out: vanished.waiting (job: vanished; elapsed: ${vanished_wait_elapsed}s; scaled cap: ${agent_status_cap_sec}s)" >&2; exit 1; }
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
  wait_for_agent_census_fresh close-race-follow
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
  # This refusal is about a runtime that cannot verify a field the envelope
  # restricts, so the envelope has to restrict it. Since the presets now grant
  # network, the request is made restrictive explicitly rather than by default.
  agent_fails unverified-deny 'cannot verify restrictive permission field network' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --permissions "$restrictive_permissions" --job-id unverified-deny
  perl -0pi -e 's/"optional": \{\}/"optional": {},\n  "waivers": {"network": ["fake"]}/' "$requirements"
  run_agent_fixture waived-deny waived-deny "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --permissions "$restrictive_permissions" --job-id waived-deny --wait
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
  cat >"$agent_repo/plans/mission-mission-alpha.contract.md" <<'EOF'
```mission
fence.wall-clock-hours=2
fence.cycles=10
fence.jobs=20
fence.concurrency=2
fence.job-cap-min=120
```
EOF
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
    HARNESS_MISSION_TURN=mission-alpha-t1-fixture \
    run_agent_fixture mission-inherited mission-inherited "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-inherited --wait
  agent_fails mission-cap 'lifecycle fence' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-cap --mission mission-alpha --cap-min 121
  [[ -f "$agent_repo/artifacts/agents/missions/mission-alpha/asks/fence-bound.json" ]] \
    || { echo "job-cap refusal did not write the batched mission ask" >&2; exit 1; }
  python3 - "$agent_repo" <<'PY'
import json, sys
from pathlib import Path
root=Path(sys.argv[1]); usage=json.loads((root/"artifacts/agents/missions/mission-alpha/usage.json").read_text())
units={(item["provider"],item["unit"]):item["value"] for item in usage["units"]}
assert units[("fake","provider.fake-unit")] == 2
for job in ("mission-explicit","mission-inherited"):
    prompt=(root/f"artifacts/agents/{job}/rounds/1/prompt.md").read_text()
    assert "\nMission: mission-alpha\n" in prompt
inherited=json.loads((root/"artifacts/agents/jobs/mission-inherited.json").read_text())
assert inherited["turnId"] == "mission-alpha-t1-fixture"
PY

  make_fence_mission() { # mission id, cycles, jobs, concurrency, wall hours
    local mission=$1 cycles=$2 jobs_limit=$3 concurrency=$4 wall=$5 mission_dir="$agent_repo/artifacts/agents/missions/$1"
    mkdir -p "$mission_dir" "$agent_repo/plans"
    cat >"$agent_repo/plans/mission-$mission.contract.md" <<EOF
\`\`\`mission
fence.wall-clock-hours=$wall
fence.cycles=$cycles
fence.jobs=$jobs_limit
fence.concurrency=$concurrency
fence.job-cap-min=120
\`\`\`
EOF
    python3 - "$mission_dir/lease.json" "$mission" "$mission_pid" "$mission_pgid" <<'PY'
import json,sys
from datetime import datetime,timezone
from pathlib import Path
now=datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
Path(sys.argv[1]).write_text(json.dumps({"missionId":sys.argv[2],"pid":int(sys.argv[3]),"pgid":int(sys.argv[4]),"instanceTag":"mission-lease-tag","startedAt":now,"renewedAt":now})+"\n")
PY
  }

  assert_fence_ask() { # mission, expected reason
    local mission=$1 reason=$2 ask="$agent_repo/artifacts/agents/missions/$1/asks/fence-bound.json"
    [[ -f "$ask" ]] || { echo "mission fence $reason refusal wrote no batched ask" >&2; exit 1; }
    grep -Fq "\`$reason\`" "$ask" || { echo "mission fence ask omitted $reason" >&2; exit 1; }
  }

  make_fence_mission mission-wall 10 10 2 1
  printf '{"schemaVersion":1,"missionId":"mission-wall","startedAt":"2000-01-01T00:00:00Z","cycles":0,"reservations":{}}\n' \
    >"$agent_repo/artifacts/agents/missions/mission-wall/fences.json"
  agent_fails fence-wall 'lifecycle fence' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-wall --mission mission-wall --wait
  assert_fence_ask mission-wall wall-clock-hours

  make_fence_mission mission-cycles 1 10 2 2
  printf '{"schemaVersion":1,"missionId":"mission-cycles","startedAt":"%s","cycles":1,"reservations":{}}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    >"$agent_repo/artifacts/agents/missions/mission-cycles/fences.json"
  agent_fails fence-cycles 'lifecycle fence' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-cycles --mission mission-cycles --wait
  assert_fence_ask mission-cycles cycles

  make_fence_mission mission-jobs 10 1 2 2
  printf '{"schemaVersion":1,"missionId":"mission-jobs","startedAt":"%s","cycles":0,"reservations":{"prior":{"reservedAt":"2000-01-01T00:00:00Z","capMin":1}}}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    >"$agent_repo/artifacts/agents/missions/mission-jobs/fences.json"
  agent_fails fence-jobs 'lifecycle fence' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-jobs --mission mission-jobs --wait
  assert_fence_ask mission-jobs jobs

  make_fence_mission mission-concurrency 10 10 1 2
  printf '{"schemaVersion":1,"missionId":"mission-concurrency","startedAt":"%s","cycles":0,"reservations":{"active":{"reservedAt":"2000-01-01T00:00:00Z","capMin":1}}}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    >"$agent_repo/artifacts/agents/missions/mission-concurrency/fences.json"
  agent_fails fence-concurrency 'lifecycle fence' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-concurrency --mission mission-concurrency --wait
  assert_fence_ask mission-concurrency concurrency

  make_fence_mission mission-timeout 10 10 2 2
  mission_timeout_result="$agent_fixture/mission-timeout.status"
  wait_for_agent_census_fresh mission-timeout-job
  (
    set +e
    cd "$agent_repo"
    scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id mission-timeout-job --mission mission-timeout --cap-min 1 --wait
    printf '%s\n' "$?" >"$mission_timeout_result"
  ) &
  mission_timeout_driver=$!
  wait_for_agent_status mission-timeout-job running
  python3 - "$agent_repo/artifacts/agents/jobs/mission-timeout-job.json" <<'PY'
import json,sys
from pathlib import Path
path=Path(sys.argv[1]); value=json.loads(path.read_text()); value["startedAt"]="2000-01-01T00:00:00Z"; path.write_text(json.dumps(value)+"\n")
PY
  run_agent_fixture mission-timeout-reap mission-timeout-job "$agent_dispatch" reap --job mission-timeout-job
  wait_for_agent_fixture_process mission-timeout-driver mission-timeout-job "$mission_timeout_driver"
  [[ "$(cat "$mission_timeout_result")" == 4 ]] || { echo "mission job timeout did not map to exit 4" >&2; exit 1; }
  assert_fence_ask mission-timeout job-cap-min

  # A provider-native unit with the same spelling from another provider stays
  # a separate typed tuple; no heterogeneous mission total exists.
  python3 - "$agent_repo/artifacts/agents/jobs/other-provider.json" <<'PY'
import json,sys
from pathlib import Path
Path(sys.argv[1]).write_text(json.dumps({"jobId":"other-provider","mission":"mission-alpha","runtime":"other","status":"completed","usage":{"availability":"native","inputTokens":3,"cachedInputTokens":None,"outputTokens":None,"reasoningTokens":None,"cost":None,"providerUnits":{"name":"fake-unit","value":5}}})+"\n")
PY
  "$agent_repo/scripts/agents/mission-fence.py" aggregate-usage --repo "$agent_repo" --mission mission-alpha
  python3 - "$agent_repo/artifacts/agents/missions/mission-alpha/usage.json" <<'PY'
import json,sys
value=json.load(open(sys.argv[1])); units={(item["provider"],item["unit"]):item["value"] for item in value["units"]}
assert units[("fake","provider.fake-unit")] == 2
assert units[("other","provider.fake-unit")] == 5
assert not any(item["unit"] == "provider.total" for item in value["units"])
PY
  agent_fails missing-mission-lease 'does not have a live' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id missing-mission --mission missing
  agent_fails ambiguous-mission 'ambiguous mission context' env HARNESS_MISSION_ID=mission-alpha HARNESS_MISSION_LEASE="$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" \
    "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-ambiguous --mission another
  [[ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mission"])' "$agent_repo/artifacts/agents/jobs/happy.json")" == None ]] \
    || { echo "unstamped interactive dispatch gained mission authority" >&2; exit 1; }
  kill "$mission_pid" 2>/dev/null || true
  wait_for_agent_fixture_process mission-lease-holder - "$mission_pid" 2>/dev/null || true
  export HARNESS_FAKE_PROCESS_IDENTITY_FILE="$agent_identity_fixture"

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

  "$agent_repo/scripts/agents/arm-supervision.sh" --repo "$agent_repo" --shutdown >/dev/null 2>&1
  agent_supervision_repo=

  # The minimal mission runner is exercised only through its fake host. The
  # repository, origin, supervision set, signed contracts, frozen gate, turn
  # records, state, and ledger are all real; only the model call is simulated.
  # Unlike the dispatch fixtures above, it owns an isolated repository armed
  # against real process sources so the census can observe each freshly
  # launched runner. This is scoped like the nested ordinary-operator fixture
  # in supervision-fixtures.sh.
  runner_process_env=(env -u HARNESS_CENSUS_PROCESS_FILE -u HARNESS_FAKE_PROCESS_IDENTITY_FILE)
  runner="$runner_repo/scripts/agents/mission-runner.sh"
  runner_origin="$agent_fixture/runner-origin.git"
  runner_mission_identity_fixture="$agent_fixture/runner-mission-process-identities.json"
  mv "$runner_repo/scripts/agents/arm-supervision.sh" \
    "$runner_repo/scripts/agents/arm-supervision-real.sh"
  cat >"$runner_repo/scripts/agents/arm-supervision.sh" <<'ARM'
#!/usr/bin/env bash
set -euo pipefail
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
fixture_root=$(git -C "$script_dir" rev-parse --show-toplevel)
called_at=$(date +%s)
"$script_dir/arm-supervision-real.sh" "$@"
[[ ${1:-} == fingerprint ]] && exit 0
for argument in "$@"; do [[ "$argument" == --shutdown ]] && exit 0; done
deadline=$((called_at + 30))
while (( $(date +%s) <= deadline )); do
  completed=$(python3 - "$fixture_root/artifacts/agents/supervision/last-census.json" <<'PY' 2>/dev/null || true
import json,sys
try: print(json.load(open(sys.argv[1]))["completedAtEpoch"])
except (OSError,ValueError,KeyError,TypeError): pass
PY
  )
  [[ ${completed:-0} -ge $called_at ]] && break
  sleep 0.05
done
if [[ -n "${HARNESS_MISSION_PROCESS_IDENTITY_FILE:-}" \
  && -f "$fixture_root/artifacts/agents/supervision/state.json" ]]; then
  python3 - "$fixture_root/artifacts/agents/supervision/state.json" \
    "$HARNESS_MISSION_PROCESS_IDENTITY_FILE" <<'PY'
import json,sys
from pathlib import Path
state=json.loads(Path(sys.argv[1]).read_text()); identities={}
for name in ("watcher","reaper"):
    value=state["components"][name]
    identities[str(value["pid"])]= {
        "pidStartedAt":value["pidStartedAt"],
        "command":f"fixture {value['instanceTag']}",
    }
Path(sys.argv[2]).write_text(json.dumps(identities)+"\n")
PY
fi
ARM
  chmod +x "$runner_repo/scripts/agents/arm-supervision.sh"
  runner_main_start=$("${runner_process_env[@]}" \
    "$runner_repo/scripts/agents/process-census.py" started-at --pid "$$")
  agent_supervision_repo=$runner_repo
  "${runner_process_env[@]}" HARNESS_AGENT_RUNTIME=fake \
    "$runner_repo/scripts/agents/arm-supervision.sh" \
    --repo "$runner_repo" --session runner-validator --pid "$$" \
    --start-time "$runner_main_start" --tag harness-main-fake-runner-validator \
    >"$agent_fixture/runner-arming.out" \
    || { echo "mission runner fixture could not arm real-source supervision" >&2; cat "$agent_fixture/runner-arming.out" >&2; exit 1; }
  mkdir -p "$runner_repo/scripts" "$runner_repo/truth"
  cat >"$runner_repo/scripts/gate.sh" <<'GATE'
#!/usr/bin/env bash
set -euo pipefail
score=$(cat candidate-score.txt)
printf 'metric=score=%s\n' "$score"
GATE
  chmod +x "$runner_repo/scripts/gate.sh"
  printf '0\n' >"$runner_repo/candidate-score.txt"
  printf 'runner truth\n' >"$runner_repo/truth/reference.txt"
  git -C "$runner_repo" config user.name harness
  git -C "$runner_repo" config user.email harness@example.invalid
  git -C "$runner_repo" add scripts/gate.sh candidate-score.txt truth/reference.txt
  git -C "$runner_repo" commit -qm 'add mission runner instruments'
  git -C "$runner_repo" tag runner-instruments
  git init -q --bare "$runner_origin"
  runner_branch=$(git -C "$runner_repo" branch --show-current)
  git -C "$runner_repo" remote add origin "$runner_origin"
  git -C "$runner_repo" push -qu -u origin "$runner_branch"
  git -C "$runner_origin" symbolic-ref HEAD "refs/heads/$runner_branch"
  git -C "$runner_repo" remote set-head origin -a >/dev/null

  make_runner_contract() { # mission, fake behavior, cycle fence, optional prompt-breaking heading
    local mission=$1 behavior=$2 cycles=$3 bad_heading=${4:-} contract="$runner_repo/plans/mission-$1.contract.md" contract_sha
    mkdir -p "$runner_repo/plans"
    cat >"$contract" <<EOF
# Intent

Advance the candidate through one unattended fake-host turn.
$bad_heading

# Non-goals

Do not publish or deploy.

# Initial streams

Keep the primary stream active until the frozen gate passes.

\`\`\`mission
gate.command=scripts/gate.sh
gate.ref=runner-instruments
gate.paths=scripts/gate.sh
truth.paths=truth/reference.txt
truth.certification=certified
gate.direction=max
gate.threshold.score=>=1
gate.noise-floor.score=0
guard.score.command=scripts/gate.sh
guard.score.floor=1
guard.score.noise=0
guard.cadence=1
ledger.cycle-budget=5
ledger.no-gain-budget=3
fence.wall-clock-hours=2
fence.cycles=$cycles
fence.jobs=4
fence.concurrency=1
fence.job-cap-min=5
host.runtime=fake
host.model=fake-model
host.turn-cap-min=1
stream.primary=FAKEHOST:$behavior advance the candidate.
envelope.dependencies=jq
exposure=EUR:1
\`\`\`
EOF
    contract_sha=$("$runner_repo/scripts/assert-mission.sh" --seal --file "$contract")
    printf '\nApproval: name=Fixture-Human; date=2026-08-04; contract-sha256=%s\n' "$contract_sha" >>"$contract"
    git -C "$runner_repo" add "plans/mission-$mission.contract.md"
    git -C "$runner_repo" commit -qm "sign mission $mission"
    git -C "$runner_repo" push -qu origin "$runner_branch"
  }

  wait_lease_released() { # mission, description
    local mission=$1 what=$2 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec ))
    # The runner writes the terminal mission status inside its cycle and
    # releases the lease as it exits, so release trails the status by design.
    while (( SECONDS < deadline )); do
      [[ -e "$runner_repo/artifacts/agents/missions/$mission/lease.d" ]] || return 0
      sleep 0.05
    done
    echo "$what retained its runner lease (elapsed: $((SECONDS - started))s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
    exit 1
  }

  run_runner_expect() { # name, expected exit, command...
    local name=$1 expected=$2 result
    shift 2
    set +e
    run_agent_fixture_captured "$name" - "$agent_fixture/$name.out" "$@"
    result=$?
    set -e
    if [[ $result -ne $expected ]]; then
      echo "mission runner fixture $name exited $result instead of $expected" >&2
      cat "$agent_fixture/$name.out" >&2
      exit 1
    fi
  }

  wait_runner_status() { # mission, expected exit
    local mission=$1 expected=$2 result=7 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec ))
    while (( SECONDS < deadline )); do
      set +e
      "$runner" status --mission "$mission" >"$agent_fixture/status-$mission.out" 2>&1
      result=$?
      set -e
      [[ $result -eq $expected ]] && return 0
      sleep 0.05
    done
    echo "mission runner status timed out: $mission -> $expected (last exit: $result)" >&2
    cat "$agent_fixture/status-$mission.out" >&2
    return 1
  }

  printf '{}\n' >"$runner_mission_identity_fixture"
  export HARNESS_MISSION_PROCESS_IDENTITY_FILE="$runner_mission_identity_fixture"

  make_runner_contract runner-cycle return-ok 5
  printf '1\n' >"$runner_repo/candidate-score.txt"
  git -C "$runner_repo" add candidate-score.txt
  git -C "$runner_repo" commit -qm 'improve mission runner candidate'
  git -C "$runner_repo" push -qu origin "$runner_branch"
  run_runner_expect runner-cycle-start 0 "${runner_process_env[@]}" HARNESS_AGENT_RUNTIME=fake "$runner" start --mission runner-cycle
  wait_runner_status runner-cycle 10
  cycle_turn=$(find "$runner_repo/artifacts/agents/missions/runner-cycle/turns" -mindepth 1 -maxdepth 1 -type d | head -1)
  "$runner_repo/scripts/assert-turn-prompt.sh" --file "$cycle_turn/prompt.md" --turn "$cycle_turn"
  python3 - "$runner_repo" "$cycle_turn/prompt.md" <<'PY'
import sys
from pathlib import Path
root,prompt_path=Path(sys.argv[1]),Path(sys.argv[2])
prompt=prompt_path.read_bytes(); header_end=prompt.index(b"\n\n")
header=prompt[:header_end].decode().splitlines()
assert [line.split(": ",1)[0] for line in header] == [
    "Mission-Id","Turn-Id","Cycle","Host-Session","Runtime","Model","Reconciliation"
]
preamble=(root/"scripts/agents/roles/orchestrator.md").read_bytes()
assert prompt[header_end+2:header_end+2+len(preamble)] == preamble
text=prompt.decode(); headings=[
    "## Mission Contract","## Ledger Tail","## Open Asks",
    "## Streams","## Reconciliation","## This Turn",
]
positions=[text.index(heading) for heading in headings]
assert positions == sorted(positions)
PY
  grep -Fq -- '- Classification: contract-improved;' \
    "$runner_repo/artifacts/agents/missions/runner-cycle/ledger.md" \
    || { echo "full mission cycle did not record runner-measured contract improvement" >&2; exit 1; }
  run_runner_expect prompt-missing-turn 1 "$runner_repo/scripts/agents/mission-prompt.py" \
    --mission runner-cycle --turn runner-cycle-t99-missing --output "$agent_fixture/missing-prompt.md"
  grep -Fq 'missing turn record' "$agent_fixture/prompt-missing-turn.out" \
    || { echo "prompt assembler did not name its missing turn record refusal" >&2; exit 1; }
  run_runner_expect prompt-oversized 1 env HARNESS_MISSION_MAX_PROMPT_KB=1 \
    "$runner_repo/scripts/agents/mission-prompt.py" --mission runner-cycle \
    --turn "$(basename "$cycle_turn")" --output "$agent_fixture/oversized-prompt.md"
  grep -Fq 'oversized block' "$agent_fixture/prompt-oversized.out" \
    || { echo "prompt assembler did not name the oversized block" >&2; exit 1; }

  make_runner_contract runner-bad-prompt return-ok 5 '## Streams'
  run_runner_expect runner-bad-prompt-start 3 "${runner_process_env[@]}" HARNESS_AGENT_RUNTIME=fake "$runner" start --mission runner-bad-prompt
  wait_runner_status runner-bad-prompt 11
  bad_turn=$(find "$runner_repo/artifacts/agents/missions/runner-bad-prompt/turns" -mindepth 1 -maxdepth 1 -type d | head -1)
  [[ ! -e "$bad_turn/raw.out" ]] || { echo "prompt-checker refusal launched the fake host" >&2; exit 1; }
  grep -Fq 'prompt-refused' "$bad_turn/turn.json" \
    || { echo "prompt-checker refusal was not recorded on the turn" >&2; exit 1; }

  make_runner_contract runner-ghost dispatch-ghost 5
  run_runner_expect runner-ghost-start 0 "${runner_process_env[@]}" HARNESS_AGENT_RUNTIME=fake "$runner" start --mission runner-ghost
  wait_runner_status runner-ghost 10
  python3 - "$runner_repo/artifacts/agents/missions/runner-ghost" <<'PY'
import json,sys
from pathlib import Path
mission=Path(sys.argv[1]); state=json.loads((mission/"state.json").read_text())
rejected=state["turnLog"][-1]["rejected"]
assert len(rejected)==1 and rejected[0]["kind"]=="dispatched"
assert "does not exist" in rejected[0]["reason"]
ask=json.loads((mission/"asks"/f"{rejected[0]['askId']}.json").read_text())
assert ask["reasonClass"]=="host-failure" and ask["answeredAt"] is None
PY

  make_runner_contract runner-fence return-ok 1
  mkdir -p "$runner_repo/artifacts/agents/missions/runner-fence"
  printf '{"schemaVersion":1,"missionId":"runner-fence","startedAt":"%s","cycles":1,"reservations":{}}\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    >"$runner_repo/artifacts/agents/missions/runner-fence/fences.json"
  run_runner_expect runner-fence-start 3 "${runner_process_env[@]}" HARNESS_AGENT_RUNTIME=fake "$runner" start --mission runner-fence
  wait_runner_status runner-fence 11
  python3 - "$runner_repo/artifacts/agents/missions/runner-fence" <<'PY'
import json,sys
from pathlib import Path
mission=Path(sys.argv[1]); state=json.loads((mission/"state.json").read_text())
assert state["status"]=="parked" and state["parkReason"]=="fence"
asks=[json.loads(path.read_text()) for path in (mission/"asks").glob("*.json")]
assert any(ask["reasonClass"]=="fence" and ask["answeredAt"] is None for ask in asks)
PY

  make_runner_contract runner-unverified return-ok 5
  run_runner_expect runner-unverified-start 3 "${runner_process_env[@]}" HARNESS_AGENT_RUNTIME=fake \
    HARNESS_FAKE_HOST_START_UNVERIFIED=1 "$runner" start --mission runner-unverified
  wait_runner_status runner-unverified 11
  unverified_ask=$(python3 - "$runner_repo/artifacts/agents/missions/runner-unverified" <<'PY'
import json,sys
from pathlib import Path
mission=Path(sys.argv[1]); state=json.loads((mission/"state.json").read_text())
assert state["parkReason"]=="host-failure"
turns=list((mission/"turns").glob("*/turn.json")); assert len(turns)==1
turn=json.loads(turns[0].read_text()); assert turn["error"]=="start-unverified"
asks=[json.loads(path.read_text()) for path in (mission/"asks").glob("*.json")]
ask=next(value for value in asks if value["reasonClass"]=="host-failure" and value["answeredAt"] is None)
print(ask["askId"])
PY
  )
  run_runner_expect runner-unverified-answer 0 "$runner" answer --mission runner-unverified \
    --ask "$unverified_ask" --answer acknowledged
  wait_runner_status runner-unverified 0

  "${runner_process_env[@]}" "$runner_repo/scripts/agents/arm-supervision.sh" \
    --repo "$runner_repo" --shutdown >/dev/null 2>&1
  agent_supervision_repo=
  [[ ! -e "$runner_repo/artifacts/agents/missions/runner-unverified/lease.d" ]] \
    || { echo "parked mission retained its runner lease" >&2; exit 1; }
  agent_supervision_repo=$runner_repo
  run_runner_expect runner-unverified-resume 0 "${runner_process_env[@]}" HARNESS_AGENT_RUNTIME=fake "$runner" resume --mission runner-unverified
  wait_runner_status runner-unverified 10
  [[ -f "$runner_repo/artifacts/agents/supervision/state.json" ]] \
    || { echo "resume did not re-arm supervision" >&2; exit 1; }
  wait_lease_released runner-unverified "completed resumed mission"
  resumed_prompt=$(find "$runner_repo/artifacts/agents/missions/runner-unverified/turns" -name prompt.md | sort | tail -1)
  grep -Fq 'Reconciliation: yes' "$resumed_prompt" \
    || { echo "resumed turn did not carry reconciliation" >&2; exit 1; }
  grep -Fq $'\tfailed\tstart-unverified' "$resumed_prompt" \
    || { echo "resumed turn omitted the failed prior turn from reconciliation" >&2; exit 1; }

  "${runner_process_env[@]}" "$runner_repo/scripts/agents/arm-supervision.sh" \
    --repo "$runner_repo" --shutdown >/dev/null 2>&1
  agent_supervision_repo=
  agent_selftest_process_fixture="$agent_fixture/selftest-processes.json"
  agent_selftest_identity_fixture="$agent_fixture/selftest-process-identities.json"
  printf '[]\n' >"$agent_selftest_process_fixture"
  printf '{}\n' >"$agent_selftest_identity_fixture"
  export HARNESS_CENSUS_PROCESS_FILE="$agent_selftest_process_fixture"
  export HARNESS_FAKE_PROCESS_IDENTITY_FILE="$agent_selftest_identity_fixture"
  agent_supervision_repo=$agent_selftest_repo
  agent_selftest_main_start=$("$agent_selftest_repo/scripts/agents/process-census.py" started-at --pid "$$")
  HARNESS_AGENT_RUNTIME=fake "$agent_selftest_repo/scripts/agents/arm-supervision.sh" \
    --repo "$agent_selftest_repo" --session selftest-validator --pid "$$" \
    --start-time "$agent_selftest_main_start" --tag harness-main-fake-selftest-validator \
    >"$agent_fixture/selftest-arming.out" \
    || { echo "adapter selftest fixture could not arm supervision" >&2; exit 1; }
  fake_selftest_adapter="$agent_selftest_repo/scripts/agents/adapters/fake.sh"
  run_agent_fixture_captured fake-selftest - "$agent_fixture/fake-selftest.out" "$fake_selftest_adapter" selftest
  grep -Fq 'full protocol sequence' "$agent_fixture/fake-selftest.out" \
    || { echo "fake adapter selftest did not run its full protocol sequence" >&2; exit 1; }
  python3 - "$agent_selftest_repo/artifacts/agents/selftests" <<'PY'
import json, sys
from pathlib import Path
paths = list(Path(sys.argv[1]).glob("fake-selftest-*.json")); assert paths
value = json.loads(max(paths, key=lambda path: path.stat().st_mtime).read_text())
assert "resume-identity" in value["provenBehaviorally"] and "network" in value["constructedOnly"]
PY

  "$agent_selftest_repo/scripts/agents/arm-supervision.sh" --repo "$agent_selftest_repo" --shutdown >/dev/null 2>&1
  agent_supervision_repo=
  unset HARNESS_CENSUS_PROCESS_FILE HARNESS_FAKE_PROCESS_IDENTITY_FILE \
    HARNESS_MISSION_PROCESS_IDENTITY_FILE
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
  cp scripts/receipt.sh scripts/harness-config.sh "$hookrepo/scripts/"
  cp harness.conf "$hookrepo/"
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

knob_fixture="$tmp/conf-consuming-scripts"
mkdir -p "$knob_fixture/receipt/scripts" "$knob_fixture/watch/scripts" "$knob_fixture/watch/jobs"
cp scripts/receipt.sh scripts/harness-config.sh "$knob_fixture/receipt/scripts/"
printf 'retro.max-receipts=0\nretro.max-age-days=30\n' >"$knob_fixture/receipt/harness.conf"
"$knob_fixture/receipt/scripts/receipt.sh" add --type implement --outcome shipped --file "$knob_fixture/receipt/receipts.log" >/dev/null
if "$knob_fixture/receipt/scripts/receipt.sh" check --file "$knob_fixture/receipt/receipts.log" >/dev/null 2>&1; then
  echo "receipt ignored the harness.conf receipt limit" >&2
  exit 1
fi
HARNESS_RETRO_MAX_RECEIPTS=2 "$knob_fixture/receipt/scripts/receipt.sh" check --file "$knob_fixture/receipt/receipts.log" >/dev/null \
  || { echo "receipt did not prefer the environment over harness.conf" >&2; exit 1; }
HARNESS_RETRO_MAX_RECEIPTS=0 "$knob_fixture/receipt/scripts/receipt.sh" check --max-receipts 2 --file "$knob_fixture/receipt/receipts.log" >/dev/null \
  || { echo "receipt did not prefer the flag over the environment" >&2; exit 1; }

cp scripts/watch-background-jobs.sh scripts/harness-config.sh "$knob_fixture/watch/scripts/"
printf 'watch.stale-min=7\nwatch.cap-min=9\n' >"$knob_fixture/watch/harness.conf"
touch "$knob_fixture/watch/state"
"$knob_fixture/watch/scripts/watch-background-jobs.sh" --dir "$knob_fixture/watch/jobs" --state "$knob_fixture/watch/state" --once >"$knob_fixture/watch.out"
grep -q 'stale=7m cap=9m' "$knob_fixture/watch.out" \
  || { echo "watcher ignored harness.conf ceilings" >&2; exit 1; }

refactor_knob="$knob_fixture/refactor"
mkdir -p "$refactor_knob/scripts"
cp scripts/refactor-baseline.sh scripts/harness-config.sh "$refactor_knob/scripts/"
printf 'refactor.max-age-minutes=1440\nrefactor.max-commits=0\n' >"$refactor_knob/harness.conf"
git init -q "$refactor_knob"
printf 'fixture\n' >"$refactor_knob/source.txt"
git -C "$refactor_knob" add source.txt harness.conf scripts
git -C "$refactor_knob" -c user.name=harness -c user.email=harness@example.invalid commit -qm initial
(cd "$refactor_knob" && scripts/refactor-baseline.sh record --gate fixture >/dev/null)
git -C "$refactor_knob" add plans/refactor-baseline
git -C "$refactor_knob" -c user.name=harness -c user.email=harness@example.invalid commit -qm baseline
if (cd "$refactor_knob" && scripts/refactor-baseline.sh check >/dev/null 2>&1); then
  echo "refactor baseline ignored harness.conf commit cadence" >&2
  exit 1
fi
(cd "$refactor_knob" && scripts/refactor-baseline.sh check --max-commits 2 >/dev/null) \
  || { echo "refactor baseline did not prefer the cadence flag" >&2; exit 1; }

rfile="$tmp/receipts.log"
scripts/receipt.sh add --type implement --outcome shipped \
  --delegate codex:fixture-code:implementer-job \
  --delegate claude:fixture-review:code-critic-job --file "$rfile" >/dev/null
grep -q '|delegate=codex:fixture-code:implementer-job,claude:fixture-review:code-critic-job|' "$rfile" \
  || { echo "receipt did not join repeated delegate triples" >&2; exit 1; }
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
# note, the skills list, delegates, and the retro summary must each stay one log line.
crlf_fixture=$(printf 'a\r\nb')
rfile_crlf="$tmp/receipts-crlf.log"
scripts/receipt.sh add --type implement --outcome shipped --note "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 1 ]] || { echo "a CRLF note corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh add --type implement --outcome shipped --skills "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 2 ]] || { echo "a CRLF skills list corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh add --type implement --outcome shipped --delegate "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 3 ]] || { echo "a CRLF delegate corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh retro "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 4 ]] || { echo "a CRLF retro summary corrupted the receipt log" >&2; exit 1; }
if LC_ALL=C grep -q $'\r' "$rfile_crlf"; then
  echo "receipt sanitizer left a carriage return in the log" >&2
  exit 1
fi

# Adopted-mode contract: a copy without the template marker validates with a
# skill pruned, and a present-but-broken skill still fails. Template mode
# only, so the nested run (which lacks meta/) cannot recurse.
if (( template_mode )); then
  adopted="$tmp/adopted"
  mkdir -p "$adopted"
copy_tree_without_artifacts() { # source root, destination
  # Only for copies whose source is the live harness root: artifacts/ is
  # runtime state, not shipped content, and copying it races
  # with any job writing lock directories, and an adoption fixture has no use
  # for it. Excluding it makes the suite safe to run while work is in flight.
  local from=$1 to=$2 entry
  mkdir -p "$to"
  (cd "$from" && for entry in * .[!.]*; do
    [[ -e "$entry" ]] || continue
    [[ "$entry" == artifacts ]] && continue
    cp -R "$entry" "$to/"
  done)
}

  copy_tree_without_artifacts "$root" "$adopted"
  rm -rf "$adopted/meta" "$adopted/skills/improve" "$adopted/plans/receipts.log" "$adopted/.claude"
  sed 's/<[^>]*>/filled/g' "$adopted/docs/project-rules.md" >"$adopted/docs/project-rules.md.new"
  mv "$adopted/docs/project-rules.md.new" "$adopted/docs/project-rules.md"
  perl -0pi -e 's/^harness\.runtimes=.*$/harness.runtimes=/m; s/^role\..*\n//mg; s/^mode\..*\.role\..*\n//mg' "$adopted/harness.conf"
  fill_harness_conf "$adopted/harness.conf" "$tmp/adopted-evidence"
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
  copy_tree_without_artifacts "$root" "$srcrepo"
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
  [[ -L "$tgt/.claude/skills/code-critique" && -f "$tgt/.claude/agents/code-critique.md" ]] \
    || { echo "adopt: code-critique was not registered for claude" >&2; exit 1; }
  [[ -f "$tgt/scripts/agents/dispatch.sh" && -f "$tgt/harness.conf" ]] \
    || { echo "adopt: orchestration payload missing" >&2; exit 1; }
  grep -qxF 'harness.runtimes=claude' "$tgt/harness.conf" \
    || { echo "adopt: default runtime selection was not recorded" >&2; exit 1; }
  grep -qxF 'role.default.runtime=claude' "$tgt/harness.conf" \
    || { echo "adopt: selected claude was not made the roster default" >&2; exit 1; }
  if grep -Eq '(^|\.)model\.(codex|devin)=|\.runtime=(codex|devin)$' "$tgt/harness.conf"; then
    echo "adopt: unselected runtime-valued keys survived the default selection" >&2
    exit 1
  fi
  grep -q systemMessage "$tgt/.claude/settings.json" || { echo "adopt: settings.json lacks the shipped hook" >&2; exit 1; }
  grep -q 'SessionStart' "$tgt/.claude/settings.json" \
    && grep -q 'supervision-hook.sh.*claude start' "$tgt/.claude/settings.json" \
    || { echo "adopt: Claude session-start supervision hook missing" >&2; exit 1; }
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
  if bash "$tgt/scripts/validate-harness.sh" >"$tmp/conf-placeholder.out" 2>&1; then
    echo "adopt: target validated while harness.conf placeholders remained" >&2
    exit 1
  fi
  grep -q 'harness.conf' "$tmp/conf-placeholder.out" \
    || { echo "adopt: placeholder failure did not name harness.conf" >&2; exit 1; }
  fill_harness_conf "$tgt/harness.conf" "$tmp/adopt-default-evidence"
  bash "$tgt/scripts/validate-harness.sh" >/dev/null 2>&1 || { echo "adopt: filled target failed validation" >&2; exit 1; }

  echo drift >>"$tgt/.claude/agents/verify.md"
  if bash "$tgt/scripts/validate-harness.sh" >"$tmp/profile-drift.out" 2>&1; then
    echo "adopt: validation missed a drifted claude profile" >&2
    exit 1
  fi
  grep -q 'profile drifted' "$tmp/profile-drift.out" \
    || { echo "adopt: profile-drift failure did not name the profile" >&2; exit 1; }
  cp "$tgt/skills/verify/agents/claude-profile.md" "$tgt/.claude/agents/verify.md"

  mv "$tgt/.claude/skills" "$tgt/.claude/skills.missing"
  if "$tgt/scripts/harness-config.sh" validate >"$tmp/missing-registration.out" 2>&1; then
    echo "adopt: configuration validation missed a selected runtime registration" >&2
    exit 1
  fi
  grep -q 'registration directory .claude/skills is missing' "$tmp/missing-registration.out" \
    || { echo "adopt: missing-registration failure did not name the directory" >&2; exit 1; }
  mv "$tgt/.claude/skills.missing" "$tgt/.claude/skills"

  bash "$adopt" "$tmp/adopt-devin" --runtimes devin >/dev/null
  [[ -f "$tmp/adopt-devin/.devin/agents/verify/AGENT.md" ]] || { echo "adopt: devin profile missing" >&2; exit 1; }
  [[ -L "$tmp/adopt-devin/.agents/skills/verify" && -L "$tmp/adopt-devin/.devin/skills/verify" ]] \
    || { echo "adopt: devin skill registrations missing" >&2; exit 1; }
  [[ -L "$tmp/adopt-devin/.devin/skills/code-critique" && -f "$tmp/adopt-devin/.devin/agents/code-critique/AGENT.md" ]] \
    || { echo "adopt: code-critique was not registered for devin" >&2; exit 1; }
  grep -qxF 'harness.runtimes=devin' "$tmp/adopt-devin/harness.conf" \
    || { echo "adopt: devin selection was not recorded" >&2; exit 1; }
  grep -qxF 'role.default.runtime=devin' "$tmp/adopt-devin/harness.conf" \
    || { echo "adopt: devin was not selected as the roster default" >&2; exit 1; }
  [[ -f "$tmp/adopt-devin/.devin/config.json" ]] \
    && grep -q 'supervision-hook.sh.*devin start' "$tmp/adopt-devin/.devin/config.json" \
    || { echo "adopt: Devin-compatible session-start supervision hook missing" >&2; exit 1; }
  [[ ! -e "$tmp/adopt-devin/.claude" ]] || { echo "adopt: devin-only target got .claude state" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-codex" --runtimes codex >/dev/null
  [[ -L "$tmp/adopt-codex/.agents/skills/verify" ]] || { echo "adopt: codex skill registration missing" >&2; exit 1; }
  [[ -L "$tmp/adopt-codex/.agents/skills/code-critique" ]] || { echo "adopt: code-critique was not registered for codex" >&2; exit 1; }
  grep -qxF 'harness.runtimes=codex' "$tmp/adopt-codex/harness.conf" \
    || { echo "adopt: codex selection was not recorded" >&2; exit 1; }
  grep -qxF 'role.default.runtime=codex' "$tmp/adopt-codex/harness.conf" \
    || { echo "adopt: codex was not selected as the roster default" >&2; exit 1; }
  [[ -f "$tmp/adopt-codex/.codex/hooks.json" ]] \
    && grep -q 'supervision-hook.sh.*codex start' "$tmp/adopt-codex/.codex/hooks.json" \
    || { echo "adopt: Codex session-start supervision hook missing" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-none" --runtimes none >/dev/null
  [[ ! -e "$tmp/adopt-none/.claude" && ! -e "$tmp/adopt-none/.devin" && ! -e "$tmp/adopt-none/.agents" ]] \
    || { echo "adopt: --runtimes none still registered a runtime" >&2; exit 1; }
  [[ -f "$tmp/adopt-none/.github/workflows/harness.yml" ]] || { echo "adopt: CI workflow skipped for --runtimes none" >&2; exit 1; }
  grep -qxF 'harness.runtimes=' "$tmp/adopt-none/harness.conf" \
    || { echo "adopt: --runtimes none did not record an empty runtime selection" >&2; exit 1; }
  if grep -Eq '^(role\.|mode\..*\.role\.)' "$tmp/adopt-none/harness.conf"; then
    echo "adopt: --runtimes none retained roster lines" >&2
    exit 1
  fi

  tier_src="$tmp/adopt-tier-src"
  cp -R "$srcrepo/." "$tier_src"
  perl -0pi -e 's/^model\.tier\.1=.*$/model.tier.1=claude:claude-model,codex:codex-model,devin:devin-model/m; s/^model\.tier\.2=.*$/model.tier.2=/m; s/^model\.tier\.3=.*$/model.tier.3=/m; s/^(.*\.model\.claude)=.*$/$1=claude-model/mg; s/^(.*\.model\.codex)=.*$/$1=codex-model/mg; s/^(.*\.model\.devin)=.*$/$1=devin-model/mg' "$tier_src/harness.conf"
  git -C "$tier_src" add harness.conf
  git -C "$tier_src" -c user.name=harness -c user.email=harness@example.invalid commit -qm tier-fixture
  bash "$tier_src/scripts/adopt.sh" "$tmp/adopt-tier-claude" --runtimes claude >/dev/null
  grep -qxF 'model.tier.1=claude:claude-model' "$tmp/adopt-tier-claude/harness.conf" \
    || { echo "adopt: model tier retained an unselected runtime" >&2; exit 1; }
  if grep -Eq '(^|\.)model\.(codex|devin)=|\.runtime=(codex|devin)$' "$tmp/adopt-tier-claude/harness.conf"; then
    echo "adopt: concrete unselected model or runtime keys survived pruning" >&2
    exit 1
  fi
  bash "$adopt" "$tmp/adopt-java" --enable debug-java >/dev/null
  [[ -f "$tmp/adopt-java/skills/debug-java/SKILL.md" ]] || { echo "adopt: --enable did not move the optional skill" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-copy" --runtimes claude,codex --copy-skills >/dev/null
  [[ -d "$tmp/adopt-copy/.claude/skills/verify" && ! -L "$tmp/adopt-copy/.claude/skills/verify" ]] \
    || { echo "adopt: --copy-skills did not copy" >&2; exit 1; }
  [[ -d "$tmp/adopt-copy/.agents/skills/verify" && ! -L "$tmp/adopt-copy/.agents/skills/verify" ]] \
    || { echo "adopt: --copy-skills did not copy the codex registration" >&2; exit 1; }
  grep -qxF 'harness.runtimes=claude,codex' "$tmp/adopt-copy/harness.conf" \
    || { echo "adopt: multi-runtime selection was not recorded" >&2; exit 1; }
  grep -qxF 'role.default.runtime=codex' "$tmp/adopt-copy/harness.conf" \
    || { echo "adopt: runtime default did not follow codex, devin, claude precedence" >&2; exit 1; }
  sed 's/<[^>]*>/filled/g' "$tmp/adopt-copy/docs/project-rules.md" >"$tmp/adopt-copy/docs/project-rules.md.new"
  mv "$tmp/adopt-copy/docs/project-rules.md.new" "$tmp/adopt-copy/docs/project-rules.md"
  fill_harness_conf "$tmp/adopt-copy/harness.conf" "$tmp/adopt-copy-evidence"
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
  if bash "$adopt" "$tmp/adopt-duplicate" --runtimes codex,codex >/dev/null 2>&1; then
    echo "adopt: accepted a duplicate runtime selection" >&2
    exit 1
  fi
  [[ ! -e "$tmp/adopt-duplicate/wow.md" ]] || {
    echo "adopt: a rejected duplicate runtime still mutated the target" >&2
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
