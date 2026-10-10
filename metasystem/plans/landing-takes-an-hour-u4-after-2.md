# Brief: landing-takes-an-hour, unit U4, correction 2 (the last allowed)

Working Mode: Implement
The uncommitted U4 build plus correction 1 are in this worktree; keep them. The read of correction 1 found:
- F1 BREAKING: cmd/metasystem/intent_work_commit.go:98-104 runs the committed proof.cheap through /bin/sh without LANDING_PROOF_BASE; with metasystem.conf:17 = `metasystem test impact` the verb exits 2 ("the unit check needs its comparison base"), so every `work commit` refuses. Pass LANDING_PROOF_BASE=<parent> there as the unit check and carry check do; add a test through `work commit` with the shipped declaration (no --base) that fails on the old code. Grep for every other caller that runs proof.cheap and fix each the same way (a fixture-class fix covers all instances).
- F2: internal/landing/batch/goadapter/impact.go:143 (`if !whole && len(tests) == 0 { continue }`) silently drops a CHANGED package (not only dependents) when no test is selected. Reproduction: insert `_ = 0` in toolVersionLine in cmd/metasystem/app.go -> plan prints only the canary groups. A changed package with a code hunk and no selection must widen to its whole package (fail closed), and a changed testdata folder must select the tests that name the folder (e.g. testdata/signedinlaunchgit -> signed_in_launch_e2e_test.go). Test both; mutation: restore the continue -> red.
- F3: production lines are ~390 against the 250 cap. While fixing, remove duplication and dead branches in the unit's own code; do not add features. Report the final non-test line count per file.
Run the changed tests by name and their mutations; nothing package-wide.




The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
