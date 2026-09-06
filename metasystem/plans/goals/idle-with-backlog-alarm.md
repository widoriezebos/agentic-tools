# idle-with-backlog-alarm

- State: claimed
- Intent: Wido 2026-09-02 ('Machinery must make this impossible and it still happened'). ROOT CAUSE, corrected by codex critique of the seat's first analysis (which was partly refuted): NOT that idle went undetected - the steward fired alert-99020c96 and transport-submitted it 96 min before the human stepped in. The real failures are (a) CAUSAL: the turn-verdict blocks a quiet stop only ONCE per unchanged backlog digest (internal/goal/turnverdict.go:124-130,411-425), so a second stop on the same queue passes and the seat idles lawfully; (b) SEMANTIC: openwork.go:30-63 counts queued goals but returns WorkNone when none is claimed LOCALLY, and verdict.go:90-97 maps that to ActNone - the steward's own decision owner deliberately calls unclaimed shared backlog 'no work'; (c) DELIVERY: transport submission was treated as sufficient without proving human receipt. DONE (Wido's bar = impossible, not detected): the turn cannot end while claimable budgeted work exists and nothing is in flight, EVERY stop not once, unless an explicitly authorized stop; the steward's no-work predicate counts claimable shared backlog as work; proven by a fixture where a SECOND unchanged stop still cannot go quiet.
- Origin: main
- Next step: CLAUDE GATE LANDED a74ca7cb (2026-09-02): the hard+loud turn-exit gate is on main - a claude Stop cannot end a turn with claimable backlog and nothing in flight unless a human ran the session-stop command; fails closed on every within-hook path; single-use local-first human marker. RESIDUAL, all their own goals: true impossibility every-runtime (idle-every-runtime-enforcement, steward re-engagement); the harness block-cap boundary documented in code; conformance-runtime-state-litter (the turn-verdict-state cleanup tax). m0 releases - the actual incident (claude seats idling) is closed. SPECIMEN m1b 2026-09-06 (Wido: 'we sure as hell need to avoid that from ever happening'): after both pinned goals were done under an instruction to stop, the idle-backlog branch of the turn verdict (internal/goal/turnverdict.go enforceIdleBacklog) blocked every stop unconditionally; Claude Code re-prompted the seat on each block; the seat answered one line and stopped again; about thirty refusal rounds replayed the whole conversation as input before the human intervened, while the message claimed 'this refusal does not repeat for the same work'. The open-work branch already blocks once per signature; the idle branch has no such slot and ignores the harness's stop_hook_active flag. FIX SET (corrected the same hour: the unconditional block is the deliberate hard gate of a74ca7cb and must not revert to block-once): (1) the idle branch keeps blocking, but counts consecutive idle blocks per session against an unchanged world (same claimable digest, no new claim, no job in flight); at a small bound (three) it escalates instead of repeating: it mints a steward intent 'seat idle with backlog' (internal/steward/intervene.go MintIntent), records the seat's refusal as an incident, and returns success with its display so the turn can end; the steward's next tick consumes the intent and launches the existing unattended steward-continuation session, which claims the next approved goal and works it the metasystem way (Wido 2026-09-06: 'can the steward maybe intervene and take care of this?' - yes, first responder); the human idle alarm fires only when the steward cannot act (no approved goal, budget exhausted, dispatch refused) - never-idle is enforced by steward plus alarm plus human, never by an unbounded re-prompt loop; (2) the hook reads the harness's stop_hook_active flag as the repeat marker for that count; (3) the message text states the bound and the escalation truthfully. Design-bearing: it amends the hard gate, so Wido's word on the direction is required before the build. Machine conduct rule recorded in the seat's memory: raise an instruction conflict once, then follow the standing never-idle law rather than loop.
- OpenedAt: 2026-09-02T05:47:20Z
- Revision: 23
- Budget: elapsedLimit=2d attemptLimit=16 reservedJobMinutesLimit=1080 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=19 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=870f88355289cf53f40d05249a897ce9bc40d8f401d597bcb37d61b943ee85ae
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=3 at=2026-09-02T05:49:11Z
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-06T11:15:21Z revision=23 accountingRevision=23
- StopCapability: generation=23 revision=23 machine=m1b claimEpoch=1 fenceEpoch=0

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
- 2026-09-02T19:10:25Z XJFPR58RMV5Y2FC39XKC8DTJC9-m0-c5dbf036 set-budget actor=human:Wido targets=idle-with-backlog-alarm
- 2026-09-02T20:54:46Z RD1X1E9V9E59PQ8QB1Q5AGP7E2-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-02T20:54:50Z AKEC75KA19R9KZDA3XQZYP9Y84-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=idle-with-backlog-alarm reason=sweep
- 2026-09-06T10:40:19Z 24SRHFXXCP33KRM9FGMY2M28FV-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=idle-with-backlog-alarm
- 2026-09-06T10:45:44Z X5GC9C423RW0WHG27WXDZH592W-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=idle-with-backlog-alarm
- 2026-09-06T10:47:50Z 7W09ASY06P74G8K2J4T3SXH9KE-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=idle-with-backlog-alarm
- 2026-09-06T11:15:21Z CF813PSNME5AVHSP5YC67XY5NZ-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=idle-with-backlog-alarm
Integrity: sha256=3952c5dd9899b0c87fec7e4dbd530c750d28bda3a032aa762ee231419b9590d6
