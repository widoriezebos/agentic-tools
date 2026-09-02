Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Round-5 critique of metasystem/plans/breach-clock-and-budget-honesty-design.md
revision 6 (landed, in your worktree). Revision 6 folds the three Fix 3 gaps
your own build reported (job breach-build-1b) as decided in
metasystem/records/misc/breach-build-1b-gaps.md (landed): Gap 1, resume from
the claimed-and-fenced shape keeps the claim with a fresh episode, from
parked-with-breach it binds no claim; Gap 2, the only-claim invariant moves
to the quota's unit — a machine with a fenced claim holds no claim OUTSIDE
that goal's arc (metasystem/internal/goal/validate.go lines 250-283 count
one arc under one claimant once), so `CloseStop` on an arc member never
collides with its siblings; Gap 3, metasystem/internal/steward/delivery.go
is added to the consumer table as unchanged.

Judge Gap 2 BY ID against the tree first, because it is a rule change: with
the restated invariant, can the breaker's own `CloseStop` publication
(metasystem/internal/goal/stop.go lines 276-341) ever be refused by
`ValidateCommit` on any claimed shape the quota admits (single goal, arc
member, whole arc under one pair, reopen into an arc)? Does the restated
rule still close BCD-R1-004 (no second claim the quota would not admit;
orientation deterministic when every claim of the machine is checked for a
fence)? Does the arc claim cascade (metasystem/internal/goal/verbs.go around
line 1501) or steal produce a tree the rule refuses when it should admit, or
admits when it should refuse? Then Gap 1: the two sentences say the same
thing and match `resumeRequest` (stop.go lines 378-411). Then Gap 3: is the
delivery role's unchanged verdict on a fenced claim a lie by the design's
own standard, and is `goal.ParseWorkingDuration` in the day-token
inventory's legacy-reader class. Confirm no regression in what rounds 1 to
4 left standing (Fix 1 whole, including the inherit rule; Fix 2 whole).

Findings material and grounded, quoting the disagreeing text or code, ids
BCD-R5-NNN. Your sandbox is read-only: verify by reading, do not run go.
Zero material findings is an acceptable, closing answer if the reading
supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
