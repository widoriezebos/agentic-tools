# Brief: no flaky tests: races and unbounded reads

Working Mode: Implement
Branch fix/flakes-race-20261009 at main 436e8ec86. Wido (2026-10-09): flakes must not exist; deterministic redesigns only. From /Users/wido/agentic-tools-evidence/flake-inventory-20261009.md (two CONFIRMED by repeated runs today):
1. CONFIRMED internal/ui/fleet TestTheOwnerRunsOnTheTicksItIsGiven (48/40,000): the clock is advanced before the first tick's attempt has read it: wait for the first attempt (observed event) before advancing.
2. CONFIRMED internal/adapter/supervisor TestFakeCancelRace (7/150): the hold child writes into holds/ after the test returns, cleanup fails: the cancel path waits for the hold to exit before returning (production fix if the real cancel has the same gap).
3. internal/ui/httpd TestPartnerEventsRide... (hung 36 min on 10-03): (*opened).line has no bound: confirm the subscription before triggering; bounded wait via testenv.AwaitOr at the test deadline.
4. internal/proofrun TestGoPlanCounts... / TestRunWatchdogExits... (hung 44 min on 10-04): a helper re-exec waits on a pipe with no bound: test-clock deadline on every helper read, or a fixture function instead of the re-exec.
5. internal/proofrun TestLaunchSuiteWritesBannerProgressAndReapsWatchdog: real bash watchdog with 1s limits and 1ms graces: use the existing fake prober/ticks seam; wait for the done file with testenv.Await.
Production changes only as seams, behaviour unchanged (except item 2 if the production cancel path has the gap). Confirm items 1 and 2 with -count=2000 (1) and -count=500 (2) after the fix; others -count=20 under -cpu 1 and under CPU load; report. Keep every assertion; walltime ratchet must not grow. Do not commit.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
