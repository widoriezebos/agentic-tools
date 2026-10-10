# Brief: review-drops-and-design-convergence, unit optional-committed-unit-drop

Working Mode: Implement
Goal review-drops-and-design-convergence. This worktree is goal/review-drops-and-design-convergence, stacked on goal 2's branch (goal/review-chain-stops-and-records at 977c8cb89; goal 2's last unit is still building there, and this goal lands after goal 2). Spec, and ONLY this text: the accepted design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/review-drops-and-design-convergence.md, Decision 1 "Replace the prepared-drop paths with one bound effect" as it applies to COMMITTED optional work, the optional-committed-unit-drop rows of the Units, Estimates and public-verb tables, the five-questions rows for it, the round-1 changes for findings 1 and 5 (the source-branch refusal sites cmd/metasystem/intent_review_binding.go:118, :309-312, cmd/metasystem/intent_unit_stop.go:137; the ask's needs is the bound drop command and AcceptableActs gains a drop act id; the drop commit's identity Drop/Goal-Drop and its read rule) and acceptance item R2-4 (landing handles the Drop commit kind: internal/goal/branch/land.go:188-208, :388, :641, red.go:80, internal/landing/plain/units.go:14 group the inverse with U's drop and count U as dropped, not landed). Confirm each site on this branch before changing it. Uncommitted-patch drops and person-required exclusion are other units.
Size: at most 250 production lines (estimate 245). R2-4 says landing handling may need its own unit: if it will not fit, build the largest usable first part within 250 (the bound drop effect for committed work first) and report the rest.
Public-verb test: the design's TestWorkReviewDropsOptionalCommittedUnit, with its mutation.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
