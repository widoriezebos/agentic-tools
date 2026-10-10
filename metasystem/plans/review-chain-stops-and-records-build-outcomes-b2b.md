# Brief: review-chain-stops-and-records, unit build-outcomes-b2b (accounting)

Working Mode: Implement
Part 2 of build-outcomes-b2's split; read its handoff at /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-review-chain-stops-and-records-build-outcomes-b2.md ("Accounting": unit launch reservation identity/revision/cap, terminal reconciliation and budget projection excluding environment executions by launch id, about 100 lines, plus production hooks about 30; this checkout has no reservations for unit launches yet). b1, b2a and their fixes are committed on this branch. Spec: plans/designs/review-chain-stops-and-records.md Decision 3, paragraph 3 ("The unit collector and launch reservation owner reconcile environment executions out of the goal's counted attempts and reserved work charge by launch id. Replaying collection cannot refund twice. Physical execution ids remain in history.").

Build: reserve actual unit launches through the dispatch reservation owner (internal/dispatch/budget.go ProjectBudget; cmd/metasystem/intent_work.go unitLaunchAuthority) so successful unit executions are in the goal's spending evidence; on collection, reconcile by physical launch id: environment executions excluded from counted attempts and from the goal's charge; replaying collection never refunds twice; the single unit round and physical launch history preserved.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest; do not stop empty.
Public-verb test: the goal projection before and after a denied (environment) execution and its retry: one unit attempt, no goal charge for the environment executions, the successful one charged; replaying collection leaves the projection unchanged. Mutations: charge an excluded execution; refund twice on replay.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
