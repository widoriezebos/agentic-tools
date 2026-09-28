# The evidence root has one owner and a default

Design, 2026-09-28, revision 4. Author: Fable (design lane). Approved shape: Wido, 2026-09-28.
Revision 2 folded Astra's round 1 (`plans/evidence-root-default-astra-critique-r1.md`); revision 3
folded round 2 (`plans/evidence-root-default-astra-critique-r2.md`, the failsafe round) as two
fixture obligations in U4; revision 4 folds Wido's answers to the open questions (Decided, below)
as a migration step with no new mechanism. Critique closed. Dispositions at the foot. Status: build
on Opus after `embed` and `u9b` land (see Sequencing).

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
   No new package, so no new coverage floors. The resolver takes the CONF FILE PATH, as `config.Get`
   does, and derives `.local` as `ConfPath + ".local"`; the checkout is found from the file's
   directory. A caller validating `candidate.conf` is judged on `candidate.conf`, never on the
   `metasystem.conf` beside it (ERD-03).
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
6. The launch stops deriving a sibling and relies on the resolver for the new machine: the copied
   `.local` has its `evidence.root` blanked so the seat's own root does not travel, and the clone
   then resolves like any checkout, to its committed conf if that carries a concrete root, else to
   `~/metasystem-evidence/<repository>-<nickname>`. The launch KEEPS its three isolation checks and
   judges the clone's RESOLVED root after the configuration is written: it refuses a fresh launch
   when that root is the source seat's, already exists and was not created by this launch, or
   resolves through a link, with the resume exemptions as today (ERD-01, ERD-02). A distinct
   destination does not prove a distinct root (`/elsewhere/ui` would resolve to this seat's
   `~/metasystem-evidence/ui`), which is why the checks stay. Writing the default explicitly would
   freeze it and duplicate the rule; deriving a sibling needed the seat's root to exist, which is
   the gap this design closes.
7. The committed `metasystem.conf` loses its `evidence.root=<placeholder>` line (a comment names the
   default), and the placeholder audit loses that alternative, so an adopted repository still
   carrying the old line passes its ordinary audit instead of being told to fill it (ERD-04).
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
adopters have nothing to fill), `internal/audit/metasystem.go:auditPlaceholderRe` (the `<durable
evidence root, outside the repository>` alternative removed; every other placeholder check stays),
`internal/audit/coverage_test.go` (the adopted-repository placeholder case at ~line 239 gains a
fixture). `internal/config/validate.go:resolvePath` (~line 820: absolute, symlinks followed on the
deepest EXISTING ancestor, the missing tail re-attached lexically) is exported as
`config.ResolvePath`, an in-package rename of its seven call sites, because U4 compares roots with
that exact algorithm; exporting one identifier is smaller than the ninth private copy of it (eight
packages carry one today).

```go
const EvidenceRootKey = "evidence.root"          // the only file that spells it
type EvidenceRootParams struct {
    ConfPath  string                           // the conf file judged; .local is ConfPath+".local"
    LookupEnv func(string) (string, bool)      // nil: os.LookupEnv; answers HOME too
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
reads "relative/dir")`, `evidence.root must be outside the repository (...)`. `validate.go` passes
its actual `confPath` (`runConfigValidate --conf` and `intent_owner_calls.go:configValidate` forward
a caller-chosen filename, an existing supported case) and additionally keeps its
`withinRepo(path, repo)` check against `--repo`, since the resolver's checkout is the `.git` walk
and `--repo` may differ. Every other caller passes `<installation>/metasystem.conf`.

