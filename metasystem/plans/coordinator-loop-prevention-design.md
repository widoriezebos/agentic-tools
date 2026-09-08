# Shared proof admission and finite delivery

Status: proposed for independent design critique. The shared admission and coverage design is not closed. The isolated witness error-propagation correction has the mechanically complete contract below and may proceed under the small-change provision in docs/orchestration.md; it still requires independent code critique.
Goal: coordinator-loop-prevention. Author: main Codex/Astra, xhigh.
Source baseline: stop/status/arm source a3131fbb, integrated through origin/main 4b4d4ab5. The progress record carries the source integration and focused canaries. This revision replaces the earlier proposal's obsolete assignment of proof process lifecycle to run.
Review budget: three rounds maximum, failsafe round 3; stop at the first round with zero material findings. Fixture-expressible findings may move to named implementation obligations only under the existing critique skill's rules. No fresh critic root resets that budget.

## Required outcome

Every supported agent runtime uses the same decision before MetaSystem starts an expensive proof. A running or completed proof cannot become a new unaccounted attempt merely because the coordinator changes runtime, run name, log path, or scratch directory. Successful coverage is consumed by normal landing when its relevant inputs still match. A real gate failure propagates without an implicit full-gate rerun.

The current production consumers are the validation, adoption, Go gate and commit coverage entrypoints. Direct arbitrary shell commands outside these entrypoints are not claimed to be governed. Mission stop-loss, existing delegate admission, goal approval and the human's right to authorize further work remain intact.

## Grounded diagnosis

The accompanying investigation records exact source anchors and independent traces. In particular:

- `proofrun.LaunchSuite` owns actual suite child creation, a checkout stop-fence creation claim, a durable launcher/suite/watchdog record, and a second fence read. It still has no accepted-goal reservation or durable cross-run proof identity. `internal/proofrun/record.go` stores one overwriteable file per suite with running/done status; done precedes final watchdog and evidence judgment and is not success.
- `dispatch.EvaluateGoalRevisionAdmission` already judges the accepted goal, its stop fence, elapsed limit, attempts, reserved minutes and concurrency without requiring a governed obligation.
- `internal/stoptransition/families.go` already inventories and stops proofrun's process records. Monitored run wrappers are a separate family and already have a suite join; an additional wrapper is unnecessary. A goal-bound ordinary run becomes BUDGET_UNKNOWN in `dispatch.ProjectBudget`, so setting GoalId alone is not a budget reservation.
- A live stalled main produces steward notification; ordinary investigation stop-loss is explicitly invoked rather than automatically applied at proof admission.
- `witness-gate.sh` conflates a failed executed gate with an unavailable witness and can launch the plain full gate again.
- Commit coverage always reruns touched packages. Landing receipts do not carry coverage measurements. The existing landing `fullBatteryCommand` begins with `--fast` and is not evidence of full coverage.

## Ownership and scope

Extend the existing `proofrun` record owner with retained admission/outcome facts and the existing `dispatch` admission/projection owners to count them. Keep proofrun's launcher, watchdog and stop integration as the sole proof process lifecycle. Extend the existing `landing` receipt owner for reusable coverage evidence. Keep coverage judgment in `audit.ParseCoverage` and `audit.CheckCoverage`. Shell changes only pass arguments, environment/context and exit status to these Go owners.

Do not introduce a second budget counter, a new custodian service, a new provider-specific stopping policy, a generic workflow language, or a cache that pretends an old full suite just ran. Do not repurpose an obligation revision of zero as an accepted governed obligation.

## Ordinary proof reservation

Add retained attempt records under the proofrun owner's artifacts/agents/proof-runs/attempts directory, keyed by a unique attempt id. The existing stoppable process record links to its attempt; it is not a second process manager. An attempt records schema version, id, goal id, accepted claim/accounting revision, nullable budget epoch, reserved minutes, observed minutes, start/deadline/end, proof scope and input identity, control and execution roots, terminal result, previous attempt and retry evidence when present. Reserve before either child starts. Failed starts, killed launchers and unknown outcomes retain their reservation. A missing terminal result is never success.

Give admitted proof process records a unique record key derived from their attempt id, preserving Suite as the logical progress/selector name. Inventory must discover every concurrent admitted proof rather than overwrite one suite-named file. Read existing records through their legacy suite key when no attempt key is present; do not reinterpret historical running/done records as successful attempts or import old proof as authority. Stop transitions continue to consume the same process identities through proofrun.ReadRecords and proofrun.Stop.

The canonical control/accounting root is separate from the scratch execution root. It owns the checkout stop fence, creation claims, process inventory, goal lock and attempt records; only command execution and source/evidence input discovery use the execution root. This preserves the ability of canonical metasystem stop to find the proof. Admission order is caller classification, accepted goal binding, goalrevision.Acquire, then the proof owner's mutation lock, in-lock caller/epoch recheck, EvaluateGoalRevisionAdmission, repeat decision and durable reservation. The goal lock serializes this with delegate/governed reservations. Release admission locks before starting or waiting for children. Preserve the new stop-fence creation claim and its second read around child creation; its generation is distinct from the goal budget epoch.

