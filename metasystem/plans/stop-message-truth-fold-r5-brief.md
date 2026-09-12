Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-message-truth)
Date: 2026-09-10

# Fold round five: rebase onto today's main, nothing else

Follow-up round on chain stop-truth-build1-20260910. Round four was
reviewed clean (stop-truth-crit4-20260910, one note). Its diff no longer
applies to main: the landing 2f764c609 of 2026-09-10 (a breach-stopped
claim no longer wedges its machine) changed internal/goal/turnverdict.go
around the idle branch and the current-goal selection, the same region
this chain changes (the in-flight suppressors, the CLAIM HELD line, the
restored idle gate).

## Mandate

1. Rebase the chain's worktree onto main at its current tip (fetch
   origin; the tip at dispatch is 3e124c3e7 or later). Resolve the
   conflict in internal/goal/turnverdict.go keeping both sides: main's
   fenced-claim rule (a claim under a standing StopFence is not this
   machine's live work, IsFencedClaim) and this chain's rules (a running
   job, a live proof attempt of this checkout or a held landing lock is
   in flight; the CLAIM HELD line only when the scan's busy list is
   empty; the pre-chain idle gate restored). Where main's change and
   this chain's meet in one function, main's fence check comes first and
   this chain's in-flight check second; a fenced claim is neither in
   flight nor held. If the tests of either side conflict, keep both
   tests. Name every conflicting file in your return.
2. Change nothing else: no new rule, no new test beyond what the merge
   needs, no contract change. The resulting diff against main must be
   the round-four change re-based.
3. Prove: go build, vet, gofmt; go test on internal/goal,
   internal/report, internal/run, internal/dispatch, cmd/metasystem (the
   two cmd tests that fail on main today are not this chain's).

## Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema and
report the round as your own. Stop at a gap that needs a decision no
page has made; report it with the resolution you propose. Never delete
written work.
