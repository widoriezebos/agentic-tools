# Brief: machinery-measures-its-own-process U5b, correction 1

Working Mode: Implement
U5b is uncommitted in this worktree. Its read found three material defects; fix exactly these.

1. cmd/metasystem/intent_process.go:1271 calls runner.Manager.Now() on a runner built as &launch.UnitRunner{Manager: inv.owners.processes.launches()} whose Manager may have no Now; TestWorkStatusNamesRoundCause (green on base) panics. Build the runner through inv.unitRunner() or fall back to inv.owners.commandNow when Now is nil. The test passes unchanged.
2. internal/channel/report.go:175 puts every process line ahead of the brain line and header even with no drift stop (~15 lines of zero hours and unknowns), so TestGoalCLIBrainStatusLine (green on base) fails "declared status did not lead with transport-free brain line" and the 12-line budget no longer covers the process block. Only a current drift stop goes first; the other report lines go after the brain line and header, inside the line budget. The test passes unchanged.
3. Decided by m1e for Wido (record it in plans/designs/machinery-measures-its-own-process.md under "Decided by m1e for Wido", 2026-10-08 13:50, reversible): status faces (goal, unit and run status, fleet) show the whole measured cost of an act; only the channel composer subtracts the delivered watermark to report what is new since the last post. cmd/metasystem/intent_process_report.go:60 passes the channel watermark to every face and processmeasure/report.go:69 reports value - previous.Hours, so after a post status shows only cost since that post. Pass no boundary for status reads. Change the new test (intent_process_report_test.go:210) to expect the full 3 minutes on goal status after delivery and the delta in the channel message. Mutation: watermark on status -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
