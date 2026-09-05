# Design: a rebuilt engine at its enrolled path re-arms itself; every other drift cause still reaches the human

Goal: engine-rebuild-rearms-itself (plans/goals/engine-rebuild-rearms-itself.md,
revision 7, tier 3 DESIGN-BEARING). Author: implementer delegate under dispatch
by m1 (lineage main-1788594343-3833-fb64b9), job
engine-rebuild-rearm-design-r1c-20260906. **Revision 1, 2026-09-06.** Every
seam cited below was read in this worktree at commit 789acb343; the prior
implementation was read as the diff between 2477e73d5 and df0597910 on branch
m1-2026-09-05-verified. Line numbers are HEAD's unless marked "branch".

The prior implementation is input, not the landing. What this design keeps
from it, changes in it, and adds to it is stated per decision, and the three
Codex findings recorded on the goal ledger are disposed of by id: **CX-1**
(MAJOR, stale state across the arm lock), **CX-2** (MINOR, the successful hook
path discards the re-arm notice), **CX-3** (MINOR, recovery's aggregate and
recovery's replace=true inside `--if-down`).

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
(`cmd/metasystem/steward_verbs.go:567-586`, `ClassHuman` or refuse).

The engine is rebuilt whenever a Go change lands and the seat pulls; the
enrolled path is `bin/metasystem` in the installation. So every landing
changed the digest, every `up` on every seat refused, and the Stop hook turned
the refusal into a blocked turn (`scripts/agents/supervision-hook.sh:422-425`
records "supervision arming failed"; 530-533 and 585-588 emit the block). A
human had to type the command at a terminal, or relay a temporary word under
R-29-m2. Wido's standing order R-37-m3 (`memory/rulings.md:64`, 2026-08-31,
verbatim "rearm after an engine rebuild is always allowed/necessary until
further notice. for all machines") already authorizes the act on every
machine without a fresh per-case word and asks that every re-arm record the
commit it consumed. Nothing mechanized it; m2 lost a night of re-prompts on
2026-09-04 and m1 lost most of 2026-09-05.

Two facts about the existing arm path govern everything below:

- `runner.go:382-447` (`arm`) takes the arm flock at 402-408 and does
  everything an arm changes inside it, BUT it reads the prior record at 418
  only for `prior.Generation`; the temporary word and review date it mints
  come from its arguments, which the caller computed before the lock. That is
  the window CX-1 names.
- `arm` mints at 424-430, then re-opens the enrollment (431), snapshots it
  (436), and launches the runner (439). The runner reads its generation from
  the record (`checkStewardRunner` compares the runner's recorded generation
  with `installedGeneration`, `health.go:560-583, 1153-1166`), so the record
  must exist before the launch. Mint-before-launch is structural, not an
  accident; Decision 4 inherits it deliberately.

## Decision 1 — what the pin protects, and which drift causes re-arm

### The threat-model reading

`identity.go:3-8` states the pin's purpose: "accident-proofing at the
repository's trust level: a stray cron job does not match a pinned record; a
same-user adversary is out of scope repo-wide." Read against the code, the
pin answers one question: **is the process asking to be the steward the engine
this repository enrolled?** It identifies the caller. It is not an integrity
seal over the bytes at the enrolled path, because at the repository's trust
level the same user who can write `bin/metasystem` can write
`identity.json` in the same tree with the same permissions (`MintIdentity`,
`identity.go:52-62`, is a plain owner-only file write); a pin that a same-user
actor can rewrite at will was never a defense against that actor, and the
header says so.

So the question the brief asks — is a rebuild at the enrolled path, invoked by
that engine, in that repository, owned by that user, a condition the pin
exists to catch — has the answer **no**. Every fact the pin checks about the
CALLER still holds in that case: the record is owned by this user with
owner-only mode, it names this repository, the enrolled path is canonical and
still names the inode that was opened, the file is a regular executable, and
the invoking process is the engine at that path. The one changed fact is the
digest of the bytes, and the bytes changed because the user's own build wrote
them at the user's own enrolled path. The stray cron job of the header runs
some other binary at some other path; it still fails the path check
(`up.go:404-408`) whether or not the digest matches. A copy of the engine
dropped at a stranger path fails the same check. Another checkout's engine
fails it. A record this user does not own fails `VerifyIdentity`. None of
those become a re-arm.

The counter-position is on record and must be answered here, not left for
the critic: `plans/goals-drafts/provenance-anchored-rearm.md` argues that an
automatic re-arm "would launder seat-authored engine bytes into armed
authority" and sequences the automatic re-arm behind goal
two-bars-for-changes, which has not landed. That concern is real but it is
about PROVENANCE (did these bytes come through the landing gates), not about
IDENTITY (is this the enrolled engine), and the pin as coded has never checked
provenance: the human at the terminal types `steward arm` against whatever
`bin/metasystem` contains, and `requireHumanStewardEnrollment` proves only
that a human's terminal asked. The human's own standing order R-37-m3 grants
the re-arm now, on every machine, "bounded by construction to engines built
from commits that arrived through the landing gates; every re-arm records the
commit it consumed" — the bound is the repository's landing discipline, and
the record is what this design writes. This design therefore mechanizes
R-37-m3 as it stands and leaves the provenance tightening ONE named seam: the
eligibility check inside the lock (Decision 2, step 3). When two-bars lands,
that step gains the provenance test and nothing else in this design moves.
The draft's "R-21 incident shape" is a seat gaining status without a gate;
here the seat gains nothing it did not already have, because the seat already
runs `bin/metasystem` for every verb with this user's permissions, and the
runner it re-arms executes the same bytes the seat is executing to ask.

### The disposition of every drift cause

Each cause is either **RE-ARM** or **REFUSE**. Refuse means: no mint, no
record write, the existing `ENROLLMENT_DRIFT` refusal with its named cause,
and the remedy rewritten as Decision 3 says. The list is exhaustive over the
refusal sites in `identity.go` and `up.go`; the implementer adds no cause
without reopening this design.

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
| C13 | **Digest mismatch at the enrolled path** (`identity.go:284-287`), AND the invoking engine's canonical executable path equals `InstallPath` | **RE-ARM** | The one cause a rebuild produces, from the one caller the pin identifies. |
| C14 | Digest mismatch AND the invoking engine is not the enrolled path (`up.go:404-408` reached after C13 typed) | REFUSE | A stranger engine at a stranger path, a `METASYSTEM_BIN` override pointing elsewhere, another checkout's build. The record is untouched. |
| C15 | Digest matches AND the invoking engine is not the enrolled path (`up.go:404-408`) | REFUSE | Same-bytes-elsewhere is still a stranger caller; unchanged from today. |
| C16 | Snapshot drift in `PrepareForExecution` (`identity.go:194-196`) or pre-exec drift in `Command` (`identity.go:228-235`) | REFUSE | The bytes moved between verification and execution; a moving target. Verifiers stay pure. |
| C17 | `ENROLLMENT_CHANGED` under the arm lock in `repairPinnedRunner` (`runner.go:322-329`) | unchanged (stop, report) | Not a drift cause; a concurrent arm won. Recovery reports it as today. |
| C18 | Record carries a temporary word whose `ReviewBy` date has passed | RE-ARM (when C13 holds), carried verbatim | The review date is the human's obligation, not the machine's; the machine neither extends nor clears it. Health shows it (Decision 2). |

C13 is the only RE-ARM row. The typed error the prior implementation added,
`ErrEngineRebuilt` wrapped beside `ErrEnrollmentDrift` at the digest-mismatch
site (branch `identity.go:27-34, 292-294`), is kept exactly: verifiers keep
refusing both types; only `up`'s ordinary path reads the second type. C14's
ordering is a change from the prior implementation: the prior code re-armed
first and checked the invoking path afterwards inside `ReArmRebuiltEngine`;
this design checks the invoking path INSIDE the lock as the first eligibility
step (Decision 2), so no record read happens for a stranger before the
decision that refuses it.

## Decision 2 — the re-arm as one act under the arm lock (folds CX-1)

### What changes in `arm`

`runner.go:382-447` is refactored so that everything minted is decided from a
record read INSIDE the lock. The mechanical shape:

```go
// mintPlan is decided under the arm lock from the prior record as it is at
// that moment. Human paths ignore prior for the word; the machine path
// re-verifies eligibility and copies the carry-forward fields from prior.
type mintPlan struct {
    Skip       bool   // true: mint nothing, the enrollment is already current
    MintedBy   string // "human-terminal" | "human-word" | "machine-rebuild"
    Word       string
    ReviewBy   string
    Witnessed  int    // HumanWitnessedGeneration to write
    WitnessedAt string
    EngineBuild string
}

func arm(repoRoot, binaryPath string, replace bool,
    decide func(prior InstallIdentity, priorErr error) (mintPlan, error)) (string, error)
```

`Arm`, `ArmTemporary`, and `Restart` keep their signatures and pass a
`decide` that returns their fixed plan (`MintedBy` "human-terminal" for `Arm`
and `Restart`, "human-word" for `ArmTemporary`; `Witnessed` = the generation
about to be minted, `WitnessedAt` = its `MintedAt`). Inside `arm`, after the
flock at 402-408 and the live-runner handling at 410-417, the prior record is
read ONCE (`VerifyIdentity`, the current line 418) and passed to `decide`; the
mint at 424-430 uses the plan's fields, never an argument computed before the
lock. `runner.go:399-401`'s comment ("EVERYTHING an arm changes happens inside
the lock") becomes true for the word and review date as well.

### The machine path, step by step

`ReArmRebuiltEngine(repoRoot, invokingBinary string) (ReArmOutcome, error)`
replaces the branch's version (branch `runner.go:172-197`). It calls `arm`
with `replace=true` and this `decide`, every step of which runs under the
arm lock:

1. `priorErr != nil` → return that error wrapped in `ErrEnrollmentDrift`; no
   mint. (Causes C1–C6 re-checked under the lock.)
2. `canonicalPath(invokingBinary) != prior.InstallPath` → refuse with
   "engine %q is not the enrolled engine %q"; no mint. (C14, decided before
   anything else.)
3. Eligibility re-check: open `prior.InstallPath` and digest the open
   descriptor exactly as `OpenEnrolledBinary` does (the implementer factors
   `identity.go:256-287` into a helper both call). Any failure other than a
   digest mismatch → refuse with that cause; no mint (C7–C12 under the lock).
   A digest EQUAL to `prior.InstallDigest` → `Skip: true`: a concurrent re-arm
   or a human arm already brought the record current; `arm` mints nothing,
   does not stop the live runner, and returns "already current (generation
   N, runner pid P)". This is where two-bars later adds the provenance test.
