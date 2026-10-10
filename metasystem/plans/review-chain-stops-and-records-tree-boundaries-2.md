# Brief: review-chain-stops-and-records, unit tree-boundaries-2 (the rest of the tree reservation)

Working Mode: Implement
tree-boundaries part 1 and its fixes are committed on this branch (internal/launch/tree_reservation.go: the reservation, waiting refusals, fresh boundary checks, the person's work stop run:RUN, never-started children). Read the part-1 handoff, /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-review-chain-stops-and-records-tree-boundaries.md, "Next step": this unit is that remainder. Spec: plans/designs/review-chain-stops-and-records.md Decision 4 paragraphs 1-3.

Build: the lock order tree, unit, run everywhere (today the owner's save takes unit/run before a non-blocking tree try, unit_run.go:1160, tree_reservation.go:57, so a competing check makes the owner fail busy); no lock held while waiting for another actor or a remote result; branch mutations (rebase, manual submission, publication of another run) reserve the tree for their whole operation, not only a check at admission; every remaining tree-writing entry goes through the reservation; committed-critic children are in the owner's custody; the fresh tree comparison under the tree lock at publication.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (lock order and whole-operation branch mutations first) and report the rest.
Public-verb test: a competing `work rebase G` that starts while U is between commands waits for U without making U's next command fail busy; a rebase holds the tree for its whole operation so a build of V started during it waits; publication with the tree changed after the read is not clean. Mutations: the old lock order; a check-only rebase.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
