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

| Run ID / subject | Duration / terminal class | Discovered condition | Orchestration defect | Coordinator decision error | Surviving protection |
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
  ordinary fix.
- Chapter 7 forbids self-acceptance. It does not forbid every
  author/operator overlap.
- Chapter 11 requires proportionality and the smallest sufficient
  machinery. It indicts the unbounded orchestration cost without
  erasing protection that proved useful.
- Chapter 12 supplies graduated-authority principles. The named
  `DRAFT -> OBSERVE -> LIMITED -> ENFORCED` states come from the
  operator-surface design, not from Chapter 12 itself.
- Chapter 13 assigns promotion authority here to Wido as legislator;
  the coordinator may propose evidence but may not promote its own
  mechanism.
- Chapter 14 requires a declared purpose and end condition for
  coexistence and clean removal on replacement. The landing gates do
  not automatically replace the full validator because the ledger
  shows unique full-sweep catches.
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

The retirement is one clean-cut landing:

- Delete `milestone-battery.sh` and `battery.sh`.
- Remove their validator enrolment and obsolete fixtures.
- Replace collaboration instructions with the one retained
  direct-validator command.
- Require a clean committed subject and record its `HEAD`.
- Remove clone/controller/checkpoint terminology.
- Verify that no canonical documentation or runnable path still names
  either retired entrypoint.

Until every item lands together and the final reference check is
green, the orchestration is not reported deleted. The parked
adoption-bed red is not declared moot; the retained direct validator
must resolve it.

## The way out: three laws

1. BUDGET BEFORE GOVERNED EXECUTION. No recurring governed run
   launches without a human-authorized obligation revision and
   complete budget. Every attempt and cost is recorded from the same
   facts; limit exhaustion refuses the next launch and raises the
   retro.

2. AUTHORITY ONLY AT THE CONSEQUENCE BOUNDARY. Experiments may exist
   and run, but cannot refuse, discharge, promote themselves, or
   authorize more spend. Human authorization plus old-basis evidence
   grants bounded authority.

3. ONE PRIMARY OWNER PER PROTECTED CONDITION. A new protection names
   the existing protection, its unique marginal purpose, and its
   review or replacement condition. A replacement landing deletes
   the replaced path.

Cost lines are a view of the attempt record. Failure retro is the
terminal output of an exhausted budget. Counselor observation uses
the same facts. No separate universal mechanism registry or
per-round stop-reason machinery is justified.

### Consequence-boundary implementation

The first implementation applies only to recurring executions that
can stop work, retry automatically, or discharge an obligation.

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
- Wido authorizes `LIMITED` or `ENFORCED`. The coordinator cannot
  promote, rebudget, or accept its own authority mechanism.
- The base engine/action boundary, not candidate code, checks the
  authorization.

This governs effect at the consequence boundary. It does not make
the existence of experiments or commands conditional on a universal
mechanism lifecycle.

### Cause-blind obligation budget

Every recurring governed obligation carries a human-set attempt,
elapsed-time, and execution-cost budget. Every admitted launch
counts, regardless of failure cause. A different red does not reset
the counter. Successful discharge or a human-authorized obligation
revision may reset it. At any limit, admission refuses the next
launch, preserves all evidence, raises retro debt, and requires a
recorded reduce/redesign/retire/extend decision.

The tuple is complete and human-set. The machinery does not infer a
number from prose, insert a default, add grace, or reset on a new
cause. This carries R-13’s no-invented-values rule into recurring
obligations.

Continue-and-collect remains inside one run so an expensive attempt
teaches everything its declared dependencies permit. The budget
boundary controls whether another run may launch.

### Independent economic path

- Counselor observes proof cost, process/product inversion, and
  repeated unsupported assurances, then reports them to Wido without
  refusal power.
- Steward independently schedules and carries that signal. It cannot
  be suppressed by the coordinator and does not compose or judge the
  counselor’s content.
