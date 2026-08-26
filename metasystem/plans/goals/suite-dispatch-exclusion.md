# suite-dispatch-exclusion

- State: claimed
- Intent: Process-owning suites and dispatch jobs share one execution guard with queueing, not refusal: a live critic defers the battery and vice versa (2026-08-24 collateral: battery vs dispatched critic fought over the checkout, battery red)
- Origin: human
- Next step: Appetite: 2h. Extend the existing gate-run guard into a checkout execution guard both battery.sh and dispatch.sh take: second arrival WAITS (bounded, with a progress note) instead of refusing or colliding. Acceptance: launch a battery while a critic job runs — the battery queues, both finish green, no OWNED-ELSEWHERE.
- OpenedAt: 2026-08-24T13:24:10Z
- Revision: 3
- Labels: custody
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-26T12:22:54Z

History:
- 2026-08-24T13:24:10Z 91G0JQGVX472YYPWYGFNSTE0X8-m2-bc1be9cb open actor=human:wido targets=suite-dispatch-exclusion
- 2026-08-26T05:39:59Z YF8FSAZXD494Z8BQ9GFBQB97JC-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-dispatch-exclusion
- 2026-08-26T12:22:54Z 2ZR0YS34W9A966S8F1MVV16FEF-m2-bc1be9cb claim actor=m2+mac-coordinator targets=suite-dispatch-exclusion
Integrity: sha256=c650086f76f9daf04c6d0efaa92d7cc606fa7f98361e9b0a45143ebb21e86dc0
