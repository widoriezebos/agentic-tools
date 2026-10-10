# Brief: no flaky tests: internal, correction 1

Working Mode: Implement
The uncommitted flake redesigns are in this worktree; keep them. The read found:
F-1 TestFakeHostHold: production's fake host sets SIGTERM to ignored before handing over (internal/adapter/supervisor/fake_host.go:129); the new Go hold fixture (internal/missionrunner/hostturn/testdata/hold/main.go) installs its own handler and stays up only because of --ignore-term, so removing line 129 still passes. Make the fixture record signal.Ignored(syscall.SIGTERM) before signal.Notify and assert that the inherited setting was ignored; mutation (remove fake_host.go:129) -> red.
F-2 TestTheFourReadinessForms/tcp: reserveListener (run_test.go:59) listens before the app launches, so the kernel accepts connections regardless and --listen-after has no effect; the tcp readiness check always passes. For tcp, the child binds 127.0.0.1:0 and reports its address (no pre-listening by the bed); mutation (app never listens) -> red.
F-3 proof/full.sh:5 now has `trap 'rm -rf "$reporter_dir"' EXIT`: Wido's standing rule forbids rm of a variable path in scripts (the HEAD comment cites it). Use a run-numbered build path with no deletion (or a lock around the build) and change TestFullScriptBuildsPrivateReporters so it no longer asserts deletion; assert concurrent runs do not share the binary.
Run the changed tests by name with -count=10. Do not commit.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
