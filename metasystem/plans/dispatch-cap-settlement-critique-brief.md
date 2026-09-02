Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-02

# Goal

Independent critique of metasystem/plans/dispatch-cap-settlement-design.md
(landed, in your workspace), the design that repairs the reservation
accounting bug Wido named in ruling R-49-m1b (metasystem/memory/rulings.md):
a dispatched job charges its cap against the goal's reserved job-minutes
for ever, whatever it ran. The design makes a terminal job charge its
observed minutes and an open job its cap, and changes the refusal
wording to show both parts.

# Inputs

The design was authored by job cap-settle-design (Claude Fable 5.1,
design mode, round 1) against base tree 4142106d. Its one reported gap
and the orchestrator's answer, so you do not re-raise it:

- Specimen count: the goal record said nine rounds consumed 1080
  minutes; the checkout holds eight job records bound to that goal (the
  ninth "round" was a dispatch refused by the brief-authority preflight,
  which writes no record). ANSWERED: the DONE test pins the eight
  records on disk (960 minutes under the old rule, 70 under the new);
  the goal text is corrected.

# Review brief

Round budget: three focused rounds, agreed before round one; failsafe
round 3. This design is deliberately small; a finding that grows the
mechanism beyond the charge rule, the settlement shape, the message and
the tests must name the requirement the small design fails.

Threat model: accidents and drift, not adversaries — a projection that
under-counts (a job charging less than it ran), one that over-counts
(the bug being repaired, or a cap charged after termination), a record
whose timestamps lie or are missing, a killed job, a job that never
started, and consumers of the projection that silently keep the old
meaning. Forged job records are OUT of scope (the record-integrity
gates own them).

Scope: the job-record loop of metasystem/internal/dispatch/budget.go,
the breach wording in metasystem/internal/dispatch/admission.go and
metasystem/internal/dispatch/governed.go, the split guard in
metasystem/cmd/metasystem/goalsync_mutations.go, and the tests in
metasystem/internal/dispatch/budget_test.go. OUT: the governed-run
settlement path (already settled to observed cost), the cap keys and
the slice norm (R-17), the four structured limits (R-13).

Materiality criterion, verbatim: would an implementer working from this
design build something DIFFERENT, or WRONG, because of this finding?
The verdict line counts only material findings.

Attack in particular: (1) the rounding and the floor — can a job charge
0 when it ran, or charge more than its cap; (2) the "never started"
proof — which fields, and can a record satisfy it while a process ran;
(3) a cancelled or timed-out job whose endedAt is written late or not
at all; (4) the settlement shape — computed on every projection from
timestamps versus written once — and what a later edit of a record's
timestamps does to the budget; (5) every consumer of
`ReservedJobMinutes` under the new meaning; (6) the existing test
assertions the design says change, and whether any it says do not
change actually do.

Return format: the design-critic schema; numbered findings with a
stable identifier each (DCS-R1-<name>), most severe first, each with
file, rule, and the concrete failure it causes; or a clean verdict
(`verdictMaterialCount: 0`) with observations that do not gate.

# Constraints

Wall-clock budget: 20 minutes. Read the design and the cited lines; do
not rewrite the design.

# Gap Rule

stop and report a gap; never fill it silently.
