Working Mode: implement
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal steward-tick-load-flake)
Date: 2026-08-31

# Goal

TestRunLoopTicksUntilTheStopFile (internal/steward/runner_test.go) no
longer fails under machine load. Its two waits assume wall-clock
patience — a fixed 5-second deadline for two 50-millisecond ticks, and a
fixed 3-second wait for loop exit — and CPU starvation blows both while
the loop is healthy (three specimens: m2's gate 2026-08-29, m3's replay
2026-08-31, m2's gate twice 2026-08-31). The fix is the suite-custody
pattern the goal text prescribes: patience anchored to observed
progress evidence, never to a fixed wall-clock allowance.

# Workspace

The dispatch-created job worktree, branched from main. May touch EXACTLY
one file: internal/steward/runner_test.go (inside the metasystem/
project). Nothing else — no production code, no other tests, no files
outside the project.

# Inputs

- internal/steward/runner_test.go, TestRunLoopTicksUntilTheStopFile
  (lines ~35-68): phase 1 polls LoadEvidence until TicksSinceAdvance >= 2
  under a time.Now()-based 5-second deadline; phase 2 selects on the
  done channel against a 3-second timer.
- internal/proofrun/watchdog.go — the repository's precedent: patience
  measured against output growth (progress), not elapsed wall time.
- plans/goals/steward-tick-load-flake.md — the prescribing intent.

Mechanical rules — these leave no judgment calls:

PHASE 1 (waiting for two ticks). Replace the fixed 5-second deadline
with progress-based patience:
- Keep the success condition exactly: TicksSinceAdvance >= 2.
- Poll LoadEvidence on the existing 20-millisecond interval.
- Track the last time the loaded evidence value changed (compare the
  whole evidence value, not only TicksSinceAdvance).
- Fail with the existing message only when the evidence has not changed
  for 10 consecutive seconds (a stalled loop), or when the overall bound
  below expires.
- Overall bound: the test's own deadline when available (t.Deadline()
  minus a 5-second safety margin); when the test has no deadline,
  120 seconds. This bound is the fail-stop, not the patience.

PHASE 2 (stop-file exit). Replace the fixed 3-second select timeout with
the same overall bound (test deadline minus the 5-second margin, or
120 seconds without one). Keep the existing failure message and the
existing success assertions: the loop exits nil and the runner record
file is gone.

No other behavior of the test changes. Helper code, if any, stays local
to this test file.

# Constraints

- Non-goals: no production code changes; no changes to any other test
  (TestRunLoopAttemptsRevivalBeforeNotifyingItsFailure and the rest stay
  byte-identical); no loosening of any assertion — the test must still
  prove two ticks, a clean exit, and record removal.
- KNOWN SANDBOX LIMIT: runner fixtures do not run in the delegate
  sandbox (your round on the previous goal recorded "no goal ledger"
  failures for this family). Do NOT treat a sandbox-environment red of
  this test as a gap: report it as environment-limited evidence. The
  orchestrator replays the decisive runs — focused, repeated, and under
  artificial load — outside the sandbox.
- Wall-clock budget: 20 minutes.

# Expected Return

Version-2 implementer JSON with all required fields. `diffBoundary`
lists exactly one path, WITH the repository prefix, verbatim:

- metasystem/internal/steward/runner_test.go

Evidence entries keep the `{command, observed, level}` schema; each
command replayable verbatim from the worktree root, including:

- `gofmt -l internal/steward/` (run from the metasystem/ directory;
  empty output expected)
- `go vet ./internal/steward/`
- `go build ./...`
- `go test ./internal/steward/ -run TestRunLoopTicksUntilTheStopFile -count=1`
  (attempt it; an environment-limited red is reportable evidence, not a
  gap)

# Acceptance Criteria

1. The diff touches exactly metasystem/internal/steward/runner_test.go.
2. gofmt reports nothing; go vet and go build pass in the worktree.
3. The test contains no fixed wall-clock patience: no bare 5-second or
   3-second allowance decides failure; failure requires either 10
   seconds without evidence change or the overall test-deadline bound.
4. All existing assertions remain: two ticks observed, stop file ends
   the loop with a nil error, the runner record is removed.
5. The orchestrator's replay (outside the sandbox) passes focused,
   repeated (-count=20), and under artificial CPU load.

# Gap Rule

stop and report a gap; never fill it silently.
