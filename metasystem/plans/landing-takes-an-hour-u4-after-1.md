# Brief: landing-takes-an-hour U4, correction 1

Working Mode: Implement
U4 is uncommitted in this worktree. Its read found four material defects; fix exactly these.

1. cmd/metasystem/test_impact.go:49-58 adds every contract group whose paths match the change: a comment appended to cmd/metasystem/intent_carry_check.go chose 26 groups and 757 of 1797 cmd tests, including section/go-engine-gate (runs `go run ./cmd/devgate gate`, the full gate), section/adoption-fixtures (60-minute timeout) and fast-static-build, some ids twice. Add only canary and non-acceptance groups, never section/* or a group covering all of cmd/**; dedupe ids. Add a fixture contract group covering cmd/** that pins this. Mutation: add every matching group -> red.
2. internal/landing/batch/goadapter/impact.go:62-80,96-117 matches tests only against changed paths and basenames, so the same one-line change chose no cmd test (its own intent_carry_check_test.go skipped) and a change to cmd/metasystem/testdata/layout/help-test.txt chose none. Build the design's U4 (b) and (d): top-level declaration names and string literals of 3+ characters that overlap the change's `git diff -U0` hunks (never local vars), fixture basenames, and the packages that import a changed package. Fixture for a non-test cmd edit: its own test file's tests are selected and the total stays a small share of the package. Mutation: path-only matching -> red.
3. Ship the default: metasystem.conf:17 (and the adopter template) proof.cheap = `metasystem test impact` once 1-2 hold.
4. impact.go:33-44 (the exact-tree branch) is reached only by a test; the verb always passes HEAD. Drop it (U3 owns that caller).
Leave the verb's Go specifics (test_impact.go:35,63-70) as they are: U3 owns the adapter boundary.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
