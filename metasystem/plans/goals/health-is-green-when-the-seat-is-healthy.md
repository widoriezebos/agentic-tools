# health-is-green-when-the-seat-is-healthy

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Health verdicts every seat, the steward and the Fleet page read; a wrongly retired check hides a real failure, a permanently red one hides all of them"
- Tier: 2
- Intent: A seat's health verdict is green when the seat works and red only for something someone can act on (Wido 2026-10-03: 'are our supervisors doing their job well?'). Measured 2026-10-03 11:20 on all eight checkouts (six seats, m1e, the lane): the process supervisors do their job (runner, supervision owner, watcher, census and narrator alive everywhere), but every checkout reads 'unhealthy' permanently: 4 to 6 of 23 health roles are dead at 5 consecutive failures marked NO_LAWFUL_REMEDY, so a real failure cannot be seen and the Fleet page's health means nothing. The standing reds: retro-debt (a receipt owed for arc-goal verbs-match-intent, on every checkout); context-budget (thresholds of 105-250 thousand tokens against sessions of 320-830 thousand, obsolete since Wido removed context caps on 2026-09-20); trunk-red (deep validation cadence overdue; deep mode cannot pass on this Mac); ledger-attention (the ledger moved and no session ran a journaling verb, 73 to 1887 minutes); capability-snapshots stale (claude, codex, devin); stop-capability-epoch on m1f and m1j (claim epoch 9 against lease epoch 1 after a budget raise from another checkout, AUTO_HEAL_ENDED; machinery item 49); session-main on the lane (it never has one); repo-watcher on ui (success owned by a previous runner pid); stop-hook-duration on m1e. Wanted: each standing red is fixed, retired or given a remedy the machinery runs itself; a role with no remedy after N ticks files one defect (goal blocked-seats-route-to-the-machinery-not-a-supervisor) instead of staying red forever; health roles that do not apply to a checkout kind (the lane) are not evaluated there.
- Origin: main
- Next step: Tier-2 design: one row per standing red (cause, fix / retire / automatic remedy, owner), the no-remedy escalation rule, per-checkout-kind role sets; Astra critique; build. Evidence: health.json of the eight checkouts at 2026-10-03 11:20 CEST.
- OpenedAt: 2026-10-03T09:22:09Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-03T09:22:09Z YF6TX22AGCF2X3HD0MT1F7VVPN-m1e-718ba0eb open actor=human:Wido targets=health-is-green-when-the-seat-is-healthy
Integrity: sha256=b58303bb21e1445958515c17bfee5c483f8fdd6f565e4de81f903c2a2a34106c
