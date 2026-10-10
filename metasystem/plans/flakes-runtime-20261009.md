# Brief: no flaky tests: runtime packages (proofrun, supervise, landing receipts)

Working Mode: Implement
Branch fix/flakes-runtime-20261009 at main 436e8ec86. Wido (2026-10-09): flakes must not exist; redesign deterministically, no larger caps, no retries, no skips. From /Users/wido/agentic-tools-evidence/flake-scan-runtime-20261009.md:
1. proofrun TestResourceCustodyPublicWriterFailureStillDrainsAndReleasesCapacity (resource_custody_output_test.go:529, script :539, cap :544): `sleep 1` window vs watchdog first poll: keep the section open until a release file appears, written after the note is seen (as the sibling TestResourceCustodyStreamsLiveCapNoteAndRetainsSpools does).
2. supervise TestKilledWatcherIsRestoredWithinOneOwnerPass (owner_test.go:216-220): SIGKILL then immediate Cycle: wait until the watcher reads Dead (or reap) before Cycle; cover the not-yet-dead branch in its own deterministic test.
3. proofrun TestLaunchSuiteSecondFenceErrorUsesPhysicalStopClock (launcher_test.go:496/:521): real 1s kill grace on a TERM-ignoring process: assert the TERM-then-KILL order and wait for Dead without a wall-clock limit (inject the grace clock if needed).
4. landing receipts (receipt_test.go:86, testing_test.go:292/720, proof_receipt_test.go:415/547): real time.Now() as a 5-minute reservation start: fixed fixture time.
Production changes only as seams; behaviour unchanged. Each changed test passes with -count=20 under -cpu 1 and while a CPU-heavy process runs; report. Keep every assertion; the walltime ratchet must not grow. Do not commit.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
