# Brief: lane-reads-its-policies U5, correction 1

Working Mode: Implement
U5 (the goal's last unit) is uncommitted in this worktree. One Opus read found two material defects; fix exactly these.

F-1. cmd/metasystem/landing_plain.go:135-190 landIncidentHold: every ordinary `work land` on a configured lane now calls handInTrunkInputs -> laneContext and plain.TrunkInputs and refuses with GOAL_LAND_TRUNK_RED "Nothing was handed in" when either read fails, even with no incident on main; TestWorkLandHandInWritesJoined (all three subtests) now fails with "the landing lane can't be read: this shell's home directory isn't known" and passes with U5's files reset. Decide by the design (plans/designs/lane-reads-its-policies.md Decision 5, trunk permission): an unreadable trunk input must fail safe for an agent and never block a person; if the design holds the hand-in on an unreadable input, give the refusal a remedy that works when followed and update the test beds to supply the lane home; otherwise do not hold when no incident can be known. Then run the hand-in and card tests that call this code by name.
F-2. landing_plain.go:313: `details := inv.writeJoinedCard(goalID)` now runs before the `!added` check (laneQueueState, ~:319), so a refused plain repeat of a returned hand-in rewrites the card from returned to joined (Owner/Job/Proof/Batch cleared; the update exempts the returned stage, :366), and each repeat on a joined card rewrites Writer.At. Write the card only when the hand-in was added or the exception recorded, after the laneQueueState early return. Test: a returned hand-in, repeated `work land G` without --again: refused, card still returned, unchanged bytes (mutation: card write before the check, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
