# fleet-coordinator-brain

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: the brain drafts and proposes and never approves, builds or dispatches, so a wrong brain wastes Wido's attention and misorders the queue rather than authorizing anything; novelty 1: the session form is a role packet and a standing instruction with the narrator's digest as input, explicitly no new build per R-67-m1; exposure 3: it is the single seat between Wido and every node in the fleet; accumulation 2: its absence has now produced a concrete failure - on 2026-09-05 m1 stood in for both the brain and the missing runner, hand-drove eight dispatches and four critique rounds, and Wido had to ask why the system was so complicated"
- Tier: 3
- Intent: One brain seat for a fleet of headless nodes. The nodes run the mission runner on approved goals and talk over the fleet conversation channel; the brain never builds anything. It works with Wido on the backlog: drafts items from his words and from what the nodes report, classifies them into tiers, proposes budgets and order, keeps the queue honest, and hands him the one act only he does, approval for execution. It watches the cluster (which node runs what, who is stuck, what a question means), answers what the records can answer, and escalates to him only what needs his word. It is the narrator plus judgment plus a hand on the queue. Three constraints: the brain is never a bottleneck (nodes proceed on rules and records; the cluster keeps working the approved queue when the brain is down); authority stays with Wido (the brain drafts and proposes, never approves execution, never mints rulings, carries his words verbatim); the first form is a Claude session on m1 with the channel, the ledger and the census, forbidden from dispatching work, and later a runner role of its own.
- Origin: main
- Next step: The role packet is written at plans/fleet-coordinator-brain-role-packet.md and is UNLANDED: landing it needs this goal approved and claimed, because the landing stamps a Goal-Item that must be held by this machine. Its voice has landed (fleet-slack-channel done) and R-67-m1's order puts it after the first headless run, which is itself paused. One word from Wido approves it; the packet then lands as a records carriage with no build.
- OpenedAt: 2026-09-03T12:14:47Z
- Revision: 4
- Arc: headless-fleet
- BudgetExceptions: 0

History:
- 2026-09-03T12:14:47Z E43RG504Z3FDHSSZBFKFC3DQMH-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=fleet-coordinator-brain
- 2026-09-03T12:18:42Z C3VEM86YH4WEJK7PGD0KJAJ8N1-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=fleet-coordinator-brain
- 2026-09-03T12:20:28Z M535KP64SQ8J85JZD1V6A29N90-m1-7bb1546e set-arc actor=m1+main-1788333680-2840-7f79f4 targets=fleet-coordinator-brain
- 2026-09-05T18:33:04Z 00Q9KJPNZV2XGNY1TQ1R3GZ600-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=fleet-coordinator-brain
Integrity: sha256=010d670a1b2338503f787ae9a03f56340db94b533ae49e0af0d5969d5c6f77ec
