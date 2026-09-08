# codex-handshake-budget

- State: parked
- Priority: 3
- Sequence: 96
- Intent: The codex adapter's session handshake budget is a fixed ten seconds (scripts/agents/adapters/codex.sh:71, sessionEstablishedTimeoutSec) while a cold codex 0.148.0 start on the host Mac takes eight to fourteen seconds to emit thread.started, so a slow start is recorded as HANDSHAKE-FAILED and spends a cause-blind attempt - failing when slow, the class R-35-m3 names as a defect. DONE means a slow but progressing codex start is never a failure: the handshake waits on progress (process alive and its session file or first stream byte appearing) with a measured ceiling, and refuses only when the process is dead or silent past the ceiling
- Origin: main
- Next step: DUPLICATE OF codex-handshake-budget-load-fragile (claimed by m0b 2026-09-02 16:03Z, same defect, same R-35-m3 remedy): its design plans/codex-handshake-design.md rev 1 is landed and in Sol critique (job ch-crit-1c); this record's specimens (m1b jobs design-critic-049b1ce0…, two-bars-cc-crit-2; 8-14 s direct runs, plugins disabled changed nothing on m1b) are cited to that critique because they disagree with m1's 1-second plugins={} measurement. DO NOT CLAIM separately; conclude this goal when the other lands, or reopen it for whatever the landed chain leaves uncovered.
- OpenedAt: 2026-09-02T11:54:55Z
- Revision: 6
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=5 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=e0ba0dc4fa0314cce159795fdc8f747576bbf0617c64bb7a0b89c976c7f240ab
- Parked: by=m1b+main-1788333346-60696-6a3256 at=2026-09-02T16:48:55Z because=duplicate of codex-handshake-budget-load-fragile (m0b, same defect, same remedy); its specimens are recorded on that goal

History:
- 2026-09-02T11:54:55Z 72WQ01G8W1CKYESQ88QC3DACDZ-m1b-fad3674e open actor=m1b+main-1788333346-60696-6a3256 targets=codex-handshake-budget
- 2026-09-02T11:55:08Z R819GZ1EN9EQTGHSSV8ZYKC894-m1b-fad3674e set-budget actor=m1b+main-1788333346-60696-6a3256 targets=codex-handshake-budget
- 2026-09-02T16:27:21Z GJ8221XK0TTBJ0MYHS03BGAK4V-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget
- 2026-09-02T16:48:55Z EFHR1MARYWMSHEEEBPSYPVGD60-m1b-fad3674e park actor=m1b+main-1788333346-60696-6a3256 targets=codex-handshake-budget reason=duplicate of codex-handshake-budget-load-fragile (m0b, same defect, same remedy); its specimens are recorded on that goal
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=codex-handshake-budget reason=sweep
- 2026-09-08T16:04:45Z D4DCMXHANEDB0EQQ31QFJK4ZVH-m1-7cd0bd60 set-priority actor=human:Wido targets=codex-handshake-budget reason=priority-order subject=codex-handshake-budget from=unranked to=3:96 requested-sequence=96
Integrity: sha256=fa711070f1e3074d0e5429cdf5f9d79d4dbc01a857dceb291fe7a4909202f2e6
