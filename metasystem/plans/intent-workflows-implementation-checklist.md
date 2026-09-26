# Remaining proof and usability checks

Root-owned handoff for the continuing implementation. These are obligations of
the accepted designs, not approval or certification. Updated 26 September 2026.
Read alongside the latest builder return; delete obsolete entries only after
recording actual proof in the design matrix.

## Positive authority fixtures

TestIntentRepairAuthority currently proves mostly real-owner refusals. Existing
synthetic authority can exercise success without new production escape hatches:

- internal/missionrunner/wall_test.go:stageHumanShell writes the existing
  METASYSTEM_FAKE_PROCESS_IDENTITY_FILE with the test PID terminal=true in an
  exact metasystem.runtimes=fake root. lease.ClassifyAt walks the ancestor terminal
  fixture, so adapter child -> real owner grandchild can classify HUMAN.
- TestResolveTaintThroughWrapper already drives freshly built owner commands for
  actual restore/adopt. This caller classification also fits brain declare and
  withdraw. Preserve real facts/constraints, not just successful argv.
- Enrolled proof for engine-floor/carry is different: humanauthority.KernelReader
  does not consume that table. Use the existing --fixture-human-authority in a
  strictly synthetic owner test through the test-only delivery.process seam;
  retain the assertion on the actual public adapter argv before appending the
  existing fixture flag. Never expose a new production bypass or treat --by as
  proof. The exact existing authority fixture setup must be followed.
- Existing positives: TestGoalRepairReachesAcceptRemoteAndRequiresBy uses
  runGoalRepairWithInputs + goalAuthorityReadFacts; TestRepairAcceptsARewoundRemoteUnderAHuman
  proves the accepted-ref mutation. TestReconcilePublishesHandEditsUnderTheHuman
  and TestRefreshOnlyCompletesADiedRefresh exercise real snapshots/publisher and
  per-instance fake repositories. Synthetic subprocess reconcile needs explicit
  METASYSTEM_OWNER_LINEAGE. Migrate uses migrateWithStatus and the deterministic
  fakeLegacyMigrationEndpoint; TestMigrateRerunSurvivesTheCutoverCheckout proves
  repetition. TestEngineFloorIsAProvenHumanRootHistoryLine and the existing CLI
  goal-cli-fixtures engine-floor scenario provide enrolled-proof positive facts.
  TestCarryLifecycleLandsThroughTheJournal and TestHCL45ConfirmedCarriedEntryIsIdempotent
  prove retained-word lifecycle; full land.sh transport is a separate obligation.
- Caution: TestIntentRepairAuthority's current nonzero/refused + argv-prefix
  assertion DOES NOT prove authority caused refusal. Missing ledger/config/input
  may pass it. The in-memory newDeliveryBed repository does not cross a real
  subprocess boundary. Strengthen the asserted reason and use the existing
  positive owner/CLI fixtures; never label an arbitrary failure authority proof.

## Checkpoint-3 observations to close

- Root CLI corpus: artifacts/agents/intent-workflows-verification/checkpoint-3.
  Invalid retry/admin/question combinations exit2; focused new help works.
  Root still 45 lines, land summary exposes read units/certified chains, design
  help exits2, start lacks ui/machine. Already required by parent/planning designs.
- check still forwards raw health remedy strings. Implement the accepted typed
  public mapping without regex guesses or fabricated authority instructions.
- Capped-critic retry must get past the older implementer-only timeout/budget-cap
  branch, use actual death proof and same cap; drive real follow_up machinery.
- Legacy job closure needs review job dispositions before close can become only
  compatibility. Full capability, not disappearance from help, is the criterion.
- All parent and planning named fixture requirements remain. Reuse one meaningful
  connected fixture when it really proves multiple rows; do not add empty test
  wrappers or duplicate an owner solely to satisfy a name.

## Source question for the final fresh user journey

Normal project-rules guidance still sends an independent bare-diff read to an
internal launch. Root is checking whether public review of manual changes already
covers that exact capability without requiring a paid builder or hidden commit
metadata. Source audit CONFIRMED the diagnostic bare-diff capability gap: review G needs a
UnitRunner result, review job requires an implementer job, and review commit
requires a Goal-Unit commit in that goal's range. Existing launch read provides
the missing diagnostic capability. Root is separately designing its smallest
public subject; do not invent it inside the parent implementation. Diagnostic
feedback must never be promoted into a landing attestation. Manual delivery's
existing owner sequence is being checked before root claims complete capability.

## Required namespace correction from bounded source audit

Actual source gaps,26 September, continuation5: status ui/checkout/job/unit cannot
show the identically named goal's work (intent_process.go status dispatcher), and
wait file/proof/job/unit/question/resume cannot wait for that goal's running work
(intent_work.go early subject refusal). `show --goal G`, build G and repair review G
already work. This is the same bounded subject-disambiguation issue as accepted
manual IM-C3, not a new workflow or another prose review chain.

Add `status goal G [--work NAME]` -> runIntentStatusGoal and make `wait goal G
[--work NAME]` without --for -> runIntentWaitWork. Explicit --for landing|human-act
keeps the current goal-event observer. Preserve legacy flag-based event selectors
and their defaults. Focused help states the qualified goal behavior; bare shortcuts
stay for unambiguous names. Generated status/wait/review continuations must use a
qualified goal form when its name is a reserved subject, never target existence
heuristics. TestIntentReservedGoalNames drives all affected ids through real
selection and proves ui/file/job target commands still mean their original targets.
These are required IW-1/2 capability proofs, reviewed with the full implementation.

## Verification under the user's explicit machinery bypass

The goal remains queued, no synthetic approval/claim/budget. Governed test run and
test verify correctly require those facts even for diagnostic purpose. Final
verification will use the existing external diagnostic drivers rebuilt against
an immutable candidate, calling the real protected-contract selector and
proofrun.RunTestPlan with risk3/2/3/2,9 workers bounded by inherited allowance,
all selected groups and exact whole-project tree. Goal-less CLI plan uses zero
risk and misses accumulation providers, so cannot stand in for this selection.
Diagnostic observations will never be labeled governed attempts/certificates.
Full coverage requires real RunGoGateTests result plus unchanged ratchet judge
and full internal-package inventory; affected-package tests alone collect no
coverage. Existing source driver is outside repository in the prior verbs evidence
verification-tools directory. Root will inspect/rebuild it at final candidate.
Focused dispatcher diagnostics use METASYSTEM_FIXTURE_ONLY=dispatch-b through the
normal parent, preserving capability minting/cleanup; filtered pass is not full gate.

## Real whole-close fixture is already available

Root source agent checked continuation5's claimed unmirrored barrier. There is
NO pre-close mirror guard in closeChain/finishedReview/criticClosure. The join
requires a real findingRegister, while dispatch.sh close mirrors all terminal
members BEFORE register-close/CloseCheck. A record unmirrored before actual
whole close is expected, not grounds to omit the positive assertion.

Reuse TestIntentConnectedJourneyRealClose in intent_connected_journey_test.go,
especially finish(): actual dispatchcore.ComputeReadSubject, subject.json,
reviewedTree-bound fake-provider return, completed/sessionId/capabilitySnapshot/
endedAt/reviewRoundLimit metadata, real CritiqueRegisterAdvance. realCloseOwner
in intent_delivery_test.go supplies evidence.root OUTSIDE checkout, capability
snapshot and record-locks; actual dispatch.sh performs mirror/close/check/stamp.
Adapt that authentic fixture to review commit --dispositions then real collection/
publication. No pre-mirroring shortcut, fake closure stamp or success-only callback.
The current commit closure test still fakes closure/attestation and is not that proof.
