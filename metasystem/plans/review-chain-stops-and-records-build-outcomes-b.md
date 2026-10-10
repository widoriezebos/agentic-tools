# Brief: review-chain-stops-and-records, unit build-outcomes-b (failure attribution)

Working Mode: Implement
Part 2 of build-outcomes' split at the size cap (plans/review-chain-stops-and-records-build-outcomes-handoff.md, "Would split into two units", item 2). Part a (holds) and its fixes are committed on this branch. Spec: plans/designs/review-chain-stops-and-records.md Decision 3, paragraphs two and three ("Every failed step uses the shared cause vocabulary..." and "The unit collector and launch reservation owner reconcile..."), with its five-questions rows; build on the shared cause vocabulary (internal/landing/plain/cause.go) and the unit-stop record already on this branch.

Build, production paths only: only demonstrated `own` spends an attempt; a test exit alone is `unclassified`; compare the same declared failing command on the round's base once (or reuse exact baseline evidence): red on both is base evidence, not charged; a known flake gets its one repeat; a command that cannot run, a lost process or a moved tree is `environment`: one retry of that step with retained inputs and a fresh output path, never another build; a second environment failure stops with its cause; a deadline stops without automatic retry; unknown attribution holds and asks, never spends a correction. Environment executions are reconciled out of the counted attempts and the goal's reserved charge by launch id; replaying collection never refunds twice. A main red joins the incident owner only when the evidence names main's tree.
Size: at most 250 production lines; if it will not fit, stop and report the split.
Public-verb test (from the design): a denied command retries exactly once, keeps one unit attempt and no goal charge, and a third starts nothing; a red without attribution is unknown. Mutation: charge or rebuild an environment retry.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
