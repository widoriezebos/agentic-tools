# Brief: review-chain-stops-and-records round-result-fixes, correction 1

Working Mode: Implement
The round-result fix unit is uncommitted in this worktree (round-result part 1 committed at 282b37e60). Its read found two material defects, both from the size cuts; fix exactly these.

1. internal/goal/branch/commit.go:610-615: the cut removed amendUnit's BeforeCommit and rebaseGate, so a correction commit is installed and pushed (cmd/metasystem/intent_unit_review.go:434) with no static or cheap gate; the read's gate (internal/goal/branch/read.go:209-239) runs only after it is installed. Decision 4 (plans/designs/review-chain-stops-and-records.md:123) requires the gate before install. Restore the amend-path gate exactly as at 282b37e60 and restore TestRoundResultCorrectionGateGitAdapter (deleted, not moved). Keep the other cuts.
2. internal/launch/unit_run.go:511-517: the cut left only the ProofIdentity write in unit_build_outcome.go:37-41 (called at unit_run.go:496), which hashes the proof list before planning; roundProofPlan (:499, :654-660) then replaces it with the planner's commands, so the identity no longer names the proof the round runs. Keep one write, placed after roundProofPlan, and drop the build-outcome one; extend the test to assert the identity matches the planned proof (mutation: write before planning, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
