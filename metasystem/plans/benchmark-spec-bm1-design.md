# Plan: BM-1, the First Benchmark Spec

- Owner: unclaimed (written 2026-08-05, single session, nothing built)
- Goal and current status: design the first benchmark spec, a task runner the metasystem builds from scratch unattended, substantial enough that building it successfully means something, and shaped so the result becomes the fixture for the two benchmarks after it. Status: design critique CLOSED (round 9); artifact critique CLOSED by join at round 5 (2026-08-05): 41 material findings across rounds BA-1 to BA-4, zero in round 5, all five rounds joined mechanically
- In flight right now: nothing
- Decisions made (and who made them): the user chose the three-benchmark structure and the task-runner subject on 2026-08-05; design decisions S-1 through S-7 taken here with recorded defaults
- Waiting on the human: nothing — the fence sizing was approved in writing (2026-08-05 blanket + the 2026-08-10 cap revision) and the series ran to completion
- Dead ends: a log summariser was drafted first and discarded: the subject was picked without asking, and it was too shallow to discriminate. A diff and patch toolkit was rejected because the core algorithm is a well-known exercise, so it would partly measure recall. An append-only store with compaction was rejected because its most interesting property, crash safety, cannot be graded in minutes
- Next step: the grader build is dispatched to Codex (item S-C, v0.1 scope per the manifest); Mission Zero's contract awaits the human's signature; fences for BM-1 still need human approval before any BM-1 run

This plan fills in item B-1 of `plans/metasystem-benchmark-design.md`, which fixed the shape of a spec but wrote neither requirements nor grader. It **does** amend that design in one place: the human replaced the log-summariser note with the task runner and the three-case structure on 2026-08-05, and the parent's BM-1 paragraph now points here as the authority. The earlier claim that this changed nothing was wrong, and two documents describing different first specs would have left an implementer with no way to know which governs.

## What this spec is for

Stated by the user: the spec exists so that once the metasystem has built it, we can judge how well the metasystem performed. It must be of substance — complex enough that building it successfully means something — and it is the first of several.

Discrimination is the design target. A task that one agent completes correctly in a single pass measures nothing, because every version of the metasystem scores the same on it. The spec earns its place only if a well-coordinated unattended run and a badly coordinated one produce visibly different results.

## The three benchmarks this artifact serves

The user's structure, which is why the subject choice matters more than it would for a single case:

1. **BM-1, build it.** The metasystem builds the task runner from this spec, starting from an almost empty repository.
2. **BM-2, change it without breaking it.** A frozen copy becomes the starting point, and the goal is a cross-cutting change: parallel execution of independent tasks. That cannot be done safely without understanding the scheduler, the cache, the failure handling and the reporting together.
3. **BM-3, fix a bug in it.** The same frozen copy with a seeded defect and a failing test that exposes it. Cache invalidation is the natural place to hide one, because a wrong cache key is invisible on a clean run and wrong on every rebuild.

Each benchmark is progressively closer to the work this metasystem will actually be asked to do.

## S-1: what gets built

A task runner. Given a file describing tasks, each with a command, declared inputs and outputs, and dependencies on other tasks, it works out the order, runs them, skips work whose inputs have not changed, and reports failures without running anything that depended on the failure. A small, honest `make`.

Why this subject discriminates:

| The metasystem claims | Where the spec applies pressure |
| --- | --- |
| Work splits across sub-agents without the pieces drifting | Text and JSON reports must carry identical numbers, so independently built parts must agree on one internal result contract |
| Structural decisions are made before building | Caching cannot be bolted on afterwards: what a cache key covers is decided when the executor is written, and retrofitting it means rewriting both |
| Delegates stop and report gaps rather than guessing | One requirement is deliberately under-determined, and correct handling is a recorded decision, not a silent choice |
| Claims are verified by running | Cache invalidation, cycle detection and failure propagation all look right on a reading and fail on a second run |
| Tests are not weakened to pass | The builder ships a map of which of its tests covers which requirement, and those tests are checked by whether they catch deliberate faults introduced into the builder's own code |

