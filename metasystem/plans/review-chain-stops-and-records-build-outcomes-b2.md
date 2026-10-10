# Brief: review-chain-stops-and-records, unit build-outcomes-b2 (attribution and accounting)

Working Mode: Implement
Part 2 of build-outcomes-b's split (its handoff: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/review-chain-stops-and-records-build-outcomes-b-handoff.md, parts 2 and 3: about 100 + 30 + 60 lines). b1 (failed-step execution, holds, person step retry) and its fixes are committed on this branch; build on them. Spec: plans/designs/review-chain-stops-and-records.md Decision 3, paragraphs 2 and 3.

Build, production paths only:
1. Attribution: an unclassified red compares the same declared failing command (exact argv, environment, working directory) on the round's retained base once, or reuses exact matching baseline evidence; red on both is base evidence, not charged; green on base and red on the change is demonstrated `own`. The handoff notes internal/landing/plain/replay.go's lane functions do not fit a unit's exact command: write the unit's own comparison, small. A test on the flake register gets its one repeat. A main incident is joined only when the evidence names main's tree (never from a branch base alone).
2. Accounting: connect the collector's physical launch ids to the reservation owner (internal/dispatch/budget.go ProjectBudget; cmd/metasystem/intent_work.go unitLaunchAuthority): environment executions are excluded from counted attempts and from the goal's charge by launch id; replaying collection never refunds twice.
Size: at most 250 production lines; stop and report if it will not fit.
Public-verb test (from the design): a red that is also red on the base is not charged; a red only on the change is own and spends an attempt; a denied command's retry keeps one unit attempt and no goal charge, and replaying collection does not refund twice. Mutations: charge a red base; refund twice on replay.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
