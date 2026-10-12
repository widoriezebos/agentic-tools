# lane-one-state U4 (D3): one lifecycle table; the keeper retires records as transitions; health lists the overdue ones

Working Mode: Implement. Goal lane-one-state, design `plans/designs/lane-one-state.md` (read it whole). Builds on U1-U3 (branch build/los1). One commit; never push. Production code ≤ 250 lines. No test may read ../../testing.json or plans/goals; no "witness" or "certification" tests.

## Build
1. `internal/landing/state/lifecycle.go`: ONE table `Lifecycle` of rows {Kind, Retires (the rule in words), Due(f LifecycleFacts, now) (bool, reason)} for every record kind under artifacts/agents/landing: a loop stop (stops.jsonl) closes when its subject landed, returned or was superseded or its proof attempt is not current (the rule the keeper got in lane-steward U2 correction 2); a person's stop never retires by rule; a question withdraws when its premise is gone (question.go:34 `WithdrawStaleLaneQuestion`, with U3's `QuestionPremise`); a fix record ends done or abandoned (fix.go:69 `readFixRecords` closeStale, with U3's `MergeOwnership`); a returned queue entry leaves the views after the day window (health.go:193-227); a result (results.jsonl) older than the retention stays in the file and out of the views; an agent step whose process is gone (agent_step.go `ProcessGone`) retires; a batch closed with no members retires. The rows are the only place these rules live: the scattered rules named above move here and their old sites call the table.
2. The keeper applies the table on its tick (the refresh entry of U1, after the shadow refresh): for every due record it performs the retirement its row names, each as a `state.Transition` (kind, from, to = retired/closed/withdrawn, by = keeper, evidence = the row's reason), so events.jsonl holds every retirement.
3. Health lists every record past its rule that is still present as a `Fired` row of kind `overdue-record` ("a loop stop of 2026-10-10 06:24 survived its subject lane-check-green, landed 2026-10-10 06:40"), computed from the same table; the steward's report counts them under "overdue records".
4. The lifecycle table is printed by `landing health --rules` (one line per row: kind, the rule in words) so a person can read what retires when without the code.

## Tests
Per row: a fixture record that is due → the keeper's tick retires it with one transition event; one not due → untouched; health names an overdue record before the tick and nothing after. The seventeen-stale-loop-stops case (finding 228): a stops.jsonl with 17 loop stops whose subjects landed → one tick closes all 17, the steward is no longer bounded by them (steward_test asserts the bound reason absent). A person's stop with landed subject → never retired by the keeper. `--rules` prints exactly the table's rows.

## Checks
`go vet ./internal/landing/state ./internal/landing/plain ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/state ./internal/landing/plain ./internal/landing/lane ./internal/layering ./internal/testenv`; `go test -count=1 -run 'TestAudit|TestLandingHealth|TestLandingSteward' ./cmd/metasystem`.

## Do not
No verb guard (U5), no wording changes beyond the two rows (U6). The keeper acts on the TABLE's facts read from the raw records, never on the shadow alone.
