Working Mode: review
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal return-normalisation-hides-boundary-mismatch)
Date: 2026-09-05

# Code critique: the return validator's path normalisation and the boundary check

Review chain rnb-build1 (implementer round 1) against the build brief `plans/return-normalisation-hides-boundary-mismatch-build-brief.md`. The change is in `internal/validate` (the `checkDiffBoundary` normalisation in `returncomplete.go` and the cumulative-boundary check in `conformance.go`) with tests.

Number every finding with the prefix RNB- (RNB-01, RNB-02, …), never a bare F-number: finding ids are unique per chain. A finding is material only if it changes what must be built and names the artifact. Judge: (1) an undeclared changed path is still reported with the exact text "changed paths fall outside the cumulative implementation boundary" whatever the boundary's path form; (2) the normalisation touches only entries that resolve to an existing file under the metasystem root and never hides a mismatch; (3) the fixture's workspace-root `source.txt` case passes as it did before b7d119b3; (4) the unit test reproduces the fixture leg; (5) no assertion weakened. Return the findings register with dispositions you recommend.
