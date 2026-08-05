# Plan: BM-1, the First Benchmark Spec

- Owner: unclaimed (written 2026-08-05, single session, nothing built)
- Goal and current status: design the first benchmark spec: a piece of software the harness builds unattended, wide enough in scope that grading the result and reading the run tells us how well the harness performed. Status: designed, not critiqued, nothing built
- In flight right now: nothing
- Decisions made (and who made them): the purpose stated by the user 2026-08-05, that the spec exists to judge harness performance and is the first of several; design decisions S-1 through S-6 taken here with recorded defaults
- Waiting on the human: the fence sizing in S-5, which is larger than Mission Zero's and costs real money
- Dead ends: none yet
- Next step: Codex critique against the stated purpose, then build it as item B-1

This plan fills in item B-1 of `plans/harness-benchmark-design.md`, which fixed the shape of a spec and named a candidate subject but wrote neither the requirements nor the grader. It changes nothing in that design.

## What this spec is for

The user's framing, which is the design target: the spec must be wide enough in scope that, once the harness has built it, we can judge how well the harness performed. It is the first of several specs; this one does not have to cover everything, but it must discriminate.

Discriminate is the operative word. A task one agent finishes correctly in a single pass tells us nothing, because every version of the harness scores the same on it. The spec earns its place only if a well-coordinated run and a badly coordinated one produce visibly different results. That means deliberately including work where coordination, sequencing, and review are what determine the outcome.

Five things the harness claims to be good at, and how this spec puts pressure on each:

| Claim | Pressure |
| --- | --- |
| Work is split across sub-agents without the pieces drifting apart | Two output formats must report identical numbers, so independently built pieces must agree on one internal contract |
| Structural decisions are made before building, not retrofitted | A performance budget that a whole-file-in-memory implementation cannot meet, and which is expensive to retrofit late |
| Delegates stop and report gaps rather than guessing | One requirement is deliberately under-determined, and correct handling is a recorded decision rather than a silent choice |
| Claims are verified by running, not asserted | Edge cases that pass a reading and fail a run: empty input, malformed lines, filters that interact |
| Tests are not weakened to pass | The builder ships its own tests plus a map of which test covers which requirement, and the grader checks those tests actually fail on the starting state |

## S-1: the subject

A command-line log summariser, `logsum.py`, Python 3.11+ standard library only.

Rejected alternatives: a parser or small query language (more design depth, but too large to finish inside any sane budget); a filesystem tool such as a deduplicator (filesystem variance makes grading non-deterministic); a key-value store with crash safety (the interesting part, crash consistency, cannot be graded in minutes).

The log summariser wins on being gradeable to the byte, having genuine edge cases, admitting a performance constraint that forces an early structural decision, and splitting naturally into parts that must agree.

## S-2: the requirements

These are the numbered requirements as they will appear in `spec.md`. Input lines are:

```
<ISO-8601 timestamp with Z> <LEVEL> <source> <message>
2026-08-05T07:12:31Z ERROR auth-service Connection refused
```

1. `python3 logsum.py <path>` summarises the named file. A path of `-` reads standard input.
2. A line parses into timestamp, level, source, and message. The message runs to end of line and may contain spaces.
3. Recognised levels are `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`, case-sensitive.
4. A line that does not parse, or carries an unrecognised level, is counted as skipped. Skipped lines never abort the run.
5. `--since <timestamp>` includes lines at or after it; `--until <timestamp>` includes lines strictly before it.
6. `--level <LEVEL>` includes that level and every more severe level, in the order `DEBUG < INFO < WARN < ERROR < FATAL`.
7. `--source <name>` may be given more than once and restricts output to the named sources.
8. The summary reports lines read, lines parsed, lines skipped, a count per level, and the top N sources by matching line count.
9. `--top <N>` sets N and defaults to 5. A value below 1, or not an integer, is a usage error.
10. `--format json` emits a JSON object; `--format text` is the default. For the same input and options, every number reported in one format equals the number reported in the other.
11. Exit codes are exactly: 0 success, 2 usage error, 1 input error such as a missing or unreadable file.
12. Input containing no lines is not an error: every count is zero and the exit code is 0.
13. A 200 MB input of roughly two million lines is summarised within 60 seconds, with peak resident memory under 256 MB.
14. No third-party packages. The tool runs under `python3 logsum.py` with the standard library alone.
15. The repository ships its own tests, runnable by one stated command, and `requirements-map.json` naming which of its tests cover each requirement above.

