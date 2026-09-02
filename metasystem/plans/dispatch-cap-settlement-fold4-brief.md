Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-02

# Goal

Revise your design metasystem/plans/dispatch-cap-settlement-design.md
to revision 4 by folding the three ACCEPTED findings of the failsafe
round (critic chain cap-settle-crit, round 3; dispositions with the
orchestrator's evidence in
metasystem/plans/dispatch-cap-settlement-dispositions-r3.md). Two are
shape-level, one is a requirement failure; all three are in the
seams the fold before this one introduced. Rewrite the affected
sections in one pass, keep every line-and-file grounding, re-run the
reject condition, and shrink where the constraints allow it.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one file, the existing
metasystem/plans/dispatch-cap-settlement-design.md (edit in place;
mark the header "revision 4" and extend the changelog with the three
finding ids).

# What revision 4 must settle — constraints fixed by the orchestrator

1. RETRY-SAFE EXCLUSION (DCS-R3-RETRY-SELF-PROJECTION).
   obligationstate.RecordTerminal commits the durable terminal spend
   BEFORE the run record's terminal write
   (metasystem/internal/obligationstate/state.go:241-268;
   metasystem/internal/run/run.go:396-430). The fresh projection at
   conclusion therefore excludes the concluding run by run id from
   BOTH stores: the run-record loop and the durable terminal attempts
   (the loop over `durable` in metasystem/internal/dispatch/budget.go
   that adds `ObservedCostMinutes`), so a retry after a partial commit
   projects the same spend as the first call and RecordTerminal's
   idempotence holds. Specify how the excluded durable attempt
   interacts with the durable-owner consistency check (an unpruned
   attempt whose run record is still open must not be reported as a
   contradiction for THIS run id). T13 gains the partial-commit retry
   case asserting identical terminal fields on the second call.
2. ONE CONSTRUCTOR FOR STORES THAT CONCLUDE (DCS-R3-PROJECTION-WIRING).
   Production run stores are built at metasystem/cmd/metasystem/run.go:54,
   metasystem/cmd/metasystem/supervise_component.go:245 (Assess) and
   metasystem/internal/lease/sweep.go:67 (SweepStale); only the first
   was to carry the spend seam. Specify one constructor for a store
   that may terminalize a governed run, carrying the seam, used by all
   three; a store without the seam REFUSES to conclude a governed
   attempt with a typed error naming the missing seam — never silent
   exhaustion; read-only stores (gaterun/weight.go:363,
   dispatch/watch.go:23, counselor/sources.go:369, report/scan.go:196,
   budget.go:404) stay bare and say why they are safe. Check the import
   graph: lease and supervise must be able to reach the constructor
   and the dispatch projection it wires (name the package the
   constructor lives in and prove no cycle with `go list -deps`). Tests
   exercise Assess and SweepStale concluding a failing governed attempt
   with the seam present and absent.
3. THE START INSTANT IS `startedAt` (DCS-R3-LATE-START-INSTANT).
   dispatch.sh spawns the runtime (:812-820) and samples `proven_at`
   afterwards (:829); `startedAt` is stamped at record creation
   (build.go:423, :635), before the spawn, immutable, same wall clock.
   The start instant becomes `startedAt`; `ownershipProof.provenAt`
   and `pidStartedAt` are not read by the settlement; the never-launched
   proof stays "no process identity"; the measured interval bounds the
   runtime from above by the creation-to-spawn gap (seconds), never
   below — state that bound. Remove the fallback branch. T12 becomes
   the startedAt-versus-provenAt case; recompute the specimen T9 and
   state the minutes.

Ground every new claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md. Self-grade again.

# Constraints

Wall-clock budget: 25 minutes. Edit only the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
