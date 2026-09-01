# Design: the supervision hook resolves the metasystem world, not the outer repository

Goal: supervision-hook-wrong-root (plans/goals/supervision-hook-wrong-root.md,
revision 6). Author: implementer delegate under dispatch by
m0b+main-1788250419-3170380-8a1fb3, 2026-09-02. Design-mode brief; every seam
cited below was read in this worktree at commit 500c1fc0, and the live-fleet
observations were read on the m0b checkout this design was authored beside.

## The defect, restated against the code

`scripts/agents/supervision-hook.sh:65` resolves the hook's world as
`git -C "$cwd" rev-parse --show-toplevel`. On the fleet's layout the metasystem
checkout is a subdirectory of a wrapper repository (observed on m0b:
`/home/wido.guest/m0b/agentic-tools/metasystem` inside the
`agentic-tools` toplevel), so `$repo` becomes the wrapper root — a directory
with no goal ledger, no enrolled steward, no armed supervision. Every
downstream consumer of `$repo` then operates on a bootstrap world.

Live evidence of the consequence, observed 2026-09-02 on m0b:

- The wrapper root's `artifacts/agents/` contains exactly the hook's
  misdirected writes: `supervision/hooks.log` (only that file), plus stray
  `steward/`, `goal.lock`, and `turn-verdict-state.json` created by the
  hook's `steward hook-attempt` and `report turn-verdict` calls against the
  wrong root.
- The real armed world lives at
  `metasystem/artifacts/agents/supervision/` (`last-census.json`,
  `owner.ndjson`, `lock.d/`, `reaper.heartbeat.json`, `census-writer.d/`) —
  where `metasystem up`, the steward enrollment, and health's
  hook-freshness role actually read and write.

So the hook's turn evidence never lands where hook-freshness is computed
(dead since enrollment on m2, m3, m0b), and on 2026-09-01 `report
turn-verdict --root <wrapper-root>` read a world with no `plans/goals/` and
could not refuse an idle turn-end while claimable work existed (goal record,
Next step, R-44-m0b).

The engine does not re-derive roots from what the hook passes:
`internal/report/openwork.go:52` (`resolveRepo`) is absolute-path and
symlink normalization only, and `internal/up/up.go:130` /
`internal/supervise/arming.go:659` default `MetasystemRoot` to `Root` rather
than searching. The fix therefore belongs at the one line where the hook
derives `$repo`; every consumer inherits it.

## Decision 1 — root resolution

### The marker

**A directory is a metasystem root exactly when it contains a regular file
named `metasystem.conf`.** One marker, one predicate: `[[ -f
"$candidate/metasystem.conf" ]]`.

Why this marker and not the others the brief names:

- `metasystem.conf` is git-tracked (`git ls-files` in this worktree lists
  exactly one: `metasystem/metasystem.conf`), so it is present in a fresh
  checkout before anything is built or run — observed: this delegate
  worktree has `metasystem.conf` and `plans/goals/` but **no** `bin/` and
  **no** `artifacts/`. It is also the engine's own root anchor: the census
  fingerprint reads config relative to the metasystem root
  (`internal/census/fingerprint.go:36-43,57`), and every fixture that
  drives the hook synthesizes or copies it because the engine refuses
  classification without it (`scripts/agents/supervision-fixtures.sh:1534`,
  comment at 1531-1533).
- `bin/metasystem` is rejected as a marker: it is an untracked build
  artifact — `make_repo`'s own comment says "Stage the engine the way
  production ships it: an untracked build artifact"
  (`supervision-fixtures.sh:472-475`), and the fresh worktree observation
  confirms it is absent before a build. Requiring it would silence the hook
  on exactly the fresh checkouts the benign-exit discipline is supposed to
  protect, not exclude.
- `artifacts/` is rejected: it is runtime state the hook itself creates
  lazily (`supervision-hook.sh:263-264` runs `mkdir -p`), absent before
  first run — and the defect under repair has already planted an
  `artifacts/` directory at the wrapper root on the live fleet (observed on
  m0b, above). An `artifacts/` marker would ratify the bug at the exact
  layout the fix targets.
- `plans/goals/` (and `plans/` generally) is rejected as a *required*
  marker: governed worlds without a goal ledger are legitimate — every
  hook-driving fixture root built by `make_repo`
  (`supervision-fixtures.sh:448-479`) has `metasystem.conf` and no `plans/`
  at all, and the hook's stop path must keep working there (the
  `stale-surface` invocation at `supervision-fixtures.sh:1289-1291` runs
  against such a root). The goal ledger is what the resolved world is
  *judged by*, not what identifies it.

