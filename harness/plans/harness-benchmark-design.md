# Plan: The Harness Benchmark

- Owner: unclaimed (design written 2026-08-04, single session, nothing built)
- Goal and current status: a benchmark where a coordinator and sub-agents build software to a fixed spec, producing two things we grade: the software itself and the logged behavior of the agents that built it. The scorecard from both becomes the harness's own fitness function, so agents can later evolve the harness inside constraints expressed in software. Status: under critique; round 1 (14 material findings) adjudicated and folded in
- In flight right now: critique round 2 on the amended design
- Decisions made (and who made them): the requirement and its intent stated by the user 2026-08-04, who also directed this design and its critique to run now, ahead of Mission Zero; design decisions D-B1 through D-B4 taken here with recorded defaults, overridable by the human
- Waiting on the human: nothing blocking; D-B1 through D-B4 stand unless overridden
- Dead ends: none yet
- Next step: critique loop to agreement; then the design waits for Mission Zero and its reconciliation pass (B-0) before anything is built

This plan layers on `plans/agent-orchestration-design.md` and amends nothing in it. Mission mode (Part 6 there) is its prerequisite and its chassis: a benchmark run is a mission, and everything the mission machinery already provides (contract grammar, runner-side measurement, fences, ledger, evidence trail) is reused rather than reinvented. The user directed the design and critique to run now; implementation stays blocked behind Mission Zero (CC-3 in `plans/prove-first-course-correction.md`), and B-0 below makes the design answer to Mission Zero's evidence before any item builds on it (BM-1-11).

## Intent

Today the harness's quality is argued: critique rounds, obligation matrices, a validation suite that proves scripts behave. None of that proves the harness makes agents build better software. The benchmark makes that measurable. A fixed spec goes in; graded software and judged behavior come out; one scorecard records both. Two consequences follow:

1. **The harness gets a fitness function.** "Did this change help" becomes a comparison of scorecards instead of an opinion.
2. **The harness can evolve itself safely.** The functional and non-functional constraints of the harness live in the benchmark kit as executable gates, alongside the validation suite that stays the gate of record. An agent proposing a harness change proves it against both. The kit is the executable statement of intent, so evolution toward the intent is enforced by software, and "going overboard or producing something not intended" fails the gates rather than needing a human to notice.

The proportionality ruling applies here unchanged: the benchmark measures cooperative, fallible agents. Its guards exist to catch mistakes and accidental metric-gaming, not adversaries.

## What a benchmark run is

One mission, run by the mission runner against a scratch repository, under a pinned harness.

- **The candidate** is the harness at one commit: its scripts, prompts, roles, skills, and docs at a named sha. That sha is what the run scores. Because scorecards are themselves committed (D-B4), the identity rule excludes them: the candidate sha is the newest commit touching anything outside `benchmark/results/`, and a results-only commit is identity-neutral and never needs a scorecard of its own (BM-1-9).
- **The kit** is the measuring stick: specs, graders, rubrics, the scorecard extractor, the compare thresholds, and pinned copies of the evidence schemas it reads. The kit lives at `benchmark/` in the template repository only, excluded from the adoption payload like `meta/`, because it measures the harness rather than serving adopted projects.
- **The target** is a fresh scratch repository provisioned from the spec's seed, with the candidate harness adopted into it. Nothing real is at risk; each run gets a fresh target.
- **The run** is a mission from a kit-owned contract template: the coordinator (the host orchestrator) dispatches sub-agents through the roster to build what the spec says, under the spec's lifecycle fences, until the mission completes or parks. All of 6.2's machinery applies verbatim: lease, host turns from the shipped prompt artifacts, runner-side measurement, ledger in the stop-loss grammar, the four end states. Delegation is the thing under test, so it is not optional: run validity requires at least one implementer job dispatched, completed, and certified per stream (BM-1-3), and whether the coordinator built around its delegates instead of through them is a judged dimension.
- **The record** is the run evidence set, enumerated below. Grading and judging read only this set.

Identity is pinned on every scorecard and comparability requires equality of all of it (BM-1-6): benchmark spec id and version, kit version, candidate sha, the full roster (runtime and model per dispatchable role from the target's `harness.conf`, plus the host runtime and model from the contract's `host.*` keys), the fences, and the repetition count. Each job's effective model must equal its requested model or the run is invalid, so the pinned roster is actual rather than declared. The machine fingerprint (OS, CPU model, core count) is recorded as a watch; product runtime budgets are sized generously enough at spec design that machine variance stays inside them, checked at Benchmark Zero.

