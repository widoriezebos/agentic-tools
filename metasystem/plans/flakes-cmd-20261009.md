# Brief: no flaky tests: cmd/metasystem (deterministic redesigns)

Working Mode: Implement
Branch fix/flakes-cmd-20261009 at main 436e8ec86. Wido (2026-10-09): "Flakes should not exist. ... They need to be redesigned to be not flaky at all on any machine." The scan /Users/wido/agentic-tools-evidence/flake-scan-cmd-20261009.md lists eight tests that can fail on a slower or loaded machine. Redesign each deterministically (no larger caps as the fix; no retries; no skips):
1. TestIntentBuilderChildInGeneratedWorktree (intent_connection_final_test.go:303): real child with 100ms grace and 5s caps: inject the clock and release the child by an observed event; caps only as a backstop at the test deadline.
2. TestCommitProofTerminalRefusesWithoutGoalRevisionAuthority and the fixture's other callers (proof_run_test.go:1826, 3085, ~3036, ~3606): 1s silence/section/evidence/stop caps with a 5ms bash watchdog poll: inject the clock or bind the caps to the test deadline; the watchdog waits on a fifo, not polling.
3. appFreePort users (app_verbs_test.go:200, app_engine_owner_test.go:15; TestAppStartLaunchesTheInvocationsOwnEngine is parallel): free-then-rebind port race: hand the app a held listener or let it bind :0 and report its address.
4. TestLandingKeeperBoundsBlockingFetchAndReportsIt (landing_hold_refresh_test.go:216): 20ms real fetch timeout vs `sleep 5`: the fetch deadline becomes a channel the test closes.
5. TestIntentSupervisorReadyTimeoutRetriesOnce (intent_step_failure_test.go:213): 1s real start cap: fake clock whose Sleep advances Now.
6. App start/stop tests (app_verbs_test.go:220 and the rest): real HTTP readiness 20s and stop 5s: readiness fifo; stop wait bound to the test deadline.
7. TestRunUtilHoldWritesStoppedFileOnTerm (hold_test.go:165): SIGTERM to the test process: inject a signal channel into runUtilHoldWithDependencies.
8. TestLaunchManagerReadsClaudeTranscriptsWhereClaudeKeepsThem (launch_claude_config_test.go:22): real HOME: temp HOME.
Production changes only as seams the tests need (injected clock/listener/signal channel/deadline), behaviour unchanged. Each redesigned test passes with -count=20 under `-cpu 1` and also while another CPU-heavy process runs (e.g. a concurrent `go build ./...` loop); report those runs. Keep every assertion. The walltime ratchet (internal/testenv TestNoTestWaitsOnWallTime) must not grow. Do not commit.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
