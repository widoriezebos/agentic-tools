# Brief: review-chain-stops-and-records build-outcomes-b1, correction 1

Working Mode: Implement
build-outcomes-b1 is uncommitted in this worktree. One Opus read found four material defects; fix exactly these.

1. internal/launch/read_sequence.go:73-80: the per-step repository snapshot adds a Git call (`rev-parse --show-toplevel`) the shared unit fixture's strict Git stub does not expect: TestRoundCauseOnlyWhenNothingWasJudged (4 rows), TestUnknownReadDoesNotConsumeCorrectionRound (2 rows), TestReviewSubjectMissingReturnRetainsUnknownRound fail with "unexpected Git call" (green on 14fee1937); the fixture is used at 72 places. Update the fixture's expected calls, and the outdated rows of TestRoundCauseOnlyWhenNothingWasJudged (a red proof with an empty cause; process-lost) to the new cause vocabulary with a comment citing Decision 3. Then run ./internal/launch/ once (this unit changed its shared fixture).
2. internal/launch/unit_run.go:489, :519-522: the round's movement check still compares with the snapshot from before the first run, so after a tree move a passing retry ends proof-wrote/environment, and a real red on a stable tree is recorded environment instead of unclassified. Base the round's movement on the step-level checks (or retake the before-snapshot after a step retry). Test: the tree moves once, the retry passes, the round is green; a red on a stable tree is unclassified (mutation: the first-run snapshot, red).
3. internal/launch/unit_step_failure.go:84-86 and cmd/metasystem/intent_unit_questions.go:104 give every failed-step hold `work revise --after N --brief FILE --reason TEXT --by NAME`, which rebuilds. Give environment and deadline stops a person act that re-runs the retained step (retained inputs, fresh output path, no build); keep revise for unclassified. Test: follow the printed environment act, the step re-runs and no build launches (mutation: revise, red).
4. internal/launch/launch.go:426 maps context.DeadlineExceeded from child start to a deadline, but OSProcesses.StartChild (process.go:46) never returns it, so only the stub reaches it; launch.go:831 classes supervisor-ready-timeout as a deadline, which suppresses that environment failure's one retry. Remove both mappings; no deadline classification until a real step deadline exists (state that in the code comment).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