## The run evidence set

Everything we grade or judge must come from a named artifact the run already writes. The set: the mission contract, state, and ledger; the turn prompts and orchestrator returns under `missions/<id>/turns/`; the job records, transcripts, and round directories under `artifacts/agents/`; the census reports; the watcher output; the scratch repository's git history; and the grader's own output. A metric with no source in this set is a logging gap first and a metric second: the fix is to log the fact, never to estimate it.

**Source ownership is recorded per metric, and it decides what a metric may do (BM-1-1).** Most behavior evidence is emitted by candidate-owned scripts: job records, census reports, the ledger. A harness change can alter what gets logged, and an unchanged extractor would faithfully score the altered claims. The kit therefore does not pretend those metrics are independent measurements; it constrains what they can decide:

- Every scorecard metric carries `sourceOwner: kit` (grader output, the benchmark runner's own wall clock, scorecard completeness) or `sourceOwner: candidate` (everything derived from job records, census, ledger, watcher output).
- The kit pins its own copies of the evidence schemas and grammars it reads. Run validity includes the evidence set parsing under those kit-pinned copies, so a candidate that changes emission semantics produces an invalid run, never a better score.
- Candidate-sourced metrics act as floors and validity checks only. They can fail a run and they can block an evolution verdict, but they are never eligible as the improvement a change claims. The pre-registered target metric must be kit-sourced.

## The two verdicts

### Product: grading the software

The grader is the spec's held-out acceptance machinery, run by the benchmark runner after the mission ends, from the kit against the produced tree. It emits `metric=<name>=<value>` lines in the shipped gate grammar (6.2b) and its thresholds live in the spec's manifest as a threshold vector, the acceptance-gate-as-data shape Part 5 already adopted from the source repositories.

Measured per spec, with floors in the manifest:

- **Acceptance**: pass rate over the held-out tests, each in the input, expected outcome, machine-checkable verification shape.
- **Requirement coverage**: the spec's numbered requirements each map to one or more acceptance tests; coverage is the share of requirements whose tests pass.
- **Build from clean**: the produced repository builds and its checks pass from a fresh clone, exactly as a stranger would receive it.
- **The product's own tests**: the spec requires the builder to ship a mapping manifest naming, per requirement id, the tests that cover it; the grader checks the manifest is complete, the named tests exist, and they pass (BM-1-14). Whether the mapped tests genuinely exercise their requirements is a judged dimension, not a grader claim.
- **Non-functional budgets**: whatever the spec declares (runtime on a given input, dependency count, artifact size), measured by the grader.
- **Gap handling (BM-1-10)**: each spec seeds one deliberate under-determination, marked in the grader but not in the spec. There is no hidden right answer: correct handling is an explicit decision record in the target (the spec names the required shape, a dated entry naming the requirement id and the chosen behavior) with the implementation consistent with that record. The grader accepts any defensible recorded choice and checks record-to-behavior consistency; silent behavior fails the gap metric even when the guess happens to pass acceptance. The seeded gap is deliberately never reserved-class, so the standing gap rule (decide, record, proceed) applies and no ask or park is required; a run that parks on it has misclassified an ordinary judgment call, which shows up in fence economy and the judged layer.

Held-out means held out by construction (D-B1): provisioning copies only the spec and seed into the target; the grader never enters it. Not a security measure; a visible acceptance test would replace the spec as the thing being built against, and reading the spec is part of what we are measuring.

### Behavior: judging the execution

Two layers, deliberately separate.

**Mechanical, computed by the extractor from the evidence set, no judgment involved** (all `sourceOwner: candidate` unless noted):

| Metric | Source |
| --- | --- |
| Protocol conformance: share of returns passing `assert-return-complete.sh` without a `protocol_error` retry | job records |
| Tracking: UNTRACKED census findings during the run | census reports |
| Critique discipline: every critique chain closed by join | `assert-critique-closed.sh` over the run's chains |
| Delegation floor: implementer jobs dispatched, completed, and certified, per stream | job records, orchestrator returns |
| Fence economy: cycles, jobs, wall clock, and per-job time used against the fences | state, job records; wall clock also measured kit-side by the benchmark runner |
| Rework: follow-up rounds per job | job records |
| Progress shape: ledger classification counts (`contract-improved`, `unresolved`, `no-progress`) and the longest no-progress streak | the ledger |
| Cost: typed usage per provider per role, never summed across providers (R9) | job records |
| Commit shape: commit count and size distribution in the target | scratch git history |

Re-dispatch against the same obligation was cut from this table (BM-1-12): no record or brief field carries a stable obligation id, so the metric would rest on prose similarity, which the no-estimation rule forbids. Repeated-work detection belongs to the judged layer.

**Judged, by a `behavior-judge` role reading the evidence set against shipped rubrics.** The rubrics are kit artifacts at `benchmark/rubrics/`, one per dimension, quoted verbatim into the judge's preamble with the byte-checked quote-block machinery from 3.1a, because prompts are never left to the agent to invent. Dimensions: brief quality (did briefs leave judgment calls), adjudication quality (did dispositions actually answer findings), delegation discipline (did the coordinator work through its delegates or around them), gap handling beyond the seeded probe, spec fidelity beyond what tests catch, repeated work across jobs, proportionality (was effort sized to the task, or did the loop go overboard), and evidence honesty (do returns' claims match their transcripts). The judge returns per-dimension scores with findings, each finding anchored to a file and line in the evidence set, schema-validated like every role return. The judge is rostered and dispatched like any role, permissions `none`. Per D-B2 the judge runs on a different runtime than the coordinator when the roster has two live runtimes; when it cannot, the scorecard records self-judging. **Judged scores never gate and are never targets (BM-1-8)**: they exist to generate findings for retros and to trend, the scorecard records them alongside judge identity, and where the judged and mechanical layers overlap their agreement is recorded as the judge's own reliability watch.

