# dispatch-cap-necessity

- State: claimed
- Intent: Reservation caps charge budgets for time never run - A BUG against the budget law's intent (Wido, R-49-m1b, highest priority): every dispatch adds its cap (flat 120 minutes) to the goal's reserved job-minutes and keeps charging it after the job ended, so nine rounds of 9-13 minutes consumed 1080 of a 1560-minute pool on 2026-09-02 and four earlier specimens starved lean tuples. DONE means a goal's reserved job-minutes equal the minutes its jobs actually ran plus the caps of the jobs still open, and every budget refusal names those two numbers
- Origin: main
- Next step: HIGHEST PRIORITY (Wido's word R-49-m1b, 2026-09-02). Appetite 4h, full ladder. HAZARD: internal/dispatch/budget.go:342-373 adds capMinutes to projection.ReservedJobMinutes for EVERY job record bound to the goal revision, terminal or not; the governed-run path beside it already settles to ObservedCostMinutes at terminalization (budget.go:483-487, terminalStateContradiction) - the delegated-job path never got the same settlement. MECHANISM: for a terminal job record (completed, failed, cancelled, timeout) the projection charges its observed minutes - endedAt minus startedAt rounded UP to the next whole minute, never less than 1 for a job that started, 0 for a job refused before it started - and for a pending or running job it keeps charging the cap (the ceiling it may still consume); a job killed at its cap therefore settles to the cap. The BUDGET_REFUSED message names both parts: 'observed=<n> open-caps=<m> limit=<L>'. The four structured limits (R-13) and the cap's fail-stop role (R-17) are unchanged; no new config key. REFUSAL SHAPE: admission refuses exactly when observed + open caps + the proposed cap exceeds the limit. TESTS (internal/dispatch budget tests, fixture-driven): a completed 10-minute job with cap 120 charges 10; a running job charges its cap; a failed handshake (ended seconds after start) charges 1; a timeout at the cap charges the cap; a record with unreadable startedAt or endedAt is unknownBudget (fail closed, the file's existing shape); the projection over today's two-bars-for-changes records yields observed minutes in the nineties, not 1080. BUILDER STARTS FROM budget.go:300-373 (the record loop), :483-487 and terminalStateContradiction (the settled pattern to mirror), admission.go:160 (the message), the budget fixtures beside them. Sequenced before every other item on this seat; m1b claims now.
- OpenedAt: 2026-09-01T13:49:31Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1
- Claimed: machine=m1b lineage=main-1788333346-60696-6a3256 at=2026-09-02T17:06:04Z revision=4
- StopCapability: generation=4 revision=4 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-01T13:49:31Z 43EKD9F0H470RXJZ1BDKFH564B-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=dispatch-cap-necessity
- 2026-09-01T20:28:29Z WE09W2DGTN02QHE75RDN1K2MF0-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=dispatch-cap-necessity
- 2026-09-02T17:05:05Z MXF9E6M3R7V8AK3N25PM4NV263-m1b-fad3674e edit actor=m1b+main-1788333346-60696-6a3256 targets=dispatch-cap-necessity
- 2026-09-02T17:06:04Z 23489DTN6KZ7BDZVE8ETYDYHBE-m1b-fad3674e claim actor=m1b+main-1788333346-60696-6a3256 targets=dispatch-cap-necessity
Integrity: sha256=4e14cf765cb2f67d4d75eb236560433c662a3fa139d049dbb8e991cfa68a3c23
