# Brief: process-changes-cover-declarations-and-interventions, unit U2b (carry admission and exact full-suite exception)

Working Mode: Implement
Branch goal/process-changes-cover-declarations-and-interventions (U0 6d2e7c23b, U2a e35e05b9f committed). Build unit U2b of plans/designs/process-changes-cover-declarations-and-interventions.md (accepted): its Units row (carry admission and exact full-suite exception: carry/subject/reconciliation, equality/exception act, command/remedy/help/status), the five questions, the defect classes and "Decided by m1e for Wido". Use the same process admission owner as U0/U2a (processchange.AdmitCheck / AdmitDeclaration); no second act store.
Item 0, fix forward from U2a's read: AdmitDeclaration (internal/processchange/check.go:194-197) validates the baseline before the Resume branch, and resolveUnitCheck (cmd/metasystem/intent_unit_check.go:24-26) sends any retained plan.json with a Declaration to admitUnitDeclaration(..., nil) ignoring --check; once a goal's declaration-<sha>.json is damaged, every resume of an admitted round holds and the printed `--check` repair takes the same branch and holds again (a remedy that cannot succeed). Check Resume before validating the baseline (a resume needs only the matching operation's act) and let a person's --check repair take the repair route. Test: damaged declaration record, resume of an admitted round proceeds; a person's printed repair succeeds.
Size: at most 250 production lines (estimate 200).
Public-verb test: the design's U2b test and mutations.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
