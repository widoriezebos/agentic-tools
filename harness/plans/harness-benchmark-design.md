# Plan: The Harness Benchmark

- Owner: unclaimed (design written 2026-08-04, single session, nothing built)
- Goal and current status: a benchmark where a coordinator and sub-agents build software to a fixed spec, producing two things we grade: the software itself and the logged behavior of the agents that built it. The scorecard from both becomes the harness's own fitness function, so agents can later evolve the harness inside constraints expressed in software. Status: under critique; rounds 1 and 2 (14 material findings each) adjudicated and folded in
- In flight right now: critique round 3 on the amended design
- Decisions made (and who made them): the requirement and its intent stated by the user 2026-08-04, who also directed this design and its critique to run now, ahead of Mission Zero; design decisions D-B1 through D-B6 taken here with recorded defaults, overridable by the human
- Waiting on the human: nothing blocking; D-B1 through D-B6 stand unless overridden
- Dead ends: none yet
- Next step: critique loop to agreement; then the design waits for Mission Zero and its reconciliation pass (B-0) before anything is built

This plan layers on `plans/agent-orchestration-design.md`. It amends the settled design in exactly one named place: the standing benchmark authorization (D-B5), an extension to mission approval that goes through the change gate when B-4 builds it. Everything else reuses mission mode (Part 6 there) as the chassis: a benchmark run is a mission, and the contract grammar, runner-side measurement, fences, ledger, and evidence trail are reused rather than reinvented. The user directed the design and critique to run now; implementation stays blocked behind Mission Zero (CC-3 in `plans/prove-first-course-correction.md`), and B-0 below makes the design answer to Mission Zero's evidence before any item builds on it (BM-1-11).

## Intent

Today the harness's quality is argued: critique rounds, obligation matrices, a validation suite that proves scripts behave. None of that proves the harness makes agents build better software. The benchmark makes that measurable. A fixed spec goes in; graded software and judged behavior come out; one scorecard records both. Two consequences follow:

1. **The harness gets a fitness function.** "Did this change help" becomes a comparison of scorecards instead of an opinion.
2. **The harness can evolve itself safely.** The functional and non-functional constraints of the harness live in the benchmark kit as executable gates, alongside the validation suite that stays the gate of record. An agent proposing a harness change proves it against both. The kit is the executable statement of intent, so evolution toward the intent is enforced by software, and "going overboard or producing something not intended" fails the gates rather than needing a human to notice.

The proportionality ruling applies here unchanged: the benchmark measures cooperative, fallible agents. Its guards exist to catch mistakes and accidental metric-gaming, not adversaries. Where a guard cannot be mechanical at reasonable cost, the design says so and routes the verdict to the human instead of pretending.

## What a benchmark run is

One mission, run by the mission runner against a scratch repository, under a pinned harness.

- **The candidate** is the harness at one commit: its scripts, prompts, roles, skills, and docs at a named sha. That sha is what the run scores. Because scorecards are themselves committed (D-B4), the identity rule excludes them: the candidate sha is the newest commit touching anything outside `benchmark/results/`, and a results-only commit is identity-neutral and never needs a scorecard of its own (BM-1-9).
- **The kit** is the measuring stick: specs, graders, rubrics, the scorecard extractor, the compare thresholds, pinned copies of the evidence schemas it reads, and the instrument-path list (below). The kit lives at `benchmark/` in the template repository only, excluded from the adoption payload like `meta/`, because it measures the harness rather than serving adopted projects.
- **The target** is a fresh scratch repository provisioned from the spec's seed, with the candidate harness adopted into it. Nothing real is at risk; each run gets a fresh target.
- **The run** is a mission instantiated from a kit-owned contract template under a standing authorization (D-B5): the coordinator (the host orchestrator) dispatches sub-agents through the roster to build what the spec says, under the spec's lifecycle fences, until the mission completes or parks. All of 6.2's machinery applies: lease, host turns from the shipped prompt artifacts, runner-side measurement, ledger in the stop-loss grammar, the four end states.
- **Delegation is the thing under test, so it is measured at the product, not just counted (BM-2-5).** Run validity requires at least one implementer job dispatched, completed, and certified per stream (the floor), and the scorecard carries the **delegated share**: the fraction of the seed-to-final product change introduced by commits made in implementer job workspaces, computed from the target's git history joined to the job records, with a floor in the spec manifest. A coordinator that dispatches one trivial job and builds the rest itself fails the share floor mechanically; whether the delegation was also sensible stays a judged dimension. B-0 verifies that commit provenance survives the harness's merge flow, which Mission Zero will show.
- **The record** is the run evidence set, enumerated below. Grading and judging read only this set.

