# effort-by-complexity-per-model

- State: approved
- Risk: severity=1 novelty=2 exposure=2 accumulation=1 basis="severity 1: a wrong effort costs money or review depth, nothing breaks; novelty 2: a generic effort scale and its per-model translation do not exist yet, the hazard table is the only effort authority today; exposure 2: every dispatch on every runtime; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Wido's word 2026-09-06 21:3x, corrected by him 2026-09-06 21:5x: effort per hazard class, per model, from the config, in the MODEL'S OWN WORDS. No generic metasystem effort scale and no translation table: the config already names the model per role and runtime (role.<role>.model.<runtime>=<model>), so the effort is written beside it in that model's native vocabulary, keyed by hazard class, for example role.implementer.effort.claude.design-bearing=high and role.implementer.effort.codex.mechanical=medium, with role.default.effort.<runtime>.<class>=<value> that a role line shadows, resolved in the same order as the model keys. Today internal/dispatch/hazard.go fixes the reasoning effort per hazard class for every runtime and model (medium for MECHANICAL, xhigh for DESIGN-BEARING and DESTRUCTIVE-REACH) with no config key. DONE means: (1) the config keys above, resolved per hazard class; (2) the engine validates a value only as one the named runtime accepts (Codex minimal/low/medium/high/xhigh; Claude its effort names; a runtime without an effort switch accepts only its no-op) and refuses a bad value at dispatch naming the key and the accepted list; (3) hazard.go keeps only the admission floor per class: a configured value weaker than the floor refuses, never silently raised; stronger is allowed; (4) the job record shows the effort sent and the key it came from; (5) fixtures: a role line shadows the default; a bad value refuses naming the key; under-floor refuses; the no-switch runtime; the recorded effort equals what the adapter received. fable-effort-high (xhigh to high for Fable) becomes the first config line this feature carries.
- Origin: main
- Next step: Tier 2, MECHANICAL: no design page. Sol builds behind the fixtures (read internal/dispatch/hazard.go, internal/dispatch/build.go reasoningEffort composition, internal/dispatch/roster.go for how role.<role>.model.<runtime> resolves, cmd/metasystem/adapter_runtime_verbs.go --reasoning-effort, metasystem.conf role lines), Fable code review, land.
- OpenedAt: 2026-09-06T19:54:04Z
- Revision: 8
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:55:31Z revision=8 opid=B583HCFB873QFD1HYHE7QVA258-m1-7cd0bd60 authority=proven digest=91187c8eb9fa3217c89f3a08f4717f90174922b58f6069d987025a538ecd4cc8

History:
- 2026-09-06T19:54:04Z 4XJ816WDYXXAX5DTCHHQM8TDDW-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=effort-by-complexity-per-model
- 2026-09-06T21:08:28Z W36FZ380Y61Z23KNQNB4PJ1FDV-m1-7cd0bd60 approve actor=human:Wido targets=effort-by-complexity-per-model
- 2026-09-06T21:53:23Z WBFFC91PZHKMBNHWYQESZNX0Q6-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=effort-by-complexity-per-model
- 2026-09-06T21:53:41Z TW1AYKWN99FDNK014JR12KFVMD-m1-7cd0bd60 unapprove actor=human:Wido targets=effort-by-complexity-per-model reason=intent corrected: model-native effort words, no generic scale
- 2026-09-06T21:54:14Z J68CTNEBJZ52YWMQZ0285NM2J0-m1-7cd0bd60 approve actor=human:Wido targets=effort-by-complexity-per-model
- 2026-09-06T21:54:57Z 763FVXAZT18NBR1PEK4AZ4MTM0-m1-7cd0bd60 unapprove actor=human:Wido targets=effort-by-complexity-per-model reason=intent corrected: model-native effort words, no generic scale
- 2026-09-06T21:55:16Z 0RHMQFYYHAXDVZ56720SQ7TE9Y-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=effort-by-complexity-per-model
- 2026-09-06T21:55:31Z B583HCFB873QFD1HYHE7QVA258-m1-7cd0bd60 approve actor=human:Wido targets=effort-by-complexity-per-model
Integrity: sha256=7ae3bd68d9dd485fbadd654796a492bb7cd73e2dc5a25a0a5f5e66e59fe6e13d
