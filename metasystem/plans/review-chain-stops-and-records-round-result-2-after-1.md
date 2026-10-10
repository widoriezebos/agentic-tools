# Brief: review-chain-stops-and-records round-result-2, correction 1

Working Mode: Implement
round-result part 2 is uncommitted in this worktree. One Opus read found two material defects; fix exactly these.

1. internal/launch/tree_reservation.go:78 with :304-314: recoverTree reads every run in the shared unit root across all goals and errors on any unreadable run.json or any run whose worktree no longer resolves (filepath.EvalSymlinks), and :78 starts the scan when the ownership record is merely missing (ErrNotExist), so one retained run with a removed builder worktree makes a person's recovery and any ordinary stop with no record fail forever. Skip runs whose worktree is gone or whose run.json is unreadable (they cannot own this tree), limit the scan to runs naming this tree, and start recovery only for an unreadable or damaged record, never a missing one. Test: an unrelated run with a deleted worktree and another with a corrupt run.json; the person's recovery and an ordinary `work stop run:RUN` with no record both succeed (mutation: error on unrelated runs, red).
2. cmd/metasystem/intent_tree.go:101: with a damaged record every writer gets "the worktree ownership cannot be read; nothing was started" with the same command as next step, which repeats forever and never names the recovery. For a damaged record print the person's `work stop run:<run>` with the run found from the run records naming this tree, or a person request when no run can be named. Test: damaged record, `work build --work V` prints the exact stop command; following it as a person releases and V proceeds.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