## The scorecard

One canonical JSON per run, schema versioned, at `benchmark/results/<candidate-sha>/<run-id>.json`, committed (D-B4); a derived Markdown projection sits beside it for humans, JSON canonical as everywhere in the protocol. Raw run evidence stays under gitignored `artifacts/` and is mirrored to the durable evidence root before it counts as disposable, per the standing rule. Field names carry their units (`wallClockSeconds`, not `duration`; the KI-1 lesson applied from birth).

Blocks: identity (the full comparability tuple above), run validity, product metrics with their thresholds and verdicts, mechanical behavior metrics each with `sourceOwner`, judged scores with finding anchors, judge identity and agreement, cost per provider, machine fingerprint, and watches.

Three classes of measurement, because they answer different questions:

- **Run-validity gates.** Every job terminal, every chain closed, zero UNTRACKED, fences enforced (nothing ran past a fence without the runner stopping it), the delegation floor met, every job's effective model equal to its requested model, the evidence set complete and parsing under the kit-pinned schemas. A run failing these is not a low score; it is a harness defect and the run's most valuable finding. The scorecard marks it invalid and says why.
- **Constraint metrics.** The floors evolution must never regress: the product threshold vector, protocol conformance, rework ceiling, cost ceiling per provider. Each carries a declared direction (`min` or `max`) and a noise floor in the kit. These are the "confines of the harness" made executable.
- **Watches.** Informative trends, never gates and never targets: validation-suite wall time (KI-2), harness line count, assembled prompt sizes, census scan duration (KI-4), commit shape, machine fingerprint. Watches inform the human; only gates block.

## The evolution loop

How a harness change gets accepted on evidence:

0. **The gate of record still gates (BM-1-2).** The candidate sha has the validation suite green in CI, per the standing rule from the course correction. `benchmark-compare` refuses to issue any verdict for a candidate whose own suite is not green; the benchmark complements the gate of record and never substitutes for it.
1. **Pre-registration.** The change proposal names its target metric before any run: a kit-sourced scalar with a declared direction (a product metric or the kit-measured wall clock or cost floor). Watches, judged scores, and candidate-sourced metrics are not eligible (BM-1-7). A change that cannot name an eligible target is not an improvement claim and does not need the benchmark.
2. **Baseline.** Valid scorecards for the baseline sha exist at the same comparability tuple, one per live spec in the kit, at the same repetition count as the candidate will run.
3. **Candidate runs.** The changed harness runs every live spec at the same kit version and repetition count.
4. **Verdict by `benchmark-compare`, and the verdict arithmetic is fixed (BM-1-5).** Cohorts are all runs at the matching tuple; any invalid run on either side is reject (an invalid run is a defect, not a data point). Per metric, the compared value is the median across repetitions. Accept requires, on every live spec: no constraint metric's candidate median worse than the baseline median by more than that metric's kit-recorded noise floor, and the target metric's candidate median better than baseline by more than its noise floor. A metric missing on either side is no-verdict, which goes to the human. Only scalars gate; distributions are watches.
5. **The verdict states its scope (BM-1-4).** A verdict names the spec set it rests on; while the kit holds one spec, every acceptance is recorded as evidence on that spec, not as general improvement. The gate widens automatically as specs land, because step 3 runs every live spec.

