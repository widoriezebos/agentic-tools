# repo-flag-resolves-one-root

- State: queued
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: a wrong resolution today produces an empty health report or a refused arm, nothing unsafe is permitted; novelty 1: the resolver already exists in the goal family and in up, this is wiring; exposure 3: every operator command and every hook on every machine passes --repo; accumulation 1: one resolver replaces three readings, nothing compounds"
- Tier: 3
- Intent: The --repo flag means the installation for health and the steward verbs but the git checkout for up and the goal family. In an adopted repository the two are the same directory, so the name is right there; in the template checkout the installation is metasystem/ under the git top, so 'steward restart --repo .' only works from inside metasystem/ and 'health --repo ..' silently reports an empty world instead of refusing. Wido (2026-09-06): the flag keeps its name and every verb respects the repository: any path inside the checkout is accepted, the scope is the git top, the installation is the directory holding metasystem.conf found by the existing template rule (goal.ResolveStateRoot), and a path that resolves to no metasystem.conf is refused, never reported on as a fresh world.
- Origin: main
- Next step: Small item, one implementer round: one resolver in internal/stateroot that takes any path inside the checkout and returns (scope, installation) or an error; health, health acknowledge-alert, and every steward verb call it in place of taking --repo literally; up keeps its behaviour but reads the same resolver; --metasystem-root stays as the fixture-only override. Fixture: health --repo <git top> and --repo <installation> report the same world from the template checkout and from an adopted fixture; health --repo <path with no metasystem.conf> exits non-zero with a refusal naming the path. Depends on nothing; coordinate with hook-root-resolver-design, which names a second state-root authority as a finding, so the two do not ship two resolvers.
- OpenedAt: 2026-09-06T08:19:18Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T08:19:18Z CY9Q25Z16BM04CE2CA62YQJDE8-m2-3ee187f1 open actor=m2+main-1788682156-7163-7f65ab targets=repo-flag-resolves-one-root
Integrity: sha256=6ca0c60cd68f6327f218e445e738f8b6f0b66241cda711caddc581b93d745ba9
