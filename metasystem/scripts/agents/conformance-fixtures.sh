#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-conformance-fixtures.XXXXXX")
trap 'rm -rf "$fixture_root"' EXIT

controller=
worktree=
base_sha=

# Record writes stage beside their target and rename, so a reader never
# observes a torn file.
write_staged() { # target file; content on stdin
  local target=$1 staged
  staged=$(mktemp "$(dirname "$target")/.staged.XXXXXX")
  cat >"$staged"
  mv "$staged" "$target"
}

# A JSON array of the given strings. Fixture paths never carry the quote
# or backslash characters JSON would need escaped.
json_string_array() {
  local out='[' item separator=''
  for item in "$@"; do out+="$separator\"$item\""; separator=','; done
  printf '%s]' "$out"
}

new_case() { # name
  local name=$1 case_root="$fixture_root/$1"
  controller="$case_root/controller"
  worktree="$case_root/worktree"
  mkdir -p "$controller/scripts/agents" "$controller/docs"
  cp "$source_root/scripts/agents/assert-conformance.sh" "$controller/scripts/agents/"
  cp "$source_root/scripts/agents/instruction-bearing-paths.txt" "$controller/scripts/agents/"
  cp "$source_root/scripts/metasystem-config.sh" "$controller/scripts/"
  # The copied config reader resolves its engine as <controller>/bin/metasystem.
  mkdir -p "$controller/bin"
  cp "$source_root/bin/metasystem" "$controller/bin/metasystem"
  printf 'artifacts/\nlocal.conf\n' >"$controller/.gitignore"
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
  git -C "$controller" init -q -b main
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
    "$controller/artifacts/agents/impl/rounds/1" \
    "$controller/artifacts/agents/impl/rounds/2"
  local branch boundary waiver_member=''
  branch=$(git -C "$worktree" branch --show-current)
  boundary=$(json_string_array "$@")
  [[ -z "$waiver" ]] || waiver_member=",\"critiqueWaived\":{\"class\":\"$waiver\"}"
  printf '{"jobId":"impl","role":"implementer","round":1,"parentJob":null,"workspaceRoot":"%s","baseSha":"%s","branch":"%s","status":"completed","effectiveModel":"shared-model"%s}\n' \
    "$worktree" "$base_sha" "$branch" "$waiver_member" \
    | write_staged "$controller/artifacts/agents/jobs/impl.json"
  printf '{"jobId":"impl","round":1,"runtime":"fake","sessionId":"implementer-session","model":{"requested":"shared-model","effective":"shared-model"},"evidence":[],"gaps":[],"mode":"implement","riskiestPart":"fixture boundary","diffBoundary":%s,"whatWasDone":"fixture implementation"}\n' \
    "$boundary" | write_staged "$controller/artifacts/agents/impl/rounds/1/return.json"
  printf 'Working Mode: implement\nMission Stream: fixture-stream\n\nSuccessor obligations: F-9.\n' \
    >"$controller/artifacts/agents/impl/brief.md"
  printf 'Fixture prompt enumerates F-9.\n' >"$controller/artifacts/agents/impl/rounds/1/prompt.md"
  # The follow-up child shares the whole implementation record shape.
  printf '{"jobId":"impl-r2","role":"implementer","round":2,"parentJob":"impl","workspaceRoot":"%s","baseSha":"%s","branch":"%s","status":"completed","effectiveModel":"shared-model"%s}\n' \
    "$worktree" "$base_sha" "$branch" "$waiver_member" \
    | write_staged "$controller/artifacts/agents/jobs/impl-r2.json"
  printf 'Implementer follow-up enumerates F-9.\n' >"$controller/artifacts/agents/impl/rounds/2/prompt.md"
}

write_followup_return() { # round, diff paths...
  local fixture_round=$1
  shift
  [[ "$fixture_round" =~ ^[0-9]+$ ]] \
    || { echo "write_followup_return: round is not a number: $fixture_round" >&2; exit 1; }
  local round_dir="$controller/artifacts/agents/impl/rounds/$fixture_round"
  mkdir -p "$round_dir"
  printf '{"jobId":"impl-r%s","round":%s,"runtime":"fake","sessionId":"implementer-session","model":{"requested":"shared-model","effective":"shared-model"},"evidence":[],"gaps":[],"mode":"implement","riskiestPart":"fixture follow-up boundary","diffBoundary":%s,"whatWasDone":"fixture follow-up implementation"}\n' \
    "$fixture_round" "$fixture_round" "$(json_string_array "$@")" \
    | write_staged "$round_dir/return.json"
}