It is also deterministic, needs no network, and is not a memorised exercise the way a diff algorithm is, so it measures engineering rather than recall.

## S-2: the requirements

**The requirements live in `benchmark/specs/bm-1/spec.md` and only there.** This section used to carry a copy of them, and that copy was the single largest source of drift in this plan's history: eight critique rounds found nothing but statements of one rule disagreeing across places. A plan that restates its artifact will always eventually contradict it.

What the requirements are, in outline, so this plan reads on its own: a command-line task runner in Java 21 built with Maven, taking a `tasks.json` of commands with declared inputs, outputs and dependencies; twenty-five numbered requirements spanning the command line, configuration errors, execution and failure propagation, caching and invalidation, two output formats that must agree, and the non-functional budget. The artifact is authority for every detail.

## S-0: language and build (the human, 2026-08-05)

Java 21, Maven through the wrapper, command line. Spring Boot was asked for and withdrawn once it was clear the deliverable is a command-line tool: a web framework would have added ceremony without adding anything the benchmark measures.

One consequence needed handling before it could bite. Maven resolves dependencies over the network, and delegates run with network denied by the permissions envelope, so the first build of an unattended run would have failed for a reason having nothing to do with the agents. Provisioning warms a Maven repository in the target image and every build command uses `-o`. The no-third-party-dependency rule survives the move: main scope declares none, and test scope may use the JUnit already present.

The move strengthens the case rather than translating it. Parsing JSON with the standard library alone is real work in Java, the performance budget tests algorithmic growth rather than JVM startup, and a Maven project gives independently built parts more structural surface to disagree across.

## S-3: the seeded ambiguity

Requirement 10 demands a deterministic order among independent tasks and deliberately never says which order. Alphabetical, configuration order, and dependency depth are all defensible.

This is the right gap. A careless reader sees "deterministic" and assumes whatever their data structure does; a careful one notices no rule is given. Every defensible answer is genuinely acceptable, so the grader holds no secret right answer. And noticing is separable from choosing well: the first is machine-checkable, the second is a judgement.

Correct handling is a `DECISIONS.md` entry naming requirement 10 and the chosen rule, with an implementation that matches it. Shipping whatever ordering the language happens to produce fails even when the output looks right, because it is not a decision.

So that the check measures noticing rather than boilerplate, the decision must be declared in a closed vocabulary the grader can act on: the `DECISIONS.md` entry for requirement 10 carries a line `order-rule:` with exactly one of three values, each of which is a total order on its own:

- `alphabetical` — by task name.
- `config-order` — the order the tasks appear in the configuration file.
- `dependency-depth` — by longest path from a task with no dependencies, ties broken alphabetically.

The grader builds a configuration on which the three disagree and checks the implementation follows the rule it declared. There is deliberately no `other`: an open-ended value would reopen the boilerplate loophole this vocabulary exists to close, since any behaviour could then be declared after the fact and nothing could be checked against it. Three genuinely defensible options are enough to prove a decision was taken and honoured.

Whether the rule is a good choice stays with the judged layer. A checker that parsed free prose would be a hidden right answer in disguise.

## S-4: the grader

Held out by construction; `grader/` is never copied into the repository the agents work in. It emits `metric=<name>=<value>` lines in the gate grammar the mission contract already uses.

