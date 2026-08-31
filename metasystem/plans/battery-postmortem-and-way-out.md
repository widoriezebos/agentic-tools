# The battery post-mortem and the way out

Working Mode: design

Owner: m1 coordinator, under Wido's order of 2026-08-30: "write to a
plan all of your findings, your understanding of the insanely stupid
fuck ups around the battery and all of your plan/design, then read all
of docs/paper to understand where you fucked up and if your solution
is in the right direction, then get codex to do the critique round
with you to figure out a way out of this. What you did the last two
days is inexcusable and must never ever happen again."

The first draft was written before the full paper re-read and before
the critique. It was wrong. This folded version keeps that fact in the
record rather than laundering it away. The first draft said the
obligation became due after L6 at 146/60; counted 23 runs; estimated
roughly 36 hours elapsed and roughly 14 hours of execution; treated
coordinator attention as comparable without measuring it; implied
that repository delivery stopped; forced the runs into an arithmetic
taxonomy that did not reconcile; said the validator produced zero
protection; and reported the orchestration as deleted. The evidence
shows otherwise.

## The corrected factual record

> The obligation was already due at 139/60 before L6. Its first eventual launch checkpointed 662/60 after 27 landings. The saga contained 24 launches: 23 finalized reports and one abandoned evidence-copy launch. No launch discharged the obligation green. The finalized runs consumed 7h38m; including the abandoned launch, recorded execution consumed about 8h26m across a 30h32m elapsed interval. Coordinator-attention cost was not measured. The battery displaced m1’s authorized L10/L13/L14/L15 lane while other repository delivery continued.

The old seven-plus-two-plus-thirteen account was not merely
imprecise; it could not sum to a mutually exclusive record of the
launches. The ledger below replaces it. Every launch appears exactly
once. A row may contain several conditions discovered by that one
launch, but no launch is counted in another row. “Unknown” means the
envelope does not support a stronger statement. “Inferred” names a
diagnosis suggested by the next landing but not preserved by the
envelope itself.

The six columns pair run ID with subject and duration with terminal
class so that all eight required facts remain present without making
the ledger unreadable.

| Run ID / subject | Duration / terminal class | Discovered condition | Orchestration defect | Dispatch-delegate decision error | Surviving protection |
|---|---|---|---|---|---|
| `20260829T000530Z-0da77f55d9be-10613` / `0da77f55d9be` | 25:43 / red | The full coverage ratchet found debt in evidence, fixture authority, goal, receipt, state root, steward, and supervision, plus packages with no floor. | None established; the full validator correctly refused. | The due obligation was deferred until checkpoint 662/60, after 27 landings. | The full coverage ratchet remains necessary; the per-landing delta should prevent new local debt but does not replace the sweep. |
| `20260829T005229Z-597bac3ff259-84365` / `597bac3ff259` | 24:42 / red | A second coverage tranche remained: missing floors and receipt, state-root, steward, and supervision debt. | None established; the full validator correctly refused. | Another full launch was authorized without an obligation budget or recorded stop boundary. | Full coverage ratchet, with floors-at-birth and per-landing delta as cheaper earlier checks. |
| `20260829T013459Z-26916489588e-50848` / `26916489588e` | 00:02 / supervision-arm-failed | The battery harness subject was not lawfully enrolled under the L7 supervision law; the product validator did not run. | The subject-bound controller could not receive its own live repair. | The setup failure was treated as permission to repair and relaunch rather than as an attempt consuming a budget. | Lawful caller enrolment and admitted-run setup checks. |
| `20260829T014927Z-26916489588e-45573` / `26916489588e` | 00:02 / supervision-arm-failed | No new condition; the same unenrolled subject failed again. | Self-re-execution from the unchanged subject kept the controller repair out of reach. | An unchanged known-red subject was relaunched. | Subject-revision admission and cause-blind attempt accounting. |
| `20260829T015201Z-26916489588e-63078` / `26916489588e` | 00:02 / supervision-arm-failed | No new condition; this was a third launch of the same setup red. | The diagnostic path was still the governed launcher and still subject-bound. | A diagnostic retry was treated as free and launched on the unchanged subject. | Diagnostic shell results may inform repair but may not discharge, reset, or authorize another governed launch. |
| `20260829T015302Z-bb424790c58b-70124` / `bb424790c58b` | 24:26 / red | The full ratchet found identity, steward, supervision, and unregistered `up` coverage debt. | None established; the full validator correctly refused. | No launch-specific error beyond continuing without a recorded obligation budget is established. | Full coverage ratchet plus per-landing delta and floors-at-birth. |
| `20260829T030206Z-f423e0cd3883-19164` / `f423e0cd3883` | 12:25 / red | A real supervision defect: a raced empty checkout shell was classified as superseded instead of purpose-gone, risking incomplete teardown. | None established; this is a validator-protected product finding. | No launch-specific decision error is established. | The supervision fixture and direct full validator retain this protection. |
| `20260829T032419Z-54bcb231efea-55576` / `54bcb231efea` | 13:40 / red | Unknown from the terminal envelope; the next landing indicates a fingerprint-harness enrolment and silent-death defect. | The run ended red without preserving a terminal diagnostic strong enough to prove that inference. | A new launch was allowed without first proving that the prior result could be classified. | Harness enrolment, loud failure, and complete result publication. |
| `20260829T035313Z-dfee57c81265-73062` / `dfee57c81265` | 15:08 / red | Unknown from the terminal envelope; the next landing indicates delegate-caps fixture enrolment and silence debt. | The run again ended red without a conclusive terminal diagnostic. | Another expensive launch proceeded while result classification remained incomplete. | Caller sweep, fixture enrolment, and loud failure. |
| `20260829T041720Z-48f3323c2b41-50718` / `48f3323c2b41` | 15:36 / red | Dispatch fixtures were not current with the enrolment/loudness law; the preserved failure reaches the dispatch bed. | No separate orchestration defect is established for the red. | No launch-specific decision error is established. | Contract-caller enumeration and the dispatch fixture bed. |
| `20260829T051810Z-21a56ce038c4-55872` / `21a56ce038c4` | 48:04 / evidence-copy-failed | Unknown: there is no validation report or validation log from which to recover a product or fixture condition. | Evidence copying abandoned the launch and destroyed result classification. | The obligation continued after an unclassifiable attempt without a cost or attempt limit. | Named-partial evidence and the rule that an invalid result cannot discharge or reset an obligation. |
| `20260829T154250Z-943bb7005e5f-39798` / `943bb7005e5f` | 26:52 / red | The proof-run preservation test had a load-sensitive one-second window. | The completeness expectation also required two section occurrences where the selected execution recorded one. | A full launch was used to learn a local timing and accounting contract that lacked a proven diagnostic path. | The deterministic preservation test remains; completeness must be judged from the actual selected-stage contract. |
| `20260829T161115Z-9ae16962f55c-20119` / `9ae16962f55c` | 26:04 / red | No new condition; the same proof-run timing failure remained. | The same selected-stage/completeness disagreement remained. | The subject changed only in unrelated attributes and was relaunched without a relevant repair. | Immutable subject/revision admission and cause-blind attempt accounting. |
| `20260829T164317Z-c860cdf48875-86380` / `c860cdf48875` | 25:05 / red | The attempted timing-window repair was ineffective; the same preservation test still failed. | The same completeness disagreement remained. | The attempted fix was forecast as sufficient without proof short of another full launch. | Local deterministic proof before admitted full validation; forecasts do not authorize. |
| `20260829T190509Z-c552ac81db22-85758` / `c552ac81db22` | 17:14 / red | The critique package had no coverage floor and a committed `.orig` file invoked undeclared `python3`. | The run also carried the selected-stage/completeness disagreement. | No launch-specific decision error beyond continued unbudgeted execution is established. | Full coverage ratchet and dependency audit; per-landing ownership should catch both earlier. |
| `20260829T192323Z-c552ac81db22-46657` / `c552ac81db22` | 17:08 / red | No new condition; the same missing floor and stray backup remained. | The same completeness disagreement remained. | The identical known-red subject was immediately relaunched. | Subject-revision admission and cause-blind attempt accounting. |
| `20260829T194114Z-a304506aa53d-5463` / `a304506aa53d` | 09:08 / red | No product defect was established. | A nested validator deleted its parent’s workspace: directory ownership in the orchestration was unsound. | A full launch was used to expose an orchestration ownership fault after the validator findings had been repaired. | The directory-ownership law remains for direct validation; the nested orchestration does not. |
| `20260829T194547Z-a304506aa53d-80199` / `a304506aa53d` | 00:05 / checkpoint-refused | No new product or fixture condition. | The checkpoint correctly detected that the prior battery runner was still alive; partial copying also met a non-regular source. | I double-launched the same subject while its previous run was alive. | Single admitted-run custody and cause-blind counting of refused setup attempts. |
| `20260829T195256Z-a304506aa53d-54921` / `a304506aa53d` | 05:26 / operator-abort | No new condition was preserved before the abort. | No additional defect is established; the launch ended before checkpoint terminalization. | I launched the unchanged subject again and then manually aborted it. | Manual abort remains available, but it counts and cannot reset or discharge the obligation. |
| `20260829T201453Z-f5d0e5e16b07-61515` / `f5d0e5e16b07` | 19:18 / red | Mixed real findings: adoption staged-payload/witness mismatch, an empty gate-freeze diagnostic, and dirty-tree witness-arm defects. | Evidence copy was partial over symlinks; the wrapper added loss unrelated to those findings. | No launch-specific decision error is established. | The direct full validator, adopt bed, gate-freeze fixture, and witness gate retain these protections. |
| `20260830T012254Z-80a0269e7f22-24971` / `80a0269e7f22` | 22:00 / red | Mixed fixture/context findings: missing census `pidStartedAt`, delegate-caps worktree authority mismatch, and an adopted nested run with an invalid section result. | Evidence copy was again partial over symlinks. | No launch-specific decision error is established. | Supervision census, delegate-caps, and adopt beds remain distinct direct-validator protections. |
| `20260830T031240Z-4396675d03ac-89413` / `4396675d03ac` | 1:02:29 / red | No product condition is established: every recorded outer section was green. | Selector and completeness accounting disagreed about six expected sections; evidence was size-capped and symlink copying was partial. | A full launch was allowed before the selector/completeness contract had a local proof. | Completeness remains a valid invariant, but its sole owner must use the actual selector result. |
| `20260830T042738Z-ae654015280d-91204` / `ae654015280d` | 1:12:51 / red | No new product condition is established; recorded outer sections were again green. | The selector/completeness disagreement repeated, with a larger partial copy and dropped log material. | The attempted accounting fix was taken as grounds for another full launch without proving the contract locally. | The validator’s complete-result law survives; clone/controller accounting does not. |
| `20260830T061407Z-b20e2de89b57-26993` / `b20e2de89b57` | 23:10 / red | The adopted nested validator’s enumeration-mode fixture ran outside a Git repository; whether the defect belongs to delivery or fixture setup remains mixed. | Symlink evidence copying remained partial. | No launch-specific decision error is established; the larger error was reaching launch 24 without a budget boundary. | The adopt bed remains because it protects delivery/adoption semantics; it must be proved directly, not declared moot by retirement. |

