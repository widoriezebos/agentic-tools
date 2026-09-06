# effort-by-complexity-per-model

- State: approved
- Risk: severity=1 novelty=2 exposure=2 accumulation=1 basis="severity 1: a wrong effort costs money or review depth, nothing breaks; novelty 2: a generic effort scale and its per-model translation do not exist yet, the hazard table is the only effort authority today; exposure 2: every dispatch on every runtime; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Wido's word 2026-09-06 21:3x: dynamic effort by complexity, per model, from the config. Today internal/dispatch/hazard.go fixes the reasoning effort per hazard class for every runtime and model (medium for MECHANICAL, xhigh for DESIGN-BEARING and DESTRUCTIVE-REACH) and each runtime's own effort constants leak into that table. DONE means: (1) a small set of GENERIC effort levels owned by the metasystem (for example low, standard, high, maximal) that briefs, hazard classes and records speak in; (2) a translation table per runtime and model from the generic level to that model's own constants (Codex xhigh/high/medium, Claude's effort names, a runtime with no effort switch maps every level to nothing), kept in config with sane shipped defaults; (3) the complexity of the work selects the level: the hazard class remains the default selector, and the config can set a different level per model per complexity class so a roster can, for instance, keep Sol at maximal for design-bearing work while running Fable at high for the same class, or pay maximal only for the complex cases; (4) the job record carries the generic level, the translated constant and the origin of each; (5) the admission check compares generic levels, so a human's per-model choice is the authority and the hazard table is the default, not a fence; (6) fixtures drive every runtime's translation and one dispatch per complexity class; (7) the human page documents the keys. Goal fable-effort-high is the first, narrow instance (one key for one model) and folds into this one or lands ahead of it as the seat sees fit.
- Origin: main
- Next step: Design first (one page: the generic scale, the key grammar such as effort.<class>.<runtime>.<model>=<level> and effort.<runtime>.<model>.<level>=<constant>, the resolution order, what the job record shows, the fixtures), Sol critique once, then build in internal/dispatch and the adapters, Fable code review, land. Read hazard.go, build.go's reasoningEffort composition, the adapters' effort flags, and the model.tier keys in metasystem.conf before designing.
- OpenedAt: 2026-09-06T19:54:04Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:08:28Z revision=2 opid=W36FZ380Y61Z23KNQNB4PJ1FDV-m1-7cd0bd60 authority=proven digest=f15ab1188ea1c88cdf7eb9ec859e9620d869b70bb5f16ac9bb0256567206d4b4

History:
- 2026-09-06T19:54:04Z 4XJ816WDYXXAX5DTCHHQM8TDDW-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=effort-by-complexity-per-model
- 2026-09-06T21:08:28Z W36FZ380Y61Z23KNQNB4PJ1FDV-m1-7cd0bd60 approve actor=human:Wido targets=effort-by-complexity-per-model
Integrity: sha256=df54762bd85589a9d116d9ea57ef2d3dcaa884178173778c2a080488c78b0d6a
