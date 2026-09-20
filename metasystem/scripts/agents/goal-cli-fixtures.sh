#!/usr/bin/env bash
set -euo pipefail

fixture_bed_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
fixture_bed_ms="${METASYSTEM_BIN:-$fixture_bed_root/bin/metasystem}"
[[ -x "$fixture_bed_ms" ]] || { echo "goal-cli fixtures: bin/metasystem is not built" >&2; exit 1; }
source "$fixture_bed_root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$fixture_bed_root"
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
    "$fixture_bed_script" migration-recovery human-lineage risk-basis labels-and-filtering structured-budget scope-bounds archive-and-prune classification-sweep abandoned-with-a-reason \
    brain-claim-refuses brain-human-word-refuses brain-classification-fails brain-stop-seeded brain-stop-corrupt brain-status-line wrong-terminal \
    carry-word carried-record carried-discharge seat-blocker power-of-attorney landing-slot budget-extension fenced-set-budget proof-grades \
    forgiving-budget-states forgiving-budget-members forgiving-budget-identity-aliases forgiving-human-refusals
fi
case "$fixture_scenario" in
  migration-recovery | human-lineage | risk-basis | labels-and-filtering | structured-budget | scope-bounds | archive-and-prune | classification-sweep | abandoned-with-a-reason | \
    brain-claim-refuses | brain-human-word-refuses | brain-classification-fails | brain-stop-seeded | brain-stop-corrupt | brain-status-line | wrong-terminal | \
    carry-word | carried-record | carried-discharge | seat-blocker | power-of-attorney | landing-slot | budget-extension | fenced-set-budget | proof-grades | \
    forgiving-budget-states | forgiving-budget-members | forgiving-budget-identity-aliases | forgiving-human-refusals) ;;
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
ms=$fixture_bed_ms
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-goal-cli.XXXXXX")
brain_fake_server_pid=
brain_fake_server_dir=
proof_grade_holder_pid=
proof_grade_holder_pid_file=
proof_grade_holder_start=
proof_grade_holder_command=
proof_grade_holder_session_pid=
proof_grade_holder_session_start=
proof_grade_holder_keeper_pid=
proof_grade_holder_keeper_start=
cleanup() {
  local status=$? keep command current_start cleanup_cap cleanup_started holder_unproven_cleanup= keeper_cleanup= session_cleanup=
  if [[ -n "$proof_grade_holder_keeper_pid" && -n "$proof_grade_holder_keeper_start" ]]; then
    current_start=$("$ms" proc started-at --pid "$proof_grade_holder_keeper_pid" 2>/dev/null || true)
    if [[ -n "$current_start" && "$current_start" == "$proof_grade_holder_keeper_start" ]]; then
      kill -TERM "$proof_grade_holder_keeper_pid" 2>/dev/null || true
      keeper_cleanup=yes
    elif [[ -z "$current_start" ]]; then
      keeper_cleanup=yes
    elif [[ -n "$current_start" ]]; then
      echo "goal CLI fixture could not prove ownership of proof-grade input keeper pid $proof_grade_holder_keeper_pid" >&2
      status=1
    fi
  fi
  if [[ -n "$proof_grade_holder_session_pid" && -n "$proof_grade_holder_session_start" ]]; then
    current_start=$("$ms" proc started-at --pid "$proof_grade_holder_session_pid" 2>/dev/null || true)
    if [[ -n "$current_start" && "$current_start" == "$proof_grade_holder_session_start" ]]; then
      kill -TERM "$proof_grade_holder_session_pid" 2>/dev/null || true
      session_cleanup=yes
    elif [[ -z "$current_start" ]]; then
      session_cleanup=yes
    else
      echo "goal CLI fixture could not prove ownership of proof-grade terminal session pid $proof_grade_holder_session_pid" >&2
      status=1
    fi
  fi
  if [[ -z "$proof_grade_holder_pid" && -n "$proof_grade_holder_pid_file" && -s "$proof_grade_holder_pid_file" ]]; then
    proof_grade_holder_pid=$(cat "$proof_grade_holder_pid_file")
    if [[ ! "$proof_grade_holder_pid" =~ ^[0-9]+$ ]]; then
      echo "goal CLI fixture found an invalid proof-grade holder pid in $proof_grade_holder_pid_file" >&2
      proof_grade_holder_pid=
      status=1
    fi
  fi
  if [[ -n "$proof_grade_holder_pid" && -n "$proof_grade_holder_start" ]]; then
    current_start=$("$ms" proc started-at --pid "$proof_grade_holder_pid" 2>/dev/null || true)
    if [[ -n "$current_start" && "$current_start" == "$proof_grade_holder_start" ]]; then
      kill -TERM "$proof_grade_holder_pid" 2>/dev/null || true
    elif [[ -n "$current_start" ]]; then
      echo "goal CLI fixture could not prove ownership of proof-grade holder pid $proof_grade_holder_pid" >&2
      status=1
    fi
    cleanup_cap=$(harness_fixture_cap mission-process-wait)
    cleanup_started=$(date +%s)
    while [[ "$("$ms" proc started-at --pid "$proof_grade_holder_pid" 2>/dev/null || true)" == "$proof_grade_holder_start" ]] &&
        (( $(date +%s) - cleanup_started < cleanup_cap )); do
      sleep 0.1
    done
    if [[ "$("$ms" proc started-at --pid "$proof_grade_holder_pid" 2>/dev/null || true)" == "$proof_grade_holder_start" ]]; then
      echo "proof-grade holder did not exit within ${cleanup_cap}s after termination" >&2
      status=1
      kill -KILL "$proof_grade_holder_pid" 2>/dev/null || true
    fi
  elif [[ -n "$proof_grade_holder_pid" ]]; then
    command=$(ps -p "$proof_grade_holder_pid" -o command= 2>/dev/null || true)
    if [[ "$command" == *metasystem-fake-agent* ]]; then
      kill -TERM "$proof_grade_holder_pid" 2>/dev/null || true
      holder_unproven_cleanup=yes
    elif [[ -n "$command" ]]; then
      echo "goal CLI fixture could not prove ownership of unrecorded proof-grade holder pid $proof_grade_holder_pid" >&2
      status=1
    fi
  fi
  if [[ -n "$holder_unproven_cleanup" ]]; then
    cleanup_cap=$(harness_fixture_cap mission-process-wait)
    cleanup_started=$(date +%s)
    command=$(ps -p "$proof_grade_holder_pid" -o command= 2>/dev/null || true)
    while [[ "$command" == *metasystem-fake-agent* ]] &&
        (( $(date +%s) - cleanup_started < cleanup_cap )); do
      sleep 0.1
      command=$(ps -p "$proof_grade_holder_pid" -o command= 2>/dev/null || true)
    done
    if [[ "$command" == *metasystem-fake-agent* ]]; then
      echo "unrecorded proof-grade holder did not exit within ${cleanup_cap}s after termination" >&2
      status=1
      kill -KILL "$proof_grade_holder_pid" 2>/dev/null || true
    fi
  fi
  if [[ -n "$keeper_cleanup" ]]; then
    cleanup_cap=$(harness_fixture_cap mission-process-wait)
    cleanup_started=$(date +%s)
    while [[ "$("$ms" proc started-at --pid "$proof_grade_holder_keeper_pid" 2>/dev/null || true)" == "$proof_grade_holder_keeper_start" ]] &&
        (( $(date +%s) - cleanup_started < cleanup_cap )); do
      sleep 0.1
    done
    if [[ "$("$ms" proc started-at --pid "$proof_grade_holder_keeper_pid" 2>/dev/null || true)" == "$proof_grade_holder_keeper_start" ]]; then
      echo "proof-grade input keeper did not exit within ${cleanup_cap}s after termination" >&2
      status=1
      kill -KILL "$proof_grade_holder_keeper_pid" 2>/dev/null || true
    fi
    wait "$proof_grade_holder_keeper_pid" 2>/dev/null || true
  fi
  if [[ -n "$session_cleanup" ]]; then
    cleanup_cap=$(harness_fixture_cap mission-process-wait)
    cleanup_started=$(date +%s)
    while [[ "$("$ms" proc started-at --pid "$proof_grade_holder_session_pid" 2>/dev/null || true)" == "$proof_grade_holder_session_start" ]] &&
        (( $(date +%s) - cleanup_started < cleanup_cap )); do
      sleep 0.1
    done
    if [[ "$("$ms" proc started-at --pid "$proof_grade_holder_session_pid" 2>/dev/null || true)" == "$proof_grade_holder_session_start" ]]; then
      echo "proof-grade terminal session did not exit within ${cleanup_cap}s after termination" >&2
      status=1
      kill -KILL "$proof_grade_holder_session_pid" 2>/dev/null || true
    fi
    wait "$proof_grade_holder_session_pid" 2>/dev/null || true
  fi
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
if [[ "$fixture_scenario" != wrong-terminal && "$fixture_scenario" != proof-grades ]]; then
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
  delegate_fixture=$tmp/wrong-terminal-delegate
  mkdir -p "$delegate_fixture"
  fake_agent=$delegate_fixture/metasystem-fake-agent
  tool_shell=$delegate_fixture/fixture-tool-shell
  cat >"$fake_agent" <<'FAKE_AGENT'
#!/usr/bin/env bash
set -euo pipefail
identity_file=$1
engine=$2
tool_shell=$3
suite_pid=$4
shift 4
started=$("$engine" proc started-at --pid $$)
printf '{"%s":{"pidStartedAt":%s,"command":"metasystem-fake-agent fixture","terminal":false},"%s":{"terminal":false}}\n' \
  "$$" "$started" "$suite_pid" >"$identity_file"
"$tool_shell" "$engine" "$@"
FAKE_AGENT
  cat >"$tool_shell" <<'FIXTURE_TOOL_SHELL'
#!/usr/bin/env bash
set -euo pipefail
engine=$1
shift
"$engine" "$@"
FIXTURE_TOOL_SHELL
  chmod +x "$fake_agent" "$tool_shell"
  set +e
  "$fake_agent" "$identity_file" "$clone/bin/metasystem" "$tool_shell" "$$" \
    stop --repo "$clone" >"$tmp/wrong-terminal.refusal" 2>&1
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

  # The bed stages the refusal's delegate ancestry. The fixture-human half
  # deliberately drops it, then makes the scratch installation's signature
  # universe agent-free before its staged terminal fact is read.
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

# Migration records its claim at the real clock. Start the two elapsed-budget
# scenarios just after that claim, then advance the fixture clock from the
# same base so the breach is independent of when the bed runs.
fixture_stamp() { # seconds since the epoch -> RFC3339 UTC
  date -u -r "$1" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "@$1" +%Y-%m-%dT%H:%M:%SZ
}
case "$fixture_scenario" in
  forgiving-budget-states | forgiving-human-refusals)
    forgiving_base_epoch=$(( $(date -u +%s) + 60 ))
    export METASYSTEM_GOAL_NOW=$(fixture_stamp "$forgiving_base_epoch")
    forgiving_breach_at=$(fixture_stamp $((forgiving_base_epoch + 6 * 3600)))
    forgiving_second_breach_at=$(fixture_stamp $((forgiving_base_epoch + 30 * 3600)))
    ;;
esac

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

if [[ "$fixture_scenario" == budget-extension ]]; then
  # Keep the fixture's tier box deliberately narrow: one completed attempt
  # earns one more attempt, so the second exhaustion is observable without a
  # row of redundant fake dispatches.
  printf '%s\n' 'metasystem.budget.tier-3=1h/1/60m/1/3' >>"$clone/metasystem.conf"
  git -C "$clone" -c core.hooksPath=/dev/null add metasystem.conf
  git -C "$clone" -c core.hooksPath=/dev/null -c user.name=fixture -c user.email=fixture@example.invalid \
    commit -qm 'narrow fixture budget box'
  git -C "$clone" push -q origin main
  "$ms" goal release --root "$clone" --id ship-widget >/dev/null
  export METASYSTEM_GOAL_NOW=2026-09-12T20:00:00Z
  "$ms" goal open --root "$clone" --id earned-extension --origin human \
    --intent "Exercise the once-by-consumption budget extension." --next "Apply the earned extension." \
    --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "budget extension fixture" >/dev/null
  approve_fixture_goal earned-extension \
    --elapsed-limit 8h --attempt-limit 1 --reserved-job-minutes-limit 60 --active-job-limit 1 --review-round-limit 3
  "$ms" goal claim --root "$clone" --id earned-extension >/dev/null
  extension_revision=$("$ms" job goal-revision --root "$clone" --goal earned-extension)

  mkdir -p "$clone/artifacts/agents/jobs"
  printf '%s\n' \
    '{"jobId":"earned-first","operationId":"earned-first","goalId":"earned-extension","goalRevision":3,"capMin":1,"status":"completed","startedAt":"2026-09-12T20:05:00Z","endedAt":"2026-09-12T20:06:00Z"}' \
    >"$clone/artifacts/agents/jobs/earned-first.json"

  # Landing evidence is read from the accepted tree. Put one shipped
  # implementation receipt on the canonical branch, then let the goal verb
  # fetch and publish from that exact tip.
  git -C "$clone" fetch -q origin
  git -C "$clone" reset -q --hard origin/main
  mkdir -p "$clone/memory"
  extension_receipt='1789245000|2026-09-12T20:30:00Z|RECEIPT|type=implement|outcome=shipped|goal=earned-extension|built_by=fixture|note=fixture advancement'
  extension_receipt_sha=$(printf '%s' "$extension_receipt" | shasum | cut -d' ' -f1)
  printf '%s\n' "$extension_receipt" >"$clone/memory/receipts.log"
  git -C "$clone" -c core.hooksPath=/dev/null add memory/receipts.log
  git -C "$clone" -c core.hooksPath=/dev/null -c user.name=fixture -c user.email=fixture@example.invalid \
    commit -qm 'fixture advancement receipt'
  git -C "$clone" push -q origin main
  "$ms" goal fetch --root "$clone" >/dev/null

  export METASYSTEM_GOAL_NOW=2026-09-12T21:00:00Z
  extension_out=$("$ms" goal extend-budget --root "$clone" --id earned-extension \
    --revision "$extension_revision" --proposed-cap 1 --role implementer \
    --dispatch-mode fresh --destructive-reach DESIGN-BEARING)
  grep -q '"outcome":"confirmed"' <<<"$extension_out" \
    || { echo "the earned extension did not confirm: $extension_out" >&2; exit 1; }
  extension_tip=$(git -C "$origin" rev-parse main)
  git -C "$clone" cat-file -p "$extension_tip:plans/goals/earned-extension.md" >"$tmp/earned-extension.md"
  grep -q "^- BudgetExtension: .* evidence=landing:1789245000-$extension_receipt_sha@2026-09-12T20:30:00Z\$" "$tmp/earned-extension.md" \
    || { echo "the extension marker does not name its landing evidence by epoch and line digest" >&2; cat "$tmp/earned-extension.md" >&2; exit 1; }

  printf '%s\n' \
    '{"jobId":"earned-second","operationId":"earned-second","goalId":"earned-extension","goalRevision":3,"capMin":1,"status":"completed","startedAt":"2026-09-12T21:01:00Z","endedAt":"2026-09-12T21:02:00Z"}' \
    >"$clone/artifacts/agents/jobs/earned-second.json"
  set +e
  second_extension=$("$ms" goal extend-budget --root "$clone" --id earned-extension \
    --revision "$extension_revision" --proposed-cap 1 --role implementer \
    --dispatch-mode fresh --destructive-reach DESIGN-BEARING 2>&1)
  second_extension_rc=$?
  set -e
  [[ $second_extension_rc -eq 1 && "$second_extension" == *"extended once at 2026-09-12T21:00:00Z"* ]] \
    || { echo "the second extension did not refuse with its marker: rc=$second_extension_rc $second_extension" >&2; exit 1; }
  unset METASYSTEM_GOAL_NOW
