# Plan: BM-1, the First Benchmark Spec

- Owner: unclaimed (written 2026-08-05, single session, nothing built)
- Goal and current status: design the first benchmark spec, a task runner the harness builds from scratch unattended, substantial enough that building it successfully means something, and shaped so the result becomes the fixture for the two benchmarks after it. Status: critique round 1 adjudicated (11 material findings, all accepted), round 2 pending, nothing built
- In flight right now: nothing
- Decisions made (and who made them): the user chose the three-benchmark structure and the task-runner subject on 2026-08-05; design decisions S-1 through S-7 taken here with recorded defaults
- Waiting on the human: the fence sizing in S-6, which is larger than Mission Zero's and costs real money
- Dead ends: a log summariser was drafted first and discarded: the subject was picked without asking, and it was too shallow to discriminate. A diff and patch toolkit was rejected because the core algorithm is a well-known exercise, so it would partly measure recall. An append-only store with compaction was rejected because its most interesting property, crash safety, cannot be graded in minutes
- Next step: critique round 2 on the amended design, then build it as item B-1

This plan fills in item B-1 of `plans/harness-benchmark-design.md`, which fixed the shape of a spec but wrote neither requirements nor grader. It **does** amend that design in one place: the human replaced the log-summariser note with the task runner and the three-case structure on 2026-08-05, and the parent's BM-1 paragraph now points here as the authority. The earlier claim that this changed nothing was wrong, and two documents describing different first specs would have left an implementer with no way to know which governs.

## What this spec is for

Stated by the user: the spec exists so that once the harness has built it, we can judge how well the harness performed. It must be of substance — complex enough that building it successfully means something — and it is the first of several.

Discrimination is the design target. A task that one agent completes correctly in a single pass measures nothing, because every version of the harness scores the same on it. The spec earns its place only if a well-coordinated unattended run and a badly coordinated one produce visibly different results.

## The three benchmarks this artifact serves

The user's structure, which is why the subject choice matters more than it would for a single case:

1. **BM-1, build it.** The harness builds the task runner from this spec, starting from an almost empty repository.
2. **BM-2, change it without breaking it.** A frozen copy becomes the starting point, and the goal is a cross-cutting change: parallel execution of independent tasks. That cannot be done safely without understanding the scheduler, the cache, the failure handling and the reporting together.
3. **BM-3, fix a bug in it.** The same frozen copy with a seeded defect and a failing test that exposes it. Cache invalidation is the natural place to hide one, because a wrong cache key is invisible on a clean run and wrong on every rebuild.

Each benchmark is progressively closer to the work this harness will actually be asked to do.

## S-1: what gets built

A task runner. Given a file describing tasks, each with a command, declared inputs and outputs, and dependencies on other tasks, it works out the order, runs them, skips work whose inputs have not changed, and reports failures without running anything that depended on the failure. A small, honest `make`.

Why this subject discriminates:

| The harness claims | Where the spec applies pressure |
| --- | --- |
| Work splits across sub-agents without the pieces drifting | Text and JSON reports must carry identical numbers, so independently built parts must agree on one internal result contract |
| Structural decisions are made before building | Caching cannot be bolted on afterwards: what a cache key covers is decided when the executor is written, and retrofitting it means rewriting both |
| Delegates stop and report gaps rather than guessing | One requirement is deliberately under-determined, and correct handling is a recorded decision, not a silent choice |
| Claims are verified by running | Cache invalidation, cycle detection and failure propagation all look right on a reading and fail on a second run |
| Tests are not weakened to pass | The builder ships a map of which of its tests covers which requirement, checked against the empty starting state |

It is also deterministic, needs no network, and has no memorised reference implementation the way a diff algorithm does.

## S-2: the requirements

As they will appear in `spec.md`. Configuration is `tasks.json`:

```json
{
  "tasks": {
    "codegen": { "command": "python3 gen.py", "inputs": ["schema.txt"], "outputs": ["gen.py.out"] },
    "build":   { "command": "cat gen.py.out > app", "inputs": ["gen.py.out"], "outputs": ["app"], "deps": ["codegen"] }
  }
}
```

**Command line**

