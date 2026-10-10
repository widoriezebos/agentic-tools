# Brief: design-round-convergence-and-exits, unit design-fold-and-split

Working Mode: Implement
design-evidence (+2), design-item-proof and design-exit-publication (+2) are committed on this branch. Build unit design-fold-and-split of plans/designs/design-round-convergence-and-exits.md (accepted; read it in full, including R3-M1, R-148-m1e in memory/rulings.md and the channel-message item) and ONLY that unit: Decision 4's one Decision-text revision, item/test mapping, buildability, resulting-size check and partial split retained on DesignExit, consumed by the stopped public dispositions; when the split opens a follow-up goal that is part of a person-created goal, the machine opens it unapproved and blocked by its source and sends one immediate channel message asking the person to approve it (R-148-m1e), if the design assigns that to this unit.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's design-fold-and-split test; mutation: the design's.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
