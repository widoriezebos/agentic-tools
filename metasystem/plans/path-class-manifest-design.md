# Path-class manifest: design (round 1)

Goal: path-class-manifest (`plans/goals/path-class-manifest.md:4`, Wido's
order verbatim; authority `memory/rulings.md:102` R-55-m1). One manifest,
one verb, three consumers, three deletions, one refusal, fixtures.

## 0. The tree as it is (facts the design stands on)

- F1. The evaluator judges INSTALLATION-relative paths: `commit.sh` roots
  itself at the metasystem directory (`scripts/agents/commit.sh:4`), hands
  the prefix subtree as the landing tree (`:292-296`), and every path the
  gittree workspace returns is workspace-relative (`internal/gittree/gittree.go:171-176`).
  Paths outside the installation (`development/`, the toplevel `.claude/`)
  never reach the evaluator.
- F2. The floor is a hard-coded table, `neverDirectFix`
  (`internal/landing/observe.go:795-816`), called from register carriage
  (`:469`) and the exact-revert floor scan (`:780-790`).
- F3. Register carriage reads `scripts/agents/register-carriage-paths.txt`
  (four lines) through `loadCarriagePolicy` (`observe.go:491-500`), parses
  it (`:542-568`), matches it (`:570-584`), then applies append-only rules
  to three named files (`:476-487`, `:586-645`). `landing-classes.json:7`
  names that allowlist as the carriage `pathRule`, checked at `:523-526`.
- F4. The waiver rule reads `scripts/agents/instruction-bearing-paths.txt`
  (`internal/validate/conformance.go:519-566`), unions the runtime
  registry's declared instruction files (`:545-550`), and refuses a
  prose-under-30 waiver that touches a hit (`:627-637`). Its paths are
  already project-relative (`:581-591`).
- F5. Goal-verb commits never meet the evaluator: the goal engine builds
  them with `commit-tree` carrying a `Goal-Transaction` trailer
  (`internal/goal/txn.go:283-291`) and publishes with `update-ref`
  (`:347-360`); the ledger is `plans/goals/` and `records/goals/`
  (`internal/goal/validate.go:31-35`), validated by `ValidateCommit`
  (`validate.go:410`). `internal/goal/goal.go:128` still names
  `plans/goals-accepted.json`, which the migration deleted (`migrate.go:94`).
- F6. `path owner` resolves a path against the repository, strips the
  vendored prefix, and answers runtime for `artifacts/` and `bin/`
  (`internal/stateroot/owner.go:58-85`); the placement rule gives the four
  trees the same names on both sides (`plans/memory-architecture-design.md:39-51`).
- F7. The window, re-measured on this worktree (`git log 4a351338..HEAD`):
  170 wrapper-stamped landings, 142 would-refuse `direct-fix-floor-refused`;
  their paths by top directory: plans 174, records 109, memory 6, scripts 1
  (`scripts/agents/adapters/codex.sh`). Only two EXISTING `records/` files
  were modified in place (`records/misc/seat-stop-analysis.md`,
  `records/misc/breach-design-critique-r3.md`).
- F8. `Goal-Item:` is a house convention in commit bodies (`c0bd82fe`,
  `77060c14`); no script or Go file produces or reads it. The `Machine:`
  trailer IS stamped by the wrapper (`commit.sh:360-363`). Of 204 markdown
  files at the root of `plans/`, 46 begin with the id of a goal open under
  `plans/goals/`; handoffs are `plans/handoff-<seat>-<date>.md` (`plans/README.md:13`).

## 1. The manifest

**File:** `scripts/agents/path-classes.txt`, plain text, committed. Plain
text over JSON because the two lists it replaces are plain text in the same
directory with the same line grammar (F3, F4), the refusal text must name a
line a human can read and a grep fixture can check (section 7), and
`landing-classes.json` rows each need a ruling id (`observe.go:512-521`).

