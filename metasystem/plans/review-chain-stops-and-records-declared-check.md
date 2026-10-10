# Brief: review-chain-stops-and-records, unit declared-check

Working Mode: Implement
Goal 2's last unit. Everything else of the goal is committed on this branch. Spec, and ONLY this text: plans/designs/review-chain-stops-and-records.md Decision 6 "Builder and unit proof execute one declared check" (all paragraphs), the declared-check rows of the Units and public-verb tables and the RC5 row, plus the two acceptance items added on 10-07 for this unit: (a) the declared cheap check stays cheap (`test plan` shows its selection before the check is frozen; its measured minutes per unit are recorded with the step times; when the risk selection resolves to deep, or its measured time exceeds a third of the last full proof's, the unit's status says so and names the setting that chose it, and a person decides); (b) `proof.audits` flags changed assertions in pre-existing tests: an agent's change without a cited design line holds for a person, a person's never holds.

Build: resolveUnitCheck freezes proof.cheap, proof.audits, proof.deadline, their source tree, working directory and execution environment at round admission from the committed content (config.CommittedContentLookup, internal/config/resolve.go:490; the landing reader at cmd/metasystem/intent_landing_prove.go:43); `test run --unit-run RUN` executes exactly that frozen check (no recursion); the builder's brief calls it and the runner invokes the same executor to attest; candidate edits to the declarations take effect only for the next round; replay onto another tip re-resolves there; publication and carry gates use the same executor (a subject-commit entry for rebase carry, internal/goal/branch/rebase.go:319 via goal_branch.go:105); missing or unreadable declarations are NeedsReview without failing a rebase, and a person's repair is executable; plus (a) and (b).
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (resolveUnitCheck, the freeze and `test run --unit-run` first) and report the rest.
Public-verb test (from the design): an unchanged agent `work build` launches against the committed declarations without adding --check; its brief calls `test run --unit-run RUN`; the fixture builder runs it, then the unit proof repeats the same cheap and audit commands and environment; both exits are retained and an audit failure makes the check red. Mutations: let a build weaken its own check after admission; resolve a fresh selection inside test run --unit-run.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
