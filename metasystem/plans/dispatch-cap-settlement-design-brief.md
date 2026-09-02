Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-02

# Goal

Author a short design for goal dispatch-cap-necessity (read
metasystem/plans/goals/dispatch-cap-necessity.md first; its Next step
carries the mechanism the orchestrator fixed). Wido's word, verbatim
from ruling R-49-m1b in metasystem/memory/rulings.md: the reservation
accounting "is just a bug ... so far from intent", highest priority.
The defect: a dispatched job charges its reservation cap (flat 120
minutes by default) against the goal's reserved job-minutes and keeps
charging it after the job ended, whatever it ran; nine rounds of 9 to
13 minutes consumed 1080 reserved minutes on two-bars-for-changes
today. DONE means a goal's reserved job-minutes equal the minutes its
jobs actually ran plus the caps of the jobs still open, and every
budget refusal names those two numbers.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one NEW file, dispatch-cap-settlement-design.md,
in the metasystem plans directory.

# What the design must settle

1. THE CHARGE. metasystem/internal/dispatch/budget.go, the job-record
   loop (the `for _, entry := range entries` loop that ends by adding
   `capMinutes` to `projection.ReservedJobMinutes` and counting an
   active job for a non-terminal status), charges every record's cap.
   Specify: a TERMINAL record (`completed`, `failed`, `cancelled`,
   `timeout`; the vocabulary is `TerminalStatus` in
   metasystem/internal/dispatch/record.go) charges its observed
   minutes — endedAt minus startedAt, rounded UP to the next whole
   minute, never below 1 for a job that started, 0 for a record that
   never started (say exactly which record fields prove "never
   started"); a `pending-setup`, `pending` or `running` record keeps
   charging its cap, the ceiling it may still consume. A job killed at
   its cap therefore settles to its cap or less, never more. The cap's
   fail-stop role (ruling R-17, the slice norm) and the four structured
   limits (R-13) are unchanged; no new configuration key.
2. THE PATTERN ALREADY IN THE FILE. The governed-run path in the same
   function settles terminal attempts to `ObservedCostMinutes` and
   reconciles them against durable obligation state
   (`terminalStateContradiction`). State whether the delegated-job path
   should mirror that shape (a settled figure written once at
   terminalization and read back) or compute observed minutes from the
   record's own timestamps on every projection, and why; name the
   fail-closed rule for a terminal record whose `startedAt` or `endedAt`
   is missing or unparseable (the file's existing `unknownBudget` shape).
3. THE MESSAGE. metasystem/internal/dispatch/admission.go builds the
   `reservedJobMinutesLimit` breach as `<used>+<proposed> proposed`
   against the limit, and its integer breach when used already meets the
   limit; metasystem/internal/dispatch/governed.go does the same for
   governed cost. Specify the new wording so a reader sees the two
   parts: observed minutes of ended jobs, caps of open jobs, the
   proposal, the limit. Keep every consumer of
   `projection.ReservedJobMinutes` (grep it: admission.go, governed.go,
   cmd/metasystem/goalsync_mutations.go's split guard) correct under the
   new meaning, and say per consumer whether it changes.
4. TESTS. metasystem/internal/dispatch/budget_test.go stages job records
   with `writeBudgetJob` (jobId, operationId, goalId, goalRevision,
   capMin, status) and no timestamps. Specify the fixture extension
   (startedAt, endedAt) and the cases by name: a completed 10-minute job
   with cap 120 charges 10; a running job charges its cap; a failed job
   that ended seconds after it started charges 1; a timeout at the cap
   charges the cap; a terminal record with no readable timestamps is
   unknownBudget; the existing tests keep their assertions or state
   exactly which assertion changes and why (the first test asserts
   `ReservedJobMinutes == 75` from a completed 30 plus a running 45 —
   under the new rule the completed record needs timestamps). Also name
   the shell or fixture bed, if any, that asserts the old wording of the
   refusal (grep `proposed` under metasystem/scripts/agents).
5. EVIDENCE OF INTENT. Cite the rulings that make this a bug and not a
   choice: R-13 (the four structured limits bound work), R-17 (the cap
   is the per-job runtime ceiling at reservation), R-49-m1b, and the
   draft's four specimens in metasystem/plans/goals-drafts/dispatch-cap-necessity.md.

Ground every claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md. Self-grade per the house
rule: confidence, weakest claim, reject condition.

# Constraints

Wall-clock budget: 25 minutes. A small design: the charge rule, the
settlement shape, the message, the tests. No essay. Do not edit anything
but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file
named under Workspace.

# Gap Rule

stop and report a gap; never fill it silently.
