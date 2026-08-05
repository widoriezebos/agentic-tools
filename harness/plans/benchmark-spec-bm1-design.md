# Plan: BM-1, the First Benchmark Spec

- Owner: unclaimed (written 2026-08-05, single session, nothing built)
- Goal and current status: design the first benchmark spec, a task runner the harness builds from scratch unattended, substantial enough that building it successfully means something, and shaped so the result becomes the fixture for the two benchmarks after it. Status: designed, not critiqued, nothing built
- In flight right now: nothing
- Decisions made (and who made them): the user chose the three-benchmark structure and the task-runner subject on 2026-08-05; design decisions S-1 through S-7 taken here with recorded defaults
- Waiting on the human: the fence sizing in S-6, which is larger than Mission Zero's and costs real money
- Dead ends: a log summariser was drafted first and discarded: the subject was picked without asking, and it was too shallow to discriminate. A diff and patch toolkit was rejected because the core algorithm is a well-known exercise, so it would partly measure recall. An append-only store with compaction was rejected because its most interesting property, crash safety, cannot be graded in minutes
- Next step: Codex critique against the stated purpose, then build it as item B-1

This plan fills in item B-1 of `plans/harness-benchmark-design.md`, which fixed the shape of a spec but wrote neither requirements nor grader. It changes nothing in that design.

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
10. Independent tasks execute in a deterministic order: the same configuration always produces the same order.
11. When a task fails, no task depending on it directly or indirectly runs, and tasks on unrelated branches still run.
12. Every task ends in exactly one reported state: `ran`, `cached`, `failed`, or `blocked` when a dependency failed.

**Caching**

13. A task is skipped as `cached` when its command, the contents of its declared inputs, and the recorded results of its dependencies are all unchanged since its last successful run.
14. Changing a task's command invalidates its cache entry.
15. Changing the contents of any declared input invalidates its cache entry.
16. Cache state is stored under a single directory stated in the spec, and deleting that directory returns the runner to a cold state without other effects.

**Reporting**

17. A run ends with a summary counting tasks in each of the four states.
18. `--format json` writes a machine-readable result; text is the default; for the same run both report identical counts and identical per-task states.

**Non-functional**

19. Python 3.11 or later, standard library only, run as `python3 taskrun.py`.
20. A configuration of 1000 tasks whose work is entirely cached completes within 5 seconds.
21. The repository ships its own tests, runnable by one stated command, and `requirements-map.json` naming which of its tests cover each requirement above.

Where the spec is silent, the builder records the decision in `DECISIONS.md`, naming the requirement and the chosen behaviour.

## S-3: the seeded ambiguity

Requirement 10 demands a deterministic order among independent tasks and deliberately never says which order. Alphabetical, configuration order, and dependency depth are all defensible.

This is the right gap. A careless reader sees "deterministic" and assumes whatever their data structure does; a careful one notices no rule is given. Every defensible answer is genuinely acceptable, so the grader holds no secret right answer. And noticing is separable from choosing well: the first is machine-checkable, the second is a judgement.

Correct handling is a `DECISIONS.md` entry naming requirement 10 and the chosen rule, with an implementation that matches it. Shipping whatever ordering the language happens to produce fails even when the output looks right, because it is not a decision.

The grader checks only two things: a decision record names requirement 10, and repeated runs over the same configuration produce identical order. Whether the rule is sensible is left to the judged layer, because a checker that parsed prose rules would be a hidden right answer in disguise.

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
| `differential_coverage` | Share of requirements with at least one mapped test failing on the seed and passing on the final tree, each test counting for at most one requirement |
| `plan_seconds` | Requirement 20, measured on a generated 1000-task configuration |
| `dependency_count` | Non-standard-library imports in the shipped code |
| `gap_handled` | A decision record names requirement 10 and repeated runs agree |

`cache_correctness` and `format_agreement` are reported separately rather than folded into `acceptance`, because they are the two most likely to separate a coordinated run from an uncoordinated one, and burying them in a pass rate would hide the signal the spec exists to produce.

## S-5: the reference implementations, and why they come first

Before any agent sees this, we build two solutions by hand: one careful, one deliberately sloppy in realistic ways — a cache key that ignores the command, a scheduler whose order depends on dictionary iteration, a failure path that runs dependents anyway.

The grader then has to score them differently, in the ways this design predicts. Until it does, it is an opinion with a number attached, and a poor first run would be indistinguishable from a poor grader.

The careful reference also solves a practical problem: BM-2 and BM-3 need a frozen artifact, and we cannot depend on BM-1 producing a usable one. The reference is the reliable fixture, and a good agent-built version can replace it later.

## S-6: fences, and why they exceed Mission Zero's

Mission Zero's fences (5 cycles, 5 jobs, one hour) were sized for a one-line fix to prove the loop turns over. Twenty-one requirements with a caching layer will not fit, and shrinking the spec to fit would destroy the substance the user asked for.

Proposed: 8 cycles, 12 jobs, concurrency 2, 15 minutes per job, 3 hours wall clock, cheap delegate models. A run that cannot finish inside that is itself a finding, and the fences remain the only real spending control.

This is the one item here that costs real money, and it is the human's to confirm.

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
| S-C | Build the two reference implementations and prove the grader separates them | NOT STARTED, before any agent run |

## Completion

Done when the critique closes and S-B and S-C are shipped, at which point item B-1 of the benchmark design is complete and the first graded run becomes possible.
