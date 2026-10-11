# Unit U1 correction 3 (ruling defect, by the steward): the real rehearsal tests obey R-104-m1e

Working Mode: Implement. Correction of U1 of lane-rehearsal-before-landing after its machinery read (code-critic-1032b8441373769cb90990f0, finding RULING-R-104-m1e, material). Worktree at commit 536395f1c; make the change uncommitted on top; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json by hand.

## The defect
`cmd/metasystem/landing_rehearse_test.go`: three real end-to-end rehearsals (`TestLandingRehearsalReal*`) run in parallel inside the cmd/metasystem package, which the always-run standard test group executes on every proof; they wait on wall time (the ten-minute scenario deadline) and observe real processes, so under load they fail or exceed the budget — exactly what R-104-m1e forbids ("a test that fails under load is a defect of the test"; real-process tests remain few, with bounds an order of magnitude above the expected time, never in the hot path).

## Fix
- The real end-to-end rehearsal runs leave the always-run group: keep ONE real-process wiring proof (the green scenario) and make it opt-in through an environment variable the rehearsal gate sets (for example `METASYSTEM_REAL_REHEARSAL=1`; skipped with a clear reason otherwise), not `t.Parallel()` with the other real one, with a bound an order of magnitude above the measured 49-159 s (so the existing ten-minute scenario deadline is the bound, and the test's own timeout is above it). The canary-mutation real run becomes the same opt-in or a fixture-level assertion of the selector (the selector's real-input test already exists: keep that one as the always-run proof).
- Register nothing in testing.json by hand; if a new group is the right home, use the `metasystem testing` verbs and say so in the report.
- Fold N-1: drop the dead `METASYSTEM_TESTING_WORKERS` setting in scratch.go (or use the real `METASYSTEM_TEST_WORKERS` if a worker count was intended). Fold N-2: a failed first setup's scratch is kept (as the package rule says); success cleanup through the diskstore release.

## Checks
`go vet ./cmd/metasystem ./internal/landing/rehearsal/...`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLandingRehears|TestAudit|TestIntentPublicCoverage'` (the real runs skipped), then once with the opt-in variable set for the single real run; `go test -count=1 -timeout 30m ./internal/landing/rehearsal/... ./internal/layering`; `go run ./cmd/devgate static`. Report: diff --stat, every exit, the real run's wall time, the production line count.
