# Brief: review-chain-stops-and-records build-outcomes-b2b-fixes, correction 1

Working Mode: Implement
The fix unit is uncommitted in this worktree. Its read found one material defect; fix exactly this.

F-1. With custody fields on the reservation (internal/dispatch/unit_launch.go:74-76) the stop scan counts a pending or running unit launch as local pending work (internal/dispatch/stop.go:537-549); the breach-stop loop cancels pending job records (internal/delegation/phases.go:283-293, internalCancel), which for a unit-launch reservation (no pid or pgid) only writes status cancelled (phases.go:207-239; pending->cancelled allowed, internal/dispatch/record.go:60); the next scan skips the terminal reservation (stop.go:512-516) and the batch reports COMPLETE while the execution in the launch store keeps running. A person's `work stop` cancels the launch through launches().Cancel (cmd/metasystem/intent_process.go:1108-1114); the breach path must too. Have the breach stop cancel unit-launch members through the launch manager before marking the reservation, and have the scan treat a unit-launch reservation as terminal only when its execution in the launch store has ended. Test: a breach stop with a held (running) unit build: the launch is cancelled, then the batch completes; before the launch ends the batch is not complete (mutation: mark the reservation only, red).
Also give the "no proven claim custody" refusal of a unit launch the remedy the dispatch refusal prints (stop.go:82-83).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