1. `python3 taskrun.py run [task...]` runs the named tasks and everything they depend on. With no task named, every task runs.
2. `--file <path>` selects the configuration, defaulting to `tasks.json`.
3. `--dry-run` prints the execution plan in the order tasks would run, and runs nothing.
4. `--force` runs every selected task regardless of cache state.
5. Exit codes are exactly: 0 when every selected task succeeded, 1 when any task failed, 2 for a usage or configuration error.

**Configuration errors**

6. A dependency naming a task that does not exist is a configuration error that names both the depending task and the missing name.
7. A dependency cycle is a configuration error that lists the task names forming the cycle.
8. Two tasks declaring the same output path is a configuration error naming both tasks and the path.

**Execution**

9. A task runs only after every one of its dependencies has succeeded.
10. The **reported** order of tasks is deterministic: the same configuration always produces the same order in the summary and in the JSON result, whatever order execution actually took. (Execution order is separate and is only constrained to respect dependencies, so a later parallel implementation does not contradict this.)
11. When a task fails, no task depending on it directly or indirectly runs, and tasks on unrelated branches still run.
12. Every task ends in exactly one reported state: `ran`, `cached`, `failed`, or `blocked` when a dependency failed.

**Caching**

13. A task is skipped as `cached` when its command, the contents of its declared inputs, and the recorded results of its dependencies are all unchanged since its last successful run.
14. Changing a task's command invalidates its cache entry.
15. Changing the contents of any declared input invalidates its cache entry.
16. A task whose declared outputs are missing is never reported as cached: it runs again. A cache that reports success while the artefact is absent is worse than no cache.
17. Cache state is stored under a single directory stated in the spec, and deleting that directory returns the runner to a cold state without other effects.

**Reporting**

18. A run ends with a summary counting tasks in each of the four states. Its final line is exactly `summary ran=<n> cached=<n> failed=<n> blocked=<n>`; everything else printed is free.
19. `--format json` writes a result matching the schema fixed in the spec: `order`, an array of task names in reported order; `tasks`, an object keyed by task name whose values are one of the four state strings; and `summary`, an object with the four integer counts. The `order` array is what makes requirement 10 observable at all, since a JSON object has no order. Text is the default. For the same run, both report identical counts and identical per-task states. Deriving one format from the other, or both from one result object, is a legitimate implementation and is not penalised.

**Non-functional**

20. Python 3.11 or later, standard library only, run as `python3 taskrun.py`.
21. A configuration of 1000 tasks whose work is entirely cached completes within 5 seconds.
22. The repository ships its own tests, runnable by one stated command, and `requirements-map.json` naming which of its tests cover each requirement above.

Where the spec is silent, the builder records the decision in `DECISIONS.md`, naming the requirement and the chosen behaviour.

**What the grader may test (SP-2-6).** Only behaviour these requirements pin, plus the one deliberate gap in requirement 10. Anything else a builder must decide — how errors are worded, whether the cache directory is hidden, how `--dry-run` formats its plan beyond being one task per line in order — is out of scope for scoring entirely, and the grader holding a preferred answer to any of it is a defect in the grader. S-B's job is to walk the requirements once more and pin every choice point that ought to be pinned; whatever survives that pass is genuinely free, and free means unscored.

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
| `requirement_coverage` | Share of requirements whose held-out tests all pass |
| `cache_correctness` | A battery: unchanged inputs skip, changed command reruns, changed input reruns, `--force` reruns everything, deleted cache directory returns to cold |
| `failure_propagation` | Dependents of a failure are `blocked`, unrelated branches still run, exit code is 1 |
| `determinism` | Identical configuration produces identical order and identical per-task states across repeated runs |
| `config_errors` | Missing dependency, cycle, and duplicate output are each detected with the required detail and exit 2 |
| `format_agreement` | Text and JSON report identical counts and per-task states |
| `build_clean` | The produced repository runs from a fresh clone with no manual steps |
| `own_tests_pass` | The builder's own suite passes under its stated command |
| `mutation_catch` | Share of requirements whose mapped tests detect a targeted mutation of the careful reference implementation that breaks exactly that requirement |
| `plan_seconds` | Requirement 20, measured on a generated 1000-task configuration |
| `dependency_count` | Non-standard-library imports in the shipped code |
| `gap_handled` | A decision record names requirement 10 and repeated runs agree |

