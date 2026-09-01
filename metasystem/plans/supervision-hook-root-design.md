# Design: the supervision hook resolves the metasystem world, not the outer repository

Goal: supervision-hook-wrong-root (plans/goals/supervision-hook-wrong-root.md,
revision 6). Author: implementer delegate under dispatch by
m0b+main-1788250419-3170380-8a1fb3. **Revision 2, 2026-09-02**: folds all five
findings of records/misc/hook-root-critique-r1.md by id (SHR-ROOT-01,
SHR-WORKTREE-01, SHR-EXIT-01, SHR-FIXTURE-01, SHR-CONSUMER-01); each fold is
tagged inline. Every seam cited below was re-read in this worktree at commit
856e5b18; the live-fleet observations were read on the m0b checkout this
design was authored beside.

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

One engine verb is the exception, and revision 1 had it backwards
(SHR-CONSUMER-01): `up` re-derives its state world from `--metasystem-root`
through `stateroot.RootForInstallation` and overwrites its root option with
the result (`cmd/metasystem/up.go:104-113,139-144`); its `--repo` flag only
becomes the census scope via that path's git toplevel
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
fix is to ask the one shipped authority that already knows the difference.

## Decision 1 — root resolution (folds SHR-ROOT-01 and SHR-WORKTREE-01)

### The authoritative predicate (SHR-ROOT-01)

**The hook's world is `stateroot.RootForInstallation` applied to the
physical installation the firing hook script belongs to.** Two inputs the
hook already owns, one compiled authority, no search:

- The installation is `$harness_root` (`supervision-hook.sh:23-24`), the
  physical (`pwd -P`) grandparent of the running script — fixed by where the
  hook file lives, not by any payload field or content marker.
- The authority is exposed through one new engine verb, **`metasystem path
  state-root --installation <dir>`**, registered in the existing `path`
  family beside `owner` (`cmd/metasystem/main.go:244-247`,
  `path_verbs.go`). The verb refuses (exit 1, stderr diagnostic) when the
  argument is not a directory or lacks a `scripts/agents` directory (the
  installation shape shipped discovery already accepts without a
  configuration file, `stateroot.go:149-153`); otherwise it prints
  `stateroot.RootForInstallation(<dir>)` and exits 0. It adds no state, no
  writes, no options beyond `--installation`.

Revision 1's marker rule — "a directory is a metasystem root exactly when it
contains a regular file named `metasystem.conf`", nearest ancestor first —
is withdrawn. The critique showed both of its failure classes: a wrapper (or
any ancestor) carrying a stray `metasystem.conf` would out-rank the real
world for a hook fired from a sibling subtree, and a template installation
that carries only `metasystem.conf.local` plus `scripts/agents` — accepted
by shipped installation discovery (`stateroot.go:149-153`) and by layered
configuration (`internal/config/resolve.go:24-27,71-95`) — would be
silently rejected. Content markers are out entirely; so are the other
candidates revision 1 already rejected (`bin/metasystem` is an untracked
build artifact absent in fresh checkouts; `artifacts/` is lazily created
runtime state the live defect has already planted at the wrong root;
`plans/` is what the world is judged by, not what identifies it).

**Uniqueness argument.** (a) There is no collision set to tie-break: the
input is the one physical directory the running script is installed in, so
a stray configuration file anywhere on disk never enters the computation.
(b) The output is computed by the same compiled function every state writer
uses — `up` calls it directly (`up.go:139`), and the executable-anchored
state writers reach it through `StateRoot` (`stateroot.go:69-95`) — so the
world the hook reports is by construction the world the engine writes state
into; the hook cannot disagree with the engine because it no longer has its
own predicate. (c) Presence of `metasystem.conf` is not part of
`RootForInstallation` at all, so the `.local`-only template installation
resolves identically to a committed-conf one. (d) Collision behavior is
therefore fully defined: one input, one deterministic output, and every
unresolvable input is benign exit 0 (Decision 2) — never a guess.

### The linked-worktree rule (SHR-WORKTREE-01)

A hook can fire inside a linked delegate worktree: the worktree checks out
the tracked tree, so it carries its own copy of this hook and, because the
template marker `development/metasystem-design.md` is tracked, its vendored
`metasystem/` is template-mode — the raw authority would answer with the
worktree's own sandbox world.

