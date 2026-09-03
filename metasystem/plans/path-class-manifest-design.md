# Path-class manifest: design (revision 2)

Goal: path-class-manifest (`plans/goals/path-class-manifest.md:4`, Wido's order verbatim;
authority `memory/rulings.md:102` R-55-m1). One manifest, one verb, three consumers, three
deletions, one refusal, fixtures. Revision 2 folds the thirteen material findings and four
gaps of `records/misc/path-class-manifest-critique-r1.md`; section 9 maps each.

## 0. The tree as it is (facts the design stands on)

- F1. The evaluator judges INSTALLATION-relative paths: `commit.sh` roots itself at the
  metasystem directory (`scripts/agents/commit.sh:4`) and hands the prefix subtree as the
  landing tree (`:292-296`); gittree paths are workspace-relative
  (`internal/gittree/gittree.go:171-176`). `development/` and the toplevel `.claude/` never reach it.
- F2. The floor is the hard-coded table `neverDirectFix` (`internal/landing/observe.go:795-816`),
  called from register carriage (`:469`) and the exact-revert floor scan (`:780-790`).
- F3. Register carriage reads `scripts/agents/register-carriage-paths.txt` through
  `loadCarriagePolicy` (`observe.go:491-500`, parse `:542-568`, match `:570-584`), then applies
  append-only rules to three named files (`:476-487`, `:586-645`); `landing-classes.json:7`
  names the allowlist as the carriage `pathRule` (`:523-526`).
- F4. The waiver rule reads `scripts/agents/instruction-bearing-paths.txt` (`internal/validate/conformance.go:519-566`),
  unions the registry's declared instruction files (`:545-550`), and refuses a prose-under-30
  waiver touching a hit (`:627-637`) in project space (`:581-591`).
- F5. Goal-verb commits never meet the evaluator (`internal/goal/txn.go:283-291`, `:347-360`);
  the ledger is `plans/goals/` and `records/goals/` (`internal/goal/validate.go:31-35`).
- F6. `path owner` knows template from adopted mode (`internal/stateroot/owner.go:38-40`) and
  in adopted mode calls everything outside the vendored installation the application's (`:83-96`).
- F7. The window (`git log 4a351338..HEAD`, 175 Machine-stamped landings): paths by top
  directory plans 174, records 109, memory 6, scripts 1. Two EXISTING `records/` files were
  modified in place; NO landing modified or deleted a root `plans/` file bound to no open
  goal, and NO landing modified another seat's handoff.
- F8. `Goal-Item:` is a house convention in commit bodies; nothing produces or reads it. The
  `Machine:` trailer IS stamped by the wrapper as `<nickname>+<lineage>` (`commit.sh:360-363`,
  lineage from `METASYSTEM_OWNER_LINEAGE`, default `human`).
- F9. `scripts/agents/land.sh`, the normal caller, rejects unknown options (`:71-75`) and forwards
  no goal (`:244-253`); `commit.sh` reads `machine_nickname` after the evaluator call (`:297-306`, `:360`).
- F10. A claimed goal file carries `- Claimed: machine=<m> lineage=<l> …`
  (`internal/goal/file.go:76-84`, `:768`); `goal.ParseFile` parses bytes (`file.go:206`);
  `validId` (`goal.go:409`) is the id grammar; `internal/goal` does not import `internal/landing`.
- F11. Goal ids are not prefix-free (`codex-handshake-budget` prefixes `codex-handshake-budget-load-fragile`).
  Root `plans/*.md` at 878522b5: 207 files; README 1, handoffs 5, bound by name to an open
  goal 49, unbound 152; 108 more files under `plans/` subdirectories other than `goals/`.
- F12. `plans/goals.md` and `plans/goals-accepted.json` are `LedgerPath`/`BaselinePath` (`internal/goal/goal.go:122-129`),
  seeded by `scripts/adopt.sh:298,327-342`, deleted by `migrate.go:283-288`; absent here, present on a fresh adoption.

## 1. The manifest

**File:** `scripts/agents/path-classes.txt`, plain text, committed (same directory and line
grammar as the lists it replaces, F3, F4).

