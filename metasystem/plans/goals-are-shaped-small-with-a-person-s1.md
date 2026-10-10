# Brief: goals-are-shaped-small-with-a-person, unit S1 (split state and lineage)

Working Mode: Implement
Branch goal/goals-are-shaped-small-with-a-person from origin/main. Build unit S1 of plans/designs/goals-are-shaped-small-with-a-person.md (accepted; read it in full, including "Decided by m1e for Wido" and the round-2 builder option: S1 may reuse the existing parked state plus the lineage field instead of a new state, if that is smaller across the state switches) and ONLY S1: the durable representation of a split parent (not done, id not retired) and of lineage both ways, its validation, and every current state reader (internal/goal/file.go, validate.go, project.go, claim/completion readers, internal/project/goals.go, the existing board/UI projection). S2 (applying the plan file) is the next unit; S1 adds no verb that splits.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's S1 test (a goal file carrying the split state and lineage round-trips through goal show / goal list / the board, validation refuses a broken lineage, claim and completion readers treat the split parent as not claimable and not done). Mutation: the design's S1 mutation.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
