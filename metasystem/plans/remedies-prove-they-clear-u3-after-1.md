# Brief: remedies-prove-they-clear U3, correction 1

Working Mode: Implement
U3 is uncommitted in this worktree. Its read found three material defects (two reproduced); fix exactly these.

1. internal/steward/health.go:2028 with remedy.go (the RoleProofAdmission branch): checkProofAdmission puts its own "a waiting heavy proof reclaims..." text into Command when a lease is red with no owner instruction, so Command is never empty and the table's "nothing to do" entry is unreachable; a dead lease tells a person to act on something that clears by itself. Set Command only when the reports carried a remedy. Strengthen TestHealthRemedyTableOwnsEveryRole: its actor check must assert the exact act, not the word "person". Mutation: always set Command -> red.
2. health.go:1896-1901 roleUnknown renders the human act into the Remedy field, which the agent's Stop hook reads (NewHookHealthPreview's Line health.go:243, hook_entry.go:441, runtime_hook_stop.go:615-641), so every dead process role shows the agent `remedy: metasystem system start`, a person's verb (intent_process.go:71); before U3 the agent saw its own `session start`. The hook and agent readers render through PublicRemedy("agent", ...) (or the field drops the act and the readers render it). Test: the Stop hook line for a dead role shows the agent's act. Mutation: render the human act -> red.
3. disk_role.go:877,881, context.go:80-349, delivery.go:320, governed.go:81, seatpresence.go:265-267 still pass remedy strings with no cause (`disk show`, `settings check` are reads, printed in the hook line and standing-defect messages), against the design's reader rule (every reader of the field reads the table's words; no printed remedy names a read). Give these roles causes through the one owner; extend the static walk to every file that fills the Remedy field, not only the three named. Mutation: a role passing a raw remedy string -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
