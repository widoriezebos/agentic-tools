Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-10

# Bounded policy correction, contract only: goal records get an owning test surface

The previous chain for this correction (dispatch-cap-surface2-20260910,
reviewed clean) cannot land: it added a selection test under
internal/testpolicy, and any candidate that changes a Go file trips the
receipt's engine/script skew preflight inside section/gate-fence-fixtures
("engine commit is older than checkout commit and engine or agent
scripts changed") because the isolated worktree runs the enrolled
engine. A contract-only change lands (c0d5b136 did). So this chain
carries the same surface entry and nothing else.

## Mandate

1. In metasystem/testing.json add the surface `goal-records` exactly
   as the previous chain did: paths `metasystem/plans/**`,
   `metasystem/records/**`, `metasystem/memory/rulings.md`,
   `metasystem/memory/receipts.log`; dependsOn `testing-policy`;
   standard groups `section/static-contract-audits` and
   `section/return-schema-fixtures`; deep and critical empty. Place it
   beside the `instructions` surface.
2. No other file changes. The selection test waits for a later chain.

## Proof

The contract's check verb and `go test ./internal/testpolicy/` (the
existing tests must still pass with the new surface); `git diff --stat`
showing only metasystem/testing.json. Report the round as your own.

## Constraints

Wall-clock budget: 10 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
