# Brief: runs-advance-on-their-own, unit D2 (exact-operation recovery and cancellation)

Working Mode: Implement
Branch goal/runs-advance-on-their-own (D1, D1b, D3, D4 committed). Build unit D2 of plans/designs/runs-advance-on-their-own.md (accepted): its Units row (launch reconciliation/supervision; work stop; existing custody), the D2 rows of the five-questions table (ResumePending / reservation reconciliation; cancellation tombstone / work stop) and "Decided by m1e for Wido". Builds on D1's pending operation (internal/launch unit_pending.go). Stay out of D5 (boundary consumption).
Size: at most 250 production lines (estimate 245); if it will not fit, build the largest usable first part and report the rest.
Public-verb test: TestDriverPendingRecoveryPublicStop as the design states (interleave work build/status/stop with a lost response, a reservation-before-supervisor crash, repeated delivery, changed target, revoked authority, changed host registration and restart; two resume callers create one child; unknown identity receives no signal/reap; cancel before admission and during the last start check); mutations as the design states. Cancellation and recovery code must fail closed (deletion and signals only on proven identity).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
