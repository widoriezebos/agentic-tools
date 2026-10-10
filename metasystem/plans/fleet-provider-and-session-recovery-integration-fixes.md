# Brief: fleet-provider-and-session-recovery, integration fixes before landing

Working Mode: Implement
HEAD 8d75c07ba (main 436e8ec86 merged in). The cheap tier found four reds; fix all in this job:
1. internal/testenv TestEveryPackageUsesSharedMain: internal/hostcapacity has _test.go files but no TestMain using testenv.Main: add main_test.go as other packages do (`func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }`).
2. Same test: cmd/metasystem/provider_recovery_test.go starts an engine-pin or steward run child without testenv.ReapFixtureProcessGroups: use the shared reaper as the other steward/engine fixtures do.
3. staticcheck SA4006: internal/hostcapacity/hostcapacity.go:91: a value of err is never used: handle the error (do not drop it), as the surrounding code's contract requires.
4. run-state audit, cmd/metasystem/intent_machine.go runIntentMachineRevive: intentInvocation.stateRoot flows into internal/steward.ReviveSeat arg 0 (root) and internal/stopfence.Closed arg 0 (root) ("run state lives under the installation, so build this path from the installation"). Build those paths the way the existing machine/system verbs pass them to steward and stopfence (look at the neighbouring callers: they derive from the installation); list a crossing in run-state-audit.json only if the existing callers of the same functions are listed the same way.
Then: `go vet ./...`, `go run ./cmd/devgate static`, `go test ./internal/testenv ./internal/hostcapacity ./internal/proofrun` and the provider recovery tests by name. Never the whole cmd package. Never loosen an assertion.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
