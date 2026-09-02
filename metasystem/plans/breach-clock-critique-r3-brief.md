Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Round-3 critique of metasystem/plans/breach-clock-and-budget-honesty-design.md
revision 3 (landed, in your worktree). Revision 3 folds your three round-2
findings (metasystem/records/misc/breach-design-critique-r2.md, landed, with
the orchestrator's decisions at its foot): BCD-R2-001 by HOLDING today's
governance (the episode axis replaces only the claim-revision filter; with a
live obligation a proof must still carry the live obligation revision as
metasystem/internal/dispatch/budget.go lines 133-149 demand; the
set-obligation inheritance question is recorded as open for Wido and NOT
built; the test becomes `TestSetObligationReturnsTheStartToTheEpisodeOrigin`);
BCD-R2-002 by a complete day-token inventory in the proof plan, one row per
site classified (a) legacy-reader, (b) converted to hours, (c) explicit
day-refusal, with the search patterns and named exclusions; BCD-R2-003 by a
writer-by-writer rollout table over `budgetTuple`, open-claim, resume,
supplied-budget claim, set-budget and journal recovery in
metasystem/cmd/metasystem/goalsync_mutations.go, the residue stated as
display-only per write on any goal, no rollback wall.

Judge those three closures BY ID against the tree: does the held rule still
close BCD-R1-003 (a raise clears the obligation, metasystem/internal/goal/verbs.go
lines 122-124, and the episode-scoped proof then counts, so the start does
not move at the raise) without reintroducing any favorable-direction
movement; is the inventory complete (re-run your own search over scripts,
docs, fixtures and _test.go files and name any site the table misses) and
does each (b) row's intent survive in hours (the norm rows at
metasystem/scripts/agents/goal-cli-fixtures.sh lines 448 and 457 are driven
by reservedJobMinutesLimit 1441 against the 1440 norm in
metasystem/internal/goal/norm.go, not by the elapsed limit); is the rollout
table complete against every caller of `budgetTuple` and `goal.NewBudget`.
Confirm no regression in what rounds 1 and 2 left standing (Fix 3 whole,
Fix 1's record and schema changes, Fix 2's decision and migration).

Findings material and grounded, quoting the disagreeing text or code, ids
BCD-R3-NNN. Your sandbox is read-only: verify by reading, do not run go.
Zero material findings is an acceptable, closing answer if the reading
supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
