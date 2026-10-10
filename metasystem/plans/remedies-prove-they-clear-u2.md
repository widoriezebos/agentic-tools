# Brief: remedies-prove-they-clear, unit U2 (alert clear re-arms and acknowledges)

Working Mode: Implement
U1 is committed on this branch (242d43058). Build unit U2 of plans/designs/remedies-prove-they-clear.md (accepted; read "Decided by m1e for Wido" 14-16 and the R2-1 acceptance item) and ONLY U2: `alert clear` re-arms ended healing (arbitration taken before the health/alert locks, the tick's lock order; newly exhausted roles re-arm despite an earlier cleared episode); a person's clear, enrolled or not (ProveTerminal + TerminalValidFor first, as session_stop.go:22-31; Prove only for helm/grant), acknowledges the ledger move; an ordinary agent's clear clears and re-arms but does not acknowledge the ledger and names the person's act; a no-op ledger examination is recorded FAILED, not PASS_COMPLETE.
Size: at most 250 production lines (the design estimates 200).
Public-verb test: the design's U2 test, including: clear and tick finish concurrently; two exhaustion/clear cycles re-arm both times; a person without enrollment acknowledges. Mutations: health lock before arbitration; Prove instead of ProveTerminal for a person.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
