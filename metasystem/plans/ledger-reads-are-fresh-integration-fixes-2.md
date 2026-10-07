# Brief: ledger-reads-are-fresh, integration-fixes 2

Working Mode: Implement
The branch (main merged in, integration fixes committed at 63c3ffbe4) has two cmd reds in its full gate. Fix exactly these:
1. TestIntentTextNamesOnlyPublicForms: cmd/metasystem/up.go:278 prints "metasystem up", an engine verb, without `internal`; name the public form the surface audit accepts.
2. TestEveryStatefulActionRepeatsAsSuccess/goal_claim (intent_idempotency_goal_test.go:32): `goal claim standing-validation --lineage m1` is refused "this session's authority can't be read; nothing was claimed" on its first run. Find whether U1's claim authority change or the fixture is wrong: a claim with an explicit, valid lineage on the repeat witness's bed must succeed and repeat as success; fix the code if the claim path now requires an authority read the design does not ask for, else give the witness the authority fixture the other claim tests use.
Check: go build ./... && go vet ./cmd/metasystem/ && go test -count=1 -timeout 60m -run 'TestIntentText|TestEveryStateful|TestGoalClaim|TestGoalNext|TestLedgerFresh|TestIntentPlanning' ./cmd/metasystem/ && go run ./cmd/devgate static
Never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits and the cause of item 2.
