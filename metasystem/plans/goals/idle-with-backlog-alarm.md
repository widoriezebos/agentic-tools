# idle-with-backlog-alarm

- State: claimed
- Intent: Wido 2026-09-02 ('Machinery must make this impossible and it still happened'). ROOT CAUSE, corrected by codex critique of the seat's first analysis (which was partly refuted): NOT that idle went undetected - the steward fired alert-99020c96 and transport-submitted it 96 min before the human stepped in. The real failures are (a) CAUSAL: the turn-verdict blocks a quiet stop only ONCE per unchanged backlog digest (internal/goal/turnverdict.go:124-130,411-425), so a second stop on the same queue passes and the seat idles lawfully; (b) SEMANTIC: openwork.go:30-63 counts queued goals but returns WorkNone when none is claimed LOCALLY, and verdict.go:90-97 maps that to ActNone - the steward's own decision owner deliberately calls unclaimed shared backlog 'no work'; (c) DELIVERY: transport submission was treated as sufficient without proving human receipt. DONE (Wido's bar = impossible, not detected): the turn cannot end while claimable budgeted work exists and nothing is in flight, EVERY stop not once, unless an explicitly authorized stop; the steward's no-work predicate counts claimable shared backlog as work; proven by a fixture where a SECOND unchanged stop still cannot go quiet.
- Origin: main
- Next step: AT THE ARCHITECTURAL WALL, AWAITING WIDO (2026-09-02). Codex critique + build both gap-stop on two facts that no single slice can close: (1) TurnVerdict fires only when a runtime loads the Stop hook - full coverage needs runtime-lifecycle enforcement (own goal); mitigated by seats being claude-runtime only, so covering the claude boundary + reliable hook enrollment closes the REAL idle-seat risk. (2) The accepted ledger is DELIBERATELY unauthenticated (ValidateAcceptedTree accepts any structurally-valid tree), so an agent with checkout write access can FORGE any in-repo human-stop authorization - true forge-proofing needs ledger authentication/signing, a foundational goal touching every human-authority verb (resume, set-obligation share this exact hole). THE FORK: (a) close the ACTUAL incident (an honest stop, no forge) - fail-closed turn-verdict + single-use holder-bound-expiring marker + legacy-world coverage + one liveness-checked in-flight predicate + reliable claude hook enrollment; impossible by accident or honest agent, forgeable only by deliberate byte-forgery which every human-authority verb already permits. (b) ALSO commit ledger-authentication as a foundational goal for true system-wide forge-proofing. m0 recommends (a) now + open (b) as its own goal; five of six folds are ready to build on that word.
- OpenedAt: 2026-09-02T05:47:20Z
- Revision: 9
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
Integrity: sha256=b394d89d92b8e0d2714263f1487c9e1870ddf98245a8d03da17a8b079134c1f8
