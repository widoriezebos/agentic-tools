# Examine existing evidence before repeating tests

- Kind: design
- Id: 01M3E9ME5GK7J1Z6ZAM22WEY05
- Status: draft
- Goals: examine-evidence-before-retesting

Design author: Codex coordinator, at Wido's request. Independent critic: Fable
5.1. This is a proposed implementation contract; no runtime behavior is changed
by this document. A clean critique does not constitute human acceptance.

## Intent and step 1

`metasystem test --goal G` establishes that the current change satisfies its
testing obligations using valid existing evidence and the additional execution
actually needed. Strict identity reuse stays automatic. Where changed inputs
require judgment, an independent examiner decides whether earlier observations
still support the current candidate before expensive test executions start.

Step 1 delivers this complete path through the existing test, review and verify
owners, including the native Go/coverage prerequisite and the ten adoption
scenarios that exposed the problem. It works when a completed passing Go gate
and nine passing adoption scenarios survive a failed outer adoption attempt:
after correcting the remaining scenario, examination can require only the
affected checks, and verification can consume the complete composed evidence.
The original failed attempt remains failed. No coverage requirement is lowered.

The explicit entry point `metasystem review evidence --goal G` performs that
same examination and reports which checks remain. It does not run those tests
or accept the candidate. There is no new top-level verb and no caller-controlled
skip or waiver flag. Public verbs continue to describe human and agent intent
(ruling R-126), with role routing behind them.

Deferred: deeper dependency analysis belongs to
`test-selection-improves-on-the-simple-rule`; overall observed-intent completion
belongs to `goal-completion-verifies-observed-intent`. Other opaque suites can
later expose components through the same result owner. Dashboards, a separate
cache or scheduler, automatic policy tuning and a standing reuse role are out
of step 1. Their absence does not prevent this first path from working.

## Existing owners and the actual gap

Source baseline: main `7b46c080b543aa1ca3b6e5ea8647e2f8971b7488`.

| Existing owner | Preserve or extend |
| --- | --- |
| `cmd/metasystem/intent_work.go` | Public `test` already delegates to the test runner. |
| `internal/testpolicy/select.go` | Required groups, obligations, risk and prerequisites remain authoritative. Examination cannot remove an obligation. |
| `internal/proofrun/test_result.go` | Already reuses complete passing groups from failed attempts; protects against newer failures, live producers, ambiguous ordering and stale freshness. Extend composition and validation. |
| `cmd/metasystem/test.go` | Insert examination after candidate/identity preparation and exact reuse, before residual worker execution. Existing preparation may build an engine. |
| `internal/proofrun/coverage.go` | Authenticates native coverage completion; current reuse additionally requires successful outer completion and a committed delivery receipt. An independently completed component needs its own custody, not removal of those predicates. |
| `cmd/metasystem/proof_run.go` | Native gate currently prints its result; retain its structured observations under the admitted attempt. |
| `scripts/adopt-fixtures.sh`, fixture-bed owner | Own scenario inventory, prerequisite, scenario execution and cleanup. Expose their completed results through existing proof custody. |
| `internal/dispatch/hazard.go`, `review_reference.go`, `internal/readsubject` | Own critic identity, fresh context, distinct session, subject and closed review reference. Extend the subject to include the evidence under examination. |
| `cmd/metasystem/intent_delivery.go` | Add the `review evidence --goal G` subject and help through the existing descriptor/router. |
| `internal/landing/testing.go` | Consume the same verifier decision used by `test verify`; no second acceptance interpretation. |

No component may become governed evidence by parsing a success sentence from
an old log. Historical external diagnostics remain diagnostics. New producers
must retain their own structured results and authenticated completion.

## Roles and authority

Paper chapters 6 and 7 govern this design:

- The builder produces a candidate and observations. It may suggest reuse but
  cannot give its suggestion delivery credit.
- The independent examiner judges applicability, records its limits and names
  missing proof. Its context contains intent, constraints, candidate, changes
  and observations, without the builder's private reasoning or abandoned path.
- The dispatch delegate arranges the examination and residual executions within
  the existing budget. It cannot certify its own choice of tests.
- The custodian mechanically checks the evidence chain and performs acceptance.
  It makes no semantic judgment about whether a change is harmless.
- The responsible authority decides genuine rule exceptions and budget changes.
  Ordinary reuse within this contract introduces no human approval step.

These are permissions at existing actions, not five new processes. Dispatch
uses the existing `code-critic` role with an evidence-subject return. Its identity
checks must prove an eligible completed examination by a fresh session distinct
from construction. The accepting action consumes that reference and cannot
substitute a caller-written role label or verdict. An
examiner repairs nothing; a repaired candidate needs examination bound to its
new identity by an eligible examiner.

