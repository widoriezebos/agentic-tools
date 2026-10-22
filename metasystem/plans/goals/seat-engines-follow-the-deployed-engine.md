# seat-engines-follow-the-deployed-engine

- State: parked
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="changes which engine binary the seats run on this computer; a wrong pin stops a seat until a person restarts it"
- Tier: 2
- Intent: Every seat on this computer runs the deployed engine after a deploy, without the steward copying it into each checkout's bin by hand; the central or per-checkout engine source is declared first (R-144-m1h decisions 2, 3, 5). Wido 2026-10-22: Q1 of a-proved-main-deploys-itself, a separate goal.
- Origin: human
- Next step: Write the design: which engine each seat pins (system restart, enrolment), the declared source choice, and the smallest change; Astra critique under the rare-case rule.
- OpenedAt: 2026-10-22T10:07:14Z
- Revision: 1
- BlockedBy: a-proved-main-deploys-itself
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Parked: by=human:Wido at=2026-10-22T10:07:14Z blocker=a-proved-main-deploys-itself because=blocked by a-proved-main-deploys-itself; returns when it is done

History:
- 2026-10-22T10:07:14Z XDV95WF4M98N9HWFSQSV7ES7PJ-m1e-4a1ba1c4 open actor=human:Wido targets=seat-engines-follow-the-deployed-engine,a-proved-main-deploys-itself
Integrity: sha256=dcc409068288f990c5216df68c47ff2b0efa1b84c31abc3036cbbedac47babbd
