# Brief: review-drops optional-committed-unit-drop-2, correction 1

Working Mode: Implement
Part 2 is uncommitted in this worktree. Its read found one material defect; fix exactly this.

F-1: internal/goal/unit_drop.go:65 RecordUnitDrop refuses unless the goal's revision still equals drop.Revision, saved once when the drop started (cmd/metasystem/intent_unit_drop.go:66 from intent_unit_stop.go:120) and never refreshed. Any goal act during the drop raises the revision (verbs.go:447 touch), so the outcome is refused after the inverse is published, and the remedy (:224, the same command) compares the same stale revision and can never succeed; landing then refuses the pending drop forever (land.go:613). Fix: on a repeat re-check the claim, requiredness and the bound decisions against the current goal (applyUnitDrop already re-derives them), then check the revision against that current revision; keep the revision out of the replay comparison. Extend the goal-moved subtest (intent_unit_drop_test.go:154) so the repeat reaches confirmed with exactly one outcome. Mutation: compare the saved revision -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
