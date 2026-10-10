# Brief: briefs-carry-their-rules, unit B3 (textual readers, deletion instructions and effort)

Working Mode: Implement
B2 is committed on this branch. Build unit B3 of plans/designs/briefs-carry-their-rules.md (accepted; read "Decided by m1e for Wido" 12:30 and its B3 acceptance item) and ONLY B3: the textual readers and deletion instructions the composer adds to a unit's brief (internal/project/, the work composer); effort: the composer always passes an empty effort so the configured launch.build.effort decides (R-90-m1; never set BuildEffort, UnitOptions.apply at internal/launch/unit_named.go:285 would override settings).
Size: at most 250 production lines (the design estimates 210).
Public-verb test: the design's B3 test without the 149/150 effort rows; mutation: the composer sets an effort -> red, plus the design's reader mutation.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
