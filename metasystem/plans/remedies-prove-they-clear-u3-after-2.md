# Brief: remedies-prove-they-clear, unit U3, correction 2 (the last allowed)

Working Mode: Implement
The uncommitted U3 build plus correction 1 are in this worktree; keep them. The read of correction 1 found one BREAKING finding, N1 (defect class 1: a remedy that cannot succeed when followed):
- internal/steward/ledgerattention.go: when no attention state exists yet (and the "no canonical tip" state), the role is marked CauseUnreadable and the person is told to "repair the unreadable evidence ... then run metasystem system check". Nothing is unreadable: the armed steward's next pass produces it. Use the remedy table's not-yet-produced cause so the line reads as the committed U2 table entry does ("nothing to do: the armed steward examines the move on its next tick").
- internal/steward/health.go checkNarratorFreshnessWithCadence: on a never-started checkout installedGeneration fails with not-found and is mapped to CauseUnreadable; the act that clears it is `system start` (process role), not repair + system check. Map a missing install identity to that; keep CauseUnreadable only for evidence that exists but cannot be read.
- Restore cmd/metasystem/testdata/layout/system-check.txt's narrator-freshness and ledger-attention lines to the correct wording (the earlier fixture text for ledger attention); never accept a wrong remedy into a fixture.
Add one test per site that fails on the old mapping (missing evidence -> unreadable) and passes now; mutation: map it back -> red. Run the changed tests by name, the layout fixture test by name, and TestHealthRemedyTableOwnsEveryRole.
Size: the smallest change; no other refactor.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