`cache_correctness` and `format_agreement` are reported separately rather than folded into `acceptance`, because they are the two most likely to separate a coordinated run from an uncoordinated one, and burying them in a pass rate would hide the signal the spec exists to produce.

**How the builder's tests reach the mutants (SP-2-3).** A builder's tests are written against its own code, so they can only run against a mutant of the reference through an interface both share: the command line fixed in requirements 1 to 5 and the output contracts in 18 and 19. `mutation_catch` therefore counts only tests that drive `python3 taskrun.py`; tests that import internal modules cannot transfer and score nothing on this metric, though they still count for `own_tests_pass`. This deliberately favours black-box tests, and says so rather than hiding it: they are the tests that survive refactoring, which is the property worth rewarding. A mutant is caught only when the mapped test fails against it and passes against the unmutated reference, so an irrelevant failure earns nothing.

**`mutation_catch` replaces a check that did not work.** The first draft asked whether the builder's mapped tests failed on the seed and passed on the final tree. That proves nothing: the seed contains no `taskrun.py`, so every test fails there merely because the file is missing, and one always-green test per requirement would have scored full marks. Mutation catching asks the question that was actually meant. The kit ships mutants of the careful reference, each declaring the requirement it targets. It does not pretend a mutant breaks exactly one requirement: the requirements overlap by design (a broken cache key offends 13, 14, 15 and 16 at once), so demanding isolation would make the corpus impossible to build and S-C would fail for reasons having nothing to do with the harness. What is required is that every requirement has at least one mutant targeting it, and the metric asks whether the tests mapped to that requirement catch one of its targeted mutants. Building those mutants is real work and belongs to S-C, and it is the only version of this metric that cannot be satisfied by writing `assert True`.

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

## S-5: the reference implementations, and why they come first

Before any agent sees this, we build solutions by hand. Not two endpoints, which would only prove the grader can tell perfect from broken, but a small set spanning what real runs plausibly produce: one careful; one with a cache key that ignores the command; one whose order depends on dictionary iteration; one that runs dependents after a failure; one correct but with tests that assert almost nothing; and one that is simply incomplete, because an unattended run stopped at its fences is the most likely outcome of all.

The grader must separate those in the ways this design predicts, and the spread between them is the first evidence that the spec discriminates at all. If the careful and the sloppy versions score within a hair of each other, the spec has failed at its only job and must change before anything is spent on running it.

Calibration also has to test the budget, not just the scoring. At least one timed hand-build tells us whether twenty-two requirements plausibly fit three hours, which is currently an estimate with nothing behind it.

**What calibration must show before anything is spent (SP-2-7).** Three conditions, each checkable:

1. **Every seeded flaw is caught by the metric designed to catch it.** The command-blind cache key drops `cache_correctness`; the dictionary-order scheduler drops `determinism`; the run-dependents-anyway variant drops `failure_propagation`; the assert-nothing variant drops `mutation_catch` while keeping `own_tests_pass`. A flaw that only shows up in the aggregate is a metric that failed.
2. **The careful reference scores strictly above every flawed variant on the metric that variant targets.** If it does not, that metric is not measuring what it claims.
3. **The incomplete variant scores between them rather than at the floor**, since a run stopped at its fences is the likeliest real outcome and a spec that cannot tell partial work from broken work will read every unfinished run as a failure.

Absolute bounds for the remaining metrics come out of this exercise, from the observed scores of the flawed variants, rather than being invented. Until all three conditions hold, the spec is not ready to be run against.

The careful reference also settles the fixture question, though the previous round settled it wrongly. Saying the fixture is *always* the reference bought stability by abandoning the human's actual structure, which was that the software the harness builds becomes the thing later benchmarks change and debug (SP-2-8).

