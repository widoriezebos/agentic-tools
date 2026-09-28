# Build report: evidence-root-default, step 1 (U1 to U6)

Builder: Claude Opus 5.5, 2026-09-28. Worktree `/Users/wido/LocalStorage/GitHub/agentic-tools-erd`, branch
`evidence-root-default`. Spec: `metasystem/plans/evidence-root-default-design.md` revision 4, plus
`metasystem/plans/evidence-root-default-build-brief.md` and the coordinator's two mid-build
messages: the late m1e-c4 note, and the batch-9 merge note (`config.ResolvePath` replaced by
`realpath.Resolve`; the internal-verb registry fallback).

## Commits (first parent, oldest first)

| Commit | Unit |
|---|---|
| 80a8fa9b7 | U1 the owner, `config validate` on it, conf/project-rules/audit |
| e11d4ce89 | U2 the engine readers |
| 79cd1234a | U3 adoption and `system start` |
| 0e789e3f1 | coordinator's merge of main f2d662338 (batch 9) |
| 617727d58 | U2 follow-up: the unclosed-app-run fixture |
| 1b9600fae | U4 the launch |
| 0e35917a1 | U5 the structural guard |
| 498b80c4c | U6 `WriteRetired` |
| 14b410f1a | U3 follow-up: a missing conf names no root |
| (this report's commit) | report |

## U1. The owner, and `config validate` on it

Built: `internal/config/evidenceroot.go` holds `EvidenceRootKey`, `EvidenceRootParams{ConfPath, LookupEnv}`,
`EvidenceRoot{Path, Origin}` with `Default()` and `Line()`, and `ResolveEvidenceRoot`. The order is env
`METASYSTEM_EVIDENCE_ROOT`, then `ConfPath+".local"`, then `ConfPath`, then
`$HOME/metasystem-evidence/<checkout basename>`. A source counts as unspecified when the key is
absent, when the value is empty after trimming, or when the value is a `<...>` placeholder. A
set-but-empty env var is unspecified. A specified value that is relative, or inside the checkout, is
refused with its source named, e.g. `evidence.root must be absolute (metasystem.conf.local reads
"relative/dir")`. The checkout is found by walking up for a `.git` entry with `os.Lstat`; if there is
none, the installation itself is the checkout. `HOME` is read through the same lookup, and an unset
one is an error that names the fix. The resolver creates nothing.

`validate.go`'s evidence block is now one resolver call on the validated `confPath`, and it keeps
its `withinRepo` check against `--repo`. The committed `metasystem.conf` loses the placeholder line
and its comment now names the default. `docs/project-rules.md` gives the default and the `.local`
sentence. The `<durable evidence root, outside the repository>` alternative is removed from
`auditPlaceholderRe`.

Tests (written first; each failed to compile or failed its assertion before the change):
- `internal/config/evidenceroot_test.go`:
  - `TestResolveEvidenceRootOrder`: env over `.local`, `.local` over committed, committed over the default, the m1c-shaped placeholder fall-through, placeholder everywhere, set-but-empty env, empty value, absent key.
  - `TestResolveEvidenceRootRefusesSpecifiedBadValues`: relative `.local`, relative env, inside the checkout, the checkout itself, duplicate key, `HOME` unset.
  - `TestResolveEvidenceRootNamesTheCheckout`: a `.git`-less installation; nested `<repo>/metasystem`.
  - `TestResolveEvidenceRootJudgesTheFileItIsGiven` (ERD-03): candidate relative with standard absolute and the reverse, the candidate's own `.local`, a candidate with no `metasystem.conf` beside it.
  - `TestEvidenceRootLine`, `TestResolvePathThroughALinkedParent`.
  - `TestResolveEvidenceRootTreatsAMissingConfAsUnspecified`, added in 14b410f1a.
- `internal/config/validate_test.go`: the "evidence required" case is removed. `TestValidateJudgesTheEffectiveEvidenceRoot` is flipped: no key passes, the placeholder alone passes, relative `.local` refuses with its source. New: `TestValidateJudgesTheCandidateConfOnItsOwnRoot`.
- `internal/audit/coverage_test.go`: subtest `adopted legacy evidence placeholder passes` (ERD-04). The legacy line passes without `AllowPlaceholders`, and `<model>` in the same file still fails.

Deviations:
- `config.ResolvePath`: I exported it in U1 as designed (80a8fa9b7). The coordinator's batch-9 merge (0e789e3f1) then removed it in favour of main's `internal/realpath.Resolve`, which has the same semantics. `evidenceroot.go` and U4 use `realpath.Resolve`. `config` exports nothing wider than the design's resolver types.
- Missing committed conf (14b410f1a): at first a missing `ConfPath` was an error, as `ConfLookup` makes it. That made `system start` refuse an installation with no `metasystem.conf` (`TestSupBArmRefusesSurvivorsAndRemoteEvidenceUntilTerminal`), which breaks decision 5 ("unset never refuses"). A missing file is now unspecified, like an absent key. A conf that exists but cannot be read is still an error.

## U2. The engine readers

Built:
- `delegation.Config` gains `LookupEnv`, held on the `Lifecycle` (nil = `os.LookupEnv`). U9b had not landed, so this is the smallest seam. I put it on `Config`, not `Env`, because tests compare `Env` with `==`. `session.mirrorRecord` resolves through the owner; an error goes to `mirrorFail(job, err.Error())` and returns `exitWith(1)`.
- `dispatch.Mirror` spells the key through `config.EvidenceRootKey`.
- `evidenceGC`'s choice of root is split out as `evidenceGCTarget(checkout, explicit, lookup)`, a resolver call; an error is `evidence-gc refused: <sentence>`.
- `appRun` gains `lookupEnv`, carried from the new `intentOwners.lookupEnv` in `intent_app.go`. `preserveRunEvidence` resolves through the owner and prefixes the owner's error to the kept-record sentence.
- `metrics.evidenceRoot` returns the error; `loadJobs` turns it into one rejected coverage detail.
- The `stateroot` `Evidence` kind, its `StateRoot` branch, its `relativeRoot` case and its two tests are deleted.

Tests:
- `internal/delegation/evidence_root_test.go`:
  - `TestReapMirrorsUnderTheDefaultEvidenceRoot`: the manifest lands at `<home>/metasystem-evidence/<bed>/agents/<segment>/<chain>/manifest.json`.
  - `TestReapLogsARelativeEvidenceRootAndMirrorsNothing`: one log line in the owner's sentence, no mirror stamp, nothing under HOME.
- `cmd/metasystem/evidence_verbs_test.go:TestEvidenceGCTargetResolvesThroughTheOwner`.
- `cmd/metasystem/app_evidence_root_test.go:TestPreserveRunEvidenceUsesTheResolvedRoot`.
- `internal/metrics/evidence_root_test.go:TestJobsReportARefusedEvidenceRoot`.
- `internal/stateroot/stateroot_test.go:TestEvidenceIsNotAStateRootKind`.
- Follow-up 617727d58: `TestAppGoalRunWithoutAnEvidenceRootIsNotClosed` pinned the old "unset refuses" behaviour. It is renamed `...WithAnInvalidEvidenceRootIsNotClosed` and plants a relative `.local` root, which keeps the closure-refused-then-retried contract.

Deviations:
- The `evidenceGC` unit test drives the extracted target function, not the whole verb, because the verb requires a lease holder. `evidence.GC` itself is unchanged.
- The copy's file is `source-001-run.txt` under the timestamp directory (`proofrun.PreserveEvidence` names it so), not `run.txt`. The test globs for it.

## U3. Setup says it

Built:
- `adopt.Deps.LookupEnv`, and `evidenceRootNote` appended last to `result.Notes`. A resolver error is said, never raised.
- The checklist's step 3 now reads: "Optionally set models, tiers and the evidence root in metasystem.conf.local; the defaults above apply until you do."
- `processOwners.evidenceRoot`: nil means the owner over `<scope.Installation>/metasystem.conf`. It is called before the transition is built, so an error is a `processRefusal` with the plain `metasystem system start: ...` form before the fence moves. On success the line is prepended to `report.Lines`.

Deviation: the design says `<scope.Root>`. In this code `scope.Root` equals `scope.Installation` (`process_verbs.go:80`); I named the installation.

Tests:
- `internal/adopt/evidence_root_test.go:TestAdoptionNotesTheEvidenceRoot`: default note naming the target's basename; the `(metasystem.conf.local)` note.
- `cmd/metasystem/intent_adopt_evidence_test.go:TestSystemAdoptSaysTheEvidenceRootIsOptional`: failed on the old checklist text.
- `cmd/metasystem/intent_process_test.go:TestSystemStartSaysTheEvidenceRoot`: the default line comes first; a relative `.local` root refuses in the owner's sentence, with zero arm calls and the fence unchanged.

Re-pinned fixtures (existing tests that pinned now-changed behaviour):
- `internal/adopt/adopt_git_integration_test.go`: the adopted conf no longer carries a placeholder to fill, so once project-rules is filled the full audit passes (ERD-04), and the last note is the evidence-root line.
- `cmd/metasystem/supervision_bed_b_test.go`: the arm page expects the evidence-root line first.

## U4. The launch relies on the resolver and keeps its isolation checks

Built:
- `OSHost` gains an `Env` field. Nil means a lookup that answers only `HOME` from this process.
- New `OSHost.EvidenceRoot(installation)`.
- `OSHost.Canonical` is `realpath.Resolve`, which never fails and resolves existing ancestors.
- `OSHost.MakeDir` does `MkdirAll(parent)` and then the exclusive leaf `Mkdir`.
- `CopyLocalConf(source, destination)` blanks the key via `validate.SetConfKeys`.
- `Host` gains `EvidenceRoot`, and `CopyLocalConf` loses its third argument.
- In `Sequencer.configuration` the order is: copy (when `.local` is absent), then judge, then the manifest and the two validations. The step's words carry the clone's `Line()`.
- `Sequencer.evidenceRoot` is rewritten. It no longer runs `config get` and derives no sibling. It resolves the source's and the clone's roots in process, and refuses `CodeEvidenceRootUnsafe` when:
  - either resolution errors;
  - `Canonical(source) == Canonical(clone)`;
  - `MakeDir` did not create the root and `record.Created.EvidenceRoot` is false;
  - the leaf resolves elsewhere.

  `Created.EvidenceRoot` is persisted the moment the root is made; the resume exemption is unchanged.
- `seat_launch.go:seatLaunchFacts` fills the fact through the new `seatLaunchEvidenceRoot`, which returns a resolver error as `&launch.Refusal{Code: CodeEvidenceRootUnsafe, ...}` before the lock.
- The H1 guide sentence spells the key through `config.EvidenceRootKey`.
- `preflight.go` is changed in comments and one message word only (U5's sweep). Its rows 375-378 did not move.

Deviation, the link check: the design says to refuse "`Canonical(clone root)` differs from it after creation". With `Canonical` now resolving every ancestor, a literal comparison would refuse any root whose ancestor is a link. Examples: macOS `/var` → `/private/var`, or a `HOME` reached through a link. The launch integration test hit exactly this. The check now compares the leaf's resolution with `resolved(parent)/basename`. That still refuses a leaf that is a link elsewhere (`TestTheEvidenceRootRefusesADirectoryThatResolvesElsewhere`), and the ancestor aliasing is covered by the source-versus-clone comparison (ERD-02).

Tests:
- `sequence_test.go`, over `fakeHost.evidenceRoots` (source `/w/evidence/m1u`, clone `/w/evidence/agentic-tools-m1f`, origin `default`):
  - `TestTheCloneResolvesItsOwnEvidenceRootAfterTheCopy`: a blanked `.local` over a placeholder committed conf is accepted; the default is made by this launch and the words carry the line.
  - `TestTheEvidenceRootRefusesThisSeatsOwnRoot`: an inherited committed root equal to the source's.
  - `...ThisSeatsOwnRootThroughALink`, `...AResolverErrorOnEitherSide`, `...ADirectoryThatResolvesElsewhere`, `...ADirectoryThisLaunchDidNotCreate`.
  - The command list no longer contains `config get`. The two "record says this launch made it" cases are unchanged and green.
- `host_evidence_test.go` (new, no Git):
  - `TestCopyLocalConfKeepsEveryOtherLineAndBlanksTheEvidenceRoot`, `TestOSHostEvidenceRootNamesTheDefaultUnderHome`.
  - `TestLaunchRefusesMissingSourceRootAlias` (ERD-02): refused as "this seat's own root", and `<bed>/real/new` is not created.
  - `TestLaunchCreatesDefaultRootWithoutEvidenceParent` (ERD-06): the default is created under a HOME with no `metasystem-evidence` and `Created.EvidenceRoot` is true; on a fresh record the occupied leaf is refused "already there and this launch did not create it".
- `integration_test.go`:
  - The source `.local` names the bed's `evidence/m1u`, and the committed conf names `<bed>/evidence/committed`.
  - Asserted: the copied `.local` carries a blank key; the clone's resolved root is the committed path with origin `conf`; the launch created that root.
  - The canned `config get` answer is gone, and so is `mustCanonical`, which became unused.

Refusal-register rows re-pinned (`internal/refusal/register.go`):

| Row | Old site | New site |
|---|---|---|
| `SEAT_LAUNCH_DISK_SHORT` | `sequence.go:354` | `:360` |
| `SEAT_LAUNCH_EVIDENCE_ROOT_UNSAFE` | `:553` | `:552` |
| its `ForwardSite` | `refusal.go:75` | `refusal.go:77` |
| `SEAT_LAUNCH_WORD_REQUIRED` | `:664` | `:677` |
| `SEAT_LAUNCH_SUPERVISION_DOWN` | `:703` | `:716` |
| `SEAT_LAUNCH_IDENTITY_UNREADABLE` | `:731` | `:744` |

The ForwardSite moved because U4's `config` import added two lines. `SEAT_LAUNCH_DISK_SHORT` moved too, because the `Host` interface grew above it; the design expected it to hold.

## U5. The structural guard

Built: `internal/config/evidenceroot_owner_test.go:TestTheEvidenceRootKeyIsSpelledOnlyByItsOwner` walks
`../..` over `cmd`, `internal` and `scripts`, reading every non-test `*.go` and every `*.sh` and
skipping `node_modules`. It fails by path on the substring outside `internal/config/evidenceroot.go`.
Comments count. When first written it failed on `internal/seat/launch/preflight.go`, the last
spelling left after U2 to U4, which is now swept. `cmd/metasystem/adoption_comparison_test.go`'s
`fillAdoptionHarnessConf` appends the fixture root when the template carries no key.

The HOME-hygiene proof, by hand: both test sets in the proof below ran with `HOME` pointed at an
empty scratch directory (`.../scratchpad/emptyhome2`). Afterwards it held only the Go toolchain's
own telemetry files (`Library/Application Support/go/telemetry/...`, 13 entries) and no
`metasystem-evidence`. `/Users/wido/metasystem-evidence` was unchanged throughout: the same seven
entries with their same timestamps.

## U6. The retirement pointer

Built: `internal/evidence/retired.go:WriteRetired(root, checkouts, now) (changed, err)` writes
`<root>/RETIRED.json` as `{"schemaVersion":1,"retiredAt":"<RFC3339 UTC>","checkouts":[...],"successor":"per-checkout
default","rule":"evidence-root-default"}` and a newline, through `atomicfile.WriteText`. It is idempotent:
- an existing pointer keeps its `retiredAt`;
- the checkouts are the union of the file's and the call's, cleaned, deduplicated and sorted;
- identical content leaves the bytes alone (`changed == false`).

It refuses a relative root or checkout, an empty checkout list, a root that is not a directory, and
an existing file that is not a pointer.

Tests: `internal/evidence/retired_test.go`:
- `TestWriteRetiredIsIdempotent`: exact format and bytes; a later clock changes nothing; a new checkout is added with `retiredAt` kept; a subset removes nothing.
- `TestWriteRetiredRefusesWhatIsNotAPointer`.

`internal/evidence` coverage is 81.8% against a floor of 80.5.

Deviation, the verb is not built: `metasystem internal evidence retire` is not added. After batch 9,
`TestVerbRatchetInternalVerbCount` holds `verbRatchetInternalVerbCeiling = 85` and the measured count
is 85, so a new internal verb would fail the ratchet. Per the coordinator's instruction I kept
`WriteRetired` and its test and dropped the verb; the migration runs from a test-only helper or by
hand. Nothing was run against `/Users/wido/metasystem-evidence`.

## Proof

1. `cd metasystem && METASYSTEM_TESTING_WORKERS=9 go run ./cmd/devgate static` at 14b410f1a: **one check red, every other check green**.
   - The red check is the Stop decision surface audit: `stop decision surface merge base: got 2 commits, want one`.
   - Cause: branch topology, not code. After the coordinator's merge 0e789e3f1, HEAD and `origin/main` (af6206b6d) have two merge bases, f9385b1ca and f2d662338 (`git merge-base --all HEAD origin/main`). The audit refuses an ambiguous base, and devgate has no base override.
   - The same audit run by hand against the branch's own main, `go run ./cmd/metasystem internal audit stop-decision-surface --root . --base f2d662338`: `added 0, moved 0, removed 0`, exit 0.
   - Against the other base, f9385b1ca, it reports batch 9's own moves (the `verbs-match-intent` declaration), none of them this slice's.
   - The gate turns green once the branch is rebased onto, or merged with, the current `origin/main` so there is one merge base. I did not rewrite history or touch refs.
2. With `HOME` set to an empty scratch directory and `METASYSTEM_TESTING_WORKERS=9`, at 14b410f1a:
   - `go test -count=1 ./internal/config/ ./internal/audit/ ./internal/dispatch/ ./internal/delegation/ ./internal/metrics/ ./internal/stateroot/ ./internal/adopt/ ./internal/seat/launch/ ./internal/evidence/ ./internal/refusal/`: all ten ok.
   - `go test -count=1 -timeout 60m ./cmd/metasystem/ -run 'Launch|Machine|Seat|Adopt|Config|Evidence|App|Audit'`: ok (798s).
   - Earlier, targeted runs of `-run 'TestSupB|TestSystemStart|TestIntentProcess|Stop|Start'`: ok.
3. Set-wrong proof for every re-pinned row: each Site was set off by 12 to 22 lines, one at a time. `TestHCL03EveryRowedSiteNamesAnEmission` failed naming that row ("does not name an emitted string or identifier within the 2-line emission window") and passed again once restored. The `ForwardSite` set wrong to `refusal.go:40` failed `TestH1EveryRowHasAStanding` and passed once restored. Final `go test ./internal/refusal/`: ok.
4. `METASYSTEM_TESTING_WORKERS=9` was used for every run above.

## For the rebase (m1e-c4)

- `internal/adopt`: one `Deps.LookupEnv` field, one `evidenceRootNote` function and one `Notes` append at the end of `Adopt`, plus the new `internal/adopt/evidence_root_test.go`. The integration test's placeholder loop is replaced by the ERD-04 assertion.
- Path resolution: nothing new. `evidenceroot.go` and `seat/launch/host.go` call `realpath.Resolve`.
- Nothing is exported from `config` beyond `EvidenceRootKey`, `EvidenceRootParams`, `EvidenceRoot` and `ResolveEvidenceRoot`.

## Left to the coordinator

- The migration's last act: dropping the hand-set line from the nine `.local` files, writing the `RETIRED.json` pointers (via `evidence.WriteRetired`, since the verb is not built), and removing the empty `.../ui` directory.
- Bringing the branch to one merge base with `origin/main`, so the static gate's Stop-surface audit can run.
