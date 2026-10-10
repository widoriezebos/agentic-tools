# Brief: runs-advance D3-fixes regression test (for a fix already made)

Working Mode: Implement
Committed at 219309577: in cmd/metasystem/steward_seat.go the advance loop now re-observes a run after a failed Continue (ObserveOnly) and stops the tick if the run's starting/running step count changed (activeUnitSteps), so an error raised after a launch (internal/launch/read_sequence.go:151 waitStep -> tree_reservation.go:473 waitLaunch -> Manager.Status failing, or a save failing after startStep) cannot let the loop start a second run in the same tick. Do not change production code unless the test shows the fix wrong.

Write the regression test: two ready runs; the oldest's Continue launches its step and then returns an error (inject a failing Manager.Status or store save through the existing test seams); within one tick the younger run starts nothing. Mutation: plain `continue` after any error -> red. Also cover the case the fix keeps: the oldest refuses before launching (e.g. UNIT_NAMED_INPUT_CHANGED) and the younger starts in the same tick.
Check: go build ./... && go vet ./cmd/metasystem/ && the new test plus TestDriverPublicStewardCollection by name. t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