fi

if [[ "$fixture_scenario" == fenced-set-budget ]]; then
	"$ms" goal release --root "$clone" --id ship-widget >/dev/null
	export METASYSTEM_GOAL_NOW=2026-09-18T10:00:00Z
	"$ms" goal open --root "$clone" --id fenced-one-step --origin human \
		--intent "Replace a stopped budget and reopen admission atomically." --next "Run the larger budget." \
		--tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fenced rebudget fixture" >/dev/null
	approve_fixture_goal fenced-one-step \
		--elapsed-limit 1m --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3
	"$ms" goal claim --root "$clone" --id fenced-one-step >/dev/null
	fenced_revision=$("$ms" job goal-revision --root "$clone" --goal fenced-one-step)
	export METASYSTEM_GOAL_NOW=2026-09-18T10:02:00Z
	fenced_stop=$("$ms" job breach-stop --root "$clone" --goal fenced-one-step --revision "$fenced_revision")
	fenced_stop_id=$("$ms" json get --value "$fenced_stop" --field stopId)
	fenced_batch=$("$ms" job stop-batch-reconcile --root "$clone" --stop "$fenced_stop_id")
	[[ "$("$ms" json get --value "$fenced_batch" --field state)" == COMPLETE ]] \
		|| { echo "fenced set-budget fixture did not complete stop batch $fenced_stop_id: $fenced_batch" >&2; exit 1; }
	export METASYSTEM_GOAL_NOW=2026-09-18T10:03:00Z
	"$ms" goal set-budget --root "$clone" --id fenced-one-step \
		--elapsed-limit 8h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3 \
		--by Wido --fixture-human-authority >/dev/null
	fenced_tip=$(git -C "$origin" rev-parse main)
	git -C "$clone" cat-file -p "$fenced_tip:plans/goals/fenced-one-step.md" >"$tmp/fenced-one-step.md"
	if grep -q '^- StopFence:' "$tmp/fenced-one-step.md"; then
		echo "one-step set-budget left the completed launch fence in place" >&2
		cat "$tmp/fenced-one-step.md" >&2
		exit 1
	fi
	grep -q " set-budget .* resumed=$fenced_stop_id" "$tmp/fenced-one-step.md" \
		|| { echo "one-step set-budget history did not name the lifted stop $fenced_stop_id" >&2; cat "$tmp/fenced-one-step.md" >&2; exit 1; }
	grep -q '^- Budget: elapsedLimit=1d attemptLimit=2 reservedJobMinutesLimit=120 activeJobLimit=1 reviewRoundLimit=3$' "$tmp/fenced-one-step.md" \
		|| { echo "one-step set-budget did not install the replacement tuple" >&2; cat "$tmp/fenced-one-step.md" >&2; exit 1; }
	set +e
	fenced_admission=$("$ms" job goal-admission --root "$clone" --stop-lineage fixture-lineage 2>&1)
	fenced_admission_rc=$?
	set -e
	[[ "$fenced_admission_rc" -eq 0 ]] \
		|| { echo "one-step set-budget did not reopen admission: $fenced_admission" >&2; exit 1; }
	unset METASYSTEM_GOAL_NOW
	exit 0
fi

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

if [[ "$fixture_scenario" == proof-grades ]]; then
  proof_grade_arc_origin=$tmp/proof-grade-arc-origin.git
  proof_grade_arc_clone=$tmp/proof-grade-arc-clone
  git clone -q --bare "$origin" "$proof_grade_arc_origin"
  git clone -q -b main "$proof_grade_arc_origin" "$proof_grade_arc_clone"
  git -C "$proof_grade_arc_clone" config metasystem.goal.machine fixture-arc-machine

  proof_grade_bed_view=$("$ms" lease classify --root "$clone" --metasystem-root "$root" --caller-pid "$$")
  proof_grade_bed_class=$("$ms" json get --value "$proof_grade_bed_view" --field class)
  case "$proof_grade_bed_class" in
    HUMAN | DELEGATE) ;;
    *) echo "proof-grade fixture bed classified $proof_grade_bed_class, not HUMAN or DELEGATE" >&2; exit 1 ;;
  esac
  export PROOF_GRADE_BED_CLASS=$proof_grade_bed_class

  # The holder is a signed fake-runtime sibling, never an ancestor of the
  # pseudo-terminal shell. The clone keeps every real runtime signature so
  # the agent assertions exercise the same signature set as production.
  proof_grade_holder_command=$tmp/metasystem-fake-agent
  cat >"$proof_grade_holder_command" <<'PROOF_GRADE_HOLDER_COMMAND'
#!/bin/bash
exec -a metasystem-fake-agent /bin/sleep "$1"
PROOF_GRADE_HOLDER_COMMAND
  chmod +x "$proof_grade_holder_command"
  proof_grade_holder_pid_file=$tmp/proof-grade-holder.pid
  proof_grade_holder_script=$tmp/proof-grade-holder.sh
  cat >"$proof_grade_holder_script" <<'PROOF_GRADE_HOLDER'
#!/usr/bin/env bash
set -euo pipefail
trap '' HUP
printf '%s\n' "$$" >"$PROOF_GRADE_HOLDER_PID_FILE"
exec "$PROOF_GRADE_HOLDER_COMMAND" 300
PROOF_GRADE_HOLDER
  chmod +x "$proof_grade_holder_script"
  export PROOF_GRADE_HOLDER_COMMAND=$proof_grade_holder_command
  export PROOF_GRADE_HOLDER_PID_FILE=$proof_grade_holder_pid_file
  proof_grade_holder_keeper_pid_file=$tmp/proof-grade-input-keeper.pid
  proof_grade_holder_keeper_script=$tmp/proof-grade-input-keeper.sh
  cat >"$proof_grade_holder_keeper_script" <<'PROOF_GRADE_KEEPER'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$$" >"$PROOF_GRADE_KEEPER_PID_FILE"
exec -a proof-grade-input-keeper /bin/sleep 600
PROOF_GRADE_KEEPER
  chmod +x "$proof_grade_holder_keeper_script"
  export PROOF_GRADE_KEEPER_PID_FILE=$proof_grade_holder_keeper_pid_file
  case "$(uname -s)" in
    Darwin) /bin/bash "$proof_grade_holder_keeper_script" | /usr/bin/script -q /dev/null /bin/bash "$proof_grade_holder_script" >"$tmp/proof-grade-holder.log" 2>&1 & ;;
    Linux) /bin/bash "$proof_grade_holder_keeper_script" | /usr/bin/script -q --return -c "exec /bin/bash '$proof_grade_holder_script'" /dev/null >"$tmp/proof-grade-holder.log" 2>&1 & ;;
    *) echo "proof-grade fixture needs a platform with the script pseudo-terminal utility" >&2; exit 1 ;;
  esac
  proof_grade_holder_session_pid=$!
  proof_grade_holder_session_start=$("$ms" proc started-at --pid "$proof_grade_holder_session_pid" 2>/dev/null || true)
  [[ -n "$proof_grade_holder_session_start" ]] || {
    echo "proof-grade terminal session is not alive with a proven start time" >&2
    cat "$tmp/proof-grade-holder.log" >&2
    exit 1
  }
  proof_grade_holder_wait_cap=$(harness_fixture_cap mission-process-wait)
  proof_grade_holder_wait_started=$(date +%s)
  while [[ ! -s "$proof_grade_holder_keeper_pid_file" ]] &&
      kill -0 "$proof_grade_holder_session_pid" 2>/dev/null &&
      (( $(date +%s) - proof_grade_holder_wait_started < proof_grade_holder_wait_cap )); do
    sleep 0.1
  done
  [[ -s "$proof_grade_holder_keeper_pid_file" ]] || {
    echo "proof-grade input keeper did not start within ${proof_grade_holder_wait_cap}s" >&2
    cat "$tmp/proof-grade-holder.log" >&2
    exit 1
  }
  proof_grade_holder_keeper_pid=$(cat "$proof_grade_holder_keeper_pid_file")
  proof_grade_holder_keeper_start=$("$ms" proc started-at --pid "$proof_grade_holder_keeper_pid" 2>/dev/null || true)
  [[ -n "$proof_grade_holder_keeper_start" ]] || {
    echo "proof-grade input keeper is not alive with a proven start time" >&2
    cat "$tmp/proof-grade-holder.log" >&2
    exit 1
  }
  while [[ ! -s "$proof_grade_holder_pid_file" ]] &&
      kill -0 "$proof_grade_holder_session_pid" 2>/dev/null &&
      (( $(date +%s) - proof_grade_holder_wait_started < proof_grade_holder_wait_cap )); do
    sleep 0.1
  done
  [[ -s "$proof_grade_holder_pid_file" ]] || {
    echo "proof-grade holder did not start within ${proof_grade_holder_wait_cap}s" >&2
    cat "$tmp/proof-grade-holder.log" >&2
    exit 1
  }
  proof_grade_holder_pid=$(cat "$proof_grade_holder_pid_file")
  proof_grade_holder_ready=
  while (( $(date +%s) - proof_grade_holder_wait_started < proof_grade_holder_wait_cap )); do
    proof_grade_holder_start=$("$ms" proc started-at --pid "$proof_grade_holder_pid" 2>/dev/null || true)
    proof_grade_holder_probe=$("$ms" proc probe --pid "$proof_grade_holder_pid" 2>/dev/null || true)
    proof_grade_holder_liveness=$("$ms" json get --value "$proof_grade_holder_probe" --field liveness 2>/dev/null || true)
    proof_grade_holder_terminal=$("$ms" json get --value "$proof_grade_holder_probe" --field terminalId --default "" 2>/dev/null || true)
    proof_grade_holder_terminal_known=$("$ms" json get --value "$proof_grade_holder_probe" --field terminalKnown 2>/dev/null || true)
    proof_grade_holder_session_leader=$("$ms" json get --value "$proof_grade_holder_probe" --field sessionLeaderPid --default 0 2>/dev/null || true)
    if [[ -n "$proof_grade_holder_start" && "$proof_grade_holder_liveness" == alive &&
        "$proof_grade_holder_terminal_known" == true && -n "$proof_grade_holder_terminal" &&
        "$proof_grade_holder_session_leader" == "$proof_grade_holder_pid" &&
        "$proof_grade_holder_probe" == *metasystem-fake-agent* ]]; then
      proof_grade_holder_ready=yes
      break
    fi
    sleep 0.1
  done
  if [[ -z "$proof_grade_holder_ready" ]]; then
    echo "proof-grade holder is not the live leader of its own controlling-terminal session: $proof_grade_holder_probe" >&2
    cat "$tmp/proof-grade-holder.log" >&2
    exit 1
  fi
  "$ms" lease announce --root "$clone" --session proof-grade-holder \
    --pid "$proof_grade_holder_pid" --start "$proof_grade_holder_start" \
    --tag proof-grade-holder --runtime fake --owner-lineage proof-grade-holder >/dev/null
  mkdir -p "$tmp/agent-shell"
  cat >"$tmp/agent-shell/metasystem-fake-agent" <<'PROOF_GRADE_AGENT_SHELL'
#!/bin/bash
exec -a metasystem-fake-agent /bin/bash "$@"
PROOF_GRADE_AGENT_SHELL
  chmod +x "$tmp/agent-shell/metasystem-fake-agent"
  export PROOF_GRADE_MS=$ms
  export PROOF_GRADE_CLONE=$clone
  export PROOF_GRADE_ARC_CLONE=$proof_grade_arc_clone
  export PROOF_GRADE_AGENT_SHELL=$tmp/agent-shell/metasystem-fake-agent
  export PROOF_GRADE_LOG_ROOT=$tmp/proof-grades
  export PROOF_GRADE_SCENARIO_LOG=$tmp/proof-grades/scenario.log
  export PROOF_GRADE_STATUS=$tmp/proof-grades/status
  mkdir -p "$PROOF_GRADE_LOG_ROOT"

  proof_grade_headless_script=$tmp/proof-grade-headless.sh
  cat >"$proof_grade_headless_script" <<'PROOF_GRADE_HEADLESS'
#!/usr/bin/env bash
set -euo pipefail
if { : </dev/tty; } 2>/dev/null; then
  echo "headless fixture process unexpectedly has a controlling terminal" >&2
  exit 1
fi
# Human authority starts at the CLI's real parent, so keep the detached shell
# alive until the command returns instead of replacing it with the CLI.
set +e
"$PROOF_GRADE_MS" goal release --root "$PROOF_GRADE_CLONE" --id ship-widget --by Wido
headless_release_rc=$?
exit "$headless_release_rc"
PROOF_GRADE_HEADLESS
  chmod +x "$proof_grade_headless_script"
  set +e
  "$ms" proc setsid -- \
    /bin/bash "$proof_grade_headless_script" </dev/null >"$PROOF_GRADE_LOG_ROOT/headless.log" 2>&1
  proof_grade_headless_rc=$?
  set -e
  if [[ $proof_grade_headless_rc -eq 0 ]] ||
      ! grep -q 'TERMINAL_NOT_REACHED' "$PROOF_GRADE_LOG_ROOT/headless.log"; then
    echo "headless human-word caller did not receive the terminal-not-reached refusal" >&2
    cat "$PROOF_GRADE_LOG_ROOT/headless.log" >&2
    exit 1
  fi

  proof_grade_script=$tmp/proof-grade-human.sh
  cat >"$proof_grade_script" <<'PROOF_GRADES'
#!/usr/bin/env bash
set -euo pipefail
unset METASYSTEM_OWNER_LINEAGE
: >"$PROOF_GRADE_SCENARIO_LOG"
exec >>"$PROOF_GRADE_SCENARIO_LOG" 2>&1
proof_grade_finish() {
  local status=$?
  printf '%s\n' "$status" >"$PROOF_GRADE_STATUS"
  if [[ $status -ne 0 && ! -s "$PROOF_GRADE_SCENARIO_LOG" ]]; then
    printf 'proof-grade assertions failed before producing a diagnostic\n' >>"$PROOF_GRADE_SCENARIO_LOG"
  fi
}
trap proof_grade_finish EXIT
printf 'proof-grade assertions started\n'

agent_refuses() { # label, expected refusal, command...
  local label=$1 expected=$2 rc
  shift 2
  set +e
  "$PROOF_GRADE_AGENT_SHELL" -c '"$@"; rc=$?; exit "$rc"' agent-shell "$@" \
    >"$PROOF_GRADE_LOG_ROOT/$label.log" 2>&1
  rc=$?
  set -e
  if [[ $rc -eq 0 ]]; then
    echo "agent shell was allowed to $label" >&2
    cat "$PROOF_GRADE_LOG_ROOT/$label.log" >&2
    exit 1
  fi
  if ! grep -Eq "$expected" "$PROOF_GRADE_LOG_ROOT/$label.log"; then
    echo "agent shell $label failed without its exact authority refusal" >&2
    cat "$PROOF_GRADE_LOG_ROOT/$label.log" >&2
    exit 1
  fi
}

human_runs() { # label, command...
  local label=$1
  shift
  if ! "$@" >"$PROOF_GRADE_LOG_ROOT/$label.log" 2>&1; then
    echo "unenrolled human terminal could not $label" >&2
    cat "$PROOF_GRADE_LOG_ROOT/$label.log" >&2
    exit 1
  fi
}

