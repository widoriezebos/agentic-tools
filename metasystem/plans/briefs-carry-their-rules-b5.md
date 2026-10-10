# Brief: briefs-carry-their-rules, unit B5 (all build/revision callers, frozen inputs and sole check)

Working Mode: Implement
B2, B3 and B4 are committed on this branch (main with goal 2 merged in). Build unit B5 of plans/designs/briefs-carry-their-rules.md (accepted; read "Decided by m1e for Wido" 12:30 and its B5 acceptance item) and ONLY B5: every build and revision caller composes the unit's brief through the composer (new-round preparation and reviseLocked, internal/launch/unit_run.go ~379/~392, unit_revise.go ~107), with frozen inputs and the declared check as the sole Check; caller-supplied brief text is placed inside one delimited quote whose headings never parse as sections and labelled non-executable; an executable Check comes only from the declarations or the `--check --reason --by` route; only agent invocations are refused for a conflicting supplied Check, a proven person's `work build --brief` is never refused; `work build --brief` and `work revise --brief` keep a person's own brief usable as the quoted text; internal/protocol/templates/brief.md and public help updated.
Size: at most 250 production lines (the design estimates 240). If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's B5 test with the acceptance item: a person's brief with its own `# Check` section is retained quoted, not refused; an agent's conflicting Check is refused; both revise and build compose. Mutations: parse the quoted headings as sections; refuse the person.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
