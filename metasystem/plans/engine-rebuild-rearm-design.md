# Design: a rebuilt engine at its enrolled path re-arms itself; every other drift cause still reaches the human

Goal: engine-rebuild-rearms-itself (plans/goals/engine-rebuild-rearms-itself.md,
revision 7, tier 3 DESIGN-BEARING). Author: implementer delegate under dispatch
by m1 (lineage main-1788594343-3833-fb64b9). Revision 1 was written by job
engine-rebuild-rearm-design-r1c-20260906 and landed at a4c5947d5; this is
**Revision 2, 2026-09-06**, written by job engine-rebuild-rearm-fold2-20260906.
Revision 2 folds the eight material findings of the round-1 critique register
(`records/misc/engine-rebuild-rearm-critique-r1.md`, landed 66702fdcc) by id,
and corrects the one non-material label. Every seam cited below was read in
this worktree at commit 66702fdcc; the prior implementation was read as the
diff between 2477e73d5 and df0597910 on branch m1-2026-09-05-verified. Line
numbers are HEAD's unless marked "branch".

The prior implementation is input, not the landing. What this design keeps
from it, changes in it, and adds to it is stated per decision, and the three
Codex findings recorded on the goal ledger are disposed of by id: **CX-1**
(MAJOR, stale state across the arm lock), **CX-2** (MINOR, the successful hook
path discards the re-arm notice), **CX-3** (MINOR, recovery's aggregate and
recovery's replace=true inside `--if-down`).

## Where each round-1 finding lands

| Finding | Folded in | One-line disposition |
| --- | --- | --- |
| ERAR-R1-01-STRAY-CRON | Decision 1 (eligibility), Decision 3 (ordering in `ordinary`) | Automatic re-arm requires a resolved session identity; the enrollment check moves after `resolveSessionIdentity`, still before any announcement, lease, or supervision write. |
| ERAR-R1-02-LOCKED-ORDER | Decision 2 (steps under the lock) | Eligibility and skip are decided under the lock before the live runner is touched; a refusal or skip stops nothing. |
| ERAR-R1-03-PROVENANCE | Decision 1 (C13 narrowed, C19 and C20 added), Decision 2 (EngineBuild read from the digested bytes) | The stamp is read from the bytes digested under the lock; absent, `dev`, `unknown`, or unlanded stamps refuse with the human remedy. The ruling R-37-m3 wins over the revision-1 deferral. |
| ERAR-R1-04-LEGACY-MIGRATION | Decision 2 (legacy records), new Migration section | A record without `MintedBy` is rendered LEGACY, never human-witnessed; the clearing act is `steward restart` (or a temporary-word arm) because `steward arm` returns already-armed beside a live runner. |
| ERAR-R1-05-VISIBILITY | Decision 3 (every Result, every hook exit, persistence) | The re-armed fact rides every Result through one exit point, is persisted in three places, and the hook carries it in the check-in tail that every Stop emission includes. |
| ERAR-R1-06-PARTIAL-MINT-API | Decision 4 (typed stage) | `arm` returns a typed outcome whose stage names the last completed step; the automatic path's rendering is specified per stage. |
| ERAR-R1-07-FIXTURE-HARNESS | Decision 5 (proof classes) | Two-engine proofs move to the supervision fixture bed as shell scenarios; concurrency proofs are Go tests with a stamped fake runner; unprovable claims are listed as residual risk. |
| ERAR-R1-08-C16-OUTCOME | Decision 1 (C16 row), Decision 3 (routing) | Typed drift from `Command` during owner or runner launch routes to the declared `ENROLLMENT_DRIFT` outcome; a test proves the mapping. |
| ERAR-R1-09-TABLE-LABEL (non-material) | Decision 1 | "One automatic cause, C13, with C18 conditional on it." |

## The defect, restated against the code

`internal/steward/identity.go:242-290` (`OpenEnrolledBinary`) authenticates the
record at `artifacts/agents/steward/identity.json` (`install.go:12-14`), opens
the enrolled path, proves the path still names the opened inode, and compares
the digest of the open descriptor with `InstallDigest`; a mismatch is
`ENROLLMENT_DRIFT` (`identity.go:284-287`). `internal/up/up.go:399-410`
(`openInvokingEnrollment`) calls it first thing in both `ordinary` (418-421)
and `recovery` (508-511) and adds one more refusal: the invoking engine's
canonical path must be the enrolled path (404-408). Every refusal renders as
`component=accepted-engine outcome=ENROLLMENT_DRIFT` with the remedy "from the
enrolled agent-free terminal, explicitly run metasystem steward arm or steward
restart" (`up.go:391-397`), and `steward arm` and `steward restart` enforce
that terminal through `requireHumanStewardEnrollment`
(`cmd/metasystem/steward_verbs.go:570-586`, `ClassHuman` or refuse).

The engine is rebuilt whenever a Go change lands and the seat pulls; the
enrolled path is `bin/metasystem` in the installation. So every landing
changed the digest, every `up` on every seat refused, and the Stop hook turned
the refusal into a blocked turn (`scripts/agents/supervision-hook.sh:422-425`
records "supervision arming failed"; 530-533 and 585-588 emit the block). A
human had to type the command at a terminal, or relay a temporary word under
R-29-m2. Wido's standing order R-37-m3 (`memory/rulings.md:64`, 2026-08-31,
verbatim "rearm after an engine rebuild is always allowed/necessary until
further notice. for all machines") already authorizes the act on every
machine without a fresh per-case word, bounds it "by construction to engines
built from commits that arrived through the landing gates", and asks that
every re-arm record the commit it consumed. Nothing mechanized it; m2 lost a
night of re-prompts on 2026-09-04 and m1 lost most of 2026-09-05.

Two facts about the existing arm path govern everything below:

- `runner.go:382-447` (`arm`) takes the arm flock at 402-408 and does
  everything an arm changes inside it, BUT it stops a live runner at 410-417
  BEFORE it reads the prior record at 418, and it reads that record only for
  `prior.Generation`; the temporary word and review date it mints come from
  its arguments, which the caller computed before the lock. That is the
  window CX-1 names, and the stop-before-decide order is the defect
  ERAR-R1-02 names.
- `arm` mints at 424-430, then re-opens the enrollment (431), snapshots it
  (436), and launches the runner (439). The runner reads its generation from
  the record (`checkStewardRunner` compares the runner's recorded generation
  with `installedGeneration`, `health.go:560-583, 1153-1166`), so the record
  must exist before the launch. Mint-before-launch is structural, not an
  accident; Decision 4 inherits it deliberately and types its stages.

One observed fact about the fleet governs the migration: the engine enrolled
on m1 at generation 9 (`artifacts/agents/steward/identity.json`, minted
2026-09-05T11:07:59Z, temporary word carrying the R-37-m3 text, review by
2026-09-06, no provenance fields) was built OUTSIDE `go-build.sh`: `go
version -m bin/metasystem` on that engine shows `vcs=git`,
`vcs.modified=true`, and no `-ldflags` setting, so its `supervise.BuildStamp`
is the default `dev` (`internal/supervise/disk.go:139`) even though
`go-build.sh:50,59` pins `-buildvcs=false` and always passes the stamp. The
fleet's live state is exactly the case ERAR-R1-03 and ERAR-R1-04 describe.

## Decision 1 — what the pin protects, and which drift causes re-arm

### The threat-model reading, corrected (folds ERAR-R1-01 and ERAR-R1-03)

`identity.go:3-8` states the pin's purpose: "accident-proofing at the
repository's trust level: a stray cron job does not match a pinned record; a
same-user adversary is out of scope repo-wide." Read against the code, the
pin answers one question: **is the process asking to be the steward the engine
this repository enrolled?** It identifies the caller. It is not an integrity
seal over the bytes at the enrolled path, because at the repository's trust
level the same user who can write `bin/metasystem` can write
`identity.json` in the same tree with the same permissions (`MintIdentity`,
`identity.go:52-62`, is a plain owner-only file write).

Revision 1 claimed that a stray cron "runs some other binary at some other
path". ERAR-R1-01 is right that nothing enforces that: `cmd/metasystem/up.go:114`
derives `Binary` from `os.Executable`, so a cron entry that runs the rebuilt
`bin/metasystem` satisfies the path test, and revision 1 re-armed before the
session identity was resolved, so the later identity failure could not undo
the mint. Revision 2 therefore binds automatic re-arm to **a resolved session
identity**, the fact a stray cron cannot supply, and to **a landed build
stamp read from the digested bytes**, the fact R-37-m3 requires:

- **Resolved session identity.** `resolveSessionIdentity` (`up.go:163-242`)
  succeeds only through one of two proofs. The runtime-signature proof
  (`census.FindAncestorProduction`, 178-184) walks the invoking process's
  ancestors for a registered runtime's executable signature; a cron entry's
  ancestors are the scheduler, so it fails. The explicit `--pid/--start-time`
  pair (188-232) requires the pair to name a live process whose start second
  matches, AND the `up` process to descend from that pid
  (`proveCallerDescendsFromTarget`, 146-161), re-checked for identity change
  on both ends; a cron entry does not descend from an agent session, so it
  fails even when it names a live one. This is the same proof `up` needs
  before it announces (`lease.AnnounceWithProofAt`, 440-441). Automatic
  re-arm runs only after it succeeds, and the minted record names the
  session (Decision 2). The announcement itself still follows the enrollment
  check, because `docs/orchestration.md:239` requires drift to refuse
  "before announcements, leases, or supervision are touched"; that sentence
  stays true (Decision 3).
- **Landed build stamp from the digested bytes.** Go records the linker
  flags in the binary's build information: `go version -m` on a binary built
  with `-ldflags "-X main.Stamp=abc1234"` prints
  `build -ldflags="-X main.Stamp=abc1234"` (verified on go1.26.5 in this
  session). `debug/buildinfo.Read` accepts an `io.ReaderAt`, and the open
  `*os.File` digested under the arm lock is one, so the stamp is read from
  exactly the bytes that are digested and enrolled, never from the invoking
  process's own `supervise.BuildStamp`. The eligibility rule is in the C13
  row below and its refusals are C19 and C20.

The counter-position is on record and is now answered by mechanism rather
than by deferral: `plans/goals-drafts/provenance-anchored-rearm.md` argues
that an automatic re-arm "would launder seat-authored engine bytes into
armed authority". Revision 1 postponed the provenance test to goal
two-bars-for-changes. ERAR-R1-03 shows that postponement contradicts
R-37-m3's own bound, and **the ruling wins**: the automatic path enrolls only
bytes whose stamp names a commit reachable from the installation
repository's `main` branch, and records that commit. What two-bars later adds
is a stronger landing proof for the same seam (Decision 2, step 4); nothing
else in this design moves when it lands. The human paths (`steward arm`,
`steward restart`, `--temporary-human-word`) are unchanged in what they
accept: the human at the terminal remains the authority for a `dev` build,
and the record now says truthfully what stamp they enrolled.

### The disposition of every drift cause

Each cause is either **RE-ARM** or **REFUSE**. Refuse means: no mint, no
record write, no runner stopped, the existing `ENROLLMENT_DRIFT` refusal with
its named cause, and the remedy rewritten as Decision 3 says. The list is
exhaustive over the refusal sites in `identity.go` and `up.go` and over the
two launch sites that execute a pinned engine; the implementer adds no cause
without reopening this design. There is **one automatic cause, C13, with C18
conditional on it** (ERAR-R1-09).

| # | Cause (site) | Disposition | Why |
| --- | --- | --- | --- |
| C1 | Record absent (`identity.go:70-73`) | REFUSE | Never enrolled, or the record was deleted: there is no human-witnessed generation to descend from and nothing to carry forward. First enrollment is the human's act (fleet-join bootstrap owns it). |
| C2 | Record mode not owner-only (`identity.go:74-76`) | REFUSE | The record itself was disturbed; the pin's own authentication failed. |
| C3 | Record owned by another uid (`identity.go:77-79`) | REFUSE | Not this user's enrollment. |
| C4 | Record unreadable or malformed (`identity.go:80-86`) | REFUSE | Nothing trustworthy to descend from. |
| C5 | `RepoIdentity` names another repository (`identity.go:87-89`) | REFUSE | The record serves a different installation. |
| C6 | `Generation` < 1 (`identity.go:90-92`) | REFUSE | No valid lineage. |
| C7 | `InstallPath` or `InstallDigest` empty (`identity.go:248-250`) | REFUSE | Incomplete enrollment; the pin never bound an engine. |
| C8 | `InstallPath` not canonical (`identity.go:251-255`) | REFUSE | The record does not name a path this code would have written. |
| C9 | Enrolled path cannot be opened or inspected (`identity.go:256-273`) | REFUSE | No engine at the enrolled path; a rebuild in progress lands here for one call and the next `up` retries. |
| C10 | Path changed while pinning, `SameFile` false (`identity.go:274-276`) | REFUSE | A moving target is never enrolled; the next `up` retries. |
| C11 | Not a regular executable (`identity.go:277-279`) | REFUSE | Not an engine. |
| C12 | Digest read failure (`identity.go:280-283`) | REFUSE | Cannot prove what the bytes are. |
| C13 | **Digest mismatch at the enrolled path** (`identity.go:284-287`), AND the invoking engine's canonical executable path equals `InstallPath`, AND `resolveSessionIdentity` succeeded for this `up`, AND the build stamp read from the digested descriptor names a commit reachable from `refs/heads/main` of the installation repository | **RE-ARM** | The one cause a landed rebuild produces, from the one caller the pin identifies, asked for by a proven agent session, enrolling bytes whose provenance R-37-m3 admits. |
| C14 | Digest mismatch AND the invoking engine is not the enrolled path (`up.go:404-408` reached after C13 typed) | REFUSE | A stranger engine at a stranger path, a `METASYSTEM_BIN` override pointing elsewhere, another checkout's build. The record is untouched and the live runner is not stopped. |
| C15 | Digest matches AND the invoking engine is not the enrolled path (`up.go:404-408`) | REFUSE | Same-bytes-elsewhere is still a stranger caller; unchanged from today. |
| C16 | Snapshot drift in `PrepareForExecution` (`identity.go:194-196`), or pre-exec drift in `Command` (`identity.go:228-235`) reached from the supervision-owner launch (`supervise/arming.go:604-608`, surfaced at `up.go:354-365`) or the steward-runner launch (`runner.go:455-458`, surfaced at `up.go:480-483`) | REFUSE, rendered as `ENROLLMENT_DRIFT` at every one of the three sites (ERAR-R1-08) | The bytes moved between verification and execution; a moving target. Verifiers stay pure. Revision 1 left the two launch sites rendering a component failure; Decision 3 routes them. |
| C17 | `ENROLLMENT_CHANGED` under the arm lock in `repairPinnedRunner` (`runner.go:322-329`) | unchanged (stop, report) | Not a drift cause; a concurrent arm won. Recovery reports it as today. |
| C18 | Record carries a temporary word whose `ReviewBy` date has passed | conditional on C13: carried verbatim when C13 re-arms; otherwise nothing | The review date is the human's obligation, not the machine's; the machine neither extends nor clears it. Health shows it (Decision 2). |
| C19 | Digest mismatch at the enrolled path, invoking path equal, session resolved, AND the digested bytes carry no readable build information, or a stamp of `dev` or `unknown` (the two defaults: `disk.go:139`, `go-build.sh:43`), or a stamp that is not a commit object in the installation repository | REFUSE with detail "rebuilt engine at <path> carries build stamp <value>; automatic re-arm is bounded to landed commits (R-37-m3)" and the human remedy | An unstamped or unattested build is exactly the seat-authored engine the ruling excludes; the human may still enroll it at the terminal. This is the m1 engine's state today. |
| C20 | As C19 but the stamp names a commit that exists and is NOT reachable from `refs/heads/main` (`git -C <installation root> merge-base --is-ancestor <stamp> refs/heads/main` fails), or `refs/heads/main` does not exist | REFUSE with detail "rebuilt engine at <path> was built from <stamp>, which is not on main" (or "the installation repository has no main branch") and the human remedy | A feature-branch build at the enrolled path did not arrive through the landing gates. The witness-path stamp (`go-build.sh:39-43`, an engine-input digest) lands here too; those trees are enrolled by the human or the fleet-join bootstrap, never by this path. |

The typed error the prior implementation added, `ErrEngineRebuilt` wrapped
beside `ErrEnrollmentDrift` at the digest-mismatch site (branch
`identity.go:27-34, 292-294`), is kept exactly: verifiers keep refusing both
types; only `up`'s ordinary path reads the second type, and only after the
session identity resolved. C14's ordering is a change from the prior
implementation: the prior code re-armed first and checked the invoking path
afterwards inside `ReArmRebuiltEngine`; this design checks the invoking path
INSIDE the lock as the first eligibility step (Decision 2), before any
runner is stopped. A stray cron never reaches C13: with no resolvable session
identity, `ordinary` returns `component=session-identity outcome=failed`
before the enrollment is opened (Decision 3), and the record and runner are
untouched.

## Decision 2 — the re-arm as one act under the arm lock (folds CX-1, ERAR-R1-02, ERAR-R1-03, ERAR-R1-04)

### What changes in `arm`

`runner.go:382-447` is refactored so that everything minted is decided from a
record read INSIDE the lock, BEFORE the live runner is touched. The
mechanical shape:

```go
// mintPlan is decided under the arm lock from the prior record and the
// enrolled bytes as they are at that moment. Human paths ignore prior for
// the word; the machine path re-verifies eligibility and copies the
// carry-forward fields from prior.
type mintPlan struct {
    Skip        bool   // true: mint nothing, stop nothing; Message explains
    Message     string // rendered when Skip is true
    MintedBy    string // "human-terminal" | "human-word" | "machine-rebuild"
    MintedBySession string // machine path only: the resolved session (Decision 1)
    Word        string
    ReviewBy    string
    Witnessed   int    // HumanWitnessedGeneration to write; 0 = unknown (legacy base)
    WitnessedAt string
    EngineBuild string // read from the digested bytes by the helper below
}

// enrolledBytes is the locked read of the enrolled path: the open
// descriptor, its digest, and the build stamp read from that descriptor.
type enrolledBytes struct {
    File   *os.File
    Digest string
    Stamp  string // "" when the bytes carry no readable build information
    Err    error  // the C7–C12 cause when the read failed; File is nil then
}

// ArmStage names the last step arm completed (Decision 4).
type ArmStage int
const (
    StageBeforeMint ArmStage = iota // nothing changed: refused, skipped, excluded, already armed
    StageMinted                     // the record was written; reopen failed
    StageReopened                   // reopened; snapshot failed
    StageSnapshotted                // snapshot ready; launch failed
    StageLaunched                   // the runner confirmed
)

type armOutcome struct {
    Stage              ArmStage
    Message            string
    Generation         int
    PreviousGeneration int
    RunnerPid          int64
}

func arm(repoRoot, binaryPath string, replace bool,
    decide func(prior InstallIdentity, priorErr error, bytes enrolledBytes) (mintPlan, error)) (armOutcome, error)
```

`Arm`, `ArmTemporary`, and `Restart` keep their public signatures
(`(string, error)`, returning `outcome.Message`) and pass a `decide` that
returns their fixed plan (`MintedBy` "human-terminal" for `Arm` and
`Restart`, "human-word" for `ArmTemporary`; `Witnessed` = the generation
about to be minted, `WitnessedAt` = its `MintedAt`; `EngineBuild` =
`bytes.Stamp`, written as read, `dev` included, because the human is the
authority there).

### The steps under the lock (ERAR-R1-02)

Every arm, human or machine, runs these steps in this order; steps 1 through
4 change nothing on disk and stop nothing:

1. `runnerExclusion` and `NotifyCommand` checks as today (`runner.go:390-395`),
   then the flock (`402-408`).
2. Read the prior record ONCE: `VerifyIdentity` (today's line 418, moved up).
3. Open and digest the enrolled bytes ONCE: the helper factored from
   `identity.go:256-287` (open, `SameFile`, regular executable, digest) is
   applied to `binaryPath` for human paths and to `prior.InstallPath` for the
   machine path; it also reads the build stamp from the same descriptor
   (`debug/buildinfo.Read(file)`, the `-ldflags` setting, the value after
   `supervise.BuildStamp=`). The descriptor stays open until the mint is
   done.
4. `decide(prior, priorErr, bytes)`. An error is a refusal: return
   `armOutcome{Stage: StageBeforeMint}` and the error. `Skip` is a no-op:
   read `liveRunner` (a read only) and return `StageBeforeMint` with the
   plan's message plus "runner pid P" when one is live. Neither touches the
   runner.
5. Only now the live runner: `liveRunner` (today's 410). If alive and
   `!replace`, return `StageBeforeMint` with "already armed (runner pid P)"
   plus the provenance clause of the standing record (Migration section). If
   alive and `replace`, `stopRunnerForReplacement`.
6. Mint generation `prior.Generation+1` from the plan and from `bytes.Digest`
   (never from a digest read outside the lock); `Stage` becomes
   `StageMinted`. Immediately after the mint, still under the lock, write the
   two persisted notices of Decision 3 (the pending notification and the
   arming-log line).
7. Reopen (`OpenEnrolledBinary`): `StageReopened` on success.
8. Snapshot (`PrepareForExecution`): `StageSnapshotted` on success.
9. Launch (`launchRunner`): `StageLaunched` on success.

Any error after step 6 returns the stage reached and the error; the mint is
never rolled back (Decision 4). `runner.go:399-401`'s comment ("EVERYTHING
an arm changes happens inside the lock") becomes true for the word, the
review date, the digest, and the stamp, and gains "and nothing is stopped
before the decision".

### The machine path's `decide`

`ReArmRebuiltEngine(repoRoot, invokingBinary string, session ReArmSession)
(ReArmOutcome, error)` replaces the branch's version (branch
`runner.go:172-197`). `ReArmSession{Runtime string; Pid, StartTime int64;
Tag string}` is the resolved identity from `up` (Decision 3);
`ReArmRebuiltEngine` refuses a session whose `Pid` or `StartTime` is below 1
before it takes the lock, so the binding of Decision 1 holds even for a
caller that bypasses `up`. It calls `arm` with `replace=true` and this
`decide`, every step of which runs under the arm lock at step 4 above:

1. `priorErr != nil` → return that error wrapped in `ErrEnrollmentDrift`; no
   mint. (Causes C1–C6 re-checked under the lock.)
2. `canonicalPath(invokingBinary) != prior.InstallPath` → refuse with
   "engine %q is not the enrolled engine %q"; no mint. (C14, decided before
   anything else, and before any stop.)
3. `bytes` failed for any reason other than a digest mismatch → refuse with
   that cause; no mint (C7–C12 under the lock). `bytes.Digest ==
   prior.InstallDigest` → `Skip: true` with message "already current
   (generation N, runner pid P)": a concurrent re-arm or a human arm already
   brought the record current; nothing is minted and the live runner is not
   stopped (fixture F3 proves the second contender's runner survives).
4. Provenance (C19, C20): `bytes.Stamp` empty, `dev`, or `unknown` → refuse
   C19. `git -C <installation root> cat-file -e <stamp>^{commit}` fails →
   refuse C19. `git -C <installation root> rev-parse --verify
   refs/heads/main` fails, or `git merge-base --is-ancestor <stamp>
   refs/heads/main` fails → refuse C20. The installation root is
   `RepoIdentity` of the prior record (the root the record serves), which is
   the metasystem installation whose `go-build.sh` stamped the bytes. This
   step is the one named seam where two-bars later substitutes its stronger
   landing proof.
5. Otherwise the plan is: `MintedBy` "machine-rebuild"; `MintedBySession`
   rendered from `session` as `<runtime> pid <pid> start <startTime> tag
   <tag>`; `Word` and `ReviewBy` copied from `prior` as read in this same
   locked section (CX-1's fix: a concurrent `ArmTemporary` that won the lock
   first is what `prior` now shows, so its word rides forward instead of
   being overwritten by stale empty strings); `Witnessed`/`WitnessedAt`
   copied from `prior` when `prior.MintedBy` is non-empty, else `0`/`""`
   (the legacy rule below); `EngineBuild` = `bytes.Stamp`.

Because the plan is decided under the lock, two concurrent re-arms
serialize: the second finds step 3's digest equal and skips without
stopping the first's runner (fixture F3). A re-arm racing `ArmTemporary`
serializes the same way: whichever takes the lock second sees the first's
record (fixture F4). `arm`'s `replace=true` on the machine path is deliberate
and bounded to `ordinary` `up`; recovery never reaches it (Decision 3).

`ReArmOutcome` is `{Status string; Stage ArmStage; Generation,
PreviousGeneration int; RunnerPid int64; EngineBuild string}` with `Status`
one of `re-armed`, `already-current`. An error return with `Stage >=
StageMinted` means the mint landed and a later step failed (Decision 4).

### The machine-minted stamp on `InstallIdentity`

Wido's ruling (2026-09-05): carry the temporary word and review date forward
AND stamp which generations were machine-minted rather than human-witnessed.
Five fields are added to `InstallIdentity` (`identity.go:28-48`):

```go
// MintedBy names the act that minted this generation: "human-terminal"
// (steward arm or restart at an agent-free terminal), "human-word"
// (steward arm --temporary-human-word), or "machine-rebuild" (ordinary up
// re-arming the rebuilt engine at its enrolled path). Absent on records
// minted before the stamp existed; such a record is LEGACY and is never
// read as human-witnessed.
MintedBy string `json:"mintedBy,omitempty"`
// MintedBySession names the resolved agent session that asked for a
// machine-rebuild mint (runtime, pid, start second, tag). Empty on human
// mints.
MintedBySession string `json:"mintedBySession,omitempty"`
// HumanWitnessedGeneration is the newest generation a human act minted;
// every generation above it up to Generation was machine-minted. Equals
// Generation on a human-minted record. Zero means no human witness is
// recorded (a machine mint descended from a legacy record).
HumanWitnessedGeneration int    `json:"humanWitnessedGeneration,omitempty"`
HumanWitnessedAt         string `json:"humanWitnessedAt,omitempty"`
// EngineBuild is the build stamp read from the enrolled bytes themselves
// (the -ldflags -X supervise.BuildStamp value in the binary's build
// information), recorded so a re-arm names the commit it consumed
// (R-37-m3). A human mint records whatever the bytes carry, "dev" included.
EngineBuild string `json:"engineBuild,omitempty"`
```

Rules: a human path writes `MintedBy` to its human value and
`HumanWitnessedGeneration`/`At` to the generation it mints, which is how a
human re-arm clears machine-minted and legacy state. There is no separate
clearing verb; the clearing act is whichever human path MINTS: `steward
restart` at the terminal (it always mints, `replace=true`), `steward arm
--temporary-human-word` (also `replace=true`), or `steward arm` after a
disarm. Plain `steward arm` beside a live runner returns "already armed"
WITHOUT minting (`runner.go:410-413`), so it clears nothing; revision 1's
sentence that the next `steward arm` clears the state was false
(ERAR-R1-04) and every remedy string in this design names `steward restart`
when a runner is live. The machine path copies the witnessed pair forward and
sets `MintedBy` "machine-rebuild". `EngineBuild` is written by every path
from the digested bytes. `VerifyIdentity` does not validate the new fields.
`TemporaryHumanWord` and `ReviewBy` are carried byte-for-byte; the machine
never validates, extends, or clears them (C18).

### Legacy records (ERAR-R1-04)

A record with empty `MintedBy` is **legacy**: minted before this design, by
a human or by the branch's machine path, and nobody can tell which from the
record. The rule is truthfulness, not optimism:

- Health renders it `enrollment generation 9 LEGACY (minted before
  provenance stamping; no human witness recorded)`.
- The machine path may descend from it (R-37-m3 authorizes the re-arm on
  every machine, and refusing here would recreate the wedge the goal exists
  to end), but the new record carries `HumanWitnessedGeneration: 0` and
  health renders `machine-minted (rebuild, engine <commit>, session <tag>)
  above LEGACY generation 9; no human witness recorded`.
- `steward arm` beside a live runner, when the standing record is legacy or
  machine-minted, returns "already armed (runner pid P); the enrollment is
  LEGACY/machine-minted — run steward restart at the terminal to witness
  it" so a human who typed the wrong verb learns the right one.

### What health shows

Health today renders the steward-runner role as "runner pid N and generation
G success are current" (`health.go:582-583`) and knows the installed
generation only as an integer (`installedGeneration`, 1153-1166). The
identity.go header's claim that "health and every reader see the temporary
state" has no reader in `health.go` (grep for the field finds none); this
design gives all three states one reader. `installedGeneration` becomes
`installedEnrollment(repoRoot) (InstallIdentity, error)` and the
steward-runner role's reason string, in every status where the record was
readable, ends with one provenance clause rendered by
`EnrollmentProvenance(id InstallIdentity) string`:

- human-minted, permanent: `enrollment generation 9 human-witnessed (engine
  <commit>)`
- machine-minted above a witnessed base: `enrollment generation 9
  machine-minted (rebuild, engine <commit>, session <tag>) above
  human-witnessed generation 5 of 2026-09-02T11:21:41Z`
- machine-minted above a legacy base, and legacy itself: the two renderings
  in the Legacy section
- with a temporary word, any case: append `; TEMPORARY under a recorded
  remote human word, review by 2026-09-06`

The aggregate does not change: a machine-minted or legacy generation is
information, not a fault, so it moves no role to dead or unknown and adds no
role. An overdue `ReviewBy` is likewise rendered, not judged; judging it is
outside this goal. The `HEALTH` line the hook previews (`health.go:192`)
carries the clause because it carries every role's reason, and that line is
in the check-in tail of every Stop emission (Decision 3), which is what
makes the persisted fact visible in later turns.

## Decision 3 — visibility and ordering (folds CX-2, CX-3, ERAR-R1-01, ERAR-R1-05, ERAR-R1-08)

### The order inside `ordinary`

`ordinary` (`up.go:412-500`) becomes: host-preflight → **session-identity**
(`resolveSessionIdentity`, no writes) → **accepted-engine** (the enrollment
open; C13 re-arms here, with the resolved identity as `ReArmSession`) →
`PrepareForExecution` → session-announcement → checkout-lease → supervision
→ steward-runner. The one move is session identity ahead of the enrollment
open; `resolveSessionIdentity` reads processes and the fixture table only
and writes nothing, so a drift refusal still happens "before announcements,
leases, or supervision are touched" (`docs/orchestration.md:239`) and
`TestOrdinaryUpRefusesDriftWithoutMintingANewGeneration` (`up_test.go:121-147`)
keeps its no-`mains`, no-`supervision` assertion. The component line order
changes (session-identity now precedes accepted-engine) and every fixture
that asserts line order is updated, not weakened. A caller with no
resolvable session, a stray cron included, returns
`component=session-identity outcome=failed` with today's remedy and never
opens the enrollment. An advisor session's `up` re-arms too: its identity
resolves before the lease classifies it read-only, and the second seat on a
machine must not stay wedged while the holder idles; the record names that
session in `MintedBySession`.

`recovery` (`up.go:502-547`) is unchanged in order and never re-arms:
`openInvokingEnrollment` takes `allowReArm bool` plus the optional
`ReArmSession`; `ordinary` passes true with its resolved identity and
`recovery` passes false. The branch's shared use (branch `up.go:534-546`) is
withdrawn. **Unattended recovery never re-arms**, for three reasons each
sufficient: the scheduler entry is `--recover-only --if-down`, whose contract
is "start only missing repository rings" (`cmd/metasystem/up.go:83`,
`docs/orchestration.md:239`), and a re-arm STOPS a live runner to replace it;
recovery has no session identity, so the Decision 1 binding cannot hold; and
the wedge this goal fixes is a seat unable to arm, which the seat's own next
`up` now resolves. A rebuilt engine at the enrolled path is
`ENROLLMENT_DRIFT` in recovery, with the remedy "run metasystem up from a
session, which re-arms a rebuilt engine at its enrolled path; or steward arm
at an agent-free terminal". The hook's no-identity Stop path (419-420) also
runs `--recover-only --if-down` and therefore also does not re-arm. With
that, CX-3's first half (recovery reporting `recovery-not-needed` after a
re-arm replaced the runner) cannot occur, and its second half (replace=true
inside `--if-down`) is disposed of by narrowing: the `--if-down` contract
stands as written. The aggregate rule at `up.go:542-545` is unchanged.

### Every Result carries the fact (ERAR-R1-05)

`Result` gains `ReArmed string`, rendered by `Lines()` (`up.go:76-106`) on
the aggregate line between `authority` and `holder` when non-empty:
`re-armed="generation=9 previous=8 engine=<commit>"`. It is set at ONE exit
point: `ordinary` becomes a wrapper around `ordinaryBody(options) (Result,
string)`; the body threads the re-armed string through its locals and the
wrapper writes it onto whatever `Result` the body returned, the `failure`
helper's Results (`up.go:108-113`) and `enrollmentDrift`'s included. So a
successful re-arm followed by a session-announcement, checkout-lease,
supervision-owner, or steward-runner failure still ends with an aggregate
line carrying `re-armed=`, which is the line every hook path tails. The
component line for the re-arm is `component=accepted-engine
outcome=re-armed detail="the enrolled engine was rebuilt; armed (runner pid
P) (generation=9 previous=8 engine=<commit> session=<tag> path=...)"`. An
`already-current` outcome renders as `component=accepted-engine
outcome=verified` with the detail "brought current by a concurrent arm"; it
sets no `re-armed` key, because this call minted nothing.

### Where the fact is persisted

Three durable records, all written before `up` can fail or be killed:

1. The identity record itself: `MintedBy: machine-rebuild`,
   `MintedBySession`, `EngineBuild` (Decision 2). Health renders it on every
   later `HEALTH` line until a human mints over it.
2. `artifacts/agents/supervision/arming.log` (`appendArmingLog`,
   `up.go:315-324`): `engine-re-armed generation=9 previous=8
   engine=<commit> session=<tag>`, written by `up` immediately after
   `ReArmRebuiltEngine` returns with `Stage >= StageMinted`, before any
   other component runs.
3. A queued steward notification (`QueueNotification`,
   `intervene.go:299-314`), nonce `engine-rearm-generation-<N>`, message
   "steward: re-armed the rebuilt engine: generation 9 previous 8 engine
   <commit> session <tag>", written under the arm lock at step 6. The runner
   delivers it through the notify channel on its first tick
   (`DeliverPending`, `notify.go:64-99`), and until delivered `steward
   pending` names it, which the hook already surfaces at every session start
   and end as "Steward incidents pending" (`supervision-hook.sh:662-663`).

### The hook (CX-2, ERAR-R1-05)

`supervision-hook.sh` reads `up`'s output only on failure (Stop: 422-425;
session start: 690-698 exits 0 on success). The additions, none blocking:

- Stop (410-452): a new variable `up_notice=` beside `up_failure`. It is
  computed from `$up_output` REGARDLESS of `up_rc`: if any line contains
  ` re-armed=`, `up_notice="Metasystem re-armed the rebuilt engine: <that
  line>"`. Then at 449, where `checkin_tail=$health_line` is assembled,
  `up_notice` is prepended when set. `checkin_tail` is the system message of
  EVERY Stop emission: the allow and block verdict paths (625-635), the
  advisor branch (547-548), the degraded branch (649), and
  `emit_failed_stop` (500, through `external_stop_json`). One insertion
  therefore covers every worker exit, including a `stop_failure` recorded
  before extras are assembled (530-533, 585-588). The block-reason rule
  (621-627: the display is the reason byte-verbatim) is untouched because
  the notice rides the system message, never the reason. `extras` gains
  nothing, so the notice is not duplicated.
- Session start (690-698): on `up` exit 0, if the last line of `$output`
  contains ` re-armed=`, emit `surface_json "Metasystem re-armed the rebuilt
  engine: <last line>"` before `exit 0`. On non-zero exit the existing line
  698 already tails the aggregate line, which now carries `re-armed=` when a
  re-arm preceded the failure; no change needed there.
- The four-second deadline parent (32-222) never reads `up`'s output and
  cannot carry a notice it never saw; when it kills the worker after a mint,
  the three persisted records above are already on disk, so the NEXT Stop's
  health line and the pending-notification line show the fact. That is the
  design's answer to "never silent end to end": the fact is durable, and
  every later turn carries it, even when the turn that made it was cut off.

The match is on the aggregate key, not on prose, so a future wording change
in the component detail cannot silence the hook.

### The refusal remedy

The remedy at `up.go:392` is rewritten for every REFUSE row so it no longer
names only the terminal: "this engine is not the enrolled one at its
enrolled path, or its build is not eligible for automatic re-arm; from an
agent-free terminal run metasystem steward restart --repo <root> (steward
arm when no runner is live), or relay the human's recorded word with
--temporary-human-word and --review-by". `<root>` is `options.Root`. The
fleet-join bootstrap design proposed the terminal half of this rewrite
(`plans/fleet-join-bootstrap-design.md:417`); this design lands it with
`restart` named first, for the ERAR-R1-04 reason.

### Typed drift at the launch sites (ERAR-R1-08)

`ensureSupervision` (`up.go:348-389`) and the steward launch (`up.go:480-483`)
gain one rule: when `errors.Is(err, steward.ErrEnrollmentDrift)`, return
`enrollmentDrift(components, err)` (the declared refusal: component
`accepted-engine`, outcome `ENROLLMENT_DRIFT`, aggregate `ENROLLMENT_DRIFT`,
exit 1) with the detail prefixed by the launch site ("supervision-owner
launch: " or "steward-runner launch: "), instead of a component failure. For
the supervision site this requires the error from `options.Command`
(`arming.go:604-608`) to survive every wrap between there and
`supervise.EnsureArmed`'s return; the implementer verifies each wrap on that
path uses `%w` and adds it where it does not. The mapping is proven by
fixture F10 with a fake `Command` at each site, and by the `errors.Is`
assertion through the real `EnsureArmed` call chain.

## Decision 4 — failure shape: mint-before-launch, visibly and by stage (folds ERAR-R1-06)

`arm` mints before it launches (`runner.go:424-439`) and a failed later step
leaves the bumped generation in place. The automatic path **inherits** this,
for a structural reason and a convergence reason. Structural: the runner
proves itself by completing a generation-bound tick against the record
(`checkStewardRunner`, `health.go:560-583`), so the record must name the new
generation before the runner starts. Convergence: after a failed later step
the record names the bytes that were at the enrolled path when they were
digested under the lock, so the next `up` either finds the enrollment
current and repairs the runner through `EnsureRunner` →
`repairPinnedRunner` (`runner.go:220-266, 306-367`, which mints nothing) or,
if yet another build landed meanwhile, finds C13 again and re-arms once
more. A mint-then-rollback would instead leave the next `up` re-minting
again, burning a generation per attempt with nothing gained.

What the automatic path does at each stage `ReArmRebuiltEngine` can return
with an error:

| Stage returned | What happened | `ordinary` renders |
| --- | --- | --- |
| `StageBeforeMint` | Refused (C1–C12, C14, C19, C20) or nothing to do | The refusal row's `ENROLLMENT_DRIFT` rendering, or `verified` for `already-current`; no `re-armed` key; no runner stopped. |
| `StageMinted` | Record written; `OpenEnrolledBinary` failed (the path changed again, or vanished, between the locked digest and the reopen) | `component=accepted-engine outcome=re-armed` (the mint happened; say so) then `component=steward-runner outcome=failed detail="after re-arm: <err>" remedy="rerun metasystem up; a further rebuild re-arms again"`, aggregate `up outcome=failed component=steward-runner re-armed=...`. |
| `StageReopened` | Reopened; `PrepareForExecution` failed (disk, or C16 snapshot drift) | Same shape; C16 drift inside this stage renders `ENROLLMENT_DRIFT` per Decision 3 with `re-armed=` still on the aggregate line. Remedy: "inspect artifacts/agents/steward/engine-pins, then rerun metasystem up". |
| `StageSnapshotted` | Snapshot ready; `launchRunner` failed (log unwritable, runner died, no confirmation in ten seconds) | Same shape; remedy "inspect artifacts/agents/steward/runner.log, then rerun metasystem up". |

In every post-mint stage the old runner was already stopped at step 5, the
pending notification and arming-log line were written at step 6, and the
failed generation carries `MintedBy: machine-rebuild`, so a reader can tell
it from a human act; the human-witnessed pair is unchanged by it. The
branch's rendering of a failed launch as `ENROLLMENT_DRIFT` "re-arming the
rebuilt engine failed" (branch `up.go:410-413`) is withdrawn: it named a
remedy (the terminal) that the state did not need.

## Decision 5 — proof (folds ERAR-R1-07)

Three classes, by what can actually run. The ordinary gate runs the Go tests
(`go-gate.sh:528, 544`) BEFORE it builds and installs `bin/metasystem`
(`555`), so a Go test that needs the real engine at `bin/metasystem` sees
last run's binary or skips (`runner_test.go:162-164`); every proof that needs
two real engine builds is therefore shell-driven and runs after the build.
The shell bed is `scripts/agents/supervision-fixtures.sh`, which already
creates and exports its own registry home (`116-118`), refuses to run
without the built engine (`120-121`), mints a fixture enrollment
(`enroll_fixture_engine`, `577-587`), and invokes `up` below an announced
live process with an explicit pid/start pair (`live_arm_driver`, `591-605`),
so session identity resolves the way ERAR-R1-01 requires. Its scenarios run
as fixture-bed children with the bed's cleanup (`18-28`, `30-77`), which
owns every runner and owner the scenario starts.

Engine builds for the shell class: engine A is the gate's `bin/metasystem`
copied into the scratch repository as line 634 does today. Engine B is
`scripts/agents/go-build.sh --out <scratch>/engine-b` (the proof build,
`go-build.sh:13-24, 49-56`, which leaves `bin/metasystem` untouched) with
`METASYSTEM_BUILD_STAMP` set to the short hash of a commit the scratch
repository has made on its `main` branch; B differs from A only in the
linked stamp string, which is enough to change the digest, and its stamp
satisfies C13's landed-commit test against the scratch repository. Engine D
("dev") is the same proof build with `METASYSTEM_BUILD_STAMP=dev`; engine X
("unlanded") is stamped with a commit the scratch repository made on a
branch that `main` does not contain. Installation onto the enrolled path is
by stage-and-rename, never by copying over the live file (a signed Mach-O
with appended bytes is killed by the kernel; a live binary overwritten in
place kills its process).

Failure injection for the launch stage needs no production seam:
`launchRunner` opens `artifacts/agents/steward/runner.log` for append
(`runner.go:450-453`) before it starts anything, so a scenario that makes
that path a directory gets a real `StageSnapshotted` launch error from
production code; removing the directory lets the following `up` repair.
The `steward run` verb never reads `BuildStamp` (`steward_verbs.go:470-494`),
and revision 1's "die" stamp is withdrawn.

| Id | Session id | Class and where | Proves |
| --- | --- | --- | --- |
| F1 real rebuild re-arms | `rearm-rebuild` | shell, `supervision-fixtures.sh` scenario | Enroll A at `<scratch>/bin/metasystem` with a `human-terminal` record (the fixture writes `MintedBy`, the witnessed pair, and `EngineBuild`); rename B onto the path; `up` via `live_arm_driver` returns exit 0, the accepted-engine line is `outcome=re-armed`, the aggregate carries `re-armed="generation=2 previous=1 engine=<B's stamp>"`; the record has generation 2, `MintedBy` machine-rebuild, `MintedBySession` naming the driver's pid and tag, `HumanWitnessedGeneration` 1, `EngineBuild` equal to the stamp B was built with; `engine-pins/generation-2-<B's digest>` exists; runner.json names a live pid; health's steward-runner reason carries the machine-minted clause; `arming.log` has the `engine-re-armed` line; `steward pending` names `engine-rearm-generation-2` until the runner delivers it. Second run of the same scenario from a legacy base (today's `enroll_fixture_engine` record): same, with `HumanWitnessedGeneration` 0 and the LEGACY clause in health. |
| F2 stranger refuses untouched | `rearm-stranger` | Go, `internal/up` (extends `TestOrdinaryUpRefusesDriftWithoutMintingANewGeneration`) with the explicit-identity fallback of `TestExplicitIdentityFallbackRequiresAndVerifiesTheRecordedPair` (`up_test.go:49-72`) so session identity resolves | Enrollment at path P generation 7; bytes at P changed AND `Binary` is a different path Q: `ENROLLMENT_DRIFT`, record still generation 7 with the old digest, no `mains`/`supervision` directories created, remedy names `steward restart` and the temporary-word relay. Second firing: bytes unchanged, `Binary` Q: same refusal (C15). Third firing: no session identity resolvable and `Binary` = P with changed bytes: `component=session-identity outcome=failed`, record untouched (the stray-cron shape). |
| F3 two concurrent re-arms mint once | `rearm-concurrent` | Go, `internal/steward`, `beforeLock`-style seam like `runner_test.go:443-446`; the runner is a stamped fake built by the test from `internal/steward/testdata/fakerunner` (writes runner.json with its own identity as `RunLoop` does, then sleeps; built with `-buildvcs=false -ldflags -X ...supervise.BuildStamp=<scratch main commit>`) | Two goroutines call `ReArmRebuiltEngine` on the same rebuilt enrollment; the seam holds the first inside the lock after step 4 until the second is blocked on the flock; outcomes are one `re-armed` and one `already-current`; the record's generation advanced by exactly one; the first's runner pid is still live after the second returns (the ERAR-R1-02 assertion). |
| F4 re-arm racing ArmTemporary keeps the human's word | `rearm-vs-human-word` | Go, `internal/steward`, same seam and fake | Permanent enrollment, engine rebuilt; `ReArmRebuiltEngine` is held BEFORE the flock while `ArmTemporary(word W, review R)` completes on the rebuilt bytes; released, the re-arm returns `already-current`, and the record carries W and R with `MintedBy` human-word. Second half: `ArmTemporary` is held before the flock while the re-arm completes; released, the human's arm mints the next generation with W and R and `HumanWitnessedGeneration` equal to its own generation. Neither order writes empty word fields over W. |
| F5 hook surfaces the re-arm | `rearm-hook-surfaces` | shell, `supervision-hook-fixtures.sh` (stub-engine pattern at 215-235) | A stub `up` prints a component line and an aggregate line ending in `re-armed="generation=9 previous=8 engine=abc1234"` and exits 0: session start emits a `systemMessage` containing "re-armed the rebuilt engine" and that line; Stop with an allow verdict carries it in the system message; Stop with a block verdict keeps the block reason byte-identical to the display and carries the notice only in the system message; Stop where the stub health verb fails (`emit_failed_stop`) still carries it. A stub `up` that exits 1 with the key on its aggregate line surfaces both the failure and the notice. A stub `up` that exits 0 without the key emits nothing new. |
| F6 failed launch after a machine mint | `rearm-launch-fails` | shell, `supervision-fixtures.sh` scenario | As F1's setup, plus `artifacts/agents/steward/runner.log` made a directory before `up`: `accepted-engine outcome=re-armed`, `steward-runner outcome=failed` with the runner.log remedy, aggregate `outcome=failed` with the `re-armed` key; the record is generation 2 `MintedBy` machine-rebuild; `arming.log` and the pending notification exist. Directory removed, a following `up` reports `steward-runner outcome=started` at generation 2 (repair, no mint). |
| F7 recovery never re-arms | `rearm-recovery-refuses` | Go, `internal/up` (beside `TestRecoveryRefusesBeforeArmingWithoutAnEnrolledEngine`) | Rebuilt bytes at the enrolled path; `recovery` with `RecoverOnly` and `IfDown` returns `ENROLLMENT_DRIFT`, the record unchanged, the remedy naming a session's `up`. |
| F8 dev-stamped rebuild refuses | `rearm-dev-stamp-refuses` | shell, `supervision-fixtures.sh` scenario | Enroll A and run one `up` with A so a runner is live; rename D onto the path; `up` returns `ENROLLMENT_DRIFT` whose detail names stamp `dev` and R-37-m3, remedy names `steward restart`; record still generation 1 with A's digest; A's runner is still live with the same pid (no stop on refusal). Then rename X onto the path: `ENROLLMENT_DRIFT` "not on main", same invariants (C20). |
| F9 arm beside a live runner reports the clearing verb | `rearm-arm-names-restart` | Go, `internal/steward` with the fake runner | Legacy record and live runner: `Arm` returns "already armed" naming `steward restart` and LEGACY, mints nothing. `Restart` mints generation N+1 with `MintedBy` human-terminal and the witnessed pair equal to N+1. |
| F10 typed drift at the launch sites | `rearm-c16-routes` | Go, `internal/up`, fake `Command` | A `Command` that returns an `ErrEnrollmentDrift`-wrapped error at the supervision-owner site, and separately at the steward-runner site, renders `component=accepted-engine outcome=ENROLLMENT_DRIFT` with the site prefix and aggregate `ENROLLMENT_DRIFT`; a plain error at the same sites still renders the component failure of today. |

F1, F6, and F8 run only outside a delegate sandbox (KI-15); the orchestrator
runs them. F3, F4, and F9 build one small stamped Go program per test run
and are the only Go tests here that launch processes; each sets
`METASYSTEM_SUPERVISION_REGISTRY_HOME` to its own `t.TempDir()`, uses a
scratch git repository with a commit on `main` as both the installation
root and the stamp source, and cleans up with `Disarm` plus a kill of the
pid recorded in runner.json, so the process-owning group owns everything it
started (the omission the critic named in `runner_test.go:156-200`).

Claims with no runnable proof, recorded as residual risk rather than
claimed: (i) a real re-arm surfaced by a real Stop hook end to end, because
the Stop hook needs a registered runtime ancestor the fixtures cannot supply
(F5 proves the hook plumbing against a stub `up`, F1 proves `up` itself);
(ii) the deadline-parent path after a mint, because the four-second kill is
timing the fixtures cannot make deterministic (the persistence in Decision 3
is what covers it, and F1 proves the three persisted records exist); (iii)
the `errors.Is` survival through `supervise.EnsureArmed` on a live owner
launch, proven only with the fake `Command` in F10 and by reading each wrap.

## Migration and build prerequisites (ERAR-R1-04)

1. Marking: absent `MintedBy` marks a legacy generation; health renders it
   LEGACY from the first run of the new engine, with no human-witnessed
   claim. The m1 record at generation 9 is legacy and, per the observed
   build information, dev-stamped; the m2 record of 2026-08-30 is legacy.
2. Clearing act: `steward restart --repo <root>` at an agent-free terminal
   (a runner is live on every fleet machine, so `steward arm` would return
   already-armed and clear nothing), or `steward arm --temporary-human-word
   ... --review-by ...` while R-29's departure path is lawful. Either mints a
   `human-terminal` or `human-word` generation with the witnessed pair
   seeded and `EngineBuild` recorded from the bytes.
3. Prerequisite for the build: the implementation lands the legacy
   rendering, the `restart`-naming remedies, and the C19 refusal in the same
   change, so that no engine built by this design can ever read a legacy
   record as human-witnessed, and so that the first post-landing rebuild on a
   machine whose enrolled engine is dev-stamped is refused with the remedy
   rather than enrolled. The landing note records, per machine, the legacy
   generation observed and that the human clearing act is pending; health
   shows LEGACY until it happens. Because the m1 engine is itself dev-stamped
   today, the first rebuild after landing on m1 must go through `go-build.sh`
   (which stamps) for the automatic path to accept it; a direct `go build`
   is refused by C19 with the human remedy, and that refusal is correct.

## Consistency pass

- Doctrine to sweep the same hour this lands (the rulings-sweep rule):
  `docs/orchestration.md:239` and the orchestration skill's copy say
  "Ordinary, advisor, and recovery-only `up` only consult that standing
  enrollment"; the sentence becomes "Recovery-only `up` only consults that
  standing enrollment; ordinary and advisor `up`, once their session
  identity is resolved, re-arm the enrolled engine when it was rebuilt at
  its enrolled path from a landed commit, and refuse every other drift
  before announcements, leases, or supervision are touched." `identity.go:3-8`
  gains one sentence naming the rebuild reading and the session binding.
  `cmd/metasystem/up.go:86`'s `--rearm` help ("ordinary up replaces an older
  generation automatically") stops being false. R-37-m3's software-home
  column points at this design; the provenance draft stays sequenced behind
  two-bars with its seam named here (Decision 2, machine `decide` step 4).
- Decision 1's binding to a resolved session and Decision 3's order agree:
  the enrollment open is the first step after identity resolution and the
  last step before any write.
- Decision 2's steps and Decision 4's stages agree: `StageBeforeMint` is
  every return before step 6; the three later stages are steps 7 through 9.
- Decision 2's provenance step and the C13/C19/C20 rows agree on the three
  tests (stamp present and not a default; commit exists; reachable from
  `refs/heads/main`) and on the installation root they run in.
- Decision 3's persistence and Decision 5's F1 and F6 agree on the three
  records and their names.
- Untouched on purpose: `VerifyEnrolledBinary` and every classifier that
  authenticates a running steward (`lease/classify.go:433`, `stage.go:84`)
  stay pure; `EnrolledExecutionPath` keys the snapshot by generation and
  digest, so the old runner's snapshot and the new one never collide; the
  `--if-down` contract; the block-once refusal record; the temporary word's
  validation (`humanauthority.ValidateTemporaryWordPair`) is not re-run on
  carry-forward, because the machine copies, it does not authorize; the
  human paths' acceptance of any stamp.
- The branch's hook changes to `find-ancestor --repo "$harness_root"` and
  the evidence directory are the hook-root goal's territory and are not part
  of this design.

## Self-grade

Grounding: every load-bearing claim is a file-and-line read at 66702fdcc
(`identity.go` whole, `runner.go` whole, `up.go` whole, the hook whole,
`steward_verbs.go:460-586`, `health.go:555-584, 1153-1166`,
`runner_test.go:150-205, 420-466`, `up_test.go:1-150` and its test names,
`supervision-fixtures.sh:1-80, 116-130, 577-605, 924-935`, `go-build.sh`
whole, `go-gate.sh:505-569`, `supervise/disk.go:126-139`,
`supervise/arming.go:595-624`, `steward/intervene.go:280-324`,
`steward/notify.go:1-117`, `memory/rulings.md:57-64`,
`docs/orchestration.md:239`, `cmd/metasystem/up.go:29-34, 112-133`, the
critique register whole, and the m1 identity record). Two facts were
executed rather than read: `go version -m` on the m1 engine (no `-ldflags`
setting, `vcs.modified=true`) and a scratch build proving Go 1.26.5 records
`-ldflags` with its `-X` value in build information. No product bytes
changed; the required gate chain was started in this worktree and its result
is reported in the return, not claimed here.

Residual risks, honestly: (a) the landed-commit test reads `refs/heads/main`
of the installation repository; an adopted repository whose default branch
is not `main` refuses every automatic re-arm (C20) until a later goal makes
the landing ref configurable, and the human remedy stands meanwhile; (b)
the build-information read depends on Go continuing to record `-ldflags`,
verified on 1.26.5, and a toolchain that stops doing so makes every rebuild
C19 (a loud refusal, never a silent enrollment); (c) the seat's local `main`
can lag origin, so a commit landed elsewhere but not yet pulled is refused
until the pull, which is the order the seat already follows; (d) the
two-engine shell scenarios add two proof builds to the supervision bed's
wall time, accepted over a byte-appended binary that macOS would kill; (e)
an advisor session's `up` re-arming the repository runner is new authority
for advisors, argued from the resolved session plus the enrolled engine plus
the landed stamp, and the critic should keep attacking it; (f) `re-armed=`
on the aggregate line and the moved session-identity component change the
output every fixture parses as stable records; each fixture asserting the
exact line set is updated, not weakened; (g) the health clause lengthens
every steward-runner line on every machine; it is the visibility Wido asked
for, and a shorter rendering is a wording choice the implementer may not
make alone; (h) the first rebuild after landing on m1 is refused if it is
not made through `go-build.sh`, because the enrolled engine there is
dev-stamped today; that is the ruling's bound applied to the live fleet and
is stated in the migration rather than hidden. Grade: pass against
everything read; the reject condition is the falsifier.

**Reject condition — reject this design if any of the following is shown:**
a drift cause not in the C1–C20 table that reaches `up` or a pinned-engine
launch site; any RE-ARM on a caller whose canonical executable path is not
the enrolled path, or without a resolved session identity, or on bytes whose
stamp is absent, `dev`, `unknown`, not a commit, or not reachable from
`refs/heads/main`, or on any cause other than C13; any `EngineBuild` value
read from anything but the descriptor digested under the arm lock; any field
of the minted record computed from a read taken outside the arm flock (the
CX-1 recreation); any refusal or skip that stops a live runner; a second
concurrent re-arm that stops the first's runner or mints a second
generation; any order of a re-arm and an `ArmTemporary` in which the human's
word or review date is replaced by empty strings; a legacy or machine-minted
record that health renders as human-witnessed, or a human mint that leaves
`MintedBy` at machine-rebuild or the witnessed pair unseeded; a remedy that
names `steward arm` as the clearing act beside a live runner; a Result
returned by `ordinary` after a mint without `re-armed=` on its aggregate
line; a Stop emission from the worker, `emit_failed_stop` included, whose
system message lacks the notice when the turn's `up` output carried the
key; a re-arm that leaves fewer than the three persisted records; any
re-arm reachable from `--recover-only`, or from the hook's no-identity Stop
path; any `--if-down` invocation that stops a live runner (the CX-3 excess);
an `arm` return from which the stage reached cannot be read without
re-reading the record; a post-mint failure reported as `ENROLLMENT_DRIFT`
except C16 drift, or with the terminal as its remedy; a machine mint that
rolls back, or whose record names bytes other than those digested under the
lock at the moment of the mint; typed `Command` drift at a launch site
rendered as a component failure; a temporary word carried forward that the
machine validated, extended, or cleared; or a fixture in Decision 5 weakened
from its stated assertion or moved out of the class that can run it.