**Grammar.** One entry per line: `<path> <class>`. `#` starts a comment.
A path ending in `/` is a directory prefix; otherwise it is an exact file.
No globs. Class is one of `behavior`, `record`, `ledger`, `runtime`.
Duplicate paths, an unknown class, an absolute path, or a `..` segment make
the manifest unreadable; the evaluator then refuses every landing with the
existing `register-carriage-policy-unreadable` code (`observe.go:497`).

**Matching.** The lookup key is the installation-relative path (F1). The
entry with the longest matching prefix wins; a file entry matches only
itself; a directory entry matches itself and everything below it. A key
matching nothing is unclassified. The evaluator reads the manifest from the
LANDING BASE tree (`workspace.FileAt(baseTree, …)` as at `observe.go:495`),
so a candidate cannot reclassify the paths it lands
(`internal/landing/promotion.go:22-24`); the verb and the waiver rule read
the checked-out file.

**Outer paths.** The verb resolves a path the way `path owner` does (F6):
inside the installation the key is installation-relative; outside it but
inside the repository the key is repository-relative, so the template's
toplevel `development/` and `.claude/` classify by the same table. The
evaluator never sees them (F1); the verb still answers.

**Initial table.** One entry per line in the file, in this order. It covers
every tracked top-level entry of the installation plus the harness and
outer names the order requires. `go.work`, `go.work.sum` and
`plans/goals-accepted.json` are absent today and listed because the code
still protects or names them (`observe.go:799`, `goal.go:128`).

- behavior (engine, scripts, instructions, docs, skills, roles, schemas,
  templates, config, harness config; reviewed chain only): `AGENTS.md`,
  `CLAUDE.md`, `wow.md`, `README.md`, `LICENSE`, `metasystem.conf`, `go.mod`,
  `go.sum`, `go.work`, `go.work.sum`, `.gitattributes`, `.gitignore`, `cmd/`,
  `internal/`, `docs/`, `scripts/`, `skills/`, `optional-skills/`, `.claude/`,
  `.codex/`, `.agents/`, `.devin/`, `.github/`, `benchmark/`, `environment/`,
  `memory/README.md`, `plans/README.md`, `records/README.md`.
- record (the paper trail and the registers; register carriage): `memory/`,
  `plans/`, `records/`, `development/`.
- ledger (goal files; goal verbs only): `plans/goals/`,
  `plans/goals-accepted.json`, `records/goals/`.
- runtime (never landed): `artifacts/`, `bin/`, `metasystem` (the built
  binary, `.gitignore:3`), `metasystem.conf.local` (the root `.gitignore:1`).

`docs/journey.md` and `docs/reviews/` accrete but stay behavior
(`plans/memory-architecture-design.md:17`); moving them is outside this
goal. The manifest lives under `scripts/`, so it is behavior and changes
only through a reviewed chain; a fixture in section 7 pins that.

## 2. The verb

`metasystem path class <path>` beside `path owner` (`cmd/metasystem/main.go:244-247`,
`cmd/metasystem/path_verbs.go:11-30`), implemented by a new package
`internal/pathclass` (loader, matcher, refusal text) that the three
consumers share. Output for scripts: one word on stdout, `behavior`,
`record`, `ledger` or `runtime`, exit 0. Unclassified: stdout `unclassified`,
exit 1, and on stderr the one refusal text every consumer reuses:

```
path <key> has no class in scripts/agents/path-classes.txt; nearest
classified ancestor: <entry> (<class>). Add one line for it to the
manifest through a reviewed chain.
```

For humans, `--explain` prints `<class> entry=<matched line> key=<key> mode=<template|adopted>`.
A path outside the repository answers `outside`, exit 1 (`owner.go:63-66`).

## 3. The three consumers

Each reads the manifest through `internal/pathclass` and nothing else.

**(a) The landing evaluator** (`internal/landing/observe.go`). Every
changed path of a landing is classified once, over the whole set, before
any class rule runs (floor precedence stays set-wide, as
`records/two-bars/floor-verdict-addendum.md:40-41` requires).

