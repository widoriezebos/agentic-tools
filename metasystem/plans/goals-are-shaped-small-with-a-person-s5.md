# Brief: goals-are-shaped-small-with-a-person, unit S5 (child approval notice)

Working Mode: Implement
S1, S2 and S4 are committed on this branch (S3, the size remedy text, stays as is). Build unit S5 of plans/designs/goals-are-shaped-small-with-a-person.md (accepted; read "Decided by m1e for Wido", the 10:45 S2/S5 acceptance item, and R-148-m1e in memory/rulings.md) and ONLY S5: when a split opens held children (a person's split, or an agent's follow-up that is part of a person-created goal under R-148-m1e), one immediate channel message asks the person to approve them (internal/channel/, internal/channel/phase/), naming the parent, the children and the approve command; sent once per split; recovery (`goal sync --recover` confirming a split, or a person's rerun of an unlanded split) sends any notice still owed exactly once; a failed send is retried by the existing channel owner and never blocks the split. The message follows the human-readable rule (line 1 the plain situational reason, line 2 the command).
Size: at most 250 production lines (the design estimates 170).
Public-verb test: the design's S5 test: a split sends exactly one approval message; a lost confirm recovered by goal sync --recover sends the owed message once; a rerun sends none twice; a failed send leaves the split applied and retries. Mutations: send per child; skip the owed notice on recovery.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
