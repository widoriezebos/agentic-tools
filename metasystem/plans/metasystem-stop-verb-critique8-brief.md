Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Review brief: the closing critique of the stop verb (chain stopverb-build1, final work round stopverb-build1-r26)

FINDING IDS: chain-unique, SVC11-01, SVC11-02, ... never F-n.

Why this review exists: this is the eighth independent read of this
chain, now twenty-six build rounds old, and the read that must close it.
Every read so far has found real defects in a tree whose tests were
green, so do not assume the well is dry. The registers are under
metasystem/records/misc/, from
metasystem/records/misc/metasystem-stop-critique-r4.md onwards:

- The fourth read found six material items; the design gained sections
  14.1 to 14.6 and two cut scenarios came back.
- The fifth found three material items; the design gained section 15.
- The sixth found the critical one, a stop that printed "nothing is
  running" and "stopped" with exit 0 over a live owned process; its
  findings were routed through a design round and became sections 16, 17
  and 18.
- The seventh (register -r8.md) found two material items, both the build
  contradicting design text that already existed.
- The eighth (register -r9.md, job stopverb-crit10) found five: a run
  line that claimed a suite was stopped for a run no suite hosted and
  then appended a second verdict, a printed verdict that disagreed with
  the record the same stop wrote, an arm verb that opened the fence with
  no caller classification when a temporary word was passed, two exits
  that dropped a supervision bookkeeping failure, and an up path that
  hid which phase of a stop it was reporting. Round 26 folded all five
  and added a test for each.

Three of those five were in code the two folds before them had added,
and two were regressions of the very rule the fold was written to
satisfy. So read the round-26 corrections themselves with the most
suspicion: the pattern in this chain is that a fold introduces the next
defect.

Chain completion under DESIGN-BEARING reach requires a fresh-context
critic whose reviewed job is the final work round, and this is that job.
Wido's instruction on this goal is explicit: nothing lands before it is
finished, tested and in order, and this change is too important to rush.
If you find something material, say it plainly; another fold round is
affordable and expected.

What this is: `metasystem stop`, `status` and `arm` for one checkout,
slice 1 of the design. The change puts a process-creation fence in front
of every path in the system that starts anything, so its blast radius is
the metasystem's ability to work at all: a fence that misjudges its own
record wedges a checkout, and a fence that fails open lets a stopped
machine resurrect itself.

Specification: metasystem/plans/metasystem-stop-verb-design.md, revision
3 with its addenda. Later sections win where they contradict earlier
ones: sections 16, 17 and 18 are the newest. Section 18 carries the
actionable-refusal rule, the one-final-line-per-identity rule, the
distinction between a closed fence and a completed stop, and the
classifier-data failure contract. The goal record is
metasystem/plans/goals/metasystem-stop-verb.md and carries Wido's
binding durability requirement in his own words: if he stops it, the
supervisor must die too instead of being started again. The build
contract is the brief series beginning
metasystem/plans/metasystem-stop-verb-build-brief.md, whose decisions D1
to D114 are the orchestrator's and are not yours to relitigate unless
one of them is wrong.

The orchestrator persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/stopverb-build1/rounds/26/review.json (the
diff sits beside it); take reviewedTree from that record.

# Mandate

1. **The five round-26 corrections.** For each one, does the code do what
   the design says, and did the fix introduce anything new? The run line
   may carry the suite phrase only for a run joined to a suite this stop
   actually stopped, and must print exactly one final verdict per
   identity. The printed run status must equal the status the conclusion
   wrote. Arm must classify its caller before anything else, whatever
   flags follow. A held supervision bookkeeping failure must be printed,
   recorded and able to set exit 1 on every exit from that function. The
   up path must describe the stop phase as section 18.3 requires, while
   a completed stop keeps its concise form.
2. **The fence and its durability.** After a stop, nothing that checkout
   runs comes back on its own, and only a human arm clears it. Is that
   true including a crashed stop, an unreadable or future-schema fence
   record, and a concurrent arm? Does anything delete or rewrite the
   record outside arm and stop? Is there any other path that opens the
   fence without classifying its caller?
