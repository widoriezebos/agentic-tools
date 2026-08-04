# Plan: The Harness Benchmark

- Owner: unclaimed (design written 2026-08-04, single session, nothing built)
- Goal and current status: a benchmark where a coordinator and sub-agents build software to a fixed spec, producing two things we grade: the software itself and the logged behavior of the agents that built it. The scorecard from both becomes the harness's own fitness function, so agents can later evolve the harness inside constraints expressed in software. Status: designed, not yet critiqued
- In flight right now: nothing
- Decisions made (and who made them): the requirement and its intent stated by the user 2026-08-04; design decisions D-B1 through D-B4 taken here with recorded defaults, overridable by the human
- Waiting on the human: nothing blocking; D-B1 through D-B4 stand unless overridden
- Dead ends: none yet
- Next step: Codex design critique in the loop until agreement; implementation stays blocked behind Mission Zero (CC-3 in `plans/prove-first-course-correction.md`)

This plan layers on `plans/agent-orchestration-design.md` and amends nothing in it. Mission mode (Part 6 there) is its prerequisite and its chassis: a benchmark run is a mission, and everything the mission machinery already provides (contract grammar, runner-side measurement, fences, ledger, evidence trail) is reused rather than reinvented. Per the prove-first correction, no item below starts before Mission Zero has run end to end; this design exists now so critique can close while Mission Zero proceeds.

## Intent

Today the harness's quality is argued: critique rounds, obligation matrices, a validation suite that proves scripts behave. None of that proves the harness makes agents build better software. The benchmark makes that measurable. A fixed spec goes in; graded software and judged behavior come out; one scorecard records both. Two consequences follow:

1. **The harness gets a fitness function.** "Did this change help" becomes a comparison of scorecards instead of an opinion.
2. **The harness can evolve itself safely.** The functional and non-functional constraints of the harness live in the benchmark kit as executable gates. An agent proposing a harness change proves it against those gates. The kit is the executable statement of intent, so evolution toward the intent is enforced by software, and "going overboard or producing something not intended" fails the gates rather than needing a human to notice.

The proportionality ruling applies here unchanged: the benchmark measures cooperative, fallible agents. Its guards exist to catch mistakes and accidental metric-gaming, not adversaries.

## What a benchmark run is

One mission, run by the mission runner against a scratch repository, under a pinned harness.

- **The candidate** is the harness at one commit: its scripts, prompts, roles, skills, and docs at a named sha. That sha is what the run scores.
- **The kit** is the measuring stick: specs, graders, rubrics, the scorecard extractor, and the compare thresholds, at its own version. The kit lives at `benchmark/` in the template repository only, excluded from the adoption payload like `meta/`, because it measures the harness rather than serving adopted projects.
- **The target** is a fresh scratch repository provisioned from the spec's seed, with the candidate harness adopted into it. Nothing real is at risk; each run gets a fresh target.
- **The run** is a mission from a shipped contract template: the coordinator (the host orchestrator, dispatching sub-agents through the roster) builds what the spec says, under the spec's lifecycle fences, until the mission completes or parks. All of 6.2's machinery applies verbatim: lease, host turns from the shipped prompt artifacts, runner-side measurement, ledger in the stop-loss grammar, the four end states.
- **The record** is the run evidence set, enumerated below. Grading and judging read only this set.

