# Brief: review-drops-and-design-convergence, unit person-required-scope-exclusion-2 (status and board)

Working Mode: Implement
Branch goal/review-drops-and-design-convergence-dsa; the exclusion's first part is committed (c6eadbcac). Build its moved remainder and ONLY that: the status/board projection of a person's required-scope exclusion (cmd/metasystem/intent_selection.go:68, internal/board/card.go, internal/board/view.go:279, internal/launch/unit_review.go, internal/launch/unit_run.go:889), derived from the current goal file (goal ExcludesScope), never from the retained run record, so `goal scope restore` clears it from work status and the board at once.
Size: at most 250 production lines (estimated ~60).
Public-verb test: exclude, see work status and the board say "person excluded required scope"; restore, both no longer say it. Mutation: read the run record's flag -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
