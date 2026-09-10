Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-10

# Bounded policy correction: goal records get an owning test surface

Goal dispatch-cap-necessity's evidence trail (its briefs and
dispositions under metasystem/plans and metasystem/records/misc) cannot
land: since 1b12f534 the testing contract metasystem/testing.json owns
no metasystem/plans/**, metasystem/records/** or
metasystem/memory/rulings.md, so a register-carriage landing of goal
records answers "delivery impact is unresolved: no surface owns changed
path ..." at land.sh's test verify. The contract's own design
(metasystem/plans/application-testing-contract-design.md, rule 6) names
the remedy, a bounded policy correction, and the same correction landed
for internal/obligationstate as c0d5b136 (the record of both findings:
the note under records/misc named for goal
goal-records-owned-by-the-testing-contract, untracked until this lands).

## Mandate

1. In metasystem/testing.json add a surface `goal-records` owning
   `metasystem/plans/**`, `metasystem/records/**`,
   `metasystem/memory/rulings.md` and `metasystem/memory/receipts.log`,
   with the same groups as the `instructions` surface
   (section/static-contract-audits, section/return-schema-fixtures),
   depending on `testing-policy` as `instructions` does. Where a path
   is already owned by `instructions` (the plan pages it names,
   memory/receipts.log), keep both owners only if the validator allows
   overlap; otherwise leave those paths to `instructions` and own the
   rest.
2. Prove with the contract's own validation (the test check verb or
   the testpolicy tests) that a records-only change (for instance a new
   file under metasystem/records/misc) resolves as delivery with the
   static groups selected, and add that case to
   internal/testpolicy's selection tests if one does not exist.
3. Nothing else changes: metasystem/testing.json and, if 2 needs it,
   one test file in internal/testpolicy.

## Proof

`go test ./internal/testpolicy/`, the contract check verb, and
`git diff --stat`. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