Identity is pinned on every scorecard: benchmark spec id and version, kit version, harness sha, roster (runtime and model per role, from the target's `harness.conf`), fences, and repetition index. Two scorecards are comparable only when spec version and kit version are equal.

## The run evidence set

Everything we grade or judge must come from a named artifact the run already writes. The set: the mission contract, state, and ledger; the turn prompts and orchestrator returns under `missions/<id>/turns/`; the job records, transcripts, and round directories under `artifacts/agents/`; the census reports; the watcher output; the scratch repository's git history; and the grader's own output. A metric with no source in this set is a logging gap first and a metric second: the fix is to log the fact, never to estimate it.

## The two verdicts

### Product: grading the software

The grader is the spec's held-out acceptance machinery, run by the benchmark runner after the mission ends, from the kit against the produced tree. It emits `metric=<name>=<value>` lines in the shipped gate grammar (6.2b) and its thresholds live in the spec's manifest as a threshold vector, the acceptance-gate-as-data shape Part 5 already adopted from the source repositories.

Measured per spec, with floors in the manifest:

- **Acceptance**: pass rate over the held-out tests, each in the input, expected outcome, machine-checkable verification shape.
- **Requirement coverage**: the spec's numbered requirements each map to one or more acceptance tests; coverage is the share of requirements whose tests pass.
- **Build from clean**: the produced repository builds and its checks pass from a fresh clone, exactly as a stranger would receive it.
- **The product's own tests**: present, passing, and touching the requirements (measured as which requirements the product's suite exercises, from the grader's mapping, not as a coverage percentage ritual).
- **Non-functional budgets**: whatever the spec declares (runtime on a given input, dependency count, artifact size), measured by the grader.
- **Gap handling**: each spec carries one seeded under-specification with the correct behavior recorded only in the grader: the right response is an explicit recorded decision or an ask, never a silent fill. The grader checks which happened.

Held-out means held out by construction (D-B1): provisioning copies only the spec and seed into the target; the grader never enters it. Not a security measure; a visible acceptance test would replace the spec as the thing being built against, and reading the spec is part of what we are measuring.

### Behavior: judging the execution

Two layers, deliberately separate.

**Mechanical, computed by the extractor from the evidence set, no judgment involved:**

| Metric | Source |
| --- | --- |
| Protocol conformance: share of returns passing `assert-return-complete.sh` without a `protocol_error` retry | job records |
| Tracking: UNTRACKED census findings during the run | census reports |
| Critique discipline: every critique chain closed by join | `assert-critique-closed.sh` over the run's chains |
| Fence economy: cycles, jobs, wall clock, and per-job time used against the fences | state, job records |
| Rework: follow-up rounds per job; re-dispatches against the same obligation | job records, briefs |
| Progress shape: ledger classification counts (`contract-improved`, `unresolved`, `no-progress`) and the longest no-progress streak | the ledger |
| Cost: typed usage per provider per role, never summed across providers (R9) | job records |
| Commit shape: commit count and size distribution in the target | scratch git history |

**Judged, by a `behavior-judge` role reading the evidence set against shipped rubrics.** The rubrics are harness artifacts at `benchmark/rubrics/`, one per dimension, quoted verbatim into the judge's preamble with the byte-checked quote-block machinery from 3.1a, because prompts are never left to the agent to invent. Dimensions: brief quality (did briefs leave judgment calls), adjudication quality (did dispositions actually answer findings), gap handling beyond the seeded probe, spec fidelity beyond what tests catch, proportionality (was effort sized to the task, or did the loop go overboard), and evidence honesty (do returns' claims match their transcripts). The judge returns per-dimension scores with findings, each finding anchored to a file and line in the evidence set, schema-validated like every role return. The judge is rostered and dispatched like any role, permissions `none`. Per D-B2 the judge runs on a different runtime than the coordinator when the roster has two live runtimes; when it cannot, the scorecard records self-judging. Judge reliability is itself watched: where the judged and mechanical layers overlap, their agreement is recorded on the scorecard, never gated.

## The scorecard

One canonical JSON per run, schema versioned, at `benchmark/results/<harness-sha>/<run-id>.json`, committed (D-B4); a derived Markdown projection sits beside it for humans, JSON canonical as everywhere in the protocol. Raw run evidence stays under gitignored `artifacts/` and is mirrored to the durable evidence root before it counts as disposable, per the standing rule. Field names carry their units (`wallClockSeconds`, not `duration`; the KI-1 lesson applied from birth).

Blocks: identity (as pinned above), run validity, product metrics with their thresholds and verdicts, mechanical behavior metrics, judged scores with finding anchors, judge identity and agreement, cost per provider, and watches.

Three classes of measurement, because they answer different questions:

- **Run-validity gates.** Every job terminal, every chain closed, zero UNTRACKED, fences enforced (nothing ran past a fence without the runner stopping it), evidence set complete. A run failing these is not a low score; it is a harness defect and the run's most valuable finding. The scorecard marks it invalid and says why.
- **Constraint metrics.** The floors evolution must never regress: the product threshold vector, protocol conformance, rework ceiling, cost ceiling per provider. These are the "confines of the harness" made executable.
- **Watches.** Informative trends, never gates: validation-suite wall time (KI-2), harness line count, assembled prompt sizes, census scan duration (KI-4). Watches inform the human; only gates block.

## The evolution loop

How a harness change gets accepted on evidence:

1. **Pre-registration.** The change proposal names its target metric from the scorecard before any run. A change that cannot name one is not an improvement claim and does not need the benchmark.
2. **Baseline.** Scorecards for the current harness sha at the pinned kit version exist (or are produced).
3. **Candidate runs.** The changed harness runs the same spec at the same kit version, same repetition count.
4. **Verdict by `benchmark-compare`.** Accept when: all candidate runs valid; no constraint metric regressed beyond its recorded noise; the pre-registered target metric improved beyond noise. Anything else is reject or, when deltas sit inside noise, "no verdict", which goes to the human. The script never claims significance the repetition count cannot support.

**The measuring-stick rule, the one Goodhart guard this needs:** a change set touches the kit or the harness, never both. Kit changes (specs, graders, rubrics, thresholds, the extractor, the compare) are human-approved commits and bump the kit version, which breaks comparability on purpose. An agent evolving the harness cannot also move the goalposts in the same change, not because it is malicious but because optimizing against a stick you are also holding bends the stick without anyone deciding that.

**Variance is measured before it is claimed.** Agent runs are stochastic and the noise is unknown until Benchmark Zero measures it (D-B3: three repetitions of the first spec on the pinned harness). Until then no comparison verdict is issued; after it, the observed spread becomes the recorded noise floor per metric in the kit, maintained like the frontier's noise floors.

