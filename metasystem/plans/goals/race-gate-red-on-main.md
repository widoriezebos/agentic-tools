# race-gate-red-on-main

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=3 basis="The full Go gate the adopt fixture runs (go test -race -cover -timeout 30m ./internal/...) is red on plain main on a quiet Mac in four packages; every landing that relies on that gate is exposed; the debt grows with each landing that does not run it."
- Tier: 3
- Intent: scripts/adopt-fixtures.sh runs go test -race -cover -timeout 30m ./internal/... (scripts/agents/go-gate.sh line 528). On m2 2026-09-04 21:13Z, on a quiet Mac at f40fcf50, that gate was red in four packages: internal/goal and internal/missionrunner timed out at 30 minutes under the race detector (18 and 24 minutes without it), internal/refusal failed TestHCL03EveryCodeRowed, and internal/steward failed TestArmConfirmsTheGuardAndDisarmEndsIt after 1042 seconds. The fast gate (go-gate.sh --fast) that landings run does not include this, so the debt is invisible at landing. DONE means the two timeouts are addressed (a per-package budget that fits the goal and mission-runner packages under -race, or those packages' slow tests marked and run in a separate long lane the adopt fixture invokes), the refusal and steward failures are fixed at their cause, and the adopt fixture is green on a quiet Mac; the evidence log of this run is at the seat's scratchpad and the failing test names are in this intent.
- Origin: main
- Next step: LANDED at 6c79648a (chain rgr-build1) and 7a6f93b5 (round-two dispositions). PROOF: the adopt fixture on landed main went red after one minute at a step this chain never touched (adopt-fixtures.sh line 485: the adopted target's engine does not accept the --tier flag the fixture passes to goal open), a pre-existing red owned by goal battery-adoption-red (parked residual, Wido 2026-08-30: features first); govulncheck also refuses on this Mac's go1.26.5 (five standard-library findings fixed in go1.26.6, which m2 already runs). So 'the adopt fixture is green on a quiet Mac' cannot be met by this goal's own work either; the race gate itself is being proven with the full go-gate on landed main (started 09:30Z). Decision with Wido at the enrolled terminal: narrow the intent to what is proven (refusal fix, the two goal races, the 60m ceiling, the race gate green on m1 and the same packages green on m2 once pulled) and conclude, or keep the steward and adopt-fixture clauses and park. Human act either way. Seat side effect to clear: steward restart --repo . (engine digest off the enrolled pin after the receipt's fast-gate rebuild).
- OpenedAt: 2026-09-04T21:14:28Z
- Revision: 11
- Labels: robustness
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=0e61f641bca538c7f64dc22b09fa17c6da5e8650f292db27090b5c9be242ae8f
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=5 at=2026-09-06T07:52:34Z
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T07:37:52Z revision=5 accountingRevision=5
- StopCapability: generation=5 revision=5 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-04T21:14:28Z WPG34JGC5TNCHRKWTV7W41BGT4-m2-5fcf08ab open actor=human:Wido targets=race-gate-red-on-main
- 2026-09-04T22:11:09Z ZH0H4W45RP14R7MMS4P85XFNNY-m2-5fcf08ab approve actor=human:Wido targets=race-gate-red-on-main authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Yes, all five (Recommended)"
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=race-gate-red-on-main reason=sweep
- 2026-09-06T07:28:59Z 3ZZ8AFSPB0S2BDG1T28CFSVY38-m1-a4f8999f set-pin actor=human:Wido targets=race-gate-red-on-main
- 2026-09-06T07:37:52Z 66JHSCQTG4E5GFX3BH13ZKB93F-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
- 2026-09-06T07:52:34Z 1X4GBRTEW68KMZ0CWF5HBQRG98-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
- 2026-09-06T07:54:56Z 5SS59ZX3JEADX82J35S0TZJH1J-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
- 2026-09-06T08:35:49Z NRGESDQFDRWTMQPTRZCCTP31SF-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
- 2026-09-06T09:05:43Z Z0AWM6AH310149Q05CJS2RPPRV-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
- 2026-09-06T09:27:48Z ZQ7WD593H4GBG7VTV2AQ3G2Z3B-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
- 2026-09-06T09:29:52Z CN1HC5K2AFG2J7XQM5CFKF4JZD-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
Integrity: sha256=70e7804bc8f111dcf9f46e81ce608e561b1ccdb0f2b294cd227a0969de0d61d5
