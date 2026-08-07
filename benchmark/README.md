# The measuring kit

This kit measures the metasystem: a fixed spec goes in, agents build it
unattended under the metasystem's rules, and two things come out graded — the
software itself and the logged behaviour of the agents that built it. The kit
lives beside the metasystem, never inside it, because the graders are held
out: a benchmark target that received them would ship its builders the answer
key. Adoption cannot ship this folder even by omission; the payload is an
allowlist.

## Contents

- `specs/bm-1/` — the first case: a task runner ("make light") in Java 21 with
  Maven, 26 requirements, one deliberately seeded under-determination. Carries
  its `spec.md` (what builders see), `seed/` (their starting repo), `grader/`
  (held out: 53 acceptance checks, five calibrated probes), `gate.sh` and
  `guard-deps.sh` (the mission instruments), and `manifest.json` — the single
  authority for metrics, formulas, fences, roster, and mission-contract
  policy.
- `extract.sh` / `extractor.py` — turns one run's evidence into the canonical
  scorecard: run-validity gates, constraint metrics, watches, cost per
  provider, and a named logging gap for anything unmeasurable. Nothing is
  ever estimated.
- `rubrics/` — the eight judged dimensions the `behavior-judge` role reads.
  Judged scores never gate; they produce findings.
- `provision.sh` — stages a run: fresh target, adopted metasystem, instruments
  tagged, contract written from the manifest, local bare origin, supervision
  armed. Stops short of sealing and signing, which stay human.
- `validate-kit.sh` — this kit's own gate. Run it beside the metasystem's
  suite whenever either changes.

## Running a trial, end to end

Targets are single-use by design: every run provisions fresh, and
comparability demands it. Do not upgrade or reuse a finished target; archive
it into the evidence store and delete it.

```
# 1. Stage (from the repository toplevel)
benchmark/provision.sh --spec benchmark/specs/bm-1 --target <fresh-dir>

# 2. Human: review the contract it printed, then seal and sign
(cd <target> && scripts/assert-mission.sh --seal --file plans/mission-bm-1.contract.md)
#    …append the Approval line with the printed hash, commit, push to the local origin.

# 3. Preflight and start
(cd <target> && scripts/assert-mission.sh --preflight --file plans/mission-bm-1.contract.md)
(cd <target> && scripts/agents/mission-runner.sh start --mission bm-1)

# 4. After it ends: grade (held-out) and extract the scorecard
benchmark/specs/bm-1/grader/grade.sh <target> > <target>/artifacts/agents/missions/bm-1/grader.out
benchmark/extract.sh <target> --spec benchmark/specs/bm-1 --out <scorecard.json>
```

Roster discipline: benchmark runs use the cheap roster pinned in the spec's
`manifest.json`, never the development roster. The run repeats; it must stay
cheap.

## Versioning and honesty

A spec below 1.0 produces scores, not verdicts, and is comparison-ineligible
(`comparisonEligible` in the manifest). Bounds are set once, at promotion to
1.0, from trial evidence and probe floors — never by calibration and never by
a builder. Trial records and their caveats live with the benchmark design
plan in the metasystem's `plans/`, and every finished trial is archived whole
in the evidence store.


## Where trials land

Provisioned trial repositories (and their `.origin.git` / `.evidence`
siblings) go to the trials root when you pass a bare name as the target.
Set it once, either way:

    export METASYSTEM_TRIALS_ROOT=~/benchmark-trials
    # or, persistently:
    echo ~/benchmark-trials > benchmark/trials-root.local   # gitignored

Unset, the root is the repository's parent directory — the original
behavior. Explicit paths (anything with a slash) always win verbatim.
