# Brief: landing-takes-an-hour, correction of the rounds and U3 (read 1)

Working Mode: Implement
The uncommitted U4 r2+r3, U5 r2+r3 and U3 changes are in this worktree; keep them. The read found (fix both, then trim):
F-1 internal/landing/plain/batch.go:342 lets a closed batch whose reason is not the push's back into the reconcile block: while the agent runs (not idle) the branch at :384-386 returns the closed batch as selected (no new batch selected); batchTerminal/batchContains (:474-504) now run on every closed batch, so a gc'd member SHA or pruned queue entry makes every later SelectBatch fail (lane wedged). Re-enter only when the reason is "all selected members are accounted for...", never return a closed batch as selected; a closed batch whose members cannot be read is skipped, not an error. Tests for the stall and the wedge; mutations red.
F-2 replay.go:183-192 anchors at main~1 whenever !running.Trunk; a plain proof of main's tip whose tip is a merge is then unclassified with no incident. Anchor at running.Commit when it equals the fetched main. Fix the fixture landing_incident_test.go:66-67 whose stub says main's first parent is main itself (real git never does) and add the merge-tip case; mutation red.
Trim (smallest thing, Wido): cmd/metasystem/test_impact.go: keep the original declared-group selection loop (:79-91 rewrite is behaviour-equal), remove the bare block and dead `id := unit` (:110-129), add the batchtest tag only under LANDING_ONLY (:130-137), put the report lines inside runNamedTestGroups instead of inlining RunNamedGroups (:145-180); replace the hard-coded "metasystem/" (:93,101) with the installation prefix. All briefs' tests must still pass; report the net production lines per brief after the trim.
Run the changed tests by name; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
