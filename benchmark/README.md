# The measuring kit

This kit measures the metasystem: a fixed task goes in, agents build it
unattended under the metasystem's rules, and two things come out graded — the
software itself and the logged behaviour of the agents that built it. Today the
kit holds the benchmark's data only; its drivers were retired (see below). The kit
lives beside the metasystem, never inside it, because the graders are held
out: a benchmark target that received them would ship its builders the answer
key. Adoption cannot ship this folder even by omission; the payload is an
allowlist.

## Vocabulary — read this first

The kit has exactly three nouns and one verb. Every record, document
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

## The two documents

### `cases/<id>/<version>/case.json` — the task half

Schema: `schemas/case.schema.json`. Annotated:

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

## Compatibility (what a driver must check)

| case `needs` | configuration | rule |
|---|---|---|
| `network: required` | `delegateNetwork: denied` | refused |
| `network: forbidden` | `delegateNetwork: allowed` | refused |
| `network: either` | any | the configuration's allowance applies |
| `os: linux/darwin` | `machineConstraint.os` | must not contradict; the running machine must satisfy both |
| `minFences.<f>` | `fences.<f>` | `fences.<f> >= minFences.<f>` |
| — | roster | implementer and code critic on one effective model require `independence: session-only` (docs/orchestration.md step 4) |
| — | fences | binary gate with `ledgerNoGainBudget < cycles` needs `acceptBinaryGateFuse` — otherwise the contract **will not seal**; a driver reports it as a warning naming the configuration (today: `devin-delegate@1`, `devin-host@1`, `devin-host-claude-delegate@1`, inherited verbatim and awaiting a human ruling) |

## The drivers were retired (2026-09-28)

The kit's driver scripts are gone: `provision.sh`, `run-cohort.sh`,
`validate-kit.sh`, `attest.sh`, `grade.sh`, `compare.sh`/`compare.py`,
`extract.sh`/`extractor.py`, `pairs.py`, `stage_evidence.py`,
`system-fingerprint.py` and their fixtures. They could not run after
2026-08-30: provisioning validated the contract with the target's
`scripts/assert-mission.sh` and cohorts started missions with
`scripts/agents/mission-runner.sh`, both deleted then (1a062750c), and
`attest.sh` ran `validate-metasystem.sh`, deleted 2026-09-27. Nothing
noticed for a month, which is how we know nobody ran them. The metasystem's
verb redesign (`metasystem/plans/designs/verbs-object-action.md`, unit U8b)
removed them rather than patch scripts that could not work. Their text is in
Git history before that unit.

What stays is the benchmark's substance, the declared extension point
(design 3.3): the cases with their seeds, instruments and held-out graders,
the configurations, the rubrics, the schemas, `aliases.json`,
`versions.lock` and `model-equivalence.json`. Running a benchmark again is a
new goal: a driver in Go behind a public `experiment` action that calls only
public actions (`system adopt`, `mission start`, `mission status`, `work land
--local`), pins the case and configuration by Git object id as described
below, and keeps the graders out of every target.

## What the retired drivers guaranteed (a rebuild owes the same)

- Versions are immutable. `versions.lock` records every version's Git object
  id (a tree for a case directory, a blob for a configuration file),
  append-only. A registered object that differs at HEAD means a version was
  edited in place; a registry line rewritten anywhere in history is refused.
- Every run pins what it ran (`benchmark-identity.json`, schema 2: `caseId`,
  `caseVersion`, `caseTree`, `configId`, `configVersion`, `configTree`), and
  seed, instruments and grader are copied from the pinned tree object, never
  the working directory. Every later reader reads the pinned objects.
- A target is single-use: every run provisions fresh.
- A metasystem verdict compares two cohorts that agree on the whole tuple
  (case and configuration pins, kit version, repetition count, machine
  fingerprint, sealed roster and fences) and differ only in the metasystem
  SHA. A run is verdict-eligible only when `case.comparisonEligible` and
  `configuration.purpose == capability`; a comparison across configurations
  is a report, never a verdict.
- Scorecards never estimate: a missing measurement is a gap, and mission
  state is trusted only after the engine has verified its hash chain and
  anchor.
- Benchmark runs use the roster pinned in the configuration, never the
  development roster.

To add a case, a case version or a configuration before a driver exists,
follow the documents above and add a new version directory or file; never
edit an existing one. Registering it in `versions.lock` waits for the
driver's registration tool.

## Versioning and honesty

A case below 1.0 produces scores, not verdicts, and is comparison-ineligible
(`comparisonEligible`). Bounds are set once, at promotion to 1.0, from trial
evidence and probe floors — never by calibration and never by a builder.
Trial records and their caveats live with the benchmark design plans in the
metasystem's `plans/`, and every finished trial is archived whole in the
evidence store, named by its pair.