**Identity and comparability (BM-2-2, BM-2-9, BM-2-10).** Every scorecard pins: benchmark spec id and version, kit version, candidate sha, cohort id and repetition index (BM-2-8), the full roster (runtime and model per dispatchable role from the target's `harness.conf`, plus the host runtime and model from the contract's `host.*` keys), the fences, the repetition count, the machine fingerprint (OS, CPU model, core count), and the measuring-harness sha (D-B6). The **comparability tuple** is all of that except the candidate sha, cohort id, and repetition index: two cohorts are comparable when spec version, kit version, roster, fences, repetition count, and machine fingerprint are all equal, and the candidate sha is the variable under test. Machine fingerprint sits inside the tuple, so a cross-machine comparison is no-verdict by construction and a faster machine can never masquerade as a better harness (BM-2-9). Benchmark missions forbid ad hoc dispatch overrides: run validity requires every job's requested runtime and model to equal the pinned roster's resolution for its role, and its effective model to equal the requested one, so the pinned roster is actual rather than declared (BM-2-10). B-8's roster-swap runs are deliberately different tuples: their output is a descriptive report, never an evolution verdict.

## The run evidence set

Everything we grade or judge must come from a named artifact the run already writes. The set: the mission contract, state, and ledger; the turn prompts and orchestrator returns under `missions/<id>/turns/`; the job records, transcripts, and round directories under `artifacts/agents/`; the census reports; the watcher output; the scratch repository's git history; and the grader's own output. A metric with no source in this set is a logging gap first and a metric second: the fix is to log the fact, never to estimate it.

**The trust model for behavior evidence is stated, not papered over (BM-2-1).** Job records, census reports, and the ledger are emitted by candidate-owned scripts; they are self-reported evidence, and no schema check can make them attestations: a schema-valid record set can still under-emit. The benchmark handles this with three layers, each honest about what it covers:

