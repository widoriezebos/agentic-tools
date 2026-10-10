# Brief: design-round-convergence-and-exits, unit design-exit-publication

Working Mode: Implement
design-evidence (+2) and design-item-proof are committed and merged on this branch (2d19226b7). Build unit design-exit-publication of plans/designs/design-round-convergence-and-exits.md (accepted; read it in full, including R3-M1, R-148-m1e and the channel-message item) and ONLY that unit: Decision 5's prepared and final design exit and the automatic accepted head on the existing terminal clean/fold routes, with goal and chain replay; Decision 6's person scope/ruling and prior impact as the design names them. The machine-opened follow-up goal and its immediate channel approval message (R-148-m1e) belong to the unit that the design assigns them to; build them here only if the design puts them in this unit.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's design-exit-publication test; mutation: the design's.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