4. Otherwise the plan is: `MintedBy` "machine-rebuild"; `Word` and `ReviewBy`
   copied from `prior` as read in this same locked section (CX-1's fix: a
   concurrent `ArmTemporary` that won the lock first is what `prior` now
   shows, so its word rides forward instead of being overwritten by stale
   empty strings); `Witnessed`/`WitnessedAt` copied from `prior` when prior
   carries them, else `prior.Generation`/`prior.MintedAt` (the legacy rule
   below); `EngineBuild` = `supervise.BuildStamp`, the invoking engine's own
   linked build commit (`internal/supervise/disk.go:139`, set by
   `scripts/agents/go-build.sh:51,60` through `-ldflags -X`; the steward
   package already imports `supervise` in `tick.go:15` and `reap.go:20`, so
   no new import direction is introduced).
5. `arm` continues unchanged: stop the live runner (it executes the old
   snapshot), mint generation `prior.Generation+1` with the plan, re-open,
   snapshot, launch.

Because the plan is decided under the lock, two concurrent re-arms serialize:
the second finds step 3's digest equal and skips (fixture F3). A re-arm
racing `ArmTemporary` serializes the same way: whichever takes the lock second
sees the first's record (fixture F4). `arm`'s `replace=true` on the machine
path is deliberate and bounded to `ordinary` `up`; recovery never reaches it
(Decision 3).