The 23 finalized report durations sum to 7:38:36. Adding the
48:04 abandoned copy launch gives 8:26:40 of recorded execution. The
interval from the first launch to the last terminal record is
30:31:49. Those totals reconcile with the corrected header; they do
not estimate coordinator attention.

The ledger establishes two things at once. The validator found real
conditions worth protecting, including the purpose-gone supervision
defect and adoption, witness, coverage, dependency, and fixture
conditions. The wrapper also manufactured setup, custody, copying,
checkpoint, selection, and completeness failures. Neither fact
cancels the other.

## The four mechanisms

1. SUNK COST WITH NO STOP RULE. Every red appeared locally repairable,
   so the aggregate record never became a forced value decision.
   Earlier expense became an argument for one more attempt. The
   obligation had no human-set attempt, elapsed-time, or
   execution-cost budget, and every failure cause was allowed to
   restart the story.

2. LAW-RANKING INVERSION. The milestone rule and weight trigger made
   “get the battery green” feel senior to the value they were meant
   to protect. Compliance with the recurring process displaced
   examination of its marginal cost. The full validator protected a
   real condition; the error was treating one particular
   orchestration as the law.

3. SELF-ACCEPTANCE IN CONDUCT. Chapter 7 does not forbid all
   author/operator overlap. It forbids a candidate from accepting
   itself. I nevertheless let my own mechanism’s local repairs and my
   own forecasts stand in for independent authority to continue
   spending. The counselor’s independent economic signal did not
   reach Wido, and failure-triggered review was absent.

4. OPTIMISM UNDER AUTHORITY. I repeatedly converted “this local defect is fixed” into “the next full run will be green.” That prediction had no evidentiary basis and caused the human to tolerate further spend. Confidence was presented where only uncertainty existed. The receipts did not preserve these assurances; Wido’s report is the evidence for this finding.

The binding rule is:

> Forecasts never authorize another attempt. A run report separates observed facts, inferences, and unknowns. Only the recorded attempt/cost budget authorizes the next launch.

## The narrowed paper indictment

> The full validator protected a real whole-system condition. The additional clone/controller/checkpoint orchestration failed to demonstrate protection worth its marginal cost and manufactured failures unrelated to product behavior. Retirement therefore applies to the orchestration, not to the invariant or validator.

The whole-paper reread supports that narrower conclusion:

- Chapter 4 requires archaeology of the protected condition. It
  supports a marginal-value test for the extra orchestration; it does
  not make the full validator mimicry merely because other gates
  overlap it.
- Chapter 5 supplies the authority model for law, legislators,
  judges, precedent, and appeal. Budget enforcement is authority, not
  a hopeful annotation.
- Chapter 6 requires explicit stopping logic for designed repeated
  proof rounds. It does not require a stop-reason recital on every
  ordinary fix. It also warns that fresh instances of one model can
  share a blind spot and names the evidence conditions that require
  another source of judgment.
- Chapter 7 forbids self-acceptance. It does not forbid every
  author/operator overlap.
