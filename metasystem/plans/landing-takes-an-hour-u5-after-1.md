# Brief: landing-takes-an-hour, unit U5, correction 1

Working Mode: Implement
The uncommitted U5 build (the landing clock) is in this worktree; keep it. The read found:
1. BREAKING: internal/landing/plain/push.go:94-113 finishes the push on the already-on-main branch whenever batchTerminal (batch.go:442-465) is true, but batchTerminal also counts RETURNED members. The design runs this branch only when every member is on main. Reproduction: B1 pushes X; B2 selected on base X, its only member G returned before any merge, HEAD stays X; `landing push` sees X on main + B1's green for X's tree, marks B2 closed as confirmed push and closeGoalStopsLocked(G, "push") closes G's lane-return stop (return_bound.go:115) though G never landed; with a hand-pushed base proven by a trunk proof it also adds a push line for B2's returned members. Fix: require every member on main; returned members never get push lines or stop closure. Test the reproduction; mutation: back to batchTerminal -> red.
2. landing_plain.go:106 sets inv.landingWords in laneQueueState, but landGoalRoute (intent_delivery.go:2010) continues for non-records entries; later refusals have Data nil and noteLanded (landing_gate.go:191) does an unchecked `result.Data.(map[string]any)["landing"]` -> panic. Use the comma-ok form and set the words only on the result actually returned. Test a refusal after a lane landing; mutation -> panic/red.
3. The 250 cap was met by deleting ~40 lines of existing doc comments (LandingTimes, Result.Reason, Entry fields, PushChecked). Restore every deleted doc comment verbatim; do not delete unrelated comments. Report the code-only net line count; if over 250, report what would move to a follow-up rather than cutting comments.
4. Words() reads step `cheap-tier` (clock.go:169) that nothing emits, so it always shows unknown. Make the name match the step the reporter emits for the first tier (LANDING_PROOF_FIRST ordering from U1) or, if no emitter exists on this branch, show it as "not measured until the pipeline's first tier" with the constant documented in one place. Test it.
Run the changed tests by name and their mutations; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
