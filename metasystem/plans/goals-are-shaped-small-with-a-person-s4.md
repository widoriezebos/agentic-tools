# Brief: goals-are-shaped-small-with-a-person, unit S4 (reverse an unstarted split)

Working Mode: Implement
S1 (split state and lineage) and S2 (apply the plan file: live split parent, held children) are committed on this branch. Build unit S4 of plans/designs/goals-are-shaped-small-with-a-person.md (accepted; read its decisions) and ONLY S4: `goal split G --reverse --reason TEXT`, a person's act (the remedy already printed by S1/S2 refusals), restores the parent from split state to its prior live state with intent, budget, spending and approval evidence kept, retires the held children, and keeps the lineage record as history; it is refused only when a child's work has started (current child-work checks: a claim, a run, a reservation or a commit on the child), naming the started child; an agent's reversal is refused; a repeat holds; re-applying the same plan after a reverse needs renamed members (internal/goal/split.go validateSplitMembers:429) and the refusal says so. S5 (the channel notice) is the next unit; S3 (the size remedy text) stays as is.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's S4 test: a person reverses an unstarted split: the parent is live with its budget, the children retired, lineage kept; a split whose child has a claim is refused naming that child; an agent's reverse is refused; a repeat holds. Mutations: reverse with a started child; let an agent reverse.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
