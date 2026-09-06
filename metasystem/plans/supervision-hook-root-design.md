# Design: the supervision hook resolves the metasystem world, not the outer repository

Goal: supervision-hook-wrong-root (plans/goals/supervision-hook-wrong-root.md,
revision 6); since the 2026-09-06 split, the design's remaining scope is
goal hook-root-resolver-design (member B), member A having landed as
43c3d3c08. Author: implementer delegate under dispatch by
m0b+main-1788250419-3170380-8a1fb3. **Revision 3, 2026-09-02**: folds all five
findings of records/misc/hook-root-critique-r2.md by id (SHR-R2-INSTALL-01,
SHR-R2-WORKTREE-ENGINE-01, SHR-R2-ENGINE-SKEW-01, SHR-R2-WORKTREE-FALLBACK-01,
SHR-R2-CONSUMER-01); each fold is tagged inline, and each refines the
revision-2 installation-derivation mechanism rather than replacing it. Every
seam cited below was re-read in this worktree at commit 5aad591f; the
live-fleet observations were read on the m0b checkout this design was authored
beside, and the skew and worktree probes recorded in the self-grade were run
here on 2026-09-02.

**Revision 4, 2026-09-02**: folds both findings of
records/misc/hook-root-critique-r3.md by id. SHR-R3-ENGINE-INSTALLATION-PAIR-01:
the engine's world answer is now computed FOR the one installation every
shell-owned consumer of the turn names, so a `METASYSTEM_BIN` override
replaces the engine and never the installation (the pairing rule, Decision
1). SHR-R3-GIT-STEERING-01: the worktree mapper runs every git call with the
compiled authority's git-steering list scrubbed, so inherited `GIT_DIR`-style
variables cannot classify an engine-less delegate worktree as an ordinary
checkout (the exact sanitation, Decision 1). Each fold is tagged inline; the
seams they cite were re-read in this worktree at commit 47e59bcd, and the
steering probe in the self-grade was re-run here. Line numbers from
revision 3 that are not re-cited by this revision still refer to commit
5aad591f.

**Revision 5, 2026-09-05**: folds all four findings of
records/misc/hook-root-critique-r4.md by id. SHR-R4-DEADLINE-PARENT-01: the
hook's Stop-deadline parent (a second root and engine owner the design did
not govern) now resolves the same script-derived, scrubbed, mapped
installation as its worker, selects its canonical engine there, and derives
the refusal-record root through the state-root verb, never payload cwd
(Decisions 1, 3, 4). SHR-R4-FAIL-CLOSED-REGRESSION-01: a missing engine and an
old engine lacking the verb both BLOCK a Stop, as the shipped hook already
does for a missing engine; the replacement block, the failure map, and the
case table say so (Decision 2). SHR-R4-UP-GIT-STEERING-01: `up`'s
census-scope query runs under the compiled authority's scrub, and the
"never selects the state world, out of scope" claim is withdrawn (Decision
4). SHR-R4-COPIED-HOOK-OVERRIDE-01: the candidate must carry its own engine
at `<candidate>/bin/metasystem` for the world to be governed, with or
without a `METASYSTEM_BIN` override; the override replaces which engine runs
and never waives that evidence (Decision 1, fixture case 6). Each fold is
tagged inline. Hook line numbers this revision introduces are at commit
12ed490c3 (marked "HEAD"), where the Stop-deadline parent occupies
`supervision-hook.sh:32-222` and the worker body starts at line 224; the
5aad591f line numbers of earlier revisions map onto that worker body
(engine resolution 226-234, payload cwd and toplevel 258-274).
**Turn-verdict-hardening slice 1b is unsafe until this revision lands**: it
edits worker verdict responses whose parent, as shipped, can discard,
replace, or record against another world (the critic's exact claim, bound
here).

**Revision 6, 2026-09-06**: written by implementer job hrd-fold6-20260906
under goal hook-root-resolver-design (member B of the 2026-09-06 split of
supervision-hook-wrong-root; member A, hook-root-installation-fix, landed
as 43c3d3c08). It folds the three findings of
records/misc/hook-root-critique-r5.md by id.
SHR-R5-DEADLINE-OVERRIDE-ENGINE-SPLIT-01: the deadline parent runs ONE
engine for everything it asks an engine — the session parse, the
state-root derivation, the validation of the worker's JSON, and the refusal
record — and that engine is the override when `METASYSTEM_BIN` is set; the
installation's own engine is provenance evidence only, in the parent
exactly as in the worker; the state-root query moves into the resolver
subshell the deadline already kills, so a slow engine cannot eat the
parent's reserve (Decision 1, parent subsection; Decision 2 failure map;
cases 11 and 13 re-pinned with an old canonical engine and a compatible
override). SHR-R5-SKEW-FIXTURE-VALIDATOR-01: the old-engine stub of case 7
answers every verb the parent and worker use except `path state-root`,
which it refuses the way the real pre-verb dispatcher does, so the parent
validates the skew literal through its structured path and the fixture
observes the literal itself; the second firing revision 5's self-grade
proposed for the no-validator fallback is withdrawn as unreachable, and the
fallback's skew acceptance goes with it (Decision 2, case 7).
SHR-R5-TEMPLATE-MARKER-SECOND-AUTHORITY-01: the compiled authority is the
only state-root authority; the worker's template-marker block and the
marker test member A added to the parent's trail helper today are deleted,
`state_root` disappears as a name, every consumer names `$repo` — the
engine's answer for the installation — and the `missing-template` fixture
is repurposed to assert the missing-engine cause and the absence of writes
(Decision 1, new subsection; Decision 3; Decision 4). It also folds two
carry-overs member A's code critique recorded on this goal: hook-freshness
stays dead on nested checkouts after member A because the steward component
record is written under the Git toplevel while health reads it under the
steward's own root — that site is named in Decision 3's consumer sweep with
the fixture that proves hook-freshness alive on a nested checkout, which is
this goal's DONE (case 14); and the operator-layout scenario's "no
artifacts beneath the vendored installation" check is repeated after its
hook Stop. One fact changed since revision 5: the engine re-arm landed
today (ea8c3ead7), so the hook keys a re-arm notice from the `up` aggregate
line on the worker's Stop path and on the start path; the consumer table
and the blast rows below read the file as it stands. Hook line numbers this
revision introduces are at commit 3c753c5ef (marked "today"), where the
Stop-deadline parent occupies `supervision-hook.sh:32-240` and the worker
body starts at line 242; revision 5's "HEAD" numbers (commit 12ed490c3)
shift by member A's and the re-arm's insertions and are re-cited here
wherever revision 6 binds them. A "Where each round-5 finding lands" table
closes Decision 4.

## The defect, restated against the code

`scripts/agents/supervision-hook.sh:65` resolves the hook's world as
`git -C "$cwd" rev-parse --show-toplevel`. On the fleet's layout the metasystem
checkout is a subdirectory of a wrapper repository (observed on m0b:
`/home/wido.guest/m0b/agentic-tools/metasystem` inside the `agentic-tools`
toplevel), so `$repo` becomes the wrapper root — a directory with no goal
ledger, no enrolled steward, no armed supervision. The flag-driven consumers
of `$repo` (`steward`, `health`, `lease`, `proc`, `supervise`,
`report turn-verdict`, and the hook's own evidence trail) take that value
as-is and operate on a bootstrap world.

One engine verb is the exception: `up` re-derives its state world from
`--metasystem-root` through `stateroot.RootForInstallation` and overwrites its
root option with the result (`cmd/metasystem/up.go:104-113,139-144`); its
`--repo` flag only becomes the census scope via that path's git toplevel
(`up.go:42-49,109,130`). The shipped authority is
`internal/stateroot/stateroot.go:100-108`: a template-mode installation
(directory named `metasystem` whose parent carries
`development/metasystem-design.md`, `stateroot.go:157-163`) owns its state
itself; any other installation resolves state to its containing git toplevel.
On m0b that marker file is git-tracked at the wrapper toplevel (verified:
`git ls-files development/metasystem-design.md` lists it, in the primary and
in this worktree), so `up` was already arming the correct world — which is
exactly the live split observed on 2026-09-02:

- the armed supervision state sits under `metasystem/artifacts/agents/
  supervision/` (`last-census.json`, `owner.ndjson`, `lock.d/`,
  `reaper.heartbeat.json`) where `up` and the steward enrollment write;
- the wrapper root's `artifacts/agents/` contains exactly the hook's
  misdirected flag-driven writes: `supervision/hooks.log` (only that file),
  stray `steward/`, `goal.lock`, and `turn-verdict-state.json`.

So the hook's turn evidence never lands where hook-freshness is computed
(dead since enrollment on m2, m3, m0b), and on 2026-09-01 `report
turn-verdict --root <wrapper-root>` read a world with no `plans/goals/` and
could not refuse an idle turn-end while claimable work existed (goal record,
Next step, R-44-m0b). The defect, precisely: the hook computed the
adopted-mode answer (outer toplevel) for a template-mode installation. The
fix is to ask the one shipped authority that already knows the difference —
and, this revision adds, to ask it in the one form that cannot be fooled by
a pathname: the engine answering for the installation it is physically part
of.

## Decision 1 — root resolution

### Candidate versus evidence (folds SHR-R2-INSTALL-01)

**The invocation pathname is a candidate, never evidence.** The round-2
critique showed why the revision-2 verb was still pathname-trusting: the
shell canonicalizes only the script's directory (`supervision-hook.sh:23`
takes `dirname` before any resolution, so a terminal symbolic link to the
hook file is never resolved), and a verb that accepts any `--installation`
directory carrying `scripts/agents` would bless an in-repository hook copy
as a world. The Go side already holds the stronger provenance rule: the
executable owner resolves symbolic links over the complete executable path
and shape-checks its grandparent (`stateroot.go:137-155`,
`installationRoot()`: `os.Executable`, then `filepath.EvalSymlinks`, then
the `metasystem.conf`-or-`scripts/agents` shape gate).

So the contract splits into two layers, and only the second is authority:

- **Candidate**: `$harness_root` — the physical (`pwd -P`) grandparent of
  the running script (`supervision-hook.sh:23-24`), mapped to its primary
  counterpart when it is a linked worktree (below). The candidate's only
  job is to LOCATE an engine. No governed decision rides it.
- **Validation**: the engine found at the candidate answers for that
  candidate. The verb is **`metasystem path state-root <installation>`**,
  registered in the existing `path` family beside `owner`
  (`cmd/metasystem/main.go:248-252`, `path_verbs.go`). It takes exactly
  one positional argument, the installation, and no options. **Revision 4
  (SHR-R3-ENGINE-INSTALLATION-PAIR-01)** withdraws revision 3's flag-less,
  executable-anchored form: round 3 showed that under a `METASYSTEM_BIN`
  override the executing engine is physically installed somewhere else, so
  a self-anchored answer names one world while every shell-owned consumer
  of the same turn keeps naming `$world_installation`. The split is proven
  by the existing killed-attempt fixture
  (`supervision-hook-fixtures.sh:356-389`), whose wrapper engine in `$tmp`
  forwards to the harness engine and would have moved the turn's state root
  to that engine's own checkout while `up` and the collector stayed in the
  fixture bed. The verb prints `stateroot.RootForCandidate(<installation>)`,
  a new exported function that (1) canonicalizes the argument the way `up`
  canonicalizes `--metasystem-root` (`filepath.Abs` then
  `filepath.EvalSymlinks`, `up.go:16-25`), (2) applies the shape gate
  `installationRoot()` already applies to the executable's grandparent
  (`metasystem.conf` present, or `scripts/agents` a directory,
  `stateroot.go:149-153`, extracted into one private helper both callers
  use with no semantic change), and (3) returns `RootForInstallation` of
  the result (`stateroot.go:100-108`); exit 0. On refusal (shape gate
  failed, or an adopted-mode installation outside any git repository) it
  prints the error to stderr and **exits 1** — the same refusal shape as
  `path owner` (`path_verbs.go:23-30`). A missing or extra argument is the
  family's usage refusal, exit 2, which the hook cannot produce and which
  lands in the visible skew branch of Decision 2 if it ever does. The verb
  adds no state and no writes; its one git call (`repositoryTop`,
  `stateroot.go:42-50`) runs with git steering scrubbed, the same scrub the
  shell mapper adopts below.

**The pairing rule (SHR-R3-ENGINE-INSTALLATION-PAIR-01).** One turn names
one installation, and the engine computes the world FOR it: the shell
derives `world_installation` once; the same bytes go to `path state-root`,
to every `--metasystem-root` flag of the turn (`up`, `lease classify`,
`health`), and to the collector's location
(`$world_installation/scripts/agents/evidence-gc.sh`); `ms` is that
installation's `bin/metasystem` unless `METASYSTEM_BIN` replaces it, and a
replacement changes which engine computes the turn, never which
installation it computes for. Because `up` derives its state world as
`RootForInstallation(canonical(--metasystem-root))` (`up.go:104-108,
139-144`), and the verb returns exactly that function of exactly those
bytes, `$repo` and `up`'s state world cannot differ — with or without an
override. The collector honors the same override (`evidence-gc.sh:17` reads
`METASYSTEM_BIN` before falling back to its own `bin/metasystem`), so a
mapped or overridden turn runs one engine throughout. The three cases the
round-2 critique required the contract to distinguish keep their
dispositions, because the candidate still reaches the verb only through the
engine found at it:

- *Directory symbolic link on the invocation path*: normalized away by the
  candidate's `pwd -P`; the physical installation is the candidate.
  Supported.
- *Terminal symbolic link to the hook file*: the candidate is the physical
  directory holding the link, not the target. If no engine lives there, the
  hook stops on the missing-engine block (Decision 2), whatever
  `METASYSTEM_BIN` says; if
  an engine does live there, that engine answers for that installation, not
  for where the link pointed. Either way no pathname is believed.
- *Copied or relocated hook*: identical rule. A bare copy finds no engine
  and stops on the missing-engine block. A full relocated installation with
  its own engine IS an installation, and its engine answers for it. The
  fixture in Decision 3 (case 6) pins the dangerous sub-case: a hook copy
  inside a governed repository does not adopt that repository's world by
  pathname — **with or without a `METASYSTEM_BIN` override** (revision 5,
  below).

**Installation provenance under an override (SHR-R4-COPIED-HOOK-OVERRIDE-01).**
Round 4 showed that revision 4's executable check,
`ms="${METASYSTEM_BIN:-$world_installation/bin/metasystem}"`, never
required an engine at the candidate once an override was set: a copied hook
in `<repo>/development/sub/scripts/agents/` passes the shape gate (a
`scripts/agents` directory suffices, `stateroot.go:149-153`),
`RootForInstallation` answers the containing toplevel because template mode
is false (`stateroot.go:100-108`), and the round-two hole reopens by
pathname. The rule now: **the candidate must carry an engine at
`<candidate>/bin/metasystem` for the world to be governed, override or
not.** That file is the installation's own evidence of being an
installation; `METASYSTEM_BIN` may replace which engine RUNS the turn but
never waives it. Mechanically the hook computes
`canonical="$world_installation/bin/metasystem"` and
`ms="${METASYSTEM_BIN:-$canonical}"`, and requires BOTH `-x "$canonical"`
and `-x "$ms"` before any governed step; either absent is the
missing-engine outcome of Decision 2 (a block on Stop). Without an override
the two tests coincide with today's one test. Under an override, an
engine-less candidate — the copied hook, the terminal symbolic link, the
engine-less delegate worktree before mapping — can no longer be governed by
substituting an engine from elsewhere; the mapped primary (which carries
the engine) still is, so the delegate turn of the worktree rule is
unaffected. The killed-attempt, failure-engine, deadline, template and
membership fixtures all stage or possess `bin/metasystem` at the
installation they fire (`supervision-hook-fixtures.sh:7-8,164,474`), so
every existing override fixture keeps passing; the one existing fixture
whose installation has no engine (`missing-template`, lines 444-462) fires
under an override and expects `"decision":"block"`, which is exactly the
missing-engine block this rule produces. Decision 3 case 6 is re-pinned to
fire under `METASYSTEM_BIN` as well as without it.

