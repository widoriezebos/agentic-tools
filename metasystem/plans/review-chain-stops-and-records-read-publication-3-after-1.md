# Brief: review-chain-stops-and-records read-publication-3, correction 1

Working Mode: Implement
read-publication part 3 is uncommitted in this worktree. One Opus read found one material defect; fix exactly this.

cmd/metasystem/intent_goals.go:930-941: after looking up projection.Tree.Done[id] (:931-934) the code calls recordUnitStopOverride (intent_unit_questions.go:20) without checking it, and each record gets a fresh operation ULID, so every repeat of `goal done G --reason X --by NAME` on a concluded goal writes another unit-stop-overrides record and prints the impact again for a conclusion that already happened; `goal show` offers that command for every done goal (intent_goals.go:466-467). When Done[id] exists and a goal-done record for id is present, skip the record and the impact print and only complete any pending closure (the worktree paths come from that earlier record, intent_unit_questions.go:57-79); offer the `goal show` next step only when cleanup is actually pending. Extend TestGoalDoneClosesReviewsWithReasonAfterPublication (goal_done_review_cleanup_test.go:100-104, :164, :192): the record count is 1 after each repeat (mutation: re-record on repeat, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
