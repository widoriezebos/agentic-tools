# metasystem stop: fifth code critique (chain stopverb-build1, round 24)

Critic stopverb-crit9 (code-critic, Opus 5) on reviewed tree bc3b3be2966d7b50c26c0f3b6287f9023d0760a5. Two material findings and one note. Both material findings are the same shape: the build contradicts design text that already existed, so neither needed a design round, only a fold.

## SVC9-01 - high - material=True

CLAIM: A monitored run that hosted a stopped test suite is signalled immediately and then recorded as ended-unknown, destroying the run's own verdict. Section 4 step 4 says the opposite: a run whose workload was a suite stopped in step 3 is given the scaled five seconds to conclude from its exit sidecar and is then reported from its own record, printing "concluded red (suite stopped)". That sentence is the whole point of stopping suites before the runs that host them, which is the accepted fold for the second design read's STOP-R2-003. The built code contained neither half.

EVIDENCE: internal/stoptransition/families.go, the run family's Stop at lines 419-451, calls the run store's stop as its first act with no wait, deadline or record re-read. internal/run/stop.go stopHeld sends the termination signal straight away (lines 248-270) and finishes through concludeStoppedHeld (lines 305-313), which writes the ended-unknown verdict unconditionally through terminalizeWithVerdict (internal/run/conclude.go line 217). The function that reads the sidecar and derives a real verdict is reached from assessRunning alone, which no stop path calls.

## SVC9-02 - high - material=True

CLAIM: When the supervision shutdown stops every process but fails at one of its bookkeeping steps, the failure is printed nowhere, recorded nowhere and does not set exit 1, and one printed line then claims something the code did not do. Two concrete failures land there: the escalated registry row that could not be appended, and the dead owner's supervision lock that could not be released. In the first case the report still names a registry row that was never written; in the second the lock directory survives a stop whose closing line says the checkout is stopped with exit 0.

EVIDENCE: internal/stoptransition/families.go, the supervision family's Stop at lines 545-565, stores the shutdown error and reads it only inside the branch where the component outcome lookup fails. A force-killed owner does have an outcome, so an error raised after the kill is dropped. internal/supervise/arming.go joins both failures into that same returned error: appendShutdownEscalated at lines 1246-1250 and releaseDeadOwnerLock at lines 1254-1256.

## SVC9-03 - low - material=False

CLAIM: In scripts/agents/dispatch.sh a creation claim can be left on disk if the dispatch process dies between the point where the script replaces its exit trap with a narrower one and the explicit claim close. Self-healing rather than dangerous: the abandoned claim's creator is dead by then, and the stop transaction removes a claim whose creator identity is dead before it does anything else.

EVIDENCE: finalize_and_launch replaces the trap before calling the fence-after-launch verb and the explicit claim close, while the earlier setup traps do include the claim cleanup. internal/stopfence/fence.go RemoveStale (lines 398-407) removes a claim once its creator probes dead, and the stop transaction calls that path in waitForCreators before the inventory runs.

## Coordinator's reading (m1b, 2026-09-08)

Both material findings accepted and folded in round 25, under brief
decisions D104 and D105: make the code match section 4 step 4 rather
than choosing between the two halves, and apply the absolute rule of
14.1, 17.5 and 18.2 to the supervision shutdown's own error. SVC9-03 was
recorded as a gap and not built (D106).

The lesson of this read is narrower than the one before it. Neither
finding needed a judgment call: the design already said what to do, and
the build had drifted from it in two places that no scenario covered.
Design text without an acceptance scenario is a promise nobody checks,
which is the same lesson the fourth code read left, now with two more
instances.
