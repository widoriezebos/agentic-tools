# Brief: landing-takes-an-hour-pipeline, unit P3, correction 1

Working Mode: Implement
This worktree (branch goal/landing-takes-an-hour-pipeline) is now at goal 8's tip eea204c27 (U1, U3, U4, U5 with their rounds) with P3's uncommitted change re-applied. One conflict remains: metasystem/skills/landing-agent/SKILL.md (goal 8's U3 recovery text vs P3's one-goal/return-only text): keep both, consistent with the accepted pages (plans/designs/landing-takes-an-hour.md and -pipeline.md), then `git add` it (do not commit).
P3's read found one BREAKING finding: its own acceptance test TestOneGoalPerBatchLandsReturnsEveryRedAndReturnsAConflict failed: replay ran once per failed unit / on a non-parent commit. Goal 8's U3 (now merged) changed replay to one run on the merge's first parent with the units joined; re-run the test against it. If it still fails, fix whichever side is wrong per the accepted designs (one replay run on the parent; the proved commit is not rerun), and explain any run on a commit that is not the parent.
Also from the read: the skill's "one recorded member" is unconditional while the built-in default batch policy is auto: state that this repository's conf sets landing.batch=1.
Run the test by name, the Resolve tests in internal/landing/plain, the changed cmd tests by name, and `go test -run 'TestAudit|TestInstruction' ./cmd/metasystem` (skill text). Nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
