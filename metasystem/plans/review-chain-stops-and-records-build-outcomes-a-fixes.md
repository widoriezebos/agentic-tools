# Brief: review-chain-stops-and-records build-outcomes-a-fixes (fix unit after the unit stop)

Working Mode: Implement
build-outcomes-a is committed and stopped by the stop rule with two material findings. Fix exactly these.

1. cmd/metasystem/intent_unit_review.go:47-52 with internal/launch/unit_build_outcome.go:117-120: AcceptBuildSize saves the acceptance (SizeAcceptedBy, state running, hold cleared) before recordUnitStopOverride writes the impact; a failed impact write leaves an accepted hold with no impact and a later work wait runs the proof. Order: validate (no write), record the impact, then save the acceptance (pass the impact write into AcceptBuildSize, run after its checks and before save). A failed impact write leaves the hold untouched; a repeat after a failure records the impact once. Test: the overrides directory unwritable -> the review refuses, the run stays held, no proof launches; restore it -> the same act records one impact and resumes (mutation: save before the impact, red).
2. internal/launch/unit_build_outcome.go:108-115: the acceptance's moved-change check compares WorktreeDiff(plan.Worktree, plan.Base), including records/ and other units' files, with the frozen diff, so a read attestation landing in metasystem/records/reads/... during the hold (internal/goal/branch/attest.go:158) refuses the person's act with "restore that change". Compare in the size's own scope: this round's builder change since the tree retained before the build, records/ excluded on both sides. Test on a real git bed: oversized change held, write metasystem/records/reads/g/other.json, the person's act proceeds (mutation: whole worktree, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then the broad cmd selection for the area you touched (at least `go test -count=1 -timeout 60m -run 'TestLanding|TestWork|TestIntent|TestGoal|TestReview|TestUnit|TestClaim|TestSession|TestHelm|TestPolicy|TestQuestion|TestKeeper|TestBuild|TestHolder|TestProof|TestDesign|TestDispatch|TestEvery|TestAudit|TestInstruction' ./cmd/metasystem/`: every test there must pass, not only yours; a red you did not cause still blocks done), then `go run ./cmd/devgate static`. Report done only when all of these exit 0. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
