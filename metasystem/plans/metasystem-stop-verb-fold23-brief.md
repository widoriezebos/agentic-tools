Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Round 25: two places the build contradicts the design (chain stopverb-build1)

The sixth read found two material items and one note. Both material
items are the implementation contradicting design text that already
exists, so there is nothing to decide: the design is determinate and
this round makes the code match it. The whole slice-1 acceptance is
green on the round-24 tree in the orchestrator's environment, sixteen
supervision scenarios and the packages the change touches, so a
regression is yours to notice.

# The findings, verbatim from the read

== SVC9-01
A monitored run that hosted a stopped test suite is signalled immediately and then recorded as "ended-unknown", destroying the run's own verdict. The design's section 4 step 4 says the opposite: a run whose workload was a suite stopped in step 3 is given the scaled five seconds to conclude from its exit sidecar and is then reported from its own record, printing "concluded red (suite stopped)". That sentence is the whole point of stopping suites before the runs that host them, which is the accepted fold for round-2 finding STOP-R3/STOP-R2-003. The built code contains neither half. The run family calls the run store's stop with no waiting period at all, and the store's stop path always writes the verdict "ended-unknown" over whatever the sidecar established. So the order is right but the evidence the order exists to preserve is thrown away: a suite that failed is recorded as a run of unknown outcome, and the person reading the report is told "already gone" or "concluded ended-unknown" for a run that had a red result on disk. This is the brief's mandate 3 question answered in the negative.
EV: In internal/stoptransition/families.go the run family's Stop (lines 419-451) calls f.store.Stop(record.RunId) as its first act, with no sleep, deadline or record re-read anywhere in the function. In internal/run/stop.go (a new file in this diff) stopHeld, at the wrapped-custody branch, checks whether the process group is already empty and otherwise sends SIGTERM straight away (lines 248-270); either way it finishes through concludeStoppedHeld (lines 305-313), which calls terminalizeWithVerdict(record, StatusEndedUnknown, nil, &note, &assessed) unconditionally. In internal/run/conclude.go, terminalizeWithVerdict (line 217) writes exactly the verdict it is handed, and the function that reads the sidecar and derives a real verdict, provisional, is called only

== SVC9-02
When the supervision shutdown succeeds in stopping every process but fails at one of its bookkeeping steps, that failure is printed nowhere, recorded nowhere, and does not set exit 1 -- and one printed line then claims something the code did not do. The supervision shutdown returns two things: an outcome for each component, and one error carrying whatever else went wrong. The stop transaction's supervision family stores that error but only ever looks at it when a component has no outcome. Since a force-killed owner does have an outcome, an error raised after the kill is silently dropped. Two concrete failures land exactly there: the escalated registry row that could not be appended, and the dead owner's supervision lock that could not be released. In the first case the report still prints "killed (TERM ignored; reaped reason=shutdown-escalated)", naming a registry row that was never written -- the report claims more than the code did. In the second the owner's lock directory survives the stop while the closing line reads "stopped <checkout>" with exit 0. This breaks the design's rule that the durable record says what the page says (15.1), the rule that after the fence closes every failure becomes a printed line rather than a discarded error (14.5, 16.6, 17.5), and section 6's specified shape for a lock that could not be released ("NOT STOPPED ... did: left the lock").
EV: In internal/stoptransition/families.go the supervision family's Stop (lines 545-565) does f.stopErr = shutdownErr after calling supervise.ShutdownAt, and the only read of f.stopErr is inside the branch guarded by the outcome lookup failing: `outcome, ok := f.outcomes[...]; if !ok { if f.stopErr != nil { return Outcome{}, f.stopErr } ... }`. When the lookup succeeds the error is never examined again, and no later code path in the file or in internal/stoptransition/transition.go reads it. In internal/supervise/arming.go the shutdown function joins both failures into that same returned error: appendShutdownEscalated at its only call site (lines 1246-1250, guarded by the owner having been force-killed, which is correct) and releaseDeadOwnerLock (lines 1254-1256


# Decisions (the orchestrator's; decided, not open)

D104. SVC9-01: make the code do what section 4 step 4 says. A monitored
run whose workload was a suite stopped in step 3 is given the scaled
five seconds to conclude from its own exit sidecar and is then reported
from its record, printing the concluded line that step names. It is not
signalled first and it is not forced to ended-unknown while its own
verdict is still arriving. The design text is the authority; if you read
it differently, STOP and report a gap rather than choosing.

D105. SVC9-02: a bookkeeping failure inside the supervision shutdown is
a failure like any other after the fence closes. Section 14.1 and
section 17.5 say every such failure becomes a printed line; section 18.2
says a line may not claim more than the code did, and that the printed
count, the durable survivor entries and the exit status agree. Apply
them to the shutdown's own error rather than only to its per-component
outcomes.

D106. The read's third item, a creation claim that can be orphaned if
the dispatch process dies between a trap replacement and the explicit
claim close, is NOT material and is not yours this round. Record it in
your return as a known gap so it is not lost.

D107. Your return names every unbuilt slice-1 scenario as a gap, as
your last five did.

# Verification

The smallest run that can fail on this: `scripts/agents/go-gate.sh
--fast`; `go test -count=1 -timeout 40m ./internal/stoptransition/
./internal/run/ ./internal/supervise/ ./internal/proofrun/`; and name the
supervision scenarios you expect green. The orchestrator runs the
supervision and suite-progress beds, since one finding touches the run
and suite families.

# Constraints

Wall-clock budget: 90 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