**Grammar.** One row per line, `#` comments. Three row kinds:
- `install:<path> <class>`: installation-relative keys, the space the evaluator judges (F1).
  `install:metasystem runtime` is the built binary; the tracked directory `metasystem/` IS
  the installation in the template, so it is never a key.
- `repo:<path> <class>`: repository-relative keys OUTSIDE the installation, consulted only in template mode (F6).
- `own:<path> <goal-id>`: the ownership section (section 5); `<path>` is an exact `install:`
  file under `plans/`, `<goal-id>` satisfies `validId`. Initially empty.

A path ending in `/` is a directory prefix; otherwise an exact file. No globs. Class is
`behavior`, `record`, `ledger` or `runtime`. An unknown row kind, a duplicate key within one
kind, an unknown class, an absolute path, a `..` segment or an invalid goal id make the
manifest unreadable; every landing then refuses (`register-carriage-policy-unreadable`, or
`direct-fix-policy-unreadable` on the exact-revert arm).

**Matching.** Within one key space the longest matching prefix wins; a file row matches only
itself; a directory row matches itself and its subtree. Because a directory row covers its
subtree, an unclassified key never has a classified ancestor: the refusal text has one form
(section 2). **Absent-but-named paths:** a row may name a path absent from the tree and
classifies it from the moment it appears; four rows are of this kind today (`go.work`,
`go.work.sum`, `plans/goals.md`, `plans/goals-accepted.json`).

**Resolution.** The evaluator classifies `install:` keys from the LANDING BASE tree
(`workspace.FileAt(baseTree, …)` as `observe.go:495`), so a candidate cannot reclassify what
it lands (`promotion.go:22-24`). In template mode a key with no row is `unclassified`; in
adopted mode it is the application's and answers `outside` (R-55-m1 binds the template; an
adopted installation's own additions keep today's rules). Ownership decides first in every
consumer; in the root layout (the installation is the repository root) ownership follows the
shipped inventory in `internal/stateroot/owner.go`. In this part the inventory names the
instruction-bearing files and the trees adoption creates: `AGENTS.md`, `CLAUDE.md`, `wow.md`,
`metasystem.conf`, `go.mod`, `go.sum`, `docs/project-rules.md` and the six named docs files, and
the trees `cmd/`, `internal/`, `scripts/`, `skills/`, `optional-skills/`, `docs/design/`,
`docs/examples/`, `memory/`, `plans/`, `records/` and `.github/`. The other docs files adoption
copies and the runtime registration directories answer application-owned in the root layout,
as they did before this feature; making the inventory equal adoption's full install set is
goal `adoption-inventory-from-install-set`, which reads the set adoption installs instead of a
hand list (revision 2, corrections of 2026-09-03 after PCM-CC4-001, PCM-CC5-001 and PCM-CC6-001). The verb and the waiver rule read
the checked-out file: outside the repository, `outside`; inside the installation, the
`install:` key; outside the installation, the `repo:` key in template mode and `outside` in
adopted mode.

**Initial table**, one row per name, in this order:
- `install:` behavior: `AGENTS.md`, `CLAUDE.md`, `wow.md`, `README.md`, `metasystem.conf`,
  `go.mod`, `go.sum`, `go.work`, `go.work.sum`, `.gitattributes`, `.gitignore`, `cmd/`,
  `internal/`, `docs/`, `scripts/`, `skills/`, `optional-skills/`, `.claude/`, `.codex/`,
  `.agents/`, `.devin/`, `.github/`, `memory/README.md`, `plans/README.md`, `records/README.md`.
- `install:` record: `memory/`, `plans/`, `records/`.
- `install:` ledger: `plans/goals/`, `plans/goals.md`, `plans/goals-accepted.json`, `records/goals/`.
- `install:` runtime: `artifacts/`, `bin/`, `metasystem`, `metasystem.conf.local`.
- `repo:` behavior: `README.md`, `LICENSE`, `.gitignore`, `.claude/`, `benchmark/`,
  `environment/`, `development/README.md`, `development/project-rules-local.md`,
  `development/devin-selftest.md` (the three instruction-bearing files of `development/`,
  read 2026-09-03: the tree's rules, this repository's invariants, an operator runbook).
