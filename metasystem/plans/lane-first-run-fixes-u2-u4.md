# Brief: lane-first-run-fixes, units 2-4: status tells the truth, remedies that succeed, steward start on an event

Working Mode: Implement
Branch fix/lane-status-remedies-20261009 at main 03762942b. Goal lane-first-run-fixes (plans/lane-first-run-fixes-design-brief.md units 2, 3, 4); evidence agentic-tools-evidence/lane-test-20261009/findings.md items 1, 2, 4, 5, 13. Three smallest changes:
2. `landing status` (internal/landing/plain/status.go and the cmd verb): after a trunk proof that ended red, line 1 names it ("main <sha> proven red: <first failed units>; ...") with the incident and the next act (the skill's recovery: hot-fix, then `landing prove --trunk`), instead of "The landing lane is idle". While a proof runs: units done/total and minutes elapsed (the proof log carries `landing package` lines; read them). Test both through the public verb; mutation -> red.
3. Hand-in remedies (cmd/metasystem work land): (a) "origin has no goal/G to land" -> the remedy is `git push origin goal/G` (or work land pushes the branch itself if a seat-side owner already does that for seats; choose the smallest); (b) "commit X has 2 parents, and a goal branch commit has one" -> the remedy names `metasystem work rebase G`. Tests through the verb; mutation -> red.
4. Deploy: `system restart`/`system start` waited a fixed 10s for the steward ("the new steward (pid N) did not finish starting on install 28 within 10s") and failed under load ~8, then succeeded on a second start: the wait is on the steward's observed start (its announcement record / readiness event) bounded only by the caller's deadline, no fixed 10s cap; test with an injected clock. No flaky behaviour on any machine (Wido).
Size: at most 250 production lines in total. Do not commit. Never open metasystem.conf.local. Report every exit.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
