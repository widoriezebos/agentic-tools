# health-is-green-when-the-seat-is-healthy

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Health verdicts every seat, the steward and the Fleet page read; a wrongly retired check hides a real failure, a permanently red one hides all of them"
- Tier: 2
- Intent: A seat's health verdict is green when the seat works and red only for something someone can act on (Wido 2026-10-03: 'are our supervisors doing their job well?'). Measured 2026-10-03 11:20 on all eight checkouts (six seats, m1e, the lane): the process supervisors do their job (runner, supervision owner, watcher, census and narrator alive everywhere), but every checkout reads 'unhealthy' permanently: 4 to 6 of 23 health roles are dead at 5 consecutive failures marked NO_LAWFUL_REMEDY, so a real failure cannot be seen and the Fleet page's health means nothing. The standing reds: retro-debt (a receipt owed for arc-goal verbs-match-intent, on every checkout); context-budget (thresholds of 105-250 thousand tokens against sessions of 320-830 thousand, obsolete since Wido removed context caps on 2026-09-20); trunk-red (deep validation cadence overdue; deep mode cannot pass on this Mac); ledger-attention (the ledger moved and no session ran a journaling verb, 73 to 1887 minutes); capability-snapshots stale (claude, codex, devin); stop-capability-epoch on m1f and m1j (claim epoch 9 against lease epoch 1 after a budget raise from another checkout, AUTO_HEAL_ENDED; machinery item 49); session-main on the lane (it never has one); repo-watcher on ui (success owned by a previous runner pid); stop-hook-duration on m1e. Wanted: each standing red is fixed, retired or given a remedy the machinery runs itself; a role with no remedy after N ticks files one defect (goal blocked-seats-route-to-the-machinery-not-a-supervisor) instead of staying red forever; health roles that do not apply to a checkout kind (the lane) are not evaluated there.
- Origin: main
- Next step: Tier-2 design: one row per standing red (cause, fix / retire / automatic remedy, owner), the no-remedy escalation rule, per-checkout-kind role sets; Astra critique; build. Evidence: health.json of the eight checkouts at 2026-10-03 11:20 CEST. FOLDED IN 2026-10-03 from stop-hook-health-cost: the stop-hook-duration red needs its root cause found (is the health call still the largest share of the Stop hook's time, per the unexplained 20-42 s regression of 2026-09-16, df5c833f0) and a real fix or automatic remedy, not a standing red.
- OpenedAt: 2026-10-03T09:22:09Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T09:22:21Z revision=2 opid=BYJBY67DBZ6Y6YKATXB10BCZE2-m1e-718ba0eb authority=proven digest=60b72b96e20792b1826e865de3c0f2a5fbd4ffdfd7909664c7639bce70d5737e episode=2

History:
- 2026-10-03T09:22:09Z YF6TX22AGCF2X3HD0MT1F7VVPN-m1e-718ba0eb open actor=human:Wido targets=health-is-green-when-the-seat-is-healthy
- 2026-10-03T09:22:21Z BYJBY67DBZ6Y6YKATXB10BCZE2-m1e-718ba0eb approve actor=human:Wido targets=health-is-green-when-the-seat-is-healthy
- 2026-10-03T09:56:08Z 27WPYVTK93Q8CB5K3NH2178CPR-m1e-718ba0eb edit actor=human:Wido targets=health-is-green-when-the-seat-is-healthy
Integrity: sha256=bc31f6f397b504fc1461e37dbf7cf91a2ed038c570421e5fb54e64b9ed295f0c
