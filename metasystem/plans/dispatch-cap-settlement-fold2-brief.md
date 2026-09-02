Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-02

# Goal

Revise your design metasystem/plans/dispatch-cap-settlement-design.md
to revision 2 by folding the five findings of critique round 1 (critic
chain cap-settle-crit, gpt-5.6-sol; dispositions with the
orchestrator's evidence and one partial refutation in
metasystem/plans/dispatch-cap-settlement-dispositions.md). Rewrite the
affected sections in one pass; keep every line-and-file grounding;
re-run your reject condition. Keep the design SMALL: every addition
below is a constraint fixed by the orchestrator, not an invitation to
grow mechanism.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one file, the existing
metasystem/plans/dispatch-cap-settlement-design.md (edit in place;
mark the header "revision 2" and add a two-line changelog naming the
five finding ids).

# What revision 2 must settle — constraints fixed by the orchestrator

1. `endedAt` BECOMES TRANSITION-OWNED (DCS-R1-TIMESTAMP-AUTHORITY).
   Today `endedAt` is not in immutableFields
   (metasystem/internal/dispatch/record.go:60-75) and RecordCAS stamps
   it only when empty at terminalization (:522-545). Specify: RecordCAS
   refuses a patch that carries `endedAt`, with the immutable-identity
   refusal shape (say the exact message), and the terminal transition
   alone stamps it; enumerate the writers you checked that never patch
   it (build.go initialises null; claim.go's recordedOutcome copies it
   into a view). Name the test.
2. THE START INSTANT IS THE PROCESS START (DCS-R1-START-PROOF, accepted
   in part). The ownership patch stamps `pid`, `pidStartedAt` (kernel
   start, epoch seconds) and `ownershipProof.provenAt` on the record
   (read a live record shape: metasystem/artifacts/agents/jobs/ is
   runtime state, cite the writer in internal/dispatch/ownership.go
   and the census's use of `pidStartedAt`). Observed minutes run from
   `pidStartedAt` to `endedAt`; when a record carries `pid` but no
   parseable `pidStartedAt`, fall back to `startedAt`; when neither
   parses, unknownBudget. The never-launched proof stays "no process
   identity" (pid absent, null or below 1). The refuted half is
   RECORDED, not built: a dispatcher that dies between spawning the
   runtime and writing ownership leaves a record with no identity that
   charges 0 for a process that ran seconds — bounded, never a refusal
   of lawful work; name it in section 8 as a residual with that bound
   (the orchestrator tokens it in memory/known-issues.md).
3. EVERY REFUSAL NAMES THE TWO NUMBERS (DCS-R1-REFUSAL-COVERAGE). The
   admission renderer (metasystem/internal/dispatch/admission.go, the
   function that formats the BUDGET_REFUSED line) prints one reserved
   line on EVERY refusal whatever limit tripped —
   `reserved observed=<n> open-caps=<m> limit=<L>` — and the governed
   refusal in metasystem/internal/dispatch/governed.go carries the same
   line; the per-limit breach objects keep their fields. Specify the
   exact rendered text for an attempt-only refusal and for a
   reserved-minutes refusal (both parts plus the proposal). Extend T10.
4. THE HUSK AFTER A DISCHARGE (DCS-R1-DISCHARGED-HUSK). The
   post-discharge filter at metasystem/internal/dispatch/budget.go:353-361
   uses `startedAt` when present, else `createdAt` (the pending-setup
   husk's only stamp, build.go:148-164); a record with neither is
   unknownBudget; the husk then reaches the charge rule and charges 0.
   T6 gains a discharge-branch subtest.
5. GOVERNED COMPONENTS PROVED (DCS-R1-GOVERNED-COMPONENT-PROOF). T8
   gains a governed subtest: one terminal governed attempt with
   ObservedCostMinutes and one live governed run with
   ExecutionCostMinutes, asserting `ObservedJobMinutes`,
   `OpenCapMinutes` and their sum; the governed coverage test asserts
   the components too.

Ground every new claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md. Self-grade again.

# Constraints

Wall-clock budget: 25 minutes. Edit only the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
