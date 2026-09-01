# dispatch-cap-necessity

- State: queued
- Intent: Reservation caps charge budgets for time never run: every dispatch reserves its cap (flat 120min default) against the goal's reserved-job-minutes pool regardless of actual runtime, so lean human tuples are eaten by accounting, not work — four specimens by 2026-09-01 (m3 twice refused admission 2026-08-31; m2's four-hour seat freeze rooted in the flat reservation; m0b's alert-channel round reserved 120min for a 10-minute run and starved a 240min pool). Design question, needing Wido's word per the draft (plans/goals-drafts/dispatch-cap-necessity.md): the budget projection consumes observed/estimated run minutes, demoting the cap to pure fail-stop, now that streaming makes progress judgeable. Interacts with R-13, R-17, and the stop-loss machinery. Promoted from draft by Wido's word R-40-m0b
- Origin: main
- Next step: Design round first (ladder law R-38-m2): a Fable-lane design answering the draft's question with the four specimens as calibration, then Sol design critique; budget tuple is Wido's word at claim
- OpenedAt: 2026-09-01T13:49:31Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-01T13:49:31Z 43EKD9F0H470RXJZ1BDKFH564B-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=dispatch-cap-necessity
- 2026-09-01T20:28:29Z WE09W2DGTN02QHE75RDN1K2MF0-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=dispatch-cap-necessity
Integrity: sha256=c4765915196fc427758a6f77d13d4608e066ceef73a2242112a86ff5106d1952
