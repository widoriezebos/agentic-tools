# lane-housekeeping-2 unit 6 correction 1: a covering read waives the read, never the build; the person's exception stays whole and says what it lets through

Working Mode: Implement. Correction after the Opus read (RETURN, 2 material). Worktree uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4 (or -p 2 under load).

## F-1 (high, R-115 item 6): naming a unit in a tip read makes it "done" even when it was never built
cmd/metasystem/intent_delivery.go ~2366-2368 (goalProgress): a row whose name appears in any clean read's CoveredUnits is skipped before the `built` check, so it never reaches Unbuilt — a tip read requested with `--units U1,U2,U3,U4` while U4 has no build lands the goal whole with U4 unbuilt; the refusal's remedy (~2486-2489) even suggests putting Unbuilt into `--units`. Fix: a covering read waives only `unread`, never `built`: move the CleanReads check after the build loop so it cannot keep a row out of Unbuilt; the remedy for Unbuilt names `work build`, never `--units`. Test: design U1-U4, builds U1-U3, tip read naming U1-U4 → refused with "U4 is not built" (mutation: restore the early skip → lands → test fails); the saved-shape test models U1..U3 as builds at their recorded commits (lane-speed/U1..U3 Goal-Unit commits), not one build named "main".

## F-2 (medium, R-142/143): the person's exception gains a failure path and lands unread builds silently
cmd/metasystem/intent_exception.go ~406-424 composeCarriedSubject reads the accepted design's Units table and any DeclaredUnits error stops the person's exception; the only consumer is data["units"], which nothing reads. Fix: remove that read (or ignore its error) — a person's decision never fails on a machine-side read that adds no safety. internal/goal/branch/land.go ~308-316: with --exception-candidate every build becomes landable and LandingReady is skipped, so builds without a clean read land without being stated. Fix: the exception's result and its recorded reason list the builds past status.Prefix ("U2, U3 land without a clean read"). Test: an exception over a branch with one unread build → lands, result and record name it (mutation: drop the listing → test fails).

## Folds (non-material)
- Drop TestWorkLandTipReadRealContract (it reads every ledger page and certifies nothing about this unit).
- Composite trailer names (`U1+U2`) in progress.Unread: the remedy prints the split names so a person can pass them to --units.

## Checks
`go vet ./cmd/metasystem ./internal/goal/... ./internal/launch`; `go test -p 4 -count=1 -timeout 30m ./internal/goal/... ./internal/launch ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestWorkLand|TestWorkReview|TestUnitReview|TestAudit'`; `go run ./cmd/devgate static`. Report: diff --stat, production line count (≤250 total), every exit, each mutation.