- Wido rules on value, budgets, and mechanism authority.
- Coordinator supplies records and implements the ruling. It cannot
  close or lower the review.

Economist sensing merges into the counselor charter, and the
economist goal closes as a duplicate. If Wido wants a separate
economist, R-1’s conflict test runs first: name a power that the
counselor cannot lawfully hold, prove that combining it would create
a responsibility conflict, and obtain human approval for the new
seat.

### Duplicate-pair dispositions

- Full-validator launchers: retain one admitted full-validator path
  and delete both wrappers.
- Coverage delta: make `commit.sh`, or one shared lower-level
  function, the sole execution owner. `land.sh` passes any ratchet
  argument through instead of running the check twice.
- Per-landing delta versus full ratchet: retain both provisionally
  with distinct purposes. The delta stops coverage debt at the
  landing that creates it; the full ratchet audits the repository.
  After a declared milestone window, the coverage owner reviews
  whether the full sweep still catches unique debt.
- Counselor versus economist: merge unless a human-approved R-1
  power conflict justifies separation.
- `run` versus standalone shell custody: only admitted `run`
  evidence may discharge a governed obligation.
- VM versus adopt bed: retain them with their separate invariant
  statements and separate review conditions.

## Minimal ownership and proof matrix

| Obligation | Decision owner | Mechanical owner | Acceptance evidence |
|---|---|---|---|
| Incident record | Post-mortem owner | Reconciled envelope ledger | 24 rows sum exactly |
| Attempt budget | Wido | Run/steward admission | N+1 refused at boundary |
| Rule activation | Wido as legislator | Base action boundary | Exact accepted revision and scope |
| Economic warning | Counselor | Steward carriage | Durable brief delivered independently |
| Orchestration retirement | R-21 custodian | Same deletion landing | No code/docs reference remains |
| Duplicate protection | Named condition owner | Existing gate owner | Purpose plus review/end condition |

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

## The proof (m1, written over the folded design, as demanded)

Wido's demand: prove that this design fixes all the fuck-ups around
the battery so they can never happen again. Proof, well argued.

THE CLAIM, stated precisely. Two parts. Part A, argued to certainty:
the observed disaster class — an unbudgeted recurring obligation
retried without bound, its aggregate cost invisible, judged only by
its own author, its learning gated on its own success, with human
conduct as the only safeguard — CANNOT RECUR once the three laws are
active, because the disaster is a conjunction and every conjunct is
mechanically negated at a boundary my conduct cannot cross. Part B,
argued honestly: novel failure classes are not excluded by any finite
design — but under the same laws any novel runaway is BOUNDED and
LOUD by construction, so the specific inexcusable shape — thirty
hours and twenty-four launches before a human forces the question —
is impossible for ANY future mechanism, known or novel.

PART A, BY NECESSARY CONDITIONS. The disaster required ALL of:

(1) EXISTENCE WITHOUT A BUDGET. Negated by Law 1: a recurring
governed run does not launch without a human-authorized revision and
complete budget tuple. The refusal sits in run/steward ADMISSION in
the base engine — the same boundary that already refuses budgetless
goal claims today, a mechanism proven refusing in production. I
cannot supply the tuple myself: R-13 (machinery never invents
values) already binds, and Law 2 forbids the coordinator rebudgeting
its own mechanism. The battery existed unbudgeted for ten days; under
Law 1 it cannot exist governed for one launch.

(2) UNBOUNDED RETRY. Negated by the cause-blind budget: every
admitted launch counts — setup failures, aborts, stale subjects,
invalid results included — and a different red does not reset the
counter. This is not conduct; it is the same admission check reading
the same attempt record. Launch N+1 past the limit is REFUSED the
way an over-budget reservation is refused today. The twenty-four
launches happened because nothing counted; under Law 1 the count IS
the admission input.

