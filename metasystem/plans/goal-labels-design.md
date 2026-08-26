# Labels on the ledger (goal-labels)

Working Mode: design

Owner: m2 coordinator brief, v2 after round 1
(design-critic-20260826t043024z-3773: 6 material, all folded
below). Failsafe at round 2 — the next round lands this with any
residue. Codex builds; one certification.

## Intent (Wido 2026-08-24)

Backlog items carry zero or more labels so they can be grouped and
listed by label. First real use: label the custody/stewardship
family from the 2026-08-24 collateral day so the group lists as one.

## Shape

- Goal record gains `Labels []string` — zero or more label tokens
  under a NEW, EXPLICIT grammar (r1-001: the id grammar is not
  reused — it allows 100 chars and shapes labels should not):
  `^[a-z][a-z0-9-]{0,31}$`, stated once as labelRe beside the id
  grammar. Zero labels is the default and never an error; the
  CANONICAL form is sorted and deduplicated, produced ONLY by verb
  writes.
- Verbs: `goal open`/`goal edit` accept repeatable `--label x`
  (additive on edit) and `--unlabel x` (edit only); `goal list` and
  `goal next` gain `--label x` filters (AND across repeats). An
  unknown-grammar label refuses with the grammar named.
- DELTA CONTRACT (r1-004): one edit computes final = (current ∪
  adds) ∖ removes; the same token in both adds and removes REFUSES
  as contradictory; a final set equal to current follows the
  existing no-change-edit behavior exactly (the builder matches
  editDeltas' shipped semantics and pins it in a fixture); the
  history delta records the label set change like any field delta,
  and concurrent publishers resolve through the existing reconcile
  conflict law — labels add no new merge machinery.
- NEXT-FILTER LAW (r1-006): --label narrows the RECOMMENDATION
  CANDIDATE SET only. It never alters the claim quota, blocker
  gating, arc ordering, or the continue-your-claimed-goal rule — a
  held claim still answers first regardless of any filter, and a
  filter that empties the candidate set reports exactly the empty
  recommendation, never an unclaim.
- File format: a `- Labels: a, b` line, absent when empty (existing
  files parse as zero labels — no migration). MAP PRECEDENT
  (r1-002): Labels follows the BLOCKED field's handling in the
  closed publish/reconcile map — a list field carried
  absent-when-empty, hand edits flowing through goal reconcile's
  lawful republication (never Pinned's hand-edit refusal, never
  Arc's dedicated-verb shape); the builder mirrors Blocked's map
  code paths one-for-one.
- PARSE/CANONICALIZE SPLIT (r1-003): the parser preserves the raw
  order and duplicates it reads (so reconcile can OBSERVE a
  noncanonical hand edit and republish it canonically, and the
  Integrity sha binds the bytes as written); canonicalization
  happens only in verb writes and reconcile republication.
- RECOVERY (r1-005): the durable-recovery path and goal open
  --claim both carry Labels — named here so the builder touches
  recover.go and the open-claim path, with fixtures for each.
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
3b. Hand-edit law: an unsorted/duplicated Labels line survives parse
   raw, reconcile republishes it canonical; add/remove contradiction
   refuses; recovery and open --claim carry labels (r1-003/004/005
   fixtures).
3c. goal next: a held claim answers first despite a non-matching
   --label filter; an empty filtered candidate set reports empty
   (r1-006 fixture).
4. First use executed: the custody family (suite-custody,
   suite-dispatch-exclusion, return-recollection-on-process-lost,
   suite-outcomes-as-steward-incidents, incident-proposal-drafting,
   continuous-self-improvement, goal-labels itself,
   critic-workspace-custody) labeled `custody` via the new flag,
   and `goal list --label custody` shows exactly the family.
