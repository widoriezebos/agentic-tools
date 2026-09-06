# race-gate-red-on-main

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=3 basis="The full Go gate the adopt fixture runs (go test -race -cover -timeout 30m ./internal/...) is red on plain main on a quiet Mac in four packages; every landing that relies on that gate is exposed; the debt grows with each landing that does not run it."
- Tier: 3
- Intent: scripts/adopt-fixtures.sh runs go test -race -cover -timeout 30m ./internal/... (scripts/agents/go-gate.sh line 528). On m2 2026-09-04 21:13Z, on a quiet Mac at f40fcf50, that gate was red in four packages: internal/goal and internal/missionrunner timed out at 30 minutes under the race detector (18 and 24 minutes without it), internal/refusal failed TestHCL03EveryCodeRowed, and internal/steward failed TestArmConfirmsTheGuardAndDisarmEndsIt after 1042 seconds. The fast gate (go-gate.sh --fast) that landings run does not include this, so the debt is invisible at landing. DONE means the two timeouts are addressed (a per-package budget that fits the goal and mission-runner packages under -race, or those packages' slow tests marked and run in a separate long lane the adopt fixture invokes), the refusal and steward failures are fixed at their cause, and the adopt fixture is green on a quiet Mac; the evidence log of this run is at the seat's scratchpad and the failing test names are in this intent.
- Origin: main
- Next step: Chain rgr-build1 (codex, DESIGN-BEARING) builds round one from brief plans/race-gate-red-on-main-brief.md: the refusal exclusion for VERIFIED_CHANNEL_ANSWER (an admission token landed 2026-09-04 without a register entry), two data races found in internal/goal on m1 (the deadline fetch reads the fetchForProjection hook inside a goroutine that outlives the deadline; TurnVerdict, WriteSessionStop and EndSessionStop rewrite the shared Store's Root while the watchdog protocol calls TurnVerdict concurrently), and the race run's per-package ceiling 30m to 60m (standalone under race on m1: goal 475s over 353 serial git-churn tests, missionrunner 414s over 296; no test above 19s). The steward test passes standalone and as a whole package on m1 (134s); its 17-minute failure on m2 is being reproduced with the full gate on m1 before any fix is briefed. Then: Fable code critique, fold, close, land with the full battery receipt, adopt fixture green on this Mac, done.
- OpenedAt: 2026-09-04T21:14:28Z
- Revision: 7
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
Integrity: sha256=06ccf563a49c4de216112856c2876eb98ed188dc34adff67e99699db84e47e3a
