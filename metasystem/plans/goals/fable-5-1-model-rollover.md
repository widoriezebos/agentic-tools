# fable-5-1-model-rollover

- State: claimed
- Intent: Move every Fable lane to Claude Fable 5.1 (Wido's order 2026-09-02, verbatim: 'Fable 5.1 is released, make sure we use that model for Fable models going forwards'). The CLI accepts model id claude-fable-5-1 (m0b probe 2026-09-02 07:00Z returned canonicalModel claude-fable-5-1). m0b's machine-local roster (metasystem.conf.local: role.default.model.claude, role.code-critic.model.claude, mode.design.role.implementer.model.claude, cap.min.* keys for the new model, and a local runtime.claude.maximal-models=claude-fable-5-1,claude-fable-5 override) is already switched. This goal carries the tracked part: metasystem.conf runtime.claude.maximal-models must list claude-fable-5-1 (keep claude-fable-5 admitted until every seat has rolled over so in-flight rounds are not refused by the maximal gate), any test or fixture pinning the old id, and the R-25-m1 lane text in memory/rulings.md. metasystem.conf sits on the never-direct-fix floor, so this rides an implementation chain per R-38-m2. Out of scope: other seats' metasystem.conf.local files (m0, m1, m2, m3 operators apply the same three key changes by hand; this goal's next step names them)
- Origin: main
- Next step: Design landed (plans/fable-5-1-rollover-design.md, Fable 5.1's first round): Wido already landed the tracked line as claude-fable-5-1 alone (d081ef07) and m1 switched every seat's local roster, so the remaining tracked work is two fixture literals in internal/dispatch/composition_test.go (TestHazardConfigurationAcceptsConfiguredMaximalModel composes against the real root and is RED on main since d081ef07) plus the R-46-m0b ruling row. Sol critique in flight (f51-crit-1); then Sol build, Fable code critique, land with --chain
- OpenedAt: 2026-09-02T07:05:41Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=3 at=2026-09-02T07:17:14Z
- Claimed: machine=m0b lineage=main-1788250419-3170380-8a1fb3 at=2026-09-02T07:05:59Z revision=3
- StopCapability: generation=3 revision=3 machine=m0b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-02T07:05:41Z AD00YSGP1Q1K2VNPR1X36FKXSX-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:05:55Z Z9W8RR79RF8JSV9D2WGCGDBSV5-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:05:59Z R4TFFXGG6QA1TZ6YER2CQCTR31-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:17:14Z NMWW34A1XXD1DA5JDHEH6Z9Z95-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
- 2026-09-02T07:29:02Z E2Q2GGD6JNXZHCS5G4EQ49ADDC-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
Integrity: sha256=e018022562274584f9cf2202d361197f50f8fa9b68da6d508159f6d15336bed8
