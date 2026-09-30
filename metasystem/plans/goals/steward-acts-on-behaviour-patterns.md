# steward-acts-on-behaviour-patterns

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="A new steward role that can pause a component and open goals on its own; a false positive pauses useful work, a miss leaves a stuck system unnoticed; actions are reversible and reported"
- Tier: 2
- Intent: The steward detects behaviour patterns across the machinery and acts on them: each pattern is a small detector over signals the machinery already records, with actions from a fixed set (report first in status, pause the component, open a step-back goal, ask the person), thresholds and on/off in metasystem.conf; adding a pattern is one detector plus one config row. First patterns: a component that keeps failing in new ways (stagnation) and the machinery churning main with repeated records
- Origin: main
- Next step: Design the smallest extensible shape (detector interface, action set, once-per-incident record, config) with the two first patterns from the 2026-09-30 landing-lane evidence (/Users/wido/LocalStorage/agentic-tools-evidence/lane-stepback-20260930/memo.md); Astra critique; build. Consider folding steward-sees-stuck-capacity's detection in as a later pattern
- OpenedAt: 2026-09-30T16:34:33Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-30T16:34:33Z NYKAP8VJ5X06GBC87A8K6NM3ME-m1e-b6a4eb0a open actor=human:wido targets=steward-acts-on-behaviour-patterns
Integrity: sha256=a0d0eff484648c855f2bf0891e21d7f76800e77f33e0ad5faab9d2661be2c290
