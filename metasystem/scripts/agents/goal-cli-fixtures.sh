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

fixture_bed_parent_log_root=
fixture_bed_parent_child_pid=
fixture_bed_parent_cleanup() {
  local status=$?
  trap - EXIT HUP INT QUIT TERM
  if [[ -n "$fixture_bed_parent_child_pid" ]]; then
    kill -TERM "$fixture_bed_parent_child_pid" 2>/dev/null || true
    wait "$fixture_bed_parent_child_pid" 2>/dev/null || true
  fi
  [[ -z "$fixture_bed_parent_log_root" ]] \
    || rm -rf "$fixture_bed_parent_log_root" 2>/dev/null || true
  return "$status"
}

run_fixture_bed_scenarios() { # bed name, success line, script, scenario names...
  local bed=$1 success_line=$2 script=$3 log_root scenario capability log rc index=0
  local failed_names=() failed_rcs=() failed_logs=()
  shift 3
  log_root=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-${bed}-scenarios.XXXXXX")
  fixture_bed_parent_log_root=$log_root
  trap fixture_bed_parent_cleanup EXIT
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 131' QUIT
  trap 'exit 143' TERM
  for scenario in "$@"; do
    log=$log_root/$index.log
    capability=$(harness_fixture_bed_mint_capability "$log_root" "$index" "$scenario")
    echo "$bed fixture scenario started: $scenario" >&2
    "$script" --fixture-bed-child "$scenario" "$capability" >"$log" 2>&1 &
    fixture_bed_parent_child_pid=$!
    set +e
    wait "$fixture_bed_parent_child_pid"
    rc=$?
    set -e
    fixture_bed_parent_child_pid=
    cat "$log"
    if [[ $rc -eq 0 ]]; then
      echo "$bed fixture scenario passed: $scenario" >&2
    else
      failed_names+=("$scenario")
      failed_rcs+=("$rc")
      failed_logs+=("$log")
      echo "$bed fixture scenario failed: $scenario (rc=$rc); continuing" >&2
    fi
    index=$((index + 1))
  done
  if (( ${#failed_names[@]} )); then
    echo "=== $bed failed scenarios ===" >&2
    for ((index = 0; index < ${#failed_names[@]}; index++)); do
      echo "- ${failed_names[$index]} (rc=${failed_rcs[$index]})" >&2
      echo "  output tail:" >&2
      tail -n 40 "${failed_logs[$index]}" | sed 's/^/    /' >&2
    done
    echo "=== end $bed failed scenarios ===" >&2
    rm -rf "$log_root"
    exit 1
  fi
  rm -rf "$log_root"
  echo "$success_line"
  exit 0
}

if (( ! fixture_bed_child )); then
  fixture_bed_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios goal-cli "goal CLI fixtures: PASSED" \
    "$fixture_bed_script" migration-recovery labels-and-filtering structured-budget scope-bounds archive-and-prune
fi
case "$fixture_scenario" in
  migration-recovery | labels-and-filtering | structured-budget | scope-bounds | archive-and-prune) ;;
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
cleanup() {
  local status=$? keep
  if [[ $status -ne 0 && -d "$tmp" ]]; then
    keep="$root/artifacts/agents/suite-failures/$(date -u +%Y%m%dT%H%M%SZ)-goal-cli-$$"
    mkdir -p "$(dirname "$keep")"
    mv "$tmp" "$keep" 2>/dev/null \
      && echo "goal CLI fixture evidence preserved: $keep" >&2
    return 0
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
"$ms" lease announce --root "$clone" --session goal-cli-fixture \
  --pid "$$" --start "$fixture_start" --tag goal-cli-fixture \
  --runtime fake --owner-lineage fixture-lineage >/dev/null

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

if [[ "$fixture_scenario" == labels-and-filtering ]]; then
# 6. Label writes are canonical whole fields. Open accepts repeated
# labels, sorts and deduplicates them, while an unlabeled open keeps
# the field absent.
export METASYSTEM_OWNER_LINEAGE=fixture-lineage
open_labels=$("$ms" goal open --root "$clone" --id labeled-one \
  --intent "First labeled goal." --next "Continue." \
  --label beta --label alpha --label beta)
grep -q '"outcome":"confirmed"' <<<"$open_labels" \
  || { echo "goal open with labels did not confirm: $open_labels" >&2; exit 1; }
"$ms" goal open --root "$clone" --id plain-goal \
  --intent "An unlabeled goal." --next "Continue." >/dev/null
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
  --intent "Must refuse." --next "Stop." --label Bad_Label 2>&1); then
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
"$ms" goal open --root "$clone" --id labeled-two \
  --intent "Second labeled goal." --next "Continue." \
  --label shared --label alpha >/dev/null
one_filter=$("$ms" goal list --root "$clone" --pretty --label shared)
grep -q '^  labeled-one' <<<"$one_filter" && grep -q '^  labeled-two' <<<"$one_filter" \
  || { echo "one-label list filtering lost a match: $one_filter" >&2; exit 1; }
if grep -q '^  plain-goal' <<<"$one_filter"; then
  echo "a zero-label goal appeared in a filtered list" >&2; exit 1
fi
two_filters=$("$ms" goal list --root "$clone" --pretty --label alpha --label shared)
grep -q '^  labeled-one' <<<"$two_filters" && grep -q '^  labeled-two' <<<"$two_filters" \
  || { echo "two-label AND filtering lost a match: $two_filters" >&2; exit 1; }
"$ms" goal open --root "$clone" --id and-a \
  --intent "Carries only a." --next "Continue." --label a >/dev/null
"$ms" goal open --root "$clone" --id and-ab \
  --intent "Carries a and b." --next "Continue." --label a --label b >/dev/null
and_probe=$("$ms" goal list --root "$clone" --pretty --label a --label b)
and_ids=$(sed -n 's/^  \([a-z][a-z0-9-]*\)$/\1/p' <<<"$and_probe")
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
fi

if [[ "$fixture_scenario" == structured-budget ]]; then
"$ms" goal release --root "$clone" --id ship-widget >/dev/null
"$ms" goal open --root "$clone" --id plain-goal \
  --intent "An unlabeled goal." --next "Continue." >/dev/null
# 11. The atomic open-and-claim path carries labels after the quota is
# free, completing the command-line carriage leg.
open_claim=$("$ms" goal open --root "$clone" --id claimed-label \
  --intent "Claimed with its group." --next "Continue." --claim --label custody \
  --elapsed-limit 4h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1)
grep -q '"outcome":"confirmed"' <<<"$open_claim" \
  || { echo "open --claim with a label did not confirm: $open_claim" >&2; exit 1; }
claim_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$claim_tip:plans/goals/claimed-label.md" >"$tmp/claimed-label.md"
grep -q '^- Labels: custody$' "$tmp/claimed-label.md" \
  || { echo "open --claim dropped its label" >&2; cat "$tmp/claimed-label.md" >&2; exit 1; }

# 12. A queued goal needs no budget. At claim, the complete tuple becomes its
# only budget; an Appetite-prefixed sentence remains inert human prose.
"$ms" goal release --root "$clone" --id claimed-label >/dev/null
export METASYSTEM_GOAL_NOW=2026-08-20T00:00:00Z
"$ms" goal open --root "$clone" --id budget-check \
	--intent "Exercise structured budget admission." \
	--next "Appetite: 4h is inert human prose, not a budget." --claim \
	--elapsed-limit 8h --attempt-limit 2 --reserved-job-minutes-limit 120 --active-job-limit 1 >/dev/null
claim_budget_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$claim_budget_tip:plans/goals/budget-check.md" >"$tmp/budget-claim.md"
grep -q '^- Budget: elapsedLimit=1d attemptLimit=2 reservedJobMinutesLimit=120 activeJobLimit=1$' "$tmp/budget-claim.md" \
  || { echo "claim did not store the complete budget tuple" >&2; cat "$tmp/budget-claim.md" >&2; exit 1; }
grep -q '^- Claimed: .* revision=1' "$tmp/budget-claim.md" \
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
grep -q 'BUDGET_REFUSED: goal budget-check revision=1 admission closed: elapsedLimit' <<<"$admission_spent" \
  || { echo "the structured refusal did not name its exact limit: $admission_spent" >&2; exit 1; }
export METASYSTEM_GOAL_NOW=2026-08-20T05:01:00Z
"$ms" goal set-budget --root "$clone" --id budget-check \
  --elapsed-limit 8h --attempt-limit 3 --reserved-job-minutes-limit 180 --active-job-limit 2 >/dev/null
rebudget_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$rebudget_tip:plans/goals/budget-check.md" >"$tmp/budget-rebudget.md"
grep -q '^- Budget: elapsedLimit=1d attemptLimit=3 reservedJobMinutesLimit=180 activeJobLimit=2$' "$tmp/budget-rebudget.md" \
  || { echo "set-budget did not replace the complete tuple" >&2; cat "$tmp/budget-rebudget.md" >&2; exit 1; }
grep -q '^- Claimed: .* at=2026-08-20T05:01:00Z revision=2' "$tmp/budget-rebudget.md" \
  || { echo "set-budget did not bind the new revision and elapsed origin" >&2; cat "$tmp/budget-rebudget.md" >&2; exit 1; }

# A separate goal can claim only by supplying its own complete tuple.
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
  "$ms" goal claim --root "$other" --id plain-goal \
    --elapsed-limit 2h --attempt-limit 1 --reserved-job-minutes-limit 30 --active-job-limit 1)
grep -q '"outcome":"confirmed"' <<<"$other_claim" \
  || { echo "a complete-tuple claim did not confirm: $other_claim" >&2; exit 1; }
unset METASYSTEM_GOAL_NOW
fi

if [[ "$fixture_scenario" == scope-bounds ]]; then
"$ms" goal release --root "$clone" --id ship-widget >/dev/null

# An over-norm existing goal and the revisionless open-and-claim shortcut both
# exercise the typed refusal. The remedy must name split; no refusal fixture
# infers success from a parser-only unit.
"$ms" goal open --root "$clone" --id norm-parent \
  --intent "Hold a large intent before decomposition." --next "Split it first." >/dev/null
if norm_refusal=$("$ms" goal set-budget --root "$clone" --id norm-parent \
  --elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 1441 \
  --active-job-limit 1 2>&1); then
  echo "over-norm set-budget succeeded without strict approval" >&2; exit 1
fi
grep -q 'GOAL_NORM_REFUSED: goal norm-parent' <<<"$norm_refusal" \
  && grep -q 'goal split --id norm-parent --members' <<<"$norm_refusal" \
  || { echo "the norm refusal did not name its type and split remedy: $norm_refusal" >&2; exit 1; }
if open_claim_refusal=$("$ms" goal open --root "$clone" --id norm-open-claim \
  --intent "Must not enter claimed over norm." --next "Stop." --claim \
  --elapsed-limit 1d --attempt-limit 2 --reserved-job-minutes-limit 1441 \
  --active-job-limit 1 2>&1); then
  echo "over-norm open --claim succeeded" >&2; exit 1
fi
grep -q 'GOAL_NORM_REFUSED: goal norm-open-claim' <<<"$open_claim_refusal" \
  && grep -q 'open it queued' <<<"$open_claim_refusal" \
  || { echo "open --claim did not exercise its three-step refusal: $open_claim_refusal" >&2; exit 1; }

# The real split command parses a closed draft, publishes both members and the
# parent conclusion atomically, and retires the parent identifier permanently.
"$ms" goal open --root "$clone" --id split-parent \
  --intent "Deliver the two-part fixture." --next "Atomize it." --label fixture >/dev/null
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
  --intent "Illicit resurrection." --next "Stop." 2>&1); then
  echo "a pruned decomposed parent id was recreated" >&2; exit 1
fi
grep -q 'goal id split-parent is retired' <<<"$recreate_refusal" \
  || { echo "the decomposition registry did not survive prune: $recreate_refusal" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == archive-and-prune ]]; then
"$ms" goal release --root "$clone" --id ship-widget >/dev/null
METASYSTEM_GOAL_NOW=2026-08-20T00:00:00Z \
  "$ms" goal open --root "$clone" --id budget-check \
    --intent "Exercise structured budget admission." \
    --next "Continue." --claim --elapsed-limit 8h --attempt-limit 2 \
    --reserved-job-minutes-limit 120 --active-job-limit 1 >/dev/null
# 13. Concluding writes the records-owned archive, reopening records a
# ledger move back to the live set, and concluding again preserves the
# canonical record bytes including its Integrity line.
"$ms" goal open --root "$clone" --id archive-roundtrip \
  --intent "Exercise concluded-goal archival." --next "Conclude it." >/dev/null
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
  "$ms" goal open --root "$clone" --claim --id admission-concluded \
    --intent "Prove admission consumes records-owned conclusions." \
    --next "Conclude after its budget is exhausted." \
    --elapsed-limit 4h --attempt-limit 1 \
    --reserved-job-minutes-limit 30 --active-job-limit 1 >/dev/null
set +e
admission_before=$(METASYSTEM_GOAL_NOW=2026-08-20T16:00:00Z \
  "$ms" job goal-admission --root "$clone" --stop-lineage fixture-lineage 2>&1)
admission_before_rc=$?
set -e
[[ "$admission_before_rc" -eq 10 ]] \
  || { echo "the exhausted live goal did not request breach-stop before conclusion (rc=$admission_before_rc): $admission_before" >&2; exit 1; }
grep -q 'BUDGET_REFUSED: goal admission-concluded revision=1 admission closed: elapsedLimit' <<<"$admission_before" \
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
