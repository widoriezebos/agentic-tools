# Brief: review-drops-and-design-convergence, unit optional-committed-unit-drop-2

Working Mode: Implement
The unit's first part (the bound inverse installed as one Goal-Drop commit, returning partial) and its fixes are committed on this branch. Its handoff, /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-review-drops-optional-committed-unit-drop.md, names the remainder; this unit is that remainder and ONLY that, per plans/designs/review-drops-and-design-convergence.md Decision 1 for committed optional work, round-1 change 5 (the drop commit's identity and read rule: exempt from a read of its own, records U as dropped, not unread) and R2-4 (landing groups the inverse with U's drop and counts U as dropped, not landed: internal/goal/branch/land.go:188-208, :388, :641, red.go:80, internal/landing/plain/units.go:14):
publication of the drop through the branch owner; the finalized goal outcome (U dropped, with the person's reason and impact); the read exemption for the drop commit with U's prior read history kept; the stopped unit's asks closed by the exact drop act id; landing handling of a published Drop per R2-4 (replacing part 1's blanket refusal of pending Drops for published ones).
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (publication, outcome and ask closure first; landing handling next) and report the rest.
Public-verb test (from the design): drive the bound review command against original and correction commits plus an unrelated unread V: one Goal-Drop commit, retained originals, exact proof, current install, durable outcome and exact matching ask closure; landing then counts U as dropped. Mutations: leave the asks open; land U as landed.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