- `repo:` record: `development/` (the other ten files: reports, retained critiques, the
  evidence index, historical designs and analyses).

`docs/journey.md` and `docs/reviews/` accrete but stay behavior (`plans/memory-architecture-design.md:17`).
The manifest lives under `scripts/`, so it is behavior; a section 7 fixture pins that.

## 2. The verb

`metasystem path class <path>` beside `path owner` (`cmd/metasystem/main.go:244-247`,
`path_verbs.go:11-30`), implemented by a new package `internal/pathclass` (loader, resolver,
refusal text) that all consumers share. Stdout one word: `behavior`, `record`, `ledger`,
`runtime` (exit 0); `unclassified` (exit 1, refusal text on stderr); `outside` (exit 1:
outside the repository, or the application's own path in adopted mode). The one refusal
text, verbatim:

```
path <key> has no class in scripts/agents/path-classes.txt; no classified ancestor; add a row for <key> or its directory to scripts/agents/path-classes.txt
```

`--explain` prints `<class> row=<matched row> key=<kind>:<key> mode=<template|adopted>`.

## 3. The three consumers

**(a) The landing evaluator** (`internal/landing/observe.go`). Every changed path is
classified once, over the whole set, before any class rule runs (floor precedence stays
set-wide, `records/two-bars/floor-verdict-addendum.md:40-41`).

| class | `--chain` (observeChain `:106-153`) | `--direct-fix register-carriage` (`:449-489`) | `--direct-fix exact-revert` (`:721-790`) |
|---|---|---|---|
| behavior | pass when certified (unchanged) | `direct-fix-floor-refused` | `direct-fix-floor-refused` (target-or-candidate union) |
| record | pass when certified; an extra path takes the carriage rules | section 5 | `exact-revert-record-refused`: an exact inverse deletes a new record or truncates an append |
| ledger | `ledger-path-not-goal-verb` | same | same |
| runtime | `runtime-path-refused` | same | same |
| unclassified | `path-unclassified` | same | same |
| outside (adopted only) | as today: pass certified, `chain-has-uncarried-paths` extra | `register-carriage-path-refused` (today's meaning, addendum `:53-54`) | pass when exact |

Ledger paths refuse because no landing the evaluator sees is a goal verb (F5). In the template,
exact revert has no lawful target; the class stays for adopted applications and as the floor's second arm.

Functions that change:
- `registerCarriage(root, candidateTree, changedPaths, goal, actor) error` (`:449`): loads the
  manifest, classifies every path, returns a `carriageError` per the table, floor first;
  record paths take section 5.
- `loadCarriagePolicy` (`:491-500`) becomes `loadPathClasses(workspace, baseTree)`;
  `parseCarriageAllowlist` (`:542-568`) and `carriagePathAllowed` (`:570-584`) are deleted;
  `loadLandingClasses` (`:523-526`) expects `path-class-record`, which `landing-classes.json:7` changes to.
- `exactRevertFloorError` (`:780-790`) becomes `exactRevertClassError` (the table's last
  column); `not-exact-revert` stays the fallback (`:718`).
- `observeChain`: after `bindCertifiedChange` (`:135`), classify certified paths plus
  extras; ledger, runtime and unclassified refuse before `:143`.
- `ObserveParams` (`:39-45`) gains `Goal` and `Actor` (`<nickname>+<lineage>`, the Machine
  trailer's exact value, F8). A set `Goal` is validated before any path rule:
  `plans/goals/<goal>.md` at the base must parse (`goal.ParseFile`, F10) with state claimed
  and `Claimed` machine and lineage equal to `Actor`; otherwise `goal-item-not-held`.
- `Observation` (`:49-57`) gains `unclassified` (JSON array of offending keys, from the BASE
  manifest) and `refusal` (the section 2 text, one line per key). `landing observe`
  (`cmd/metasystem/landing_verbs.go`) gains `--goal` and `--actor`. `knownRefusalCode`
  (`promotion.go:90-117`) gains the six new codes of section 6.
- `commit.sh`: the nickname block (`:356-362`) moves before the evaluator call (`:297`);
  `--goal <id>` beside `--chain` (`:40-76`), checked against the `validId` grammar (exit 2),
  forwarded as `--goal` with `--actor "${machine_nickname}+${METASYSTEM_OWNER_LINEAGE:-human}"`,
  and stamped as `Goal-Item: <id>` beside `Machine` (`:363`) from one variable. Goal-Item is
  wrapper-owned: the wrapper scans `-m`, `--message[=]`, `--trailer[=]` values and `-F`/`--file[=]`
  file contents for a line matching `^Goal-Item:` and refuses a hit (exit 2, "Goal-Item is
  stamped by --goal, never typed"); `-F -`, `--file=-`, `-c`, `-C`, `--reuse-message`, `--reedit-message`,
  `-t`, `--template`, `--amend`, `--squash`, `--fixup` refuse as unscannable message sources (no
  caller in the tree uses them). The refusal switch (`:322-339`) gains one branch per new code;
  `path-unclassified` prints the Observation's `refusal` field through `json get`; the refusal path never calls the verb.
- `land.sh`: `--goal <id>` (once, F9) in the option loop and usage, forwarded in
  `commit_changes` (`:244-253`).

**(b) The critique-waiver rule** (`conformance.go:519-566`). The list read and
`isInstructionBearing` become `pathclass.Load(r.root).Class(path)`; the waiver refuses when
any path is `behavior`, `ledger`, `runtime` or `unclassified` (`:627-637` keeps its shape,
message "prose-under-30 touches a path that is never waivable"); `record` and `outside` stay
waivable. The registry union (`:545-550`) becomes an invariant: every
`runtimes.InstructionFiles()` entry, as an `install:` key, must classify `behavior`, else
`conformance failure: runtime instruction file <path> has manifest class <class>, not
behavior`. A new runtime lands its row in the same reviewed chain.

**(c) Register carriage.** Its allowlist becomes "every record path", (a)'s record column; no separate file or function.

## 4. The deletions

| deleted | readers today, all rewritten or removed in slice 1 |
|---|---|
| `neverDirectFix` (`observe.go:792-816`) | `observe.go:469`, `:782`; comment `:792-794` |
| `scripts/agents/register-carriage-paths.txt` | `observe.go:495`; `observe_test.go:29` (fixture copy), `:478`; `scripts/agents/static-reproof-fixtures.sh:24`; `landing-classes.json:7` |
| `scripts/agents/instruction-bearing-paths.txt` | `conformance.go:519`; `internal/validate/instructionowners_test.go:15,39`; `nested_conformance_test.go:233`; `conformance_test.go:53-54`; `scripts/agents/conformance-fixtures.sh:34`; `scripts/validate-metasystem.sh:1003` |

History (`docs/reviews/`, `records/`, `plans/`) stays as written; "instruction-bearing" at
`internal/runtimes/runtimes.go:62` names a concept. The G-5 lint (`instructionowners_test.go:12-18`)
is kept as a manifest check: every rule-owning document classifies as behavior.

## 5. Record semantics, stated once

Applied by the evaluator to record paths under register carriage; rules go by location, first
matching row wins. "Held" means `Goal-Item` is set and validated as in section 3: the actor holds that goal at the base.

| record files | rule |
|---|---|
| `memory/receipts.log`, `records/narrator-digest.log` | append-only (`appendOnly`, `observe.go:586-621`) |
| `memory/rulings.md` | append-only rows (`addRulingRowsOnly`, `:623-645`) |
| `plans/handoff-<seat>-*.md` | new, or modified when `<seat>` equals the actor's nickname (before `+`); else `record-not-owned` |
| `plans/<goal-id>-*.md` at the root of `plans/`, `<goal-id>` being the LONGEST complete id of a file in `plans/goals/` at the base that is a prefix of the filename followed by `-` (F11 tie-break); or any file named by an `own:` row | new or modified when held and `Goal-Item` equals that id; else `record-not-owned` |
| any other EXISTING file under `plans/` | frozen: modified or deleted refuses `record-not-owned`; changes only through a reviewed chain or an `own:` row |
| `memory/` (other than the rows above and `README.md`), `development/` record part | shared register: new, or modified when held; else `record-not-owned` (the evaluator never meets `development/`, F1) |
| every other record path (`records/` except `records/goals/`, new files under `plans/`) | new file; an existing file may be appended to when held; replacement or deletion refuses `register-carriage-not-append-only`; a missing owner refuses `record-not-owned` (`records/README.md:3-6`; the append rule is the closing design review's obligation PCM-R2-002, stated here 2026-09-03) |

**Migration note (one-time, not a build task).** At 878522b5 the frozen set is the 152 root
`plans/*.md` files that are not `README.md`, not `handoff-*`, and do not begin with an open
goal id plus `-`, plus the 108 files under `plans/` subdirectories other than `goals/` (F11).
No window landing touched them (F7). A seat that needs one adds an `own:` row through a
reviewed chain; concluded designs move to `records/` by that same chain.

## 6. The fail-closed code set and what this slice promotes

Promotion is one string per code in `refuseCodes` (`scripts/agents/landing-promotion.json:3`, `applyPromotion`,
`promotion.go:36-38`) plus one ruling row as R-40-m0 was (`memory/rulings.md:88`). The complete set the evaluator can return:

- PROMOTED in slice 2 under R-55-m1 ("a path with no class is refused"), because this design
  introduces them or they are the manifest's own failure: `path-unclassified`, `ledger-path-not-goal-verb`,
  `runtime-path-refused`, `exact-revert-record-refused`, `goal-item-not-held`, `record-not-owned`,
  `register-carriage-policy-unreadable`, `direct-fix-policy-unreadable` (both: the one manifest
  unreadable at the base), `register-carriage-not-append-only`. `record-not-owned` is promoted
  now, not observed: the frozen and handoff rules would have hit zero window landings (F7)
  and its inputs land in the same slice.
- Already promoted by R-40-m0: `missing-declaration`, `conflicting-declarations`.
- OBSERVED, promoted afterwards by the one-line-plus-ruling step the goal names:
  `direct-fix-floor-refused` (now exactly: a carriage or revert landing changed a behavior path).
- OBSERVED, unchanged in meaning and outside this goal's order (R-40-m0's narrow promotion
  stands): `malformed-candidate-tree`, `candidate-tree-unreadable`, the nine chain codes
  `malformed-chain-id` through `chain-has-uncarried-paths` (`promotion.go:96-104`), `register-carriage-path-refused`
  (adopted payload under carriage), `malformed-revert-commit`, `not-exact-revert`, `unknown-direct-fix-class`.
- Wrapper-only, already refusing for agent commits (`commit.sh:323-334`):
  `evaluator-unavailable`, `promotion-base-unreadable`, `promotion-record-malformed`.

## 7. Fixtures, deterministic

Go, beside `observe.go` (style of `observe_test.go:124`, `:277`, `:476`):
- `TestObserveClassifiesEachPathClass`: under `register-carriage`, `internal/x.go` refuses
  `direct-fix-floor-refused`; `plans/handoff-fixture-1.md` (new) passes; `plans/goals/x.md`
  refuses `ledger-path-not-goal-verb`; `bin/metasystem` refuses `runtime-path-refused`;
  `product.txt` refuses `path-unclassified` with `unclassified=["product.txt"]` and the section 2 text.
- `TestObserveChainRefusesLedgerRuntimeAndUnclassifiedPaths`: the same three as certified
  paths of a valid closed chain (`observe_test.go:124-140`).
- `TestObserveExactRevertRefusesByClass`: five legs, one per class, each asserting its column code; an adopted-mode `outside` inverse passes.
- `TestObserveRecordSemantics`: digest rewrite refuses; modified `records/misc/x.md` refuses
  `record-not-owned`; `plans/fx-design.md` modified with goal `fx`, actor `m9+L1`, base goal
  claimed `machine=m9 lineage=L1` passes; actor `m1+L2` refuses `goal-item-not-held` (cross-seat);
  goal `other` refuses `record-not-owned`; `plans/fx-load-x.md` with goals `fx` and `fx-load` at
  base is owned by `fx-load` (tie-break); `plans/legacy.md` modified refuses (frozen) and passes once
  `own:plans/legacy.md fx` is in the base manifest; `plans/handoff-m9-x.md` modified with actor `m9+L1` passes, `m1+L1` refuses.
- `TestObserveUnclassifiedDetailFromBase`: the candidate lands only `product.txt`, a path the base
  manifest does not classify, while the checked-out manifest outside the candidate is altered to add
  `install:product.txt record`; the verdict is `path-unclassified` and `refusal` names `product.txt`,
  which proves classification reads the landing base and never the working tree. (Settled 2026-09-03
  at the second part's build: the earlier shape edited the manifest inside the candidate, which the
  floor refuses first, because precedence is set-wide and a behavior path anywhere in the candidate
  refuses `direct-fix-floor-refused` before the ledger, runtime and unclassified checks run.)
- `TestObserveManifestIsBehavior`: editing `path-classes.txt` under carriage refuses `direct-fix-floor-refused` (replaces `:476-484`).
- `internal/pathclass`: `TestLongestPrefixWins`, `TestRowKindsAreDistinctKeySpaces`
  (`install:metasystem` runtime, `metasystem/cmd/x.go` behavior in template mode),
  `TestAdoptedModeAnswersOutside`, `TestManifestRejectsMalformedLines`,
  `TestAbsentNamedPathsClassify` (the four rows of section 1),
  `TestRepositoryManifestClassifiesEveryTrackedPath` (walks `git ls-files` of the
  installation and, in template mode, of the repository; any unclassified path fails, naming it).
- `internal/validate`: `TestMergeWaiverRefusesBehaviorPath`, `TestMergeWaiverRefusesUnclassifiedPath`;
  `TestConformanceProtectsDeclaredInstructionFile` (`conformance_test.go:372-383`) gains a
  `plans/NEWRT.md` declaration that fails with the section 3(b) message;
  `TestInstructionOwnersAreBehavior` replaces `instructionowners_test.go`.
- `cmd/metasystem`: `TestPathClassVerbOneWordAndRefusalText` asserts the exact section 2 line for `product.txt`.

Shell, `scripts/agents/path-class-fixtures.sh`, registered beside `static-reproof-fixtures.sh`
(`validate-metasystem.sh:977`, `:1069`):
- `TestPathClassVerbAnswersFromManifest`: four words for four paths, the exact refusal line,
  `outside` for a path outside the repository.
- `TestDeletedListsHaveNoReader`: `rg -n 'register-carriage-paths|instruction-bearing-paths|neverDirectFix'`
  over every existing path of a behavior row (`install:` and `repo:`) read from the manifest,
  minus `docs/reviews/` and `docs/journey.md`; prints nothing.
- `TestCommitWrapperStampsGoalItemTrailer` (extends `static-reproof-fixtures.sh:12`): `--goal fx`
  lands `Goal-Item: fx`; `--goal 'Bad Id'`, `-m` text carrying `Goal-Item: victim`,
  `--trailer 'Goal-Item: fx'` and `-F -` each exit 2.
- `TestLandForwardsGoalToEvaluator` (real engine, as `static-reproof-fixtures.sh:17-40`):
  `land.sh --goal fx` on a modified `plans/fx-note.md` with base goal claimed `machine=fx
  lineage=L`, `git config metasystem.goal.machine fx`, `METASYSTEM_OWNER_LINEAGE=L` lands
  `Goal-Item: fx` and `pass`; with lineage `other` the wrapper refuses `goal-item-not-held`.
- `scripts/agents/land-fixtures.sh`: `make_leg goal` proves the option is accepted and
  forwarded to the stubbed wrapper (`:34-80`).

## 8. Size and diff boundary

Two build slices, each under 240 reserved minutes with a correction round.

**Slice 1: manifest, verb, resolver, conformance, deletions.** The evaluator keeps today's
codes: the floor reads behavior rows, carriage eligibility is "record class and one of the
three append-only files", other paths refuse `register-carriage-path-refused` as today; no
verdict widens. Boundary: new `scripts/agents/path-classes.txt`, `internal/pathclass/pathclass.go`,
`pathclass_test.go`, `scripts/agents/path-class-fixtures.sh`, `cmd/metasystem/path_verbs_test.go`;
deleted `scripts/agents/register-carriage-paths.txt`, `scripts/agents/instruction-bearing-paths.txt`;
modified `internal/landing/observe.go`, `observe_test.go`, `internal/validate/conformance.go`,
`conformance_test.go`, `nested_conformance_test.go`, `instructionowners_test.go`, `cmd/metasystem/path_verbs.go`,
`main.go`, `scripts/agents/landing-classes.json`, `static-reproof-fixtures.sh`, `conformance-fixtures.sh`,
`scripts/validate-metasystem.sh`.

**Slice 2: class table, exact revert, ownership, wrapper inputs, promotion, end-to-end.**
Boundary: `internal/landing/observe.go`, `observe_test.go`, `promotion.go`,
`cmd/metasystem/landing_verbs.go`, `scripts/agents/commit.sh`, `land.sh`, `land-fixtures.sh`,
`landing-promotion.json`, `path-class-fixtures.sh`, `static-reproof-fixtures.sh`,
`memory/rulings.md` (one promotion row).

Out of scope: moving `docs/journey.md`, `docs/reviews/` or frozen `plans/` files; the goal engine; existing commits' `Goal-Item` prose lines.

## 9. Fold record

| id | fold (artifact) |
|---|---|
| PCM-R1-001 | ledger rows `plans/goals.md`, `plans/goals-accepted.json`; absent-but-named rule; `TestAbsentNamedPathsClassify` (sections 1, 7) |
| PCM-R1-002 | `install:`/`repo:` row kinds; adopted mode answers `outside`; resolver and two pathclass tests (sections 1, 2, 7) |
| PCM-R1-003 | three `repo:` behavior rows for `development/`, the rest record (section 1) |
| PCM-R1-004 | record under exact revert refuses `exact-revert-record-refused`; five-leg revert test (sections 3, 7) |
| PCM-R1-005 | held = base `Claimed` machine+lineage equals the wrapper actor; `goal-item-not-held`; cross-seat leg (sections 3, 5, 7) |
| PCM-R1-006, gap 2 | longest complete goal id followed by `-`; tie-break leg (sections 5, 7) |
| PCM-R1-007 | `land.sh --goal`; nickname resolved before the evaluator; `land-fixtures.sh` leg; `TestLandForwardsGoalToEvaluator` (sections 3, 7, 8) |
| PCM-R1-008 | Goal-Item wrapper-owned: grammar check, input scan, unscannable sources refused; wrapper legs (sections 3, 7) |
| PCM-R1-009 | `Observation.unclassified` and `refusal` from the base manifest; wrapper prints them, never calls the verb; base-versus-candidate test (sections 3, 7) |
| PCM-R1-010, gap 4 | complete code set with promote/observe and reason; nine codes promoted in slice 2 (section 6) |
| PCM-R1-011 | conformance invariant: declared instruction files classify behavior; test extended (sections 3b, 7) |
| PCM-R1-012, gap 1 | one refusal form with the literal sentinel; the ancestor clause is unreachable and dropped; exact text in two tests (sections 1, 2, 7) |
| PCM-R1-013 | reader search set derived from behavior rows minus the two history trees (section 7) |
| gap 3 | existing unbound `plans/` files frozen; `own:` ownership rows; one-time migration note with counts (sections 1, 5) |
| PCM-R1-N01 | two slices with exhaustive boundaries (section 8) |

## 10. Self-grade

- Confidence: high on sections 1 to 4 and 6 to 9, every cited line read in this worktree on
  2026-09-03; medium on section 5's shared-register row (`memory/` registers modifiable under
  any held goal is the loosest rule left). Weakest claim: promoting `record-not-owned` now rests on one window (F7).
- Reject condition: a record kind the evaluator must accept that section 5 refuses; a `Goal-Item`
  scan that refuses a message form a caller in the tree uses; or `--goal` breaking the proved-tree postcondition (`commit.sh:347-355`).
