# Brief: runs-advance-on-their-own, unit D4 (recorded decisions through immediate review/publication)

Working Mode: Implement
Branch goal/runs-advance-on-their-own-d4, stacked on goal 2's branch (ee8ba2f5c); it merges into goal/runs-advance-on-their-own (D3 is building there in parallel; stay out of the shared step entry D3 owns: runner.Continue and the work wait/build/revise run:RUN guard). Build unit D4 of plans/designs/runs-advance-on-their-own.md (accepted; read "Decided by m1e for Wido" 10:05 and its D4 acceptance item) and ONLY D4: the worker submits dispositions and brief through `work revise GOAL --work UNIT --after N --brief FILE --dispositions FILE` (cmd/metasystem/intent_selection.go:792, reaching internal/launch/unit_revise.go:71), never `work revise j2:<review>`; the driver only wakes the worker and observes the resulting revision; recorded decisions flow through immediate review and publication.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's D4 test with the corrected route; mutation: the design's, plus the driver calling revise itself -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
