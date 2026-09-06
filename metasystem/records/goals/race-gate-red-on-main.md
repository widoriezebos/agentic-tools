# race-gate-red-on-main

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=3 basis="The full Go gate the adopt fixture runs (go test -race -cover -timeout 30m ./internal/...) is red on plain main on a quiet Mac in four packages; every landing that relies on that gate is exposed; the debt grows with each landing that does not run it."
- Tier: 3
- Intent: scripts/adopt-fixtures.sh runs go test -race -cover -timeout 30m ./internal/... (scripts/agents/go-gate.sh line 528). On m2 2026-09-04 21:13Z, on a quiet Mac at f40fcf50, that gate was red in four packages: internal/goal and internal/missionrunner timed out at 30 minutes under the race detector (18 and 24 minutes without it), internal/refusal failed TestHCL03EveryCodeRowed, and internal/steward failed TestArmConfirmsTheGuardAndDisarmEndsIt after 1042 seconds. The fast gate (go-gate.sh --fast) that landings run does not include this, so the debt is invisible at landing. DONE means the two timeouts are addressed (a per-package budget that fits the goal and mission-runner packages under -race, or those packages' slow tests marked and run in a separate long lane the adopt fixture invokes), the refusal and steward failures are fixed at their cause, and the adopt fixture is green on a quiet Mac; the evidence log of this run is at the seat's scratchpad and the failing test names are in this intent.
- Origin: main
- Next step: RELEASED by m1c 09:41Z with the work landed (6c79648a, 7a6f93b5). PROOF on landed main 7a6f93b5 (m1): the race step verbatim, 10 minutes, green everywhere but the missionrunner terminate-group flake; the full go-gate under GOTOOLCHAIN=go1.26.6 (govulncheck passes there; this Mac's brew go1.26.5 fails it on five stdlib findings), 11.5 minutes, same single red: TestTerminateGroupLeaksNoGroupsUnderCompression, owned by goal missionrunner-terminate-flake (it fired in both full runs here and as TestTerminateGroup on m2). The goal, refusal and steward packages are green on m1 and, at m2's last run, the same two goal races were the only goal-package red there. Unmeetable as written: the steward clause (the 2026-09-04 failure text is lost, the test took 8.86s not 1042s, four runs since without recurrence) and the adopt-fixture clause (red at a pre-existing --tier step owned by goal battery-adoption-red). Wido's act at the enrolled terminal: narrow the intent to what is proven and conclude (recommended), or park.
- Concluded: Landed 6c79648a (chain rgr-build1, two Fable critique rounds, dispositions at 7a6f93b5): the refusal exclusion, the two goal-package races fixed at cause, the 60-minute ceiling. Proven on m1: the race step and the full gate under go1.26.6 green except the missionrunner terminate-group flake owned by goal missionrunner-terminate-flake; m2's record 7eb1ab9b shows the same packages green there apart from the same flake family.
- OpenedAt: 2026-09-04T21:14:28Z
- Revision: 15
- Labels: robustness
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=0e61f641bca538c7f64dc22b09fa17c6da5e8650f292db27090b5c9be242ae8f
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=5 at=2026-09-06T07:52:34Z

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
- 2026-09-06T09:40:58Z FHGMFCXWXBPSS08S1WVQH226KB-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
- 2026-09-06T09:41:48Z QHHBEVHVYX7FCHPAJ2VFRQMMX7-m1c-7cd0bd60 release actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
- 2026-09-06T09:53:02Z QTSG9RRRH304V3JBCVA1DCQTBG-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=race-gate-red-on-main
- 2026-09-06T10:58:53Z XBHTMT9EFQMZ56HVBMW05B1Z9W-m1c-7cd0bd60 done actor=human:Wido targets=race-gate-red-on-main
Integrity: sha256=d03a286cfcb4a90917f54ee08ecbb8dec2bb9681e248971cb8ec583aee385493
