# Brief: design-round-convergence-and-exits, unit design-round-cutover-6, correction 1

Working Mode: Implement
The uncommitted cutover-6 build is in this worktree; keep it. The read found:
1. internal/protocol/roles/design-critic.md:18 (embedded via internal/protocol/protocol.go:20 into every design critic's packet) still says "only a critical finding earns a second round. Every other finding is folded and recorded after one examination". Restate it under the design's rule (at most four examinations; convergence by falling material counts; no severity buys a round; fold concrete requirements in one revision). `git grep -n -i 'earns a second\|second round\|one examination'` over internal/ skills/ docs/ for any other copy. Rerun the protocol tests and `go test -run 'TestAudit|TestInstruction' ./cmd/metasystem` (the role-packet digest may move: update it through its owner, never by hand-editing a generated file).
2. intent_design_convergence_test.go:42: `fourth-positive` was moved to goalFree, converting the goal-bound scenario (counts 5,4,3,2, critical severity, reviewRoundLimit raised to 20). Restore `fourth-positive` on the goal exactly as before and add a separate goal-free scenario `fourth-positive-goal-free`. Both must pass.
Nothing package-wide beyond internal/protocol.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
