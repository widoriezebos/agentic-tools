# Brief: runs-advance-on-their-own D3, correction 1

Working Mode: Implement
D3 is uncommitted in this worktree. One Opus read found two material defects (its probes: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/f77e7ebb-e1ab-4137-9935-206f37035290/scratchpad/d3/metasystem/cmd/metasystem/zz_probe*_test.go); fix exactly these.

1. The steward never advances a run: cmd/metasystem/steward_seat.go:202 DriveWork always calls Continue with ObserveOnly, and Advance returns right after collecting (internal/launch/unit_run.go, the observeOnly return), so under seat.driver=auto with no helm nothing advances; the design's D3 says auto admits the next step and its test says the next ready step starts. Under auto with no helm, advance one ready run (oldest first) without ObserveOnly; keep observe-only under person policy or the helm. Change TestDriverPublicStewardCollection to expect the step to start under auto and to stay unstarted under the helm; the tick.go:78 comment then holds.
2. The guard is not the one shared entry: continuationAllowed (internal/launch/tree_reservation.go:455) returns nil unless `continuing` is set, which only Continue sets (unit_named.go:91). Bypasses (reproduced): re-entry `work build GOAL UNIT --brief F` -> AdvancePrepared -> advanceNamedLocked -> bound.Advance(Resume) (unit_named.go:306) started the proof step under the helm; `work revise run:RUN --brief F --reason T --by Wido` -> runner.Revise -> continueRunning (unit_revise.go:284) started round 2's build under helm proof; traced: `work review run:RUN` -> RetryUnknownRead -> advanceStep (unit_stop.go:378). Apply the policy to every start of a pending step on an existing run (Advance with Resume, Revise, RetryUnknownRead), or drop the `continuing` condition for every start after round 1's first step. Add the work build re-entry and revise --by cases to the public test, each with a mutation.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
