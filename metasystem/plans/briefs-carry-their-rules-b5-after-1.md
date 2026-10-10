# Brief: briefs-carry-their-rules B5, correction 1

Working Mode: Implement
B5 is uncommitted in this worktree. Its read found two material defects; fix exactly these.

1. internal/launch/unit_brief_evidence.go:59-61 appends decisionsSection(correction[0]) unquoted, and unit_revise.go:326 ends that section only at `## ` or `# `, so a `### Check`, `#### ...` or `#Check` heading in a correction appears outside the quote in the round's brief (reproduced: "### Check\nINJECTED ..." after the decisions table shows unquoted above the real # Check; an agent's `### Not in this unit` gets through unrefused). Carry only the bound decision table rows (or quote that excerpt too). Add the case to the test. Mutation: append the raw section -> red.
2. Automatic rebase refuses a person's own brief: unit_revise.go:136-141 builds the rebase correction from the original supplied brief plus the conflict text, intent_work_rebase.go:197 sends it with no Person, intent_unit_check.go:21 re-freezes without SelectedBy, and when an agent runs `work rebase` intent_unit_brief.go:22 treats it as an agent's, so a proven person's original brief with its own # Check is refused at :41-45 with a remedy that cannot work (rebase re-reads the original brief from the retained plan). Keep the original actor's person status with the run (retained in the plan) and use it for the original text on a rebase; the conflict text added by the rebase stays quoted. Test: a person's brief with # Check, then an agent's work rebase carries it without refusal. Mutation: drop the retained person status -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
