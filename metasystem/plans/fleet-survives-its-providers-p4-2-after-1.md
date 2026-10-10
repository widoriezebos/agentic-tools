# Brief: fleet-survives-its-providers P4-2, correction 1

Working Mode: Implement
P4-2 is uncommitted in this worktree. Its read found one material defect; fix exactly this.

internal/steward/seat_start.go:520-522 (seatBrief, written for every steward-started seat at :591) always gives the stop-at-boundary / capture / poll / end paragraph, but internal/steward/unit_handoff.go:24-25 withholds binding when seat.driver=person, so a person-driver seat is told to stop and poll for a binding that never comes. The design: a person-driver session gets no automatic handoff binding, stop instruction or signal. Give that paragraph only when the same seat.driver read (config.ResolvePolicy, key seat.driver, the checkout's metasystem.conf) is not person; read it once in the caller and pass it in; a read error gives the paragraph out and shows the error in the brief's facts. Assert it in the person-driver case of TestFleetBoundaryPublicHandoff (the brief has no handoff paragraph). Mutation: always include it -> red.
Also restore plans/handoff-fleet-survives-its-providers-p4.md if your diff deletes it (git checkout it); never touch plans/.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
