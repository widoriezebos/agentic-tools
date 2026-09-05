# supervision-hook-wrong-root

- State: queued
- Intent: The harness supervision hook resolves the wrong repository on nested checkouts: run from metasystem/ inside the agentic-tools-m3 clone it derives the git toplevel (the outer repo) as its metasystem root, reports a bootstrap world (no ledger, no steward), and its turn evidence never lands where health's hook-freshness role reads - m3 has hook-freshness=dead since enrollment with the hook firing every turn. DONE means the hook resolves the metasystem project root deterministically on nested checkouts, its turn evidence lands, and hook-freshness goes alive, proven by a fixture running the hook from a nested layout
- Origin: main
- Next step: Fence lifted: the 2026-09-02T06:53Z set-budget already raised attemptLimit to 10, so the WAITS ON WIDO note it replaced is stale; the claim now refuses for a different reason, APPROVAL_REQUIRED, which only Wido clears with goal approve. Built and verified on m1 2026-09-05, uncommitted on the current tip: the hook passes the INSTALLATION root to proc find-ancestor (the adapters live there, so the Git toplevel resolved to nothing and every session start in this layout refused arming) and writes its evidence under the installation, pinned by a new fixture that runs the real ancestor walk under a real signature-matching parent - the template scenario beside it stubs find-ancestor, which is why the suite never saw this. Also fixed: the open-work scanner read a fenced example in goal-scope-bounds-design.md as that plan's own next step and refused turn ends over work nobody wrote. All 15 hook fixtures pass. Landing needs a reviewed implementation chain (commit.sh refuses without --chain), so on approval: claim, dispatch a code-critic, fold, close, land. Still owed after that: steward, health, lease and report turn-verdict still take the outer root, which is why hook-freshness reads dead against the installation; that is design revision 3's full consumer sweep.
- OpenedAt: 2026-09-01T07:25:56Z
- Revision: 12
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=6 at=2026-09-01T22:28:54Z

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
- 2026-09-01T23:35:31Z GTZ8BA0T05NB9DV9KWXY7EJSZH-m0b-6638932d release actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-02T06:53:09Z 504Q8HYVBQSC634T92TB9CZEMJ-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-05T10:02:47Z 3N1CNSERR0KTC4XW0G7TNWT6X3-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=supervision-hook-wrong-root
Integrity: sha256=059f5bec667e522ba407d9161fc65baef73ede025712a9d8b22a99b498dcdd16
