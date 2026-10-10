# Brief: design-round-convergence-and-exits, unit design-round-cutover-5 (the last of Decisions 2-3)

Working Mode: Implement
Branch goal/design-round-convergence-and-exits; design-round-cutover parts 1-4 committed. Build what part 4's read lists as missing for unit design-round-cutover of plans/designs/design-round-convergence-and-exits.md (Decisions 2-3, "Cutover, obligations and deferred homes"; handoffs plans/handoff-design-round-cutover-*.md in this worktree):
(a) automatic publication on a zero count; (b) held-policy asks and numeric review.stop; (c) the rest of the legacy closure moved onto committed exits; (d) guidance and mechanism 2's owner: skills/design-critique/SKILL.md (lines ~61,66 still teach one round with a critical second round), docs/orchestration.md, docs/design/design-obligation-gate.md, as the design states (instruction edits run the audits: `-run 'TestAudit|TestInstruction'`); (e) a public-path proof: configured zero gives four examinations and a fifth is refused.
Also: the goal-zero refusal prints "<nil>" ("cannot resolve a positive goal review-round limit: <nil>"): give it a plain reason and remedy.
Size: at most 250 production lines; if it will not fit, build (a), (b), (e) first and report the rest.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
