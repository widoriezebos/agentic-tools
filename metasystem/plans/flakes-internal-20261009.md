# Brief: no flaky tests: internal (applaunch, testenv git isolation, proof, false passes, UI sort)

Working Mode: Implement
Branch fix/flakes-internal-20261009 at main 436e8ec86. Wido (2026-10-09): flakes must not exist; redesign deterministically, no larger caps, no retries, no skips. From /Users/wido/agentic-tools-evidence/flake-scan-internal-20261009.md and -runtime-:
1. applaunch TestLogTailAndFollow (run_test.go:908; Follow launch.go:158): Follow starts at EOF; if its goroutine starts after the append it never sees the line: Follow signals once positioned (hook or split open-then-loop); the test appends after that.
2. applaunch freePort (run_test.go:53; 15 parallel bed tests): child binds 127.0.0.1:0 and reports its address, or the bed passes an inherited listener.
3. applaunch TestSupervisorInterruptedBetweenSpawnAndChildWrite (run_test.go:327): replace the 10s elapsed assert with a causal assert (readiness pipe closed when the supervisor is reaped).
4. applaunch supervise join (supervise_test.go:337 time.After(20s)): wait on the supervisor's done event.
5. testenv (testenv.go:~615) shared test setup: set GIT_CONFIG_NOSYSTEM=1 and GIT_CONFIG_GLOBAL to a namespaced empty file for every test process, so no machine git config (signing, hooks, fsmonitor, default branch) leaks into fixtures; remove now-redundant per-test guards only if identical.
6. proof/full.sh:5 builds to a fixed proof/.full-reporter: build to a per-run path (or lock around the build).
7. False passes: adapter/supervisor/fake_test.go:600 and missionrunner/hostturn/fake_test.go:471 (50ms/300ms negative waits): wait for a positive event after the window; config/roster_write_test.go:159, ledgerfence/fence_test.go:386, spend/measure_test.go:850 (unchanged mtime as proof): use a write counter or content compare.
8. UI (internal/ui/web/_app): src/backlog/filters.ts:133, reorder.ts:54, decisions.ts:470 localeCompare without locale: explicit locale or plain compare; run the app tests (npm test) if the toolchain is available; if the bundle must be rebuilt and `npm run bundle` refuses on the audit, do not bypass: report it (the bundle is being fixed separately).
Production changes only as seams (hooks, listeners, done events), behaviour unchanged. Each changed test passes with -count=20 under -cpu 1 and while a CPU-heavy process runs; report. Keep every assertion; the walltime ratchet must not grow. Do not commit.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
