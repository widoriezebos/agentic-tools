# Brief: landing-takes-an-hour, U4 reverse dependents, correction 1 (cost)

Working Mode: Implement
The uncommitted reverse-dependents fix is in this worktree; keep its helper selection, invalid-base refusal and parameter removal. The read measured the cost: a one-line comment in internal/dispatch/admission.go selects 81 of 174 packages whole, including `whole: ./cmd/metasystem` and ./proof, because cmd/metasystem is a dependent of nearly every internal package (internal/landing/batch/goadapter/impact.go: `whole := full || slices.Contains(dependents, pkg) || ...`). The unit check would again run the whole cmd package, which this goal exists to avoid.
Fix: select reverse dependents WHOLE only when they are under ./internal/ (where the measured reds were); an importer under cmd/ (and any other non-internal tree) keeps the existing symbol/name selection of its tests. Restore TestTestImpactImportersAndDeletedSymbolsGitAdapter's original assertions (cmd importers run the three named tests and leave out TestNoise) and add a case where an internal importer of a changed internal package is selected whole. Mutation: select cmd whole again -> red. Report the measured selection for the same one-line change in internal/dispatch/admission.go after the fix (packages count, and that cmd/metasystem is not whole).
Run the TestTestImpact* tests by name; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
