Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-10

# One-line policy correction: the obligation-state package gets an owner

Goal dispatch-cap-necessity's reviewed chain cannot land: the testing
contract metasystem/testing.json declares no surface for
metasystem/internal/obligationstate/**, so `landing test-receipt --mode
auto` on the candidate answers "delivery impact is unresolved: no
surface owns changed path metasystem/internal/obligationstate/state_test.go".
The design's contract (metasystem/plans/application-testing-contract-design.md,
rule 6) names the remedy: a bounded policy correction.

## Mandate

1. In metasystem/testing.json, add `metasystem/internal/obligationstate/**`
   to the `dispatch-goal-mission` surface's paths, and add a unit group
   `obligationstate-standard` (kind unit, adapter go, cwd metasystem,
   packages ["internal/obligationstate"], the same inputs, tools and
   obligations shape as `goal-decision-standard`) to that surface's
   groups. Nothing else in the contract changes.
2. Prove it with the contract's own checks: `go run ./cmd/metasystem
   test check --root .` (or the verb that validates the contract; the
   existing testpolicy tests) and `go test ./internal/testpolicy/`.
3. Nothing else changes: one file, the smallest diff that satisfies 1.

## Proof

The commands in 2, gofmt not needed (JSON), and `git diff --stat`
showing exactly metasystem/testing.json. Report the round as your own.

## Constraints

Wall-clock budget: 10 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
