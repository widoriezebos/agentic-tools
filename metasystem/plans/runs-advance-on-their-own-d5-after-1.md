# Brief: runs-advance-on-their-own, unit D5, correction 1

Working Mode: Implement
The uncommitted D5 build (first part) is in this worktree; keep it. Do NOT build the rest (plans/runs-advance-on-their-own-d5b.md stays for the next part; do not edit plans/). The read found one BREAKING finding:
M1: cmd/metasystem/intent_selection.go goalUnitStages appends "; <summary>; next: <cmd>" to every unit's stage once a goal has a prepared next act; that text flows into units[].Stage, which config.Units returns (steward_seat.go:268-276) to goal 3's ObserveUnitBoundary, whose exact match internal/steward/unit_boundary.go:110 (`stage.Stage == "committed, ready to land without a read"`) and prefix check (:113) then fail, so no further boundary event is recorded and the headless handoff never fires (the run stalls). Fix by subtraction: keep the prepared act in view["boundaryAct"] and the printed lines only; leave UnitStage.Stage unchanged. Test: a goal with a unit committed without a read and a prepared act: the next boundary event is still observed; mutation (append to Stage) -> red.
Also (cheap, from the read): a handoff that does not bind makes AdvanceBoundary return an error for the whole pass, blocking preparation for every other goal: report it for that goal and continue with the others; test.
Run the changed tests by name plus TestDriverPublicBoundaryNextUnit; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
