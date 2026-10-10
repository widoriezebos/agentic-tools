# Brief: runs-advance-on-their-own, unit D2b, correction 2 (the last allowed)

Working Mode: Implement
The uncommitted D2b build plus correction 1 are in this worktree; keep them. The read of correction 1 found one BREAKING finding (defect class 2, an agent gets a person's power):
ResumePending (internal/launch/pending_recovery.go:31-49) saves the person's proof into AdapterData["unitPersonInvocation"] BEFORE m.Admit(spec). When admission is refused (LAUNCH_BUILD_CAPACITY, LAUNCH_BUILD_PERSON: the common capacity wait) or the caller is interrupted, it returns early and the proof stays on an unclaimed Starting record. `metasystem launch supervise --id X` is public (main.go:190) and Supervise has no admission gate: any agent or a later automatic path can claim the record, consume the stranded proof, and start a child under seat.driver=person with no person present.
Fix: save the proof only after Admit/createAdmitted succeed, immediately before startSupervisor, and clear it on every path that returns without a supervisor claim (refusal, error, interruption: write it in the same locked update that starts the supervisor, or remove it on the failure path). Test: admission refused, then a direct runLaunchSuperviseIn must hold (no child); and an interrupted resume leaves no proof on the record. Mutation: save before Admit -> red.
Size: D2b is at 249 of 250 lines: make the fix by moving code, not adding; report the final count.
Run the changed tests by name plus TestDriverPendingRecoveryAsyncAuthority and TestDriverPendingRecoveryPublicStop; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
