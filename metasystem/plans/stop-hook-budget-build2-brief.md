Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Goal

Re-issue the closed, reviewed slice-1 chain of goal
stop-hook-budget-is-ours onto today's main. Chain shbo-build1-20260906
(three rounds, two reviews, closed with zero material findings; its
dispositions are in
metasystem/plans/dispositions/stop-hook-budget-code-critique-r1.md and
metasystem/plans/dispositions/stop-hook-budget-code-critique-r2.md) no
longer applies to main: the engine-rearm landing (commit ea8c3ead)
added metasystem/cmd/metasystem/steward_verbs_test.go, and the chain
created a file of that name. A closed chain cannot take a follow-up, so
this is a new chain whose round 1 is that certified change, byte for
byte where main allows it, reconciled where it does not. No new
behaviour. The change itself is described in
metasystem/plans/stop-hook-budget-build-brief.md,
metasystem/plans/stop-hook-budget-fix-brief.md and
metasystem/plans/stop-hook-budget-fold2-brief.md; read them so you know
what the patch does.

# Workspace

The delegate worktree the dispatcher created for this job, at main.

# The steps

1. The certified diff of the old chain's round 3 is the file
   /Users/wido/LocalStorage/GitHub/agentic-tools-m1d/metasystem/artifacts/agents/shbo-build1-20260906/rounds/3/diff.patch
   (40,086 bytes; its paths are relative to the metasystem directory).
   Apply it with `git apply --directory=metasystem --reject` or by
   hand; every hunk must land except the creation of
   metasystem/cmd/metasystem/steward_verbs_test.go, which collides with
   main's file of that name.
2. Take the test function(s) from the patch's version of that file and
   add them to main's metasystem/cmd/metasystem/steward_verbs_test.go,
   keeping main's tests untouched.
3. If a hunk in metasystem/internal/steward/health_test.go or
   metasystem/internal/steward/ledgerattention_test.go no longer fits
   (main touched both today), reconcile so the tree compiles and the
   tests pass without changing the behaviour the patch carries.
4. Confirm by reading that the result equals the patch everywhere else:
   the three registrations at sixty, the deadline parent's fifty-seven,
   the elapsed measurement on every emission path with the exit-2
   retry, the pointer fields, the fixture assertions, the two project.go
   comments.

# Gate

`cd metasystem && GOTOOLCHAIN=go1.26.5 go build ./... && GOTOOLCHAIN=go1.26.5 go vet ./... && gofmt -l .` (empty);
`GOTOOLCHAIN=go1.26.5 go test ./internal/steward/ ./cmd/metasystem/ -count=1` green
(the host moved to Go 1.27.1 today and the pinned static checker cannot
read it; the toolchain variable is the workaround until the go.mod pin
lands);
`bash -n scripts/agents/supervision-hook.sh scripts/agents/supervision-hook-fixtures.sh`;
`bash scripts/agents/supervision-hook-fixtures.sh` as far as the sandbox
allows (the deadline scenario needs process inspection; the seat replays
the suite outside before landing).

# Constraints

Wall-clock budget: 30 minutes. Declare the boundary as every file that
differs from main. Gap rule: stop and report a gap; a hunk that cannot
be reconciled without a behaviour choice is a gap.

# Expected Return

Version-2 implementer JSON as the role schema requires.

# Gap Rule

stop and report a gap; never fill it silently.
