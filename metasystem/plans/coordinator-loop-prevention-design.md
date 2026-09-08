# Shared proof admission and finite delivery

Status: proposed for independent design critique. The shared admission and coverage design is not closed. The isolated witness error-propagation correction has the mechanically complete contract below and may proceed under the small-change provision in docs/orchestration.md; it still requires independent code critique.
Goal: coordinator-loop-prevention. Author: main Codex/Astra, xhigh.
Source baseline: 8ed97738, with subsequent goal-journal commits only.
Review budget: three rounds maximum, failsafe round 3; stop at the first round with zero material findings. Fixture-expressible findings may move to named implementation obligations only under the existing critique skill's rules. No fresh critic root resets that budget.

## Required outcome

Every supported agent runtime uses the same decision before MetaSystem starts an expensive proof. A running or completed proof cannot become a new unaccounted attempt merely because the coordinator changes runtime, run name, log path, or scratch directory. Successful coverage is consumed by normal landing when its relevant inputs still match. A real gate failure propagates without an implicit full-gate rerun.

The current production consumers are the validation, adoption, Go gate and commit coverage entrypoints. Direct arbitrary shell commands outside these entrypoints are not claimed to be governed. Mission stop-loss, existing delegate admission, goal approval and the human's right to authorize further work remain intact.

## Grounded diagnosis

The accompanying investigation records exact source anchors and independent traces. In particular:

- `proofrun.LaunchSuite` owns actual suite child creation but has no accepted-goal reservation or durable cross-run proof identity.
- `dispatch.EvaluateGoalRevisionAdmission` already judges the accepted goal, its stop fence, elapsed limit, attempts, reserved minutes and concurrency without requiring a governed obligation.
- The run store already owns non-job execution records and lifecycle locks. Its existing governed extension mixes budget facts with obligation-specific assumptions. A goal-bound ordinary run currently becomes BUDGET_UNKNOWN in `dispatch.ProjectBudget`; it cannot be made accounted by setting GoalId alone.
- A live stalled main produces steward notification; ordinary investigation stop-loss is explicitly invoked rather than automatically applied at proof admission.
- `witness-gate.sh` conflates a failed executed gate with an unavailable witness and can launch the plain full gate again.
- Commit coverage always reruns touched packages. Landing receipts do not carry coverage measurements. The existing landing `fullBatteryCommand` begins with `--fast` and is not evidence of full coverage.

## Ownership and scope

Extend the existing `run` record/store and `dispatch` admission/projection owners for ordinary proof accounting. Extend `proofrun` for proof identity and launch integration. Extend the existing `landing` receipt owner for reusable coverage evidence. Keep coverage judgment in `audit.ParseCoverage` and `audit.CheckCoverage`. Shell changes only pass arguments, environment/context and exit status to these Go owners.

Do not introduce a second budget counter, a new custodian service, a new provider-specific stopping policy, a generic workflow language, or a cache that pretends an old full suite just ran. Do not repurpose an obligation revision of zero as an accepted governed obligation.

## Ordinary proof reservation

Add an optional ordinary proof binding to the existing run record, mutually exclusive with GovernedAttempt. It records the owning goal's accepted claim revision, the existing budget epoch when present, reserved minutes, observed minutes at terminalization, proof scope, input identity, execution root, previous attempt reference when any, and retry decision evidence. Existing run identity, caller coordinates, status, kernel process custody and log/evidence fields remain the lifecycle owner.

The owning accounting root is separate from the scratch execution root. Top-level proof admission resolves the real coordinator's lease/goal at the accounting root, holds the existing goal-revision fence and run-store mutation lock, calls `EvaluateGoalRevisionAdmission`, and durably writes the reservation before any child starts. The implementation must preserve a single lock order with current delegate/governed admission; it must not perform a check and a later unlocked reservation.

Charge each new top-level proof through the existing `ProjectBudget`: one attempt, its positive reserved duration, and one active execution while live. Account terminal observed duration using the existing rounding convention. Failed launch, timeout, cancellation and unknown outcome retain their attempted work record. Ordinary proof records cannot be pruned while their accepted goal accounting epoch can still consume them; archive eligibility uses the existing accepted goal state. Source changes, new run identifiers and new scratch directories never reset accounting.

Use the existing configured dispatch cap as the default positive proof reservation/bound, with the accepted goal limit remaining authoritative. The run's own process bound must enforce its reservation. A caller may request a smaller positive bound; it cannot exceed the existing configured maximum or the accepted remaining budget.

The command layer wires admission callbacks, avoiding a proofrun-to-dispatch dependency cycle. An ordinary proof does not require a separate human-approved recurring obligation. Existing governed runs retain their current contract and validation.

## Shared launch and nested proof context

Apply the reservation path at the shared Go launch owner used by `validate-metasystem.sh` and `adopt-fixtures.sh`; route standalone full `go-gate.sh` and commit coverage through that same owner. Fast static checks and focused test commands remain the small checks they currently are.

An admitted parent carries its accounting root, run id and existing run custody evidence into its actual children. A child may join that reservation only after the run owner verifies live kernel ancestry and the recorded execution relationship; strings in environment variables alone are not authorization. Nested fixture work remains within the admitted parent's reservation and does not create a new top-level attempt for each fixture. A missing, dead, forged or mismatched parent context cannot silently exempt a new proof.

Standalone coordinator calls must resolve an accepted claimed goal before expensive proof launch. An ambiguous accounting root or goal refuses with the exact required input. Existing explicitly goal-free human/bootstrap and exact fake-runtime fixture paths keep their supported behavior; an active coordinator may not declare its claimed goal's work free by changing the scratch root. Verify this boundary using the existing lease/caller and delegate-custody owners rather than creating another runtime detector.

