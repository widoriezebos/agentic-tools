---
name: "take-a-step-back"
description: "Use when work is stuck, repeated tweaks are not converging, progress is unclear, abstraction layers are growing without a current production consumer, or the user asks to step back. Force an evidence-first workflow with mandatory stop-loss and necessity gates: freeze symptoms, challenge the parent premise, require cycle contracts, classify real product progress, prefer existing owners/deletion, choose the lowest-total-lifecycle-complexity robust solution, and implement only evidence-justified designs."
---

# Take A Step Back

Use this skill when the work is drifting into low-value iteration, patching symptoms, or weak "try something" loops.

The point is to stop editing on intuition and force an evidence-backed redesign.

## Modes

Choose the stopping point from the user's intent:

- `analysis-only`: stop after `Supported Theories` or `Investigation Redesign`
- `design-only`: continue through `Design`, `Design-Principles Compliance`, and `Implementation Plan`, but do not edit code
- `implement`: continue through implementation

If the user explicitly asks for analysis only, do not continue into implementation. If the user explicitly asks for design only, stop after the plan. Otherwise continue through implementation.

## Hard Rules

- Treat roadmap, sketch, and earlier-plan architecture goals as hypotheses until the current user request or a named
  current production need makes them mandatory.
- Before adding a compiler, interpreter, generic executor, executable schema/DSL, registry/plugin mechanism, receipt
  framework, or bulk migration, name the current production caller, user/product outcome, existing owner, smallest
  direct-code/deletion/no-change alternative, first production slice, and code replaced now. No caller means stop,
  delete, or park unless the user explicitly requested an extension language/plugin surface.
- A fresh-eyes critique must answer “should this exist?” before improving mechanics. It must compare the existing owner,
  direct typed code, deletion, and no-change alternatives.
- Choose the robust solution with the lowest total lifecycle complexity. Do not equate the strongest guarantee, largest
  atomic boundary, or broadest abstraction with the best design. Compare plausible current-system alternatives such as
  no change, operational mitigation, direct correction, idempotent retry, transactional outbox, extension of an
  existing transaction, a broader transaction, and a reusable framework. This is a candidate set, not a universal
  complexity ranking. Count production code, persistent state, migration, recovery, testing, monitoring, and operations.
  Select a more complex design only when evidence shows that a lower-complexity alternative cannot satisfy the actual
  contract.
- Two preparatory checkpoints without a production-contract change trigger stop-loss. A child used only by another
  unimplemented child returns to the parent necessity decision and cannot spawn support infrastructure.
- Tests, schemas, registries, scanners, inventories, ledgers, and receipts for an unconsumed abstraction are enabling
  evidence, not `contract-improved` product outcomes.
- Freeze the full reproduction envelope before theorizing.
- Before proposing new theories, run the `Known-Diagnosis Gate`. Do not re-prove a root failure mode that the ledger
  or artifacts already support.
- Every evidence cycle, benchmark run, debugger pass, or code patch must pass the `Novel-Learning Gate`: it must name
  what new decision-relevant fact it can produce that prior artifacts do not already establish.
- Every evidence cycle, benchmark run, debugger pass, model-heavy run, or code patch must have a written `Cycle Contract`
  before it starts and a written `Run Classification` immediately after it ends.
- No production-logic changes before at least one theory is strongly supported by evidence.
- When a supported diagnosis points to a broken invariant, state transition, selection gate, closure gate, or ownership
  boundary, run the `Boundary Trace Gate` before design or implementation. Do not patch until the owner and focused
  invariant test shape are explicit.
