# Brief: landing-takes-an-hour, unit U1b (the cheap tier first)

Working Mode: Implement
U1 is committed on this branch (5423205bf): proof/full.sh runs static first, then sharded parallel tests. Build the 19:50 cheap-tier acceptance item of plans/designs/landing-takes-an-hour.md ("Decided by m1e for Wido") and ONLY that: proof/full.sh orders the run as static, then the test packages changed since the last proven main (LANDING_PROOF_BASE or the batch's base; whole for internal/*, the touched files' tests for cmd/metasystem), then every remaining shard; the cheap tier's reds are printed the moment they are known while the full run continues so one proof still finds every red; a red cheap tier marks the attempt red; no test runs twice (cheap-tier tests are removed from the remaining shards).
Size: at most 250 production lines (estimated ~100).
Public-verb test: a red in a changed package is reported before the remaining shards finish and the run is red; a green cheap tier with a red elsewhere is red; no test runs twice. Mutations: run the cheap tier's tests again in the shards; report cheap-tier reds only at the end.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
