# Brief: review-drops-and-design-convergence, integration fixes before landing

Working Mode: Implement
This branch (HEAD c83cc2f1c, main bd20fb8a4 merged in) has two internal reds that pass on main, from this goal's design-size-admission unit:
1. internal/config TestNoConfigurationFileResolvesTheCompiledDefaults (defaults_test.go:287): Get(design.unit-lines-max) without a configuration file = 1, "cannot read metasystem configuration: open : no such file": the new key has no compiled default. Give it its compiled default in the config defaults owner (the design's unit cap, 250) like the other keys, and check its other new keys the same way.
2. internal/launch TestSizesFromTableStopsAtTheFirstNonTableLine (admit_test.go:219): units=[] size=0 err="unit Count the output with command has no number in its total size column": the size-table parser no longer stops at the first non-table line, or now demands a total size column the fixture's table does not have. Decide by the design (plans/designs/review-drops-and-design-convergence.md, design-size-admission): production where the parser broke an existing table form, fixture only if the design changed the table.
Then run both by name, the whole internal/config and internal/launch packages, `go vet ./...` and `go run ./cmd/devgate static`. Never the whole cmd package. Never loosen an assertion. Report cause and fix per test.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
