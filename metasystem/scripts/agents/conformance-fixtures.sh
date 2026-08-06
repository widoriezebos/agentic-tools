#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-conformance-fixtures.XXXXXX")
trap 'rm -rf "$fixture_root"' EXIT

controller=
worktree=
base_sha=

new_case() { # name
  local name=$1 case_root="$fixture_root/$1"
  controller="$case_root/controller"
  worktree="$case_root/worktree"
  mkdir -p "$controller/scripts/agents" "$controller/docs"
  cp "$source_root/scripts/agents/assert-conformance.sh" "$controller/scripts/agents/"
  cp "$source_root/scripts/metasystem-config.sh" "$controller/scripts/"
  printf 'artifacts/\n' >"$controller/.gitignore"
  printf 'base\n' >"$controller/source.txt"
  printf 'base\n' >"$controller/docs/note.md"
  printf '#!/usr/bin/env bash\nprintf "base\\n"\n' >"$controller/scripts/tool.sh"
  cat >"$controller/metasystem.conf" <<'CONF'
metasystem.version=1
metasystem.runtimes=fake
role.default.runtime=fake
role.default.model.fake=fixture-default
evidence.root=/tmp/metasystem-conformance-fixture-evidence
CONF
  git -C "$controller" init -q
  git -C "$controller" add .
  git -C "$controller" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm base
  git -C "$controller" worktree add -q -b "fixture-$name" "$worktree" HEAD
  base_sha=$(git -C "$worktree" rev-parse HEAD)
}

configure_critic() {
  cat >>"$controller/metasystem.conf" <<'CONF'
role.code-critic.runtime=fake
role.code-critic.model.fake=critic-model
CONF
}

write_implementer() { # waiver class or empty, diff paths...
  local waiver=$1
  shift
  mkdir -p "$controller/artifacts/agents/jobs" \
    "$controller/artifacts/agents/impl/rounds/1"
  python3 - "$controller" "$worktree" "$base_sha" "$waiver" "$@" <<'PY'
import json, subprocess, sys
from pathlib import Path

root, worktree, base, waiver, *paths = sys.argv[1:]
root, worktree = Path(root), Path(worktree)
record = {
    "jobId": "impl",
    "role": "implementer",
    "round": 1,
    "parentJob": None,
    "workspaceRoot": str(worktree),
    "baseSha": base,
    "branch": subprocess.check_output(["git", "-C", str(worktree), "branch", "--show-current"], text=True).strip(),
    "status": "completed",
    "effectiveModel": "shared-model",
}
if waiver:
    record["critiqueWaived"] = {"class": waiver}
(root / "artifacts/agents/jobs/impl.json").write_text(json.dumps(record, indent=2) + "\n")
result = {
    "jobId": "impl",
    "round": 1,
    "runtime": "fake",
    "sessionId": "implementer-session",
    "model": {"requested": "shared-model", "effective": "shared-model"},
    "evidence": [],
    "gaps": [],
    "mode": "implement",
    "riskiestPart": "fixture boundary",
    "diffBoundary": paths,
    "whatWasDone": "fixture implementation",
}
(root / "artifacts/agents/impl/rounds/1/return.json").write_text(json.dumps(result, indent=2) + "\n")
(root / "artifacts/agents/impl/brief.md").write_text(
    "Working Mode: implement\nMission Stream: fixture-stream\n\n"
    "Successor obligations: F-9.\n",
    encoding="utf-8",
)
(root / "artifacts/agents/impl/rounds/1/prompt.md").write_text(
    "Fixture prompt enumerates F-9.\n", encoding="utf-8"
)
PY
}

write_critic() { # reviewed tree, material id or empty, exhaustion mode, effective model
  local tree=$1 material_id=$2 exhaustion=$3 model=$4
  mkdir -p "$controller/artifacts/agents/critic/rounds/1"
  python3 - "$controller" "$tree" "$material_id" "$exhaustion" "$model" <<'PY'
import json, sys
from pathlib import Path

root, tree, material_id, exhaustion, model = Path(sys.argv[1]), *sys.argv[2:]
items = []
if exhaustion == "one":
    items = [{"round": 1, "openFindingIds": ["F-9"], "successorJobId": "impl"}]
elif exhaustion == "missing-successor-finding":
    items = [{"round": 1, "openFindingIds": ["F-10"], "successorJobId": "impl"}]
elif exhaustion == "two":
    items = [
        {"round": 1, "openFindingIds": ["F-9"], "successorJobId": "impl"},
        {"round": 4, "openFindingIds": ["F-11"], "successorJobId": "impl"},
    ]
record = {
    "jobId": "critic",
    "role": "code-critic",
    "round": 1,
    "parentJob": None,
    "reviews": "impl",
    "status": "completed",
    "effectiveModel": model,
    "chainClosed": True,
    "critiqueExhaustions": items,
}
(root / "artifacts/agents/jobs/critic.json").write_text(json.dumps(record, indent=2) + "\n")
findings = []
if material_id:
    findings = [{
        "id": material_id,
        "severity": "high",
        "material": True,
        "claim": "fixture material finding remains open",
        "evidence": "fixture evidence",
    }]
result = {
    "jobId": "critic",
    "round": 1,
    "runtime": "fake",
    "sessionId": "critic-session",
    "model": {"requested": model, "effective": model},
    "evidence": [],
    "gaps": [],
    "mode": "implement",
    "reviewedTree": tree,
    "findings": findings,
    "verdictMaterialCount": len(findings),
}
(root / "artifacts/agents/critic/rounds/1/return.json").write_text(json.dumps(result, indent=2) + "\n")
PY
}

