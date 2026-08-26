# suite-dispatch-exclusion

- State: queued
- Intent: Process-owning suites and dispatch jobs share one execution guard with queueing, not refusal: a live critic defers the battery and vice versa (2026-08-24 collateral: battery vs dispatched critic fought over the checkout, battery red)
- Origin: human
- Next step: Appetite: 2h. Extend the existing gate-run guard into a checkout execution guard both battery.sh and dispatch.sh take: second arrival WAITS (bounded, with a progress note) instead of refusing or colliding. Acceptance: launch a battery while a critic job runs — the battery queues, both finish green, no OWNED-ELSEWHERE.
- OpenedAt: 2026-08-24T13:24:10Z
- Revision: 2
- Labels: custody

History:
- 2026-08-24T13:24:10Z 91G0JQGVX472YYPWYGFNSTE0X8-m2-bc1be9cb open actor=human:wido targets=suite-dispatch-exclusion
- 2026-08-26T05:39:59Z YF8FSAZXD494Z8BQ9GFBQB97JC-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-dispatch-exclusion
Integrity: sha256=6ebe7ec154726e08e3e7e2bbb6cad14e299e1fa6ed52e2cbffbe342e35148928
