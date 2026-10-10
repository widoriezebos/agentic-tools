# Brief: fleet-provider-and-session-recovery, integration fixes 2 (shard gate 1: two reds)

Working Mode: Implement
HEAD bf6f4f073. The sharded whole-cmd gate found two reds (both pass on main 436e8ec86, so this goal causes them); logs /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/f77e7ebb-e1ab-4137-9935-206f37035290/scratchpad/shard-fpsr/shard-*.log:
1. TestFleetProviderPausePublicStatus (fleet_provider_pause_test.go:338): "genuine recovery did not retain the first answer and closing cause": after R3's interval binding (RecoveryAfter on the mark/interval) the cleared episode no longer keeps the first answer and closing cause goal 3's test expects. Keep goal 3's behaviour and R3's fields together (production where R3 dropped a field; never loosen the assertion).
2. TestWorkDropStatusContinuesPendingPublication (intent_unit_drop_completion_test.go:80): "the drop's seat has no unique card: Cards:[] Unknown:[{Seat:...}]": the board/card read now classifies the drop's seat as unknown, likely from R4's usage/registry changes to the board or machine view (working directory registration unknown). The drop's card must still be found; fix production if R4 changed how the board resolves a seat.
Run both by name, then their neighbours (`-run 'TestFleetProvider|TestWorkDrop|TestFleetUsage'`), `go vet ./...`, `go run ./cmd/devgate static`. Never the whole cmd package. Never loosen an assertion.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
