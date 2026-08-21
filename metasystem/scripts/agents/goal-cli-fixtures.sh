#!/usr/bin/env bash
set -euo pipefail

# The F17 fold, shell half: the goal CLI verbs proven end to end
# through the REAL binary in a two-repository sandbox — source
# digest, migration, the read-side fetch, the identity-preserving
# rerun, and recovery. This also certifies the F16 lineage fix at
# the surface it matters: METASYSTEM_OWNER_LINEAGE must reach the
# synthesized claim record, not collapse to the literal "session".

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "goal-cli fixtures: bin/metasystem is not built" >&2; exit 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-goal-cli.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

origin="$tmp/origin.git"
clone="$tmp/clone"
git init -q --bare "$origin"
git init -q -b main "$clone"
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
# the migration precondition proves the digest matches goals.md.
python3 - "$clone/plans/goals.md" "$clone/plans/goals-accepted.json" <<'PY'
import hashlib, json, sys
ledger = open(sys.argv[1]).read()
json.dump({"schemaVersion": 1, "ledger": ledger,
           "sha256": hashlib.sha256(ledger.encode()).hexdigest()}, open(sys.argv[2], "w"))
PY
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

echo "goal CLI fixtures: PASSED"
