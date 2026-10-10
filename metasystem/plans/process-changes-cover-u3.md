# Brief: process-changes-cover-declarations-and-interventions, unit U3 (declaration delta inverse through branch publication)

Working Mode: Implement
Branch goal/process-changes-cover-declarations-and-interventions (U0, U2a, U2b committed). Build unit U3 of plans/designs/process-changes-cover-declarations-and-interventions.md (accepted): its Units row (delta, branch-owned patch entry, command/act/replay/remedy; internal/processchange, internal/goal/branch, work command), the five questions, defect classes and "Decided by m1e for Wido". Same admission owner and act store.
Item 0, fix forward from U2b's read: cmd/metasystem/intent_carry_check.go (~100-131) reuses the newest check-* folder; UnitCheck.Run (internal/launch/unit_check.go:48) creates it before the check and writes result.json only on finish, so an interrupted check holds every later carry with "the newest subject execution is unavailable", and the only remedy is the person's no-audits --check. Treat a check-* folder without result.json as never started and run the check again under the same admitted act; print a remedy for a failed retained check (rerun under the same act). Tests for both; mutations red.
Size: at most 250 production lines (estimate 245).
Public-verb test: the design's U3 test and mutations.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
