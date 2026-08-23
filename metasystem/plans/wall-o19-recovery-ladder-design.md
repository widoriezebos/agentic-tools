# The wall recovery ladder: runner-owned tier 2 (HIW-O19, D117)

- Status: STOPPED BY THE APPETITE LAW 2026-08-23 (mac-coordinator).
  Round 1 (8 findings, 3 critical) folded structurally; round 2
  returned 9 findings with 4 critical — the trajectory diverged, and
  every round-2 critical is a real transactional design (the ledger
  booking/restore/anchor transaction, the resume entry that must
  outrank the raw taint stop AND ordinary reconciliation, the
  per-detection-site phase table, the durable source binding across
  the detection-to-park crash window). No honest design of that
  weight fits the 6h token; per the backlog mechanism, the answer is
  slicing, not a stretched appetite. The round-2 findings are
  recorded below as the OPEN-QUESTION LEDGER, mapped to the slice
  proposal awaiting ratification. Everything above the ledger is the
  converged round-1 shape and remains the slices' shared foundation.
- Ruling: D117 (Wido, 2026-08-19): wall violations must not freeze the
  mission for the human by default; the machinery recovers the
  mechanical cases; the human is asked only for ambiguity or big
  implications. Amends slice-6's all-resolution-human-reserved
  doctrine as WIDO'S RULING, dated, not a delegated choice.
- Seed: plans/recovery-ladder-design-draft.md, re-grounded against the
  landed WSS machinery and corrected by review round 1.
- Loop discipline, declared at loop start: codex gpt-5.6-sol xhigh,
  read-only, both-must-agree; ABSOLUTE FAILSAFE ROUND 4, no-gain tier
  two consecutive rounds; land-with-residue on either tier.

## The ladder

**Tier 1 — record (unchanged).** Every violation books its taint,
writes wall.json evidence, anchors the disputed trees. Identical
whether recovery is automatic or human.

**Tier 2 — runner auto-recovery, as a DURABLE PHASE (O19-R1-1).** The
wall-violation park no longer writes its ask eagerly. The park's state
write carries a validator-owned PENDING-RECOVERY record; recovery runs
after that write is durable; the ask becomes the DERIVED final step of
proved ineligibility — never a side effect of parking.

**Tier 3 — human escalation (narrowed, never removed).** The ask is
raised and the park stays exactly on the PROVED-INELIGIBLE outcomes of
the table below. Adoption stays human without exception.

## The pending-recovery record (validator-owned)

Written IN the park's own state write, on wall-violation parks only:

    recovery: {
      taintId, phase,            // "workspace-gate" | "workspace-postverify" | "ledger"
      safeTree | safeLedgerSha,  // the SOURCE BINDING, captured at detection
      boundAt,                   // capture instant
      attempts: [ {at, outcome, detail} ... ]   // bounded, observable
    }

- The SOURCE BINDING is captured AT DETECTION, before the park's own
  booking or anchor can move anything (O19-R1-2): for ledger taints,
  the safe digest is the anchored ledger truth the in-turn guard
  itself read when it detected the divergence — the park anchor NEVER
  redefines the recovery source; for workspace taints, the safe tree
  is the phase-specific target below, an already-anchored chain value.
- The record is immutable except appends to `attempts` and the final
  outcome; the validator refuses a recovery record naming a resolved
  taint, an unknown phase, or a safe target that is not the recorded
  chain value for that phase.

## Phase-specific safe targets (O19-R1-5)

- `workspace-gate` (violation at the wall gate, nothing consumed): the
  safe tree is the turn's PRE-TREE — the taint's recorded baseline.
- `workspace-postverify` (violation at post-verification, acceptance
  pending): the ACCEPTANCE'S recorded post-posture is the
  chain-authoritative, already-judged target — restoring toward it
  PRESERVES the consumed authorizations; the disputed bytes are the
  motion AFTER the acceptance, not the acceptance itself. The
  concluding verification then re-runs against the same recorded
  posture, and the pending acceptance concludes through the landed
  WSS-12 lane. Restoring to the pre-tree here would discard consumed
  authorized work and is refused by the validator (wrong safe target
  for the phase).
