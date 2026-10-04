# Brief: lane-reproves-only-what-a-change-can-affect, unit 1 (affected groups and the Go closure)

Goal state: claimed by m1f, tier 3, approved box 1d/10/1200m/1/20; this unit has at most six review rounds (fleet rule of 2026-10-04); at the sixth it lands with its non-breaking findings as follow-ups.

# Goal

When main moves by changes that only some tests can see, the landing lane re-runs only the test groups that read the changed files and keeps its green for the rest, so proofs stay correct and the lane stays live (Wido 2026-10-03). Binding requirement (Wido, 2026-10-03 22:22 CEST): the mechanism is generic for any adopted application in any language; the lane side is language-neutral; which units a change affects comes from a per-language adapter behind one neutral interface; a language with no adapter gets the full suite; the impact set never comes from a hand-kept list.

# Workspace

Branch goal/lane-reproves-only-what-a-change-can-affect. Its workspace does not exist yet; `metasystem work build` prepares it. Leave the change there, uncommitted.

# Inputs

- Design: /Users/wido/LocalStorage/GitHub/agentic-tools-m1f/metasystem/plans/designs/lane-reproves-only-what-a-change-can-affect.md (accepted 2026-10-04 after Astra rounds 2/1/0). Its section 4 "Selection" steps 3 and 4, its section 5 (the Java rules) and the unit 1 row of its Units table are binding for this unit.
- Code sites (paths from `metasystem/`):
  - `internal/testpolicy/contract.go` (`Contract`, `Group` with `Inputs` and `PackageSelection`, lines 103-145); `internal/pathpattern/pattern.go` (`Parse` lines 21-48, `Covers` lines 79-84).
  - `internal/testpolicy/adapter/adapter.go` (`Closure` lines 18-23, `Units`, the `Adapter` interface lines 64-75, `Detect` lines 136-155).
  - `internal/landing/batch/goadapter/adapter.go` (`Detects` line 30, `Closure` lines 34-42, `closureOf`) and `internal/landing/batch/goadapter/unitgate.go` (`unitGateModuleRoot` lines 27-39, `SelectUnitPackages` lines 43-52, `selectUnitPackagesWithSnapshot` lines 54-61).
  - `internal/gopackages/select.go` (`Selection` lines 22-28, `SelectWithWorkspaceSnapshot` lines 70-73, `selectPackages` lines 75-190: `go.mod`/`go.sum` select everything at lines 135-140, an unowned path widens to `./...` at lines 166-171).
  - Test seams: `internal/landing/batch/goadapter/unitgate_test.go` (`newPackageTreeFixture` with its `prefix` for a nested module, `fixture.workspace()` returning `gittree.Workspace{Dir, RawSource}`; `TestWorkingUnitSelectionAcceptsNestedModuleTreeBase` is the nested-module pattern).
- Facts the design relies on: `gittree.Workspace.TreeOf` resolves the subtree of the workspace directory, so a selection run at the module root sees module-relative paths; run at a repository root with no `go.mod` it fails with "go package candidate has no go.mod". This repository's module is `metasystem/go.mod`; testing-contract inputs are repository-relative (`metasystem/plans/**`, `.gitattributes`).

# Units

| Unit | Lines |
| --- | ---: |
| affected-and-closure | 430 |

## What this unit builds

