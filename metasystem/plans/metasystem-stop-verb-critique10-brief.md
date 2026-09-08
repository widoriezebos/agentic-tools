Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Review brief: the closing critique of the stop verb (chain stopverb-build1, final work round stopverb-build1-r29)

FINDING IDS: chain-unique, SVC13-01, SVC13-02, ... never F-n.

Why this review exists: this is the tenth independent read of this
chain, now twenty-nine build rounds old, and the read that must close
it. Chain completion under DESIGN-BEARING reach requires a fresh-context
critic whose reviewed job is the final work round, and this is that job.
Wido's instruction on this goal is explicit: nothing lands before it is
finished, tested and in order, and this change is too important to rush.

The registers of every earlier read are under
metasystem/records/misc/, from
metasystem/records/misc/metasystem-stop-critique-r4.md onwards. Read
-r12.md and -r11.md before you start, because they carry the pattern
this chain has: a fold that patches an instance leaves the class open,
and the next read finds it again. The counts across the last four reads
were six, five, four and three, so this chain is converging, and the
last read found one thing that still mattered a great deal: a second
command that can open a stopped checkout, mission start and resume,
classified its caller against the top of the checkout rather than the
directory holding the engine, so in a subdirectory installation the
classifier had no agent signature to match at all.

Rounds 28 and 29 answered that read:

- Every verb-classifier call site now receives the resolved
  installation, and the round-28 return enumerates them all.
- The shared phase description, its remedy and its health prefix now
  refuse a record that is not closed, and every creation path separates
  a genuinely closed fence from an open newer generation, describing the
  second as a completed stop followed by a human arm.
- All seven refusal families of stop and arm now preserve the caller's
  installation flag, and the round-28 return gives the exact second line
  each one prints.
- Health uses the shared phase spelling, including the case where a
  record claims a completed stop while still listing unresolved
  entries.
- The mission refusal now prints a command that works, which deviates
  from the design's reader table; the return records that so the page
  can be corrected separately.
- Round 29 changed no file. It exists only to declare
  scripts/agents/brain-fixtures.sh in the cumulative boundary, whose
  pinned list of classifier call sites had to move with them.

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
to D123 are the orchestrator's and are not yours to relitigate unless
one of them is wrong.

The orchestrator persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/stopverb-build1/rounds/29/review.json (the
diff sits beside it); take reviewedTree from that record.

# Mandate

1. **Every path that opens the fence.** Enumerate them yourself rather
   than trusting the return: arm, the mission handover, and anything
   else. For each, is the caller classified, and is the classifier given
   an installation directory that actually holds the runtime adapters?
   This is the class Wido's requirement lives in and the last two reads
   each found one instance of it.
2. **The class fixes of rounds 27 and 28.** Is the run line derived from
   what the stop did and the record it wrote, in every branch? Is the
   classifier call-site enumeration in the round-28 return complete? Do
   the shared phase helpers refuse every input they cannot describe? Do
   all seven refusal families name a command that works, with only flags
   their own verb accepts?
3. **The fence and its durability.** After a stop, nothing that checkout
   runs comes back on its own, and only a human arm clears it. Is that
   true including a crashed stop, an unreadable or future-schema fence
   record, and a concurrent arm? Does anything delete or rewrite the
   record outside arm and stop? Is there any path that opens the fence
   without classifying its caller? The last three reads found nothing
   here; verify rather than assume.
4. **The creation-claim handshake.** Section 13.1 claims completeness: a
   creator either publishes before stop's last pass and is stopped, or
   its own second read sees the closed fence and ends what it started.
   Is that argument sound against the real seams, and are the tests
   deterministic rather than timing-dependent?
5. **The order and the deadlines.** Suites before the runs that host
   them; the owner's published teardown ceiling instead of a fixed wait;
   the late-arrivals pass; section 4 step 4's scaled five seconds for a
   run whose workload was a stopped suite.
6. **Honesty.** Every line comes from an outcome; NOT STOPPED sets exit
   1 and lists the survivor; the escalated registry row is appended only
   when the caller force-killed the owner; the stopped state names its
   actor. Can any line claim more than the code did? Section 9's
   precedence rule: a gate that fails only because the checkout is
   stopped must never be the reason a caller is given.
7. **Authority and the seat.** stop and arm are human acts at a
   terminal, status is open; the seat's own session, announcement and
   lease survive a stop, and its Stop hook cannot resurrect anything.
8. **Scope.** Nothing outside the declared boundary; no test weakened.
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
the reviewedTree from the persisted round-29 review record. Gap rule:
stop and report a gap; never fill it silently. Your sandbox cannot run
the fixture beds or several of the packages; do not treat that as
evidence about the diff, and do not weaken anything to make it runnable.

# Orchestrator runs

On the reviewed tree (1c439ebc0191818c4d5fce9d841d1765b0cd8e4e, which
the round-29 review record names; round 29 changed no file, so this is
the round-28 tree), from this Mac, outside any delegate sandbox, the
orchestrator ran:

- `go test -count=1 -timeout 40m` over all eleven packages of the
  boundary: stopfence, stoptransition, up, supervise, steward, run,
  proofrun, dispatch, missionrunner, goal and cmd/metasystem. Exactly
  one test fails,
  TestGoalTemporaryAuthorityRefusesPastAndBeyondHorizon, which fails the
  same way on main because the temporary-authority horizon passed on
  2026-09-06; goal fixture-review-by-date-rolls-over owns it. The
  process-group ownership probe the builder's sandbox cannot observe
  passes here.
- `scripts/agents/go-gate.sh --fast` exited 0: formatting, vet, static
  analysis, refusal register and build.
- `scripts/agents/supervision-fixtures.sh`: sixteen of seventeen
  scenarios passed, including all six of slice 1's acceptance set
  (stop-everything, seat-survives, status-is-live, stop-fence,
  arm-again, arm-refuses-survivor). The failure is rearm-launch-fails,
  which the orchestrator ran alone against main with a minted fixture
  capability: it fails there too with the same symptom, "the next up did
  not report a repaired runner". Another seat's goal record
  (hook-root-resolver-design) also names it red and owned elsewhere.
- The supervision-hook, brain, suite-progress, dispatch, goal-cli and
  mission beds are running as this brief is written; on the round-27
  tree, which differs from this one only by the round-28 corrections,
  all six exited 0. If any of them fails on this tree the orchestrator
  will say so in its own report and treat it as this chain's problem,
  not yours. The brain bed matters this round: its actor-seam scenario
  pins the classifier call-site list that D119 moved.
- The builder ran `bash -n` over the amended supervision hook.
- In a throwaway clone of this repository with the chain's diff applied,
  `metasystem status --repo .` printed the checkout and "nothing is
  running", and `metasystem stop --repo .` refused the orchestrator with
  "stop is a human act at a terminal; this caller is DELEGATE" and named
  the command to type at an agent-free terminal. No agent can prove the
  human half of this verb; that proof is Wido's to run.

So the acceptance is proven and the gate is clean. What is NOT proven is
anything about the code's judgment, which is what you are for.