write_critic() { # reviewed tree, material id or empty, exhaustion mode, effective model
  local tree=$1 material_id=$2 exhaustion=$3 model=$4
  mkdir -p "$controller/artifacts/agents/critic/rounds/1"
  local items findings material_count
  case $exhaustion in
    one) items='[{"round":1,"openFindingIds":["F-9"],"successorJobId":"impl-r2"}]' ;;
    missing-successor-finding) items='[{"round":1,"openFindingIds":["F-10"],"successorJobId":"impl-r2"}]' ;;
    critic-successor) items='[{"round":1,"openFindingIds":["F-9"],"successorJobId":"other-critic"}]' ;;
    two) items='[{"round":1,"openFindingIds":["F-9"],"successorJobId":"impl-r2"},{"round":4,"openFindingIds":["F-11"],"successorJobId":"impl-r2"}]' ;;
    *) items='[]' ;;
  esac
  printf '{"jobId":"critic","role":"code-critic","round":1,"parentJob":null,"reviews":"impl","status":"completed","effectiveModel":"%s","chainClosed":true,"critiqueExhaustions":%s}\n' \
    "$model" "$items" | write_staged "$controller/artifacts/agents/jobs/critic.json"
  if [[ "$exhaustion" == critic-successor ]]; then
    printf '{"jobId":"other-critic","role":"code-critic","round":1,"parentJob":null,"reviews":"unrelated","status":"completed","effectiveModel":"%s","chainClosed":true,"critiqueExhaustions":%s}\n' \
      "$model" "$items" | write_staged "$controller/artifacts/agents/jobs/other-critic.json"
  fi
  findings='[]'
  material_count=0
  if [[ -n "$material_id" ]]; then
    findings="[{\"id\":\"$material_id\",\"severity\":\"high\",\"material\":true,\"claim\":\"fixture material finding remains open\",\"evidence\":\"fixture evidence\"}]"
    material_count=1
  fi
  printf '{"jobId":"critic","round":1,"runtime":"fake","sessionId":"critic-session","model":{"requested":"%s","effective":"%s"},"evidence":[],"gaps":[],"mode":"implement","reviewedTree":"%s","findings":%s,"verdictMaterialCount":%s}\n' \
    "$model" "$model" "$tree" "$findings" "$material_count" \
    | write_staged "$controller/artifacts/agents/critic/rounds/1/return.json"
}

# G-5, the canonical-instruction protection amendment: derive rule-owning
# references from the contract, role preambles, and host-turn instruction, then
# prove that the one conformance-owned list covers every one of them.
# The G-5 instruction-owner coverage lint moved to
# TestInstructionOwnersAreInstructionBearing (internal/validate, under
# the go gate — script-fixtures-001/D46). Post-D17 adopted repositories
# carry the engine source and run the full gate in their own CI, so the
# adopted-mode enforcement the verifier feared losing rides the same
# rails as every other gate test now.

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
reviewed_tree=$("$source_root/bin/metasystem" json get \
  --file "$controller/artifacts/agents/impl/rounds/1/review.json" --field reviewedTree)

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
expect_failure exhausted-successor 'prompt does not enumerate open findings: F-10' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl
write_critic "$reviewed_tree" F-9 critic-successor critic-model
expect_failure exhausted-wrong-party 'is not an implementer follow-up in the reviewed implementation chain' \
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
"$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl \
  >"$fixture_root/session-only.out"
grep -Fq 'independence=session-only recorded in gate evidence' "$fixture_root/session-only.out" \
  || { echo "session-only independence was accepted without an honest evidence record" >&2; exit 1; }

# CC-1-1, the ignored-local-file boundary: ignored files are outside both the
# temporary review index and the implementer's declared boundary.
new_case ignored-local
printf 'changed\n' >>"$worktree/source.txt"
printf 'role.code-critic.runtime=fake\n' >"$worktree/local.conf"
write_implementer '' source.txt
"$controller/scripts/agents/assert-conformance.sh" --stage review --job impl >/dev/null
grep -Fq 'local.conf' "$controller/artifacts/agents/impl/rounds/1/diff.patch" && {
  echo "review artifact included an ignored local configuration file" >&2
  exit 1
}

