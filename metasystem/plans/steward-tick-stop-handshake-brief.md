Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3
Date: 2026-09-01

# Goal

TestRunLoopTicksUntilTheStopFile (metasystem/internal/steward/runner_test.go)
stops and drains its RunLoop goroutine on EVERY exit path, including
assertion failures, so a failing run can never leak the live loop goroutine
into TempDir teardown. This is goal steward-tick-stop-on-failure: the landed
test writes the runner stop file only on the happy path; the port below
closes that gap with a cleanup handshake. The patience logic already in the
test (progress-based wait, overall deadline) is correct and MUST NOT change.

# Workspace

The delegate worktree the dispatcher created for this job. Touch exactly one
file: metasystem/internal/steward/runner_test.go, and inside it exactly one
test function: TestRunLoopTicksUntilTheStopFile. Nothing else — not the other
tests in the file, not the steward package code, never plans/.

# Inputs

The current test (as landed, metasystem/internal/steward/runner_test.go
lines 35-81): launches RunLoop in a goroutine sending its error into
`done := make(chan error, 1)`, waits for two ticks with progress-based
patience, writes the stop file, receives from `done`, and asserts the runner
record is removed.

The reference fix, built and certified on another machine against an OLDER
version of this test (commit 97336c30 on branch machine/m0, chain
steward-tick-patience-handshake, MECHANICAL, closed). Its handshake portion,
which is what you port — the patience portions of that diff are already
superseded by the landed test and must NOT be re-applied:

```go
	done := make(chan error, 1)
	go func() {
		done <- RunLoop(root, census, nil, 50*time.Millisecond)
		close(done)
	}()
	t.Cleanup(func() {
		// The loop must be stopped and drained before its checkout is torn down.
		if err := os.WriteFile(runnerStopPath(root), []byte("stop\n"), 0o644); err != nil && !os.IsExist(err) {
			t.Errorf("stop RunLoop during cleanup: %v", err)
		}
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Errorf("RunLoop did not exit after stop: checkout %s", root)
		}
	})
```

The port, mechanically: (1) add `close(done)` after the RunLoop send inside
the launch goroutine; (2) register exactly the t.Cleanup block above
immediately after the goroutine launch, before the first wait. The happy
path still receives the error value from `done` at the existing select; the
`close(done)` makes the cleanup's drain return immediately in that case, and
makes it receive the zero value promptly when an assertion failed after
RunLoop already exited. Cleanup runs before TempDir removal by LIFO order,
which is the point.

# Constraints

Non-goals: any change to the patience/deadline logic, any change to
TestSecondRunnerRefusesBesideALiveOne or any other test, any change to
non-test code. Do not reformat untouched lines. Wall-clock budget: this is a
single small edit plus a focused test run; stop and report if you are past
30 minutes.

# Expected Return

Version-2 implementer JSON with schemaVersion, jobId, round, runtime,
sessionId, model, evidence, gaps, mode, riskiestPart, diffBoundary,
whatWasDone, claimed. diffBoundary lists exactly
metasystem/internal/steward/runner_test.go. Evidence commands are each
replayable verbatim from the worktree's metasystem/ directory:

- `go test ./internal/steward/ -run TestRunLoopTicksUntilTheStopFile -race -count=20` — level ran, observed PASS. Run this WITHOUT any artificial CPU load; the loaded-profile proof is run by the orchestrator outside your sandbox afterward.
- `go vet ./internal/steward/` — level ran, observed clean.

# Acceptance Criteria

- The launch goroutine closes `done` after sending RunLoop's result.
- A t.Cleanup registered after the launch writes the runner stop file and
  drains `done` with a 30-second failsafe, exactly the reference block.
- The focused test passes 20/20 under -race with no goroutine-leak,
  double-close, or double-receive failure.
- `git diff --stat` in the worktree shows exactly one changed file with a
  handful of added lines and no deletions outside that test function.

# Gap Rule

stop and report a gap; never fill it silently.
