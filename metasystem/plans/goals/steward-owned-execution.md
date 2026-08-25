# steward-owned-execution

- State: queued
- Intent: The Go steward owns execution of scheduled and long-running work: suites and revived runs spawn as the steward's own announced children, so no cron/launchd exists and no launcher can sever identity (Wido 2026-08-24, after the detached-launch battery red)
- Origin: human
- Next step: Appetite: 3h for the design slice. Design first (custody arc discipline): the steward gains an execute surface — run a declared unit (battery, suite leg, revival) as a child it announces, supervises, and stamps; tickers (session hooks, an operator loop) only ask the steward, never spawn work themselves. This collapses suite-custody's detached-launch hole, suite-outcomes-as-steward-incidents' registration step, and the idle-watchdog ticker question into one mechanism; those items consume this design. Related: KI-43. RESIDUE RECEIVED 2026-08-25 (suite-custody failsafe, R3-001): bash defers trapped signals until the foreground leg returns, so suite teardown guarantees custody-at-exit but not latency — the steward's runner owns hard-kill escalation (the second, unforgiving signal after a declared grace) for stuck legs.
- OpenedAt: 2026-08-24T15:40:53Z
- Revision: 2

History:
- 2026-08-24T15:40:53Z 2GJPZ1DZ5F0117MGT83XFW9959-m2-bc1be9cb open actor=human:wido targets=steward-owned-execution
- 2026-08-25T19:20:02Z YF8VS4CM7CCPKVYRKASZ6XQQX7-m2-bc1be9cb edit actor=m2+mac-coordinator targets=steward-owned-execution
Integrity: sha256=fc5db0ba07d7f5985d434dbc3cf60eec7a433759b27fe378c394f337248b74ce