Tests (all in `evidenceroot_test.go`, table-driven over a fixture installation with a `.git` file
two levels up and a map-backed `LookupEnv`): env wins over `.local`; `.local` over committed;
committed over default; placeholder in `.local` falls through to a committed real path (the
m1c-shaped case, named as such); placeholder everywhere resolves to
`<home>/metasystem-evidence/<checkout basename>` with `Origin=="default"`; empty and set-but-empty
env are unspecified; a relative `.local` value is refused naming `.local`; a value inside the
checkout is refused; `HOME` unset is an error; a `.git`-less installation names its own basename; a
nested `<repo>/metasystem` names `<repo>`; duplicate key is an error; two neighbouring files in one
directory, `metasystem.conf` with an absolute root and `candidate.conf` with a relative one, each
resolve by their own file (the candidate refuses, the standard file passes, and the reverse), and
`candidate.conf` with no `metasystem.conf` beside it still resolves (ERD-03). `validate_test.go`: a
conf with no key passes; the placeholder alone passes; relative and in-repository still refuse; a
`candidate.conf` beside a valid `metasystem.conf` is refused on its own relative root.
`internal/audit/coverage_test.go`: an adopted repository whose `metasystem.conf` keeps
`evidence.root=<durable evidence root, outside the repository>` passes without `AllowPlaceholders`,
while `<model>` in the same file still fails (ERD-04).

### U2. The engine readers

Files and functions:
- `internal/delegation/reap.go:session.mirrorRecord`: resolver over `<s.root>/metasystem.conf` with
  the lookup U9b threads through the session; an error goes to `mirrorFail(job, err.Error())` and
  returns `exitWith(1)` exactly as today (KI-6's shape is untouched).
- `internal/dispatch/mirror.go:Mirror`: logic unchanged; the message uses `config.EvidenceRootKey`
  (dispatch already imports config).
- `cmd/metasystem/evidence_verbs.go:evidenceGC`: the `config.Get` replaced by the resolver when
  `evidenceRoot == ""`; an error is the refusal line.
- `cmd/metasystem/app.go:appRun.preserveRunEvidence`: resolver over
  `<r.roots.Installation>/metasystem.conf` with `intentOwners.lookupEnv` (new field beside
  `commandNow`, nil = `os.LookupEnv`, carried onto `appRun`); the "record is kept" sentence stays,
  prefixed by the resolver's error.
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

- `internal/adopt/adopt.go:Adopt`: after the payload lands, resolve over the target's
  `metasystem.conf` with a new `Deps.LookupEnv` (nil = `os.LookupEnv`) and append
  `EvidenceRoot.Line()` to `result.Notes`. `cmd/metasystem/intent_adopt.go:runIntentAdopt` checklist step 3 becomes
  "Optionally set models, tiers and the evidence root in metasystem.conf.local; the defaults above
  apply until you do." No test pins the old checklist text (grepped `cmd/metasystem/*_test.go`,
  `internal/adopt/*_test.go`).
- `cmd/metasystem/process_verbs.go:processOwners.arm`: a new `evidenceRoot func(conf string)
  (config.EvidenceRoot, error)` field (default: the resolver over `<scope.Root>/metasystem.conf`),
  called before `armSteps`; its line is
  the first of `report.Lines`; an error is a `processRefusal` with the resolver's sentence and the
  plain `metasystem system start: ...` form, before the fence moves.

Tests: `internal/adopt/adopt_test.go` (fake Git, as its existing cases): a target with the template
conf (placeholder) yields the default note naming the target basename; a target whose `.local` sets
a root yields the `(metasystem.conf.local)` note. `cmd/metasystem/intent_process_test.go` (injects
`armSteps` already): a start with nothing configured prints the default line; a relative `.local`
root refuses before the fence with the resolver's sentence.

### U4. The launch relies on the resolver and keeps its isolation checks (ERD-01, ERD-02)

- `internal/seat/launch/host.go`: `OSHost.CopyLocalConf(source, destination)` (two arguments)
  blanks the key via `validate.SetConfKeys(name, {Key: config.EvidenceRootKey, Value: ""})`; blank
  is unspecified (decision 2; a `DropConfKeys` is Later), so the clone falls through to its
  committed conf and then the default. New `OSHost.EvidenceRoot(installation string)
  (config.EvidenceRoot, error)` resolves `<installation>/metasystem.conf` with a lookup that
  answers only `HOME` from this process, which is the scrubbed view the machine's own steps run
  under (`OSHost` gains an `Env func(string) (string, bool)` field, nil meaning that lookup, so a
  filesystem fixture can point `HOME` at a bed without `t.Setenv`). `Host` gains `EvidenceRoot` and
  keeps `MakeDir` and `Canonical`. `OSHost.Canonical` becomes `config.ResolvePath`: a path whose
  leaf is absent is still resolved through its existing ancestors, so `<bed>/alias/new` and
  `<bed>/real/new` compare equal when `alias` links to `real` (ERD-02); once the leaf exists the
  answer is the full canonical path, so the redirect refusal below is unchanged. `OSHost.MakeDir`
  first creates the missing parents with `os.MkdirAll(filepath.Dir(path), 0o755)`, then keeps the
  exclusive leaf `os.Mkdir` and its created-versus-existing answer, so the first launch on a host
  whose `$HOME/metasystem-evidence` does not exist yet can make the clone's default root (ERD-06);
  this is the one place in step 1 that creates a parent, and it is a writer. The launch package
  imports `internal/config` (no cycle: config imports nothing above it).