- `ledger`: the detection-time authenticated blob digest, restored
  byte-exact; the landed CLI refusal of ledger RESTORE stays for
  humans (tree equality cannot prove a ledger; the runner path
  restores from the AUTHENTICATED blob it bound at detection, a
  stronger predicate than the one the CLI refusal guards against).

## The total outcome table (O19-R1-8)

Every tier-2 attempt lands in exactly one:

1. **SAFE-RESTORED**: materialization succeeded AND the full landed
   verification chain re-proved it (stable capture, observed==tree,
   staged==tree, judgeScope carriers, final equalTo +
   judgeCaptureIntegrity). The constrained resolution transition
   lands; the mission continues; NO ask ever exists.
2. **PROVED-INELIGIBLE** (→ tier 3, ask raised, park stays):
   adoption is the question; the bound safe source is missing or
   fails authentication; the post-restore verification REFUSED with a
   repository answer; or repeat offense. The ask names the proof.
3. **COULD-NOT-RUN** (→ contained retry, NO ask, NO human): spawn
   failures, timeouts, publication-uncertain outcomes stay inside the
   durable phase — `attempts` records each, resume re-enters
   idempotently, and a bounded attempt budget (3, then a steward
   notice — observable, never silent) keeps it from spinning. A
   machine condition is never escalated as a judgment call
   (the landed WSS could-not-run/ran-and-answered distinction,
   applied to recovery).

## Crash-boundary table (O19-R1-1, O19-R1-2)

- After park write, before recovery: resume sees parkReason
  wall-violation + recovery record with no terminal outcome → enters
  recovery BEFORE the parked-status refusal (the same resume-lane
  pattern as the landed completePendingVerification). No ask exists
  yet and none is needed for this resume to proceed.
- After materialization, before judgment: materialization is
  idempotent toward the bound target (re-running converges); judgment
  runs after; nothing was recorded, nothing is lost.
- After judgment, before resolution write: resume re-materializes and
  re-judges (cheap, idempotent); the chain is unchanged.
