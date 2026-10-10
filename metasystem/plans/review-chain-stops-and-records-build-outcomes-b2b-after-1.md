# Brief: review-chain-stops-and-records build-outcomes-b2b, correction 1

Working Mode: Implement
b2b is uncommitted in this worktree. One Opus read found two material defects; fix exactly these, plus one migration case.

F-1. internal/launch/read_sequence.go:97-102, unit_accounting.go:18-23, unit_run.go:649-653: the reservation is written before Manager.Start; when Start refuses before creating a launch record (launch check, no supervisor, kind unavailable) the step is marked failed, but saving runs collection, which errors on the missing record outside StepStarting, so the failure is never saved, the state stays StepStarting, every Advance rejoins the same pending reservation (120 open minutes, 1 active job) and fails, and a rejoin after a revision or cap change is refused (internal/dispatch/unit_launch.go:22). When Start returns no record, settle the reservation terminal-not-executed with no charge; collection accepts a missing record for a failed step that has no launch. Test (the critic's: Supervisor nil, `work build` twice): the step fails and is saved, the reservation settles with no charge, a later advance proceeds (mutation: leave it pending, red).
F-2. internal/dispatch/unit_launch.go:11-32 and cmd/metasystem/intent_work.go:585-620: ReserveUnitLaunch reserves unconditionally; other dispatch paths check ProjectBudget against ReservedJobMinutesLimit, AttemptLimit, ActiveJobLimit and live breaches (internal/dispatch/admission.go:316-350, governed.go:181-214). Inside the record lock, refuse when the goal's budget is unknown, breached, or the cap exceeds the remaining reserved minutes, with the person's `metasystem goal budget` remedy (a person's budget act takes effect; never refuse a person for anything but damage). Test: a breached goal refuses the unit reservation and launches nothing; after the person's budget act it proceeds (mutation: no check, red).
Migration: a launch with no reservation (runs started before b2b, proof launches that were never gated) is reconciled as unaccounted (recorded, no charge, no error), so collection never fails on it. Test: a pre-b2b run advances.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
