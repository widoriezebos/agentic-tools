# Brief: machinery-measures-its-own-process U3 (first part), correction 1

Working Mode: Implement
U3's first part (drift stops with bands) is uncommitted in this worktree. One Opus read found two material defects; fix exactly these.

1. internal/processmeasure/decide.go:39-41 and read.go:52: the integration band compares IntegrationReds with IntegrationBaseline, and no production code sets either, so the band is always unknown and the missing-baseline test (intent_process_drift_test.go:79, :132) passes under its mutation (nil limit -> zero). Remove the integration band and both fields until an integration-result producer exists (record it in the handoff as moved), and drop the test's integration assertions; keep elapsed and suite-minute bands.
2. A new goal episode opens a second stop while the first stands: processDrift (cmd/metasystem/intent_process_drift.go:13-18) sums every retained run of the unit across episodes and read.go measures elapsed from the earliest build; after release and a fresh claim (internal/goal/verbs.go:473 bindClaim) Observe reuses a stop only with the same episode (internal/processchange/drift.go:78) and writes a new one (:85). Count only the current episode's runs (Decision 3 line 151: old elapsed must not block a new episode), and reuse an unresolved stop for the same goal and unit if one stands. Test through the public verb: release, claim again, collect: no second stop. Mutation: sum all episodes -> red.
Also: intent_process_drift.go:13 must not ignore GoalRuns' error; show it as unknown on status.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