`ReArmOutcome` is `{Status string; Generation, PreviousGeneration int;
RunnerPid int64; Minted bool}` with `Status` one of `re-armed`,
`already-current`. An error return with `Minted: true` means the mint landed
and the launch failed (Decision 4).

### The machine-minted stamp on `InstallIdentity`

Wido's ruling (2026-09-05): carry the temporary word and review date forward
AND stamp which generations were machine-minted rather than human-witnessed.
Three fields are added to `InstallIdentity` (`identity.go:28-48`):

```go
// MintedBy names the act that minted this generation: "human-terminal"
// (steward arm or restart at an agent-free terminal), "human-word"
// (steward arm --temporary-human-word), or "machine-rebuild" (ordinary up
// re-arming the rebuilt engine at its enrolled path). Absent on records
// minted before the stamp existed, which are read as human-witnessed.
MintedBy string `json:"mintedBy,omitempty"`
// HumanWitnessedGeneration is the newest generation a human act minted;
// every generation above it up to Generation was machine-minted. Equals
// Generation on a human-minted record.
HumanWitnessedGeneration int    `json:"humanWitnessedGeneration,omitempty"`
HumanWitnessedAt         string `json:"humanWitnessedAt,omitempty"`
// EngineBuild is the linked build commit of the engine this generation
// enrolled (supervise.BuildStamp, "dev" on an unstamped build), recorded so
// a re-arm names the commit it consumed (R-37-m3).
EngineBuild string `json:"engineBuild,omitempty"`
```

