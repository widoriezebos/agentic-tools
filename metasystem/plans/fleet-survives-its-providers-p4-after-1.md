# Brief: fleet-survives-its-providers P4 (first part), correction 1

Working Mode: Implement
P4's first part is uncommitted in this worktree. One Opus read found three material defects; fix exactly these.

1. internal/steward/runner.go:369-376 (with cmd/metasystem/steward_seat.go:191): when the boundary event write fails the loop does `continue`, skipping the engine re-arm, the landing-lane keeper, the seat start on revive and the revival resume on every pass; lease.CurrentHolder returns ErrLeaseAbsent with no lease file (lease.go:86) and a holder without a matching announcement hits "unit boundary session is unknown" (unit_boundary.go:62), so a dead or unarmed seat is never restarted. Fix: report the failure and still run the later steps; an absent lease or session is "no event"; only the end-session step waits on the event.
2. internal/steward/seat_idle.go:91-94: with AtBoundary any `running` record counts as busy; before, a record whose supervisor died was an orphan and did not block. SeatAtUnitBoundary is also the engine re-arm check (cmd/metasystem/steward_verbs.go:401), so after a builder crash the engine never re-arms. Fix: drop the new branch; to hold starting or capacity-held steps check the step state, not the stale record.
3. internal/steward/unit_boundary.go:34 (read at cmd/metasystem/context_verbs.go:116): every past entry is checked against today's canonicalPath(root) and must be complete, so one bad or foreign entry makes `session handoff --status` exit with an error and feeds 1 on every pass. Fix: on status show the error beside the existing reading; skip or report entries of another seat.
Test gap behind 1: both runner sides are faked (cmd/metasystem/fleet_boundary_test.go:124-129, internal/steward/fleet_boundary_runner_test.go:381). Add one test running the real observer inside the real loop with the lease absent: revive still happens. Mutations: `continue` on the event error; the running-record branch back; strict entry check on status.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