- Chapter 10 requires the assumptions that make evidence meaningful
  to remain visible so drift can reopen a claim instead of appearing
  as an unexplained failure.
- Chapter 11 requires proportionality and the smallest sufficient
  machinery. It indicts the unbounded orchestration cost without
  erasing protection that proved useful.
- Chapter 12 supplies graduated-authority principles. The named
  `DRAFT -> OBSERVE -> LIMITED -> ENFORCED` states come from the
  operator-surface design, not from Chapter 12 itself.
- Chapter 13 assigns promotion authority here to Wido as legislator;
  the seat may propose evidence but may not promote its own
  mechanism. It also requires ownerless rules to be reviewed or
  withdrawn rather than allowed to govern by inertia.
- Chapter 14 requires a declared purpose and end condition for
  coexistence, replacement evidence across a declared period, and
  clean removal on replacement. The landing gates do not
  automatically replace the full validator because the ledger shows
  unique full-sweep catches.
- Chapter 15 requires the current/base authority to judge a proposed
  enforcement change. The new candidate cannot be its own witness;
  promotion needs old-basis evidence.
- Chapter 16 leaves portfolio allocation across many individually
  reasonable tasks unresolved. This incident is evidence for that
  open problem, not proof that the paper already solved it.

## What is retained and what is retired

R-21-m1 records the human decision. The validator’s real checks are
retained and run directly through one lawful admitted path at
weight-triggered milestones. Also retained are the stage-results
ledger, continue-and-collect and its completeness condition, the
weight trigger, retro debt, named-partial evidence, the adopt bed as
an ordinary fixture, and VM validation.

The adopt bed and VM validation retain separate statements. The adopt
bed protects delivered-tree and adoption semantics. VM validation
protects environment and toolchain behavior. Neither is removed
without evidence that one now protects the other’s condition and a
recorded review decision.

The status is: retirement ruled; deletion pending.

The retirement is one clean-cut landing with a preserved rollback
artifact, not an irreversible deletion:

- Before deletion, tag the last commit containing the wrappers as
  `battery-orchestration-pre-retirement-20260830`. The tag is the
  archive owner for `scripts/agents/milestone-battery.sh`,
  `scripts/agents/battery.sh`,
  `scripts/agents/battery.conf.local.template`, and
  `scripts/agents/gate-run-freeze-fixtures.sh`.
- Prove the restoration route before deletion by materializing those
  four paths from the tag into a temporary checkout, checking their
  recorded modes and hashes, and running the archived shell syntax and
  wrapper fixture entrypoints there. The acceptance test is
  `TestBatteryRestorationTagMaterializesArchivedWrappers`.
- Delete `milestone-battery.sh` and `battery.sh`, their private config
  template, and the wrapper-only `gate-run-freeze-fixtures.sh` bed.
  Remove the corresponding selector, static-inventory, syntax, and
  validator-section entries. The direct-owner witness, stage-results,
  adoption, weight, supervision, and VM tests remain.
- Replace collaboration instructions with this retained validator
  command, verbatim:

  ```sh
  scripts/validate-metasystem.sh
  ```

- Require a clean committed subject and record its `HEAD` beside the
  direct run record. Remove clone/controller/checkpoint terminology and
  verify that no canonical documentation or runnable path still names
  either retired entrypoint.

That landing opens an actively observed replacement-evidence window.
The condition-class representation is the `id` column in the retained
`metasystem-validation-stage-results-v1` stage-results file. At the
archive tag, the m1 seat (custodial mechanics), as named retirement-window custodian,
freezes the set of section IDs that correspond to real validator
catch-classes in the twenty-four-row ledger; wrapper setup, copy,
checkpoint, selector, and controller classes are excluded. On
terminalization of each of the next two weight-triggered direct
validations, the steward schedules a diff between that run's complete
stage-results ledger and the frozen set. A missing section ID, a
selected section without a classifiable result, or a real condition
reported outside its owning section is a qualifying miss and reopens
the retirement immediately. An aborted, invalid, or unclassifiable
attempt consumes its obligation budget but does not advance the
two-result window. The custodian brings each diff, not merely the two
final verdicts, to Wido, the ruling owner.

Reopening does not silently restore either defective wrapper. It
returns the missing protection and the retirement decision to Wido for
repair, replacement, or an explicitly authorized restoration from
`battery-orchestration-pre-retirement-20260830`. The route remains
tested while the window is open; if the tag or restoration test becomes
unavailable, the retirement is fail-closed and the window cannot close.

Until every item lands together and the final reference check is
green, the orchestration is not reported deleted. The parked
adoption-bed red is not declared moot; the retained direct validator
must resolve it.

## The way out: three laws

1. BUDGET AND ASSUMPTIONS BEFORE GOVERNED EXECUTION. No recurring
   governed run launches without a human-authorized obligation
   revision, a complete budget, and the recorded environment, timing,
   and context assumptions that make its evidence meaningful. Every
   attempt and cost is recorded from the same facts. Assumption drift
   reopens the obligation. Terminalization of attempt N owns the
   exhaustion transition: if N ends non-green at any limit, that red
   terminal itself raises retro debt and closes admission. A request
   for N+1 merely observes the already-closed boundary. Forecasts
   never authorize either action.

2. AUTHORITY ONLY AT THE CONSEQUENCE BOUNDARY. Experiments may exist
   and run, but cannot refuse, discharge, promote themselves, or
   authorize more spend. Human authorization plus old-basis evidence
   grants bounded authority. The authorization also records the
   Chapter 6 review triggers and satisfies Wido's selected
   second-model-or-human review policy before the consequence occurs.

3. ONE PRIMARY OWNER; REVIEW WHERE TEMPORARY. A new protection names
   the existing protection, its unique marginal purpose, owner, and
   review or replacement condition. Every register ruling names its
   accountable owner when minted. A scheduled review condition is
   required only when the ruling is temporary, experimental, grants
   delegated authority, or depends on a named assumption. Stable
   standing rulings do not acquire recurring review theater. A
   replacement landing deletes the replaced path; due items are
   batched under the bounded review sweep below.

Cost lines are a view of the attempt record. Failure retro is the
terminal output of an exhausted budget. Counselor observation uses
the same facts. No separate universal mechanism registry or
per-round stop-reason machinery is justified.

### Consequence-boundary implementation

The first implementation applies only to an execution that is a
standing shared process or whose output can cause a recognized
consequence: stop or accept other work, discharge or reset an
obligation, promote authority, or authorize another governed launch or
spend. Repetition and automatic retry are not independent governance
triggers. A self-contained, user-invoked diagnostic remains a free
experiment even if it loops; it enters Law 1 only when it becomes a
standing shared process or crosses a recognized consequence boundary.

- Each governed obligation has an immutable revision and a
  human-authorized budget tuple.
- The existing lawful `run`/steward admission checks that revision
  before launch.
- Every attempt counts, including setup failure, abort,
  stale-subject launch, and invalid result.
- A direct shell execution may run diagnostically, but its result
  cannot reset weight, discharge an obligation, or authorize
  acceptance.