- **Drift detection**: the kit pins copies of the evidence schemas and grammars it reads; an evidence set that fails to parse under them makes the run invalid. This catches format drift, nothing more.
- **Floors, not credit**: every metric carries `sourceOwner: kit` (grader output, the benchmark runner's own wall clock, git history the runner reads itself, scorecard completeness) or `sourceOwner: candidate` (everything derived from job records, census, ledger, watcher output). Candidate-sourced metrics act as floors and validity checks; improvements in them are credited only under a human-ratified verdict (see verdict grades below).
- **The instrument tripwire**: the kit names the candidate's evidence-emitting surfaces (the dispatcher's record writer, the census, the ledger writer, the adapters' event emission) as its **instrument-path list**. When a candidate diff touches any listed path, `benchmark-compare` never auto-accepts: the verdict is downgraded to human-ratified with the touched paths named. Changing how evidence is emitted is legal; having a machine credit the resulting scores without a human looking is not.

## The two verdicts

### Product: grading the software

The grader is the spec's held-out acceptance machinery, run by the benchmark runner after the mission ends, from the kit against the produced tree. It emits `metric=<name>=<value>` lines in the shipped gate grammar (6.2b) and its thresholds live in the spec's manifest as a threshold vector, the acceptance-gate-as-data shape Part 5 already adopted from the source repositories.

Measured per spec, with floors in the manifest:

- **Acceptance**: pass rate over the held-out tests, each in the input, expected outcome, machine-checkable verification shape.
- **Requirement coverage**: the spec's numbered requirements each map to one or more acceptance tests; coverage is the share of requirements whose tests pass.
- **Build from clean**: the produced repository builds and its checks pass from a fresh clone, exactly as a stranger would receive it.
- **The product's own tests (BM-2-13)**: the spec requires the builder to ship a mapping manifest naming, per requirement id, the tests that cover it. The grader checks the manifest is complete and the named tests exist and pass — and then runs the differential check that gives the metric teeth: for each requirement, at least one mapped test must fail on the spec's seed and pass on the final tree. A trivial always-green test passes on both and therefore covers nothing. Whether the mapped tests exercise their requirements more deeply than that is a judged dimension.
- **Non-functional budgets**: whatever the spec declares (runtime on a given input, dependency count, artifact size), measured by the grader.
- **Gap handling (BM-1-10, BM-2-11)**: each spec seeds one deliberate under-determination, marked in the grader but not in the spec. The authority chain is explicit so this never contradicts the repository's ask-first and no-judgment-calls rules: the benchmark contract template carries a pre-authorized envelope for spec-interpretation decisions, bounded to "recorded in the spec-declared decision file", so deciding is in-contract for the coordinator; implementer briefs still leave no judgment calls, so the correct path is implementer stops and reports the gap, coordinator decides under the envelope, records the decision naming the requirement id, and re-briefs. The grader checks mechanically: the decision record exists, and the implementation is consistent with it. Specs are designed so consistency is machine-checkable without a hidden allowlist (the seeded gap is a choice among behaviors the grader can verify for internal consistency, such as an unspecified ordering that must merely be total and stable, not a choice the grader must approve of). Silent behavior fails the gap metric even when acceptance happens to pass; parking on the seeded gap is a misclassification of an envelope-covered decision, visible in fence economy and the judged layer.

Held-out means held out by construction (D-B1): provisioning copies only the spec and seed into the target; the grader never enters it. Not a security measure; a visible acceptance test would replace the spec as the thing being built against, and reading the spec is part of what we are measuring.

### Behavior: judging the execution

Two layers, deliberately separate.

**Mechanical, computed by the extractor from the evidence set, no judgment involved** (`sourceOwner: candidate` unless noted):

| Metric | Source |
| --- | --- |
| Protocol conformance: share of returns passing `assert-return-complete.sh` without a `protocol_error` retry | job records |
| Tracking: UNTRACKED census findings during the run | census reports |
| Critique discipline: every critique chain closed by join | `assert-critique-closed.sh` over the run's chains |
| Delegation floor and delegated share, per stream | job records joined to git history (share is kit-read from git) |
| Fence economy: cycles, jobs, wall clock, and per-job time used against the fences | state, job records; wall clock also measured kit-side by the benchmark runner |
| Rework: follow-up rounds per job | job records |
| Progress shape: ledger classification counts (`contract-improved`, `unresolved`, `no-progress`) and the longest no-progress streak | the ledger |
| Cost: typed usage per provider per role, never summed across providers (R9) | job records |
| Commit shape: commit count and size distribution in the target | scratch git history (kit-read) |

Re-dispatch against the same obligation was cut from this table (BM-1-12): no record or brief field carries a stable obligation id, so the metric would rest on prose similarity, which the no-estimation rule forbids. Repeated-work detection belongs to the judged layer.

**Judged, by a `behavior-judge` role reading the evidence set against kit rubrics (D-B6, BM-2-14).** The judge is an ordinary harness role — preamble, requirements file, and return schema at `scripts/agents/roles/` and `scripts/agents/schemas/` like every role — so stock dispatch machinery applies unchanged. What the kit owns is the measuring text: the rubrics at `benchmark/rubrics/`, which travel inside the kit-authored judge brief, one rubric per dimension, so the kit changes what is judged without touching the harness. The judge is dispatched by the benchmark runner from the template repository, on the measuring side, never inside the candidate's target: judging is measurement, and the measuring-harness sha on the scorecard records what performed it. Dimensions: brief quality (did briefs leave judgment calls), adjudication quality (did dispositions actually answer findings), delegation discipline (was the delegated share honest work or ceremony), gap handling beyond the seeded probe, spec fidelity beyond what tests catch, repeated work across jobs, proportionality (was effort sized to the task, or did the loop go overboard), and evidence honesty (do returns' claims match their transcripts). The judge returns per-dimension scores with findings, each anchored to a file and line in the evidence set, schema-validated like every role return, permissions `none`. Per D-B2 the judge runs on a different runtime than the coordinator when the roster has two live runtimes; when it cannot, the scorecard records self-judging. **Judged scores never gate and are never auto-accept targets (BM-1-8)**: they generate findings for retros and trends, and where the judged and mechanical layers overlap their agreement is recorded as the judge's own reliability watch.

