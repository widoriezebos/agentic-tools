# fable-5-1-model-rollover

- State: queued
- Intent: Move every Fable lane to Claude Fable 5.1 (Wido's order 2026-09-02, verbatim: 'Fable 5.1 is released, make sure we use that model for Fable models going forwards'). The CLI accepts model id claude-fable-5-1 (m0b probe 2026-09-02 07:00Z returned canonicalModel claude-fable-5-1). m0b's machine-local roster (metasystem.conf.local: role.default.model.claude, role.code-critic.model.claude, mode.design.role.implementer.model.claude, cap.min.* keys for the new model, and a local runtime.claude.maximal-models=claude-fable-5-1,claude-fable-5 override) is already switched. This goal carries the tracked part: metasystem.conf runtime.claude.maximal-models must list claude-fable-5-1 (keep claude-fable-5 admitted until every seat has rolled over so in-flight rounds are not refused by the maximal gate), any test or fixture pinning the old id, and the R-25-m1 lane text in memory/rulings.md. metasystem.conf sits on the never-direct-fix floor, so this rides an implementation chain per R-38-m2. Out of scope: other seats' metasystem.conf.local files (m0, m1, m2, m3 operators apply the same three key changes by hand; this goal's next step names them)
- Origin: main
- Next step: Small item (4h). Design brief -> Fable 5.1 design (one paragraph is enough: the exact conf line, the fixture/test list from grep claude-fable-5 outside artifacts/records/plans) -> Sol critique -> Sol build -> Fable code critique -> land with --chain. Fleet note for m0/m1/m2/m3 operators: in metasystem.conf.local set role.default.model.claude, role.code-critic.model.claude and mode.design.role.implementer.model.claude to claude-fable-5-1 and add cap.min.<role>.claude.claude-fable-5-1 rows mirroring the existing claude-fable-5 rows
- OpenedAt: 2026-09-02T07:05:41Z
- Revision: 1

History:
- 2026-09-02T07:05:41Z AD00YSGP1Q1K2VNPR1X36FKXSX-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=fable-5-1-model-rollover
Integrity: sha256=d53c34466986ecdebe292cadc991cbcf105b53670c0191368b1e92fdffa7547d
