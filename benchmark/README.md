# The measuring kit

This kit measures the metasystem: a fixed task goes in, agents build it
unattended under the metasystem's rules, and two things come out graded — the
software itself and the logged behaviour of the agents that built it. The kit
lives beside the metasystem, never inside it, because the graders are held
out: a benchmark target that received them would ship its builders the answer
key. Adoption cannot ship this folder even by omission; the payload is an
allowlist.

## Vocabulary — read this first

The kit has exactly three nouns and one verb. Every script, record, document
and conversation uses them; if a sentence needs a fourth word, the sentence
is wrong.

- **Benchmark case** — *what* is built and how it is judged: the task
  specification, the seed repository, the held-out grader with its calibration
  probes, the instruments (gate and guards), the metrics and noise floors, the
  seeded under-determination, and what the task itself needs from any
  environment (`needs`). A case has **versions**; a case version is one
  immutable directory, `cases/<caseId>/<caseVersion>/`. Its identity is
  `caseId@caseVersion` — `taskrun@0.1`. Any change to spec, seed, grader,
  instruments or `case.json` is a *new* version directory. Today there is one
  case, `taskrun`, in two versions: `0.1` (boolean completion gate) and `0.2`
  (count gate, threshold 26).
- **Benchmark configuration** — *who* builds it and under *what limits*: the
  roster (host runtime and model; delegate runtimes and models, per role when
  they differ; independence declaration; acceptable effective-model aliases),
  the fences (cycles, jobs, caps, wall clock, ledger budgets, the binary-gate
  fuse acknowledgement), the host's native caps when a human has ruled on
  them, the delegate network allowance, the machine constraint the roster
  imposes, the exposure statement, and the **purpose** (`capability` — a
  measurement; `orchestration-health` — a probe of whether the orchestration
  mechanically works, whose score is never a verdict). A configuration has
  versions too; each is one immutable file,
  `configurations/<configId>/<configVersion>.json`. Identity `configId@configVersion`
  — `cheap@1`. Reusable across cases.
- **Benchmark run** (synonym: trial) — one case version under one
  configuration version, repetition *n*, on one machine, at one metasystem
  sha. A **cohort** is N repetitions of one pair. When a sentence needs one
  word for the pair, it is *the benchmark* — "the benchmark taskrun@0.1 under
  cheap@1" — never a bare id.
- The verb: **run CASE under CONFIG**. "Run taskrun@0.1 under devin-host@1,
  cohort of three." "Compare cheap and sol on taskrun@0.1." "New case X." "New
  version of taskrun." "New configuration Y."
- **Alias** — a retired spec id (`bm-1` … `bm-2d-og`) that resolves,
  read-only, to a pair (`aliases.json`). Alias mode keeps the legacy naming
  (mission id, contract name, instrument tag) so cohorts begun before the
  migration stay uniform; it adds the pair to the identity and changes nothing
  else. Aliases never rewrite what an old run *was*.

