# Brief: ledger-reads-are-fresh, unit gate-fixes (the integration gate's reds)

Working Mode: Implement
U1 is committed on this branch. The integration gate (this branch merged onto main) found these reds; fix exactly these.

1. internal/testenv TestNoTestWaitsOnWallTime: internal/boundedexec/total_deadline_test.go reads time.Now()+2s; replace with an injected clock or a cancel-only context, as the repository's wall-time rule requires (artificial clocks, never load-fragile waits). internal/proofrun TestTestEnvironmentStandardInventoryMatchesObserved nests this audit and clears with it.
2. internal/seat/launch TestAFreshCloneOfTheSelfHostedTemplateFetchesItsLedger (integration_test.go:252) and TestAClonedMachineIsAFleetSeatBeforeItIsEnrolled (:177): "the record carries no orientation line from the clone's own reading". U1 made `goal next` refuse an agent that is not the main holder; the seat launcher's orientation call (internal/seat/launch/sequence.go:673) ignores that error and loses its orientation line. The orientation read is advisory and must keep working for the launcher: give it a read-only path (advisory next, no claim authority needed) or make `goal next`'s advisory output available to a non-holder, without letting a non-holder claim. Test: the two seat-launch tests pass; an agent that is not the holder still cannot claim through next.

Check: go build ./... && go vet ./... && go test -count=1 -timeout 30m ./internal/boundedexec/ ./internal/testenv/ ./internal/seat/... ./internal/goal/ && go test -count=1 -timeout 30m -run 'TestTestEnvironmentStandardInventory' ./internal/proofrun/ && go test -count=1 -timeout 30m -run 'TestGoalNext|TestGoalClaim|TestLedgerFresh|TestSession' ./cmd/metasystem/ && go run ./cmd/devgate static
Never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits and git diff --stat.