Chapter 6 prohibits an old observation authorizing a new candidate merely
because their difference looks harmless. The new evidence is an examination
of applicability for the exact current candidate, together with any specified
current executions. Old test results retain their original source identities.
This extension must be implemented and enforced at verification before that
combination can authorize delivery; a prose note alone does not authorize it.

## One execution flow

1. Resolve G and freeze the current candidate using existing candidate snapshot
   and contract owners. Compute the current selection, identities, tools,
   environment and freshness requirements. Do not test a moving worktree.
2. Apply existing exact reuse without a model call. Missing or incomplete proof
   enters the residual execution set. Find completed prior observations whose
   differing input identities make semantic examination potentially useful.
3. If such observations exist, the existing review owner prepares an immutable
   evidence subject and dispatches one bounded independent examination. Repeated
   identical requests attach to the same in-flight or completed assessment.
   With no potential carry-forward, proceed directly to residual execution.
4. The examiner assigns every remaining required group or exposed component
   either an applicability decision citing original evidence, or fresh execution
   with concrete completion conditions. It states the changed inputs examined,
   causal rationale and limits. Uncertainty becomes a named test or explicit
   unresolved requirement; it never silently removes proof.
5. `test` runs the residual through existing runners. `review evidence` returns
   the same assessment without launching residual tests. Both commands report
   examination cost and the outstanding checks through existing reporting.
6. Verification recomputes identities and validates the joins. With unchanged
   subject and the specified passing executions, no second model call is needed
   just to acknowledge success. Failure remains failure; contradictory or
   unexpected observations return a specific unresolved requirement. Source,
   contract or relevant environment changes invalidate the old assessment's
   applicability to the new subject, without erasing the original observations.

An examiner may request a broader run when a shared effect is unresolved. Mere
identity mismatch does not automatically require a full suite. Explicit fresh
execution, cadence, platform and policy requirements remain mandatory. Budget
exhaustion or an unavailable examiner returns the normal resumable waiting or
refusal result; no silent unbounded full-run fallback and no silent acceptance.
The ordinary force-execution path remains available within existing authority.

Both entry points use one evidence-kind examination owner and attach to its
same assessment; step 1 does not combine a commit-review subject and an evidence
subject in one return. Add `role.code-critic.evidence.tool-calls` to the existing
configuration owner, shipping the value 24 in metasystem.conf. The review owner
writes that value into `Maximum reader tool calls: N` in the evidence brief;
`test` supplies no caller flag. Missing, malformed or non-positive values refuse
before dispatch, naming the key. The approved goal budget still bounds admission:
one new assessment consumes one review admission, and attachment consumes none.
Exhaustion returns the existing named budget remedy before a model starts. No
separate spending account is added. Subject equality uses
the existing freshness episode/binding and expiry, never a new invocation ID
or current timestamp. Expiry invalidates reuse without inventing a new episode.

Prepare the frozen subject and finish examination before admitting the heavy
residual test attempt. The examiner has its ordinary bounded review-job admission;
it holds no live proof attempt while considering reuse. Before admitting the
residual, recheck subject identity and any new contradictory observations. An
already admitted proof producer for that context is observed through the existing
wait/reuse path, not duplicated. A combined code-review/evidence return is deferred.

## Result contract and custody

Use the existing review subject and testing result stores. Add an evidence
subject kind and an assessment reference, with these fields:

| Field | Meaning |
| --- | --- |
| Current subject | Exact candidate tree, candidate-engine identity, effective contract/selection digest, environment/tool identity and freshness context. |
| Original observations | Attempt and group/component IDs, original execution identities, immutable result/report hashes and original source revisions. |
| Decisions | Current required group/component and its existing obligations; `applicable` with cited original observation, rationale and limits, or `execute` with selected checks and exact completion conditions. |
| Examination reference | Authenticated code-critic job/session, evidence subject, immutable terminal return and review-owner completion reference. |
| Residual results | References to actual current executions that discharge the `execute` decisions. |

An evidence assessment is complete when its authenticated examiner job ends
`completed` with a schema-valid decision return bound to the exact evidence
subject. The review owner collects that immutable return and records completion;
`ReturnBindsSubject` and the reference validator gain the evidence-kind branch.
That branch validates the fresh critic identity, terminal return digest and
complete decision map. It does not require an author-folded finding register
or `metasystem close --dispositions`, and it does not mark an ordinary code/design
review chain closed. No builder-supplied disposition can alter a decision.
`execute` entries are completed examination decisions whose execution remains
outstanding; they are not findings for the builder to dismiss. Malformed or
unfinished assessments remain incomplete. Existing code/design review closure
semantics stay unchanged. This is evidence examination completion, not candidate
acceptance. Citations resolve only within the subject's frozen observation set.
The reference validator re-derives completion from the admitted job and immutable
return; it checks fresh dispatch, no parent/resumed session, and a session absent
from the construction chain. The decision map must exactly cover its subject's
required groups/components. A mismatched return digest or missing/extra decision
refuses even if a caller presents a completion pointer.

