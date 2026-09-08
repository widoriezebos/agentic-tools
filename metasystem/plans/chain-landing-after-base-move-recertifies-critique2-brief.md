Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal chain-landing-after-base-move-recertifies)
Date: 2026-09-08

# Review brief: the closing critique of recertification (chain clbm-build2, round 1)

FINDING IDS: chain-unique, CLC-01, CLC-02, ... never F-n.

This is the independent read that closes the chain. Chain completion
under DESIGN-BEARING reach requires a fresh-context critic whose
reviewed job is the final work round, and this is that job.

## What this change is, and why it matters more than its size

When main touches a file a reviewed chain also touched, the landing
applies the certified patch to current HEAD, finds the result differs
from the critic's reviewed tree, and refuses. No candidate tree can
satisfy that check, so a finished, green, independently reviewed chain
becomes unlandable. It has happened three times, and one of those
landings only completed because a human authorised a `--no-verify`
bypass. A headless node has no such escape.

So this change adds a path that lets such a chain land. That is exactly
the shape of change where a mistake is expensive: it sits on the seam
that decides whether unreviewed code can reach main. Read it as a
security boundary, not as a convenience.

Specification: metasystem/plans/chain-landing-after-base-move-recertifies-design.md,
revision 2, final. Its critique ladder is closed: one independent read,
job clbm-crit1, five material findings, all folded, disposition at
metasystem/records/misc/chain-landing-after-base-move-recertifies-critique-r1.md.
The contract is
metasystem/plans/goals/chain-landing-after-base-move-recertifies.md.
The build contract is
metasystem/plans/chain-landing-after-base-move-recertifies-build-brief.md
and the continuation brief beside it.

The orchestrator persisted the diff and reviewed tree at
metasystem/artifacts/agents/clbm-build2/rounds/1/review.json; take
reviewedTree from that record.

## Mandate

1. **Can anything unreviewed reach main through this path?** Attack it.
   A candidate that quietly changes a certified file. A recertification
   record that is forged, stale, replayed from another chain, or points
   at a different target. A merge that is not actually disjoint. A
   receipt for a different tree or a different command. For each, does
   the landing hard-refuse, and does it refuse for the reason that
   defence exists rather than incidentally?
2. **Is the binding re-aimed or weakened?** The landing now compares
   against a mechanically derived merged tree instead of the critic's
   reviewed tree. Say whether the guarantee that survives is still
   "every line was either reviewed or came from main", and name any way
   a third line could enter.
3. **The text predicate.** The design requires ONE function to decide
   what content the proof accepts, used by the producer, the proof
   recomputation and the canary alike. Verify there is exactly one, that
   all three call it, and that its rule matches the page: valid UTF-8,
   control ranges forbidden except tab, newline and carriage return.
   A second copy of that rule anywhere is a finding.
4. **The overlap decision.** Read the hunk parsing and the disjointness
   test against Git's real behaviour, including the optional section
   suffix on hunk headers and the no-final-newline marker. Would two
   implementations agree? Can adjacent or boundary-touching hunks be
   called disjoint when they are not?
5. **Review accounting untouched.** No merge-only critic lane exists by
   design. Verify nothing lets a review round escape the box, that
   `reviewRoundLimit` and `criticRoundsConsumed` are unchanged, and that
   the park path cannot reset accounting.
6. **The area-width command.** It must be an explicit coordinator input
   with no default and no inference, frozen in the record and both
   digests, and matched byte for byte against the receipt. Confirm there
   is no path where an empty, inferred or substituted command is
   accepted.
7. **The headless failure contract.** One durable move, a park that a
   reader can tell from a stall, and never a bypass. Check what happens
   when recording the park itself fails.
8. **The canaries.** Three exist and pass. Can each fail for the right
   reason? The refusal observations were corrected once already in this
   design, because the first revision asked only that the landing not
   pass, which many wrong implementations also satisfy. Verify the
   corrected versions keep every unrelated prerequisite valid so that
   only the defence under test can produce the refusal.
9. **Scope.** No change to what a claim or approval means, no widening
   of what an agent may do, no edits to finding registration or goal
   budgets.

## One thing to know about this chain's history

Round 1 was killed at its 120-minute cap before it could write a return.
Its code survived in a worktree, and the orchestrator carried that diff
into a fresh worktree by hand and seeded it with a marker file, because
the machinery has no supported way to continue a capped round. The
continuation round then finished the work: it resolved the one untouched
boundary target with a stated reason, fixed stale help text, added a
missing refusal inventory entry, removed an obsolete entry and corrected
four static-analysis findings.

That history is disclosed because it bears on your read in one specific
way: the code you are reading was produced across two rounds with a
hand carry between them. Check for the marks of that, a half-applied
change, a duplicated block, a file the seed brought that nothing
references, and the seed marker itself, which must not appear anywhere
in the reviewed tree.

## What to do with what you find

Say for each finding whether it lets unreviewed code reach main, changes
behaviour, or is only a sentence that reads badly. That grading decides
whether this folds or lands, and the orchestrator will follow your
severity rather than argue with it.

## Constraints

Wall-clock budget: 60 minutes. Return per the code-critic schema with
the reviewedTree from the persisted review record. Gap rule: stop and
report a gap; never fill it silently. Your sandbox cannot run the
fixture beds or the process-group probe in cmd/metasystem; do not treat
that as evidence about the diff, and do not weaken a test to make it
runnable.

## Orchestrator runs

On the reviewed tree, from this Mac, outside any delegate sandbox:

- All three design canaries pass, run individually:
  `go test ./internal/gittree -run '^TestDisjointMergeProof$'` in 4.1s;
  `go test ./cmd/metasystem -run '^TestChainLandingRecertifiesAfterBaseMove$'`
  in 14.6s;
  `go test ./cmd/metasystem -run '^TestRecertifiedLandingParksOnOriginMove$'`
  in 8.8s.
- `scripts/agents/go-gate.sh --fast` exits 0.
- The five touched packages are running in full as this brief is
  written: gittree, landing, validate, dispatch and cmd/metasystem. If
  any fails the orchestrator will say so and treat it as this chain's
  problem, not yours.
- The receipt-bound battery runs at the landing gate, not here.

So the acceptance is proven at the canary level and the gate is clean.
What is NOT proven is anything about the code's judgment, which is what
you are for.
