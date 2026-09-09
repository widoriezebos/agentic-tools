# effort-by-complexity-per-model

- State: approved
- Priority: 3
- Sequence: 98
- Risk: severity=1 novelty=2 exposure=2 accumulation=1 basis="severity 1: a wrong effort costs money or review depth, nothing breaks; novelty 2: a generic effort scale and its per-model translation do not exist yet, the hazard table is the only effort authority today; exposure 2: every dispatch on every runtime; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Wido's word 2026-09-06 21:3x, corrected by him 2026-09-06 21:5x: effort per hazard class, per model, from the config, in the MODEL'S OWN WORDS. No generic metasystem effort scale and no translation table: the config already names the model per role and runtime (role.<role>.model.<runtime>=<model>), so the effort is written beside it in that model's native vocabulary, keyed by hazard class, for example role.implementer.effort.claude.design-bearing=high and role.implementer.effort.codex.mechanical=medium, with role.default.effort.<runtime>.<class>=<value> that a role line shadows, resolved in the same order as the model keys. Today internal/dispatch/hazard.go fixes the reasoning effort per hazard class for every runtime and model (medium for MECHANICAL, xhigh for DESIGN-BEARING and DESTRUCTIVE-REACH) with no config key. DONE means: (1) the config keys above, resolved per hazard class; (2) the engine validates a value only as one the named runtime accepts (Codex minimal/low/medium/high/xhigh; Claude its effort names; a runtime without an effort switch accepts only its no-op) and refuses a bad value at dispatch naming the key and the accepted list; (3) hazard.go keeps only the admission floor per class: a configured value weaker than the floor refuses, never silently raised; stronger is allowed; (4) the job record shows the effort sent and the key it came from; (5) fixtures: a role line shadows the default; a bad value refuses naming the key; under-floor refuses; the no-switch runtime; the recorded effort equals what the adapter received. fable-effort-high (xhigh to high for Fable) becomes the first config line this feature carries.
- Origin: main
- Next step: STATE 2026-09-07 01:35Z (m1c): three build rounds; critique r1 (ebc-critic1, tree c9826186) found one material defect (follow-ups on pre-change job records would refuse for the missing effort source) plus two small ones, all folded in round three (ebc-build1-r3, brief plans/effort-by-complexity-per-model-fold-r3-brief.md); dispositions records/misc/effort-by-complexity-per-model-critique-r1-dispositions.md. Seat proofs green on round two (dispatch, adapter, config, command packages). The dispatch fixture bed cannot certify the new legs until the temporary authority horizon is decided (goal fixture-review-by-date-rolls-over): recorded as owed. Then critique r2 (the last), close, land (area), rebuild, up, done.
- OpenedAt: 2026-09-06T19:54:04Z
- Revision: 16
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:55:31Z revision=8 opid=B583HCFB873QFD1HYHE7QVA258-m1-7cd0bd60 authority=proven digest=91187c8eb9fa3217c89f3a08f4717f90174922b58f6069d987025a538ecd4cc8
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=9 at=2026-09-07T00:18:41Z

History:
- 2026-09-06T19:54:04Z 4XJ816WDYXXAX5DTCHHQM8TDDW-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=effort-by-complexity-per-model
- 2026-09-06T21:08:28Z W36FZ380Y61Z23KNQNB4PJ1FDV-m1-7cd0bd60 approve actor=human:Wido targets=effort-by-complexity-per-model
- 2026-09-06T21:53:23Z WBFFC91PZHKMBNHWYQESZNX0Q6-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=effort-by-complexity-per-model
- 2026-09-06T21:53:41Z TW1AYKWN99FDNK014JR12KFVMD-m1-7cd0bd60 unapprove actor=human:Wido targets=effort-by-complexity-per-model reason=intent corrected: model-native effort words, no generic scale
- 2026-09-06T21:54:14Z J68CTNEBJZ52YWMQZ0285NM2J0-m1-7cd0bd60 approve actor=human:Wido targets=effort-by-complexity-per-model
- 2026-09-06T21:54:57Z 763FVXAZT18NBR1PEK4AZ4MTM0-m1-7cd0bd60 unapprove actor=human:Wido targets=effort-by-complexity-per-model reason=intent corrected: model-native effort words, no generic scale
- 2026-09-06T21:55:16Z 0RHMQFYYHAXDVZ56720SQ7TE9Y-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=effort-by-complexity-per-model
- 2026-09-06T21:55:31Z B583HCFB873QFD1HYHE7QVA258-m1-7cd0bd60 approve actor=human:Wido targets=effort-by-complexity-per-model
- 2026-09-07T00:16:50Z MBM0TQQWBY870YKSFVM7KEDWAA-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=effort-by-complexity-per-model
- 2026-09-07T00:18:41Z T29JKEVADY2ZFMH39V7NNQQBR7-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=effort-by-complexity-per-model
- 2026-09-07T00:19:47Z 5ARR74DN4SJ1T8CTKYZCVDAMQC-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=effort-by-complexity-per-model
- 2026-09-07T00:44:50Z W6Q4YT8F4TB6NCSDJK8JE77ETG-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=effort-by-complexity-per-model
- 2026-09-07T01:04:25Z 8A48DMM4T40WT7VE7GSP85XX6B-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=effort-by-complexity-per-model
- 2026-09-07T01:23:03Z ZXKQHCQ8BV5ER4AG1MSS3AE2HD-m1c-7cd0bd60 park actor=m1c+main-1788680061-17829-64951c targets=effort-by-complexity-per-model reason=Parked 2026-09-07 01:35Z by m1c (park-and-move-on): chain ebc-build1 is built through three rounds (tree 6d5883cc) and reviewed twice; the second critique found one material transition defect (the chain-closure gate compares a critic record's recorded effort word by string equality to the new floor, so pre-change critic records refuse to close after landing). The fix needs a fourth build round and a third critique; the tier-2 box allows two. Wido's word needed: a review-round exception, or that the closure-gate fold may land as a MECHANICAL chain. Dispositions in records/misc/effort-by-complexity-per-model-critique-r2-dispositions.md; worktree ebc-build1 holds the built tree.
- 2026-09-08T16:04:51Z 89GFS3NXN2V1QKG93JSPPSR0G4-m1-7cd0bd60 set-priority actor=human:Wido targets=effort-by-complexity-per-model reason=priority-order subject=effort-by-complexity-per-model from=unranked to=3:98 requested-sequence=98
- 2026-09-09T09:24:18Z EA9066Q4PTX23C08Q79VGA4E3T-m1-1701c13c unpark actor=m1+main-1788940932-18533-7fa6c2 targets=effort-by-complexity-per-model
Integrity: sha256=f529d33baa070796a259a08597d86cea361dffc8f24b7a869106eeab6e5b835c
