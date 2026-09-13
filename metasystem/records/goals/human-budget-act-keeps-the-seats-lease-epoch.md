# human-budget-act-keeps-the-seats-lease-epoch

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: the seat's next dispatch refuses and the seat must release and re-claim, restarting its elapsed clock and accounting; novelty 1: the rebind reads the wrong lease; exposure 3: every human budget act made from a checkout other than the claimant's; accumulation 2: it happened twice in one day on one goal"
- Tier: 3
- Intent: goal set-budget and goal resume rebind the claimed goal through bindClaim with the CALLER'S checkout lease claim epoch. When the human runs them from a checkout other than the one holding the claim (the normal case: the human's enrolled terminal is on its own checkout), the goal's StopCapability.ClaimEpoch becomes that checkout's epoch, and the claimant seat's next dispatch refuses: 'goal breach-stop-wedges-seat revision 21 belongs to claim epoch 6, not current epoch 3'. Seen twice on 2026-09-09 and 10 (m1d): after a human set-budget and after a human resume, each remedied only by the seat releasing and re-claiming its own goal, which restarts its elapsed clock and accounting revision. A human budget act changes the tuple, not who holds the claim; the claim's lease epoch must stay the claimant's. DONE means set-budget and resume by a human keep the existing claim binding's machine, lineage and claim epoch, with a fixture where a human act from a second checkout leaves the claimant's dispatch admitted.
- Origin: main
- Next step: Appetite: 1h. Read bindClaim's callers in internal/goal/verbs.go (SetBudget) and internal/goal/stop.go (resumeRequest): both pass the caller's claim epoch; pass the existing binding's epoch when the actor is human and the claim stands. Canary: the fixture in DONE.
- Concluded: Absorbed into goal:hp-resume-takes-its-budget-from-the-ledger in the 2026-09-13 backlog consolidation on Wido's word; Human set-budget or resume keeps the claimant's lease epoch. Its clause is appended to that goal's next step.
- OpenedAt: 2026-09-10T08:42:32Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T08:42:32Z 9YARC9X274Q532WZH0Y6PNP7PA-m1-c6925449 open actor=human:Wido targets=human-budget-act-keeps-the-seats-lease-epoch
- 2026-09-13T08:19:28Z G2JHWFP42Y1CK94NHFQBPWG3SF-m1-c6925449 done actor=human:Wido targets=human-budget-act-keeps-the-seats-lease-epoch
Integrity: sha256=dac9e3937fe430ab6c6e5cf5a8199a70b8d5306f747d87dd1073797b46ec3704
