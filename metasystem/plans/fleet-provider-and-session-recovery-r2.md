# Brief: fleet-provider-and-session-recovery, unit R2 (person revival and shared automatic recovery record)

Working Mode: Implement
Branch goal/fleet-provider-and-session-recovery (R1 committed at 16865e0ca). Build unit R2 of plans/designs/fleet-provider-and-session-recovery.md (accepted): "R2 — Give a person one recorded act to revive an existing seat" in Decisions, its Units row (cmd/metasystem/intent_{process,machine}.go, cmd/metasystem/steward_seat.go, internal/steward/seat_start.go), Estimates, the five questions, the four defect classes; "Decided by m1e for Wido" binds. Goal 3's restart remedy already names `machine revive` as unavailable until this unit: make that remedy real. Stay out of R3 (recovery request/declaration) and R4 (usage views).
Item 0 (fix forward from R1's read): in internal/steward/fleet_recovery_test.go, the "human" case of TestFleetRearmPreservesContinuation mints Generation++ with MintedBy "human-terminal", so the intent is refused by the `installed.MintedBy == "machine-rebuild"` clause and the witness comparison at dispatch_authorization.go:46-49 is never exercised (mutating it to `true &&` stays green). Follow the human mint with a machine-rebuild mint that keeps the new witness and assert the old intent is refused; verify that mutation now goes red.
Size: at most 250 production lines (estimate 250); if it will not fit, build the largest usable first part and report the rest.
Public-verb test: TestMachineRevivePublicAct as the design states (printed `machine revive SEAT --after L` with person proof: impact statement, one real launch/result, policy and automatic restart history unchanged; omitting --after behaves as the design states); mutations as the design states.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
