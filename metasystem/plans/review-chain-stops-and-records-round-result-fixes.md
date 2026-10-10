# Brief: review-chain-stops-and-records round-result-fixes (fix unit after the stop)

Working Mode: Implement
round-result's first part is committed and stopped with two material findings; fix them and apply the read's size cuts (the part is about 364 production lines; bring it toward 250 without losing behaviour).

F1. internal/launch/round_result.go:40-44 with cmd/metasystem/intent_unit_review.go:357-360: on UNIT_WAIT_RETRY with an already-finished check the scratch tree is closed and GateWorktree cleared, but GateSnapshot and GateLaunches are kept; after an unrelated move the repeat opens a fresh tree whose HEAD differs from the stale snapshot and fails "the publication checks changed the result tree" with a revise (a rebuild), on every repeat. Keep the scratch tree until the same command consumes the finished result, or clear snapshot and launches together with the tree. Test (the critic's probe): terminal-retake case, then an origin advance: the repeat publishes, no revise (mutation: keep the snapshot, red).
F2. round_result.go:84-87 with intent_unit_review.go:388-391: any gate error that is not a red check (BeforeModelLaunch budget refusal, AdmitChild failure, Manager.Start error) is reported "the publication checks failed" with a revise. Only a check that finished red gets the failure-and-revise remedy; any other refusal returns with its own remedy (for the budget: the person's `metasystem goal budget`) and the same command repeats afterwards. Test: a budget refusal of the gate prints the budget remedy, not revise (mutation: revise for all, red).
Cuts: move the amend-path gate in commit.go (amendUnit BeforeCommit and rebaseGate) out to read-publication (it publishes corrections); one helper for the duplicated KeepWorktree defer; drop the PublicationPending.Running branch if F1 keeps the tree; drop the legacy plan-path fallback in ProveRoundResult; drop the second ProofIdentity write in advanceRunning.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
