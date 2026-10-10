# Brief: machinery-measures-its-own-process, unit U1b (the cost reader), after its size stop

Working Mode: Implement
Your previous run stopped at the size cap and left a handoff: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-machinery-measures-its-own-process-u1b.md (read it: its owner map and dead ends stand). The coordinator's split (design page, "Decided by m1e for Wido" 2026-10-07 23:50): this unit is the typed cost reader `internal/processmeasure.Read` with all arithmetic (hours by step from launch start/end and step collection, person waits from question intervals unioned with open waits as lower bounds, job waits without double counting running work, tokens with coverage and missing usage unavailable, never zero), the full-suite argv frozen at admission (committed proof.full split with strings.Fields, exact argv compare), the unit route `work status G --work U` reading only from it, and the 23:45 change: a missing design or Estimates row records "estimate unavailable" and proceeds, only a working-tree page differing from the committed accepted page holds an agent, and a person's failed first admission does not freeze "no estimate" once a row exists. NOT here (U5): the goal-level status route, the all-run read across earlier worktrees, the publication timestamp; elapsed finish stays "unavailable" until U5.
Size: at most 250 production lines.
Public-verb test: TestProcessStepCostPublicStatus restricted to the unit route (synthetic launches with known times, one correction, a question wait; `work status G --work U` shows step hours against the estimate), plus a goal with no Estimates row building with "estimate unavailable". Mutations: count missing token usage as zero; hold on a missing row.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
