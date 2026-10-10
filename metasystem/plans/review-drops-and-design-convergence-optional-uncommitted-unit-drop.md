# Brief: review-drops-and-design-convergence, unit optional-uncommitted-unit-drop

Working Mode: Implement
The committed drop (parts 1-3 and fixes) is on this branch. Build unit optional-uncommitted-unit-drop of plans/designs/review-drops-and-design-convergence.md and ONLY that: Decision 1's exact pending-patch removal for an optional unit whose work is not committed, and the mixed committed/uncommitted case, reusing the first unit's operation, proof, impact and ask owners (the bound stopped review, the retained checks on the exact candidate, the outcome with the person's reason and impact, ask closure by the exact drop act). Only the unit's own pending patch is removed; nothing else in the tree changes.
Size: at most 250 production lines (the design estimates 135).
Public-verb test: the design's test for this unit: a stopped optional unit with uncommitted work is dropped through the bound review: exactly its patch removed, retained checks run on the resulting tree, outcome recorded, asks closed; the mixed case drops the committed part by inverse and the uncommitted part by removal in one operation. Mutations: remove another unit's change; skip the checks on the candidate.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
