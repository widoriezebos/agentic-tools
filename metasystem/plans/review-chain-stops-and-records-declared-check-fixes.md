# Brief: review-chain-stops-and-records declared-check-fixes (fix unit after the stop)

Working Mode: Implement
declared-check part 1 is committed and stopped with two material findings; fix them.

F-1. cmd/metasystem/intent_unit_check.go:82 runs plan.Check.Run(inv.cwd, ...); the Codex builder starts in the worktree root (internal/launch/unit_run.go:317, :492) while the runner and gate run in check.Directory, which is <worktree>/metasystem (intent_work.go:988, worktreeFolderHere), so `go test ./cmd/metasystem` fails "go.mod file not found" for every builder while the proof passes; the bed's folder equals its worktree (intent_unit_check_test.go:395) so no test saw it. Re-root check.Directory relative to plan.Worktree onto the top level of the calling tree (never the raw cwd). Test: the folder is a subdirectory and the builder runs from the worktree root: same directory as the proof (mutation: raw cwd, red).
F-2. Run writes both the runner's record and the builder's into record.Worktree/artifacts/unit-checks/..., which the builder can write; CollectLaunch (intent_work.go:495-503) copies every round's check-*/result.json on every collection, overwriting the round-directory copies without marking the source, so a later builder can replace an earlier round's runner evidence. The runner's proof writes its record into the round directory only; collect only the builder's records, only for the round that just ended, under a builder-only name, never overwriting. Test: a round-2 builder rewrites round 1's worktree record; round 1's retained runner record is unchanged (mutation: collect every round, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
