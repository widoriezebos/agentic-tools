# unit-run-truthful-and-self-advancing

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Lane-test findings 306, 307, 308 (2026-10-14): a work build run of goal process-changes-cover stalled 25 min after its launch ended, judged a RESULT: PARTIAL build as passed, spent a 15-minute proof the builder had reported unreachable, and its cancel left a base-comparison job pending; interventions S119, S120."
- Tier: 2
- Intent: A unit run judges its build from the builder's result block on a completed launch, finishes a recorded failure after its attribution, advances on its own launch events under the helm, checks a declared deadline before spending it, and speaks to the person in intent (no run ids, steps or record states). Design metasystem/plans/designs/unit-run-truthful-and-self-advancing.md (Id 01M4WVQA75GZ7MB3SRKZMV4NDD, accepted, Astra critique closed at round 3 on 0 material).
- Origin: human
- Next step: U1: result-block judgement on a completed launch and Continue finishing a recorded failure after its attribution (brief agentic-tools-evidence/lane-test-20261009/briefs/unit-run-truthful-U1.md; built by Codex in worktree agentic-tools-m1e-unitrun on 364e776f4, read by Opus 5.5, handed in by Wido)
- OpenedAt: 2026-10-14T11:50:12Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-14T11:50:12Z FB8G2D31X6F6JRRXMG709WRY84-m1e-c6925449 open actor=human:Wido targets=unit-run-truthful-and-self-advancing
Integrity: sha256=6a023cc40dc21646f8d17e5f33af21cbdd3098593503d173bc3ee7879f0f21ed