(3) INVISIBLE AGGREGATE COST. Negated structurally, not by a
reporting habit: the attempt record that admission reads is the cost
line — one fact store, two views. The narrator digest renders it
(delivery proven live in every Stop payload since the digest
landing), but even if narration failed entirely, admission still
refuses at the limit, because visibility and enforcement read the
same record. The disaster needed cost to be both unenforced and
unseen; now enforcement does not depend on seeing.

(4) SELF-JUDGMENT AND SELF-PROMOTION. Negated by Law 2: no mechanism
acquires governing effect — refusing work, discharging an
obligation, resetting weight, authorizing spend — without Wido's
recorded authorization checked at the base action boundary, per
ch.13 (legislator) and ch.15 (a change cannot be its own witness).
DRAFT and OBSERVE states can only record what they would have done.
I authored, operated, and judged the battery; under Law 2 the third
seat is structurally not mine to hold, and the check runs in the
base engine, not in anything I can edit into agreement.

(5) SUCCESS-GATED LEARNING. Negated by making the failure retro the
TERMINAL OUTPUT of an exhausted budget: the retro debt rises at the
halt, mechanically, from the same admission event. The battery's
retro was waiting for the green that never came; under Law 1 the
retro is what exhaustion produces.

(6) CONDUCT AS THE LAST SAFEGUARD. Negated by composition: every
negation above fires at an admission or action boundary in the base
engine. My optimism cannot authorize a launch (the binding forecast
rule: forecasts never authorize; only the recorded budget does — and
the budget is not mine to set). My diligence is no longer required
for any protection to hold; it is redundant on top of refusal. The
counselor-steward carriage delivers the economic warning on the
steward's schedule, which I cannot suppress — the independent seat
uses machinery that already survives me (the steward has run
unattended through this entire saga).

A conjunction with every conjunct false is false. The observed
disaster class cannot recur. That is Part A, and I state it with
certainty CONDITIONAL on exactly one thing, named below.

EVIDENCE COMPLETENESS. The twenty-four-row ledger above assigns
every launch of the saga to a terminal class. Walk the classes:
orchestration self-defects (the largest class) die with the deleted
orchestration — a mechanism that does not exist cannot fail; its
deletion is verified by the clean-cut landing spec (no runnable path
or doc names the retired entrypoints). Debt-discovery reds (coverage,
callers, fixture-law lag) are covered by per-landing gates that run
at every landing at real HEAD — and those gates are themselves
protected by Law 3's single-owner rule from silently duplicating or
decaying. The two real product defects are found identically by the
retained direct validator — retirement removed the wrapper, not one
check. Coordinator decision errors (double-launch, stale-subject
launch, landing past a red gate) are absorbed by (2): they consume
budget and hit the breaker instead of compounding invisibly. No row
of the ledger falls outside a negated class. The mapping is total
over everything observed.

PART B, THE HONEST BOUNDARY. Per ch.6 and ch.15, no proof exceeds
its boundary, and a design cannot exclude failure classes nobody has
imagined — claiming otherwise would be the exact fluent false
certainty that fed this disaster. What the laws guarantee for ANY
future mechanism, novel failures included: it launches with a
human-set budget or it has no governing effect at all; it spends at
most its tuple before admission halts it and raises the retro; its
cumulative cost is one durable record readable by human, counselor,
and admission alike; and it cannot judge or promote itself. The
worst possible novel failure is therefore: a mechanism spends
exactly its human-authorized budget, stops, and files for its own
post-mortem — in the open, at a cost Wido chose in advance. The
inexcusable shape — unbounded, invisible, self-judged — is
unreachable for mechanisms that do not yet exist, which is the
strongest claim about the unknown that honesty permits.

THE ONE CONDITION. This proof holds when the three laws are LANDED
and ACTIVATED — and per Law 2 itself, activation is Wido's act as
legislator, not mine. Until his word, the protections are conduct,
and conduct is exactly what this document proves insufficient. The
proof therefore ends in a request, not a declaration: authorize the
three laws, and the machinery makes this class of failure
impossible; every day before that word, the only safeguard is the
one that already failed.
