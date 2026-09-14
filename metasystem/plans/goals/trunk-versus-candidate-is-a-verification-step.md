# trunk-versus-candidate-is-a-verification-step

- State: approved
- Priority: 1
- Sequence: 53
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: without a same-input comparison a behaviour change hides behind a text change, as a Stop-message build did on 2026-09-13; novelty 2: the engine has no notion of running one input against two trees; exposure 3: every landing that changes an output the seat or the human reads; accumulation 2: each hand-crafted comparison is redone per landing"
- Tier: 2
- Intent: Delivery-efficiency phase D. The engine runs one input through trunk and through the candidate and reports where their outputs differ, as a verification step a landing can require, so a change that claims to alter only text is proven to alter only text. Why: on 2026-09-13 a build quietly closed two Stop allowances and was caught only by a scratchpad script comparing every Stop-decision assertion against trunk; on 2026-09-14 the real cause of a hook regression appeared only when the seat ran the same fixture row against a trunk engine and diffed the bytes. Both comparisons were crafted by hand each time. DONE: metasystem test compare --tree TREE --input ROW runs a named fixture row or command against the accepted tree and the candidate tree with the same input and clock, and reports the byte diff of their outputs classified as decision, text or evidence; a landing may declare which classes it is permitted to change and the gate refuses others; a fixture proves a decision change is refused when only text was declared. Absorbs the decision-surface script that lives in a scratchpad today, with stop-decision-surface-is-a-gate as its first declared class.
- Origin: human
- Next step: Design in the verification-loop program page: what an input is (fixture row, command, Stop payload), how the accepted tree is materialised beside the candidate, how outputs are classified, and the declaration a landing carries. Then build the verb, the classifier and the gate refusal.
- OpenedAt: 2026-09-14T19:26:09Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T19:26:15Z revision=2 opid=AA518S7DT15ZAQCA093Y1EYRVV-m1e-c6925449 authority=proven digest=3a306357c87cc7d1d983338ca68cafd137b42d032b55b484caa7a317ba3a3e01

History:
- 2026-09-14T19:26:09Z F328BFJ3QRXCKXGDJQ0YCMNB25-m1e-c6925449 open actor=human:Wido targets=trunk-versus-candidate-is-a-verification-step
- 2026-09-14T19:26:15Z AA518S7DT15ZAQCA093Y1EYRVV-m1e-c6925449 approve actor=human:Wido targets=trunk-versus-candidate-is-a-verification-step
- 2026-09-14T19:26:21Z 1AX3TZ3WQKWEBGH71X9DJ7X51T-m1e-c6925449 set-priority actor=human:Wido targets=trunk-versus-candidate-is-a-verification-step reason=priority-order subject=trunk-versus-candidate-is-a-verification-step from=unranked to=1:53 requested-sequence=append
Integrity: sha256=4d0ca3f830834eeb10d8bab1c3fc3fd55eb7f0947c556c4baead7b4aeb6ba593