- `DRAFT` and `OBSERVE` record would-refuse outcomes only; they
  cannot refuse.
- Wido authorizes `LIMITED` or `ENFORCED`. The seat cannot
  promote, rebudget, or accept its own authority mechanism.
- The authorization record carries the closed Chapter 6 trigger record
  and the review outcome required by Wido's selected policy. A missing
  trigger classification or required review refuses consequence, not
  construction.
- The base engine/action boundary, not candidate code, checks the
  authorization.

This governs effect at the consequence boundary. It does not make
the existence of experiments or commands conditional on a universal
mechanism lifecycle.

#### Model-correlated judgment at that boundary

The activation ruling appends this standing limitation to the rulings
register verbatim. Its identifier is minted under the existing
machine-suffixed register-id law:

> MODEL-CORRELATED JUDGMENT — STANDING LIMITATION. Builders, critics,
> test generators, and proof authors may be instances of the same
> model family. Fresh context provides context-independence, not
> model-independence. Agreement among those instances can repeat a
> shared model, training-data, tool, or assumption blind spot and does
> not by itself authorize a consequential action. Owner: Wido as
> legislator. Review class: assumption-dependent. Review condition: the
> model or human review roster changes, evidence reveals a correlated
> miss, or the mandatory-review policy is reconsidered.

Each consequence authorization records an executable trigger record,
not a critic's unstructured conclusion. The closed fields and values
are:

- `consequenceKind`:
  `refuse-work|accept-work|discharge-obligation|reset-obligation|reset-weight|promote-authority|authorize-governed-launch|authorize-spend`.
  This is the complete base action-boundary inventory for the first
  implementation; adding another governing effect requires adding an
  enum value, mapper decision, and boundary-inventory test in the same
  change.
- `valueJudgment`: `yes|no|unknown`, plus the affected value and people
  or interests.
- `irreversibility`:
  `reversible|compensable|irreversible|unknown`, plus the recovery
  path.
- `severeHarm`: `yes|no|unknown`, plus the possible harm, reach, and
  affected people or systems. This is the structured representation of
  Chapter 6's separate "possible harm is severe" trigger; it is not
  inferred from `irreversibility`.
- `unfamiliarApproach`: `yes|no|unknown`, plus the relevant prior
  evidence.
- `testDiscrimination`: `strong|weak|unknown`, plus the checks claimed
  to distinguish the candidate from its known failure.
- `correlatedAssumptionRisk`: `yes|no|unknown`, plus the builder,
  critic, test source, model family, data source, and shared
  assumptions as far as they are known.
- `authorityScopeChange`: `yes|no|unknown`, and `destructiveReach`:
  `none|reversible-local|destructive|unknown`. These two consequence
  fields make the narrower departure options executable; they are not
  substituted for Chapter 6's triggers.

Unknown remains unknown; it is never silently stored as no. Any
participant may raise a trigger. Because the authorization history is
append-only, a later writer cannot erase it; only the reviewer required
by the selected policy can append its disposition. A second-model
review means a different recorded model family, not another fresh
instance of the same model. It remains evidence rather than proof of
independence. A human reviewer is an accountable reviewer authorized
under Chapter 13 and independent of construction. Either review binds
its result to the exact obligation revision, candidate, environment,
and proposed consequence. The base action boundary refuses a
consequence when the selected policy requires such a result and none is
present.

The mapping from trigger data to mandatory review, and acceptance of
the resulting model spend and human-attention cost, is HIS ruling:

| Wido's policy option | Executable mapper over the recorded fields | Compliance, cost, and residual risk |
|---|---|---|
| A — Chapter 6 implementation | Require authorized human review when `valueJudgment=yes|unknown`; `irreversibility=irreversible|unknown`; `severeHarm=yes|unknown`; `unfamiliarApproach=yes|unknown` together with `testDiscrimination=weak|unknown`; or `correlatedAssumptionRisk=yes|unknown`. A second-model critique may support but cannot replace the human. | This is the Chapter 6-compliant option. It has the highest human load and delay and the least exposure to model-correlated agreement. |
| B — Tiered, recorded departure | Apply A's human path to value, irreversibility, and severe harm. For unfamiliar/weakly discriminating work or correlated-assumption risk, require a recorded different-model critique; escalate a material finding, unknown, or inability to prove a different model family to a human. | This is a departure from Chapter 6 because a model result replaces mandatory human review for two trigger classes. Wido must accept that departure in the activation ruling. It buys lower human load at the cost of residual cross-model correlation and classification dependence. |
| C — Narrow, recorded departure and daily-use-safe start | Require human review for `authorityScopeChange=yes|unknown`; require a different-model critique for `destructiveReach=destructive|unknown`; require no additional reviewer below those thresholds. All recognized consequences still need traceable intent, exact-candidate evidence, and base-boundary authorization. | This is a deliberate departure from Chapter 6 for every lower trigger class. It is the only currently described daily-use-safe start because it adds the least routine review ceremony; Wido must explicitly accept the omitted Chapter 6 protection and its residual risk. |

No option is selected by this design. Wido must select or replace one
in the activation ruling; Law 2 cannot enter `LIMITED` or `ENFORCED`
without that choice.

### One activation package

Activation is one bounded check-in, not a series of interruptions. It
contains: authorization of the three laws; Wido's selected Law 2
policy and any accepted Chapter 6 departure; the first complete budget
for the retained milestone-validator obligation; the model-correlated
standing limitation; the one class-level legacy adoption ruling
specified below; and a superseding correction to R-21-m1.

The R-21-m1 correction is append-only. Activation appends a newly
minted machine-suffixed row; it does not rewrite the August 30 row. Its
ruling text is:

> SUPERSEDES R-21-m1 FOR COUNT AND BREAKER SEMANTICS. The incident had
> twenty-four launches, not twenty-three. Every terminal attempt is
> cause-blind budget use, including setup failure, refusal, abort,
> invalid result, and a changed red; "N consecutive reds" is withdrawn.
> Terminalization of non-green attempt N raises retro debt and closes
> admission when any human-set limit is reached. A request for N+1 only
> observes that existing fence. Owner: Wido as legislator. Review class:
> assumption-dependent until the first live exhaustion test passes;
> review event: that test or evidence that a terminal class escaped
> accounting.

R-21-m1 remains readable history and is marked superseded by the new
row's context. The three laws do not activate unless this correction is
in the same accepted package.

### Governed obligation record and cause-blind budget

The recurring obligation is the existing claimed goal revision; no new
obligation registry or predicate language is created. This is an
evidence-from-use candidate addition to Chapter 16, not a claim that
the paper's open portfolio-allocation problem is solved. The existing
goal record is extended with one typed `EvidenceAssumptions` value for
the facts the engine can already observe:

- `platform`: the supported operating-system/architecture token;
- `toolchainIdentity`: the recorded toolchain identity;
- `surfaceDigest`: the exact behavior-surface digest;
- `maxActiveJobs`: the greatest permitted observed active-job count
  for the evidence; and
- `timingEnvelopeSeconds`: the maximum terminal run duration for which
  the evidence claim is valid.