The real requirement is not that the fixture be hand-built; it is that it never changes once anything has been scored against it. So: **the fixture is chosen once and frozen forever.** After BM-1's first runs, we take either the best agent-built artifact or the careful reference, whichever is sound enough to build on, freeze it as `taskrunner-v1`, and never swap it. A later, better artifact becomes `taskrunner-v2` alongside a new spec version, never a quiet replacement of v1. The human's intent is preserved, and comparability is preserved, because the thing that breaks comparability is swapping after scoring, not choosing an agent-built artifact in the first place.

The bar for choosing the agent-built one is stated rather than felt: it must pass the full held-out grader, contain no requirement scored zero, and be readable enough that the BM-2 change is a fair ask. If nothing meets that, the reference is the fixture and we say so.

## S-6: fences, and why they exceed Mission Zero's

Mission Zero's fences (5 cycles, 5 jobs, one hour) were sized for a one-line fix to prove the loop turns over. Twenty-one requirements with a caching layer will not fit, and shrinking the spec to fit would destroy the substance the user asked for.

Proposed, as a complete vector, because a partial one cannot be sealed: `fence.cycles=8`, `fence.jobs=12`, `fence.concurrency=2`, `fence.job-cap-min=15`, `fence.wall-clock-hours=3`, `host.turn-cap-min=20`, `ledger.cycle-budget=8`, `ledger.no-gain-budget=3`.

The last three matter as much as the first five and were missing: without a host turn cap a single stuck orchestrator turn consumes the wall clock, and without the ledger budgets the mission has no stop-loss, so a run that ships receipts while the gate never moves would burn the whole envelope before anything noticed.

The roster must be pinned by exact model identifier, not by the phrase "cheap models", because the comparability tuple in the parent design requires that two cohorts share an identical roster and a job's requested model must equal what it actually ran. A word like "cheap" is not a roster identity and two runs a month apart would silently differ. The exact identifiers are filled in when the manifest is written, from whatever the project's tier 1 declares at that moment, and recorded in the scorecard.

A run that cannot finish inside that envelope is itself a finding, and the fences remain the only real spending control. This is the one item here that costs real money, and no run happens until the human confirms it.

## S-7: where the artifact lives

Frozen as `benchmark/fixtures/taskrunner-v1/` inside the kit. The kit must be self-contained with no network fetches, and a spec cited by any scorecard is immutable, so a separate repository would break both. BM-2 and BM-3 each take their own copy into their `seed/` when those specs are written, so nothing shifts underneath a benchmark that has already been scored.

`seed/` for BM-1 itself is a git repository containing only `README.md`, `spec.md`, and an empty `tests/`. No `taskrun.py` exists, so any test mapped to a requirement necessarily fails on the seed, which is what makes the differential check meaningful.

## Open questions for the critique

1. Does this discriminate? If two harness versions of genuinely different quality ran it, which metrics separate them and which look identical either way?
2. Is twenty-one requirements the right size for three hours with cheap models, or does it guarantee every run scores badly for reasons unrelated to harness quality?
3. Is the ordering rule the best available seeded gap, or does a better one separate noticing from guessing?
4. Does anything here have a right answer the grader secretly holds, which would punish a defensible choice?
5. Is `format_agreement` trivially satisfiable by generating one format from the other? That is a legitimate implementation and must not be penalised.
6. Does the artifact actually support BM-2 and BM-3, or does building it to this spec produce something with no room for a cross-cutting change and nowhere good to hide a bug?

## Items

| Id | Item | Status |
| --- | --- | --- |
| S-A | Critique this design with Codex against the stated purpose | NOT STARTED, next |
| S-B | Write `spec.md`, `manifest.json`, and `seed/` | NOT STARTED |
| S-C | Build the reference and its flawed variants, the mutant corpus, and prove calibration's three conditions hold | NOT STARTED, before any agent run |

All three remain blocked behind the canonical gates and this plan does not reorder them (SP-2-10): S-A is design work and runs now, but S-B and S-C are item B-1 of the parent design, which B-0 blocks, which in turn waits on Mission Zero. Designing the spec early is deliberate; building it before the runner has been watched running is the mistake this whole sequence exists to avoid.

## Completion

Done when the critique closes and S-B and S-C are shipped, at which point item B-1 of the benchmark design is complete and the first graded run becomes possible.
