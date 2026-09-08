Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Fold brief: round 4 of chain bolnext-build1

The second read (job bolnext-crit2) returned two material findings, one
high and one medium, and three notes. Its register landed on trunk after
this worktree was cut, so everything is restated below.

Read this first, because it changes how you should treat the previous
brief: both material findings trace to that brief being followed
narrowly rather than to a mistake of yours. It told you to call the
shared claim-admission owner and never said what to do with the errors
that owner returns. It told you to bring one sentence back to the page's
scope and you achieved that by deleting a different, standing one. So
where this brief is ambiguous, ask what the sentence is protecting
rather than what it literally permits, and report a gap if the answer is
not in the page.

## D1 - BOO-01, high - an unanswerable question is not an empty backlog

The frontier calls the admission gate and drops every goal whose call
returns an error, into none of the four categories, with no error
escaping the computation. Two different things are being conflated and
both need separating.

**A goal-specific refusal.** A goal whose approved budget exceeds its
tier norm is currently invisible: absent from ready, blocked and
awaiting, while still listed at its rank by `goal list`. A human sees a
full backlog and a machine that says there is nothing to do, and nothing
anywhere names the norm refusal. Put such a goal where the design's own
category definitions place work that cannot be taken right now, and
carry the reason so a reader can see it. If the page does not settle
which category, say so in the return and do not invent a new one.

**An indeterminate failure.** The gate reads repository configuration on
every call and errors on a retired key, a malformed tier-budget value, a
budget key in the process environment outside a fixture-authorised root,
or a budget override in a `.local` sibling. None of those say anything
about any goal, yet the error is attributed to each goal in turn, so one
configuration typo empties every seat's backlog simultaneously. Worse,
idle-backlog enforcement resets its refusal counter and returns without
blocking whenever the claimable list is empty, so the same typo switches
off the IDLE WITH BACKLOG safety refusal fleet-wide with no diagnostic.

That must not be possible. An error that is not about a goal has to
leave the computation as an error, so the caller learns the question
could not be answered. `goal next` must say that rather than "no
claimable goal". The status report must say it rather than "none
claimable". And idle enforcement must keep refusing, because "I could
not tell" is not "there is nothing to do".

**Canaries, and they must fail for the right reason.** One where the
configuration read fails and the frontier surfaces an error rather than
an empty list, with idle enforcement still refusing. One where an
over-norm goal is visible with its stated reason instead of vanishing.
Each must fail if the old behaviour returns, not merely if something
goes wrong.

## D2 - BOO-04, folded here because it is the same repair

The readiness check reads the configuration file once per approved goal,
in fact several times, including a retired-key scan across the file and
its `.local` sibling. While you are making that call answer once and
correctly, make it read once per frontier computation rather than once
per goal. Do not restructure anything else for performance.

## D3 - BOO-02, medium - the contract and the documents must agree

AGENTS.md previously bound every runtime to a turn-end read of `goal
next`. The design asked only that seat guidance require the
machine-scoped fetching read when the seat is free; it did not repeal
the turn-end read, and it explicitly contemplates that read returning a
held result, which presupposes a seat with a claim still performs it.
The new sentence scopes the read to a free seat while keeping the clause
calling it the universal transport every runtime has, which only makes
sense for a read that always happens.

Meanwhile docs/design/turn-verdict-delivery-contract.md still says
AGENTS.md instructs every main to read it at turn end, under the section
certifying the hook-less delivery path, and wow.md still routes the goal
thread under ending a turn.

Restore the standing turn-end read and keep the machine-scoped fetching
requirement for a free seat; they are not in conflict. Then confirm in
the return that the two other documents are consistent with what
AGENTS.md now says.

## Recorded, not actioned

BOO-03: the steward's pinned list also moved to rank order and is
persisted that way, raising no extra event because it is compared as a
set. BOO-05: every canary can fail for the right reason, though the
proofs are not distributed the way the row names suggest. Leave both.

## Scope

These three and their tests. No new categories invented, no reservation
semantics, no authority widened, and nothing else in the frontier
restructured.

## Verification

Canary first, each reported individually, including the two new ones and
what they observed:

- `go test ./internal/goal -run 'TestNextPriority|TestPriority'`
- `go test ./cmd/metasystem -run 'TestGoalPriority'`
- `go test ./internal/channel -run 'TestReportPriority'`
- `go build ./...`, then `go test` once for the packages you touch.
- `bash -n scripts/agents/goal-cli-fixtures.sh`
- `scripts/agents/go-gate.sh --fast`, once, at the end.

The orchestrator runs the goal-command bed outside your sandbox. Your
return must carry your own job id.

Gap rule: stop and report a gap; never fill it silently.
