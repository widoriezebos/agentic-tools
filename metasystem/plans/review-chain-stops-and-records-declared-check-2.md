# Brief: review-chain-stops-and-records, unit declared-check-2 (the rest of Decision 6)

Working Mode: Implement
declared-check part 1 and its fixes are committed on this branch (resolveUnitCheck, the frozen check, test run --unit-run, builder and runner on one executor, the publication replay in the commit's tree). The part-1 handoff, /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-review-chain-stops-and-records-declared-check.md, names the remainder; this unit is that remainder and ONLY that (plans/designs/review-chain-stops-and-records.md Decision 6 and RC5, plus the two 10-07 acceptance items):
1. Rebase carry's subject-commit entry: carry (internal/goal/branch/rebase.go:319 via cmd/metasystem/goal_branch.go:105) resolves the committed proof.cheap, proof.audits and proof.deadline at that exact subject commit, freezes them and runs them on the same executor; a missing or unreadable declaration marks the carry NeedsReview without failing the rebase; the cached GateRunID is not reused for a new subject (internal/goal/branch/read.go:203-233); a person's repair is executable.
2. Cheap stays cheap: before the check is frozen `test plan` shows its selection for the unit; the measured minutes per unit are recorded with the step times; when the selection resolves to deep, or its measured time exceeds a third of the last full proof's, the unit's status says so with the setting that chose it, and a person decides.
3. The changed-assertion audit: proof.audits flags changed assertions in pre-existing tests; an agent's change without a cited design line holds for a person; a person's change never holds.
4. Candidate edits to the declarations take effect only in the next round.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (item 1 first, then 2) and report the rest.
Public-verb test: a rebase carry runs the subject commit's declared check and marks NeedsReview on a missing declaration without failing the rebase; a unit whose selection resolves to deep shows the hold with its setting; an agent's weakened pre-existing assertion holds for a person, a person's does not. Mutations: reuse the cached gate for a new subject; skip the deep hold.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
