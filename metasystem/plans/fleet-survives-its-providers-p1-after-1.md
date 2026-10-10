# Brief: fleet-survives-its-providers P1, correction 1

Working Mode: Implement
P1 is uncommitted in this worktree. One Opus read found one material defect; fix exactly this.

internal/hostcapacity/hostcapacity.go:70 (called from cmd/metasystem/intent_machine.go:265-270) reads raw launch records (launch.Store.List()), which nothing reconciles, so a build whose supervisor and child are dead shows as running and active (the critic's scratch test TestZZPhantomBuild: a Running record with a dead supervisor pid appears in builds and goals while the same command's jobs section, through Manager.List() at internal/launch/launch.go:641-652, marks it failed supervisor-lost). Read builds through the launch manager's reconciled list (inv.owners.processes.launches().List()) instead of the raw store. Add a dead-supervisor record to TestFleetHostViewPublicMachineList: it is not listed as a running build (mutation: the raw store read, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
