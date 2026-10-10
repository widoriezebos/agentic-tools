# Brief: machinery-measures-its-own-process, unit U5a (goal-level cost status)

Working Mode: Implement
U1a, U1b, U1b-fixes and U2 are committed on this branch. U3 (drift) waits for goal 2 (review-chain-stops-and-records) to land, because it builds on goal 2's stop vocabulary. This unit builds the part of U5 that does not need U3, the items moved here from U1b (design page "Decided by m1e for Wido" 2026-10-07 23:50): the goal-level status route, the all-run read (costs from every run of the goal's units, including earlier worktrees), and the publication timestamp so a unit's elapsed finish is known. Spec: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/machinery-measures-its-own-process.md, Decision 5 "Report own process cost without inventing causal savings" and the U5 rows, plus Decision 1 (one reader owns all arithmetic: internal/processmeasure.Read; no stored totals). The U1b handoff (plans/handoff-machinery-measures-its-own-process-u1b.md in that checkout) maps the owners: goal and named-unit status in cmd/metasystem/intent_selection.go and direct-run status in intent_process.go; NamedWork reads only the current run, so an all-run read is needed; UnitSubject.Published has no timestamp today.

Build: record the publication time where a unit's read is published (one field, written once); an all-run read for a goal's units through the same reader; `goal status G` showing per unit and for the goal: hours by step against the frozen estimate, corrections and fix units, suite minutes, person waits, and the process acts recorded for the goal (U2's ProcessAct records, own-caused, listed before anything external), every value derived on read. Drift stops are U3's and are not here.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: a goal with two units, one run in an earlier worktree, published reads: `goal status G` shows both units' step hours and the goal total derived from them, elapsed finish from the publication time, and a recorded agent process act listed first. Mutations: read only the current run; store the total.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
