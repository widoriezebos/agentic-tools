# fleet-coordinator-brain

- State: queued
- Intent: One brain seat for a fleet of headless nodes. The nodes run the mission runner on approved goals and talk over the fleet conversation channel; the brain never builds anything. It works with Wido on the backlog: drafts items from his words and from what the nodes report, classifies them into tiers, proposes budgets and order, keeps the queue honest, and hands him the one act only he does, approval for execution. It watches the cluster (which node runs what, who is stuck, what a question means), answers what the records can answer, and escalates to him only what needs his word. It is the narrator plus judgment plus a hand on the queue. Three constraints: the brain is never a bottleneck (nodes proceed on rules and records; the cluster keeps working the approved queue when the brain is down); authority stays with Wido (the brain drafts and proposes, never approves execution, never mints rulings, carries his words verbatim); the first form is a Claude session on m1 with the channel, the ledger and the census, forbidden from dispatching work, and later a runner role of its own.
- Origin: main
- Next step: Wido's direction, 2026-09-03 (R-67-m1): the fleet's target state is headless nodes coordinated through the channel plus this one brain working with him. Queued, not approved for execution. Order after the four features and the first headless run: the session form starts the day the fleet conversation channel lands (no new build: a role packet and a standing instruction that forbids dispatch, plus the narrator's digest as its input); the runner-role form is a tier-3 design afterwards. Depends on: fleet-slack-channel (its voice), human-approval-for-execution (the act it hands Wido), severity-tiered-rigor (the tiers it assigns), token-spend-fence (the ceilings it watches), first-headless-run (the nodes it coordinates).
- OpenedAt: 2026-09-03T12:14:47Z
- Revision: 1

History:
- 2026-09-03T12:14:47Z E43RG504Z3FDHSSZBFKFC3DQMH-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=fleet-coordinator-brain
Integrity: sha256=394a40139c719d7eb5cbca6aff7f737cf2871cdf9ed3258d5aa42fea5cdf1fd3
