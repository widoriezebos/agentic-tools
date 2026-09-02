Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-02

# Goal

Round 3, the declared failsafe round, of your critique of
metasystem/plans/dispatch-cap-settlement-design.md, now revision 3
(landed, in your workspace). Round 2 closed on three accepted findings
— DCS-R2-STALE-RESERVED-SNAPSHOT, DCS-R2-END-BEFORE-DEATH,
DCS-R2-MIXED-START-END-CLOCKS — with the orchestrator's evidence in
metasystem/plans/dispatch-cap-settlement-dispositions-r2.md. Revision 3
folds them: the governed exhaustion check re-projects at conclusion,
the lease sweep stamps the end only after death evidence, and start
and end share one clock domain (the launcher's provenAt).

# Inputs: decisions already taken, so you do not re-raise them

- The clock-step residual (a host clock step during a job shifts the
  charge by the step, bounded by the clamp and the floor) is RECORDED,
  not built; no monotonic cross-process measure.
- KI-45 (a dispatcher dying between spawn and ownership write charges
  a seconds-long process 0) stays a recorded residual.
- Rendering: one reserved segment appended to every refusal line; the
  per-limit breach texts unchanged.

- Three interpretation choices the fold reported are CONFIRMED by the
  orchestrator: an unknown fresh projection at conclusion exhausts a
  failing attempt with a reason naming the unknown record; the
  group-absence check moves to `identity.GroupAbsent` (lease cannot
  import supervise); a present but unparseable `provenAt` fails closed
  while an absent one falls back to `startedAt`. Attack them on their
  merits only.

# Review brief

Round budget: this is round 3 of three, the failsafe. After it, only a
demonstrated requirement failure or a shape-level defect reopens
prose; mechanical-grain findings become fixture obligations and
implementation begins with a mandatory code critique. Threat model,
scope and materiality criterion unchanged from round 1
(metasystem/plans/dispatch-cap-settlement-critique-brief.md).

Verify first that each round-2 finding is actually folded, by reading
the revised sections, and say so per finding id. Then attack revision
3 where it is new: the re-projection at conclusion (is the concluding
attempt excluded exactly once; what happens on BudgetUnknown; does any
other reader of `ReservedBefore` still decide anything); the sweep's
death ladder (the bound, the SIGKILL step, the group-absence check,
what the record becomes when the group will not die and who retries);
the provenAt start instant (its writer, its clock, the fallback); and
whether the specimen and the named tests still discriminate.

Return format: the design-critic schema; stable identifiers
DCS-R3-<name>; for each material finding say whether it is
mechanical-grain (a value, a format, a bounded choice) or
invariant-grade (a contract or a shape); a clean verdict is
`verdictMaterialCount: 0` with any non-material observations recorded.

# Constraints

Wall-clock budget: 20 minutes. Do not rewrite the design.

# Gap Rule

stop and report a gap; never fill it silently.
