#!/usr/bin/env bash
set -euo pipefail

fixture_bed_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$fixture_bed_root/scripts/agents/fixture-budget.sh"
fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario goal-cli "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO

source "$fixture_bed_root/scripts/agents/fixture-bed-scenarios.sh"

if (( ! fixture_bed_child )); then
  fixture_bed_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios goal-cli "goal CLI fixtures: PASSED" \
	"$fixture_bed_script" migration-recovery human-lineage risk-basis labels-and-filtering structured-budget scope-bounds archive-and-prune classification-sweep \
	brain-claim-refuses brain-human-word-refuses brain-classification-fails brain-stop-seeded brain-stop-corrupt brain-status-line wrong-terminal \
	carry-word carried-record carried-discharge
fi
case "$fixture_scenario" in
	migration-recovery | human-lineage | risk-basis | labels-and-filtering | structured-budget | scope-bounds | archive-and-prune | classification-sweep | \
	brain-claim-refuses | brain-human-word-refuses | brain-classification-fails | brain-stop-seeded | brain-stop-corrupt | brain-status-line | wrong-terminal | \
	carry-word | carried-record | carried-discharge) ;;
  *) echo "goal CLI fixtures: unknown scenario: $fixture_scenario" >&2; exit 64 ;;
esac

# The F17 fold, shell half: the goal CLI verbs proven end to end
# through the REAL binary in a two-repository sandbox — source
# digest, migration, the read-side fetch, the identity-preserving
# rerun, and recovery. This also certifies the F16 lineage fix at
# the surface it matters: METASYSTEM_OWNER_LINEAGE must reach the
# synthesized claim record, not collapse to the literal "session".

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$root/scripts/agents/fixture-budget.sh"
harness_fixture_warn_if_engine_stale "$root"
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "goal-cli fixtures: bin/metasystem is not built" >&2; exit 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-goal-cli.XXXXXX")
brain_fake_server_pid=
brain_fake_server_dir=
cleanup() {
  local status=$? keep command
  if [[ -n "$brain_fake_server_pid" ]]; then
    command=$(ps -p "$brain_fake_server_pid" -o command= 2>/dev/null || true)
    if [[ "$command" == *"$brain_fake_server_dir"* ]]; then
      kill -TERM "$brain_fake_server_pid" 2>/dev/null || true
      wait "$brain_fake_server_pid" 2>/dev/null || true
    else
      echo "goal CLI fixture could not prove ownership of fake channel server pid $brain_fake_server_pid" >&2
      status=1
    fi
  fi
  if [[ $status -ne 0 && -d "$tmp" ]]; then
    keep="$root/artifacts/agents/suite-failures/$(date -u +%Y%m%dT%H%M%SZ)-goal-cli-$$"
    mkdir -p "$(dirname "$keep")"
    mv "$tmp" "$keep" 2>/dev/null \
      && echo "goal CLI fixture evidence preserved: $keep" >&2
    return "$status"
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

origin="$tmp/origin.git"
clone="$tmp/clone"
git init -q --bare "$origin"
git -C "$origin" config metasystem.goal.machine fixture-machine
git init -q -b main "$clone"
git -C "$clone" config metasystem.goal.machine fixture-machine
git -C "$clone" remote add origin "$origin"
git -C "$clone" -c user.name=fixture -c user.email=fixture@example.invalid commit -q --allow-empty -m seed
mkdir -p "$clone/plans"
cat >"$clone/plans/goals.md" <<'LEDGER'
# Goals

## Current goal: ship-widget — Ship the widget end to end
- Origin: human
- Next step: Wire the widget into the release train.
- Evidence: plans/widget.md

## Queued goal: fix-docs — Bring the docs current
- Origin: main
- Next step: Rewrite the quickstart against the new CLI.

## Parked goal: perf-pass — Cut p99 latency in half
- Origin: main
- Parked because: Blocked on the vendor's profiler fix.
- Next step: Re-profile once the vendor ships.

## Done goal: port-engine — Port the engine to Go
- Origin: human
- Concluded: Landed and gated on both hosts.
LEDGER
# The REAL baseline shape: the accepted ledger's bytes and digest —
# the migration precondition proves the digest matches goals.md. The
# ledger value must be the file's exact bytes, so the trailing newline
# command substitution eats is put back; schemaVersion must be a JSON
# number, which json set --int provides.
ledger=$(cat "$clone/plans/goals.md" && printf x) && ledger=${ledger%x}
"$ms" json object ledger="$ledger" \
  sha256="$(shasum -a 256 "$clone/plans/goals.md" | cut -d' ' -f1)" \
  >"$clone/plans/goals-accepted.json"
"$ms" json set --file "$clone/plans/goals-accepted.json" --int schemaVersion=1
# The sandbox ships the guard so the CLI's enrollment (R2-11) has
# something to enroll — a fresh clone has no hooks at all.
mkdir -p "$clone/scripts/agents"
cp "$root/scripts/agents/pre-commit-guard.sh" "$clone/scripts/agents/"
cp -R "$root/scripts/agents/adapters" "$clone/scripts/agents/"
printf '%s\n' 'metasystem.runtimes=fake' >"$clone/metasystem.conf"
git -C "$clone" add plans scripts metasystem.conf
git -C "$clone" -c user.name=fixture -c user.email=fixture@example.invalid commit -qm "legacy ledger"
git -C "$clone" push -q origin main

# This suite is intentionally headless. Enroll its shell as the exact fake
# checkout holder so claim-bearing goal fixtures carry a real claim epoch.
fixture_start=$("$ms" proc started-at --pid "$$")
if [[ "$fixture_scenario" != wrong-terminal ]]; then
  "$ms" lease announce --root "$clone" --session goal-cli-fixture \
    --pid "$$" --start "$fixture_start" --tag goal-cli-fixture \
    --runtime fake --owner-lineage fixture-lineage >/dev/null
fi

if [[ "$fixture_scenario" == wrong-terminal ]]; then
  mkdir -p "$clone/bin"
  cp "$ms" "$clone/bin/metasystem"
  expected_checkout=$(cd "$clone" && pwd -P)
  identity_file=$tmp/wrong-terminal-identities.json
  export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE=$identity_file
  printf '{"%s":{"terminal":false}}\n' "$$" >"$identity_file"
  set +e
  "$clone/bin/metasystem" stop --repo "$clone" >"$tmp/wrong-terminal.refusal" 2>&1
  refusal_rc=$?
  set -e
  (( refusal_rc == 1 )) \
    || { echo "a non-terminal caller was allowed to stop the metasystem" >&2; cat "$tmp/wrong-terminal.refusal" >&2; exit 1; }
  printf '%s\n' \
    "metasystem stop: stop is a human act at a terminal; this caller is DELEGATE." \
    "at an agent-free terminal, run: metasystem stop --repo $expected_checkout" \
    >"$tmp/wrong-terminal.expected"
  cmp -s "$tmp/wrong-terminal.expected" "$tmp/wrong-terminal.refusal" \
    || { echo "wrong-terminal refusal did not match the process-verb grammar" >&2; diff -u "$tmp/wrong-terminal.expected" "$tmp/wrong-terminal.refusal" >&2 || true; exit 1; }

  # The refusal above deliberately inherits the bed's delegate ancestry. For
  # the fixture-human half, make the scratch installation's signature universe
  # agent-free before its staged terminal fact is read, as the supervision bed
  # does for control-plane scenarios.
  for adapter in "$clone"/scripts/agents/adapters/*.sh; do
    case "${adapter##*/}" in fake.sh | runtime-common.sh) ;;
      *) rm -f "$adapter" ;;
    esac
  done
  printf '{"%s":{"terminal":true}}\n' "$$" >"$identity_file"
  "$clone/bin/metasystem" stop --repo "$clone" >"$tmp/wrong-terminal.stop"
  fence_changed=$("$clone/bin/metasystem" json get --file "$clone/artifacts/agents/supervision/transition.json" --field changedAt)
  fence_pid=$("$clone/bin/metasystem" json get --file "$clone/artifacts/agents/supervision/transition.json" --field by.pid)
  printf '%s\n' "checkout $expected_checkout" "nothing is running" \
    "stopped $expected_checkout; start again: metasystem arm --repo $expected_checkout" >"$tmp/wrong-terminal.stop.expected"
  cmp -s "$tmp/wrong-terminal.stop.expected" "$tmp/wrong-terminal.stop" \
    || { echo "the fixture-human stop report changed grammar" >&2; diff -u "$tmp/wrong-terminal.stop.expected" "$tmp/wrong-terminal.stop" >&2 || true; exit 1; }
  "$clone/bin/metasystem" status --repo "$clone" >"$tmp/wrong-terminal.status"
  printf '%s\n' "checkout $expected_checkout" "nothing is running" \
    "stopped since $fence_changed by stop pid $fence_pid; start again: metasystem arm --repo $expected_checkout" >"$tmp/wrong-terminal.status.expected"
  cmp -s "$tmp/wrong-terminal.status.expected" "$tmp/wrong-terminal.status" \
    || { echo "status did not report the fixture-human stop" >&2; diff -u "$tmp/wrong-terminal.status.expected" "$tmp/wrong-terminal.status" >&2 || true; exit 1; }
  exit 0
fi

# 1. source-digest speaks the exact bytes.
digest=$("$ms" goal source-digest --root "$clone")
want=$(shasum -a 256 "$clone/plans/goals.md" | cut -d' ' -f1)
[[ "$digest" == "$want" ]] \
  || { echo "goal source-digest disagrees with sha256: $digest vs $want" >&2; exit 1; }

manifest="$tmp/manifest.md"
cat >"$manifest" <<MANIFEST
# Queue amendments

MIGRATION_EPOCH: 2026-08-20T00:00:00Z
REVIEWED_SOURCE_SHA256: $digest

### amend-goal: fix-docs
- next: The amended next step.
MANIFEST

