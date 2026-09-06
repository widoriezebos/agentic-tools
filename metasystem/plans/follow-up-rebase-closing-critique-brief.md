Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal delegate-follow-up-cannot-merge-main)
Date: 2026-09-06

# Review brief: fresh independent critique of the final tree (chain followup-rebase-build1, work round followup-rebase-build1-r5)

FINDING IDS: chain-unique, FRC-01, FRC-02, ... never F-n.

Why this review exists: chain completion under DESIGN-BEARING reach
requires a fresh-context critic chain whose reviewed job is the final
work round. Two critic chains came before this one: followup-rebase-crit1
examined round 1 (FRB-01 to FRB-04, folded in round 4) and
followup-rebase-crit2 examined round 4 (FRF-01 to FRF-03, folded in
round 5). This chain examines the FINAL tree in a fresh session and
closes the build chain.

Round budget: 1 focused round. R-60-m1's rule: a finding is material
only if it changes what gets built and names the artifact it would
change.

Threat model and scope: as in
metasystem/plans/follow-up-rebase-code-critique-brief.md. Contract:
metasystem/plans/follow-up-rebase-build-brief.md (D1 to D6), the fold
decisions D7 to D11 in metasystem/plans/follow-up-rebase-fold2b-brief.md
and metasystem/plans/follow-up-rebase-fold2c-brief.md, the round-5 fold
decisions D12 to D14 in metasystem/plans/follow-up-rebase-fold3-brief.md
with the dispositions of the previous critique in
metasystem/plans/dispositions/follow-up-rebase-final-critique-r1.md, and
the goal record metasystem/plans/goals/delegate-follow-up-cannot-merge-main.md.
The orchestrator persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/followup-rebase-build1/rounds/5/review.json
(the diff sits beside it); take reviewedTree from that record. The
orchestrator's runs on the reviewed tree are listed at the end.

# Mandate

1. The three round-5 folds, judged in the dispatcher script: D12, a
   failed restore keeps the tagged stash and the refusal names its hash
   and tag, never dropping it; D13, brief authority runs on the caller's
   brief file and never on the dispatcher's prefixed message, so a
   modify/delete conflict is admitted and retryable; D14, the conflict
   paragraph asks the builder to resolve AND stage each conflicted path
   with git add. For each: done as decided, or does the fold move the
   problem? Rule by decision id.
2. Everything the earlier chains already accepted still holds on the
   final tree: the rebase runs only when a chain path moved on the
   trunk; the stash is addressed by hash and tag; the fast-forward is
   ff-only; the apply counts as a conflicted success only when every
   untracked stash path stands in the worktree with the stash's bytes;
   a wrapper that finds unmerged entries records them and prepends the
   paragraph; the plan verb's boundary, dirty-set and unmergedPaths
   rules; the record builder's three rebase-field shapes.
3. The dispatch fixture scenarios cover the three folds (a failed
   restore that retains and identifies its stash, an admitted and
   retryable modify/delete conflict, instructions that require staging)
   and would fail against the round-4 code; no test weakened; nothing
   outside the declared eleven-path boundary; the two .orig deletions
   touch nothing else.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema with
the reviewedTree from the persisted round-5 review record. Gap rule:
stop and report a gap; never fill it silently.

# Orchestrator runs

On the reviewed tree (9b51f3d3f5df960168405ee0a785db67c7e0860b), from
this Mac, evidence level ran: `gofmt -l` on the dispatch and command
packages printed nothing; `go vet` passed for both; `bash -n
scripts/agents/dispatch.sh` passed; `go test ./internal/dispatch/
-count=1` passed in 25.5 seconds; `git apply --check
--directory=metasystem` of the round-5 diff against main at 887e4384
succeeded although main moved dispatch-fixtures.sh since the chain's
base. The implementer ran the required full gate green in its sandbox:
`scripts/agents/go-gate.sh --fast`, the whole dispatch fixture bed
including the three new scenarios, and the goal command fixtures.
