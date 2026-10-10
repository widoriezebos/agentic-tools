# Brief: fleet-survives-its-providers P5 Part A, correction 1

Working Mode: Implement
P5 Part A (the abnormal restart bound) is uncommitted in this worktree. One Opus read found two material defects; fix exactly these.

1. A seat that finishes normally counts as a death: internal/steward/revive.go:470 takes the death class from whatever state the last seat launch ended in, including `completed`, and internal/steward/seat_start.go:585-643 reserves every seat start after the first as an abnormal restart; the seat brief tells a seat to end after one unit, so two normal completions with progress in an hour stop all automatic starts ("the same death class occurred twice: completed"). The public test hides it (cmd/metasystem/fleet_handoff_test.go:609 uses `completed` as an ending in its hour-cap scenario). Only abnormal endings (failed, cancelled, missing, or no progress) are a death class; a completed ending with progress consumes no attempt and reserves nothing. Change the hour-cap scenario to abnormal endings and assert a completed-with-progress seat start reserves nothing. Mutation: count completed -> red.
2. An unreserved start with an unknown outcome gives an uncounted restart: when a start was not reserved (the first start, or after a provider hold) and the launcher errors, seat_start.go:649-655 records `start-failed` with no launch state; abnormalRestartState (revive.go:470-475, :500) then returns no class and the next start runs unreserved; Manager.Start can error after the supervisor started (internal/launch/launch.go:254-257). When the last current seat's outcome is `start-failed`, hold with the unknown-outcome remedy (or reserve every start after an unknown outcome). Test: an unreserved start errors after its process started and dies; at most two automatic restarts follow in the hour. Mutation: treat start-failed as no class -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