Why the positional argument is not revision 2's trust hole: revision 2's
`--installation` took any pathname as the world's source with no engine
required at it and before any identification. Revision 4's argument is
admitted only after the mapper identified it and the `-x` test found the
engine at it (or the operator explicitly substituted one), it is
shape-gated inside the verb, and it is byte-identical to what `up` — a
state-writing verb of the same turn — already receives and trusts. The verb
extends no trust the turn does not already extend; it removes the one place
where the turn's engine answered for something other than the turn's
installation.

`METASYSTEM_BIN` therefore overrides the engine and nothing else. Under it
the world is the override engine's `RootForInstallation` answer for the
same `$world_installation` every consumer names, so the override cannot
split a turn; revision 3's sentence that the world "follows the override
engine's own answer" is withdrawn as the split round 3 proved. Fixture
consequence, traced: the killed-attempt fixture fires
`$line_root/scripts/agents/supervision-hook.sh` under
`METASYSTEM_BIN=$tmp/kill-engine` forwarding to the harness engine
(`supervision-hook-fixtures.sh:356-381`); the candidate is `$line_root`
(not a linked worktree), `$line_root/bin/metasystem` exists (staged by `cp`
at line 164), the verb returns `RootForInstallation($line_root)` =
`$line_root` (no template marker, adopted mode, its own git toplevel), and
the attempt evidence lands at
`$line_root/artifacts/agents/steward/components/supervision-hook.json`
exactly as the fixture asserts (lines 176, 388-389). Every other
wrapper-engine fixture in that file (lines 128, 260-336, 416-453, 535-598)
resolves the same way, because each fires a hook whose own `bin/metasystem`
was staged beside it. Fixtures stage engines by `cp`, never by symbolic link
(verified at commit 47e59bcd: `supervision-fixtures.sh:634,1577,1680,1732`);
under revision 4 that fact no longer carries the world, but it keeps every
fixture engine's own executable-anchored writers — none of which the hook
reaches — inside the fixture bed.

Revision 1's `metasystem.conf` marker rule remains withdrawn for the
reasons revision 2 recorded (stray-marker capture; silent rejection of a
`.local`-only template installation accepted by `stateroot.go:149-153` and
`internal/config/resolve.go:24-27,71-95`), and content markers stay out
entirely.

**Uniqueness argument, recast.** (a) There is no collision set to
tie-break: the candidate is the one physical directory the running script
is installed in, and a stray configuration file anywhere on disk never
enters the computation. (b) The output is *the same function*
(`RootForInstallation`) applied by *the same engine* to *the same
installation bytes* that every state-writing consumer of the turn
receives, so the hook cannot disagree with `up` about the world under
symbolic links, copies, relocations, or an engine override
(SHR-R2-INSTALL-01's residue closed; SHR-R3-ENGINE-INSTALLATION-PAIR-01's
split closed). (c) Presence of `metasystem.conf` is
not part of the answer, so a `.local`-only template installation resolves
identically to a committed-conf one. (d) Every unresolvable or unprovable
input is benign exit 0, and the only non-silent degradations are the two
fixed blocking reports of Decision 2 (missing engine, engine/hook skew)
— never a guess.

### The linked-worktree rule (folds SHR-R2-WORKTREE-ENGINE-01 and SHR-R2-WORKTREE-FALLBACK-01)

The decision itself stands from revision 2: **a hook firing inside a linked
worktree reports the primary checkout's world.** The turn verdict and the
hook evidence trail exist to keep the seat's session honest — they read the
goal ledger, the stream plans, and the job records beneath one root
(`internal/report/openwork.go:23-28,72-100`) and land evidence where
supervision is armed. A linked worktree carries the tracked half of that
state and never the ignored half; worktree-local reporting would tell a
delegate's turn-end "open plans, zero jobs in flight" while the delegate's
own job is the running work, and its evidence would vanish with the
sandbox. Suppressing delegate hooks stays rejected: a silent hook is
indistinguishable from an uninstalled one (`supervision-hook.sh:261-262`).

What changes is that the rule is now grounded in the delegate layout as
actually shipped, which revision 2's ordering made unreachable
(SHR-R2-WORKTREE-ENGINE-01):

