Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold round eleven: journal replay refuses only the acts that need a human

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round ten. The eighth critic (hp-terminal-crit8-20260909) found
one material defect in round ten's replay change; it folds here, and
nothing else.

## HPT-26 (high): the replay refusal over-reaches

The live verbs ask for a human proof only under conditions
(internal/goal/verbs.go: release when the goal is claimed by a
different machine-and-lineage pair; park when the goal's origin is the
human origin or its claim belongs to another pair; unpark when the park
was recorded by a human). Round ten added one switch case to
internal/goal/recover.go that refuses EVERY stored park, unpark and
release without consulting those conditions and closes the entry
rejected with the re-run-at-the-human-boundary sentence. The commonest
recovery, a machine that died part-way through releasing its own
claim, now closes rejected and the goal stays claimed by the dead pair
until a human's foreign release or a steal. Before round ten, replay
rebuilt the act through the real verb, which refused only when a human
was genuinely required. Round ten's own test strands a park of a
main-origin, never-claimed goal and expects it rejected, which is the
over-reach in a test.

Fix: replay rebuilds park, unpark and release through the real verb
again, exactly as before round ten. Only when that verb's own
human-authority requirement fires (the requireHuman row asks for a
grade and the replayed request carries no authority) does replay
refuse, and then with the explicit wording that the act cannot be
replayed from journal text and must be re-run from the human authority
boundary, closing the entry rejected. An act the verb admits without a
human proof replays and heals as it always did. Tests: the recover
tests prove (a) a stranded release of the dead pair's OWN claim
replays and frees the goal; (b) a stranded park of a main-origin,
never-claimed goal replays; (c) a stranded foreign release, a park of a
human-origin goal and an unpark of a human's park are each refused
with the explicit wording and change nothing.

## HPT-27 (note, no change)

Nothing checks the refusal register's cited lines, so they will drift
again when the constants block moves. Recorded; not this round.

## Mandate

1. HPT-26 as above.
2. Nothing else changes.

## Proof

internal/goal recover and authority tests, internal/humanauthority;
gofmt, vet. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