human_refuses() { # label, expected refusal, command...
  local label=$1 expected=$2 rc
  shift 2
  set +e
  "$@" >"$PROOF_GRADE_LOG_ROOT/$label.log" 2>&1
  rc=$?
  set -e
  if [[ $rc -eq 0 ]]; then
    echo "agent-descended terminal was allowed to $label" >&2
    cat "$PROOF_GRADE_LOG_ROOT/$label.log" >&2
    exit 1
  fi
  if ! grep -Eq "$expected" "$PROOF_GRADE_LOG_ROOT/$label.log"; then
    echo "agent-descended terminal $label failed without its exact authority refusal" >&2
    cat "$PROOF_GRADE_LOG_ROOT/$label.log" >&2
    exit 1
  fi
}

authority_refusal='AGENT_IN_AUTHORITY_CHAIN: [[:alnum:]_-]+'
agent_refuses release "$authority_refusal" "$PROOF_GRADE_MS" goal release --root "$PROOF_GRADE_CLONE" \
  --id ship-widget --by Wido --lineage agent-shell
agent_refuses park "$authority_refusal" "$PROOF_GRADE_MS" goal park --root "$PROOF_GRADE_CLONE" \
  --id ship-widget --because "agent stop" --by Wido --lineage agent-shell
agent_refuses release-arc "$authority_refusal" "$PROOF_GRADE_MS" goal release --root "$PROOF_GRADE_ARC_CLONE" \
  --id ship-widget --by Wido --lineage agent-shell --arc proof-grade
agent_refuses park-arc "$authority_refusal" "$PROOF_GRADE_MS" goal park --root "$PROOF_GRADE_ARC_CLONE" \
  --id ship-widget --because "agent arc stop" --by Wido --lineage agent-shell --arc proof-grade
agent_refuses unpark-arc "$authority_refusal" "$PROOF_GRADE_MS" goal unpark --root "$PROOF_GRADE_ARC_CLONE" \
  --id ship-widget --by Wido --lineage agent-shell --arc proof-grade
agent_refuses approve "$authority_refusal" "$PROOF_GRADE_MS" goal approve --root "$PROOF_GRADE_CLONE" \
  --id fix-docs --budget box --by Wido --lineage agent-shell
agent_refuses session-stop 'caller classifies DELEGATE' "$PROOF_GRADE_MS" session stop --root "$PROOF_GRADE_CLONE" --by Wido

if [[ "$PROOF_GRADE_BED_CLASS" == HUMAN ]]; then
  human_runs release "$PROOF_GRADE_MS" goal release --root "$PROOF_GRADE_CLONE" \
    --id ship-widget --by Wido
  human_runs park "$PROOF_GRADE_MS" goal park --root "$PROOF_GRADE_CLONE" \
    --id ship-widget --because "human stop" --by Wido
  agent_refuses unpark "$authority_refusal" "$PROOF_GRADE_MS" goal unpark --root "$PROOF_GRADE_CLONE" \
    --id ship-widget --by Wido --lineage agent-shell
  human_runs unpark-to-queued "$PROOF_GRADE_MS" goal unpark --root "$PROOF_GRADE_CLONE" \
    --id ship-widget --by Wido
  human_runs release-arc "$PROOF_GRADE_MS" goal release --root "$PROOF_GRADE_ARC_CLONE" \
    --id ship-widget --by Wido --arc proof-grade
  human_runs park-arc "$PROOF_GRADE_MS" goal park --root "$PROOF_GRADE_ARC_CLONE" \
    --id ship-widget --because "human arc stop" --by Wido --arc proof-grade
  human_runs unpark-arc-to-queued "$PROOF_GRADE_MS" goal unpark --root "$PROOF_GRADE_ARC_CLONE" \
    --id ship-widget --by Wido --arc proof-grade

  release_journal=
  for journal in "$PROOF_GRADE_CLONE"/artifacts/agents/goal-transactions/*.json; do
    [[ -f "$journal" ]] || continue
    if grep -q '"verb": "release"' "$journal" && grep -q '"lineage": "terminal-.*-0"' "$journal"; then
      release_journal=$journal
      break
    fi
  done
  [[ -n "$release_journal" ]] || {
    echo "the human release journal did not record its derived terminal-grade lineage" >&2
    exit 1
  }
  derived_terminal_lineage=$(sed -n 's/^[[:space:]]*"lineage": "\(terminal-[^"]*-0\)",*$/\1/p' "$release_journal")
  [[ -n "$derived_terminal_lineage" ]] || {
    echo "the human release journal carried no terminal-<id>-0 lineage" >&2
    exit 1
  }
  for verb in release park unpark; do
    recorded=
    for journal in "$PROOF_GRADE_CLONE"/artifacts/agents/goal-transactions/*.json; do
      [[ -f "$journal" ]] || continue
      if grep -q "\"verb\": \"$verb\"" "$journal" &&
          grep -q "\"lineage\": \"$derived_terminal_lineage\"" "$journal"; then
        recorded=yes
        break
      fi
    done
    [[ -n "$recorded" ]] || {
      echo "the human $verb journal did not record derived lineage $derived_terminal_lineage" >&2
      exit 1
    }
  done

  set +e
  "$PROOF_GRADE_MS" goal approve --root "$PROOF_GRADE_CLONE" --id fix-docs \
    --budget box --by Wido >"$PROOF_GRADE_LOG_ROOT/approve-human.log" 2>&1
  approve_rc=$?
  set -e
  if [[ $approve_rc -eq 0 ]] || ! grep -q 'TERMINAL_NOT_ENROLLED' "$PROOF_GRADE_LOG_ROOT/approve-human.log"; then
    echo "unenrolled human approval did not name its enrolled-grade refusal" >&2
    cat "$PROOF_GRADE_LOG_ROOT/approve-human.log" >&2
    exit 1
  fi

  human_runs session-stop "$PROOF_GRADE_MS" session stop --root "$PROOF_GRADE_CLONE" --by Wido
else
  printf "proof-grade allow path was not proven in this agent-descended bed; prove it at an agent-free terminal or in the steward's scheduled run\n"
  human_refuses release-human "$authority_refusal" "$PROOF_GRADE_MS" goal release --root "$PROOF_GRADE_CLONE" \
    --id ship-widget --by Wido
  human_refuses park-human "$authority_refusal" "$PROOF_GRADE_MS" goal park --root "$PROOF_GRADE_CLONE" \
    --id ship-widget --because "human stop" --by Wido
  human_refuses release-arc-human "$authority_refusal" "$PROOF_GRADE_MS" goal release --root "$PROOF_GRADE_ARC_CLONE" \
    --id ship-widget --by Wido --arc proof-grade
  human_refuses park-arc-human "$authority_refusal" "$PROOF_GRADE_MS" goal park --root "$PROOF_GRADE_ARC_CLONE" \
    --id ship-widget --because "human arc stop" --by Wido --arc proof-grade
  human_refuses unpark-arc-human "$authority_refusal" "$PROOF_GRADE_MS" goal unpark --root "$PROOF_GRADE_ARC_CLONE" \
    --id ship-widget --by Wido --arc proof-grade
  agent_refuses unpark "$authority_refusal" "$PROOF_GRADE_MS" goal unpark --root "$PROOF_GRADE_CLONE" \
    --id perf-pass --by Wido --lineage agent-shell
  human_refuses unpark-to-queued "$authority_refusal" "$PROOF_GRADE_MS" goal unpark --root "$PROOF_GRADE_CLONE" \
    --id perf-pass --by Wido
  human_refuses approve-human "$authority_refusal" "$PROOF_GRADE_MS" goal approve --root "$PROOF_GRADE_CLONE" \
    --id fix-docs --budget box --by Wido
  human_refuses session-stop-human 'caller classifies DELEGATE' "$PROOF_GRADE_MS" session stop \
    --root "$PROOF_GRADE_CLONE" --by Wido
fi
PROOF_GRADES
  chmod +x "$proof_grade_script"
  set +e
  case "$(uname -s)" in
    Darwin) /usr/bin/script -q /dev/null /bin/bash "$proof_grade_script" >"$tmp/proof-grade-assertions.pty.log" 2>&1 ;;
    Linux) /usr/bin/script -q --return -c "exec /bin/bash '$proof_grade_script'" /dev/null >"$tmp/proof-grade-assertions.pty.log" 2>&1 ;;
    *) echo "proof-grade fixture needs a platform with the script pseudo-terminal utility" >&2; exit 1 ;;
  esac
  proof_grade_script_rc=$?
  set -e
  proof_grade_reported_rc=
  [[ -f "$PROOF_GRADE_STATUS" ]] && proof_grade_reported_rc=$(cat "$PROOF_GRADE_STATUS")
  if [[ $proof_grade_script_rc -ne 0 || ! "$proof_grade_reported_rc" =~ ^[0-9]+$ || "$proof_grade_reported_rc" -ne 0 ]]; then
    if [[ ! -s "$PROOF_GRADE_SCENARIO_LOG" ]]; then
      echo "proof-grade assertion failure produced no scenario diagnostic" >&2
      cat "$tmp/proof-grade-assertions.pty.log" >&2
      exit 1
    fi
    cat "$PROOF_GRADE_SCENARIO_LOG" >&2
    exit 1
  fi
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
  "$ms" goal open --root "$clone" --id brain-approved-one --origin human \
    --intent "Wait for the first node." --next "A node claims this." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "brain stop fixture" >/dev/null
  approve_fixture_goal brain-approved-one --budget box
  "$ms" goal open --root "$clone" --id brain-approved-two --origin human \
    --intent "Wait for a node." --next "A node claims this." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "brain stop fixture" >/dev/null
  approve_fixture_goal brain-approved-two --budget box
  METASYSTEM_OWNER_LINEAGE=earlier-brain-lineage "$ms" goal open --root "$clone" --id brain-draft --origin human \
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

if [[ "$fixture_scenario" == abandoned-with-a-reason ]]; then
  "$ms" goal release --root "$clone" --id ship-widget >/dev/null
  export METASYSTEM_SUPERVISION_REGISTRY_HOME="$tmp/abandon-registry"
  mkdir -p "$METASYSTEM_SUPERVISION_REGISTRY_HOME"
  export METASYSTEM_GOAL_NOW=2026-08-20T00:00:00Z
  "$ms" goal open --root "$clone" --id abandon-b --origin human \
    --intent "Stay blocked until the abandoned work is re-pointed." --next "Wait for the successor." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "abandon fixture" >/dev/null
  approve_fixture_goal abandon-b --budget box
  "$ms" goal claim --root "$clone" --id abandon-b >/dev/null
  "$ms" goal open --root "$clone" --id abandon-a --origin main --blocks abandon-b \
    --intent "Retire work that no longer belongs in the backlog." --next "Prove abandonment." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "abandon fixture" >/dev/null
  "$ms" goal edit --root "$clone" --id abandon-a --tier 1 \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "abandon fixture member" >/dev/null
  approve_fixture_goal abandon-a \
    --elapsed-limit 1m --attempt-limit 2 --reserved-job-minutes-limit 20 --active-job-limit 1 --review-round-limit 0
  "$ms" goal claim --root "$clone" --id abandon-a >/dev/null
  export METASYSTEM_GOAL_NOW=2026-08-20T00:02:00Z
  "$ms" job breach-stop --root "$clone" --goal abandon-a --revision 4 >"$tmp/abandon-stop.json"
  if release_refusal=$("$ms" goal release --root "$clone" --id abandon-a 2>&1); then
    echo "release cleared a breach-stopped claim" >&2; exit 1
  fi
  grep -q 'only goal resume may clear its launch fence' <<<"$release_refusal" \
    || { echo "release did not preserve the launch fence: $release_refusal" >&2; exit 1; }

  if floor_refusal=$("$ms" goal abandon --root "$clone" --id abandon-a --by Wido --because fixture --fixture-human-authority 2>&1); then
    echo "abandon succeeded without an engine floor" >&2; exit 1
  fi
  grep -q 'ledger has no record that the fleet runs it' <<<"$floor_refusal" \
    || { echo "abandon did not name the missing fleet floor: $floor_refusal" >&2; exit 1; }
  engine_status=$("$ms" supervise status --repo "$clone")
  engine_stamp=$("$ms" json get --value "$engine_status" --field engineBuild)
  if [[ "$engine_stamp" =~ ^dev-([0-9a-f]{40})-dirty$ ]]; then
    engine_commit=${BASH_REMATCH[1]}
  elif [[ "$engine_stamp" =~ ^[0-9a-f]{40}$ ]]; then
    engine_commit=$engine_stamp
  else
    echo "fixture binary has no source-linked build stamp: $engine_stamp" >&2; exit 1
  fi
  git -C "$clone" fetch -q "$(git -C "$root" rev-parse --show-toplevel)" "$engine_commit"
  "$ms" goal engine-floor --root "$clone" --commit "$engine_commit" --by Wido --fixture-human-authority >/dev/null
  if dependent_refusal=$("$ms" goal abandon --root "$clone" --id abandon-a --by Wido --because fixture --fixture-human-authority 2>&1); then
    echo "abandon succeeded with an uncovered dependent" >&2; exit 1
  fi
  grep -q 'goal abandon-b is blocked by abandon-a' <<<"$dependent_refusal" \
    || { echo "abandon did not name the uncovered dependent: $dependent_refusal" >&2; exit 1; }

  "$ms" goal open --root "$clone" --id abandon-successor --origin human \
    --intent "Carry the still-live constraint." --next "Complete the successor." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "abandon fixture" >/dev/null
	done_before=$("$ms" goal list --root "$clone" | sed -n '1s/.* done=\([0-9][0-9]*\) .*/\1/p')
  "$ms" goal abandon --root "$clone" --id abandon-a --by Wido --because fixture \
    --carried abandon-successor --fixture-human-authority >/dev/null
  abandon_tip=$(git -C "$origin" rev-parse main)
  git -C "$clone" cat-file -p "$abandon_tip:records/goals/abandon-a.md" >"$tmp/abandon-a.md"
  grep -q '^- State: abandoned$' "$tmp/abandon-a.md" \
    && grep -q '^- Abandoned: by=human:Wido .*stopId=.* carried=abandon-successor because=fixture$' "$tmp/abandon-a.md" \
    && grep -q '^- StopFence:' "$tmp/abandon-a.md" \
    || { echo "the abandoned stopped record is incomplete" >&2; cat "$tmp/abandon-a.md" >&2; exit 1; }
  if git -C "$clone" cat-file -e "$abandon_tip:plans/goals/abandon-a.md" 2>/dev/null; then
    echo "the abandoned goal remained in the live path" >&2; exit 1
  fi
	summary=$("$ms" goal list --root "$clone")
	grep -q "^.* done=$done_before abandoned=1 tip=" <<<"$summary" \
		|| { echo "listing conflated done and abandoned: $summary" >&2; exit 1; }
	listing=$("$ms" goal list --root "$clone" --json)
  abandoned_json=$("$ms" json get --value "$listing" --field abandoned)
  done_json=$("$ms" json get --value "$listing" --field done)
  [[ "$abandoned_json" == *'"Id":"abandon-a"'* && "$done_json" != *'"Id":"abandon-a"'* ]] \
    || { echo "JSON listing omitted or conflated the abandoned id: $listing" >&2; exit 1; }
  next=$("$ms" goal next --root "$clone")
  [[ "$next" != "next ready goal: abandon-a" && "$next" != "next ready goal: abandon-b" ]] \
    || { echo "goal next offered abandoned or blocked work: $next" >&2; exit 1; }
  shown=$("$ms" goal show --root "$clone" --id abandon-a)
  [[ "$shown" == *'"Because":"fixture"'* ]] \
    || { echo "goal show omitted the abandon reason: $shown" >&2; exit 1; }
  cat >"$tmp/abandon-split.md" <<'DRAFT'