- The runtime registration is tracked and relative: every checkout's
  `.claude/settings.json` fires `cd "$CLAUDE_PROJECT_DIR/metasystem" &&
  bash scripts/agents/supervision-hook.sh claude <event>` (read in this
  worktree's own copy), so a session whose project directory is a delegate
  worktree fires the **worktree's own tracked hook copy**.
- That copy's absolute path sits physically *inside the primary checkout*,
  because delegate worktrees live at
  `<primary>/metasystem/artifacts/agents/worktrees/<job-id>/` — so no
  path-prefix test can distinguish the worlds; only the git common-dir
  identity can.
- The worktree ships tracked files only: verified in this very worktree,
  which has `metasystem/scripts/agents/supervision-hook.sh` and
  `metasystem/plans/` but **no `metasystem/bin/metasystem` and no
  `metasystem/artifacts/`**, while its own pending job record sits in the
  primary's `artifacts/agents/jobs/`.

Revision 2 kept the engine check first and the mapping after, so in this
real layout the hook died at "engine missing" before the mapper could run,
and its fixture papered over that by staging an engine inside the worktree.
Both are corrected: **the worktree identification and mapping run before
any engine work, using only git and shell, and the engine then resolves at
the mapped world** — `ms="${METASYSTEM_BIN:-$world_installation/bin/
metasystem}"`. In the shipped delegate layout the mapping is exactly what
makes the turn possible at all: the sandbox has no engine, the primary
does. Revision 2's placement claim ("resolution ordering is unchanged,
after the executable and registry checks") and its "not consumers" claim
that `$ms` keeps resolving from `$harness_root` "including in worktrees"
are both withdrawn.

Mechanism: query `git -C "$harness_root" rev-parse --path-format=absolute
--git-dir --git-common-dir` (one call, two output lines; requires git ≥
2.31 — observed 2.39.5 on the fleet, and re-run in this worktree, where it
returns `<wrapper>/.git/worktrees/<job-id>` and `<wrapper>/.git`). If the
two paths are equal, the installation is not a linked worktree and
`world_installation=$harness_root`. If they differ: require the common
dir's basename to be `.git`; `primary_top` is the physical parent of the
common dir; `wt_top` is the physical git toplevel of `$harness_root`; the
installation's path relative to `wt_top` is re-rooted onto `primary_top`
to form `world_installation` (for the fleet layout,
`<primary>/metasystem`); the counterpart must carry a `scripts/agents`
directory. The mapping runs at most once; a counterpart that is itself a
linked worktree is not re-mapped.

**Git steering is scrubbed per call (SHR-R3-GIT-STEERING-01).** Both git
invocations of the mapper — the identification query and the
`--show-toplevel` query — run through one shell function, `hook_git`, that
executes `env -u <name> ... git "$@"` over exactly the twenty variables the
compiled authority removes before its own git call (`gitSteeringVariables`,
`stateroot.go:32-40`, applied by `scrubGitSteering` at
`stateroot.go:42-64`): `GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`,
`GIT_INDEX_FILE`, `GIT_CEILING_DIRECTORIES`,
`GIT_DISCOVERY_ACROSS_FILESYSTEM`, `GIT_OBJECT_DIRECTORY`,
`GIT_ALTERNATE_OBJECT_DIRECTORIES`, `GIT_CONFIG`, `GIT_CONFIG_PARAMETERS`,
`GIT_CONFIG_COUNT`, `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`,
`GIT_CONFIG_NOSYSTEM`, `GIT_GRAFT_FILE`, `GIT_SHALLOW_FILE`,
`GIT_REPLACE_REF_BASE`, `GIT_IMPLICIT_WORK_TREE`, `GIT_NO_REPLACE_OBJECTS`,
`GIT_PREFIX`. The list is a name-for-name copy of the Go list and changes
only with it; the house pattern is `scripts/adopt.sh:53-61`
(`scrubbed_env`), whose shorter list is not reused because the mapper must
be no weaker than the authority it feeds. The scrub is per call, never
process-wide: the hook does not `unset` these variables, so engine children
keep the environment they receive today and the engine scrubs for its own
git call as it already does. Why this is required and not hygiene:
`GIT_DIR` is exported inside every git hook and by rebase and bisect
subprocesses (`internal/gittree/gittree.go:61-67`), and round 3 reproduced
the failure — with `GIT_DIR` set to the primary's `.git` and
`GIT_WORK_TREE` to a real delegate worktree, the identification query
returned two equal paths, so the mapper would have kept the engine-less
sandbox as the world and the turn would have ended on the missing-engine
line instead of blocking. Re-run in this worktree at commit 47e59bcd:
steered, both lines are `<wrapper>/.git`; scrubbed, they are
`<wrapper>/.git/worktrees/<job-id>` and `<wrapper>/.git` again. `env -u` of
an unset name is a no-op; `env` or `git` absent from `PATH` makes the
substitution fail and takes the existing `|| exit 0` guard; nothing in the
scrub touches temporary storage, so the B1 guarantee and the failure map of
Decision 2 are unchanged. The mapper's outcome no longer depends on any
inherited process state other than the script's own physical location.

**The proof burden is inverted (SHR-R2-WORKTREE-FALLBACK-01).** Revision 2
mapped a failed identification (`git_ids=$(...) || git_ids=`) to "not a
linked worktree" and carried on — which, for a template worktree, is
precisely the sandbox-local answer this design rejects as structurally
false. A failed identification proves nothing. The rule now: **worktree
identification must succeed before anything governed rides the candidate;
every failure of the query — not a repository, git older than 2.31, git
absent from PATH, unparseable output, vanished directory — and every
failure of the mapping steps is benign silent exit 0, never the
non-worktree outcome.** No governed layout loses visibility by this:
today's hook is already silent when its resolution input is outside any
git repository (`supervision-hook.sh:65`), and every supported layout in
the case table sits inside one. On git older than 2.31 the hook now
degrades to universal silence instead of to a wrong world; the version
floor is declared rather than engineered around, since the fleet runs
2.39.5.

### The replacement block

Normatively, the engine-resolution block (5aad591f lines 23-31; HEAD
226-234) becomes the candidate/mapping/engine sequence below; the registry
and payload staging (HEAD 235-252) stand unchanged; the verb call replaces
the deleted payload-cwd/session-env/toplevel block (5aad591f 50-66; HEAD
258-274). **Revision 5**: `hook_git` and the candidate/mapping sequence are
defined as one shell function, `hook_world_installation`, placed BEFORE the
Stop-deadline parent block (HEAD line 32), because the parent needs the
same answer (below); the function prints `world_installation` on success
and returns 1 on every identification or mapping failure, and each caller
maps that return — the worker to its benign `exit 0`, the parent to its
"world unknown" state. The block is shown here in the worker's inline
form for readability; the function form is byte-equivalent apart from
`return 1` replacing each `exit 0`. The missing-engine and skew emissions
are the shipped `$raw_missing_engine_stop` block (HEAD line 21) and a
literal block of the same shape, not the `systemMessage` lines revision 4
printed (SHR-R4-FAIL-CLOSED-REGRESSION-01, Decision 2):

```bash
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/../.." && pwd -P)

# Git steering inherited from the parent (exported inside every git hook
# and by rebase/bisect subprocesses) must not redirect identification to
# another repository. The list is the compiled authority's own
# (internal/stateroot/stateroot.go, gitSteeringVariables), scrubbed per
# call so engine children keep their environment.
hook_git() {
  env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR -u GIT_INDEX_FILE \
    -u GIT_CEILING_DIRECTORIES -u GIT_DISCOVERY_ACROSS_FILESYSTEM \
    -u GIT_OBJECT_DIRECTORY -u GIT_ALTERNATE_OBJECT_DIRECTORIES \
    -u GIT_CONFIG -u GIT_CONFIG_PARAMETERS -u GIT_CONFIG_COUNT \
    -u GIT_CONFIG_GLOBAL -u GIT_CONFIG_SYSTEM -u GIT_CONFIG_NOSYSTEM \
    -u GIT_GRAFT_FILE -u GIT_SHALLOW_FILE -u GIT_REPLACE_REF_BASE \
    -u GIT_IMPLICIT_WORK_TREE -u GIT_NO_REPLACE_OBJECTS -u GIT_PREFIX \
    git "$@"
}

# Candidate world: the physical installation this script sits in, mapped
# to its primary counterpart when that installation is a linked worktree.
# Identification must SUCCEED; every failure is benign exit 0, never a
# guess and never the non-worktree outcome.
world_installation=$harness_root
git_ids=$(hook_git -C "$harness_root" rev-parse --path-format=absolute \
  --git-dir --git-common-dir 2>/dev/null) || exit 0
[[ "$git_ids" == *$'\n'* ]] || exit 0
git_dir=${git_ids%%$'\n'*}
git_common=${git_ids#*$'\n'}
[[ -n "$git_dir" && -n "$git_common" ]] || exit 0
if [[ "$git_dir" != "$git_common" ]]; then
  [[ "$(basename -- "$git_common")" == .git ]] || exit 0
  primary_top=$(cd -- "$(dirname -- "$git_common")" 2>/dev/null && pwd -P) || exit 0
  wt_top=$(hook_git -C "$harness_root" rev-parse --show-toplevel 2>/dev/null) || exit 0
  wt_top=$(cd -- "$wt_top" 2>/dev/null && pwd -P) || exit 0
  case "$harness_root" in
    "$wt_top") rel= ;;
    "$wt_top"/*) rel=${harness_root#"$wt_top"/} ;;
    *) exit 0 ;;
  esac
  world_installation=$primary_top${rel:+/$rel}
  [[ -d "$world_installation/scripts/agents" ]] || exit 0
fi

# The installation's OWN engine is the evidence that the candidate is an
# installation; METASYSTEM_BIN may replace which engine runs, never that
# evidence. A missing engine blocks a Stop because no safe verdict can be
# made (the shipped contract, HEAD lines 4-6 and 21).
canonical="$world_installation/bin/metasystem"
ms="${METASYSTEM_BIN:-$canonical}"
if [[ ! -x "$canonical" || ! -x "$ms" ]]; then
  if [[ "$event" == stop ]]; then
    printf '%s\n' "$raw_missing_engine_stop"
  fi
  exit 0
fi
# ... registry membership and payload staging, byte-identical to today ...

# The world is the ENGINE's answer FOR THE INSTALLATION every consumer of
# this turn names (the same bytes every --metasystem-root below carries),
# never the pathname's own. Exit 1 is the verb's own refusal (ungoverned
# installation): silent here; the Stop parent converts silence to its
# generic block. Any other failure is engine/hook skew in a governed world:
# it blocks a Stop and names itself, or a fleet that rebuilds its engines
# daily cannot tell a skewed hook from one that never fired.
repo_rc=0
repo=$("$ms" path state-root "$world_installation" 2>/dev/null) || repo_rc=$?
if (( repo_rc == 1 )); then
  exit 0
elif (( repo_rc != 0 )) || [[ -z "$repo" ]]; then
  if [[ "$event" == stop ]]; then
    printf '%s\n' "$raw_engine_skew_stop"
  fi
  exit 0
fi
repo=$(cd -- "$repo" 2>/dev/null && pwd -P) || exit 0
```

`raw_engine_skew_stop` is a literal defined beside `raw_missing_engine_stop`
(HEAD line 21), engine-independent by construction:
`{"decision":"block","reason":"Metasystem engine and hook are out of step: this engine does not answer path state-root, so stopping safety cannot be judged; rebuild bin/metasystem before stopping."}`.
The parent's no-validator fallback (HEAD 172-175) accepts this literal
exactly as it accepts the missing-engine literal (below). **Revision 6
withdraws that sentence**: the parent validates the skew literal through
its structured path, because a worker that emitted it had both
executables and so does its parent; the fallback stays as shipped
(Decision 1, parent subsection; Decision 2).

### The Stop-deadline parent (folds SHR-R4-DEADLINE-PARENT-01)

The shipped hook wraps every Stop in a deadline parent (HEAD 32-222): the
parent stages the payload, relaunches this script as a worker (55-57),
resolves its record coordinates "alongside the worker, never ahead of it"
(60-63), validates the worker's JSON with `deadline_validator` (144-175),
and on timeout writes a stop-refusal record through `deadline_canonical`
(203-217). Revision 4 governed none of this. As shipped the parent is a
second, contradictory root and engine owner: `deadline_harness_root` is the
unmapped script grandparent (44-45), `deadline_validator` and
`deadline_canonical` both sit beneath it (46-47), and the refusal-record
root is `git -C "$deadline_cwd" rev-parse --show-toplevel` over the
PAYLOAD cwd, unscrubbed (85-87). In the engine-less delegate worktree the
revision-4 worker reaches the primary engine, but its parent has no
validator, so the no-validator fallback (172-175) accepts only the exact
missing-engine literal and replaces every ordinary verdict with the generic
block (176-177); on timeout the parent either records under the cwd-derived
worktree or wrapper root, or, lacking a canonical engine, emits the
record-failure allow line (216). Revision 5 governs the parent by one rule:
**the parent and the worker name one installation, one canonical engine,
and one world, computed by the same function from the same script
location.**

- **Ordering is preserved.** The certified worker-first ordering stands:
  the worker is launched first (55-57), and the parent resolves its own
  coordinates while the worker runs, exactly where the shipped resolver
  subshell runs today (64-75). Nothing governed is computed ahead of the
  worker's launch, so a slow resolution cannot eat the worker's budget.
- **Installation.** While the worker runs, the parent calls
  `hook_world_installation` (the function defined above, before line 32);
  the answer is `deadline_installation`. On failure the parent's world is
  unknown: `deadline_validator` (revision 6: `deadline_engine`) and
  `deadline_canonical` are set empty, so
  `-x` fails, the no-validator fallback governs completion, and the
  timeout path takes the shipped record-failure branch. The worker in the
  same state exited silently at identification, which that fallback
  converts to the generic block — the same fail-closed answer the shipped
  hook gives a cwd outside any repository (HEAD 268-270, 176-177).
- **Engines.** `deadline_canonical=$deadline_installation/bin/metasystem`
  and `deadline_validator="${METASYSTEM_BIN:-$deadline_canonical}"`:
  the same two names, the same override rule, and the same provenance
  requirement as the worker (the canonical engine must be executable for
  the world to be governed). In the mapped worktree both now resolve at
  the primary, where the engine is, so the parent can validate the
  worker's ordinary JSON and can write a refusal record. The resolver
  subshell (67-75) keeps using `deadline_canonical` for `json get`; its
  cwd line becomes unused and is dropped with `deadline_cwd`. **Revised
  by revision 6** (below): `deadline_validator` is renamed
  `deadline_engine`, it is set only when both executables exist, the
  subshell runs on it, and its second line carries the state root.
- **Refusal-record root.** `deadline_resolve_record` (82-93) no longer
  runs git over the payload cwd. It computes
  `deadline_repo=$("$deadline_canonical" path state-root
  "$deadline_installation" 2>/dev/null) || return 1`, physically
  normalizes it as today (87), and keeps the slug and record path
  (88-92). The record therefore lands at
  `<world>/artifacts/agents/supervision/stop-refusals/<slug>.json` for
  the world the worker's verdict was computed in — the primary for a
  mapped worktree, `<wrapper>/metasystem` on the fleet — never under a
  cwd-derived toplevel. The verb's exit 1 (ungoverned installation) and
  every other failure leave `deadline_record` empty, which is the shipped
  record-failure branch (207-217): the parent emits the allow line so a
  record failure cannot recreate the refusal loop. `deadline_cwd`, its
  `sed` extraction (77), and the engine-coordinate capture of a second
  line (98-106) are deleted; only the session line of the resolution file
  remains, and `deadline_session` keeps its shell-then-engine precedence.
  **Revised by revision 6** (below): the query runs on `deadline_engine`,
  inside the resolver subshell rather than synchronously, and the
  resolution file's second line is kept to carry its answer;
  `deadline_resolve_record` consumes `deadline_repo` and computes nothing
  else.
- **Validation is unchanged in shape.** The parent's acceptance predicate
  (165-171) and its two fallbacks (172-177) are untouched; the fold only
  moves which engine validates and where the record lands. Because
  Decision 2 now makes the missing-engine and skew outcomes literal
  blocks, the no-validator fallback accepts both literals byte-for-byte
  (174 gains `|| [[ "$deadline_raw" == "$raw_engine_skew_stop" ]]`).
  **Withdrawn by revision 6** (below): the fallback stays exactly as
  shipped and accepts only the missing-engine literal.

**One engine in the parent (revision 6, folds
SHR-R5-DEADLINE-OVERRIDE-ENGINE-SPLIT-01).** Round 5 showed that revision
5 kept two engines in the parent: the override validated the worker's
JSON, but the canonical engine still derived the refusal-record world and
still wrote the record (`report stop-block`, today's line 224). On the
fleet that split is not hypothetical: engines are rebuilt daily, so an
installation whose `bin/metasystem` predates `path state-root` while the
operator's `METASYSTEM_BIN` names a rebuilt one is the ordinary
post-landing state. Under revision 5 that turn's worker governed
correctly through the override, and its parent, on timeout, asked the old
engine for the state root, got exit 2, and emitted the record-failure
allow line — the exact loss SHR-R4-DEADLINE-PARENT-01 was folded to
prevent, reopened by the engine choice. The rule, stated once for both
halves of the hook: **the installation's own engine is evidence that the
candidate is an installation, and nothing else; the engine that RUNS is
`METASYSTEM_BIN` when set, and it runs everything.** In the worker that is
already the `canonical`/`ms` pair of the provenance rule. In the parent
the same pair is `deadline_canonical` and `deadline_engine`:

- **Names.** `deadline_canonical=$deadline_installation/bin/metasystem`
  as revision 5 has it. `deadline_engine` replaces `deadline_validator`
  and is set to `"${METASYSTEM_BIN:-$deadline_canonical}"` only when BOTH
  `-x "$deadline_canonical"` and `-x "$deadline_engine"` hold; otherwise
  it is the empty string. Every engine invocation in the parent block
  names `$deadline_engine`: the resolver subshell's `json get` (today's
  line 69), the state-root query (below), the six validation calls
  (today's lines 160-171), and the timeout record (today's line 224).
  `deadline_canonical` appears in the parent in exactly one place, the
  executable test. An empty `deadline_engine` is what the shipped code
  already treats as "no validator" (`[[ -x "$deadline_validator" ]]` at
  today's line 159 fails on an empty string), so the no-validator fallback
  and the record-failure branch govern without a new branch.
- **Placement: the state-root query is timeout-safe because it runs where
  the deadline can kill it.** The shipped parent keeps its main flow free
  of engine waits until the worker finishes or the deadline passes: engine
  parsing runs in the resolver subshell (today's lines 67-75), which the
  wait loop harvests (`deadline_capture_engine_coordinates`, 110-123) and
  the deadline stops (`deadline_stop_resolver`, 124-135). Revision 5 put
  `path state-root` in `deadline_resolve_record`, which the parent calls
  synchronously right after the worker launch (today's line 94); on an
  engine that is slow for that verb the parent would enter its wait loop
  late and the Stop would overrun Claude's five seconds with no emission
  at all. Revision 6 moves the query into the subshell: its second line,
  which today carries the payload cwd (line 70), carries the engine's
  state-root answer instead — `resolver_root=$("$deadline_engine" path
  state-root "$deadline_installation" 2>/dev/null) || resolver_root=` —
  and the session line keeps its `|| exit 1` shape only for itself: the
  subshell writes both lines whatever each returned, so a failed session
  parse cannot discard a good root and a refused root cannot discard a
  good session. `deadline_capture_engine_coordinates` reads the first
  non-empty line into `deadline_session` (as today) and the second
  non-empty line into `deadline_repo`, then calls
  `deadline_resolve_record`, which now needs only `deadline_session` and
  `deadline_repo`: it physically normalizes the root (today's line 87,
  guarded `|| return 1`), keeps the slug and record path (88-92), and
  clears the failure text. The synchronous call at today's line 94 is
  deleted with `deadline_cwd` and its `sed` (line 77): before the engine
  answers there is nothing a shell can resolve, by design. The subshell is
  started only when `deadline_engine` is non-empty (today's line 67, with
  the double test in place of the single one).
- **What the timeout path can and cannot do.** If the subshell answered
  inside the worker's wait, the timeout writes the refusal record through
  `$deadline_engine` under the state world and emits the block, as the
  deadline fixture asserts today. If it did not — the engine is slow for
  `path state-root`, the verb refused (exit 1), the engine lacks the verb
  (exit 2), or the installation could not be identified —
  `deadline_record` is empty and the parent emits the shipped
  record-failure allow line (today's lines 232-234). That last outcome is
  narrower than it was: under revision 5 an old canonical engine reached it
  on every overridden timeout; under revision 6 only an engine that cannot
  answer for itself does, and such an engine has already blocked the
  worker's Stop with the skew literal on every completed turn, so the
  drift is visible before the first timeout. The parent's wall-clock
  contract is unchanged: no engine call sits between the worker launch and
  the wait loop.
- **The record write is on the one engine.** `report stop-block
  --refusal-record ...` at today's line 224 runs `"$deadline_engine"`, so
  an overridden timeout writes through the engine that computed the
  world; the canonical engine never writes anything in an overridden turn.
  Fixture consequence: the shipped deadline fixture's wrapper
  (`supervision-hook-fixtures.sh:405-413`) sleeps on `runtime list` and
  execs the real engine for every other verb, so its two firings keep
  their record location and both assertions; the failure-engine and
  kill-engine wrappers forward every parent verb unchanged.
- **The no-validator fallback is as shipped.** Revision 5 widened it to
  accept the skew literal for the race window between the worker's and the
  parent's executable tests. That widening is withdrawn: the skew literal
  is emitted only in a world where both executables exist, and in that
  world the parent has `deadline_engine` and validates through the
  structured path, so no consistent world reaches the fallback with the
  skew literal (SHR-R5-SKEW-FIXTURE-VALIDATOR-01's second part). In the
  race window the fallback rejects the literal and the parent emits its
  generic block — still a block, without the rebuild remedy, for one Stop.
  The fallback compares one literal, the missing-engine block, exactly as
  today's line 189 does.

Consequence for the one-world claim of Decision 4: in the mapped case the
parent's validator, canonical engine, and refusal record are the primary's,
so "every moving part of the turn names the primary" now includes the
parent; and on the fleet the refusal record moves from the wrapper root's
stray `artifacts/` (where the cwd-derived toplevel put it) to the state
world, beside the attempt evidence. The fixtures of Decision 3 (cases
10-13) pin normal completion and timeout for the engine-less worktree,
under git steering and under `METASYSTEM_BIN`.

The payload's `cwd` field and the `runtime session-env` fallback chain
(lines 50-64) are deleted with this change: the world is a function of the
installation and its engine, so depth and direction of the session's
working directory can no longer change the answer — the goal's DONE
determinism directly. The engine verb `runtime session-env` itself is
untouched; only the hook's call is removed (the hook was its sole shell
caller, and no fixture drives that fallback). Every
`--metasystem-root "$harness_root"` in the hook (`up` at lines 148, 154,
342, 348; `lease classify` at 122, 131; `health` at 161) becomes
`--metasystem-root "$world_installation"`, so the whole turn rides one
world; where no mapping occurred the value is byte-identical to today's.
Today those sites are `up` at 433, 439, 711, 724; `lease classify` at 377,
391; `health` at 451; and member A's `proc find-ancestor --repo
"$harness_root"` at 357 joins the list: the adapters belong to the
installation, so it takes `$world_installation` too (Decision 3's sweep).

### The compiled authority is the only one (revision 6, folds SHR-R5-TEMPLATE-MARKER-SECOND-AUTHORITY-01)

Revision 5 replaced the cwd-and-toplevel block (today's lines 276-292)
with the verb call and said nothing about the block that follows it.
Today's lines 293-306 are a second state-root authority written in shell:
`state_root=$repo`, then a test for `$repo/development/metasystem-design.md`,
then either `exit 1` on a Stop (when the hook's installation is not
`$repo/metasystem` or carries no `metasystem.conf`) or
`state_root=$harness_root`. Member A's landing today added a third copy:
`deadline_log_stop_outcome` (today's lines 95-109) derives
`supervision_root` from the same marker test (99-103) so the parent's trail
line lands beside the worker's. Both copies exist because, before this
design, `$repo` was the Git toplevel and something had to find the
installation's state again; after this design `$repo` IS the state world,
computed by `templateMode` and `RootForInstallation`
(`stateroot.go:157-163,100-108`) inside `path state-root`, so the shell
copies can only agree with the engine or contradict it. They contradict it
in two supported cases the critic named: an adopted installation inside a
repository that also carries the marker (the engine answers the toplevel
because the installation is not named `metasystem`; the shell block exits
1 on Stop, which the parent turns into the generic block, so an
engine-approved world is refused by a pathname test), and a template
installation whose configuration is `.local`-only (the engine accepts it,
`stateroot.go:149-153` and `internal/config/resolve.go:24-27,71-95`; the
shell block refuses it for lacking `metasystem.conf`). Those are the two
defects revision 1's marker rule was withdrawn for, alive in shipped code.
The verdict revision 6 binds:

- **The worker block at today's lines 293-306 is deleted entire**, with
  the three names it introduces (`state_root`, `template_marker`,
  `template_installation`). No compatibility alias is kept: an alias is a
  second name for one value, and a second name is where the next
  divergence starts. **Every consumer of `$state_root` names `$repo`**:
  `lease classify --root` (today's 377, 391), the trail directory in
  `emit_stop_payload` (485), `lease protocol-growth --root` (550),
  `lease protocol-advance --root` (584, 687), `lease renew --root` (590),
  the trail directory before the collector (613), `report turn-verdict
  --root` (630), and `session end --root` (701). The implementer greps
  `state_root` in the hook after the edit and finds no hit.
- **The parent's marker test at today's lines 99-103 is deleted** with
  `supervision_root`; `deadline_log_stop_outcome` writes to
  `"$deadline_repo/artifacts/agents/supervision"`, where `deadline_repo`
  is the state-root answer harvested from the resolver subshell (parent
  subsection above). The helper's `[[ -n "${deadline_repo:-}" ]] ||
  return 0` guard (today's line 97) stands: a parent that could not name
  the world logs nothing, as today.
- **Member A's intent is kept by construction, not by a test.** Member A
  moved the three trail sites onto the marker-derived root so the trail
  sits beside the supervision state in both layouts. Under revision 6 the
  trail sites name `$repo` and `$deadline_repo`, which are
  `RootForInstallation(world_installation)`: the installation in the
  self-hosting layout (template mode), the repository scope in the
  vendored-operator layout (adopted mode) — the same two answers, from the
  authority `up` arms with. The template-layout assertions member A added
  (`supervision-hook-fixtures.sh:680-683`: the trail exists under the
  nested installation and not under the outer root) and the nested-root
  scenario's case 3 pin the self-hosting half; the operator-layout
  after-Stop check (Decision 3) pins the vendored half.
- **The `missing-template` fixture (`supervision-hook-fixtures.sh:522-540`)
  is repurposed, not retired.** Its bed is a hook copy at
  `<outer>/metasystem/scripts/agents/` under a marker-carrying outer root,
  with no `bin/metasystem` at the installation, fired on Stop under an
  override that forwards to a real engine. Today it blocks because the
  shell marker block exits 1; under revision 5's double executable test it
  blocks because the canonical engine is missing; its one assertion,
  `"decision":"block"`, passes for either authority and so proves nothing
  about which one answered. It now asserts what only the compiled
  authority's path produces: the output contains `engine missing` (the
  shipped literal's remedy text) and no `HEALTH unknown` line, exactly the
  shipped missing-engine fixture's pair of assertions
  (`supervision-hook-fixtures.sh:152-157`), and after the firing
  `[[ ! -e "$missing_template_outer/artifacts" && ! -e
  "$missing_template_root/artifacts" ]]`. Before the fold the output is
  the parent's generic block (`could not prove that stopping is safe`),
  which carries no `engine missing`, so the fixture fails before and
  passes after. It is the hook-fixtures-file twin of Decision 3 case 6:
  a hook without its own engine is not an installation, override or not,
  marker or not.

Case table — every hook firing resolves to exactly one world or one
defined degradation:

| Layout | Installation (script location) | Result |
| --- | --- | --- |
| Flat adopted (make_repo fixtures, adopted repositories) | the repository root | engine at the root answers: adopted mode → its own git toplevel; byte-identical to today |
| Fleet template nested (m0b, m2, m3) | `<wrapper>/metasystem`, template marker tracked at the wrapper | engine at `<wrapper>/metasystem/bin` answers: template mode → `<wrapper>/metasystem` — the fix |
| Operator nested adopted (`operator-layout` fixture: no template marker at the scope) | `<scope>/metasystem` | engine answers: adopted mode → `<scope>` — identical to today's cwd-toplevel answer; see Decision 4 |
| Linked delegate worktree of the fleet checkout (tracked files only: no engine, no artifacts in the sandbox) | `<worktree>/metasystem` | mapped to `<primary>/metasystem` **before** engine work; the primary's engine answers: template mode → the primary world |
| Hook copy or terminal hook symlink inside some repository, no engine at the candidate — with or without `METASYSTEM_BIN` | anywhere | the missing-engine BLOCK on stop, exit 0; no world, no writes (revision 5) |
| Governed installation whose engine predates this design | any supported layout | the engine/hook-skew BLOCK on stop, exit 0 (Decision 2, revision 5) |
| Governed installation whose own engine predates this design, under a `METASYSTEM_BIN` override that is a rebuilt engine (the fleet's daily post-landing state) | any supported layout | the override answers for the installation in the worker AND in the parent: an ordinary verdict on completion, a refusal record under the state world on timeout (revision 6) |
| Adopted installation (not named `metasystem`) inside a repository that also carries `development/metasystem-design.md` | `<repo>/<anything-else>` | the engine answers: adopted mode → the repository toplevel; today's shell marker block refused this world with exit 1 on Stop, and is deleted (revision 6) |
| Template installation with `.local`-only configuration (no `metasystem.conf`) | `<wrapper>/metasystem` | the engine answers: template mode → the installation; today's shell marker block refused it, and is deleted (revision 6) |
| Hook staged outside any git repository | anywhere | identification fails → worker silent exit 0; on stop the parent converts silence to its generic block (today 191-193) |

### Placement and the binding header contract

The resolver is a shell block in the hook plus the one read-only engine
verb. The ordering **changes** (SHR-R2-WORKTREE-ENGINE-01): identification
and mapping now precede the engine check, because they are the step that
locates the engine. The hook's binding header comment
(`supervision-hook.sh:4-15`) is rewritten in the same change to: *(1)
shell-owned syntax refusals — unchanged; (2) world identification — the
physical installation this script belongs to, mapped to its primary
counterpart when it is a linked worktree, by git common-dir identity
alone, with inherited git steering scrubbed per call, before any engine
work; every identification failure is benign exit 0, never a guess; (3)
executable resolution at the identified world — the installation's own
`bin/metasystem` must exist whether or not `METASYSTEM_BIN` replaces the
engine that runs, and a missing engine blocks a Stop because no safe
verdict can be made; (4) registry membership — an unknown runtime exits 2;
(5) world validation by the engine for the identified installation —
`path state-root <installation>`, the shipped state-root authority applied
to the same installation every consumer flag of the turn names: its exit-1
refusal is benign silence, any other failure blocks a Stop as engine/hook
skew; (6) the Stop-deadline parent resolves the same installation by the
same function, runs one engine — the override when set — for its session
parse, its state-root query, its validation, and its refusal record, and
records refusals under the same world; (7) the state world has one name,
`$repo`, and no shell test re-derives it from a marker, a pathname, or a
configuration file.* The old items (4) session environment and (5) cwd
resolution are deleted with their code; the shipped item (2) wording ("a
missing engine blocks a Stop", today's lines 4-6) is kept, not weakened.
Items (6) and (7) are revision 6's. The B1
guarantee ("a missing engine with an unusable TMPDIR must still exit 0
benign") is preserved: the mapping uses no temporary files, the
missing-engine block is a literal `printf`, and payload staging still
happens only after the engine exists.

## Decision 2 — failure shape (folds SHR-R2-ENGINE-SKEW-01 and SHR-R2-WORKTREE-FALLBACK-01)

The hook runs under `set -euo pipefail` (`supervision-hook.sh:2`), so an
unguarded command substitution is an abort, not a benign exit. The
discipline stands from revision 2 and binds the implementation: **every
command substitution the resolver introduces is written in the guarded
form `value=$(... 2>/dev/null) || <mapped outcome>`**, and the assignment
carries the substitution's status so the guard catches it before `set -e`
can.

What revision 2 got wrong, per the critique, was the *direction* of two
mappings. First, identification failure flowed into the non-worktree
branch (fixed above: it is now silence). Second, every engine error at the
verb was silence — but engine binaries are untracked build artifacts
(`bin/metasystem` is ignored; this fleet rebuilds engines daily), so
source-versus-binary skew is a normal post-landing state, and a
present-but-older engine that lacks the new verb would have turned a
governed Stop hook silently off. That is indistinguishable from a hook
that never fired — the exact confusion the evidence trail exists to
prevent, and the design's own argument against suppressing worktree
hooks. **A mismatched engine is neither absence nor an ungoverned
installation: it is drift in a governed world, and drift is visible.**

**The shipped contract is fail-closed, and revision 4 regressed it
(SHR-R4-FAIL-CLOSED-REGRESSION-01).** Between revision 2's tracing and
today, the hook's Stop contract moved: a missing engine now BLOCKS a Stop
"because no safe verdict can be made" (HEAD 4-6), the emission is the
literal `raw_missing_engine_stop` block (HEAD 18-21, 229-233), and the
missing-engine fixture rejects any `HEALTH unknown` line and requires
`"decision":"block"` (`supervision-hook-fixtures.sh:137-157`). Revision 4's
replacement block still printed the old `systemMessage` for a missing
engine and a second `systemMessage` for skew — and the deadline parent,
when it has a validator, accepts a lone `systemMessage` as a valid
non-blocking answer (HEAD 168-169), so an older engine lacking the verb
would have let the turn end before any verdict. Reconciled: **a missing
engine and an old engine lacking the verb both BLOCK a Stop.** The
missing-engine emission is the shipped literal, unchanged; the skew
emission is the new literal `raw_engine_skew_stop` of the same shape
(Decision 1); both are engine-independent `printf`s; the existing
missing-engine fixture is kept as-is and an old-engine fixture (case 7,
re-pinned) asserts the block. "Visible" in the sentences above now means
"blocks and names itself", which is strictly stronger than the one-line
report revision 2 argued for. On start and end events both outcomes stay
silent exit 0, as today.

The verb's exit codes make the three-way split mechanical, and both
non-zero shapes were verified against real binaries: the verb itself
refuses with exit 1 (the `path owner` refusal shape it sits beside), and
the shipped `path` family dispatcher answers an unknown verb with exit 2
and a stderr diagnostic (ran `metasystem path state-root` against the
currently installed pre-fix engine on m0b: `metasystem path: unknown verb
"state-root"`, exit status 2 — the critic's probe, reproduced). An engine
so old it lacks the whole `path` family also exits non-{0,1} through the
top-level dispatcher, landing in the same skew branch.

The complete failure map:

| Operation | Failure | Mapped outcome |
| --- | --- | --- |
| `hook_git rev-parse --path-format=absolute --git-dir --git-common-dir` | not a repository, git < 2.31, git or `env` absent, vanished directory | **silent exit 0** — identification failure proves nothing and is never treated as "not linked"; the call runs with git steering scrubbed, so an inherited `GIT_DIR`-class variable can neither cause nor mask any of these (SHR-R3-GIT-STEERING-01) |
| two-line output parse | second line missing or empty | silent exit 0 |
| common-dir basename test | not `.git` (bare or exotic layout) | silent exit 0 |
| `primary_top` / `wt_top` physical normalization | directory vanished or unreadable | each is `$(cd -- ... 2>/dev/null && pwd -P) || exit 0` |
| containment `case` | installation not at or below its worktree toplevel | silent exit 0 |
| primary counterpart shape check | no `scripts/agents` directory there | silent exit 0 |
| engine executable test at the world: `-x "$canonical"` AND `-x "$ms"` | no engine at `<installation>/bin/metasystem`, or an override that is not executable | **the shipped missing-engine BLOCK on stop** (`raw_missing_engine_stop`, HEAD 21), exit 0 — the fail-closed contract, kept; the candidate's own engine is required whether or not `METASYSTEM_BIN` is set (SHR-R4-COPIED-HOOK-OVERRIDE-01) |
| `path state-root "$world_installation"`, exit 1 | the verb's own refusal: `$world_installation` fails the shape gate, or is an adopted-mode installation outside any git repository | worker silent exit 0; on stop the parent's generic block |
| `path state-root "$world_installation"`, any other nonzero exit, or exit 0 with empty stdout | verb absent from an older engine (verified exit 2), family absent, a usage refusal (exit 2), or a broken answer | **the engine/hook-skew BLOCK on stop** (`raw_engine_skew_stop`, a literal `printf` mirroring the missing-engine block), exit 0 (SHR-R4-FAIL-CLOSED-REGRESSION-01) |
| final `repo` physical normalization | directory vanished or unreadable | worker silent exit 0; on stop the parent's generic block |
| evidence-trail `mkdir -p "$supervision_dir"` (HEAD 580, reachable with a write-denied primary from a sandboxed worktree session) | permission denied | gains `2>/dev/null || true`; the appends that follow already carry their own guards (HEAD 582, 613-614, 640-641) |
| parent: `hook_world_installation` (SHR-R4-DEADLINE-PARENT-01) | any identification or mapping failure | `deadline_engine` and `deadline_canonical` empty; completion takes the no-validator fallback (today 187-190), timeout takes the record-failure allow line (232-234) — the shipped shape for an unresolvable world |
| parent: `-x "$deadline_canonical" && -x "$deadline_engine"` (revision 6) | no engine at the mapped installation, or an override that is not executable | `deadline_engine` empty; same as the row above; the worker in the same state emitted the missing-engine block, which the fallback accepts byte-for-byte |
| parent resolver subshell: `"$deadline_engine" path state-root "$deadline_installation"` (revision 6) | exit 1 (ungoverned), any other failure, empty answer, or no answer before the deadline (a slow engine) | the subshell's root line is empty or never arrives; `deadline_repo` and `deadline_record` stay empty → on timeout the record-failure allow line (today 232-234), never a record under a cwd-derived root and never a wait in the parent's main flow; the worker in the same world already blocked its completed Stops with the skew literal, so the drift is visible before the first timeout |
| parent: `"$deadline_engine" report stop-block` on timeout (revision 6) | the one engine cannot write the record | `deadline_record_failure` set → the record-failure allow line, as today (224-234); the canonical engine writes nothing in an overridden turn |
| parent: structured validation of the skew literal (revision 6) | the one engine's `json get` or `json strip` fails on the literal | the generic block (today 191-193), as for any unparseable worker answer; a real pre-verb engine answers both verbs, and case 7's stub forwards them, so the literal validates as an ordinary block and passes through |

Worker silence and the parent: every "worker silent exit 0" row above is
the worker's own behavior; on a Stop the shipped parent converts an empty
worker answer into its generic block (`emit_raw_stop_block`, today
191-193; the contract stated at 286-288). Revision 5 inherits that
conversion unchanged — it is the fail-closed floor beneath every silent
row — and the "exactly two fixed outputs" sentence below counts the
worker's own emissions.

How the parent sees the two literals (revision 6): the missing-engine
literal is emitted by a worker with no `deadline_engine` counterpart in
the parent, so it reaches the no-validator fallback and is accepted
byte-for-byte (today's line 189). The skew literal is emitted by a worker
whose two executable tests passed, so the parent in the same world has
`deadline_engine` and validates the literal through the structured path
(today 159-186): decision `block`, a non-empty string reason, no
`systemMessage`, nothing else — which the literal satisfies. The fallback
therefore never needs to know the skew literal, and revision 5's widening
of it is withdrawn.

A cwd outside any git repository — today's benign case at line 65 —
remains benign by construction: cwd no longer participates at all.
Downstream writes that the worktree decision points at a possibly
write-protected primary (steward attempt/complete, `up`, turn-verdict
state) flow through the hook's existing disclosed degradation channels
(`hook_evidence_failure`, `up_failure`, the degraded-verdict branch at
lines 306-320): the hook reports degraded, emits, and exits 0.

The asymmetry rule now has three legs: a too-strict resolution degrades to
today's silence (which the parent turns into a block on Stop); a guessed
resolution recreates this defect somewhere else; and drift in a governed
world blocks a Stop and names itself. The resolver introduces no new
nonzero exits, and exactly **one** new output: the skew block, on the same
channel, in the same shape, and under the same stop-only condition as the
existing missing-engine block (HEAD 229-233).

## Decision 3 — fixtures (folds SHR-R2-WORKTREE-ENGINE-01, SHR-R2-ENGINE-SKEW-01, SHR-R2-INSTALL-01; pins the worktree and fallback rules)

House pattern throughout: a `fixture_scenario` guard block in
`scripts/agents/supervision-fixtures.sh` like `stop-hook-monitor` (line
1515). The shipped block-once behavior suppresses a second identical block
for one session (`supervision-fixtures.sh:1553-1555`), so **every case
below that asserts `"decision":"block"` carries its own named
`session_id` — a fixture obligation, not a suggestion.** Assertions
identify the resolved world by what the hook then reads and writes — the
sentinel step its block reason quotes and where its evidence lands — never
by comparing a path string. Engines are staged by `cp`, matching every
existing fixture; never by symbolic link.

New scenario `nested-root` (template-mode nested; models the fleet):

- Construction: `scope=$tmp/nested-root`; build the world at
  `$scope/metasystem` exactly the way `stop-hook-monitor` builds
  `stop_root` (`supervision-fixtures.sh:1519-1544`: copy the hook, arm
  script, pre-commit guard, adapters; stage the engine by `cp` at
  `bin/metasystem`; print the fake-runtime `metasystem.conf`; write
  `plans/stream.md` with `Next step: dispatch the nested runner` — the
  sentinel that exists only in this world). Create the template marker as
  an empty regular file `$scope/development/metasystem-design.md`, and
  `mkdir -p "$scope/development/sub"` as a marker-free sibling subtree.
  `git -C "$scope" init`, add, and commit (the commit is required for the
  worktree case). Leave the scope toplevel bare of `metasystem.conf` and
  `plans/` (matching the observed fleet wrapper root). Register
  `$scope/metasystem` in `fixture_harness_roots`; the hook is synchronous,
  so no waits and no owned pids.
- **Case 1, session `nested-sibling`** — payload cwd
  `$scope/development/sub`, event `stop`, fired through
  `$scope/metasystem/scripts/agents/supervision-hook.sh`. Assert
  `"decision":"block"` with a reason quoting `dispatch the nested runner`.
  Pre-fix the hook resolved this firing to the bare scope toplevel and
  could not block, so this case fails before the fix and passes after:
  the regression the goal demands. (The payload still carries a cwd
  because runtimes send one; the fix makes it inert.)
- **Case 2, session `nested-inside`** — payload cwd
  `$scope/metasystem/scripts/agents`, same hook, same assertion: two
  firings, one world, one answer.
- **Case 3, evidence lands in the world** — after cases 1-2, assert
  `$scope/metasystem/artifacts/agents/supervision/hooks.log` is non-empty
  and `[[ ! -e "$scope/artifacts" ]]` — the misdirected-write signature
  observed live on m0b must not reappear.
- **Case 4, session `nested-worktree` (the linked-worktree pin,
  re-grounded)** — `git -C "$scope" worktree add "$tmp/nested-wt" HEAD`.
  **Stage nothing inside the worktree**: it carries tracked files only —
  no `metasystem/bin/metasystem`, no `metasystem/artifacts/` — exactly the
  real delegate layout this worktree itself exhibits; revision 2's staged
  worktree engine masked the production condition and is withdrawn.
  Register `$tmp/nested-wt/metasystem` in `fixture_harness_roots`. Edit
  the **primary's** `$scope/metasystem/plans/stream.md` to `Next step:
  recover the primary sentinel` (uncommitted, so the worktree keeps the
  old text). Fire the **worktree's own tracked hook copy** with payload
  cwd inside `$tmp/nested-wt` and session `nested-worktree`. The mapping
  must run before engine work and find the primary's engine, or this case
  cannot block at all. Assert `"decision":"block"` with a reason quoting
  `recover the primary sentinel` — a string that exists only in the
  primary world, so the hook can only have said it by reporting the
  primary through the primary's engine. Assert the evidence appended to
  the primary's `hooks.log`, and `[[ ! -e "$tmp/nested-wt/metasystem/bin"
  && ! -e "$tmp/nested-wt/metasystem/artifacts" && ! -e
  "$tmp/nested-wt/artifacts" ]]`. Block-once state lives in the primary,
  but the session id is fresh, so the block asserts cleanly. `up`
  failures in this case (the fixture world is unenrolled) surface as the
  disclosed `arming failed` line exactly as `stop-hook-monitor` already
  tolerates (`supervision-fixtures.sh:1546-1548`); the assertions here are
  only the block reason, the evidence location, and the absence of
  sandbox writes.
- **Case 5, no world** — a separate root holding copies of the hook,
  adapters, engine, and fake conf, with no git repository and no template
  marker: the hook exits 0 with empty output and creates no `artifacts/`
  anywhere under it. Under this revision the exit happens at the
  identification stage — the inverted fallback burden observed directly.
  (The existing `idle-hook` scenario at lines 1382-1400 stays the
  *governed*-idle coverage; this is the ungoverned case.)
- **Case 6, candidate is not evidence (SHR-R2-INSTALL-01 pin)** — copy
  the hook and adapters (no engine) to
  `$scope/development/sub/scripts/agents/`, and create a terminal
  symbolic link `$scope/development/sub/scripts/agents/hook-link.sh`
  pointing at the real `$scope/metasystem/scripts/agents/
  supervision-hook.sh`. Fire each on `stop` with fresh sessions, **twice:
  once with `METASYSTEM_BIN` unset, once with
  `METASYSTEM_BIN=$scope/metasystem/bin/metasystem` — the governed world's
  own real engine, the strongest legitimate override
  (SHR-R4-COPIED-HOOK-OVERRIDE-01 re-pin)**. All four firings resolve a
  candidate inside the governed repository with no engine at
  `<candidate>/bin/metasystem`, and all four must emit exactly the
  missing-engine block (`"decision":"block"` and `engine missing`), exit
  0, and write nothing: no `artifacts/` under `$scope/development`, and
  no new bytes in the world's `hooks.log` or
  `stop-refusals/` from these firings. Under revision 4 the two override
  firings would have passed the shape gate, adopted `$scope` by pathname
  through the substituted engine, and blocked quoting the world's
  sentinel — so those two fail before the fold and pass after. A copied
  or symlinked hook never adopts a world by pathname, override or not.
- **Case 7, engine skew (SHR-R2-ENGINE-SKEW-01 pin; the old-engine
  fixture of SHR-R4-FAIL-CLOSED-REGRESSION-01)** — a separate flat root
  `skew_root` built like `stop_root` (hook, adapters, fake conf, sentinel
  plans, `git init`), except `bin/metasystem` is a stub executable script
  that prints the fixture's runtime name for `runtime list` and exits 2
  for every other argument — modeling an engine built before this verb
  existed. Fire `stop` with a fresh session; assert exit 0, an output
  containing `"decision":"block"` and `does not answer path state-root`
  exactly once, no `HEALTH unknown` line, and no `hooks.log` entry (the
  trail needs a working engine; the block IS the visibility). The parent
  has a validator here (the stub is executable), and validates the
  literal as an ordinary block; the fixture's `HEALTH unknown` negative
  is the same assertion the shipped missing-engine fixture makes at
  `supervision-hook-fixtures.sh:152-153`, which stays as it is.
  **Re-specified by revision 6 (SHR-R5-SKEW-FIXTURE-VALIDATOR-01).** Round
  5 showed the exit-2-for-everything stub cannot prove the block: the
  parent's validator is that stub, its six `json get`/`json strip` calls
  (today 160-171) all exit 2, validation fails, and the parent replaces
  the literal with its generic block, so `does not answer path state-root`
  never reaches the output. The stub must model what an engine built
  before this verb actually does, which is to answer every verb it has
  and refuse the one it lacks. **The stub `skew_root/bin/metasystem` is a
  script that, when its first two arguments are `path` and `state-root`,
  prints `metasystem path: unknown verb "state-root"` to stderr and exits
  2 — the exact observed behavior of the pre-fix binary (Decision 2) —
  and otherwise `exec`s the fixture harness's real engine `$ms`
  (`supervision-fixtures.sh:120`).** What the fixture then observes, and
  why it discriminates: the worker passes both executable tests (the stub
  is executable and is the canonical engine; no override is set), stages
  its payload through the stub's forwarded `runtime list` and `json get`,
  calls `path state-root`, gets exit 2, and prints the skew literal; the
  parent has `deadline_engine` (the stub), parses the literal through the
  forwarded `json get` and `json strip`, finds decision `block`, a string
  reason, no `systemMessage`, and passes the worker's bytes through
  unchanged. Assertions: exit 0; `"decision":"block"` and `does not
  answer path state-root` each exactly once in the output; no `HEALTH
  unknown` line; no `could not prove that stopping is safe` (the generic
  block's text, which is what the exit-2 stub produced); no
  `hooks.log` under `skew_root`; and no `stop-refusals/` under
  `skew_root` (the parent's resolver got exit 2 from the stub for its own
  state-root query and so wrote nothing). Against today's hook the same
  bed runs a full turn through the forwarded engine — the hook never calls
  `path state-root` — and ends with an ordinary verdict quoting
  `skew_root`'s sentinel; that output contains neither `does not answer
  path state-root` nor a block-shaped refusal of engine drift, so the
  case fails before the fold and passes after. Revision 5's self-grade
  (k) proposed a second firing with `METASYSTEM_BIN` pointing at a
  non-executable file to reach the no-validator fallback with the skew
  literal; that firing fails the double executable test and produces the
  missing-engine literal, as the critic showed, and is withdrawn — the
  fallback no longer accepts the skew literal (Decision 1), so there is
  nothing left to pin there.
- **Case 8, session `nested-worktree-steered` (SHR-R3-GIT-STEERING-01
  pin)** — case 4 repeated with the exact steering the critic used
  exported into the hook's environment: `GIT_DIR=$scope/.git
  GIT_WORK_TREE=$tmp/nested-wt`. Fire the worktree's own tracked hook copy
  with a fresh session and the same payload cwd as case 4. Assert exactly
  case 4's outcomes: `"decision":"block"` quoting `recover the primary
  sentinel`, evidence appended to the primary's `hooks.log`, and no `bin/`
  or `artifacts/` under `$tmp/nested-wt` — plus the negative that the
  output contains no missing-engine line. Without the scrub the
  identification query returns two equal paths, the mapper keeps the
  engine-less sandbox as the world, and the turn ends on the
  missing-engine block quoting no sentinel, so this case fails before the
  fold and passes after. The two variables are the critic's reproduction;
  the full list is pinned by being a copy of the Go authority's, not by
  enumerating twenty firings. **Census-scope assertion
  (SHR-R4-UP-GIT-STEERING-01):** in the same environment, run
  `"$scope/metasystem/bin/metasystem" up --metasystem-root
  "$scope/metasystem" --repo "$scope/metasystem" --print-scheduler-entry`
  and assert the printed entry's `--repo` value is `$scope` — the primary
  wrapper — and not `$tmp/nested-wt`. The printer runs after the scope
  query and before any enrollment check or write (`up.go:109,135-137`;
  `up.go:611-614` embeds `options.Scope` as the `--repo` value), so this
  pins the census scope without enrolling the fixture world. Before the
  fold the steered query returns the delegate worktree (the critic's
  read-only probe, reproduced); after it, the primary wrapper.
- **Case 9, session `nested-override` (SHR-R3-ENGINE-INSTALLATION-PAIR-01
  pin)** — write `$tmp/pair-engine`, a wrapper in the exact shape of the
  killed-attempt fixture's engine (`supervision-hook-fixtures.sh:356-375`)
  that `exec`s the fixture harness's own engine `$ms`
  (`supervision-fixtures.sh:120`), whose physical installation is never
  `$scope/metasystem`. Fire case 1's payload through
  `$scope/metasystem/scripts/agents/supervision-hook.sh` with
  `METASYSTEM_BIN=$tmp/pair-engine` and a fresh session. Assert
  `"decision":"block"` quoting `dispatch the nested runner` and evidence
  appended to `$scope/metasystem/artifacts/agents/supervision/hooks.log`.
  Under revision 3's self-anchored verb the override engine would have
  answered for its own checkout, the block reason could not have quoted
  the fixture sentinel, and the evidence would have left the fixture bed;
  under revision 4 the override changes the engine and the installation
  stays `$scope/metasystem`. The existing killed-attempt fixture remains
  the standing regression pin for the flat layout under an override.
- **Cases 10-13, the deadline parent in an engine-less worktree
  (SHR-R4-DEADLINE-PARENT-01 pins)** — all four fire the worktree's own
  tracked hook copy `$tmp/nested-wt/metasystem/scripts/agents/
  supervision-hook.sh` on `stop` with fresh sessions, through the real
  parent (no `METASYSTEM_STOP_DEADLINE_PARENT` in the environment), with
  the primary's sentinel at `recover the primary sentinel` as in case 4:
  - **Case 10, `nested-wt-parent`, normal completion**: assert
    `"decision":"block"` quoting `recover the primary sentinel` in the
    PARENT's stdout, and the negative that the output does not contain
    `could not prove that stopping is safe` — the generic block the
    shipped parent substitutes when it has no validator (HEAD 172-177).
    Before the fold the parent has no validator in the sandbox and
    replaces the worker's verdict with that generic block, so this case
    fails before and passes after.
  - **Case 11, `nested-wt-parent-timeout`, timeout**: fire under
    `METASYSTEM_BIN=$tmp/wt-deadline-engine`, a wrapper in the exact shape
    of the deadline fixture's engine (`supervision-hook-fixtures.sh:
    405-413`: sleeps 4.5 s on `runtime list`, then `exec`s the primary's
    `$scope/metasystem/bin/metasystem`). Assert exit 0, elapsed under
    five seconds, `"decision":"block"` and `deadline expired before a safe
    turn verdict` in the output, and the refusal record at
    `$scope/metasystem/artifacts/agents/supervision/stop-refusals/
    nested-wt-parent-timeout.json` with `sessionId` equal to the session
    — plus `[[ ! -e "$tmp/nested-wt/metasystem/artifacts" && ! -e
    "$tmp/nested-wt/artifacts" && ! -e "$scope/artifacts" ]]`. Before the
    fold the parent has no canonical engine in the sandbox and emits the
    record-failure allow line, or records under the cwd-derived root; so
    this case fails before and passes after.
  - **Case 12, `nested-wt-parent-steered`**: case 10 repeated with case
    8's steering exported (`GIT_DIR=$scope/.git
    GIT_WORK_TREE=$tmp/nested-wt`); same assertions as case 10.
  - **Case 13, `nested-wt-parent-steered-timeout`**: case 11 repeated
    with the same steering; same assertions as case 11, including the
    record location. Steering would have sent the cwd-derived record root
    to the worktree; the state-root verb, computed for the mapped
    installation under the scrub, sends it to the primary.

  The critic named four fixtures — normal completion and timeout, under
  git steering and under `METASYSTEM_BIN` — and these are they: 10 and 12
  are completion without and with steering, 11 and 13 are timeout under
  `METASYSTEM_BIN` without and with steering. Case 11's override engine is
  itself the `METASYSTEM_BIN` leg for completion's counterpart: the
  parent's validator is the override, its canonical engine and record
  root are the primary's.

  **Cases 11 and 13 re-pinned by revision 6
  (SHR-R5-DEADLINE-OVERRIDE-ENGINE-SPLIT-01).** As written above, the
  override merely forwards to the primary's own compatible engine, so a
  parent that asked the canonical engine for the state root would get the
  same answer and the split could not show. The two timeout cases now run
  LAST in the `nested-root` scenario, after every other case, and just
  before them the fixture replaces the primary's engine: `cp` the case-7
  stub over `$scope/metasystem/bin/metasystem`, so the installation's own
  engine is one that answers every verb but refuses `path state-root`
  with exit 2 — the fleet's post-landing state, an old binary at the
  enrolled path. `$tmp/wt-deadline-engine` keeps the deadline fixture's
  shape (`supervision-hook-fixtures.sh:405-413`: sleep 4.5 s on `runtime
  list`) but `exec`s the fixture harness's real engine `$ms`, a compatible
  engine whose physical installation is never `$scope/metasystem` (the
  case-9 pairing condition, now on the parent). What each half does: the
  worker's two executable tests pass (the stub is executable at the
  mapped primary; the override is executable), its `runtime list` through
  the override sleeps past the deadline, and the parent kills it; the
  parent's `deadline_engine` is the override, its resolver subshell parses
  the session and asks `path state-root "$scope/metasystem"` through the
  override, gets `$scope/metasystem` (template mode) inside the worker's
  wait, and on timeout writes the record through the override. Assertions
  are case 11's and 13's as stated — exit 0, elapsed under five seconds,
  `"decision":"block"` and `deadline expired before a safe turn verdict`,
  the record at `$scope/metasystem/artifacts/agents/supervision/
  stop-refusals/<session>.json` with `sessionId` equal to the session, and
  no `artifacts/` under `$tmp/nested-wt/metasystem`, `$tmp/nested-wt`, or
  `$scope` — plus one negative: the output does not contain `could not
  update the stop-refusal record` (the record-failure allow line, today's
  line 234). Under revision 5 the parent asked the stub for the state
  root, got exit 2, left `deadline_record` empty, and emitted exactly that
  allow line with no record anywhere, so both cases fail before the fold
  and pass after. Cases 10 and 12 (completion) are unchanged and run
  before the engine swap; they already exercise the override as the
  parent's validator, and their outcome does not depend on which engine
  derives the state root, which is why they could not carry this pin.
  After case 13 nothing in the scenario fires the primary's engine again;
  the stub is left in place.

- **Case 14, session `nested-freshness` — hook-freshness alive on a nested
  checkout (revision 6; this goal's DONE).** Fired in the `nested-root`
  scenario immediately after case 3, before any worktree case and long
  before the case-11 engine swap, through
  `$scope/metasystem/scripts/agents/supervision-hook.sh` on `stop` with
  payload cwd `$scope/development/sub` (case 1's payload, fresh session).
  It pins the carry-over member A's code critique recorded: after member
  A the hook still writes the steward component record with the Git
  toplevel (`steward hook-attempt --repo "$repo"`, today's line 337, where
  `$repo` is today's cwd toplevel), while health reads that record under
  the steward's own root — `checkHookFreshnessAt` calls
  `loadComponentEvidenceForHealth(repoRoot, "supervision-hook")`
  (`internal/steward/health.go:381-383`), which opens
  `ComponentEvidencePath(repoRoot, ...)` =
  `<repoRoot>/artifacts/agents/steward/components/supervision-hook.json`
  (`internal/steward/component_evidence.go:93-95,397-403`), and the
  runner's `repoRoot` is the `--repo` `up` armed it with, the state world
  (`steward run --repo`, `cmd/metasystem/steward_verbs.go:470-481`;
  `RunTick` → `ObserveHealth(repoRoot)`, `internal/steward/tick.go:271`).
  Nothing in Go reads `hooks.log`, so member A's trail move could not
  revive the role; only the sweep can. Assertions, in order: the firing
  exits 0 and blocks quoting `dispatch the nested runner` (the turn ran to
  its verdict); the record exists at
  `$scope/metasystem/artifacts/agents/steward/components/supervision-hook.json`
  with `result` `OK` and `outcome` `EMITTED` (read through `json get
  --file`; `emit_stop_payload` completes every emitted response that way,
  today's lines 517-519, block or not); `[[ ! -e
  "$scope/artifacts/agents/steward" ]]` (no record at the wrapper
  toplevel, the misdirected write observed live); and the health line is
  read exactly where the steward would read it — `world=$(
  "$scope/metasystem/bin/metasystem" path state-root "$scope/metasystem")`,
  then `"$scope/metasystem/bin/metasystem" health --repo "$world"
  --metasystem-root "$scope/metasystem"` — asserting the printed line
  contains `hook-freshness=alive` and not `hook-freshness=dead`
  (`HealthVerdict.Line`, `health.go:176-193`, renders each role as
  `name=status`). The plain `health` verb, not `--hook-preview`, is the
  steward's own reader (`ObserveHealth`, `health.go:198`; the preview form
  is the hook's mid-turn view with the current attempt allowed); the
  fixture world is unenrolled, so other roles are dead and the aggregate
  is `unhealthy`, which the assertion ignores — only the one role is the
  claim. Before the fold the record lands under `$scope`, the state-world
  read finds nothing, and the role prints `hook-freshness=dead (no hook
  turn generation is recorded ...)` (`health.go:385-386`); after it the
  role is alive. This case fails before and passes after, and it is the
  first fixture anywhere that reads hook-freshness on a nested layout.

**The consumer sweep (revision 6).** The sweep is the list of every hook
site whose root or engine argument changes, by today's line, so the
implementer edits by list and the critic checks by list. Three names go
in: `$world_installation` (the mapped installation), `$repo` (the
engine's state-root answer for it), and `$ms` (the one engine). Three
names go out: `$harness_root` as an argument, `$state_root`, and `$cwd`.

| Today's line | Site | After the sweep |
| --- | --- | --- |
| 244-252 | `harness_root`, `ms`, the single `-x` test | the candidate/mapping/engine sequence of Decision 1 (`hook_world_installation`, `canonical`, `ms`, the double test) |
| 276-292 | payload cwd, `runtime session-env`, `git rev-parse --show-toplevel` | deleted; `repo=$("$ms" path state-root "$world_installation")` with Decision 2's three-way exit map |
| 293-306 | the shell template-marker block, `state_root` | deleted (Decision 1, compiled-authority subsection) |
| 337 | `steward hook-attempt --repo "$repo"` | unchanged text, new value: `$repo` is the state world — **the hook-freshness site; case 14 is its proof** |
| 357 | `proc find-ancestor --repo "$harness_root"` (member A) | `--repo "$world_installation"`: the adapters belong to the installation, and in a mapped worktree that is the primary, where the adapters and the engine are |
| 377, 391 | `lease classify --root "$state_root" --metasystem-root "$harness_root"` | `--root "$repo" --metasystem-root "$world_installation"` |
| 433-435, 439-440 | `up --metasystem-root "$harness_root" --repo "$repo"` | `--metasystem-root "$world_installation" --repo "$repo"`; the re-arm notice keyed at 442-445 and folded into `checkin_tail` at 474-475 reads this call's aggregate line and needs no edit |
| 451 | `health --hook-preview --repo "$repo" --metasystem-root "$harness_root"` | `--repo "$repo" --metasystem-root "$world_installation"`: the preview reads the record case 14 asserts, at the root it was written under |
| 457, 512 | `steward digest-pending` / `digest-advance --repo "$repo"` | unchanged text, new value |
| 485-488 | trail directory in `emit_stop_payload`, `$state_root` | `"$repo/artifacts/agents/supervision"` |
| 493, 499, 505, 517 | `steward hook-complete --repo "$repo"` | unchanged text, new value; the completion that makes case 14's record `OK`/`EMITTED` |
| 545 | `stop_refusal_record="$repo/..."` | unchanged text, new value: the worker's refusal record and the parent's (`deadline_record`) now share one root |
| 550, 584, 590, 687 | `lease protocol-growth` / `protocol-advance` / `renew --root "$state_root"` | `--root "$repo"` |
| 600 | `supervise watchdog-report --repo "$repo"` | unchanged text, new value |
| 613-616 | trail directory before the collector, `mkdir -p`, `"$script_dir/evidence-gc.sh"` | `"$repo/artifacts/agents/supervision"`, `mkdir -p ... 2>/dev/null \|\| true` (Decision 2), `"$world_installation/scripts/agents/evidence-gc.sh"` |
| 630 | `report turn-verdict --root "$state_root"` | `--root "$repo"` |
| 647-648, 674-675 | verdict lines appended to `hooks.log` under `$supervision_dir` | unchanged text; the directory is `$repo`'s |
| 696 | `steward pending --repo "$repo"` | unchanged text, new value |
| 701 | `session end --root "$state_root"` | `--root "$repo"` |
| 711-713, 724-726 | `up --metasystem-root "$harness_root" --repo "$repo"` on end and start | `--metasystem-root "$world_installation" --repo "$repo"`; the start path's re-arm notice at 730-742 reads this call's aggregate line and needs no edit |
| parent 44-47 | `deadline_harness_root`, `deadline_validator`, `deadline_canonical` | `deadline_installation=$(hook_world_installation)`, `deadline_canonical`, `deadline_engine` (Decision 1, parent subsection) |
| parent 67-75 | resolver subshell, `json get` of `session_id` and `cwd` via `deadline_canonical` | via `deadline_engine`; the second line is `path state-root "$deadline_installation"` |
| parent 77, 84-86, 94 | `deadline_cwd`, its `sed`, the git toplevel query, the synchronous `deadline_resolve_record` | deleted |
| parent 95-109 | `deadline_log_stop_outcome` and its marker test (member A) | `"$deadline_repo/artifacts/agents/supervision"`, no marker test |
| parent 110-123 | `deadline_capture_engine_coordinates`, second line into `deadline_cwd` | second line into `deadline_repo`, then `deadline_resolve_record` |
| parent 159-171 | validation via `deadline_validator` | via `deadline_engine` |
| parent 189 | the no-validator fallback | unchanged |
| parent 224 | `report stop-block` via `deadline_canonical` | via `deadline_engine` |

The sweep leaves `$script_dir` one use, the `dirname` at line 244 that
seeds the candidate, and leaves `$harness_root` only as the candidate the
mapper starts from. `grep -n 'harness_root\|state_root\|deadline_cwd\|
deadline_validator\|deadline_harness_root' supervision-hook.sh` after the
edit shows `harness_root` inside `hook_world_installation` and nowhere
else.

Two fixture-file facts since revision 5, so the implementer does not
reconcile them silently: member A's landing added a SessionStart
ancestry bed named `$tmp/nested-root` to `supervision-hook-fixtures.sh`
(today's lines 542-588), which fires `fake start` through a nested
installation and asserts identification and arming; it is not the
`nested-root` Stop scenario of this Decision, which lives in
`supervision-fixtures.sh`, and neither replaces the other. And the
operator-layout scenario (`supervision-fixtures.sh:774-930`) asserts
`[[ ! -e "$operator_harness/artifacts" ]]` at line 902, BEFORE its hook
Stop at 912-917; **the same assertion is repeated immediately after the
Stop**, before the shutdown at 923, so the vendored-operator half of
member A's trail rule is proven where the trail is actually written. It
holds under revision 6 because the hook's world there is `$operator_scope`
(adopted mode) and every write of the turn names it; the collector's
lease gate, which is the one hook-reachable step rooted at the vendored
installation, is read-only (`lease.RequireHolderAt` →
`ClassifyAt` and `loadLease`, `internal/lease/verbs.go:359-373`,
`lease.go:76-101`; no directory is created on that path) and refuses
without writing when the fixture's caller holds no lease.

Extension to the existing `stop-hook-monitor` scenario — **flat, deep
firing, session `t-deep`**: immediately after the block-once replay
assertion (line 1555), one more payload with `session_id` `t-deep` and cwd
`$stop_root/scripts/agents`, asserting the same block quoting `dispatch
the runner`. It must run before the scenario settles the plans to `Next
step: none` (line 1558), and its distinct session is what lets it assert a
fresh block.

## Decision 4 — blast: every consumer of `$repo` in the hook (folds SHR-R2-CONSUMER-01)

Line numbers at commit 5aad591f (unchanged from revision 2's commit for
this file). On the flat layout the resolved value is byte-identical to
today's, so every row is a no-op there. "Fleet" means the template-nested
layout; "worktree" means the mapped-primary case.

| Line | Consumer | Behavior under the new resolution |
| --- | --- | --- |
| 23-31 | engine resolution `$ms` | **New row (SHR-R2-WORKTREE-ENGINE-01).** The engine resolves at `$world_installation`, after the mapping. Non-mapped layouts: byte-identical to today (`$world_installation == $harness_root`). Worktree: the primary's engine runs the whole turn — the sandbox ships none, so revision 2's harness-anchored engine claim is withdrawn. `METASYSTEM_BIN` replaces the engine only (revision 4): `$world_installation`, every flag riding it, and the verb's argument are untouched by the override. |
| 50-66 | cwd resolution and toplevel derivation | Replaced by the Decision 1 verb call; payload cwd and the session-env fallback are deleted. `$repo` is now the engine's `RootForInstallation` answer for `$world_installation`, physically normalized — the same derivation `up` applies to the same bytes (revision 4). |
| 92 | `steward hook-attempt --repo` | Takes the world directly from the flag (`cmd/metasystem/steward_verbs.go:114-123`). Fleet: attempt evidence lands beside the enrolled steward state under `metasystem/artifacts/`, so `generation`/`attemptSeq` resolve and hook-freshness revives — the goal's DONE condition. Worktree: lands in the primary trail; a write-denied sandbox degrades to the disclosed `hook_evidence_failure`. |
| 109 | `proc find-ancestor --repo` | Reads runtime adapters beneath the flag root; the wrapper root has no `scripts/agents/adapters/`, so fleet identity resolution was structurally empty. Now reads the real adapters. |
| 122, 131 | `lease classify --root "$repo" --metasystem-root "$world_installation"` | Fleet: root and metasystem-root now both name `metasystem/`, where `up`-armed sessions actually write announcements and leases (observed live). Worktree: both name the primary. Non-worktree firings pass a byte-identical metasystem-root to today's. |
| 148-155, 342, 348 (HEAD 413-420, 677-679, 690-692) | `up --metasystem-root "$world_installation" --repo "$repo"` | `up`'s state world is `stateroot.RootForInstallation(--metasystem-root)` and is independent of `--repo` (`up.go:104-113,139-144`); `--repo` sets the census scope as its git toplevel (`up.go:42-49,109,130`). **Revision 5 (SHR-R4-UP-GIT-STEERING-01):** that scope query ran git with the inherited environment, so the round-three steering (`GIT_DIR` at the primary, `GIT_WORK_TREE` at a worktree) moved the scope — and with it the census fingerprint, the owner scope and the owner tag prefix (`up.go:336-344,597`) — to the worktree while the state world stayed primary: a mapped turn could write primary state while re-arming it for a sandbox census scope. The claim that the query "never selects the state world" and is out of scope is withdrawn. `upRepositoryScope` now runs under the compiled authority's scrub: `stateroot.go` exports `RepositoryTop(path string) (string, error)`, a one-line wrapper returning `repositoryTop(path)` (the existing scrubbed query at `stateroot.go:42-50`; the private variable stays, so its test substitution at `stateroot_test.go:39,114,128,147` and `owner_test.go:37,111` is untouched), and `upRepositoryScope` becomes `top, err := stateroot.RepositoryTop(supplied)`, the same `--repo is not inside a git repository` error on failure, then `canonicalPath(top)`. One scrub list, one git-query implementation in the engine. Why the census scope may still differ from the state world, and what pins it: in template mode the state world is `<wrapper>/metasystem` while the scope is the wrapper toplevel — whole-repository process coverage, by design — and the scope is now exactly `RepositoryTop($repo)` under the scrub, a function of the state-root bytes alone, so the pair (state world, scope) is determined by `$world_installation` and nothing inherited. Pinned by a Go test beside `cmd/metasystem/up_test.go:36` (create a repository with a commit and a linked worktree, `t.Setenv` the two steering variables at the primary `.git` and the worktree, call `upRepositoryScope(primary)`, assert the primary toplevel) and by case 8's scheduler-entry assertion. On the fleet nothing observable changes: the scope stays the wrapper toplevel because that is the git toplevel of `metasystem/` too. Worktree: both flags point at the primary; `up` verifies the already-armed rings, a delegate session gains at most advisor standing (the up contract: a second live session receives advisor, without displacement), and a sandboxed failure surfaces as the non-fatal `up_failure` line. |
| HEAD 44-47 | parent: `deadline_harness_root`, `deadline_validator`, `deadline_canonical` | **New row (SHR-R4-DEADLINE-PARENT-01).** Replaced by `deadline_installation=$(hook_world_installation)` while the worker runs, `deadline_canonical=$deadline_installation/bin/metasystem`, `deadline_validator="${METASYSTEM_BIN:-$deadline_canonical}"`, both empty on identification failure. Non-mapped layouts: byte-identical values to today. Worktree: both name the primary's engine, so the parent can validate the worker's ordinary verdict and can write a record. Override: the validator is the override engine, the canonical engine is the installation's own — the pairing rule applied to the parent. **Revision 6 (SHR-R5-DEADLINE-OVERRIDE-ENGINE-SPLIT-01):** `deadline_validator` becomes `deadline_engine`, set only when both executables exist, and it is the ONLY engine the parent runs — session parse, state root, validation, record. `deadline_canonical` is the executable test and nothing more. |
| HEAD 67-75 (today 67-75) | parent: resolver subshell `"$deadline_canonical" json get ... session_id / cwd` | Keeps the session line; the cwd line and the second resolution line are dropped with `deadline_cwd`. **Revision 6:** the subshell runs on `deadline_engine`, and its second line is the engine's `path state-root "$deadline_installation"` answer, written even when the session parse failed; the deadline kills the subshell as today, so a slow engine costs the record, never the parent's reserve. |
| HEAD 76-77, 82-94 (today 76-94) | parent: `deadline_cwd` and `deadline_resolve_record` via `git -C "$deadline_cwd" rev-parse --show-toplevel` | The unscrubbed cwd-derived root the consumer table missed. Replaced: `deadline_repo=$("$deadline_canonical" path state-root "$deadline_installation" 2>/dev/null) \|\| return 1`, then the shipped normalization and slug. Fleet: the refusal record moves from the wrapper root's stray `artifacts/` to `<wrapper>/metasystem/artifacts/agents/supervision/stop-refusals/`, beside the attempt evidence. Worktree: the primary's. Payload cwd no longer participates anywhere in the hook. **Revision 6:** the synchronous query is withdrawn; `deadline_resolve_record` takes `deadline_repo` from the harvested subshell line and does only normalization, slug, and path; the call at today's line 94 is deleted. |
| today 95-109 | parent: `deadline_log_stop_outcome` with member A's marker test | **New row (revision 6, SHR-R5-TEMPLATE-MARKER-SECOND-AUTHORITY-01).** The marker test (99-103) and `supervision_root` are deleted; the trail line lands at `"$deadline_repo/artifacts/agents/supervision/hooks.log"`, the state world by construction, in both layouts. |
| today 110-123 | parent: `deadline_capture_engine_coordinates` | **New row (revision 6).** The second harvested line sets `deadline_repo`, not `deadline_cwd`; the rest is as shipped. |
| HEAD 144-177 (today 146-201) | parent: validation of the worker's JSON | Unchanged predicate; the validator is now the mapped installation's (or override) engine. The no-validator fallback (172-175) accepts both the missing-engine and the skew literal. **Revision 6:** the validator is `deadline_engine`; the fallback (today 187-190) is as shipped and accepts only the missing-engine literal; the skew literal validates through the structured path (Decision 2). |
| HEAD 203-217 (today 203-239) | parent: timeout record through `"$deadline_canonical" report stop-block --refusal-record` | Writes under the state-root world; an empty `deadline_record` keeps the shipped record-failure allow line. **Revision 6:** through `"$deadline_engine"`; the canonical engine writes nothing in an overridden turn. |
| 161 | `health --hook-preview --repo "$repo" --metasystem-root "$world_installation"` | Health reads the same world the attempt evidence lands in; hook-freshness is computable instead of structurally dead. |
| 166, 204 | `steward digest-pending` / `digest-advance --repo` | The digest cursor advances against the real steward state rather than an empty bootstrap world. |
| 185, 191, 197, 209 | `steward hook-complete --repo` | Completion evidence lands beside the attempt record it closes. |
| 221, 244, 249, 322-324 | `lease protocol-growth` / `renew` / `protocol-advance --root` | Same lease world as classification; growth counts and renewals touch the lease that exists. |
| 256 | `supervise watchdog-report --repo` | Reads job records and supervision state where dispatch writes them. |
| 263-265, 282-283, 309-310 | `supervision_dir="$repo/artifacts/agents/supervision"`, `hooks.log` | The fired-vs-never-fired trail lands beside the armed supervision state; the stray wrapper-root `artifacts/` stops growing; line 264's `mkdir -p` gains the Decision 2 guard. |
| 265 | `"$script_dir/evidence-gc.sh"` | **New row (SHR-R2-CONSUMER-01).** The collector derives its root and its own engine from its own script location (`evidence-gc.sh:16-18`) and roots `lease require-holder`, `lease run-held`, and `evidence gc` there — revision 2 left it invoked from `$script_dir`, splitting a mapped turn between two worlds. The invocation becomes `"$world_installation/scripts/agents/evidence-gc.sh"`. Non-mapped layouts: the same file byte-for-byte (`$script_dir` is `$world_installation/scripts/agents`), so behavior is identical, including the operator-nested layout's existing collector root at the vendored installation — pre-existing, unchanged, not a new split. Worktree: the primary's collector runs against the primary's lease and evidence state; a failure still lands in the primary `hooks.log` under the existing `|| true`. Override: the collector reads `METASYSTEM_BIN` itself (`evidence-gc.sh:17`), so an overridden turn runs one engine through the collector too (revision 4). |
| 274 | `report turn-verdict --root` | The consequence specimen: the verdict reads the real `plans/goals/` ledger and stream plans (`openwork.go:23-28`), and — in the worktree case — the primary's job records, so a delegate's active job is visible in-flight work instead of a phantom idle. An idle turn-end with claimable work is refused instead of waved through blind. |
| 332 | `steward pending --repo` | Session-start incident surfacing reads the real steward's pending set. |
| today 293-306 | the shell template-marker block: `state_root`, `template_marker`, `template_installation` | **New row (revision 6, SHR-R5-TEMPLATE-MARKER-SECOND-AUTHORITY-01).** Deleted. Every `--root "$state_root"` (377, 391, 550, 584, 590, 630, 687, 701) and both trail directories (485, 613) name `$repo`. Fleet: identical values, because the shell block's template answer and the engine's coincide there. The two layouts the block got wrong (an adopted installation under a marker-carrying repository; a `.local`-only template installation) go from a Stop refused by pathname to the engine's answer. |
| today 442-445, 474-475, 730-742 | the re-arm notice keyed from `up`'s aggregate line (landed today, ea8c3ead7) | **New row (revision 6).** Not consumers of `$repo`; they read the last line of the `up` call beside them. Under the sweep those `up` calls name `$world_installation` and `$repo`, so the notice describes the arming of the one world the turn runs in; the notice rides `checkin_tail` into the Stop response's `systemMessage` on the worker path, which the parent's structured validation admits beside a block (today 181-182) or alone (183-184), and rides `surface_json` on the start path. No edit; consistency stated so the blast table matches the file. |

Not consumers of `$repo`, unchanged: the session and tag plumbing.

**The one-world claim for a mapped linked worktree, redone
(SHR-R2-CONSUMER-01):** in the mapped case every moving part of the turn
names the primary — the engine binary executing every verb
(`$world_installation/bin/metasystem`), every `--repo`, `--root`, and
`--metasystem-root` flag, the evidence trail (`hooks.log`), and the
invoked evidence collector with its lease and garbage-collection roots.
Nothing in the turn reads or writes sandbox state; the sandbox contributes
only the hook bytes that started it. Decision 3 case 4 asserts the
positive half (primary block reason, primary evidence) and the negative
half (no `bin/`, no `artifacts/` materializing in the sandbox); case 8
asserts the same under inherited git steering, and case 9 asserts that an
engine override keeps every part of the turn on the one installation.

Existing-fixture blast, retraced against this revision:

- `make_repo` roots (`gate_repo`, `$tmp/repo`), `stop_root`, `idle_repo`:
  flat adopted layouts, each its own git toplevel, each staging its engine
  by `cp` into `<root>/bin/metasystem` — the engine's executable-anchored
  answer is the root itself; resolution is byte-identical to today's,
  including `stop-hook-monitor`'s expected `ENROLLMENT_DRIFT` line
  (1546-1548), because `up`'s state world for the never-enrolled
  `stop_root` is unchanged.
- `operator-layout` (`supervision-fixtures.sh:586-738`): the fixture
  copies the shipped tree into `<scope>/metasystem` only (lines 599-604)
  and stages the engine at `<scope>/metasystem/bin/metasystem` (line
  612), so the scope carries no `development/metasystem-design.md` and
  the engine answers adopted-mode: the scope toplevel — the same value
  today's `git -C <harness> rev-parse --show-toplevel` produces for the
  hook firing at lines 723-725. Every consumer behaves byte-identically,
  and every existing assertion (census state at the scope, line 711; no
  split state beneath the vendored installation, lines 713 and 736)
  survives. The scenario keeps covering the adopted-nested flavor; the
  new `nested-root` scenario covers the template-nested flavor the fleet
  actually runs.

Existing-fixture blast of the parent fold, traced (revision 5): the
shipped deadline fixture (`supervision-hook-fixtures.sh:414-442`) fires
the checkout's own hook `$hook` with payload cwd `$line_root`, and its
refusal record today lands under `$line_root` because the cwd chose it;
under revision 5 the record root is the firing installation's state
world, so both firings move to `$line_root/scripts/agents/
supervision-hook.sh` (staged at line 161, whose engine at line 164 is the
parent's canonical engine) and every assertion (block, `deadline expired
before a safe turn verdict`, `occurrence 2` on the second firing) holds
unchanged there. The membership firing at lines 128-130 fires `$hook`
with cwd `/` and today ends silently at the toplevel query (HEAD 273);
with cwd inert it would resolve the checkout's own state world and run a
full turn against it from inside a fixture, so it moves to the same
`$line_root` copy — that firing's only assertion is the exit status, which
is unchanged. No fixture may fire the checkout's own hook on `stop` once
cwd is inert; the implementer greps `bash "$hook" claude stop` in that
file and re-roots every hit.

Out of scope, stated so nobody fills it silently: cleaning up the
misdirected `artifacts/agents/` residue at the three seats' wrapper roots
is an operational task on the live machines; no other script's root
resolution (`dispatch.sh`, `commit.sh`, adapters — all resolve from their
own location or an explicit flag) is modified; the `runtime session-env`
engine verb, the `stateroot` package's existing semantics, and
`evidence-gc.sh`'s own internals are untouched (only the hook's invocation
path of the collector changes, and `stateroot` gains the one exported
wrapper `RepositoryTop` with no semantic change). Revision 4's clause that
`up`'s census-scope query is out of scope is withdrawn (SHR-R4-UP-GIT-
STEERING-01, Decision 4 row above). The `pathclass` package's private copy
of the scrub (`pathclass.go:381-389`) is not reached by this hook and is
not touched. Revision 6 adds to the out-of-scope list: the collector's own
root (its script location, `evidence-gc.sh:16`), which in the
vendored-operator layout is the installation while the hook's world is the
scope — pre-existing, read-only on the hook's path, and not this design's
to move; and the cleanup of the stray steward component records and
refusal records at the three seats' wrapper roots, which joins the
operational residue already listed.

### Where each round-5 finding lands

| Finding | Section that answers it | What changes | What proves it |
| --- | --- | --- | --- |
| SHR-R5-DEADLINE-OVERRIDE-ENGINE-SPLIT-01 | Decision 1, "One engine in the parent"; Decision 2 failure map (parent rows); Decision 4 parent rows | `deadline_engine` replaces `deadline_validator` and is the only engine the parent runs; the state-root query moves into the deadline-killed resolver subshell; `report stop-block` runs on it; `deadline_canonical` is the executable test only | cases 11 and 13 re-pinned: an old canonical engine at the primary (the case-7 stub) and a compatible slow override; the record lands under the primary and the allow line is absent; both fail under revision 5 |
| SHR-R5-SKEW-FIXTURE-VALIDATOR-01 | Decision 3 case 7; Decision 2 ("How the parent sees the two literals"); Decision 1 parent subsection (fallback as shipped) | the stub refuses exactly `path state-root` with exit 2 and forwards every other verb, so the parent validates the literal structurally; the fallback's skew acceptance and self-grade (k)'s second firing are withdrawn | case 7 observes the skew literal's own text once, no generic block, no trail, no record; it fails against today's hook, which never calls the verb |
| SHR-R5-TEMPLATE-MARKER-SECOND-AUTHORITY-01 | Decision 1, "The compiled authority is the only one"; header contract item (7); Decision 3 sweep; Decision 4 rows for today's 293-306 and 95-109 | the worker's marker block and the parent's marker test are deleted; `state_root` disappears; every consumer names `$repo` / `$deadline_repo`, the engine's answer | the repurposed `missing-template` fixture asserts `engine missing`, no `HEALTH unknown`, no writes; member A's template trail assertions and case 3 (self-hosting), the operator-layout after-Stop check (vendored); the case-table rows for the two layouts the shell block refused |
| carry-over: hook-freshness dead on nested checkouts | Decision 3 case 14 and the consumer sweep (line 337) | `steward hook-attempt --repo "$repo"` writes under the state world because `$repo` is that world | case 14: the record at the nested installation, none at the wrapper, and `hook-freshness=alive` from `health --repo "$(path state-root ...)"` — this goal's DONE |
| carry-over: operator-layout check before the Stop only | Decision 3, after the sweep table | the `[[ ! -e "$operator_harness/artifacts" ]]` assertion is repeated after the hook Stop | the scenario itself; traced read-only through the collector's lease gate |
| new fact: the re-arm notice (ea8c3ead7) | Decision 4 row for today's 442-445, 474-475, 730-742 | none; consistency stated | the row |

## Consistency pass

Revision 3 was re-read end to end against itself: the candidate/validation
split of Decision 1 is what the failure map of Decision 2 guards (silence
for unprovable identification, the fixed line for absent engines, the
fixed line for skewed engines, exit-1 silence for ungoverned answers), what
every fixture of Decision 3 identifies by sentinel and evidence location
(cases 4-7 each pin one round-2 finding), and what every row of Decision 4
inherits; `world_installation` appears in the resolver, the engine
resolution, the `--metasystem-root` switch list, the collector invocation,
and the corresponding blast rows with one meaning; the verb carried one
shape everywhere it was named (flag-less in revision 3; revision 4 gives
it one positional argument everywhere, see below); no surviving sentence
claims cwd participates in
resolution, that a pathname alone selects a world, that resolution ordering
is unchanged, that the worktree turn runs on a sandbox engine, that an
identification failure implies "not a worktree", or that every engine
failure is silent.

Revision 4 was re-read end to end against itself after the two folds: the
verb is named with its one positional argument in the validation contract,
the replacement block, the header contract, the failure map, and the
Decision 4 rows; `world_installation` is the bytes the verb, every
`--metasystem-root`, and the collector location receive, and the pairing
rule says so in one place; `hook_git` wraps both git calls of the
replacement block and the failure map's identification row names it; case
8 pins the scrub and case 9 pins the pairing, each with the outcome that
fails before the fold; and no surviving sentence claims the verb is
flag-less, that the engine answers from its own executable, that the world
follows the override engine's own answer, or that a mapper git call runs
with the inherited environment.

Revision 5 was re-read end to end against itself after the four folds:
the missing-engine outcome is the shipped block in the replacement block,
the header contract, the case table, the failure map, Decision 4, case 6
and the existing missing-engine fixture; the skew outcome is the
`raw_engine_skew_stop` block in the same places and in case 7; the
`canonical`-and-`ms` double test appears in the replacement block, the
provenance rule, the header contract, the failure map, and case 6's
override firings; `hook_world_installation` is the one function both the
worker and the parent call, and the parent's installation, engines,
record root, and failure rows are named in Decision 1's parent
subsection, the failure map, Decision 4's HEAD rows, and cases 10-13;
`upRepositoryScope` runs under the scrub in Decision 4's `up` row, the
withdrawn out-of-scope clause, case 8's census assertion, and the Go test;
no surviving sentence claims a missing or old engine emits a
`systemMessage`, that the parent resolves from the unmapped harness root
or from payload cwd, that `up`'s scope query is out of scope, or that an
override can govern an engine-less candidate. "Silent exit 0" in the
failure map is the worker's own behavior in every row, and the parent's
conversion of silence into its generic block on Stop is stated once and
inherited unchanged.

Revision 6 was re-read end to end against itself after the three folds and
the two carry-overs: `deadline_engine` is the parent's one engine in the
parent subsection, the header contract item (6), the failure map's five
parent rows, Decision 4's parent rows, the sweep table's parent rows, and
cases 11 and 13, and `deadline_validator` survives only inside quoted
revision-5 text that the same sentence withdraws; the state-root query
sits in the resolver subshell in every place the parent's resolution is
described, and no surviving sentence has the parent call `path
state-root` synchronously or through `deadline_canonical`; the
no-validator fallback accepts one literal in Decision 1, Decision 2, the
failure map, Decision 4, and case 7, and revision 5's widening is marked
withdrawn at both places it was stated; the case-7 stub forwards every
verb but one in Decision 3, Decision 2's skew row, the landing table, and
cases 11 and 13, which reuse it; `state_root` appears only as the name
being deleted, and every `--root` and trail site names `$repo` in the
compiled-authority subsection, the sweep table, and Decision 4's new row;
`deadline_repo` is the parent's world in the trail helper, the record
resolver, the capture function, and the failure map; case 14 names the
hook-attempt site, the Go reader, and the health token, and the sweep
table's line-337 row points back at it; the re-arm sites are read from
today's file and described once in Decision 4; the two fixture-file facts
since revision 5 (the hook-fixtures `nested-root` bed, the operator-layout
check placement) are stated where the implementer would otherwise
reconcile them; and no surviving sentence claims a shell test derives the
state world, that the parent's timeout record can be written by an engine
the worker did not run, that the fallback knows the skew literal, or that
an exit-2-for-everything stub proves the skew block.

## Self-grade

Grounding: every load-bearing claim is a file-and-line read in this
worktree at commit 5aad591f (the hook, `evidence-gc.sh`, the fixtures,
`up.go`, `stateroot.go` including `installationRoot()`'s symbolic-link
evaluation, `resolve.go`, `openwork.go`, `steward_verbs.go`,
`path_verbs.go`, `main.go:244-248`, and this worktree's own
`.claude/settings.json` hook registration), a command run here on
2026-09-02 (`git rev-parse --path-format=absolute --git-dir
--git-common-dir` inside this real linked worktree, returning
`<wrapper>/.git/worktrees/<job-id>` and `<wrapper>/.git`; `git --version`
2.39.5; the absence of `metasystem/bin` and `metasystem/artifacts` here;
`metasystem path state-root` against the installed pre-fix engine
returning exit 2 with `unknown verb` — the skew case observed on the real
binary; `grep` confirming every fixture engine is staged by `cp`), or a
directly observed live-fleet fact (the m0b wrapper root's stray
`artifacts/` versus the armed world under `metasystem/artifacts/`).
Residual risks, honestly: (a) on git older than 2.31 the hook now goes
silent everywhere rather than resolving a wrong world — declared as a
version floor, and strictly safer than revision 2's silent sandbox-local
degradation; (b) m2 and m3 are assumed to match m0b's layout — the
template marker is git-tracked, so any full checkout carries it; (c) the
skew split rests on the family dispatcher answering an unknown verb with
an exit other than 0 or 1 — verified exit 2 on the current binary and
pinned forward by fixture case 7, but not proven for arbitrarily ancient
engines beyond the top-level dispatcher argument above; (d) the
write-denied-sandbox degradation path is traced through the hook's
existing failure channels, not executed live; (e) the
`$CLAUDE_PROJECT_DIR` registration fact is claude-runtime-specific — other
runtimes may register differently, but the mechanism depends only on the
firing script's physical location, which is runtime-independent.

Revision 4 grounding: the pairing rule rests on `up.go:16-25,104-108,
139-144` (canonicalize, then `RootForInstallation` of the explicit
`--metasystem-root`), `lease.go:68-86` (`--metasystem-root` selects
adapters and configuration, `--root` the state), `evidence-gc.sh:16-18`
(collector root from its own location, engine from `METASYSTEM_BIN`
first), `stateroot.go:42-64,100-108,137-163`, `path_verbs.go:14-33`,
`main.go:248-252`, and the killed-attempt fixture at
`supervision-hook-fixtures.sh:164,176,356-389`; no hook-reachable steward
verb anchors on the executable (`stateroot.StateRoot` appears in that
file only under `steward revive`, `steward_verbs.go:414`). The scrub rests
on `stateroot.go:32-40`, `adopt.sh:53-61`, `gittree.go:61-75`, and the
probe re-run here at commit 47e59bcd with git 2.50.1: plain and scrubbed
queries return the linked-worktree pair, the steered query returns the
primary `.git` twice. Residual risks added: (f) a `bin/metasystem` that is
itself a symbolic link to another installation's binary no longer moves
the hook's world — the turn is flag-driven, and reject clause one keeps
executable-anchored writers out of it — but that engine's private
`StateRoot` writers, if ever reached, would anchor elsewhere; (g) the shell
scrub list is a copy of the Go list and can drift when the Go list changes,
and the fixture pins only the two variables that reproduced the defect;
(h) under `METASYSTEM_BIN` the operator's engine is trusted to be a
metasystem engine — an override that is not one lands in the visible skew
branch, never in a silent wrong world.

Revision 5 grounding: every HEAD citation was read in this worktree at
commit 12ed490c3 — the parent block (`supervision-hook.sh:32-222`: engines
at 44-47, worker launch at 55-57, resolver subshell 67-75, cwd-derived
record root 82-94, validation 144-177, timeout record 203-217), the
shipped fail-closed contract (4-6, 18-21, 229-233) and its fixture
(`supervision-hook-fixtures.sh:137-157`), the unscrubbed scope query
(`up.go:42-49`) against the scrubbed authority (`stateroot.go:42-64`), the
scheduler-entry printer that embeds the scope before any write
(`up.go:109,135-137,611-614`), the `up` options the scope feeds
(`up.go:336-344,597`), the test seams on `repositoryTop`, and the
existing fixtures that stage or lack an engine under an override
(`supervision-hook-fixtures.sh:7-8,164,444-462,474`). The deadline-parent
fold was traced by reading, not executed: no fixture in this worktree
runs the parent against a linked worktree yet — cases 10-13 are that
proof, and building them is the implementer's first obligation. Residual
risks added: (i) the parent and the worker call the same function at
nearly the same time, so a filesystem change between the two calls could
give them different worlds; the window is the worker's own resolution
time and the outcome is the parent's fail-closed generic block, never a
record under a foreign root; (j) the parent's timeout allow line for an
unresolvable world is the shipped shape and is inherited — a hook that
cannot name any world cannot record a refusal, and the shipped hook
already prefers that over a refusal loop; (k) the skew literal's exact
bytes are now a fixed contract like the missing-engine literal, and the
parent's fallback compares them byte-for-byte, so an edit to one without
the other breaks the no-validator path visibly (case 7 covers the
validator path, and the fallback path is covered by firing case 7's stub
world with `METASYSTEM_BIN` pointing at a non-executable file, which the
implementer adds as case 7's second firing — **withdrawn by revision 6**:
that firing fails the double executable test and yields the
missing-engine literal, and the fallback no longer accepts the skew
literal). Grade: pass against
everything observed; the reject condition below is the falsifier the
implementation and its critique must actively test.

Revision 6 grounding: every "today" citation was read in this worktree at
commit 3c753c5ef — the parent block (`supervision-hook.sh:32-240`:
engines 44-47, worker launch 55-57, resolver subshell 67-75, cwd `sed` 77,
record resolver 82-93 with the synchronous call at 94, member A's trail
helper 95-109, coordinate harvest 110-123, resolver stop 124-135,
validation 159-186, fallback 187-190, generic block 191-193, timeout
record 224-234), the worker (engine 244-252, registry 253-261, payload
263-274, cwd and toplevel 276-292, the marker block 293-306, hook-attempt
337, find-ancestor 357, lease classify 377 and 391, `up` 433-441 with the
re-arm notice 442-445 and 474-475, health 451, the trail sites 485-488 and
613-616, the refusal record 545, the `--root` sites 550/584/590/630/687/
701, start-path `up` and notice 724-742), member A's diff (43c3d3c08) and
the re-arm diff (ea8c3ead7) as landed, the Go readers the hook-freshness
carry-over names (`health.go:176-193,198,381-417`,
`component_evidence.go:93-103,166-184,397-415`,
`steward_verbs.go:44-77,116-178,221-235,470-481`, `tick.go:271`), the
unchanged authorities (`stateroot.go:32-64,100-108,137-163`,
`up.go:16-25,42-49,104-143`, `path_verbs.go:14-33`, `main.go:248-254`,
`runtime_verbs.go:29-50` for `runtime list` answering from the compiled
registry, which is why a forwarding stub can serve it), the collector
(`evidence-gc.sh:16-27`) and its lease gate
(`lease.go:76-101`, `verbs.go:359-373`), and the fixtures the folds
touch (`supervision-hook-fixtures.sh:120-157,161-176,405-413,522-540,
542-588,680-683`; `supervision-fixtures.sh:120,774-930,1920-1975`).
Nothing was executed against a built engine in this round: the round is
read-only by brief, the pre-fix skew exit was observed on a real binary in
revision 3 and the stub reproduces its bytes, and cases 7, 11, 13, and 14
and the repurposed `missing-template` fixture are specified with the
before/after outcome each must show. Residual risks added: (l) the
timeout record now depends on the engine answering `path state-root`
inside the worker's wait; a machine so loaded that one engine start plus
one git call takes four seconds loses the record and gets the disclosed
allow line, where today's git-derived root would still have named a
record path (and then handed the same slow engine the write) — the price
of having no shell authority, bounded by the fact that the same machine's
worker also could not finish, and visible in the trail as
`deadline-expired-record-failure-allow`; (m) the case-11/13 engine swap
leaves the primary's `bin/metasystem` as the stub for the rest of the
scenario, so any case added after them must be added before them or
restore the engine — stated in the case text; (n) the sweep renames ten
`--root` arguments and two trail directories by list, and a missed site
would be a `state_root` reference that `set -u` turns into an abort on
the first firing — loud, not silent, and caught by the post-edit grep the
sweep prescribes; (o) `health` without `--hook-preview` in case 14
advances the alert breaker and observation state in the fixture world
(`ObserveHealth`, `health.go:198-235`), which is disposable and never
enrolled, so the side effect is contained in the fixture bed.

**Reject condition — reject this design if any of the following is
shown:** a state-writing engine verb reachable from this hook whose world
is neither its explicit `--repo`/`--root` flag nor `RootForInstallation`
of an explicit metasystem root (an authority split the consumer table
missed); a supported layout in which neither the firing script's own
physical installation nor its mapped primary counterpart holds the
governing engine (the hook would go benign inside a governed world); any
path on which a governed decision rides a world that the turn's engine did
not compute, by `RootForInstallation`, from the same installation bytes
every `--metasystem-root` consumer of that turn receives (the
SHR-R2-INSTALL-01 recreation); any turn — under a `METASYSTEM_BIN`
override included — in which the verb's answer and any shell-owned
consumer name different installations, or in which the collector runs a
different engine than the hook (the SHR-R3-ENGINE-INSTALLATION-PAIR-01
split); any git invocation of the resolver whose result an inherited
variable from the `stateroot.go:32-40` list can change, or a scrub list
that is not name-for-name that list (the SHR-R3-GIT-STEERING-01
exposure); `up` writing supervision state at any root other than
`RootForInstallation(--metasystem-root)`; a linked worktree whose primary
counterpart at the same relative installation path is not the governing
installation (the one-step mapping would misdirect or silence it); any
identification-failure branch that proceeds as "not a worktree"
(the SHR-R2-WORKTREE-FALLBACK-01 inversion undone); any engine-failure
branch other than the verb's own exit-1 refusal that stays silent on stop
(the SHR-R2-ENGINE-SKEW-01 silence recreated); any turn in a mapped
worktree that reads or writes sandbox state, including through the
evidence collector (the SHR-R2-CONSUMER-01 split recreated); any Stop
on which the deadline parent validates with, records through, or roots a
refusal record under anything other than the engine and world the worker
computed from the same script location — a parent resolving from the
unmapped harness root or from payload cwd included (the
SHR-R4-DEADLINE-PARENT-01 split); any missing-engine or old-engine Stop
that ends without `"decision":"block"`, or any fixture weakened from that
expectation (the SHR-R4-FAIL-CLOSED-REGRESSION-01 regression); any engine
git query reachable from this turn — `up`'s census scope included — whose
answer an inherited variable from the `stateroot.go:32-40` list can change
(the SHR-R4-UP-GIT-STEERING-01 exposure); any path on which a candidate
without its own `bin/metasystem` is governed because `METASYSTEM_BIN`
supplied an engine (the SHR-R4-COPIED-HOOK-OVERRIDE-01 reopening); any
Stop on which the deadline parent asks one engine for the state root or
the refusal record and another to validate the worker, or on which any
parent engine call runs on `deadline_canonical` rather than
`deadline_engine`, or on which an engine call sits in the parent's main
flow between the worker launch and the wait loop (the
SHR-R5-DEADLINE-OVERRIDE-ENGINE-SPLIT-01 split, or its timeout exposure);
any old-engine fixture whose stub cannot answer the parent's own
validation verbs, so that the skew literal is never observed as itself
(the SHR-R5-SKEW-FIXTURE-VALIDATOR-01 gap); any surviving shell test —
marker, pathname, or configuration file — that derives, replaces, or
refuses the state world after the engine answered, in the worker or in
the parent, or any name other than `$repo` and `$deadline_repo` carrying
that world (the SHR-R5-TEMPLATE-MARKER-SECOND-AUTHORITY-01 second
authority); any nested-layout turn after which the steward component
record sits anywhere but under `path state-root`'s answer, or after
which `health --repo <that answer>` reports hook-freshness dead (the
carry-over this goal's DONE binds); or any new resolver failure path that
exits nonzero or emits anything beyond the two fixed blocking reports
(the missing-engine and skew literals) under `set -euo pipefail`.
