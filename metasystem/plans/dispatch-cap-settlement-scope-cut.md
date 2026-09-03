# Scope cut: the reservation settlement design builds on its box

Recorded by the dispatch delegate m1b+main-1788333346-60696-6a3256 on
2026-09-02 after Wido challenged the critique loop's length ("what the
hell happened to the critique stop criterium"). Under ruling R-25b-m1 a
scope cut by the seat never happens silently: it returns through the
design lane or rises to Wido as a decision. This one rises to Wido —
stated here, applied to the build brief, retracted if he objects.

## What the loop did

The design (plans/dispatch-cap-settlement-design.md) went through four
Fable revisions and four Sol rounds (5, 3, 3, 3 material findings).
Round 1's findings were mostly in the fix's own box. Round 2 accepted
two neighbouring, pre-existing defects into the design (the governed
exhaustion check's frozen snapshot; the lease sweep stamping the end
before death), which grew it by a conclusion-time re-projection seam, a
store constructor and a kill ladder. Rounds 3 and 4 found defects only
in that new machinery. That is the design-critique skill's "loop
critiquing itself" stop, which the orchestrator failed to apply at
round 3.

## The box that is built (revision 4 sections)

- 1 (the charge rule) with 1.1, 1.2, 1.4, 1.5, 1.6, 1.7, 1.8; the
  start instant is `ownershipProof.provenAt` per revision 3's rule 1.3
  (round 4's finding DCS-R4-STARTEDAT-UNBOUNDED-OVERCOUNT: `startedAt`
  can over-count without bound; `provenAt` under-counts by at most one
  minute at a boundary, the safe direction; fallback `startedAt` only
  when the proof is absent, unparseable proof fails closed).
- 2 (settlement computed from the record), 3 (the two projection
  fields), 4.1 and 4.2 (the reserved line on every refusal and the
  consumer table, minus the 4.3 row).
- 5: tests T1-T12 (T12 as the provenAt-versus-startedAt case), the
  existing-test changes as listed, T11 (the `endedAt` patch refusal).
- 6, 7, 8 as written, with the residuals below.

## What is cut and tokened (R-4: residue demands a token)

- Section 4.3 (the conclusion-time re-projection: `ProjectSpend`,
  `NewConcludingRunStore`, `ErrNoSpendProjection`, the two-store
  exclusion) and tests T13, T15 → goal governed-exhaustion-reprojection,
  carrying the open findings DCS-R4-EXCLUSION-HIDES-DUPLICATE-OWNER and
  DCS-R4-T13-POST-DEBT-RETRY. Until it lands the governed exhaustion
  check keeps today's frozen-snapshot behaviour; under the settled
  meaning that snapshot can only be smaller than today's, so the cut
  makes nothing worse.
- Section 1.9 (the lease sweep's death ladder) and test T14 → goal
  lease-sweep-death-evidence. Until it lands a sweep-stamped record may
  settle a few minutes short while its group finishes dying; the old
  rule charged that job its full cap.

## Residuals recorded

KI-45 (a dispatcher dying between spawn and ownership write charges a
seconds-long process 0); the clock-step residual (a host clock step
during a job shifts the charge by the step, bounded by the clamp and
the floor); the proof-time under-count (at most one minute per job at a
minute boundary).

## Arbiter

The build's own tests and the mandatory Fable code critique judge the
box; the design's critique loop is closed at round 4 with its rounds
retained verbatim (plans/dispatch-cap-settlement-dispositions-r4.md).
