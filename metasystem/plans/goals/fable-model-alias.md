# fable-model-alias

- State: approved
- Intent: Wido's order 2026-09-03 (verbatim: 'i want claude-fable-5 to be an alias for claude-fable-5.1 to avoid running into DESSIGNM-BEARING'): a seat whose machine-local roster still names claude-fable-5 must dispatch claude-fable-5-1, not be refused REFUSED-HAZARD-CONFIGURATION by the DESIGN-BEARING maximal-models gate. Today internal/dispatch/hazard.go compares the roster id literally against runtime.claude.maximal-models and nothing canonicalizes model ids, so this is dispatcher code, not configuration. Consistent with R-46-m0b: the retired id resolves to 5.1 rather than reaching the API
- Origin: main
- Next step: Full ladder (R-38-m2): design under 100 lines (where the alias lives: a tracked runtime.claude.model-alias.<from>=<to> table or a hard-coded retired-id map in the roster read; the alias applied once at roster resolution so every later record, cap row, canonicalModelKey and hazard check sees the canonical id; the cap rows keyed by the old id follow the alias or are refused by name; a test that a roster on claude-fable-5 dispatches DESIGN-BEARING with effectiveModel claude-fable-5-1), one critique, build, one code review, land with --chain. Seat: whoever Wido approves; m3 is free
- OpenedAt: 2026-09-03T16:47:30Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1
- Approved: by=human:Wido at=2026-09-03T16:51:36Z revision=2 opid=H5S94Z57Q09HHGMGJ57AMBXTJK-m3-a5da21ff authority=relayed digest=fd56f116369d11eda2804e3743704f46a24d946abb09ee5f43726e440ec536c7 reviewBy=2026-09-06

History:
- 2026-09-03T16:47:30Z 48SYYNGN6Z6KGW1JQ8PY01MNER-m3-a5da21ff open actor=m3+mac-m3 targets=fable-model-alias
- 2026-09-03T16:51:36Z H5S94Z57Q09HHGMGJ57AMBXTJK-m3-a5da21ff approve actor=human:Wido targets=fable-model-alias authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="ok, later, not now. Can yu approve for me?"
Integrity: sha256=d742433f29ae25dfd30622f19635baf97c64318c4acce4529026d354c84cad8a
