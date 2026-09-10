Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-10

# Fold round fourteen: typed outcomes pass through the recovery wrapper

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round thirteen. The eleventh critic (hp-terminal-crit11-20260909)
found two material things in round thirteen's wrapper; both fold here,
nothing else.

## HPT-34 (medium): the wrapper erases the verb's typed outcomes

In internal/goal/recover.go the wrapper reads: run the mutation; if the
typed human-authority error, refuse; if the entry is named, refuse with
the cannot-replay sentence; else return. The second branch fires
whatever the mutation returned. When the dead owner's push lands while
recovery is rebuilding a human-run park, unpark or release, the verb
reports the "already applied" classification that internal/goal/txn.go
documents as the outcome the transaction layer closes as confirmed; the
wrapper replaces it with the refusal, so the journal says rejected
while the goal is in fact parked, unparked or released on the canonical
branch. "Lost to a competitor" and "nothing to do" are erased the same
way. Fix: let every typed outcome the mutation returns pass through
unchanged; refuse a named entry only where the mutation would otherwise
have succeeded (it returned changes and no error). Tests: a named park
whose act already landed closes confirmed as already applied and
writes nothing new; a named release lost to a competitor keeps that
outcome; the refusal still fires for a named entry that would have
succeeded.

## HPT-35 (low): the comment above intentArgs

internal/goal/verbs.go's comment above intentArgs still says recovery
reconstructs authority from the stored args. Rewrite it to the
doctrine round thirteen restored: the stored name records who intended
the act; recovery refuses to replay a named entry and never carries the
name into an actor.

## Mandate

1. HPT-34 and HPT-35 as above.
2. Nothing else changes.

## Proof

internal/goal recover and transaction tests; gofmt, vet. Report the
round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
