# Brief: runs-advance-on-their-own, unit D1b (managed suite: the deadline starts after resource acquisition)

Working Mode: Implement
Branch goal/runs-advance-on-their-own (D1 first part committed at 87d641719). Build the rest of unit D1 of plans/designs/runs-advance-on-their-own.md: the five-questions row "D1 suite deadline boundary / existing QueueDurationMS / managed test command" — the existing lease Waited() evidence; the execution allowance begins after acquire; recheck before the command. Today cmd/metasystem/test.go creates the execution deadline (around line 884) and its context (889) before resources are acquired (917), so waiting for resources spends the deadline. Move the deadline and context after acquisition, record the wait through the existing QueueDurationMS path (retained once, separate from execution), recheck admission before running the command. No new admission formula, no policy.
Size: at most 250 production lines (expect well under 100).
Test: the design's D1 test's managed-suite half — drive the managed suite's real acquire with a held resource for longer than the deadline, then release: the command runs with its full deadline and QueueDurationMS shows the wait once; mutation: create the deadline before acquire -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