commit_worktree() {
  git -C "$worktree" add .
  git -C "$worktree" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm fixture-change
}

expect_failure() { # name, expected text, command...
  local name=$1 expected=$2 output="$fixture_root/$1.out" status
  shift 2
  set +e
  "$@" >"$output" 2>&1
  status=$?
  set -e
  [[ $status -eq 1 ]] || {
    echo "conformance fixture $name exited $status instead of refusing with exit 1" >&2
    cat "$output" >&2
    exit 1
  }
  grep -Fq "$expected" "$output" || {
    echo "conformance fixture $name did not report: $expected" >&2
    cat "$output" >&2
    exit 1
  }
}

# One case proves review-stage creation, the missing-chain diagnostic, stale
# and material refusals, exhaustion, a successful independent chain, and the
# explicit weaker-independence declaration.
new_case core
printf 'changed\n' >>"$worktree/source.txt"
write_implementer '' source.txt
"$controller/scripts/agents/assert-conformance.sh" --stage review --job impl \
  >"$fixture_root/review-stage.out"
grep -Fq 'reviewedTree=' "$fixture_root/review-stage.out" \
  || { echo "review stage did not emit the reviewed tree" >&2; exit 1; }
[[ -s "$controller/artifacts/agents/impl/rounds/1/diff.patch" \
   && -s "$controller/artifacts/agents/impl/rounds/1/review.json" ]] \
  || { echo "review stage did not persist both review artifacts" >&2; exit 1; }
reviewed_tree=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["reviewedTree"])' \
  "$controller/artifacts/agents/impl/rounds/1/review.json")

expect_failure missing-chain 'reviews field names implementer job' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl
grep -Fq 'role.code-critic.runtime' "$fixture_root/missing-chain.out" \
  || { echo "missing-chain refusal did not name the exact configuration key" >&2; exit 1; }

configure_critic
commit_worktree
write_critic "0000000000000000000000000000000000000000" '' none critic-model
expect_failure stale-tree 'is stale' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl

write_critic "$reviewed_tree" F-7 none critic-model
cat >"$controller/artifacts/agents/critic/rounds/1/dispositions.md" <<'DISPOSITIONS'
| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-7 | accepted | fixture disposition | fixture amendment |
DISPOSITIONS
expect_failure dispositioned-material 'still has material findings despite any dispositions: F-7' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl

write_critic "$reviewed_tree" F-9 one critic-model
expect_failure exhausted-open 'open material findings: F-9' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl
write_critic "$reviewed_tree" F-10 missing-successor-finding critic-model
expect_failure exhausted-successor 'brief does not enumerate open findings: F-10' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl
write_critic "$reviewed_tree" F-9 two critic-model
expect_failure second-exhaustion 'waiting on the human is the only remedy' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl

write_critic "$reviewed_tree" '' none critic-model
"$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl >/dev/null
write_critic "$reviewed_tree" '' none shared-model
expect_failure same-model 'implementer job '\''impl'\'' uses effective model '\''shared-model'\''' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl
grep -Fq 'code-critic chain '\''critic'\'' uses effective model '\''shared-model'\''' "$fixture_root/same-model.out" \
  || { echo "same-model refusal did not name both jobs and models" >&2; exit 1; }
grep -Fq 'dispatch a critic on a different model' "$fixture_root/same-model.out" \
  || { echo "same-model refusal omitted the different-model remedy" >&2; exit 1; }
grep -Fq 'declare independence=session-only' "$fixture_root/same-model.out" \
  || { echo "same-model refusal omitted the session-only remedy" >&2; exit 1; }
printf 'independence=session-only\n' >>"$controller/metasystem.conf"
"$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl >/dev/null

# The only waiver class rejects behavior-bearing paths, accepts and counts a
# small non-instruction Markdown diff, and rejects the same prose above 30
# changed lines.
new_case waiver-script
printf 'printf "changed\\n"\n' >>"$worktree/scripts/tool.sh"
write_implementer prose-under-30 scripts/tool.sh
commit_worktree
expect_failure waiver-script 'instruction-bearing paths that are never waivable' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl

new_case waiver-small
for number in 1 2 3 4 5 6 7 8 9 10; do printf 'line %s\n' "$number" >>"$worktree/docs/note.md"; done
write_implementer prose-under-30 docs/note.md
commit_worktree
"$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl \
  >"$fixture_root/waiver-small.out"
grep -Fq 'count=1' "$fixture_root/waiver-small.out" \
  || { echo "genuine prose waiver was not counted" >&2; exit 1; }

new_case waiver-large
for number in $(seq 1 40); do printf 'line %s\n' "$number" >>"$worktree/docs/note.md"; done
write_implementer prose-under-30 docs/note.md
commit_worktree
expect_failure waiver-large 'the maximum is 30 additions plus deletions' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl

echo "conformance fixtures passed"
