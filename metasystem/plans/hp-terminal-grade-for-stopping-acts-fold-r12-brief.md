Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold round twelve: a replayed human act keeps the human's name

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round eleven. The ninth critic (hp-terminal-crit9-20260909)
found one material defect and two notes; all three fold here, and
nothing else. This round also rebases the chain onto today's main (the
dispatcher does it); re-check the eight citation lines in
internal/refusal/register.go afterwards, because main gained rows in
that file.

## HPT-28 (high): recovery replays a human's park as a machine park

The live verb stamps the human's name into the journaled intent
(intentArgs in internal/goal/verbs.go sets the "by" key whenever the
actor carries a human name), but recovery rebuilds the actor from the
entry's machine and lineage only (actorFromEntry in
internal/goal/recover.go), so a replayed park writes Parked.By as
"<machine>+<lineage>" instead of "human:<name>". Both unpark gates this
chain added test that field for the "human:" prefix, so after such a
replay any agent can unpark the goal with no proof. Round eleven's
wrapper does not catch it because a main-origin, unclaimed goal's park
raises no human-proof requirement.

Fix: recovery rebuilds the actor exactly as journaled: when the entry's
intent carries the "by" name, the replayed actor carries that human
name, so the replayed record is byte-for-byte what the live act would
have written (Parked.By "human:<name>", the history actor likewise).
The journal is the ledger's own write-ahead record, written after the
live act was admitted, so completing it restores the admitted truth;
it grants nothing new. Test: a stranded human park of a main-origin,
unclaimed goal replays with Parked.By "human:<name>" and an agent's
unpark of it is then refused exactly as for a live human park.

## HPT-29 (note): the refusal names the real condition

Round eleven's recovery wrapper replaces the typed error's own detail
with one fixed sentence keyed on the verb name, so an agent's own
stranded park of a human-origin goal is told to re-run at the human
boundary, which is addressed to nobody. Keep the explicit
cannot-replay sentence and append the typed error's own condition (the
row name and grade it carries), so the closed entry says which
condition fired.

## HPT-30 (note): the chain is behind main

main gained rows and exclusions in internal/refusal/register.go after
this chain's base. The dispatcher rebases your worktree for this round;
after it, verify the eight human-authority citation lines against the
constants' real lines in internal/humanauthority/authority.go and
correct any that moved. Nothing else from the rebase is yours.

## Mandate

1. HPT-28, HPT-29, HPT-30 as above.
2. Nothing else changes.

## Proof

internal/goal recover, park and unpark tests; internal/refusal;
internal/humanauthority; gofmt, vet. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
