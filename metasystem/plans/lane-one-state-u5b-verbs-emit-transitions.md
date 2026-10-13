# lane-one-state U5b (D4, item 4 of U5 split out): every verb that changes a record emits its transition; a healthy lane has zero drift

Working Mode: Implement. Goal lane-one-state, design `plans/designs/lane-one-state.md` (read D1, D4, D6 step 2). Builds on U5. One commit; never push. Production code ≤ 250 lines. No test may read ../../testing.json or plans/goals; no "witness" or "certification" tests; no wall clocks.

## Build
Every writer of a raw lane record is followed, under the same lock, by the matching `state.Transition` (kind, from, to, by = the verb or the keeper, evidence): the pause/stop and resume writers (lane/pause.go, the resumed record), the queue's hand-in, return, give-back and landing writers (queue.go), the batch's prepare, extend, seal, run and close (batch.go), the proof's start and end (prove.go), the fix record's begin, abandon, close (fix.go, resolve.go), the stops' open and close (prove.go, lifecycle), the questions' ask and withdraw (question.go), the agent step's begin and end (agent_step.go), the push (push.go). A transition refused because the record lags reality is recorded as the defect event U1 defined and does not fail the verb. The shadow's rebuild diff on a healthy lane is therefore empty: the keeper's drift event never fires in the drive-through, and health shows no drift row.

## Tests
The drive-through (pause → resume → hand-in → batch → gate → proof → push → return) on the status bed asserts after each step: one transition event per raw write, `by` the verb, and an empty rebuild diff at the next refresh; the drift counter stays zero. A hand edit of the pause record between steps still yields exactly one keeper drift event (the mechanism is intact).

## Checks
`go vet ./internal/landing/state ./internal/landing/plain ./internal/landing/lane ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/state ./internal/landing/plain ./internal/landing/lane`; `go test -count=1 -run 'TestEveryStatefulActionRepeatsAsSuccess|TestEveryPublicActionHasAnIdempotencyRow|TestIntentObjectHelpGroupsEveryActionOnce|TestIntentPublicCoverage|TestHelmDrain|TestPlainLaneProve|TestLanding' ./cmd/metasystem`.