Extend the existing result schema/version negotiation so older engines reject
unsupported examined reuse. Preserve legacy/exact reuse compatibility. Add a
distinct `examined-reuse` group/component outcome; do not label it a fresh pass
or an exact match, or copy the current execution identity onto the old result.
`ValidateTestResult`, delivery recomputation, retained verification, cost/report
accounting and landing must agree on this outcome and its required joins.

The verifier checks that every currently selected obligation and prerequisite
has complete passing execution, valid exact reuse or a supported examination
decision plus its residual proof. It checks immutable references, authorized
independence, source/environment binding, collection completeness, freshness,
newer contradictions and policy. Duplicate/missing/unknown mappings, forged
references and stale assessments refuse with the missing fact and next action.
Free-text rationale is retained for examination and audit, never evaluated as
an executable approval rule. Changing a stored rationale breaks its digest.

Contradictions are evaluated at the granularity being cited, within the
parent group's relevant execution context (the cited or current identities,
tools, environment and freshness). A completed component is not contradicted
by its own enclosing attempt's later failure. It is blocked by a newer failed
or incomplete observation of that component, an ambiguously ordered observation,
or a live producer for the relevant parent. A newer failed/incomplete parent
observation without component detail blocks reuse: legacy opacity cannot hide
a contradiction. A newer detailed parent constrains its named components and
parent-only checks; it does not erase a passed sibling merely because another
sibling failed. Unrelated candidate contexts are not blanket vetoes, but all
relevant intervening observations belong to the examiner's frozen subject.
Before acceptance, recheck for observations that arrived after that snapshot;
a new relevant contradiction invalidates the assessment. The current residual
attempt supplies its actual results and cannot reuse its own unfinished work.

## Native coverage and adoption components

Retain native `GroupResult` observations at the existing producer boundary,
including expected and observed terminals, skip reasons, input identities and
report hashes. A successful subgroup does not imply successful collection of
an incomplete native run.

The authenticated coverage-completion owner records a durable completed
coverage component inside the existing attempt, with the actual native gate
success, complete package inventory, raw measurements, ratchet digest, source,
instrumentation/tool/platform identities and reports. Pending coverage alone
cannot be reused. Native success is derived from the gate owner's retained expected/observed
terminals and actual zero exit, never from a shell success argument. Completion
is written under the existing mutation lock while the producer owns a live,
uncancelled slot. Later reuse requires the attempt to be terminal, of any
outcome, plus complete authenticated component custody. The parent may
subsequently fail; that does not alter the component's terminal record. Preserve existing whole-attempt receipt reuse for
old records; add the narrowly authenticated component route for new records.

The new component route records a versioned `CoverageCodeIdentity` through
`CompleteCoverage`. Its deterministic projection includes the full Go source
and test trees (including testdata and embedded assets), go.mod/go.sum, resolved
dependency/toolchain identities, instrumentation/build flags, selected native
test inventory, package inventory, ratchet and platform. Reuse the existing
engine-projection/manifest implementation and native discovery owners to build
it; do not use `FullDigest` as this narrower code identity. A changed projection
requires the existing complete coverage producer in step 1.

Projection equality is necessary, not sufficient: Go tests can read files
outside their source directories. Keep the original full input manifest and
compare the complete old/current input delta in the evidence subject. Any change
outside the projection requires the examiner to establish that it cannot alter
native test selection, execution paths, fixture inputs or measured statements;
otherwise require fresh coverage. This is bounded semantic examination, not a
claim that a file-path projection discovers all runtime dependencies. For the
audit-only example the examiner must check the shell expectation's consumers
and the actual native invocation, not just its extension. A changed native
input that affects measurement is never covered by unchanged Go source alone.
An applicable coverage decision enumerates every examined outside-projection
path; verification requires set equality with the recorded complete input delta,
refusing any omitted or extra path. Coverage values remain the old producer's
observations, explicitly referenced by the new applicability decision; no fresh
percentage is invented.

Old whole-attempt coverage reuse keeps its full-identity predicates. Only the
new authenticated component route consumes the code identity plus examination
of any remaining delta. Package-by-package numerical coverage composition is
deferred; it is not needed for the failed-parent/audit-correction case.