Rules: a human path writes `MintedBy` to its human value and
`HumanWitnessedGeneration`/`At` to the generation it mints, which is how a
human re-arm clears the machine-minted state — there is no separate clearing
verb, the next `steward arm` or `steward restart` at the terminal (or
`--temporary-human-word` relay) is the clearing act, exactly as it is today
for the temporary word. The machine path copies the witnessed pair forward and
sets `MintedBy` "machine-rebuild". `EngineBuild` is written by every path.
`VerifyIdentity` does not validate the new fields; a legacy record (empty
`MintedBy`) is read as human-witnessed at its own generation, and the
self-grade names the one known exception. `TemporaryHumanWord` and `ReviewBy`
are carried byte-for-byte; the machine never validates, extends, or clears
them (C18).

### What health shows

Health today renders the steward-runner role as "runner pid N and generation
G success are current" (`health.go:582-583`) and knows the installed
generation only as an integer (`installedGeneration`, 1153-1166). The
identity.go header's claim that "health and every reader see the temporary
state" has no reader in `health.go` (grep for the field finds none); this
design gives both states one reader. `installedGeneration` becomes
`installedEnrollment(repoRoot) (InstallIdentity, error)` and the
steward-runner role's reason string, in every status where the record was
readable, ends with one provenance clause rendered by
`EnrollmentProvenance(id InstallIdentity) string`:

- human-minted, permanent: `enrollment generation 9 human-witnessed`
- machine-minted: `enrollment generation 9 machine-minted (rebuild, engine
  <commit>) above human-witnessed generation 5 of 2026-09-02T11:21:41Z`
- with a temporary word, either case: append `; TEMPORARY under a recorded
  remote human word, review by 2026-09-06`

The aggregate does not change: a machine-minted generation is information,
not a fault, so it moves no role to dead or unknown and adds no role. An
overdue `ReviewBy` is likewise rendered, not judged; judging it is outside
this goal. The `HEALTH` line the hook previews (`health.go:192`) carries the
clause because it carries every role's reason.

