# The wall recovery ladder (Wido's 2026-08-19 ruling; slice 8 draft)

## The ruling, verbatim intent
Wall violations must not freeze the mission for the human by default.
The main agent's machinery first figures out how to recover; the human
is asked only when there is ambiguity or big implications.

## What changes (amends slice-6 doctrine)
Slice 6 made ALL resolution human-reserved. Under the ladder, the
RUNNER owns tier-2 recovery; the human owns tier-3 judgment.

**Tier 1 — record (unchanged).** Every violation still books its
taint entry, writes wall.json evidence with unaccounted paths, and
anchors the disputed tree. Nothing is ever lost; the audit trail is
identical whether recovery is automatic or human.

**Tier 2 — automatic recovery (new).** Immediately after the park
lands (same runner pass, before any ask is raised), the runner
attempts MECHANICAL recovery, defined narrowly:
- workspace restore: the recorded safe tree exists (the violated
  turn's pre-tree / current E-point), and `git` can materialize it;
  after materialization the runner re-verifies byte-exact equality
  through the same recordedSafeTree + observed==tree path the human
  verb uses — recovery reuses ResolveTaint's engine internals with
  resolvedBy="runner:auto-restore", never a parallel code path.
- ledger restore: the anchored blob exists and authenticates (the
  slice-6 predicate); bytes are rewritten from the blob.
On success: resolution recorded (variant=restore, runner identity,
reason naming the violation), segment advances, mission continues.
NO ASK IS RAISED. A registered event (wall-auto-recovered) carries
the taint id and what was restored.

**Tier 3 — human escalation (narrowed, not removed).** An ask is
raised, and the mission stays parked, ONLY when:
1. adoption is the question — keeping disputed work waives
   attribution; inherently human;
2. no mechanically verifiable restore exists (no authenticated safe
   tree/blob; materialization fails; re-verification fails);
3. REPEAT OFFENSE: the same mission suffers another violation within
   K turns of an auto-recovery (K=1 proposed: the very next turn) —
   a pattern is a judgment call, not an accident. The ask names the
   prior auto-recovery so the human sees the pattern.
The escalation ask keeps slice-6's taint binding and the resolve-taint
verb keeps working exactly as landed for these cases.

## Mechanics to reuse (no new machinery classes)
- ResolveTaint's internals split into an engine core callable by the
  runner (classification gate applies only to the CLI/human entry;
  the runner's tier-2 call is in-process and recorded as such).
- The repeat-offense counter derives from the hash-chained taint
  entries themselves (resolution.resolvedBy prefix "runner:" within
  the lookback window) — no new state fields.
- Ask suppression/creation, waiting lists, anchors: all as landed.

## Design-doc edits
- hiw-critique-r2 §4 wording "human-reserved" amends to the ladder,
  recorded as WIDO'S RULING (not delegated) with the date.
- HIW-O6 row gains the tier-2 fixtures; a new row (or O6b) if the
  critic prefers a separate obligation for the ladder.

## Fixtures (minimum)
- auto-restore end to end: solo-build offense → park → SAME-pass
  auto-restore → mission continues → no ask exists.
- ledger-domain auto-restore from the anchored blob.
- adoption still escalates (ask raised, human verb resolves).
- repeat offense: offense → auto-restore → next-turn offense →
  ask raised naming the prior auto-recovery; resolve-taint works.
- no-safe-tree shape escalates.
