# Brief: runs-advance-on-their-own, unit D5b, correction 1

Working Mode: Implement
The uncommitted D5b build is in this worktree; keep it. The read found:
F-1 BREAKING (defect classes 1 and 2): the new BoundaryAdmission (internal/steward/advance_boundary.go ~75-110) ignores policy. Under person policy the design (D5, design line 88) requires no automatic handoff and AdvanceBoundary skips the handoff and death checks, but an agent's `work build G --work next` reaches boundaryBuildAdmission -> BoundaryAdmission, which reads a nonexistent intents/.json (empty Handoff) and refuses, or waits for a predecessor death that never comes (a person's session is never ended); the Lineage=="" branch refuses the same way; no agent build of the goal is admitted again. Fix by subtraction: apply the same policy gate as AdvanceBoundary. Tests: person policy, agent builds the next unit; Lineage empty; mutations red.
Hand-in: boundaryHandIn calls landGoalRoute and queues a hand-in on the landing lane. Binding rule (Wido 2026-10-09: nothing lands until the lane is discussed): the hand-in is PREPARED as the exact public command shown in status, never executed. Remove the landGoalRoute call from the automatic path; keep the prepared command and its test; mutation (calls landGoalRoute) -> red.
Also report the final check exits and the mutation list (the build's log ended before its report).
Run the changed tests by name plus TestDriverPublicBoundaryNextUnit; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
