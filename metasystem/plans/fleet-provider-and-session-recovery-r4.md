# Brief: fleet-provider-and-session-recovery, unit R4 with R3b (usage views; recovery request in machine list)

Working Mode: Implement
Branch goal/fleet-provider-and-session-recovery (R1, R2, R3 committed; R3 at 386cf4328). Build:
Item 0 (R3b, about 35 lines): `machine list` shows each provider recovery request's effective interval, due time and "pending success" state as the design's R3 Decision states (the read of R3 found it unbuilt); a test through the public verb.
Then unit R4 of plans/designs/fleet-provider-and-session-recovery.md (accepted): "R4 — Display measured usage with honest account coverage", its Units row (internal/launch/, internal/hostcapacity/, internal/ui/httpd/board.go, Fleet frontend, machine list), Estimates, the five questions, the four defect classes; "Decided by m1e for Wido" binds. If the Fleet frontend changes, regenerate the UI bundle only if `npm run bundle` succeeds; if its npm audit refuses (known: KaTeX and source-map-js advisories await Wido's dependency decision), leave the frontend part unbuilt and report it rather than bypassing the audit.
Size: at most 250 production lines for R4 plus item 0 (R4 estimate 250): if it will not fit, build item 0 and the largest usable R4 part (machine list --json and /api/board) and report the rest.
Public-verb test: TestFleetUsagePublicViews as the design states (machine list --json and GET /api/board agree on counts, token categories, peak and coverage; duplicate cumulative resume records count once; an active 24-hour session ...), mutations as the design states.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
