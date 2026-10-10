# Brief: review-chain-stops-and-records build-outcomes-b2b-fixes (fix unit after the stop)

Working Mode: Implement
b2b is committed and stopped with two material findings; fix exactly these.

1. internal/launch/unit_attribution.go:151-163 with unit_accounting.go:18-27: the base-comparison (-base) launch reserves, then Manager.Start fails before a record exists; the code returns startErr and never settles the reservation (F-1 covered only step launches); collectLaunches then fails on the missing Comparison.LaunchID, so every later `work build run:<run>` exits 1, and a budget refusal of the comparison becomes "stopped unclassified" with a revise next step (a correction spent). Settle it terminal-not-executed with no charge, accept a missing comparison record in collection, and treat a budget refusal of the comparison as a hold with the person's `metasystem goal budget` remedy, not a correction. Test (the critic's probe: Supervisor nil after the proof starts): the reservation settles, a later advance proceeds, no correction spent (mutation: return startErr unsettled, red).
2. internal/dispatch/unit_launch.go:73-74, :86 read by internal/dispatch/stop.go:512-534: unit reservations carry no machineId or claimEpoch and the unaccounted record no goalRevision, so ReconcileStopBatch makes the goal's stop batch INDETERMINATE ("unproven custody coordinates", "revisionless"): a breach stop or a person's `goal stop` cannot complete for any goal that ran a unit launch. Write machineId and claimEpoch on the reservation and goalRevision on the unaccounted record, and have the stop scan skip terminal unit-launch and unaccounted records. Test: after a unit build (and after a pre-b2b run), a person's `goal stop` completes (mutation: no custody fields, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
