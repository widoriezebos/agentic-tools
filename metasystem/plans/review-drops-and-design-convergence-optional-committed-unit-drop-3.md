# Brief: review-drops-and-design-convergence, unit optional-committed-unit-drop-3 (read exemption and landing)

Working Mode: Implement
The committed drop's parts 1-2 and their fixes are on this branch (8cfaa48d2): the inverse as one Goal-Drop commit, publication, the finalized outcome, ask closure. Build the remainder of Decision 1 for committed optional work in plans/designs/review-drops-and-design-convergence.md and ONLY that (handoffs: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-review-drops-optional-committed-unit-drop-2.md): round-1 change 5, the drop commit is exempt from a read of its own and records U as dropped, not unread, with U's prior read history kept; R2-4, landing groups the published inverse with U's drop and counts U as dropped, not landed (internal/goal/branch/land.go:188-208, :388, :641, red.go:80, internal/landing/plain/units.go:14), replacing part 1's blanket refusal of pending Drops for published ones (an unpublished Drop is still refused, with a next step `work status` can actually clear); the Goal-Drop commit never folds into the next unit's group (land.go:200-202); completion consumers and the board show U dropped.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (landing first) and report the rest.
Public-verb test: after a published drop of U next to an unrelated unread V, landing counts U as dropped and V as needing its read, the inverse in U's group; no read is asked for the drop commit; an unpublished drop is refused with a working next step. Mutations: land U as landed; fold the inverse into V's group.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
