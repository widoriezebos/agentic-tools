# Brief: runs-advance-on-their-own, unit D5b, correction 2 (the last allowed)

Working Mode: Implement
The uncommitted D5b build plus correction 1 are in this worktree; keep them. The read of correction 1 found one BREAKING finding (defect class 1, a remedy that cannot succeed):
At the start of every pass the runner clears the boundary's engine stamp (internal/steward/runner.go:338 ResetBoundary); it is set again only later in the pass after DriveWork and the re-arm (advanceBoundary, RetainBoundary(BuildStamp)). BoundaryAdmission checks the stamp before policy (`event.Engine == "" || event.Engine != engine`), so when DriveWork (runner.go:367) continues a pending build, the child's start check (checkUnitChildAuthority -> unitLaunchAuthority -> boundaryBuildAdmission) runs while the stamp is empty and is refused "waits for this engine's successful re-arm"; every retry falls in the same window. Same for an agent's work build during a pass.
Fix by subtraction: do not clear the stamp at the start of a pass; replace it only when a re-arm fails or its result is unknown (then clear), and set it on a successful re-arm. Test through the real child start check (the bed's starter must run checkUnitChildAuthority): a pending build continued by DriveWork during a pass starts; after a failed re-arm it is held. Mutation: clear at pass start -> red.
Size: D5b production is +204/-9: make the fix by removing code; report the final count.
Run the changed tests by name plus TestDriverPublicBoundaryNextUnit; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
