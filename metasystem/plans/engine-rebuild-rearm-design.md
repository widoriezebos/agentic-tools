# Design: a rebuilt engine at its enrolled path re-arms itself; every other drift cause still reaches the human

Goal: engine-rebuild-rearms-itself (plans/goals/engine-rebuild-rearms-itself.md,
revision 15, tier 3 DESIGN-BEARING). Author: implementer delegate under
dispatch by m1 (lineage main-1788594343-3833-fb64b9). Revision 1 was written by
job engine-rebuild-rearm-design-r1c-20260906 and landed at a4c5947d5; revision
2 by job engine-rebuild-rearm-fold2-20260906 landed at ce6e10c56; this is
**Revision 3, 2026-09-06**, written by job err-fold3-20260906. Revision 3
re-aims the eligibility rule on Wido's decision of 2026-09-06 (provenance-based
eligibility, recorded verbatim in the goal's next step) and folds the six
material findings of the round-2 critique register
(`records/misc/engine-rebuild-rearm-critique-r2.md`, landed; its claims and
evidence bind) by id. Every seam cited below was read in this worktree at
commit c209abcb5; the prior implementation was read in revision 2 as the diff
between 2477e73d5 and df0597910 on branch m1-2026-09-05-verified and is prior
art only (it no longer compiles against main). Line numbers are HEAD's unless
marked "branch".

The three Codex findings on the goal ledger stay disposed of by id: **CX-1**
(MAJOR, stale state across the arm lock: Decision 2), **CX-2** (MINOR, the
successful hook path discards the re-arm notice: Decision 3), **CX-3** (MINOR,
recovery's aggregate and replace=true inside `--if-down`: Decision 3).

## Wido's decision, and what it dissolves

Revision 2 tried to prove WHO is asking: it bound automatic re-arm to a
resolved session identity as "the fact a stray cron cannot supply".
ERAR-R2-01-SESSION-PROOF showed no such proof exists: `proveCallerDescendsFromTarget`
(`internal/up/up.go:146-161`) returns success at once when the caller pid IS
the target pid, `cmd/metasystem/up.go:129-133` passes the command's parent as
`CallerPid`, and the fixture driver (`scripts/agents/supervision-fixtures.sh:591-605`)
passes its own pid and start time. A cron shell can do the same. Nothing in
`resolveSessionIdentity` (`up.go:163-242`) proves the target is an agent
session.

Revision 3 proves WHAT THE BYTES ARE instead. **A rebuilt engine at the
enrolled path is eligible for automatic re-arm if and only if its build stamp,
read from the bytes digested under the arm lock, names a landed commit on an
owned landing ref.** The caller's identity no longer participates in
eligibility. A stray cron that re-arms a provably landed build has done nothing
harmful: the bytes it enrolled are the bytes the landing gates produced, and the
record says which commit. A stray cron that invokes anything else refuses to
the human exactly as today. R-37-m3 (`memory/rulings.md:64`) already bounds
re-arm "by construction to engines built from commits that arrived through the
landing gates" and asks that every re-arm record the commit it consumed; this
revision makes that bound the eligibility rule itself.

## Where each round-2 finding lands

| Finding | Folded in | One-line disposition |
| --- | --- | --- |
| ERAR-R2-01-SESSION-PROOF (high) | Decision 1 (rule), Decision 3 (order) | **Dissolved, not answered.** The session-identity binding leaves the eligibility rule; `MintedBySession` leaves the record; session resolution returns to where `up` has it today, after the enrollment open. |
| ERAR-R2-02-PROVENANCE-SCHEMA (high) | Decision 1 (stamp grammar, witness resolution, "landed"), Decision 2 (`decide` step 4), Decision 5 (F8, F11) | Witness-digest stamps count: the stamp resolves to the newest commit on the owned ref whose ENGINE-projection digest matches, by the same computation the gate used. A stampless or `dev` engine refuses to the human. |
| ERAR-R2-03-LANDING-REF (high) | Decision 1 (C21), Migration | The ref is owned: `metasystem.steward.landing-ref`, installation-scoped git configuration, seeded by adoption and fleet bootstrap, validated under the arm lock, failing closed when absent. `refs/heads/main` is withdrawn as a hard-coded anchor. Fixture F1 runs on a non-main ref. |
| ERAR-R2-04-PRE-MINT-STOP-STAGE (medium) | Decision 4 | The stage contract gains `StageStopping`: the restart marker was written and signals were sent, the mint did not happen; its rendering names the runner it touched. |
| ERAR-R2-05-VISIBILITY-DURABILITY (medium) | Decision 3 | One owner and one order: the identity record is the record of truth, written first under the lock; the pending notification is written second under the lock and its failure is reported, never fatal; the arming-log line is `up`'s best-effort trail, written after `arm` returns, and stays best-effort. Each kill window is named with what a reader sees after it. |
| ERAR-R2-06-C16-FIXTURE-SEAM (medium) | Decision 3, Decision 5 (F10 split) | Two package-level function variables in `internal/up`, on the `sessionParentPid` precedent (`up.go:127`), are the seam; the steward half is proven separately by inducing real command-time drift against production code. |

Round-1 findings keep their revision-2 dispositions except where this revision
moves them: ERAR-R1-01-STRAY-CRON is now answered by provenance (a cron cannot
make an unlanded build eligible), and ERAR-R1-03-PROVENANCE is now the rule
rather than one of two bindings. ERAR-R1-02, -04, -05, -06, -07, -08 and the
non-material -09 stand as folded in revision 2 and are restated below where
they touch changed text.

## The defect, restated against the code

`internal/steward/identity.go:242-290` (`OpenEnrolledBinary`) authenticates the
record at `artifacts/agents/steward/identity.json`, opens the enrolled path,
proves the path still names the opened inode, and compares the digest of the
open descriptor with `InstallDigest`; a mismatch is `ENROLLMENT_DRIFT`
(`identity.go:284-287`). `internal/up/up.go:399-410` (`openInvokingEnrollment`)
calls it first thing in both `ordinary` (418-421) and `recovery` (508-511) and
adds one refusal: the invoking engine's canonical path must be the enrolled
path (404-408). Every refusal renders as `component=accepted-engine
outcome=ENROLLMENT_DRIFT` with the terminal remedy (`up.go:391-397`), and
`steward arm` and `steward restart` enforce that terminal through
`requireHumanStewardEnrollment` (`cmd/metasystem/steward_verbs.go:570-586`).

The engine is rebuilt whenever a Go change lands and the seat pulls; the
enrolled path is `bin/metasystem` in the installation. Every landing changed
the digest, every `up` on every seat refused, and the Stop hook turned the
refusal into a blocked turn (`scripts/agents/supervision-hook.sh:422-425`
records "supervision arming failed"). Nothing mechanized R-37-m3; m2 lost a
night of re-prompts on 2026-09-04 and m1 lost most of 2026-09-05.

Two facts about the existing arm path govern everything below:

- `runner.go:382-447` (`arm`) takes the arm flock at 402-408 and does
  everything an arm changes inside it, BUT it stops a live runner at 410-417
  BEFORE it reads the prior record at 418, and the temporary word and review
  date it mints come from its arguments, computed before the lock (CX-1,
  ERAR-R1-02).
- `arm` mints at 424-430, then reopens the enrollment (431), snapshots it
  (436), and launches the runner (439). The runner proves itself by a
  generation-bound tick against the record (`checkStewardRunner`,
  `health.go:560-583`), so mint-before-launch is structural; Decision 4 types
  its stages.

Two facts about how engines are stamped govern the eligibility rule:

- `scripts/agents/go-build.sh:43-59` is the one fenced build. Its stamp is
  `METASYSTEM_BUILD_STAMP` when set, else `git rev-parse --short HEAD`, else
  the literal `unknown`; it links the stamp as
  `supervise.BuildStamp` (`internal/supervise/disk.go:139`, default `dev`)
  with `-buildvcs=false`. Go records the `-ldflags` value in the binary's
  build information (verified in revision 2 on go1.26.5), and
  `debug/buildinfo.Read` reads it from an open descriptor.
- The canonical gate does NOT stamp a commit. `scripts/agents/go-gate.sh:492-508`
  computes the ENGINE-projection digest of the judged tree
  (`gate_surface_identity`, 121-131, through `behavior-surface digest
  --root <root> --projection ENGINE`, no `--prefix`), exports
  `METASYSTEM_BUILD_STAMP="witness-${witness_digest:0:12}"` (503), and every
  build in that run, the final installed one included (549-556), carries
  that stamp. `scripts/agents/witness-gate.sh:100-144` runs that gate inside
  a `git archive HEAD` snapshot (`HEAD:<prefix>` for a nested metasystem) and
  installs the snapshot's engine onto `bin/metasystem` by stage-and-rename
  (135-140). The engine a seat gets from ordinary validation is therefore
  stamped `witness-<12 hex>`; revision 2's rule rejected it (ERAR-R2-02).

One observed fact about the fleet governs the migration: the engine enrolled
on m1 at generation 9 (minted 2026-09-05T11:07:59Z, temporary word carrying
the R-37-m3 text, review by 2026-09-06, no provenance fields) was built
outside `go-build.sh`; its build information shows `vcs.modified=true` and no
`-ldflags` setting, so its stamp is `dev`. That is the case C19 refuses.

## Decision 1 — what the pin protects, and which drift causes re-arm

### The threat-model reading, corrected again (folds ERAR-R2-01, -02, -03)

`identity.go:3-8` states the pin's purpose: accident-proofing at the
repository's trust level; a same-user adversary is out of scope repo-wide.
Revision 1 read the pin as identifying the caller's path; revision 2 as
identifying the caller's session. Neither is a fact the metasystem can prove
against a stray scheduler entry. What the metasystem CAN prove, from the
bytes alone, is where they came from: a stamp linked into the binary by the
one fenced build, resolved against the repository's own landing history.
Revision 3 therefore binds automatic re-arm to three facts, all read under the
arm lock, none about the caller:

1. **The bytes at the enrolled path changed** (the digest mismatch at
   `identity.go:284-287`), and the invoking engine's canonical executable path
   is the enrolled path (`up.go:404-408`, re-checked under the lock: C14).
2. **The installation owns a landing ref.** `metasystem.steward.landing-ref`
   is git configuration of the installation repository, read as `git -C
   <installation root> config --get metasystem.steward.landing-ref` (the
   same family and read pattern as `metasystem.steward.notify-command` and
   `metasystem.steward.tick-seconds`, `runner.go:55`). It is
   installation-scoped: per clone, never committed, never a `metasystem.conf`
   key, because which ref a machine lands on is that machine's fact (the
   landing wrapper pushes the checkout's current branch, `commit.sh:550-556`;
   `land.sh:232-236` reads the same). The value must be a fully qualified ref
   (starts with `refs/`) and `git -C <installation root> rev-parse --verify
   --quiet <value>^{commit}` must succeed at the moment of the locked read.
   Absent, unqualified, or unresolvable is **C21, refuse**, with the remedy
   naming the exact `git config` command. The engine carries no default:
   `refs/heads/main` is withdrawn as a hard-coded anchor (ERAR-R2-03). The
   installation root is `installationRoot(options)` in `up`
   (`up.go:129-134`: `--metasystem-root` when given, else the repository
   root), passed into the steward package as an argument; for the
   metasystem's own checkout it is the repository root, for an adopted tree
   it is the nested metasystem directory, and `git -C` resolves either to
   the enclosing repository's configuration.
