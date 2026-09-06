# design-pages-land-without-the-battery

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a wrong rule lets prose land unproven, which is exactly what prose is; novelty 1: the path classes already exist in the landing package; exposure 2: every design chain on a tier-3 goal hits this; accumulation 1: one rule, one fixture"
- Tier: 2
- Intent: A design page (one new markdown file under plans/) produced by a design chain must land through land.sh --chain without the full battery receipt. Tonight (2026-09-06) the brain seat design page was refused: 'chain is full-width (its goal's accumulation is 2 or more); make the full battery receipt'. The battery proves code; a page that changes no code earns none of it, and the workaround (land the page as a records carriage under records/misc/) hides design pages where nobody looks for them. Decide the rule (a chain whose certified change is prose-only under plans/ or records/ lands on the two bars alone, whatever the goal's accumulation) and build it in the landing package with a fixture: a full-width goal, a chain whose diff is one plans/ markdown file, lands without --test-receipt; the same chain with one Go file refuses as today.
- Origin: main
- Next step: Design-bearing? No: one rule in internal/landing (the full-width check keys on the certified change's path classes, not the goal alone), one fixture in the land fixtures. Sol builds behind the fixture, Fable reviews, land.
- OpenedAt: 2026-09-06T21:22:39Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T21:22:39Z SBN46GF1YFFA20FN7YNH5GVM77-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=design-pages-land-without-the-battery
Integrity: sha256=552d2a570f7c8458620d2a8bd4b7ff76a1be25097a812b01643f29a8a1c6ebf1
