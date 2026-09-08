Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Review brief: the closing critique of slice 2, taking work through the rank (chain bolnext-build1, round 1)

FINDING IDS: chain-unique, BON-01, BON-02, ... never F-n.

This is the independent read that closes the chain. Chain completion
under DESIGN-BEARING reach requires a fresh-context critic whose
reviewed job is the final work round, and this is that job.

## What this is

Slice 1 gave every goal record a priority and a sequence, a human verb
that sets and re-sequences them, and an ordered listing. It landed at
f32fbfeb. Wido then ranked the backlog and said the machinery does not
follow it: the listing prints the order but nothing routes work by it.

Slice 2 is what routes work by it. `goal next --machine <nick>` hands a
free machine the first approved unclaimed goal in priority-then-sequence
order that it is allowed to take. The channel status shows the head. The
seat projections display the same order. Seats are told to take work
through the verb instead of reading records.

That makes this change the thing that decides what every machine in the
fleet does next, which is why it gets a real read rather than a glance.

Specification: metasystem/plans/backlog-ordered-by-priority-design.md,
revision 2, final, sections 4 and 5 and the seat projection paragraphs
after section 4. Its critique ladder is closed: one independent read,
disposition at
metasystem/records/misc/backlog-ordered-by-priority-critique-r1.md. The
slice-1 reads are at -r2.md and -r3.md and are worth skimming, because
their pattern recurs. The contract is
metasystem/plans/goals/backlog-ordered-by-priority.md.

The build contract is
metasystem/plans/backlog-ordered-by-priority-slice2-brief.md. The
orchestrator persisted the diff and reviewed tree at
metasystem/artifacts/agents/bolnext-build1/rounds/1/review.json.

## Mandate

1. **Can two machines be sent to the same goal, or can a goal be
   skipped forever?** The page is explicit that `next` is a read rather
   than a reservation, so two machines may receive the same candidate
   and claim publication admits one winner while the loser reselects.
   Verify that is what happens, that the loser actually reselects rather
   than stalling, and that no goal can be permanently passed over by the
   ordering.
2. **Eligibility.** One claim per machine, pins as a filter and never as
   a rank, blocked and unapproved goals skipped, unranked last. Prove a
   pinned goal cannot jump its rank, and that a machine is never handed
   work it may not claim.
3. **Honest emptiness.** What does the verb do when nothing qualifies,
   and can a caller tell that from an error? A silent empty answer that
   looks like a failure would strand a headless node.
4. **The diagnostics stay diagnostics.** The page insists the seat
   projections display the order rather than deciding eligibility
   differently, and that the idle digest stays insensitive to order so a
   reorder does not reset idle enforcement. Check both, because a
   diagnostic that quietly becomes a second eligibility rule is the
   defect that would be hardest to see later.
5. **The channel head.** Global and local heads have distinct meanings
   in the page. Are they distinct in the code, and can the report claim
   a head that a machine could not actually take?
6. **The two carried items.** The fetch option must route through the
   existing projection rather than a second fetch path. The nil-record
   assertion must report a missing record without dereferencing it.
7. **The canaries.** Can each fail for the right reason? Four canaries
   in this program have been found unable to, the last because it asked
   whether an unsigned-parsed integer could be negative. Check the
   pinned-machine row especially: it must fail if a pin ever confers
   rank.
8. **Scope.** No change to what a claim or an approval means, no
   reservation semantics added to `next`, no new authority. The metrics
   defect from the slice-1 read is deliberately out of scope; confirm it
   was left alone rather than half-touched.

## What to do with what you find

Say for each finding whether it changes what a machine does next, is a
diagnostic that misleads a human, or is only a sentence that reads
badly. That grading decides whether this folds or lands.

## Constraints

Wall-clock budget: 60 minutes. Return per the code-critic schema with
the reviewedTree from the persisted review record. Gap rule: stop and
report a gap; never fill it silently. Your sandbox cannot run the
fixture beds; do not treat that as evidence about the diff, and do not
weaken a test to make it runnable.

## Orchestrator runs

On the reviewed tree, from this Mac, outside any delegate sandbox:

- The priority and next canaries pass: `go test ./internal/goal -run
  'TestNextPriority|TestPriority'` in 39.2s, `go test ./cmd/metasystem
  -run 'TestGoalPriority'` in 3.3s, `go test ./internal/channel -run
  'TestReportPriority'` in 1.4s.
- `scripts/agents/go-gate.sh --fast` exits 0.
- The command-package process-identity probe that the builder reported
  failing in its sandbox passes here in 0.2 seconds, so that failure was
  the sandbox and not the change.
- The goal, channel, steward and command packages in full, plus the
  goal-command fixture bed, are running as this brief is written. If any
  fails the orchestrator will say so and treat it as this chain's
  problem, not yours.

So the acceptance is proven at the canary level and the gate is clean.
What is NOT proven is anything about the code's judgment, which is what
you are for.
