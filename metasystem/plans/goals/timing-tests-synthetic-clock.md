# timing-tests-synthetic-clock

- State: queued
- Intent: Timing-logic tests run on an injected synthetic clock and finish in microseconds; only legs that spawn real processes keep the real clock — the race gate's 25min is mostly wall-clock waits on arithmetic (Wido 2026-08-27 evening: why does timing-sensitive testing take so long, should we use a synthetic clock)
- Origin: main
- Next step: AUDIT SLICE DELIVERED via missionrunner-suite-speed's landing (see its conclusion): per-test table measured (1424s/289 tests; top-12 only 463s — flat ~5s tail; giants are real git churn); wait-compression seam landed opt-in; FOUR named blockers pinned in code with reasons (taint identity second-granular; nested double-spawn windows compound; recovery-family livelocks; cross-test state leak wedging a later test after env restore). CONVERSION SLICES NOW SIZED: (1) 2h fix the cross-test leak + make compression the package default; (2) 3h sub-second taint identity + recovery-window audit; (3) 3h t.Parallel for the independent full-cycle beds (blocked today by t.Setenv coupling — 18 sites, 7 files). Coordinate with m1 (shared label) before slice 3 touches bed fixtures.
- OpenedAt: 2026-08-27T17:12:26Z
- Revision: 3
- Labels: shared

History:
- 2026-08-27T17:12:26Z GRZ4RPVHPK0D6H2SKE8P1X46EV-m2-bc1be9cb open actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-27T17:15:51Z 8TK863Y9F7XH960CTKX092C0AN-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:00Z 6RK75MKGKCSA79BE53CY00SBD0-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
Integrity: sha256=33096d99d782cbdfa487a7873db69815bc01c3eac628e2d9c67c52b5246ab807
