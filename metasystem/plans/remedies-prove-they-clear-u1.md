# Brief: remedies-prove-they-clear, unit U1 (one remedy owner; three acts proven in the bed)

Working Mode: Implement
Branch goal/remedies-prove-they-clear from origin/main. Build unit U1 of plans/designs/remedies-prove-they-clear.md (accepted; read it in full, including "Decided by m1e for Wido" 14-16 and the round tables) and ONLY U1: one remedy owner and the renderer cutover; the three bed-proven acts; `system start` for a person's process roles; the cause passed only at internal/... health.go:1765-1770, trunkred.go:45-46 and the typed budget fact; publicRemedyForFact deleted. The remaining call sites are U3.
Size: at most 250 production lines (the design estimates 180).
Public-verb test: the design's U1 test (each of the three remedies, followed in the bed, clears its refusal); mutation: the design's.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