## The scorecard

One canonical JSON per run, schema versioned, at `benchmark/results/<candidate-sha>/<cohort-id>/<run-id>.json`, committed (D-B4); a derived Markdown projection sits beside it for humans, JSON canonical as everywhere in the protocol. Raw run evidence stays under gitignored `artifacts/` and is mirrored to the durable evidence root before it counts as disposable, per the standing rule. Field names carry their units (`wallClockSeconds`, not `duration`; the KI-1 lesson applied from birth).

Blocks: identity (the full pinned set above, including cohort id and repetition index), run validity, product metrics with their thresholds and verdicts, mechanical behavior metrics each with `sourceOwner`, judged scores with finding anchors, judge identity and agreement, cost per provider, machine fingerprint, and watches.

Three classes of measurement, because they answer different questions:

- **Run-validity gates.** Every job terminal, every chain closed, zero UNTRACKED, fences enforced (nothing ran past a fence without the runner stopping it), the delegation floor met, every job's requested runtime and model equal to the roster's resolution and effective equal to requested, the evidence set complete and parsing under the kit-pinned schemas. A run failing these is not a low score; it is a harness defect and the run's most valuable finding. The scorecard marks it invalid and says why.
- **Constraint metrics.** The floors evolution must never regress: the product threshold vector, protocol conformance, rework ceiling, cost ceiling per provider, delegated share. Each carries a declared direction (`min` or `max`), an absolute floor or ceiling in the kit or spec manifest, and a noise floor. These are the "confines of the harness" made executable.
- **Watches.** Informative trends, never gates and never auto-accept targets: validation-suite wall time (KI-2), harness line count, assembled prompt sizes, census scan duration (KI-4), commit shape. Watches inform the human; only gates block.

## The evolution loop

How a harness change gets accepted on evidence:

