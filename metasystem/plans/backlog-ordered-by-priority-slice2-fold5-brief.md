Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Fold brief: round 6 of chain bolnext-build1, one line of noise

The third read (job bolnext-crit3) returned one material finding, medium,
and three notes. Its register landed on trunk after this worktree was
cut, so it is restated here. Everything else in round 5 is confirmed
good, verified by the orchestrator outside the sandbox: the goal-command
bed exits 0, the three canaries pass, the fast gate exits 0, and the
goal, channel, steward and command packages all pass in full.

Fold exactly one thing.

## D1 - BOP2-01 - a quiet machine must stay quiet

The status report folds the projection's staleness notice into its new
backlog line, and that notice carries the accepted tree's age rounded to
the minute. The channel decides whether to post by hashing the report
with only the status header removed, so the age is inside the hash. The
notice appears whenever the accepted tip's commit is more than thirty
minutes old, which is the ordinary state of a repository between ledger
commits, and the age grows with the clock. So the hash changes at every
cadence tick and a machine posts a fresh status every interval even when
the backlog head, its own candidate, the questions, the deliveries and
the undelivered count are all unchanged.

Before this chain the report carried no projection notices, so a quiet
machine posted once and then stayed silent. Section 5 of the page rules
the new behaviour out in its own words: the existing digest and post
decision already notice changed content at the normal cadence, and there
is no extra immediate post or new channel event.

The critic named two repairs and either is acceptable:

- fold a fixed staleness phrase carrying no varying number; or
- hash the report with the reserved notice removed, the way the header
  already is.

Choose one, say which and why in the return, and keep the notice visible
to a human reader either way. The point is that the post decision stops
seeing a change that is only the clock.

**Canary, and it must fail for the right reason.** Compose the report
twice with the same content and two different tree ages, and assert the
post decision does not fire the second time. It must fail if the age
re-enters the hash. Watch it fail before you make it pass, by leaving
the age in, and say in the return that you did.

## Not in this round

Three notes stay recorded and untouched. A malformed tier travels on the
could-not-answer channel although it is a fact about one goal; the
steward's pinned list moved from alphabetical to backlog order because
it shares a loop with the waiting queue; and re-ranking raises a
waiting-queue attention event, which is the intended consequence of the
diagnostics following the order. Do not widen the error separation and
do not re-sort the pinned list. Unasked changes have cost this chain
three rounds already.

## Verification

- The new canary, individually, and the statement that you watched it
  fail first.
- `go test ./internal/channel -run 'TestReportPriority'`
- `go test ./internal/goal -run 'TestNextPriority|TestPriority'`
- `go build ./...`, then `go test` once for the packages you touch.
- `scripts/agents/go-gate.sh --fast`, once, at the end.

The orchestrator runs the goal-command bed. Your return must carry its
own job id.

Gap rule: stop and report a gap; never fill it silently.