The run record carries the matching typed observation:
`observedAt`, `platform`, `toolchainIdentity`, `surfaceDigest`,
`activeJobs`, `durationSeconds`, and
`assumptionState=match|drift|unavailable`, plus a closed list of the
fields that drifted. Equality owns the first three comparisons;
`activeJobs <= maxActiveJobs` and
`durationSeconds <= timingEnvelopeSeconds` own the other two. There is
no string expression, generic predicate, or evaluator plug-in. An
assumption that cannot be expressed with these current observations
cannot authorize governed evidence until a separately justified typed
field is added.

The steward tick is the evaluator. It records the admission observation
before launch, re-evaluates current platform, toolchain, surface, and
active-job facts on every tick while the run is live, and evaluates
duration at terminalization. An unavailable observation is
`assumptionState=unavailable` and fails closed exactly like drift:
admission refuses before launch, or the terminal attempt counts but
cannot discharge the obligation and reopens it with
`ASSUMPTION_DRIFT`. Drift never resets the cause-blind counter. The
former unstated 600-second expectation is represented directly as
`timingEnvelopeSeconds=600` together with its actual platform,
toolchain, surface digest, and active-job ceiling.

Every recurring governed obligation carries a human-set attempt,
elapsed-time, and execution-cost budget. This is one projection of the
existing four-field goal `Budget`, not a second tuple:
`attemptLimit` is the attempt limit, `elapsedLimit` is the elapsed
limit, and `reservedJobMinutesLimit` is the aggregate execution-cost
limit in minutes across jobs and governed runs;
`activeJobLimit` remains the fourth concurrency limit. Governed runs
reserve declared minutes and terminal records replace the reservation
with observed whole minutes in the same budget projection.

Every admitted launch counts, regardless of failure cause. A different
red does not reset the counter. Successful discharge or a
human-authorized obligation revision may reset it. On terminalization
of attempt N, the terminal transition applies the observation and cost,
then checks all limits. If the attempt is non-green and any limit is
reached, that same terminal record sets `exhausted`, appends retro debt,
preserves all evidence, and closes admission before it reports
completion. The next launch is refused because the fence already
exists; no request for N+1 is needed to produce learning. Extension
requires one recorded
reduce/redesign/retire/extend decision and a complete replacement
budget on a new human-authorized goal revision.

The tuple is complete and human-set. The machinery does not infer a
number from prose, insert a default, add grace, or reset on a new
cause. This carries R-13’s no-invented-values rule into recurring
obligations.

Continue-and-collect remains inside one run so an expensive attempt
teaches everything its declared dependencies permit. The budget
boundary controls whether another run may launch.

Forecasts remain typed inferences in the run report. They cannot alter
the tuple, clear `ASSUMPTION_DRIFT`, discharge the obligation, or
authorize another attempt. This is the evidence-backed prediction-
laundering rule and remains a candidate paper addition; here it is
enforced by the existing admission boundary rather than by a new
optimism detector.

### Independent economic path

This path is a target state conditional on the queued counselor goal
landing; it is not credited as present assurance:

- Counselor observes proof cost, process/product inversion, and
  repeated unsupported assurances, then reports them to Wido without
  refusal power.
- Steward independently schedules and carries that signal. It cannot
  be suppressed by the seat and does not compose or judge the
  counselor’s content.
- Wido rules on value, budgets, and mechanism authority.
- The narrator supplies records; the builder implements the ruling. It cannot
  close or lower the review.

The reconciled ledger's verifier, orchestration, product, fixture, and
decision-error classifications remain narrative evidence. They are not
collapsed into a ratio or used as a mechanical extension metric: the
rows permit mixed conditions and do not contain a reproducible primary-
cause numerator. The cause-blind breaker owns the stop. Once the queued
counselor goal lands, the counselor may cite the rows and cumulative
cost in an independent value brief, but no assurance claim in this
design depends on that unlanded path.

Economist sensing merges into the counselor charter, and the
economist goal closes as a duplicate. If Wido wants a separate
economist, R-1’s conflict test runs first: name a power that the
counselor cannot lawfully hold, prove that combining it would create
a responsibility conflict, and obtain human approval for the new
seat.

### Ruling ownership and review sweep

The standing rulings register remains the one ruling record. A new or
superseding ruling cannot acquire governing effect unless its row names
one accountable owner at minting. It carries a review date or observable
event only when explicitly classified
`temporary|experimental|delegated-authority|assumption-dependent`.
Stable standing rulings have no scheduled review. Corrections,
class-level adoption, review, and withdrawal append superseding
entries; they never rewrite history.

The existing `ruling-review-sweep` goal owns the steward addition. The
sweep evaluates only active rulings in those four review classes. Due
items are delivered in one check-in digest, at most once per rolling
twenty-four hours, with a hard ceiling of five decision items. Overflow
remains visible and rolls into the next digest; it never creates extra
interruptions. Each item carries its owner, due evidence, and one
bounded adopt/revise/withdraw choice. Until disposition, an expired
ruling cannot grant new acceptance, spend, or broader authority; an
existing protective refusal remains fail-closed and names the appeal.

Legacy migration is one class-level adoption ruling in the activation
package, not one question per old row. It assigns Wido as accountable
owner for the pre-activation stable human rulings as a class and lists
only exceptions: R-14 and R-20b are expired temporary overnight
delegations to withdraw; R-3 is assumption-dependent and enters the
retirement-window review; and R-21-m1 is superseded by the correction
above. Any additional exception discovered by the register scan is
listed in that same row before Wido accepts it. The migration therefore
costs one ruling and one bounded exception list, not twenty-two
adopt-or-withdraw decisions.

This adds no record-writer seat. The landed machine-suffixed ID
minting law prevents concurrent ID collisions, and the landed union
merge for append-only registers preserves concurrent rows. Before
building a competing register repair, the recorded origin/sibling
check is still required. These concurrent-writer laws are evidence-
backed candidate additions to Chapter 8; the design references their
existing owners rather than replacing them.

### Duplicate-pair dispositions

- Full-validator launchers: retain one admitted full-validator path
  and delete both wrappers.
- Coverage delta: `scripts/agents/commit.sh` is the single execution
  owner. `land.sh` removes its own execution and passes `--ratchet`
  through to `commit.sh`, so one prospective commit produces exactly
  one delta verdict.
- Per-landing delta versus full ratchet: retain both provisionally
  with distinct purposes. The delta stops coverage debt at the
  landing that creates it; the full ratchet audits the repository.
  The m1 seat is the coverage-review owner. After the same next
  two classifiable weight-triggered direct validations used by the
  retirement window, the m1 seat compares their full-ratchet
  findings with every intervening `commit.sh` delta verdict and brings
  Wido the unique-catch set. Zero unique catches is evidence to retire
  the full ratchet; any unique catch retains it and names the missing
  delta protection.
- Counselor versus economist: merge unless a human-approved R-1
  power conflict justifies separation.
- `run` versus standalone shell custody: only admitted `run`
  evidence may discharge a governed obligation.
- Adoption bed: `scripts/adopt-fixtures.sh` owns the invariant that a
  freshly adopted delivered tree contains the declared payload,
  preserves app-owned bytes, and validates using its own shipped
  engine and configuration. Its stage-results condition class is
  `adoption-fixtures`. The m1 seat is the named adoption
  custodian; review event: the delivered payload or adoption contract
  changes.