Where the spec is silent, the builder records the decision in `DECISIONS.md`, naming the requirement and the chosen behaviour.

## S-3: the seeded ambiguity

Requirement 8 says "top N sources by matching line count" and deliberately never says how ties are broken. Two sources with equal counts have no defined order.

This is the right gap for three reasons. It is invisible to a careless reader and obvious to a careful one. Any total, deterministic rule is defensible, so the grader is not secretly holding a right answer. And "did they notice" is separable from "did they choose well": noticing is machine-checkable, the quality of the choice is a judgement.

Correct handling is a `DECISIONS.md` entry naming requirement 8 with the chosen rule, and an implementation that matches it. Silently shipping whatever ordering the language happens to produce fails, even when the output looks right on inputs without ties.

The grader checks two things mechanically: a decision record exists naming requirement 8, and repeated runs over a tie-inducing input produce identical output. It deliberately does not demand a particular rule. Whether the recorded rule is sensible, and whether the implementation genuinely follows it, is left to the judged layer, because a checker that parsed prose rules would be a hidden right answer wearing a disguise.

## S-4: the grader

Held out by construction: `grader/` is never copied into the repository the agents work in. It emits `metric=<name>=<value>` lines in the gate grammar the mission contract already uses.

| Metric | What it measures |
| --- | --- |
| `acceptance` | Share of held-out tests passing, across all fifteen requirements |
| `requirement_coverage` | Share of requirements whose held-out tests all pass |
| `build_clean` | The produced repository runs from a fresh clone with no manual steps |
| `own_tests_pass` | The builder's own suite passes under its stated command |
| `differential_coverage` | Share of requirements with at least one mapped test that fails on the seed and passes on the final tree, each test counting for at most one requirement |
| `format_agreement` | Text and JSON report identical numbers across a battery of inputs and option combinations |
| `perf_seconds`, `peak_rss_mb` | Requirement 13, measured on a generated input |
| `dependency_count` | Non-standard-library imports found in the shipped code |
| `gap_handled` | The decision record names requirement 8 and repeated runs on tie-inducing input agree |

`format_agreement` is called out separately from `acceptance` because it is the metric most likely to separate a coordinated run from an uncoordinated one, and burying it inside a pass rate would hide exactly the signal the spec exists to produce.

## S-5: fences, and why they are bigger than Mission Zero's

Mission Zero's fences (5 cycles, 5 jobs, one hour) were sized for a one-line fix whose purpose was to prove the loop turns over. Fifteen requirements with a performance constraint will not fit in them, and shrinking the spec to fit would destroy the scope the user asked for.

Proposed: 8 cycles, 12 jobs, concurrency 2, 15 minutes per job, 3 hours wall clock, cheap delegate models. A run that cannot finish inside that is itself a finding, and the fences remain the only real spending control.

This is the one item on this page that costs real money and it is the human's to confirm.

## S-6: the seed

`seed/` is a git repository containing `README.md` pointing at the spec, `spec.md` itself, an empty `tests/` directory, and nothing else. No `logsum.py` exists, so any test the builder maps to a requirement necessarily fails on the seed, which is what makes the differential check meaningful.

## Open questions for the critique

1. Does this spec actually discriminate? If two harness versions of genuinely different quality ran it, which metrics would separate them, and which would look identical either way?
2. Is fifteen requirements the right size for three hours with cheap models, or does it guarantee that every run scores badly for reasons unrelated to harness quality?
3. Is the tie-break the best available seeded gap, or is there one that better separates noticing from guessing?
4. Does anything here have a right answer the grader is secretly holding, which would punish a defensible choice?
5. Is `format_agreement` measurable without being trivially satisfiable by generating one format from the other, which is a legitimate implementation and should not be penalised?

## Items

| Id | Item | Status |
| --- | --- | --- |
| S-A | Critique this design with Codex against the stated purpose | NOT STARTED, next |
| S-B | Write `spec.md`, `manifest.json`, and `seed/` | NOT STARTED |
| S-C | Write the held-out grader and prove it against two hand-built reference solutions, one good and one deliberately sloppy, so we know it separates them before any agent sees it | NOT STARTED |

Item S-C matters more than it looks: a grader nobody has calibrated is an opinion. Building one correct and one deliberately poor solution by hand, and checking the grader scores them differently in the ways this design predicts, is what makes the first real run interpretable.

## Completion

Done when the critique closes and S-B and S-C are shipped, at which point item B-1 of the benchmark design is complete and the first graded run becomes possible.
