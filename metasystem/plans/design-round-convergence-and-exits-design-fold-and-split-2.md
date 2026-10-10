# Brief: design-round-convergence-and-exits, unit design-fold-and-split part 2 (the split's follow-up goal, channel message, author attempt)

Working Mode: Implement
Branch goal/design-round-convergence-and-exits (HEAD be96b5b52: all earlier units plus a main merge). Build the remainder of Decision 4 of plans/designs/design-round-convergence-and-exits.md (accepted) that part 1 (e86eef0a9, "one Decision-text revision per finding ... retained on DesignExit and consumed by the stopped public dispositions") left out: when the resulting-size check splits the design, open the split's follow-up goal through the existing goal owner (unapproved, blocked by the source, as R-148 rules for agent-opened follow-ups), send its one channel message, and record the author attempt; each exactly once on a repeat. Stay out of the cutover (Decisions 2-3: canonical admission, examination retry, stop/register/ask, loopstop) — the next unit builds that here after you.
Size: at most 250 production lines; if it will not fit, build the largest usable first part and report the rest.
Public-verb test: the design's Decision 4 test for the split route, repeated twice (one goal, one message, one attempt); mutation: opening the goal approved, or a second goal on repeat -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
