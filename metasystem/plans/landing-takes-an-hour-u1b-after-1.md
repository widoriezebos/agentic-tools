# Brief: landing-takes-an-hour U1b (the cheap tier), correction 1

Working Mode: Implement
U1b is uncommitted in this worktree (U1 committed). Its read found two material defects (both reproduced against the real repository); fix exactly these. Note the design now folds the cheap tier into U2 (round 4); the code stays as built here.

1. internal/repoproof/cheap.go:94-107 treats every declaration and string literal in a changed file as a symbol (local vars inside functions, ValueSpec at :100, strings like ""), so a one-line change in intent_landing_prove.go selected 1768 of ~1792 cmd/metasystem tests: the cheap tier is the whole package. Keep only top-level declarations and literals of 3+ characters that overlap a `git diff -U0` hunk of the changed file (the design's U4 scan rule). Test against a fixture with a local var and a one-line hunk: only the tests naming the changed top-level symbol are selected. Mutation: use every declaration -> red.
2. cheap.go:49-72 stops at the first folder holding .go files, so a change under testdata/ (e.g. internal/deploy/testdata/fixtureadapter, or the nested module internal/rootaudit/testdata/module/crossing) becomes its own package; RunNativeInventory then fails "go package discovery omitted one or more declared package directories" and full.go stops with LANDING-NOT-RUN before any test; retrying hits the same error. Keep only folders present in the `./...` inventory (climb past testdata/vendor to the owning package). Test: a change under testdata selects the owning package and the proof runs. Mutation: first .go folder -> red.
Also (not material): with LANDING_PROOF_BASE empty, cheap.go:25-33 takes the batch base from landing status without checking its state; take it only from an open batch, else the merge-base with origin/main.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
