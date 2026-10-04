# Brief: design-gate-at-dispatch, unit typed-roots-fix

Goal state: approved, tier 2, box 3d/10/1200m/3/20. One review round at most (fleet rule of 2026-10-04 16:14: one committed read per unit; non-breaking findings become notes).

# Goal

The goal's eight units (gate2, switch, fixes, landing, lane-rebased, governance, digest, and the design gate) are built and read on goal/design-gate-at-dispatch (tip 55ebdd8b714e). The lane returned the branch twice today because the merged tree with main does not compile: main's one-folder slices (d09578485, push 25) typed the state roots (`stateroot.Layout.InstallationRoot` is now `stateroot.Installation`, `RootForInstallation` returns `stateroot.State`), after this branch's base. The branch itself merges onto main with no textual conflict (m1l, 16:33).

# Workspace

Branch goal/design-gate-at-dispatch at its current tip; `metasystem work build` prepares or reuses the goal worktree. Do NOT rebase the branch and do not touch the eight unit commits: their reads must stay valid. Leave the change uncommitted in the worktree.

# Units

| Unit | Lines |
| --- | ---: |
| typed-roots-fix | 30 |

## What this unit builds

One commit on top of the branch that makes the branch's own code compile on both bases (the branch's base and current main), using plain conversions, so the lane's merge of this branch onto main builds:

1. `cmd/metasystem/intent_design_gate.go` around lines 352 and 361: `RootForInstallation` now returns `stateroot.State`; take the path with the typed value's accessor (`.Path()` or the conversion main's callers use; read how `cmd/metasystem/launch_verbs.go` and `intent_work.go` on main call it) rather than a bare string.
2. `cmd/metasystem/intent_work.go` around line 600: `layout.InstallationRoot` is `stateroot.Installation`; where the branch passes it as a string, convert with `string(...)` or the accessor main uses.
3. `internal/steward/lane_silent_test.go` line 29: the `Layout.InstallationRoot` field takes a typed value; build it as main's tests do (`stateroottest.Installation(t, path)` or the typed conversion).
4. Anything else `go build ./... && go vet ./...` reports on the branch tip merged with main. To see what main needs, run the check in a scratch merge: `git -C <worktree> merge-tree` or create a temporary branch `git worktree`-free merge in a temp clone; simplest: in the goal worktree run `git fetch origin && git merge --no-commit --no-ff origin/main`, build, note the errors, then `git merge --abort` and fix the branch's own files so that both the branch alone and the merge compile.

## Not in this unit

Any behavior change; any edit to the eight unit commits; any rebase.

# Constraints

Go only; conversions and accessors, no new helpers. Both `go build ./... && go vet ./...` on the branch tip alone and on the temporary merge with origin/main must pass; `go run ./cmd/devgate static` on the branch tip must pass. Tests: `go test -count=1 -timeout 20m ./cmd/metasystem/ -run 'TestDesignGate|TestGate|TestAudit' ./internal/steward/`.

Maximum reader tool calls: 30

# Expected Return

The uncommitted change in the goal worktree with a report naming each converted site, the build and vet results on the branch alone and on the merge, and the line count.

# Acceptance Criteria

- Branch tip alone: `go build ./... && go vet ./...` and the static gate pass.
- Branch tip merged with origin/main (temporary merge, aborted afterwards): `go build ./... && go vet ./...` pass.
- The named tests pass on the branch tip.
- No file outside the three named and whatever the merge build names is changed.

# Gap Rule

If a conversion is ambiguous, use what main's own callers of the same function use; say so in the report.
