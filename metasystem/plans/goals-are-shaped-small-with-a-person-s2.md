# Brief: goals-are-shaped-small-with-a-person, unit S2 (apply the existing plan file)

Working Mode: Implement
S1 (the split state and lineage, its readers) is committed on this branch. Build unit S2 of plans/designs/goals-are-shaped-small-with-a-person.md (accepted; read "Decided by m1e for Wido" including the 10:45 S2/S5 acceptance item on recovery) and ONLY S2: `goal split G --plan FILE` (the existing plan-file grammar, internal/goal/split.go:63) replaces the destructive effect (StateDone at split.go:315, id retired at root.go:348) with a live split parent and held children (unapproved, blocked by the source), recording lineage both ways, in one transaction; a repeat holds (AlreadyHolds); `goal sync --recover`'s confirm branch (internal/goal/recover.go:124 -> recoverConfirmedEffect :669-675 -> raiseSplitOldArcDebt split.go:486-501) does not fail a split whose parent is live in the split state (no old-arc debt; only archived or decomposed parents keep the old path); recover.go:223 still refuses to replay an unlanded person split, whose remedy is the person's rerun. The channel notice is S5; reversal is S4.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's S2 test as amended: a person's split leaves the parent live in split state with children held; a pushed split whose confirm was lost, then `goal sync --recover` confirms it; a rerun holds. Mutations: mark the parent done; raise old-arc debt for a live split parent.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
