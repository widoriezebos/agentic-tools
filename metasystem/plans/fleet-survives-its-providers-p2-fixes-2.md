# Brief: fleet-survives-its-providers P2-fixes-2 (each caller checks its own provider)

Working Mode: Implement
P2-fixes is committed and stopped: internal/steward/seat_start.go:669 standingProviderOutage now resolves the steward-continuation roster for EVERY caller, but only revivals run on it. Seat starts launch on settings.SeatRuntime (seatRecheck seat_start.go:664, seatDecision :457 via tick.go:535, health.go:1317); handoffs resume on HandoffBinding.Runtime, the seat's runtime (handoff.go:115, revive.go:168 when a handoff, handoff_capture.go:984); the runner reads it at runner.go:501/:514. A critic's probe: seat runtime codex with a codex mark, continuation roster claude -> standing=false, so codex seats launch into the marked provider; and any roster error holds every seat start "role steward-continuation is assigned to main".

Fix: standingProviderOutage and providerWaitReason take the runtime to check. Seat-start paths pass settings.SeatRuntime; revival passes the revived intent's runtime, or the continuation roster when there is no intent; handoff passes the binding's runtime; the tick decides per decision with the runtime of the work that decision would launch (read each mark from the one ReadProviders result; do not re-read per decision). A roster error affects only the decision that needs that roster. Every caller listed above passes its runtime; no caller resolves the continuation roster except revival without an intent.
Tests: seat runtime codex, continuation claude, a codex mark: the seat start is held and a claude revival is not; a claude mark: the revival is held and the codex seat start is not; a broken continuation roster does not hold a seat start. Mutations: the continuation roster for every caller; the seat runtime for revival.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
