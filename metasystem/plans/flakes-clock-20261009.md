# Brief: no flaky tests: real clocks in production paths that tests hit

Working Mode: Implement
Branch fix/flakes-clock-20261009 at main 436e8ec86. Wido (2026-10-09): flakes must not exist; deterministic redesigns only (no larger caps, retries or skips). From /Users/wido/agentic-tools-evidence/flake-inventory-20261009.md:
1. The 4-second fresh ledger fetch timer (internal/goal/project.go:222 "fresh canonical ledger fetch timed out after 4s"): e52ae5263 routed 2 call sites through goal.ProjectWithDeadline; 7 remaining goal.Project(..., true, ...) sites still use the real 4s timer (TestIntentLandWholeOwnerGitAdapter/refused_atomic..., TestIntentLandProvesTheReceiptInThisProcess failed on it under load). Route all 7 through the injected deadline (test beds pass one that never expires, production keeps 4s), and add a static audit test that refuses a new direct call.
2. cmd TestFrozenPublicVersionOneCorpus...: its deadline is the package timeout minus 1m30s: give it its own deadline on an injected clock.
3. cmd TestStewardTickStartsASeatLaunch: hit the real 30s wiring limit: injected clock and a fixture ledger.
4. cmd TestIntentManualWorkLandsOnEndpoint: the disk sweep runs a live host process census: inject the census (the seam exists).
5. goal TestHandoverWaitsTheConfiguredClaimLockWait: real 1s lock wait: injected clock.
Production changes only as seams (deadline/clock/census injection), behaviour unchanged. Each changed test passes with -count=20 under -cpu 1 and while a CPU-heavy process runs; report. Keep every assertion; the walltime ratchet must not grow. Do not commit.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
