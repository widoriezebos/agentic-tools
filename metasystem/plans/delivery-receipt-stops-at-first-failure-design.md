# delivery-receipt-stops-at-first-failure: design

Goal 9 of plans/delivery-efficiency-plan.md; R-96-m1e is the word. One
critique read, then the build behind the fixtures in section 5.

## 1. What is wrong today

`proofrun.RunTestPlan` (internal/proofrun/test_build.go) runs every stage of
the plan and, inside a stage, every group, whatever the earlier groups did
(R-16 continue-and-collect). For a delivery attempt that is waste: the
attempt cannot become sufficient once one required group failed, yet it keeps
paying for the rest. 31 failed attempts consumed 26 hours, 10.3 of them after
the first red (proof-attempts.md finding 3). The retry then pays again for the
groups that had passed, because `reusedTestResult` (test_result.go) reuses a
group only from an attempt whose terminal is `success`.

## 2. The change, in two mechanisms

### 2.1 The runner stops launching at the first failed group (delivery only)

- `RunTestPlan` reads `request.Plan.Purpose`. For `delivery` it runs in
  stop-at-first-failure mode; `cadence` and `diagnostic` keep
  continue-and-collect exactly as today. R-96 amends R-16 for delivery alone,
  and a diagnostic run exists to see every failure.
- Inside a stage, `runStageGroups` launches groups through a bounded pool.
  In stop mode, the first group that ends with a status other than `passed`
  or `reused` closes the launch gate: groups not yet launched are recorded as
  `not-run` with `NotRunReason` = `delivery attempt stopped at the first
  failed group <id>`; groups already running finish and report as they do
  now, so no evidence that was being produced is lost. The gate is checked
  before taking a pool slot and again after taking it, so a slot freed by the
  failing group cannot launch one more.
- Across stages, once a stage reported a failure in stop mode, every later
  stage's runnable groups are recorded `not-run` with the same reason; groups
  the caller supplied as reused stay reused (they cost nothing and are
  evidence). A supplied reuse the runner judges invalid (forged or stale)
  is a failed group too: it closes the gate for its own stage and the later
  ones (design critique F2).
- The first failure's exit status stays the attempt's status, as today.
  With a pool wider than one, the group that closed the gate is the first to
  finish red, while the status comes from the first red in plan order; the
  two can differ, and the not-run reason names the gate-closer (design
  critique, not material).
  `RecomputeDelivery` already lists `not-run` groups as missing and the
  failed one as failing, so the attempt is insufficient for the same reasons
  a reader expects.
- Progress records: halted groups get no `start`/`end` progress event (the
  existing not-run path for a progress-record failure records none either),
  so `AssertSectionProgress` keeps its start/end pairing; the reason lives in
  the attempt's result, where `test verify` and the receipt read it.

### 2.2 The retry reuses the predecessor's passed groups (delivery only)

- `reusedTestResult` composes, per selected group, the newest retained
  attempt of the same goal and accounting revision whose component identity
  for that group equals the current identity. Today it takes the group only
  when that attempt's terminal is `success`. The lift: for a template whose
  purpose is `delivery`, an attempt whose terminal is `failed` also serves,
  and only its groups that are `passed` (or `reused`), collection-complete and
  identity-equal are taken, each stamped with `ReuseAttempt`. The group that
  failed in that attempt is not passed there, so it is not reused; a `not-run`
  group has no proof and is not reused. The newest-attempt rule keeps an
  older success from masking a newer failure of the same group
  (TestComponentRepeatDecisionSpansPlanChanges keeps holding).
- Scope stays the same goal and accounting revision, and the contract,
  policy-engine and behavior-policy digests must still match; the wider
  identity work is goal retained-proof-reuse-crosses-claims-and-attempts.
  The source attempt's purpose is not consulted (a failed cadence or
  diagnostic attempt of the same goal and revision serves a delivery retry
  too): what is taken is always a passed, complete group at the current
  identity, which is the evidence, whatever run produced it (code critique,
  not material).
- `ExactReusableTestResult` (whole-attempt reuse that keeps the committed
  receipt) stays success-only: a failed attempt owns no receipt.
- The admission protocol is untouched: after a failed attempt `test run`
  still answers retry-required until the caller supplies a retry decision;
  with one, the worker composes `Reused` through `ReusedTestResultExcluding`
  as today, and now finds the predecessor's passes in it.
- Three checks demand a `success` terminal on the attempt a reused group
  names, and all three learn the same purpose-bound rule (`ReusableTerminal`
  in proofrun): `reusedTestResult` when composing, `validateRetainedGroupReuse`
  when the runner accepts a supplied reuse, and the receipt's owner walk
  `validateTestingAttemptOwners` (internal/landing/testing.go), which runs
  when the worker prepares the receipt and again when a landing verifies it.
  Each still reads the named attempt and requires the group there to be
  passed and collection-complete, so forged or stale reuse stays refused; the
  result's own attempt must still be a success or the live attempt being
  finalized. Without the third check a green retry would be retained as
  failed at the receipt (design critique F1).

## 3. What does not change

- Cadence and diagnostic attempts: every selected group runs, results are
  collected, the battery is swept whole.
- The plan, the stages, the concurrency pool, the watchdog and the evidence
  collection.
- The status vocabulary: `not-run` with a reason already exists and validates.

## 4. Risks and their answers

- A group already running when the first failure lands still consumes its
  budget. Accepted: killing it would lose evidence and the pool is small.
- A retry could reuse a pass produced beside a failure that poisoned the
  machine (a hung sibling, a load storm). The reused group is the same bytes
  and identity that passed; a pass is a pass. The load attribution of
  receipt-admission-caps-concurrent-batteries is the place for that concern.
- Reuse from a failed attempt makes a green retry cheaper to reach with less
  fresh execution. The accounting revision still bounds it to the same goal
  revision, and every reused group names its source attempt in the result.

## 5. Fixtures (build behind these)

1. `TestDeliveryAttemptStopsLaunchingAtTheFirstFailedGroup` (proofrun): stage
   canary {first pass, second fail, third pass}, stage deep {deep pass},
   concurrency 1, purpose delivery: launches 2, `third` and `deep` not-run
   with the reason naming `second`, delivery insufficient, status = second's
   exit code, first's and second's evidence complete.
2. The existing `TestStageCollectsIndependentFailuresAndNativePrerequisiteResults`
   becomes a cadence-purpose plan and keeps asserting four launches; a
   diagnostic twin asserts the same.
3. `TestDeliveryRetryReusesTheFailedPredecessorsPassedGroups` (proofrun): a
   finalized failed attempt with first passed, second failed, third not-run;
   the projection reuses first from that attempt and leaves second and third
   not-run (missing-proof); then `RunTestPlan` with that `Reused` map and a
   fixed second script launches only second and third and ends sufficient.
4. A cadence template with the same failed predecessor reuses nothing.

## 6. Field measure

DONE's field part (time-to-green per goal and failed-attempt mean duration
under 15 minutes over a week) is read from the attempt records a week after
landing; the landing records the mechanism and the fixtures, not the week.
