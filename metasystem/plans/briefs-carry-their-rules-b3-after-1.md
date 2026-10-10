# Brief: briefs-carry-their-rules B3, correction 1

Working Mode: Implement
B3 is uncommitted in this worktree. Its read found two material defects (it ran the design's B2, B3 and B5 sections through the code with real git); fix exactly these.

1. cmd/metasystem/intent_brief_readers.go:66-73 detects deletion intent by the words delete/deletion/remove/prune (stripping only an adjacent negation), so B3 and B5 ("Carry explicit unit deletion intent", "no ... deletion executor") get all five deletion rules and `MISSING DECISION: the deletion scope`, which briefScaffold adds to missing (intent_work.go:2241) and work build then refuses (intent_work.go:807) with a remedy that would need an accepted design edited. Take deletion intent only from an explicit declaration in the unit's specification (a Deletion row or a `Deletes:` line, as the design names it); anything else is no intent. Add a test with the real B3 and B5 wording: no deletion rules, no refusal. Mutation: keyword detection -> red.
2. internal/project/brief_readers.go:81-85 matches by plain substring over every tracked file: for B3 the readers section is 4,442 lines (351 KB), `high` hits 3,665 lines ("highlight"), and it takes 109 s (one git cat-file per file). Match whole identifiers only; never search setting values cited in the text (high, xhigh and the like are values, not identifiers); read all files in one batch git call (cat-file --batch). Keep every true match (no truncation); group matches by file. Test with a fixture that contains highlight/xhigh noise and the real identifier. Mutation: substring matching -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
