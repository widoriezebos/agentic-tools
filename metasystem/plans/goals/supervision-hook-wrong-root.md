# supervision-hook-wrong-root

- State: claimed
- Intent: The harness supervision hook resolves the wrong repository on nested checkouts: run from metasystem/ inside the agentic-tools-m3 clone it derives the git toplevel (the outer repo) as its metasystem root, reports a bootstrap world (no ledger, no steward), and its turn evidence never lands where health's hook-freshness role reads - m3 has hook-freshness=dead since enrollment with the hook firing every turn. DONE means the hook resolves the metasystem project root deterministically on nested checkouts, its turn evidence lands, and hook-freshness goes alive, proven by a fixture running the hook from a nested layout
- Origin: main
- Next step: WAITS ON WIDO (attempt fence): design at revision 3, TWO findings from closure (register records/misc/hook-root-critique-r3.md; fold-4 brief landed and ready). The prior box's six attempts were spent by six ladder ROUNDS (~15 min each), not failures — the R-44 calibration finding: a full ladder is 7+ rounds, so attempt-limit 6 can never fit one. The machinery refuses both the identical re-box (no new revision) and the split (recorded work). One word resumes it: either a tuple with attempts 10 for this goal, or the standing R-44 tuple amended to attempts 10, or attempts redefined to count failures. Everything is landed; any seat resumes cold from this note
- OpenedAt: 2026-09-01T07:25:56Z
- Revision: 9
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
- 2026-09-01T23:35:27Z CDJQKRR80476C6GJPVDJNRH7CP-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
Integrity: sha256=f8e91a520ac4f53903ccd6c9539463940855cfcdf902ee25e9f1775d6a40b7b1
