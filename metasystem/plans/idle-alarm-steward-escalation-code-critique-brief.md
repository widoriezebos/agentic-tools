Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal idle-with-backlog-alarm)
Date: 2026-09-06

# Review brief: the idle gate's bounded escalation (chain idle-escalate-build1)

FINDING IDS: chain-unique, IGE-01, IGE-02, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review; a further fresh critic over the final round closes the
chain (the goal record carries reviewRoundLimit 3). R-60-m1's rule: a
finding is material only if it changes what gets built and names the
artifact it would change.

Threat model: the hard turn-exit gate (a74ca7cb) weakened beyond the
bounded escalation the brief decides (a seat able to idle with backlog
without a claim, an intent and an incident all three existing); the
count resetting on an unchanged world, or failing to reset on a changed
one; the escalation claiming a goal this machine may not claim, or
claiming under the wrong actor; a minted intent the steward's runner
will not launch because the seat's main is alive; a duplicate
continuation past the one-active guard; the verdict writing state or
minting on a path that then still blocks (a block with side effects);
uncertainty (unreadable ledger, state file) turning into a release
instead of a block; the human alarm silenced when the steward path
failed; the hook's flag read breaking the payload parse for older
harnesses that omit it; a weakened or deleted test; anything outside
the declared boundary. Out of scope: the never-idle law itself; the
continuation role's conduct; taste.

Scope: the computed diff of implementer job idle-escalate-build1.
Contract: metasystem/plans/idle-alarm-steward-escalation-build-brief.md
(decisions D1 to D6 and the facts) and the goal record
metasystem/plans/goals/idle-with-backlog-alarm.md. The orchestrator
persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/idle-escalate-build1/rounds/1/review.json
(the diff sits beside it); take reviewedTree from that record. Your tool
surface is read-only; the orchestrator's runs on the reviewed tree are
listed at the end.

# Mandate

1. Trace enforceIdleBacklog end to end for stops one, two and three on
   an unchanged world, then for a changed world, then for a refused
   claim, then for an unreadable state file: name the outcome, the
   state written and the side effects of each.
2. Confirm the steward's launch predicate accepts a seatIdle intent
   with a live main, and that the one-active-continuation guard still
   refuses a second.
3. Confirm the human alarm path is untouched and still fires when the
   steward path could not act.
4. The hook reads stop_hook_active tolerantly and passes it through;
   the verdict verb accepts the flag; old payloads without it behave as
   before.
5. The pins of D6 exist and would fail against the old code.
6. Nothing outside the declared boundary changed; no test weakened.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 45 minutes. Return per the code-critic schema with
the reviewedTree from the persisted review record. Gap rule: stop and
report a gap; never fill it silently.

# Orchestrator runs

On the reviewed tree (21a1a0a6493d3966b83181178f69690214380604), from
this Mac, evidence level ran: `gofmt -l .` printed nothing; `go vet
./...` passed; `bash -n scripts/agents/supervision-hook.sh` passed; `go
test` passed for the packages goal (515 seconds), steward (168 seconds),
report, lease and the whole command package; `bash
scripts/agents/supervision-hook-fixtures.sh` passed every scenario,
including the new bounded template backlog escalation scenario. The
implementer's own gate also ran the goal and steward packages green;
its sandbox could not finish the hook bed.
