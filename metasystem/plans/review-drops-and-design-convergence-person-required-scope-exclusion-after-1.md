# Brief: review-drops person-required-scope-exclusion, correction 1

Working Mode: Implement
The unit is uncommitted in this worktree, at ~297 production lines (over the 250 cap). Its read found four material findings. Fix 1-3 now and bring the unit within 250 by moving the status/board projection out:

1. internal/goal/unit_drop.go:63 RecordUnitDrop requires the claim holder's machine and lineage (ownPair) even for a proven person's required-scope exclusion; a person at their enrolled terminal gets the terminal-enrollment lineage (cmd/metasystem/goalsync_mutations.go:631-675), so the act is refused after the inverse commit and proof ran (the bed hides it: goal_cli_bed_test.go:225 sets owner lineage = claim lineage). With an exclusion present, admit proven human authority as a foreign-human mutation instead of ownPair. Test with an empty owner lineage. Mutation: ownPair -> red.
2. Removing ApplyScope at internal/goal/branch/land_repository.go:179 or at cmd/metasystem/intent_delivery.go:425 leaves the new tests green (the landing status and goalProgress are fed test-made statuses). Drive `work land` and progress through the real status path in TestPersonScopeCompletionReadersAndRestore. Mutations: remove either ApplyScope -> red.
3. intent_delivery.go:2272-2290 dropped a `break`, so goalProgress merges every design's Units table (changing count, order and index for multi-design goals), unrelated to this unit. Restore the break.
4. Move the status/board projection (intent_selection.go:68, board/card.go, board/view.go:279, launch/unit_review.go, launch/unit_run.go:889; ~45 lines) out of this unit; keep the restore command (the printed impact names it). Report the moved part; it is the next unit, where the flag will be derived from the current goal file (ExcludesScope) so a restore clears it.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
