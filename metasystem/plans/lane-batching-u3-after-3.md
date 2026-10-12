# lane-batching U3 correction 3 (reduction): keep `landing prove --fresh`, remove the reuse machinery

Working Mode: Implement. The design's D5 is PARKED (read plans/designs/lane-batching.md D5: the sound rule caches nothing in this repository and its scan costs 8 minutes per proof). This correction REDUCES the unit to the part that stands on its own: `landing prove --fresh`. Worktree uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4 (or -p 2 under load).

## Remove (by subtraction, until `git diff origin/main` shows only the --fresh plumbing and its tests)
- internal/landing/plain/reuse.go (the per-group changed/unknown computation) and everything that feeds LANDING_PROOF_RUN_GROUPS (prove.go, repoproof/full.go's env and the VM bash -c string, the refusal register exclusions for the two environment names).
- internal/proofrun: the child-launch scan (goTestChildLaunches and its callers), gatherGoCacheFacts / declaredGoCacheFacts additions, the reuse_policy.go changes (restore the file to origin/main unless --fresh needs one line of it), the -count=1 change (every go group runs with -count=1 again as on main), groups.go's per-named-group scan.
- The reporter's "cached (reuse: …)" / "ran (reuse refused: …)" lines (restore the 5-field lines).
- Every test and fixture that exists only for reuse (named_reuse, native_reuse, the two-package fixture module, TestPackageReusePolicyAgainstRealContractPlan, …).

## Keep (and make sure it still passes alone)
- `landing prove --fresh`: carried through Settled, Start (joins only a fresh run), acceptsGreen, proofScope (full), the inherited and impact-reuse skips, the gate baseline, the detached --attempt launch, the retry argv; recorded as "fresh" on the result; R-129: a repeated --fresh while a fresh proof runs answers "already proving". Tests: TestFreshProofExecutesSavedGreenRecord and the real-go gate test for --fresh (keep those two; adapt them if they relied on reuse lines). Production total ≤ 80 lines.

## Checks
`go vet ./internal/proofrun ./internal/repoproof ./internal/landing/plain ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/proofrun ./internal/repoproof ./proof ./internal/landing/plain ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestLandingProve|TestARedProof|TestAudit|TestLanding'`; `go run ./cmd/devgate static`. Report: `git diff --stat origin/main` (must list only the --fresh files), production line count, every exit.