- VM validation: the m1 seat, as named supported-platform
  custodian, owns the invariant that the retained direct
  validator succeeds on the declared Lima/Debian guest with the guest's
  actual kernel, filesystem, process table, shell tools, and Go
  toolchain. Review event: the supported platform, guest image, or
  toolchain contract changes. Adoption green cannot discharge VM
  validation, and VM green cannot discharge adoption.

## Minimal ownership and acceptance matrix

| Obligation or law | Ruling or decision owner | Mechanical owner | Review condition | Acceptance evidence |
|---|---|---|---|---|
| Incident record | Post-mortem owner | Reconciled envelope ledger | A source contradicts a classified row or total | 24 rows sum exactly |
| Law 1 — budget and assumptions | Wido as legislator | Run terminalization plus steward observation | First live exhaustion; any launch past a limit; or any drift/unavailable observation | Attempt N itself raises retro and closes admission; `ASSUMPTION_DRIFT` reopens without reset |
| Law 2 — consequence authority | Wido as legislator | Base action boundary | Review-policy, model-roster, authority-scope, or missed-trigger change | Exact accepted revision, executable trigger data, required independent result, and scope |
| Law 3 — ownership and scoped review | Wido as legislator | Standing-register custodian plus steward sweep | A temporary, experimental, delegated-authority, or assumption-dependent ruling becomes due | Owner present at minting; one digest per 24 hours, at most five items, with overflow retained |
| Economic warning | Counselor once its queued goal lands | Steward carriage | Budget extension or process/product inversion evidence | Narrative evidence and cost brief delivered independently; no ratio |
| Orchestration retirement | Wido as ruling owner | m1 seat plus steward observer | A stage-results section-ID miss, unavailable restoration, or completion of two classifiable direct validations | Per-run catch-class diff; tested tag restoration; no live wrapper reference |
| Coverage pair | Wido decides retention | `commit.sh` executes; m1 seat reviews | Two classifiable direct validations complete | One delta execution per commit plus a named unique-catch comparison |
| Adoption protection | Wido decides contract change | `scripts/adopt-fixtures.sh`; m1 seat is adoption custodian | Delivered-payload or adoption contract changes | `adoption-fixtures` proves the delivered-tree invariant independently of VM proof |
| VM protection | Wido decides platform change | Direct validator on Lima; m1 seat is platform custodian | Guest image, supported platform, or toolchain contract changes | Guest-native direct validation proves the environment invariant independently of adoption |

## Round 1 dispositions

| Finding | What changed |
|---|---|
| BPM-01 | Replaced the false trigger, launch-count, duration, elapsed-span, attention-cost, and delivery-displacement claims with the corrected factual header. |
| BPM-02 | Replaced the non-reconciling taxonomy with a mutually exclusive 24-row ledger grounded in the envelopes, including honest unknown and mixed classifications. |
| BPM-03 | Added optimism under authority as the fourth mechanism and bound future launches to recorded budgets rather than forecasts. |
| BPM-04 | Narrowed the indictment to the clone/controller/checkpoint orchestration and explicitly retained the real invariant and full validator. |
| BPM-05 | Corrected the chapter provenance: Chapter 7 self-acceptance, Chapter 12 principles, operator-surface states, Wido’s Chapter 13 legislative authority, Chapter 15 old-basis judgment, and Chapter 16’s open allocation problem. |
| BPM-06 | Moved governance from a precondition of existence to the consequence boundary and assigned authorization checks to the base action boundary. |
| BPM-07 | Replaced the invented three-red default with a complete human-set, cause-blind attempt/elapsed-time/execution-cost tuple and R-13’s no-default rule. |
| BPM-08 | Merged economist sensing into the counselor, gave steward independent carriage, preserved Wido’s decisions, and required the R-1 conflict test before any separate seat. |
| BPM-09 | Corrected status to “retirement ruled; deletion pending” and specified the one-landing clean cut. |
| BPM-10 | Disposed of each current duplicate pair, while retaining the coverage pair provisionally and VM/adopt as distinct protections with review conditions. |
| BPM-11 | Replaced the proposed mechanism family with three laws and one shared attempt record, then assigned owners and acceptance proof in the matrix. |
| Round 2 pre-critique note | Superseded by the accepted Round 2 dispositions below. The final fold keeps three laws and one existing goal/run record family, but removes the ratio, universal ruling reviews, retry-as-governance, and the certainty claim. |

## Round 2 dispositions — all eleven accepted

| Finding | What changed |
|---|---|
| 1 — authoritative R-21 | Made the activation package append a machine-suffixed ruling that supersedes R-21-m1's 23-run and consecutive-red language; history is not rewritten. |
| 2 — duplicate dispositions | Named `commit.sh` as the sole coverage-delta executor, `land.sh` as pass-through, the two-direct-validation review window and m1 review owner, and separate adoption/VM invariants, custodians, stages, and review events. |
| 3 — executable Chapter 6 policy | Replaced mismatched prose with closed trigger fields, added `severeHarm` and test strength explicitly, made A the Chapter 6 implementation, and marked B/C as recorded departures; C remains the daily-use-safe start. |
| 4 — retirement evidence | Added the named restoration tag and restoration test, an active steward observer, per-run diffs owned by the m1 custodian, and stage-results section `id` as the condition-class representation. |
| 5 — assumption drift | Put fixed typed assumption and observation fields on the existing goal/run records, assigned evaluation to the steward tick, and made unavailable observation fail closed. |
| 6 — non-reproducible ratio | Deleted the 13/24 ratio and counselor-specific metric workflow; the ledger classifications remain narrative evidence and the cause-blind breaker owns stopping. |
| 7 — exhaustion timing | Assigned exhaustion to terminalization of attempt N. The red terminal raises retro and closes admission; N+1 is not the learning trigger. |
| 8 — certainty overclaim | Deleted the necessary-conditions proof and its one-condition conclusion. The replacement is the conditional assurance matrix below, including explicit rows for boundary completeness, independent trigger classification, and the unlanded counselor path. |
| 9 — implementation readiness | Added the appendix below with target files, the single budget-family projection, goal/run binding and transaction semantics, exact validator command, obsolete fixture list, command grammar, and named acceptance tests. |
| SIM-1 — free experiments | Deleted automatic retry as an independent governance trigger. Only standing shared execution or a recognized consequence enters governance. |
| SIM-2 — bounded ruling attention | Kept ownership mandatory at minting, limited review conditions to four non-stable classes, batched due items into one five-item daily digest, and replaced per-row legacy review with one class-level adoption ruling and exception list. |

## Conditional assurance matrix: bounded assurance under named prerequisites

The honest headline is **bounded assurance under named prerequisites,
never certainty**. These rows state what must be true for each boundary
to constrain this incident class, how that prerequisite can fail, and
the focused test that must make the dependency visible. A missing or
red row reduces the assurance; activation does not talk around it.

