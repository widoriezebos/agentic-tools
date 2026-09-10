# The account-provenance wedge now holds m1d's claim slot, and nothing opens it

Recorded by m1d, 2026-09-09, as the continuation of
`records/misc/account-provenance-resume-is-wedged.md` (m1b, 2026-09-08).
Written here for the same reason that record was: editing the goal is a
human act, and the goal's own next step still carries the 2026-09-07 ask
for a budget raise that the wedge record proves is refused. Anyone who
reads the goal record before this note is sent at a locked door; the stop
hook did exactly that on this seat, seven refusals in a row.

## What changed since the m1b record

Wido took the goal to tier 3 with risk 2/3/2/2 (approved 2026-09-08, revision
21). That is the first half of the old ask done. It did not help, because the
block was never the budget; it is the fence. He then opened
`account-provenance-carried` (2026-09-09 05:59Z) to carry the work, and put
`account-provenance` itself at the bottom of priority 3. His word this
morning: do not revive it, start the new one.

## The seat is wedged shut

m1d cannot claim anything. `goal claim landing-receipt-survives-records-drift`
(the head of the queue in Wido's order) refuses: *machine m1d claims
account-provenance, landing-receipt-survives-records-drift: the quota is one
claim per machine*. The claim on account-provenance belongs to
`m1d+main-1788764558-63534-a15b0d`, whose pid 63534 is dead (`proc classify`
says so). A dead session's claim counts against the live machine's one slot.

## A sixth locked door

The m1b record lists five: resume, set-budget, breach-stop at the current
revision, steal, recover. `goal release` is a sixth. Every verb that clears a
claim — release, park, done, unapprove, steal — runs through
`clearClaimBinding` (`internal/goal/verbs.go:276`), which refuses while
`StopFence` is set: *only goal resume may clear its launch fence*. A human
`--by` does not bypass it; the human check in `releaseRequest` sits before the
call, the fence check inside it. So no verb, human or agent, frees the slot.

## The sovereign route is closed too

Probed on a dangling commit, no branch touched: delete only the `- StopFence:`
line from `plans/goals/account-provenance.md` and run the tree through
`ValidateCommit`. Refused: *Integrity mismatch: recorded f805eff8…, computed
a3418898…*. The goal file carries a seal, so a hand edit would have to forge
it. Not proposed.

## Delegates cannot route around it

Dispatch binds a job to this machine's claim (`internal/dispatch/stop.go`,
`servinggoal.go` read `file.Claimed`). An approved-but-unclaimed goal has no
binding, so the seat cannot work `breach-stop-wedges-seat` through delegates
without first holding it, and it cannot hold it for the reason above.

## The steward leaks intents at the wedge

Each stop-hook refusal on this seat selects account-provenance as the held
goal and mints a steward continuation intent for it. Four now sit in
`artifacts/agents/steward/intents/`: 1f0a5fd7ca962a31, 554ffe2e728e5813,
a8bcb766c7acaa4e, d625e34b74f10868, all unnotified and undispatched, all
carrying `fenceAtMint=19`, the stale revision. None can ever run. There is
no verb to cancel one. This is a facet of `steward-revives-a-done-goal`
(1/15) and belongs on whichever goal fixes the steward's choice of a
continuation target: a fenced goal is not a continuation target.

## The way out, and who can take it

`breach-stop-wedges-seat` (1/8, approved, appetite 1h, budget 4h/6/240) is
this defect by name; its DONE is that a breach-stopped goal does not hold the
quota slot, with the fence on *resuming* preserved. The quota rule is
`internal/goal/validate.go:298-330`; the narrowest change is that a claim
under a StopFence does not count. It has to be built from a machine that can
claim it, which m1d is not. Wido's word 2026-09-09: build through delegates
(Sol implements, with the fixture the goal names; Fable critiques). When it
lands, m1d pulls, rebuilds, and claims the head of the queue.

Until then this seat has no lawful work, and a seventh refusal changes
nothing. Spend no further commands on account-provenance itself.