## Identity and repeat decisions

A proof identity contains its declared scope/command class, complete relevant source manifest including modes, effective proof configuration, selected section inventory, platform and toolchain identity. Reuse the existing manifest framing and behavior-surface policy. ENGINE equality may authorize coverage reuse only; it never stands for equality of an entire validation/adoption suite. Full-scope identity must include the complete non-runtime source/input inventory, not just ENGINE. Runtime name, process/run ids, log names and scratch locations are not distinguishing inputs.

Under the run-store lock, inspect previous attempts for the same owning goal and proof identity before reserving:

1. A live attempt refuses a duplicate launch and identifies that run for observation.
2. A successful attempt refuses a duplicate expensive launch and identifies its evidence for the delivery consumer. This is not a forged new success result or a new receipt.
3. A failed/unknown attempt requires an explicit retry decision. The decision names the prior run, the diagnosed cause, a concrete evidence artifact and why another attempt may succeed. The engine records the evidence digest and charges the new attempt to the same goal budget. Missing diagnosis or the words "technical issue" alone do not satisfy the decision contract.
4. Machinery may make at most one automatic retry for a typed transient failure before the proof body starts, such as a temporary process-resource error. It must record that classification and the failed attempt. A command's generic nonzero exit, test assertion failure, missing executable/configuration or unknown outcome is not automatically classified as transient.
5. Changed relevant inputs permit a new attempt within the same goal budget and retain the earlier outcome. An intentionally repeated measurement with unchanged inputs requires its declared measurement purpose and accepted goal authority; ordinary delivery verification cannot rename itself into that exception.

Retry evidence is accountable coordinator judgment, not a claim that Go can infer the semantics of arbitrary failure logs. Hard budget admission and retained history are mandatory even when the retry explanation is supplied. The independent critic must reject any route where a new root/id/runtime erases prior attempts.

## Failure propagation

In the clean witness path, distinguish inability to prepare/use the optimization before execution from the exit of an executed full gate. Once the full gate actually starts, preserve and return its nonzero exit. Do not launch the plain full gate after a failed executed gate. A successful gate that cannot publish a valid witness is an evidence failure, not authority to rerun automatically. Existing setup-only fallback remains available only before expensive execution has begun.

This change does not revive a dead controller witness, extend its lifetime by assertion, accept seed/fast results as full proof, or weaken any assertion.

## Coverage handoff and landing

Extend the existing landing receipt with a versioned coverage component written by the actual full gate producer after successful measured coverage, complete inventory judgment and unchanged relevant input checks. Bind it to ENGINE bytes and modes, behavior policy version, platform, complete Go toolchain identity and the exact selected coverage ratchet bytes. Preserve the actual package measurements and the producer's command class. Fast, seed, interrupted, incomplete, failed or mutated runs cannot publish reusable coverage authority.

The normal coverage boundary consults this component through Go. Matching evidence reuses its measurements and performs zero additional coverage test launches. Absent or mismatched evidence runs the existing affected-package coverage check. Goal and peer-record changes alone may preserve ENGINE equality; they do not invalidate coverage. Source/test/mode, toolchain/platform, policy or floor changes require fresh coverage. Preserve all ordinary index/working-tree, review-chain, static proof and landing checks.

Future final verification uses the real receipt producer and its supported bound; do not import the temporary R27 helper JSON as production authority. The existing 40-minute receipt-command limit is below the measured 56-minute full suite. Use the existing configured proof reservation/bound consistently, within the accepted goal budget, rather than introduce another timeout knob.

## Required focused proof

| Id | Severity | Requirement | Owning proof |
| --- | --- | --- | --- |
| LOOP-1 | HIGH | Ordinary proof reservation and delegate reservations share the same accepted budget, with no check/reserve race | Go admission/store integration tests, including simultaneous launches and exhausted budget |
| LOOP-2 | HIGH | Different runtime, worktree, log and run names cannot hide the same proof attempt | Shared entrypoint tests for Claude, Codex and Devin caller/custody contexts, including fresh scratch execution roots |
| LOOP-3 | HIGH | Live/successful duplicate launches create no child; retry requires retained diagnosis and budget | Go fake-command launch counter and persisted run-state assertions |
| LOOP-4 | HIGH | Typed transient retry is bounded; real test failure does not auto-retry | Child launch-count tests, own exit assertions, failed/unknown/cancelled evidence preservation |
| LOOP-5 | HIGH | Full measured coverage reaches normal commit without another coverage test process | Actual receipt producer-to-consumer canary with a fake test executable counting invocations; invalidate each bound input separately |
| LOOP-6 | HIGH | Nested fixtures join only authenticated live parent custody; standalone attempts cannot escape accounting | Real process ancestry/custody fixture plus forged, stale, cross-goal and missing-context negative controls |
| LOOP-7 | HIGH | Existing mission stop-loss, independent review, source equality, floors and normal landing remain enforced | Focused existing-owner canaries and final settled full validation |

First reproduce the failed-witness double-launch and duplicate coverage behavior with focused Go-driven fixtures on old source. Do not rerun historical expensive failures. The reviewer may challenge this design's premise or choose deletion/reuse when it names the current requirement that remains satisfied.

## Delivery and stopping rule

Independent design critique precedes shared admission and coverage implementation. The isolated witness correction follows its complete failure-propagation contract without a separate design artifact, as permitted for mechanically small work. Native implementation and a fresh independent code critic follow. Main runs focused canaries while changing. Once the final candidate and review are settled, one final full battery gates normal commit/push in the same failure-stopping chain. Any failure is reported with its specific outcome before another repair cycle is contracted. No automatic new critic root, budget reset, full-suite fallback, or unrelated repair extends this work.