## Decision 3 — visibility (folds CX-2 and CX-3)

### The outcome line

`ordinary` reports the component as the branch did
(`component=accepted-engine outcome=re-armed detail="the enrolled engine was
rebuilt; armed (runner pid P) (generation=9 previous=8 path=...)"`) and, new,
the aggregate line carries one optional key so a consumer that reads only the
last line sees it: `up outcome=armed authority=writer re-armed="generation=9
previous=8"`. `Result` gains `ReArmed string`, rendered by `Lines()`
(`up.go:76-106`) between `authority` and `holder` when non-empty. An
`already-current` outcome (Decision 2, step 3) renders as
`component=accepted-engine outcome=verified` with the detail noting "brought
current by a concurrent arm"; it sets no `re-armed` key, because this call
minted nothing. The ordering in `ordinary` is unchanged: the re-arm happens
inside `openInvokingEnrollment` (418-421), before session identity, the
announcement, the lease, and supervision, because its authority is the
engine's own identity at the enrolled path (Decision 1), not the session's;
an advisor session's `up` re-arms too, or the second seat on a machine stays
wedged while the holder idles.

The refusal remedy at `up.go:392` is rewritten for every REFUSE row so it no
longer names only the terminal: "this engine is not the enrolled one at its
enrolled path; from an agent-free terminal run metasystem steward arm --repo
<root> (steward restart when a runner is live), or relay the human's recorded
word with --temporary-human-word and --review-by". `<root>` is
`options.Root`. The fleet-join bootstrap design proposed the same rewrite
(`plans/fleet-join-bootstrap-design.md:417`); this design lands it.

### The hook on success (CX-2)

`supervision-hook.sh` reads `up`'s output only on failure (Stop: 422-425;
session start: 690-698 exits 0 on success). Two additions, both benign and
never blocking:

- Session start (690-698): on `up` exit 0, if the last line of `$output`
  contains ` re-armed=`, emit `surface_json "Metasystem re-armed the rebuilt
  engine: <last line>"` before `exit 0`. Any other success stays silent as
  today.
- Stop (410-425): a new variable `up_notice=` beside `up_failure`; on
  `up_rc == 0` with ` re-armed=` in the last line of `$up_output`, set
  `up_notice="Metasystem re-armed the rebuilt engine: <last line>"`. It joins
  `extras` (616-619) exactly as `up_failure` does, so it rides the
  non-blocking channel and never enters a block reason (the 621-627 rule that
  the display is the block reason byte-verbatim is untouched). The advisor
  branch (539-554) and the degraded branch (637-650) append it where they
  append `up_failure`.

The match is on the aggregate key, not on prose, so a future wording change
in the component detail cannot silence the hook.

### Recovery's aggregate, and whether unattended recovery may re-arm (CX-3)

**Unattended recovery never re-arms.** `recovery` (`up.go:502-547`) keeps
calling the pure `OpenEnrolledBinary` path: a rebuilt engine at the enrolled
path is `ENROLLMENT_DRIFT` there, with the remedy "run metasystem up from a
session, which re-arms a rebuilt engine at its enrolled path; or steward arm
at an agent-free terminal". Three reasons, each sufficient: the scheduler
entry is `--recover-only --if-down`, whose contract is "start only missing
repository rings" (`cmd/metasystem/up.go:83`, `docs/orchestration.md:239`),
and a re-arm STOPS a live runner to replace it; nobody reads recovery's
output, so a machine-minted generation there would be the one place the stamp
has no witness at all; and the wedge this goal fixes is a seat unable to arm,
which the seat's own next `up` now resolves. The hook's no-identity Stop path
(419-420) also runs `--recover-only --if-down` and therefore also does not
re-arm; an unidentified session cannot arm today either, and that path is
unchanged. `openInvokingEnrollment` therefore takes a boolean
`allowReArm` that `ordinary` passes true and `recovery` passes false; the
branch's shared use (branch `up.go:534-546`) is withdrawn.