**Self-evolution closes the recursion with no new machinery:** a harness-evolution mission is an ordinary mission whose contract sets `gate.command` to `benchmark-compare` and whose `gate.paths` are the kit. The mission machinery already freezes instruments, measures runner-side, and refuses to certify what the gate rejects. Instruction-ledger discipline still applies on top: rule changes surfacing from benchmark evidence go through the retro and the human veto, unchanged.

## What we want to learn, beyond the scores

The scorecard is the verdict; these are the lessons the same evidence yields, read at retros:

- **Proportionality per mechanism** (the CC-5 question, now answerable): which harness mechanisms fired during real runs — follow-ups, parks, reaps, fence trips, census catches — and which never did. Dead weight shows up as a mechanism that no run ever exercised.
- **Prompt gaps**: judged deviations traceable to a missing or ambiguous shipped instruction become instruction-ledger candidates with the run as evidence.
- **Roster deltas**: the same spec under a swapped roster (Codex coordinating, Claude implementing; later Devin) measures the runtime-neutrality claim instead of asserting it.
- **Fence sizing**: measured fence economy across runs replaces guessed fence values.
- **Failure taxonomy**: park reasons and stall shapes accumulate into a named list of how unattended runs actually die, feeding `known-issues.md`.

## The specs

A spec is a versioned directory under `benchmark/specs/<id>/`: `spec.md` (numbered functional requirements, non-functional requirements with explicit budgets, and the seeded under-specification, written so a careful reader notices the gap), `manifest.json` (id, version, threshold vector, fences sized to the spec, seed description; the morpheus case shape), `seed/` (the target's starting content), and `grader/` (held out). A spec cited by any scorecard is immutable; changes create a new version. Requirements for any spec: self-contained, no network beyond the runtimes themselves, gradeable in minutes, both functional and non-functional requirements present, sized to finish inside its fences.

**BM-1, the first spec**, stays deliberately small: a command-line text-processing tool of roughly fifteen requirements (candidate subject decided at B-1: a log summarizer with defined input format, output contract, edge cases, a runtime budget, and a no-dependencies constraint), one seeded ambiguity, fences in the Mission Zero size class. The ladder beyond BM-1 is names only until BM-1 has produced scorecards: BM-2 change-under-tests (modify seeded working code without breaking its suite), BM-3 bugfix-in-unfamiliar-code. Designing those now would repeat the design-outran-proof mistake.

## Decisions taken here (overridable)

- **D-B1, grader visibility**: held out by construction; the target never contains the grader. Rationale under Product above.
- **D-B2, judge identity**: cross-runtime when two live runtimes exist; self-judging allowed and recorded otherwise. Forbidding self-judging would block the benchmark on single-runtime machines for a bias we can see and discount.
- **D-B3, Benchmark Zero**: three repetitions of BM-1 on the pinned harness to measure variance; no compare verdicts before it. Three is the smallest count that shows spread; the cost stays bounded by BM-1's fences and cheap delegate models.
- **D-B4, where results live**: scorecards committed in the template repository; raw evidence gitignored and mirrored to the evidence root. Scorecards are small, diffable, and are the record comparisons read; raws are bulky evidence like every other run's.

## Items

Sequenced strictly after CC-3 (Mission Zero); B-2 is first because Mission Zero's evidence is its test corpus.

| Id | Item | Status |
| --- | --- | --- |
| B-1 | Spec format, BM-1 spec, seed, and held-out grader emitting the gate grammar | NOT STARTED |
| B-2 | Scorecard schema and the extractor for the mechanical metrics, validated against Mission Zero's actual evidence set before any benchmark run | NOT STARTED |
| B-3 | Rubric artifacts, the `behavior-judge` role preamble with byte-checked quote blocks, and its return schema | NOT STARTED |
| B-4 | The benchmark runner: provision target from seed, adopt the candidate harness, run the mission from the shipped contract template, grade, extract, judge, emit the scorecard | NOT STARTED |
| B-5 | `benchmark-compare`: run-validity gates, constraint floors, noise rule, pre-registered target metric, no-verdict path | NOT STARTED |
| B-6 | Benchmark Zero: three scored BM-1 runs on the pinned harness, variance recorded, noise floors written into the kit, findings to the retro | NOT STARTED |
| B-7 | The evolution gate: the measuring-stick rule lands in the change-gate documentation, plus the harness-evolution mission contract template with `benchmark-compare` as its gate | NOT STARTED |
| B-8 | First roster-swap run: same spec, coordinator and implementer runtimes exchanged, scorecards compared as the runtime-neutrality measurement | NOT STARTED, after B-6 |

Process per the standing regime: this design goes to Codex critique in the loop until agreement; implementation by Codex per item, code critiqued by the main agent until agreement, one commit per item with fixtures where an item ships a script.

## Completion

The design is done when the critique loop closes. The plan is done when B-1 through B-7 are shipped and Benchmark Zero has produced three valid scorecards with recorded variance; then this file is deleted, the kit and its documentation being the durable owners, and B-8 onward continue as ordinary backlog.
