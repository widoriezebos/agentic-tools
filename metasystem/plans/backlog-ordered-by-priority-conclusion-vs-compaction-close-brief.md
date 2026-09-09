Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal backlog-ordered-by-priority)
Date: 2026-09-09

# Review brief: the read that closes chain bolboc-build1 (final work round bolboc-build1, round 1)

FINDING IDS: chain-unique, BCE-01, BCE-02, ... never F-n.

## Why this read exists

Chain completion under DESIGN-BEARING reach requires a fresh-context critic
whose reviewed job is the final work round. This is that read. Read for
judgment, not for green: the tests the builder ran are listed at the end and
they passed; what is not proven is anything about the code's judgment.

Specification: `metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md`
revision 2 (0260aef6), critique ladder closed at
`metasystem/records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r2.md`.
Build contract: `metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-build-brief.md`.
The orchestrator persisted the diff and the reviewed tree at
`metasystem/artifacts/agents/bolboc-build1/rounds/1/review.json`; take
`reviewedTree` from that record.

## Mandate, in order of consequence

1. **The helper is the only reader (page 1.4 step 5).** After this change no
   function in `internal/metrics` scans history for `done` or `split` except
   `concludedAt`. Confirm by search, not by the builder's word; a private
   scan left anywhere is the finding this read exists to catch. Confirm
   `concludedAt` matches the page's printed code in the one behaviour that
   matters: a malformed stamp on the archive line yields no conclusion, and
   it does not fall through to an earlier line.
2. **The canaries failed for the right reason.** The builder's evidence must
   show `split-parent` and `waiting-row` red on the untouched tree with the
   `Value` line naming `proving_hours=12.000 waiting_share=0.500`, observed
   (level `ran`), not derived. Then green after. Check the observed text is
   the page's, not a paraphrase, and that no assertion was loosened to pass.
3. **The premise pins pin the writers, not the page.** The three
   `TestPriorityLifecycle` extensions must assert facts the writers produce
   and must have passed on the untouched tree. Check none was adjusted.
4. **The Refused category (page 2.3).** The branch at the gate call keeps
   the gate's answer and adds no second traversal; `SelectNext` never reads
   Refused; `idleBacklogDigest` ignores it and the fixture proves two
   readings differing only in Refused hash the same; `enforceIdleBacklog`
   neither blocks nor counts; the three surface strings are the page's
   exactly; `goal list` is untouched.
5. **The wall (page section 3).** The diff touches exactly the section 3
   change list plus the new test file and nothing in the untouched list.
   `AGENTS.md` untouched. No rank set, inferred or moved.
6. **Comments.** No round, slice or finding references in source comments.

## Constraints

Wall-clock budget: 45 minutes. Return per the code-critic schema with
`reviewedTree` from the persisted round record. Gap rule: stop and report a
gap. Your sandbox cannot run the fixture beds; the orchestrator ran them and
the results are below. Do not weaken anything to make them runnable.

## Orchestrator runs

On the reviewed tree (55e0fe87e2c8ac3cdc071ab60ef61e2764ab8171, base commit
40924132, 12 files, diff sha256 31da3c0c5b77d8caf51eff6ac00faa2b53ce33b4869630eb06754402940014da),
from this Mac, outside any delegate sandbox:

- The orchestrator ran `go test ./internal/metrics`, the goal package's
  `TestPriorityLifecycle`, `TestNextPriority` and
  `TestRefusedBacklogIsReportedWithoutBlocking`, the command
  `TestGoalPrioritySelection` and the channel `TestReportPriority`: all
  green on the build's tree.
- The builder's evidence, all at level ran: `split-parent` and `waiting-row`
  RED on the untouched tree with Value naming
  `proving_hours=12.000 waiting_share=0.500`; the three
  `TestPriorityLifecycle` premise pins GREEN on the untouched tree; all eight
  conclusion canaries and the metrics package green after the repair; the
  command canary red with `no matching eligible work` then green; the two
  channel canaries red without the `skipped` clause then green; the seat
  test green; the fast gate green; a search showing the only production
  `done`/`split` literals in `internal/metrics` are `archiveVerbs`.
- The builder reports one optional widened run was not green for reasons
  off the changed paths: the goal package hit the shared two-minute timeout
  in an unrelated authority fixture and the command package failed an
  unrelated two-second process-ownership probe. Judge whether that is truly
  unrelated; it is the one soft spot in the evidence.
- Process-owning beds, run by the orchestrator on the reviewed tree:
  RUNNING at the time of this dispatch (started 10:06Z on the reviewed tree,
  as a tracked task on the orchestrator's side). Their result is appended to
  the critique register before certification. A failed bed folds the chain
  whatever this read finds, so do not wait on them and do not infer anything
  from their absence here. The same two beds passed on the pre-build tree
  this morning in a clean re-run.

So the acceptance is proven and the gate is clean. What is NOT proven is
anything about the code's judgment, which is what you are for.