| Metric | What it measures |
| --- | --- |
| `acceptance` | Share of held-out tests passing across all twenty-one requirements |
| `requirement_coverage` | Share of requirements whose held-out tests all pass, derived from a per-requirement line the grader also emits, `metric=requirement_<n>=1` or `0`. The per-requirement lines exist because the fixture filter below asks whether any requirement scored zero, and a filter cannot rest on a number nothing publishes |
| `cache_correctness` | A battery: unchanged inputs skip, changed command reruns, changed input reruns, `--force` reruns everything, deleted cache directory returns to cold |
| `failure_propagation` | Dependents of a failure are `blocked`, unrelated branches still run, exit code is 1 |
| `determinism` | Identical configuration and identical starting cache state produce identical order and identical per-task states. Each repetition begins from a deleted cache directory, because a correct cache legitimately reports `ran` the first time and `cached` the next, and comparing those two would fail every correct implementation |
| `config_errors` | Missing dependency, cycle, and duplicate output are each detected with the required detail and exit 2 |
| `format_agreement` | Text and JSON report identical order, counts, and per-task states, each invoked from an identical cold cache state so the two runs are comparable at all |
| `build_clean` | The produced repository runs from a fresh clone with no manual steps |
| `own_tests_pass` | The builder's own suite passes under its stated command |
| `mutation_catch` | Share of pinned requirements whose mapped tests detect one of the mutants targeting that requirement. Mutants declare a target rather than isolating one, and requirement 10 is excluded, so the denominator is the pinned requirements only |
| `plan_seconds` | Requirement 20, measured on a generated 1000-task configuration |
| `dependency_count` | Non-standard-library imports in the shipped code |
| `gap_handled` | A decision record names requirement 10 and repeated runs agree |

`cache_correctness` and `format_agreement` are reported separately rather than folded into `acceptance`, because they are the two most likely to separate a coordinated run from an uncoordinated one, and burying them in a pass rate would hide the signal the spec exists to produce.

**How mutation testing works here (SP-2-3, superseded 2026-08-05).** The transfer problem this paragraph used to solve no longer exists. Mutation testing operates on the builder's own code: their tests run against their own implementation with a fault introduced, so nothing has to cross an interface and no test architecture is favoured. A mutant counts as caught when the tests mapped to its target requirement fail against the mutant and pass against the unmutated code.

**`mutation_catch` replaces a check that did not work, and the replacement does not use the seed at all.** The first draft asked whether the builder's mapped tests failed on the seed and passed on the final tree. That proves nothing: the seed contains no `taskrun.py`, so every test fails there merely because the file is missing, and one always-green test per requirement would have scored full marks. Mutation catching asks the question that was actually meant. The kit ships mutants of the careful reference, each declaring the requirement it targets. It does not pretend a mutant breaks exactly one requirement: the requirements overlap by design (a broken cache key offends 13, 14, 15 and 16 at once), so demanding isolation would make the corpus impossible to build and S-C would fail for reasons having nothing to do with the metasystem. What is required is that every pinned requirement has at least one mutant targeting it — every requirement except 10, whose exclusion is explained below — and the metric asks whether the tests mapped to that requirement catch one of its targeted mutants.

Requirement 10 is deliberately excluded from the corpus. Mutating the reference's ordering would create exactly the hidden right answer this design keeps refusing: a builder that chose `config-order` writes a correct test asserting `config-order`, which fails against a reference that chose `alphabetical`, and would be scored for disagreeing rather than for being wrong. The ordering gap is measured by `gap_handled` alone, which compares each builder against its own declared rule. Building those mutants is real work and belongs to S-C, and it is the only version of this metric that cannot be satisfied by writing `assert True`.

## S-4a: the scoring contract

Every metric needs a domain and a direction before a manifest can be written, and the mission runner cannot classify a cycle without them. Thresholds that can be known in advance are fixed here; the rest are deliberately deferred to calibration rather than invented, and until they exist no run can be scored.

| Metric | Domain | Direction | Absolute bound now |
| --- | --- | --- | --- |
| `acceptance` | 0.0 to 1.0 | max | none until calibration |
| `requirement_coverage` | 0.0 to 1.0 | max | none until calibration |
| `cache_correctness` | 0.0 to 1.0 | max | none until calibration |
| `failure_propagation` | 0 or 1 | max | none until calibration |
| `determinism` | 0 or 1 | max | must be 1: a nondeterministic runner is not a runner |
| `config_errors` | 0.0 to 1.0 | max | none until calibration |
| `format_agreement` | 0 or 1 | max | none until calibration |
| `build_clean` | 0 or 1 | max | must be 1 |
| `own_tests_pass` | 0 or 1 | max | none until calibration |
| `mutation_catch` | 0.0 to 1.0 | max | none until calibration |
| `plan_seconds` | seconds, ≥ 0 | min | must be ≤ 5 |
| `dependency_count` | integer, ≥ 0 | min | must be 0 |
| `gap_handled` | 0 or 1 | max | none until calibration |

