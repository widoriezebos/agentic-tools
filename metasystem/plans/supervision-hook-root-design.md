# Design: the supervision hook resolves the metasystem world, not the outer repository

Goal: supervision-hook-wrong-root (plans/goals/supervision-hook-wrong-root.md,
revision 6). Author: implementer delegate under dispatch by
m0b+main-1788250419-3170380-8a1fb3. **Revision 3, 2026-09-02**: folds all five
findings of records/misc/hook-root-critique-r2.md by id (SHR-R2-INSTALL-01,
SHR-R2-WORKTREE-ENGINE-01, SHR-R2-ENGINE-SKEW-01, SHR-R2-WORKTREE-FALLBACK-01,
SHR-R2-CONSUMER-01); each fold is tagged inline, and each refines the
revision-2 installation-derivation mechanism rather than replacing it. Every
seam cited below was re-read in this worktree at commit 5aad591f; the
live-fleet observations were read on the m0b checkout this design was authored
beside, and the skew and worktree probes recorded in the self-grade were run
here on 2026-09-02.

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
- **Validation**: the engine found at the candidate answers for itself.
  The verb is **`metasystem path state-root`**, registered in the existing
  `path` family beside `owner` (`cmd/metasystem/main.go:244-248`,
  `path_verbs.go`). It takes **no options** — revision 2's
  `--installation` flag is withdrawn, because a caller-supplied pathname
  was exactly the trust hole the critique named. The verb prints
  `stateroot.SelfRoot()`, a new exported wrapper composing the two
  existing private steps `RootForInstallation(installationRoot())`
  (`stateroot.go:100-108,137-155`) with no semantic change to either, and
  exits 0. On any resolution refusal (executable not at
  `<installation>/bin/metasystem` per the shape gate, or an adopted-mode
  installation outside any git repository) it prints the error to stderr
  and **exits 1** — the same refusal shape as `path owner`
  (`path_verbs.go:20-27`). It adds no state and no writes.

Because the verb answers from `os.Executable` with full symbolic-link
evaluation, the hook's world is by construction the world every
executable-anchored state writer in that same binary uses (`StateRoot`,
`stateroot.go:69-95`), and pathname games cannot move it. The three cases
the critique required the contract to distinguish are now each dispositioned:

- *Directory symbolic link on the invocation path*: normalized away by the
  candidate's `pwd -P`; the physical installation is the candidate.
  Supported.
- *Terminal symbolic link to the hook file*: the candidate is the physical
  directory holding the link, not the target. If no engine lives there, the
  hook stops benignly (visible missing-engine line on stop, Decision 2); if
  an engine does live there, that engine answers for where it physically
  is, not for where the link pointed. Either way no pathname is believed.
- *Copied or relocated hook*: identical rule. A bare copy finds no engine
  and stops benignly. A full relocated installation with its own engine IS
  an installation, and its engine answers for it. The fixture in Decision 3
  (case 6) pins the dangerous sub-case: a hook copy inside a governed
  repository does not adopt that repository's world by pathname.

`METASYSTEM_BIN` remains an explicit override of the engine, and under it
the world follows the override engine's own answer — consistent, because
every verb of that turn executes in that same binary and its
executable-anchored writers anchor the same way. Fixtures are unaffected:
every fixture stages its engine by `cp`, never by symbolic link (verified:
`supervision-fixtures.sh:475,1393,1481,1533`), so a staged engine's
physical location is the fixture world itself.

Revision 1's `metasystem.conf` marker rule remains withdrawn for the
reasons revision 2 recorded (stray-marker capture; silent rejection of a
`.local`-only template installation accepted by `stateroot.go:149-153` and
`internal/config/resolve.go:24-27,71-95`), and content markers stay out
entirely.