3. **The stamp read from the digested bytes names a landed commit on that
   ref.** The stamp grammar is closed:
   - `[0-9a-f]{7,40}`: a commit stamp (go-build.sh's default). It resolves
     through `git -C <installation root> rev-parse --verify --quiet
     <stamp>^{commit}` to a full commit id; an ambiguous or unknown stamp is
     C19.
   - `witness-[0-9a-f]{12}`: a witness stamp (go-gate.sh:503). It resolves as
     the next subsection says; no match is C20.
   - `dev`, `unknown`, empty, unreadable build information, or any other
     shape: C19, refuse.

   **"Landed" means reachable from the owned ref's tip** as the installation
   repository holds it at the moment of the locked read: `git -C
   <installation root> merge-base --is-ancestor <full commit>
   <landing-ref>` succeeds. The ref is what the landing tooling advances;
   its ownership is the authority the ruling names. The landing wrapper's
   `Landing-Provenance` trailers (`commit.sh:519-525`) are NOT consulted by
   this rule: the human decided the rule as "a landed commit on an owned
   landing ref", and the stronger proof that a commit traversed the wrapper
   is the seam goal two-bars-for-changes later substitutes at Decision 2's
   `decide` step 4. An operator who points the key at
   `refs/remotes/origin/<branch>` gets the stricter "pushed" reading of
   landed; that is their choice, and the engine does not decide it.

The counter-position (`plans/goals-drafts/provenance-anchored-rearm.md`: an
automatic re-arm "would launder seat-authored engine bytes into armed
authority") is answered by the rule itself: seat-authored bytes carry a `dev`
stamp, a branch commit, or a witness digest of an unlanded tree, and every one
of those refuses. The human paths (`steward arm`, `steward restart`,
`--temporary-human-word`) are unchanged in what they accept: the human at the
terminal remains the authority for a `dev` build, and the record now says
truthfully what stamp they enrolled.

### How a witness stamp resolves to a landed commit (ERAR-R2-02)

The witness digest is `Policy.Digest(root, Engine)` (`internal/behaviorsurface/policy.go:335-343`):
a sorted, NUL-separated list of `<path> <kind> <sha256>` records over the
files the ENGINE projection includes (`policy.v2.json` `enginePaths`:
`cmd/**`, `internal/**`, `scripts/agents/**`, `go.mod`, `go.sum`,
`records/misc/goals-migration-manifest.md`), hashed with the policy version
compiled into the engine that computed it. It is a pure function of the
committed ENGINE-projection content of the judged tree, because the gate
computes it inside a `git archive HEAD` extract with a clean tree
(`witness-gate.sh:118-124`, `go-gate.sh:494-501`). So a witness stamp
identifies a tree, and the tree identifies the commits that carry it.

Resolution, under the arm lock, in the steward package:

```go
// resolveWitnessStamp finds the newest commit reachable from ref whose
// ENGINE-projection digest starts with hex12. It reproduces the gate's own
// computation: extract the commit's metasystem subtree into a private
// directory and digest it with the compiled policy and no prefix.
func resolveWitnessStamp(installationRoot, ref, hex12 string) (commit string, err error)
```

1. `toplevel := git -C <installation root> rev-parse --show-toplevel`;
   `prefix` is the installation root relative to `toplevel` (empty when
   equal), the same computation `witness-gate.sh:116-117` makes.
2. Candidates: `git -C <installation root> rev-list --max-count=64 <ref>`,
   newest first. The bound is fixed at 64 and is not configuration: a seat
   rebuilds from the tip it just pulled, so the first candidate matches in
   the ordinary case; a witness digest older than 64 landings behind the tip
   is refused (C20) with the human remedy, which is loud, never a silent
   enrollment.
3. For each candidate: create a private directory (mode 0700, removed before
   return), `git -C <toplevel> archive <commit>:<prefix>` (or `<commit>`
   when `prefix` is empty) restricted to the pathspecs in
   `Policy.EnginePaths` piped into `tar -x -C <dir>`, then
   `policy.Digest(dir, Engine)`. A partial extract that contains every
   included path yields the gate's digest exactly, because excluded files
   contribute no record; fixture F11 proves the equality against the gate's
   own digest of the same tree.
4. The first candidate whose digest has prefix `hex12` is the landed commit;
   `merge-base --is-ancestor` is implied by construction (the candidate list
   is the ref's history) and is run anyway so the two stamp shapes share one
   final test.

The policy used is the invoking engine's compiled policy. By C13 the invoking
engine is the engine at the enrolled path, which is the rebuilt engine the
gate stamped with that same policy; a version mismatch (the path replaced
under a running process by an engine with another policy) yields no match and
refuses. Cost: one archive and one digest walk per candidate, first candidate
first, inside a lock that already waits up to ten seconds for a runner.

### The disposition of every drift cause

Each cause is either **RE-ARM** or **REFUSE**. Refuse means: no mint, no
record write, no runner stopped, the existing `ENROLLMENT_DRIFT` refusal with
its named cause, and the remedy rewritten as Decision 3 says. The list is
exhaustive over the refusal sites in `identity.go` and `up.go`, the landing-ref
and stamp tests of this decision, and the two launch sites that execute a
pinned engine; the implementer adds no cause without reopening this design.
There is **one automatic cause, C13, with C18 conditional on it**.

| # | Cause (site) | Disposition | Why |
| --- | --- | --- | --- |
| C1 | Record absent (`identity.go:70-73`) | REFUSE | Never enrolled, or deleted: nothing to descend from. First enrollment is the human's act (fleet-join bootstrap owns it). |
| C2 | Record mode not owner-only (`identity.go:74-76`) | REFUSE | The record itself was disturbed. |
| C3 | Record owned by another uid (`identity.go:77-79`) | REFUSE | Not this user's enrollment. |
| C4 | Record unreadable or malformed (`identity.go:80-86`) | REFUSE | Nothing trustworthy to descend from. |
| C5 | `RepoIdentity` names another repository (`identity.go:87-89`) | REFUSE | The record serves a different installation. |
| C6 | `Generation` < 1 (`identity.go:90-92`) | REFUSE | No valid lineage. |
| C7 | `InstallPath` or `InstallDigest` empty (`identity.go:248-250`) | REFUSE | Incomplete enrollment. |
| C8 | `InstallPath` not canonical (`identity.go:251-255`) | REFUSE | Not a path this code would have written. |
| C9 | Enrolled path cannot be opened or inspected (`identity.go:256-273`) | REFUSE | No engine at the path; a rebuild in progress lands here once and the next `up` retries. |
| C10 | Path changed while pinning, `SameFile` false (`identity.go:274-276`) | REFUSE | A moving target is never enrolled. |
| C11 | Not a regular executable (`identity.go:277-279`) | REFUSE | Not an engine. |
| C12 | Digest read failure (`identity.go:280-283`) | REFUSE | Cannot prove what the bytes are. |
| C13 | **Digest mismatch at the enrolled path** (`identity.go:284-287`), AND the invoking engine's canonical executable path equals `InstallPath`, AND `metasystem.steward.landing-ref` is configured and resolves (not C21), AND the stamp read from the digested descriptor resolves to a commit reachable from that ref (not C19, not C20) | **RE-ARM** | The one cause a landed rebuild produces, enrolling bytes whose provenance R-37-m3 admits, whoever asked. |
| C14 | Digest mismatch AND the invoking engine is not the enrolled path (`up.go:404-408`, re-checked under the lock) | REFUSE | A stranger engine at a stranger path, a `METASYSTEM_BIN` override elsewhere, another checkout's build. |
| C15 | Digest matches AND the invoking engine is not the enrolled path (`up.go:404-408`) | REFUSE | Same-bytes-elsewhere is still a stranger caller; unchanged. |
| C16 | Snapshot drift in `PrepareForExecution` (`identity.go:194-196`), or pre-exec drift in `Command` (`identity.go:228-235`) reached from the supervision-owner launch (`supervise/arming.go:603-608`, surfaced at `up.go:354-365`) or the steward-runner launch (`runner.go:455-458`, surfaced at `up.go:480-483`) | REFUSE, rendered `ENROLLMENT_DRIFT` at all three sites (ERAR-R1-08) | The bytes moved between verification and execution. Decision 3 routes the two launch sites and names the seam that proves it (ERAR-R2-06). |
| C17 | `ENROLLMENT_CHANGED` under the arm lock in `repairPinnedRunner` (`runner.go:322-329`) | unchanged (stop, report) | Not a drift cause; a concurrent arm won. |
| C18 | Record carries a temporary word whose `ReviewBy` date has passed | conditional on C13: carried verbatim when C13 re-arms; otherwise nothing | The review date is the human's obligation; the machine neither extends nor clears it. Health shows it. |
| C19 | Digest mismatch, invoking path equal, ref configured, AND the digested bytes carry no readable build information, or a stamp of `dev` or `unknown`, or a commit-shaped stamp that `rev-parse --verify <stamp>^{commit}` does not resolve, or a stamp outside the grammar | REFUSE with detail "rebuilt engine at <path> carries build stamp <value>; automatic re-arm is bounded to landed commits (R-37-m3)" and the human remedy | An unstamped or unattested build is the seat-authored engine the ruling excludes; the human may still enroll it at the terminal. The m1 engine's state today. |
| C20 | As C19 but the commit stamp resolves and is NOT reachable from the landing ref (`merge-base --is-ancestor` fails), or the witness stamp matches no commit among the newest 64 on the ref | REFUSE with detail "rebuilt engine at <path> was built from <stamp>, which is not landed on <ref>" (witness: "whose witness digest matches no commit among the newest 64 on <ref>") and the human remedy | A feature-branch build, or a witness of an unlanded tree, did not arrive through the landing gates. |
| C21 | Digest mismatch, invoking path equal, AND `metasystem.steward.landing-ref` is unset, does not start with `refs/`, or does not resolve to a commit in the installation repository | REFUSE with detail "the installation owns no landing ref (metasystem.steward.landing-ref is <unset/value>)" and the remedy "run  git -C <installation root> config metasystem.steward.landing-ref refs/heads/<landing branch>  once on this machine, or re-arm at the terminal" | Fail closed when the anchor is absent (ERAR-R2-03): without an owned ref, "landed" has no meaning and the human decides. |

The typed error the prior implementation added, `ErrEngineRebuilt` wrapped
beside `ErrEnrollmentDrift` at the digest-mismatch site (branch
`identity.go:27-34, 292-294`), is kept: verifiers keep refusing both types;
only `up`'s ordinary path reads the second type. C14 is decided INSIDE the
lock as the first eligibility step (Decision 2), before any runner is stopped.

## Decision 2 — the re-arm as one act under the arm lock (folds CX-1, ERAR-R1-02, -03, -04; ERAR-R2-02, -04)

### What changes in `arm`

`runner.go:382-447` is refactored so that everything minted is decided from a
record read INSIDE the lock, BEFORE the live runner is touched:

```go
// mintPlan is decided under the arm lock from the prior record and the
// enrolled bytes as they are at that moment. Human paths ignore prior for
// the word; the machine path re-verifies eligibility and copies the
// carry-forward fields from prior.
type mintPlan struct {
    Skip         bool   // true: mint nothing, stop nothing; Message explains
    Message      string // rendered when Skip is true
    MintedBy     string // "human-terminal" | "human-word" | "machine-rebuild"
    Word         string
    ReviewBy     string
    Witnessed    int    // HumanWitnessedGeneration to write; 0 = unknown (legacy base)
    WitnessedAt  string
    EngineBuild  string // the stamp as read from the digested bytes
    LandedCommit string // machine path: the full commit id the stamp resolved to
    LandingRef   string // machine path: the ref it was proven reachable from
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
    StageBeforeMint  ArmStage = iota // nothing changed: refused, skipped, excluded, already armed
    StageStopping                    // stopRunnerForReplacement was entered and returned an error; no mint
    StageMinted                      // the record was written; reopen failed
    StageReopened                    // reopened; snapshot failed
    StageSnapshotted                 // snapshot ready; launch failed
    StageLaunched                    // the runner confirmed
)

type armOutcome struct {
    Stage              ArmStage
    Message            string
    Generation         int
    PreviousGeneration int
    RunnerPid          int64 // the launched runner (StageLaunched)
    StoppedRunnerPid   int64 // the runner stopRunnerForReplacement targeted (StageStopping and later)
    NoticeErr          error // QueueNotification's error at step 6b, reported, never fatal
}

func arm(repoRoot, binaryPath string, replace bool,
    decide func(prior InstallIdentity, priorErr error, bytes enrolledBytes) (mintPlan, error)) (armOutcome, error)
```

`Arm`, `ArmTemporary`, and `Restart` keep their public signatures
(`(string, error)`, returning `outcome.Message`) and pass a `decide` that
returns their fixed plan (`MintedBy` "human-terminal" for `Arm` and
`Restart`, "human-word" for `ArmTemporary`; `Witnessed` = the generation
about to be minted, `WitnessedAt` = its `MintedAt`; `EngineBuild` =
`bytes.Stamp` written as read, `dev` included, because the human is the
authority there; `LandedCommit` and `LandingRef` empty, because no provenance
test gated the act and the record must not imply one).

### The steps under the lock (ERAR-R1-02, ERAR-R2-04)

Every arm, human or machine, runs these steps in this order; steps 1 through
4 change nothing on disk and stop nothing:

1. `runnerExclusion` and `NotifyCommand` checks as today (`runner.go:390-395`),
   then the flock (`402-408`).
2. Read the prior record ONCE: `VerifyIdentity` (today's line 418, moved up).
3. Open and digest the enrolled bytes ONCE: the helper factored from
   `identity.go:256-287` (open, `SameFile`, regular executable, digest)
   applied to `binaryPath` for human paths and to `prior.InstallPath` for
   the machine path; it also reads the build stamp from the same descriptor
   (`debug/buildinfo.Read(file)`, the `-ldflags` setting, the value after
   `supervise.BuildStamp=`). The descriptor stays open until the mint is
   done.
4. `decide(prior, priorErr, bytes)`. An error is a refusal: return
   `armOutcome{Stage: StageBeforeMint}` and the error. `Skip` is a no-op:
   read `liveRunner` (a read only) and return `StageBeforeMint` with the
   plan's message plus "runner pid P" when one is live.
5. Only now the live runner: `liveRunner` (today's 410). If alive and
   `!replace`, return `StageBeforeMint` with "already armed (runner pid P)"
   plus the provenance clause of the standing record. If alive and
   `replace`, set `StoppedRunnerPid` and call `stopRunnerForReplacement`
   (`runner.go:479-509`). **That function writes the restart marker and
   sends SIGTERM, and possibly SIGKILL, before any of its error returns**
   (480-485, 496-499, 508); an error here returns `Stage: StageStopping`
   with `StoppedRunnerPid` set and the error, and mints nothing. The runner
   may be stopping or dead; the record is unchanged; the next `up` finds C13
   again and either finds no live runner (mint and launch) or stops it
   again. Convergent, and never reported as a no-op (Decision 4).
6. Mint generation `prior.Generation+1` from the plan and from `bytes.Digest`
   (never from a digest read outside the lock): (6a) `MintIdentity`, an
   atomic rename; `Stage` becomes `StageMinted` the moment it returns. (6b)
   Immediately after, still under the lock, `QueueNotification` for the
   machine path (Decision 3); its error goes into `NoticeErr` and does not
   change the stage or abort the arm.
7. Reopen (`OpenEnrolledBinary`): `StageReopened` on success.
8. Snapshot (`PrepareForExecution`): `StageSnapshotted` on success.
9. Launch (`launchRunner`): `StageLaunched` on success.

Any error after 6a returns the stage reached and the error; the mint is never
rolled back (Decision 4). `runner.go:399-401`'s comment becomes true for the
word, the review date, the digest, the stamp, and the landing test, and gains
"and nothing is stopped before the decision".

### The machine path's `decide`

`ReArmRebuiltEngine(repoRoot, installationRoot, invokingBinary string)
(ReArmOutcome, error)` replaces the branch's version (branch
`runner.go:172-197`) and revision 2's `ReArmSession` argument, which is
withdrawn with the binding it served. It calls `arm` with `replace=true` and
this `decide`, every step of which runs under the arm lock at step 4 above:

1. `priorErr != nil` → return that error wrapped in `ErrEnrollmentDrift`; no
   mint (C1–C6 re-checked under the lock).
2. `canonicalPath(invokingBinary) != prior.InstallPath` → refuse "engine %q
   is not the enrolled engine %q"; no mint (C14, decided before any stop).
3. `bytes.Err != nil` → refuse with that cause (C7–C12 under the lock).
   `bytes.Digest == prior.InstallDigest` → `Skip: true` with "already
   current (generation N, runner pid P)": a concurrent re-arm or a human arm
   already brought the record current (fixture F3).
4. Provenance, in this order, each a refusal that mints nothing and stops
   nothing:
   - Landing ref (C21): read `metasystem.steward.landing-ref` from the
     installation repository; require the `refs/` prefix and a resolving
     `rev-parse --verify --quiet <ref>^{commit}`.
   - Stamp shape (C19): empty, `dev`, `unknown`, or outside the two grammars
     refuses.
   - Commit stamp: `rev-parse --verify --quiet <stamp>^{commit}` → full id,
     else C19; `merge-base --is-ancestor <full id> <ref>` else C20.
   - Witness stamp: `resolveWitnessStamp(installationRoot, ref, hex12)` →
     full id, else C20; then the same `merge-base` test.
   This step is the one named seam where goal two-bars-for-changes later
   substitutes its stronger landing proof; nothing else moves when it lands.
5. Otherwise the plan is: `MintedBy` "machine-rebuild"; `Word` and
   `ReviewBy` copied from `prior` as read in this same locked section (CX-1's
   fix: a concurrent `ArmTemporary` that won the lock first is what `prior`
   now shows); `Witnessed`/`WitnessedAt` copied from `prior` when
   `prior.MintedBy` is non-empty, else `0`/`""` (the legacy rule);
   `EngineBuild` = `bytes.Stamp`; `LandedCommit` = the full id from step 4;
   `LandingRef` = the ref from step 4.

Two concurrent re-arms serialize on the flock: the second finds step 3's
digest equal and skips without stopping the first's runner (F3). A re-arm
racing `ArmTemporary` serializes the same way (F4). `replace=true` on the
machine path is deliberate and bounded to `ordinary` `up`; recovery never
reaches it (Decision 3).

`ReArmOutcome` is `{Status string; Stage ArmStage; Generation,
PreviousGeneration int; RunnerPid, StoppedRunnerPid int64; EngineBuild,
LandedCommit, LandingRef string; NoticeErr error}` with `Status` one of
`re-armed`, `already-current`. An error with `Stage == StageStopping` means a
runner was signalled and nothing was minted; an error with `Stage >=
StageMinted` means the mint landed and a later step failed (Decision 4).

### The machine-minted stamp on `InstallIdentity`

Wido's ruling (2026-09-05): carry the temporary word and review date forward
AND stamp which generations were machine-minted rather than human-witnessed.
Six fields are added to `InstallIdentity` (`identity.go:28-48`);
`MintedBySession` of revision 2 is NOT added, because the session is not
resolved when the record is minted and the record must not name a fact the
act did not have:

```go
// MintedBy names the act that minted this generation: "human-terminal"
// (steward arm or restart at an agent-free terminal), "human-word"
// (steward arm --temporary-human-word), or "machine-rebuild" (ordinary up
// re-arming the rebuilt engine at its enrolled path). Absent on records
// minted before the stamp existed; such a record is LEGACY and is never
// read as human-witnessed.
MintedBy string `json:"mintedBy,omitempty"`
// HumanWitnessedGeneration is the newest generation a human act minted;
// every generation above it up to Generation was machine-minted. Equals
// Generation on a human-minted record. Zero means no human witness is
// recorded (a machine mint descended from a legacy record).
HumanWitnessedGeneration int    `json:"humanWitnessedGeneration,omitempty"`
HumanWitnessedAt         string `json:"humanWitnessedAt,omitempty"`
// EngineBuild is the build stamp read from the enrolled bytes themselves
// (the -ldflags -X supervise.BuildStamp value in the binary's build
// information), never from the minting process's own stamp. A human mint
// records whatever the bytes carry, "dev" included.
EngineBuild string `json:"engineBuild,omitempty"`
// LandedCommit and LandingRef are the provenance a machine-rebuild mint
// proved under the arm lock: the full commit id EngineBuild resolved to and
// the owned landing ref it was reachable from (R-37-m3: every re-arm
// records the commit it consumed). Empty on human mints.
LandedCommit string `json:"landedCommit,omitempty"`
LandingRef   string `json:"landingRef,omitempty"`
```

Rules: a human path writes `MintedBy` to its human value and
`HumanWitnessedGeneration`/`At` to the generation it mints, which is how a
human re-arm clears machine-minted and legacy state. The clearing act is
whichever human path MINTS: `steward restart` at the terminal, `steward arm
--temporary-human-word` (both `replace=true`), or `steward arm` after a
disarm; plain `steward arm` beside a live runner returns "already armed"
without minting (`runner.go:410-413`) and clears nothing (ERAR-R1-04). The
machine path copies the witnessed pair forward and sets `MintedBy`
"machine-rebuild". `EngineBuild` is written by every path from the digested
bytes. `VerifyIdentity` does not validate the new fields.
`TemporaryHumanWord` and `ReviewBy` are carried byte-for-byte; the machine
never validates, extends, or clears them (C18).

### Legacy records (ERAR-R1-04)

A record with empty `MintedBy` is **legacy**: minted before this design, by a
human or by the branch's machine path, and nobody can tell which. The rule is
truthfulness:

- Health renders it `enrollment generation 9 LEGACY (minted before
  provenance stamping; no human witness recorded)`.
- The machine path may descend from it (R-37-m3 authorizes the re-arm on
  every machine), but the new record carries `HumanWitnessedGeneration: 0`
  and health renders `machine-minted (rebuild, engine <stamp>, landed
  <short commit> on <ref>) above LEGACY generation 9; no human witness
  recorded`.
- `steward arm` beside a live runner, when the standing record is legacy or
  machine-minted, returns "already armed (runner pid P); the enrollment is
  LEGACY/machine-minted — run steward restart at the terminal to witness
  it".

### What health shows

`installedGeneration` (`health.go:1153-1166`) becomes
`installedEnrollment(repoRoot) (InstallIdentity, error)` and the
steward-runner role's reason string (`health.go:582-583`), in every status
where the record was readable, ends with one provenance clause rendered by
`EnrollmentProvenance(id InstallIdentity) string`:

- human-minted, permanent: `enrollment generation 9 human-witnessed (engine
  <stamp>)`
- machine-minted above a witnessed base: `enrollment generation 9
  machine-minted (rebuild, engine <stamp>, landed <short commit> on <ref>)
  above human-witnessed generation 5 of 2026-09-02T11:21:41Z`
- machine-minted above a legacy base, and legacy itself: the two renderings
  in the Legacy section
- with a temporary word, any case: append `; TEMPORARY under a recorded
  remote human word, review by 2026-09-06`

The aggregate does not change: a machine-minted or legacy generation is
information, not a fault. An overdue `ReviewBy` is rendered, not judged. The
`HEALTH` line the hook previews carries the clause because it carries every
role's reason, and that line is in the check-in tail of every Stop emission
(Decision 3), which is what makes the persisted fact visible in later turns.

## Decision 3 — visibility, ordering, durability, and the launch-site seam (folds CX-2, CX-3, ERAR-R1-05, -08; ERAR-R2-01, -05, -06)

### The order inside `ordinary` (ERAR-R2-01)

`ordinary` (`up.go:412-500`) keeps today's order: host-preflight →
**accepted-engine** (the enrollment open; C13 re-arms here) →
session-identity (`resolveSessionIdentity`) → `PrepareForExecution` →
session-announcement → checkout-lease → supervision → steward-runner.
Revision 2's move of session identity ahead of the enrollment open is
withdrawn: session resolution returns to where `up` has it, so the component
line order every fixture parses today is unchanged, and
`TestOrdinaryUpRefusesDriftWithoutMintingANewGeneration` (`up_test.go:121-147`)
keeps its shape. A drift refusal still happens "before announcements, leases,
or supervision are touched" (`docs/orchestration.md:239`).

Consequences, stated plainly: a caller with no resolvable session identity, a
stray cron included, that invokes ordinary `up` on a landed rebuild re-arms
it and then fails at `component=session-identity outcome=failed`; the
aggregate line carries `re-armed=` and the exit code is 1. That re-arm is
harmless by the rule: the enrolled bytes are landed bytes and the record
names the commit. The same caller on a `dev`, unlanded, or unresolvable build
refuses to the human before anything is touched. An advisor session's `up`
re-arms too; the authority for that is provenance alone.

`recovery` (`up.go:502-547`) is unchanged in order and never re-arms:
`openInvokingEnrollment` takes `allowReArm bool`; `ordinary` passes true
and `recovery` false. The branch's shared use (branch `up.go:534-546`) is
withdrawn. **Unattended recovery never re-arms**, for two reasons each
sufficient: the scheduler entry is `--recover-only --if-down`, whose contract
is "start only missing repository rings" (`cmd/metasystem/up.go:83`,
`docs/orchestration.md:239`), and a re-arm STOPS a live runner to replace it;
and the wedge this goal fixes is a seat unable to arm, which the seat's own
next ordinary `up` now resolves. A rebuilt engine at the enrolled path is
`ENROLLMENT_DRIFT` in recovery, with the remedy "run metasystem up from a
session, which re-arms a rebuilt engine at its enrolled path; or steward arm
at an agent-free terminal". The hook's no-identity Stop path
(`supervision-hook.sh:419-420`) runs `--recover-only --if-down` and therefore
also does not re-arm. CX-3's first half cannot occur and its second half is
disposed of by narrowing: the `--if-down` contract stands as written; the
aggregate rule at `up.go:542-545` is unchanged.

### Every Result carries the fact (ERAR-R1-05)

`Result` gains `ReArmed string`, rendered by `Lines()` (`up.go:76-106`) on
the aggregate line between `authority` and `holder` when non-empty:
`re-armed="generation=9 previous=8 engine=<stamp> landed=<short commit>"`.
It is set at ONE exit point: `ordinary` becomes a wrapper around
`ordinaryBody(options) (Result, string)`; the body threads the re-armed
string through its locals and the wrapper writes it onto whatever `Result`
the body returned, the `failure` helper's Results (`up.go:108-113`) and
`enrollmentDrift`'s included. A successful re-arm followed by any later
component failure, session-identity included, still ends with an aggregate
line carrying `re-armed=`. The component line for the re-arm is
`component=accepted-engine outcome=re-armed detail="the enrolled engine was
rebuilt; armed (runner pid P) (generation=9 previous=8 engine=<stamp>
landed=<short commit> ref=<ref> path=...)"`, with `; pending notification
not written: <NoticeErr>` appended when step 6b failed. An
`already-current` outcome renders as `component=accepted-engine
outcome=verified` with the detail "brought current by a concurrent arm" and
sets no `re-armed` key.

### Where the fact is persisted, by owner and order (ERAR-R2-05)

Revision 2 claimed three records "all written before `up` can fail or be
killed" while assigning them to two owners. That claim is withdrawn and
replaced by this contract:

| Record | Owner | When | On write failure |
| --- | --- | --- | --- |
| 1. The identity record (`MintedBy: machine-rebuild`, `EngineBuild`, `LandedCommit`, `LandingRef`, the carried word) | `arm`, under the lock, step 6a | First write of the mint, atomic rename (`MintIdentity`, `identity.go:52-62`) | The arm fails at `StageBeforeMint` with the error; nothing was minted. |
| 2. The pending steward notification (`QueueNotification`, `intervene.go:299-314`; nonce `engine-rearm-generation-<N>`; message "steward: re-armed the rebuilt engine: generation N previous M engine <stamp> landed <short commit> on <ref>") | `arm`, under the lock, step 6b | Immediately after 1 | Reported in `NoticeErr`, rendered on the accepted-engine line and in the arming-log line; never fatal, never rolls back 1. The runner delivers it on its first tick (`DeliverPending`, `notify.go:64-99`); until then `steward pending` names it. |
| 3. The arming-log line (`appendArmingLog`, `up.go:315-324`): `engine-re-armed generation=N previous=M engine=<stamp> landed=<short commit> ref=<ref>` (plus `notice-error="..."` when 2 failed) | `up`, after `ReArmRebuiltEngine` returns with `Stage >= StageMinted`, before any other component runs | After 1 and 2 | `appendArmingLog` stays best-effort and keeps discarding errors: it is a human-readable trail, not a record of truth, and every fact it carries is also in 1. Making it fatal would fail an armed repository over a log line. |

**The invariant is narrowed to what the mechanism supports:** the identity
record is the one record of truth, and it is complete before any other
effect of the mint exists. Records 2 and 3 are visibility aids that are
recovered from 1: health's provenance clause (Decision 2) is rendered from 1
on every later `HEALTH` line, so a reader who missed 2 and 3 still sees the
fact at the next Stop. The reject condition is restated accordingly.

What the deadline parent can interrupt, from reading
`supervision-hook.sh:32-222`: the four-second parent runs the hook body as a
child worker (56-58) and, on expiry, sends TERM then KILL to THAT WORKER only
(199-211: `kill -TERM "$deadline_worker"`, then `kill -KILL`), never to the
`up` process the worker spawned inside its command substitution (415-421).
`up` runs on to completion; it prints its lines only after `Run` returns
(`cmd/metasystem/up.go:64, 151`), so the only thing a worker kill can cost
is the rendering, never a write. The kill windows that remain are a machine
or process death of `up` itself:

| Killed... | On disk | What a reader sees next |
| --- | --- | --- |
| before step 5 | nothing changed | next `up` finds C13 again and re-arms |
| inside step 5 (`StageStopping`) | restart marker written, old runner signalled or dead, record unchanged | next `up` finds C13, finds no live runner (or stops it again), mints and launches |
| after 6a, before 6b | record 1 complete | health's clause on the next `HEALTH` line; no pending notification; no arming-log line |
| after 6b, before 3 | records 1 and 2 | health's clause, and `steward pending` names the notification (the hook surfaces "Steward incidents pending", `supervision-hook.sh:662-663`) |
| after 3, before launch | records 1, 2, 3; no runner | next `up` finds the enrollment current and repairs the runner through `EnsureRunner` → `repairPinnedRunner` (`runner.go:220-266, 306-367`, mints nothing) |

### The hook (CX-2, ERAR-R1-05)

`supervision-hook.sh` reads `up`'s output only on failure (Stop: 422-425;
session start: 690-698 exits 0 on success). The additions, none blocking:

- Stop (410-452): a new variable `up_notice=` beside `up_failure`, computed
  from `$up_output` REGARDLESS of `up_rc`: if any line contains
  ` re-armed=`, `up_notice="Metasystem re-armed the rebuilt engine: <that
  line>"`. Where `checkin_tail=$health_line` is assembled (449),
  `up_notice` is prepended when set. `checkin_tail` is the system message of
  EVERY Stop emission: the allow and block verdict paths, the advisor
  branch, the degraded branch, and `emit_failed_stop` (through
  `external_stop_json`). The block-reason rule (the display is the reason
  byte-verbatim) is untouched because the notice rides the system message.
- Session start (690-698): on `up` exit 0, if the last line of `$output`
  contains ` re-armed=`, emit `surface_json "Metasystem re-armed the rebuilt
  engine: <last line>"` before `exit 0`. On non-zero exit line 698 already
  tails the aggregate line, which carries `re-armed=` when a re-arm
  preceded the failure.
- The deadline parent never reads `up`'s output; when it kills the worker
  after a mint, record 1 is on disk (the table above) and the next Stop's
  health line shows the fact. That is the answer to "never silent end to
  end": the fact is durable in the record of truth, and every later turn
  carries it.

The match is on the aggregate key, not on prose.

### The refusal remedy

The remedy at `up.go:392` is rewritten for every REFUSE row: "this engine is
not the enrolled one at its enrolled path, or its build is not eligible for
automatic re-arm (a landed commit on the installation's landing ref
metasystem.steward.landing-ref); from an agent-free terminal run metasystem
steward restart --repo <root> (steward arm when no runner is live), or relay
the human's recorded word with --temporary-human-word and --review-by".
`<root>` is `options.Root`. C21 substitutes its own remedy (the `git config`
command). The fleet-join bootstrap design proposed the terminal half of this
rewrite (`plans/fleet-join-bootstrap-design.md:417`); this design lands it
with `restart` named first.

### Typed drift at the launch sites, and the seam that proves it (ERAR-R1-08, ERAR-R2-06)

`ensureSupervision` (`up.go:348-389`) and the steward launch (`up.go:480-483`)
gain one rule: when `errors.Is(err, steward.ErrEnrollmentDrift)`, return
`enrollmentDrift(components, err)` with the detail prefixed by the launch
site ("supervision-owner launch: " or "steward-runner launch: "), instead of
a component failure.

`up` has no injectable command at either site today: line 354 assigns the
concrete method `enrolled.Command` and line 480 calls the concrete function
`steward.EnsureRunner`, whose launch calls `EnrolledBinary.Command` directly
(`runner.go:455`). The seam is two package-level function variables in
`internal/up`, exactly the shape `sessionParentPid` already has (`up.go:127`):

```go
// Test seams on the sessionParentPid precedent: production never reassigns
// them; the C16 routing test swaps each for a launcher that fails with a
// typed drift error.
var stewardEnsureRunner = steward.EnsureRunner
var enrolledCommand = func(enrolled *steward.EnrolledBinary) func(args ...string) (*exec.Cmd, error) {
    return enrolled.Command
}
```

Line 354 becomes `armingOptions.Command = enrolledCommand(enrolled)` and line
480 calls `stewardEnsureRunner(...)`. Proof is in two halves, because the
seam proves `up`'s mapping and not the steward chain:

- **`up` half (F10):** the supervision site is driven through the REAL
  `supervise.EnsureArmed` with `enrolledCommand` swapped for a launcher that
  returns an `ErrEnrollmentDrift`-wrapped error; the error survives because
  `launchOwner` returns `options.Command`'s error unwrapped
  (`arming.go:604-608`) and `EnsureArmed` returns `launchOwner`'s error
  unwrapped on the establish path (`arming.go:718-721`), which is the path a
  fresh scratch root takes. The steward site is driven with
  `stewardEnsureRunner` swapped the same way.
- **steward half (F10b):** real command-time drift induced against
  production code with no seam: an `EnrolledBinary` opened on a fixture
  executable, `PrepareForExecution` run, then another file renamed over the
  pin path so `Command`'s `SameFile` test fails (`identity.go:232-235`);
  `launchRunner` returns that error unwrapped (`runner.go:455-458`),
  `repairPinnedRunner` returns it unwrapped (`352-355`), and `EnsureRunner`
  returns it unwrapped (`247-250`); the test asserts `errors.Is` through
  `repairPinnedRunner` in a scratch repository.

Together the two halves prove the mapping at both sites; the one residual is
the takeover and replacement paths of `EnsureArmed` (`arming.go:727-800`),
which wrap other errors but never reach `launchOwner` except by `continue`
back to the establish path, read rather than executed.

## Decision 4 — failure shape: mint-before-launch, visibly and by stage (folds ERAR-R1-06, ERAR-R2-04)

`arm` mints before it launches and a failed later step leaves the bumped
generation in place. The automatic path inherits this for the structural and
convergence reasons revision 2 gave (the runner proves itself against the
record; a mint-then-rollback would re-mint on every attempt). Revision 3 adds
the destructive pre-mint stage ERAR-R2-04 named.

What `ordinary` renders at each stage `ReArmRebuiltEngine` can return with an
error:

| Stage returned | What happened | `ordinary` renders |
| --- | --- | --- |
| `StageBeforeMint` | Refused (C1–C12, C14, C19, C20, C21) or nothing to do | The refusal row's `ENROLLMENT_DRIFT` rendering, or `verified` for `already-current`; no `re-armed` key; no runner stopped. |
| `StageStopping` | Eligible; `stopRunnerForReplacement` wrote the restart marker and signalled runner pid P, then returned an error (`runner.go:483-485, 496-499, 508`); nothing minted | `component=accepted-engine outcome=failed detail="re-arm eligible (engine <stamp> landed <short commit> on <ref>); stopping runner pid P for replacement failed: <err>; the enrollment still names generation N" remedy="prove runner pid P is gone (artifacts/agents/steward/runner.json), then rerun metasystem up"`, aggregate `up outcome=failed component=accepted-engine`, no `re-armed` key. Never rendered as a refusal or a no-op. |
| `StageMinted` | Record written; `OpenEnrolledBinary` failed (the path changed again, or vanished, between the locked digest and the reopen) | `component=accepted-engine outcome=re-armed` (the mint happened; say so) then `component=steward-runner outcome=failed detail="after re-arm: <err>" remedy="rerun metasystem up; a further rebuild re-arms again"`, aggregate `up outcome=failed component=steward-runner re-armed=...`. |
| `StageReopened` | Reopened; `PrepareForExecution` failed (disk, or C16 snapshot drift) | Same shape; C16 drift renders `ENROLLMENT_DRIFT` per Decision 3 with `re-armed=` still on the aggregate line. Remedy: "inspect artifacts/agents/steward/engine-pins, then rerun metasystem up". |
| `StageSnapshotted` | Snapshot ready; `launchRunner` failed (log unwritable, runner died, no confirmation in ten seconds) | Same shape; remedy "inspect artifacts/agents/steward/runner.log, then rerun metasystem up". |

In every post-mint stage the old runner was already stopped at step 5, the
pending notification was written (or its failure recorded) at 6b, and the
failed generation carries `MintedBy: machine-rebuild`. The branch's rendering
of a failed launch as `ENROLLMENT_DRIFT` (branch `up.go:410-413`) is
withdrawn.

## Decision 5 — proof (folds ERAR-R1-07, ERAR-R2-02, -03, -04, -06)

Three classes, by what can actually run. The ordinary gate runs the Go tests
(`go-gate.sh:528, 544`) BEFORE it builds and installs `bin/metasystem`
(`555`), so every proof that needs two real engine builds is shell-driven and
runs after the build. The shell bed is `scripts/agents/supervision-fixtures.sh`,
which creates and exports its own registry home (`116-118`), refuses to run
without the built engine (`120-121`), mints a fixture enrollment
(`enroll_fixture_engine`, `577-587`), and invokes `up` below an announced live
process (`live_arm_driver`, `591-605`). Its scenarios run as fixture-bed
children with the bed's cleanup, which owns every runner and owner the
scenario starts.

Every scratch installation repository in these proofs is a git repository
whose landing ref is `refs/heads/trunk`, configured with `git config
metasystem.steward.landing-ref refs/heads/trunk` (ERAR-R2-03's non-main
fixture is the default shape, not a variant); it has at least one commit on
`trunk` and one on a branch `side` that `trunk` does not contain.

Engine builds for the shell class: engine A is the gate's `bin/metasystem`
copied into the scratch repository. Engine B is `scripts/agents/go-build.sh
--out <scratch>/engine-b` with `METASYSTEM_BUILD_STAMP` set to the short hash
of the `trunk` tip; engine D ("dev") is stamped `dev`; engine X ("unlanded")
is stamped with the `side` commit; engine W ("witness") is stamped
`witness-<first 12 hex of the scratch repository's ENGINE digest at the trunk
tip>`, where the digest is computed by `bin/metasystem behavior-surface
digest --root <archive extract of trunk> --projection ENGINE --endpoint
fixture` over a scratch tree that commits a small `cmd/` file and `go.mod` so
the projection is non-empty. Each differs from A only in the linked stamp,
which changes the digest. Installation onto the enrolled path is by
stage-and-rename, never by copying over the live file.

Failure injection for the launch stage needs no production seam:
`launchRunner` opens `artifacts/agents/steward/runner.log` for append
(`runner.go:450-453`) before it starts anything, so a scenario that makes that
path a directory gets a real `StageSnapshotted` launch error. Failure
injection for the stop stage needs none either: `stopRunnerForReplacement`
returns an error when `kill(SIGTERM)` fails with anything but `ESRCH`
(`runner.go:483-485`), and a `runner.json` naming pid 1 with its true start
second makes `liveRunner` report a live runner (`runner.go:572-577`) that the
test user cannot signal (`EPERM`), after the restart marker was written.

| Id | Session id | Class and where | Proves |
| --- | --- | --- | --- |
| F1 real rebuild re-arms on a non-main ref | `rearm-rebuild` | shell, `supervision-fixtures.sh` scenario | Enroll A with a `human-terminal` record; rename B onto the path; `up` via `live_arm_driver` returns exit 0; the accepted-engine line is `outcome=re-armed`; the aggregate carries `re-armed="generation=2 previous=1 engine=<B's stamp> landed=<trunk tip short>"`; the record has generation 2, `MintedBy` machine-rebuild, `HumanWitnessedGeneration` 1, `EngineBuild` = B's stamp, `LandedCommit` = the full trunk tip, `LandingRef` = `refs/heads/trunk`; `engine-pins/generation-2-<B's digest>` exists; runner.json names a live pid; health's steward-runner reason carries the machine-minted clause naming the commit and ref; `arming.log` has the `engine-re-armed` line; `steward pending` names `engine-rearm-generation-2` until the runner delivers it. Second run from a legacy base (today's `enroll_fixture_engine` record): same, with `HumanWitnessedGeneration` 0 and the LEGACY clause. |
| F2 stranger refuses untouched; stray-cron shapes | `rearm-stranger` | Go, `internal/up` (extends `TestOrdinaryUpRefusesDriftWithoutMintingANewGeneration`) | Enrollment at path P generation 7; bytes at P changed AND `Binary` is a different path Q: `ENROLLMENT_DRIFT`, record still generation 7, no `mains`/`supervision` directories, remedy names `steward restart` and the temporary-word relay. Second firing: bytes unchanged, `Binary` Q: same (C15). Third firing, the stray-cron shape ERAR-R2-01 asked for: `Binary` = P, landed rebuild at P, session identity supplied as the caller's OWN pid and start time (the explicit-pair route) → `accepted-engine outcome=re-armed`, then whatever session-identity decides; and with NO identity → `re-armed` then `component=session-identity outcome=failed`, aggregate carrying `re-armed=`, exit 1. Fourth firing: same two callers on an `X`-shaped (unlanded) rebuild → `ENROLLMENT_DRIFT` C20, record untouched. |
| F3 two concurrent re-arms mint once | `rearm-concurrent` | Go, `internal/steward`, `beforeLock`-style seam like `runner_test.go:443-446`; the runner is a stamped fake built from `internal/steward/testdata/fakerunner` | Two goroutines call `ReArmRebuiltEngine`; the seam holds the first inside the lock after step 4 until the second blocks on the flock; outcomes are one `re-armed` and one `already-current`; generation advanced by exactly one; the first's runner pid is still live after the second returns. |
| F4 re-arm racing ArmTemporary keeps the human's word | `rearm-vs-human-word` | Go, `internal/steward`, same seam and fake | Held before the flock while `ArmTemporary(W, R)` completes on the rebuilt bytes: the re-arm returns `already-current` and the record carries W and R with `MintedBy` human-word. Reverse order: the human's arm mints the next generation with W and R and `HumanWitnessedGeneration` equal to its own generation. Neither order writes empty word fields over W. |
| F5 hook surfaces the re-arm | `rearm-hook-surfaces` | shell, `supervision-hook-fixtures.sh` (stub-engine pattern at 215-235) | A stub `up` printing an aggregate line ending in `re-armed="generation=9 previous=8 engine=abc1234 landed=def5678"` and exiting 0: session start emits a `systemMessage` containing "re-armed the rebuilt engine"; Stop allow and block verdicts carry it in the system message with the block reason byte-identical to the display; `emit_failed_stop` carries it. A stub exiting 1 with the key surfaces both the failure and the notice. A stub exiting 0 without the key emits nothing new. |
| F6 failed launch after a machine mint | `rearm-launch-fails` | shell, `supervision-fixtures.sh` scenario | As F1 plus `runner.log` made a directory before `up`: `accepted-engine outcome=re-armed`, `steward-runner outcome=failed` with the runner.log remedy, aggregate `outcome=failed` with the `re-armed` key; record generation 2 `MintedBy` machine-rebuild; `arming.log` and the pending notification exist. Directory removed, a following `up` reports `steward-runner outcome=started` at generation 2 (repair, no mint). |
| F7 recovery never re-arms | `rearm-recovery-refuses` | Go, `internal/up` | Rebuilt bytes at the enrolled path; `recovery` with `RecoverOnly` and `IfDown` returns `ENROLLMENT_DRIFT`, record unchanged, remedy naming a session's `up`. |
| F8 ineligible builds refuse, eligible witness re-arms | `rearm-provenance` | shell, `supervision-fixtures.sh` scenario | Enroll A and run one `up` so a runner is live. Rename D onto the path: `ENROLLMENT_DRIFT` naming stamp `dev` and R-37-m3, remedy names `steward restart`; record still generation 1 with A's digest; A's runner still live with the same pid. Rename X: `ENROLLMENT_DRIFT` "not landed on refs/heads/trunk" (C20), same invariants. Unset `metasystem.steward.landing-ref`, rename B: `ENROLLMENT_DRIFT` "owns no landing ref" with the `git config` remedy (C21), same invariants; set it to `main` (unqualified): C21 again. Restore the key, rename W: `outcome=re-armed`, `EngineBuild` = `witness-<12 hex>`, `LandedCommit` = the full trunk tip, `LandingRef` = `refs/heads/trunk` (ERAR-R2-02). |
| F9 arm beside a live runner reports the clearing verb | `rearm-arm-names-restart` | Go, `internal/steward` with the fake runner | Legacy record and live runner: `Arm` returns "already armed" naming `steward restart` and LEGACY, mints nothing. `Restart` mints N+1 with `MintedBy` human-terminal and the witnessed pair equal to N+1. |
| F10 typed drift at the launch sites, `up` half | `rearm-c16-routes` | Go, `internal/up`, the two package-level seams | `enrolledCommand` swapped for a launcher returning an `ErrEnrollmentDrift`-wrapped error, driven through the real `supervise.EnsureArmed` on a fresh scratch root: `component=accepted-engine outcome=ENROLLMENT_DRIFT` with the "supervision-owner launch: " prefix and aggregate `ENROLLMENT_DRIFT`. `stewardEnsureRunner` swapped the same way: the "steward-runner launch: " prefix. A plain error at each site still renders today's component failure. |
| F10b typed drift, steward half | `rearm-c16-steward-chain` | Go, `internal/steward` | Real command-time drift (another file renamed over the pin path after `PrepareForExecution`): `repairPinnedRunner` returns an error for which `errors.Is(err, ErrEnrollmentDrift)` holds; no runner was started. |
| F11 witness resolver equals the gate's digest | `rearm-witness-resolves` | Go, `internal/steward` | On a scratch repository with a nested metasystem prefix and one without: `resolveWitnessStamp` of the first 12 hex of `Policy.Digest` over a full archive extract of the trunk tip returns that tip; the same digest of the `side` commit returns not-found against `refs/heads/trunk`; a digest matching only the 65th-newest commit returns not-found; the partial-pathspec extract's digest equals the full extract's digest. |
| F12 stop failure before the mint | `rearm-stop-fails-pre-mint` | Go, `internal/steward` | Rebuilt enrollment; `runner.json` names pid 1 with its true start second; `ReArmRebuiltEngine` returns an error with `Stage == StageStopping` and `StoppedRunnerPid == 1`; the `stop` marker exists with `restart`; the record is unchanged (generation and digest); the `up` rendering test asserts the `StageStopping` row of Decision 4 verbatim in shape. |

F1, F6, and F8 run only outside a delegate sandbox (KI-15); the orchestrator
runs them. F3, F4, F9, and F12 build one small stamped Go program per test run
(F12 needs none) and are the only Go tests here that launch processes; each
sets `METASYSTEM_SUPERVISION_REGISTRY_HOME` to its own `t.TempDir()`, uses a
scratch git repository with `trunk` and `side` as both the installation root
and the stamp source, and cleans up with `Disarm` plus a kill of the pid
recorded in runner.json.

Claims with no runnable proof, recorded as residual risk rather than claimed:
(i) a real re-arm surfaced by a real Stop hook end to end, because the Stop
hook needs a registered runtime ancestor the fixtures cannot supply (F5
proves the hook plumbing against a stub `up`, F1 proves `up` itself); (ii)
the deadline-parent path after a mint, because the four-second kill is
timing the fixtures cannot make deterministic (the reading that it kills the
worker and not `up` is what covers it, and F1 proves the records exist);
(iii) `EnsureArmed`'s takeover and replacement paths reaching `launchOwner`,
read rather than executed.

## Migration and build prerequisites (ERAR-R1-04, ERAR-R2-03)

1. Marking: absent `MintedBy` marks a legacy generation; health renders it
   LEGACY from the first run of the new engine. The m1 record at generation 9
   is legacy and dev-stamped; the m2 record of 2026-08-30 is legacy.
2. Landing ref on the live fleet: each machine's installation needs
   `git config metasystem.steward.landing-ref refs/heads/main` once (main is
   the branch the fleet lands on: the goal ledger's origin and every landing
   in this checkout's history). Until it is set, every automatic re-arm on
   that machine refuses with C21's remedy, which names the command. Going
   forward, `scripts/adopt.sh` seeds the key on the target beside
   `metasystem.goal.machine` (`adopt.sh:384-389`) with the target's
   checked-out branch (`git -C <target> symbolic-ref --quiet HEAD`, refusing
   adoption of a detached target), and the fleet-join bootstrap
   (`plans/fleet-join-bootstrap-design.md`, step 3) seeds `refs/heads/main`
   unless its `--landing-ref` argument says otherwise. The engine itself
   never defaults it.
3. Clearing act: `steward restart --repo <root>` at an agent-free terminal
   (a runner is live on every fleet machine, so `steward arm` would return
   already-armed and clear nothing), or `steward arm --temporary-human-word
   ... --review-by ...` while R-29's departure path is lawful.
4. Prerequisite for the build: the implementation lands the legacy rendering,
   the `restart`-naming remedies, C19, C20, and C21 in the same change, so
   that no engine built by this design can read a legacy record as
   human-witnessed, and so that the first post-landing rebuild on a machine
   whose enrolled engine is dev-stamped is refused rather than enrolled. The
   landing note records, per machine, the legacy generation observed, whether
   the landing ref was set, and that the human clearing act is pending.
   Because the m1 engine is dev-stamped today, the first rebuild after landing
   on m1 must go through `go-build.sh` or the witness gate (both stamp); a
   direct `go build` is refused by C19, and that refusal is correct.

## Consistency pass

- Doctrine to sweep the same hour this lands (the rulings-sweep rule):
  `docs/orchestration.md:239` and the orchestration skill's copy say
  "Ordinary, advisor, and recovery-only `up` only consult that standing
  enrollment"; the sentence becomes "Recovery-only `up` only consults that
  standing enrollment; ordinary and advisor `up` re-arm the enrolled engine
  when it was rebuilt at its enrolled path from a commit landed on the
  installation's owned landing ref (`metasystem.steward.landing-ref`), and
  refuse every other drift before announcements, leases, or supervision are
  touched." `identity.go:3-8` gains one sentence naming the provenance
  reading. `cmd/metasystem/up.go:86`'s `--rearm` help stops being false.
  R-37-m3's software-home column points at this design; the provenance draft
  stays sequenced behind two-bars with its seam named here (Decision 2,
  machine `decide` step 4).
- Decision 1's rule and Decision 3's order agree: eligibility is decided from
  the bytes and the ref alone, so the enrollment open stays the first step
  after host-preflight and the last step before any write; session identity
  follows it and decides nothing about the mint.
- Decision 1's C13 and the drift-cause table agree: C13 is the one automatic
  cause, now by provenance; C18 is conditional on it; C19, C20, and C21 are
  its three refusals, in the order `decide` step 4 tests them (ref, shape,
  resolution and reachability).
- Decision 2's steps and Decision 4's stages agree: `StageBeforeMint` is every
  return before step 5's stop; `StageStopping` is a step-5 error; the three
  later stages are steps 7 through 9.
- Decision 2's record fields, Decision 3's records, and Decision 5's F1 and
  F8 agree on `EngineBuild`, `LandedCommit`, `LandingRef`, and the notification
  nonce; `MintedBySession` appears nowhere.
- Decision 3's durability table and the reject condition agree: the record of
  truth is one, and the other two are visibility aids whose absence is
  recoverable from it.
- Decision 3's seam and Decision 5's F10/F10b agree: the seam proves `up`'s
  mapping; the steward chain is proven by real drift.
- Untouched on purpose: `VerifyEnrolledBinary` and every classifier that
  authenticates a running steward stay pure; `EnrolledExecutionPath` keys the
  snapshot by generation and digest; the `--if-down` contract; the block-once
  refusal record; the temporary word's validation is not re-run on
  carry-forward; the human paths' acceptance of any stamp; the fixture
  driver's explicit-pair identity route, which now decides nothing about
  eligibility and so needs no hardening here.
- The branch's hook changes to `find-ancestor --repo "$harness_root"` are
  the hook-root goal's territory.

## Self-grade

Grounding: every load-bearing claim is a file-and-line read at c209abcb5
(`identity.go` whole, `runner.go` whole, `up.go` whole, `cmd/metasystem/up.go:78-151`,
`supervise/arming.go:45-75, 590-800`, `supervise/disk.go:139`,
`steward/intervene.go:299-314`, `behaviorsurface/policy.go:330-420` and
`policy.v2.json`'s `enginePaths`, `go-build.sh` whole, `go-gate.sh:106-165,
480-570`, `witness-gate.sh:1-60, 100-170`, `commit.sh:500-570`,
`land.sh:220-245`, `contract.go:1300-1325`, `supervision-hook.sh:32-222,
405-455`, `supervision-fixtures.sh:570-605`, `adopt.sh:378-392`,
`fleet-join-bootstrap-design.md:204-222`, `memory/rulings.md:64`,
`docs/orchestration.md:239`, the round-2 register whole, and the goal record).
Two facts are carried from revision 2's execution rather than re-executed:
`go version -m` on the m1 engine (no `-ldflags`, `vcs.modified=true`) and
the go1.26.5 scratch build proving `-ldflags` is recorded in build
information. No product bytes changed; the required gate chain's result is
reported in the return, not claimed here.

Residual risks, honestly: (a) the witness resolver's 64-candidate bound is a
fixed number; a seat that rebuilds from a tree more than 64 landings behind
the ref's tip is refused with the human remedy, loudly; (b) the resolver
digests with the invoking engine's compiled policy, so a policy-version
change between the stamping gate and the invoking engine yields a refusal,
never a silent enrollment; (c) the build-information read depends on Go
continuing to record `-ldflags`, verified on 1.26.5; (d) `refs/heads/<x>`
as the ref makes a wrapper commit landed locally but not yet pushed eligible;
an operator wanting the pushed reading points the key at the remote-tracking
ref; (e) an advisor session's `up` and a stray scheduler entry can both
re-arm a landed rebuild; the rule says that act is harmless because the bytes
are landed bytes and the record says so, and the critic should keep attacking
that claim; (f) the one-time `git config` on the four fleet machines is an
installation-configuration write; this design has the seat perform it under
R-37-m3's authority and names it in the landing note, and Wido may prefer to
type it himself at the terminal re-approval the goal already requires; (g)
`re-armed=` on the aggregate line changes the output fixtures parse; each
fixture asserting the exact line set is updated, not weakened; (h) the
resolver runs `git archive` and a digest walk under the arm lock, one
candidate in the ordinary case, up to 64 in the worst; (i) F12 signals pid 1
with SIGTERM from an unprivileged test, which the kernel refuses with EPERM;
a test run as root would deliver it, so the test skips when the effective uid
is 0. Grade: pass against everything read; the reject condition is the
falsifier.

**Reject condition — reject this design if any of the following is shown:**
a drift cause not in the C1–C21 table that reaches `up` or a pinned-engine
launch site; any RE-ARM on a caller whose canonical executable path is not
the enrolled path, or on bytes whose stamp is absent, `dev`, `unknown`,
outside the two grammars, unresolvable, or not reachable from the configured
landing ref, or with the landing ref absent, unqualified, or unresolvable, or
on any cause other than C13; any eligibility test that reads the caller's
session, pid, or ancestry; any `EngineBuild`, `LandedCommit`, or `LandingRef`
value computed from anything but the descriptor digested under the arm lock
and the ref read under that same lock; any field of the minted record
computed from a read taken outside the arm flock; any refusal or skip that
stops a live runner; a stop failure reported as a refusal or a no-op, or
without the runner pid it signalled; a second concurrent re-arm that stops
the first's runner or mints a second generation; any order of a re-arm and an
`ArmTemporary` in which the human's word or review date is replaced by empty
strings; a legacy or machine-minted record that health renders as
human-witnessed, or a human mint that leaves `MintedBy` at machine-rebuild or
the witnessed pair unseeded; a remedy that names `steward arm` as the clearing
act beside a live runner; a Result returned by `ordinary` after a mint without
`re-armed=` on its aggregate line; a Stop emission from the worker,
`emit_failed_stop` included, whose system message lacks the notice when the
turn's `up` output carried the key; a mint whose identity record is not the
first write, or a notification failure that aborts or rolls back a mint; any
re-arm reachable from `--recover-only`, or from the hook's no-identity Stop
path; any `--if-down` invocation that stops a live runner; an `arm` return
from which the stage reached cannot be read without re-reading the record; a
post-mint failure reported as `ENROLLMENT_DRIFT` except C16 drift, or with the
terminal as its remedy; a machine mint that rolls back; typed `Command` drift
at a launch site rendered as a component failure; a witness stamp resolved by
any computation other than the gate's own digest over the commit's tree; a
temporary word carried forward that the machine validated, extended, or
cleared; a hard-coded landing ref anywhere in the engine; or a fixture in
Decision 5 weakened from its stated assertion or moved out of the class that
can run it.
