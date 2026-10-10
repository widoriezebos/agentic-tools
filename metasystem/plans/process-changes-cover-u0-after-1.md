# Brief: process-changes-cover U0, correction 1

Working Mode: Implement
U0 is uncommitted in this worktree. Its read found two material defects; fix exactly these.

1. cmd/metasystem/intent_unit_check_test.go:261-273 seeded a previous check selection so TestIntentDeclaredCheckManualRepairRequiresPerson/person stays green, hiding that production now reads the broken declarations on a person's first manual repair (internal/processchange/check.go:63-64 calls Applicable(), which runs git show through landingProofCommand); with the seed removed the test fails at :253 "manual repair read broken declarations". Restore the fixture unchanged and skip the rung read on a person's repair (record before as unknown). Mutation: read the rung on a person's repair -> red.
2. When the process lock is busy (check.go:37 TryExclusive) or the selection write fails (check.go:131), an ordinary check is skipped and the build launches (intent_process_check.go:72-74) while the retained selection keeps its old value; a later agent `--check X` then counts as unchanged and launches without a hold under person policy. Take the lock blocking (lock.Exclusive) for ordinary checks, and when recording fails mark the history unknown so the next agent change is held. Test: an ordinary check under contention updates the selection; a failed record makes the next agent change held. Mutation: TryExclusive -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
