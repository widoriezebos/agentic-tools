Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Fold brief: round 5 of chain bolnext-build1, the half that needs no ruling

Round 4 stopped and reported a gap rather than inventing, which was
correct and is why this brief exists. Its gap: the accepted design does
not say where an over-norm goal belongs. It is not Blocked, having no
open dependency, and not Awaiting, being approved and unexpired, and a
fifth category is forbidden. That needs a design ruling and it is being
taken to the human separately.

The coordinator has split the finding. Only the visibility of a
goal-specific refusal depends on that ruling. The severe half does not,
and it is the half that can stop the fleet, so build it now.

## D1 - an indeterminate failure must never look like an empty backlog

This needs no category and no new data shape for goals.

The admission gate reads repository configuration on every call and
errors on a retired key, a malformed tier-budget value, a budget key in
the process environment outside a fixture-authorised root, or a budget
override in a `.local` sibling. None of those say anything about any
goal. Today each such error is attributed to the goal being tested and
the goal is silently dropped, so one configuration typo empties every
seat's backlog at once. Because idle-backlog enforcement resets its
refusal counter and returns without blocking whenever the claimable list
is empty, the same typo also switches off the IDLE WITH BACKLOG safety
refusal fleet-wide, with no diagnostic anywhere.

Required behaviour:

- An error that is not a judgement about a specific goal leaves the
  frontier computation as an error. It is not attributed to a goal and
  it does not silently shrink any list.
- `goal next` says the question could not be answered, and does not say
  there is no claimable goal.
- The status report says the same, and does not report none claimable.
- Idle-backlog enforcement keeps refusing. "I could not tell" is not
  "there is nothing to do", and the safety refusal must survive exactly
  the failure that would otherwise disable it.

Distinguishing the two kinds of error is the whole point. If the gate
does not currently let a caller tell a goal-specific refusal from an
indeterminate failure, make it able to; that is a mechanical change, not
a category decision.

## D2 - read the configuration once per computation

The readiness check reads the configuration file once per approved goal,
several times in fact, including a retired-key scan across the file and
its `.local` sibling. While you are making that call answer once and
correctly, make it read once per frontier computation. Change nothing
else for performance.

## D3 - the contract and the documents must agree

AGENTS.md previously bound every runtime to a turn-end read of `goal
next`. The design asked only that seat guidance require the
machine-scoped fetching read when the seat is free; it never repealed
the turn-end read, and it contemplates that read returning a held
result, which presupposes a seat with a claim still performs it. The
current sentence scopes the read to a free seat while keeping the clause
calling it the universal transport every runtime has, which only makes
sense for a read that always happens. Meanwhile
docs/design/turn-verdict-delivery-contract.md still says AGENTS.md
instructs every main to read it at turn end, under the section
certifying the hook-less path, and wow.md still routes the goal thread
under ending a turn.

Restore the standing turn-end read and keep the machine-scoped fetching
requirement for a free seat. They are not in conflict. Confirm in the
return that the two other documents match what AGENTS.md then says.

## Explicitly NOT in this round

The visibility of a goal that the gate refuses for its own sake, such as
an over-norm budget. That is the part your predecessor correctly refused
to guess, and it waits on a ruling. Leave those goals behaving exactly
as they do today, and say in the return that you did.

## Canaries, and they must fail for the right reason

- The configuration read fails: the frontier surfaces an error rather
  than an empty list, `goal next` reports that it could not answer, and
  idle enforcement still refuses. It must fail if the old
  swallow-and-empty behaviour returns.
- The single-read change: assert the configuration is read once for a
  frontier with several approved goals, so a regression to per-goal
  reads fails the test.

## Verification

Each canary individually, then:

- `go test ./internal/goal -run 'TestNextPriority|TestPriority'`
- `go test ./cmd/metasystem -run 'TestGoalPriority'`
- `go test ./internal/channel -run 'TestReportPriority'`
- `go build ./...`, then `go test` once for the packages you touch
- `bash -n scripts/agents/goal-cli-fixtures.sh`
- `scripts/agents/go-gate.sh --fast`, once, at the end

The orchestrator runs the goal-command bed outside your sandbox. Your
return must carry its own job id.

Gap rule: stop and report a gap; never fill it silently. Round 4 did
exactly that and was right to.
