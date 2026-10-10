# Brief: fleet-survives-its-providers P5b, correction 1

Working Mode: Implement
P5b (current-load build admission) is uncommitted in this worktree. One Opus read found three material defects; fix exactly these.

1. cmd/metasystem/intent_work.go:445 stores any policyParams("host.builds") error as BuildPolicyError and internal/launch/hostcapacity.go:56 then refuses every agent build (LAUNCH_BUILD_CAPACITY); in a bed without a git repository TestDesignGateRefusesOnlyWhenSwitchedOn, TestDesignGateNeverStallsWhenItBreaksUnderRefuse and TestAllowBuildWithoutDesignShowsAndRecordsItsImpact now fail "no git repository contains ...". An unreadable seat-policy layer falls back to config.Get (conf, then default); only an unreadable conf itself holds agents. Those three tests pass unchanged.
2. intent_work.go:721-722 changed the design gate's `person` from "no lineage" to directPersonProof, outside P5's scope (R-146-m1k gate refusals change; TestDesignGateRefusesOnlyWhenSwitchedOn/person now returns BUILD_DESIGN_NOT_ACCEPTED). Keep the design gate's old person rule; use runner.Actor only for build admission.
3. unitRunner() sets Actor only for `work build` (intent_work.go:447), so builds launched from `work revise` (intent_selection.go:795 -> Revise -> advanceRunning -> unit_run.go:393) and `work revise run:` (intent_work.go:1439) carry an empty actor and a person's correction build is refused for load, cap or person policy, with a remedy (intent_work.go:1244) that would be refused again. Set the actor from directPersonProof for the revise verbs too. Add a public-verb row: a person's `work revise` over host.load-max starts its build; an agent's is refused. Mutation: empty actor on revise -> red.
Also (not material, cheap): an agent `work build` under a grant appends a false "refused act=work build" line to the attorney log (intent_work.go:447 -> :2523); probe person proof without writing that log.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
