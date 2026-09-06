Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal delegate-follow-up-cannot-merge-main)
Date: 2026-09-06

# Review brief: fresh independent critique of the final tree (chain followup-rebase-build1, work round followup-rebase-build1-r4)

FINDING IDS: chain-unique, FRF-01, FRF-02, ... never F-n.

Why this review exists: chain completion under DESIGN-BEARING reach
requires a fresh-context critic chain whose reviewed job is the final
work round. The earlier critic chain (followup-rebase-crit1) examined
round 1 and found four material items (FRB-01 to FRB-04), all accepted
and folded in round 4 (rounds 2 and 3 changed nothing: an empty brief
and a boundary gap, both orchestrator errors). This chain is the independent examination
of the FINAL tree in a fresh session, and it closes the build chain.

Round budget: 1 focused round. R-60-m1's rule: a finding is material
only if it changes what gets built and names the artifact it would
change.

Threat model and scope: as in
metasystem/plans/follow-up-rebase-code-critique-brief.md. Contract:
metasystem/plans/follow-up-rebase-build-brief.md (D1 to D6), the fold
decisions D7 to D11 in metasystem/plans/follow-up-rebase-fold2b-brief.md
with the boundary correction and D10 shapes in
metasystem/plans/follow-up-rebase-fold2c-brief.md, the goal record
metasystem/plans/goals/delegate-follow-up-cannot-merge-main.md, and the
round-1 dispositions
metasystem/plans/dispositions/follow-up-rebase-code-critique-r1.md.
The orchestrator persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/followup-rebase-build1/rounds/4/review.json
(the diff sits beside it); take reviewedTree from that record. The
orchestrator's runs on the reviewed tree are listed at the end.

# Mandate

1. The follow-up path in the dispatcher: the rebase runs only when a
   chain path moved on the trunk; the tagged stash is addressed by hash
   and tag, never by position; the fast-forward is ff-only; the apply
   counts as a conflicted success only when every untracked path in
   the stash stands in the worktree with the stash's bytes, and any
   other failure restores the worktree and leaves no stash entry
   behind; a wrapper that finds unmerged entries without rebasing
   records them and prepends the paragraph; the repeated wrapper
   reproduces the paragraph from the standing record.
2. The plan verb: a round without a readable boundary contributes no
   paths and only an undecodable return refuses; the dirty set carries
   every uncommitted change; unmergedPaths; its tests.
3. The record builder admits exactly the three rebase-field shapes and
   rejects every other combination.
4. The dispatch fixture scenarios (overlap warning, conflicted
   fast-forward, untracked collision refusal, refuse-then-retry, the
   repeated rebased wrapper) exist and would fail against the old
   code; no test weakened; the two .orig deletions touch nothing else;
   nothing outside the declared boundary.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema with
the reviewedTree from the persisted round-4 review record. Gap rule:
stop and report a gap; never fill it silently.

# Orchestrator runs

On the reviewed tree (e962f76a1db88ee0c1a8a07237bcd38c3264537f), from
this Mac, evidence level ran: `gofmt -l .` printed nothing; `go vet`
passed for the dispatch and command packages; `bash -n
scripts/agents/dispatch.sh` passed; `go test ./internal/dispatch/
-count=1` passed. The implementer ran the required full gate green in
its sandbox: `scripts/agents/go-gate.sh --fast`, the whole dispatch
fixture bed including the untracked-collision refusal, the repeated
rebased follow-up, the conflict rediscovery after an authority refusal
and the no-overlap warning, and the goal command fixtures.