Charge each new top-level proof through the existing ProjectBudget: one attempt, its positive reserved duration, and one active execution while live. Preserve the projection's Claimed.AccountingRevision and nullable budget-epoch rules so later claims do not erase accountable attempts. Use the existing rounding convention for observed duration. Retain ordinary proof history while the accepted goal accounting epoch can consume it. Source changes, new ids and new scratch directories never reset accounting. Malformed or missing required accounting facts produce BUDGET_UNKNOWN, never a free attempt.

Use dispatch.cap-min as the default positive reservation and dispatch.cap-max as its ceiling, with the accepted goal limit authoritative. Add the resulting absolute deadline to the existing sibling watchdog; expiry uses its identity-checked stop ladder and retains failed/unknown evidence. A caller may request a smaller positive bound, never more than the configured or accepted remaining limit. Finalize successful attempt outcome only after child exit, watchdog exit, banner/section checks and relevant-input parity all succeed. The existing early markDone remains process bookkeeping, not reusable proof.

The command layer wires admission and outcome callbacks, avoiding a proofrun-to-dispatch dependency cycle. An ordinary proof does not require a separate human-approved recurring obligation. Existing governed runs retain their contract. If a verified governed run wrapper already reserves this exact execution, link to that reservation and do not count the same execution twice; the proof outcome still belongs to proofrun. No obligation revision of zero is invented.

## Shared launch and nested proof context

Apply the reservation path at the shared Go launch owner used by `validate-metasystem.sh` and `adopt-fixtures.sh`; route standalone full `go-gate.sh` and commit coverage through that same owner. Fast static checks and focused test commands remain the small checks they currently are.

Add --control-root, --goal and an optional positive --cap-min at the proof command boundary. Propagate an admitted parent tuple containing control root and attempt id to real children; the proof owner authenticates it against the live recorded launcher/suite identity and actual caller ancestry. The tuple is a locator, never authority by itself. Nested fixtures and snapshot gates stay inside that reservation and deadline, including across execution roots. Missing, dead, forged, cross-goal or mismatched context refuses rather than silently making work free. Existing MS_SUITE_PROGRESS_ACTIVE strings alone do not prove parent custody.

Resolve control root and goal from authenticated parent context first, then an authenticated native delegate locator where present, otherwise explicit options or the installation's active caller/claimed goal. The existing delegate variables are METASYSTEM_HOOK_DELEGATE_STATE_ROOT, METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT and METASYSTEM_HOOK_DELEGATE_JOB; lease.HookDelegate authenticates them. An ambiguous scratch root requires explicit configuration/input. Do not infer shared custody from Git directory names or a provider's name. Existing delegate verification capability limits remain: the main owns the full suite outside delegate sandboxes.

Standalone coordinator calls must resolve an accepted claimed goal before expensive proof launch. An ambiguous accounting root or goal refuses with the exact required input. Existing explicitly goal-free human/bootstrap and exact fake-runtime fixture paths keep their supported behavior; an active coordinator may not declare its claimed goal's work free by changing the scratch root. Verify this boundary using the existing lease/caller and delegate-custody owners rather than creating another runtime detector.

## Identity and repeat decisions

A proof identity contains its declared scope/command class, complete relevant source manifest including modes, effective proof configuration, selected section inventory, platform and toolchain identity. Reuse the existing manifest framing and behavior-surface policy. ENGINE equality may authorize coverage reuse only; it never stands for equality of an entire validation/adoption suite. Full-scope identity must include the complete non-runtime source/input inventory, not just ENGINE. Runtime name, process/run ids, log names and scratch locations are not distinguishing inputs.

Under the proof-owner mutation lock, inspect retained attempts for the same owning goal and proof identity before reserving:

1. A live attempt refuses a duplicate launch and identifies that run for observation.
2. A successful attempt refuses a duplicate expensive launch and identifies its evidence for the delivery consumer. This is not a forged new success result or a new receipt.
3. A failed/unknown attempt requires an explicit retry decision. The decision names the prior run, the diagnosed cause, a concrete evidence artifact and why another attempt may succeed. The engine records the evidence digest and charges the new attempt to the same goal budget. Missing diagnosis or the words "technical issue" alone do not satisfy the decision contract.
4. Machinery may make at most one automatic retry for a typed transient failure before the proof body starts, such as a temporary process-resource error. It must record that classification and the failed attempt. A command's generic nonzero exit, test assertion failure, missing executable/configuration or unknown outcome is not automatically classified as transient.
5. Changed relevant inputs permit a new attempt within the same goal budget and retain the earlier outcome. General repeated-measurement workflows are outside these delivery entrypoints; do not add a new workflow language or free measurement exception.

