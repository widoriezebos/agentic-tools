# idle-with-backlog-alarm

- State: claimed
- Intent: Wido 2026-09-02 ('Machinery must make this impossible and it still happened'). ROOT CAUSE, corrected by codex critique of the seat's first analysis (which was partly refuted): NOT that idle went undetected - the steward fired alert-99020c96 and transport-submitted it 96 min before the human stepped in. The real failures are (a) CAUSAL: the turn-verdict blocks a quiet stop only ONCE per unchanged backlog digest (internal/goal/turnverdict.go:124-130,411-425), so a second stop on the same queue passes and the seat idles lawfully; (b) SEMANTIC: openwork.go:30-63 counts queued goals but returns WorkNone when none is claimed LOCALLY, and verdict.go:90-97 maps that to ActNone - the steward's own decision owner deliberately calls unclaimed shared backlog 'no work'; (c) DELIVERY: transport submission was treated as sufficient without proving human receipt. DONE (Wido's bar = impossible, not detected): the turn cannot end while claimable budgeted work exists and nothing is in flight, EVERY stop not once, unless an explicitly authorized stop; the steward's no-work predicate counts claimable shared backlog as work; proven by a fixture where a SECOND unchanged stop still cannot go quiet.
- Origin: main
- Next step: AGENT-AGNOSTIC REQUIRED (Wido 2026-09-02: 'this should also work on codex and Devin'). The F-HOOK-COVERAGE gap is NOT deferred: the guarantee must bind every runtime, not only the claude Stop hook. DESIGN DIRECTION: the STEWARD TICK is the universal enforcer - it runs as machinery for every seat regardless of runtime and already owns the openwork/verdict decision; it (not only the per-runtime turn hook) must enforce idle-with-backlog fail-closed, with the claude turn-block as an additional fast path. The in-flight build idle-fix-bar-a does the runtime-INDEPENDENT verdict-logic folds (fail-closed, single-use holder-bound marker, liveness in-flight, legacy world) - all still correct; the added requirement is that the steward is an ENFORCING owner (escalate + the watch-verb acting-side re-engagement), not a detecting one, so a codex/devin seat with backlog cannot sit idle either. Fold this into the next round: name where every runtime's turn boundary meets the invariant, and make the steward the floor that does not depend on any hook loading.
- OpenedAt: 2026-09-02T05:47:20Z
- Revision: 10
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=480 activeJobLimit=1
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=3 at=2026-09-02T05:49:11Z
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-09-02T10:29:00Z revision=8
- StopCapability: generation=8 revision=8 machine=m0 claimEpoch=2 fenceEpoch=0

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
Integrity: sha256=921fca8453ef6014be1e47c7a33439f3bd6a2f80c29e0391d26837edd87dded7