# 2. The migration, under the runner's REAL lineage export: the
# synthesized claim must carry it (F16 — the second env spelling
# collapsed every session to the literal "session").
migrate_out=$(cd "$clone" && METASYSTEM_OWNER_LINEAGE=fixture-lineage \
  "$ms" goal migrate --root "$clone" --source-digest "$digest" --manifest "$manifest" --by wido)

if [[ "$fixture_scenario" == migration-recovery ]]; then
grep -q '"outcome": "confirmed"' <<<"$migrate_out" \
  || { echo "goal migrate did not confirm: $migrate_out" >&2; exit 1; }
identity=$(sed -n 's/.*"identity": "\([^"]*\)".*/\1/p' <<<"$migrate_out" | head -1)
[[ -n "$identity" ]] || { echo "goal migrate reported no identity" >&2; exit 1; }
tip=$(git -C "$clone" rev-parse origin/main 2>/dev/null || true)
canonical_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$canonical_tip:plans/goals/ship-widget.md" >"$tmp/claimed.md"
grep -q "lineage=fixture-lineage" "$tmp/claimed.md" \
  || { echo "the claim does not carry METASYSTEM_OWNER_LINEAGE (F16 regression):" >&2; cat "$tmp/claimed.md" >&2; exit 1; }
if git -C "$clone" cat-file -e "$canonical_tip:plans/goals.md" 2>/dev/null; then
  echo "goals.md survived the cutover commit" >&2; exit 1
fi

# 2b. The mutation ENROLLED the guard (R2-11): a fresh clone has no
# hooks, and the migrate installed the composer before publishing.
grep -q "pre-commit-guard.sh" "$clone/.git/hooks/pre-commit" \
  || { echo "goal migrate did not enroll the pre-commit guard (R2-11)" >&2; exit 1; }

# 3. The read-side fetch reports the canonical tip and settles on
# already-current (the migration's confirm advanced the accepted
# ref, so both calls are consistency checks).
fetch_out=$("$ms" goal fetch --root "$clone")
grep -q "tip=$canonical_tip" <<<"$fetch_out" \
  || { echo "goal fetch does not report the canonical tip: $fetch_out" >&2; exit 1; }
fetch_again=$("$ms" goal fetch --root "$clone")
grep -q "already at the canonical tip" <<<"$fetch_again" \
  || { echo "the second fetch is not already-current: $fetch_again" >&2; exit 1; }

# 4. The post-cutover rerun: goals.md gone from the checkout, no
# --identity supplied — the CLI adopts the ledger's standing
# identity and classifies idempotent (F4 residue).
git -C "$clone" fetch -q origin
git -C "$clone" reset -q --hard origin/main
[[ ! -e "$clone/plans/goals.md" ]] || { echo "the cutover checkout still carries goals.md" >&2; exit 1; }
rerun_out=$(cd "$clone" && METASYSTEM_OWNER_LINEAGE=fixture-lineage \
  "$ms" goal migrate --root "$clone" --source-digest "$digest" --manifest "$manifest" --by wido)
grep -q '"outcome": "confirmed"' <<<"$rerun_out" \
  || { echo "the rerun did not confirm: $rerun_out" >&2; exit 1; }
grep -q '"detail": "idempotent"' <<<"$rerun_out" \
  || { echo "the rerun did not classify idempotent: $rerun_out" >&2; exit 1; }
grep -q "\"identity\": \"$identity\"" <<<"$rerun_out" \
  || { echo "the rerun re-minted an identity (F4 regression): $rerun_out" >&2; exit 1; }

# 5. Recovery runs clean on a healthy journal.
recover_out=$("$ms" goal recover --root "$clone")
[[ $? -eq 0 ]] || { echo "goal recover refused a healthy journal: $recover_out" >&2; exit 1; }
fi

if [[ "$fixture_scenario" != migration-recovery ]]; then
  git -C "$clone" fetch -q origin
  git -C "$clone" reset -q --hard origin/main
  export METASYSTEM_OWNER_LINEAGE=fixture-lineage
fi

approve_fixture_goal() { # goal id, optional complete tuple flags
  local goal_id=$1 output
  shift
  if ! output=$("$ms" goal approve --root "$clone" --id "$goal_id" --by Wido \
      --fixture-human-authority "$@" 2>&1); then
    echo "fixture approval for $goal_id failed: $output" >&2
    exit 1
  fi
}