Noise floors cannot be invented either: agent runs vary, and how much is unknown until Benchmark Zero measures it. Until then this spec produces scores, not verdicts, exactly as the parent design requires.

## S-5: calibration, and why it comes first

Before any agent sees this, we build solutions by hand. Not two endpoints, which would only prove the grader can tell perfect from broken, but a small set spanning what real runs plausibly produce: one careful; one with a cache key that ignores the command; one whose order differs on every invocation with certainty, not probability: it keeps a counter beside its cache and reverses the reported order on alternate runs, so any configuration with two or more independent tasks orders differently each time. Two obvious choices are both wrong here and for the same reason. Iterating a `set` only *probably* varies, since hash randomisation might not change the order. An unseeded shuffle only probably varies too: a shuffle of three tasks repeats its order roughly one run in six. Either would let calibration see a deterministic run, accept the seeded defect, and report that a broken metric works. A flaw planted to test a metric must fail that metric on every repetition or it calibrates nothing; one that runs dependents after a failure; one correct but with tests that assert almost nothing; and one that is simply incomplete, because an unattended run stopped at its fences is the most likely outcome of all.

The grader must separate those in the ways this design predicts, and the spread between them is the first evidence that the spec discriminates at all. If the careful and the sloppy versions score within a hair of each other, the spec has failed at its only job and must change before anything is spent on running it.

Calibration also bounds the work, but it cannot size the run, and the earlier claim that it could was wrong (SP-3-9). A timed hand-build measures how long the task takes a person who already knows the answer; an unattended run spends its time on delegation, review, correction and rework, which is most of the cost and none of the hand-build. There is no way to derive the second from the first.

So the budget is set generously and treated as an experiment rather than a constraint we believe in, with a stated promotion rule so the transition is not left to be invented (SP-4-4):

- Trial runs happen under spec version `0.x`, this spec's own version rather than the shared measuring-kit version. Their scorecards are recorded and readable, and are **never** comparison-eligible: `benchmark-compare` refuses a cohort whose spec version is below 1.0.
- After the trials, each fence is `ceil(observed_max * 1.5)` where `observed_max` is the largest value that fence reached across all trial runs, in that fence's own unit: cycles, jobs, concurrent jobs, minutes, hours. Trials that parked at a fence are excluded from its own maximum, since a run stopped at eight cycles tells us it wanted at least eight, not that eight was enough; if every trial parked at a fence, that fence has no evidence yet and more trials are needed rather than a guessed number.
- Those values are written into the manifest, the spec is published as version `1.0`, and from that point it is immutable: changing a fence later means version `1.1` and a fresh baseline, exactly like any other kit change.

A trial run that stops at its fences is a measurement, not a failure. This is the same prove-first discipline the rest of the metasystem now follows, and the alternative is a number invented today that quietly decides every future verdict.

**What calibration must show before anything is spent (SP-2-7).** Three conditions, each checkable:

1. **Every seeded flaw is caught by the metric designed to catch it, on every repetition.** The command-blind cache key drops `cache_correctness`; the alternating-order scheduler drops `determinism`; the run-dependents-anyway variant drops `failure_propagation`; the assert-nothing variant drops `mutation_catch` while keeping `own_tests_pass`. Each is checked over the same repetition count a real cohort uses, and a flaw caught on some repetitions and not others has not been caught: the metric is measuring luck.
2. **Each probe's must-not-disturb metrics stay undisturbed.** The manifest's calibration rule is the authority: a probe declares its target and the metrics it must not touch, and calibration checks both directions. There is deliberately no aggregate score, because a single number would let a strong metric mask a broken one.
3. **The incomplete probe's `acceptance` sits strictly between zero and one**, visibly partial rather than at the floor, since a run stopped at its fences is the likeliest real outcome and a spec that cannot tell partial work from broken work will read every unfinished run as a failure.

