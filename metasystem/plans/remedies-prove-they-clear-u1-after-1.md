# Brief: remedies-prove-they-clear U1, correction 1

Working Mode: Implement
U1 is uncommitted in this worktree. Its read found one material defect; fix exactly this.

cmd/metasystem/intent_process.go:1697 now marks MAIN and DELEGATE readers as agent, but Render (internal/steward/remedy.go:14-24) rewrites only system start, session start and goal release, so budget-missing, budget-malformed and budget-breach still print `metasystem goal budget G BOX` to an agent (remedy.go:44-46), a person's verb (cmd/metasystem/intent.go:205), against the design's U1 rule "an Act is a public verb of the actor who reads it". Render looks up the act's audience in the intent table: when the verb is a person's and the reader is an agent, the agent gets a plain line ("a person runs: metasystem goal budget G BOX") instead of a command to run. The static test (remedy_test.go:107) walks every cause for both readers against the intent table instead of a hard-coded two-verb list; add one test running `system check` as an agent through the command. Mutations: print the person's verb as the agent's act; classify every reader as human.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