**The measuring-stick rule, the one Goodhart guard this needs:** a change set touches the kit or the harness, never both. Kit changes (specs, graders, rubrics, thresholds, the extractor, the compare, the pinned schema copies) are human-approved commits and bump the kit version, which breaks comparability on purpose. And a harness change set may not reference benchmark spec content: `benchmark-compare` greps the candidate diff for the kit's spec identifiers and distinctive spec strings and flags hits for the human (BM-1-4) — a cheap tripwire for accidental spec-specific tuning, not a proof. An agent evolving the harness cannot also move the goalposts in the same change, not because it is malicious but because optimizing against a stick you are also holding bends the stick without anyone deciding that.

**Variance is measured before it is claimed.** Agent runs are stochastic and the noise is unknown until Benchmark Zero measures it (D-B3: three repetitions of the first spec on the pinned harness). Until then no comparison verdict is issued; after it, the per-metric observed range across repetitions becomes the recorded noise floor in the kit, maintained like the frontier's noise floors and widened when later runs show larger spread.

**Self-evolution closes the recursion with no new machinery:** a harness-evolution mission is an ordinary mission whose contract sets `gate.command` to `benchmark-compare` and whose `gate.paths` are the kit. The mission machinery already freezes instruments, measures runner-side, and refuses to certify what the gate rejects. Instruction-ledger discipline still applies on top: rule changes surfacing from benchmark evidence go through the retro and the human veto, unchanged.

## What we want to learn, beyond the scores

The scorecard is the verdict; these are the lessons the same evidence yields, read at retros:

- **Proportionality per mechanism** (the CC-5 question, now answerable): which harness mechanisms fired during real runs — follow-ups, parks, reaps, fence trips, census catches — and which never did. Dead weight shows up as a mechanism that no run ever exercised.
- **Prompt gaps**: judged deviations traceable to a missing or ambiguous shipped instruction become instruction-ledger candidates with the run as evidence.
- **Roster deltas**: the same spec under a swapped roster (Codex coordinating, Claude implementing; later Devin) measures the runtime-neutrality claim instead of asserting it.
- **Fence sizing**: measured fence economy across runs replaces guessed fence values.
- **Failure taxonomy**: park reasons and stall shapes accumulate into a named list of how unattended runs actually die, feeding `known-issues.md`.

## The specs