0. **The gate of record still gates (BM-1-2), with an evidence interface (BM-2-12).** The candidate sha has the validation suite green in CI. `benchmark-compare` takes this as a committed attestation file produced by a small kit script that queries the operator's authenticated `gh` for the candidate sha's workflow conclusion and records run URL, sha, conclusion, and retrieval time; compare verifies the sha matches and the conclusion is green, embeds the attestation in its verdict, and performs no network calls itself. No attestation, no verdict.
1. **Pre-registration.** The change proposal names its target metric before any run, with a declared direction. Kit-sourced scalars (product metrics, kit-measured wall clock, cost) are auto-accept-eligible targets. Candidate-sourced mechanical metrics may be named as targets for behavior-directed changes — the benchmark exists partly to improve behavior, and BM-2-6 is right that shutting that door contradicts the intent — but such a comparison can only ever end in a human-ratified verdict, never an auto-accept. Watches and judged scores are never targets.
2. **Baseline.** A valid baseline cohort exists for the baseline sha at the same comparability tuple, one per live spec in the kit. After any kit version bump, old baselines are obsolete by construction, and the first act under the new kit is re-running the baseline cohort on the unchanged harness (BM-2-7); a kit change that alters evidence-schema pins states in its commit which harness emission change it prepares for, and the compatibility is explicit: the new kit accepts the old form until the harness change lands, so both cohorts under the new kit version parse.
3. **Candidate cohort.** The changed harness runs every live spec at the same tuple and repetition count. A cohort is one benchmark-runner invocation producing N repetitions under one cohort id (BM-2-8); compare consumes exactly one named cohort per side. An invalid run inside a consumed cohort is reject — a defect, not a data point. A cohort abandoned for an environmental failure stays committed and visible, and re-running a fresh cohort is legitimate and leaves the discarded one on the record; compare names the cohorts it used.
4. **Verdict arithmetic (BM-1-5, BM-2-3).** Per metric, the compared value is the median across the cohort's repetitions. Accept requires, on every live spec, all three of: every constraint metric on the candidate meets its **absolute** floor or ceiling; no constraint metric's candidate median is worse than the baseline median by more than its kit-recorded noise floor; and the target metric's candidate median is better than baseline by more than its noise floor. A metric missing on either side is no-verdict, which goes to the human. Only scalars gate; distributions are watches.
5. **Verdict grades (BM-2-1, BM-2-6).** `accepted` (auto): all conditions met, kit-sourced target, candidate diff touches no instrument path. `human-ratified` required instead when the target is candidate-sourced or the diff touches the instrument-path list: compare reports everything it measured plus the tripwire hits, and the human's recorded ratification completes the verdict. `rejected` and `no-verdict` as above.
6. **The verdict states its scope (BM-1-4).** A verdict names the spec set it rests on; while the kit holds one spec, every acceptance is recorded as evidence on that spec, not as general improvement. The gate widens automatically as specs land, because step 3 runs every live spec.

**The measuring-stick rule:** a change set touches the kit or the harness, never both. Kit changes (specs, graders, rubrics, thresholds, the extractor, the compare, the pinned schema copies, the instrument-path list) are human-approved commits and bump the kit version, which breaks comparability on purpose. And a harness change set may not reference benchmark spec content: `benchmark-compare` greps the candidate diff for the kit's spec identifiers and distinctive spec strings and flags hits for the human (BM-1-4) — a cheap tripwire for accidental spec-specific tuning, not a proof. An agent evolving the harness cannot also move the goalposts in the same change, not because it is malicious but because optimizing against a stick you are also holding bends the stick without anyone deciding that.

**Variance is measured before it is claimed.** Agent runs are stochastic and the noise is unknown until Benchmark Zero measures it (D-B3: three repetitions of the first spec on the pinned harness). Until then no comparison verdict is issued; after it, the per-metric observed range across repetitions becomes the recorded noise floor in the kit, maintained like the frontier's noise floors and widened when later runs show larger spread.

**Authorization for unattended runs is signed once, ahead, and bounded (D-B5, BM-2-4).** The mission machinery requires the human to seal and sign each concrete contract, and benchmark repetitions would otherwise need a signature per run. The standing benchmark authorization is a human-signed kit file binding: the contract template's hash, the spec version, the fence vector, the exposure bound per run and per cohort, the roster, and an expiry (a date or a run count). The benchmark runner instantiates and seals each concrete contract from the template, and its approval line cites the standing authorization; `assert-mission.sh` accepts such a contract when the instantiated bytes match the authorized template modulo the seal-generated block and the candidate sha, and the authorization is unexpired. This is the one amendment to the settled mission design this plan makes, it widens nothing (the human still signs, earlier and with explicit bounds, the same envelope idea the contract already uses for reserved decisions), and it lands through the change gate as part of B-4.

**Self-evolution closes the recursion with no new machinery beyond that:** a harness-evolution mission is an ordinary mission whose contract sets `gate.command` to `benchmark-compare` and whose `gate.paths` are the kit. The mission machinery already freezes instruments, measures runner-side, and refuses to certify what the gate rejects; a `human-ratified` compare outcome surfaces as a reserved ask, parking the stream until the human answers, exactly like every reserved decision. Instruction-ledger discipline still applies on top: rule changes surfacing from benchmark evidence go through the retro and the human veto, unchanged.

## What we want to learn, beyond the scores

