Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Review brief: the closing critique of the stop verb (chain stopverb-build1, final work round stopverb-build1-r27)

FINDING IDS: chain-unique, SVC12-01, SVC12-02, ... never F-n.

Why this review exists: this is the ninth independent read of this
chain, now twenty-seven build rounds old, and the read that must close
it. Chain completion under DESIGN-BEARING reach requires a fresh-context
critic whose reviewed job is the final work round, and this is that job.
Wido's instruction on this goal is explicit: nothing lands before it is
finished, tested and in order, and this change is too important to rush.

The registers of every earlier read are under
metasystem/records/misc/, from
metasystem/records/misc/metasystem-stop-critique-r4.md onwards. Read the
two most recent before you start, -r11.md and -r9.md, because they carry
the pattern this chain has: a fold that patches an instance leaves the
class open, and the next read finds it again. Three reads in a row found
the same fault in one line builder. Round 27 was therefore written as
class fixes:

- The three verbs resolve their scope one way again, through the
  installation the caller names, and no refusal names a flag the verb
  that produced it rejects.
- The monitored-run outcome now carries both the status read when stop
  acts and the final durable status, and the whole grammar of that one
  line is pinned by a test rather than one branch at a time.
- Every human-facing reader of the fence record was enumerated, named in
  the round-27 return, and made to describe the durable phase and choose
  its matching remedy. The builder's own enumeration also found one more
  defect nobody had reported: health classified a stopped-phase record
  carrying unresolved survivors as complete.

So the first question of this read is whether those three classes are
actually closed, not whether the four reported instances are fixed.

What this is: `metasystem stop`, `status` and `arm` for one checkout,
slice 1 of the design. The change puts a process-creation fence in front
of every path in the system that starts anything, so its blast radius is
the metasystem's ability to work at all: a fence that misjudges its own
record wedges a checkout, and a fence that fails open lets a stopped
machine resurrect itself.

Specification: metasystem/plans/metasystem-stop-verb-design.md, revision
3 with its addenda. Later sections win where they contradict earlier
ones; sections 16, 17 and 18 are the newest. The goal record is
metasystem/plans/goals/metasystem-stop-verb.md and carries Wido's
binding durability requirement in his own words: if he stops it, the
supervisor must die too instead of being started again. The build
contract is the brief series beginning
metasystem/plans/metasystem-stop-verb-build-brief.md, whose decisions D1
to D118 are the orchestrator's and are not yours to relitigate unless
one of them is wrong.

The orchestrator persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/stopverb-build1/rounds/27/review.json (the
diff sits beside it); take reviewedTree from that record.

# Mandate

1. **The three class fixes.** Is the run line now derived from what the
   stop did and the record it wrote, in every branch? Is the reader
   enumeration in the round-27 return complete and honest, including the
   readers it declares as needing no phase? Do the three verbs share one
   scope resolution in every layout, and does every refusal name only
   flags its own verb accepts?
2. **The fence and its durability.** After a stop, nothing that checkout
   runs comes back on its own, and only a human arm clears it. Is that
   true including a crashed stop, an unreadable or future-schema fence
   record, and a concurrent arm? Does anything delete or rewrite the
   record outside arm and stop? Is there any path that opens the fence
   without classifying its caller? The last three reads found nothing
   here; verify rather than assume.
3. **The creation-claim handshake.** Section 13.1 claims completeness: a
   creator either publishes before stop's last pass and is stopped, or
   its own second read sees the closed fence and ends what it started.
   Is that argument sound against the real seams, and are the tests
   deterministic rather than timing-dependent?
4. **The order and the deadlines.** Suites before the runs that host
   them; the owner's published teardown ceiling instead of a fixed wait;
   the late-arrivals pass; section 4 step 4's scaled five seconds for a
   run whose workload was a stopped suite.
5. **Honesty.** Every line comes from an outcome; NOT STOPPED sets exit
   1 and lists the survivor; the escalated registry row is appended only
   when the caller force-killed the owner; the stopped state names its
   actor. Can any line claim more than the code did? Section 9's
   precedence rule: a gate that fails only because the checkout is
   stopped must never be the reason a caller is given.
6. **Authority and the seat.** stop and arm are human acts at a
   terminal, status is open; the seat's own session, announcement and
   lease survive a stop, and its Stop hook cannot resurrect anything.
7. **Scope.** Nothing outside the declared boundary; no test weakened.
   The chain's declared gaps are the five unbuilt slice-1b acceptance
   scenarios (proof-run-stop, remote-job, slow-owner, crash-recovery,
   ignored-signal), the slice-2 fleet form, the dispatch creation claim
   that can be orphaned if the dispatch process dies between replacing
   its cleanup trap and closing the claim, and the mission loop's two
   fence reads rather than one per iteration. Judge whether the deferral
   is honest or whether one of them hides a defect in what was built.

# What to do with what you find

Say plainly whether each finding changes behaviour, can wedge a
checkout, or is a sentence that reads badly. That distinction decides
whether this chain folds again or lands, and the orchestrator will
follow your severity rather than argue with it.

# Constraints

Wall-clock budget: 60 minutes. Return per the code-critic schema with
the reviewedTree from the persisted round-27 review record. Gap rule:
stop and report a gap; never fill it silently. Your sandbox cannot run
the fixture beds or several of the packages; do not treat that as
evidence about the diff, and do not weaken anything to make it runnable.

# Orchestrator runs

On the reviewed tree (b44a3a38c4847177d9157f9c53152b221a33d149, which
the round-27 review record names), from this Mac, outside any delegate
sandbox, the orchestrator ran:

- `go test -count=1 -timeout 40m` over all eleven packages of the
  boundary: stopfence, stoptransition, up, supervise, steward, run,
  proofrun, dispatch, missionrunner, goal and cmd/metasystem. Exactly
  one test fails,
  TestGoalTemporaryAuthorityRefusesPastAndBeyondHorizon, which fails the
  same way on main because the temporary-authority horizon passed on
  2026-09-06; goal fixture-review-by-date-rolls-over owns it. The
  process-group ownership probe the builder's sandbox cannot observe
  passes here.
- `scripts/agents/supervision-fixtures.sh`: sixteen of seventeen
  scenarios passed, including all six of slice 1's acceptance set
  (stop-everything, seat-survives, status-is-live, stop-fence,
  arm-again, arm-refuses-survivor). The failure is rearm-launch-fails,
  which the orchestrator then ran alone against main with a minted
  fixture capability: it fails there too, with the same symptom, "the
  next up did not report a repaired runner". Another seat's goal record
  (hook-root-resolver-design) also names it red and owned elsewhere. It
  is not this chain's.
- `scripts/agents/supervision-hook-fixtures.sh` exited 0, which matters
  because round 27 changed that hook script.
- `scripts/agents/suite-progress-fixtures.sh`, `dispatch-fixtures.sh`,
  `goal-cli-fixtures.sh` and `mission-fixtures.sh` all exited 0.
- The builder ran `scripts/agents/go-gate.sh --fast` green on this tree,
  and `bash -n` over the amended supervision hook.
- In a throwaway clone of this repository with the round-25 diff applied,
  `metasystem status --repo .` printed the checkout and "nothing is
  running", and `metasystem stop --repo .` refused the orchestrator with
  "stop is a human act at a terminal; this caller is DELEGATE" and named
  the command to type at an agent-free terminal. No agent can prove the
  human half of this verb; that proof is Wido's to run.

So the acceptance is proven and the gate is clean. What is NOT proven is
anything about the code's judgment, which is what you are for.
