# Brief: design-round-convergence-and-exits, unit design-round-cutover-3 (the last part of Decisions 2-3)

Working Mode: Implement
Branch goal/design-round-convergence-and-exits; design-round-cutover parts 1 (db5b4b500) and 2 (f9dcb392a) are committed. Build what the handoffs plans/handoff-design-round-cutover-1.md and -2.md in this worktree report PARTIAL/unbuilt for unit design-round-cutover of plans/designs/design-round-convergence-and-exits.md (Decisions 2-3, "Cutover, obligations and deferred homes"): zero semantics at admission, the legacy and generic close routes, held asks, automatic publication on a zero count, and the legacy critical branch at internal/dispatch/critique.go:37 (no severity buys a round for any root). Item 0, fix forward from part 2's read:
- F1 intent_design_review.go:327-330: review.stop=person returns intentInProgress whose next step is the same command (holds forever, refuses a person, prints <nil> in Details when the cause is the policy). Name the act that releases the hold, as intent_design_acceptance.go:239 does ("release that hold to resume"); a person is never refused.
- F2: a root of THIS design recorded under an old path with an unreadable round-1 subject.json is skipped by designCritiqueChains (sameRecord false, path differs), so design review starts a fresh root and resets the four-examination allowance; the design forbids a fresh root (mutation "root reset by rename"). Match this design's roots by identity/alias even when the subject is unreadable, and refuse with a remedy rather than resetting.
- F3 finding_register.go:400-409 rewrites every earlier accepted/refuted finding as withdrawn when the next read does not repeat it, recording the author's acceptance as a critic withdrawal. Keep accepted/refuted as they are; make the close check read them correctly instead.
Each with a test failing on the old code. Size: at most 250 production lines; if it will not fit, build item 0 and the close routes first and report the rest.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
