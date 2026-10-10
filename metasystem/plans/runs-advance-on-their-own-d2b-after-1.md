# Brief: runs-advance-on-their-own, unit D2b, correction 1

Working Mode: Implement
The uncommitted D2b build is in this worktree; keep it. The read found one BREAKING finding (defect classes 1 and 4):
After a supervisor hold writes Reason "authority-held: ..." (internal/launch/launch.go:476), ResumePending (pending_recovery.go:30-38) changes only AdapterData, so the old reason stays. On re-entry startSupervisor starts the production supervisor; OSSupervisorStarter returns when the process starts, before it claims the launch (process.go:172-173); the first poll sees no supervisor plus the old reason (launch.go:322) and returns UNIT_LAUNCH_HELD, so `work build run:X` reports held and exits; the detached supervisor then claims and clears the reason (launch.go:380) and probes os.Getppid() for person authority (launch_verbs.go:224) after the parent may have exited: under seat.driver=person it holds again (the printed remedy can never succeed), under auto with approval it starts the child while the person was told "held". Tests miss it because cancelRecoveryStarter runs the supervisor synchronously.
Fix by subtraction: clear the hold reason when the reservation is resumed, before the supervisor starts; and carry the invocation's person proof into the supervisor explicitly (recorded with the resume, invocation-scoped) instead of probing the parent process later. Add a test whose starter returns BEFORE the supervisor claims the launch (asynchronous, injected ordering, no wall-clock waits): a person's recovery after a hold starts exactly one child and reports started, not held; an agent's stays held. Mutation: keep the old reason -> red.
Run the changed tests by name plus TestDriverPendingRecoveryPublicStop; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
