# Brief: briefs-carry-their-rules, unit B1, correction 1

Working Mode: Implement
The uncommitted B1 build is in this worktree; keep it. The read found, on this repository's real accepted designs (a probe over every accepted design in metasystem/plans/designs is at /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/f77e7ebb-e1ab-4137-9935-206f37035290/scratchpad/b1mut/cmd/b1probe/main.go: reuse it as a check):
F-1: unitTableSection (internal/project/unit_brief.go) uses `(?mi)^#{1,6}[^\n]*\bUnits\s*\n` and takes the first heading ending in "units": "## Scope moved before the units" (goals-are-shaped-small-with-a-person.md:36), "### Scope split before the units" (machinery-measures-its-own-process.md:31); no table there, so every unit is MISSING DECISION. Select the section whose heading is the Units table heading (a heading that is exactly "Units", optionally numbered/qualified, AND that contains the unit table), not any heading ending in "units". Test with both real pages' heading shapes.
F-2: decisionNumber matches only "Decision N" headings; accepted designs number Decisions as `### 1. ...` under a "## Decisions..." heading while rows say "Decision 1" (review-drops-and-design-convergence, review-chain-stops-and-records, machinery-housekeeping). Resolve "Decision N" to the Nth numbered heading under the Decisions section too. Test with that shape; unit-round names in backticks must match without the backticks.
After the fix, run the probe over every accepted design and report how many units still lack a Decision and why (prose-only "Decided by" items are out of scope).
Clean-up while there: readerSpec's discarded Decision half — remove the second Decision selector so one owner selects Decisions.
Run the changed tests by name plus TestWorkBriefCarriesSelectedDecision; nothing package-wide.




The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
