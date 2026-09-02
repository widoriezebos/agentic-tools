Working Mode: build
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fable-5-1-model-rollover)
Date: 2026-09-02

# Goal

Execute section 6 (build plan) of metasystem/plans/fable-5-1-rollover-design.md,
certified by Sol (metasystem/records/misc/fable-5-1-rollover-critique-r2.md,
zero findings). Zero judgment calls: the design decided everything.

# Workspace

The delegate worktree the dispatcher created for this job.

# The change

1. metasystem/internal/dispatch/composition_test.go: in
   TestHazardConfigurationAcceptsConfiguredMaximalModel the two
   "claude-fable-5" literals (the requested Model and the record.Model
   assertion, lines 256 and 262 on the base tree) become "claude-fable-5-1".
   Nothing else in the file changes.
2. metasystem/memory/rulings.md: append the R-46-m0b row exactly as written in
   section 4 of the design, as the last row of the rulings table. Do not touch
   R-25-m1 or any other row.
3. metasystem/metasystem.conf: no change.

# Gate

`go test ./internal/dispatch/ -run TestHazardConfiguration -count=1` green.
Then `go vet ./internal/dispatch/` and `gofmt -l internal/dispatch/` empty.

# Constraints

Wall-clock budget: 15 minutes. MECHANICAL reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the two files in the change.

# Gap Rule

stop and report a gap; never fill it silently.
