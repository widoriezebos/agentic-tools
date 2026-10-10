# Brief: runs-advance-on-their-own, unit D5 (ordered boundary consumption, next unit and one hand-in)

Working Mode: Implement
Branch goal/runs-advance-on-their-own (D1, D1b, D2, D2b, D3, D4 committed). Build unit D5 of plans/designs/runs-advance-on-their-own.md (accepted): its Units row (steward handoff consumer; work preparation/progress; delivery), the D5 rows of the five-questions table (AdvanceBoundary / event consumption and next act; brief preparation / existing work brief and build) and "Decided by m1e for Wido". Goal 3's unit boundary event and headless handoff are on main: consume them, do not add a second boundary owner. The whole-goal hand-in stops at hand-in: do NOT land (the lane will take it).
Size: at most 250 production lines (estimate 250); if it will not fit, build the largest usable first part and report the rest.
Public-verb test: TestDriverPublicBoundaryNextUnit as the design states (two declared units through the public work routes and the real fleet boundary/handoff adapter; withholding the bound handoff, predecessor death and the RearmAtBoundary result in turn gives no next admission; a re-arm failure and replacement-engine restart); mutations as the design states.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