With that, CX-3's first half (recovery reporting `recovery-not-needed` after
a re-arm replaced the runner) cannot occur, and its second half (replace=true
inside `--if-down`) is disposed of by narrowing rather than by accepting: the
`--if-down` contract stands as written. The aggregate rule at `up.go:542-545`
is unchanged.

## Decision 4 — failure shape: the automatic path inherits mint-before-launch, visibly

`arm` mints before it launches (`runner.go:424-439`) and a failed launch
leaves the bumped generation in place. The automatic path **inherits** this,
for a structural reason and a convergence reason. Structural: the runner
proves itself by completing a generation-bound tick against the record
(`checkStewardRunner`, `health.go:560-583`), so the record must name the new
generation before the runner starts; a mint-after-launch would have the
runner prove a generation that does not exist. Convergence: after a failed
launch the record is TRUE — the bytes at the enrolled path are the bytes it
names — so the next `up` finds the enrollment current and repairs the runner
through `EnsureRunner` → `repairPinnedRunner` (`runner.go:220-266, 306-367`),
which is the shipped repair path and mints nothing. A mint-then-rollback would
instead leave the next `up` re-minting again, burning a generation per
attempt with nothing gained.

What the automatic path must NOT do is report the failure as drift.
`ReArmRebuiltEngine` returns `Minted: true` with the launch error, and
`ordinary` renders `component=accepted-engine outcome=re-armed` (the mint
happened; say so) followed by `component=steward-runner outcome=failed
detail=<launch error> remedy="inspect artifacts/agents/steward/runner.log,
then rerun metasystem up"`, aggregate `up outcome=failed component=steward-runner`
with the `re-armed=` key present. The branch's rendering of this case as
`ENROLLMENT_DRIFT` "re-arming the rebuilt engine failed" (branch
`up.go:410-413`) is withdrawn: it named a remedy (the terminal) that the state
did not need. The failed generation carries `MintedBy: machine-rebuild`, so a
reader can tell it from a human act; the human-witnessed pair is unchanged by
it.

## Decision 5 — fixtures

Each fixture names its session id; Go tests carry it as the `Options.Session`
value where `up` is involved and as the fixture's tag otherwise. Every fixture
that launches a runner runs in a process-owning group with its own registry
home and notify command, the way `TestArmConfirmsTheGuardAndDisarmEndsIt`
(`runner_test.go:156-200`) already does with the built engine at
`bin/metasystem`. Two DIFFERENT valid engine builds are needed for a real
rebuild: on macOS, appending bytes to a signed Mach-O invalidates its
signature and the kernel kills it, so "rebuild" is staged as a second real
build of the engine with a different linked build commit value (the go-gate
already builds one; the fixture builds the second with the same `-ldflags -X
...supervise.BuildStamp=<value>` form `scripts/agents/go-build.sh:51,60`
uses, naming a different value), installed at the enrolled path by
stage-and-rename, never by copying over the live file.