3. **The creation-claim handshake.** Section 13.1 claims completeness: a
   creator either publishes before stop's last pass and is stopped, or
   its own second read sees the closed fence and ends what it started.
   Is that argument sound against the real seams, and are the tests
   deterministic rather than timing-dependent?
4. **The order and the deadlines.** Suites before the runs that host
   them; the owner's published teardown ceiling instead of a fixed wait;
   the late-arrivals pass; section 4 step 4's five scaled seconds for a
   run whose workload was a stopped suite. Does the built order preserve
   the evidence the report reads, and can a slow but cooperating
   component be force-killed?
5. **Honesty.** Every line comes from an outcome; NOT STOPPED sets exit
   1 and lists the survivor; the escalated registry row is appended only
   when the caller force-killed the owner; the stopped state names its
   actor. Can any line claim more than the code did? The precedence rule
   is section 9's: a gate that fails only because the checkout is
   stopped must never be the reason a caller is given.
6. **The readers.** Enumerate the creation paths in the tree and check
   each against the reader table. A path that creates without reading
   the fence is the finding this review exists to catch.
7. **Authority and the seat.** stop and arm are human acts at a
   terminal, status is open; the seat's own session, announcement and
   lease survive a stop, and its Stop hook cannot resurrect anything.
8. **Scope.** Nothing outside the declared boundary; no test weakened.
   The chain's honest gaps are the five unbuilt slice-1b acceptance
   scenarios (proof-run-stop, remote-job, slow-owner, crash-recovery,
   ignored-signal), the slice-2 fleet scenario, the dispatch creation
   claim that can be orphaned if the dispatch process dies between
   replacing its cleanup trap and closing the claim, and the mission
   loop's two fence reads rather than one per iteration. Judge whether
   the deferral is honest or whether one of them hides a defect in what
   was built.

# Constraints

Wall-clock budget: 60 minutes. Return per the code-critic schema with
the reviewedTree from the persisted round-26 review record. Gap rule:
stop and report a gap; never fill it silently. Your sandbox cannot run
the fixture beds or several of the packages; do not treat that as
evidence about the diff, and do not weaken anything to make it runnable.

# Orchestrator runs

On the reviewed tree (213aa359c171f749166e3788c17860bc18bde6f1, which
the round-26 review record names), from this Mac, outside any delegate
sandbox, the orchestrator ran:

- `go test -count=1 -timeout 40m` over all eleven packages of the
  boundary: stopfence, stoptransition, up, supervise, steward, run,
  proofrun, dispatch, missionrunner, goal and cmd/metasystem. Ten are
  green. cmd/metasystem has exactly one failure,
  TestGoalTemporaryAuthorityRefusesPastAndBeyondHorizon, which fails the
  same way on main because the temporary-authority horizon passed on
  2026-09-06; goal fixture-review-by-date-rolls-over owns it. The
  process-group visibility probe that the builder's sandbox could not
  observe passes here.
- `scripts/agents/supervision-fixtures.sh`: all seventeen scenarios
  passed, including all six of slice 1's acceptance set
  (stop-everything, seat-survives, status-is-live, stop-fence,
  arm-again, arm-refuses-survivor) and including rearm-launch-fails,
  which was red on main earlier in this chain.
- `scripts/agents/suite-progress-fixtures.sh`, `dispatch-fixtures.sh`,
  `goal-cli-fixtures.sh` and `mission-fixtures.sh` all exited 0.
- The builder ran `scripts/agents/go-gate.sh --fast` green on this tree:
  formatting, vet, static analysis, refusal register and build.
- In a throwaway clone of this repository with this diff applied,
  `metasystem status --repo .` printed the checkout and "nothing is
  running", and `metasystem stop --repo .` refused the orchestrator with
  "stop is a human act at a terminal; this caller is DELEGATE" and named
  the command to type at an agent-free terminal. No agent can prove the
  human half of this verb; that proof is Wido's to run.

So the acceptance is proven and the gate is clean. What is NOT proven is
anything about the code's judgment, which is what you are for.