Calibration proves each metric fires; it no longer sets bounds. The midway rule it used to state compared against the careful reference's score, and that reference no longer exists, so the rule silently lost its left operand (BA-3-6). Bounds are set once, at promotion to 1.0, from trial-run evidence plus probe floors, per S-6's promotion rule. Until then the spec produces scores, not verdicts.

The careful reference also settles the fixture question, though the previous round settled it wrongly. Saying the fixture is *always* the reference bought stability by abandoning the human's actual structure, which was that the software the metasystem builds becomes the thing later benchmarks change and debug (SP-2-8).

The real requirement is not that the fixture be hand-built; it is that it never changes once anything has been scored against it. So: **the fixture is chosen once and frozen forever.** After BM-1's first runs, we take either the best agent-built artifact or the careful reference, whichever is sound enough to build on, freeze it as `taskrunner-v1`, and never swap it. A later, better artifact becomes `taskrunner-v2` alongside a new spec version, never a quiet replacement of v1. The human's intent is preserved, and comparability is preserved, because the thing that breaks comparability is swapping after scoring, not choosing an agent-built artifact in the first place.

Choosing between candidates is the human's decision, taken once and recorded, not an agent's judgement (SP-3-8). Two of the criteria are mechanical and act as a filter: the candidate passes the full held-out grader, and no requirement scores zero. The third, whether the code is a fair basis for the BM-2 change, is a judgement no metric captures, and pretending otherwise would let different operators freeze materially different codebases and quietly change how hard the next two benchmarks are. The kit records which candidate was frozen, its scores, and who chose it. If no candidate passes the filter, the reference is the fixture and the record says so.

## S-6: fences, and why they exceed Mission Zero's

Mission Zero's fences (5 cycles, 5 jobs, one hour) were sized for a one-line fix to prove the loop turns over. Twenty-one requirements with a caching layer will not fit, and shrinking the spec to fit would destroy the substance the user asked for.

Proposed, as a complete vector, because a partial one cannot be sealed: `fence.cycles=8`, `fence.jobs=12`, `fence.concurrency=2`, `fence.job-cap-min=15`, `fence.wall-clock-hours=3`, `host.turn-cap-min=20`, `ledger.cycle-budget=8`, `ledger.no-gain-budget=3`.

The last three matter as much as the first five and were missing: without a host turn cap a single stuck orchestrator turn consumes the wall clock, and without the ledger budgets the mission has no stop-loss, so a run that ships receipts while the gate never moves would burn the whole envelope before anything noticed.

The roster must be pinned by exact model identifier, not by the phrase "cheap models", because the comparability tuple in the parent design requires that two cohorts share an identical roster and a job's requested model must equal what it actually ran. A word like "cheap" is not a roster identity and two runs a month apart would silently differ. The exact identifiers are filled in when the manifest is written, from whatever the project's tier 1 declares at that moment, and recorded in the scorecard.

A run that cannot finish inside that envelope is itself a finding, and the fences remain the only real spending control. This is the one item here that costs real money, and no run happens until the human confirms it.

## S-7: where the artifact lives

**The case itself lives at `benchmark/specs/bm-1/`**, once S-B writes it: `spec.md` carrying the requirements of S-2, `manifest.json` carrying the thresholds, fences and roster, `seed/`, and the held-out `grader/` with its mutant corpus. This plan is the design *for* that directory and is scaffolding: when S-B and S-C ship, the durable content lives in the spec directory and this file is deleted, per `plans/README.md`. Nothing in `plans/` is ever the artifact.

