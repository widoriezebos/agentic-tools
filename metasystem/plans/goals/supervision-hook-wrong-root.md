# supervision-hook-wrong-root

- State: claimed
- Intent: The harness supervision hook resolves the wrong repository on nested checkouts: run from metasystem/ inside the agentic-tools-m3 clone it derives the git toplevel (the outer repo) as its metasystem root, reports a bootstrap world (no ledger, no steward), and its turn evidence never lands where health's hook-freshness role reads - m3 has hook-freshness=dead since enrollment with the hook firing every turn. DONE means the hook resolves the metasystem project root deterministically on nested checkouts, its turn evidence lands, and hook-freshness goes alive, proven by a fixture running the hook from a nested layout
- Origin: main
- Next step: Design at revision 3, round-3 critique returned TWO findings (converging 5-5-2; register records/misc/hook-root-critique-r3.md). Re-boxed under R-44 with disclosure: the prior box's six attempts were spent by six ladder rounds (~15min each), not by work size — R-44 CALIBRATION FINDING for Wido: a full ladder (design + folds + critiques + build + code critique) is 7+ rounds, so attempt-limit 6 cannot fit one ladder; either the tuple's attempt count rises or attempts should count failures, not rounds. Sequence: fold-4 (two findings, brief landed), closing critique, build, code critique
- OpenedAt: 2026-09-01T07:25:56Z
- Revision: 8
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=6 at=2026-09-01T22:28:54Z
- Claimed: machine=m0b lineage=main-1788250419-3170380-8a1fb3 at=2026-09-01T22:27:35Z revision=6
- StopCapability: generation=6 revision=6 machine=m0b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-01T07:25:56Z HJPEPF3NATCRT1F2FE5080H6S6-m3-a5da21ff open actor=m3+mac-m3 targets=supervision-hook-wrong-root
- 2026-09-01T08:36:18Z M4Y1ZAC9GBG995JWNQZX6MFE6Z-m3-a5da21ff edit actor=m3+mac-m3 targets=supervision-hook-wrong-root
- 2026-09-01T08:37:13Z DPEJA3AF5F4Y3TSBB5H4JWMPRE-m2-bc1be9cb edit actor=m2+mac-coordinator targets=supervision-hook-wrong-root
- 2026-09-01T20:25:08Z S5WF9QRVKT4BYY75R5KTYJQCT2-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T20:27:36Z R4E2ZEDCD08WGMAMXPT868345Q-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T22:27:35Z P90TZBA3Z73HR3ZBG5NV1QXBYY-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T22:28:54Z CB1V3T4WTDEN0P0KBMQKR4WHMV-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T23:34:07Z 4080GD1YPFG2RJ5Z4ZYD9WX56Z-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
Integrity: sha256=fb01241211bda50462a247c895b4d322f83ba914d375696d842b3177533b5a0d
