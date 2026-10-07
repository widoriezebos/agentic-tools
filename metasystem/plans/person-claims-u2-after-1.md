# Brief: person-claims U2, correction 1

Working Mode: Implement
U2 is uncommitted in this worktree. One Opus read found two material defects; fix exactly these.

F-1 (remedy). cmd/metasystem/claim_holder.go:56-57 sends a delegate, supervision, adapter-supervisor or steward caller to refuse(), which prints `metasystem session start` then "repeat the claim" (:42-44); the repeat classifies the same pid as ClassDelegate again and is refused again, and it invites a delegate to prepare a session. For these classes print no session-start step: say the claim is made from the session that holds the checkout (name it when known). TestClaimSessionDelegateRefusedBeforePreparation asserts the Next/remedy text and that following it does not loop (mutation: the old session-start next, red).
F-2 (older entry hides state). intent_planning.go:1148 turns a confirmed claim into intentPartial when preparation leaves an adoption pending, and acquireClaim (:1412) returns that; intent_work.go:678, intent_design_review.go:385 and intent_manual_submit.go:128 treat anything but intentConfirmed as "the claim was not granted ... nothing was built" although the goal is claimed. Make acquireClaim return whether the claim was granted (the claim's own outcome) separately from the adoption facts, and the three callers decide on the claim; the pending adoption is reported, not treated as refusal. Test through work build: an agent's claim granted with an adoption pending under another lineage builds (mutation: decide on the outcome word, red).

Also replace the stubbed git worktree add in TestClaimSessionForeignWriterKeepsOwnerAndNamesIsolation with the real effect where the bed allows it (testexec git), or say why it cannot.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then the broad cmd selection for the area you touched (at least `go test -count=1 -timeout 60m -run 'TestLanding|TestWork|TestIntent|TestGoal|TestReview|TestUnit|TestClaim|TestSession|TestHelm|TestPolicy|TestQuestion|TestKeeper|TestUp|TestHook|TestLedgerFresh|TestPerson|TestSeat|TestSteward|TestDesign|TestManual|TestEvery|TestAudit|TestInstruction' ./cmd/metasystem/`: every test there must pass, not only yours; a red you did not cause still blocks done), then `go run ./cmd/devgate static`. Report done only when all of these exit 0. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.

Known, not yours: TestIntentReadVerdictFromRetainedFindings, TestIntentGeneratedUnitPlan, TestIntentBuildRoundLimitAndReadBudget, TestIntentBuildRetainedRequest, TestIntentBuildConcurrentRepeat can fail under parallel load with "no store record of this owner names the path" on the base too; rerun them alone and report.
