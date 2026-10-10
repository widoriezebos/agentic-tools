# Brief: fleet-survives-its-providers P2-fixes (fix unit after the stop)

Working Mode: Implement
P2 part 1 is committed and stopped with two material findings; fix them.

1. internal/steward/seat_start.go:668-672 (with tick.go:535 -> verdict.go:151 and revive.go:168): the revival hold looks up the mark for settings.SeatRuntime, but a dead worker is revived by an `internal delegate --revive` job on the steward-continuation roster runtime (cmd/metasystem/steward_verbs.go:219-233). With the shipped default launch.seat.runtime=off the lookup is Provider("off"), never marked, so revivals launch into a marked provider. Pick the hold's provider from the runtime of the work being revived (the continuation roster here). Remove the launch.seat.runtime=claude crutch from the newDecisionTickRepository fixture (tick_decision_repository_test.go:30) so TestTickHoldsRevivalDuringOutage runs with the empty conf (red before the fix).
2. internal/steward/revive.go:168-176: when provider state is unknown, standingProviderOutage still reports standing and revival cancels "the model provider is overloaded; holding revival until the provider recovers" (handoff: "became overloaded before launch"), whose waiting never succeeds; use providerWaitReason here as at the three corrected sites. Test: unknown provider state at revival prints the registration remedy (mutation: the overloaded text -> red).
Fold if cheap (not material): tick.go:544-546 replaces unrelated reasons (operator needed) with the unknown-provider text; keep the original reason and add the provider line.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
