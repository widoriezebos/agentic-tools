# Brief: machinery-measures-its-own-process U2 (first part), correction 1

Working Mode: Implement
U2's first part (the settings act, its question and recovery; 248 production lines) is uncommitted in this worktree. One Opus read found two material defects, both reproduced (the critic's scratch test zz_crit_test.go: TestCritStaleRemedy, TestCritReproposeCollision); fix exactly these, keeping the unit within 250 production lines (make room by tightening, not by dropping behaviour).

F-1. internal/processchange/setting.go:93-98 with cmd/metasystem/intent_process_setting.go:35-37: when a person has meanwhile set a third value, the printed `settings set ... --act ID` is refused "superseded" with no next step and the question stays open forever. Keep refusing to apply the stale act (design line 118), but mark the act superseded, close its question, and return a next step the person can run (the plain `metasystem settings set KEY VALUE --repo CHECKOUT`). Test: agent proposes, person sets a third value, person follows the printed act: refused with that next step, question closed (mutation: leave the question open, red).
F-2. setting.go:66-67 derives the act id from checkout, key, before, after, goal, lineage, rule, measure, reason, so after an applied A->B and a person's later A, the agent's new A->B proposal collides with the applied act and is refused "superseded" with no question. When the retained act with this id is applied or superseded and the current value is no longer its After, the new proposal gets a fresh id (include the predecessor id in the identity); a replay of a still-pending proposal still collapses into one act. Test: the re-proposal creates a new act and one question (mutation: the old id, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