**Uniqueness argument, recast.** (a) There is no collision set to
tie-break: the candidate is the one physical directory the running script
is installed in, and a stray configuration file anywhere on disk never
enters the computation. (b) The output is not merely *the same function*
the state writers use — it is *the same binary* answering from its own
resolved executable location, so the hook cannot disagree with the engine
about the world even under symbolic links, copies, or relocations
(SHR-R2-INSTALL-01's residue closed). (c) Presence of `metasystem.conf` is
not part of the answer, so a `.local`-only template installation resolves
identically to a committed-conf one. (d) Every unresolvable or unprovable
input is benign exit 0, and the only non-silent degradations are the two
fixed one-line reports of Decision 2 — never a guess.

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

Normatively, the engine-resolution block (lines 23-31) becomes the
candidate/mapping/engine sequence below; the registry and payload staging
(lines 32-44) stand unchanged; the verb call replaces the deleted
payload-cwd/session-env/toplevel block (lines 50-66):

```bash
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/../.." && pwd -P)

# Candidate world: the physical installation this script sits in, mapped
# to its primary counterpart when that installation is a linked worktree.
# Identification must SUCCEED; every failure is benign exit 0, never a
# guess and never the non-worktree outcome.
world_installation=$harness_root
git_ids=$(git -C "$harness_root" rev-parse --path-format=absolute \
  --git-dir --git-common-dir 2>/dev/null) || exit 0
[[ "$git_ids" == *$'\n'* ]] || exit 0
git_dir=${git_ids%%$'\n'*}
git_common=${git_ids#*$'\n'}
[[ -n "$git_dir" && -n "$git_common" ]] || exit 0
if [[ "$git_dir" != "$git_common" ]]; then
  [[ "$(basename -- "$git_common")" == .git ]] || exit 0
  primary_top=$(cd -- "$(dirname -- "$git_common")" 2>/dev/null && pwd -P) || exit 0
  wt_top=$(git -C "$harness_root" rev-parse --show-toplevel 2>/dev/null) || exit 0
  wt_top=$(cd -- "$wt_top" 2>/dev/null && pwd -P) || exit 0
  case "$harness_root" in
    "$wt_top") rel= ;;
    "$wt_top"/*) rel=${harness_root#"$wt_top"/} ;;
    *) exit 0 ;;
  esac
  world_installation=$primary_top${rel:+/$rel}
  [[ -d "$world_installation/scripts/agents" ]] || exit 0
fi

ms="${METASYSTEM_BIN:-$world_installation/bin/metasystem}"
if [[ ! -x "$ms" ]]; then
  if [[ "$event" == stop ]]; then
    printf '%s\n' '{"systemMessage":"HEALTH unknown — hook-freshness=unknown (metasystem engine missing; reinstall or rebuild bin/metasystem)"}'
  fi
  exit 0
fi
# ... registry membership and payload staging, byte-identical to today ...

# The world is the ENGINE's answer, not the pathname's. Exit 1 is the
# verb's own refusal (ungoverned installation): silent. Any other failure
# is engine/hook skew and must be visible, or a fleet that rebuilds its
# engines daily cannot tell a skewed hook from one that never fired.
repo_rc=0
repo=$("$ms" path state-root 2>/dev/null) || repo_rc=$?
if (( repo_rc == 1 )); then
  exit 0
elif (( repo_rc != 0 )) || [[ -z "$repo" ]]; then
  if [[ "$event" == stop ]]; then
    printf '%s\n' '{"systemMessage":"HEALTH unknown — hook-freshness=unknown (engine/hook skew: this metasystem engine does not answer path state-root; rebuild bin/metasystem)"}'
  fi
  exit 0
fi
repo=$(cd -- "$repo" 2>/dev/null && pwd -P) || exit 0
```

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

Case table — every hook firing resolves to exactly one world or one
defined degradation:

| Layout | Installation (script location) | Result |
| --- | --- | --- |
| Flat adopted (make_repo fixtures, adopted repositories) | the repository root | engine at the root answers: adopted mode → its own git toplevel; byte-identical to today |
| Fleet template nested (m0b, m2, m3) | `<wrapper>/metasystem`, template marker tracked at the wrapper | engine at `<wrapper>/metasystem/bin` answers: template mode → `<wrapper>/metasystem` — the fix |
| Operator nested adopted (`operator-layout` fixture: no template marker at the scope) | `<scope>/metasystem` | engine answers: adopted mode → `<scope>` — identical to today's cwd-toplevel answer; see Decision 4 |
| Linked delegate worktree of the fleet checkout (tracked files only: no engine, no artifacts in the sandbox) | `<worktree>/metasystem` | mapped to `<primary>/metasystem` **before** engine work; the primary's engine answers: template mode → the primary world |
| Hook copy or terminal hook symlink inside some repository, no engine at the candidate | anywhere | visible missing-engine line on stop, exit 0; no world, no writes |
| Governed installation whose engine predates this design | any supported layout | visible engine/hook-skew line on stop, exit 0 (Decision 2) |
| Hook staged outside any git repository | anywhere | identification fails → silent benign exit 0 |

### Placement and the binding header contract

The resolver is a shell block in the hook plus the one read-only engine
verb. The ordering **changes** (SHR-R2-WORKTREE-ENGINE-01): identification
and mapping now precede the engine check, because they are the step that
locates the engine. The hook's binding header comment
(`supervision-hook.sh:4-15`) is rewritten in the same change to: *(1)
shell-owned syntax refusals — unchanged; (2) world identification — the
physical installation this script belongs to, mapped to its primary
counterpart when it is a linked worktree, by git common-dir identity
alone, before any engine work; every identification failure is benign exit
0, never a guess; (3) executable resolution at the identified world — a
missing engine stays the visible-on-stop benign exit 0; (4) registry
membership — an unknown runtime exits 2; (5) world validation by engine
identity — `path state-root`, the engine's executable-anchored answer: its
exit-1 refusal is benign silence, any other failure is the visible-on-stop
skew line.* The old items (4) session environment and (5) cwd resolution
are deleted with their code. The B1 guarantee ("a missing engine with an
unusable TMPDIR must still exit 0 benign") is preserved: the mapping uses
no temporary files, and payload staging still happens only after the
engine exists.

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
| `git rev-parse --path-format=absolute --git-dir --git-common-dir` | not a repository, git < 2.31, git absent, vanished directory | **silent exit 0** — identification failure proves nothing and is never treated as "not linked" |
| two-line output parse | second line missing or empty | silent exit 0 |
| common-dir basename test | not `.git` (bare or exotic layout) | silent exit 0 |
| `primary_top` / `wt_top` physical normalization | directory vanished or unreadable | each is `$(cd -- ... 2>/dev/null && pwd -P) || exit 0` |
| containment `case` | installation not at or below its worktree toplevel | silent exit 0 |
| primary counterpart shape check | no `scripts/agents` directory there | silent exit 0 |
| engine executable test at the world | no engine file | **today's visible missing-engine line on stop**, exit 0 — true absence stays benign and stays as visible as it already is |
| `path state-root`, exit 1 | the verb's own refusal: the executable-anchored installation is not a governed world (shape gate, or adopted mode outside git) | silent exit 0 |
| `path state-root`, any other nonzero exit, or exit 0 with empty stdout | verb absent from an older engine (verified exit 2), family absent, or a broken answer | **the one-line skew report on stop** (emitted by literal `printf`, engine-independent, mirroring the missing-engine line), exit 0 |
| final `repo` physical normalization | directory vanished or unreadable | silent exit 0 |
| evidence-trail `mkdir -p "$supervision_dir"` (line 264, reachable with a write-denied primary from a sandboxed worktree session) | permission denied | gains `2>/dev/null || true`; the appends that follow already carry their own guards (lines 265, 282-283, 309-310) |

A cwd outside any git repository — today's benign case at line 65 —
remains benign by construction: cwd no longer participates at all.
Downstream writes that the worktree decision points at a possibly
write-protected primary (steward attempt/complete, `up`, turn-verdict
state) flow through the hook's existing disclosed degradation channels
(`hook_evidence_failure`, `up_failure`, the degraded-verdict branch at
lines 306-320): the hook reports degraded, emits, and exits 0.

The asymmetry rule now has three legs: a too-strict resolution degrades to
today's silence; a guessed resolution recreates this defect somewhere
else; and drift in a governed world is reported in one fixed line. The
resolver introduces no new nonzero exits, and exactly **one** new output:
the skew line, on the same channel, in the same shape, and under the same
stop-only condition as the existing missing-engine line
(`supervision-hook.sh:27-29`).

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
  supervision-hook.sh`. Fire each on `stop` with fresh sessions. Both
  candidates resolve inside the governed repository, and both must emit
  exactly the visible missing-engine line, exit 0, and write nothing: no
  `artifacts/` under `$scope/development`, and no new bytes in the
  world's `hooks.log` from these firings. A copied or symlinked hook
  never adopts a world by pathname.
- **Case 7, engine skew (SHR-R2-ENGINE-SKEW-01 pin)** — a separate flat
  root `skew_root` built like `stop_root` (hook, adapters, fake conf,
  sentinel plans, `git init`), except `bin/metasystem` is a stub
  executable script that prints the fixture's runtime name for
  `runtime list` and exits 2 for every other argument — modeling an
  engine built before this verb existed. Fire `stop` with a fresh
  session; assert exit 0, an output containing the engine/hook-skew line
  exactly once, no `"decision":"block"`, and no `hooks.log` entry (the
  trail needs a working engine; the drift line IS the visibility).

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
| 23-31 | engine resolution `$ms` | **New row (SHR-R2-WORKTREE-ENGINE-01).** The engine resolves at `$world_installation`, after the mapping. Non-mapped layouts: byte-identical to today (`$world_installation == $harness_root`). Worktree: the primary's engine runs the whole turn — the sandbox ships none, so revision 2's harness-anchored engine claim is withdrawn. `METASYSTEM_BIN` still overrides. |
| 50-66 | cwd resolution and toplevel derivation | Replaced by the Decision 1 verb call; payload cwd and the session-env fallback are deleted. `$repo` is now the engine's own executable-anchored answer, physically normalized. |
| 92 | `steward hook-attempt --repo` | Takes the world directly from the flag (`cmd/metasystem/steward_verbs.go:114-123`). Fleet: attempt evidence lands beside the enrolled steward state under `metasystem/artifacts/`, so `generation`/`attemptSeq` resolve and hook-freshness revives — the goal's DONE condition. Worktree: lands in the primary trail; a write-denied sandbox degrades to the disclosed `hook_evidence_failure`. |
| 109 | `proc find-ancestor --repo` | Reads runtime adapters beneath the flag root; the wrapper root has no `scripts/agents/adapters/`, so fleet identity resolution was structurally empty. Now reads the real adapters. |
| 122, 131 | `lease classify --root "$repo" --metasystem-root "$world_installation"` | Fleet: root and metasystem-root now both name `metasystem/`, where `up`-armed sessions actually write announcements and leases (observed live). Worktree: both name the primary. Non-worktree firings pass a byte-identical metasystem-root to today's. |
| 148-155, 342, 348 | `up --metasystem-root "$world_installation" --repo "$repo"` | `up`'s state world is `stateroot.RootForInstallation(--metasystem-root)` and is independent of `--repo` (`up.go:104-113,139-144`); `--repo` only sets the census scope as its git toplevel (`up.go:42-49,109,130`). On the fleet `up` therefore already armed the correct world (template mode: the marker is tracked at the wrapper toplevel), and this change alters neither its state world nor its census scope there — the scope stays the wrapper toplevel because that is the git toplevel of `metasystem/` too, so whole-repository process coverage is preserved. Worktree: both flags point at the primary; `up` verifies the already-armed rings, a delegate session gains at most advisor standing (the up contract: a second live session receives advisor, without displacement), and a sandboxed failure surfaces as the non-fatal `up_failure` line. |
| 161 | `health --hook-preview --repo "$repo" --metasystem-root "$world_installation"` | Health reads the same world the attempt evidence lands in; hook-freshness is computable instead of structurally dead. |
| 166, 204 | `steward digest-pending` / `digest-advance --repo` | The digest cursor advances against the real steward state rather than an empty bootstrap world. |
| 185, 191, 197, 209 | `steward hook-complete --repo` | Completion evidence lands beside the attempt record it closes. |
| 221, 244, 249, 322-324 | `lease protocol-growth` / `renew` / `protocol-advance --root` | Same lease world as classification; growth counts and renewals touch the lease that exists. |
| 256 | `supervise watchdog-report --repo` | Reads job records and supervision state where dispatch writes them. |
| 263-265, 282-283, 309-310 | `supervision_dir="$repo/artifacts/agents/supervision"`, `hooks.log` | The fired-vs-never-fired trail lands beside the armed supervision state; the stray wrapper-root `artifacts/` stops growing; line 264's `mkdir -p` gains the Decision 2 guard. |
| 265 | `"$script_dir/evidence-gc.sh"` | **New row (SHR-R2-CONSUMER-01).** The collector derives its root and its own engine from its own script location (`evidence-gc.sh:16-18`) and roots `lease require-holder`, `lease run-held`, and `evidence gc` there — revision 2 left it invoked from `$script_dir`, splitting a mapped turn between two worlds. The invocation becomes `"$world_installation/scripts/agents/evidence-gc.sh"`. Non-mapped layouts: the same file byte-for-byte (`$script_dir` is `$world_installation/scripts/agents`), so behavior is identical, including the operator-nested layout's existing collector root at the vendored installation — pre-existing, unchanged, not a new split. Worktree: the primary's collector runs against the primary's lease and evidence state; a failure still lands in the primary `hooks.log` under the existing `|| true`. |
| 274 | `report turn-verdict --root` | The consequence specimen: the verdict reads the real `plans/goals/` ledger and stream plans (`openwork.go:23-28`), and — in the worktree case — the primary's job records, so a delegate's active job is visible in-flight work instead of a phantom idle. An idle turn-end with claimable work is refused instead of waved through blind. |
| 332 | `steward pending --repo` | Session-start incident surfacing reads the real steward's pending set. |

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
half (no `bin/`, no `artifacts/` materializing in the sandbox).

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

Out of scope, stated so nobody fills it silently: cleaning up the
misdirected `artifacts/agents/` residue at the three seats' wrapper roots
is an operational task on the live machines; no other script's root
resolution (`dispatch.sh`, `commit.sh`, adapters — all resolve from their
own location or an explicit flag) is modified; the `runtime session-env`
engine verb, the `stateroot` package's existing semantics, and
`evidence-gc.sh`'s own internals are untouched (only the hook's invocation
path of the collector changes).

## Consistency pass

Revision 3 was re-read end to end against itself: the candidate/validation
split of Decision 1 is what the failure map of Decision 2 guards (silence
for unprovable identification, the fixed line for absent engines, the
fixed line for skewed engines, exit-1 silence for ungoverned answers), what
every fixture of Decision 3 identifies by sentinel and evidence location
(cases 4-7 each pin one round-2 finding), and what every row of Decision 4
inherits; `world_installation` appears in the resolver, the engine
resolution, the `--metasystem-root` switch list, the collector invocation,
and the corresponding blast rows with one meaning; the verb is flag-less
everywhere it is named; no surviving sentence claims cwd participates in
resolution, that a pathname alone selects a world, that resolution ordering
is unchanged, that the worktree turn runs on a sandbox engine, that an
identification failure implies "not a worktree", or that every engine
failure is silent.

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
firing script's physical location, which is runtime-independent. Grade:
pass against everything observed; the reject condition below is the
falsifier the implementation and its critique must actively test.

**Reject condition — reject this design if any of the following is
shown:** a state-writing engine verb reachable from this hook whose world
is neither its explicit `--repo`/`--root` flag nor `RootForInstallation`
of an explicit metasystem root (an authority split the consumer table
missed); a supported layout in which neither the firing script's own
physical installation nor its mapped primary counterpart holds the
governing engine (the hook would go benign inside a governed world); any
path on which a governed decision rides a pathname-derived world that the
engine at that world did not itself answer (the SHR-R2-INSTALL-01
recreation); `up` writing supervision state at any root other than
`RootForInstallation(--metasystem-root)`; a linked worktree whose primary
counterpart at the same relative installation path is not the governing
installation (the one-step mapping would misdirect or silence it); any
identification-failure branch that proceeds as "not a worktree"
(the SHR-R2-WORKTREE-FALLBACK-01 inversion undone); any engine-failure
branch other than the verb's own exit-1 refusal that stays silent on stop
(the SHR-R2-ENGINE-SKEW-01 silence recreated); any turn in a mapped
worktree that reads or writes sandbox state, including through the
evidence collector (the SHR-R2-CONSUMER-01 split recreated); or any new
resolver failure path that exits nonzero or emits anything beyond the two
fixed one-line reports under `set -euo pipefail`.
