# Brief: review-chain-stops-and-records, unit build-outcomes-b2a (attribution)

Working Mode: Implement
build-outcomes-b2 stopped at the size cap (about 310 lines); read its handoff at /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-review-chain-stops-and-records-build-outcomes-b2.md: its owner map and five answers stand. This unit is its proposed part 1 (about 180 lines with hooks). Spec: plans/designs/review-chain-stops-and-records.md Decision 3, paragraph 2. b1 and its fixes are committed on this branch.

Build: an unclassified red compares the same declared failing command (exact argv, environment, working directory) on the round's retained base tree once, in isolation, recoverable after a lost process; red on both is base evidence, not charged; green on base and red on the change is demonstrated `own`; stale comparison evidence (a different base, command or environment) is refused; a test on the flake register gets exactly one repeat; a main incident is joined only when the evidence names main's tree (never from a branch base). Accounting (unit-launch reservations, charge exclusion) is part b2b, not here.
Size: at most 250 production lines.
Public-verb test: a red also red on the base is not charged and not own; a red only on the change is own and spends an attempt; stale comparison evidence is not reused; a flake repeats once; a branch-base red opens no main incident. Mutations: charge a red base; reuse stale evidence.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
