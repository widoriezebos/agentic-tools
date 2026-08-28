# Benchmark cases and configurations: one task, many ways of running it

Working Mode: design

Owner: main session (Claude, this checkout's lease holder), under Wido's
ruling of 2026-08-19: "we don't have multiple benchmarks, we have a single
benchmark case with multiple benchmark configurations… designed, critiqued and
then implemented… also update the documentation so there is no confusion by
anybody including agents working on other machines". Status: CONVERGED at r10 (2026-08-19). Ten adversarial critique rounds by
codex gpt-5.6-sol: r1 11 findings, r2 8, r3 5, r4 6, r5 3, r6 3, r7 2, r8 1,
r9 1, r10 ACCEPT with two low non-material notes (folded: the tag bound is
62 characters; the history-shortening fixture also covers grafts). Outputs at
plans/benchmark-case-configuration-critique-r{1..10}.json. Implementation
follows this document; deviations are recorded in the decision entry.

## 1. The problem, in evidence

`benchmark/specs/` holds six "specs". Five share byte-identical `spec.md`,
`seed/`, `grader/`, `gate.sh` and `guard-deps.sh`; the sixth (`bm-2s`) shares
spec, seed, grader and guard but carries a DIFFERENT gate — the v0.2 count gate
(threshold `>=26`, human ruling 2026-08-11) — while `bm-2`, `bm-2d`, `bm-2dc`
and `bm-2d-og` say "0.2" in their manifests and ship the v0.1 boolean gate.

| dir | manifest says | gate actually shipped | host | delegates | fences c/j/cap/host/wall/no-gain | os | purpose |
|---|---|---|---|---|---|---|---|
| bm-1 | 0.1 | v0.1 boolean `>=1` | claude:claude-opus-5 | codex:gpt-5.6-luna (+claude critic) | 8/12/15m/20m/3h/8 | any | capability |
| bm-2 | 0.2 | v0.1 boolean | claude:claude-opus-5 | devin:swe-1-7 | 8/12/180m/240m/12h/5 | any | capability |
| bm-2d | 0.2 | v0.1 boolean | devin:swe-1-7 | devin:swe-1-7 | 8/12/180m/240m/12h/5 | linux | health probe |
| bm-2d-og | 0.2 | v0.1 boolean | devin:claude-opus-5-high | devin:gpt-5-5-high | 8/12/180m/240m/12h/8 | linux | health probe |
| bm-2dc | 0.2 | v0.1 boolean | devin:swe-1-7 | claude:claude-opus-5 | 8/12/180m/240m/12h/5 | linux | health probe |
| bm-2s | 0.2 | **v0.2 count `>=26`** | claude:claude-opus-5 | codex:gpt-5.6-sol | 8/12/30m/30m/3h/8 | any | capability |

So: ONE task in TWO instrument versions, six ways of running it, five copies
of the seed and grader, six copies of ~300 lines of task policy, version labels
that do not describe the instruments, and ids that bake the way-of-running into
the task's name. `bm-2d-og`'s `fixtureSpec` reaches back into `bm-2d`: a patch
over the symptom. Nothing in the kit can say "the same task, a different
roster" — the comparison the benchmark exists to make.

## 2. Vocabulary (the contract; every document and message uses these words)

- **Benchmark case** — WHAT is built and how it is judged: task specification,
  seed repository, held-out grader and calibration probes, instruments (gate,
  guards), requirements, seeded under-determination, metrics and noise floors,
  watches, completion gate, and what the task itself needs from any
  environment. A case has **versions**; a case version is an immutable,
  self-contained directory: `cases/<caseId>/<caseVersion>/`. Its identity is
  `caseId@caseVersion` (`taskrun@0.1`, `taskrun@0.2`). Any change to spec,
  seed, grader, instruments or `case.json` is a NEW version directory — a
  convention git already enforces by commit identity; the kit does not police
  in-place edits with a self-referential digest (r2 critique G-4/G-8). Instead
  every run PINS what it ran: the identity records `caseTree`, the git tree
  object id of the case version directory (`git rev-parse HEAD:benchmark/cases/<id>/<v>`),
  and provisioning copies the seed, instruments and grader **from that tree
  object** (`git archive <caseTree>`, i.e. the id it just recorded), never
  from the working directory or from a path re-resolved later — so ignored or untracked bytes under `seed/` can neither ride
  along nor evade the pin (r4 critique H-1; adoption already exports its
  payload from tracked HEAD for the same reason). Provisioning REFUSES when
  the case version directory has uncommitted or untracked changes (`git status
  --porcelain --ignored=no -- <dir>` non-empty); there is no dirty fallback
  (r3 critique G-13). A later edit can never be mistaken for the content an
  old run used.
- **Benchmark configuration** — WHO builds it and under WHAT limits: roster
  (host runtime and model; delegate runtimes and models, per role when they
  differ; independence declaration; acceptable effective-model aliases),
  fences (cycles, jobs, caps, wall clock, ledger budgets, binary-gate fuse
  acknowledgement), the host's native caps when the human has ruled on them,
  the delegate network allowance, the machine constraint the roster imposes,
  the exposure statement, and the purpose. Identity `configId@configVersion`
  (`cheap@1`, `devin-host@1`); like cases, every version is its own immutable
  file on disk, `configurations/<configId>/<configVersion>.json` (r3 critique
  G-12: archived proposals and cohorts name a configuration version and must
  stay checkable after a bump), and every run pins `configTree`, the git blob
  id of that file at provision time (`git rev-parse HEAD:<path>`; refused when
  the file is uncommitted) — r4 critique H-2. Every later reader that needs
  the configuration or case document of a RUN (extractor, compare, resume)
  reads it from the pinned object (`git cat-file -p <configTree>`,
  `git cat-file -p <caseTree>:case.json`). There is NO path fallback for a
  pinned run: an unreachable pinned object makes the run UNREADABLE for that
  purpose (extraction records the gap and no metrics that depend on the
  document; compare refuses the run; resume refuses) — r6 critique J-1. The
  only path read for an EXISTING run is a schema-1 identity that never had a
  pin: it resolves through the alias table, is reported as `legacyId`, and is
  never verdict-eligible against a pinned run. Current paths are for
  AUTHORING and for provisioning NEW runs; pinned ids are for everything about
  an existing run (r5 critique I-1). Reusable across cases.
- **Benchmark run** (synonym: trial) — one execution of a case version under a
  configuration: `caseId@caseVersion × configId@configVersion`, repetition n,
  on one machine fingerprint, at one metasystem sha. A **cohort** is N
  repetitions of one pair.
- **Benchmark** — the pair, when a sentence needs one word: "the benchmark
  taskrun@0.1 under cheap@1". Never a bare id.
- The verb: **run CASE under CONFIG**. "Run taskrun under devin-host, cohort of
  three." "Compare cheap and sol on taskrun." "New case X." "New configuration
  Y." "New version of taskrun." The CLI accepts exactly these nouns.
- **Alias** — a retired id (`bm-1` … `bm-2d-og`) that resolves, read-only, to
  a pair. Aliases never rewrite what an old run WAS: an old identity record
  keeps its legacy id and its legacy version label; the alias table adds the
  resolved pair beside them.

## 3. Comparability and eligibility, stated once

- **Metasystem verdict comparison** (what `compare.py` does): two runs are
  comparable only when the whole comparability tuple matches — `caseId`,
  `caseVersion`, `configId`, `configVersion`, `caseTree`, `configTree`,
  `measuringKitVersion`, `repetitionCount`, `machineFingerprint`,
  `measuringMetasystemSha`, and the sealed effective roster and fences —
  today's tuple with the spec id and version replaced by the four pair fields
  plus the pinned case tree (r3 critique G-11); cohort record and scorecard
  must agree on all of them. Nothing else in `compare.py`'s
  tuple, ancestry or cohort logic loosens.
- **Configuration comparison** is a separate, non-verdict report:
  `compare.sh --configurations` prints, for one `caseId@caseVersion` and a
  fixed kit version + machine fingerprint + measuring sha, the scorecards of
  each configuration side by side. It never emits a verdict.
- **Verdict eligibility** of a run = `case.comparisonEligible` (task maturity:
  below 1.0 → scores, not verdicts) AND `config.purpose == "capability"`. A
  configuration whose purpose is `orchestration-health` is never verdict-
  eligible, whatever the case's maturity (bm-2d, bm-2dc, bm-2d-og today).
  `acceptanceOnly` is a configuration launch-cadence flag and says nothing
  about scores.

## 4. Layout

```
benchmark/
  cases/taskrun/0.1/       case.json  spec.md  seed/  grader/  gate.sh  guard-deps.sh
  cases/taskrun/0.2/       (same, with the count gate and its case.json)
  configurations/<configId>/<configVersion>.json   e.g. cheap/1.json, devin-host/1.json
  aliases.json             legacy ids -> pair (+ legacy version label, note)
  versions.lock            registry: case@version -> caseTree, config@version -> configTree
  schemas/case.schema.json configuration.schema.json aliases.schema.json (+ identity, scorecard, proposal, cohort as today, versioned)
  runs/                    trials root default; one directory per run
  results/cohorts/…        records carry the pair AND, for legacy, the legacy id
```

Adding a task = one directory. Adding a way of running = one file. Changing a
task's instruments = a new case version directory; changing a roster or fence
= a new configuration version file.

`benchmark/versions.lock` is the **version registry**: one line per
`caseId@caseVersion → caseTree` and `configId@configVersion → configTree`,
appended (never rewritten) when a version is added. `validate-kit` enforces
BOTH halves of that: (a) every registered id's current tree/blob equals its
registered value ("edited in place — add a new version"); (b) append-only
across history — it walks `git log -- benchmark/versions.lock`, collects
every `id → object` pair the file has ever contained, and refuses HEAD when
any id now maps to a different object than it ever did (r7 critique K-1: a
commit that edits an object AND rewrites its registry line is refused by the
history check). The walk is only meaningful over COMPLETE history, so the
check first requires it: a shallow clone (`git rev-parse
--is-shallow-repository` = true) or a graft/replace-shortened history makes
`validate-kit` REFUSE with the remedy (`git fetch --unshallow`), never
certify (r8 critique L-1); the README's another-machine procedure states the
full-history prerequisite up front. Provisioning refuses to run a version whose HEAD object
differs from the registry (r6 critique J-3). This is what makes "immutable"
enforceable without a self-referential digest: the record lives beside the
objects, and its own history is part of the check. (Rewriting published
history is a human-reserved act elsewhere in this repository; the kit does
not defend against it.) No copies across
configurations, no `fixtureSpec`.

## 5. Field projection: every current manifest key, its owner, its output

Legend: C = case.json, K = configuration json, → = where the join writes it.

| today's manifest JSON pointer | owner | output |
|---|---|---|
| `id` | C `id` (caseId) + K `id` (configId) | identity, cohort id, mission id `<caseId>`, tag |
| `version` | C `version` (a real instrument version) + K `version` | identity |
| `title` | C `title` (the task) + K `title` (the way of running) | contract intent line = `<case.title> — <config.title>`; the equivalence fixture lists title as a permitted difference (legacy titles were hand-composed per spec) |
| `product`, `language`, `seededGap`, `metrics*`, `noiseFloors*`, `watches`, `deferredMetrics`, `completionGate` | C | extractor (metrics, noise), contract (`completionGate.command` as today) |
| `comparisonEligible*` | C | eligibility (§3), with K `purpose` |
| `acceptanceOnly` | K (`acceptanceOnly` block) | cadence only |
| `machineConstraint` | K `machineConstraint` (roster-imposed) — and C `needs.os` only if the TASK demands an OS | provision refuses when intersection excludes the running machine |
| `seed`, `grader` (+`calibration`) | C | copy from the case version directory |
| `roster.*` (host, delegates, delegateRoles, independence, acceptableEffective, ruling) | K | contract `host.*`, `metasystem.conf` role lines, extractor aliases |
| `rosterNote`, `roster.independenceReason` (bm-2d-og) | K `notes.roster`, K `roster.independenceReason` | none (kept verbatim) |
| `fences.*` incl. `ledger*Budget`, `hostTurnCapMin`, `approved`; `fencesNote` | K `fences`, K `notes.fences` | contract `fence.*`, `ledger.*`, `host.turn-cap-min` |
| `fences.hostMaxTurns`, `fences.hostMaxBudgetUsd` (bm-1 only) | K `hostCaps` (OPTIONAL) | contract `host.max-turns`, `host.max-budget-usd` when present; absent → not written, seal warns (today's behaviour). Legacy: only `cheap` carries 150 / 5.00; the other five have none — nothing is synthesized |
| `fences.acceptBinaryGateFuse` (bm-1) | K `fences.acceptBinaryGateFuse` | contract `ledger.accept-binary-gate-fuse` |
| `environment.delegateNetwork(+Reason)` | K `environment.delegateNetwork` = the ALLOWANCE (`allowed`/`denied`); the task-side statement moves to C `needs.network` = `required`/`forbidden`/`either` with C `needs.networkNote` (bm-2d's reasoning is about the TASK — "no published task runner satisfies it" — so it lives with the task) | effective = the configuration's allowance, admitted only if compatible (§6); `metasystem.conf dispatch.permissions.network` |
| `environment.denyMechanism`, `environment.maven` (task-environment prose in every manifest) | C `needs.environmentNotes` (verbatim) | none; documentation only |
| `missionContract.gate/guard/truth/streams/instruments(+Note)/note` | C `mission` | contract as today; instrument files come from the case version dir |
| `missionContract.envelope` | C `needs.dependencies` (task) — today the only category | contract `envelope.dependencies` |
| `missionContract.exposure`, `exposureNote` | K `exposure.statement`, K `exposure.note` (roster-specific, verbatim) | contract `exposure=` |
| `fixtureSpec` (bm-2d-og) | removed | — |
| `roster.delegateRoles`, `roster.independence` (added 2026-08-18) | K roster | as today |

Rule checked by the kit: `case.mission.streams` text must name the gate's
metric and threshold consistently (bm-2s's stream still says
"self-assessment=1" against a `>=26` gate — fixed in `taskrun@0.2`).

## 6. Compatibility (refused at provision, checked in validate-kit for every alias pair)

| case `needs` | configuration | rule |
|---|---|---|
| `network: required` | `delegateNetwork: denied` | refuse |
| `network: forbidden` | `delegateNetwork: allowed` | refuse |
| `network: either` | any | effective = configuration's value |
| `os: linux/darwin` | `machineConstraint.os` | intersection must be non-empty AND contain the running machine |
| `minFences.<f>` | `fences.<f>` | `fences.<f> >= minFences.<f>` for each declared floor |
| — | roster | implementer/critic effective models differ, or `independence=session-only` (issue #7's rule) |
| — | fences | binary gate: `ledgerNoGainBudget >= cycles` or `acceptBinaryGateFuse` (issues #4/#8). This is the SEAL-time rule of the contract validator; provisioning does not refuse on it (as today) and `validate-kit` reports it as a named WARNING per configuration. Three inherited configurations (`devin-delegate@1`, `devin-host@1`, `devin-host-claude-delegate@1`: no-gain 5, cycles 8, no acknowledgement) are reproduced verbatim and are therefore UNSEALABLE until a human rules (issue #8's family); the README says so beside them. Nothing is synthesized. |
| — | hostCaps | optional; when present, positive integers/decimals |

## 7. Identity, evidence, results, proposals

- `benchmark-identity.json` **schemaVersion 2**: `caseId`, `caseVersion`,
  `configId`, `configVersion`, `caseTree`, `configTree` replace `benchmarkSpecId/Version`; for runs
  provisioned through an alias, `legacyId` and `legacyVersionLabel` are also
  written. Readers accept schemaVersion 1 by resolving `benchmarkSpecId` through
  `aliases.json` and reporting `legacyId` = the old id.
- Cohort record and resume state carry the four fields (and `specId` for
  legacy states). Cohort ids: `<caseId>-<configId>-<stamp>-<pid>`. **Resume of a
  pre-migration cohort keeps working, on any machine**: its state carries only
  `specId`; resume resolves it through `aliases.json` to a pair, derives the
  grader from the resolved case version directory (`cases/<id>/<v>/grader/`),
  and keeps the mission id and contract path exactly as the state and target
  already record them (`mission-<specId>.contract.md` — read, never re-derived).
  The instruments a legacy target runs are the ones sealed in its contract.
  Migration step 3 (delete `specs/`) therefore needs no drain: nothing resumes
  from `specs/` once resume resolves aliases. A fixture resumes a synthetic
  schema-1 cohort state after `specs/` is gone.
- Mission id for runs provisioned by pair = `<caseId>` (`mission-taskrun.contract.md`).
  Runs provisioned THROUGH AN ALIAS (`--spec bm-1`, or a legacy cohort's
  next repetition) keep the legacy naming end to end — mission id `bm-1`,
  contract `mission-bm-1.contract.md`, tag `bm-1-instruments-v0.1`, identity
  `legacyId=bm-1` — so a cohort begun before the migration is uniform across
  all its repetitions (r5 critique I-2). Alias mode is legacy mode; it adds
  the pair and the pins to the identity, changes nothing else.
- Instrument tag = `<caseId>-instruments-v<caseVersion>` for pair-provisioned
  runs; alias mode keeps `<legacyId>-instruments-v<legacyVersionLabel>` (r2 critique G-5:
  no digest in the ref — the sealed contract already records
  `sealed.gate-integrity.sha256` over the instrument bytes, and a case version
  is one immutable bundle). `caseId` matches `^[a-z0-9][a-z0-9-]{0,31}$` and
  `caseVersion` `^[0-9]+(\.[0-9]+){0,3}$` with at most 16 characters
  (schema-enforced; likewise `configId` and `configVersion`), so the tag is at
  most 32+14+16 = 62 characters and a valid ref component (r4 critique H-5). Tags are per target; legacy targets keep theirs.
- **Proposals** (evolution): schemaVersion 2, canonical structured form
  `benchmarks: [{"case":"taskrun","caseVersion":"0.1","config":"cheap","configVersion":"1"}]`;
  `compare.py` tests pair membership against the proposal, then evaluates
  eligibility, roster and fences from the RUN's pinned objects (`caseTree`,
  `configTree` in the scorecard/cohort identity), never from current paths
  (r6 critique J-2); the registry (below) guarantees the named
  `case@version` / `config@version` still denote those objects. Compare
  results: schemaVersion 2 with the pair. schemaVersion-1 proposals (`specs`
  list) are read through the alias table. `evolution-fixtures.sh` covers:
  eligible pair, ineligible (health config), pair mismatch, kit-version
  mismatch, ancestry, legacy proposal read.
- Scorecard identity block gains the four fields (+legacy when present).

## 8. CLI grammar

```
benchmark/provision.sh   --case taskrun@0.1 --config cheap@1 --target <dir-or-name>
benchmark/run-cohort.sh  --case taskrun@0.1 --config cheap@1 --repetitions 3 [--proposal id]
benchmark/extract.sh     <run> --case taskrun@0.1 --config cheap@1 --out scorecard.json
benchmark/grade.sh       <run> --case taskrun@0.1
benchmark/compare.sh     … (verdict, as today) | --configurations (report)
benchmark/provision.sh   --spec bm-1 …   → resolves through aliases.json, prints
                          "bm-1 = taskrun@0.1 under cheap@1 (alias; use --case/--config)"
```
`--case` REQUIRES `@version` everywhere (r2 critique G-6): a cohort is durable
evidence and a proposal names an explicit version, so nothing may drift to
"newest" silently; an unpinned `--case` is refused with the list of available
versions. `--config` likewise REQUIRES `@version` (`--config cheap@1`). Ids resolve under `cases/` and `configurations/`; paths are honoured.

## 9. Migration (each step its own gated commit)

1. Add `cases/taskrun/0.1` (from bm-1's files) and `cases/taskrun/0.2` (bm-2s's
   files; stream text corrected), the six configuration files, `aliases.json`,
   the new schemas (§10), and the kit checks (§6 table, stream/gate agreement,
   schema validation). Nothing removed yet.
2. Teach `provision.sh`, `run-cohort.sh`, `extract.sh`/`extractor.py`,
   `grade.sh`, `compare.py`, `validate-kit.sh` the pair; identity v2; `--spec`
   through aliases. Byte-equivalence fixture: for each alias, provision the
   legacy spec and the pair into two targets and diff `metasystem.conf`, the
   contract and the instrument bytes. The ONE permitted-difference list, used
   by the fixture and stated in the README: mission id (`bm-1` → `taskrun`),
   the contract file name that carries it, the instrument tag
   (`bm-1-instruments-v0.1` → `taskrun-instruments-v0.1`), the contract intent
   line (`<case.title> — <config.title>` replaces the hand-composed legacy
   title), absolute paths (target, evidence root), and the identity block's
   new fields. Everything else must be byte-identical (r4 critique H-4).
3. After the in-flight check: delete `specs/` and the `fixtureSpec` code.
4. Documentation (§10). Decision record D-next in the delegated-decisions
   review; `development/evidence-index.md` gains the pair for archived runs.

## 10. Documentation (normative, so another machine needs no other source)

`benchmark/README.md` gets: the three nouns and the alias rule; the layout;
**annotated `case.json` and configuration examples** with every field's
required/optional status and owner (the projection table of §5, condensed);
version-bump rules for both objects and the immutability check; the
compatibility table of §6; the join (what the contract and `metasystem.conf`
receive from which half); mission id and instrument tag rules; the CLI grammar
of §8; comparability and eligibility of §3; the alias table's read-only
status; **procedures**: "add a case", "add a case version", "add a
configuration", "run one under the other", "compare", "resume a cohort begun
before the migration (alias mode)", "read the exact bytes a run used" (the
`git cat-file` recipes over `caseTree`/`configTree`, and — when an object is
unreachable in the local clone — fetch the run's recorded
`measuringMetasystemSha` from the origin (`git fetch origin <sha>`), whose tree
contains that case version and configuration file, then retry; r7 critique
K-2), each ending with the validation command that proves it
(`benchmark/validate-kit.sh`). JSON schemas
in `benchmark/schemas/` are the machine-checked half of the same contract and
the README links each: `case.schema.json`, `configuration.schema.json`,
`aliases.schema.json` (new), `benchmark-identity.schema.json` (v2),
`scorecard.schema.json` (identity block v2), `proposal.schema.json` and
`cohort.schema.json` (new: today's proposal and cohort records had no schema),
`compare-result.schema.json` (new, v2). `validate-kit.sh` validates every
shipped case, configuration, alias table and fixture proposal against them.
Annotated examples in the README cover the case, the configuration AND a
proposal. `metasystem/docs/` mentions of "benchmark spec" become
"benchmark case/configuration"; the glossary gains the four terms.

## 11. Decisions taken in r2 (were open in r1)

- Fences belong to the configuration; no third "profile" object (add only if
  fences ever vary independently of rosters).
- `purpose` is configuration; `comparisonEligible` is case; eligibility is
  their conjunction (§3).
- Case versions: `taskrun@0.1` = the boolean gate (bm-1, bm-2, bm-2d, bm-2dc,
  bm-2d-og — the last four were mislabelled 0.2; the alias table records the
  label they carried); `taskrun@0.2` = the count gate (bm-2s). No relabelling
  of archived evidence.
- Host caps optional; nothing synthesized for legacy configurations.
- Network split into task requirement and configuration allowance (§5, §6).
- Configuration ids: `cheap`, `sol`, `devin-delegate`, `devin-host`,
  `devin-host-claude-delegate`, `devin-opus-gpt55` (open to renaming; ids are
  descriptive of the roster, never of history). All start at version 1.
- Case and configuration versions are always pinned on the command line and
  in every record; there is no "newest" resolution anywhere.
- Aliases stay forever, read-only.

## 12. Acceptance

- `validate-kit.sh` green with: two case versions, six configurations, all six
  aliases; the byte-equivalence fixture passes for every alias under the one
  permitted-difference list; every shipped case/configuration/alias validates
  against its schema; the registry matches every version's object id and an
  in-place edit is refused (fixture); a rewritten registry line is refused by
  the history walk (fixture: a throwaway clone with an object+lock rewrite
  committed on top); a SHALLOW or graft-shortened clone is refused with the
  unshallow remedy (fixtures: `git clone --depth 1` of the kit, and a clone
  with a `git replace --graft`, each running `validate-kit` and expecting the
  refusal; r9 critique M-1); §6 refusals each have a negative fixture; a
  dirty case directory is refused by provisioning (fixture).
- `run-cohort.sh --case taskrun@0.1 --config cheap@1` provisions; an unpinned
  `--case taskrun` is refused with the available versions; cohort record and
  identity carry the pair and `caseTree`; a legacy resume state still resumes.
- `extract.sh` on an archived run (bm1-trial-001) extracts with `legacyId`
  reported and the pair resolved through the alias table.
- `compare.py` verdicts unchanged in strictness; `--configurations` report
  exists; evolution fixtures cover the six paths in §7.
- README passes the "another machine" test: an agent can add a configuration
  and a case version from the README and the schemas alone.