# split abandon-a

## member abandon-a-one
- Intent: First impossible split member.
- Next step: Never run.

## member abandon-a-two
- Intent: Second impossible split member.
- Next step: Never run.
DRAFT
  if split_refusal=$("$ms" goal split --root "$clone" --id abandon-a --members "$tmp/abandon-split.md" 2>&1); then
    echo "an abandoned goal split" >&2; exit 1
  fi
  grep -q 'in the archive; there is nothing to split' <<<"$split_refusal" \
    || { echo "split did not recognize the abandoned archive: $split_refusal" >&2; exit 1; }
  "$ms" goal prune --root "$clone" --keep 0 >/dev/null
  prune_tip=$(git -C "$origin" rev-parse main)
  git -C "$clone" cat-file -e "$prune_tip:records/goals/abandon-a.md"
  unset METASYSTEM_GOAL_NOW
  exit 0
fi

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
  "$ms" goal open --root "$clone" --id brain-claim-target --origin human \
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
  "$ms" goal open --root "$clone" --id classification-target --origin human \
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

  # Every row carries the human's word as --by, which is the carrier the brain
  # fence refuses at goalsync_mutations.go:697. Three rows used to add
  # --temporary-human-word so the relayed-word carrier was covered too. That
  # carrier is unreachable today: authority.go:318 refuses a --review-by past
  # the 2026-09-06 horizon while the verb refuses one already in the past, so
  # no date exists that both accept. --fixture-human-authority is NOT a
  # substitute for it. It sets FixtureOnly, the one input line 698 exempts from
  # the fence, so a row carrying it demands a refusal from an input designed
  # never to refuse and then reports a pass because nothing refused. Found by
  # m1c. Restore the relayed-word coverage the day the horizon moves, by
  # appending --temporary-human-word "..." --review-by <date inside it>.
  assert_human_word_matrix() { # expected text
    local expected=$1
    assert_brain_goal_refusal "$expected" "$ms" goal approve --root "$clone" --id fix-docs --by Wido --budget box
    assert_brain_goal_refusal "$expected" "$ms" goal resume --root "$clone" --id fix-docs --by Wido \
      --elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3
    assert_brain_goal_refusal "$expected" "$ms" goal resume --root "$clone" --id fix-docs --by Wido \
      --elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3 \
      --approved-ref fixture-answer
    assert_brain_goal_refusal "$expected" "$ms" goal set-obligation --root "$clone" --id fix-docs --by Wido \
      --state LIMITED --owner Wido --recurrence single-experiment --platform darwin-arm64 \
      --toolchain-identity go-fixture --surface-digest fixture-surface --max-active-jobs 1 --timing-envelope-sec 60 \
      --effect local-write --value-judgment no --reversibility reversible --severe-harm no \
      --unfamiliar-approach no --test-discrimination strong --correlated-assumption-risk no \
      --authority-scope-change no --destructive-reach reversible-local
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

  "$ms" goal open --root "$clone" --id brain-fixture-authority-target --origin human \
    --intent "Prove fixture human authority crosses the brain seam." --next "Approve this classified goal." \
    --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "fixture human-authority positive path" >/dev/null
  fixture_approval=$("$ms" goal approve --root "$clone" --id brain-fixture-authority-target \
    --by Wido --budget box --fixture-human-authority)
  [[ "$fixture_approval" == *'"outcome":"confirmed"'* ]] || {
    echo "fixture-only human authority did not cross the brain seam: $fixture_approval" >&2
    exit 1
  }

  "$ms" goal open --root "$clone" --id brain-channel-target --origin human \
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
"$ms" goal open --root "$clone" --id explicit-lineage-approval --origin human \
  --intent "Record an approval under the fixture coordinator." --next "Compare its operation identity." \
  --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture identity comparison" >/dev/null
"$ms" goal open --root "$clone" --id derived-lineage-approval --origin human \
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

case "$fixture_scenario" in
  forgiving-*) export PATH="$(dirname "$ms"):$PATH" ;;
esac

open_forgiving_fixture_goal() { # goal id, optional goal-open flags
  local goal_id=$1
  shift
  "$ms" goal open --root "$clone" --id "$goal_id" \
    --intent "Exercise the forgiving budget command for $goal_id." --next "Inspect the recorded box." \
    --origin human --by Wido --fixture-human-authority \
    --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" "$@" >/dev/null
}

write_forgiving_fixture_enrollment() { # optional human name
  local human=${1:-} human_line=
  [[ -z "$human" ]] || human_line="  \"human\": \"$human\","$'\n'
  mkdir -p "$clone/artifacts/agents/authority"
  {
    printf '%s\n' '{' '  "schema": 1,' '  "enrolledAt": "2026-09-01T09:00:00Z",' '  "generation": 1,'
    printf '%s' "$human_line"
    printf '%s\n' '  "terminalId": "fixture-terminal",' \
      '  "terminalRef": {"pid": 20, "pidStartedAt": 200},' \
      '  "sessionLeaderRef": {"pid": 10, "pidStartedAt": 100}' '}'
  } >"$clone/artifacts/agents/authority/human-terminal.json"
}

read_forgiving_goal() { # goal id, destination
  local goal_id=$1 destination=$2 tip
  tip=$(git -C "$origin" rev-parse main)
  git -C "$clone" cat-file -p "$tip:plans/goals/$goal_id.md" >"$destination"
}

