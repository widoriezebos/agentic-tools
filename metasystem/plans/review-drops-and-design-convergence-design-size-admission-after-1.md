# Brief: review-drops design-size-admission, correction 1

Working Mode: Implement
design-size-admission is uncommitted in this worktree. Its read found three material defects; fix exactly these.

1. internal/launch/unit_sizes.go:23-31 (with admit.go:207-237) accepts a size column only for exact headers (lines, size, alloc, cap, production lines), dropping headers the old parser read (Changed lines, Estimated changed lines, Total changed lines, Lines production/test, tables with only Production lines); TestBuildSizeComesFromTheUnitRowsOrTheBriefLine and TestDeclaredUnitsReadThePagesUnitsTable fail "the page has no sized units table", and a Production-lines-only page refuses a person's build (intent_work.go:1243-1254). Keep the old matching for the total column (any header containing "line" that is not the production column); with no total column fall back to the production value for build sizing. Both tests pass unchanged; add a Production-lines-only case.
2. unit_sizes.go:40-79 and internal/designgate/size.go:23-37 read every sized table, including **Total** rows and Estimates tables repeating the units (e.g. plans/designs/fleet-provider-and-session-recovery.md, total 990), giving "duplicate unit identity" or "990 production lines, exceeding 250" with a remedy that cannot succeed. Skip total/sum rows and treat identical rows of the same unit within one design as one unit. Test with that page's shape. Mutation: count the total row -> red.
3. cmd/metasystem/intent_design_gate.go:173-193 decides the pre-declaration exemption from the tree of the first commit that declares the size keys (this unit's own commit on the goal branch), so designs accepted on main but absent there (machinery-measures-its-own-process.md, review-chain-stops-and-records.md) lose the R2-2 exemption after main is merged in. Decide from the page's own acceptance history relative to when the declaration reached origin/main (a page accepted before the declaration first appears on origin/main is exempt). Test: a page accepted on main before the declaration, merged into a branch that declares it: exempt. Mutation: the branch commit's tree -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
