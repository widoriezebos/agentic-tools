# Brief: review-chain-stops-and-records declared-check (first part), correction 1

Working Mode: Implement
declared-check's first part is uncommitted in this worktree. One Opus read found two material defects; fix exactly these, within 250 production lines for the part.

1. internal/launch/unit_check.go:47 and cmd/metasystem/intent_unit_check.go:63-64, reached from internal/launch/round_result.go:47-52: the publication gate replays `test run --unit-run RUN` with its Dir moved into the commit worktree (intent_unit_review.go:333-341), but UnitCheck.Run ignores the directory it starts in and runs in the frozen check.Directory (the round's goal worktree), so a moved branch or an amend publishes green on the old tree. Pass the gate's tree to the executor as the run directory (the frozen commands, deadline and environment stay; only the subject tree changes, and the record names it). Test: the gate replay runs the frozen check in the commit worktree's tree (a file only there is seen; mutation: frozen directory, red).
2. unit_check.go:41: Run first creates check-* under the round directory (~/.metasystem/unit/RUN/round-N; unit_run.go:1216), which a Codex builder under its default workspace-write sandbox (codex.go:100-108) cannot write, so the builder's own check fails with a permission error before it starts and nothing reports the difference. Write the builder's execution record where the builder can write (inside its worktree's ignored artifacts path, collected by the runner afterwards), or report the sandbox difference as an environment cause; test one run with the round directory unwritable.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