- After resolution write, before its anchor: the landed
  anchor-lag heal covers it (the resolution write is an ordinary
  state advance; reconciliation's one-step heal re-anchors).
- Ledger restore specifically: restore bytes (atomic file write,
  pending stamp re-recorded) THEN the resolution write; a crash
  between leaves ledger==bound-safe-bytes with the taint still
  pending-recovery → resume re-verifies bytes==bound digest
  (idempotent) and proceeds to the write. The park anchor made no
  claim about post-park ledger bytes (it is refused as a recovery
  source by construction), so no reconciliation shape breaks.
- applyPark's current ask-first ordering and swallowed wall-anchor
  failures are AMENDED for wall-violation parks: state (with
  recovery record) first, anchor next with failures surfacing into
  `attempts` as could-not-run, ask only as tier-3's derived step.
  Non-wall parks keep their landed ordering untouched.

## The constrained transition (O19-R1-3)

Runner recovery is NOT an identity-shaped ordinary resolution. The
validator admits a `resolvedBy: "runner:auto-restore"` resolution ONLY
when ALL of:

- the previous state carries a pending-recovery record for exactly
  this taint, with no terminal outcome;
- variant is restore; adoption with a runner identity refuses;
- the resolution's tree/digest EQUALS the record's bound safe target
  (the phase rule above) — the writer cannot choose a target;
- the reason equals the taint's recorded reason verbatim;
- repeat eligibility holds (below) — a repeat-offense state refuses
  the runner resolution and demands the tier-3 lane;
- the pending-acceptance condition matches the phase (a
  workspace-postverify recovery leaves the acceptance pending for the
  landed verification lane; it never closes it).

The runner entry point DERIVES every one of these from the chain; its
caller passes only the taint id. A forged runner resolution — from the
CLI, from a hand-built write, or from a confused future caller — fails
the transition on the recovery-record join, not on string taste.

## Identity namespace (O19-R1-6)

- Every human-facing entry (the resolve-taint CLI, any future verb)
  REFUSES `--by` values with the `runner:` prefix: the namespace is
  reserved at the boundary, and the runner mode assigns its identity
  internally — no parameter selects it.
- Repeat detection keys on the validated recovery record (an
  authenticated kind), never on a resolvedBy string prefix.

## The repeat rule (O19-R1-7)

Taint entries gain a validated `violationCycle` field (the reserved
cycle the wall booked the violation against — the runner knows it at
parkWallViolation time; the validator requires it on new entries and
requires it to match the booking). Repeat offense = a new violation
whose violationCycle is exactly `priorRecoveryCycle + 1`, where the
prior recovery is the newest taint resolved through a pending-recovery
record. One clean intervening cycle resets the ladder. Resume and
unrelated state writes do not move cycles and therefore cannot
manufacture or destroy adjacency. Fixtures pin: immediate next-cycle
violation escalates naming the prior recovery; one clean cycle between
recovers again; resume between the two changes nothing.

## The materialization owner (O19-R1-4)

One new primitive owns workspace materialization —
`gittree.MaterializeTree(safeTree)`:

- projection: the SAME ledger-filtered workspace projection the wall
  judges (the safe tree IS such a projection — chain trees are);
  materialization writes tracked content, modes, and symlinks from
  the tree; paths present in the worktree but absent from the tree
  and UNTRACKED are preserved (the wall's identity never covered
  them); tracked-but-absent paths are removed; the mission ledger
  path is NEVER touched by workspace materialization (it is
  filter-excluded and owned by the ledger domain).
- mechanics: isolated temp index seeded from the safe tree,
  `checkout-index` into the worktree, then index alignment — the same
  scrubbed-env, pinned-config invocation discipline as every gittree
  call; no step consults the live index as authority.
- atomicity contract: per-file atomic writes; the primitive is
  IDEMPOTENT toward its tree (re-running converges byte-exact), which
  is the crash story — "refused judgment writes nothing" is amended
  to "a partial materialization is re-entered idempotently and judged
  only at the end" (O19-R1-4's honest correction).
- nested checkouts: materialization is workspace-scope only; the
  toplevel fence re-judges after (part of the verification chain).

## Design-doc and registry edits (same landing as implementation)

- host-implementer-wall-design.md: hiw-critique-r2 §4 amends to the
  ladder as Wido's dated ruling; O19 row gains these anchors; O6
  cross-references the tier-2 fixtures.
- event-registry.json: wall-auto-recovered (missionId, taintId,
  phase, target), wall-recovery-retry (attempt bookkeeping), and the
  repeat-offense ask naming the prior recovery.
- State schema: the recovery record and violationCycle join the
  validator with the same append-only/immutability discipline as the
  turn log (schema bump per the landed convention).

## Fixtures (minimum)

1. Gate-phase auto-restore end to end: offense → park with recovery
   record → same-pass restore → constrained resolution → mission
   continues → no ask ever existed.
2. Post-verify-phase: motion after acceptance → restore toward the
   acceptance's recorded posture → pending acceptance concludes
   through the landed verification lane; consumed authorizations
   preserved (the R1-5 positive case).
3. Ledger-domain restore from the detection-bound blob; the park
   anchor's bytes are proven NOT to be the source.
4. Adoption escalates; runner-identity adoption refuses in the
   validator; the CLI refuses --by runner:*.
5. Repeat offense: offense → recovery → next-cycle offense → ask
   naming the prior recovery; one-clean-cycle sibling recovers again;
   resume between changes nothing.
6. No-safe-source escalates (missing/unauthenticated binding).
7. Could-not-run containment: injected spawn failure lands attempt
   1 recorded, no ask; resume re-enters and completes; attempt-budget
   exhaustion surfaces the steward notice.
8. Crash boundaries: after park write / after materialization / after
   judgment / after resolution write — each resumes into the table's
   single lawful continuation (the completePendingVerification-lane
   pattern).
9. Forged runner resolution refuses: right shape, no recovery record;
   wrong safe target; wrong reason; closing a pending acceptance.
10. Materialization contract: tracked modification, deletion,
    untracked preservation, modes/symlinks, staged-only motion,
    mid-materialization fault → idempotent re-entry.


## Open-question ledger (round 2, verbatim substance) and the slice proposal

Round 2's nine findings, each a design obligation for the slice that
owns it:

- R2-1 (CRITICAL, ledger): the park currently books cycle N+1 onto the
  DISPUTED ledger bytes and can anchor them; restoring the bound
  predecessor then conflicts with state. One end-to-end ledger
  transaction must be designed: never book onto disputed bytes —
  restore-then-book, or an explicitly unbooked cycle — with state,
  count, pending stamp, and anchor bound to the exact result. → SLICE 2.
- R2-2 (CRITICAL, resume): the pending-recovery resume entry must be
  authenticated and run BEFORE both the runner's raw unresolved-taint
  stop and ordinary reconciliation, admitting only the bound
  divergence. → SLICE 2 for the ledger lane; SLICE 1 carries the
  workspace-only resume entry (which composes with ordinary
  reconciliation unchanged, the easier half).
- R2-3 (CRITICAL, phases): workspace-gate conflates the in-turn gate
  (preTree safe) with reservation continuity (fresh preTree IS the
  disputed tree; expectedNow is safe). A complete
  detection-site-to-phase-to-target table is required. → SLICE 1
  (in-turn gate only); SLICE 3 adds reservation continuity and
  post-verification.
- R2-4 (CRITICAL, binding durability): the detection-time source
  binding must survive the detection-to-park crash window —
  predecessor-state-hash + anchor locator + digest authenticated at
  park replay; wall.json stays evidence, never authority. → SLICE 2
  (ledger); SLICE 1's workspace binding is chain-derived at park time
  and does not cross that window (the pre-tree is already in the
  chain).
- R2-5 (HIGH, schema): the ledger resolution needs its own tagged
  schema — digest+locator, unchanged workspace E-tree, posture,
  sequence behavior — the validator must refuse digest-as-tree. →
  SLICE 2.
- R2-6 (HIGH, dual targets): post-verification recovery must restore
  worktree AND staged targets separately (stagedTreePost != postTree
  is lawful), workspace-scoped, sibling-preserving. → SLICE 3.
- R2-7 (HIGH, certificates): recovery history needs explicit
  pending/terminal certificate shapes with a separately clearable
  active pointer; repeat eligibility binds to a terminal
  SAFE-RESTORED certificate. → SLICE 4 (repeat rule), certificate
  shape founded in SLICE 1.
- R2-8 (HIGH, attempt accounting): attempts must be durably reserved
  BEFORE effects; state never advances past an unanchored
  predecessor; anchor refusals keep the could-not-run vs
  repository-answer split. → SLICE 1 (the containment machinery is
  its core).
- R2-9 (HIGH, ask tail): PROVED-INELIGIBLE needs the landed
  drain-stall pattern — deterministic ask derived from state, terminal
  write+anchor carrying the ask identity, idempotent publication,
  missing-ask repair before terminal refusal. → SLICE 1.

### The slice proposal (for coordinator ratification)

1. **o19-slice-1 — the in-turn workspace-gate ladder** (proposed
   Appetite: 6h design+implement behind fixtures): pending-recovery
   record, constrained transition, MaterializeTree, could-not-run
   containment with durable attempt slots (R2-8), the ask tail
   (R2-9), workspace resume lane (R2-2 easy half), recovery
   certificate shape (R2-7's foundation). In-turn gate detections
   only; every other detection site keeps today's human-reserved
   behavior. Independently landable; delivers D117 for the most
   common violation shape.
2. **o19-slice-2 — the ledger transaction** (proposed Appetite: 1d,
   design first): R2-1's booking/restore/anchor transaction, R2-2's
   authenticated resume entry, R2-4's durable binding, R2-5's schema.
3. **o19-slice-3 — post-verification and reservation-continuity
   phases** (proposed Appetite: 6h): the phase table (R2-3), dual
   worktree+staged targets (R2-6), nested-checkout fixtures.
4. **o19-slice-4 — the repeat rule** (proposed Appetite: 3h):
   violationCycle validation, certificate-bound adjacency (R2-7),
   the escalation ask naming the prior recovery.