A spec is a versioned directory under `benchmark/specs/<id>/`: `spec.md` (numbered functional requirements, non-functional requirements with explicit budgets, the required decision-record and test-mapping shapes, and the seeded under-determination, written so a careful reader notices the gap), `manifest.json` (id, version, threshold vector with per-metric direction, fences sized to the spec, seed description; the morpheus case shape), `seed/` (the target's starting content), and `grader/` (held out). A spec cited by any scorecard is immutable; changes create a new version. Requirements for any spec: self-contained, no network beyond the runtimes themselves, gradeable in minutes, both functional and non-functional requirements present, sized to finish inside its fences, runtime budgets generous enough that machine variance stays inside them.

**BM-1, the first spec**, stays deliberately small: a command-line text-processing tool of roughly fifteen requirements (candidate subject decided at B-1: a log summarizer with defined input format, output contract, edge cases, a runtime budget, and a no-dependencies constraint), one seeded under-determination, fences in the Mission Zero size class. The ladder beyond BM-1 is names only until BM-1 has produced scorecards: BM-2 change-under-tests (modify seeded working code without breaking its suite), BM-3 bugfix-in-unfamiliar-code. Designing those now would repeat the design-outran-proof mistake; until BM-2 lands, every evolution verdict says "on BM-1" and means it.

## Decisions taken here (overridable)

- **D-B1, grader visibility**: held out by construction; the target never contains the grader. Rationale under Product above.
- **D-B2, judge identity**: cross-runtime when two live runtimes exist; self-judging allowed and recorded otherwise. Forbidding self-judging would block the benchmark on single-runtime machines for a bias we can see and discount — and the discount is structural, because judged scores never gate (BM-1-8).
- **D-B3, Benchmark Zero**: three repetitions of BM-1 on the pinned harness to measure variance; no compare verdicts before it. Three is the smallest count that shows spread; the cost stays bounded by BM-1's fences and cheap delegate models.
- **D-B4, where results live**: scorecards committed in the template repository; raw evidence gitignored and mirrored to the evidence root. Scorecards are small, diffable, and are the record comparisons read; raws are bulky evidence like every other run's. Results-only commits are identity-neutral per the candidate-sha rule.

## Items

Sequenced strictly after CC-3 (Mission Zero). B-0 is the hinge: nothing else starts until it closes.

| Id | Item | Status |
| --- | --- | --- |
| B-0 | Reconcile this design against Mission Zero's actual runner and evidence contract: a dated pass over every claim this design makes about mission machinery, amendments recorded here, re-critiqued if material (BM-1-11) | NOT STARTED, blocks all below |
| B-1 | Spec format, BM-1 spec, seed, decision-record and test-mapping shapes, and held-out grader emitting the gate grammar | NOT STARTED |
| B-2 | Scorecard schema and the extractor for the mechanical metrics, with kit-pinned evidence schema copies; validated against a fixture corpus (fake-adapter runs plus hand-authored evidence sets covering each metric's positive and failure case, BM-1-13); Mission Zero's evidence is the first real smoke, not the validation | NOT STARTED |
| B-3 | Rubric artifacts, the `behavior-judge` role preamble with byte-checked quote blocks, and its return schema | NOT STARTED |
| B-4 | The benchmark runner: provision target from seed, adopt the candidate harness, run the mission from the kit-owned contract template, grade, extract, judge, emit the scorecard | NOT STARTED |
| B-5 | `benchmark-compare`: gate-of-record check, run-validity gates, constraint floors with directions, the fixed verdict arithmetic, noise rule, pre-registered target eligibility, spec-reference tripwire, no-verdict path | NOT STARTED |
| B-6 | Benchmark Zero: three scored BM-1 runs on the pinned harness, variance recorded, noise floors written into the kit, machine-variance check on runtime budgets, findings to the retro | NOT STARTED |
| B-7 | The evolution gate: the measuring-stick rule and target-eligibility rule land in the change-gate documentation, plus the harness-evolution mission contract template with `benchmark-compare` as its gate | NOT STARTED |
| B-8 | First roster-swap run: same spec, coordinator and implementer runtimes exchanged, scorecards compared as the runtime-neutrality measurement | NOT STARTED, after B-6 |

Process per the standing regime: this design goes to Codex critique in the loop until agreement; implementation by Codex per item, code critiqued by the main agent until agreement, one commit per item with fixtures where an item ships a script.

## Critique Ledger

- **Round 1** (job `design-critic-20260804t102631z-67c9`, codex/gpt-5.6-sol, 2026-08-04): 14 findings, all material. Dispositions: BM-1-1 accepted (source-ownership rule: kit-pinned schemas, candidate-sourced metrics floor-only, targets kit-sourced). BM-1-2 accepted (gate of record joined to the evolution gate as step 0). BM-1-3 accepted (delegation floor as run validity plus judged dimension). BM-1-4 accepted (scoped verdicts, every-live-spec runs, spec-reference tripwire). BM-1-5 accepted (verdict arithmetic fixed: cohorts, medians, range-based noise, missing-metric no-verdict, scalars-only gating). BM-1-6 accepted (comparability tuple widened; effective-model equality as validity; machine fingerprint watch). BM-1-7 accepted (target eligibility rule). BM-1-8 accepted (judged scores never gate, never target). BM-1-9 accepted (results-only commits identity-neutral). BM-1-10 accepted (gap probe rewritten: recorded-decision-plus-consistency semantics, no hidden right answer, never reserved-class). BM-1-11 accepted in the amendment, timing partially rejected: the user directed design and critique to run now; B-0 makes the design answer to Mission Zero before anything builds. BM-1-12 accepted (metric cut to judged layer). BM-1-13 accepted (fixture corpus for B-2; Mission Zero demoted to smoke). BM-1-14 accepted (builder-shipped test mapping, graded for presence and passing; depth judged).

## Completion

The design is done when the critique loop closes AND B-0's reconciliation against Mission Zero has passed. The plan is done when B-0 through B-7 are shipped and Benchmark Zero has produced three valid scorecards with recorded variance; then this file is deleted, the kit and its documentation being the durable owners, and B-8 onward continue as ordinary backlog.
