# steward-owned-execution

- State: queued
- Intent: The Go steward owns execution of scheduled and long-running work: suites and revived runs spawn as the steward's own announced children, so no cron/launchd exists and no launcher can sever identity (Wido 2026-08-24, after the detached-launch battery red)
- Origin: human
- Next step: Appetite: 3h for the design slice. Design first (custody arc discipline): the steward gains an execute surface — run a declared unit (battery, suite leg, revival) as a child it announces, supervises, and stamps; tickers (session hooks, an operator loop) only ask the steward, never spawn work themselves. This collapses suite-custody's detached-launch hole, suite-outcomes-as-steward-incidents' registration step, and the idle-watchdog ticker question into one mechanism; those items consume this design. Related: KI-43.
- OpenedAt: 2026-08-24T15:40:53Z
- Revision: 1

History:
- 2026-08-24T15:40:53Z 2GJPZ1DZ5F0117MGT83XFW9959-m2-bc1be9cb open actor=human:wido targets=steward-owned-execution
Integrity: sha256=abaccf4b68ae93a0e7cc27ee8de3ba64cadfacd5b10ec7959ac69e05ebd29aa4
