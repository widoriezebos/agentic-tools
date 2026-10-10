# Brief: landing-takes-an-hour, unit U1 (the gate survives a panic and runs in parallel)

Working Mode: Implement
Branch goal/landing-takes-an-hour from origin/main. Build unit U1 of plans/designs/landing-takes-an-hour.md (accepted; read it in full, including "Decided by m1e for Wido" 18:45 with its U1 acceptance items R2-1 and R2-2, and the round tables) and ONLY U1: the lane's full proof path (proof/full.sh -> internal/repoproof/full.go runHost, today one `go test ./...` counting only fail events) runs through the sharded parallel executor the design names (proofrun's native inventory, internal/... and cmd/... in parallel, cmd/metasystem sharded so a panic loses one shard); every package and shard prints one line and only `ok` lines count as green (a panic, a missing line, or a TestMain non-zero exit after passing tests is red, naming the package); `landing environment` is emitted first on every run; LANDING_PROOF_GROUPS / LANDING_ONLY keep declared-group execution (a declared group id is a group, otherwise a Go package or named test); a hand gate and the lane's proof run the same command.
Size: at most 250 production lines (the design estimates 205).
Public-verb test: the design's U1 test: a deliberately panicking test still leaves every other package and shard reported and the run red naming that package; a TestMain non-zero exit is red; a scoped verb-ratchet replay runs its declared group. Mutations: count fail events only; treat a group id as a package.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
