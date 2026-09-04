# path-class-fixture-ripgrep

- State: done
- Tier: 1
- Intent: scripts/agents/path-class-fixtures.sh (TestDeletedListsHaveNoReader) calls ripgrep (rg), a command outside the declared inventory in docs/project-rules.md; on a machine without it the fixture dies 'rg: command not found' and reports a false reader of the deleted tables (m2, 2026-09-03). Replace the two rg calls with grep -rE plus --exclude-dir/--exclude so the fixture runs on every supported host.
- Origin: main
- Next step: TIER 1 per R-54-m1 (a fixture): build, run the fixture on a host without rg and on one with it, land as a declared direct fix; no design round, no review. Box 1h/3/60m/1. Origin: the m2 gate replay of the path-class manifest's second part, where the leg failed for the missing command while the equivalent grep found no reader.
- Concluded: Landed 07de953c: the deleted-lists search uses grep, skips binaries, and reports a broken search as such; suite green seat-side.
- OpenedAt: 2026-09-03T13:27:19Z
- Revision: 7
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T06:14:20Z revision=3 opid=PEBN8R2M6WNA1596FNQERTWD2E-m2-5fcf08ab authority=relayed digest=b679adc8ff5790917d0b23cc8112e6af34100fbcc714f43d68ca1762c99b2480 reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=4 at=2026-09-04T09:43:09Z

History:
- 2026-09-03T13:27:19Z 2F3FDXPRSZZTQ1A8ZPKNBVCYF6-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=path-class-fixture-ripgrep
- 2026-09-04T06:13:55Z CNFJ5Z3RHFGGHND5FV6QBTY7Q6-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=path-class-fixture-ripgrep
- 2026-09-04T06:14:20Z PEBN8R2M6WNA1596FNQERTWD2E-m2-5fcf08ab approve actor=human:Wido targets=path-class-fixture-ripgrep authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="the bugs yu mentioned are approved to fix too"
- 2026-09-04T09:42:44Z 8XCEH5FZ3S7F8SR2KD8ZQ1X4MK-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=path-class-fixture-ripgrep
- 2026-09-04T09:43:09Z A2TD87X3V0V3YED4KC1MVZZ5H9-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=path-class-fixture-ripgrep
- 2026-09-04T09:52:56Z 1W6SNV679ZQDSENMB0X0BKDY53-m2-5fcf08ab release actor=m2+main-1788441779-14484-82d6ed targets=path-class-fixture-ripgrep
- 2026-09-04T11:11:45Z QK606RAV5X2PYCE2M5Q03JYQ4K-m2-5fcf08ab done actor=m2+main-1788441779-14484-82d6ed targets=path-class-fixture-ripgrep
Integrity: sha256=f989ffbfe253e3a54843f6ab109865d850e0cc442127fe73c4edc2221fea5c19