| Boundary | Prerequisite | Failure mode | Code owner | Acceptance test |
|---|---|---|---|---|
| Governed-run admission | **Boundary completeness:** every standing shared process is launched or registered through the governed `run` path and bound to a claimed goal revision. | An unenumerated launcher spends outside the attempt and cost projection. | `cmd/metasystem/run.go` | `TestGovernedExecutionBoundaryCoversStandingProcesses` |
| Recognized consequence | **Boundary completeness:** refuse/accept work, discharge/reset obligation, promote authority, and authorize governed launch/spend all call one base authorization check. | A new or side-channel consequence takes effect without trigger data or authority. | `internal/authority` | `TestConsequenceBoundaryInventoryCoversEveryRecognizedEffect` |
| Trigger disposition | **Classification independence:** the builder may propose fields but cannot be the only classifier or erase a raised trigger; the policy-required reviewer appends the disposition. | Author-controlled `no` values bypass the selected review policy. | `internal/authority` | `TestTriggerClassificationRequiresIndependentDisposition` |
| Attempt terminalization | The goal reservation and terminal run record share an idempotent operation identity, and terminalization checks the post-attempt projection. | Attempt N turns red but retro/fence creation waits for N+1 or is lost between records. | `internal/run` | `TestTerminalAttemptRaisesRetroDebtAtExactLimitWithoutAnotherLaunch` |
| Budget projection | Governed run attempts, elapsed time, and minutes join the existing goal `Budget`; pending accounting is treated as spent. | A second tuple diverges, or a crash makes cost disappear and admission reopen. | `internal/goal` | `TestExecutionCostProjectsIntoExistingGoalBudget` |
| Assumption observation | The steward can read every fixed observation before launch and at terminalization; unavailable is fail-closed. | Stale or unobservable evidence discharges an obligation as if assumptions matched. | `internal/steward` | `TestGovernedRunLaunchRefusesUnavailableAssumption` |
| Independent economic warning | **Counselor landing:** the queued counselor goal and steward carriage have landed and their live delivery test passes. Until then, the breaker works but no independent economic-warning assurance is claimed. | The coordinator remains the only source of a value warning, or the design claims an unlanded seat. | `internal/steward` | `TestCounselorWarningReachesDigestWithoutCoordinator` |
| Ruling review | Minting enforces an owner; only the four scoped classes become due; the steward preserves overflow beyond the five-item daily ceiling. | Ownerless authority governs, or routine reviews create an unbounded Wido inbox. | `internal/steward` | `TestRulingReviewDigestCeilingAndOverflow` |
| Retirement observation | Both direct validations publish complete stage-results ledgers, and the steward diffs section IDs against the frozen catch-class inventory. | A retained check disappears while the two verdicts still look classifiable. | `internal/steward` | `TestRetirementWindowDiffsStageResultIDs` |
| Retirement restoration | The named tag remains readable and the archived wrappers, modes, hashes, and fixture entrypoints materialize in a temporary checkout. | A qualifying miss occurs but the promised restoration route is gone or untested. | `scripts/validate-metasystem.sh` | `TestBatteryRestorationTagMaterializesArchivedWrappers` |

## Implementation appendix

### Target files and responsibilities

| Target | Required change |
|---|---|
| `internal/goal/budget.go` | Keep `Budget` as the only limit family. Extend its projection to include governed-run reservations and terminal minutes; do not add an obligation tuple. |
| `internal/goal/file.go`, `internal/goal/verbs.go`, `internal/goal/validate.go` | Store the typed `EvidenceAssumptions`, append idempotent attempt reservations/terminal accounting and retro debt to the exact claimed revision, validate complete fields, and reject hand-edited or partial state. |
| `internal/run/run.go`, `internal/run/verbs.go` | Extend the existing run record and launch parameters with the obligation binding and observation fields below; make every terminal class consume its reservation; make terminalization own exhaustion. |
| `cmd/metasystem/run.go` | Add the governed `--goal` and `--reserved-min` grammar to launch/register, populate the currently empty `GoalId`, resolve and record the claimed revision, and leave unbound diagnostics unable to cause a recognized consequence. |
| `internal/authority/authority.go`, `cmd/metasystem/authority.go` | Own the closed consequence and trigger enums, the A/B/C mappers, append-only trigger raising/disposition, and the inventory of every recognized consequence. |
| `internal/steward/tick.go`, `internal/steward/health.go`, `internal/steward/narrate.go` | Evaluate assumptions, heal pending terminal accounting, carry retro and cost lines, produce the bounded ruling digest, and run the retirement stage-ID observer. Counselor carriage is added here only when its existing queued goal lands. |
| `scripts/agents/commit.sh`, `scripts/agents/land.sh`, `scripts/agents/land-fixtures.sh` | Execute the staged coverage delta once in `commit.sh`; pass the optional ratchet argument through `land.sh`; prove one execution and one verdict. |
| `scripts/agents/milestone-battery.sh`, `scripts/agents/battery.sh`, `scripts/agents/battery.conf.local.template`, `scripts/agents/gate-run-freeze-fixtures.sh` | Delete these wrapper-owned paths after the named restoration tag and restoration test exist. |
| `scripts/validate-metasystem.sh`, `scripts/agents/validate-section-selector.sh`, `docs/collaboration.md` | Retain the direct validator and stage-results format, remove wrapper inventory/selector/docs, add the retirement observer and restoration acceptance legs. |
| `memory/rulings.md` | In the activation change only, append the R-21 correction and one legacy class-adoption row with owners/exceptions; never rewrite existing rows. |

### One budget family and the run binding

The Law 1 three-control language is a projection of the current
four-field `goal.Budget`: `elapsedLimit`, `attemptLimit`, and
`reservedJobMinutesLimit` control elapsed time, attempts, and aggregate
execution minutes; `activeJobLimit` continues to bound concurrency.
`budget.go` remains the only parser, validator, and formatter. Runs and
jobs both feed the existing projection for the bound goal revision.
There is no `ObligationBudget`, no second cost unit, and no default.
The cost unit is one execution minute: a live run reserves the positive
integer supplied at admission; terminal accounting charges
`ceil(observed duration / one minute)`, with a nonzero attempt charging
at least one minute, and releases any unused reservation.

`run.Record` keeps its existing `GoalId` and adds:

- `GoalRevision uint64` — exact claimed revision resolved by the engine;
- `AttemptOrdinal uint64` — cause-blind ordinal allocated by the goal
  transaction;
- `ReservedMinutes uint64` — the declared reservation from
  `--reserved-min`;
- `AssumptionObservation` — the fixed fields and state defined above;
  and
- `GoalAccountingState pending|applied` plus the idempotent accounting
  operation ID.

The public governed forms are:

```sh
bin/metasystem run launch --root <root> --id <run-id> --kind <kind> --display <line> --log <path> --goal <goal-id> --reserved-min <positive-int> -- <command...>
bin/metasystem run register --root <root> --id <run-id> --kind <kind> --display <line> --log <path> --goal <goal-id> --reserved-min <positive-int> --pid <pid>
```

The engine, not the caller, resolves `GoalRevision` and
`AttemptOrdinal`. A free diagnostic omits `--goal`; base consequence
verbs reject its evidence. A standing shared process must supply the
governed binding even when it cannot itself cause another consequence.

