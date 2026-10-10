# Brief: landing-takes-an-hour U1, correction 1 (with the cheap tier)

Working Mode: Implement
U1 is uncommitted in this worktree. Its read found two material defects; fix them and add the 19:50 cheap-tier acceptance item, which belongs to U1 (plans/designs/landing-takes-an-hour.md "Decided by m1e for Wido" 19:50).

1. internal/proofrun/test_go.go:315 changed `-timeout 0` to `-timeout 30m` for every native Go group (also devgate gate and test run/test groups), reversing the accepted rule from c08fe2cb8 ("a go test group is bounded by what it consumes, never by the clock"); supervisor_test.go:28 was renamed and changed to 30m. Restore `-timeout 0` and the original test unchanged.
2. internal/repoproof/full.go:221 reports batchtest failures under the plain package name instead of the declared group `go-batchtest`; replay (internal/landing/plain/replay.go:247) then runs LANDING_ONLY=metasystem/cmd/metasystem, full.go:214 skips the batchtest leg, and a real red on main reads green (no main cause, Repeat=allowed). Report the batchtest leg under `go-batchtest` so it replays as a declared group. Test: a batchtest-only failure replays and stays red. Mutation: report under the package name -> red.
3. The cheap tier: proof/full.sh orders the run as static, then the test packages changed since the last proven main (LANDING_PROOF_BASE or the batch's base; whole for internal/*, the touched files' tests for cmd/metasystem), then every remaining shard; the cheap tier's reds are printed the moment they are known (the full run continues); the cheap tier never runs a test twice (its tests are removed from the remaining shards). Test: a red in a changed package is reported before the remaining shards finish and the run is red; no test runs twice. If this pushes U1 over 250 production lines, build items 1-2 and report the cheap tier's size.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
