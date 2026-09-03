Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Goal

Revise your design metasystem/plans/token-spend-fence-design.md to
revision 2 by folding the four ACCEPTED findings of the one Sol review
(dispositions with the orchestrator's evidence in
metasystem/plans/token-spend-fence-dispositions.md). This is the ONE
fold the tier-3 ladder allows (R-54-m1); a closing review follows and
any point still disputed after it becomes a named test obligation
(R-60-m1). Rewrite the affected sections in one pass, keep every
line-and-file grounding, stay under 300 lines, and do not add
mechanism beyond the four constraints below.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one file, the existing
metasystem/plans/token-spend-fence-design.md (edit in place; mark the
header "revision 2" and add a two-line changelog naming the four
finding ids).

# What revision 2 must settle — constraints fixed by the orchestrator

1. ONE EPISODE PER CROSSING (TSF-R1-alert-crossing-identity). Today
   episodes are keyed by the whole health digest and cleared only when
   the entire aggregate is healthy
   (metasystem/internal/steward/alert_episode.go). Specify an episode
   identity per (scope-id, ceiling) crossing, each submitted once, a
   new episode at each further multiple, and a clearance that re-arms
   that crossing's episode when the crossing alone clears, whatever the
   other roles' status. Choose the smallest mechanism: a per-crossing
   register file beside the episode store that the spend role owns, or
   per-key episodes inside the store; say which and why, and name the
   test TestSpendFenceCrossingsHaveIndependentEpisodesAndRearmWhileOtherRoleDead.
2. EXCLUDE DELEGATES BY SESSION ID (TSF-R1-shared-checkout-double-count).
   Job records carry the runtime session id (`sessionId`, and
   `resumedSessionId` on follow-ups; see metasystem/internal/dispatch/
   build.go and a live record). The seat reader excludes every
   transcript whose sessionId appears on any job record, whatever the
   launch mode; the worktree-cwd rule stays as a second guard. Test
   TestSeatTranscriptExcludesSharedCheckoutDelegateSession.
3. THE SEAT METER NEVER LOSES SPEND SILENTLY (TSF-R1-seat-omission-honesty).
   The 48-hour age filter applies to the DAY scope only; the goal scope
   reads every present transcript file. An assistant request whose
   usage shape is absent or unrecognised is counted as
   `seat unmeasured requests=<n>` (its own ledger line and health-line
   segment), never dropped; files skipped by age are counted and
   printed. Tests TestSeatTranscriptShapeFailureIsUnmeasured and
   TestSeatGoalDoesNotSilentlyLoseAgedTranscriptSpend.
4. AN UNREADABLE JOB RECORD CANNOT DISAPPEAR (TSF-R1-job-record-read-honesty).
   metasystem/internal/mission/fence.go:657-660 continues past a record
   that fails to parse. In the lifted rule and in Measure, such a record
   becomes an explicit unmeasured ledger entry naming the file and the
   error, counted in the health line; health goes unknown only when the
   jobs directory itself cannot be listed. Test
   TestUnreadableJobRecordCannotDisappear. State whether AggregateUsage's
   own behaviour changes (it should not; say how the lifted function
   serves both).

Update the health-line example bytes and the proof plan accordingly.
Ground every new claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md. Self-grade again.

# Constraints

Wall-clock budget: 25 minutes. Edit only the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