False-positive analysis for the single marker: the repository tracks exactly
one `metasystem.conf`; delegate worktrees under
`artifacts/agents/worktrees/<job>/` carry their own copy, but each worktree
is its own git toplevel, so resolution from inside one is bounded to the
worktree and yields that worktree's own vendored world — the nearest
governing world, which is correct (see the worktree row in the case table
below).

### The search rule

Replace `supervision-hook.sh:65-66` with this resolution, mechanically:

1. `toplevel=$(git -C "$cwd" rev-parse --show-toplevel 2>/dev/null) || exit 0`
   — cwd outside any git repository stays the benign exit it is today.
   Normalize `toplevel` with `cd ... && pwd -P` (today's line 66 discipline).
2. Normalize the starting candidate: `candidate=$(cd "$cwd" && pwd -P)`.
   If the physical candidate is not `toplevel` or below (symlinked cwd whose
   physical path escapes the physical toplevel), set `candidate=$toplevel`.
   The containment test is the quoted-prefix case pattern
   `case "$candidate/" in "$toplevel"/*) ;; *) candidate=$toplevel ;; esac`.
3. **Ascend:** from `candidate` upward to and including `toplevel`, take the
   first directory satisfying the marker predicate as `repo` and stop. The
   loop is bounded by the toplevel; never ascend above it.
4. **One descendant probe:** if the ascent found nothing, test exactly
   `"$toplevel/metasystem"` against the same predicate — the fleet's actual
   layout. If it matches, `repo=$(cd "$toplevel/metasystem" && pwd -P)`.
   No other descendant is probed, ever: a recursive descent would be a
   guess, and the discipline in Decision 2 forbids guessing.
5. If `repo` is still empty, `exit 0`.

Ancestor wins over the descendant probe by construction (the probe only runs
when the ascent found nothing), so a cwd inside a nested world never gets
re-routed to a sibling world.

Both mandated layouts resolve to the same answer under the one rule:

| Layout | cwd | Resolution path | Result |
| --- | --- | --- | --- |
| Flat (metasystem root IS the toplevel) | root, or any depth below | ascent finds `metasystem.conf` at or before toplevel | the toplevel |
| Nested (fleet) | anywhere under `metasystem/` | ascent finds `metasystem/` before toplevel | `toplevel/metasystem` |
| Nested (fleet) | toplevel, or under a sibling subtree (`development/…`) | ascent exhausts at toplevel, descendant probe hits | `toplevel/metasystem` |
| Delegate worktree (own git toplevel, vendored `metasystem/`) | anywhere inside | same two steps, bounded by the worktree toplevel | the worktree's own `metasystem/` — the nearest governing world |
| Toplevel has a `metasystem/` directory that carries no `metasystem.conf` | outside any world | ascent and probe both miss | benign exit 0 |

Every cwd inside one world resolves to that one world — the determinism the
goal's DONE condition names.

### Placement and the binding header contract

The resolver is a small bash block in the hook itself, in place of lines
65-66 — not a new engine verb. The hook's header comment
(`supervision-hook.sh:4-15`) pins resolution as a shell-owned contract
(item 5, cwd resolution), fixtures drive the hook script directly, and every
consumer inside the script inherits the one `$repo` variable. The header
comment is binding context and must be amended in the same change: append a
sixth item — *"(6) world resolution: the nearest ancestor of the resolved
cwd, bounded by its git toplevel, carrying `metasystem.conf`; else the
single probe `toplevel/metasystem`; no world found stays benign exit 0,
never a guess."*

Ordering is unchanged: resolution still happens after the executable and
registry checks (lines 23-40), so "missing engine stays benign exit 0"
(header item 2) is untouched.

## Decision 2 — failure shape

The benign-exit discipline is preserved exactly:

- cwd not in a git repository → `exit 0` (today's `|| exit 0` at line 65,
  unchanged).
- git toplevel found but no directory on the ascent and no
  `toplevel/metasystem` satisfies the marker → `exit 0`, silently, before
  any payload-dependent side effect. The hook writes nothing, arms nothing,
  and never substitutes the toplevel (today's behavior) or any other guess.
- The resolver introduces no new nonzero exits and no new output channels.

A too-strict resolution degrades to today's silence; a guessed resolution
would recreate this very defect somewhere else. Given that asymmetry, every
unresolvable case is silence.

## Decision 3 — fixture

One new scenario block in `scripts/agents/supervision-fixtures.sh` (house
pattern: a `fixture_scenario` guard like `stop-hook-monitor` at line 1515),
named `nested-root`, plus one payload variant inside the existing flat
scenario. Assertions identify the resolved root **by the world the hook then
reports** — the goal-bearing plans it reads and where its evidence lands —
never by comparing the resolved path string alone.

Construction (nested): `scope=$tmp/nested-root`; build the world at
`$scope/metasystem` exactly the way `stop-hook-monitor` builds `stop_root`
(`supervision-fixtures.sh:1519-1544`: copy the hook, arm script, adapters;
copy the engine to `bin/metasystem`; print the fake-runtime
`metasystem.conf`; write a `plans/stream.md` naming open work); then
`git -C "$scope" init` and commit so the **scope** is the toplevel and the
world is nested. Create `mkdir -p "$scope/development/sub"` as a
marker-free sibling subtree. Plant the open-work sentinel text **only** in
the nested world's plans; leave the toplevel bare of `plans/` and
`metasystem.conf` (matching the observed fleet wrapper root).

Cases and assertions:

1. **Nested, cwd deep in a sibling subtree** — payload cwd
   `$scope/development/sub`, event `stop`. Assert the response carries
   `"decision":"block"` and the block reason quotes the sentinel step that
   exists only in `$scope/metasystem/plans/` — the hook can only have said
   that by reading the nested world's ledger. Under the pre-fix resolution
   this cwd yields the bare toplevel and no block, so the case fails before
   the fix and passes after: it is the regression the goal demands.
2. **Nested, cwd inside the world** — payload cwd
   `$scope/metasystem/scripts/agents`, event `stop`. Assert the identical
   block behavior: two cwds, one world, one answer.
3. **Evidence lands in the world** — after cases 1-2, assert
   `$scope/metasystem/artifacts/agents/supervision/hooks.log` is non-empty
   (the `stop-hook-monitor` evidence assertion at
   `supervision-fixtures.sh:1560-1561`, retargeted) **and** assert
   `[[ ! -e "$scope/artifacts" ]]` — the misdirected-write signature
   observed live on m0b must not reappear.
4. **Flat, cwd deep** — extend the existing `stop-hook-monitor` scenario
   with one more payload whose cwd is `$stop_root/scripts/agents` instead
   of `$stop_root`, asserting the same block/evidence behavior as the
   root-cwd payload. The existing scenario already covers flat-layout
   root-cwd; this variant proves depth does not change the answer. (Its
   block-once state means the deep-cwd payload must run as its own
   session id or before the settled transition, mechanically: give it a
   distinct `session_id`.)
5. **No world** — a git repository containing neither `metasystem.conf`
   anywhere on the ascent nor a qualifying `metasystem/` child: the hook
   exits 0 with no output and creates no `artifacts/`. (The existing
   `idle-hook` scenario at lines 1382-1400 stays the *governed*-idle
   coverage; this case is the ungoverned one.)

Fixture discipline per the harness rules: the hook is synchronous, so no
waits and no owned pids; register the world root in
`fixture_harness_roots` as the existing scenarios do
(`supervision-fixtures.sh:450`).

## Decision 4 — blast: every consumer of `$repo` in the hook

Line numbers at commit 500c1fc0. "Correct" below means: behaves correctly
once `$repo` is the governing metasystem root, on both layouts (on the flat
layout the resolved value is byte-identical to today's, so every row is a
no-op there).

| Line | Consumer | Why it is correct under the new resolution |
| --- | --- | --- |
| 66 | `pwd -P` normalization | Subsumed by the resolver (every branch normalizes with `pwd -P`). |
| 92 | `steward hook-attempt --repo` | Turn evidence now lands in the world where the steward is enrolled (observed live: the enrolled/armed state is under `metasystem/artifacts/`), so `generation`/`attemptSeq` resolve and hook-freshness revives — the goal's DONE condition. |
| 109 | `proc find-ancestor --repo` | The ancestor walk reads runtime adapters from this root (fixture comment, `supervision-fixtures.sh:1522-1526`); the wrapper root has no `scripts/agents/adapters/`, so identity resolution was structurally empty on the fleet. It now reads the real adapters. |
| 122, 131 | `lease classify --root "$repo" --metasystem-root "$harness_root"` | Root and metasystem-root now coincide on the fleet — the configuration the engine already supports as the flat default (`internal/up/up.go:130`, `internal/supervise/arming.go:659-660` default `MetasystemRoot` to `Root`). Announcements and leases are read where `up`-armed sessions write them (observed live under `metasystem/artifacts/agents/`). |
| 148-155, 342, 348 | `up --metasystem-root "$harness_root" --repo "$repo"` (stop / end / start) | `up` verifies and arms at the enrolled root instead of returning `ENROLLMENT_DRIFT` against the never-enrolled wrapper root. Census scope is still derived inside the machinery as the git toplevel of the root (`scripts/metasystem-config.sh:49`), so process-census coverage of the whole outer repository is preserved — the scope does not narrow. |
| 161 | `health --hook-preview --repo` | Health now reads the same world the attempt evidence lands in; hook-freshness is computable instead of structurally dead. |
| 166, 204 | `steward digest-pending` / `digest-advance --repo` | The digest cursor advances against the real steward state rather than an empty bootstrap world that never has a pending digest. |
| 185, 191, 197, 209 | `steward hook-complete --repo` | Completion evidence lands beside the attempt record it closes; the attempt/complete pair is finally in one world. |
| 221, 244, 249, 322-324 | `lease protocol-growth` / `renew` / `protocol-advance --root` | Same lease world as classification (rows above); growth counts and renewals now touch the lease that actually exists. |
| 256 | `supervise watchdog-report --repo` | The watchdog reads job records and supervision state where dispatch writes them (`artifacts/agents/jobs/` under the metasystem root). |
| 263-265 | `supervision_dir="$repo/artifacts/agents/supervision"` + `hooks.log`, and the verdict log lines at 282-283, 309-310 | The fired-vs-never-fired evidence trail (comment at 261-262) lands beside the armed supervision state the watcher and census own (skill contract: `<metasystem-root>/artifacts/agents/supervision/`); the stray wrapper-root `artifacts/` stops growing. |
| 274 | `report turn-verdict --root` | The consequence specimen: the verdict now reads the real `plans/goals/` ledger and stream plans, so an idle turn-end with claimable work is refused instead of waved through blind. |
| 332 | `steward pending --repo` | Session-start incident surfacing reads the real steward's pending set. |

Not consumers of `$repo`, unchanged by design: `$harness_root` (lines
23-25, resolved from the script's own location) and the `$ms` engine
resolution; the session, tag, and payload plumbing.

Existing-fixture blast, traced:

- `make_repo` roots (`gate_repo`, `$tmp/repo`), `stop_root`, `idle_repo`:
  flat layouts with `metasystem.conf` at the toplevel — resolution is
  byte-identical to today's; no behavior change.
- `operator-layout` (`supervision-fixtures.sh:585-725`): the one existing
  nested fixture. Its hook invocation (line 723-725, cwd
  `$operator_harness`) now resolves the harness instead of the scope —
  which is this design's intended behavior. Its assertions survive, each
  for a traced reason: the `! -e $operator_harness/artifacts` check
  (line 714) runs **before** the hook fires; the hook's `up` call against
  the harness cannot arm anything there because enrollment exists only at
  the scope (`enroll_fixture_engine "$operator_scope"`, line 612) and
  ordinary/recovery `up` returns `ENROLLMENT_DRIFT` before supervision is
  touched (skill contract), so the final
  `! -e …/lock.d/owner.json` check (line 738) holds; and nothing greps
  `operator-hook.out` for root-dependent content. The scenario's
  "state at the Git repository scope" assertions (lines 712-715) concern
  **arming state written by `up` under the operator's explicit `--repo
  $operator_scope` flag** — a path this design does not touch. If
  implementation shows any of these traced survivals wrong, that is a
  fixture-expectation update to the *corrected* behavior, escalated in the
  return — never a weakened assertion.

Out of scope, stated so nobody fills it silently: cleaning up the
misdirected `artifacts/agents/` residue at the three seats' wrapper roots is
an operational task on the live machines, not part of this change; and no
other script's root resolution (`dispatch.sh`, `commit.sh`, adapters — all
resolve from their own script location or an explicit flag, not from a
payload cwd) is modified.

## Self-grade

Grounding: every load-bearing claim above is either a file-and-line read in
this worktree (the hook, the fixtures, `up.go`, `arming.go`,
`fingerprint.go`, `openwork.go`, `metasystem-config.sh`) or a directly
observed live-fleet fact (the m0b wrapper root's stray `artifacts/` versus
the armed world under `metasystem/artifacts/`; the fresh worktree lacking
`bin/` and `artifacts/` while carrying `metasystem.conf`). No claim rests on
assumed code structure. The residual risks, honestly: (a) the
`operator-layout` survival trace depends on `ENROLLMENT_DRIFT` firing before
any supervision write when `up` runs against an unenrolled root — read from
the skill contract and the fixture's own drift assertions, not stepped
through `up.go` line by line; (b) the marker's false-negative universe is
argued from every *current* fixture and checkout, not from a proof about
future adopted layouts.

**Reject condition — reject this design if any of the following is shown:**
a hook-driving fixture root or supported checkout layout whose governing
world lacks `metasystem.conf` at its root (marker false-negative); an
in-repository ancestor directory between a real cwd and its toplevel that
carries a `metasystem.conf` without being the governing world (marker
false-positive the single-tracked-file argument missed); `up` writing any
supervision state at an unenrolled root before its enrollment check
(breaking the operator-layout survival trace); or a supported fleet layout
whose metasystem root is neither an ancestor of cwd nor exactly
`toplevel/metasystem` (the single descendant probe would silence it, and the
search rule itself would need reopening). Grade: pass against everything
observed; the reject condition is the falsifier the implementation and its
critique must actively test.
