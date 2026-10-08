# Design brief: engine inputs decide re-arm and session start (for goal 7, finding 17)

Dropped from lane-drain-and-fresh-claims (its U3) on 2026-10-07 under the stop rule: three reads found 1, 2, 2 material and the last fix introduced a defect. The built attempt is on branch work/ldfc-u3 (119b72998 U3, then U3-fixes) as evidence; do not merge it.

Intent (unchanged, from lane-drain-and-fresh-claims Decision 3): re-arm, enrolled-source verification and dispatch preflight compare ENGINE inputs (internal/behaviorsurface/policy.v2.json) instead of whole commits or path lists, so ledger, records and settings commits never defer session start or force a rebuild.

What the reads proved the design must decide first (each with its evidence):
1. Which commit is "landed" for every comparison: the merge-base of HEAD and the owned landing ref, in session start AND in the rebuilt-engine re-arm (runner.go:743-768); never name `devgate build` while HEAD differs from the landed base only by unlanded commits (devgate stamps HEAD, cmd/devgate/build.go:163-199, which then fails ownership: rearm_resolver.go:505-507 -> ErrEnrollmentDrift -> a person act, up.go:746-763).
2. A checkout behind its engine's source: equal inputs reuse, different inputs defer; and the minted enrollment must record the landed source, not the older HEAD (rearm_resolver.go:728-731), or every later verification fails ancestry (verifyEnrollmentBuildSourceWithDeps).
3. Dispatch preflight keeps fail-open where a rebuild cannot help (internal/delegation/infra.go:57-64).
4. One owner for "landed base" and one for "equal inputs", with a test matrix over: equal/different inputs x HEAD behind/equal/ahead/diverged of the landing ref x witness/commit stamp; each cell names the outcome and the remedy, and the remedy followed must succeed without a person.
The five design questions apply. Run the design critique to convergence before any build.

## Found 10-07: findings-store registration under parallel tests

Under `go test -parallel` the cmd tests that use the unit read's findings store (TestIntentReadVerdictFromRetainedFindings, TestIntentGeneratedUnitPlan, TestIntentBuildRoundLimitAndReadBudget, TestIntentBuildRetainedRequest, TestIntentBuildConcurrentRepeat) fail with "release the read's findings store ...: no store record of this owner names the path" or "is not a registered store of the unit" (intent_work_test.go:664, :859); each passes alone; reproduced on goal/person-claims base 28601997d with -count=5 -parallel 64 (Opus read, 10-07). Likely a shared store registry and TMPDIR swept by a real steward serving in the test registry home. Fix the isolation (per-test registry/TMPDIR or the sweep honouring live owners), with a test that runs the pair in parallel -count=5.