Sibling cases sit beside it as `benchmark/specs/bm-2/` and `bm-3/`, each a self-contained directory with its own version. Adding one bumps no shared version and invalidates no existing baseline, which is what makes accumulating cases possible; that property was missing until 2026-08-05 and is now stated in the parent design.

Frozen as `benchmark/fixtures/taskrunner-v1/` inside the kit. The kit must be self-contained with no network fetches, and a spec cited by any scorecard is immutable, so a separate repository would break both. BM-2 and BM-3 each take their own copy into their `seed/` when those specs are written, so nothing shifts underneath a benchmark that has already been scored.

`seed/` for BM-1 itself is a git repository containing only `README.md`, `spec.md`, and an empty `tests/`. It is empty because BM-1 builds from scratch, and for no other reason: the earlier justification, that a mapped test necessarily fails on an empty seed, was the reasoning behind the differential check that round 1 removed as vacuous. Test quality is measured against the mutant corpus, which does not involve the seed at all.

## Open questions for the critique

1. Does this discriminate? If two metasystem versions of genuinely different quality ran it, which metrics separate them and which look identical either way?
2. Is twenty-one requirements the right size for three hours with cheap models, or does it guarantee every run scores badly for reasons unrelated to metasystem quality?
3. Is the ordering rule the best available seeded gap, or does a better one separate noticing from guessing?
4. Does anything here have a right answer the grader secretly holds, which would punish a defensible choice?
5. Is `format_agreement` trivially satisfiable by generating one format from the other? That is a legitimate implementation and must not be penalised.
6. Does the artifact actually support BM-2 and BM-3, or does building it to this spec produce something with no room for a cross-cutting change and nowhere good to hide a bug?

## Items

| Id | Item | Status |
| --- | --- | --- |
| S-A | Critique this design with Codex against the stated purpose | NOT STARTED, next |
| S-B | Write `spec.md`, `manifest.json`, and `seed/` | NOT STARTED |
| S-C | Build the grading baseline: the acceptance suite from the spec, a mutation metasystem that operates on the builder's own code, small per-metric calibration probes, the check scripts emitting the gate grammar, and the calibration proving each metric fires | NOT STARTED, before any agent run. Nothing correct is hand-built here: the human refused that premise on 2026-08-05 and was right, since mutating what the builder wrote asks the question directly, and writing a working solution is an expensive way to test a checker |

All three remain blocked behind the canonical gates and this plan does not reorder them (SP-2-10): S-A is design work and runs now, but S-B and S-C are item B-1 of the parent design, which B-0 blocks, which in turn waits on Mission Zero. Designing the spec early is deliberate; building it before the runner has been watched running is the mistake this whole sequence exists to avoid.

RETIRED: differential seed-versus-final check -- mutation_catch against the reference mutant corpus

RETIRED: order-rule other -- the closed vocabulary alphabetical, config-order, dependency-depth

RETIRED: reference implementation -- mutation testing on the builder's own code, and per-metric calibration probes

RETIRED: logsum -- the taskrun case in benchmark/specs/bm-1/

## Critique Ledger

One chain, job `design-critic-20260805t093318z-b195`, codex/gpt-5.6-sol, all rounds 2026-08-05. Material findings by round — round 1: 11, round 2: 10, round 3: 9, round 4: 5, round 5: 3, round 6: 2, round 7: 1, round 8: 1, round 9: 0 — 43 in total, every one accepted. Closed by join at round 9 with zero material findings.

