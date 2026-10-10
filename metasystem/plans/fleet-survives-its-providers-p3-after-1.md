# Brief: fleet-survives-its-providers P3 (first part, the pause), correction 1

Working Mode: Implement
P3's first part (the pause) is uncommitted in this worktree. Its read found five material findings; findings 1 and 5 (stale expiry, its once-per-mark alert, the person's clear, and their test assertions) are the unbuilt remainder and go to the next unit P3b. Fix exactly these three now:

2. internal/steward/tick.go:549 sets ev.Age but only evidence.go:32 and the test read it; the steward decides on TicksSinceAdvance, which pauses only when a whole sample falls inside an outage, so a mostly-outage tick counts in full. Make the staleness decision use the paused Age (or pause the tick counter by the overlapping fraction); test with partly overlapping samples through the public tick. Mutation: decide on the unpaused counter -> red.
3. internal/dispatch/budget_provider.go:57-60 treats a goal as depending on a provider from a job's start until the next job or now, ignoring the job's recorded end; a goal whose job completed and now waits on a person is paused by a later outage (for days with a distant reset), so a budget can be overrun. The dependency ends at the job's recorded end unless the job ended on a provider failure. Test: a completed job, then an outage: no pause. Mutation: ignore the end -> red.
4. internal/dispatch/budget.go:835 returns unknown whenever there is any provider wait and IdleSeconds > 0, and admission (admission.go:149) then refuses the goal for the rest of its claim with no remedy. All idle time falls before the current claim began: count provider waits only from the current claim's start. Test: idle seconds plus a provider wait in the current claim -> a known budget with the wait subtracted once. Mutation: unknown on any idle -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
