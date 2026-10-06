# Brief: lane-lands-finished-goals, unit admission-fixes (split from unit admission under the stop rule)

Working Mode: Implement
Goal: lane-lands-finished-goals. Unit admission is committed (2dca1b66d). Its second Opus read found two material defects; fix exactly these.

## What this unit fixes

1. F-3: `cmd/metasystem/intent_delivery_owner_test.go:32-33` — `newWholeOwnerLanding` writes `plans/designs/landing-work.md` into `f.mainRoot` and never commits it; the goal page is committed at `:55-56`, so the main checkout is dirty and 11 carried-delivery and exception-landing tests fail with `carried-checkout-dirty` (TestIntentCarriedGoalDeliveryGitAdapter, TestIntentCarriedReplay, TestIntentCarriedReplacement, TestIntentCarriedFromTheLinkedGoalCheckout, TestIntentCarriedChannelWordAdoption, TestIntentCarriedAmbiguousRetainedBaseNeedsReplacement, TestIntentCarriedPushReadsTheGateAgain, TestExceptionLandingReleasesItsLandedWorkspaces, TestExceptionLandingCrashAfterThePushIsFinishedByTheRetry, TestExceptionLandingCrashAfterThePushIsFinishedByTheNextLand, TestExceptionLandingPostsTheDeliveredSentence). Fix: commit the design page with the goal page in that fixture.
2. F-4: a records hand-in hides an earlier landing: `plain.Latest` (`internal/landing/plain/queue.go:223`) returns the newest entry including records entries; `cmd/metasystem/intent_delivery.go:2022` keeps `landed` only when `!entry.Records`, so after a landed records hand-in the landed-once check at `:2028` is false and a landed goal hands in again on the lane route. Fix: in the landed-once lookup take the goal's newest non-records entry (from `plain.Entries` or equivalent), derive its landed state, and use that. Test through `work land` on the lane bed: land the finished goal, hand in records and land them, add a read unit, `work land` is refused with GOAL_LAND_ONCE (mutation: use Latest again, red).

## Check (run all, with exits)

`go build ./... && go vet ./cmd/metasystem/ && go test -count=1 -timeout 30m -run 'TestIntentCarried|TestExceptionLanding|TestWorkLand|TestLandingPlain|TestHandIn|TestLanded|TestRecords|TestIntentDelivery|TestIntentLand|TestGoalProgress|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL' ./cmd/metasystem/`

Constraints: Go only; every new test calls t.Parallel(); -timeout 30m; never weaken a test. Leave the change uncommitted.
