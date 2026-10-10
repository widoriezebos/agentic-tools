# Brief: review-chain-stops-and-records round-result (first part), correction 1

Working Mode: Implement
round-result's first part is uncommitted in this worktree. One Opus read found four material defects; fix exactly these, staying within 250 production lines.

1. cmd/metasystem/intent_unit_review.go:343-351 treats any branch.StaleCode as a replay conflict, but that code also comes from origin moving between the fetch and publication (internal/goal/branch/push.go:284), the branch checked out in another worktree (commit.go:448), unstaged changes (commit.go:373, :437) and a failed checkout (commit.go:478); an unrelated move is recorded as subject.Conflict and asks for a person's revision (a rebuild). Give the Apply refusal (commit.go:390) its own replay-conflict code; every other stale error returns to repeating the command. Test: origin moves after the fetch: the command repeats and publishes, no conflict recorded (mutation: all StaleCode as conflict, red).
2. internal/launch/round_result.go:81-87 with intent_unit_review.go:360-363: a cheap check still running at the wait cap (launch.wait.cap.seconds, default 240; Manager.Wait returns not-terminal without error) or a UNIT_WAIT_RETRY is reported as "the publication checks failed" asking for a correction, and a repeat starts a fresh check that times out again. On not-terminal or UNIT_WAIT_RETRY return in progress with the same command and resume the recorded check launch. Test: a check longer than the cap publishes on the repeat without a second launch (mutation: new check each repeat, red).
3. internal/goal/branch/commit.go:398-401, :601-604: `close = func(){}` on every BeforeCommit error leaves the scratch git worktree, its token and its .git/worktrees registration behind. Keep the scratch tree only while a launched check is still running, record its path on the subject, and close it on every terminal outcome (remove by the path the code created). Test: after a terminal check failure no scratch folder and no worktree entry remain.
4. round_result.go:90-92 with unit_run.go:1075: the before/after snapshot includes the shared repository's refs/heads, so another goal's commit while locks are released fails "checks changed the result tree" and asks for a correction. Compare only the scratch worktree's HEAD, index and tree. Test: another branch commits during the check; publication proceeds.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
