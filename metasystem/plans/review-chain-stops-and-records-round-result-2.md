# Brief: review-chain-stops-and-records, unit round-result-2 (critic custody and recovery)

Working Mode: Implement
round-result part 1 and its fixes are committed on this branch. This unit is the two items moved here from tree-boundaries (design page "Decided by m1e for Wido" 2026-10-08 03:50), and only those: (1) committed-critic children (the read's critic launches) are in the tree owner's custody across commands, including an interrupted dispatch and every live round, so the tree is not released while a critic still runs and a person's `work stop run:RUN` cancels it with exact custody; (2) a person can recover through `work stop run:RUN` when the advisory ownership record (internal/launch/tree_reservation.go) is unreadable or damaged: the person's act quiesces the exact owner it can prove from run and launch records and replaces the record; an agent cannot; no act claims two writers safely own one live tree. Spec: plans/designs/review-chain-stops-and-records.md Decision 4 paragraph 2 ("Fresh run/launch custody proves a child ended before releasing ownership ... a person may request replacement despite unreadable advisory state, but no act claims that two writers safely own one live tree").
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: a committed critic outlives its command and the tree stays owned until it ends, and `work stop run:RUN` by a person cancels it and releases the tree; with the ownership record corrupted, a person's `work stop run:RUN` recovers and V proceeds, an agent's is refused. Mutations: release with a live critic; let an agent replace a damaged record.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