**Decision: a hook firing inside a linked worktree reports the primary
checkout's world.** Grounded in what turn reporting is for: the turn verdict
and the hook evidence trail exist to keep the seat's session honest — they
read the goal ledger, the stream plans, and the job records beneath one
root (`internal/report/openwork.go:23-28,72-100`) and land evidence where
supervision is armed. A linked worktree is a disposable sandbox that carries
the tracked half of that state and never the ignored half: verified in this
very delegate worktree, which has `metasystem/plans/` but no
`metasystem/artifacts/` at all, while its own pending job record sits in the
primary world's `artifacts/agents/jobs/`. Worktree-local reporting would
therefore tell a delegate's turn-end "open plans, zero jobs in flight" —
structurally false while the delegate's own job is the running work — and
its evidence would vanish with the sandbox. Suppressing delegate hooks is
rejected for the same reason the evidence trail exists at all
(`supervision-hook.sh:261-262`): a silent hook is indistinguishable from an
uninstalled one. One world per machine gets one evidence trail; the
worktree's turn report joins it.

Mechanism, in the hook, before the verb call: query
`git -C "$harness_root" rev-parse --path-format=absolute --git-dir
--git-common-dir` (one call, two output lines; requires git ≥ 2.31 —
observed 2.39.5 on the fleet). If the two paths are equal, the installation
is not a linked worktree and `world_installation=$harness_root`. If they
differ: require the common dir's basename to be `.git` (else benign exit 0);
`primary_top` is the physical parent of the common dir; `wt_top` is the
physical git toplevel of `$harness_root`; the installation's path relative
to `wt_top` is re-rooted onto `primary_top` to form `world_installation`
(the primary counterpart of the same installation path — for the fleet
layout, `<primary>/metasystem`); if `$harness_root` is not at or below
`wt_top`, or the counterpart lacks a `scripts/agents` directory, benign
exit 0. The mapping runs at most once; a counterpart that is itself a linked
worktree is not re-mapped (benign exit 0 via the verb or the checks above if
anything fails). Every `--metasystem-root "$harness_root"` in the hook
(`up` at lines 148, 154, 342, 348; `lease classify` at 122, 131; `health`
at 161) becomes `--metasystem-root "$world_installation"`, so the whole
turn rides one world; where no mapping occurred the value is byte-identical
to today's.

### The replacement block

Lines 50-66 of the hook (the payload-cwd/session-env resolution and the
toplevel derivation) are replaced by, normatively:

```bash
world_installation=$harness_root
git_ids=$(git -C "$harness_root" rev-parse --path-format=absolute \
  --git-dir --git-common-dir 2>/dev/null) || git_ids=
if [[ -n "$git_ids" ]]; then
  git_dir=${git_ids%%$'\n'*}
  git_common=${git_ids#*$'\n'}
  if [[ "$git_dir" != "$git_common" ]]; then
    [[ "$(basename -- "$git_common")" == .git ]] || exit 0
    primary_top=$(cd -- "$(dirname -- "$git_common")" 2>/dev/null && pwd -P) || exit 0
    wt_top=$(git -C "$harness_root" rev-parse --show-toplevel 2>/dev/null) || exit 0
    wt_top=$(cd -- "$wt_top" 2>/dev/null && pwd -P) || exit 0
    rel=
    case "$harness_root" in
      "$wt_top") rel= ;;
      "$wt_top"/*) rel=${harness_root#"$wt_top"/} ;;
      *) exit 0 ;;
    esac
    world_installation=$primary_top${rel:+/$rel}
    [[ -d "$world_installation/scripts/agents" ]] || exit 0
  fi
fi
repo=$("$ms" path state-root --installation "$world_installation" 2>/dev/null) || exit 0
[[ -n "$repo" ]] || exit 0
repo=$(cd -- "$repo" 2>/dev/null && pwd -P) || exit 0
```

The payload's `cwd` field and the `runtime session-env` fallback chain
(lines 50-64) are deleted with this change: the world is a function of the
installation, so depth and direction of the session's working directory can
no longer change the answer, which is the goal's DONE determinism directly.
The engine verb `runtime session-env` itself is untouched; only the hook's
call is removed (verified: the hook was its sole shell caller, and no
fixture drives that fallback).

