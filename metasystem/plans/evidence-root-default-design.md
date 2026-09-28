# The evidence root has one owner and a default

Design, 2026-09-28. Author: Fable (design lane). Approved shape: Wido, 2026-09-28. Status: for
Astra's critique, then a build on Opus after `embed` and `u9b` land (see Sequencing).

## Problem

`evidence.root` is the durable, host-local directory outside the checkout where run evidence is
mirrored so it survives `git clean` of the gitignored `artifacts/`: terminal delegate job records
and their payload (`internal/dispatch/mirror.go:Mirror`), app run records
(`cmd/metasystem/app.go:preserveRunEvidence`), and the collector that may then dispose of the raw
copies (`internal/evidence/gc.go:GC`). Suite-failure artifacts are NOT under it: the watchdog
preserves them at `<checkout>/artifacts/agents/suite-failures/` (`internal/proofrun/watchdog.go`,
`internal/proofrun/evidence.go`); the brief's claim that they are mirrored to the root does not
hold.

The committed `metasystem.conf` ships `evidence.root=<durable evidence root, outside the
repository>` and expects each seat to set its own in the uncommitted `metasystem.conf.local`.
Nothing in setup sets it or checks it: adoption prints "Fill metasystem.conf with ... the durable
evidence root" (`cmd/metasystem/intent_adopt.go:runIntentAdopt`, checklist step 3); `system start`
(`cmd/metasystem/process_verbs.go:processOwners.arm`) and steward arming never touch it; the
placeholder audit exempts the template checkout (`internal/audit/metasystem.go`, `isTemplate`).

Eight sites read the key themselves (two more take the resolved path as an argument) and each
handles "unset" its own way (all verified at whole-function depth on 2026-09-28):

| Reader | How it reads | On unset or placeholder |
|---|---|---|
| `internal/delegation/reap.go:session.mirrorRecord` | `s.configGet("evidence.root","")` (`helpers.go:configGet` = `config.Get` over `<root>/metasystem.conf`) | `mirrorFail(job, "evidence.root must be absolute")`, one log line in `artifacts/agents/mirror-failures.log`, caller continues (KI-6) |
| `internal/dispatch/mirror.go:Mirror` | takes the path as an argument; refuses one inside the checkout | n/a (message spells the key) |
| `internal/evidence/gc.go:GC` | takes the path as an argument | refuses a non-absolute root loudly |
| `cmd/metasystem/evidence_verbs.go:evidenceGC` | `config.Get`, then `filepath.IsAbs` | "evidence-gc refused: evidence.root is not configured" |
| `cmd/metasystem/app.go:appRun.preserveRunEvidence` | `config.Get` over `roots.Installation` | refuses closure; record and tree are kept, human told to set the key and stop again |
| `internal/metrics/data.go:evidenceRoot` | `config.Get`, non-absolute becomes `""` | mirrored records silently absent from metrics |
| `internal/stateroot/stateroot.go:Resolver.StateRoot(Evidence)` | `config.Get` without a default | error; no production caller, tests only |
| `cmd/metasystem/seat_launch.go:seatLaunchFacts` and `internal/seat/launch/preflight.go:evidenceRootSet` | `config.Get`; refuses a non-absolute root before the clone | `SEAT_LAUNCH_EVIDENCE_ROOT_UNSAFE` |
| `internal/seat/launch/sequence.go:Sequencer.evidenceRoot` | runs `metasystem config get --key evidence.root` in the source seat, makes the SIBLING directory named for the nickname, writes it into the clone's `.local` via `host.go:OSHost.CopyLocalConf` and `internal/validate/confset.go:SetConfKeys` | refuses (same code) |
| `internal/config/validate.go:validateWithRunner` (~line 645) | judges the effective value: committed, then `.local`, then env; fixed in 12611b98c | "evidence.root is required" / "must be absolute" / "must be outside the repository" |

Host facts, surveyed 2026-09-28 with `bin/metasystem config get --key evidence.root --conf
<checkout>/metasystem/metasystem.conf` (the value only; no `.local` was read):

