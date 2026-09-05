# fleet-coordinator-brain

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: the brain drafts and proposes and never approves, builds or dispatches, so a wrong brain wastes Wido's attention and misorders the queue rather than authorizing anything; novelty 1: the session form is a role packet and a standing instruction with the narrator's digest as input, explicitly no new build per R-67-m1; exposure 3: it is the single seat between Wido and every node in the fleet; accumulation 2: its absence has now produced a concrete failure - on 2026-09-05 m1 stood in for both the brain and the missing runner, hand-drove eight dispatches and four critique rounds, and Wido had to ask why the system was so complicated"
- Tier: 3
- Intent: One brain seat for a fleet of headless nodes. The nodes run the mission runner on approved goals and talk over the fleet conversation channel; the brain never builds anything. It works with Wido on the backlog: drafts items from his words and from what the nodes report, classifies them into tiers, proposes budgets and order, keeps the queue honest, and hands him the one act only he does, approval for execution. It watches the cluster (which node runs what, who is stuck, what a question means), answers what the records can answer, and escalates to him only what needs his word. It is the narrator plus judgment plus a hand on the queue. Three constraints: the brain is never a bottleneck (nodes proceed on rules and records; the cluster keeps working the approved queue when the brain is down); authority stays with Wido (the brain drafts and proposes, never approves execution, never mints rulings, carries his words verbatim); the first form is a Claude session on m1 with the channel, the ledger and the census, forbidden from dispatching work, and later a runner role of its own.
- Origin: main
- Next step: The session form's role packet is landed at records/misc/fleet-coordinator-brain-role-packet.md (a records carriage; plans/ turned out not to be the register's path class). It binds the next brain session: never build, never dispatch, never approve, never a bottleneck; when the work needs a node and none is running, say so and stop. Still owed for the session form per R-67-m1: the standing instruction wired into the seat's boot so it loads unprompted, and the narrator's digest as its input. The runner-role form is a tier-3 design afterwards.
- OpenedAt: 2026-09-03T12:14:47Z
- Revision: 7
- Arc: headless-fleet
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:human:Wido at=2026-09-05T21:14:27Z revision=5 opid=G2JAANGCE4VB9ADXEPMJXEA6K0-m1-a4f8999f authority=relayed digest=cce26a4e1a354ce8451020f4d14bfcbbc17533f5d69ffcef25dd08547a68313f reviewBy=2026-09-06
- Claimed: machine=m1 lineage=main-1788594343-3833-fb64b9 at=2026-09-05T21:14:33Z revision=6 accountingRevision=6
- StopCapability: generation=6 revision=6 machine=m1 claimEpoch=5 fenceEpoch=0

History:
- 2026-09-03T12:14:47Z E43RG504Z3FDHSSZBFKFC3DQMH-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=fleet-coordinator-brain
- 2026-09-03T12:18:42Z C3VEM86YH4WEJK7PGD0KJAJ8N1-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=fleet-coordinator-brain
- 2026-09-03T12:20:28Z M535KP64SQ8J85JZD1V6A29N90-m1-7bb1546e set-arc actor=m1+main-1788333680-2840-7f79f4 targets=fleet-coordinator-brain
- 2026-09-05T18:33:04Z 00Q9KJPNZV2XGNY1TQ1R3GZ600-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=fleet-coordinator-brain
- 2026-09-05T21:14:27Z G2JAANGCE4VB9ADXEPMJXEA6K0-m1-a4f8999f approve actor=human:human:Wido targets=fleet-coordinator-brain authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="I approve fleet-coordinator-brain"
- 2026-09-05T21:14:33Z 354510KMRJW3FJ7WTD795D6N2W-m1-a4f8999f claim actor=m1+main-1788594343-3833-fb64b9 targets=fleet-coordinator-brain
- 2026-09-05T21:17:43Z ZXA56AZTGNJ4BHS36N311W1V12-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=fleet-coordinator-brain
Integrity: sha256=f4cf6ace00523fa14c094b230ce44ff67910f9175a492709933a9c1f9dc1db07
