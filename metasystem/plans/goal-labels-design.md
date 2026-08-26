# Labels on the ledger (goal-labels)

Working Mode: design

Owner: m2 coordinator brief. Appetite 3h (Wido-stamped). Single
design round (a schema/verb extension with an existing round-trip
law), failsafe at round 2; codex builds; one certification.

## Intent (Wido 2026-08-24)

Backlog items carry zero or more labels so they can be grouped and
listed by label. First real use: label the custody/stewardship
family from the 2026-08-24 collateral day so the group lists as one.

## Shape

- Goal record gains `Labels []string` — zero or more short
  kebab-case tokens (the goal-id grammar, reused: lowercase
  alphanumerics and dashes, 1..32). Zero labels is the default and
  never an error; label ORDER is canonical (sorted, deduplicated at
  write) so publish/reconcile round-trips are byte-stable.
- Verbs: `goal open`/`goal edit` accept repeatable `--label x`
  (additive on edit) and `--unlabel x` (edit only); `goal list` and
  `goal next` gain `--label x` filters (AND across repeats). An
  unknown-grammar label refuses with the grammar named.
- File format: a `- Labels: a, b` line in the goal file, absent when
  empty (existing files without the line parse as zero labels — no
  migration); the publish/reconcile map carries the field with the
  same absent-when-empty rule so the canonical branch stays clean.
- History: label changes are `edit` history entries like any field
  delta; no new verb class.

## Constraints

- Round-trip law: open-with-labels → publish → fetch → reconcile
  reproduces the sorted label set byte-for-byte; a hand-edited
  unsorted line canonicalizes on the next lawful verb, not on read.
- Unlabeled goals are untouched bytes (no migration churn on the
  ledger).
- The label grammar is the goal-id grammar — one grammar, no new
  regexes.

## Acceptance (fast tests)

1. Fixtures: open with two labels (stored sorted-deduped); edit
   add/remove; filter list/next by one and by two labels (AND);
   zero-label goals valid everywhere; bad grammar refuses.
2. Round-trip fixture through publish/reconcile per the law.
3. Existing goal-cli fixtures green unchanged (no-migration proof).
4. First use executed: the custody family (suite-custody,
   suite-dispatch-exclusion, return-recollection-on-process-lost,
   suite-outcomes-as-steward-incidents, incident-proposal-drafting,
   continuous-self-improvement, goal-labels itself,
   critic-workspace-custody) labeled `custody` via the new flag,
   and `goal list --label custody` shows exactly the family.