| class | `--chain` (observeChain `:106-153`) | `--direct-fix register-carriage` (registerCarriage `:449-489`) | `--direct-fix exact-revert` (exactRevert `:721-778`) |
|---|---|---|---|
| behavior | pass when certified (`:135-152`, unchanged) | `direct-fix-floor-refused` (code kept, `floor-verdict-addendum.md:12`) | `direct-fix-floor-refused` for any behavior path in the target-or-candidate union (`:780-790`) |
| record | as an extra path it needs carriage (`:140-142`, unchanged) | record semantics, section 5 | pass when the inverse is exact (an exact inverse of a record is allowed) |
| ledger | `ledger-path-not-goal-verb` | `ledger-path-not-goal-verb` | `ledger-path-not-goal-verb` |
| runtime | `runtime-path-refused` | `runtime-path-refused` | `runtime-path-refused` |
| unclassified | `path-unclassified` | `path-unclassified` | `path-unclassified` |

How the evaluator recognizes a goal-verb commit today: it does not, and
need not (F5). A goal-verb commit is built by the goal engine and never
passes `commit.sh`; every landing the evaluator sees is by construction not
a goal verb, so a ledger path in any landing refuses. The goal engine keeps
`ValidateCommit` as its own gate (F5).

Functions that change, with returns:
- `registerCarriage(root, candidateTree, changedPaths, goal, machine) error`
  (`:449`): loads the manifest instead of the allowlist; classifies every
  path; returns a `carriageError` coded per the table, floor first; for
  record paths applies section 5 (`register-carriage-not-append-only` or
  `record-not-owned`); nil when every path is a lawful record change.
- `loadCarriagePolicy` (`:491-500`) becomes `loadPathClasses(workspace, baseTree) (pathclass.Manifest, error)`;
  `parseCarriageAllowlist` (`:542-568`) and `carriagePathAllowed` (`:570-584`) are deleted.
- `loadLandingClasses` (`:502-540`): the `pathRule` check at `:523-526`
  expects `path-class-record`; `landing-classes.json:7` changes to that string.
- `exactRevertFloorError` (`:780-790`) becomes `exactRevertClassError`:
  classifies the union and returns the table's exact-revert column; `not-exact-revert`
  stays the fallback (`:718`).
- `observeChain` (`:106-153`): after `bindCertifiedChange` (`:135`), classify
  certified paths plus extras; ledger, runtime and unclassified refuse
  before the carriage branch at `:143`.
- `ObserveParams` (`:39-45`) gains `Goal` and `Machine`; `landing observe`
  (`cmd/metasystem/landing_verbs.go:9-25`) gains `--goal` and `--machine`.
- `knownRefusalCode` (`internal/landing/promotion.go:90-117`) gains
  `path-unclassified`, `ledger-path-not-goal-verb`, `runtime-path-refused`,
  `record-not-owned`.
- `commit.sh`: a `--goal <id>` flag beside `--chain` (`:40-76`), forwarded
  as `--goal` and stamped as a `Goal-Item: <id>` trailer next to `Machine`
  (`:363-365`) so line and check cannot disagree; `--machine "$machine_nickname"`
  from `:360`; the refusal switch (`:319-346`) gains a branch per new code,
  and `path-unclassified` prints the verb's text for each listed path (`:320`).

