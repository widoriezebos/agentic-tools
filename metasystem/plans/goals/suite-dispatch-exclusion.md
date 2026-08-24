# suite-dispatch-exclusion

- State: queued
- Intent: Process-owning suites and dispatch jobs share one execution guard with queueing, not refusal: a live critic defers the battery and vice versa (2026-08-24 collateral: battery vs dispatched critic fought over the checkout, battery red)
- Origin: human
- Next step: Appetite: 2h. Extend the existing gate-run guard into a checkout execution guard both battery.sh and dispatch.sh take: second arrival WAITS (bounded, with a progress note) instead of refusing or colliding. Acceptance: launch a battery while a critic job runs — the battery queues, both finish green, no OWNED-ELSEWHERE.
- OpenedAt: 2026-08-24T13:24:10Z
- Revision: 1

History:
- 2026-08-24T13:24:10Z 91G0JQGVX472YYPWYGFNSTE0X8-m2-bc1be9cb open actor=human:wido targets=suite-dispatch-exclusion
Integrity: sha256=746ec8adef1c08e08bef5da66aaadaa92fb98084570378e11583804e5041d647
