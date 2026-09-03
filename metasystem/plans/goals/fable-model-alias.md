# fable-model-alias

- State: approved
- Intent: Wido's order 2026-09-03 (verbatim: 'i want claude-fable-5 to be an alias for claude-fable-5.1 to avoid running into DESSIGNM-BEARING'): a seat whose machine-local roster still names claude-fable-5 must dispatch claude-fable-5-1, not be refused REFUSED-HAZARD-CONFIGURATION by the DESIGN-BEARING maximal-models gate. Today internal/dispatch/hazard.go compares the roster id literally against runtime.claude.maximal-models and nothing canonicalizes model ids, so this is dispatcher code, not configuration. Consistent with R-46-m0b: the retired id resolves to 5.1 rather than reaching the API
- Origin: main
- Next step: Full ladder (R-38-m2): design under 100 lines (where the alias lives: a tracked runtime.claude.model-alias.<from>=<to> table or a hard-coded retired-id map in the roster read; the alias applied once at roster resolution so every later record, cap row, canonicalModelKey and hazard check sees the canonical id; the cap rows keyed by the old id follow the alias or are refused by name; a test that a roster on claude-fable-5 dispatches DESIGN-BEARING with effectiveModel claude-fable-5-1), one critique, build, one code review, land with --chain. Seat: whoever Wido approves; m3 is free
- OpenedAt: 2026-09-03T16:47:30Z
- Revision: 6
- Budget: elapsedLimit=4h attemptLimit=12 reservedJobMinutesLimit=360 activeJobLimit=1
- Approved: by=human:Wido at=2026-09-03T18:12:06Z revision=5 opid=XTF1476XPZAYQ83KTVH3Y2DJCD-m3-a5da21ff authority=relayed digest=da00c7db56614d68807480c4754c0b0958a65613bc802e37ccb7857ec75e14c7 reviewBy=2026-09-06
- Sliced: machine=m3 lineage=mac-m3 revision=3 at=2026-09-03T16:54:50Z

History:
- 2026-09-03T16:47:30Z 48SYYNGN6Z6KGW1JQ8PY01MNER-m3-a5da21ff open actor=m3+mac-m3 targets=fable-model-alias
- 2026-09-03T16:51:36Z H5S94Z57Q09HHGMGJ57AMBXTJK-m3-a5da21ff approve actor=human:Wido targets=fable-model-alias authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="ok, later, not now. Can yu approve for me?"
- 2026-09-03T16:52:27Z 16CGNHZMBYP1V926M0TYAMMJK8-m3-a5da21ff claim actor=m3+mac-m3 targets=fable-model-alias
- 2026-09-03T16:54:50Z 8486340MQXQARN8Y0PP637JKQY-m3-a5da21ff slice-start actor=m3+mac-m3 targets=fable-model-alias
- 2026-09-03T18:12:06Z XTF1476XPZAYQ83KTVH3Y2DJCD-m3-a5da21ff set-budget actor=human:Wido targets=fable-model-alias authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="2 (Wido's whole answer, choosing the seat's option 2 verbatim: Stop the prose loop now, fixtures as arbiter; the six findings go into the build brief as named fixture obligations, Sol builds from revision 2 plus the dispositions record, Fable's code review is the arbiter; minutes 360, attempts 12)"
- 2026-09-03T19:12:32Z CBESN31V1HAKD2DQNQH4MC1KJD-m3-a5da21ff release actor=m3+mac-m3 targets=fable-model-alias
Integrity: sha256=70ccf47134b65c12917c43f19029356c476b7c3dbe365398047ae288ad29486b