- `internal/seat/launch/sequence.go:Sequencer.configuration`: the order becomes copy (when `.local`
  is absent), then judge, then the manifest and the two validations. `Sequencer.evidenceRoot` is
  REWRITTEN, not deleted: it no longer runs `config get` and derives no sibling. It resolves the
  SOURCE seat's root (`Host.EvidenceRoot(sourceInstall)`) and the CLONE's root
  (`Host.EvidenceRoot(destinationInstall)`) after the configuration is written, and refuses
  `CodeEvidenceRootUnsafe` when: either resolution errors (an explicitly invalid root, in the
  resolver's sentence); the clone's root is the source's, compared as
  `Host.Canonical(source) == Host.Canonical(clone)` with `Canonical` resolving existing ancestors
  whether or not the leaf exists (ERD-02: a missing leaf under a symlinked parent is the same
  directory as its target's; a plain clean would call them different); `MakeDir(clone root)` did
  not create it and `record.Created.EvidenceRoot` is false; or `Canonical(clone root)` differs
  from it after creation. `record.Created.EvidenceRoot` is set and persisted the
  moment the directory is made, exactly as today, and the resume exemption is unchanged. The
  judgement runs on every pass of the step, resume included; `MakeDir` is a no-op on a directory the
  record says this launch made. The step's words carry the clone's `EvidenceRoot.Line()`. A clone
  whose committed conf carries a concrete root resolves to it once `.local` is blanked and is
  accepted only if it is not the source's, not already there and not a link; a committed root the
  source seat also uses is refused with the H1 remedy (a root of its own in `.local`).
- `internal/seat/launch/preflight.go`: unchanged; `Facts.EvidenceRoot` and `evidenceRootSet` stay.
  `cmd/metasystem/seat_launch.go:seatLaunchFacts` fills the fact from
  `config.ResolveEvidenceRoot` over the seat's conf (the `config.Get` goes) and returns a resolver
  error as `&launch.Refusal{Code: launch.CodeEvidenceRootUnsafe, Message: err.Error()}` before the
  lock, the shape `CodeFleetUnreadable` already takes at line 266.
- `internal/seat/launch/refusal.go`: `CodeEvidenceRootUnsafe` and the H1 branch of `Refusal.Error`
  stay; the guide sentence spells the key through `config.EvidenceRootKey`.
- `internal/refusal/register.go`: row `SEAT_LAUNCH_EVIDENCE_ROOT_UNSAFE` stays; its `Site` is
  re-stated to the rewritten function's first `refuse(CodeEvidenceRootUnsafe, ...)` line.

Tests: `sequence_test.go`: the four `TestTheEvidenceRoot...` cases (~262-330) stay, adapted to
resolver-derived paths through `fakeHost.evidenceRoots map[string]config.EvidenceRoot` keyed by
installation (source `/w/evidence/m1u`; clone `/w/evidence/agentic-tools-m1f`, origin `default`): a
clone resolving to the source's root refuses (an inherited committed root); a clone root already
present refuses; one resolving elsewhere refuses; a resolver error on either side refuses in its
sentence; the placeholder case becomes "a blanked `.local` over a placeholder committed conf
resolves to the default and is accepted". The two "record says this launch made the evidence root"
cases (~463-482) stay. New: the copied conf's key is blanked; the step's words carry the line.
`preflight_test.go`: unchanged. `host_test` (new, no Git): `CopyLocalConf` keeps every other line
and blanks the key; `EvidenceRoot` on a fixture installation names
`<HOME>/metasystem-evidence/<basename>`. Two fixture obligations from the failsafe round, both
filesystem tests over `OSHost` driving `Sequencer.evidenceRoot` directly on two temp installations
(no Runner, no Git): `TestLaunchRefusesMissingSourceRootAlias` (ERD-02) makes `<bed>/real`, links
`<bed>/alias` to it, gives the source `.local` `evidence.root=<bed>/alias/new` and the clone's
committed conf `evidence.root=<bed>/real/new`, both leaves absent, and asserts
`CodeEvidenceRootUnsafe` and that `<bed>/real/new` was not created;
`TestLaunchCreatesDefaultRootWithoutEvidenceParent` (ERD-06) points `Env("HOME")` at a bed with no
`metasystem-evidence` directory, leaves the clone's conf without the key, and asserts the default
root is created with `record.Created.EvidenceRoot` true, then, on a fresh record with the leaf
pre-created, asserts the "already there and this launch did not create it" refusal still holds. `integration_test.go` (the one narrowly named real-Git
test): the source `.local` names the bed's created `evidence/m1u` directory (today
`/the/launching/seat`, which the source-side comparison would now read as absent); the committed
conf keeps a concrete root but as a bed path, `<bed>/evidence/committed` (the literal `/not/this/one`
would make `MakeDir` fail on this host and turn the test into an environment-dependent refusal);
the test asserts the clone's RESOLVED root is that committed path (through
`config.ResolveEvidenceRoot` over the clone's conf with a `HOME`-only lookup), that the launch
created it, and that the copied `.local` carries the blank key; the canned `config get` answer
(line 72) goes.

### U5. The structural guard

`internal/config/evidenceroot_owner_test.go`: walks the module (`../..`, as
`internal/layering/r9_test.go` does) over `cmd/**`, `internal/**` and `scripts/**`, every `*.go` not
`_test.go` and every `*.sh`, skipping `node_modules`, and fails by path when any file other than
`internal/config/evidenceroot.go` contains the substring `evidence.root`. Comments count,
deliberately: the spelling lives in one place, and prose says "the evidence root". This unit also
sweeps the last spellings U2-U4 left (`evidence_verbs.go:21`, `sequence.go:65`, `preflight.go:67`
and `:90`, `refusal.go:75`, `host.go` comments, `validate.go` comment) and is green only once
nothing outside the owner names the key.

Fixture hygiene, a safety matter of this unit: until now a bed that copied the template conf and
never set the key mirrored nothing; from U1 on it resolves to a real directory under the test
process's `$HOME`. Every bed that reaches a mirror or an app copy must name a root inside its temp
dir. The known one: `cmd/metasystem/adoption_comparison_test.go` (~line 437) rewrites the template
conf key by key and substitutes its fixture root only when the line is present, so it must append
the key when absent. The build proves the rest once, by hand, not by mechanism: run the Go tests
with `HOME` pointed at an empty scratch directory and assert that directory is still empty
afterwards; any bed that wrote there names its root.

### U6. The retirement pointer

`internal/evidence/retired.go:WriteRetired(root string, checkouts []string, now time.Time) (changed
bool, err error)` writes `<root>/RETIRED.json` exactly as m1e-c4 fixed it:
`{"schemaVersion":1,"retiredAt":"<RFC3339 UTC>","checkouts":["<absolute checkout path>", ...],
"successor":"per-checkout default","rule":"evidence-root-default"}`. Idempotent: an existing file is
read first; its `retiredAt` is kept; the checkouts are the union of the file's and the call's,
deduplicated and sorted, so a rerun with the same content leaves the file alone (`changed ==
false`, bytes identical) and a new checkout is added, never removed. The write is atomic through
`internal/atomicfile.WriteText` (temp file, then rename). Its caller is the one new verb of this
design, internal and unavoidable because the migration must be rerunnable by the engine's own
writer: `metasystem internal evidence retire --root ROOT --checkout PATH` (repeatable), in
`cmd/metasystem/evidence_verbs.go` beside `evidence-gc`, refusing a relative root or checkout.
Part B's census owns the reading.

Test: `internal/evidence/retired_test.go:TestWriteRetiredIsIdempotent` (parallel, temp dir, no
Git): the first write produces the format with the given checkouts and `retiredAt`; a second write
with the same checkouts and a later clock changes nothing (bytes equal, `changed == false`); a
third with one more checkout adds it, keeps `retiredAt`, reports `changed == true`; a fourth with a
subset removes nothing. `internal/evidence`'s floor (80.5) holds with it.

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

- The resolver honours an explicit absolute value wherever a person sets one (`Origin` names the
  source). On this host, though, every hand-set value moves onto the convention (Wido, Decided Q2
  and Q3), so that nobody needs to set it and the defaults just work.
- Placeholder seats start resolving to `~/metasystem-evidence/<basename>` on first use; they have
  nothing to migrate (zero records each).
- Hand-set roots move onto the convention, as the LAST act of the build, after U5 is on main and
  each seat runs an engine built from it (dropping the line earlier would make every live seat
  mirror against the placeholder, which fails silently under KI-6). Per checkout, in this order:
  1. Write the pointer in the root being retired, so m1e-c4's disk census reads it as "retired root
     of <checkouts>, N bytes" instead of a silent orphan: `<old root>/RETIRED.json`, m1e-c4's fixed
     format, written by U6's `metasystem internal evidence retire --root <old root> --checkout
     <absolute path>...`. `/Users/wido/metasystem-evidence/agentic-tools` lists `agentic-tools`,
     `agentic-tools-m1b`, `agentic-tools-m1b-retired-20260906`, `agentic-tools-m1c`,
     `agentic-tools-m1e`, `agentic-tools-paper`, `agentic-tools-slc-r4` (all under
     `/Users/wido/LocalStorage/GitHub/`); `/Users/wido/metasystem-evidence/agentic-tools-m1d` lists
     `agentic-tools-m1d` (its explicit root equals its default, but the line goes too and the
     non-empty root keeps the pointer). `/Users/wido/metasystem-evidence/ui` is empty and is simply
     removed; no pointer, nothing to orphan.
  2. Drop the one key from `<checkout>/metasystem/metasystem.conf.local` with a line that reads
     nothing else and prints nothing: `sed -i '' '/^evidence\.root=/d' <local>` (BSD sed, no
     backup copy of a secrets file). The engine's own writer is not used: `validate.SetConfKeys`
     cannot delete and `config tailor` rewrites a whole conf for a runtime set. The rest of the file
     is never read, printed or diffed. Checkouts: the eight above plus `agentic-tools-ui`.
  3. Verify with the value only: `bin/metasystem config get --key evidence.root --conf
     <checkout>/metasystem/metasystem.conf` no longer prints the hand-set path, and the seat's next
     `system start` prints `evidence root: /Users/wido/metasystem-evidence/<basename> (default; ...)`.
- Nothing moves: old evidence stays under the old roots and stays readable by hand, named by its
  pointer. The cost, stated: `internal/metrics/data.go:loadJobs` reads mirrored records only under
  the CURRENT root, so metrics on those seats stop counting the old mirrors from the moment the
  line is dropped. m1e has been told. m1c keeps its committed placeholder, harmless under the
  resolver (it resolves to the default like every other checkout).
- Machines launched earlier keep the explicit sibling root in their `.local`. Launch records keep
  `Created.EvidenceRoot`; a resume of an earlier launch is judged as before.
- Adopted repositories carrying the old committed placeholder resolve to the default through the
  transition rule AND pass their ordinary audit, because U1 removes that alternative from
  `auditPlaceholderRe`; without that they would keep failing `internal audit metasystem` for a
  value nothing requires any more (ERD-04).
- `metasystem config get --key evidence.root` keeps working as a generic read; nothing in the engine
  uses it for this key any more.
- `/Users/wido/LocalStorage/agentic-tools-evidence` (22 hand-named run trees such as
  `review-room-20260928`, `batch-lane-20260928`; six of m1e's worktrees live under
  `agent-help-20260926/`; referenced by nine plans documents as archives) is unrelated to
  `evidence.root`: no conf key points there, no reader or writer of the engine touches it, and the
  default keeps a separate tree, `~/metasystem-evidence`. It stays where it is.

## Pinned checks to re-pin

- `internal/refusal/register.go`, ±2-line window (`register_test.go:refusalSiteWindowRadius = 2`):
  row 381 (`SEAT_LAUNCH_EVIDENCE_ROOT_UNSAFE`, today `sequence.go:553`) STAYS and is re-stated to
  the rewritten `evidenceRoot`'s first refusal line; its `ForwardSite refusal.go:75` and H1
  `StandingGuide` forward `machine start` hold (the guide sentence changes wording only through the
  constant). Re-pin `SEAT_LAUNCH_WORD_REQUIRED` (`sequence.go:664`),
  `SEAT_LAUNCH_SUPERVISION_DOWN` (`:703`), `SEAT_LAUNCH_IDENTITY_UNREADABLE` (`:731`) by whatever
  the rewrite above them shifts; `SEAT_LAUNCH_DISK_SHORT` (`:354`) sits above it and holds. The
  `preflight.go` rows (375-378) do not move: the file is unchanged. No row names `validate.go`,
  `app.go`, `evidence_verbs.go`, `reap.go`, `data.go`, `process_verbs.go`, `intent_adopt.go`,
  `adopt.go`, `audit/metasystem.go` or `seat/launch/host.go`, so revision 3's `OSHost.MakeDir` and
  `Canonical` changes and the `ResolvePath` rename in `config/validate.go` move no pinned site.
- `testing-parallel-ratchet.json`: no new serial tests; deletions lower counts, which the ratchet
  allows.
- Coverage ratchet (full gate only, `scripts/agents/coverage-ratchet.json` and
  `coverage-ratchet-linux.json`): no new package, so no new floor; the touched packages' floors must
  hold: `internal/config` 82.7, `internal/dispatch` 75.9, `internal/delegation` 72.1,
  `internal/metrics` 85.0, `internal/stateroot` 86.0, `internal/seat/launch` 74.6, `internal/adopt`
  81.3, `internal/evidence` 80.5 (U6's `WriteRetired` fully covered by its test); `cmd/metasystem`
  is exempt. U6's internal verb adds a row to `cmd/metasystem/main.go`'s verb table and no
  refusal-register row (it refuses in prose, unrouted).
- Stop decision surface audit (`cmd/devgate/owners.go:stopSurface`): the deleted tests carry no
  `Verdict{ShouldBlock` line; not triggered. SessionStart exit audit: no hook touched.
- `internal/layering/r9_test.go`: `internal/config` is neither an owner nor a composition package;
  unaffected.
- `cmd/metasystem/adoption_comparison_test.go` (~line 437) must append the key it can no longer
  substitute (U5, fixture hygiene); `internal/validate/conftailor_test.go` writes its own fixture
  conf with the key and is unaffected. `internal/audit/coverage_test.go` ~line 239 (one violation,
  "unreplaced placeholders") keeps passing only if its fixture's placeholder is not the evidence
  one; the build checks which placeholder it plants and adds the ERD-04 case beside it.
  `internal/audit/shipped_installation_test.go` audits the shipped template with
  `AllowPlaceholders: true` and is unaffected.

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
- ERD-05: the structural guard protects the spelling, not ownership; a reader could call
  `config.Get` with `config.EvidenceRootKey` and keep a private fallback unseen. Extend the guard to
  reads through the exported key when a second owner appears; step 1 migrates every current reader
  by hand.
- `~/LocalStorage/agentic-tools-evidence` (100+ GB/day of source copies and private `gocache-*`
  dirs, pruned by hand) is the boundary of m1e-c4's disk-lifetimes design, Part B, amendment
  "unregistered consumers": the sweeper's floor report inventories the largest unregistered
  consumers with a reclaim command each and machinery deletes none of them. The design is not yet in
  the repository: its goal is `plans/goals/engine-owns-disk-lifetimes.md` (next step names
  `plans/designs/engine-owns-disk-lifetimes.md`), and the text is at
  `/Users/wido/LocalStorage/agentic-tools-evidence/disk-lifetimes-20260927/engine-owns-disk-lifetimes.md`
  (amendment at line 1649, dated 2026-09-29, which reads as the same clock error). Its GC is not
  designed here.
- The blob store: the large bytes under `~/LocalStorage/agentic-tools-evidence` were Go caches and
  source copies from hand-run builders, not duplicate evidence; m1e-c4's Part B revision 4c makes
  the blob store one per host and refuses caches under evidence. This design does nothing about it.

## Decided (Wido, 2026-09-28)

Wido's rule, verbatim: "I want the easiest solution, by convention if possible, without anybody
ever needing to set these themselves (because the defaults just work) rarely having to override
these."

1. Catch-up mirror (brief question a): NO, as recommended. Every placeholder checkout has zero job
   records; the only unmirrored terminal jobs are ten on m1d and m1e from 2026-09-10, on seats
   with explicit roots, and belong to KI-6's sweep.
2. The shared root (brief question b): REVERSED against the recommendation to leave it. Seven
   checkouts shared `/Users/wido/metasystem-evidence/agentic-tools` with no record of a decision
   (only path mentions in `records/`, `plans/`, `memory/` of ui and m1e); job mirrors were safe
   there because `dispatch.CheckoutSegment` keys them by checkout path. Hand-set values move onto
   the convention: the build drops the `evidence.root` line from the `.local` of every checkout that
   set one by hand (main, m1b, m1b-retired-20260906, m1c, m1e, paper, slc-r4, ui, and m1d whose
   explicit root equals its default), leaves a pointer in each retired root, moves no evidence, and
   accepts that metrics stop counting the old mirrors (Migration). m1e has been told; m1c keeps its
   committed placeholder.
3. `ui`: takes the default, `/Users/wido/metasystem-evidence/agentic-tools-ui`; its line is dropped
   with the others and the empty `.../ui` directory is removed.
4. `system start` refuses an explicitly invalid root, as designed (U3): by the resolver's sentence,
   before the fence moves. Unset never refuses.

## Dispositions of Astra's rounds 1 and 2

| Finding | Disposition | Folded where |
|---|---|---|
| ERD-02 (round 2, reopened) missing roots can alias through a symlinked parent | accepted, fixture obligation | U4: roots compared through `Host.Canonical` = `config.ResolvePath` (exported in U1), which resolves existing ancestors when the leaf is absent; the "cleaned where it does not exist" exception withdrawn; fixture `TestLaunchRefusesMissingSourceRootAlias` |
| ERD-06 (round 2) `OSHost.MakeDir` fails on a fresh default evidence tree | accepted, fixture obligation | U4: `MkdirAll` of the parent, then the exclusive leaf `Mkdir` with its created-versus-existing answer; fixture `TestLaunchCreatesDefaultRootWithoutEvidenceParent`, which also asserts an occupied leaf is still refused |
| ERD-01 blanking `.local` does not make a launch use the default | accepted | Decision 6; U4: the clone's RESOLVED root is judged after its configuration is written, so an inherited committed concrete root is covered by the same three checks; integration fixture asserts the resolved destination root |
| ERD-02 an unused destination does not prove an unused root | accepted | Decision 6; U4: the three isolation checks stay (source's root, existing and not created by this launch, link elsewhere) on the resolver's result, resume exemptions as today; `SEAT_LAUNCH_EVIDENCE_ROOT_UNSAFE` stays, row re-stated; the Later "launch postcondition" bullet withdrawn |
| ERD-03 the resolver cannot judge the file explicitly requested | accepted | Decision 1; U1: `EvidenceRootParams.ConfPath`, `.local` derived as `ConfPath + ".local"`, validate passes its own `confPath`; two-neighbouring-files test |
| ERD-04 legacy placeholders stay mandatory under the audit | accepted | Decision 7; U1: the alternative removed from `auditPlaceholderRe`, adopted fixture keeping the old line in `internal/audit/coverage_test.go`; Migration |
| ERD-05 the guard protects spelling, not ownership | deferred | Later |