Case table — every hook firing resolves to exactly one world:

| Layout | Installation (script location) | Result |
| --- | --- | --- |
| Flat adopted (make_repo fixtures, adopted repositories) | the repository root | adopted mode → its own git toplevel: unchanged from today, byte-identical |
| Fleet template nested (m0b, m2, m3) | `<wrapper>/metasystem`, template marker tracked at the wrapper | template mode → `<wrapper>/metasystem` — the fix |
| Operator nested adopted (`operator-layout` fixture: no template marker at the scope) | `<scope>/metasystem` | adopted mode → `<scope>` — identical to today's cwd-toplevel answer; see Decision 4 |
| Linked delegate worktree of the fleet checkout | `<worktree>/metasystem` | mapped to `<primary>/metasystem`, then template mode → the primary world |
| Hook copy staged outside git without the template marker | anywhere | verb fails (adopted mode needs a toplevel) → benign exit 0 |

### Placement and the binding header contract

The resolver stays a shell block in the hook plus the one read-only engine
verb — resolution ordering is unchanged (after the executable and registry
checks, lines 23-40, so "missing engine stays benign exit 0" is untouched).
The hook's binding header comment (`supervision-hook.sh:4-15`) is amended in
the same change: items (1)-(3) stand; items (4) (session environment) and
(5) (cwd resolution) are deleted with their code; the new item (4) reads —
*"(4) world resolution: the engine's state-root authority (`path
state-root`) applied to the physical installation this script belongs to; a
linked-worktree installation is first mapped to its primary counterpart at
the same relative path; every resolution failure is benign exit 0, never a
guess."*

## Decision 2 — failure shape (folds SHR-EXIT-01)

The hook runs under `set -euo pipefail` (`supervision-hook.sh:2`), so an
unguarded command substitution is an abort, not a benign exit — the critique
demonstrated exactly that with revision 1's prescribed
`candidate=$(cd "$cwd" && pwd -P)`. That shape is banned. The discipline,
binding on the implementation: **every command substitution the resolver
introduces is written in the guarded form `value=$(... 2>/dev/null) ||
<mapped outcome>`** (the assignment carries the substitution's status, and
the guard catches it before `set -e` can), and no new failure path emits
output on any channel. The complete mapping:

| New operation | Failure | Mapped outcome |
| --- | --- | --- |
| `git rev-parse --path-format=absolute --git-dir --git-common-dir` | not a repository, git < 2.31, vanished directory | `|| git_ids=` → treated as not-linked; resolution continues on the installation itself |
| common-dir basename test | not `.git` (bare or exotic layout) | `exit 0` |
| `primary_top` / `wt_top` / final `repo` physical normalization | directory vanished or unreadable | each is `$(cd -- ... 2>/dev/null && pwd -P) || exit 0` |
| containment `case` | installation not at or below its worktree toplevel | `exit 0` |
| primary counterpart shape check | no `scripts/agents` directory there | `exit 0` |
| `path state-root` verb | invalid installation, adopted mode outside git, engine error | `|| exit 0`, then `[[ -n "$repo" ]] || exit 0` |
| evidence-trail `mkdir -p "$supervision_dir"` (existing line 264, now reachable with a write-denied primary from a sandboxed worktree session) | permission denied | gains `2>/dev/null || true`; the appends that follow already carry their own guards (lines 265, 282-283, 309-310) |

A cwd outside any git repository — today's benign case at line 65 — remains
benign by construction: cwd no longer participates, and an installation that
cannot resolve exits 0 through the verb row. Downstream writes that the
worktree decision newly points at a possibly write-protected primary
(steward attempt/complete, `up`, turn-verdict state) already flow through
the hook's disclosed degradation channels (`hook_evidence_failure`,
`up_failure`, the degraded-verdict branch at lines 306-320): the hook
reports degraded, emits, and exits 0. The asymmetry rule stands: a
too-strict resolution degrades to today's silence; a guessed resolution
recreates this defect somewhere else; so every unresolvable case is silence,
and the resolver introduces no new nonzero exits and no new output channels.

## Decision 3 — fixtures (folds SHR-FIXTURE-01; pins SHR-WORKTREE-01)

House pattern throughout: a `fixture_scenario` guard block in
`scripts/agents/supervision-fixtures.sh` like `stop-hook-monitor` (line
1515). The shipped block-once behavior suppresses a second identical block
for one session (`supervision-fixtures.sh:1553-1555`), so **every case
below that asserts `"decision":"block"` carries its own named `session_id`
— this is a fixture obligation, not a suggestion.** Assertions identify the
resolved world by what the hook then reads and writes — the sentinel step
its block reason quotes and where its evidence lands — never by comparing a
path string.

New scenario `nested-root` (template-mode nested; models the fleet):

- Construction: `scope=$tmp/nested-root`; build the world at
  `$scope/metasystem` exactly the way `stop-hook-monitor` builds
  `stop_root` (`supervision-fixtures.sh:1519-1544`: copy the hook, arm
  script, pre-commit guard, adapters; stage the engine at
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
- **Case 1, session `nested-sibling`** — payload cwd `$scope/development/sub`,
  event `stop`, fired through `$scope/metasystem/scripts/agents/
  supervision-hook.sh`. Assert `"decision":"block"` with a reason quoting
  `dispatch the nested runner`. Pre-fix the hook resolved this firing to the
  bare scope toplevel and could not block, so this case fails before the fix
  and passes after: the regression the goal demands. (The payload still
  carries a cwd because runtimes send one; the fix makes it inert.)
- **Case 2, session `nested-inside`** — payload cwd
  `$scope/metasystem/scripts/agents`, same hook, same assertion: two
  firings, one world, one answer.
- **Case 3, evidence lands in the world** — after cases 1-2, assert
  `$scope/metasystem/artifacts/agents/supervision/hooks.log` is non-empty
  and `[[ ! -e "$scope/artifacts" ]]` — the misdirected-write signature
  observed live on m0b must not reappear.
- **Case 4, session `nested-worktree` (the linked-worktree pin)** —
  `git -C "$scope" worktree add "$tmp/nested-wt" HEAD`; stage the engine at
  `$tmp/nested-wt/metasystem/bin/metasystem` (worktrees check out tracked
  files only — verified: a real delegate worktree has no `metasystem/bin/`
  and no `metasystem/artifacts/`); register that installation in
  `fixture_harness_roots`. Then edit the **primary's**
  `$scope/metasystem/plans/stream.md` to `Next step: recover the primary
  sentinel` (uncommitted, so the worktree keeps the old text). Fire the
  **worktree's own hook copy** with payload cwd inside `$tmp/nested-wt` and
  session `nested-worktree`. Assert `"decision":"block"` with a reason
  quoting `recover the primary sentinel` — a string that exists only in the
  primary world, so the hook can only have said it by reporting the primary.
  Assert the evidence appended to the primary's `hooks.log`, and
  `[[ ! -e "$tmp/nested-wt/metasystem/artifacts" && ! -e
  "$tmp/nested-wt/artifacts" ]]`. Block-once state lives in the primary,
  but the session id is fresh, so the block asserts cleanly. `up` failures
  in this case (the fixture world is unenrolled) surface as the disclosed
  `arming failed` line exactly as `stop-hook-monitor` already tolerates
  (`supervision-fixtures.sh:1546-1548`); the assertions here are only the
  block reason, the evidence location, and the absence of sandbox writes.
- **Case 5, no world** — a separate root holding copies of the hook,
  adapters, engine, and fake conf, with no git repository and no template
  marker: the hook exits 0 with empty output and creates no `artifacts/`
  anywhere under it. (The existing `idle-hook` scenario at lines 1382-1400
  stays the *governed*-idle coverage; this is the ungoverned case.)

Extension to the existing `stop-hook-monitor` scenario — **flat, deep
firing, session `t-deep`**: immediately after the block-once replay
assertion (line 1555), one more payload with `session_id` `t-deep` and cwd
`$stop_root/scripts/agents`, asserting the same block quoting `dispatch the
runner`. It must run before the scenario settles the plans to `Next step:
none` (line 1558), and its distinct session is what lets it assert a fresh
block.

## Decision 4 — blast: every consumer of `$repo` in the hook (folds SHR-CONSUMER-01)

Line numbers at commit 856e5b18. On the flat layout the resolved value is
byte-identical to today's, so every row is a no-op there. "Fleet" means the
template-nested layout; "worktree" means the mapped-primary case.

| Line | Consumer | Behavior under the new resolution |
| --- | --- | --- |
| 50-66 | cwd resolution and toplevel derivation | Replaced by the Decision 1 block; payload cwd and the session-env fallback are deleted. |
| 92 | `steward hook-attempt --repo` | Takes the world directly from the flag (`cmd/metasystem/steward_verbs.go:114-123`). Fleet: attempt evidence lands beside the enrolled steward state under `metasystem/artifacts/`, so `generation`/`attemptSeq` resolve and hook-freshness revives — the goal's DONE condition. Worktree: lands in the primary trail; a write-denied sandbox degrades to the disclosed `hook_evidence_failure`. |
| 109 | `proc find-ancestor --repo` | Reads runtime adapters beneath the flag root; the wrapper root has no `scripts/agents/adapters/`, so fleet identity resolution was structurally empty. Now reads the real adapters. |
| 122, 131 | `lease classify --root "$repo" --metasystem-root "$world_installation"` | Fleet: root and metasystem-root now both name `metasystem/`, where `up`-armed sessions actually write announcements and leases (observed live). Worktree: both name the primary. Non-worktree firings pass a byte-identical metasystem-root to today's. |
| 148-155, 342, 348 | `up --metasystem-root "$world_installation" --repo "$repo"` | **Corrected row.** `up`'s state world is `stateroot.RootForInstallation(--metasystem-root)` and is independent of `--repo` (`up.go:104-113,139-144`); `--repo` only sets the census scope as its git toplevel (`up.go:42-49,109,130`). On the fleet `up` therefore already armed the correct world (template mode: the marker is tracked at the wrapper toplevel), and this change alters neither its state world nor its census scope there — the scope stays the wrapper toplevel because that is the git toplevel of `metasystem/` too, so whole-repository process coverage is preserved. Revision 1's claim that fixing `$repo` would redirect `up`, and its `ENROLLMENT_DRIFT` account of the operator fixture, were wrong and are withdrawn. Worktree: both flags point at the primary; `up` verifies the already-armed rings, a delegate session gains at most advisor standing (the up contract: a second live session receives advisor, without displacement), and a sandboxed failure surfaces as the non-fatal `up_failure` line. |
| 161 | `health --hook-preview --repo "$repo" --metasystem-root "$world_installation"` | Health reads the same world the attempt evidence lands in; hook-freshness is computable instead of structurally dead. |
| 166, 204 | `steward digest-pending` / `digest-advance --repo` | The digest cursor advances against the real steward state rather than an empty bootstrap world. |
| 185, 191, 197, 209 | `steward hook-complete --repo` | Completion evidence lands beside the attempt record it closes. |
| 221, 244, 249, 322-324 | `lease protocol-growth` / `renew` / `protocol-advance --root` | Same lease world as classification; growth counts and renewals touch the lease that exists. |
| 256 | `supervise watchdog-report --repo` | Reads job records and supervision state where dispatch writes them. |
| 263-265, 282-283, 309-310 | `supervision_dir="$repo/artifacts/agents/supervision"`, `hooks.log` | The fired-vs-never-fired trail lands beside the armed supervision state; the stray wrapper-root `artifacts/` stops growing; line 264's `mkdir -p` gains the Decision 2 guard. |
| 274 | `report turn-verdict --root` | The consequence specimen: the verdict reads the real `plans/goals/` ledger and stream plans (`openwork.go:23-28`), and — in the worktree case — the primary's job records, so a delegate's active job is visible in-flight work instead of a phantom idle. An idle turn-end with claimable work is refused instead of waved through blind. |
| 332 | `steward pending --repo` | Session-start incident surfacing reads the real steward's pending set. |

Not consumers of `$repo`, unchanged: the `$ms` engine resolution from
`$harness_root` (the binary that shipped with the firing hook keeps running
the turn, including in worktrees); the session and tag plumbing.

Existing-fixture blast, retraced against the corrected mechanism:

- `make_repo` roots (`gate_repo`, `$tmp/repo`), `stop_root`, `idle_repo`:
  flat adopted layouts, each its own git toplevel —
  `RootForInstallation(root)` returns the root itself; resolution is
  byte-identical to today's, including `stop-hook-monitor`'s expected
  `ENROLLMENT_DRIFT` line (1546-1548), because `up`'s state world for the
  never-enrolled `stop_root` is unchanged.
- `operator-layout` (`supervision-fixtures.sh:586-738`): **corrected
  trace.** The fixture copies the shipped tree into `<scope>/metasystem`
  only (lines 599-604), so the scope carries no
  `development/metasystem-design.md` and the installation is adopted-mode:
  `RootForInstallation(<scope>/metasystem)` is the scope toplevel — the
  same value today's `git -C <harness> rev-parse --show-toplevel` produces
  for the hook firing at line 723-725. Every consumer therefore behaves
  byte-identically, and every existing assertion (census state at the
  scope, line 711; no split state beneath the vendored installation, lines
  713 and 736) survives without appeal to any enrollment-drift story. The
  scenario keeps covering the adopted-nested flavor; the new `nested-root`
  scenario covers the template-nested flavor the fleet actually runs.

Out of scope, stated so nobody fills it silently: cleaning up the
misdirected `artifacts/agents/` residue at the three seats' wrapper roots is
an operational task on the live machines; no other script's root resolution
(`dispatch.sh`, `commit.sh`, adapters — all resolve from their own location
or an explicit flag) is modified; the `runtime session-env` engine verb and
the `stateroot` package semantics are untouched.

## Consistency pass

Revision 2 was re-read end to end against itself: the predicate named in
Decision 1 is the one the failure map in Decision 2 guards, the one every
fixture in Decision 3 identifies by sentinel and evidence location, and the
one every row of Decision 4 inherits; `world_installation` appears in the
resolver, the `--metasystem-root` switch list, and the corresponding blast
rows with the same meaning; no surviving sentence still claims cwd
participates in resolution, that `metasystem.conf` identifies a world, that
a worktree's vendored world is "correct", or that `--repo` selects `up`'s
state world.

## Self-grade

Grounding: every load-bearing claim is a file-and-line read in this worktree
(the hook, the fixtures, `up.go`, `stateroot.go`, `resolve.go`,
`openwork.go`, `steward_verbs.go`, `path_verbs.go`, `main.go`), a command
run here (`git ls-files development/metasystem-design.md` in primary and
worktree; `git rev-parse --path-format=absolute --git-dir --git-common-dir`
inside this real linked worktree, returning the two distinct absolute paths
the mapping rule consumes; `git --version` 2.39.5; the absence of
`metasystem/bin` and `metasystem/artifacts` in this worktree), or a directly
observed live-fleet fact (the m0b wrapper root's stray `artifacts/` versus
the armed world under `metasystem/artifacts/`). Residual risks, honestly:
(a) on git older than 2.31 the `--path-format` query fails and the
worktree mapping silently degrades to worktree-local resolution — declared
as a version floor rather than engineered around, since the fleet runs
2.39.5; (b) m2 and m3 are assumed to match m0b's layout — stronger than
revision 1's assumption because the template marker is git-tracked, so any
full checkout carries it; (c) the write-denied-sandbox degradation path is
traced through the hook's existing failure channels, not executed live.
Grade: pass against everything observed; the reject condition below is the
falsifier the implementation and its critique must actively test.

**Reject condition — reject this design if any of the following is shown:**
a state-writing engine verb reachable from this hook whose world is neither
its explicit `--repo`/`--root` flag nor `RootForInstallation` of an explicit
metasystem root (an authority split the consumer table missed); a supported
layout in which the firing hook script does not physically live inside its
governing installation (the script-location input would then be wrong by
construction); `up` writing supervision state at any root other than
`RootForInstallation(--metasystem-root)` (breaking the corrected consumer
row); a linked worktree whose primary counterpart at the same relative
installation path is not the governing installation (the one-step mapping
would misdirect or silence it); or any new resolver failure path that exits
nonzero or emits output under `set -euo pipefail` (a Decision 2 violation
of the kind the critique demonstrated against revision 1).