| Id | Session id | Where | Proves |
| --- | --- | --- | --- |
| F1 real rebuild re-arms | `rearm-rebuild` | `internal/up` process-owning test | Arm engine A at `<root>/bin/metasystem`; rename engine B onto it; `ordinary` with `Binary` = that path returns `outcome=armed`, component `accepted-engine outcome=re-armed`, aggregate carries `re-armed="generation=2 previous=1"`; the record has generation 2, `MintedBy` machine-rebuild, `HumanWitnessedGeneration` 1, `EngineBuild` = B's commit; the live runner's snapshot digest is B's; health's steward-runner reason carries the machine-minted clause. |
| F2 stranger refuses untouched | `rearm-stranger` | `internal/up` unit test (extends `TestOrdinaryUpRefusesAForeignEngineWithoutMintingANewGeneration`, branch) | Enrollment at path P generation 7; bytes at P changed AND `Binary` is a different path Q: `ENROLLMENT_DRIFT`, record still generation 7 with the old digest, no `mains`/lease directories created, remedy names steward arm and the temporary-word relay. Second firing: bytes unchanged, `Binary` Q: same refusal (C15). |
| F3 two concurrent re-arms mint once | `rearm-concurrent` | `internal/steward` test using a `beforeLock`-style seam like `runner_test.go:443-446` | Two goroutines call `ReArmRebuiltEngine` on the same rebuilt enrollment; the seam holds the first inside the lock until the second is blocked on the flock; outcomes are one `re-armed` and one `already-current`; the record's generation advanced by exactly one; one live runner. |
| F4 re-arm racing ArmTemporary keeps the human's word | `rearm-vs-human-word` | `internal/steward` test with the same seam | Permanent enrollment, engine rebuilt; `ReArmRebuiltEngine` is held BEFORE the flock while `ArmTemporary(word W, review R)` completes on the rebuilt bytes; released, the re-arm returns `already-current`, and the record carries W and R with `MintedBy` human-word. Second half: `ArmTemporary` is held before the flock while the re-arm completes; released, the human's arm mints the next generation with W and R and `HumanWitnessedGeneration` equal to its own generation. Neither order ever writes empty word fields over W (the CX-1 laundering, both orders). |
| F5 hook surfaces the re-arm | `rearm-hook-surfaces` | `scripts/agents/supervision-hook-fixtures.sh` (the stub-engine pattern at 215-235) | A stub `up` prints a component line and an aggregate line ending in `re-armed="generation=9 previous=8"` and exits 0: session start emits a `systemMessage` containing "re-armed the rebuilt engine" and that line; Stop with an allow verdict emits it in the non-blocking `systemMessage`; Stop with a block verdict keeps the block reason byte-identical to the display and carries the notice only in the system message. A stub `up` that exits 0 without the key emits nothing new. |
| F6 failed launch after a machine mint | `rearm-launch-fails` | `internal/up` process-owning test | Engine B is a build whose `steward run` exits immediately (a second `-ldflags` value the runner reads as "die"); `ordinary` reports `accepted-engine outcome=re-armed`, `steward-runner outcome=failed`, aggregate `outcome=failed` with the `re-armed` key; the record is generation 2 `MintedBy` machine-rebuild; a following `ordinary` with a good engine C at the path re-arms again to generation 3 and its runner lives. |
| F7 recovery never re-arms | `rearm-recovery-refuses` | `internal/up` unit test (beside `TestRecoveryRefusesBeforeArmingWithoutAnEnrolledEngine`) | Rebuilt bytes at the enrolled path; `recovery` with `RecoverOnly` and `IfDown` returns `ENROLLMENT_DRIFT`, the record unchanged, the remedy naming a session's `up`. |

F1, F3, F4, and F6 are the fixtures that launch runners; they are
unrunnable in a delegate sandbox (KI-15) and the orchestrator runs them
outside it.

## Consistency pass

- Doctrine to sweep the same hour this lands (the rulings-sweep rule):
  `docs/orchestration.md:239` and the orchestration skill's copy say
  "Ordinary, advisor, and recovery-only `up` only consult that standing
  enrollment"; the sentence becomes "Recovery-only `up` only consults that
  standing enrollment; ordinary and advisor `up` re-arm the enrolled engine
  when it was rebuilt at its enrolled path and refuse every other drift."
  `identity.go:3-8` gains one sentence naming the rebuild reading.
  `cmd/metasystem/up.go`'s `--rearm` help ("ordinary up replaces an older
  generation automatically") stops being false. R-37-m3's software-home
  column points at this design; the provenance draft stays sequenced behind
  two-bars with its seam named here.
