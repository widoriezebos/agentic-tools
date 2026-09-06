# defect-analysis-gate

- State: approved
- Intent: Wido's word via the paper seat 2026-09-02 (docs/paper/12-learning-systems.md 'How a bug becomes a fix', human-ratified, pushed 1320cbfa): the analysis of a defect must EARN the fix item. DONE means a defect-fix goal cannot open (and/or a defect-fix dispatch cannot admit) unless a CHALLENGED analysis is attached - fail-closed at a real boundary, not prose. The analysis record keeps observed apart from suspected and names a cause; the challenge scales with the four risk questions: low-risk = the reproduction PLUS one causal test (vary/remove the suspected cause, watch the failure follow); severe/unfamiliar/wide-reaching = a fresh cross-family critic attacks the diagnosis (rerun the reproduction, hunt other causes, test over/under-coverage). The surviving analysis attaches to the goal as evidence, the intent states the outcome (defect gone, reproduction as the counting observation), ruled-out causes travel with it. COMPOSITION (must be ONE mechanism, not two): two-bars-for-changes's Defect-Proof - the red-on-old, green-on-new reproduction against baseline and candidate trees - IS this ladder's reproduction bar; reuse it, do not reinvent. Distinct from the queued siblings: critique-always (mandatory critique of the BUILD), design-gate-at-dispatch (dispatch needs a design artifact), commit-goal-binding (commit links a goal) - this gate is about the ANALYSIS earning the item.
- Origin: main
- Next step: INTENT: machinery makes a diagnosis survive challenge before a fix item exists. CONSTRAINTS: fail-closed at goal intake or dispatch admission (or both) refusing a defect-fix item carrying no challenged analysis; the reproduction primitive is two-bars' Defect-Proof, joined not duplicated; the challenge join is a candidate for the existing validate critique-closed mechanism; a refusal names what is missing. FREEDOMS (design under critique): where the analysis record lives and its schema, how the challenge is recorded and joined, whether the boundary is intake/admission/both, how the low-risk causal test is proven. ROSTER (R-25): Fable designs, Sol critiques the design, Sol builds, Fable critiques the build; the feature takes the FULL ladder (design, adversarial critique to closure, build, code critique, closure, landing). SLICING: expect an arc - the analysis-record schema + reproduction-join as slice 1, the risk-scaled challenge gate as slice 2, the intake/admission enforcement as slice 3. Budget is Wido's word at claim.
- OpenedAt: 2026-09-02T11:36:02Z
- Revision: 4
- Pinned: m1b
- Budget: elapsedLimit=3d attemptLimit=12 reservedJobMinutesLimit=720 activeJobLimit=2 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=4 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=25973739d6f21de31e3e9661d0da93b1328d8cab1b5995befe15a524ff704f5f

History:
- 2026-09-02T11:36:02Z X2TBEZEYHEK83X4YNEE0GCHB7S-m0-c5dbf036 open actor=human:Wido targets=defect-analysis-gate
- 2026-09-02T12:12:53Z ASCJK0K9R11JTSBZBT28RRXTWE-m0-c5dbf036 set-budget actor=human:Wido targets=defect-analysis-gate
- 2026-09-02T16:21:49Z Y4HWG7F2XT63GNKN5XRFZ49N82-m0-c5dbf036 set-pin actor=human:Wido targets=defect-analysis-gate
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=defect-analysis-gate reason=sweep
Integrity: sha256=ed633d27419a2c9cdd6b189b4caecf98c4735d6f40ff00d279e2962117e04d46