The scorecard is the verdict; these are the lessons the same evidence yields, read at retros:

- **Proportionality per mechanism** (the CC-5 question, now answerable): which harness mechanisms fired during real runs — follow-ups, parks, reaps, fence trips, census catches — and which never did. Dead weight shows up as a mechanism that no run ever exercised.
- **Prompt gaps**: judged deviations traceable to a missing or ambiguous shipped instruction become instruction-ledger candidates with the run as evidence.
- **Roster deltas**: the same spec under a swapped roster (Codex coordinating, Claude implementing; later Devin) measures the runtime-neutrality claim instead of asserting it — as a descriptive report across tuples, never an evolution verdict.
- **Fence sizing**: measured fence economy across runs replaces guessed fence values.
- **Failure taxonomy**: park reasons and stall shapes accumulate into a named list of how unattended runs actually die, feeding `known-issues.md`.

## The specs

A spec is a versioned directory under `benchmark/specs/<id>/`: `spec.md` (numbered functional requirements, non-functional requirements with explicit budgets, the required decision-record and test-mapping shapes, and the seeded under-determination, written so a careful reader notices the gap), `manifest.json` (id, version, threshold vector with per-metric direction and absolute floors, delegated-share floor, fences sized to the spec, seed description; the morpheus case shape), `seed/` (the target's starting content), and `grader/` (held out). A spec cited by any scorecard is immutable; changes create a new version. Requirements for any spec: self-contained, no network beyond the runtimes themselves, gradeable in minutes, both functional and non-functional requirements present, sized to finish inside its fences, runtime budgets generous enough that machine variance stays inside them, and a seeded gap whose record-consistency is machine-checkable.

**BM-1, the first spec**, stays deliberately small: a command-line text-processing tool of roughly fifteen requirements (candidate subject decided at B-1: a log summarizer with defined input format, output contract, edge cases, a runtime budget, and a no-dependencies constraint), one seeded under-determination, fences in the Mission Zero size class. The ladder beyond BM-1 is names only until BM-1 has produced scorecards: BM-2 change-under-tests (modify seeded working code without breaking its suite), BM-3 bugfix-in-unfamiliar-code. Designing those now would repeat the design-outran-proof mistake; until BM-2 lands, every evolution verdict says "on BM-1" and means it.

## Decisions taken here (overridable)

- **D-B1, grader visibility**: held out by construction; the target never contains the grader. Rationale under Product above.
- **D-B2, judge identity**: cross-runtime when two live runtimes exist; self-judging allowed and recorded otherwise. Forbidding self-judging would block the benchmark on single-runtime machines for a bias we can see and discount — and the discount is structural, because judged scores never gate (BM-1-8).
- **D-B3, Benchmark Zero**: three repetitions of BM-1 on the pinned harness to measure variance; no compare verdicts before it. Three is the smallest count that shows spread; the cost stays bounded by BM-1's fences and cheap delegate models.
- **D-B4, where results live**: scorecards committed in the template repository; raw evidence gitignored and mirrored to the evidence root. Scorecards are small, diffable, and are the record comparisons read; raws are bulky evidence like every other run's. Results-only commits are identity-neutral per the candidate-sha rule.
- **D-B5, standing benchmark authorization (BM-2-4)**: the human signs the contract template, spec version, fences, roster, per-run and per-cohort exposure bounds, and expiry once; the runner instantiates, seals, and cites. The alternative, a signature per repetition, makes unattended cohorts impossible and adds ceremony without adding judgment.
- **D-B6, judge execution boundary (BM-2-14)**: the judge is a stock harness role dispatched on the measuring side by the benchmark runner; the kit owns the rubrics, which travel in the judge's brief. The alternative, injecting kit role files into the candidate's paths, would have the candidate under test supply the judge's machinery.

## Items

Sequenced strictly after CC-3 (Mission Zero). B-0 is the hinge: nothing else starts until it closes.

| Id | Item | Status |
| --- | --- | --- |
| B-0 | Reconcile this design against Mission Zero's actual runner and evidence contract: a dated pass over every claim this design makes about mission machinery — commit provenance through the merge flow (the delegated-share join), the evidence set's real shape, the contract-instantiation seam — amendments recorded here, re-critiqued if material (BM-1-11) | NOT STARTED, blocks all below |
| B-1 | Spec format, BM-1 spec, seed, decision-record and test-mapping shapes, and held-out grader emitting the gate grammar, including the differential seed-versus-final test check | NOT STARTED |
| B-2 | Scorecard schema (cohort id, repetition index, sourceOwner, measuring-harness sha included) and the extractor, with kit-pinned evidence schema copies; validated against a fixture corpus (fake-adapter runs plus hand-authored evidence sets covering each metric's positive and failure case, BM-1-13); Mission Zero's evidence is the first real smoke, not the validation | NOT STARTED |
| B-3 | The `behavior-judge` stock role (preamble with byte-checked quote blocks, requirements, return schema) plus the kit rubrics that travel in its brief | NOT STARTED |
| B-4 | The benchmark runner: provision target from seed, adopt the candidate harness, instantiate and seal the contract under the standing authorization (the D-B5 change-gate item), run the mission, grade, extract, judge, emit the scorecard under a cohort id | NOT STARTED |
| B-5 | `benchmark-compare`: CI attestation intake, run-validity gates, absolute floors plus relative regression plus target arithmetic, cohort selection, verdict grades with the instrument-path tripwire and spec-reference tripwire, no-verdict path | NOT STARTED |
| B-6 | Benchmark Zero: three scored BM-1 runs on the pinned harness under one cohort id, variance recorded, noise floors written into the kit, machine-variance check on runtime budgets, findings to the retro | NOT STARTED |
| B-7 | The evolution gate: the measuring-stick rule, target-eligibility and verdict-grade rules land in the change-gate documentation, plus the harness-evolution mission contract template with `benchmark-compare` as its gate | NOT STARTED |
| B-8 | First roster-swap run: same spec, coordinator and implementer runtimes exchanged, reported descriptively as the runtime-neutrality measurement | NOT STARTED, after B-6 |

Process per the standing regime: this design goes to Codex critique in the loop until agreement; implementation by Codex per item, code critiqued by the main agent until agreement, one commit per item with fixtures where an item ships a script.

## Critique Ledger

- **Round 1** (job `design-critic-20260804t102631z-67c9`, codex/gpt-5.6-sol, 2026-08-04): 14 findings, all material, all accepted except BM-1-11's timing claim (user directed the timing; its amendment, B-0, accepted). Dispositions recorded in the round 1 entry of this ledger as committed at 14a4618; the fixes are integrated throughout.
- **Round 2** (same chain, round 2, 2026-08-04): 14 findings, all material, all accepted. BM-2-1 trust model stated, instrument-path tripwire and verdict grades added; BM-2-2 candidate sha removed from the comparability tuple; BM-2-3 absolute floors joined to the verdict arithmetic; BM-2-4 standing authorization (D-B5); BM-2-5 delegated-share floor from git history; BM-2-6 candidate-sourced targets allowed under human-ratified verdicts; BM-2-7 kit-bump baseline re-run and explicit compatibility rule; BM-2-8 cohort ids and repetition indexes, compare consumes named cohorts; BM-2-9 machine fingerprint moved into the comparability tuple; BM-2-10 roster-resolution equality as validity, B-8 demoted to descriptive report; BM-2-11 gap-probe authority chain via the contract envelope, implementer path unchanged, consistency-checkable gaps required; BM-2-12 CI attestation interface; BM-2-13 differential seed-versus-final test check; BM-2-14 judge as stock role dispatched measuring-side (D-B6).

## Completion

The design is done when the critique loop closes AND B-0's reconciliation against Mission Zero has passed. The plan is done when B-0 through B-7 are shipped and Benchmark Zero has produced three valid scorecards with recorded variance; then this file is deleted, the kit and its documentation being the durable owners, and B-8 onward continue as ordinary backlog.
