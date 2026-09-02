# idle-with-backlog-alarm

- State: claimed
- Intent: Wido 2026-09-02 ('Machinery must make this impossible and it still happened'). ROOT CAUSE, corrected by codex critique of the seat's first analysis (which was partly refuted): NOT that idle went undetected - the steward fired alert-99020c96 and transport-submitted it 96 min before the human stepped in. The real failures are (a) CAUSAL: the turn-verdict blocks a quiet stop only ONCE per unchanged backlog digest (internal/goal/turnverdict.go:124-130,411-425), so a second stop on the same queue passes and the seat idles lawfully; (b) SEMANTIC: openwork.go:30-63 counts queued goals but returns WorkNone when none is claimed LOCALLY, and verdict.go:90-97 maps that to ActNone - the steward's own decision owner deliberately calls unclaimed shared backlog 'no work'; (c) DELIVERY: transport submission was treated as sufficient without proving human receipt. DONE (Wido's bar = impossible, not detected): the turn cannot end while claimable budgeted work exists and nothing is in flight, EVERY stop not once, unless an explicitly authorized stop; the steward's no-work predicate counts claimable shared backlog as work; proven by a fixture where a SECOND unchanged stop still cannot go quiet.
- Origin: main
- Next step: ARCHITECTURAL WALL FOUND, AWAITING WIDO (2026-09-02, after ~6 build rounds + terminal critique). THE WALL: Claude Code (the harness) caps how many times a Stop hook may block a turn - so NO hook-based gate can make idle IMPOSSIBLE; after the cap the turn ends regardless of our code. The steward (runtime-independent) actively re-engaging an idle seat is the ONLY floor that can make it impossible - that is goal idle-every-runtime-enforcement, and this finding means it is the ONLY path to Wido's bar, not an optional agnostic extra. Also: the seat's judgment to defer the marker-replay finding as forge-class was REFUTED by the critic (an honest agent reaches it) - a real bar-(a) hole, correction recorded. THE FORK: (a) LAND the hook gate anyway as the best-effort claude layer - fail-closed within its reach, blocks up to the harness cap, single-use marker with the replay hole fixed, steward escalation; it makes idle HARD + LOUD + impossible-by-accident, and is real value, but is NOT 'impossible' and cannot be alone; then idle-every-runtime-enforcement (steward re-engagement) carries the true bar. (b) DROP the hook-gate polishing and go straight to steward active enforcement (the only impossible path) on every runtime, folding the buildable hook pieces into it. m0 recommends (a): land the hard+loud layer now (four specimens all ended quietly; even a capped block would have caught them), and make the steward re-engagement goal the owner of 'impossible'. The harness cap is the reason no turn-exit gate alone suffices - this is the honest end of the hook approach.
- OpenedAt: 2026-09-02T05:47:20Z
- Revision: 15
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
- 2026-09-02T17:53:07Z WB778QJH8F1MASFCDRJFTRETNS-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
Integrity: sha256=4a46f7fd61a57f4869d646530aa70bded72aff70c7358cf3ba3018d55dad650e
