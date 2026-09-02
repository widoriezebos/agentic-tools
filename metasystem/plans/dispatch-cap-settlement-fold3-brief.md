Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-02

# Goal

Revise your design metasystem/plans/dispatch-cap-settlement-design.md
to revision 3 by folding the three ACCEPTED findings of critique round
2 (critic chain cap-settle-crit, round 2; dispositions with the
orchestrator's evidence in
metasystem/plans/dispatch-cap-settlement-dispositions-r2.md). Round 3
is the declared failsafe: rewrite the affected sections in one pass,
keep every line-and-file grounding, re-run your reject condition, and
keep the design small — each item below is a constraint fixed by the
orchestrator.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one file, the existing
metasystem/plans/dispatch-cap-settlement-design.md (edit in place;
mark the header "revision 3" and extend the changelog with the three
finding ids).

# What revision 3 must settle — constraints fixed by the orchestrator

1. THE GOVERNED EXHAUSTION CHECK RE-PROJECTS (DCS-R2-STALE-RESERVED-SNAPSHOT).
   metasystem/internal/dispatch/governed.go:149 freezes `ReservedBefore`
   at admission and metasystem/internal/run/conclude.go:316-318 decides
   exhaustion as `ReservedBefore + observedMinutes >= limit`. Under the
   settled meaning an open cap in that snapshot may have settled by
   conclusion. Specify: at conclusion the check uses a FRESH projection
   taken at that instant — its observed and open-cap parts EXCLUDING
   this attempt — plus this attempt's observed minutes; say exactly how
   the running attempt is excluded (the record being concluded is still
   non-terminal in the store at that moment; cite the read path) and
   what happens when the fresh projection is BudgetUnknown (fail closed:
   which outcome). `ReservedBefore` stays on the record as the
   admission-time fact for the audit trail and no longer decides. Name
   the test: admitted at equality with another job's open cap, the other
   job settles, this attempt fails, no exhaustion; and the converse where
   exhaustion is real.
2. THE LEASE SWEEP STAMPS AFTER DEATH (DCS-R2-END-BEFORE-DEATH).
   metasystem/internal/lease/sweep.go:198-204 returns as soon as SIGTERM
   delivery succeeds and concludeStaleJob (:146-170) stamps `endedAt` at
   once. Specify the same ladder dispatch.sh climbs at :339-368 —
   bounded wait for the group, SIGKILL on expiry, a final
   `kernelGroupAbsent` check (metasystem/internal/supervise/arming.go:356)
   — before the terminal stamp; state the bound in seconds and what the
   sweep does when the group will not die (the record must not become
   terminal while the group lives; say what it becomes and who retries).
   Name the test that proves no stamp while the group lives. Confirm the
   other three terminal writers act on death or self-report (the
   reaper's timeout after the kill, RecordProtocolError from the
   adapter's own exit, the creator-abandoned reconciliation) and cite.
3. ONE CLOCK DOMAIN FOR START AND END (DCS-R2-MIXED-START-END-CLOCKS).
   The start instant becomes `ownershipProof.provenAt` (the launcher's
   wall clock at the ownership write, dispatch.sh:829,
   ownership.go:75), same clock as `endedAt`; fallback `startedAt`;
   `pidStartedAt` is no longer read by the settlement (it stays the
   census's identity fact). Update the rule, T12 and the specimen T9
   (recompute the eight records from provenAt; state the minutes).
   Record the residual in section 8: a host clock step during a job
   shifts the charge by the step size, bounded above by the clamp and
   below by the minute floor (the KI-37 family); no monotonic
   cross-process measure is built.

Ground every new claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md. Self-grade again.

# Constraints

Wall-clock budget: 25 minutes. Edit only the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
