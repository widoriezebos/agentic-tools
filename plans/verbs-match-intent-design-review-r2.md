# Final design critique dispositions

Goal: verbs-match-intent. Critic: Fable 5.1, same session as round 1.
Root read the entire findings body and agrees: no material findings, full
scope covered, smallest sufficient design. Closed after round 2 with the
seven named implementation fixtures in the design.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| VMI-R2-01 | accepted | Existing unpark does support --under with --verified; a stopped claim is different. | State-specific attorney mapping in section 4; TestIntentResumeAttorney. |
| VMI-R2-02 | accepted | A newly minted run's lock is too late to deduplicate initial calls. | Goal+unit reservation within existing run store before newRun; TestIntentBuildConcurrentRepeat. |
| VMI-R2-03 | accepted | Existing admission consumes table rows; the caller's estimate must reach that format. | Generated row or selected units-page row; TestIntentBuildSizeInput. |
| VMI-R2-04 | accepted | dispatch.sh close is currently the complete closure owner, also used by missionrunner. | Invoke full owner; TestIntentCloseWholeOwner checks both invocation and behavior. |
| VMI-R2-05 | accepted | Existing batch-root and actual evidence determine routing. | Concrete policy in section 6; TestIntentLandRouteEvidence. |
| VMI-R2-06 | refuted | The defect class is retained, but the suggested edit --tier N remedy is not executable and lacks the required four risk answers/basis (goalsync_mutations.go:1846-1851 and orchestration.md risk contract). Never invent N. | TestIntentTierlessApprovalRemedy requires truthful missing inputs or an actual complete classification recipe. |
| VMI-R2-07 | accepted | Existing review-brief template and launch.read.model supply the missing concrete choices. | Named template/setting in section 6; TestIntentGeneratedUnitPlan. |
