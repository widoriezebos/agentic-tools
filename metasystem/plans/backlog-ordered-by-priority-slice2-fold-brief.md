Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Fold brief: round 2 of chain bolnext-build1

The closing read (job bolnext-crit1) returned two material findings, both
high, and three notes. Its register landed on trunk after this worktree
was cut, so everything is restated below. The specification is unchanged:
metasystem/plans/backlog-ordered-by-priority-design.md, revision 2,
sections 4 and 5 and the seat projection paragraphs.

Round 1 is good work. Fold these four items and change nothing else.

## D1 - BON-02, high - `next` must never return what `claim` will refuse

This is the one that matters, and it is proven by execution. The
frontier admits a goal after checking labels, approval expiry,
dependencies and pins. The claim verb applies three further tests the
frontier does not: the goal must carry a budget, its approval record
must be structurally valid, and its budget must fit the machine's tier
box or carry a recorded norm approval.

A goal that passes the first set and fails the second sits at the head
of the rank and is handed to every free machine, forever. The critic
built exactly that: an approved goal ranked first reserving 2400 job
minutes against a tier-three box of 1200. `Next` returned it and
selected it; the claim admission gate refused the same goal with
GOAL_NORM_REFUSED, and accepted the goal ranked behind it.

Because slice 2 makes this verb the sanctioned way to take work, and
the new seat guidance tells a seat to claim only the returned goal and
to reselect rather than reach for a remembered fallback, the seat cannot
lawfully step past the blockage. The rank is global, so every free
machine in the fleet converges on the same unclaimable head while
claimable work waits behind it.

**Do not re-implement the claim gate's tests inside the frontier.** Call
the same predicate, so one owner decides eligibility and the two cannot
drift apart again. This program has already paid twice for two spellings
of one rule.

If making them share an owner requires a decision the design page does
not settle, stop and report it as a gap rather than choosing. The page
names the frontier's checks and is silent on the claim gate's three.

Prove it with a canary that fails for the right reason: an over-norm
goal ranked ahead of a claimable one, where `next` must return the
claimable one, and where the assertion fails if the over-norm goal is
returned. Not merely "returns something".

## D2 - BON-01, high - the empty answers stay distinct, and the bed stays strict

`scripts/agents/goal-cli-fixtures.sh` step 10 requires the output of the
label-filtered empty case to be exactly `no goal matches --label
absent`. Round 1 prints `no claimable goal for machine <nick>; no
matching eligible work for --label absent`, which fails the bed and
conflates two different empty conditions in one sentence.

Restore a distinct sentence per condition. The label-filtered empty case
keeps the exact sentence the bed pins. If the machine-scoped empty case
needs its own sentence, give it a different one and add a bed case that
pins it too.

Do not weaken or delete the bed's assertion to make the new text pass.
The orchestrator ran the bed and reproduced this independently, so it is
a real failure of the gate this design names, not a stale expectation.

## D3 - BON-03 - the seat guidance says what the page says

The page requires the fetching form when the seat is free. The new
guidance requires it at turn end and whenever the seat is free, and
treats a fetch failure as a hard error. Bring the sentence back to the
page's scope.

## D4 - BON-05 - remove the map nothing reads

Removing pin promotion from the idle diagnostic left the pinned-goal map
on the claimable-work record built but with no production reader. Delete
it, or state in the return why it must stay.

## Not in this round

BON-04 is recorded and not actioned: re-ranking now raises a steward
ledger-attention event because the waiting queue is ordered by rank and
compared position by position. The event is true, and suppressing a
truthful change notice to keep a diagnostic quiet is the wrong trade.
Leave it.

The metrics defect from the slice-1 read stays out of scope, as before.

## Scope

These four and their tests. No change to what a claim or an approval
means, no reservation semantics on `next`, no new authority.

## Verification

Canary first, each reported individually:

- The new over-norm canary from D1, including what it observed.
- The priority and next canaries: `go test ./internal/goal -run
  'TestNextPriority|TestPriority'`, `go test ./cmd/metasystem -run
  'TestGoalPriority'`, `go test ./internal/channel -run
  'TestReportPriority'`.
- `go build ./...`
- `go test` once for the packages you touch.
- `bash -n scripts/agents/goal-cli-fixtures.sh`
- `scripts/agents/go-gate.sh --fast`, once, at the end.

The orchestrator runs the goal-command bed outside your sandbox, and
that bed is the proof D2 is fixed. Do not attempt it. Two sandbox limits
are known and not yours: the command-package process-identity probe
cannot observe its own process group, and the steward package shells out
to a build that needs network.

Gap rule: stop and report a gap; never fill it silently.
