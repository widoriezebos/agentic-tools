# idle-with-backlog-alarm

- State: claimed
- Intent: Wido 2026-09-02 ('Machinery must make this impossible and it still happened'). ROOT CAUSE, corrected by codex critique of the seat's first analysis (which was partly refuted): NOT that idle went undetected - the steward fired alert-99020c96 and transport-submitted it 96 min before the human stepped in. The real failures are (a) CAUSAL: the turn-verdict blocks a quiet stop only ONCE per unchanged backlog digest (internal/goal/turnverdict.go:124-130,411-425), so a second stop on the same queue passes and the seat idles lawfully; (b) SEMANTIC: openwork.go:30-63 counts queued goals but returns WorkNone when none is claimed LOCALLY, and verdict.go:90-97 maps that to ActNone - the steward's own decision owner deliberately calls unclaimed shared backlog 'no work'; (c) DELIVERY: transport submission was treated as sufficient without proving human receipt. DONE (Wido's bar = impossible, not detected): the turn cannot end while claimable budgeted work exists and nothing is in flight, EVERY stop not once, unless an explicitly authorized stop; the steward's no-work predicate counts claimable shared backlog as work; proven by a fixture where a SECOND unchanged stop still cannot go quiet.
- Origin: main
- Next step: INTENT: a causal invariant, not another alarm. CONSTRAINTS: fix the two decision owners (turnverdict.go every-stop block; openwork/verdict shared-backlog-is-work), not a parallel health role that leaves them inert (codex F-OWNER-006); the fresh-ledger 'claimable' binding must be canonical not a stale snapshot (F-LEDGER-009); an explicit human stop is the one lawful quiet exit. FREEDOMS: whether hook-freshness gets the applicability fix here or splits to hook-enrollment-per-checkout. Budget: raise as needed under the envelope, highest priority. TEST SHAPE: second unchanged stop with claimable work + nothing running = blocked; explicit stop = quiet; zero claimable = quiet; work in flight = quiet.
- OpenedAt: 2026-09-02T05:47:20Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=480 activeJobLimit=1
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=3 at=2026-09-02T05:49:11Z
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-09-02T06:37:33Z revision=6
- StopCapability: generation=6 revision=6 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-09-02T05:47:20Z AQM3YVNMMY0P3G0XSSDCBZDT2T-m0-c5dbf036 open actor=human:Wido targets=idle-with-backlog-alarm
- 2026-09-02T05:47:24Z 2EBGS3Z4TCJA8HTR582E5XWMB6-m0-c5dbf036 set-budget actor=human:Wido targets=idle-with-backlog-alarm
- 2026-09-02T05:47:27Z 97954GZMVSHFPT2ZBFTPPJN22Y-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T05:49:11Z KDSKX0K6Z5HZ5WCQRPZ7GH5G1H-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T06:36:46Z 7SYQ3X7MTH7C5BGSJSBRG7SZFP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T06:37:33Z HSXTQDWRBABPTV7M0FJ49HGDBD-m0-c5dbf036 set-budget actor=human:Wido targets=idle-with-backlog-alarm
Integrity: sha256=ae338ca6a438c27190fb3318dac2cd2fa2c5ac88abcd26a215cc6f9645441229