run_forgiving_refusal_remedy() { # label, goal id, twin id, long-form command..., --refusal, refused command...
	local label=$1 goal_id=$2 twin_id=$3 output_file=$tmp/forgiving-refusal.out error_file=$tmp/forgiving-refusal.err
	local remedy rc goal_budget twin_budget goal_history twin_history
	local -a long_form=()
	shift 3
	while [[ $# -gt 0 && $1 != --refusal ]]; do
		long_form+=("$1")
		shift
	done
	[[ $# -gt 0 ]] || { echo "$label did not separate its long-form and refused commands" >&2; exit 1; }
	[[ ${#long_form[@]} -gt 0 ]] || { echo "$label did not name a long-form twin command" >&2; exit 1; }
	shift
	set +e
	"$@" >"$output_file" 2>"$error_file"
  rc=$?
  set -e
  [[ $rc -ne 0 ]] || { echo "$label did not refuse" >&2; exit 1; }
	[[ $(wc -l <"$error_file" | tr -d ' ') -eq 2 ]] \
		|| { echo "$label did not print exactly two refusal lines" >&2; cat "$error_file" >&2; exit 1; }
	remedy=$(sed -n 's/^run: //p' "$error_file")
	[[ -n "$remedy" ]] || { echo "$label did not print a runnable remedy" >&2; cat "$error_file" >&2; exit 1; }
	eval "$remedy" >/dev/null \
		|| { echo "$label printed a command that did not complete" >&2; cat "$error_file" >&2; exit 1; }
	read_forgiving_goal "$goal_id" "$tmp/$goal_id-remedy.md"
	# The guard above makes the empty case unreachable. The + idiom stays only
	# because oldest-bash-gate.sh is flow-insensitive and flags the plain form.
	${long_form[@]+"${long_form[@]}"} >/dev/null \
		|| { echo "$label's long-form twin command did not complete" >&2; exit 1; }
	read_forgiving_goal "$twin_id" "$tmp/$twin_id-long-form.md"
	goal_budget=$(sed -n 's/^- Budget: //p' "$tmp/$goal_id-remedy.md")
	twin_budget=$(sed -n 's/^- Budget: //p' "$tmp/$twin_id-long-form.md")
	goal_history=$(awk '/^History:$/ { in_history=1; next } in_history && /^- / { value=$4 " " $5 } END { print value }' "$tmp/$goal_id-remedy.md")
	twin_history=$(awk '/^History:$/ { in_history=1; next } in_history && /^- / { value=$4 " " $5 } END { print value }' "$tmp/$twin_id-long-form.md")
	[[ -n "$goal_budget" && "$goal_budget" == "$twin_budget" && -n "$goal_history" && "$goal_history" == "$twin_history" ]] \
		|| { echo "$label's remedy did not match its long-form twin" >&2; diff -u "$tmp/$twin_id-long-form.md" "$tmp/$goal_id-remedy.md" >&2 || true; exit 1; }
}

release_forgiving_claim_if_present() { # goal id
	local goal_id=$1 tip record
	record=$tmp/$goal_id-release-check.md
	tip=$(git -C "$origin" rev-parse main)
	if git -C "$clone" cat-file -e "$tip:plans/goals/$goal_id.md" 2>/dev/null; then
		git -C "$clone" cat-file -p "$tip:plans/goals/$goal_id.md" >"$record"
		if grep -q '^- State: claimed$' "$record"; then
			"$ms" goal release --root "$clone" --id "$goal_id" >/dev/null
		fi
	fi
}

claim_forgiving_goal() { # goal id
	local goal_id=$1 claim_output
	claim_output=$("$ms" goal claim --root "$clone" --id "$goal_id" 2>&1) \
		|| { echo "claiming $goal_id failed: $claim_output" >&2; return 1; }
	grep -q '"outcome":"confirmed"' <<<"$claim_output" \
		|| { echo "claiming $goal_id did not confirm: $claim_output" >&2; return 1; }
}

breach_stop_forgiving_goal() { # goal id
	local goal_id=$1 revision breach_stop breach_stop_id record
	record=$tmp/$goal_id-before-stop.md
	read_forgiving_goal "$goal_id" "$record"
	revision=$(sed -n 's/^- Claimed: .* revision=\([0-9][0-9]*\).*/\1/p' "$record")
	[[ -n "$revision" ]] \
		|| { echo "$goal_id has no claimed revision for its breach-stop" >&2; cat "$record" >&2; return 1; }
	breach_stop=$("$ms" job breach-stop --root "$clone" --goal "$goal_id" --revision "$revision")
	breach_stop_id=$(sed -n 's/.*"stopId":"\([^"]*\)".*/\1/p' <<<"$breach_stop")
	[[ -n "$breach_stop_id" ]] \
		|| { echo "breach-stop did not print a stop identifier for $goal_id: $breach_stop" >&2; return 1; }
	"$ms" job stop-batch-reconcile --root "$clone" --stop "$breach_stop_id" >/dev/null
}

if [[ "$fixture_scenario" == forgiving-budget-states ]]; then
write_forgiving_fixture_enrollment Wido

# A queued goal accepts the token after its flags and re-approval changes only
# its approval tuple. The parked transaction keeps the pause intact.
"$ms" goal budget --root "$clone" --id fix-docs --fixture-human-authority 2h/4/240m/1/2 >/dev/null
read_forgiving_goal fix-docs "$tmp/fix-docs-approved.md"
grep -q '^- State: approved$' "$tmp/fix-docs-approved.md" \
  && grep -q '^- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=2$' "$tmp/fix-docs-approved.md" \
  && grep -q ' approve actor=human:Wido ' "$tmp/fix-docs-approved.md" \
  || { echo "queued goal budget did not publish an approval" >&2; cat "$tmp/fix-docs-approved.md" >&2; exit 1; }
"$ms" goal budget 3h/5/300m/1/2 --root "$clone" --id fix-docs --by Wido --fixture-human-authority >/dev/null
read_forgiving_goal fix-docs "$tmp/fix-docs-reapproved.md"
[[ $(grep -c ' approve actor=human:Wido ' "$tmp/fix-docs-reapproved.md") -eq 2 ]] \
  && grep -q '^- Budget: elapsedLimit=3h attemptLimit=5 reservedJobMinutesLimit=300 activeJobLimit=1 reviewRoundLimit=2$' "$tmp/fix-docs-reapproved.md" \
  || { echo "approved goal budget did not re-approve" >&2; cat "$tmp/fix-docs-reapproved.md" >&2; exit 1; }

"$ms" goal budget --root "$clone" --id perf-pass 2h/4/240m/1/2 --by Wido --fixture-human-authority >/dev/null
read_forgiving_goal perf-pass "$tmp/perf-pass-parked.md"
grep -q '^- State: parked$' "$tmp/perf-pass-parked.md" \
  && grep -q '^- Parked: ' "$tmp/perf-pass-parked.md" \
  && grep -q '^- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=2$' "$tmp/perf-pass-parked.md" \
  && grep -q ' approve actor=human:Wido ' "$tmp/perf-pass-parked.md" \
  || { echo "parked goal budget moved or erased the park" >&2; cat "$tmp/perf-pass-parked.md" >&2; exit 1; }

# A claimed goal routes through set-budget and advances the claim binding.
read_forgiving_goal ship-widget "$tmp/ship-widget-before-budget.md"
claim_revision_before=$(sed -n 's/^- Claimed: .* revision=\([0-9][0-9]*\).*/\1/p' "$tmp/ship-widget-before-budget.md")
"$ms" goal budget --root "$clone" --id ship-widget 4h/6/600m/1/2 --by Wido --fixture-human-authority >/dev/null
read_forgiving_goal ship-widget "$tmp/ship-widget-after-budget.md"
claim_revision_after=$(sed -n 's/^- Claimed: .* revision=\([0-9][0-9]*\).*/\1/p' "$tmp/ship-widget-after-budget.md")
[[ "$claim_revision_after" -gt "$claim_revision_before" ]] \
  && grep -q ' set-budget actor=human:Wido ' "$tmp/ship-widget-after-budget.md" \
  || { echo "claimed goal budget did not rebind through set-budget" >&2; cat "$tmp/ship-widget-after-budget.md" >&2; exit 1; }

# The stop custodian closes the claimed revision; keep then routes through the
# standing resume transaction.
export METASYSTEM_GOAL_NOW=$forgiving_breach_at
breach_stop=$("$ms" job breach-stop --root "$clone" --goal ship-widget --revision "$claim_revision_after")
breach_stop_id=$(sed -n 's/.*"stopId":"\([^"]*\)".*/\1/p' <<<"$breach_stop")
[[ -n "$breach_stop_id" ]] || { echo "breach-stop did not print its stop identifier: $breach_stop" >&2; exit 1; }
"$ms" job stop-batch-reconcile --root "$clone" --stop "$breach_stop_id" >/dev/null
set +e
"$ms" goal budget --root "$clone" --id ship-widget keep --approved-ref rejected-on-resume --by Wido --fixture-human-authority \
	>"$tmp/stopped-approved-ref.out" 2>"$tmp/stopped-approved-ref.err"
stopped_approved_ref_rc=$?
set -e
stopped_approved_ref_remedy=$(sed -n 's/^run: //p' "$tmp/stopped-approved-ref.err")
[[ $stopped_approved_ref_rc -ne 0 && -n "$stopped_approved_ref_remedy" && "$stopped_approved_ref_remedy" != *"--approved-ref"* ]] \
	|| { echo "the stopped-goal keep remedy retained its rejected approval reference" >&2; cat "$tmp/stopped-approved-ref.err" >&2; exit 1; }
eval "$stopped_approved_ref_remedy" >/dev/null
read_forgiving_goal ship-widget "$tmp/ship-widget-resumed.md"
grep -q ' resume actor=human:Wido ' "$tmp/ship-widget-resumed.md" \
  && ! grep -q '^- StopFence:' "$tmp/ship-widget-resumed.md" \
  || { echo "keep did not resume the breach-stopped goal" >&2; cat "$tmp/ship-widget-resumed.md" >&2; exit 1; }

for refused_id in port-engine absent-goal; do
  set +e
  "$ms" goal budget --root "$clone" --id "$refused_id" norm --by Wido --fixture-human-authority \
    >"$tmp/$refused_id.out" 2>"$tmp/$refused_id.err"
  refused_rc=$?
  set -e
  [[ $refused_rc -ne 0 && $(wc -l <"$tmp/$refused_id.err" | tr -d ' ') -eq 2 ]] \
    && grep -q '^no command completes this:' "$tmp/$refused_id.err" \
    && ! grep -q '^run:' "$tmp/$refused_id.err" \
    || { echo "$refused_id did not use the terminal words refusal" >&2; cat "$tmp/$refused_id.err" >&2; exit 1; }
done
fi

if [[ "$fixture_scenario" == forgiving-budget-members ]]; then
write_forgiving_fixture_enrollment Wido

open_forgiving_fixture_goal norm-preset
"$ms" goal budget --root "$clone" --id norm-preset norm --fixture-human-authority >/dev/null
read_forgiving_goal norm-preset "$tmp/norm-preset.md"
grep -q '^- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3$' "$tmp/norm-preset.md" \
  || { echo "norm did not resolve to the tier-three fixture box" >&2; cat "$tmp/norm-preset.md" >&2; exit 1; }

open_forgiving_fixture_goal keep-preset \
  --elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2
"$ms" goal budget --root "$clone" --id keep-preset keep --fixture-human-authority >/dev/null
read_forgiving_goal keep-preset "$tmp/keep-preset.md"
grep -q '^- Budget: elapsedLimit=3h attemptLimit=5 reservedJobMinutesLimit=300 activeJobLimit=1 reviewRoundLimit=2$' "$tmp/keep-preset.md" \
  || { echo "keep did not preserve the opened box" >&2; cat "$tmp/keep-preset.md" >&2; exit 1; }

open_forgiving_fixture_goal empty-member \
  --elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2
"$ms" goal budget --root "$clone" --id empty-member 4h//// --fixture-human-authority >/dev/null
read_forgiving_goal empty-member "$tmp/empty-member.md"
grep -q '^- Budget: elapsedLimit=4h attemptLimit=5 reservedJobMinutesLimit=300 activeJobLimit=1 reviewRoundLimit=2$' "$tmp/empty-member.md" \
  || { echo "empty compact members did not inherit the standing limits" >&2; cat "$tmp/empty-member.md" >&2; exit 1; }

open_forgiving_fixture_goal keep-without-standing-twin
"$ms" goal budget --root "$clone" --id keep-without-standing-twin norm --by Wido --fixture-human-authority >/dev/null
"$ms" goal unapprove --root "$clone" --id keep-without-standing-twin --because "prepare a fieldless budget twin" --by Wido --fixture-human-authority >/dev/null
run_forgiving_refusal_remedy "keep without a standing box" fix-docs keep-without-standing-twin \
	"$ms" goal budget --root "$clone" --id keep-without-standing-twin --fixture-human-authority \
		--elapsed-limit 1d --attempt-limit 10 --reserved-job-minutes-limit 1200 --active-job-limit 1 --review-round-limit 3 \
	--refusal "$ms" goal budget --root "$clone" --id fix-docs keep --fixture-human-authority
read_forgiving_goal fix-docs "$tmp/keep-without-standing.md"
grep -q '^- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3$' "$tmp/keep-without-standing.md" \
  || { echo "the printed norm remedy did not approve the goal" >&2; cat "$tmp/keep-without-standing.md" >&2; exit 1; }

open_forgiving_fixture_goal four-members \
	--elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2
open_forgiving_fixture_goal four-members-twin \
	--elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2
run_forgiving_refusal_remedy "four-member compact box" four-members four-members-twin \
	"$ms" goal budget --root "$clone" --id four-members-twin --fixture-human-authority \
		--elapsed-limit 4h --attempt-limit 6 --reserved-job-minutes-limit 360 --active-job-limit 1 --review-round-limit 2 \
	--refusal "$ms" goal budget --root "$clone" --id four-members 4h/6/360m/1 --fixture-human-authority
open_forgiving_fixture_goal invalid-member \
	--elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2
open_forgiving_fixture_goal invalid-member-twin \
	--elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2
run_forgiving_refusal_remedy "invalid compact member" invalid-member invalid-member-twin \
	"$ms" goal budget --root "$clone" --id invalid-member-twin --fixture-human-authority \
		--elapsed-limit 4h --attempt-limit 5 --reserved-job-minutes-limit 360 --active-job-limit 1 --review-round-limit 2 \
	--refusal "$ms" goal budget --root "$clone" --id invalid-member 4h/many/360m/1/2 --fixture-human-authority
open_forgiving_fixture_goal mixed-box \
	--elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2
open_forgiving_fixture_goal mixed-box-twin \
	--elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2
run_forgiving_refusal_remedy "compact and long form together" mixed-box mixed-box-twin \
	"$ms" goal budget --root "$clone" --id mixed-box-twin --fixture-human-authority \
		--elapsed-limit 4h --attempt-limit 6 --reserved-job-minutes-limit 360 --active-job-limit 1 --review-round-limit 2 \
	--refusal "$ms" goal budget --root "$clone" --id mixed-box 4h/6/360m/1/2 --fixture-human-authority \
		--elapsed-limit 8h --attempt-limit 10 --reserved-job-minutes-limit 1200 --active-job-limit 1 --review-round-limit 3

open_forgiving_fixture_goal extra-member
open_forgiving_fixture_goal extra-member-twin
run_forgiving_refusal_remedy "extra compact member" extra-member extra-member-twin \
	"$ms" goal budget --root "$clone" --id extra-member-twin --fixture-human-authority \
		--elapsed-limit 4h --attempt-limit 6 --reserved-job-minutes-limit 360 --active-job-limit 1 --review-round-limit 2 \
	--refusal "$ms" goal budget --root "$clone" --id extra-member 4h/6/360m/1/2/ignored --fixture-human-authority

open_forgiving_fixture_goal over-round-limit
set +e
"$ms" goal budget --root "$clone" --id over-round-limit 4h/6/360m/1/4 --fixture-human-authority \
	>"$tmp/over-round-limit.out" 2>"$tmp/over-round-limit.err"
over_round_limit_rc=$?
set -e
[[ $over_round_limit_rc -ne 0 && $(wc -l <"$tmp/over-round-limit.err" | tr -d ' ') -eq 2 ]] \
	&& grep -q '^no command completes this: reviewRoundLimit 4 exceeds configured maximum 3' "$tmp/over-round-limit.err" \
	&& ! grep -q '^run:' "$tmp/over-round-limit.err" \
	|| { echo "an over-limit review-round member printed a retry command" >&2; cat "$tmp/over-round-limit.err" >&2; exit 1; }

open_forgiving_fixture_goal approved-completion \
	--elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2
"$ms" goal budget --root "$clone" --id approved-completion keep --fixture-human-authority >/dev/null
set +e
"$ms" goal budget --root "$clone" --id approved-completion 3h/5/300m/1 --fixture-human-authority \
	>"$tmp/approved-completion.out" 2>"$tmp/approved-completion.err"
approved_completion_rc=$?
set -e
[[ $approved_completion_rc -ne 0 && $(wc -l <"$tmp/approved-completion.err" | tr -d ' ') -eq 2 ]] \
	&& grep -q '^no command completes this: the goal already carries that box' "$tmp/approved-completion.err" \
	&& ! grep -q '^run:' "$tmp/approved-completion.err" \
	|| { echo "an approved standing-box completion printed a no-op command" >&2; cat "$tmp/approved-completion.err" >&2; exit 1; }

open_forgiving_fixture_goal rejected-reference
set +e
"$ms" goal budget --root "$clone" --id rejected-reference norm --approved-ref missing-reference --fixture-human-authority \
	>"$tmp/rejected-reference.out" 2>"$tmp/rejected-reference.err"
rejected_reference_rc=$?
set -e
[[ $rejected_reference_rc -ne 0 && $(wc -l <"$tmp/rejected-reference.err" | tr -d ' ') -eq 2 ]] \
	&& grep -q '^no command completes this: the approved reference must cover this exact goal revision and box' "$tmp/rejected-reference.err" \
	&& ! grep -q '^run:' "$tmp/rejected-reference.err" \
	|| { echo "a rejected approval reference printed the failing command again" >&2; cat "$tmp/rejected-reference.err" >&2; exit 1; }

open_forgiving_fixture_goal fixture-over-norm
set +e
"$ms" goal budget --root "$clone" --id fixture-over-norm 8h/10/1201m/1/3 --fixture-human-authority \
  >"$tmp/fixture-over-norm.out" 2>"$tmp/fixture-over-norm.err"
fixture_over_norm_rc=$?
set -e
[[ $fixture_over_norm_rc -ne 0 && $(wc -l <"$tmp/fixture-over-norm.err" | tr -d ' ') -eq 2 ]] \
  && grep -q '^no command completes this: run the over-norm box at a real enrolled terminal' "$tmp/fixture-over-norm.err" \
  || { echo "fixture authority entered the enrolled-terminal over-norm branch" >&2; cat "$tmp/fixture-over-norm.err" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == forgiving-budget-identity-aliases ]]; then
write_forgiving_fixture_enrollment Wido
"$ms" goal release --root "$clone" --id ship-widget >/dev/null
open_forgiving_fixture_goal default-human
"$ms" goal budget --root "$clone" --id default-human 2h/4/240m/1/2 --fixture-human-authority >/dev/null
read_forgiving_goal default-human "$tmp/default-human.md"
grep -q ' approve actor=human:Wido ' "$tmp/default-human.md" \
  || { echo "the enrollment name did not default --by" >&2; cat "$tmp/default-human.md" >&2; exit 1; }

open_forgiving_fixture_goal explicit-human
"$ms" goal budget --root "$clone" --id explicit-human 2h/4/240m/1/2 --by Alice --fixture-human-authority >/dev/null
read_forgiving_goal explicit-human "$tmp/explicit-human.md"
grep -q ' approve actor=human:Alice ' "$tmp/explicit-human.md" \
  || { echo "an explicit --by did not win over the enrollment name" >&2; cat "$tmp/explicit-human.md" >&2; exit 1; }

write_forgiving_fixture_enrollment
open_forgiving_fixture_goal nameless-enrollment
set +e
"$ms" goal budget --root "$clone" --id nameless-enrollment norm --fixture-human-authority \
  >"$tmp/nameless-enrollment.out" 2>"$tmp/nameless-enrollment.err"
nameless_rc=$?
set -e
[[ $nameless_rc -ne 0 && $(wc -l <"$tmp/nameless-enrollment.err" | tr -d ' ') -eq 2 ]] \
  && grep -q '^goal budget: the enrolled terminal has no recorded name' "$tmp/nameless-enrollment.err" \
  && grep -q '^no command completes this:' "$tmp/nameless-enrollment.err" \
  && ! grep -q '^run:' "$tmp/nameless-enrollment.err" \
  || { echo "a fieldless enrollment did not produce the words-only name refusal" >&2; cat "$tmp/nameless-enrollment.err" >&2; exit 1; }
write_forgiving_fixture_enrollment Wido

for alias_goal in alias-approve-box direct-approve-box alias-approve-long direct-approve-long alias-set-budget direct-set-budget; do
  open_forgiving_fixture_goal "$alias_goal"
done
"$ms" goal approve --root "$clone" --id alias-approve-box --budget box --by Wido --fixture-human-authority \
  >"$tmp/alias-approve-box.out" 2>"$tmp/alias-approve-box.err"
grep -q '^hint: metasystem goal budget .*--id alias-approve-box .*norm$' "$tmp/alias-approve-box.err" \
  || { echo "approve --budget box did not print its goal budget norm hint" >&2; cat "$tmp/alias-approve-box.err" >&2; exit 1; }
"$ms" goal budget --root "$clone" --id direct-approve-box norm --by Wido --fixture-human-authority >/dev/null

"$ms" goal approve --root "$clone" --id alias-approve-long --by Wido --fixture-human-authority \
  --elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2 \
  >"$tmp/alias-approve-long.out" 2>"$tmp/alias-approve-long.err"
grep -q '^hint: metasystem goal budget .*--id alias-approve-long .*3h/5/300m/1/2$' "$tmp/alias-approve-long.err" \
  || { echo "approve long form did not print its compact goal budget hint" >&2; cat "$tmp/alias-approve-long.err" >&2; exit 1; }
"$ms" goal budget --root "$clone" --id direct-approve-long 3h/5/300m/1/2 --by Wido --fixture-human-authority >/dev/null

"$ms" goal budget --root "$clone" --id alias-set-budget norm --by Wido --fixture-human-authority >/dev/null
"$ms" goal claim --root "$clone" --id alias-set-budget >/dev/null
"$ms" goal set-budget --root "$clone" --id alias-set-budget --by Wido --fixture-human-authority \
  --elapsed-limit 3h --attempt-limit 5 --reserved-job-minutes-limit 300 --active-job-limit 1 --review-round-limit 2 \
  >"$tmp/alias-set-budget.out" 2>"$tmp/alias-set-budget.err"
grep -q '^hint: metasystem goal budget .*--id alias-set-budget .*3h/5/300m/1/2$' "$tmp/alias-set-budget.err" \
  || { echo "set-budget did not print its compact goal budget hint" >&2; cat "$tmp/alias-set-budget.err" >&2; exit 1; }
"$ms" goal release --root "$clone" --id alias-set-budget >/dev/null
"$ms" goal budget --root "$clone" --id direct-set-budget norm --by Wido --fixture-human-authority >/dev/null
"$ms" goal claim --root "$clone" --id direct-set-budget >/dev/null
"$ms" goal budget --root "$clone" --id direct-set-budget 3h/5/300m/1/2 --by Wido --fixture-human-authority >/dev/null

for pair in 'alias-approve-box direct-approve-box approve' 'alias-approve-long direct-approve-long approve' 'alias-set-budget direct-set-budget set-budget'; do
  read -r alias_id direct_id history_verb <<<"$pair"
  read_forgiving_goal "$alias_id" "$tmp/$alias_id.md"
  read_forgiving_goal "$direct_id" "$tmp/$direct_id.md"
  alias_budget=$(sed -n 's/^- Budget: //p' "$tmp/$alias_id.md")
  direct_budget=$(sed -n 's/^- Budget: //p' "$tmp/$direct_id.md")
  [[ "$alias_budget" == "$direct_budget" ]] \
    && grep -q " $history_verb actor=human:Wido " "$tmp/$alias_id.md" \
    && grep -q " $history_verb actor=human:Wido " "$tmp/$direct_id.md" \
    || { echo "$alias_id did not land the same budget and history verb as $direct_id" >&2; exit 1; }
done
fi

if [[ "$fixture_scenario" == forgiving-human-refusals ]]; then
write_forgiving_fixture_enrollment Wido

open_forgiving_fixture_goal dropped-shape
open_forgiving_fixture_goal dropped-shape-twin
run_forgiving_refusal_remedy "shared flag-shape refusal" dropped-shape dropped-shape-twin \
	"$ms" goal budget --root "$clone" --id dropped-shape-twin --by Wido --fixture-human-authority \
		--elapsed-limit 1d --attempt-limit 10 --reserved-job-minutes-limit 1200 --active-job-limit 1 --review-round-limit 3 \
	--refusal "$ms" goal budget --root "$clone" --id dropped-shape norm --by Wido --fixture-human-authority --label stray

open_forgiving_fixture_goal fixture-pair
open_forgiving_fixture_goal fixture-pair-twin
run_forgiving_refusal_remedy "fixture and temporary authority refusal" fixture-pair fixture-pair-twin \
	"$ms" goal budget --root "$clone" --id fixture-pair-twin --by Wido --fixture-human-authority \
		--elapsed-limit 1d --attempt-limit 10 --reserved-job-minutes-limit 1200 --active-job-limit 1 --review-round-limit 3 \
	--refusal "$ms" goal budget --root "$clone" --id fixture-pair norm --by Wido --fixture-human-authority \
		--temporary-human-word "Wido authorizes this relay" --review-by 2026-09-06

set +e
"$ms" goal approve --root "$clone" --sweep --budget box --by Wido --fixture-human-authority \
	>"$tmp/approval-sweep-extras.out" 2>"$tmp/approval-sweep-extras.err"
approval_sweep_extras_rc=$?
set -e
approval_sweep_extras_remedy=$(sed -n 's/^run: //p' "$tmp/approval-sweep-extras.err")
[[ $approval_sweep_extras_rc -ne 0 && -n "$approval_sweep_extras_remedy" ]] \
	|| { echo "approval sweep extras did not print a runnable sweep" >&2; cat "$tmp/approval-sweep-extras.err" >&2; exit 1; }
approval_sweep_preview=$(eval "$approval_sweep_extras_remedy")
grep -q '^listing-sha256=' <<<"$approval_sweep_preview" \
	|| { echo "approval sweep extras printed a command that did not preview" >&2; exit 1; }

open_forgiving_fixture_goal sweep-with-id
open_forgiving_fixture_goal sweep-with-id-twin
run_forgiving_refusal_remedy "approval sweep with a named goal" sweep-with-id sweep-with-id-twin \
	"$ms" goal budget --root "$clone" --id sweep-with-id-twin --by Wido --fixture-human-authority \
		--elapsed-limit 1d --attempt-limit 10 --reserved-job-minutes-limit 1200 --active-job-limit 1 --review-round-limit 3 \
	--refusal "$ms" goal approve --root "$clone" --sweep --id sweep-with-id --confirm stale \
		--by Wido --fixture-human-authority

for parked_id in parked-completion parked-completion-twin; do
	open_forgiving_fixture_goal "$parked_id"
	"$ms" goal budget --root "$clone" --id "$parked_id" 3h/5/300m/1/2 --by Wido --fixture-human-authority >/dev/null
	"$ms" goal park --root "$clone" --id "$parked_id" --because "hold the completed-box fixture" \
		--by Wido --fixture-human-authority >/dev/null
done
run_forgiving_refusal_remedy "parked compact completion" parked-completion parked-completion-twin \
	"$ms" goal budget --root "$clone" --id parked-completion-twin --by Wido --fixture-human-authority \
		--elapsed-limit 4h --attempt-limit 6 --reserved-job-minutes-limit 360 --active-job-limit 1 --review-round-limit 2 \
	--refusal "$ms" goal budget --root "$clone" --id parked-completion 4h/6/360m/1 --by Wido --fixture-human-authority

set +e
"$ms" goal resume --root "$clone" --id ship-widget --by Wido --fixture-human-authority \
	--elapsed-limit 8h --attempt-limit 10 --reserved-job-minutes-limit 1200 --active-job-limit 1 --review-round-limit 3 \
	>"$tmp/resume-without-fence.out" 2>"$tmp/resume-without-fence.err"
resume_without_fence_rc=$?
set -e
[[ $resume_without_fence_rc -ne 0 && $(wc -l <"$tmp/resume-without-fence.err" | tr -d ' ') -eq 2 ]] \
	&& grep -q '^run: metasystem goal budget .*--id ship-widget .*norm$' "$tmp/resume-without-fence.err" \
	|| { echo "resume without a fence did not print the norm budget command" >&2; cat "$tmp/resume-without-fence.err" >&2; exit 1; }
resume_without_fence_remedy=$(sed -n 's/^run: //p' "$tmp/resume-without-fence.err")
eval "$resume_without_fence_remedy" >/dev/null
read_forgiving_goal ship-widget "$tmp/ship-widget-after-resume-remedy.md"
grep -q ' set-budget actor=human:Wido ' "$tmp/ship-widget-after-resume-remedy.md" \
	&& grep -q '^- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3$' "$tmp/ship-widget-after-resume-remedy.md" \
	|| { echo "resume without a fence printed a command that did not land the norm box" >&2; cat "$tmp/ship-widget-after-resume-remedy.md" >&2; exit 1; }

# Earlier fixture rows can leave the machine's claim authority occupied. Free
# every claim this scenario inherited before constructing the claimed twins.
release_forgiving_claim_if_present direct-set-budget
release_forgiving_claim_if_present ship-widget
for claimed_id in claimed-completion claimed-completion-twin; do
	open_forgiving_fixture_goal "$claimed_id"
	"$ms" goal budget --root "$clone" --id "$claimed_id" 3h/5/300m/1/2 --by Wido --fixture-human-authority >/dev/null
done
claim_forgiving_goal claimed-completion
run_claimed_completion_twin() {
	release_forgiving_claim_if_present claimed-completion
	claim_forgiving_goal claimed-completion-twin
	"$ms" goal budget --root "$clone" --id claimed-completion-twin --by Wido --fixture-human-authority \
		--elapsed-limit 4h --attempt-limit 6 --reserved-job-minutes-limit 360 --active-job-limit 1 --review-round-limit 2
}
run_forgiving_refusal_remedy "claimed compact completion" claimed-completion claimed-completion-twin \
	run_claimed_completion_twin \
	--refusal "$ms" goal budget --root "$clone" --id claimed-completion 4h/6/360m/1 --by Wido --fixture-human-authority

for completed_id in parked-completion claimed-completion-twin; do
	set +e
	"$ms" goal budget --root "$clone" --id "$completed_id" 4h/6/360m/1 --by Wido --fixture-human-authority \
		>"$tmp/$completed_id-same.out" 2>"$tmp/$completed_id-same.err"
	completed_rc=$?
	set -e
	[[ $completed_rc -ne 0 && $(wc -l <"$tmp/$completed_id-same.err" | tr -d ' ') -eq 2 ]] \
		&& grep -q '^no command completes this: the goal already carries that box' "$tmp/$completed_id-same.err" \
		&& ! grep -q '^run:' "$tmp/$completed_id-same.err" \
		|| { echo "$completed_id printed a no-op completed box as a command" >&2; cat "$tmp/$completed_id-same.err" >&2; exit 1; }
done

# The same completed box is an act when the claim is breach-stopped: the
# compact refusal must print keep, and keep must match an explicit resume.
export METASYSTEM_GOAL_NOW=$forgiving_breach_at
breach_stop_forgiving_goal claimed-completion-twin
run_stopped_completion_twin() {
	release_forgiving_claim_if_present claimed-completion-twin
	claim_forgiving_goal claimed-completion
	export METASYSTEM_GOAL_NOW=$forgiving_second_breach_at
	breach_stop_forgiving_goal claimed-completion
	"$ms" goal resume --root "$clone" --id claimed-completion --by Wido --fixture-human-authority \
		--elapsed-limit 4h --attempt-limit 6 --reserved-job-minutes-limit 360 --active-job-limit 1 --review-round-limit 2
}
run_forgiving_refusal_remedy "breach-stopped claimed compact completion" claimed-completion-twin claimed-completion \
	run_stopped_completion_twin \
	--refusal "$ms" goal budget --root "$clone" --id claimed-completion-twin 4h/6/360m/1 --by Wido --fixture-human-authority
release_forgiving_claim_if_present claimed-completion
release_forgiving_claim_if_present claimed-completion-twin
claim_forgiving_goal ship-widget

set +e
"$ms" goal unapprove --root "$clone" --id fix-docs --by Wido --fixture-human-authority \
  >"$tmp/unapprove-missing.out" 2>"$tmp/unapprove-missing.err"
unapprove_missing_rc=$?
set -e
[[ $unapprove_missing_rc -ne 0 && $(wc -l <"$tmp/unapprove-missing.err" | tr -d ' ') -eq 2 ]] \
  && grep -q '^no command completes this:' "$tmp/unapprove-missing.err" \
  || { echo "unapprove's unseen reason did not produce a words refusal" >&2; cat "$tmp/unapprove-missing.err" >&2; exit 1; }

mkdir -p "$clone/artifacts/agents/jobs"
cat >"$clone/artifacts/agents/jobs/fixture-risk.json" <<'JSON'
{"jobId":"fixture-risk","role":"code-critic","round":1,"parentJob":null,"status":"completed","goalId":"ship-widget","findingRegisterRound":1,"reviewRoundLimit":3,"criticRoundsConsumed":3,"demotions":[],"findingRegister":[{"findingId":"RISK-1","critic":"fixture-critic","rigorClass":"severe","factsDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","facts":{"local":true},"artifact":"metasystem/fixture.go","title":"fixture severe finding","status":"open","resolution":"","decisionOpid":"","evidence":"direct fixture evidence","evidenceDigest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","multiplicity":1}]}
JSON
set +e
"$ms" goal accept-risk --root "$clone" --id ship-widget --finding RISK-1 --chain fixture-risk --why "fixture pair" --by Wido \
	--fixture-human-authority --temporary-human-word "Wido authorizes this relay" \
	>"$tmp/accept-risk-pair.out" 2>"$tmp/accept-risk-pair.err"
accept_risk_pair_rc=$?
set -e
[[ $accept_risk_pair_rc -ne 0 && $(wc -l <"$tmp/accept-risk-pair.err" | tr -d ' ') -eq 2 ]] \
	&& grep -q '^run: metasystem goal accept-risk ' "$tmp/accept-risk-pair.err" \
	|| { echo "accept-risk's temporary pair did not produce a two-line command refusal" >&2; cat "$tmp/accept-risk-pair.err" >&2; exit 1; }
accept_risk_pair_remedy=$(sed -n 's/^run: //p' "$tmp/accept-risk-pair.err")
eval "$accept_risk_pair_remedy" >/dev/null
read_forgiving_goal ship-widget "$tmp/ship-widget-accepted-risk.md"
grep -q '^- AcceptedRisk: finding=RISK-1 chain=fixture-risk by=Wido opid=' "$tmp/ship-widget-accepted-risk.md" \
	|| { echo "accept-risk's printed pair remedy did not accept the real fixture finding" >&2; cat "$tmp/ship-widget-accepted-risk.md" >&2; exit 1; }

set +e
"$ms" goal set-obligation --root "$clone" --id ship-widget --fixture-human-authority \
  >"$tmp/set-obligation-missing.out" 2>"$tmp/set-obligation-missing.err"
set_obligation_missing_rc=$?
set -e
[[ $set_obligation_missing_rc -ne 0 && $(wc -l <"$tmp/set-obligation-missing.err" | tr -d ' ') -eq 2 ]] \
  && grep -q '^no command completes this:' "$tmp/set-obligation-missing.err" \
  || { echo "set-obligation's absent values did not produce a words refusal" >&2; cat "$tmp/set-obligation-missing.err" >&2; exit 1; }

set +e
"$ms" goal enroll-terminal --root "$clone" --lineage fixture-lineage \
  >"$tmp/enroll-missing-name.out" 2>"$tmp/enroll-missing-name.err"
enroll_missing_name_rc=$?
set -e
[[ $enroll_missing_name_rc -ne 0 && $(wc -l <"$tmp/enroll-missing-name.err" | tr -d ' ') -eq 2 ]] \
  && grep -q '^no command completes this:' "$tmp/enroll-missing-name.err" \
  || { echo "enroll-terminal without a name did not produce a words refusal" >&2; cat "$tmp/enroll-missing-name.err" >&2; exit 1; }

refusal_draft="$tmp/forgiving-classification.txt"
cat >"$refusal_draft" <<'DRAFT'
ship-widget 3,1,1,1 claimed migration
fix-docs 1,1,1,1 queued migration
perf-pass 2,1,1,1 parked migration
DRAFT
set +e
"$ms" goal classify-sweep --root "$clone" --draft "$refusal_draft" \
	>"$tmp/classify-missing-mode.out" 2>"$tmp/classify-missing-mode.err"
classify_missing_mode_rc=$?
set -e
classify_missing_mode_remedy=$(sed -n 's/^run: //p' "$tmp/classify-missing-mode.err")
[[ $classify_missing_mode_rc -ne 0 && -n "$classify_missing_mode_remedy" ]] \
	|| { echo "classify-sweep's missing mode did not print a preview command" >&2; cat "$tmp/classify-missing-mode.err" >&2; exit 1; }
classify_missing_mode_preview=$(eval "$classify_missing_mode_remedy")
grep -q '^listing-digest ' <<<"$classify_missing_mode_preview" \
	|| { echo "classify-sweep's missing-mode command did not run" >&2; exit 1; }

set +e
"$ms" goal classify-sweep --root "$clone" --draft "$tmp/missing-classification.txt" --preview \
  >"$tmp/classify-unreadable.out" 2>"$tmp/classify-unreadable.err"
classify_unreadable_rc=$?
set -e
[[ $classify_unreadable_rc -ne 0 && $(wc -l <"$tmp/classify-unreadable.err" | tr -d ' ') -eq 2 ]] \
  && grep -q '^no command completes this:' "$tmp/classify-unreadable.err" \
  || { echo "classify-sweep's unreadable draft did not produce a words refusal" >&2; cat "$tmp/classify-unreadable.err" >&2; exit 1; }

classification_preview=$("$ms" goal classify-sweep --root "$clone" --draft "$refusal_draft" --preview)
classification_digest=$(sed -n 's/^listing-digest //p' <<<"$classification_preview")
sed 's/claimed migration/changed migration/' "$refusal_draft" >"$tmp/changed-forgiving-classification.txt"
set +e
"$ms" goal classify-sweep --root "$clone" --draft "$tmp/changed-forgiving-classification.txt" \
	--confirm "$classification_digest" --by Wido --fixture-human-authority \
	>"$tmp/classify-changed.out" 2>"$tmp/classify-changed.err"
classify_changed_rc=$?
set -e
classify_changed_remedy=$(sed -n 's/^run: //p' "$tmp/classify-changed.err")
[[ $classify_changed_rc -ne 0 && -n "$classify_changed_remedy" ]] \
	|| { echo "classify-sweep's changed listing did not print a preview command" >&2; cat "$tmp/classify-changed.err" >&2; exit 1; }
classify_changed_preview=$(eval "$classify_changed_remedy")
grep -q '^listing-digest ' <<<"$classify_changed_preview" \
	|| { echo "classify-sweep's changed-listing command did not run" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == risk-basis ]]; then
set +e
tier_alone=$("$ms" goal open --root "$clone" --id unanswered-risk \
  --intent "Refuse a tier without the four answers." --next "Answer the risk questions." --tier 2 2>&1)
tier_alone_rc=$?
set -e
[[ $tier_alone_rc -ne 0 && "$tier_alone" == *"answer the four questions: --risk severity=,novelty=,exposure=,accumulation= --basis"* ]] \
  || { echo "goal open --tier alone did not name the four unanswered questions: rc=$tier_alone_rc output=$tier_alone" >&2; exit 1; }
"$ms" goal open --root "$clone" --id risk-basis --origin human \
  --intent "Exercise risk-derived intake." --next "Keep the risk basis visible." \
  --risk severity=2,novelty=1,exposure=1,accumulation=1 \
  --basis "moderate consequence with landed precedent" >/dev/null
risk_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$risk_tip:plans/goals/risk-basis.md" >"$tmp/risk-basis.md"
risk_line=$(grep -n '^- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="moderate consequence with landed precedent"$' "$tmp/risk-basis.md" | cut -d: -f1)
tier_line=$(grep -n '^- Tier: 2$' "$tmp/risk-basis.md" | cut -d: -f1)
[[ -n "$risk_line" && -n "$tier_line" && $tier_line -eq $((risk_line + 1)) ]] \
  || { echo "risk-basis open did not render Risk immediately above derived Tier" >&2; cat "$tmp/risk-basis.md" >&2; exit 1; }
# The tier derives from severity and novelty; exposure and accumulation
# weight the proof (goal tier-from-severity-and-novelty). A recorded tier
# above the derivation stands when a seat re-states the answers, the probe
# lists it, and only a person lowers it with --tier and the same answers.
"$ms" goal open --root "$clone" --id exposed-legacy --origin human \
  --intent "Wide but routine, tiered by the earlier formula." --next "Lower it." \
  --tier 3 --why "the earlier formula" --risk severity=1,novelty=1,exposure=3,accumulation=1 \
  --basis "wide but routine" >/dev/null
probe_out=$("$ms" goal tier-probe --root "$clone" --pretty)
grep -q '^lowerable: exposed-legacy queued recorded=3 derived=1' <<<"$probe_out" \
  || { echo "the tier probe does not list the exposure-lifted goal: $probe_out" >&2; exit 1; }
"$ms" goal edit --root "$clone" --id exposed-legacy \
  --risk severity=1,novelty=1,exposure=3,accumulation=1 --basis "wide but routine, reworded" >/dev/null
restate_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$restate_tip:plans/goals/exposed-legacy.md" >"$tmp/exposed-legacy.md"
grep -q '^- Tier: 3$' "$tmp/exposed-legacy.md" \
  || { echo "a re-stated basis lowered the recorded tier" >&2; cat "$tmp/exposed-legacy.md" >&2; exit 1; }
if seat_lower=$("$ms" goal edit --root "$clone" --id exposed-legacy --tier 1 \
  --risk severity=1,novelty=1,exposure=3,accumulation=1 --basis "wide but routine, reworded" --why "the formula changed" 2>&1); then
  echo "a seat lowered a recorded tier: $seat_lower" >&2; exit 1
fi
grep -q 'human act' <<<"$seat_lower" \
  || { echo "the seat's lowering refusal does not name the human act: $seat_lower" >&2; exit 1; }
"$ms" goal edit --root "$clone" --id exposed-legacy --tier 1 \
  --risk severity=1,novelty=1,exposure=3,accumulation=1 --basis "wide but routine, reworded" --why "the formula changed" \
  --by Wido --fixture-human-authority >/dev/null
lower_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$lower_tip:plans/goals/exposed-legacy.md" >"$tmp/exposed-legacy.md"
grep -q '^- Tier: 1$' "$tmp/exposed-legacy.md" \
  || { echo "the person's lowering with unchanged answers did not land" >&2; cat "$tmp/exposed-legacy.md" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == labels-and-filtering ]]; then
# 6. Label writes are canonical whole fields. Open accepts repeated
# labels, sorts and deduplicates them, while an unlabeled open keeps
# the field absent.
export METASYSTEM_OWNER_LINEAGE=fixture-lineage
open_labels=$("$ms" goal open --root "$clone" --id labeled-one --origin human \
	--intent "First labeled goal." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" \
  --label beta --label alpha --label beta)
grep -q '"outcome":"confirmed"' <<<"$open_labels" \
  || { echo "goal open with labels did not confirm: $open_labels" >&2; exit 1; }
"$ms" goal open --root "$clone" --id plain-goal --origin human \
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
if bad_label=$("$ms" goal open --root "$clone" --id bad-label --origin human \
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
summary_goal_ids() {
  awk '$1 ~ /^[0-3]:[0-9]+$/ && $2 ~ /^(queued|approved|claimed|parked)$/ && $3 == "tier" && $5 ~ /^[a-z][a-z0-9-]*$/ { print $5 }'
}
"$ms" goal open --root "$clone" --id labeled-two --origin human \
	--intent "Second labeled goal." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" \
  --label shared --label alpha >/dev/null
one_filter=$("$ms" goal list --root "$clone" --label shared)
one_ids=$(summary_goal_ids <<<"$one_filter")
grep -Fxq 'labeled-one' <<<"$one_ids" && grep -Fxq 'labeled-two' <<<"$one_ids" \
  || { echo "one-label list filtering lost a match: $one_filter" >&2; exit 1; }
if grep -Fxq 'plain-goal' <<<"$one_ids"; then
  echo "a zero-label goal appeared in a filtered list" >&2; exit 1
fi
two_filters=$("$ms" goal list --root "$clone" --label alpha --label shared)
two_ids=$(summary_goal_ids <<<"$two_filters")
grep -Fxq 'labeled-one' <<<"$two_ids" && grep -Fxq 'labeled-two' <<<"$two_ids" \
  || { echo "two-label AND filtering lost a match: $two_filters" >&2; exit 1; }
"$ms" goal open --root "$clone" --id and-a --origin human \
	--intent "Carries only a." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --label a >/dev/null
"$ms" goal open --root "$clone" --id and-ab --origin human \
	--intent "Carries a and b." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --label a --label b >/dev/null
and_probe=$("$ms" goal list --root "$clone" --label a --label b)
and_ids=$(summary_goal_ids <<<"$and_probe")
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
"$ms" goal open --root "$clone" --id machine-only --origin human \
	--intent "Parked matching work is not claimable." --next "Wait for it to be unparked." \
	--risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "machine-scoped empty fixture" --label machine-only >/dev/null
"$ms" goal park --root "$clone" --id machine-only --by Wido --because "Keep the matching goal unavailable." --fixture-human-authority >/dev/null
machine_empty=$("$ms" goal next --root "$clone" --machine fixture-machine --label machine-only)
[[ "$machine_empty" == "no claimable goal for machine fixture-machine; no matching eligible work" ]] \
  || { echo "the machine-scoped empty candidate message is not distinct: $machine_empty" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == structured-budget ]]; then
"$ms" goal release --root "$clone" --id ship-widget >/dev/null
"$ms" goal open --root "$clone" --id plain-goal --origin human \
	--intent "An unlabeled goal." --next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" >/dev/null
# 11. The separated intake, approval, and claim path preserves labels.
"$ms" goal open --root "$clone" --id claimed-label --origin human \
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
"$ms" goal open --root "$clone" --id budget-check --origin human \
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
"$ms" goal open --root "$clone" --id norm-parent --origin human \
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
# The parent is the seat's own open, so it names the claimed goal it blocks
# (R-93-m1e); the split then rewrites that goal's edge and park to the members.
"$ms" goal open --root "$clone" --id split-parent --blocks norm-parent \
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
git -C "$clone" cat-file -p "$split_tip:plans/goals/norm-parent.md" >"$tmp/norm-parent-after-split.md"
grep -q '^- BlockedBy: split-parent-one, split-parent-two$' "$tmp/norm-parent-after-split.md" \
  && grep -q ' blocker=split-parent-one because=' "$tmp/norm-parent-after-split.md" \
  || { echo "the split did not move the blocked goal's edge and park to the members" >&2; cat "$tmp/norm-parent-after-split.md" >&2; exit 1; }
if reopen_refusal=$("$ms" goal reopen --root "$clone" --id split-parent 2>&1); then
  echo "a decomposed parent reopened" >&2; exit 1
fi
grep -q 'a decomposed parent never returns' <<<"$reopen_refusal" \
  || { echo "reopen did not name permanent decomposition: $reopen_refusal" >&2; exit 1; }
"$ms" goal prune --root "$clone" --keep 0 >/dev/null
if recreate_refusal=$("$ms" goal open --root "$clone" --id split-parent --origin human \
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
  --confirm "$classification_digest" --by Wido --fixture-human-authority)
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
  "$ms" goal open --root "$clone" --id budget-check --origin human \
		--intent "Exercise structured budget admission." \
		--next "Continue." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" --elapsed-limit 8h --attempt-limit 2 \
		--reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3 >/dev/null
approve_fixture_goal budget-check \
	--elapsed-limit 8h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 3
METASYSTEM_GOAL_NOW=2026-08-20T00:00:00Z "$ms" goal claim --root "$clone" --id budget-check >/dev/null
# 13. Concluding writes the records-owned archive, reopening records a
# ledger move back to the live set, and concluding again preserves the
# canonical record bytes including its Integrity line.
"$ms" goal open --root "$clone" --id archive-roundtrip --origin human \
	--intent "Exercise concluded-goal archival." --next "Conclude it." --tier 3 --risk severity=3,novelty=1,exposure=1,accumulation=1 --basis "fixture risk" >/dev/null
"$ms" goal done --root "$clone" --id archive-roundtrip --by Wido \
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
"$ms" goal done --root "$clone" --id archive-roundtrip --by Wido \
  --conclude "Archived again after the recorded reopen." >/dev/null

# Admission must stop charging a concluded goal even when its only conclusion
# is in records/goals. Prove the causal change by exhausting a claimed goal,
# observing refusal, concluding it, and observing acceptance at the same clock.
"$ms" goal release --root "$clone" --id budget-check >/dev/null
METASYSTEM_GOAL_NOW=2026-08-20T10:00:00Z \
	"$ms" goal open --root "$clone" --id admission-concluded --origin human \
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
  "$ms" goal done --root "$clone" --id admission-concluded --by Wido \
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
dual_list=$("$ms" goal list --root "$clone")
grep -q ' done=3 abandoned=0 tip=' <<<"$dual_list" \
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

if [[ "$fixture_scenario" == seat-blocker ]]; then
# 16. A seat opens only the defect that blocks its claimed goal (R-93-m1e).
# Without --blocks the open is refused with the ruling and the ledger does
# not move. With it, one publish opens the blocker and parks the blocked
# goal with the blocker recorded and the seat's claim cleared; an agent
# cannot lift that park early; the park lifts by itself in the publish that
# concludes the blocker, and the goal is claimable again.
seat_tip=$(git -C "$origin" rev-parse main)
set +e
stray=$("$ms" goal open --root "$clone" --id stray-idea \
  --intent "An improvement that blocks nothing." --next "Do it." \
  --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "seat blocker fixture" 2>&1)
stray_rc=$?
set -e
[[ $stray_rc -ne 0 && "$stray" == *"R-93-m1e"* && "$stray" == *"--blocks"* ]] \
  || { echo "a seat open without --blocks was not refused with the ruling: rc=$stray_rc output=$stray" >&2; exit 1; }
[[ $(git -C "$origin" rev-parse main) == "$seat_tip" ]] \
  || { echo "the refused seat open moved the ledger" >&2; exit 1; }
# ship-widget is this seat's claimed goal and the person opened it: its
# blocker parks it all the same, with the blocker on record.
"$ms" goal open --root "$clone" --id widget-defect --blocks ship-widget \
  --intent "The defect that blocks ship-widget." --next "Fix it." \
  --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "seat blocker fixture" >/dev/null
blocker_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -e "$blocker_tip:plans/goals/widget-defect.md" \
  || { echo "the blocker did not open" >&2; exit 1; }
git -C "$clone" cat-file -p "$blocker_tip:plans/goals/ship-widget.md" >"$tmp/ship-widget-parked.md"
grep -q '^- State: parked$' "$tmp/ship-widget-parked.md" \
  || { echo "the blocked goal did not park in the open's publish" >&2; cat "$tmp/ship-widget-parked.md" >&2; exit 1; }
grep -q '^- BlockedBy: widget-defect$' "$tmp/ship-widget-parked.md" \
  || { echo "the blocked goal did not record the edge" >&2; cat "$tmp/ship-widget-parked.md" >&2; exit 1; }
grep -q ' blocker=widget-defect because=blocked by widget-defect' "$tmp/ship-widget-parked.md" \
  || { echo "the park does not name its blocker" >&2; cat "$tmp/ship-widget-parked.md" >&2; exit 1; }
if grep -q '^- Claimed:' "$tmp/ship-widget-parked.md"; then
  echo "the park kept the seat's claim" >&2; cat "$tmp/ship-widget-parked.md" >&2; exit 1
fi
set +e
early=$("$ms" goal unpark --root "$clone" --id ship-widget 2>&1)
early_rc=$?
set -e
[[ $early_rc -ne 0 && "$early" == *"returns by itself"* ]] \
  || { echo "an agent lifted a blocker park early: rc=$early_rc output=$early" >&2; exit 1; }
# The seat's one claim is free for the blocker; concluding the blocker
# returns the blocked goal in the same publish.
approve_fixture_goal widget-defect --budget box
"$ms" goal claim --root "$clone" --id widget-defect >/dev/null
"$ms" goal done --root "$clone" --id widget-defect --conclude "Fixed; ship-widget continues." >/dev/null
return_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$return_tip:plans/goals/ship-widget.md" >"$tmp/ship-widget-returned.md"
if grep -q '^- State: parked$' "$tmp/ship-widget-returned.md"; then
  echo "the blocked goal did not return when its blocker was done" >&2; cat "$tmp/ship-widget-returned.md" >&2; exit 1
fi
grep -q ' unpark actor=fixture-machine+fixture-lineage .*reason=blocker widget-defect is done' "$tmp/ship-widget-returned.md" \
  || { echo "the return is not recorded on the goal" >&2; cat "$tmp/ship-widget-returned.md" >&2; exit 1; }
if grep -q '^- State: queued$' "$tmp/ship-widget-returned.md"; then
  approve_fixture_goal ship-widget \
    --elapsed-limit 4h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 0
fi
claim_back=$("$ms" goal claim --root "$clone" --id ship-widget)
grep -q '"outcome":"confirmed"' <<<"$claim_back" \
  || { echo "the returned goal is not claimable again: $claim_back" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == landing-slot ]]; then
# 18. Built work lands beside the seat's claim (goal 20): land-ready keeps
# the claim but frees the machine's one claim; a second slot is refused;
# goal next continues the working claim and names the landing goal; a
# person concludes the landing goal; and the same pair's release and
# re-claim keep the accounting episode with the gap as idle seconds. The
# stamps are computed from the real clock (the migrated claim carries it)
# so every publish is later than the one before.
landing_base=$(( $(date -u +%s) + 60 ))
land_at=$(fixture_stamp "$landing_base")
open_at=$(fixture_stamp $((landing_base + 300)))
release_at=$(fixture_stamp $((landing_base + 3600)))
reclaim_at=$(fixture_stamp $((landing_base + 3600 + 7200)))
export METASYSTEM_GOAL_NOW=$land_at
"$ms" goal land-ready --root "$clone" --id ship-widget >/dev/null
landing_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$landing_tip:plans/goals/ship-widget.md" >"$tmp/ship-widget-landing.md"
grep -q "^- Landing: at=$land_at opid=" "$tmp/ship-widget-landing.md" \
  || { echo "land-ready did not write the Landing record" >&2; cat "$tmp/ship-widget-landing.md" >&2; exit 1; }
grep -q '^- Claimed: machine=fixture-machine lineage=fixture-lineage' "$tmp/ship-widget-landing.md" \
  || { echo "land-ready dropped the claim" >&2; cat "$tmp/ship-widget-landing.md" >&2; exit 1; }
grep -q ' land-ready actor=fixture-machine+fixture-lineage targets=ship-widget' "$tmp/ship-widget-landing.md" \
  || { echo "land-ready wrote no history line" >&2; cat "$tmp/ship-widget-landing.md" >&2; exit 1; }
set +e
repeat=$("$ms" goal land-ready --root "$clone" --id ship-widget 2>&1)
repeat_rc=$?
set -e
[[ "$repeat" == *"already in landing"* ]] \
  || { echo "a repeated land-ready was not nothing to do: rc=$repeat_rc output=$repeat" >&2; exit 1; }
# The seat's one claim is free: the next goal claims beside the landing goal.
export METASYSTEM_GOAL_NOW=$open_at
"$ms" goal open --root "$clone" --id next-widget --origin human \
  --intent "The next goal the seat takes while ship-widget waits to land." --next "Build it." \
  --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "landing slot fixture" >/dev/null
approve_fixture_goal next-widget --budget box
next_claim=$("$ms" goal claim --root "$clone" --id next-widget)
grep -q '"outcome":"confirmed"' <<<"$next_claim" \
  || { echo "the seat could not claim beside its landing goal: $next_claim" >&2; exit 1; }
set +e
second_slot=$("$ms" goal land-ready --root "$clone" --id next-widget 2>&1)
second_rc=$?
set -e
[[ $second_rc -ne 0 && "$second_slot" == *"one landing slot per machine"* ]] \
  || { echo "a second landing slot was not refused: rc=$second_rc output=$second_slot" >&2; exit 1; }
next_out=$("$ms" goal next --root "$clone" --machine fixture-machine)
grep -q "^LANDING ship-widget: land-ready since $land_at; the queue is open\$" <<<"$next_out" \
  || { echo "goal next does not name the landing goal: $next_out" >&2; exit 1; }
grep -q '^continue your claimed goal: next-widget$' <<<"$next_out" \
  || { echo "goal next does not continue the working claim: $next_out" >&2; exit 1; }
"$ms" goal list --json --root "$clone" >"$tmp/landing-list.txt"
grep -q "\"Landing\":{\"At\":\"$land_at\"" "$tmp/landing-list.txt" \
  || { echo "goal list does not show the landing slot" >&2; cat "$tmp/landing-list.txt" >&2; exit 1; }
# The person concludes the landing goal; the archive keeps the land-ready
# line and drops the slot with the claim.
"$ms" goal done --root "$clone" --id ship-widget --by Wido \
  --conclude "Landed by the person while next-widget was claimed." >/dev/null
done_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$done_tip:records/goals/ship-widget.md" >"$tmp/ship-widget-done.md"
grep -q ' land-ready actor=' "$tmp/ship-widget-done.md" \
  || { echo "the land-ready line did not survive into the archive" >&2; cat "$tmp/ship-widget-done.md" >&2; exit 1; }
if grep -q '^- Landing:' "$tmp/ship-widget-done.md"; then
  echo "done kept the landing slot" >&2; cat "$tmp/ship-widget-done.md" >&2; exit 1
fi
# The same pair releases and re-claims: the accounting revision and episode
# hold and the two-hour gap is idle time.
git -C "$clone" cat-file -p "$done_tip:plans/goals/next-widget.md" >"$tmp/next-widget-claimed.md"
accounting_before=$(sed -n 's/^- Claimed: .* accountingRevision=\([0-9]*\) .*/\1/p' "$tmp/next-widget-claimed.md")
[[ -n "$accounting_before" ]] || { echo "the claim carries no accounting revision" >&2; cat "$tmp/next-widget-claimed.md" >&2; exit 1; }
export METASYSTEM_GOAL_NOW=$release_at
"$ms" goal release --root "$clone" --id next-widget >/dev/null
release_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$release_tip:plans/goals/next-widget.md" >"$tmp/next-widget-released.md"
grep -q "^- Episode: machine=fixture-machine lineage=fixture-lineage accountingRevision=$accounting_before .* idleSeconds=0 released=$release_at\$" "$tmp/next-widget-released.md" \
  || { echo "the own pair's release did not keep the episode" >&2; cat "$tmp/next-widget-released.md" >&2; exit 1; }
export METASYSTEM_GOAL_NOW=$reclaim_at
reclaim=$("$ms" goal claim --root "$clone" --id next-widget)
grep -q '"outcome":"confirmed"' <<<"$reclaim" \
  || { echo "the same pair could not re-claim: $reclaim" >&2; exit 1; }
reclaim_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$reclaim_tip:plans/goals/next-widget.md" >"$tmp/next-widget-reclaimed.md"
grep -q "^- Claimed: machine=fixture-machine lineage=fixture-lineage at=$reclaim_at revision=[0-9]* accountingRevision=$accounting_before episodeAt=.* idleSeconds=7200\$" "$tmp/next-widget-reclaimed.md" \
  || { echo "the re-claim did not keep the episode with the gap idle" >&2; cat "$tmp/next-widget-reclaimed.md" >&2; exit 1; }
if grep -q '^- Episode:' "$tmp/next-widget-reclaimed.md"; then
  echo "the re-claim did not consume the kept episode" >&2; cat "$tmp/next-widget-reclaimed.md" >&2; exit 1
fi
unset METASYSTEM_GOAL_NOW
fi

if [[ "$fixture_scenario" == power-of-attorney ]]; then
# 17. A person records a power of attorney (R-95-m1e); the seat then approves
# and set-budgets a tier-1 goal with no terminal and no --by, as its own act
# with the entry on the line; anything outside the entry's scope is refused.
"$ms" goal release --root "$clone" --id ship-widget >/dev/null
export METASYSTEM_GOAL_NOW=2026-08-20T09:00:00Z
"$ms" goal open --root "$clone" --id poa-small --origin human \
  --intent "Tier-one work the seat may approve under attorney." --next "Approve it under the entry." \
  --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "power of attorney fixture" >/dev/null
"$ms" goal open --root "$clone" --id poa-medium --origin human \
  --intent "Tier-two work outside the entry." --next "Refuse it under the entry." \
  --risk severity=2,novelty=1,exposure=1,accumulation=1 --basis "power of attorney fixture" >/dev/null
"$ms" goal open --root "$clone" --id poa-late --origin human \
  --intent "Tier-one work asked for after the entry expires." --next "Refuse it under the entry." \
  --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "power of attorney fixture" >/dev/null
grant_out=$("$ms" goal grant --root "$clone" --by Wido --fixture-human-authority \
  --tiers 1 --verbs approve,set-budget --expires 2026-08-25)
grep -q '"outcome":"confirmed"' <<<"$grant_out" \
  || { echo "goal grant did not confirm: $grant_out" >&2; exit 1; }
entry=$(sed -n 's/.*"entry":"\([^"]*\)".*/\1/p' <<<"$grant_out")
[[ -n "$entry" ]] || { echo "goal grant printed no entry id: $grant_out" >&2; exit 1; }
grant_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$grant_tip:plans/goals/backlog.md" >"$tmp/backlog-grant.md"
grep -q "^- $entry by=human:Wido tiers=1 verbs=approve,set-budget since=2026-08-20T09:00:00Z expires=2026-08-25$" "$tmp/backlog-grant.md" \
  || { echo "the root record does not carry the entry" >&2; cat "$tmp/backlog-grant.md" >&2; exit 1; }
under_out=$("$ms" goal approve --root "$clone" --id poa-small --under "$entry")
grep -q '"outcome":"confirmed"' <<<"$under_out" \
  || { echo "approve under attorney did not confirm: $under_out" >&2; exit 1; }
under_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$under_tip:plans/goals/poa-small.md" >"$tmp/poa-small.md"
grep -q '^- Approved: by=fixture-machine+fixture-lineage .* authority=attorney ' "$tmp/poa-small.md" \
  || { echo "the approval is not the seat's own act under attorney" >&2; cat "$tmp/poa-small.md" >&2; exit 1; }
grep -q " approve actor=fixture-machine+fixture-lineage targets=poa-small authorityOutcome=POWER_OF_ATTORNEY authorityRuling=$entry" "$tmp/poa-small.md" \
  || { echo "the history line does not name the entry" >&2; cat "$tmp/poa-small.md" >&2; exit 1; }
"$ms" goal claim --root "$clone" --id poa-small >/dev/null
budget_out=$("$ms" goal set-budget --root "$clone" --id poa-small --under "$entry" \
  --elapsed-limit 1h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 0)
grep -q '"outcome":"confirmed"' <<<"$budget_out" \
  || { echo "set-budget under attorney did not confirm: $budget_out" >&2; exit 1; }
if over_out=$("$ms" goal set-budget --root "$clone" --id poa-small --under "$entry" \
  --elapsed-limit 8h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 --review-round-limit 0 2>&1); then
  echo "an over-box set-budget under attorney succeeded: $over_out" >&2; exit 1
fi
grep -q 'GOAL_NORM_REFUSED' <<<"$over_out" \
  || { echo "the over-box refusal is not the norm refusal: $over_out" >&2; exit 1; }
if tier_out=$("$ms" goal approve --root "$clone" --id poa-medium --under "$entry" 2>&1); then
  echo "a tier-2 goal was approved under a tier-1 entry: $tier_out" >&2; exit 1
fi
grep -q 'covers tier 1 only' <<<"$tier_out" \
  || { echo "the tier refusal does not name the scope: $tier_out" >&2; exit 1; }
# An entry may cover tier 2 as well (R-95-m1e, since tier-from-severity-and-novelty).
wide_out=$("$ms" goal grant --root "$clone" --by Wido --fixture-human-authority \
  --tiers 1,2 --verbs approve --expires 2026-08-24)
grep -q '"outcome":"confirmed"' <<<"$wide_out" \
  || { echo "a tiers 1,2 grant did not confirm: $wide_out" >&2; exit 1; }
wide=$(sed -n 's/.*"entry":"\([^"]*\)".*/\1/p' <<<"$wide_out")
wide_approve=$("$ms" goal approve --root "$clone" --id poa-medium --under "$wide")
grep -q '"outcome":"confirmed"' <<<"$wide_approve" \
  || { echo "approve of a tier-2 goal under a tiers 1,2 entry did not confirm: $wide_approve" >&2; exit 1; }
wide_tip=$(git -C "$origin" rev-parse main)
if ! git -C "$clone" cat-file -p "$wide_tip:plans/goals/backlog.md" >"$tmp/backlog-wide.md"; then
  echo "could not read the backlog at $wide_tip" >&2
  exit 1
fi
grep -q "^- $wide by=human:Wido tiers=1,2 verbs=approve " "$tmp/backlog-wide.md" \
  || { echo "the root record does not carry the tiers 1,2 entry" >&2; cat "$tmp/backlog-wide.md" >&2; exit 1; }
if by_out=$("$ms" goal approve --root "$clone" --id poa-late --under "$entry" --by Wido 2>&1); then
  echo "--under combined with --by: $by_out" >&2; exit 1
fi
grep -q "seat's own act" <<<"$by_out" \
  || { echo "the --by refusal does not say whose act it is: $by_out" >&2; exit 1; }
# The attorney unpark (R-105-m1e): a seat under an entry naming unpark lifts
# a park a person recorded on a tier-1 goal, saying what it verified; an
# entry without the verb, a tier-2 goal and a blocker park refuse.
"$ms" goal park --root "$clone" --id poa-late --by Wido --because "wait for the vendor's 1.2 release" --fixture-human-authority >/dev/null
if noverb_out=$("$ms" goal unpark --root "$clone" --id poa-late --under "$entry" --verified "1.2 is on the vendor's page" 2>&1); then
  echo "an entry without unpark lifted a person's park: $noverb_out" >&2; exit 1
fi
grep -q 'covers approve,set-budget, not unpark' <<<"$noverb_out" \
  || { echo "the verb refusal does not name the entry's verbs: $noverb_out" >&2; exit 1; }
if bare_out=$("$ms" goal unpark --root "$clone" --id poa-late 2>&1); then
  echo "a seat lifted a person's park with no entry: $bare_out" >&2; exit 1
fi
grep -q "lifting a human's pause is a human act" <<<"$bare_out" \
  || { echo "the bare unpark refusal changed: $bare_out" >&2; exit 1; }
lift_grant=$("$ms" goal grant --root "$clone" --by Wido --fixture-human-authority \
  --tiers 1,2 --verbs unpark --expires 2026-08-24)
grep -q '"outcome":"confirmed"' <<<"$lift_grant" \
  || { echo "a grant naming unpark did not confirm: $lift_grant" >&2; exit 1; }
lift=$(sed -n 's/.*"entry":"\([^"]*\)".*/\1/p' <<<"$lift_grant")
if noreason_out=$("$ms" goal unpark --root "$clone" --id poa-late --under "$lift" 2>&1); then
  echo "an attorney unpark ran without --verified: $noreason_out" >&2; exit 1
fi
grep -q -- '--verified' <<<"$noreason_out" \
  || { echo "the missing --verified refusal does not name the flag: $noreason_out" >&2; exit 1; }
if lift_by=$("$ms" goal unpark --root "$clone" --id poa-late --under "$lift" --verified "it holds" --by Wido 2>&1); then
  echo "unpark --under combined with --by: $lift_by" >&2; exit 1
fi
grep -q "seat's own act" <<<"$lift_by" \
  || { echo "the --by refusal on unpark does not say whose act it is: $lift_by" >&2; exit 1; }
if stray_verified=$("$ms" goal unpark --root "$clone" --id poa-late --verified "it holds" 2>&1); then
  echo "unpark --verified without --under ran: $stray_verified" >&2; exit 1
fi
grep -q -- '--under' <<<"$stray_verified" \
  || { echo "the stray --verified refusal does not name --under: $stray_verified" >&2; exit 1; }
lift_out=$("$ms" goal unpark --root "$clone" --id poa-late --under "$lift" --verified "1.2 is on the vendor's page")
grep -q '"outcome":"confirmed"' <<<"$lift_out" \
  || { echo "the attorney unpark did not confirm: $lift_out" >&2; exit 1; }
lift_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$lift_tip:plans/goals/poa-late.md" >"$tmp/poa-late.md"
grep -q '^- State: queued$' "$tmp/poa-late.md" \
  || { echo "the lifted goal did not return to its resting state" >&2; cat "$tmp/poa-late.md" >&2; exit 1; }
grep -q " unpark actor=fixture-machine+fixture-lineage targets=poa-late authorityOutcome=POWER_OF_ATTORNEY authorityRuling=$lift reason=verified: 1.2 is on the vendor's page" "$tmp/poa-late.md" \
  || { echo "the attorney unpark's history line does not carry the entry and what was verified" >&2; cat "$tmp/poa-late.md" >&2; exit 1; }
# poa-medium is tier 2 (approved under the tiers 1,2 entry above): the
# ruling grants the unpark for tier-1 goals only, whatever the entry covers.
"$ms" goal park --root "$clone" --id poa-medium --by Wido --because "the person pauses a tier-2 goal" --fixture-human-authority >/dev/null
if tier_lift=$("$ms" goal unpark --root "$clone" --id poa-medium --under "$lift" --verified "it holds" 2>&1); then
  echo "a tier-2 person park was lifted under attorney: $tier_lift" >&2; exit 1
fi
grep -q 'tier-1 goals only' <<<"$tier_lift" \
  || { echo "the tier refusal does not name the ruling's scope: $tier_lift" >&2; exit 1; }
# poa-small is the seat's claimed tier-1 goal: a defect opened against it
# parks it behind the blocker, and no entry lifts that park (R-93-m1e).
"$ms" goal open --root "$clone" --id poa-defect --blocks poa-small \
  --intent "The defect that blocks poa-small." --next "Fix it." \
  --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis "power of attorney fixture" >/dev/null
if blocker_lift=$("$ms" goal unpark --root "$clone" --id poa-small --under "$lift" --verified "the defect is fixed" 2>&1); then
  echo "a blocker park was lifted under attorney: $blocker_lift" >&2; exit 1
fi
grep -q 'returns by itself' <<<"$blocker_lift" \
  || { echo "the blocker refusal does not say the park returns by itself: $blocker_lift" >&2; exit 1; }
export METASYSTEM_GOAL_NOW=2026-08-26T09:00:00Z
if late_out=$("$ms" goal approve --root "$clone" --id poa-late --under "$entry" 2>&1); then
  echo "an expired entry approved a goal: $late_out" >&2; exit 1
fi
grep -q 'expired 2026-08-25' <<<"$late_out" \
  || { echo "the expiry refusal does not name the date: $late_out" >&2; exit 1; }
unset METASYSTEM_GOAL_NOW
fi
