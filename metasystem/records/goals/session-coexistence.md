# session-coexistence

- State: done
- Intent: Two coordinator sessions on one machine must not fight: land.sh acquires a working-tree landing lease (reusing internal/lease) before staging and releases after push — a second lander is refused with the holder named, never silently queued; codex-companion job ownership is recorded mechanically (session id on the dispatching goal or a jobs ledger) so an orphaned builder has one adopter. Prompted by Wido 2026-08-27 while running two m1 sessions; today's protections (goal quota, landing closure) make fights loud, not impossible.
- Origin: human
- Next step: Small mechanism goal (4h): landing lease + job ownership; queue after the severity build decision.
- Concluded: Split and absorbed (backlog triage 2026-08-31, Wido's order to combine): the job-ownership half landed as the L12 claim machine (2637083 + GroupOwnership, jobs ledger, ownership-patch verb) - orphaned builders have one owner on record; the landing-lease half is now a named requirement on land-verb-pruning slice 1, where the land verb internalizes into the engine and can hold a real lease with the holder named. Nothing of this goal's scope is unowned.
- OpenedAt: 2026-08-27T05:37:43Z
- Revision: 2

History:
- 2026-08-27T05:37:43Z 39JDBCEQD9DAXANP76CC9PTETW-m1-bf243850 open actor=human:wido targets=session-coexistence
- 2026-08-31T10:25:11Z H0W4GD41YB85EHHF83TB8XCHFP-m2-bc1be9cb done actor=human:wido targets=session-coexistence
Integrity: sha256=0a844fabc48ab2e14318891eb4f1c2c2323517e9f9729c36774728f033e12821
