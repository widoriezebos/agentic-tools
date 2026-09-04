# Tiering machinery, part three: re-review on the fresh chain (chain str-build3b-cc1)

Reviewed tree f76faf663dbb5bfbc3281b8bbfcd7eac4a08f276 (chain str-build3b: the preserved part-three tree 71f3ac42 plus the correction). Critic: Claude Fable 5.1. Brief: plans/severity-tiered-rigor-build3b-code-critique-brief.md. Zero material findings; the chain closes and part three lands.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| STR3P3B-01 | noted | The full-battery measurement (6.67 minutes) is a lower bound: the dispatch fixture bed went red before the goal CLI leg ran, so the forty-minute default's margin over the full three-leg battery is unproven. Recorded for fixture-suite-drift-after-approval-gate: re-measure once the bed is green. | none |
| STR3P3B-02 | noted | The exact full-battery command is red on this host today (two dispatch fixture scenarios, pre-existing), so a root declaring gateWidth full cannot obtain a passing receipt until the bed is green. Not caused by this diff; same backlog item. | none |
| STR3P3B-03 | noted | A goal without a Tier line refuses tier-one landings (tier1-goal-tier-0-refused) during the classification migration; fail-closed by the strict comparison; the classify sweep is the remedy. | none |
