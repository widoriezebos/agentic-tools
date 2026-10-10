# Brief: remedies-prove-they-clear, unit U3 (the remaining refusals pass their cause)

Working Mode: Implement
U1 and U2 are committed on this branch. Build unit U3 of plans/designs/remedies-prove-they-clear.md (accepted; read "Decided by m1e for Wido" 14-16 and the R2-6 item) and ONLY U3: the remaining call sites (the 103 call-site lines across internal/steward health.go, trunkred.go, ledgerattention.go the design names) pass their cause to U1's remedy owner; healthStopped by role class; the per-role switch deleted. Split trigger: supervisionRemedy's six callers split first if the unit passes 250 production lines.
Size: at most 250 production lines (the design estimates 200).
Public-verb test: the design's U3 test (each remaining refusal prints its cause's remedy through the one owner; a person's and an agent's reader each get their own act); mutation: the design's, plus keep the per-role switch.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
