# Review brief: wall-o19 recovery ladder

Round budget: 3 focused rounds — agreed before round one; exhaustion
follows the critique skills' budget rules, never a silent round 4.

Threat model: one trusted human operator; no external adversaries.
IN SCOPE: accidents and crashes around violation recovery (a crash
mid-restore, a re-verification that lies, a repeat pattern hidden by
silent recovery); a mission host gaming the ladder — engineering a
violation whose automatic recovery would launder bytes or whose
repetition stays invisible; forged or stale safe-tree/ledger-blob
records. OUT OF SCOPE: the unbuilt isolation tier (D100 ruling 2),
runtime compromise, hostile third parties, and the sealed-dirty
composition (wall-o14 — stopped and raised separately; this design
must not depend on its outcome).

Appetite: 6h for this design; findings whose fixes exceed it pause
and go to the human.

Scope: the tier structure below; ResolveTaint's split into an engine
core; the runner's tier-2 recovery pass inside the park flow; the
repeat-offense derivation; event registration and emission; the
fixture set. OUT: any new state schema fields; any change to the
human resolve-taint verb's CLI contract; recovery of anything but
workspace and ledger domains.

Return format: numbered findings, most severe first, each with
file, rule, and the concrete failure it causes; or AGREE with
observations that do not gate.

---

# Design: the wall recovery ladder (revision 1, from the D117 draft)

Wido's ruling, verbatim intent: wall violations must not freeze the
mission for the human by default. The machinery first figures out
how to recover; the human is asked only on ambiguity or big
implications.

## Tier 1 — record (unchanged from the landed slice-6 doctrine)

Every violation books its taint entry (the mint returns the id and
the taint-set event carries it), writes wall.json with unaccounted
paths, and anchors the disputed tree. The audit trail is identical
whether recovery is automatic or human.

## Tier 2 — automatic recovery (the new runner authority)

Runs in the SAME runner pass as the park, after the park's state
proposal lands and before any ask is raised.

1. INSPECTION FIRST, AS ITS OWN RECORDED STEP. The runner evaluates
   mechanical recoverability and emits the already-registered
   recovery-inspected event with verdict restorable | unrestorable
   | adoption-question, before attempting anything. An unrestorable
   or adoption verdict falls through to tier 3 with the verdict on
   the ask.
2. WORKSPACE RESTORE: the recorded safe tree (the violated turn's
   pre-tree / current expected point) exists as an authenticated
   anchor and git can materialize it. After materialization the
   runner re-verifies byte-exact equality through the SAME
   recordedSafeTree + observed==tree path the human verb uses —
   tier 2 calls ResolveTaint's engine core with
   resolvedBy="runner:auto-restore", never a parallel code path.
   The engine core is the existing function body behind the verb;
   the human/CLI entry keeps its authority classification, the
   in-process runner entry records runner identity instead. One
   body, two authenticated entries.
3. LEDGER RESTORE: the anchored blob exists and authenticates under
   the landed slice-6 predicate; bytes rewrite from the blob through
   the same core.
4. ON SUCCESS: the resolution records variant=restore with the
   runner identity and a reason naming the violation; the segment
   advances; the mission CONTINUES; no ask exists. A registered
   wall-auto-recovered event carries the taint id and the restored
   domain(s). Restoring discards the violating bytes by
   construction — an engineered violation gains nothing from its
   own restoration, which is why tier 2 can be automatic at all.
5. ON ANY FAILURE — materialization error, re-verification
   mismatch, crash — the taint stays unresolved and tier 3 raises;
   a crash between restore and resolution record leaves the taint
   unresolved and the run-mode taint STOP (already landed) refuses
   every run mode at resume, so a half-recovery cannot be ridden.

## Tier 3 — human escalation (narrowed, not removed)

An ask is raised and the mission stays parked exactly when:
1. adoption is the question — waiving attribution is inherently
   human;
2. no mechanically verifiable restore exists (no authenticated
   safe tree or blob; materialization fails; re-verification
   fails);
3. REPEAT OFFENSE: the same mission suffers another violation
   within K=1 turns of an auto-recovery — the very next turn. A
   pattern is a judgment call, not an accident. The ask names the
   prior auto-recovery's taint id so the human sees the pattern.

The repeat-offense predicate derives from the hash-chained taint
entries alone: an unresolved-now entry whose predecessor entry
carries resolution.resolvedBy with the "runner:" prefix and a
turnId within the lookback window. No new state fields.

The human resolve-taint verb is byte-identical in contract for
every tier-3 case.

## Events

- recovery-inspected (registered, emission lands here): verdict +
  turnId, one per violation, before any attempt.
- wall-auto-recovered (registered in this change): taintId +
  restored domains + turnId. Records remain the authority.

## Doctrine edits in the same landing

- hiw-critique-r2 §4's "human-reserved" amends to the ladder,
  recorded as Wido's ruling with its date.
- The HIW-O6 row gains the tier-2 fixtures.

## Fixture obligations (the arbiter)

- F1: solo-build offense → park → same-pass inspection
  (recovery-inspected restorable) → auto-restore → byte-exact
  re-verification → mission continues → NO ask exists.
- F2: ledger-domain auto-restore from the authenticated blob.
- F3: adoption question escalates: ask raised, human verb resolves,
  ladder never touches it.
- F4: no-safe-tree shape escalates with verdict unrestorable.
- F5: repeat offense: offense → auto-restore → next-turn offense →
  ask raised naming the prior auto-recovery's taint id; the human
  verb works unchanged.
- F6: crash between restore and resolution record: taint stays
  unresolved, resume refuses through the landed taint STOP, no
  double-restore on the next pass.
- F7: the runner's tier-2 entry cannot be reached from the CLI:
  the classification gate still refuses non-human callers of the
  verb (regression of the landed authority matrix).
- F8: re-verification mismatch (tampered anchor content) refuses
  the auto-restore and escalates — the restore trusts the
  verification, never the anchor's name.

---

# ROUND 1 STATE: nine material findings — STOPPED AND RAISED

Codex round one (archived under artifacts/agents/critiques/wall-o19/)
returned nine material findings, four CRITICAL, all transaction- or
invariant-level: no crash-safe park-recover-ask ordering exists (the
landed park writes asks first); the landed taint STOP contradicts the
promised crash escalation; the park can anchor a poisoned ledger that
tier 2 would then faithfully restore; "inside the park flow" is four
different detection phases needing a phase table and a typed outcome;
the destructive restore target is ambiguous exactly where choosing
wrongly loses accepted work; the adoption verdict has no derivable
predicate; the K=1 repeat window cannot be derived from taint entries
as they exist; the runner: prefix is presentation text, not
provenance; and events are best-effort while the design leaned on one
as evidence.

The stop is not only the appetite. Two findings embed decisions that
are HUMAN-RESERVED:

1. THE ADOPTION POLICY (O19-R3-06): for a restorable violation that
   might contain valuable work, does the inspector always discard
   (restore, never ask) or always ask? This decides how often the
   human is interrupted — the exact quantity D117 rules on — and no
   recorded fact can derive it.
2. RUNNER PROVENANCE (O19-R3-08): distinguishing automatic from
   human resolutions authentically either reserves a prefix in the
   human CLI's --by (a contract change) or adds an authenticated
   actor field (a schema change); both cross lines the scope
   declared closed.

Recommendation for re-scoping when Wido rules: fold the nine
findings under a fresh brief with the phase table and the ledger
transaction as the design's spine, appetite a full day (the o14
lesson: wall design rows are systematically bigger than their
original tokens). The chain stands at one round spent; any
successor enumerates all nine findings.
