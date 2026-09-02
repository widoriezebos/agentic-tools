# fable-5-1-model-rollover

- State: queued
- Intent: Move every Fable lane to Claude Fable 5.1 (Wido's order 2026-09-02, verbatim: 'Fable 5.1 is released, make sure we use that model for Fable models going forwards'). Model id claude-fable-5-1 (needs Claude Code 2.1.251+; guest CLI 2.1.258). Wido landed the tracked maximal-models line as the single new id (d081ef07); m1 switched every seat's machine-local roster; this goal carried the tracked remainder that landing left behind: the red composition_test fixture and ruling R-46-m0b
- Origin: main
- Next step: DONE, landed by m0b via the full ladder (design f51-design-6 by Fable 5.1, Sol critiques f51-crit-1/2, Sol build f51-build-1, Fable 5.1 code critique f51-code-crit-1). Main's internal/dispatch tests are green again. Concluding is the opener's act
- OpenedAt: 2026-09-02T07:05:41Z
- Revision: 7
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=3 at=2026-09-02T07:17:14Z

History:
- 2026-09-02T07:05:41Z AD00YSGP1Q1K2VNPR1X36FKXSX-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:05:55Z Z9W8RR79RF8JSV9D2WGCGDBSV5-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:05:59Z R4TFFXGG6QA1TZ6YER2CQCTR31-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:17:14Z NMWW34A1XXD1DA5JDHEH6Z9Z95-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:29:02Z E2Q2GGD6JNXZHCS5G4EQ49ADDC-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:56:07Z V14HAAJP0A5D8CFYWR347W3B1X-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:56:10Z XP0Z54G2K9MA1606M6SF2TTPG9-m0b-6638932d release actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
Integrity: sha256=dd79cd03900067bd411230f66cf5960afbd20084fdf7f1a25a5a0030b73db8aa
