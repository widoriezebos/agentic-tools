# Brief: runs-advance-on-their-own, unit D2, correction 1

Working Mode: Implement
The uncommitted D2 build (first part) is in this worktree; keep it. The read found (the unbuilt D2 rest, F-3, goes to unit D2b; do not build it here):
F-1 BREAKING: runIntentWorkStopGoal (cmd/metasystem/intent_process.go, new lines) loops over every run GoalRuns returns (internal/launch/unit_named.go:637-656, no state filter), skipping only awaiting-judgement; RequestCancel writes a cancelled marker and CancelRun (internal/launch/tree_reservation.go ~274) sets State "cancelled" unchecked, so `work stop GOAL` rewrites completed and build-size runs as cancelled (and can fail on "another run owns the worktree" by listing order). Cancel only runs that are not finished; a finished run is left exactly as it is. Test: a goal with a completed run and a live run; stop cancels only the live one, the completed record is byte-identical; mutation -> red.
F-2: runIntentWorkStopGoal calls directPersonProof whenever the goal has any run, so an agent's `work stop GOAL` is refused before stopping any dispatch job; the design keeps the existing stop authority and adds none. Use the existing stop authority (whatever authorized work stop before this unit) for both jobs and runs; never add a person requirement. Test an agent's stop of a live run; mutation -> red.
Run the changed tests by name plus TestDriverPendingRecoveryPublicStop; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
