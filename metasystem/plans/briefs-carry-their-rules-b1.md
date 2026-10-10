# Brief: briefs-carry-their-rules, merge main, then unit B1 (select and carry one unit's exact specification)

Working Mode: Implement
Step 1 (merge). In this worktree `git merge --no-ff origin/main` is in progress with one conflict: cmd/metasystem/intent_design_gate_test.go. origin/main (436e8ec86) now has review-drops-and-design-convergence (unit drops, person scope exclusion, design size admission with design.unit-lines-max/design.goal-units-max and the production-size reader B1 waits for) and remedies-prove-they-clear. Resolve keeping both sides whole (main's behaviour, this branch's additions on top), `git add`, `git -c core.hooksPath=/dev/null commit --no-edit`, `go run ./cmd/devgate build`, `go vet ./...`, `go run ./cmd/devgate static`, and the tests of the conflicted file by name. Fix reds forward; never loosen an assertion.
Step 2 (B1, uncommitted). Build unit B1 of plans/designs/briefs-carry-their-rules.md (accepted): Decision B1 "select and carry one unit's exact specification", its Units row (internal/project/, cmd/metasystem/intent_work.go, the production-size reader now on main from review-drops-and-design-convergence: reuse it, do not write a second), Estimates, the five questions, the defect classes; "Decided by m1e for Wido" binds. B2-B5 are committed on this branch: B1 must fit them (read their code first).
Size: at most 250 production lines (estimate 240).
Public-verb test: TestWorkBriefCarriesSelectedDecision as the design states (work brief G --work B1 --out FILE on a two-unit accepted design with a folded Decision passage and a public-test row: the exact passage, its item/test ids, both size meanings and global limits, no B2 scope; and the other cases the design lists); mutations as the design states.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
