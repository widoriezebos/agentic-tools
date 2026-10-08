# Brief: review-chain-stops-and-records build-outcomes-a, correction 1

Working Mode: Implement
build-outcomes-a is uncommitted in this worktree. One Opus read found three material defects; fix exactly these. A probe showed freezeBuildOutcome measuring the whole worktree against the unit's original base: on a correction round it counted 131 lines against 5 declared, including another unit's files and six records/reads/*.json files.

1. internal/launch/unit_build_outcome.go:18-40: the gap test (`len(TrimSpace(diff)) == 0`) runs on the whole-unit diff, so from round 2 a builder that changes nothing is never a gap and proof and read launch. Retain the tree right before each build and decide the gap on this round's own builder change. Test: a correction round that leaves the tree unchanged is a gap and launches nothing after the build (mutation: the whole-unit diff, red).
2. unit_build_outcome.go:28-41: the size counts other units' commits and records/ against this unit's estimate. Count only this unit's change (this round's builder change for the hold; never other units' commits, never records/). Restore cmd/metasystem/intent_connected_journey_test.go:321 and intent_unit_review_test.go:601 to expect the correction to proceed (they were changed to expect a size hold). Mutation: count the whole worktree, red on both restored tests.
3. internal/launch/unit_stop.go:269-271 allowCorrection: the build-gap early return sits before the ReviewPolicy read, so under review.stop=person or an unreadable policy an agent's plain `work revise ... --brief FILE` starts a build. Move the gap exemption below the policy check; under person the gap's printed act is the `--reason TEXT --by NAME` person form; unreadable policy never becomes auto. Tests: ReviewPolicy returning person, and returning an error, with an agent's gap revise refused and the person form printed (mutation: exemption before the policy, red).
Also: write the size impact record only after AcceptBuildSize succeeds (a refused acceptance records nothing).
Size: the unit stays at most 250 production lines in total.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then the broad cmd selection for the area you touched (at least `go test -count=1 -timeout 60m -run 'TestLanding|TestWork|TestIntent|TestGoal|TestReview|TestUnit|TestClaim|TestSession|TestHelm|TestPolicy|TestQuestion|TestKeeper|TestBuild|TestHolder|TestProof|TestDesign|TestDispatch|TestEvery|TestAudit|TestInstruction' ./cmd/metasystem/`: every test there must pass, not only yours; a red you did not cause still blocks done), then `go run ./cmd/devgate static`. Report done only when all of these exit 0. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
