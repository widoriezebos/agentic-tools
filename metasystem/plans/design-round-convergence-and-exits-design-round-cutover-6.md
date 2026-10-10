# Brief: design-round-convergence-and-exits, unit design-round-cutover-6 (guidance and the last gaps)

Working Mode: Implement
Branch goal/design-round-convergence-and-exits; design-round-cutover parts 1-5 committed (latest d03f254e2). DESIGN_ROUND_ONE is gone from production code; its row remains in internal/refusal/register.go: remove it if no code emits the token (the refusal audit tells). Build the rest of unit design-round-cutover of plans/designs/design-round-convergence-and-exits.md:
1. Guidance: skills/design-critique/SKILL.md (around lines 61 and 66 still teach "one round ... only a critical finding earns a second round" and "design admission also requires a critical finding for a second round"), docs/orchestration.md, docs/design/design-obligation-gate.md and mechanism 2's design owner, updated to the design's rule (at most four examinations; convergence by falling counts; no severity buys a round; held policy asks its holder). Run `go test -run 'TestAudit|TestInstruction' ./cmd/metasystem` (instruction budgets).
2. Held asks for goal-free reviews: designEffectPolicy raises an ask only when the review has a goal; a held goal-free review must record its ask too (or the design's stated owner for goal-free designs).
3. A public-path proof with a POSITIVE fourth count (e.g. 5, 4, 3, 2): the fifth examination is refused by the frozen cap (not because the chain closed). Mutation: raise the cap -> red.
Size: at most 250 production lines (guidance text is not production code).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
