Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Review brief: the closing critique of the stop verb (chain stopverb-build1, final work round stopverb-build1-r25)

FINDING IDS: chain-unique, SVC10-01, SVC10-02, ... never F-n.

Why this review exists: this is the SEVENTH independent read of this
chain, now twenty-five build rounds old, and the read that closes it.
Every previous read found real defects in a tree whose tests were all
green, so assume there is one more rather than that the well is dry:

- The fourth read (register
  metasystem/records/misc/metasystem-stop-critique-r4.md) found six
  material items; the design gained sections 14.1 to 14.6 and two cut
  scenarios came back because they would have caught two findings by
  execution.
- The fifth read (register
  metasystem/records/misc/metasystem-stop-critique-r5.md) found three
  material items plus a note the orchestrator accepted; the design
  gained section 15.
- The sixth read (register
  metasystem/records/misc/metasystem-stop-critique-r6.md) found the
  critical one: a stop printed "nothing is running" and "stopped" with
  exit 0 over a live owned process. Its findings were routed through a
  design round and became sections 16, 17 and 18 of the specification.
- The seventh read of the chain's own build (register
  metasystem/records/misc/metasystem-stop-critique-r7.md) found two
  material items, both the build contradicting design text that already
  existed: a monitored run hosting a stopped suite was signalled and
  recorded as ended-unknown instead of being given its scaled five
  seconds to conclude from its sidecar, and a supervision-shutdown
  bookkeeping failure was printed nowhere, recorded nowhere and did not
  set exit 1 while a printed line claimed otherwise. Round 25 folded
  both.

Read for judgment, not for green. Chain completion under DESIGN-BEARING
reach requires a fresh-context critic whose reviewed job is the final
work round, and this is that job. Wido's instruction on this goal is
explicit: nothing lands before it is finished, tested and in order, and
this change is too important to rush. If you find something material,
say it plainly; another fold round is expected, not resented.

What this is: `metasystem stop`, `status` and `arm` for one checkout,
slice 1 of the design. The change puts a process-creation fence in front
of every path in the system that starts anything, so its blast radius is
the metasystem's ability to work at all: a fence that misjudges its own
record wedges a checkout, and a fence that fails open lets a stopped
machine resurrect itself.

Specification: metasystem/plans/metasystem-stop-verb-design.md, revision
3 with its addenda. Later sections win wherever they contradict earlier
ones: sections 16, 17 and 18 are the newest and were written after the
sixth read, and section 18 in particular carries the actionable-refusal
rule, the one-final-line-per-identity rule, the distinction between a
closed fence and a completed stop, and the classifier-data failure
contract. The goal record is
metasystem/plans/goals/metasystem-stop-verb.md and carries Wido's
binding durability requirement in his own words: if he stops it, the
supervisor must die too instead of being started again. The build
contract is the brief series beginning
metasystem/plans/metasystem-stop-verb-build-brief.md, whose decisions D1
to D107 are the orchestrator's and are not yours to relitigate unless
one of them is wrong.

The orchestrator persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/stopverb-build1/rounds/25/review.json (the
diff sits beside it); take reviewedTree from that record. The
orchestrator's runs are listed at the end.

# Mandate

1. **The fence and its durability.** Wido's requirement: after a stop,
   nothing that checkout runs comes back on its own, and only a human
   arm clears it. Is that true of the built code, including a crashed
   stop, a fence record that is unreadable or from a future schema, and
   a concurrent arm? Does anything delete or rewrite the record outside
   arm and stop?
2. **The creation-claim handshake.** Section 13.1 claims completeness: a
   creator either publishes before stop's last pass and is stopped, or
   its own second read sees the closed fence and ends what it started.
   Is that argument sound against the real seams, and are the tests
   deterministic rather than timing-dependent? Name any creator that can
   still leave a process behind.
3. **The order, the deadlines, and the run that hosted a suite.** Suites
   before the runs that host them; the owner's published teardown
   ceiling instead of a fixed wait; the late-arrivals pass; and section
   4 step 4, which gives a run whose workload was a suite stopped in
   step 3 the scaled five seconds to conclude from its exit sidecar and
   then reports it from its own record as concluded red. Round 25 was
   written to make the code match that text. Check the fold rather than
   trusting it: does the run's own verdict now survive, is the wait
   bounded, and can a run that never hosted a stopped suite be delayed
   or mislabelled by the same path?
4. **Honesty.** Every line comes from an outcome; NOT STOPPED sets exit
   1 and lists the survivor; the escalated registry row is appended only
   when the caller force-killed the owner; the stopped state names its
   actor. Can any line claim more than the code did? Round 25 also
   changed how supervision-shutdown bookkeeping failures are surfaced:
   check that a registry-publication or owner-lock-release failure is
   both printed and recorded, sets exit 1, and does not leave a
   contradicting line behind. The precedence rule is section 9's: a gate
   that fails only because the checkout is stopped must never be the
   reason a caller is given.
5. **The readers.** Enumerate the creation paths in the tree and check
   each against the reader table. A path that creates without reading
   the fence is the finding this review exists to catch.
6. **Authority and the seat.** stop and arm are human acts at a
   terminal, status is open; the seat's own session, announcement and
   lease survive a stop, and its Stop hook cannot resurrect anything.
7. **Scope.** Nothing outside the declared boundary; no test weakened.
   The chain's honest gaps are the five unbuilt slice-1b acceptance
   scenarios (proof-run-stop, remote-job, slow-owner, crash-recovery,
   ignored-signal), the slice-2 fleet scenario, and the dispatch
   creation claim that can be orphaned if the dispatch process dies
   between replacing its cleanup trap and closing the claim explicitly.
   Those are declared, not hidden. Judge whether the deferral is honest
   or whether one of them hides a defect in what was built.

# Constraints

Wall-clock budget: 60 minutes. Return per the code-critic schema with
the reviewedTree from the persisted round-25 review record. Gap rule:
stop and report a gap; never fill it silently. Your sandbox cannot run
the fixture beds or several of the packages; do not treat that as
evidence about the diff, and do not weaken anything to make it runnable.

# Orchestrator runs

On the reviewed tree (68e82f80bf4275ee7729868b0a3d136d28a5be45, which
the round-25 review record names), from this Mac, outside any delegate
sandbox, the orchestrator ran:

- `go test -count=1 -timeout 40m ./internal/stoptransition/ ./internal/run/ ./internal/supervise/ ./internal/proofrun/` GREEN. These are the four
  packages round 25 touched, and three of them the delegate sandbox can
  never run.
- `scripts/agents/supervision-fixtures.sh`: every scenario passed,
  including all six of slice 1's acceptance set (stop-everything,
  seat-survives, status-is-live, stop-fence, arm-again,
  arm-refuses-survivor), except `rearm-launch-fails`, which is red on
  main and owned by another goal.
- `scripts/agents/suite-progress-fixtures.sh` exit 0.
- On the round-24 tree, which differs from this one only by the round-25
  fold, all eleven packages of the boundary were green, and
  `scripts/agents/dispatch-fixtures.sh`, `goal-cli-fixtures.sh` and
  `mission-fixtures.sh` all exited 0, as did
  `scripts/agents/go-gate.sh --fast`.

So the acceptance is proven and the gate is clean. What is NOT proven is
anything about the code's judgment, which is what you are for.
