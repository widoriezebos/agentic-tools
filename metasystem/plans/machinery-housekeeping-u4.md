# Brief: machinery-housekeeping, unit U4 (the flake facts in the views people use)

Working Mode: Implement
U1, U2 and U3 are committed on this branch. Spec: the accepted design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/machinery-housekeeping.md: Decision 4 "Put the fact in the views people already use" and the U4 rows (sites: cmd/metasystem/intent_planning.go:2354 incident listing, incident_list_view.go:15 rendering, intent_table.go:513 test status routing, test.go:1172).

Build: `test status --tree TREE` adds the flake facts observed for that tree (test name, original and repeat outcome, evidence, fix ownership) in text and JSON; it stays a read: flake metadata never satisfies missing proof, changes proof's exit code or launches a test; a metadata read failure reports "flake information unavailable" apart from the proof verdict (unknown metadata is never a green flake). `incident list` shows ordinary flakes only with a main observation; candidate-only facts appear through test status; a main-red record keeps its identity and blocking; allowanced main flakes stay visible with their allowance; incident claim and close resolve the existing entry id with their existing authority.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's U4 test (a tree with a flake sighting: test status shows it with both outcomes and evidence and keeps the proof verdict and exit; unreadable metadata shows unavailable; incident list shows a main-observed flake and not a candidate-only one). Mutations: let a flake satisfy missing proof; show a candidate-only flake in incident list.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
