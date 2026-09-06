Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal delegate-follow-up-cannot-merge-main)
Date: 2026-09-06

# Review brief: the follow-up rebase in the dispatcher (chain followup-rebase-build1)

FINDING IDS: chain-unique, FRB-01, FRB-02, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review (reviewRoundLimit 2), then a fresh critic closes. R-60-m1's
rule: material only if it changes what gets built and names the
artifact.

Threat model: the dispatcher losing a round's work (a stash entry left
behind, dropped before a failed apply, or applied onto the wrong base);
a stash from another session popped or applied (the stack is shared
across the checkout and its worktrees: entries must be addressed by
hash and tag, never by position); a rebase performed when no chain
path moved, or skipped when one did (the union of round boundaries plus
dirty paths is the rule); conflict markers not recorded on the record
or not named in the delivered brief; a fast-forward attempted on a
worktree whose HEAD is not an ancestor of the trunk; the refusal path
leaving the worktree at the wrong HEAD; the plan verb reading rounds
that do not exist; the brief authority check running before the
rebase; the two .orig deletions touching anything else. Out of scope:
checkpoint commits on agent branches (decided out); taste.

Scope: the computed diff of implementer job followup-rebase-build1.
Contract: metasystem/plans/follow-up-rebase-build-brief.md (D1 to D6)
and the goal record
metasystem/plans/goals/delegate-follow-up-cannot-merge-main.md. The
orchestrator persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/followup-rebase-build1/rounds/1/review.json
with the diff beside it; take reviewedTree from that record. The
orchestrator's runs on the reviewed tree are listed at the end.

# Mandate

1. Trace the follow-up path from the chain-closed check to the brief
   authority check: when the rebase runs, what it does step by step,
   and every failure branch's end state of the worktree and the stash
   stack.
2. The plan verb: the path union, the behind count, the overlap, and
   its refusals; its tests.
3. The record fields and the delivered brief paragraph on the
   conflicted path.
4. The dispatch fixture scenarios exist and would fail against the old
   code; no test weakened; nothing outside the boundary.

If nothing material remains, say so; a fresh critic then closes the
chain.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema with
the reviewedTree from the persisted review record. Gap rule: stop and
report a gap; never fill it silently.

# Orchestrator runs

On the reviewed tree (9bbf59a01623ec588a67460db043e7c79de36061), from
this Mac, evidence level ran: `gofmt -l .` printed nothing; `go vet`
passed for the dispatch and command packages; `bash -n
scripts/agents/dispatch.sh` passed; `go test ./internal/dispatch/
-count=1` passed. The implementer ran the whole dispatch fixture bed,
the mission-runner fixtures and the adapter self-tests green in its
sandbox, including the two new scenarios.
