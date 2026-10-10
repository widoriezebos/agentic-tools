# Brief: runs-advance-on-their-own, unit D1b, correction 1

Working Mode: Implement
The uncommitted D1b build is in this worktree; keep it. The read found:
F-1 BREAKING: the new recheck in checkAdmission (cmd/metasystem/test.go:912-913, run before every command at :991) compares goal.BudgetEpisodeRevision(binding.File) with attempt.AccountingRevision (Claimed.AccountingRevision or binding.Revision, proof_run.go:1158-1161,1179). BudgetEpisodeRevision returns Approved.EpisodeRevision or the minimum of approved/claim/episode revisions (internal/goal/file.go:229-238) while a claim sets AccountingRevision to the claim's own revision (internal/goal/verbs.go:457): approve at A, claim at A+1 -> refused "the goal changed before the test run's command started". The fixture hides it by claiming (2) before approving (3) (goal_landing_portable_test.go:78-80). Also `!binding.Eligibility.Ready` is a new condition the original reservation check (proof_run.go:1147, Fence only) never had. Fix: recheck exactly the values admission recorded, by the same rule as the reservation check (no new conditions). Add a fixture in the usual order (approve, then claim) that fails on the old code.
F-2: the producer and resource waits now use only checkAdmission (:942, :951); nativeContext has no deadline, so with ReservationOwner != nil a nested run's wait can outlive the owner's limit (formerly ended by deadlineCheck). Add the owner-deadline check to the wait's check when a reservation owner exists; standalone runs keep waiting until cancel or goal change. Test with an injected clock; mutation -> red.
Run the changed tests by name; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
