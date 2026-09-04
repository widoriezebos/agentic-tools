# The finding-register scope fix: code review (chain frc-build1-cc1)

Reviewed tree b93188afb6c448f127b824b34fdbbad1ed60ae6f (chain frc-build1, round 1). Critic: Claude Fable 5.1. Brief: plans/finding-register-id-collision-across-chains-code-critique-brief.md. Zero material findings; the dispatch package passes seat-side (84 seconds); the chain closes and the fix lands.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| FRC-01 | noted | A root with neither a reviews target nor a reviewed tree never unions; only design-critic roots are subject-less, and the lawful union is tree-scoped. No fixture pins it; backlog if it matters. | none |
| FRC-02 | noted | Subject equality is the literal reviews target or tree, so critics of different correction rounds of one implementer chain are distinct subjects; matches the brief and the design's tree-scoped union. Recorded so the reading is known. | none |
| FRC-03 | noted | The subject resolver walks every record before the register check; milliseconds; optional cleanup. | none |
