#!/usr/bin/env bash
set -euo pipefail

# The F17 fold, shell half: the goal CLI verbs proven end to end
# through the REAL binary in a two-repository sandbox — source
# digest, migration, the read-side fetch, the identity-preserving
# rerun, and recovery. This also certifies the F16 lineage fix at
# the surface it matters: METASYSTEM_OWNER_LINEAGE must reach the
# synthesized claim record, not collapse to the literal "session".

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$root/scripts/agents/fixture-budget.sh"
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "goal-cli fixtures: bin/metasystem is not built" >&2; exit 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-goal-cli.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

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
git -C "$clone" add plans scripts
git -C "$clone" -c user.name=fixture -c user.email=fixture@example.invalid commit -qm "legacy ledger"
git -C "$clone" push -q origin main

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

# 11. The atomic open-and-claim path carries labels after the quota is
# free, completing the command-line carriage leg.
open_claim=$("$ms" goal open --root "$clone" --id claimed-label \
  --intent "Claimed with its group." --next "Continue." --claim --label custody)
grep -q '"outcome":"confirmed"' <<<"$open_claim" \
  || { echo "open --claim with a label did not confirm: $open_claim" >&2; exit 1; }
claim_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$claim_tip:plans/goals/claimed-label.md" >"$tmp/claimed-label.md"
grep -q '^- Labels: custody$' "$tmp/claimed-label.md" \
  || { echo "open --claim dropped its label" >&2; cat "$tmp/claimed-label.md" >&2; exit 1; }

# 12. Appetite checkpoints use a deterministic command clock. The claim
# snapshots 4h; claimant prose cannot move it, a human-attributed edit can,
# and the latest estimate adds and can then clear only the forecast STOP.
"$ms" goal release --root "$clone" --id claimed-label >/dev/null
export METASYSTEM_GOAL_NOW=2026-08-20T00:00:00Z
"$ms" goal open --root "$clone" --id appetite-check \
  --intent "Exercise appetite checkpoints." \
  --next "Appetite: 4h run the bounded work." --claim >/dev/null
claim_appetite_tip=$(git -C "$origin" rev-parse main)
git -C "$clone" cat-file -p "$claim_appetite_tip:plans/goals/appetite-check.md" >"$tmp/appetite-claim.md"
grep -q '^- Claimed: .* appetite=4h$' "$tmp/appetite-claim.md" \
  || { echo "claim did not snapshot the parsed appetite" >&2; cat "$tmp/appetite-claim.md" >&2; exit 1; }

export METASYSTEM_GOAL_NOW=2026-08-20T03:00:00Z
[[ -z $("$ms" goal banners --root "$clone") ]] \
  || { echo "within-band appetite emitted a banner" >&2; exit 1; }
export METASYSTEM_GOAL_NOW=2026-08-20T04:30:00Z
escalate=$("$ms" goal banners --root "$clone")
grep -q '^BREACH-ESCALATE: goal appetite-check ' <<<"$escalate" \
  || { echo "the grace band did not escalate: $escalate" >&2; exit 1; }
export METASYSTEM_GOAL_NOW=2026-08-20T05:01:00Z
stop=$("$ms" goal banners --root "$clone")
grep -q '^BREACH-STOP: goal appetite-check ' <<<"$stop" \
  || { echo "past-grace work did not stop: $stop" >&2; exit 1; }
show_stop=$("$ms" goal show --root "$clone" --id appetite-check)
grep -q 'BREACH-STOP: goal appetite-check ' <<<"$show_stop" \
  || { echo "goal show did not carry the touched goal's STOP banner: $show_stop" >&2; exit 1; }
measured_estimate=$("$ms" goal estimate --root "$clone" --id appetite-check --remaining 1m)
grep -q '^BREACH-STOP: goal appetite-check ' <<<"$measured_estimate" \
  || { echo "a small estimate cleared a measured STOP: $measured_estimate" >&2; exit 1; }

"$ms" goal edit --root "$clone" --id appetite-check \
  --next "Appetite: 20h claimant prose has no authority." >/dev/null
claimant_stop=$("$ms" goal banners --root "$clone")
grep -q '^BREACH-STOP: goal appetite-check ' <<<"$claimant_stop" \
  || { echo "claimant edit moved the claim-time threshold: $claimant_stop" >&2; exit 1; }
"$ms" goal edit --root "$clone" --id appetite-check --by wido \
  --next "Appetite: 8h Wido raises the appetite." >/dev/null
[[ -z $("$ms" goal banners --root "$clone") ]] \
  || { echo "human appetite raise did not return within-band" >&2; exit 1; }

estimate_stop=$("$ms" goal estimate --root "$clone" --id appetite-check --remaining 6h)
grep -q '^BREACH-STOP: goal appetite-check ' <<<"$estimate_stop" \
  || { echo "remaining estimate did not tighten to STOP: $estimate_stop" >&2; exit 1; }
estimate_within=$("$ms" goal estimate --root "$clone" --id appetite-check --remaining 4h)
if grep -q 'BREACH-STOP' <<<"$estimate_within"; then
  echo "a latest within-band estimate did not clear the forecast STOP: $estimate_within" >&2; exit 1
fi

# A stopped claim elsewhere is a banner, not a backlog-wide claim refusal.
"$ms" goal estimate --root "$clone" --id appetite-check --remaining 6h >/dev/null
other="$tmp/other"
env -u GIT_OBJECT_DIRECTORY -u GIT_ALTERNATE_OBJECT_DIRECTORIES git clone -q "$origin" "$other"
git -C "$other" config metasystem.goal.machine fixture-other
mkdir -p "$other/scripts/agents"
cp "$root/scripts/agents/pre-commit-guard.sh" "$other/scripts/agents/"
other_claim=$(cd "$other" && METASYSTEM_OWNER_LINEAGE=other-lineage \
  "$ms" goal claim --root "$other" --id plain-goal)
grep -q '"outcome":"confirmed"' <<<"$other_claim" \
  || { echo "another goal's STOP refused a lawful claim: $other_claim" >&2; exit 1; }
grep -q '^BREACH-STOP: goal appetite-check ' <<<"$other_claim" \
  || { echo "claim elsewhere did not surface the standing STOP: $other_claim" >&2; exit 1; }
unset METASYSTEM_GOAL_NOW

echo "goal CLI fixtures: PASSED"