- On the placeholder:
  `agentic-tools-{l10,l13,mrspeed,pmimpl,synclock,verbs,witness,custody,backlog-plan-20260907}`, the
  nine `agentic-tools-*-20260926` worktrees and about ninety `verbs-*` worktrees. Every one of them
  has ZERO records under `artifacts/agents/jobs/` and no `mirror-failures.log`. `agentic-tools-ui`
  is NOT on the placeholder: it resolves to `/Users/wido/metasystem-evidence/ui` (directory created
  2026-09-28 20:00, empty). No `agentic-tools-testbed` exists on this host (one sits in `~/.Trash`).
- Sharing `/Users/wido/metasystem-evidence/agentic-tools`: `agentic-tools` (499 job records), `m1b`
  (858), `m1b-retired-20260906` (46), `m1c` (376), `m1e` (93), `paper` (0), `slc-r4` (24); `m1d` has
  `/Users/wido/metasystem-evidence/agentic-tools-m1d` (226). Mirror failures on record: m1d 8 lines
  and m1e 2 lines, all `evidence.root must be absolute` on 2026-09-10 (before their roots were set);
  main 1 line, a copy failure.
- `m1c` is the ordinary way round: its committed conf carries the placeholder and its `.local`
  carries the real path (contradicting seat m1e-a0's report). The fall-through rule below is right
  regardless and is tested.

Resolution order today (`internal/config/resolve.go:Get`): flag, environment (`EnvName`:
`METASYSTEM_EVIDENCE_ROOT`), mode-scoped conf (role keys only), `.local`, committed conf, default.

## Decision summary

1. One owner: `internal/config/evidenceroot.go:ResolveEvidenceRoot`. Every reader and the launch go
   through it. Order: env `METASYSTEM_EVIDENCE_ROOT`, `metasystem.conf.local`, committed
   `metasystem.conf`, then the COMPILED-IN default. Placed in `internal/config` because every reader
   already imports it and `internal/evidence` cannot be the owner (`internal/evidence` imports
   `internal/dispatch`, which imports `internal/stateroot`, which would import the owner: a cycle).
   No new package, so no new coverage floors.
2. Unspecified at a source means: key absent, value empty after trimming, or a placeholder (`<...>`:
   begins with `<` and ends with `>`). Unspecified falls through to the NEXT source, so a
   placeholder in `.local` above a real committed value resolves to the committed value. A specified
   value that is relative, or that lies inside the checkout, is REFUSED with the source named; it
   never falls through and is never replaced. A set-but-empty environment variable is unspecified.
3. Default: `$HOME/metasystem-evidence/<checkout basename>`, e.g.
   `~/metasystem-evidence/agentic-tools-ui`. `HOME` is read through the same lookup as the key (on
   this platform `os.UserHomeDir` is `$HOME`); unset `HOME` is an error naming the fix. The checkout
   is the nearest directory, from the installation upward, holding a `.git` entry (file or
   directory, by `os.Lstat`; no Git command); with none, the installation directory itself (a fresh
   adoption before `git init`). Template `<repo>/metasystem`, adopted `<app>/metasystem`, a
   worktree, and an adopted root installation all name the right directory.
4. The resolver creates nothing. Writers create on first use, which they already do
   (`dispatch.Mirror` `MkdirAll`s the destination; `proofrun.preserveEvidence` `MkdirAll`s). Readers
   must not fail on a missing root: `metrics.loadJobs` globs (empty result), `evidence.GC` finds no
   manifests and keeps every chain "not mirrored", which is the correct verdict.
5. Setup says it and never requires it: adoption, `system start` and a machine launch each print one
   line, `evidence root: PATH (default; set evidence.root in metasystem.conf.local to change)` or
   `evidence root: PATH (metasystem.conf.local)` / `(METASYSTEM_EVIDENCE_ROOT)` /
   `(metasystem.conf)`. An explicitly invalid root refuses `system start` by the resolver's
   sentence; unset never does.
6. The launch stops deriving a sibling and relies on the default for the new machine: the clone's
   default is `~/metasystem-evidence/<destination basename>` =
   `~/metasystem-evidence/<repository>-<nickname>`, distinct per machine by construction because the
   preflight already refuses an existing destination. The copied `.local` has its `evidence.root`
   blanked so the seat's own root does not travel. Writing the default explicitly would freeze it
   and duplicate the rule; deriving a sibling needed the seat's root to exist, which is the gap this
   design closes.
7. The committed `metasystem.conf` loses its `evidence.root=<placeholder>` line (a comment names the
   default). Otherwise the adopted-repository placeholder audit would still demand a value.
8. Held by tests as listed per unit; a structural test keeps the key's spelling in one file.
9. KI-6 (the silent mirror failure) stays out; listed under Later.

## Step 1

Each unit is one build, green under `cd metasystem && go run ./cmd/devgate static` (the fast gate:
dependency and parallel ratchets, gofmt, shell parse, vet, staticcheck, the refusal register test,
the SessionStart and Stop-surface audits, project check, build;
`cmd/devgate/static.go:gateRun.static`). Every new test is `t.Parallel()` with per-test lookups; no
`t.Setenv`, because `testing-parallel-ratchet.json` refuses any increase in a package's serial test
count (`cmd/devgate/owners.go:parallelRatchet`). No test runs Git: fixtures write a `.git` file
where a checkout is meant.

### U1. The owner, and `config validate` on it

Files: `internal/config/evidenceroot.go` (new), `internal/config/evidenceroot_test.go` (new),
`internal/config/validate.go:validateWithRunner` (the evidence block replaced by one resolver call),
`internal/config/validate_test.go` (cases "evidence required" and
`TestValidateJudgesTheEffectiveEvidenceRoot` flipped), `metasystem/metasystem.conf` (line 152
removed; comment at 149-151 reworded to name the default), `docs/project-rules.md` line 13 (the
`<path outside the repository>` placeholder replaced by the default and the `.local` sentence, so
adopters have nothing to fill).

```go
const EvidenceRootKey = "evidence.root"          // the only file that spells it
type EvidenceRootParams struct {
    Installation string                        // the directory holding metasystem.conf
    LookupEnv    func(string) (string, bool)   // nil: os.LookupEnv; answers HOME too
}
type EvidenceRoot struct {
    Path   string // absolute, cleaned
    Origin string // "env", "conf-local", "conf" or "default"
}
func (r EvidenceRoot) Default() bool
func (r EvidenceRoot) Line() string             // the one setup line, decision 5
func ResolveEvidenceRoot(p EvidenceRootParams) (EvidenceRoot, error)
```

Behavior: decisions 2-4. Reads through `ConfLookup` (strict duplicates, as today). Refusal sentences
keep validate's phrases so its tests survive: `evidence.root must be absolute (metasystem.conf.local
reads "relative/dir")`, `evidence.root must be outside the repository (...)`. `validate.go`
additionally keeps its `withinRepo(path, repo)` check against `--repo`, since the resolver's
checkout is the `.git` walk and `--repo` may differ.

Tests (all in `evidenceroot_test.go`, table-driven over a fixture installation with a `.git` file
two levels up and a map-backed `LookupEnv`): env wins over `.local`; `.local` over committed;
committed over default; placeholder in `.local` falls through to a committed real path (the
m1c-shaped case, named as such); placeholder everywhere resolves to
`<home>/metasystem-evidence/<checkout basename>` with `Origin=="default"`; empty and set-but-empty
env are unspecified; a relative `.local` value is refused naming `.local`; a value inside the
checkout is refused; `HOME` unset is an error; a `.git`-less installation names its own basename; a
nested `<repo>/metasystem` names `<repo>`; duplicate key is an error. `validate_test.go`: a conf
with no key passes; the placeholder alone passes; relative and in-repository still refuse.

### U2. The engine readers

Files and functions:
- `internal/delegation/reap.go:session.mirrorRecord`: resolver over `s.root` with the lookup U9b
  threads through the session; an error goes to `mirrorFail(job, err.Error())` and returns
  `exitWith(1)` exactly as today (KI-6's shape is untouched).
- `internal/dispatch/mirror.go:Mirror`: logic unchanged; the message uses `config.EvidenceRootKey`
  (dispatch already imports config).
- `cmd/metasystem/evidence_verbs.go:evidenceGC`: the `config.Get` replaced by the resolver when
  `evidenceRoot == ""`; an error is the refusal line.
- `cmd/metasystem/app.go:appRun.preserveRunEvidence`: resolver over `r.roots.Installation` with
  `intentOwners.lookupEnv` (new field beside `commandNow`, nil = `os.LookupEnv`, carried onto
  `appRun`); the "record is kept" sentence stays, prefixed by the resolver's error.
- `internal/metrics/data.go:evidenceRoot`: resolver; an error becomes one `coverage.Details` line
  instead of silence.
- `internal/stateroot/stateroot.go`: the `Evidence` kind, its `StateRoot` branch and its
  `relativeRoot` case are deleted with their two tests (`stateroot_test.go:119`,
  `TestEvidenceRootMustBeConfiguredAndAbsolute`); it has no production caller and would otherwise be
  an eighth reader.

Tests: a delegation bed (`internal/delegation/bed_test.go:newBed`, in-process, fake ports) whose
conf carries no key and whose U9b lookup answers `HOME`: a terminal job's mirror lands at
`<home>/metasystem-evidence/<bed basename>/agents/<segment>/<chain>/manifest.json`; the same bed
with `evidence.root=relative` logs one `mirror-failures.log` line naming `.local`/conf and mirrors
nothing. `evidenceGC` unit: no key collects against the default; relative refuses.
`preserveRunEvidence` unit with a fixture `applaunch.Record` and roots: the copy lands under
`<home>/metasystem-evidence/<checkout>/goals/g1/app/<key>/<ts>/run.txt`. Metrics fixture
(`internal/metrics/fixture_test.go`) with a relative root: one rejected detail, jobs otherwise read.

### U3. Setup says it: adoption and `system start`

- `internal/adopt/adopt.go:Adopt`: after the payload lands, resolve over the target installation
  with a new `Deps.LookupEnv` (nil = `os.LookupEnv`) and append `EvidenceRoot.Line()` to
  `result.Notes`. `cmd/metasystem/intent_adopt.go:runIntentAdopt` checklist step 3 becomes
  "Optionally set models, tiers and the evidence root in metasystem.conf.local; the defaults above
  apply until you do." No test pins the old checklist text (grepped `cmd/metasystem/*_test.go`,
  `internal/adopt/*_test.go`).
- `cmd/metasystem/process_verbs.go:processOwners.arm`: a new `evidenceRoot func(installation string)
  (config.EvidenceRoot, error)` field (default: the resolver), called before `armSteps`; its line is
  the first of `report.Lines`; an error is a `processRefusal` with the resolver's sentence and the
  plain `metasystem system start: ...` form, before the fence moves.

Tests: `internal/adopt/adopt_test.go` (fake Git, as its existing cases): a target with the template
conf (placeholder) yields the default note naming the target basename; a target whose `.local` sets
a root yields the `(metasystem.conf.local)` note. `cmd/metasystem/intent_process_test.go` (injects
`armSteps` already): a start with nothing configured prints the default line; a relative `.local`
root refuses before the fence with the resolver's sentence.

### U4. The launch relies on the default

- `internal/seat/launch/sequence.go`: delete `Sequencer.evidenceRoot`; `configuration` copies with
  `Host.CopyLocalConf(source, destination)` (two arguments) and then asks
  `Host.EvidenceRoot(destinationInstall)` for the line, appended to the step's words.
  `Record.Created.EvidenceRoot` is deleted (records carrying it are read unchanged; unknown fields
  are ignored). The package header comment's `METASYSTEM_EVIDENCE_ROOT` sentence stays: the scrub is
  still what makes the file the last word.
- `internal/seat/launch/host.go`: `OSHost.CopyLocalConf` blanks the key via
  `validate.SetConfKeys(name, {Key: config.EvidenceRootKey, Value: ""})` (empty is unspecified by
  decision 2; a `DropConfKeys` is Later). New `OSHost.EvidenceRoot(installation)` calls the resolver
  with a lookup that answers only `HOME` from this process, which is the scrubbed view the machine's
  own steps run under. `Host` loses `MakeDir` and `Canonical` (their only user was the sibling
  derivation) and gains `EvidenceRoot(installation string) (line string, err error)`.
- `internal/seat/launch/preflight.go`: delete `Facts.EvidenceRoot` and `evidenceRootSet`;
  `Preflight` ends at `destination`. `cmd/metasystem/seat_launch.go:seatLaunchFacts` drops its
  `config.Get`, the file's only use of `config`, so the import goes with it.
- `internal/seat/launch/refusal.go`: delete `CodeEvidenceRootUnsafe` and the H1 branch of
  `Refusal.Error` that names it.
- `internal/refusal/register.go`: delete row `SEAT_LAUNCH_EVIDENCE_ROOT_UNSAFE` (line 381); re-pin
  the rows the deletions shift (below).

Tests: `sequence_test.go`: the four `TestTheEvidenceRoot...` cases and the two "record says this
launch made the evidence root" cases (lines ~262-330, ~463-482) are deleted; new: the copied conf's
evidence root is blanked and the configuration step's words carry the fake host's line; a host whose
`EvidenceRoot` errors fails the configuration step in its words.
`preflight_test.go:TestASeatWithNoEvidenceRootIsRefusedBeforeAnythingIsCloned` deleted. `host_test`
(new, no Git): `OSHost.CopyLocalConf` on a fixture `.local` keeps every other line and blanks the
key; `OSHost.EvidenceRoot` on a fixture installation names `<HOME>/metasystem-evidence/<basename>`.
`integration_test.go` (the one narrowly named real-Git test, allowed by the rule) drops its canned
`config get` answer and asserts the blanked key instead of the sibling path (lines 72, 141-150).

### U5. The structural guard

`internal/config/evidenceroot_owner_test.go`: walks the module (`../..`, as
`internal/layering/r9_test.go` does) over `cmd/**`, `internal/**` and `scripts/**`, every `*.go` not
`_test.go` and every `*.sh`, skipping `node_modules`, and fails by path when any file other than
`internal/config/evidenceroot.go` contains the substring `evidence.root`. Comments count,
deliberately: the spelling lives in one place, and prose says "the evidence root". This unit also
sweeps the last spellings U2-U4 left (`evidence_verbs.go:21`, `sequence.go:65`, `preflight.go:67`,
`host.go` comments, `validate.go` comment) and is green only once nothing outside the owner names
the key.

Fixture hygiene, a safety matter of this unit: until now a bed that copied the template conf and
never set the key mirrored nothing; from U1 on it resolves to a real directory under the test
process's `$HOME`. Every bed that reaches a mirror or an app copy must name a root inside its temp
dir. The known one: `cmd/metasystem/adoption_comparison_test.go` (~line 437) rewrites the template
conf key by key and substitutes its fixture root only when the line is present, so it must append
the key when absent. The build proves the rest once, by hand, not by mechanism: run the Go tests
with `HOME` pointed at an empty scratch directory and assert that directory is still empty
afterwards; any bed that wrote there names its root.

## Sequencing (against m1e's three units in flight)

The `embed` and `u9b` branches are NOT visible from this seat: `git branch -a` in
`agentic-tools-ui`, `agentic-tools-m1e` and `agentic-tools` lists no branch by either name, and
m1e's worktrees (`git worktree list`) carry `codex/verb-*` branches only; m1e's
`internal/stateroot/stateroot.go` at HEAD is byte-identical to this checkout's. This design is
therefore written against the coordinator's description of them, not their code.

- (a) `embed` changes `stateroot.go`'s installation marker (`installationShape`,
  `validateInstallationShape`: `scripts/agents` becomes `metasystem.conf`). U2 deletes the
  `Evidence` kind in other hunks of the same file. Build after `embed`; U2 rebases onto it. The
  resolver never consults `stateroot`, so the marker change does not affect it.
- (b) `u9b` threads a config lookup through `internal/dispatch` and `internal/delegation` in place
  of `os.LookupEnv`. U2's `mirrorRecord` passes that lookup as `EvidenceRootParams.LookupEnv`, so
  the bed test injects `HOME` through it. Build after `u9b`; if `u9b` moves the reader, U2 follows
  it.
- (c) C4 compiles the shipped defaults in and leaves `metasystem.conf` as overrides only, under
  Wido's ruling of 2026-09-28 (evening CEST, reviewing `metasystem/scripts`): "data the engine reads
  at runtime to decide behaviour is source code: compile it in". `ResolveEvidenceRoot` IS this key's
  compiled-in default; m1e leaves `evidence.root` to it in C4. U1's removal of the committed line is
  C4-shaped and lands first. Once no checkout carries the old conf, the placeholder rule is dead but
  harmless; it stays as the transition rule.

## Migration and compatibility

- Explicit settings are untouched: every seat with an absolute value keeps it (`Origin` names the
  source). `ui` keeps `/Users/wido/metasystem-evidence/ui`, which is NOT the default
  (`~/metasystem-evidence/agentic-tools-ui` would be) and is not changed.
- Placeholder seats start resolving to `~/metasystem-evidence/<basename>` on first use; they have
  nothing to migrate (zero records each).
- Nothing moves anyone's existing evidence. The seven checkouts on the shared root stay on it; any
  change to m1e waits for a heads-up to m1e.
- Machines launched earlier keep the explicit sibling root in their `.local`. Old launch records
  with `Created.EvidenceRoot` load unchanged.
- Adopted repositories carrying the old committed placeholder resolve to the default through the
  transition rule; the audit's `<durable evidence root, outside the repository>` alternative in
  `auditPlaceholderRe` is left in place (nothing matches it after U1; removing it is not needed).
- `metasystem config get --key evidence.root` keeps working as a generic read; nothing in the engine
  uses it for this key any more.
- `/Users/wido/LocalStorage/agentic-tools-evidence` (22 hand-named run trees such as
  `review-room-20260928`, `batch-lane-20260928`; six of m1e's worktrees live under
  `agent-help-20260926/`; referenced by nine plans documents as archives) is unrelated to
  `evidence.root`: no conf key points there, no reader or writer of the engine touches it, and the
  default keeps a separate tree, `~/metasystem-evidence`. It stays where it is.

## Pinned checks to re-pin

- `internal/refusal/register.go`, ±2-line window (`register_test.go:refusalSiteWindowRadius = 2`):
  delete row 381 (`SEAT_LAUNCH_EVIDENCE_ROOT_UNSAFE`, `sequence.go:553`, `ForwardSite
  refusal.go:75`; also its H1 `StandingGuide` forward `machine start`). Re-pin
  `SEAT_LAUNCH_WORD_REQUIRED` (`sequence.go:664`), `SEAT_LAUNCH_SUPERVISION_DOWN` (`:703`),
  `SEAT_LAUNCH_IDENTITY_UNREADABLE` (`:731`), which shift by the ~60 lines U4 removes above them;
  `SEAT_LAUNCH_DISK_SHORT` (`:354`) sits above the removal and holds. Re-pin
  `SEAT_LAUNCH_NICKNAME_INVALID` (`preflight.go:99`), `NICKNAME_TAKEN` (`:113`),
  `DESTINATION_EXISTS` (`:125`), `DESTINATION_INSIDE_A_CHECKOUT` (`:142`), which shift by the ~10
  lines of `evidenceRootSet`. No row names `validate.go`, `app.go`, `evidence_verbs.go`, `reap.go`,
  `data.go`, `process_verbs.go`, `intent_adopt.go` or `adopt.go`.
- `testing-parallel-ratchet.json`: no new serial tests; deletions lower counts, which the ratchet
  allows.
- Coverage ratchet (full gate only, `scripts/agents/coverage-ratchet.json` and
  `coverage-ratchet-linux.json`): no new package, so no new floor; the touched packages' floors must
  hold: `internal/config` 82.7, `internal/dispatch` 75.9, `internal/delegation` 72.1,
  `internal/metrics` 85.0, `internal/stateroot` 86.0, `internal/seat/launch` 74.6, `internal/adopt`
  81.3, `internal/evidence` 80.5 (unchanged code); `cmd/metasystem` is exempt.
- Stop decision surface audit (`cmd/devgate/owners.go:stopSurface`): the deleted tests carry no
  `Verdict{ShouldBlock` line; not triggered. SessionStart exit audit: no hook touched.
- `internal/layering/r9_test.go`: `internal/config` is neither an owner nor a composition package;
  unaffected.
- `cmd/metasystem/adoption_comparison_test.go` (~line 437) must append the key it can no longer
  substitute (U5, fixture hygiene); `internal/validate/conftailor_test.go` writes its own fixture
  conf with the key and is unaffected.

## Later, when it hurts

- KI-6: a failed mirror is a log line and a swallowed return; a per-job field the watcher reports,
  and a re-mirror sweep for terminal records without a `mirror` field (the ten 2026-09-10 failures
  on m1d and m1e are this shape).
- A catch-up mirror of pre-existing evidence: not needed now (placeholder seats hold no records); a
  sweep would be KI-6's.
- `validate.DropConfKeys` so a launched machine's `.local` carries no blank `evidence.root=` line.
- App run copies are not segmented by checkout (`<root>/goals/<goal>/app/<key>/<ts>`), unlike job
  mirrors (`agents/<CheckoutSegment>/`); two checkouts with one root or one basename interleave per
  goal without overwriting. Add the segment when someone reads them across seats.
- A launch postcondition that the clone's resolved root differs from the launching seat's (only an
  explicit setting could make them equal).
- `~/LocalStorage/agentic-tools-evidence` (100+ GB/day of source copies and private `gocache-*`
  dirs, pruned by hand) is the boundary of m1e-c4's disk-lifetimes design, Part B, amendment
  "unregistered consumers": the sweeper's floor report inventories the largest unregistered
  consumers with a reclaim command each and machinery deletes none of them. The design is not yet in
  the repository: its goal is `plans/goals/engine-owns-disk-lifetimes.md` (next step names
  `plans/designs/engine-owns-disk-lifetimes.md`), and the text is at
  `/Users/wido/LocalStorage/agentic-tools-evidence/disk-lifetimes-20260927/engine-owns-disk-lifetimes.md`
  (amendment at line 1649, dated 2026-09-29, which reads as the same clock error). Its GC is not
  designed here.

## Open questions for Wido

1. Catch-up mirror (brief question a): recommend NO, and not in step 1. Evidence: every placeholder
   checkout has zero job records; the only unmirrored terminal jobs are ten on m1d and m1e from
   2026-09-10, which sit on seats with explicit roots and belong to KI-6's sweep.
2. The shared root (brief question b): seven checkouts share
   `/Users/wido/metasystem-evidence/agentic-tools`, with no record of a decision (grepped
   `records/`, `plans/`, `memory/` in ui and m1e: only path mentions). It is safe for job mirrors
   because `dispatch.CheckoutSegment` keys them by checkout path (five segments are visible there,
   by design of `records/misc/multi-main-coexistence.md` lines 172-181); only app copies interleave.
   The launch's "roots must not mix" rule guarded a NEW machine's root, and the default now gives
   each machine its own. Recommend: leave all seven as they are, change no setting, and if you want
   m1e separated later it is one `.local` line after a heads-up, at the cost that metrics on m1e
   stop seeing the mirrors left under the old root.
3. The default naming: `<checkout basename>` puts `ui` at `agentic-tools-ui` while its explicit
   setting says `ui`. Keep the explicit setting, or drop the line and take the default?
4. Should `system start` refuse an explicitly invalid root (this design: yes, by the resolver's
   sentence, before the fence moves), or only print it and let the mirrors fail as today?