prepare_carried_record_fixture() {
  git -C "$clone" fetch -q origin
  git -C "$clone" reset -q --hard origin/main
  printf 'human carried landing fixture\n' >"$clone/carried-payload.txt"
  git -C "$clone" add -- carried-payload.txt
  carry_project=$(git -C "$clone" write-tree)
  carry_workspace=$("$ms" landing workspace --root "$clone" --tree "$carry_project")
  carry_output=$("$ms" goal carry --root "$clone" --id ship-widget --by Wido \
    --tree "$carry_project" --past missing-declaration --why "fixture carries one named refusal" \
    --raise-format --fixture-human-authority)
  carry_word=$(sed -n 's/^carry=\([^ ]*\) workspace=.*/\1/p' <<<"$carry_output")
  [[ -n "$carry_word" ]] || { echo "carried fixture received no carry word: $carry_output" >&2; exit 1; }
  reservation_output=$("$ms" goal carrying --root "$clone" --id ship-widget --ref "$carry_word" --tree "$carry_project")
  carrying_row=${reservation_output#carrying=}
  carrying_row=${carrying_row%% ledger=*}
  carrying_ledger=${reservation_output##* ledger=}
  [[ -n "$carrying_row" && "$carrying_ledger" =~ ^[0-9a-f]{40}$ ]] \
    || { echo "carried fixture received an incomplete reservation: $reservation_output" >&2; exit 1; }

  git -C "$clone" fetch -q origin
  git -C "$clone" reset -q --hard origin/main
  printf 'human carried landing fixture\n' >"$clone/carried-payload.txt"
  git -C "$clone" add -- carried-payload.txt
  carried_project=$(git -C "$clone" write-tree)
  carried_workspace=$("$ms" landing workspace --root "$clone" --tree "$carried_project")
  [[ "$carried_workspace" == "$carry_workspace" ]] \
    || { echo "carry-forward changed the workspace projection" >&2; exit 1; }
  judge_digest=$(printf 'fixture carried judge' | shasum -a 256 | cut -d' ' -f1)
  git -C "$clone" -c user.name=fixture -c user.email=fixture@example.invalid commit -qm "carried fixture" \
    --trailer "Goal-Item: ship-widget" \
    --trailer "Carry: $carry_word" \
    --trailer "Carried-By: human:Wido" \
    --trailer "Carried-Tree: workspace=$carried_workspace project=$carried_project" \
    --trailer "Carried-Past: missing-declaration" \
    --trailer "Carried-Battery: green" \
    --trailer "Carried-Judge: live sha256=$judge_digest" \
    --trailer "Carried-Ledger: $carrying_ledger" \
    --trailer "Landing-Provenance: carried opid=$carry_word past=missing-declaration"
  carried_commit=$(git -C "$clone" rev-parse HEAD)
  git -C "$clone" push -q origin main
  "$ms" goal carried --root "$clone" --id ship-widget --ref "$carry_word" \
    --rebuild-from-commit "$carried_commit" >/dev/null
}

if [[ "$fixture_scenario" == carry-word ]]; then
  printf 'carry word fixture\n' >"$clone/carry-word.txt"
  git -C "$clone" add -- carry-word.txt
  carry_tree=$(git -C "$clone" write-tree)
  git -C "$clone" config goal.sync-remote local
  set +e
  remote_ask=$("$ms" goal carry --root "$clone" --id ship-widget --by Wido \
    --tree "$carry_tree" --past missing-declaration --why "fixture remote fence" \
    --fixture-human-authority 2>&1)
  remote_rc=$?
  set -e
  [[ $remote_rc -eq 3 && "$remote_ask" == *"carry-remote-required: a carried landing needs a code remote: set goal.sync-remote"* ]] \
    || { echo "single-machine carry did not ask for a code remote: rc=$remote_rc $remote_ask" >&2; exit 1; }
  git -C "$clone" config goal.sync-remote origin
  set +e
  format_ask=$("$ms" goal carry --root "$clone" --id ship-widget --by Wido \
    --tree "$carry_tree" --past missing-declaration --why "fixture format fence" \
    --fixture-human-authority 2>&1)
  format_rc=$?
  set -e
  [[ $format_rc -eq 3 && "$format_ask" == *"carry-format-required"* ]] \
    || { echo "format-1 carry did not ask for the one-way raise: rc=$format_rc $format_ask" >&2; exit 1; }
  word_output=$("$ms" goal carry --root "$clone" --id ship-widget --by Wido \
    --tree "$carry_tree" --past missing-declaration --why "fixture raises the carry format" \
    --raise-format --fixture-human-authority)
  word=$(sed -n 's/^carry=\([^ ]*\) workspace=.*/\1/p' <<<"$word_output")
  [[ -n "$word" && $(wc -l <<<"$word_output" | tr -d ' ') -eq 7 ]] \
    || { echo "goal carry did not speak its seven counselor lines: $word_output" >&2; exit 1; }
  grep -Fq "open carries: 1 on seat fixture-machine" <<<"$word_output"
  grep -Fq "carry debt: obligations=0 inflight=0" <<<"$word_output"
  grep -Fq "ledger format: 2" <<<"$word_output"
  set +e
  cap_ask=$("$ms" goal carry --root "$clone" --id ship-widget --by Wido \
    --tree "$carry_tree" --past missing-declaration --why "fixture proves the cap" \
    --fixture-human-authority 2>&1)
  cap_rc=$?
  set -e
  [[ $cap_rc -eq 3 && "$cap_ask" == *"carry-cap-reached"* && "$cap_ask" == *"$word"* ]] \
    || { echo "a second open carry did not ask with the existing word: rc=$cap_rc $cap_ask" >&2; exit 1; }
  successor_output=$("$ms" goal carry --root "$clone" --id ship-widget --by Wido \
    --tree "$carry_tree" --past missing-declaration --why "fixture supersedes the word" \
    --supersede "$word" --fixture-human-authority)
  successor=$(sed -n 's/^carry=\([^ ]*\) workspace=.*/\1/p' <<<"$successor_output")
  [[ -n "$successor" && "$successor" != "$word" ]] \
    || { echo "supersede did not mint a distinct word: $successor_output" >&2; exit 1; }
  echo "carry-word passed"
  exit 0
fi

if [[ "$fixture_scenario" == carried-record ]]; then
  prepare_carried_record_fixture
  carried_tip=$(git -C "$origin" rev-parse main)
  carried_goal=$(git -C "$origin" show "$carried_tip:plans/goals/ship-widget.md")
  grep -Fq "approvedRef=$carry_word" <<<"$carried_goal" \
    || { echo "goal carried wrote no row for the carry word" >&2; exit 1; }
  grep -Fq "finding=carried:$carried_commit chain=human-carried artifact=\"commit:$carried_commit\" test=\"pending\" state=open" <<<"$carried_goal" \
    || { echo "goal carried wrote no exact review obligation" >&2; exit 1; }
  grep -Fq -- "- BudgetExceptions: 1" <<<"$carried_goal" \
    || { echo "goal carried did not count its budget exception" >&2; exit 1; }
  "$ms" goal carried --root "$clone" --id ship-widget --ref "$carry_word" \
    --rebuild-from-commit "$carried_commit" >/dev/null
  replay_tip=$(git -C "$origin" rev-parse main)
  [[ "$replay_tip" == "$carried_tip" ]] \
    || { echo "goal carried replay advanced the accepted ledger" >&2; exit 1; }
  echo "carried-record passed"
  exit 0
fi

if [[ "$fixture_scenario" == carried-discharge ]]; then
  prepare_carried_record_fixture
  finding="carried:$carried_commit"
  accepted=$("$ms" goal accept-risk --root "$clone" --id ship-widget --finding "$finding" \
    --chain human-carried --by Wido --why "fixture accepts the deferred review" --fixture-human-authority)
  [[ "$accepted" == *'"outcome":"confirmed"'* ]] \
    || { echo "human-carried accept-risk did not confirm: $accepted" >&2; exit 1; }
  accepted_tip=$(git -C "$origin" rev-parse main)
  accepted_goal=$(git -C "$origin" show "$accepted_tip:plans/goals/ship-widget.md")
  grep -Fq "finding=$finding chain=human-carried artifact=\"commit:$carried_commit\" test=\"accepted-risk:" <<<"$accepted_goal" \
    || { echo "accepted risk did not discharge the carried obligation" >&2; exit 1; }
  grep -Fq 'state=discharged' <<<"$accepted_goal" \
    || { echo "accepted risk left the carried obligation open" >&2; exit 1; }
  register="$clone/records/counselor/accepted-risk-register.jsonl"
  [[ -s "$register" && $(wc -l <"$register" | tr -d ' ') -eq 1 ]] \
    || { echo "accepted-risk counselor register was not append-once" >&2; exit 1; }
  echo "carried-discharge passed"
  exit 0
fi

declare_brain_fixture() {
  local registry=$tmp/brain-registry
  mkdir -p "$registry"
  export METASYSTEM_SUPERVISION_REGISTRY_HOME=$registry
  "$ms" brain declare --root "$clone" --by Wido --fixture-human-authority >/dev/null
}

assert_brain_goal_refusal() { # expected fragment, command...
  local expected=$1 output rc
  shift
  set +e
  output=$("$@" 2>&1)
  rc=$?
  set -e
  [[ $rc -ne 0 && "$output" == *"$expected"* ]] || {
    echo "brain goal fence wanted '$expected', got rc=$rc: $output" >&2
    exit 1
  }
}

prepare_brain_stop_bed() {
  "$ms" goal release --root "$clone" --id ship-widget >/dev/null
  "$ms" goal open --root "$clone" --id brain-approved-one \
    --intent "Wait for the first node." --next "A node claims this." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "brain stop fixture" >/dev/null
  approve_fixture_goal brain-approved-one --budget box
  "$ms" goal open --root "$clone" --id brain-approved-two \
    --intent "Wait for a node." --next "A node claims this." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "brain stop fixture" >/dev/null
  approve_fixture_goal brain-approved-two --budget box
  METASYSTEM_OWNER_LINEAGE=earlier-brain-lineage "$ms" goal open --root "$clone" --id brain-draft \
    --intent "Draft work for Wido." --next "Wido approves this draft." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "brain draft fixture" >/dev/null
  mkdir -p "$clone/artifacts/agents/channel/questions"
  printf '%s\n' '{"id":"brain-ask","goal":"brain-draft","kind":"other","machine":"fixture-machine","openedAt":"2026-09-07T00:00:00Z","wants":"Wido chooses","state":"open"}' \
    >"$clone/artifacts/agents/channel/questions/brain-ask.json"
  mkdir -p "$clone/plans"
  printf '%s\n' '# Brain open plan' '- Next step: Finish the local note.' >"$clone/plans/brain-open-plan.md"
  printf '%s\n' '# Brain waiting plan' '- Waiting on the human: Choose a direction.' '- Next step: Continue after the answer.' \
    >"$clone/plans/brain-waiting-plan.md"
  declare_brain_fixture
}

if [[ "$fixture_scenario" == brain-stop-seeded ]]; then
  prepare_brain_stop_bed
  for stop in 1 2 3; do
    verdict=$("$ms" report turn-verdict --root "$clone" --session brain-stop --stop-hook-active=true)
    display=$("$ms" json get --value "$verdict" --field display)
    first=${display%%$'\n'*}
    second=${display#*$'\n'}; second=${second%%$'\n'*}
    third=${display#*$'\n'}; third=${third#*$'\n'}; third=${third%%$'\n'*}
    [[ "$first" == BRAIN\ SEAT:* && "$first" == *'2 approved goals'* && "$first" == *'1 asks await Wido (brain-ask)'* && \
       "$first" == *'1 drafts await approval (brain-draft)'* && "$second" == 'OPEN WORK (1)' && \
       "$third" == 'OPEN-WORK plans/brain-open-plan.md: Finish the local note.' ]] \
      || { echo "brain display lost its open work, action, ask, or draft on stop $stop: $display" >&2; exit 1; }
    [[ "$("$ms" json get --value "$verdict" --field idleRefusal)" == false ]] \
      || { echo "brain stop $stop entered idle refusal" >&2; exit 1; }
    block_source=$("$ms" json get --value "$verdict" --field blockSource --default '')
    [[ "$block_source" != idle-backlog ]] || { echo "brain stop $stop used idle-backlog source" >&2; exit 1; }
    if (( stop == 1 )); then
      [[ "$block_source" == open-work ]] || { echo "first brain stop did not preserve open-work block: $block_source" >&2; exit 1; }
      [[ "$("$ms" json get --value "$verdict" --field brainStatusDue)" == true ]] \
        || { echo "first brain stop did not mark status due" >&2; exit 1; }
      "$ms" json set --file "$clone/artifacts/agents/brain-status.json" --field lastPostedAt=2026-09-07T00:00:00Z
      export METASYSTEM_GOAL_NOW=2026-09-07T01:00:00Z
    else
      [[ -z "$block_source" ]] || { echo "repeated brain stop blocked again: $block_source" >&2; exit 1; }
      [[ "$("$ms" json get --value "$verdict" --field brainStatusDue)" == false ]] \
        || { echo "fresh brain status remained due" >&2; exit 1; }
    fi
  done
  [[ -f "$clone/artifacts/agents/brain-status.json" ]] || { echo "brain stop wrote no status record" >&2; exit 1; }
  if find "$clone/artifacts/agents/steward/intents" -type f -name '*.json' -print -quit 2>/dev/null | grep -q .; then
    echo "brain stop staged a steward continuation intent" >&2
    exit 1
  fi
  echo "brain-stop-seeded passed"
  exit 0
fi

if [[ "$fixture_scenario" == brain-stop-corrupt ]]; then
  prepare_brain_stop_bed
  printf '%s\n' '{broken' >"$clone/artifacts/agents/brain.json"
  verdict=$("$ms" report turn-verdict --root "$clone" --session brain-corrupt --stop-hook-active=true)
  display=$("$ms" json get --value "$verdict" --field display)
  first=${display%%$'\n'*}
  second=${display#*$'\n'}; second=${second%%$'\n'*}
  third=${display#*$'\n'}; third=${third#*$'\n'}; third=${third%%$'\n'*}
  fourth=${display#*$'\n'}; fourth=${fourth#*$'\n'}; fourth=${fourth#*$'\n'}; fourth=${fourth%%$'\n'*}
  [[ "$first" == BRAIN\ SEAT:* && "$second" == "this checkout's brain declaration is unreadable"* && \
     "$third" == 'OPEN WORK (1)' && "$fourth" == 'OPEN-WORK plans/brain-open-plan.md: Finish the local note.' ]] \
    || { echo "corrupt brain lost its leading summary, remedy, or following action: $display" >&2; exit 1; }
  [[ "$("$ms" json get --value "$verdict" --field idleRefusal)" == false ]] \
    || { echo "corrupt brain entered idle refusal" >&2; exit 1; }
  echo "brain-stop-corrupt passed"
  exit 0
fi

if [[ "$fixture_scenario" == brain-status-line ]]; then
  "$ms" goal release --root "$clone" --id ship-widget >/dev/null
  mkdir -p "$clone/records/misc"
  cp "$root/records/misc/fleet-coordinator-brain-role-packet.md" "$clone/records/misc/"
  declare_brain_fixture
  "$ms" brain boot --root "$clone" --repo "$clone" --bytes 10000 --deadline-ms 5000 >/dev/null
  status=$("$ms" channel status --root "$clone")
  first=${status%%$'\n'*}
  [[ "$first" == "BRAIN: fixture-machine for ledger "* && "$first" != *"://"* ]] \
    || { echo "declared status did not lead with transport-free brain line: $status" >&2; exit 1; }
  "$ms" brain withdraw --root "$clone" --by Wido --fixture-human-authority >/dev/null
  undeclared_status=$("$ms" channel status --root "$clone")
  [[ "$undeclared_status" != BRAIN:* ]] || { echo "undeclared status retained brain line" >&2; exit 1; }
  echo "brain-status-line passed"
  exit 0
fi

if [[ "$fixture_scenario" == brain-claim-refuses ]]; then
  "$ms" goal release --root "$clone" --id ship-widget >/dev/null
  "$ms" goal open --root "$clone" --id brain-claim-target \
    --intent "A node must claim this approved goal." --next "Claim it on a node." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "fixture claim fence" >/dev/null
  approve_fixture_goal brain-claim-target --budget box
  declared_tip=$(git -C "$origin" rev-parse main)
  declare_brain_fixture
  claim_command="this checkout is declared the brain; the brain never claims. A node claims: metasystem goal claim --root <checkout> --id <id>"
  assert_brain_goal_refusal "$claim_command" "$ms" goal claim --root "$clone" --id brain-claim-target
  assert_brain_goal_refusal "$claim_command" "$ms" goal open --root "$clone" --id brain-open-claim-target \
    --intent "A node must open and claim this goal." --next "Open it on a node." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "fixture open claim fence" \
    --claim --elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 0
  [[ $(git -C "$origin" rev-parse main) == "$declared_tip" ]] || {
    echo "brain claim refusal advanced the ledger tip" >&2
    exit 1
  }
  printf '%s\n' '{broken' >"$clone/artifacts/agents/brain.json"
  assert_brain_goal_refusal "this checkout's brain declaration is unreadable" "$ms" goal claim --root "$clone" --id brain-claim-target
  assert_brain_goal_refusal "this checkout's brain declaration is unreadable" "$ms" goal open --root "$clone" --id brain-open-claim-corrupt \
    --intent "A corrupt brain still cannot claim." --next "Repair the declaration." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "fixture corrupt claim fence" \
    --claim --elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 0
  [[ $(git -C "$origin" rev-parse main) == "$declared_tip" ]] || {
    echo "corrupt brain claim refusal advanced the ledger tip" >&2
    exit 1
  }
  echo "brain-claim-refuses passed"
  exit 0
fi

if [[ "$fixture_scenario" == brain-classification-fails ]]; then
  "$ms" goal release --root "$clone" --id ship-widget >/dev/null
  "$ms" goal open --root "$clone" --id classification-target \
    --intent "Exercise fail-closed caller classification." --next "Keep the ledger unchanged." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "fixture classification fence" >/dev/null
  declare_brain_fixture
  mkdir -p "$clone/artifacts/agents/supervision"
  printf '%s\n' '{broken' >"$clone/artifacts/agents/supervision/state.json"
  classifier_text="this checkout is declared the brain and the caller could not be classified"
  assert_brain_goal_refusal "$classifier_text" "$ms" goal approve --root "$clone" --id classification-target \
    --by Wido --budget box --fixture-human-authority
  assert_brain_goal_refusal "$classifier_text" "$ms" goal steal --root "$clone" --id classification-target --by Wido
  rm "$clone/artifacts/agents/supervision/state.json"
  "$ms" brain withdraw --root "$clone" --by Wido --fixture-human-authority >/dev/null
  undeclared_approve=$("$ms" goal approve --root "$clone" --id classification-target --by Wido --budget box --fixture-human-authority)
  [[ "$undeclared_approve" == *'"outcome":"confirmed"'* ]] || {
    echo "undeclared fixture approval no longer behaved as before: $undeclared_approve" >&2
    exit 1
  }
  set +e
  undeclared_steal=$("$ms" goal steal --root "$clone" --id classification-target --by Wido 2>&1)
  set -e
  [[ "$undeclared_steal" != *"caller could not be classified"* ]] || {
    echo "undeclared steal inherited the brain classification refusal: $undeclared_steal" >&2
    exit 1
  }
  echo "brain-classification-fails passed"
  exit 0
fi

if [[ "$fixture_scenario" == brain-human-word-refuses ]]; then
  "$ms" goal release --root "$clone" --id ship-widget >/dev/null
  mkdir -p "$clone/artifacts/agents/channel/questions"
  printf '%s\n' '{broken' >"$clone/artifacts/agents/channel/questions/undeclared-broken.json"
  undeclared_question_verdict=$("$ms" report turn-verdict --root "$clone" --session brain-undeclared-question)
  undeclared_question_display=$("$ms" json get --value "$undeclared_question_verdict" --field display)
  [[ "$undeclared_question_display" != *'inputs unreadable:'* && "$undeclared_question_display" != *'undeclared-broken.json'* ]] || {
    echo "an undeclared checkout inherited the brain-only malformed-question failure: $undeclared_question_display" >&2
    exit 1
  }
  rm "$clone/artifacts/agents/channel/questions/undeclared-broken.json"
  classification_draft=$tmp/brain-human-word-classification.txt
  cat >"$classification_draft" <<'DRAFT'
fix-docs 1,1,1,1 queued fixture
perf-pass 2,1,1,1 parked fixture
ship-widget 3,1,1,1 released fixture
DRAFT
  classification_preview=$("$ms" goal classify-sweep --root "$clone" --draft "$classification_draft" --preview)
  classification_digest=$(sed -n 's/^listing-digest //p' <<<"$classification_preview")
  [[ "$classification_digest" =~ ^[0-9a-f]{64}$ ]] || {
    echo "brain human-word fixture could not prepare its classification confirmation" >&2
    exit 1
  }
  declare_brain_fixture
  cp "$clone/artifacts/agents/brain.json" "$tmp/valid-brain.json"

  assert_human_word_matrix() { # expected text
    local expected=$1
    assert_brain_goal_refusal "$expected" "$ms" goal approve --root "$clone" --id fix-docs --by Wido --budget box \
      --temporary-human-word "Wido approves this fixture" --review-by 2026-09-08
    assert_brain_goal_refusal "$expected" "$ms" goal resume --root "$clone" --id fix-docs --by Wido \
      --elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3 \
      --temporary-human-word "Wido resumes this fixture" --review-by 2026-09-08
    assert_brain_goal_refusal "$expected" "$ms" goal resume --root "$clone" --id fix-docs --by Wido \
      --elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3 \
      --approved-ref fixture-answer
    assert_brain_goal_refusal "$expected" "$ms" goal set-obligation --root "$clone" --id fix-docs --by Wido \
      --state LIMITED --owner Wido --recurrence single-experiment --platform darwin-arm64 \
      --toolchain-identity go-fixture --surface-digest fixture-surface --max-active-jobs 1 --timing-envelope-sec 60 \
      --effect local-write --value-judgment no --reversibility reversible --severe-harm no \
      --unfamiliar-approach no --test-discrimination strong --correlated-assumption-risk no \
      --authority-scope-change no --destructive-reach reversible-local \
      --temporary-human-word "Wido authorizes this fixture" --review-by 2026-09-08
    assert_brain_goal_refusal "$expected" "$ms" goal steal --root "$clone" --id fix-docs --by Wido
    assert_brain_goal_refusal "$expected" "$ms" goal set-pin --root "$clone" --id fix-docs --pin node --by Wido
    assert_brain_goal_refusal "$expected" "$ms" goal classify-sweep --root "$clone" --draft "$classification_draft" \
      --confirm "$classification_digest" --by Wido
    assert_brain_goal_refusal "$expected" "$ms" goal set-budget --root "$clone" --id fix-docs --by Wido \
      --elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3
    assert_brain_goal_refusal "$expected" "$ms" goal unapprove --root "$clone" --id fix-docs --by Wido --because "fixture reversal"
    assert_brain_goal_refusal "$expected" "$ms" goal accept-risk --root "$clone" --id fix-docs --finding F1 \
      --chain brain-fixture-chain --by Wido --why "fixture risk decision"
    assert_brain_goal_refusal "$expected" "$ms" goal discharge-review-obligation --root "$clone" --id fix-docs \
      --finding F1 --chain brain-fixture-chain --by Wido --test "fixture test"
    assert_brain_goal_refusal "$expected" "$ms" goal repair --root "$clone" --accept-remote --by Wido
  }

  declared_tip=$(git -C "$origin" rev-parse main)
  assert_human_word_matrix "this checkout is declared the brain; the brain never carries a human's word into goal"
  cp "$clone/metasystem.conf" "$tmp/fake-root.conf"
  printf '%s\n' 'metasystem.runtimes=codex' >"$clone/metasystem.conf"
  assert_brain_goal_refusal "this checkout is declared the brain; the brain never carries a human's word into goal approve" \
    "$ms" goal approve --root "$clone" --id fix-docs --by Wido --budget box --fixture-human-authority
  cp "$tmp/fake-root.conf" "$clone/metasystem.conf"
  [[ $(git -C "$origin" rev-parse main) == "$declared_tip" ]] || {
    echo "brain human-word refusal advanced the ledger tip" >&2
    exit 1
  }
  printf '%s\n' '{broken' >"$clone/artifacts/agents/brain.json"
  assert_human_word_matrix "this checkout's brain declaration is unreadable"
  [[ $(git -C "$origin" rev-parse main) == "$declared_tip" ]] || {
    echo "corrupt brain human-word refusal advanced the ledger tip" >&2
    exit 1
  }
  cp "$tmp/valid-brain.json" "$clone/artifacts/agents/brain.json"

  "$ms" goal open --root "$clone" --id brain-fixture-authority-target \
    --intent "Prove fixture human authority crosses the brain seam." --next "Approve this classified goal." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "fixture human-authority positive path" >/dev/null
  fixture_approval=$("$ms" goal approve --root "$clone" --id brain-fixture-authority-target \
    --by Wido --budget box --fixture-human-authority)
  [[ "$fixture_approval" == *'"outcome":"confirmed"'* ]] || {
    echo "fixture-only human authority did not cross the brain seam: $fixture_approval" >&2
    exit 1
  }

  "$ms" goal open --root "$clone" --id brain-channel-target \
    --intent "Let a verified channel answer approve this draft." --next "Poll the verified answer." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "fixture verified channel answer" >/dev/null
  brain_fake_server_dir=$tmp/brain-channel-fake
  mkdir -p "$brain_fake_server_dir"
  "$ms" channel fake serve --dir "$brain_fake_server_dir" >"$tmp/brain-channel-server.log" 2>&1 &
  brain_fake_server_pid=$!
  deadline=$((SECONDS + 30))
  while [[ ! -s "$brain_fake_server_dir/base-url" ]]; do
    (( SECONDS < deadline )) || { echo "brain channel fake did not start within 30 seconds" >&2; exit 1; }
    sleep 0.05
  done
  secret=JBSWY3DPEHPK3PXP
  cat >>"$clone/metasystem.conf.local" <<CONF
channel.destination.fleet.adapter=fake
channel.destination.fleet.fake.dir=$brain_fake_server_dir
channel.human.slack.user-id=UWIDO
channel.human.totp-secret=$secret
channel.poll-timeout-sec=15
CONF
  qid=$("$ms" channel ask --root "$clone" --goal brain-channel-target --kind budget-above-norm \
    --fact "Approve the brain channel fixture." --option "approve: continue" \
    --elapsed-limit 1h --attempt-limit 1 --reserved-job-minutes-limit 60 --active-job-limit 1 --review-round-limit 3 \
    --recommend approve --wants "goal=brain-channel-target minutes=60 reviewRounds=3 goalRevision=3")
  question_file=$clone/artifacts/agents/channel/questions/$qid.json
  thread_object=$("$ms" json get --file "$question_file" --field thread)
  thread_id=$("$ms" json get --value "$thread_object" --field id)
  answer_token=$("$ms" json get --file "$question_file" --field wants)
  code=$("$ms" channel fake code --secret "$secret")
  printf '{"thread_ts":"%s","user":"UWIDO","text":"%s %s"}\n' "$thread_id" "$answer_token" "$code" >>"$brain_fake_server_dir/replies.jsonl"
  "$ms" channel poll --root "$clone" >/dev/null
  channel_tip=$(git -C "$origin" rev-parse main)
  channel_goal=$(git -C "$origin" show "$channel_tip:plans/goals/brain-channel-target.md")
  grep -q 'approve actor=human:UWIDO.*authorityOutcome=VERIFIED_CHANNEL_ANSWER' <<<"$channel_goal" || {
    echo "verified channel answer did not approve through the brain checkout" >&2
    printf '%s\n' "$channel_goal" >&2
    exit 1
  }
  echo "brain-human-word-refuses passed"
  exit 0
fi

if [[ "$fixture_scenario" == human-lineage ]]; then
# A fixture-authorized human act still gets its coordinator identity from the
# checkout's local terminal enrollment. The fixture writes the exact strict
# JSON shape consumed by humanauthority.ReadEnrollment because its headless
# shell cannot lawfully enroll itself as an attended terminal.
"$ms" goal open --root "$clone" --id explicit-lineage-approval \
  --intent "Record an approval under the fixture coordinator." --next "Compare its operation identity." \
  --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture identity comparison" >/dev/null
"$ms" goal open --root "$clone" --id derived-lineage-approval \
  --intent "Record an approval under the enrolled terminal." --next "Inspect its transaction journal." \
  --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture identity comparison" >/dev/null

explicit_approval=$("$ms" goal approve --root "$clone" --id explicit-lineage-approval \
  --by Wido --budget box --fixture-human-authority)
grep -q '"outcome":"confirmed"' <<<"$explicit_approval" \
  || { echo "the fixture-lineage approval did not confirm: $explicit_approval" >&2; exit 1; }
explicit_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$explicit_tip:plans/goals/explicit-lineage-approval.md" >"$tmp/explicit-lineage-approval.md"
explicit_opid=$(sed -n 's/^- Approved: .* opid=\([^ ]*\) authority=.*/\1/p' "$tmp/explicit-lineage-approval.md")
[[ -n "$explicit_opid" ]] \
  || { echo "the fixture-lineage approval recorded no operation identifier" >&2; exit 1; }
grep -q '"lineage": "fixture-lineage"' "$clone/artifacts/agents/goal-transactions/$explicit_opid.json" \
  || { echo "the explicit approval journal lost fixture-lineage" >&2; exit 1; }

mkdir -p "$clone/artifacts/agents/authority"
cat >"$clone/artifacts/agents/authority/human-terminal.json" <<'ENROLLMENT'
{
  "schema": 1,
  "enrolledAt": "2026-09-06T08:00:00Z",
  "generation": 7,
  "terminalId": "ttys:fixture",
  "terminalRef": {"pid": 1, "pidStartedAt": 1},
  "sessionLeaderRef": {"pid": 1, "pidStartedAt": 1}
}
ENROLLMENT
unset METASYSTEM_OWNER_LINEAGE

derived_approval=$("$ms" goal approve --root "$clone" --id derived-lineage-approval \
  --by Wido --budget box --fixture-human-authority)
grep -q '"outcome":"confirmed"' <<<"$derived_approval" \
  || { echo "the enrollment-derived approval did not confirm: $derived_approval" >&2; exit 1; }
derived_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$derived_tip:plans/goals/derived-lineage-approval.md" >"$tmp/derived-lineage-approval.md"
derived_opid=$(sed -n 's/^- Approved: .* opid=\([^ ]*\) authority=.*/\1/p' "$tmp/derived-lineage-approval.md")
derived_lineage=terminal-ttys-fixture-7
derived_lineage_hash=$(printf '%s' "$derived_lineage" | shasum -a 256 | cut -c1-8)
derived_opid_suffix=${derived_opid##*-}
[[ -n "$derived_opid" && "$derived_opid_suffix" == "$derived_lineage_hash" ]] \
  || { echo "the derived approval operation identifier suffix $derived_opid_suffix does not match lineage hash $derived_lineage_hash" >&2; exit 1; }
grep -q "\"lineage\": \"$derived_lineage\"" "$clone/artifacts/agents/goal-transactions/$derived_opid.json" \
  || { echo "the derived approval journal did not record $derived_lineage" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == risk-basis ]]; then
set +e
tier_alone=$("$ms" goal open --root "$clone" --id unanswered-risk \
  --intent "Refuse a tier without the four answers." --next "Answer the risk questions." --tier 2 2>&1)
tier_alone_rc=$?
set -e
[[ $tier_alone_rc -ne 0 && "$tier_alone" == *"answer the four questions: --risk severity=,novelty=,exposure=,accumulation= --basis"* ]] \
  || { echo "goal open --tier alone did not name the four unanswered questions: rc=$tier_alone_rc output=$tier_alone" >&2; exit 1; }
"$ms" goal open --root "$clone" --id risk-basis \
  --intent "Exercise risk-derived intake." --next "Keep the risk basis visible." \
  --risk severity=2,novelty=1,exposure=1,accumulation=1 \
  --basis "moderate consequence with landed precedent" >/dev/null
risk_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$risk_tip:plans/goals/risk-basis.md" >"$tmp/risk-basis.md"
risk_line=$(grep -n '^- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="moderate consequence with landed precedent"$' "$tmp/risk-basis.md" | cut -d: -f1)
tier_line=$(grep -n '^- Tier: 2$' "$tmp/risk-basis.md" | cut -d: -f1)
[[ -n "$risk_line" && -n "$tier_line" && $tier_line -eq $((risk_line + 1)) ]] \
  || { echo "risk-basis open did not render Risk immediately above derived Tier" >&2; cat "$tmp/risk-basis.md" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == labels-and-filtering ]]; then
# 6. Label writes are canonical whole fields. Open accepts repeated
# labels, sorts and deduplicates them, while an unlabeled open keeps
# the field absent.
export METASYSTEM_OWNER_LINEAGE=fixture-lineage
open_labels=$("$ms" goal open --root "$clone" --id labeled-one \
	--intent "First labeled goal." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" \
  --label beta --label alpha --label beta)
grep -q '"outcome":"confirmed"' <<<"$open_labels" \
  || { echo "goal open with labels did not confirm: $open_labels" >&2; exit 1; }
"$ms" goal open --root "$clone" --id plain-goal \
	--intent "An unlabeled goal." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" >/dev/null
labels_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$labels_tip:plans/goals/labeled-one.md" >"$tmp/labeled-one.md"
grep -q '^- Labels: alpha, beta$' "$tmp/labeled-one.md" \
  || { echo "open did not store sorted, deduplicated labels" >&2; cat "$tmp/labeled-one.md" >&2; exit 1; }
git -C "$clone" cat-file -p "$labels_tip:plans/goals/plain-goal.md" >"$tmp/plain-goal.md"
if grep -q '^- Labels:' "$tmp/plain-goal.md"; then
  echo "an unlabeled open wrote a Labels line" >&2; exit 1
fi

# 7. Edit computes one replacement field from adds and removes. An
# equal final set follows the shipped edit behavior and still records
# an edit; contradictory and malformed tokens refuse with the grammar.
"$ms" goal edit --root "$clone" --id labeled-one \
  --label shared --unlabel beta >/dev/null
edit_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$edit_tip:plans/goals/labeled-one.md" >"$tmp/labeled-one-edited.md"
grep -q '^- Labels: alpha, shared$' "$tmp/labeled-one-edited.md" \
  || { echo "label add/remove produced the wrong whole field" >&2; cat "$tmp/labeled-one-edited.md" >&2; exit 1; }
revision_before=$(sed -n 's/^- Revision: //p' "$tmp/labeled-one-edited.md")
"$ms" goal edit --root "$clone" --id labeled-one --label alpha >/dev/null
noop_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$noop_tip:plans/goals/labeled-one.md" >"$tmp/labeled-one-noop.md"
revision_after=$(sed -n 's/^- Revision: //p' "$tmp/labeled-one-noop.md")
[[ "$revision_after" -eq $((revision_before + 1)) ]] \
  || { echo "an equal final label set did not follow existing edit behavior" >&2; exit 1; }
if contradiction=$("$ms" goal edit --root "$clone" --id labeled-one \
  --label alpha --unlabel alpha 2>&1); then
  echo "a contradictory label edit succeeded" >&2; exit 1
fi
grep -q 'both --label and --unlabel' <<<"$contradiction" \
  || { echo "the contradictory edit did not name its refusal: $contradiction" >&2; exit 1; }
if bad_label=$("$ms" goal open --root "$clone" --id bad-label \
	--intent "Must refuse." --next "Stop." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --label Bad_Label 2>&1); then
  echo "a malformed label succeeded" >&2; exit 1
fi
grep -Fq 'must match ^[a-z][a-z0-9-]{0,31}$' <<<"$bad_label" \
  || { echo "the malformed label refusal did not name the grammar: $bad_label" >&2; exit 1; }
if orphan_label=$("$ms" goal claim --root "$clone" --id plain-goal --label x 2>&1); then
  echo "goal claim silently accepted an orphan --label flag" >&2; exit 1
fi
[[ "$orphan_label" == "goal claim does not take --label" ]] \
  || { echo "the orphan label refusal did not name the verb: $orphan_label" >&2; exit 1; }

# 8. List filters use AND across repeated labels and leave zero-label
# goals lawful but absent from a filtered result.
pretty_goal_ids() {
  awk 'NF == 5 && $1 ~ /^([123]|-)$/ && $2 ~ /^([0-9]+|-)$/ && $3 ~ /^(queued|approved|claimed|parked)$/ && $5 ~ /^[a-z][a-z0-9-]*$/ { print $5 }'
}
"$ms" goal open --root "$clone" --id labeled-two \
	--intent "Second labeled goal." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" \
  --label shared --label alpha >/dev/null
one_filter=$("$ms" goal list --root "$clone" --pretty --label shared)
one_ids=$(pretty_goal_ids <<<"$one_filter")
grep -Fxq 'labeled-one' <<<"$one_ids" && grep -Fxq 'labeled-two' <<<"$one_ids" \
  || { echo "one-label list filtering lost a match: $one_filter" >&2; exit 1; }
if grep -Fxq 'plain-goal' <<<"$one_ids"; then
  echo "a zero-label goal appeared in a filtered list" >&2; exit 1
fi
two_filters=$("$ms" goal list --root "$clone" --pretty --label alpha --label shared)
two_ids=$(pretty_goal_ids <<<"$two_filters")
grep -Fxq 'labeled-one' <<<"$two_ids" && grep -Fxq 'labeled-two' <<<"$two_ids" \
  || { echo "two-label AND filtering lost a match: $two_filters" >&2; exit 1; }
"$ms" goal open --root "$clone" --id and-a \
	--intent "Carries only a." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --label a >/dev/null
"$ms" goal open --root "$clone" --id and-ab \
	--intent "Carries a and b." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --label a --label b >/dev/null
and_probe=$("$ms" goal list --root "$clone" --pretty --label a --label b)
and_ids=$(pretty_goal_ids <<<"$and_probe")
[[ "$and_ids" == "and-ab" ]] \
  || { echo "the two-label list filter did not return exactly the goal carrying both labels: $and_probe" >&2; exit 1; }

# 9. A published canonical file survives fetch and a clean reconcile
# byte-for-byte. A later raw unsorted, duplicated hand edit remains raw
# when parsed from disk and reconcile republishes it canonically.
git -C "$clone" fetch -q origin
git -C "$clone" reset -q --hard origin/main
before_roundtrip=$(shasum -a 256 "$clone/plans/goals/labeled-one.md" | cut -d' ' -f1)
clean_reconcile=$("$ms" goal reconcile --root "$clone" --by wido)
grep -q '"rows":0' <<<"$clean_reconcile" \
  || { echo "the clean label round trip mapped a delta: $clean_reconcile" >&2; exit 1; }
after_roundtrip=$(shasum -a 256 "$clone/plans/goals/labeled-one.md" | cut -d' ' -f1)
[[ "$before_roundtrip" == "$after_roundtrip" ]] \
  || { echo "publish/fetch/reconcile changed canonical label bytes" >&2; exit 1; }
conf_edit "$clone/plans/goals/labeled-one.md" replace-line-first \
  '^- Labels: alpha, shared$' '- Labels: shared, alpha, shared'
grep -q '^- Labels: shared, alpha, shared$' "$clone/plans/goals/labeled-one.md" \
  || { echo "the raw hand-edit fixture was not formed" >&2; exit 1; }
hand_reconcile=$("$ms" goal reconcile --root "$clone" --by wido)
grep -q '"rows":1' <<<"$hand_reconcile" \
  || { echo "the raw label hand edit did not map to one edit: $hand_reconcile" >&2; exit 1; }
grep -q '^- Labels: alpha, shared$' "$clone/plans/goals/labeled-one.md" \
  || { echo "reconcile did not canonicalize the raw label field" >&2; exit 1; }

# 10. A held claim answers first even when it misses the filter. Once
# released, an empty filtered candidate set uses the distinct message.
held_next=$("$ms" goal next --root "$clone" --label absent)
grep -q '^continue your claimed goal: ship-widget$' <<<"$held_next" \
  || { echo "a label filter hid the held claim: $held_next" >&2; exit 1; }
"$ms" goal release --root "$clone" --id ship-widget >/dev/null
empty_next=$("$ms" goal next --root "$clone" --label absent)
[[ "$empty_next" == "no goal matches --label absent" ]] \
  || { echo "the empty filtered candidate message is not distinct: $empty_next" >&2; exit 1; }
"$ms" goal open --root "$clone" --id machine-only \
	--intent "Parked matching work is not claimable." --next "Wait for it to be unparked." \
	--risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "machine-scoped empty fixture" --label machine-only >/dev/null
"$ms" goal park --root "$clone" --id machine-only --because "Keep the matching goal unavailable." >/dev/null
machine_empty=$("$ms" goal next --root "$clone" --machine fixture-machine --label machine-only)
[[ "$machine_empty" == "no claimable goal for machine fixture-machine; no matching eligible work" ]] \
  || { echo "the machine-scoped empty candidate message is not distinct: $machine_empty" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == structured-budget ]]; then
"$ms" goal release --root "$clone" --id ship-widget >/dev/null
"$ms" goal open --root "$clone" --id plain-goal \
	--intent "An unlabeled goal." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" >/dev/null
# 11. The separated intake, approval, and claim path preserves labels.
"$ms" goal open --root "$clone" --id claimed-label \
	--intent "Claimed with its group." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --label custody >/dev/null
approve_fixture_goal claimed-label --budget box
claim_out=$("$ms" goal claim --root "$clone" --id claimed-label)
grep -q '"outcome":"confirmed"' <<<"$claim_out" \
	|| { echo "approved label claim did not confirm: $claim_out" >&2; exit 1; }
claim_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$claim_tip:plans/goals/claimed-label.md" >"$tmp/claimed-label.md"
grep -q '^- Labels: custody$' "$tmp/claimed-label.md" \
	|| { echo "separated claim dropped its label" >&2; cat "$tmp/claimed-label.md" >&2; exit 1; }

# 12. A queued goal needs no budget. At claim, the complete tuple becomes its
# only budget; an Appetite-prefixed sentence remains inert human prose.
"$ms" goal release --root "$clone" --id claimed-label >/dev/null
export METASYSTEM_GOAL_NOW=2026-08-20T00:00:00Z
"$ms" goal open --root "$clone" --id budget-check \
		--intent "Exercise structured budget admission." \
		--next "Appetite: 4h is inert human prose, not a budget." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" \
		--elapsed-limit 8h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3 >/dev/null
approve_fixture_goal budget-check \
	--elapsed-limit 8h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3
"$ms" goal claim --root "$clone" --id budget-check >/dev/null
claim_budget_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$claim_budget_tip:plans/goals/budget-check.md" >"$tmp/budget-claim.md"
grep -q '^- Budget: elapsedLimit=1d attemptLimit=2 reservedJobMinutesLimit=120 activeJobLimit=1 reviewRoundLimit=3$' "$tmp/budget-claim.md" \
	|| { echo "claim did not store the complete budget tuple" >&2; cat "$tmp/budget-claim.md" >&2; exit 1; }
grep -q '^- Claimed: .* revision=3' "$tmp/budget-claim.md" \
  || { echo "claim did not bind its goal revision" >&2; cat "$tmp/budget-claim.md" >&2; exit 1; }
if grep -q '^- Claimed: .* appetite=' "$tmp/budget-claim.md"; then
  echo "claim froze inert prose into a budget field" >&2; cat "$tmp/budget-claim.md" >&2; exit 1
fi

export METASYSTEM_GOAL_NOW=2026-08-20T05:01:00Z
set +e
admission_within=$("$ms" job goal-admission --root "$clone" --stop-lineage fixture-lineage 2>&1)
admission_within_rc=$?
set -e
[[ "$admission_within_rc" -eq 0 ]] \
  || { echo "the structured claim was refused while within all four limits: $admission_within" >&2; exit 1; }
export METASYSTEM_GOAL_NOW=2026-08-20T12:00:00Z
set +e
admission_spent=$("$ms" job goal-admission --root "$clone" --stop-lineage fixture-lineage 2>&1)
admission_spent_rc=$?
set -e
[[ "$admission_spent_rc" -eq 10 ]] \
  || { echo "the claim at its structured breach boundary did not request breach-stop (rc=$admission_spent_rc): $admission_spent" >&2; exit 1; }
grep -q 'BUDGET_REFUSED: goal budget-check revision=3 admission closed: elapsedLimit' <<<"$admission_spent" \
  || { echo "the structured refusal did not name its exact limit: $admission_spent" >&2; exit 1; }
export METASYSTEM_GOAL_NOW=2026-08-20T05:01:00Z
"$ms" goal set-budget --root "$clone" --id budget-check \
	--elapsed-limit 8h --attempt-limit 3 --reserved-job-minutes-limit 180 --active-job-limit 2 --review-round-limit 3 \
	--by Wido --fixture-human-authority >/dev/null
rebudget_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$rebudget_tip:plans/goals/budget-check.md" >"$tmp/budget-rebudget.md"
grep -q '^- Budget: elapsedLimit=1d attemptLimit=3 reservedJobMinutesLimit=180 activeJobLimit=2 reviewRoundLimit=3$' "$tmp/budget-rebudget.md" \
  || { echo "set-budget did not replace the complete tuple" >&2; cat "$tmp/budget-rebudget.md" >&2; exit 1; }
grep -q '^- Claimed: .* at=2026-08-20T05:01:00Z revision=4' "$tmp/budget-rebudget.md" \
  || { echo "set-budget did not bind the new revision and elapsed origin" >&2; cat "$tmp/budget-rebudget.md" >&2; exit 1; }

# A separate goal claims the budget already bound by its approval.
approve_fixture_goal plain-goal --budget box
other="$tmp/other"
env -u GIT_OBJECT_DIRECTORY -u GIT_ALTERNATE_OBJECT_DIRECTORIES git clone -q "$origin" "$other"
git -C "$other" config metasystem.goal.machine fixture-other
mkdir -p "$other/scripts/agents"
cp "$root/scripts/agents/pre-commit-guard.sh" "$other/scripts/agents/"
cp -R "$root/scripts/agents/adapters" "$other/scripts/agents/"
"$ms" lease announce --root "$other" --session goal-cli-other \
  --pid "$$" --start "$fixture_start" --tag goal-cli-fixture \
  --runtime fake --owner-lineage other-lineage >/dev/null
other_claim=$(cd "$other" && METASYSTEM_OWNER_LINEAGE=other-lineage \
	"$ms" goal claim --root "$other" --id plain-goal)
grep -q '"outcome":"confirmed"' <<<"$other_claim" \
	|| { echo "an approved-tuple claim did not confirm: $other_claim" >&2; exit 1; }
unset METASYSTEM_GOAL_NOW
fi

if [[ "$fixture_scenario" == scope-bounds ]]; then
"$ms" goal release --root "$clone" --id ship-widget >/dev/null

# An over-norm existing goal and the revisionless open-and-claim shortcut both
# exercise the typed refusal. The remedy must name split; no refusal fixture
# infers success from a parser-only unit.
"$ms" goal open --root "$clone" --id norm-parent \
	--intent "Hold a large intent before decomposition." --next "Split it first." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" >/dev/null
approve_fixture_goal norm-parent --budget box
"$ms" goal claim --root "$clone" --id norm-parent >/dev/null
if norm_refusal=$("$ms" goal set-budget --root "$clone" --id norm-parent \
	--elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 1441 \
	--active-job-limit 1 --review-round-limit 3 --by Wido \
	--fixture-human-authority 2>&1); then
  echo "over-norm set-budget succeeded without strict approval" >&2; exit 1
fi
grep -q 'GOAL_NORM_REFUSED: goal norm-parent' <<<"$norm_refusal" \
	&& grep -q 'split it into an arc of members within the box' <<<"$norm_refusal" \
  || { echo "the norm refusal did not name its type and split remedy: $norm_refusal" >&2; exit 1; }
if open_claim_refusal=$("$ms" goal open --root "$clone" --id norm-open-claim \
	--intent "Must not enter claimed over norm." --next "Stop." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --claim \
	--elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 1441 \
	--active-job-limit 1 --review-round-limit 3 2>&1); then
  echo "over-norm open --claim succeeded" >&2; exit 1
fi
grep -q 'APPROVAL_REQUIRED: open --claim is retired' <<<"$open_claim_refusal" \
	|| { echo "retired open --claim did not name the separated approval path: $open_claim_refusal" >&2; exit 1; }

# The real split command parses a closed draft, publishes both members and the
# parent conclusion atomically, and retires the parent identifier permanently.
"$ms" goal open --root "$clone" --id split-parent \
	--intent "Deliver the two-part fixture." --next "Atomize it." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --label fixture >/dev/null
draft="$tmp/split-parent.md"
{
  printf '%s\n' '# split split-parent'
  printf '%s\n' '' '## member split-parent-one'
  printf '%s\n' '- Intent: Deliver part one.' '- Next step: Build part one.'
  printf '%s\n' '' '## member split-parent-two'
  printf '%s\n' '- Intent: Deliver part two.' '- Next step: Build part two.' '- BlockedBy: split-parent-one'
} >"$draft"
split_out=$("$ms" goal split --root "$clone" --id split-parent --members "$draft")
grep -q '"outcome":"confirmed"' <<<"$split_out" \
  || { echo "goal split did not confirm: $split_out" >&2; exit 1; }
split_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$split_tip:plans/goals/split-parent-one.md" >"$tmp/split-one.md"
git -C "$clone" cat-file -p "$split_tip:records/goals/split-parent.md" >"$tmp/split-parent-done.md"
git -C "$clone" cat-file -p "$split_tip:plans/goals/backlog.md" >"$tmp/split-root.md"
grep -q '^- Arc: split-parent$' "$tmp/split-one.md" \
  && grep -q '^- Ratified: tier=main ' "$tmp/split-parent-done.md" \
  && grep -q 'goal:split-parent-one' "$tmp/split-parent-done.md" \
  && grep -q '^- split-parent opid=' "$tmp/split-root.md" \
  || { echo "the atomic split records are incomplete" >&2; exit 1; }
if reopen_refusal=$("$ms" goal reopen --root "$clone" --id split-parent 2>&1); then
  echo "a decomposed parent reopened" >&2; exit 1
fi
grep -q 'a decomposed parent never returns' <<<"$reopen_refusal" \
  || { echo "reopen did not name permanent decomposition: $reopen_refusal" >&2; exit 1; }
"$ms" goal prune --root "$clone" --keep 0 >/dev/null
if recreate_refusal=$("$ms" goal open --root "$clone" --id split-parent \
	--intent "Illicit resurrection." --next "Stop." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" 2>&1); then
  echo "a pruned decomposed parent id was recreated" >&2; exit 1
fi
grep -q 'goal id split-parent is retired' <<<"$recreate_refusal" \
  || { echo "the decomposition registry did not survive prune: $recreate_refusal" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == classification-sweep ]]; then
# STR3-MIGRATION-BOOTSTRAP-01 and the classify-sweep command contract: the
# migrated ledger supplies one queued, one claimed, and one parked tierless
# goal. Preview is inert and normalized; confirmation applies one transaction
# per goal and installs TierLaw only with the last edit.
"$ms" goal set-budget --root "$clone" --id ship-widget \
  --elapsed-limit 8h --attempt-limit 10 --reserved-job-minutes-limit 1200 \
  --active-job-limit 1 --review-round-limit 3 --by Wido \
  --fixture-human-authority >/dev/null
before_binding=$("$ms" job goal-binding --root "$clone" --goal ship-widget)
grep -q '"goalTier":3' <<<"$before_binding" \
  || { echo "tierless pre-TierLaw claim did not resolve under tier-three rules: $before_binding" >&2; exit 1; }
pre_sweep_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$pre_sweep_tip:plans/goals/ship-widget.md" >"$tmp/pre-sweep-ship-widget.md"

classification_draft="$tmp/classification-draft.txt"
cat >"$classification_draft" <<'DRAFT'
ship-widget 3,1,1,1 claimed migration
fix-docs 1,1,1,1 queued migration
perf-pass 2,1,1,1 parked migration
DRAFT
classification_preview=$("$ms" goal classify-sweep --root "$clone" --draft "$classification_draft" --preview)
classification_lines=$(sed '/^listing-digest /d' <<<"$classification_preview")
expected_lines=$'fix-docs 1,1,1,1 tier=1 queued migration\nperf-pass 2,1,1,1 tier=2 parked migration\nship-widget 3,1,1,1 tier=3 claimed migration'
[[ "$classification_lines" == "$expected_lines" ]] \
  || { echo "classification preview was not normalized and sorted: $classification_preview" >&2; exit 1; }
classification_digest=$(sed -n 's/^listing-digest //p' <<<"$classification_preview")
[[ "$classification_digest" =~ ^[0-9a-f]{64}$ ]] \
  || { echo "classification preview did not print its SHA-256 digest: $classification_preview" >&2; exit 1; }

assert_classification_refusal() { # code, draft bytes
  local code=$1 body=$2 output rc
  printf '%s\n' "$body" >"$tmp/refusal-draft.txt"
  set +e
  output=$("$ms" goal classify-sweep --root "$clone" --draft "$tmp/refusal-draft.txt" --preview 2>&1)
  rc=$?
  set -e
  [[ $rc -ne 0 && "$output" == *"$code"* ]] \
    || { echo "classification refusal $code did not fire: rc=$rc output=$output" >&2; exit 1; }
}
assert_classification_refusal SWEEP_UNKNOWN_GOAL $'fix-docs 1,1,1,1 queued migration\nperf-pass 2,1,1,1 parked migration\nship-widget 3,1,1,1 claimed migration\nabsent-goal 1,1,1,1 unknown'
assert_classification_refusal SWEEP_DUPLICATE_GOAL $'fix-docs 1,1,1,1 queued migration\nfix-docs 2,1,1,1 duplicate\nperf-pass 2,1,1,1 parked migration\nship-widget 3,1,1,1 claimed migration'
assert_classification_refusal SWEEP_INCOMPLETE $'fix-docs 1,1,1,1 queued migration\nship-widget 3,1,1,1 claimed migration'
assert_classification_refusal SWEEP_MALFORMED_ROW $'fix-docs 1 invalid tier\nperf-pass 2,1,1,1 parked migration\nship-widget 3,1,1,1 claimed migration'

sed 's/claimed migration/changed migration/' "$classification_draft" >"$tmp/changed-classification-draft.txt"
set +e
changed_output=$("$ms" goal classify-sweep --root "$clone" --draft "$tmp/changed-classification-draft.txt" \
  --confirm "$classification_digest" --by Wido 2>&1)
changed_rc=$?
set -e
[[ $changed_rc -ne 0 && "$changed_output" == *SWEEP_LISTING_CHANGED* ]] \
  || { echo "changed classification draft did not refuse by digest: rc=$changed_rc output=$changed_output" >&2; exit 1; }

classification_confirm=$("$ms" goal classify-sweep --root "$clone" --draft "$classification_draft" \
  --confirm "$classification_digest" --by Wido)
grep -q '"outcome":"confirmed"' <<<"$classification_confirm" \
  || { echo "classification confirmation did not confirm: $classification_confirm" >&2; exit 1; }
classified_tip=$(git -C "$origin" rev-parse main)
for tier_goal in 'fix-docs 1 0' 'perf-pass 2 2' 'ship-widget 3 3'; do
  read -r classified_id classified_tier classified_rounds <<<"$tier_goal"
  classified_file="$tmp/classified-$classified_id.md"
  git -C "$clone" cat-file -p "$classified_tip:plans/goals/$classified_id.md" >"$classified_file"
  grep -q "^- Tier: $classified_tier$" "$classified_file" \
    && grep -q "reviewRoundLimit=$classified_rounds$" "$classified_file" \
    || { echo "classification did not normalize $classified_id to tier $classified_tier and $classified_rounds rounds" >&2; cat "$classified_file" >&2; exit 1; }
done
git -C "$clone" cat-file -p "$classified_tip:plans/goals/backlog.md" >"$tmp/classified-root.md"
grep -q '^- TierLaw: since=' "$tmp/classified-root.md" \
  || { echo "classification confirmation did not install TierLaw" >&2; cat "$tmp/classified-root.md" >&2; exit 1; }

# Retain the classified root but restore the valid pre-sweep claimed record to
# prove the active dispatch binding refuses a tierless goal after TierLaw.
git -C "$clone" fetch -q origin
git -C "$clone" reset -q --hard origin/main
cp "$tmp/pre-sweep-ship-widget.md" "$clone/plans/goals/ship-widget.md"
git -C "$clone" add plans/goals/ship-widget.md
git -C "$clone" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -q --no-verify -m "fixture tierless goal after TierLaw"
git -C "$clone" push -q origin HEAD:main
git -C "$clone" update-ref refs/metasystem/goals/accepted HEAD
set +e
after_binding=$("$ms" job goal-binding --root "$clone" --goal ship-widget 2>&1)
after_binding_rc=$?
set -e
[[ $after_binding_rc -ne 0 && "$after_binding" == *"classify the goal first"* ]] \
  || { echo "post-TierLaw tierless goal did not refuse dispatch binding: rc=$after_binding_rc output=$after_binding" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == archive-and-prune ]]; then
"$ms" goal release --root "$clone" --id ship-widget >/dev/null
METASYSTEM_GOAL_NOW=2026-08-20T00:00:00Z \
  "$ms" goal open --root "$clone" --id budget-check \
		--intent "Exercise structured budget admission." \
		--next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --elapsed-limit 8h --attempt-limit 2 \
		--reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3 >/dev/null
approve_fixture_goal budget-check \
	--elapsed-limit 8h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3
METASYSTEM_GOAL_NOW=2026-08-20T00:00:00Z "$ms" goal claim --root "$clone" --id budget-check >/dev/null
# 13. Concluding writes the records-owned archive, reopening records a
# ledger move back to the live set, and concluding again preserves the
# canonical record bytes including its Integrity line.
"$ms" goal open --root "$clone" --id archive-roundtrip \
	--intent "Exercise concluded-goal archival." --next "Conclude it." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" >/dev/null
"$ms" goal done --root "$clone" --id archive-roundtrip \
  --conclude "Archived in the records-owned location." >/dev/null
archive_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$archive_tip:records/goals/archive-roundtrip.md" >"$tmp/archive-roundtrip.md"
grep -q '^Integrity: sha256=' "$tmp/archive-roundtrip.md" \
  || { echo "the records-owned conclusion lost its Integrity line" >&2; exit 1; }
if git -C "$clone" cat-file -e "$archive_tip:plans/goals/done/archive-roundtrip.md" 2>/dev/null; then
  echo "goal done wrote the legacy archive" >&2; exit 1
fi
"$ms" goal reopen --root "$clone" --id archive-roundtrip >/dev/null
reopen_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$reopen_tip:plans/goals/archive-roundtrip.md" >"$tmp/archive-reopened.md"
grep -q ' reopen actor=' "$tmp/archive-reopened.md" \
  || { echo "goal reopen did not record its History event" >&2; exit 1; }
if git -C "$clone" cat-file -e "$reopen_tip:records/goals/archive-roundtrip.md" 2>/dev/null; then
  echo "goal reopen left the concluded record behind" >&2; exit 1
fi
"$ms" goal done --root "$clone" --id archive-roundtrip \
  --conclude "Archived again after the recorded reopen." >/dev/null

# Admission must stop charging a concluded goal even when its only conclusion
# is in records/goals. Prove the causal change by exhausting a claimed goal,
# observing refusal, concluding it, and observing acceptance at the same clock.
"$ms" goal release --root "$clone" --id budget-check >/dev/null
METASYSTEM_GOAL_NOW=2026-08-20T10:00:00Z \
	"$ms" goal open --root "$clone" --id admission-concluded \
		--intent "Prove admission consumes records-owned conclusions." \
		--next "Conclude after its budget is exhausted." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" \
		--elapsed-limit 4h --attempt-limit 1 \
		--reserved-job-minutes-limit 30 --active-job-limit 1 --review-round-limit 3 >/dev/null
approve_fixture_goal admission-concluded \
	--elapsed-limit 4h --attempt-limit 1 --reserved-job-minutes-limit 30 --active-job-limit 1 --review-round-limit 3
METASYSTEM_GOAL_NOW=2026-08-20T10:00:00Z "$ms" goal claim --root "$clone" --id admission-concluded >/dev/null
set +e
admission_before=$(METASYSTEM_GOAL_NOW=2026-08-20T16:00:00Z \
  "$ms" job goal-admission --root "$clone" --stop-lineage fixture-lineage 2>&1)
admission_before_rc=$?
set -e
[[ "$admission_before_rc" -eq 10 ]] \
  || { echo "the exhausted live goal did not request breach-stop before conclusion (rc=$admission_before_rc): $admission_before" >&2; exit 1; }
grep -q 'BUDGET_REFUSED: goal admission-concluded revision=3 admission closed: elapsedLimit' <<<"$admission_before" \
  || { echo "the pre-conclusion refusal did not charge the exhausted goal: $admission_before" >&2; exit 1; }
METASYSTEM_GOAL_NOW=2026-08-20T16:00:00Z \
  "$ms" goal done --root "$clone" --id admission-concluded \
    --conclude "The records-owned conclusion must leave the admission budget." >/dev/null
admission_record_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -e "$admission_record_tip:records/goals/admission-concluded.md"
set +e
admission_after=$(METASYSTEM_GOAL_NOW=2026-08-20T16:00:00Z \
  "$ms" job goal-admission --root "$clone" --stop-lineage fixture-lineage 2>&1)
admission_after_rc=$?
set -e
[[ "$admission_after_rc" -eq 0 ]] \
  || { echo "the records-located conclusion still consumed admission budget (rc=$admission_after_rc): $admission_after" >&2; exit 1; }
if grep -q 'BUDGET_' <<<"$admission_after"; then
  echo "admission emitted a budget verdict after consuming the records-located conclusion: $admission_after" >&2; exit 1
fi

# 14. The soak reader accepts a legacy conclusion alongside records-owned
# conclusions. The fixture installs the already-accepted legacy shape with Git
# plumbing because production admission correctly refuses new legacy writes.
git -C "$clone" fetch -q origin
git -C "$clone" reset -q --hard origin/main
mkdir -p "$clone/plans/goals/done"
mv "$clone/records/goals/archive-roundtrip.md" "$clone/plans/goals/done/archive-roundtrip.md"
git -C "$clone" add plans/goals/done/archive-roundtrip.md records/goals/archive-roundtrip.md
git -C "$clone" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -q --no-verify -m "fixture accepted legacy archive"
git -C "$clone" push -q origin HEAD:main
legacy_tip=$(git -C "$clone" rev-parse HEAD)
git -C "$clone" update-ref refs/metasystem/goals/accepted "$legacy_tip"
dual_list=$("$ms" goal list --root "$clone" --pretty)
grep -q '^done: 3 archived$' <<<"$dual_list" \
  || { echo "the dual-location soak reader did not count both conclusions: $dual_list" >&2; exit 1; }
dual_show=$("$ms" goal show --root "$clone" --id archive-roundtrip)
grep -q '"where":"archived"' <<<"$dual_show" \
  || { echo "goal show did not read the legacy conclusion during the soak: $dual_show" >&2; exit 1; }

# 15. Prune removes concluded files from both locations but leaves the
# tombstone History event in the root ledger record.
"$ms" goal prune --root "$clone" --keep 0 >/dev/null
prune_tip=$(git -C "$origin" rev-parse main)
if git -C "$clone" ls-tree -r --name-only "$prune_tip" -- records/goals plans/goals/done | grep -q '\.md$'; then
  echo "goal prune left a concluded record outside its retention closure" >&2; exit 1
fi
git -C "$clone" cat-file -p "$prune_tip:plans/goals/backlog.md" >"$tmp/backlog-after-prune.md"
grep -q ' prune actor=' "$tmp/backlog-after-prune.md" \
  || { echo "goal prune removed files without its root History tombstone" >&2; exit 1; }
fi
