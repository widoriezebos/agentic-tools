# idle-with-backlog-alarm

- State: queued
- Intent: Wido's order 2026-09-02 (verbatim: 'Machinery must make this impossible and it still happened'): a machine holding NO claim while claimable budgeted goals exist and nothing runs is a dead health verdict on the steward tick - the class that produced m0's soft stall, which three guards each missed for a different reason (the turn-verdict hook configured one directory above the session and never loaded; its death reported every tick as hook-freshness=dead and misread as cosmetic; the idle-with-backlog verdict absent from the steward taxonomy, parked in watch-verb's unbuilt acting side). DONE means: the steward's health output goes dead with a plain message naming the claimable count and idle duration when this machine holds no claim, has no non-terminal delegate jobs, and the ledger shows claimable budgeted goals, for longer than a configured grace (default 15 minutes); proven with tests including the grace boundary and the no-claimable-work quiet case.
- Origin: main
- Next step: INTENT: the steward alarms on idleness the way it alarms on burn. CONSTRAINTS: same health-role pattern as claimed-goal-delivery (thread now, fail-safe direction: unreadable ledger = dead); detection only - claiming FOR the machine is watch-verb acting-side scope; the grace default is config (metasystem.steward.idle-grace-minutes), never hardcoded. FREEDOMS: role name; whether pinned-to-other-machine goals count as claimable (argue it). Budget 4h per Wido's standing word (8h if bigger, split beyond). Also fold the hook-freshness lesson: that role's DEAD verdict joins the alert episodes at the same severity as steward-runner death - the watchdog's death is never cosmetic. TEST SHAPE: idle+claimable past grace = dead naming count and duration; idle with zero claimable = alive; active claim = alive; hook-freshness dead raises an alert episode.
- OpenedAt: 2026-09-02T05:47:20Z
- Revision: 1

History:
- 2026-09-02T05:47:20Z AQM3YVNMMY0P3G0XSSDCBZDT2T-m0-c5dbf036 open actor=human:Wido targets=idle-with-backlog-alarm
Integrity: sha256=35b14de78a9711ad4d24822154fdf51c2322f2fc5e62170cb87f799c74560714