- Untouched on purpose: `VerifyEnrolledBinary` and every classifier that
  authenticates a running steward (`lease/classify.go:433`, `stage.go:84`)
  stay pure; `EnrolledExecutionPath` keys the snapshot by generation and
  digest, so the old runner's snapshot and the new one never collide; the
  `--if-down` contract; the block-once refusal record; the temporary word's
  validation (`humanauthority.ValidateTemporaryWordPair`) is not re-run on
  carry-forward, because the machine copies, it does not authorize.
- The branch's hook changes to `find-ancestor --repo "$harness_root"` and
  the evidence directory are the hook-root goal's territory and are not part
  of this design.

## Self-grade

Grounding: every load-bearing claim is a file-and-line read at 789acb343
(`identity.go` whole, `runner.go` whole, `up.go` whole, the hook's Stop and
start bodies at 380-698 and its header at 1-60, `steward_verbs.go:496-586`,
`health.go:180-262, 515-583, 1153-1166`, `runner_test.go:156-200, 425-460`,
`up_test.go` test names, the hook fixture's stub engine at 215-235 and
303-306, `memory/rulings.md:57, 64`, `docs/orchestration.md:239`,
`plans/goals-drafts/provenance-anchored-rearm.md`, and the branch diff
2477e73d5..df0597910 for identity.go, runner.go, up.go, the hook, and the
tests). Nothing was executed; this is a design job and the go-gate was not
run because no product bytes changed.

Residual risks, honestly: (a) `EngineBuild` reads `supervise.BuildStamp`,
which is "dev" on any engine built without `go-build.sh`'s `-ldflags`; a
machine-minted record then names "dev" rather than a commit, which is
truthful about the build but weaker than R-37-m3's "records the commit it
consumed" — the fixture engines are built with the flag so the field is
exercised, and a "dev" stamp on a fleet machine is a build-discipline
finding for health, not something this design hides; (b) the
three generations m1 minted on 2026-09-05 from the branch carry no stamp and
will read as human-witnessed under the legacy rule — the self-grade names
them so the m1 record is re-armed by a human once after this lands, which
also seeds `HumanWitnessedGeneration`; (c) the two-build fixture needs the
gate to build a second engine, adding build time to F1 and F6, accepted over
a byte-appended binary that macOS would kill; (d) an advisor session's `up`
re-arming the repository runner is new authority for advisors and is argued
from the engine's identity, not the session's, which the critic should
attack; (e) `re-armed=` on the aggregate line is a new key in an output the
fixtures parse as stable records — additive, but every fixture that asserts
the exact aggregate string must be checked; (f) the health clause lengthens
every steward-runner line on every machine; it is the visibility Wido asked
for, and a shorter rendering is a wording choice the implementer may not make
alone. Grade: pass against everything read; the reject condition is the
falsifier.

**Reject condition — reject this design if any of the following is shown:**
a drift cause not in the C1–C18 table that reaches `up`; any RE-ARM on a
caller whose canonical executable path is not the enrolled path, or on any
cause other than C13; any field of the minted record computed from a read
taken outside the arm flock (the CX-1 recreation); any order of a re-arm and
an `ArmTemporary` in which the human's word or review date is replaced by
empty strings; two concurrent re-arms of one rebuild that mint two
generations; a machine-minted generation whose record cannot be told from a
human-minted one, or a human re-arm that leaves `MintedBy` at
machine-rebuild; a successful re-arm that a session-start or Stop hook fires
without surfacing (the CX-2 silence); any re-arm reachable from
`--recover-only`, or from the hook's no-identity Stop path; any `--if-down`
invocation that stops a live runner (the CX-3 excess); a failed launch after a
machine mint reported as `ENROLLMENT_DRIFT` or with the terminal as its
remedy; a machine mint that rolls back or leaves the record naming bytes that
are not at the enrolled path; a temporary word carried forward that the
machine validated, extended, or cleared; or a fixture in Decision 5 weakened
from its stated assertion.
