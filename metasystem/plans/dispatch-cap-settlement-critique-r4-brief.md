Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-02

# Goal

Round 4 of your critique of metasystem/plans/dispatch-cap-settlement-design.md,
now revision 4 (landed, in your workspace). The first three-round
budget of this chain is exhausted with material findings remaining;
per the design-critique skill this focused follow-up enumerates every
open finding id and opens one fresh three-round budget on the same
chain. Open ids from round 3, all ACCEPTED and folded into revision 4
(dispositions in metasystem/plans/dispatch-cap-settlement-dispositions-r3.md):
DCS-R3-RETRY-SELF-PROJECTION (the conclusion-time projection now
excludes the concluding run from both the run records and the durable
attempts), DCS-R3-PROJECTION-WIRING (one constructor for every store
that may conclude a governed run, carrying the spend seam; a seamless
store refuses to conclude), DCS-R3-LATE-START-INSTANT (the start
instant is the record's own `startedAt`).

# Inputs: decisions already taken, so you do not re-raise them

- The residuals recorded in earlier rounds stand: KI-45 (a dispatcher
  dying between spawn and ownership write charges 0), the clock-step
  residual, the rendering (one reserved segment per refusal line).
- The lane is the pure design lane; if you find a seam the design
  cannot see from outside, name it as such so the orchestrator can
  route it.

- Three fold notes are ANSWERED (metasystem/plans/dispatch-cap-settlement-dispositions-r3.md,
  fold section): the fuller read-only store inventory stands; the
  import graph is as revision 4 measures it (the group-absence check
  stays in supervise, the constructor lives in dispatch) — verify that
  measurement, not revision 3's text; the never-drained retry stamp is
  a recorded residual outside this design.

# Review brief

Round budget: three focused rounds on this fresh budget (rounds 4-6);
failsafe round 6. If material findings exhaust this second budget the
design stops and waits on the human. Threat model, scope and
materiality criterion unchanged from round 1
(metasystem/plans/dispatch-cap-settlement-critique-brief.md).

Verify first that each round-3 finding is actually folded, by reading
the revised sections, and say so per finding id. Then attack revision
4 where it is new: the two-store exclusion by run id and its
interaction with the durable-owner consistency check; the constructor
(its package, the import graph proof, the typed refusal when the seam
is absent, the read-only stores left bare); the `startedAt` start
instant and its stated upper bound; the recomputed specimen; the tests
T13 and T14 as discriminators.

Return format: the design-critic schema; stable identifiers
DCS-R4-<name>; for each material finding say whether it is
mechanical-grain or invariant-grade; a clean verdict is
`verdictMaterialCount: 0` with any non-material observations recorded.

# Constraints

Wall-clock budget: 20 minutes. Do not rewrite the design.

# Gap Rule

stop and report a gap; never fill it silently.
