#!/usr/bin/env bash
# Apply the approved report order through the shipped human-terminal verb.
set -euo pipefail

rank_engine='/Users/wido/LocalStorage/GitHub/agentic-tools-backlog-plan-20260907/metasystem/artifacts/backlog-rank-20260908/metasystem'
rank_root='/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem'
rank_plan='/Users/wido/LocalStorage/GitHub/agentic-tools/development/backlog-ranks-ready-2026-09-08.tsv'
rank_evidence=$(mktemp -d /tmp/metasystem-rank-application.XXXXXX)
rank_current='preflight'
trap 'rank_exit=$?; if (( rank_exit != 0 )); then printf "Stopped at %s; exit %s. Published ranks, if any, remain valid. Evidence: %s\n" "$rank_current" "$rank_exit" "$rank_evidence" >&2; fi' EXIT

[[ -x "$rank_engine" && -r "$rank_plan" ]] || {
  printf 'The prepared engine or ranking table is missing. No ranks changed.\n' >&2
  exit 1
}
printf 'Applying the preserved backlog order. Evidence: %s\n' "$rank_evidence"
"$rank_engine" goal fetch --root "$rank_root"
"$rank_engine" goal list --root "$rank_root" > "$rank_evidence/before.json"

rank_counts=(0 0 0 0)
rank_applied=0
rank_skipped=0
while IFS=$'\t' read -r rank_priority rank_old_sequence rank_id rank_rest; do
  [[ "$rank_priority" != priority ]] || continue
  rank_current=$rank_id
  case "$rank_priority" in 1|2|3) ;; *) printf 'Invalid priority in prepared table: %s\n' "$rank_priority" >&2; exit 1 ;; esac
  "$rank_engine" goal show --root "$rank_root" --id "$rank_id" > "$rank_evidence/current.json"
  rank_state=$("$rank_engine" json get --file "$rank_evidence/current.json" --field goal.State)
  if [[ "$rank_state" == done ]]; then
    printf 'Already concluded; skipping %s\n' "$rank_id"
    rank_skipped=$((rank_skipped + 1))
    continue
  fi
  rank_counts[$rank_priority]=$(( ${rank_counts[$rank_priority]} + 1 ))
  rank_sequence=${rank_counts[$rank_priority]}
  printf 'Set %s.%02d: %s\n' "$rank_priority" "$rank_sequence" "$rank_id"
  "$rank_engine" goal set-priority --root "$rank_root" --by Wido \
    --id "$rank_id" --priority "$rank_priority" --sequence "$rank_sequence" \
    >> "$rank_evidence/operations.jsonl"
  rank_applied=$((rank_applied + 1))
done < "$rank_plan"

rank_current='final verification'
"$rank_engine" goal fetch --root "$rank_root"
"$rank_engine" goal list --root "$rank_root" > "$rank_evidence/after.json"
rank_counts=(0 0 0 0)
rank_verified=0
while IFS=$'\t' read -r rank_priority rank_old_sequence rank_id rank_rest; do
  [[ "$rank_priority" != priority ]] || continue
  "$rank_engine" goal show --root "$rank_root" --id "$rank_id" > "$rank_evidence/current.json"
  rank_state=$("$rank_engine" json get --file "$rank_evidence/current.json" --field goal.State)
  [[ "$rank_state" != done ]] || continue
  rank_counts[$rank_priority]=$(( ${rank_counts[$rank_priority]} + 1 ))
  rank_sequence=${rank_counts[$rank_priority]}
  rank_actual_priority=$("$rank_engine" json get --file "$rank_evidence/current.json" --field goal.Priority)
  rank_actual_sequence=$("$rank_engine" json get --file "$rank_evidence/current.json" --field goal.Sequence)
  if [[ "$rank_actual_priority" != "$rank_priority" || "$rank_actual_sequence" != "$rank_sequence" ]]; then
    printf 'Rank changed during application: %s, expected %s:%s, found %s:%s. Rerun to reconcile the remaining live order.\n' \
      "$rank_id" "$rank_priority" "$rank_sequence" "$rank_actual_priority" "$rank_actual_sequence" >&2
    exit 1
  fi
  rank_verified=$((rank_verified + 1))
done < "$rank_plan"
"$rank_engine" goal list --root "$rank_root" --pretty > "$rank_evidence/ordered-backlog.md"
printf 'Verified ranks for %s current goals; %s already-concluded goals skipped during application.\n' "$rank_verified" "$rank_skipped"
printf 'Full readback: %s/ordered-backlog.md\n' "$rank_evidence"
