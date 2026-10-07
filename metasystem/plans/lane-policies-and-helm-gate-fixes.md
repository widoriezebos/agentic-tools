# Brief: lane-policies-and-helm, unit gate-fixes (the full gate's reds)

Working Mode: Implement
The goal branch is complete; its full cmd gate has three reds. Fix exactly these.

1. TestGoalCLILedgerLandingSlot (goal_cli_ledger_test.go:329) and TestGoalCLIBudgetStructuredBudget (goal_cli_budget_test.go:316): the claim line now always carries `areas=... areas-source=... areas-known=false areas-warnings=...` (U5's render in internal/goal/file.go), so ledger lines change for every legacy claim. Render the area fields only when the snapshot has known areas or warnings to record (omit them when areas-known is false, the list is empty and there is no warning); parsing a line without them reads as unknown. Keep the two tests' intent unchanged. A test: a claim with unknown areas renders the legacy line byte-for-byte (mutation: always render, red); a claim with known areas round-trips.
2. TestBrainBedActorSeamCoverage (brain_actor_seam_test.go:94): the actor allow-list changed. Inspect every added goal.Actor or caller-classification site the test lists (U1 settingsPerson and intent_policy.go, U4 helm take/return, U5 claim areas, and the goal.go sites it names). For each lawful site add it to the allow-list with a one-line reason; if any lets an agent act as a person, fix the site and say so.

Check: go build ./... && go vet ./... && go test -count=1 -timeout 30m ./internal/goal/... && go test -count=1 -timeout 30m -run 'TestGoalCLI|TestBrainBed|TestGoalClaim|TestClaim|TestPolicy|TestHelm|TestAudit|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static
t.Parallel(); never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, each test with its mutation, and the allow-list sites with reasons.
