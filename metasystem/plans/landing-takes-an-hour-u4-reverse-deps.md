# Brief: landing-takes-an-hour, unit U4 fix forward: impact selection includes reverse dependents (and test helpers)

Working Mode: Implement
Branch goal/landing-takes-an-hour (HEAD 8cbe6b00b: U1, U4, U5 merged). U4's impact selection (internal/landing/batch/goadapter/impact.go, cmd/metasystem/test_impact.go) selects the changed packages and the impacted cmd tests. Measured 2026-10-09: two hand landings left main red in internal packages that IMPORT a changed package (internal/delegation and internal/evidence fixtures broke when internal/dispatch changed), because only changed packages ran. Fix forward:
1. Reverse dependents: every package whose build or tests import a changed package (go list Deps/TestImports/XTestImports, within the module) is selected whole, through the Go adapter (the launch/cmd side stays language-generic). Test: change a package that another package imports; the importer is selected; mutation (drop reverse dependents) -> red.
2. From U4's last read (fix forward): a hunk in a non-Test function of a _test.go file (a test helper) adds that function's name to the symbols, so the tests calling the helper are selected. Test; mutation -> red.
3. Remove UnitImpact's unused third parameter (dead code from the last read).
4. A base that is not a commit refuses (fails closed) with a plain reason; test it.
Size: at most 120 production lines. Run the changed tests by name and the TestTestImpact* tests by name; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
