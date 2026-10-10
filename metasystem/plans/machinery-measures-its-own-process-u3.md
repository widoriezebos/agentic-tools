# Brief: machinery-measures-its-own-process, unit U3 (drift uses the stop vocabulary)

Working Mode: Implement
U1a, U1b(+fixes), U2 and U5a(+fixes) are committed on this branch, and goal 2's branch (review-chain-stops-and-records, its stop vocabulary: internal/loopstop, the unit stop record and asks) is merged in (960f02ea3). Build Decision 3 of plans/designs/machinery-measures-its-own-process.md and ONLY that, including the acceptance items folded at 22:55 that name U3 and the 2026-10-08 decision choosing twice the frozen estimate:
the drift stops persisted beside ProcessAct (<target-state-root>/process/episodes/<goal-episode>/stops/<stop-id>.json), opened through the stop vocabulary when a measure leaves its band (the table: unit elapsed above twice its frozen estimate, equality in band; integration-first red count above the goal's declared baseline, a missing baseline is unknown, never zero; instrumented suite minutes above the declared check's allowance, a unit's full-suite allowance is zero); one stop, no automatic process-change attempts while it stands; the first act printed for a process-change stop is the exact reverse command with impact and before/after; reversion as a new act through the same writer: `settings set KEY BEFORE --repo CHECKOUT --undo ID`, and `settings unset KEY --repo CHECKOUT --undo ID` only for an act whose before-layer was absent; an undone change resolves the episode's intervention hold ("resolved by removal") without declaring the cumulative measure in band.
The U4 repeat windows (process/windows/...) are NOT in this unit.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (the drift stop with its bands first, then undo/unset) and report the rest.
Public-verb test: a unit whose measured elapsed exceeds twice its frozen estimate opens one drift stop shown in `goal status G` with its reverse command; following `settings set ... --undo ID` records a new act and resolves the hold; equality stays in band; a missing baseline shows unknown. Mutations: treat a missing baseline as zero; open a second stop while one stands.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
