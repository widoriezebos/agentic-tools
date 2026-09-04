# dispatch-fixture-critic-close-register-fold

- State: done
- Tier: 1
- Intent: The dispatch fixture suite's dispatch scenario, once past its permission-envelope leg, fails at a chain-close leg with 'cannot close a critic chain whose register is folded through round 1 while terminal round 2 exists' (internal/dispatch/close.go line 59): the material-stop landing (78018dd5, m3) made close refuse a critic chain whose finding register was not advanced to its terminal round, and the fixture's leg closes such a chain without advancing the register first. Seen seat-side on m2 2026-09-04 14:03Z with the preflight-warning fix applied; the leg was latent behind five earlier reds. DONE means the leg advances the register (job critique-register-advance) before the close the way a seat does, or asserts the refusal if that is what it tests, and the dispatch scenario proceeds past it.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one fixture leg): build, run dispatch-fixtures.sh seat-side, land through a chain; box 1h/3/60m/1. Approved under R-76-m2 once picked.
- Concluded: Landed b5ee69ff: both critic-chain closes advance the register first, the fixtures' goal openings answer the four risk questions, and the channel fixture's budget questions carry their box. The scenario's next reds are scheduled: goal:dispatch-fixture-recollection-wallclock-cap, goal:channel-fixture-answer-history-drift.
- OpenedAt: 2026-09-04T14:08:57Z
- Revision: 5
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T16:53:26Z revision=2 opid=G1566MNAY17FFJW23DXN0C3KZ7-m2-5fcf08ab authority=relayed digest=4637dcf5c10129034a9018ae9bc6fdb2eb25f492d673c4872d9dda469a2000d8 reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-04T16:57:05Z

History:
- 2026-09-04T14:08:57Z E88YC1N4G1SYZRF6VNWTVM9RVE-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=dispatch-fixture-critic-close-register-fold
- 2026-09-04T16:53:26Z G1566MNAY17FFJW23DXN0C3KZ7-m2-5fcf08ab approve actor=human:Wido targets=dispatch-fixture-critic-close-register-fold authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
- 2026-09-04T16:54:07Z 6FY147XGKCT884Y9J7APYWVJX0-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=dispatch-fixture-critic-close-register-fold
- 2026-09-04T16:57:05Z YHEMCN0V62C0A2RDJG75YYQQPX-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=dispatch-fixture-critic-close-register-fold
- 2026-09-04T17:37:34Z EZ12TJJPQYYYDKKW1ZG82P1Z16-m2-5fcf08ab done actor=m2+main-1788441779-14484-82d6ed targets=dispatch-fixture-critic-close-register-fold
Integrity: sha256=9ec99a67dd227459cfaf5efa179e441b2360f4b99cf01f7c76deb8b44b311ae9
