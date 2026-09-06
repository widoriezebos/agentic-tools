Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal human-goal-verbs-forgiving)
Date: 2026-09-06

# Review brief: one verb gives a goal a box (chain hgvf-build1-20260906)

FINDING IDS: chain-unique, HGV-01, HGV-02, ... never F-n.

Round budget: this is the one independent critique the goal's tier
allows on the build, and the goal's only critique of the design as
well: the engine refuses a design critic on a tier 2 goal, so the
design-critique mandate in
metasystem/plans/human-goal-verbs-forgiving-critique-brief.md is
carried here and read against the code. One focused round, then at
most one correction and its re-review (the goal's review-round limit
is 2). Material only if it changes what gets built and names the
artifact.

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/human-goal-verbs-forgiving-design.md
(revision 1, the design), metasystem/plans/human-goal-verbs-forgiving-build-brief.md
(the build brief with the orchestrator's four rulings) and the goal
record metasystem/plans/goals/human-goal-verbs-forgiving.md.

Threat model: authority widening (a seat, a fixture, a temporary word
or a channel relay recording an over-norm box or a human name without
a human at the enrolled terminal); a routing that lands the wrong
ledger transaction for a state (a parked goal unparked by a budget, a
breach-stopped goal resumed by a new box, a claim rebound without a
new revision); a printed command that does not succeed when run; a
record line whose format changed; the enrollment record's new field
breaking an engine or a record already on disk; the fixture suite
weakened or a new scenario that passes without proving its claim.
Out: taste; the working-duration grammar's honesty (goal
breach-clock-and-budget-honesty).

# Mandate

1. Authority. Read the over-norm branch in
   metasystem/internal/goal/norm.go and the proof classes in
   metasystem/internal/humanauthority/authority.go as changed: is the
   branch reachable only by a real enrolled-terminal proof, and does a
   test prove the exclusion of fixture-only, temporary-word and
   channel proofs? Does the `--by` default read only the enrollment
   record the proof walked to, and never git config or an argument?
2. Routing. For every goal state the design's section 2 lists, and for
   abandoned, interrupted and a claim held by another machine: does the
   verb land exactly the transaction the table says, with the history
   verb and record lines unchanged in format? Does the approve
   transaction admitting a parked goal move nothing else?
3. Grammar. The compact box parser shared with the config
   (metasystem/internal/goalbudget/budget.go and
   metasystem/internal/config/budget.go): does every tier key in the
   config still parse to the same tuple, and does the positional token
   never swallow a goal id or a flag value?
4. The refusal function. For each row of the design's refusal table
   that the diff implements: is the printed command one that succeeds
   with the values seen? Name each row where it cannot, and any
   human-only refusal in metasystem/cmd/metasystem/goalsync_mutations.go
   that still bypasses the function.
5. Fixtures. In metasystem/scripts/agents/goal-cli-fixtures.sh: does
   each new scenario run the printed command verbatim and compare the
   resulting history and Budget lines with a long-form twin, and do the
   package tests cover what a headless bed cannot drive (the terminal
   over-norm fold)?
6. Nothing outside the design's section 7 changed.

If nothing material remains, say so; that closes the chain and the
build lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
hgvf-build1-20260906. Gap rule: stop and report a gap; never fill it
silently.