- Problems are symptoms, not causes. Keep them separate.
- Classify each problem as `anchor symptom`, `consequential symptom`, or `unknown causal position`.
- Prefer 2-4 theories. Use 5 only when they are genuinely distinct.
- Every theory must be falsifiable.
- Rank theories by explanatory fit to anchor symptoms first, then impact, discriminability, and cost to falsify.
- Theories must explain anchor symptoms directly. Consequential symptoms may corroborate scope or fallout, but must not drive theory selection on their own.
- For every theory, record supporting evidence, contradicting evidence, decisive missing evidence, and the cheapest check.
- Prefer existing artifacts, tests, logs, and debugger inspection before adding new logging.
- Add the smallest possible structured logging at the owning decision point only when no cheaper decisive check exists. Do not add broad narrative logging.
- Run one high-signal reproducer per evidence cycle.
- Persist theory status in a committed investigation ledger before reverting or discarding a line of work when the run produced a real learning.
- If a case-specific investigation ledger already exists, use it as an input to the investigation rather than recreating the prior theory history from scratch.
- After two consecutive evidence cycles that do not narrow the top theory set or materially reduce uncertainty, stop and redesign the investigation itself before running anything else.
- After two failed implementation attempts inside the same mechanism or abstraction, mark that mechanism exhausted.
  Do not make a third tweak there unless new evidence reopens it; redesign the owning boundary instead.
- For benchmark/model-heavy investigations, one expensive `no-progress` run is enough to stop. Do not run another similar
  expensive check without an explicit user-approved budget extension after reporting the stop-loss trigger.
- Progress must be measured against the explicit user or benchmark contract, not against plausible-looking answers,
  cleaner logs, broader coverage, or local proxy metrics.
- For benchmark or expensive model work, establish a stop budget before running. If no contract-relevant progress appears
  after two failed implementation attempts or two hours, whichever comes first, stop and report unless the user explicitly
  extends the budget.
- If a run invalidates the current problem framing, restart from `Problems` instead of forcing the old story forward.
- Map every design decision back to one or more supported theories.
- Before design or implementation, write a `Local Learning Memo` and prove the proposed design does not violate it.
- Before implementation or any expensive validation of a design, run the `Design Obligation Gate`: extract the design
  obligations into a matrix with severity, owner, code proof, test proof, runtime proof, status, and next action. Do not
  implement or loop while any critical/high obligation is missing an owner. Do not start expensive validation while any
  critical/high obligation is `MISSING`, `PARTIAL`, or `CONTRADICTED`.
- Remove temporary instrumentation unless it is clearly valuable as permanent observability.
- Preserve meaningful progress with checkpoint commits before continuing into more edits or expensive validation.

## Non-Negotiable Stop-Loss Protocol

This section overrides any local urge to keep going, any prompt that says "loop", and the default `implement` mode.
If a stop-loss trigger fires, stop active work, report it, and do not self-authorize another patch or run.

Before every patch, test, debug run, benchmark, or model-heavy run, write a `Cycle Contract` in the response or the
investigation ledger:

- cycle id
- mechanism family being tested
- command, artifact, or files to change
- novel decision-relevant fact this cycle can produce
- exact contract signal that counts as progress
- max wall-clock, run-count, and token/cost budget for the cycle
- checkpoint strategy: whether the current state is already recoverable, or which commit/stash will preserve it before
  the next expensive run
- expected classifications and automatic next action for each:
  - `contract-improved`
  - `falsified`
  - `no-progress`
  - `invalid-run`

Do not start the cycle if the novel fact is missing, duplicates prior ledger evidence, or only promises cleaner logs,
broader coverage, a more plausible answer, or another local proxy metric.

Immediately after every cycle, write a `Run Classification`:

- `contract-improved`: the explicit user/benchmark contract improved, or the exact invariant became true where it was
  previously false.
- `falsified`: a leading theory or mechanism family was ruled out by decisive evidence and the do-not-retry list was
  updated.
- `no-progress`: the contract did not improve and no leading theory/mechanism was ruled out. This includes nicer
  answers, extra adjacent sources, more telemetry, longer runs, or local cleanup that does not change the contract.
- `invalid-run`: parity, timeout, stale artifact, or environment problems prevent interpreting the result.

Only `contract-improved` and `falsified` count as progress.

Local correctness of an unconsumed abstraction is `no-progress` against a product contract unless the user explicitly
requested that artifact. Do not upgrade it to `contract-improved` because tests or architecture gates pass.

For every `falsified` result, immediately add one of these sub-classifications:

- `falsified-continue`: the result names a new first owner with the required facts, a focused invariant test shape, and
  a mechanism family that is not exhausted.
- `falsified-dead-end`: the result proves the current mechanism family is exhausted, or proves no current owner has the
  required facts in the needed data shape.

`falsified-dead-end` stops the loop. Do not treat it as permission to redesign locally, patch a neighboring class, run
another reproducer, or continue because the investigation learned something useful.

### Checkpoint Preservation Protocol

Checkpoint commits are mandatory progress guardrails for stop-loss-governed work. They preserve recoverable states
because we cannot know up front whether the next change will prove useful or become a dead end.

Apply these rules before continuing after every evidence cycle:

- Do not checkpoint an unexamined dirty worktree. First identify unrelated or user-owned changes and avoid mixing them
  into the checkpoint unless the user explicitly asked for that scope.
- Before any expensive validation after a meaningful patch, ensure the current state is recoverable by an existing
  commit or a named stash. If it is not, create a checkpoint commit or named stash first.
- After `contract-improved`, commit the exact recoverable working state before further edits or validation. If the state
  is a benchmark frontier or materially best-known result, follow the repository frontier rule instead: commit, label or
  tag, push, and only then continue.
- After `falsified-continue`, preserve the learning before continuing. Commit the exact working state when the behavior
  changes remain useful evidence or a plausible stepping stone. If the behavior changes should not survive, commit only
  ledger, prompt, design, or skill evidence and stash or revert the failed behavior.
- After `no-progress` or `falsified-dead-end`, do not stack more behavior changes on top of the failed attempt. Preserve
  learning or ledger evidence separately, then revert or stash failed behavior before any redesign.
- Record checkpoint commit SHAs or stash names in the investigation ledger when a ledger exists.
- Checkpoint commits stay local unless the state is a frontier or the user explicitly asks to push them.

Mandatory stop-loss triggers:

- one `no-progress` result from a benchmark, debug reproducer, or model-heavy run that exceeded 30 minutes, materially
  increased token/cost, or repeated the same failure signature
- two `no-progress` cycles in the same investigation, regardless of mechanism wording
- two failed implementation attempts in the same mechanism family or abstraction
- two preparatory design/implementation checkpoints without a current production-contract change
- a proposed support layer whose only consumer is another unimplemented plan or abstraction
- one `falsified-dead-end` result
- elapsed time exceeds the declared stop budget
- the current cycle cannot name a novel decision-relevant fact
- the proposed next patch is a narrower tweak inside an exhausted mechanism family
- the investigation has spent more than 90 minutes since the last `contract-improved` or `falsified` result, unless the
  user explicitly approved a longer budget after seeing the stop-loss report

When a trigger fires:

1. Say `STOP-LOSS TRIGGERED`.
2. Kill or let finish no more than the current essential command; do not start another expensive run.
3. Record the last useful artifact and classification in the ledger when a ledger exists.
4. Revert behavior changes from the failed attempt after preserving ledger evidence, unless the state is a frontier or
   the user explicitly says to keep it.
5. If preserving the failed behavior for inspection is useful, stash it with a descriptive name; otherwise revert it.
   Leave the worktree clean or with learning/docs changes only.
6. Commit or prepare the ledger evidence separately from failed behavior changes when the learning matters.
7. Report the exhausted mechanism families and the next higher-level redesign question.

Branch hygiene after stop-loss:

- Do not promote a behavior branch after `falsified-dead-end`.
- Do not merge failed behavior commits into a frontier or main branch.
- If learning must be preserved on another branch, cherry-pick or commit only ledger, prompt, design, or skill files.
- If the next viable work needs a new data shape, owner, or boundary, start from a clean baseline with a new Cycle
  Contract instead of continuing on top of the failed attempt.

Do not bypass this by renaming the same mechanism. The same mechanism family includes variants of the same selector,
ranker, prompt shape, boundary handoff, threshold, or candidate expansion that attempt to fix the same failure without a
new decisive fact.

## Step 0. Freeze The Frame

Before theorizing, establish:

- exact failure signature
- impact and frequency
- current reproduction command or scenario
- exact repo state: `git` SHA and any relevant local changes
- build/runtime parity: whether the running jar/artifact is fresh relative to source
- config, profile, flags, and environment that influence behavior
- model, effort, or execution mode when relevant
- workspace, inputs, or dataset involved
- current run ids and artifact paths already in play
- affected scope
- non-goals
- success criteria
- suspected violated boundary or invariant, if already known
- recent failed fix attempts that should not be retried blindly

If the work is benchmark- or case-driven, first check for an existing ledger under:

- `plans/benchmark-investigations/<case-id>.md`

If a ledger exists, load and reuse:

- strongest known checkpoint
- already-invalidated theories
- surviving theories
- already-supported root diagnoses
- do-not-retry list
- prior decisive artifact paths

If any of these are unknown, discover them first from code, tests, logs, execution logs, benchmark artifacts, or debugger evidence.

Also establish a stop budget:

- max implementation attempts inside one abstraction
- max reproducer or benchmark runs
- max wall-clock time before reporting
- max token or cost budget when model-heavy runs are involved
- exact contract signal that counts as progress
- current investigation start time, last `contract-improved` or `falsified` timestamp, and hard deadline
- current mechanism family and whether it is already exhausted
- whether the next action is allowed by the `Non-Negotiable Stop-Loss Protocol`

For benchmark/source-coverage work, the contract signal must be the exact scored condition, such as required paths
becoming read-backed and final-cited. Do not count a better-looking answer as progress unless it improves that contract.

## Step 0A. Known-Diagnosis Gate

Before listing new theories, compare the current symptom against the investigation ledger and recent artifacts.

Classify the frame as one of:

- `new unknown`: no prior artifact explains the anchor symptom
- `known diagnosis`: prior artifacts already support the root failure mode
- `known diagnosis with new contradiction`: prior diagnosis exists, but a new artifact contradicts or materially narrows it

If the frame is `known diagnosis`:

1. Do not run another evidence cycle just to confirm it.
2. Write a `Known Diagnosis Decision` with:
   - the supported diagnosis
   - decisive artifact paths
   - invalidated mechanisms or approaches
   - the boundary or invariant that must change
   - the next design action
3. If the owning boundary, required data, and invariant test shape are not explicit, run the `Boundary Trace Gate`.
4. Otherwise skip to `Local Learning Memo` and `Design From Supported Theories`.

If the diagnosis arose inside an unconsumed abstraction, revalidate whether the parent abstraction should exist before
redesigning a child mechanism. Prefer deletion. A schema, loader, registry, or receipt platform is not a valid response
to a dead end in a component with no production caller.

The known-diagnosis fast path is always:

1. `Known Diagnosis Decision`
2. `Boundary Trace`, when the owner or invariant is not explicit
3. `Local Learning Memo`
4. `Design From Supported Theories`

Only continue collecting evidence when the next check can falsify the diagnosis, resolve a real contradiction, identify
the owning decision point, or validate a boundary-level invariant.

## Step 0B. Boundary Trace Gate

Use this gate when a supported diagnosis says a decision boundary, state transition, closure gate, selector, planner, or
invariant is wrong or missing.

The goal is to find the first owner that has, or should have, all facts needed to enforce the invariant. This prevents
another downstream patch that makes the symptom look better while the real boundary remains unchanged.

Prefer existing artifacts, ledgers, tests, code ownership, execution logs, and debugger state before running anything
new. A new run is allowed only if it can identify the owner, prove a contradiction, or validate the proposed invariant.

The default trace budget is one artifact trace plus one static ownership pass. If those do not name the owner and test
shape, record `patch decision: blocked` and redesign the investigation around ownership. Do not continue static
inspection until it becomes another unbounded evidence loop.

Produce a `Boundary Trace` with:

