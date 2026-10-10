# Brief: fleet-provider-and-session-recovery, unit R3, correction 1

Working Mode: Implement
The uncommitted R3 build is in this worktree; keep it. The read found (the `machine list` display, F-1, moves to unit R3b; do not build it here):
F-2: internal/outage/providers.go Observe returns an error before writing the mark when provider.recovery-alert-after is invalid (e.g. "soon") or config.Get fails; callers internal/adapter/adjudicate.go:288 (logs) and internal/missionrunner/loop.go:2104 continue without a stored mark, so the provider-limit hold is never written and seats relaunch into the limit. Fix by subtraction: always record the mark; leave the interval empty so RecoveryDue reports a source obligation. Test with an invalid value: the mark is stored; mutation -> red.
F-3: internal/steward/recovery_request.go:44-46 returns on the first seat's error (no terminal observation :113; unreadable episode/failed-launch time :141; a closed interval from before this change without recoveryAfter :146-148), so later seats are never asked or closed. Per the design an absent field is a source obligation, never an error that stops the pass: continue per seat, report the obligation for that seat. Test two seats where the first has an old interval: the second is still asked; mutation -> red.
F-4: internal/steward/seat_start.go:633-643 refuses with "inspect metasystem machine list", which shows no recovery requests; name the act that works (plain `metasystem machine revive SEAT`, without --after, or the exact question to answer). Test the remedy succeeds when followed.
Run the changed tests by name plus TestProviderRecoveryAskPublicLifecycle and TestMachineRevivePublicAct; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
