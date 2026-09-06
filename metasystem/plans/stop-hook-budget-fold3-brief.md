Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Goal

Round 4 of slice 1 of goal stop-hook-budget-is-ours (chain
shbo-build1-20260906): a rebase round, no new behaviour. The closing
review found nothing material (its dispositions are in
metasystem/plans/dispositions/stop-hook-budget-code-critique-r2.md), but
main has moved since your worktree's base: the engine-rearm landing
(commit ea8c3ead) added metasystem/cmd/metasystem/steward_verbs_test.go,
and your round 3 created a file of the same name, so the certified diff
no longer applies to main. Bring the worktree to main and reconcile
that one file; everything else from rounds 1 to 3 stays byte-for-byte.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

# The steps

1. Your untracked test file blocks the merge: rename it aside (any
   name outside the repository, or a name ending in .orig that you
   delete before returning), then `git merge main` (the merge touches
   nothing you edited except tests main also touched; resolve any
   conflict in favour of keeping both behaviours).
2. Move your test function(s) from the set-aside file into main's
   metasystem/cmd/metasystem/steward_verbs_test.go, keeping main's
   tests untouched; delete the set-aside file.
3. If the merge brought a changed signature or helper into a file you
   edited (metasystem/internal/steward/health_test.go and
   metasystem/internal/steward/ledgerattention_test.go were both
   touched on main today), make the tree compile and the tests pass
   without changing any behaviour of rounds 1 to 3.

# Gate

`cd metasystem && GOTOOLCHAIN=go1.26.5 go build ./... && GOTOOLCHAIN=go1.26.5 go vet ./... && gofmt -l .` (empty);
`GOTOOLCHAIN=go1.26.5 go test ./internal/steward/ ./cmd/metasystem/ -count=1` green
(the host moved to Go 1.27.1 today and the pinned static checker cannot
read it; the toolchain variable is the workaround until the go.mod pin
lands);
`bash -n scripts/agents/supervision-hook.sh scripts/agents/supervision-hook-fixtures.sh`;
`bash scripts/agents/supervision-hook-fixtures.sh` as far as the sandbox
allows (the seat replays it outside).

# Constraints

Wall-clock budget: 30 minutes. Declare the boundary as every file that
differs from main after the merge. Gap rule: stop and report a gap.

# Expected Return

Version-2 implementer JSON as before.

# Gap Rule

stop and report a gap; never fill it silently.
