# Brief: machinery-measures-its-own-process U5a, correction 1

Working Mode: Implement
U5a is uncommitted in this worktree. One Opus read found four material defects; fix exactly these and bring the unit within 250 production lines (it is at 263): share one publication-stamp helper between the two review paths, and drop the --work early-return edit (finding 4).

1. internal/launch/unit_named.go:571-597 GoalRuns fails the whole read when any run folder lacks run.json (newRunWithID creates the folder before run.json, unit_run.go:323-345), so `status G` and `goal status G` fail for every goal during any run start or after an interrupted start (reproduced: mkdir unit/unrelated-run). Skip a missing run.json as NamedWork does; show unreadable records of other goals as unknown, never a failure; same for ReadActs (internal/processchange/setting.go:179-187): one damaged act of any goal must not fail status. Test: an unrelated empty run folder and a damaged act of another goal; status G succeeds.
2. intent_unit_review.go:396-401 and intent_delivery.go:1249-1257 stamp PublishedAt with now on Confirmed or Unchanged, including a replay and a publish that reports `current`. Stamp only when this call pushed the read (published.State != "current"); otherwise the finish stays unavailable. Fix TestProcessPublicationPublicReview, which asserts the wrong case (its subject is already published and the fake returns current): it must leave the finish unavailable; add a case that pushes and stamps once.
3. internal/processmeasure/read.go:83-104: the goal-level finish covers only units with a run, so a goal with units still to start shows a finish. Leave the goal-level finish unavailable until the goal is done; keep per-unit finishes.
4. intent_selection.go:180, :224-235: the edit requiring --work for the early return regressed `status G` for a goal whose only work is one manual unit. Drop that edit (keep main's behaviour), keeping the costs.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
