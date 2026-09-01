# Watch-verb terminal critique (WVFC) — the five riding lows, verbatim

Critic: claude-fable-5 (watch-verb-final-critique) over the corrected
terminal round (reviewedTree 80184e2f). Zero material findings closed
the chain; these five non-material lows ride the goal and bind the
hardening slice.

## WVFC-N1 [low]

The new completed-rounds class is silent for a goal that has never produced any landing receipt: a completed-but-unconsumed first round on such a goal renders as an ordinary completed job with no UNKNOWN-CONSUMPTION item, so one shape of the original incident remains invisible in this class.

Evidence: readCompletedRounds in the diff executes 'continue' when the goal has no receipt entry, and design revision 4 section 3.2 states this explicitly: 'A missing newest receipt does not satisfy the strict postdates predicate.' Because the narrowing is declared in the design alongside the named absent substrate (no persisted return-consumption marker), it satisfies the accepted WVJC-02 remedy rather than repeating its silent omission; the durable consumption marker remains deferred future substrate.

## WVFC-N2 [low]

On an active repository, every legitimately consumed completed round whose endedAt postdates its goal's newest landing receipt (for example critic rounds consumed via dispositions, which append no receipt) renders UNKNOWN-CONSUMPTION, so the aggregate can sit at UNKNOWN with exit 2 indefinitely on a healthy repository - an alarm-fatigue cost, though truthful and fail-safe.

Evidence: readCompletedRounds flags every goal-bound completed record strictly newer than the goal's newest goal= receipt line; itemNeedsUnknown maps UNKNOWN-CONSUMPTION to the unknown aggregate; no recency or role bound limits the candidate set. The design's own rule ('this class never guesses consumed or unconsumed') makes this behavior specified, not a defect.

## WVFC-N3 [low]

A goal-bound failure that occurs after a fresh healthy health observation renders HEALTHY with exit 0 for up to two steward tick intervals (twenty minutes at the default cadence) until the steward re-observes; the fail-open WVJC-01 hole is closed but a bounded producer-latency window remains.

Evidence: readHealth accepts any record younger than two tick intervals; itemNeedsAttention for the jobs class fires only on explicit-null goal-less failures, deferring bound-goal interpretation to the persisted delivery role. Design revision 4 section 3.2 codifies exactly this ('A fresh owning delivery verdict still determines the interpretation of a goal-bound failed job'), and freshness guarantees the owner is alive to re-judge within the window, so the WVJC-01 unbounded fail-open cannot recur.

## WVFC-N4 [low]

readCompletedRounds re-reads and re-validates every job record already validated by readJobs, so one corrupt job file produces duplicate UNREADABLE items in two sections; output noise and doubled I/O only, with the fail-safe direction preserved.

Evidence: Both readJobs and readCompletedRounds independently enumerate artifacts/agents/jobs/*.json and append unreadableItem entries on the same decode failures, each degrading its own section toward the UNKNOWN aggregate.

## WVFC-N5 [low]

The command-level stale-health test's freshness window depends on the test host's global git configuration lacking metasystem.steward.tick-seconds, because the temporary fixture root is not a git repository and git config --get falls through to global scope; a global value above 1800 seconds would make the test fail spuriously.

Evidence: TestWatchStaleHealthAndGoalFailurePrintsDeadRecordAge writes a one-hour-old record and relies on the 600-second default from TickSeconds (runner.go:52-59). The coupling can only cause a spurious failure, never a spurious pass, so the proof is not weakened; the internal-package stale test injects a fixed clock and is fully hermetic.
