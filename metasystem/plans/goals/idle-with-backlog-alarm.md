# idle-with-backlog-alarm

- State: claimed
- Intent: Wido's order 2026-09-02 (verbatim: 'Machinery must make this impossible and it still happened'): a machine holding NO claim while claimable budgeted goals exist and nothing runs is a dead health verdict on the steward tick - the class that produced m0's soft stall, which three guards each missed for a different reason (the turn-verdict hook configured one directory above the session and never loaded; its death reported every tick as hook-freshness=dead and misread as cosmetic; the idle-with-backlog verdict absent from the steward taxonomy, parked in watch-verb's unbuilt acting side). DONE means: the steward's health output goes dead with a plain message naming the claimable count and idle duration when this machine holds no claim, has no non-terminal delegate jobs, and the ledger shows claimable budgeted goals, for longer than a configured grace (default 15 minutes); proven with tests including the grace boundary and the no-claimable-work quiet case.
- Origin: main
- Next step: INTENT: the steward alarms on idleness the way it alarms on burn. CONSTRAINTS: same health-role pattern as claimed-goal-delivery (thread now, fail-safe direction: unreadable ledger = dead); detection only - claiming FOR the machine is watch-verb acting-side scope; the grace default is config (metasystem.steward.idle-grace-minutes), never hardcoded. FREEDOMS: role name; whether pinned-to-other-machine goals count as claimable (argue it). Budget 4h per Wido's standing word (8h if bigger, split beyond). Also fold the hook-freshness lesson: that role's DEAD verdict joins the alert episodes at the same severity as steward-runner death - the watchdog's death is never cosmetic. TEST SHAPE: idle+claimable past grace = dead naming count and duration; idle with zero claimable = alive; active claim = alive; hook-freshness dead raises an alert episode.
- OpenedAt: 2026-09-02T05:47:20Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-09-02T05:47:27Z revision=3
- StopCapability: generation=3 revision=3 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-09-02T05:47:20Z AQM3YVNMMY0P3G0XSSDCBZDT2T-m0-c5dbf036 open actor=human:Wido targets=idle-with-backlog-alarm
- 2026-09-02T05:47:24Z 2EBGS3Z4TCJA8HTR582E5XWMB6-m0-c5dbf036 set-budget actor=human:Wido targets=idle-with-backlog-alarm
- 2026-09-02T05:47:27Z 97954GZMVSHFPT2ZBFTPPJN22Y-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=idle-with-backlog-alarm
Integrity: sha256=ddd5cf6b0f00c03e651a388c4cd4a0e31a6ce52b5f4bf9e5d4fcc2f01ee7a8e3