- contract signal: exact user, product, or benchmark condition that counts as progress
- violated boundary: the decision, state transition, closure gate, selector, planner, or invariant under suspicion
- owner candidate: class, method, module, or component that currently makes or should make the decision
- owner alternatives checked: earlier and later owner candidates, and why they are accepted or rejected
- first owner with required facts: the earliest owner that has, or should receive, all data needed to enforce the invariant
- input state at the boundary: only fields/facts that affect the decision
- competing evidence or candidates: stronger and weaker alternatives present at the boundary
- decision outcome: what the system did and how the downstream symptom followed
- missing data: facts the owner needs but does not currently have
- ownership correction: whether to move the decision, move the data, or create a narrow collaborator
- focused invariant test: neutral setup and expected result
- patch decision: `allowed` only when the first viable owner, invariant, data, and test shape are explicit

If the trace cannot name the owner and test shape within the trace budget, do not patch. The next action is to redesign
the investigation around ownership, not to continue reading adjacent code paths.

Record the boundary trace in the investigation ledger when the work is benchmark-, incident-, or case-driven.

## Step 1. Problems

List only observable problems. For each one capture:

- problem id
- observed behavior
- expected behavior
- artifact or code reference
- severity / impact
- whether it is `anchor symptom`, `consequential symptom`, or `unknown causal position`
- if not `anchor symptom`, what upstream symptom or mechanism it likely follows from

Do not include causes or fixes in this section.

Use the smallest set of `anchor symptom` entries that the investigation must explain directly.

- `anchor symptom`: upstream observable problem that theories must explain directly
- `consequential symptom`: downstream fallout that may disappear if an anchor symptom is fixed
- `unknown causal position`: symptom whose placement in the chain is not yet supported by evidence

## Step 2. Top Theories

Produce up to five theories, ranked by expected investigation value.

Rank using:

- explanatory fit to the anchor symptoms
- ability to account for consequential symptoms without overfitting to them
- impact if true
- discriminability from competing theories
- cost and speed of falsification

Each theory must be:

- falsifiable
- distinct from the others
- able to explain one or more anchor symptoms directly
- treat consequential symptoms as corroboration, not the primary justification
- tagged as `root cause`, `amplifier`, or `secondary effect`

Reject theories that are just restated symptoms or disguised solutions.

## Step 3. Evidence Matrix

Build a matrix with one row per theory and these fields:

- `anchor symptoms explained`
- `consequential symptoms explained`
- `supports`
- `contradicts`
- `unknown`
- `missing evidence`
- `cheapest decisive check`
- `logging to add`
- `predicted result if true`
- `predicted result if false`

Evidence can come from:

- code ownership and data flow
- tests
- execution logs
- stdout/stderr
- runtime logs
- benchmark artifacts
- debugger state when needed

Prefer contradiction evidence over piling up more support for the current favorite theory.

## Step 4. Logging / Instrumentation Needed

If evidence is missing, add minimal structured observability.

Before adding logging, exhaust cheaper decisive checks from:

- existing code and ownership boundaries
- current tests
- execution logs and runtime logs
- benchmark or replay artifacts
- debugger inspection when justified

Prefer logs that answer branch and ownership questions directly:

- which owner made the decision
- the input state that mattered
- branch or gate result
- candidates considered and rejection reasons
- budgets, limits, or thresholds
- terminal stop reason
- correlation ids or run ids

Guidelines:

- instrument the owning component, not a random caller
- log both positive and negative outcomes
- log structured values, not prose summaries
- preserve stack traces for exceptions
- keep the signal permanently only if it is operationally useful after the investigation

## Step 5. Single Run To Gather Evidence

Run one reproducer chosen to discriminate between theories as fast as possible.

Before running, write the `Cycle Contract`. It must state:

- why this run is decisive
- what new fact this run can teach that prior artifacts do not already establish
- which theories it can confirm or kill
- what exact outputs, log lines, or artifacts you expect
- the contract signal that will count as progress
- the stop or revert decision tied to each expected outcome
- the max wall-clock, run-count, and token/cost budget for this run

If the run cannot produce a novel decision-relevant fact, do not run it.

After the run, write the `Run Classification` immediately, then update the evidence matrix.
If the result materially changes the theory set, update the investigation ledger immediately as well.

