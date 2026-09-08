# steward-owned-execution

- State: approved
- Priority: 3
- Sequence: 12
- Intent: The Go steward owns execution of scheduled and long-running work: suites and revived runs spawn as the steward's own announced children, so no cron/launchd exists and no launcher can sever identity (Wido 2026-08-24, after the detached-launch battery red)
- Origin: human
- Next step: Appetite: 3h for the design slice. Boundary inputs recorded at plans/goals-drafts/steward-owned-execution-boundaries.md (provisional coordinator picks 2026-08-26, Wido may overrule): registry-bounded authority, per-run announced identity, one-unit-per-checkout with the freeze-worktree battery exception, complementary cross-cited relationship to gate-run-freeze. Design first (custody arc discipline): the steward gains an execute surface - run a declared unit (battery, suite leg, revival) as a child it announces, supervises, and stamps; tickers only ask the steward, never spawn work themselves. Consumes suite-custody detached-launch hole, suite-outcomes-as-steward-incidents registration, idle-watchdog ticker question. Case-study-day mapping (2026-08-30 audit): the steward's channel is the CARRIER for the burn-without-delivery tripwire (class 1, facts owned by the coordinator's ledger - see the burn-without-delivery-tripwire item) and for the counselor's ambient noticings (classes 2/4/5) - content-blind envelopes both ways per the responsibility matrix; the execute-surface design must keep the incident hook suite-outcomes-as-steward-incidents and incident-proposal-drafting consume. Related: KI-43. RESIDUE R3-001: steward's runner owns hard-kill escalation after declared grace.
- OpenedAt: 2026-08-24T15:40:53Z
- Revision: 7
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=6 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=e86ee5e011418018c4a9afde7aa610e3a7efadc0698f9a36e6bba5a232beed04

History:
- 2026-08-24T15:40:53Z 2GJPZ1DZ5F0117MGT83XFW9959-m2-bc1be9cb open actor=human:wido targets=steward-owned-execution
- 2026-08-25T19:20:02Z YF8VS4CM7CCPKVYRKASZ6XQQX7-m2-bc1be9cb edit actor=m2+mac-coordinator targets=steward-owned-execution
- 2026-08-26T21:45:17Z M440HRS0AQCP8XQZRN6TE8N5F0-m2-bc1be9cb edit actor=m2+mac-coordinator targets=steward-owned-execution
- 2026-08-30T06:24:53Z C5KPPRD32A4H27R10QVGK1XRFF-m2-bc1be9cb edit actor=m2+mac-coordinator targets=steward-owned-execution
- 2026-09-01T20:29:43Z V17HAAZ8D1T4VVPMM7J4Q2P7VS-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=steward-owned-execution
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=steward-owned-execution reason=sweep
- 2026-09-08T15:59:46Z 3YQFEJ4TZR3HFS054VXV40CWT1-m1-7cd0bd60 set-priority actor=human:Wido targets=steward-owned-execution reason=priority-order subject=steward-owned-execution from=unranked to=3:12 requested-sequence=12
Integrity: sha256=6d4c6634e3c0029ead64439abcc6c399cb5b33f5f5d804b4cb715b52a0c0231c
