# Brief: design-round-convergence-and-exits, unit design-round-cutover-2 (the rest of Decisions 2-3)

Working Mode: Implement
Branch goal/design-round-convergence-and-exits; design-round-cutover part 1 is committed (db5b4b500: canonical history per root, unknown examination evidence, one retry matching its root, uncharged failed/retry executions, register and round stop together). Build the rest of unit design-round-cutover of plans/designs/design-round-convergence-and-exits.md (Decisions 2-3, Public-verb tests, five questions, "Cutover, obligations and deferred homes"), as listed in the builder's handoff plans/handoff-design-round-cutover-1.md in this worktree:
- the four completed examinations limit and fixed-section repeats; no severity buys another round (internal/dispatch/critique.go:37 still grants a second round for a critical finding: remove that once the new stop owns it);
- the design Decide adapter, stop/register/held-ask and all public close routes (legacy close cutover);
- TestDesignReviewConvergesAndStopsOnOneRoot as the design states (counts 5,3,1,0 converge; equal/rising counts, fixed-section recurrence and a positive fourth count stop) and its mutations;
- item 0, F-1 from part 1's read: ReadDesignCritiqueChains (internal/dispatch/design_chain.go:40-55) checks every design-critic root of the goal; a legacy root of another design with no frozen subject refuses `design review` for every design of the goal with a remedy that cannot succeed. Check only the roots matching this design's identity or alias; test it.
- add a test for the suspected double fill: source evidence readable again after a retry was reserved rewinds findingRegisterRound (finding_register.go:217-226); if the retry child is folded too, the same examination must not fill twice.
Size: at most 250 production lines; if it will not fit, build the largest usable part with item 0 and the convergence test first, and report the rest.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