The adoption parent exposes the native/coverage prerequisite, each scenario,
and its own required setup/validation/cleanup results. Scenario IDs come from
the existing fixture inventory; do not maintain a second scenario list. The
existing fixture-bed capability binds each child result to its admitted parent,
exact scenario and candidate. Publish completion only after required child
cleanup; a killed child or incomplete collection cannot earn a passing result.

Keep one aggregate adoption obligation. Its verifier requires the entire
current scenario inventory and all parent prerequisites/checks, each satisfied
by actual or applicable evidence. A partial rerun reports component completion,
never a newly passed full historical parent. Residual scenario execution uses
the existing scenario selection mechanism under admitted authority, without
asserting a fabricated full-gate witness or skipping a parent-only check.
Extend the current result/worker protocol for these nested components; a
component result names its parent, inventory ID, identity, actual exit,
collection/cleanup status and report digest. Add the corresponding structured
section-result records; a partial selection has aggregate status `partial`,
never `pass`, so `parseSectionResult` cannot mistake it for a full run. The
trusted parent publishes them after its process reaper and cleanup complete.
Use the parent's named-selection entry point, never direct child-environment
invocation. There is no arbitrary DAG runner or log-fragment import.

## Acceptance examples and proof obligations

| Id | Severity | Required behavior | Owner | Implementation proof to add |
| --- | --- | --- | --- | --- |
| ER-1 | critical | Only an eligible independent examination grants semantic reuse; construction/dispatch cannot self-certify. | dispatch/readsubject + verifier | Reject builder session, forged role/reference, wrong subject and incomplete examination; accept legitimate fresh examiner. |
| ER-2 | critical | The decision binds the actual candidate and every required obligation. | proofrun/testpolicy | Missing/duplicate obligation, changed tree/environment, tampered report, newer failure or live producer prevents acceptance. |
| ER-3 | critical | Successful native/coverage components survive failed parents without laundering incomplete evidence. | proofrun coverage/native owners | Full native success plus later adoption failure is reusable; truncated terminals, pending coverage, missing inventory and altered measurement inputs refuse. |
| ER-4 | high | A corrected adoption scenario can complete the current obligation with prior applicable components. | fixture bed + proofrun | Ten original scenarios, nine pass, one audit failure; correct it, examiner requires audit and failed scenario, rerun those and required current parent checks; all obligations complete while old parent remains failed. |
| ER-5 | high | Evidence review runs before expensive repeated tests and is safe to repeat. | test/review commands | Exact reuse makes zero model calls; identical requests attach to one examination; explicit review runs no residual tests; test follows the same assessment. |
| ER-6 | high | Residual execution and failure remain truthful and budgeted. | dispatch/proofrun | Failed residual, timeout, unavailable examiner and exhausted budget remain incomplete; unchanged successful residual needs no second model call. |
| ER-7 | high | Semantic reuse cannot disguise fresh execution or weaken coverage. | result schema/report/landing | Unsupported schema refused; separate outcome/provenance and cost; no lowered ratchet, relabeled historical pass or diagnostic-log import. |

These are implementation obligations, not claims of executed proof. The build
must use the existing tests and fixture injection seams, with focused tests
first and the selected testing contract at integration. No runtime suites are
required merely to save and review this design document.

The examiner's semantic quality needs separate evaluation from deterministic
custody. Replay the audit-only change, a shared initializer change, generated
asset change and dependency/tool change. Known changed behavior must not gain
unsupported credit; benign changes should avoid unnecessary broad execution.
Record missed-regression findings and avoidable execution cost. A correct
bookkeeping test cannot by itself certify the model's judgment.

## Implementation handoff

Implement in the existing owners in this order: durable component completion;
evidence subject and independent assessment; test orchestration and result
composition; public help/reporting and landing consumption. These are parts of
one usable step 1, not independently advertised half-features. Enable semantic
reuse only once its final verifier and discrimination fixtures exist.

Update the testing contract documentation and the paper's implementation-gap
account to describe the new current-candidate applicability evidence accurately.
The paper's role separation and exact-candidate requirement remain intact.
Install the compatible result/worker protocol on participating seats before
enabling this path; an older engine refuses the new schema explicitly. No new
dependencies, blanket skipping policy, permanent role or separate approval
hierarchy are required.

The motivating incident and its prior external diagnostics are summarized
here so the design does not depend on an untracked evidence directory. The
tracked verb verification record at `plans/verbs-match-intent-verification.md`
preserves historical provenance. This design and its Fable critique/dispositions
are the authoritative implementation handoff for this goal.