The ledger update should capture at minimum:

- theory ids and new status
- decisive run id
- decisive artifact paths
- cycle classification: `contract-improved`, `falsified`, `no-progress`, or `invalid-run`
- keep/revert decision
- any new do-not-retry guidance
- whether the owning abstraction is now exhausted
- whether the contract signal improved, regressed, or stayed unchanged
- whether a stop-loss trigger fired

For source-coverage failures, do not treat "some related file was read" as success. Track whether the exact required
source became read-backed and final-cited, or whether the requirement was deliberately kept unresolved.

If the classification is `no-progress`, apply the `Non-Negotiable Stop-Loss Protocol` before considering another run or
patch. Do not turn `no-progress` into a new local implementation task by default.

## Investigation Redesign Gate

If two consecutive evidence cycles fail to narrow the top theory set, rule out at least one leading theory, or materially reduce uncertainty:

1. stop repeating the same style of checks
2. state why the current evidence plan is non-discriminating
3. redesign the investigation itself
4. choose a new decisive check or conclude that evidence is insufficient for a safe change

Do not move into design or implementation until this gate is satisfied.

For benchmark/model-heavy loops, a single expensive `no-progress` run triggers this gate immediately. Do not spend a
second expensive run to learn that the same failure signature still exists unless the user explicitly extends the budget.

If two implementation attempts in the same mechanism fail to improve the contract signal:

1. mark that mechanism or abstraction `exhausted` in the ledger
2. list the exact patch families that must not be retried
3. stop editing inside that abstraction
4. redesign the owning boundary or invariant

Do not reword an exhausted mechanism as a new theory unless new evidence materially distinguishes it.

An `Investigation Redesign` must change at least one of these: owning boundary, invariant, data flow, reproducer
predicate, or mechanism family. It is not a redesign if it only changes prompt wording, ordering, thresholds, caps,
candidate counts, or another parameter inside an exhausted mechanism.

## Step 6. Supported Theories

Summarize:

- theories now strongly supported
- theories partially supported
- theories ruled out
- anchor symptoms now explained
- residual ambiguity that still matters

If no theory is strongly supported, go back to `Evidence Matrix`. Do not design yet.

Before design or implementation, cross-check the surviving theory set against the ledger. Do not reuse a theory marked
`invalidated` unless new evidence explicitly reopens it.

If the user explicitly asked for analysis only, you may stop here once the supported theories and next decisive check are clear.

## Step 6A. Local Learning Memo

Before any design or implementation, write a concise memo that the next design must obey:

- already-supported root diagnosis
- exact contract signal that matters
- approaches already invalidated
- abstractions or mechanisms now exhausted
- local improvements that do not count as progress
- boundary or invariant that must change
- boundary trace result, including owner, data available, data missing, and focused invariant test shape
- owner alternatives checked and why the selected owner is the earliest viable owner
- cheapest verification that will prove the design worked

Then explicitly check the proposed design against the memo. If the design is another variant of an invalidated approach,
do not implement it.

## Step 7. Design From Supported Theories

Design only against supported or partially supported theories.

For each design element, state:

- which theory it addresses
- what responsibility owns the behavior
- boundary changes
- state or invariant changes
- observability changes
- tests that should own verification

Prefer structural fixes over threshold tuning, heuristic branching, or patching symptoms.

Run the abstraction necessity check before design: current caller, current outcome, existing owner, smallest direct or
deletion alternative, first end-to-end slice, and immediate replacement/deletion. A rollback design must reduce net
production surface and must not introduce a replacement framework, global lint system, migration program, or synthetic
extensibility proof.

Before selecting the mechanism, record a minimum-complexity decision: the required guarantee, plausible alternatives,
their residual risks and total lifecycle costs, the selected design, and the evidence that any apparently simpler
alternative cannot meet the contract. Do not choose by theoretical guarantee strength or by a fixed solution ladder.

For known-diagnosis cases, design directly around the violated invariant. Do not design another selector, ranker,
threshold, prompt wording change, or search expansion unless the `Local Learning Memo` explains why that mechanism is
not exhausted.

