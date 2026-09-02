# idle-with-backlog-alarm

- State: claimed
- Intent: Wido 2026-09-02 ('Machinery must make this impossible and it still happened'). ROOT CAUSE, corrected by codex critique of the seat's first analysis (which was partly refuted): NOT that idle went undetected - the steward fired alert-99020c96 and transport-submitted it 96 min before the human stepped in. The real failures are (a) CAUSAL: the turn-verdict blocks a quiet stop only ONCE per unchanged backlog digest (internal/goal/turnverdict.go:124-130,411-425), so a second stop on the same queue passes and the seat idles lawfully; (b) SEMANTIC: openwork.go:30-63 counts queued goals but returns WorkNone when none is claimed LOCALLY, and verdict.go:90-97 maps that to ActNone - the steward's own decision owner deliberately calls unclaimed shared backlog 'no work'; (c) DELIVERY: transport submission was treated as sufficient without proving human receipt. DONE (Wido's bar = impossible, not detected): the turn cannot end while claimable budgeted work exists and nothing is in flight, EVERY stop not once, unless an explicitly authorized stop; the steward's no-work predicate counts claimable shared backlog as work; proven by a fixture where a SECOND unchanged stop still cannot go quiet.
- Origin: main
- Next step: FOURTH SPECIMEN (m1, relayed 2026-09-02): m1 ended its turn at 15:45 on a decision-ask while 84 goals were queued and a Claude-lane round needed no answer; Wido had to wake it. Nothing mechanical refused the stop - the turn-exit gate and the stop marker are not landed yet. This is a CLAUDE-lane seat idling, reinforcing fork option (a): the un-landed claude turn-exit gate would have caught exactly this; land it, do not wait for the every-runtime version. --- AT THE EVERY-RUNTIME WALL, AWAITING WIDO (third critique, not converging 6->6->7). The bar splits: the CLAUDE-runtime honest-agent case is closeable now (fail-closed + hook + single-use marker, plus six fixable fold holes the critique names: library-auth bypass, world-detection fail-open, 5s deadline not end-to-end, template state-root split, live-process-treated-as-work, marker no-hook lifecycle). IMPOSSIBLE on codex/devin requires the steward to ACTIVELY RE-ENGAGE an idle seat (the watch-verb acting side, a large separate mechanism). FORK: (a) close the six holes, land CLAUDE-runtime-impossible now, make every-runtime its own goal building steward re-engagement; (b) hold and build steward re-engagement first (big, likely joint-round per R-39-m0). m0 recommends (a). Four specimens now - m0's 8h night, m1 twice, m1 again 15:45 - all CLAUDE seats, all catchable by the un-landed claude gate.
- OpenedAt: 2026-09-02T05:47:20Z
- Revision: 14
- Budget: elapsedLimit=2d attemptLimit=14 reservedJobMinutesLimit=900 activeJobLimit=1
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=3 at=2026-09-02T05:49:11Z
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-09-02T17:42:44Z revision=14
- StopCapability: generation=14 revision=14 machine=m0 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-02T05:47:20Z AQM3YVNMMY0P3G0XSSDCBZDT2T-m0-c5dbf036 open actor=human:Wido targets=idle-with-backlog-alarm
- 2026-09-02T05:47:24Z 2EBGS3Z4TCJA8HTR582E5XWMB6-m0-c5dbf036 set-budget actor=human:Wido targets=idle-with-backlog-alarm
- 2026-09-02T05:47:27Z 97954GZMVSHFPT2ZBFTPPJN22Y-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T05:49:11Z KDSKX0K6Z5HZ5WCQRPZ7GH5G1H-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T06:36:46Z 7SYQ3X7MTH7C5BGSJSBRG7SZFP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T06:37:33Z HSXTQDWRBABPTV7M0FJ49HGDBD-m0-c5dbf036 set-budget actor=human:Wido targets=idle-with-backlog-alarm
- 2026-09-02T10:28:56Z 2E6WG9PM8Z0TZQMQM4TFPRK9CN-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T10:29:00Z S2RC4QA6A52AKSCE2SRX3ZN88S-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T10:58:04Z CM66N9MPVC694NCC9R7P1BZRDV-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T11:33:24Z 773WB1B2ACTGY6J7DF15C2EDFP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T14:27:34Z HPKFBT5J48N2R1EPWXZ5ZJ5573-m0-c5dbf036 set-budget actor=human:Wido targets=idle-with-backlog-alarm
- 2026-09-02T14:47:45Z ZKHTTHV7AJ3QE6DA71JAZCP6R1-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T15:52:16Z JE70VXX6THH25MRW8JBV28BAWF-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T17:42:44Z 2XFYRMCKF9WJCX4WP1JMV8QTCZ-m0-c5dbf036 set-budget actor=human:Wido targets=idle-with-backlog-alarm
Integrity: sha256=8ad705de886be3b279b3d111b74632bd571662ea44d83ff755f7fb44264d0e06
