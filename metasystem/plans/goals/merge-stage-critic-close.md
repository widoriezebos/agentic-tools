# merge-stage-critic-close

- State: queued
- Intent: The merge stage of validate conformance (internal/validate/conformance.go, the code-critic chain checks) accepts a landing only when a code-critic chain root carries chainClosed, but a critic root dispatched at DESIGN-BEARING reach can never close: its own hazard closure (internal/dispatch/hazard.go, validateIndependentCritiqueReference) demands a distinct independent critique of the critique, while the implementer's closure demands a DESIGN-BEARING critic (maximal effort). So --stage merge refuses every real chain with "is not closed", and the landing evaluator, which never needed the critic closed, is the only working gate. Make the two agree: hazard closure applies to work chains (implementer roots) and not to critic, warden or verifier roots, so a critic chain closes on its terminal completed round, and the merge stage binds to the closing critic named by independentCritiqueJobRef instead of scanning every critic chain for a clean one.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside two existing owners): build plus one code review, no design round. Specimen: the path-class manifest's first part on 2026-09-03; four closing-review chains, none closable, merge stage refused all four while the landing evaluator accepted the certified tree. Fixture legs: a DESIGN-BEARING critic root closes without a reference; an implementer root still refuses without one; merge stage passes on the closing critic and ignores earlier critic chains of the same implementer; a stale closing critic still refuses. Waits for human approval for execution.
- OpenedAt: 2026-09-03T11:22:04Z
- Revision: 3
- Arc: headless-fleet

History:
- 2026-09-03T11:22:04Z GP2DM3NA2BMTS2TZFWNQV2CTNR-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=merge-stage-critic-close
- 2026-09-03T12:17:20Z DEP9KXQ3568Z9V73VPKJ45MMY1-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=merge-stage-critic-close
- 2026-09-03T12:20:51Z 8800B7G7H57XBNW7EJZE9JH1FS-m1-7bb1546e set-arc actor=m1+main-1788333680-2840-7f79f4 targets=merge-stage-critic-close
Integrity: sha256=d9e4bc4d0089f7396ecc50f92a6d121b13cb06a7aced11b99f1798b162521341