If the `Boundary Trace Gate` returned `patch decision: blocked`, design the ownership or data-flow correction needed to
make one owner capable of enforcing the invariant before changing behavior.

If the user explicitly asked for design only, you may stop after this section or after `Implementation Plan`, depending on how much detail was requested.

## Step 8. Design-Principles Compliance

Before finalizing the redesign:

1. Read `docs/design/design-principles.md` if it exists.
2. Obey repo-specific `AGENTS.md` instructions.
3. Explicitly check:
   - one clear owner per important behavior
   - honest boundaries
   - explicit state, lifecycle, and failure modes
   - operability and diagnosability
   - clean cuts that delete superseded behavior
   - no hidden dual paths or compatibility detours
   - no hardcoded domain or language heuristics where repo rules forbid them

If the best fix needs a deviation, name it explicitly and justify it. Otherwise redesign again.

## Step 8A. Design Obligation Gate

Before critiquing an implementation-bound plan, implementing a design, critiquing a claimed implementation as complete,
or starting expensive validation:

1. Extract each critical design obligation into a matrix.
2. Include these columns:
   - `Obligation id`
   - `Severity`
   - `Design source`
   - `Required behavior`
   - `Owner`
   - `Code proof`
   - `Test proof`
   - `Runtime proof`
   - `Status`
   - `Next action`
3. Use these statuses:
   - `DONE`: implemented, focused-test proven, and runtime-proven when runtime has been attempted
   - `READY_FOR_RUNTIME`: owner/code/test proof exists, but no runtime proof exists yet
   - `PARTIAL`: some code exists, but the obligation is not fully proven
   - `MISSING`: no owner or no real implementation exists
   - `CONTRADICTED`: runtime artifacts disprove the obligation
   - `BLOCKED`: a named blocker prevents proof or implementation
4. Before implementation, treat ownerless critical/high rows or rows without code/test targets as a stop gate.
5. Treat a missing matrix as a blocker when the work is design-, benchmark-, stop-loss-, or model-heavy.
6. Before expensive validation, treat critical/high `MISSING`, `PARTIAL`, `CONTRADICTED`, or `BLOCKED` rows as a stop
   gate.

When the user asks to critique a plan, start with the obligation matrix. A plan without critical/high owners and proof
paths is not ready for implementation, even when the prose design sounds coherent.

Proof rules:

- Component existence is not proof.
- Green tests are not proof unless they map to a named obligation.
- Negative tests are not enough for positive obligations.
- Unit-row tests do not prove relationship/edge/sequence behavior.
- Prompt text is not implementation proof unless the runtime contract gives the model the required inputs and structured
  output fields.
- Runtime proof must cite exact run ids and artifacts when they exist.

For repositories with `plans/no-more-fuckups.md` and design-obligation gate scripts, follow that protocol and run the
repository gate before expensive validation. Prefer the discovery wrapper when present:

```bash
scripts/quality/run-design-obligation-gate.sh --file <plan-or-ledger.md>
```

Otherwise run the assertion script directly:

```bash
scripts/quality/assert-design-obligation-gate.sh --file <plan-or-ledger.md>
```

Use the skill-local template at `templates/design-obligation-template.md` as the default portable shape for new
implementation-bound designs, then copy the filled matrix into the owning project plan or ledger.

If the script fails, do not run a benchmark or model-heavy canary. Update the matrix, ledger, and design first.

## Step 9. Detailed Implementation Plan

Plan in bounded steps:

1. observability changes
2. model or responsibility refactor
3. behavior changes
4. tests
5. cleanup of temporary instrumentation
6. validation runs

Each step should name the target files or components, the invariant being protected, and the verification for that step.

The implementation plan must map steps back to the obligation matrix. A step is not complete until its obligation row is
updated with code proof and test proof. Runtime proof is added only after inspecting the runtime artifact.

Before the first code edit, write a behavior-change `Cycle Contract` for the whole implementation attempt. It must name
the mechanism family, focused invariant test, debug or benchmark predicate, rollback trigger, and the exact condition
that will mark the attempt `contract-improved`, `falsified`, `no-progress`, or `invalid-run`.

