Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Review brief: the closing critique of the backlog order, slice 1 (chain backlogorder-build1, final work round backlogorder-build1-r2)

FINDING IDS: chain-unique, BOC-01, BOC-02, ... never F-n.

This is the independent read that closes the chain. Chain completion
under DESIGN-BEARING reach requires a fresh-context critic whose
reviewed job is the final work round, and this is that job.

There was one read before you, of round 1 (job backlogorder-crit2,
register metasystem/records/misc/backlog-ordered-by-priority-critique-r2.md).
It found two material items and three notes, and round 2 folded four of
the five. Read that register first, because its high finding is the
pattern to carry into your own read: round 1 passed thirty focused
canaries while breaking the fixture bed the design itself names as its
gate, and no canary could see it. So ask what else reads a shape this
build changed.

Round 2 taught the goal-command bed the new table while keeping all
three of its proofs including the negative one that had silently stopped
matching, replaced a canary clause that looked for a bare hyphen and
therefore could never fail, documented the two fields and the human verb
in the mechanism document, and made the malformed-tree refusal name the
existing repair route. Check those four as suspiciously as the rest: in
this chain's sibling goal, folds introduced the next defect twice.

What this is: slice 1 of ordering the backlog. Every goal record gains a
priority and a sequence, a human verb sets and moves them, and the
listing prints the order. It touches the ledger's record grammar and its
tree validation, so a defect here can refuse a lawful human act or wedge
the whole queue behind a validation that no verb can satisfy.

Specification: metasystem/plans/backlog-ordered-by-priority-design.md,
revision 2, final. Its critique ladder is closed and its disposition is
metasystem/records/misc/backlog-ordered-by-priority-critique-r1.md.
Sections 1, 2 and 3 are in scope for this build; section 4, section 5
and the seat projections are slice 2 and deliberately untouched. The
contract behind the design is
metasystem/plans/goals/backlog-ordered-by-priority.md, carrying Wido's
own sentence.

The build contract is
metasystem/plans/backlog-ordered-by-priority-build-brief.md. The
orchestrator persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/backlogorder-build1/rounds/2/review.json;
take reviewedTree from that record.

# Mandate

1. **The invariant against every lawful act.** The tree validation
   requires each priority's ranked open goals to occupy exactly 1..N,
   once each. Walk every path that can add, remove or move a ranked
   record: set-priority, done, reopen, split, the approval sweep,
   reconcile batches, steal, resume, park, unpark, arc detach. Can any
   lawful human act now be refused because the invariant cannot be
   satisfied? Can any leave a hole or a duplicate? This is the finding
   that would wedge the queue, so spend your budget here first.
2. **Rank is a human act.** Can any agent path set, shift or erase a
   rank: reconcile, recovery, the sweep, a stored human string from a
   dead owner, a seat verb? The build claims recovery refuses an
   unpublished set-priority intent; check that claim.
3. **The re-sequencing rule.** Insert, append, move between priorities,
   omit the sequence, and the no-op. Does each do what section 2 says,
   touch only the records it must, and produce one event and one
   revision where the design says one?
4. **Concurrency.** The design leans on the existing publish seam for
   rank-versus-rank and rank-versus-claim collisions. Are the tests
   deterministic rather than timing-dependent, and do they prove what
   they claim, in both publication orders?
5. **The canaries themselves.** Section 6 names a smallest proving run
   per fixture. For each row the build implemented, can that canary
   actually fail for the right reason? A canary that cannot fail is the
   defect the previous read of this design found twice, so check rather
   than assume.
6. **Scope.** Nothing from slice 2. No existing goal record edited. The
   approval sweep payload and digest unchanged. No new authority, and no
   change to what a claim or an approval means. The design names four
   things the builder had to refuse to invent; check none of them
   appeared.

# What to do with what you find

Say for each finding whether it changes behaviour, can wedge the queue
or refuse a lawful human act, or is only a sentence that reads badly.
That grading decides whether this folds or lands, and the orchestrator
will follow your severity rather than argue with it.

# Constraints

Wall-clock budget: 60 minutes. Return per the code-critic schema with
the reviewedTree from the persisted review record. Gap rule: stop and
report a gap; never fill it silently. Your sandbox cannot run the
fixture beds; do not treat that as evidence about the diff, and do not
weaken a test to make it runnable.

# Orchestrator runs

On the reviewed tree (eaa78a1dca2ae64674694277ee9d9f7abaaf725c, which
the round-2 review record names), from this Mac, outside any delegate
sandbox:

- `scripts/agents/goal-cli-fixtures.sh` exits 0. This is the run that
  failed on round 1 and the reason round 2 exists, so it is the direct
  proof of that fold.
- `scripts/agents/go-gate.sh --fast` exits 0.
- `go test ./cmd/metasystem -run 'TestGoalPriority'` passes, including
  the corrected listing canary.
- On the round-1 tree, which differs only by this fold, the goal,
  command, channel and steward packages all passed in full, and the
  build's own thirty focused canaries passed.
- `scripts/agents/dispatch-fixtures.sh` is in flight as this brief is
  written. If it fails the orchestrator will say so and treat it as this
  chain's problem, not yours.
- The builder rebuilt the engine on round 1 and drove
  `goal set-priority` and `goal list` in a private fake-runtime ledger,
  observing the reordered output and a signed priority refused before
  publication.

So the acceptance is proven and the gate the design asks for is green.
What is NOT proven is anything about the code's judgment, which is what
you are for.