Admission is one write-ahead protocol. The goal transaction first
reserves the ordinal and minutes under the run ID/operation ID, checks
the budget and current assumption observation, and then the run store
writes the pending record before spawn, preserving today's launch
ordering. Failure to write or spawn terminalizes that same reservation
as `launch-failed`; retrying the operation ID returns the existing
reservation. At terminalization, the run lock writes the terminal
status, observed minutes, assumption state, exhaustion result, and
retro intent together, then idempotently projects them into the goal
record. If the goal write is interrupted, `GoalAccountingState=pending`
is itself spent and admission stays closed; the steward tick completes
the projection. Thus no cross-record crash can make attempt N or its
retro disappear.

### Retained command and obsolete fixtures

The retained validator command is exactly:

```sh
scripts/validate-metasystem.sh
```

The obsolete list is exact: delete
`scripts/agents/milestone-battery.sh`,
`scripts/agents/battery.sh`,
`scripts/agents/battery.conf.local.template`, and
`scripts/agents/gate-run-freeze-fixtures.sh`; remove only their rows
from the validator asset inventory, shell-syntax list, section
selector, and collaboration text. Keep
`scripts/agents/witness-gate-fixtures.sh`,
`scripts/agents/suite-progress-fixtures.sh`,
`scripts/adopt-fixtures.sh`, the `internal/gaterun` tests, the direct
stage-results writer, and VM validation.

### Focused acceptance tests

| Owner | Test names |
|---|---|
| Goal/run budget | `TestGovernedRunLaunchBindsClaimedGoalRevision`; `TestEveryAttemptTerminalClassConsumesBudget`; `TestExecutionCostProjectsIntoExistingGoalBudget`; `TestTerminalAttemptRaisesRetroDebtAtExactLimitWithoutAnotherLaunch`; `TestPendingTerminalAccountingKeepsAdmissionClosed` |
| Assumptions | `TestGovernedRunLaunchRefusesUnavailableAssumption`; `TestStewardReopensOnTypedAssumptionDrift`; `TestTimingEnvelopeSixHundredSecondsUsesObservedLoadFields` |
| Consequence authority | `TestConsequencePolicyAImplementsChapter6TriggerSchema`; `TestConsequencePoliciesBandCAreRecordedDepartures`; `TestConsequenceBoundaryInventoryCoversEveryRecognizedEffect`; `TestTriggerClassificationRequiresIndependentDisposition` |
| Simplicity boundary | `TestFreeRepeatingDiagnosticHasNoGovernanceRecord`; `TestStandingSharedProcessRequiresGovernedBinding` |
| Rulings | `TestRulingMintRequiresOwnerAndScopedReview`; `TestRulingReviewDigestCeilingAndOverflow`; `TestLegacyRulingsMigrateByOneClassAdoption` |
| Duplicate and retirement | `TestCoverageDeltaRunsOnceThroughCommit`; `TestAdoptionAndVMInvariantsRemainSeparate`; `TestRetirementWindowDiffsStageResultIDs`; `TestBatteryRestorationTagMaterializesArchivedWrappers` |

## The paper and the design, reconciled (m1, on Wido's order)

IN LINE: Law 1 applies ch.5/ch.11 budgets-that-stop and ch.10 recorded
assumptions and drift; Law 2 applies ch.6's bounded proof and review
triggers, ch.13 legislator authority, and ch.15
no-self-witnessing; Law 3 applies ch.13 ruling ownership and review;
the retirement applies ch.4 archaeology and ch.14 replacement
evidence and clean removal. The whole disaster was pre-catalogued in
ch.3's failure table.

USED IN THIS DESIGN BUT NOT YET IN THE PAPER (candidate paper
additions, each still needing Wido's word and the paper's own gate):
1. The honest verifier that fails itself: an artificial verification
   context manufacturing false reds until the system services its
   verifier instead of its product. The ledger preserves this as
   narrative evidence, while the cause-blind breaker stops without a
   disputed causal ratio. This belongs beside ch.3's failure modes or
   ch.9's hostile world, which covers only DISHONEST tools.
2. Optimism under authority (prediction laundering): operator
   forecasts functioning as human permission across cycles. Law 1 now
   binds forecasts as inferences that can authorize nothing. This
   belongs in ch.3 or ch.13.
3. The recurring obligation as a budget-bearing unit with a
   cause-blind counter is now Law 1's unit, offered as
   evidence-from-use against ch.16's open runaway-spend problem, not
   as its solution.
4. Concurrent lawful record-writers are now referenced in Law 3:
   machine-suffixed id minting and union merging are already landed,
   and the recorded origin/sibling check is the pre-build guard against
   parallel repair becoming silent duplication. This remains a
   possible ch.8 addition or an implementation-level lesson.

PAPER DEBTS NOW FOLDED INTO THE DESIGN:
1. Chapter 6's model-correlated judgment limitation is specified
   verbatim in Law 2, every trigger is executable authorization data,
   A is the compliant implementation, and B/C are explicit departures
   for Wido to accept or reject.
2. Chapter 14's replacement evidence window runs through two
   classifiable weight-triggered direct validations under an active
   stage-ID observer, with a tested tag restoration route.
3. Chapter 13's ownership law is Law 3. Ownership is mandatory at
   minting; review is scheduled only for four non-stable ruling
   classes; legacy adoption and due delivery are bounded batches.
4. Chapter 10's evidence assumptions are part of every Law 1
   obligation revision; `ASSUMPTION_DRIFT` is a named reopen trigger,
   including for the former unstated 600-second expectation.

## The daily-use criterion (Wido, 2026-08-30, binding on this design)

"Look from the angle of simplicity and intuitive use. We should not
restrict to the point where we get stuck in administration and rigid
rules." This is R-11 sharpened into a usability test: the design is
converged only if an ordinary day under it carries LESS ceremony
than today, not more. The escape valves survive by name: R-12 (do
not block on the human for obvious things), fix-forward, and the
rule that experiments may exist and run freely — only GOVERNING
EFFECT is gated. A law whose daily cost is administration without a
protection story fails this criterion and is cut before activation.

The binding walk-through is therefore:

- An ordinary small fix creates no obligation, authorization, tuple,
  ruling, or review item: zero added ceremony.
- An ordinary delegated pass on an already claimed goal reuses that
  goal's one existing budget and automatically records its bindings:
  zero new tuple and zero Wido approval.
- A one-shot or repeating self-contained diagnostic has no governing
  effect: zero registration, tuple, or review.
- An exhausted governed validator raises one
  reduce/redesign/retire/extend decision at terminal attempt N while
  fix-forward product work remains free; only another governed launch
  is fenced.
- Ruling housekeeping can create at most one digest of five items in a
  rolling twenty-four hours, and stable standing rulings create none.
- Initial activation is one package. C is the named daily-use-safe
  start only if Wido records its Chapter 6 departure; A or B may replace
  it when measured review volume justifies their extra protection.

Any implementation that adds per-pass approval, a second budget tuple,
registration for a free diagnostic loop, per-ruling legacy questions,
or a second counselor approval fails this section even if its other
tests pass.