# G-6, the cumulative implementation boundary: each round declares only its
# own files, while review compares the merge-base-to-tree path set with the
# union of all declarations through that round.
new_case multi-round
printf 'round one\n' >>"$worktree/source.txt"
write_implementer '' source.txt
"$controller/scripts/agents/assert-conformance.sh" --stage review --job impl >/dev/null
commit_worktree
printf 'round two\n' >>"$worktree/docs/note.md"
write_followup_return 2 docs/note.md
"$controller/scripts/agents/assert-conformance.sh" --stage review --job impl-r2 >/dev/null
printf 'undeclared\n' >"$worktree/extra.txt"
expect_failure cumulative-boundary-outside 'some implementation round must declare every changed path' \
  "$controller/scripts/agents/assert-conformance.sh" --stage review --job impl-r2
grep -Fq 'extra.txt' "$fixture_root/cumulative-boundary-outside.out" \
  || { echo "cumulative boundary refusal did not name the undeclared path" >&2; exit 1; }

# CC-1-2 and CC-1-3: advancing the merge target does not make an immutable
# review-stage boundary fail, and merge never overwrites the critic's artifact,
# whether the first merge attempt passes or refuses a stale review.
new_case recovery
printf 'changed\n' >>"$worktree/source.txt"
write_implementer '' source.txt
"$controller/scripts/agents/assert-conformance.sh" --stage review --job impl >/dev/null
first_tree=$("$source_root/bin/metasystem" json get \
  --file "$controller/artifacts/agents/impl/rounds/1/review.json" --field reviewedTree)
commit_worktree
printf 'advanced target\n' >"$controller/upstream.txt"
git -C "$controller" add upstream.txt
git -C "$controller" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm upstream
controller_branch=$(git -C "$controller" branch --show-current)
git -C "$worktree" -c user.name=metasystem -c user.email=metasystem@example.invalid rebase -q "$controller_branch"
"$controller/scripts/agents/assert-conformance.sh" --stage review --job impl >/dev/null
final_tree=$(git -C "$worktree" rev-parse 'HEAD^{tree}')
artifact_sha=$(shasum -a 256 "$controller/artifacts/agents/impl/rounds/1/diff.patch" | awk '{print $1}')
configure_critic
write_critic "$first_tree" '' none critic-model
expect_failure recovery-stale 'is stale' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl
[[ "$(shasum -a 256 "$controller/artifacts/agents/impl/rounds/1/diff.patch" | awk '{print $1}')" == "$artifact_sha" ]] \
  || { echo "a refused merge rewrote the stored review artifact" >&2; exit 1; }
write_critic "$final_tree" '' none critic-model
"$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl >/dev/null
[[ "$(shasum -a 256 "$controller/artifacts/agents/impl/rounds/1/diff.patch" | awk '{print $1}')" == "$artifact_sha" ]] \
  || { echo "a successful merge rewrote the stored review artifact" >&2; exit 1; }

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

# CC-2-2: waivers never bypass trusted plans/ or the normally ignored agent
# control plane, even when every committed path otherwise qualifies as prose.
new_case waiver-plan
mkdir -p "$worktree/plans"
printf 'delegate plan\n' >"$worktree/plans/note.md"
write_implementer prose-under-30 plans/note.md
commit_worktree
expect_failure waiver-plan 'trusted plans/ state changed: plans/note.md' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl

new_case waiver-control-plane
printf 'small prose change\n' >>"$worktree/docs/note.md"
mkdir -p "$worktree/artifacts/agents"
printf 'tamper\n' >"$worktree/artifacts/agents/tamper"
write_implementer prose-under-30 docs/note.md
commit_worktree
expect_failure waiver-control-plane 'agent control plane contains delegate-created files' \
  "$controller/scripts/agents/assert-conformance.sh" --stage merge --job impl

# The critique-exhaustion owner is proven in-process under the go gate
# (script-fixtures-021/D44): TestCritiqueExhaustionDesignCritic,
# TestCritiqueExhaustionRoundOffBudget, TestCritiqueExhaustionGuards
# (protocol-error recovery with an absent return), and the ported
# TestCritiqueExhaustionCodeCriticChain (critic-cannot-own-successor,
# recorded-implementer-successor budget reopen, record-round-beats-
# lying-return). The one refusal string covers both the second-budget
# and human-only-remedy assertions the shell greps split across. The
# assert-conformance.sh stage-level E2E above remains this file's job.

echo "conformance fixtures passed"
