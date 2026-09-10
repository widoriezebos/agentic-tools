Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold round ten: three small corrections from the seventh critic

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round nine. The seat gate on round nine is green everywhere
(all packages, the live walk to process 1, proof-grades, the hook
suite) and the seventh critic (hp-terminal-crit7-20260909) found one
material defect and two notes; all three fold here, and nothing else.

## HPT-23 (material): the refusal register cites the wrong lines

internal/refusal/register.go cites a line for each of the eight
human-authority refusal codes; the round-nine change moved the constant
block in internal/humanauthority/authority.go down by two lines and
corrected the register by one, so every row now points one line above
its constant. Fix the eight cited lines to the constants' real lines in
your tree (the critic read them as 31, 35, 38, 34, 36, 37, 33, 32 for
agent-in-authority-chain, ancestry-unreadable, ancestry-cycle,
terminal-not-enrolled, ancestry-changed, argv-unreadable,
process-reused, terminal-not-reached in the register's own row order;
verify against your tree, do not copy).

## HPT-24 (note): a test name says the opposite of its body

In internal/goal/authority_test.go the test named for "mixed arc
cascades move only eligible members" now asserts that a cascade with a
member claimed by another pair is refused as a whole. Rename it and its
lead sentence to say that a foreign claim makes the whole cascade a
human act; nothing else in it changes.

## HPT-25 (note): journal replay of the three stopping verbs

internal/goal/recover.go rebuilds park, unpark and release (cascade
forms included) through the same builders without an authority proof,
and cmd/metasystem/goalsync_verbs.go's recover command takes no human
name, so the new per-member requirement refuses the replay with the
agent-removing-a-human's-reservation wording. That fails closed but
tells the operator the wrong thing. Do what the replay switch already
does for set-budget and set-priority: name park, unpark and release as
proof-bearing and refuse them with the same explicit wording that they
cannot be replayed from journal text and must be re-run from the human
authority boundary. One test in the recover tests for one of the three.

## Mandate

1. HPT-23, HPT-24, HPT-25 as above.
2. Nothing else changes.

## Proof

Run internal/refusal, internal/humanauthority, the goal-package tests
that mention arcs, recover, replay, journal, humans or authority, and
the command-package recover tests; gofmt, vet. Say what ran. The
orchestrator runs the full packages, the goal-cli bed and the hook
suite on the seat. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