**(b) The critique-waiver rule** (`conformance.go:519-566`). The file read
and the `isInstructionBearing` closure become `pathclass.Load(r.root).Class(path)`;
the waiver refuses when any path is `behavior` or unclassified (`:627-637`
keeps its shape, message "prose-under-30 touches a behavior path that is
never waivable"). The registry union (`:545-550`) is dropped: a new root
instruction file is unclassified and refuses everywhere, stronger than the union.

**(c) Register carriage.** Its allowlist becomes "every record path", which
is (a)'s record column; no separate file or function.

## 4. The deletions

| deleted | readers today, all rewritten or removed in the same slice |
|---|---|
| `neverDirectFix` (`observe.go:792-816`) | `observe.go:469`, `:782`; comment `:792-794` |
| `scripts/agents/register-carriage-paths.txt` | `observe.go:495`; `observe_test.go:29` (fixture copy), `:478` (self-change leg); `scripts/agents/static-reproof-fixtures.sh:24`; `landing-classes.json:7` (by name of rule) |
| `scripts/agents/instruction-bearing-paths.txt` | `conformance.go:519`; `internal/validate/instructionowners_test.go:15,39`; `nested_conformance_test.go:233`; `conformance_test.go:53-54`; `scripts/agents/conformance-fixtures.sh:34`; `scripts/validate-metasystem.sh:1003` (asset list) |

Nothing else reads them: the section 7 grep, run today, hits exactly the
nine files above. History (`docs/reviews/2026-08-12-full-system-review.md:1972,2229`,
`records/`, `plans/`) stays as written; "instruction-bearing" at
`internal/runtimes/runtimes.go:62` names a concept, not the file. The G-5
lint (`instructionowners_test.go:12-18`) is kept as a manifest check: every
rule-owning document must classify as behavior.

## 5. Record semantics, stated once

The evaluator applies these to record paths under register carriage. Rules
go by location, never by a list of names.

| record files | rule | evaluator check |
|---|---|---|
| `memory/receipts.log`, `records/narrator-digest.log` | append-only | `appendOnly` (`observe.go:586-621`): the file only grows |
| `memory/rulings.md` | append-only, rows | `addRulingRowsOnly` (`:623-645`) |
| any other path under `records/` (except `records/goals/`, ledger) | new file only | base entry absent; a modified or deleted existing file refuses `record-not-owned` (`records/README.md:1-6`; F7: two window landings would have refused) |
| `plans/handoff-<seat>-*.md` | seat-owned | new file, or modified when `<seat>` equals the wrapper's machine nickname (the `Machine` value before `+`, `commit.sh:360`, `:363`); else `record-not-owned` |
| `plans/<goal-id>-*.md` where `<goal-id>` names a file in `plans/goals/` at the base | goal-owned | new file, or modified when the landing's `Goal-Item` equals that goal id; else `record-not-owned` |
| every other record path (`memory/` registers, remaining `plans/` files, `development/`) | shared register | new file, or modified when `Goal-Item` names a goal open at the base (`plans/goals/<id>.md` present); no goal, refuse `record-not-owned` |

A path matching several rows takes the first. The name binding is the only
mechanical record-to-goal link the tree has (F8); a legacy design whose
name binds to no open goal is a shared register, and concluded designs
belong in `records/`, new-file only. Weakest point, see section 9.

## 6. The second bar afterwards

With the manifest as the source, `direct-fix-floor-refused` means exactly:
a `register-carriage` or `exact-revert` landing changed a behavior path.
Promoting it is one string added to `refuseCodes` in
`scripts/agents/landing-promotion.json:3`, effective through
`applyPromotion` (`promotion.go:36-38`) and the wrapper's refusal branch
(`commit.sh:319-346`), plus one ruling row as R-40-m0 was for the first two
codes (`memory/rulings.md:88`). Nothing else. THIS slice promotes
`path-unclassified`, `ledger-path-not-goal-verb` and `runtime-path-refused`
under R-55-m1 ("A path with no class is refused"); no window landing would
have hit them (F7). `direct-fix-floor-refused` and `record-not-owned` stay
observed until the separate promotion the goal names.

## 7. Fixtures, deterministic

Go, beside `observe.go` (style of `observe_test.go:124`, `:277`, `:476`):
- `TestObserveClassifiesEachPathClass`: one path per class under
  `register-carriage`: `internal/x.go` refuses `direct-fix-floor-refused`;
  `plans/handoff-fixture-1.md` (new) passes bar b; `plans/goals/x.md` refuses
  `ledger-path-not-goal-verb`; `bin/metasystem` refuses `runtime-path-refused`;
  `product.txt` refuses `path-unclassified`.
- `TestObserveChainRefusesLedgerRuntimeAndUnclassifiedPaths`: the same three
  under a valid closed chain (`observe_test.go:124-140` fixture).
- `TestObserveExactRevertRefusesByClass`: a behavior target refuses; a
  record inverse passes.
- `TestObserveRecordSemantics`: digest rewrite refuses (keeps `:506-516`);
  a modified `records/misc/x.md` refuses `record-not-owned`; `plans/fx-design.md`
  modified under `Goal: fx` with `plans/goals/fx.md` at base passes and under
  `Goal: other` refuses; `plans/handoff-m9-x.md` modified with `Machine: m9`
  passes and with `m1` refuses.
- `TestObserveManifestIsBehavior`: a candidate that edits `path-classes.txt`
  under carriage refuses `direct-fix-floor-refused` (replaces `:476-484`).
- `internal/pathclass`: `TestLongestPrefixWins`, `TestManifestRejectsMalformedLines`,
  `TestRepositoryManifestClassifiesEveryTrackedPath` (walks `git ls-files`
  of the installation; any unclassified path fails, naming it).
- `internal/validate`: `TestMergeWaiverRefusesBehaviorPath` and
  `TestMergeWaiverRefusesUnclassifiedPath` replace the instruction-bearing
  legs; `TestInstructionOwnersAreBehavior` replaces `instructionowners_test.go`.
- `cmd/metasystem`: `TestPathClassVerbOneWordAndRefusalText`.

Shell, `scripts/agents/path-class-fixtures.sh`, registered beside
`static-reproof-fixtures.sh` (`validate-metasystem.sh:977`, `:1069`):
- `TestPathClassVerbAnswersFromManifest`: four words for four paths, and
  the refusal text names the manifest and the nearest ancestor.
- `TestDeletedListsHaveNoReader`: `rg -n 'register-carriage-paths|instruction-bearing-paths|neverDirectFix' cmd internal scripts skills AGENTS.md wow.md docs --glob '!docs/reviews/**'`
  prints nothing.
- `TestCommitWrapperStampsGoalItemTrailer`: `--goal fx` lands a
  `Goal-Item: fx` trailer (extends `static-reproof-fixtures.sh:12`).

## 8. Size and diff boundary

One build slice within the reserved 240 minutes. Exhaustive boundary:

- new: `scripts/agents/path-classes.txt`; `internal/pathclass/pathclass.go`,
  `pathclass_test.go`; `scripts/agents/path-class-fixtures.sh`
- deleted: `scripts/agents/register-carriage-paths.txt`, `scripts/agents/instruction-bearing-paths.txt`
- `internal/landing/observe.go`, `observe_test.go`, `promotion.go`
- `internal/validate/conformance.go`, `conformance_test.go`, `nested_conformance_test.go`,
  `instructionowners_test.go`
- `cmd/metasystem/path_verbs.go`, `main.go` (verb row at `:247`),
  `landing_verbs.go`, plus a verb test file
- `scripts/agents/commit.sh`, `landing-classes.json`, `landing-promotion.json`,
  `static-reproof-fixtures.sh`, `conformance-fixtures.sh`
- `scripts/validate-metasystem.sh` (asset row `:1003`, fixture rows at `:977`, `:1069`)

Out of scope: moving `docs/journey.md`, `docs/reviews/` or legacy `plans/`
designs; the goal engine; existing commits' `Goal-Item` prose lines.

## 9. Self-grade

- Confidence: high on sections 1 to 4 and 6 to 8, every cited line read in
  this worktree; medium on section 5.
- Weakest claim: the goal-owned rule binds by name (`plans/<goal-id>-*.md`).
  It covers 46 of 204 root plans files today (F8); the rest become shared
  registers revisable under any open goal, looser than "only the goal that
  owns it" for legacy files.
- Reject condition: a record kind the evaluator must accept that section 5
  refuses; a window landing touching a path outside `plans/`, `records/`,
  `memory/`, `scripts/` (F7 says none); or `--goal` breaking the proved-tree
  postcondition (`commit.sh:347-355`).
