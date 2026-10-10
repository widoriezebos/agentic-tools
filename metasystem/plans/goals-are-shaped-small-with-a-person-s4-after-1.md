# Brief: goals-are-shaped-small S4, correction 1

Working Mode: Implement
S4 (reverse an unstarted split) is uncommitted in this worktree. Its read found one material defect (reproduced); fix exactly this.

internal/goal/split_reverse.go:53-58 decides "started" from the current claim, slice and episode fields plus a fixed list of history verbs (claim, steal, slice-start, release, a displaced line). A child that was approved, unblocked, grouped while the group was claimed (bindClaim binds a claim), then ungrouped (internal/goal/verbs.go:630 drops it) leaves history `set-arc` then `detach`, no displaced marker, empty episode, so the reverse exits 0 and retires a child that once held a claim; the design (S4) says a claim counts as started even after it ends. Record that a claim happened at the moment it is made, in bindClaim (a durable field or history line), and check that record here instead of extending the verb list. Add the critic's "ungrouped claim" scenario to TestGoalSplitReversesUnstartedChildren (goal_split_reverse_test.go): the reverse is refused naming that child. Mutation: ignore the recorded claim -> red.
Also (not material, cheap): `goal show` (cmd/metasystem/goal_show_view.go:92) prints "Split into goals: ..." after a reversal; add "(reversed <date> by <person>)".


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
