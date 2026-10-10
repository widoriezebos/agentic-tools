# Brief: runs-advance-on-their-own, unit D5b (the rest of D5)

Working Mode: Implement
Branch goal/runs-advance-on-their-own (D5 first part committed at fa3c16816). Build the rest of D5 of plans/designs/runs-advance-on-their-own.md as listed in the worktree's plans/runs-advance-on-their-own-d5b.md (read it; do not edit plans/). The hand-in is prepared, never executed; no landing.
Item 0, fix forward from D5's read: internal/steward/advance_boundary.go:57-59: an error from prepare still ends the whole pass (errors.Join return); prepareBoundary fails for every closed goal ("no longer open") and nothing prunes old events from unit-boundaries.json, so after one goal closes, other goals go unprepared every pass. Handle it like the handoff error: note it for that goal, collect the error, continue; a closed goal's event is skipped quietly. Test; mutation red.
Also cover what D5's first read found untested: the public test drives the real run loop for re-arm failure, unknown result and engine replacement (not a direct AdvanceBoundary call), a second headless unit, and repeated delivery before and after a crash.
Size: at most 250 production lines.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
