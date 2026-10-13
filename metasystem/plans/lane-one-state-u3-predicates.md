# lane-one-state U3 (D2): one predicate library; resume, deploy, the steward, the batch, the questions and the returns call it

Working Mode: Implement. Goal lane-one-state, design `plans/designs/lane-one-state.md` (read it whole). Worktree on main AFTER the lane-recovery stack has landed (resume.go, deploy.go, giveback.go must be present); builds on U1+U2 (branch build/los1). One commit; never push. Production code ≤ 250 lines (a move counts its new lines only). Behaviour-preserving: every caller decides exactly what it decided before, except where two callers disagreed (then the corrected version wins and the commit message names the difference). No test may read ../../testing.json or plans/goals; no "witness" or "certification" tests.

## Build
1. `internal/landing/state/predicates.go` (pure functions over plain fact structs; no reads, no locks, no import of plain or lane). Each has its decision table in its doc comment, one row per outcome:
   - `MergeOwnership(f MergeFacts) Ownership` — facts: record present, job id, job alive, writers settled (resume.go:248 `resolutionSettled`, :269 `ResolutionWritersSettled`), checkpoint and tip, main moved since the checkpoint, closed; outcomes `Abandoned` (no live job, writers settled, record not closed), `Owned` (a live job or writers active), `Settled` (closed or merged), `Unknown` (a fact unreadable) with the fact that decided it.
   - `Admissibility(f AdmissionFacts) Admission` — facts from `batchAdmission` (batch.go:872), `checkBatchLocked` (batch.go:627) and `admissionAtSeal` (health.go:638): eligible, cap reached, policy, held by an incident, the person's seal, returned, already landed; outcomes with the word each caller prints today.
   - `Actor(f ActorFacts) Who` — from `ActProvenance` (batch.go:93 `PersonSelection`, return_bound.go:23 `returnOrigin`): `Person`, `Agent`, `Unknown`, with the enrollment/lineage fact that decided it.
   - `QuestionPremise(f PremiseFacts) Premise` — from question.go:47 `LaneQuestionPremise`: `Current`, `Gone`, `Unknown`.
   - `StopOwner(f StopFacts) Owner` — from the steward's bound (steward.go:100 "a person's stop or proof, an agent step, or a proof is active") and the keeper's loop-stop closing: `Person`, `Loop`, with the stop's record field that decided it.
2. Switch the callers to the library, each keeping its own reads and acting on the outcome: `Resume` (resume.go:32, the abandoned-merge abort), `deployMerge` (deploy.go:145), `StewardStep` bounds (steward.go:95-110), `batchAdmission`/`checkBatchLocked`/`admissionAtSeal`, `LaneQuestionPremise`/`WithdrawStaleLaneQuestion` (question.go:34), `returnOrigin`. Delete the local decision code they replace.
3. The no-copy audit, `cmd/metasystem/audit_lane_predicates_test.go` (go/ast over internal/landing and cmd/metasystem): (a) each named caller contains a call to its predicate; (b) no function outside `internal/landing/state` is named or doc-commented as deciding these five things (names matching abandoned|owned|admissib|eligib|isPerson|premise|stopOwner outside the package fail with the file:line); the allow-list is empty and stays in the test.

## Tests
Table tests per predicate, one case per decision-table row, plus the disagreement cases of 2026-10-12 (findings 212-215: a merge judged stale from "main moved" alone; an abandoned record of another merge; 226: deploy missing resume's case) each asserting the corrected outcome. Every existing test of the six callers stays green unchanged except where the commit message names a corrected disagreement.

## Checks
`go vet ./internal/landing/state ./internal/landing/plain ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/state ./internal/landing/plain ./internal/landing/lane ./internal/layering ./internal/testenv`; `go test -count=1 -run 'TestAudit|TestLandingResume|TestLandingDeploy|TestLandingSteward' ./cmd/metasystem`.

Always also: `go test -count=1 -run 'TestEveryStatefulActionRepeatsAsSuccess|TestEveryPublicActionHasAnIdempotencyRow|TestIntentObjectHelpGroupsEveryActionOnce|TestIntentPublicCoverage|TestHelmDrain|TestPlainLaneProve' ./cmd/metasystem` (the audits the full gate caught in 244 and 251).

## Do not
No lifecycle rules (U4), no verb guard (U5), no wording changes (U6). Nothing acts on the shadow.
