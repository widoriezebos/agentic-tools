# Brief: fleet-survives-its-providers P4 (first part), correction 2

Working Mode: Implement
P4's first part plus correction 1 is uncommitted in this worktree. The second read found one material defect; fix exactly this.

F1: internal/steward/unit_boundary.go:92-95 bails out for the whole seat as soon as any unit of any held goal is not at "reviewed; its read is collected and published". A read-waived goal (tier 1 or zero review rounds) cannot use work build (cmd/metasystem/intent_work.go:669), so its hand commits sit at "committed, ready to land without a read" (intent_selection.go:277) and the seat never records a boundary again, including for its other goals' completed units. Hold only on stages that mean open work (running, starting, built or committed but not yet read, under examination, review refused, stopped needing revision); "ready to land without a read" does not block. Test: a seat holding a read-waived goal with a hand-committed unit and another goal whose unit completed with its read published records that boundary. Mutation: the old any-stage check -> red.
Also (not material, cheap): make the `continue`-after-event-error mutation fail by an assertion in TestFleetBoundaryRunnerOrdersConsumers, not by the 10-minute timeout (bound the test's wait with an injected clock or a short context).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