Why the split: the six former "specs" were one task and six ways of running
it (five identical copies of the seed and grader; ids that baked the roster
into the task's name). The kit could not say "the same task, a different
roster" — the comparison it exists to make. The design record is
`metasystem/plans/benchmark-case-configuration-design.md` (ten critique
rounds; converged 2026-08-19).

## Contents

```
benchmark/
  cases/<caseId>/<caseVersion>/     case.json  spec.md  seed/  grader/  gate.sh  guard-*.sh
  configurations/<configId>/<configVersion>.json
  aliases.json                      retired spec ids -> pair (read-only history)
  versions.lock                     append-only registry: every version's git object id
  pairs.py                          the ONE resolver: refs, aliases, compatibility, the join, the registry
  provision.sh                      stage one run (case version under configuration version)
  run-cohort.sh                     drive N repetitions through the human seal/sign boundary
  grade.sh                          the case's held-out grader, from its pinned tree
  extract.sh / extractor.py         evidence -> scorecard (never estimates)
  compare.sh / compare.py           metasystem verdicts (baseline vs candidate) and the configurations report
  validate-kit.sh                   the kit's own gate
  schemas/                          case, configuration, aliases, versions-lock, proposal, cohort,
                                    benchmark-identity (v2; v1 kept for legacy), scorecard, evidence/
  rubrics/                          the eight judged dimensions (behavior-judge)
  results/                          cohorts, proposals, scorecards, compares, attestations
```

## The two documents

### `cases/<id>/<version>/case.json` — the task half

Schema: `schemas/case.schema.json` (validated by `validate-kit.sh`). Annotated:

```jsonc
{
  "schemaVersion": 1,
  "id": "taskrun",                    // ^[a-z0-9][a-z0-9-]{0,31}$ ; equals the parent-of-parent directory name
  "version": "0.1",                   // ^[0-9]+(\.[0-9]+){0,3}$, ≤16 chars ; equals the directory name
  "title": "taskrun: a dependency-aware task runner (Java, Maven, command line)",
  "comparisonEligible": false,        // task maturity: below 1.0 → scores, not verdicts
  "comparisonEligibleNote": "…",
  "product": {…}, "seededGap": {…}, "language": {…}, "watches": {…}, "deferredMetrics": {…},
  "metrics": {…}, "metricsNote": "…", "noiseFloors": null,   // per-metric floors; null while unset
  "seed":   {"path": "seed/", "description": "…"},            // copied into every target
  "grader": {"path": "grader/", "heldOut": true, "calibration": {…}},   // NEVER copied; run after the mission
  "completionGate": {"command": "bash gate.sh", "decision": "…", "rationale": "…"},
  "needs": {                          // what the TASK requires of any configuration
    "dependencies": "jdk21,maven",    // → contract envelope.dependencies
    "network": "either",              // required | forbidden | either  (see Compatibility)
    "networkNote": "…",
    "os": "any",                      // optional: linux | darwin — only if the TASK demands one
    "minFences": {"cycles": 6},       // optional floors a configuration must meet
    "environmentNotes": {"denyMechanism": "…", "maven": "…"}
  },
  "mission": {                        // the task's part of the mission contract
    "gate":  {"command": "bash gate.sh", "metric": "self-assessment", "direction": "max",
              "threshold": ">=1", "noiseFloor": 0, "paths": "gate.sh,guard-deps.sh", "refKind": "tag"},
    "guard": {…}, "truth": {…},
    "instruments": ["gate.sh", "guard-deps.sh"],   // shipped in this directory; tagged in the target
    "streams": {"build": "… so that gate.sh reports self-assessment=1."},  // must name the gate it runs
    "note": "…"
  }
}
```

### `configurations/<id>/<version>.json` — the running half

Schema: `schemas/configuration.schema.json`. Annotated:

```jsonc
{
  "schemaVersion": 1,
  "id": "cheap", "version": "1",
  "title": "Opus host, gpt-5.6-luna delegates (Claude Opus code critic), 3 h fences",   // the way of running
  "purpose": "capability",            // capability | orchestration-health
  "acceptanceOnly": {…},              // optional: launch cadence only (never on a standing cadence)
  "machineConstraint": {"os": "linux", "reason": "…"},   // optional: what the ROSTER needs
  "roster": {
    "host": {"runtime": "claude", "model": "claude-opus-5"},
    "delegates": {"codex": "gpt-5.6-luna", "claude": "claude-opus-5"},   // one model per runtime slot
    "delegateRoles": {"code-critic": {"runtime": "claude", "model": "claude-opus-5"}, …},   // optional per-role
    "independence": "session-only",   // optional; required when implementer and code critic share a model
    "acceptableEffective": {…}, "ruling": "…"
  },
  "fences": {"cycles": 8, "jobs": 12, "concurrency": 2, "jobCapMin": 15, "hostTurnCapMin": 20,
             "wallClockHours": 3, "ledgerCycleBudget": 8, "ledgerNoGainBudget": 8,
             "acceptBinaryGateFuse": true,   // optional; only meaningful when no-gain < cycles on a binary gate
             "approved": "…"},
  "hostCaps": {"maxTurns": 150, "maxBudgetUsd": "5.00"},   // optional; sealed as host.max-* when present
  "environment": {"delegateNetwork": "allowed"},           // the ALLOWANCE (allowed | denied)
  "exposure": {"statement": "EUR:40", "note": "…"},
  "notes": {"roster": "…", "fences": "…"},
  "provenance": {…}                   // informational: where this configuration came from
}
```

### Ownership, in one table

| Concern | Case | Configuration | Written into the run |
|---|---|---|---|
| id, version, title | task title | way-of-running title | contract intent line = `<case.title> — <config.title>`; mission id = `caseId` |
| spec, seed, grader, instruments | ✓ | | seed and instruments copied from the pinned tree; grader never copied |
| metrics, noise floors, watches, seeded gap, completion gate | ✓ | | extractor; contract `completionGate` |
| gate/guard/truth/streams | ✓ (`mission`) | | contract; instruments tagged `<caseId>-instruments-v<caseVersion>` |
| dependencies, task network requirement, task OS, min fences | ✓ (`needs`) | | `envelope.dependencies`; compatibility |
| roster, independence | | ✓ | contract `host.*`; `metasystem.conf` role lines |
| fences, ledger budgets, fuse acknowledgement | | ✓ | contract `fence.*`, `ledger.*` |
| host caps | | ✓ (optional) | contract `host.max-turns`, `host.max-budget-usd` |
| network allowance | | ✓ | `metasystem.conf dispatch.permissions.network` |
| machine constraint | | ✓ | provisioning refuses off-constraint |
| exposure | | ✓ | contract `exposure=` |
| purpose, acceptanceOnly | | ✓ | verdict eligibility; launch cadence |
| comparisonEligible | ✓ | | verdict eligibility |

## Compatibility (checked at provision and by the kit gate)

| case `needs` | configuration | rule |
|---|---|---|
| `network: required` | `delegateNetwork: denied` | refused |
| `network: forbidden` | `delegateNetwork: allowed` | refused |
| `network: either` | any | the configuration's allowance applies |
| `os: linux/darwin` | `machineConstraint.os` | must not contradict; the running machine must satisfy both |
| `minFences.<f>` | `fences.<f>` | `fences.<f> >= minFences.<f>` |
| — | roster | implementer and code critic on one effective model require `independence: session-only` (docs/orchestration.md step 4) |
| — | fences | binary gate with `ledgerNoGainBudget < cycles` needs `acceptBinaryGateFuse` — otherwise the contract **will not seal**; the kit reports it as a warning naming the configuration (today: `devin-delegate@1`, `devin-host@1`, `devin-host-claude-delegate@1`, inherited verbatim and awaiting a human ruling) |

## Immutability and pinning

Versions are immutable. `versions.lock` records every version's git object id
(a tree for a case directory, a blob for a configuration file), appended by
`pairs.py register`, never edited by hand. `validate-kit.sh` refuses when a
registered object differs at HEAD ("edited in place — add a new version") and
when any id ever registered maps to a different object anywhere in the file's
git history (a rewritten registry line); it needs full history and refuses a
shallow or grafted clone (`git fetch --unshallow`).

Every run pins what it ran: `benchmark-identity.json` (schema 2) carries
`caseId`, `caseVersion`, `caseTree`, `configId`, `configVersion`, `configTree`.
Provisioning copies the seed, instruments and grader **from the pinned tree
object** (`git archive <caseTree>`), never from the working directory, and
refuses a case directory or configuration file with uncommitted changes.
Every later reader of a run — the extractor, `compare.py`, a cohort resume —
reads the case and configuration documents from those pinned objects, never
from current paths; an unreachable pin makes the run unreadable for that
purpose (fetch the run's `measuringMetasystemSha` from origin, then retry).
Legacy runs (identity schema 1) have no pins: they resolve through
`aliases.json`, are reported with `legacyId`, and are never verdict-compared
against pinned runs.

## Comparability and eligibility

- **Metasystem verdict** (`benchmark/compare.sh <baseline-cohort> <candidate-cohort>`):
  the two cohorts must agree on the whole tuple — `caseId`, `caseVersion`,
  `caseTree`, `configId`, `configVersion`, `configTree`, `measuringKitVersion`,
  `repetitionCount`, `machineFingerprint`, `measuringMetasystemSha`, sealed
  roster and fences. The task and the way of running are held constant; the
  metasystem sha is what varies.
- **Verdict eligibility** of a run = `case.comparisonEligible` (≥ 1.0) **and**
  `configuration.purpose == capability`. An `orchestration-health`
  configuration is never verdict-eligible.
- **Configuration report** (`benchmark/compare.sh --configurations <cohort> <cohort> …`):
  the same case version under different configurations, side by side, kit
  version / machine / measuring sha held constant. A report, never a verdict.
- **Proposals** (`results/proposals/<id>.json`, schema 2) name pairs:
  `"benchmarks": [{"case":"taskrun","caseVersion":"0.1","config":"cheap","configVersion":"1"}]`.
  Schema-1 proposals (`"specs": ["bm-1"]`) are read through the alias table.

## Running a trial, end to end

Targets are single-use by design: every run provisions fresh, and
comparability demands it. Do not upgrade or reuse a finished target; archive
it into the evidence store and delete it. Versions are always pinned on the
command line; an unpinned reference is refused with the versions available.

```
# 1. Stage (from the repository toplevel; any shell — genesis admits the provisioner for the goal-free first baseline)
benchmark/provision.sh --case taskrun@0.1 --config cheap@1 --target <fresh-dir>

# 2. Human: review the contract it printed, then seal and sign
(cd <target> && scripts/assert-mission.sh --seal --file plans/mission-taskrun.contract.md)
#    …append the Approval line with the printed hash, commit, push to the local origin.

# 3. Preflight and start
(cd <target> && scripts/assert-mission.sh --preflight --file plans/mission-taskrun.contract.md)
(cd <target> && scripts/agents/mission-runner.sh start --mission taskrun)

# 4. After it ends: grade (held-out) and extract the scorecard
benchmark/grade.sh --case taskrun@0.1 <target> > <target>/artifacts/agents/missions/taskrun/grader.out
benchmark/extract.sh <target> --case taskrun@0.1 --config cheap@1 --out <scorecard.json>
```

Cohorts (`benchmark/run-cohort.sh --case taskrun@0.1 --config cheap@1 --repetitions 3`)
stop at each human seal/sign boundary and print the exact `--resume` command;
a resumed cohort runs the repetition through grading and extraction, then
provisions the next fresh target or completes. Its materialized case
(`<runs>/<cohort>/case/`: the pinned tree plus the merged manifest and
`pair.json`) is what grading and extraction read.

Roster discipline: benchmark runs use the roster pinned in the configuration,
never the development roster. The run repeats; it must stay cheap.

## Adding a case, a case version, a configuration

**A new case** (a genuinely different task):

1. `mkdir -p benchmark/cases/<id>/0.1` and write `spec.md`, `seed/`,
   `grader/` (with `grade.sh` and its calibration probes), the instruments,
   and `case.json` per the schema. The grader must be held out (`heldOut:
   true`) and no seed path may overlap it. Every stream must name the gate it
   runs (metric and threshold).
2. `git add benchmark/cases/<id>/0.1 && benchmark/pairs.py register --case <id>@0.1`
   (computes the tree id from the index and appends it to `versions.lock`).
3. `git add benchmark/versions.lock && benchmark/validate-kit.sh` — schemas,
   seams, registry, compatibility with every configuration you intend to
   pair it with (`pairs.py resolve --case <id>@0.1 --config <cfg>@<v>`).
4. Commit the directory and the registry line together. From then on the
   directory is immutable.

**A new case version** (any change to spec, seed, grader, instruments or
`case.json`): copy the previous version directory to `<id>/<new-version>/`,
edit, register the new version, commit. Never edit an existing version
directory; the kit refuses it. Say in `case.json` what changed and why.

**A new configuration** (a new roster, new fences, or a new purpose):

1. Write `benchmark/configurations/<id>/1.json` per the schema. Give it a
   title that describes the way of running. If implementer and code critic
   share an effective model, declare `roster.independence: "session-only"`
   and say why. If the gate is binary and `ledgerNoGainBudget < cycles`,
   either raise the budget or acknowledge the fuse — a human ruling either
   way; do not leave it to be discovered at seal time.
2. `git add` it, `benchmark/pairs.py register --config <id>@1`, add the lock,
   `benchmark/validate-kit.sh`, commit both together.

**A new configuration version**: new file `<id>/<n+1>.json`, register,
commit; the old file stays.

## Aliases and pre-migration cohorts

`aliases.json` maps `bm-1`, `bm-2`, `bm-2d`, `bm-2dc`, `bm-2s`, `bm-2d-og` to
their pairs and records the version label each manifest carried (`bm-2`,
`bm-2d`, `bm-2dc`, `bm-2d-og` said 0.2 over v0.1 instruments; the alias
resolves them to `taskrun@0.1` and keeps the label). `provision.sh --spec
bm-1` and `run-cohort.sh --spec bm-1` still work and provision the same pair
with the legacy naming; a cohort begun before the migration resumes through
the alias table (`run-cohort.sh --resume <cohort-id>`), grades from the
resolved case version's tree, and keeps its mission id and contract path as
recorded. Use `--case/--config` for anything new.

To read the exact bytes a run used: `git cat-file -p <caseTree>:case.json`,
`git archive <caseTree>`, `git cat-file -p <configTree>` — from a clone that
has the run's `measuringMetasystemSha` (`git fetch origin <sha>` if not).

## Versioning and honesty

A case below 1.0 produces scores, not verdicts, and is comparison-ineligible
(`comparisonEligible`). Bounds are set once, at promotion to 1.0, from trial
evidence and probe floors — never by calibration and never by a builder.
Trial records and their caveats live with the benchmark design plans in the
metasystem's `plans/`, and every finished trial is archived whole in the
evidence store, named by its pair.

## Where benchmark repositories land: the trials root

Every benchmark run creates real directories — the trial repository plus
its `.origin.git` and `.evidence` siblings, and for cohorts a `cohorts/`
tree. By default these land beside the kit's own repository, which
clutters whatever folder you keep your checkouts in. Designate a home for
them once, either way:

    # per shell:
    export METASYSTEM_TRIALS_ROOT=~/meta-system-benchmarks

    # or persistently (the file is gitignored — a per-machine choice):
    echo ~/meta-system-benchmarks > benchmark/trials-root.local

With a root set:

- `benchmark/provision.sh … --target my-trial` puts the trial and both
  siblings under the root. Only BARE names resolve this way — any target
  containing a slash is honored verbatim, so scripted callers with explicit
  paths are unaffected.
- `benchmark/run-cohort.sh` places its cohort runs under `<root>/cohorts/`
  instead of the kit-local `benchmark/.runs`.

The environment variable wins over the file; with neither set, the
historical defaults apply unchanged (repository parent for trials,
`benchmark/.runs` for cohorts). Resolution is identical in both scripts,
so one setting governs everything a benchmark writes outside the kit.
