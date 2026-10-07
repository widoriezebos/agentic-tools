# lane-drain-and-fresh-claims

- State: done
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="A drain decides what the lane admits for every seat and engine comparison decides whether seats start (severity 3, exposure 3); designed against accepted pages (novelty 2); three units, one goal branch (accumulation 2)"
- Tier: 3
- Intent: The lane can drain (admit nothing new, finish its queue, hold) and only a person reopens it; the helm drains the lane; re-arm, source validation and dispatch compare engine inputs only
- Origin: main
- Next step: Build U1a drain and U3 engine inputs by hand with delegates, then U1b after 1b lands
- Concluded: Integrated whole on main at a63e88c05 (U1a drain, U1b helm drain, gate-fixes) after the full gate was green on the exact tree; U3 engine inputs dropped to goal 7 under the stop rule. Delivered by hand with delegates.
- OpenedAt: 2026-10-06T22:08:06Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-06T22:08:18Z revision=2 opid=CVNT32V7CJCBK6YYA02CCNSHYK-m1e-718ba0eb authority=proven digest=7a4e5f6f6a9bae84278705897c720ced5b8efbcfb789510d5b92f23cdaba3f4a episode=2

History:
- 2026-10-06T22:08:06Z CNV7A034XGKFR2BHDRVNYN9672-m1e-718ba0eb open actor=human:Wido targets=lane-drain-and-fresh-claims
- 2026-10-06T22:08:18Z CVNT32V7CJCBK6YYA02CCNSHYK-m1e-718ba0eb approve actor=human:Wido targets=lane-drain-and-fresh-claims
- 2026-10-07T04:27:29Z YM7YQ1KBJWFMFAKPAZBNQVSWW6-m1e-718ba0eb done actor=human:Wido targets=lane-drain-and-fresh-claims
Integrity: sha256=646712a47830c150ee1ce076364b944d4ed76ce08d9508195191f5e22d616b69
