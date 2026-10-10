# Brief: review-drops optional-committed-unit-drop-2-fixes (fix unit after the stop)

Working Mode: Implement
Part 2 is committed (fb64d4160) and stopped with one material finding; fix it.

F-1: cmd/metasystem/intent_unit_stop.go:120 takes the drop's revision from inv.projection(), this checkout's last-fetched goal list (goal.Project(endpoint, false, ...), intent_goals.go:40), and internal/goal/unit_drop.go RecordUnitDrop compares it with the freshly fetched tip; when the goal moves on another machine or in the UI, the refusal path (txn.go:1055 terminalFromMutate) never refreshes the local copy, so the advertised remedy (the same command, intent_unit_drop.go after goalAct) is refused forever while the inverse is already published. The test fixture hid it: intent_unit_drop_test.go:118 moves the local copy with the shared goal list (`f.bed.repo.canonical, f.bed.repo.accepted = id, id`). Fix: derive the revision and the inherited-obligation requiredness from a fetched read (Project(endpoint, true, ...)) on the drop path, or check them inside Mutate against the tip it is given. Make the goal-moved subtest move only the shared goal list (`f.bed.repo.canonical = id`); it reaches confirmed with exactly one outcome. Mutation: the unfetched projection -> red.
Noted, fold if cheap: when an accepted design page is edited after the inverse is published, the refusal at intent_unit_drop.go:91 says "code is retained", which is then false; say the inverse is published and name the next step.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
