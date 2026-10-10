# Brief: runs-advance-on-their-own, unit D1 (durable pending capacity and separate wait/execution evidence)

Working Mode: Implement
Branch goal/runs-advance-on-their-own (D3, D4 and D3-fixes committed; main with goal 3 merged in). Build unit D1 of plans/designs/runs-advance-on-their-own.md (accepted): its Units row (launch step/request; suite deadline boundary; work status), the D1 rows of the five-questions table (pending act / startStep / build re-entry; suite deadline boundary / existing QueueDurationMS / managed test command; status projection / exact command) and "Decided by m1e for Wido". Goal 3 is now on main: build admission (host.builds/host.load-max) and provider outage pauses are the capacity refusals D1 must keep pending rather than fail; reuse those owners, do not add a second admission. Stay out of D2 (recovery/cancellation) and D5 (boundary consumption).
Size: at most 250 production lines (estimate 245); if it will not fit, build the largest usable first part and report the rest.
Public-verb test: TestDriverPendingWorkPublicBuildStatus as the design states (work build under a capacity refusal, work status, repeated command, later capacity: one operation and one eventual child, retained exact target/command, continuous wait interval, no failed execution or correction; the managed suite's real acquire and QueueDurationMS path); mutations as the design states.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
