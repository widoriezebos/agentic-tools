# Brief: design-gate-at-dispatch, unit main-drift-fix

Goal state: approved, tier 2, box 3d/10/1200m/3/20. One committed read at most (fleet rule 2026-10-04 16:14); non-breaking findings become notes.

# Goal

The branch goal/design-gate-at-dispatch (eight units plus the typed-roots-fix unit, tip 4c2fcd65c21e) is in the landing lane. The lane's proof of the branch merged on main (tree 1306faf31413, 17:53 CEST) is red in three places that come from main moving since the branch's base, not from the units' logic:

1. `internal/testenv` `TestEveryPackageUsesSharedMain` (and its `shared_TestMain` subtest): main now requires every test package to use the shared test main; a test package the branch added or touched lacks it.
2. `internal/proofrun` `TestTestEnvironmentStandardInventoryMatchesObserved`: the test-environment inventory on main does not match what the merged tree observes (most likely the same new or changed test package, or a new fixture the inventory must list).
3. `cmd/metasystem` `TestAuditOutputLayoutJSONUnchanged` subtests `goal-list`, `goal-show`, `goal-show-stopped` and siblings: the branch changed the goal list/show output (the design gate's fields) and the layout goldens on main were not regenerated for the merged output.

# Workspace

The goal worktree of design-gate-at-dispatch; `metasystem work build` reuses it. Do NOT rebase; do not touch the nine existing commits. Leave the change uncommitted.

# Units

| Unit | Lines |
| --- | ---: |
| main-drift-fix | 120 |

## What this unit builds

1. Reproduce on the merged tree: in the goal worktree run `git fetch origin && git merge --no-commit --no-ff origin/main`, then `go test -count=1 -timeout 20m ./internal/testenv/ -run TestEveryPackageUsesSharedMain`, `go test -count=1 -timeout 20m ./internal/proofrun/ -run TestTestEnvironmentStandardInventoryMatchesObserved`, and `go test -count=1 -timeout 40m ./cmd/metasystem/ -run TestAuditOutputLayoutJSONUnchanged`. Read each failure's message: it names the package, fixture or golden that is off.
2. Fix on the branch's own files only: add the shared test main to the branch's test package(s) exactly as main's packages do (look at how main's packages declare it); make the inventory match (add the new entry where main's inventory lives, or remove what the branch should not have added); regenerate the audit layout goldens with the repository's own update path (look for how `TestAuditOutputLayoutJSONUnchanged` refreshes its goldens, e.g. an `-update` flag or a golden-writing helper in `cmd/metasystem/testdata/layout/`), and read the regenerated golden diff to confirm it shows only the gate's new fields.
3. Abort the temporary merge (`git merge --abort`), then verify the branch alone still passes the same three test selections and `go run ./cmd/devgate static`. Redo the temporary merge once more and run the three selections to confirm green on the merge, then abort again.

## Not in this unit

Any behavior change in the gate; any rebase; any edit outside the branch's own test packages, the inventory entry and the goldens.

# Constraints

Go and golden files only. No real Git in tests, no process environment set by tests. The worktree must be left with no merge in progress (`git status` clean apart from the unit's changes).

Maximum reader tool calls: 30

# Expected Return

The uncommitted change, a report naming each of the three fixes with the test output before and after on the merged tree, the golden diff summary, and the line count.

# Acceptance Criteria

- The three test selections pass on the branch alone and on the temporary merge with origin/main.
- `go run ./cmd/devgate static` passes on the branch.
- No merge in progress is left in the worktree; only the unit's files changed.

# Gap Rule

If a golden's regenerated content shows a change that is not the gate's new fields, stop and report it instead of accepting the golden.