The shape of the loop is worth recording. Rounds 1 to 4 attacked the design and changed it substantially: the vacuous test-quality check, the unobservable ordering decision, the impossible mutant corpus, the fixture rule that abandoned the human's three-case structure, and the determinism check that would have failed every correct implementation. Rounds 5 to 9 found only one species of defect, a rule correctly changed in one place and left stale in another statement of the same rule, eight times. That is drift between statements, not a defect in the design, and the remedy is a mechanical consistency check rather than further rounds; it is being built rather than recommended.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SP-1-1 | accepted | The design does not define an executable scoring contract: every grader metric lacks its numeric domain, direction, absolute floor or ceiling, noise f… | folded into the design at the section it names |
| SP-1-2 | accepted | Requirements 17-18 provide no public text grammar or JSON schema, making the held-out `format_agreement` grader impossible to implement without a secr… | folded into the design at the section it names |
| SP-1-3 | accepted | The proposed differential-coverage check is vacuous: because mapped tests are absent from the seed, their failure there proves only that the test arti… | folded into the design at the section it names |
| SP-1-4 | accepted | Requirement 10 is not the sole seeded under-determination; the specification contains multiple accidental choice points, and the blanket `DECISIONS.md… | folded into the design at the section it names |
| SP-1-5 | accepted | A fully conforming implementation can report a task as cached after its declared output has been deleted or corrupted, so the specified artifact is no… | folded into the design at the section it names |
| SP-1-6 | accepted | The seeded-gap grader does not check the recorded decision against the implementation, so it measures the presence of boilerplate rather than whether … | folded into the design at the section it names |
| SP-1-7 | accepted | The plan leaves two binding benchmark authorities in direct conflict while claiming to change nothing, so an implementer cannot know which BM-1 size, … | folded into the design at the section it names |
| SP-1-8 | accepted | The proposed run cannot be sealed or cost-bounded from this design because its fence vector is both unapproved and incomplete, and “cheap delegate mod… | folded into the design at the section it names |
| SP-1-9 | accepted | The design does not ensure that BM-1’s produced artifact serves BM-2 and BM-3; it substitutes a hand-built reference by default and leaves an undefine… | folded into the design at the section it names |
| SP-1-10 | accepted | The proposed BM-2 change conflicts with BM-1’s deterministic-order contract because the design never distinguishes deterministic launch order, complet… | folded into the design at the section it names |
| SP-1-11 | accepted | S-5’s calibration obligation is too weak to establish discrimination or budget fit: two hand-authored endpoints merely scoring differently does not sh… | folded into the design at the section it names |
| SP-1-12 | accepted | Generating the text report from the JSON report, or both from one result object, is not itself a defect and should not be penalized.… | folded into the design at the section it names |
| SP-2-1 | accepted | The mutation corpus has an impossible construction requirement: several numbered requirements overlap logically, so a mutant cannot break exactly one … | folded into the design at the section it names |
| SP-2-2 | accepted | The seeded ordering decision and determinism metric are not representable in the pinned output contracts, so the grader has no specification-defined t… | folded into the design at the section it names |
| SP-2-3 | accepted | `mutation_catch` has no executable test-transfer protocol and can reward irrelevant failures, secretly favoring one test architecture over another.… | folded into the design at the section it names |
| SP-2-4 | accepted | The `other` ordering value reopens the exact boilerplate loophole the closed vocabulary was meant to close, and `dependency-depth` does not itself def… | folded into the design at the section it names |
| SP-2-5 | accepted | The amended child and canonical parent still prescribe mutually exclusive own-test graders, leaving an implementer to build the removed differential c… | folded into the design at the section it names |
| SP-2-6 | accepted | The accepted finding that requirement 10 was not the only ambiguity has not landed: the design still sends several public behaviors to `DECISIONS.md` … | folded into the design at the section it names |
| SP-2-7 | accepted | Calibration still has no acceptance contract, so the manifest writer must invent both the score separation that proves discrimination and most absolut… | folded into the design at the section it names |
| SP-2-8 | accepted | The fixed-reference amendment resolves fixture stability by abandoning the stated three-benchmark purpose: BM-1's produced software never becomes the … | folded into the design at the section it names |
| SP-2-9 | accepted | S-6 still does not define the complete sealable execution envelope it claims to define, leaving host time and mission stop-loss behavior unconstrained… | folded into the design at the section it names |
| SP-2-10 | accepted | The target's implementation sequence bypasses the canonical B-0 gate, so following this plan would start B-1 before its settled prerequisite has passe… | folded into the design at the section it names |
| SP-3-1 | accepted | Calibration is guaranteed to reject the proposed dictionary-order variant for the wrong reason: on the mandated Python runtime, dictionary/configurati… | folded into the design at the section it names |
| SP-3-2 | accepted | Adding `order` to JSON fixed only half of the reporting contract: the text result still has no required task sequence or per-task states, so requireme… | folded into the design at the section it names |
| SP-3-3 | accepted | The determinism and format-agreement procedures can fail every correct cache implementation because repeated CLI invocations do not begin from identic… | folded into the design at the section it names |
| SP-3-4 | accepted | Mutation catching introduces a hidden right answer for the deliberate ordering gap: a valid builder test that asserts its chosen rule can receive no c… | folded into the design at the section it names |
| SP-3-5 | accepted | The mutant transfer protocol still lacks the mapping and individual-test execution interface needed to implement it.… | folded into the design at the section it names |
| SP-3-6 | accepted | The round-2 corrections were not joined across the binding documents, leaving contradictory grader and fixture contracts at HEAD.… | folded into the design at the section it names |
| SP-3-7 | accepted | The new calibration conditions are not checkable as written and still do not determine the deferred thresholds.… | folded into the design at the section it names |
| SP-3-8 | accepted | Fixture selection is still an unowned multi-objective judgment, so different operators can freeze different codebases and materially change BM-2 and B… | folded into the design at the section it names |
| SP-3-9 | accepted | A timed hand-build does not validate the proposed unattended mission budget, so the design still has no evidence that three hours separates metasystem qu… | folded into the design at the section it names |
| SP-4-1 | accepted | The replacement nondeterminism variant and its grader are probabilistic, so calibration can intermittently accept the seeded defect or treat it as ful… | folded into the design at the section it names |
| SP-4-2 | accepted | The mandatory calibration conditions still describe the removed dictionary variant and an undefined aggregate score, so S-C has no coherent completion… | folded into the design at the section it names |
| SP-4-3 | accepted | The mutation contract is still unjoined across its authoritative surfaces, changing both the corpus denominator and whether the grader implements muta… | folded into the design at the section it names |
| SP-4-4 | accepted | The new experimental-fence policy has no promotion rule from trial evidence to the immutable comparison spec, so an implementer must invent both the f… | folded into the design at the section it names |
| SP-4-5 | accepted | The fixture's mechanical filter references a per-requirement score that the grader does not define or emit, so fixture eligibility remains implementat… | folded into the design at the section it names |
| SP-5-1 | accepted | SP-4-1 did not land: replacing set iteration with an unseeded shuffle leaves the seeded determinism flaw probabilistic, despite requiring it to fail o… | folded into the design at the section it names |
| SP-5-2 | accepted | SP-4-3 only landed in the parent; the BM-1 design still gives mutually exclusive mutation-corpus contracts.… | folded into the design at the section it names |
| SP-5-3 | accepted | SP-4-4 leaves the fence-promotion formula ambiguous, so it does not yet determine the immutable 1.0 fence vector.… | folded into the design at the section it names |
| SP-6-1 | accepted | SP-5-1 did not fully land: the operative calibration condition still directs S-C to build or assess the rejected probabilistic shuffling variant inste… | folded into the design at the section it names |
| SP-6-2 | accepted | SP-5-2 did not fully land: only the metric row was corrected, while the same section and seed contract still retain the mutually exclusive all-require… | folded into the design at the section it names |
| SP-7-1 | accepted | SP-6-2 still did not land document-wide: the corrected metric and seed paragraph coexist with remaining instructions to use the empty seed and cover e… | folded into the design at the section it names |
| SP-8-1 | accepted | SP-7-1 remains only partially answered: the parent-ledger bookkeeping is now correctly annotated, but two current BM-1 design statements still prescri… | folded into the design at the section it names |

## Completion

Done when the critique closes and S-B and S-C are shipped, at which point item B-1 of the benchmark design is complete and the first graded run becomes possible.
