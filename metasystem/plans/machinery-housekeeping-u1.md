# Brief: machinery-housekeeping, unit U1 (fixture ownership: the flakes at their source)

Working Mode: Implement
Goal machinery-housekeeping (plan goal 7). Spec: the accepted design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/machinery-housekeeping.md (read it there; not yet on this branch): Decision 1 and the U1 rows, round-1 changes 1 and 2, and round-2 acceptance items R2-M1 (the never-expiring deadline dependency threaded through the connection's claimCheck composition too, asserted at every claim-check and branch-state call) and R2-M3 (a deterministic registry-isolation proof with a barrier-held live store). This worktree is goal/machinery-housekeeping from main. Wido 09-12: artificial clocks, never load-fragile tests; fix the cause, never retry or widen timeouts.

Build: TestWorkRebaseGitAdapterHoldsAfterHistory gets the deadline dependency that never expires in its real-Git case (expiry proven only in the artificial-clock case); TestLauncherDeathKillsTheTest's owned cleanup per the design; the findings-store isolation fixed at the fixture: TestStewardRunPublicVerbRefreshRefusalStillRevivesAndDelivers (whose RunLoop calls SweepDisk in apply mode, runner.go:346) gets its own Home/registry following intent_disk_test.go:72; no production function only a fixture uses.
Size: about 50 production lines, at most 250. If it will not fit, build the largest usable first part within 250 and report the rest.
Proof: each fixed test's mutation (restore the production deadline in the real-Git case; share the registry home) turns red deterministically, and the five findings-store tests pass with -count=3 -parallel 64 by name.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
