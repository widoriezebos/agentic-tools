# build-jobs-fit-one-builder-context

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="A launcher check on every build; a wrong cap delays a job, never lands code."
- Tier: 2
- Intent: A build job never exceeds what its builder holds in one context. DONE: (1) the launcher estimates a job's changed lines from the unit rows of its page and refuses a job above 1,500 lines, splitting it into serial jobs on the goal branch; (2) every build return records the builder's compaction count and call count; (3) the retro reports jobs over the cap and compactions per job, zero over the cap after landing.
- Origin: human
- Next step: Evidence 2026-09-17: every 2,200-line Codex job compacted three times inside Codex's 258K window; Build B's 13 material read items were page-conformance drift, the mark of a builder that lost the page; effort high against xhigh changed neither tokens nor time (7.2M and 8.8M per job). No design first: brief the estimate, the refusal and the return fields, build, read. Lands on the launcher of delegate-launchers-become-go-verbs.
- OpenedAt: 2026-09-17T13:39:30Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-17T13:39:49Z revision=2 opid=E80BMY94DX3D1QGDWXDSME1P3E-m1e-c6925449 authority=proven digest=eb9c220c45e2adb1895b52b25afe2e9d8476adf4454042a155a15381ef1826fd

History:
- 2026-09-17T13:39:30Z J7NVPQ4V8JBT3C49N8JWA815HD-m1e-c6925449 open actor=human:Wido targets=build-jobs-fit-one-builder-context
- 2026-09-17T13:39:49Z E80BMY94DX3D1QGDWXDSME1P3E-m1e-c6925449 approve actor=human:Wido targets=build-jobs-fit-one-builder-context
Integrity: sha256=3b27c0868b7030497fefb3ee97b569810da0e23f3a41a10622cdafd5367b55be
