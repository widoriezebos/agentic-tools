# seat-engines-follow-the-deployed-engine

- State: approved
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="changes which engine binary the seats run on this computer; a wrong pin stops a seat until a person restarts it"
- Tier: 2
- Intent: Every seat on this computer runs the deployed engine after a deploy, without the steward copying it into each checkout's bin by hand; the central or per-checkout engine source is declared first (R-144-m1h decisions 2, 3, 5). Wido 2026-10-22: Q1 of a-proved-main-deploys-itself, a separate goal.
- Origin: human
- Next step: Write the design: which engine each seat pins (system restart, enrolment), the declared source choice, and the smallest change; Astra critique under the rare-case rule.
- OpenedAt: 2026-10-22T10:07:14Z
- Revision: 3
- BlockedBy: a-proved-main-deploys-itself
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-22T10:07:59Z revision=2 opid=YAWVSCJGBSH5J8F40Y5N5A9Q4D-m1e-4a1ba1c4 authority=proven digest=9e01dff9b53c7779dd594b77528790b1d4f0102705efc0f5a03a0f2f120d50a5 episode=2

History:
- 2026-10-22T10:07:14Z XDV95WF4M98N9HWFSQSV7ES7PJ-m1e-4a1ba1c4 open actor=human:Wido targets=seat-engines-follow-the-deployed-engine,a-proved-main-deploys-itself
- 2026-10-22T10:07:59Z YAWVSCJGBSH5J8F40Y5N5A9Q4D-m1e-4a1ba1c4 approve actor=human:Wido targets=seat-engines-follow-the-deployed-engine
- 2026-10-22T18:23:23Z N93H3TM4RBARTKM3EFR213N6WE-m1e-c951e3fd unpark actor=human:Wido targets=seat-engines-follow-the-deployed-engine reason=blocker a-proved-main-deploys-itself is done; the park lifts
Integrity: sha256=2bd5a9ac159e847d7fba233d652969f0dbdd36e67fd04a1d6f47d1b512165cf9
