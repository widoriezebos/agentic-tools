# Goal integration fixes 3: the full gate's remaining reds on the goal tree

Working Mode: Implement. Third integration step of goal lane-proves-at-the-batch-risk (the worktree holds U1-U4 and two integration fixes committed; change only what this brief names).

## 1. internal/layering: TestGoDecidesNativelyExecsOnlyLawfulPrograms

`cmd/metasystem/test_impact.go:259` (`fullGroupCount`, unit U2) execs `go list` directly; ruling R-138-m1e: Go decides natively, and a fixed program is lawful only for the files listed in `internal/layering/nativeexec_test.go` (`lawfulExecPrograms["go"]`: cmd/devgate/main.go, cmd/metasystem/proof_run.go, intent_adopt.go, landing_path.go, internal/testenv/toolchain.go, internal/rootaudit/load.go, internal/landing/batch/goadapter/impact.go). Fix: move the package expansion into the lawful adapter: a function in `internal/landing/batch/goadapter` (e.g. `TestPackageCount(moduleRoot string, buildTags []string) (int, error)`, or reuse the adapter's existing `go list -json ./...` enumeration) that returns the number of test-bearing packages for the group's build tags; `fullGroupCount` calls it and execs nothing. Do not add cmd/metasystem/test_impact.go to the lawful list. Test: the layering test passes; `fullGroupCount`'s existing tests (TestLandingDepth*, share computation) still pass with the same numbers.

## 2. cmd/metasystem: TestARedProofIsToldFromMainsWithOneReplayRun (subtests gate/main-moved=false and gate/main-moved=true)

The full gate on the goal tree reports these red; they are green on main. The test tells a red proof from main's with one replay run; U3 changed the gate's records (Requested/Attributed, the parent replay for a static red, the shared proof-command seam) and U1 the scopes. Run the test with -v, read its bed and assertions, and find whether (a) the gate now runs its parent replay or static group where the test's bed does not fake it (then the bed needs the seam the other beds got in U3's correction 2, or the production path must honour the bed's existing fakes), or (b) the gate's record shape changed what the test reads (`last_gate`/Attributed): then the test's expectations follow the design D3 shape. Prefer fixing production behaviour to match the test's intent (one replay run tells a red proof from main's) over loosening the test; explain the cause in the report.

## Checks

`go test -count=1 -timeout 30m ./internal/layering`; the named cmd tests of section 2; `-run 'TestLandingDepth|TestLandingProve|TestLandingImpact|TestTestImpact|TestLandingCheck'`; `./internal/landing/plain ./internal/landing/batch/goadapter`; `go run ./cmd/devgate static`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat, each exit.