## Step 10. Implement

Only now implement the design, and only when the user asked for implementation or the task clearly requires code changes.

While implementing:

- keep changes aligned to the approved plan
- avoid opportunistic cleanup outside the design
- preserve or improve observability
- update obligation rows as implementation steps become code- and test-proven
- validate incrementally
- remove temporary logging unless it was intentionally promoted to permanent diagnostics

Before any expensive validation, run the design-obligation gate for the relevant plan or ledger when the repository
provides one. If it fails, stop and report the blocking obligations instead of running.

After validation, classify the implementation attempt. If it is `no-progress`, preserve ledger evidence, revert behavior
changes unless instructed otherwise, and stop. Do not start another patch in the same mechanism family.

## Response Shape

Use full mode by default. Use short mode only when the frame and evidence matrix already exist in the current thread.

When this skill is invoked, use this order and omit only the sections that are intentionally not reached because the user requested `analysis-only` or `design-only`:

1. `Problems`
2. `Top Theories`
3. `Evidence Matrix`
4. `Logging / Instrumentation Needed`
5. `Cycle Contract`
6. `Single Run Plan` or `Run Results`
7. `Run Classification`
8. `Stop-Loss Decision`
9. `Investigation Redesign` when the evidence-cycle cap is hit
10. `Supported Theories`
11. `Boundary Trace` when a supported diagnosis points to a broken boundary or invariant
12. `Design`
13. `Design-Principles Compliance`
14. `Design Obligation Gate`
15. `Implementation Plan`
16. `Implementation Result`

When a known diagnosis is already supported, include `Known Diagnosis Decision` and `Local Learning Memo` before
`Design`, include `Boundary Trace` between them when the owner or invariant is not already explicit, and omit fresh
`Top Theories` unless a real contradiction requires them. The order is `Known Diagnosis Decision`, `Boundary Trace`,
`Local Learning Memo`, then `Design`.

In short mode, include only the sections that changed, but preserve the same names and order.

## Anti-Rabbit-Hole Checks

Stop and reframe if:

- the current run or patch cannot name a novel decision-relevant fact
- a required `Cycle Contract` is missing
- a prior run lacks a `Run Classification`
- the latest classification is `no-progress` and the next action is another local implementation tweak
- an expensive run produced `no-progress`, even once
- the current symptom matches a supported ledger diagnosis and there is no new contradiction
- the boundary trace has used one artifact trace and one static ownership pass without naming the owner and test shape
- the same mechanism has been tweaked twice without decisive new evidence
- two implementation attempts inside the same abstraction failed to improve the contract signal
- the selected mechanism is justified by being the strongest or most comprehensive rather than by having the lowest
  total lifecycle complexity that satisfies the contract
- two evidence cycles have passed without narrowing the top theory set
- a theory survives only because it keeps being reworded
- the proposed fix is broader than the evidence
- the proposed patch cannot name the owner, violated invariant, data available, data missing, and focused invariant test
- the logs cannot distinguish between the top two theories
- a design choice cannot be mapped back to supported evidence
- progress is being argued from answer plausibility, extra sources, or broader coverage instead of the explicit contract
- elapsed time, run count, or token/cost budget exceeded the stop budget
- more than 90 minutes have elapsed since the last `contract-improved` or `falsified` result in a benchmark/model-heavy
  investigation

## Resume Protocol

If the user interrupts mid-loop, resume by first stating:

- the current step
- the best-supported theory so far
- whether the diagnosis is already known
- the decisive missing evidence, if any
- exhausted mechanisms and do-not-retry guidance
- whether a boundary trace exists and whether it allows patching
- last `Cycle Contract`, last `Run Classification`, and whether it counted as progress
- stop budget, elapsed time, and whether a stop-loss trigger has fired
- the next single action

If a stop-loss trigger has fired, the next single action is to stop, report, preserve evidence, and revert failed
behavior changes when appropriate. Do not jump straight back into editing.