Retry evidence is accountable coordinator judgment, not a claim that Go can infer the semantics of arbitrary failure logs. Hard budget admission and retained history are mandatory even when the retry explanation is supplied. The independent critic must reject any route where a new root/id/runtime erases prior attempts.

## Failure propagation

In the clean witness path, distinguish inability to prepare/use the optimization before execution from the exit of an executed full gate. Once the full gate actually starts, preserve and return its nonzero exit. Do not launch the plain full gate after a failed executed gate. A successful gate that cannot publish a valid witness is an evidence failure, not authority to rerun automatically. Existing setup-only fallback remains available only before expensive execution has begun.

This change does not revive a dead controller witness, extend its lifetime by assertion, accept seed/fast results as full proof, or weaken any assertion.

## Coverage handoff and landing

Extend the existing landing receipt with a versioned coverage component written by the actual full gate producer after successful measured coverage, complete inventory judgment and unchanged relevant input checks. Bind it to ENGINE bytes and modes, behavior policy version, platform, complete Go toolchain identity and the exact selected coverage ratchet bytes. Preserve the actual package measurements and the producer's command class. Fast, seed, interrupted, incomplete, failed or mutated runs cannot publish reusable coverage authority.

The normal coverage boundary consults this component through Go. Matching evidence reuses its measurements and performs zero additional coverage test launches. Absent or mismatched evidence runs the existing affected-package coverage check. Goal and peer-record changes alone may preserve ENGINE equality; they do not invalidate coverage. Source/test/mode, toolchain/platform, policy or floor changes require fresh coverage. Preserve all ordinary index/working-tree, review-chain, static proof and landing checks.

Normal full-width landing currently accepts only the literal fast-plus-two-fixtures command in landing/observe.go and landing/tierone.go. Add acceptance of the real unselected, unseeded canonical scripts/validate-metasystem.sh receipt, produced by CreateTestReceipt and accompanied by complete successful selector/progress evidence. Preserve acceptance of the existing command for compatibility, but never treat it as measured coverage. A selected section or arbitrary command string cannot qualify as the canonical complete validator. This lets the real final full suite reach normal landing without executing the older partial battery again. Extend the existing producer's coverage component through the nested snapshot/witness path; publish usable coverage only after the producing full gate succeeds, and publish the enclosing full receipt only after its complete command succeeds.

Future final verification uses the real receipt producer and its supported bound; do not import the temporary R27 helper JSON as production authority. The existing 40-minute receipt-command limit is below the measured 56-minute full suite. Use the existing configured proof reservation/bound consistently, within the accepted goal budget, rather than introduce another timeout knob.

## Required focused proof

| Id | Severity | Requirement | Owning proof |
| --- | --- | --- | --- |
| LOOP-1 | HIGH | Ordinary proof reservation and delegate reservations share the same accepted budget, with no check/reserve race | Go proofrun record/admission and dispatch projection integration tests, including simultaneous launches and exhausted budget |
| LOOP-2 | HIGH | Different runtime, worktree, log and run names cannot hide the same proof attempt | Shared entrypoint tests for Claude, Codex and Devin caller/custody contexts, including fresh scratch execution roots |
| LOOP-3 | HIGH | Live/successful duplicate launches create no child; retry requires retained diagnosis and budget | Go fake-command launch counter and persisted run-state assertions |
| LOOP-4 | HIGH | Typed transient retry is bounded; real test failure does not auto-retry | Child launch-count tests, own exit assertions, failed/unknown/cancelled evidence preservation |
| LOOP-5 | HIGH | Full measured coverage reaches normal commit without another coverage test process | Actual receipt producer-to-consumer canary with a fake test executable counting invocations; invalidate each bound input separately |
| LOOP-6 | HIGH | Nested fixtures join only authenticated live parent custody; standalone attempts cannot escape accounting | Real process ancestry/custody fixture plus forged, stale, cross-goal and missing-context negative controls |
| LOOP-7 | HIGH | Existing checkout stop/arm, mission stop-loss, independent review, source equality, floors and normal landing remain enforced | Focused current proofrun/stoptransition canaries, unique concurrent process inventory assertions and final settled full validation |

First reproduce the failed-witness double-launch and duplicate coverage behavior with focused Go-driven fixtures on old source. Do not rerun historical expensive failures. The reviewer may challenge this design's premise or choose deletion/reuse when it names the current requirement that remains satisfied.

## Delivery and stopping rule

Independent design critique precedes shared admission and coverage implementation. The isolated witness correction follows its complete failure-propagation contract without a separate design artifact, as permitted for mechanically small work. Native implementation and a fresh independent code critic follow. Main runs focused canaries while changing. Once the final candidate and review are settled, one final full battery gates normal commit/push in the same failure-stopping chain. Any failure is reported with its specific outcome before another repair cycle is contracted. No automatic new critic root, budget reset, full-suite fallback, or unrelated repair extends this work.