1. **`testpolicy.Affected(contract Contract, paths []string)`**, new, pure (no file or process access), in a new file `internal/testpolicy/affected.go`. Paths and inputs are repository-relative. It returns, in a result type the builder names:
   - the groups with at least one declared input covering at least one path (`pathpattern.Parse(input).Covers(path)`), in contract order, each with the changed paths its inputs cover (the lane records them as the group's reason);
   - the paths no group's inputs cover;
   - the paths covered by a template group (a group whose `PackageSelection` is not empty, such as `go-affected`); a template group is never itself listed among the covered groups.
   An input that does not parse is an error naming the group and the input; the caller treats any error as "prove in full". Empty `paths` returns an empty result.
2. **`adapter.Closure` gains `Unowned []string`**: changed paths, repository-relative, that no unit of the adapter's language owns. `Units()` and `Contains` are unchanged (they ignore `Unowned`). The opaque `command`/`section` adapter leaves it empty.
3. **The Go adapter's `Closure(root, base, tree)`** takes the repository root (or a module root, as today) and finds the module root itself with `unitGateModuleRoot`; no module found is an error. Its selection does not widen an unowned path to every package: each such path goes to `Unowned`, prefixed with the module directory relative to `root` (`metasystem/plans/x.md` for this repository; unchanged when the module is at `root`). `go.mod` and `go.sum` still select every package, as today. Paths outside the module directory are not seen by the selection and are not reported; the lane checks them against the contract itself.
4. **`gopackages`**: give the selection a way to report unowned paths without widening (an option on the existing snapshot entry point or a sibling entry point; the builder chooses, the smallest change). Every existing caller keeps its exact behaviour: `metasystem test run`'s `go-affected` expansion (`internal/proofrun/test_go_selection.go`) must still widen an unowned path to `./...`, because a test may open an asset no package owns. `Selection` may carry the unowned paths in a new field for both modes.
5. A testable seam for item 3: `Closure` reaches the selection through an unexported helper that takes the workspace and the snapshot opener, so tests drive it with `newPackageTreeFixture` and never run Git.

## Not in this unit

The lane's decision, the environment variables, the records, the owed full proof, the `test groups` verb, the real-contract test, removing the ledger shortcut, any change to `metasystem test run`'s selection, other languages' adapters, any change to `Detect`'s rule of exactly one adapter.

# Constraints

The accepted design is the specification. Build in Go; no new dependencies. Behaviour tests stub Git (project rule): use `gittree.Workspace{RawSource}` through the existing fixtures and the snapshot seam; no test runs real Git, sets process environment or reads `metasystem.conf.local`. Source comments state behaviour and invariants in plain English, never review rounds or history. Keep the change to the files named plus their tests; the gap rule covers anything else.

Maximum reader tool calls: 48

# Expected Return

The change, uncommitted in the goal worktree, on top of the branch's current tip, with a report: what moved (file by file), the exported names and signatures added, the test list with each test's mutation check (break the rule, see the test fail, restore), and the changed-line count (production and test separately).

# Acceptance Criteria

- `go test -count=1 -timeout 30m ./internal/testpolicy/... ./internal/gopackages/ ./internal/landing/batch/goadapter/` and `go test -count=1 -run 'GoExpansion|GoalLandingGo' ./internal/proofrun/` pass (the proof runner's Go selection tests, not its whole package).
- `go build ./...` and `go vet ./internal/testpolicy/... ./internal/gopackages/ ./internal/landing/batch/goadapter/` pass.
- Tests exist and pass, each failing under its named mutation:
  - `internal/testpolicy/affected_test.go` `TestAffectedGroupsDeclareAChangedPath`: a three-group contract (one group with `metasystem/plans/**`, one with `metasystem/internal/**` and an exact file input `metasystem/testing.json`, one template group with `packageSelection` and `metasystem/docs/**`); a plans path selects only the first group with that path as its reason; `metasystem/docs/x.md` is reported as template-covered and the template group is not listed; `README.md` is uncovered; a path under a directory declared as an exact name is covered (`Covers`); an unparseable input is an error naming the group. Mutations: match with `Match` instead of `Covers`; list the template group as covered.
  - `internal/landing/batch/goadapter` `TestClosureNamesUnownedPaths`: a change to `base/base.go` and to `plans/x.md` (a directory no package owns) gives `Changed` `./base`, its dependents, and `Unowned` with the plans path; a `go.mod` change still selects every package. Mutation: restore the widening.
  - `internal/landing/batch/goadapter` `TestClosureFindsANestedModule`: `Closure` given the directory above `metasystem/go.mod` (fixture `prefix` `metasystem/`) returns the module's units and `Unowned` as `metasystem/plans/x.md`, with no error. Mutation: pass `root` through unchanged.
  - An existing or new `internal/proofrun` or `internal/gopackages` test proves that the existing entry point still widens an unowned path to `./...`. Mutation: make the existing entry point stop widening.

# Gap Rule

If the design or this brief does not say what to do, choose the smallest change that keeps every existing caller's behaviour byte-for-byte unchanged, and say so in the report; stop and report a gap when no such choice exists.
