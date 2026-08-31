# codex-148-adapter-drift

- State: claimed
- Intent: RESCOPED BY WIDO (2026-08-31, verbatim: 'Total crap. Kill it.'): the codex config filter is RETIRED. Value audit found the mechanism's entire recorded benefit was suppressing a handful of spurious capability re-probes (three on 2026-08-07, seconds each) and one misblame incident, while its maintenance had already cost a broken version-range on the first CLI upgrade, a refuted chain, and a proposed 600-minute design slice. The config identity hash itself STAYS (knowing the runtime config changed is real value); only the filtered-view mechanism goes: the filter file, its loader/version-range path in internal/config, the fixture pinning, and adapter references. After removal every canonical key is hashed always; the rare CLI-state churn costs one cheap self-healing re-probe (KI-19's original annoyance, accepted with eyes open). Supersedes the schema-v2 decision of earlier today - value review outranked it.
- Origin: main
- Next step: DONE, landed by m0 (account Wido@M0): the filter mechanism is retired - 13 files, -397/+22, identity hashes every canonical key always, KI-19's occasional cheap re-probe re-accepted with eyes open. Chain config-filter-retirement closed; the R-33 triage rule rode the landing as register carriage. Residual for the next up: the live engine binary predates this landing and still accepts --filter; the accepted-engine generation refresh follows its own lifecycle.
- OpenedAt: 2026-08-31T12:57:44Z
- Revision: 7
- Budget: elapsedLimit=6d attemptLimit=6 reservedJobMinutesLimit=600 activeJobLimit=1
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-08-31T14:18:18Z revision=4
- StopCapability: generation=4 revision=4 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-31T12:57:44Z 4F2MG3NFZ2E4HVTW9VAZ2CHSFG-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
- 2026-08-31T12:57:44Z V1975H9RWY2PVW2DZ6XKFDMZMY-m0-c5dbf036 set-budget actor=human:Wido targets=codex-148-adapter-drift
- 2026-08-31T13:45:44Z N3WTTSXHWJPGE63RY977TWP6A5-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
- 2026-08-31T14:18:18Z FQ72AD9ZASKW05FJA39CAZBA46-m0-c5dbf036 set-budget actor=human:Wido targets=codex-148-adapter-drift
- 2026-08-31T14:18:19Z 9T63FXCXKSXTSVCHCCYBDYWKA7-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
- 2026-08-31T17:52:14Z YRA6KP357GAV03QKWX2DX80G8D-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
- 2026-08-31T18:06:22Z 215H2MRZ24RE0BZ4T6KJVM1MY4-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
Integrity: sha256=9e2a1d3622e22ac063b76e7b523eca804d876fd09222dacf71f5a31f87acefbc
