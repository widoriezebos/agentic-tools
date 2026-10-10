# Brief: review-drops optional-committed-unit-drop-3, correction 1

Working Mode: Implement
Drop part 3 (read exemption and landing) is uncommitted in this worktree. Its read found one material defect; fix exactly this.

F-1: after a published drop, `work rebase` replays the drop commit like any other (internal/goal/branch/rebase.go:300-301), so U's commits and the inverse get new ids; applyDrops matches only drop.Commit == commit.ID and the covered ids (internal/goal/branch/drop_status.go:47, :56-62), so the drop is pending again; landing refuses "pending drop ... run: metasystem work status G --work U" (land.go:620-621); the closed run record makes workContinuation hand back the landing step (cmd/metasystem/intent_selection.go:390-392) and rerunning the drop returns early (intent_unit_drop.go:25): a loop; workStage meanwhile says "dropped" from the old record (intent_selection.go:68-70). Rebase is landing's own remedy (land.go:715). Fix: rebase carries the published drop across (map the old ids to the new ones for the drop commit and its covered commits, and re-record the outcome through the same owner), so a rebased branch keeps the drop published; workStage reads the branch's drop state, not only the run record. Test: publish a drop, `work rebase` onto a moved endpoint, then landing counts U dropped with no pending refusal. Mutation: rebase without carrying the drop -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
