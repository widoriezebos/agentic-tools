# Design: the supervision hook resolves the metasystem world, not the outer repository

Goal: supervision-hook-wrong-root (plans/goals/supervision-hook-wrong-root.md,
revision 9); since the 2026-09-06 split, the design's remaining scope is
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

**Revision 7, 2026-09-13**: written by the design delegate under dispatch by
m1c+main-1789191340-90689-8975a9, on the worktree at trunk c905ca8d with the
Codex port of chain hrd-build1-20260906 applied. Revision 6 predates five
trunk landings that changed what the hook may do on a Stop, and the
independent Opus read of the port (scratchpad g10, opus-read-r1) found the
page silent or stale at seven material and eight minor points where the
builder then chose for himself. This revision settles each against the code
as it stands, keeps everything of revision 6 that still holds, and says
here what changed. Every line number this revision introduces is marked
"c905ca8d" and was re-read at that commit; the "today" numbers of revision
6 still refer to 3c753c5ef and the "HEAD" numbers of revision 5 to
12ed490c3, and both are re-cited below wherever revision 7 binds them. The
eight points and where each lands:

1. *Missing engine and engine skew (read finding M1).* Trunk goal
   stop-infrastructure-allows-the-seat-to-stop (529d8a64, fed9f5d9e,
   286efe41e) retired the fail-closed Stop: a failure of the hook's own
   infrastructure allows the stop under a degraded notice, and
   `raw_missing_engine_stop` is now that allowance (c905ca8d line 32).
   Revision 7 follows the landed class rule for both outcomes: a missing
   engine and an engine that does not answer `path state-root` are each a
   fixed degraded allowance, never a block (Decision 2; the header contract;
   the replacement block; case 6, case 7 and the `missing-template` row).
   The alternative — keep the skew block the read's own smallest fix kept —
   is named in Decision 2 with its consequence.
2. *Blocks under the once-per-plan-line rule (M3).* Open work blocks once
   per plan-line digest in any session (`internal/goal/turnverdict.go:947-959`,
   `internal/report/openwork.go:271-323`, both c905ca8d). Every fixture case
   that asserts a block now writes its own `Next step:` line into the world
   it expects to answer, immediately before firing, and asserts the block
   quotes that line; case 14 asserts the goal's DONE on the component record
   and the health line, with the block as the turn-completing side
   (Decision 3, house pattern and each case).
3. *Which installation a bed firing runs from (M2).* No fixture in any bed
   fires the checkout's own hook on any event once cwd is inert: every
   firing runs a hook copy staged in a fixture installation that carries its
   own `cp`'d engine, and every world whose Stop must block is enrolled and
   armed first, because on trunk a recorded arming failure turns the Stop
   into an infrastructure allowance (Decision 3, fixture rules). *The
   "because" clause is false and is withdrawn by revision 9: a seat-actionable
   block survives an arming failure; arming is a fidelity choice.*
4. *Engine override and enrollment (M4).* Refused: `up` keeps its landed
   enrollment gate (`internal/up/up.go:520-537`, c905ca8d) under
   `METASYSTEM_BIN`; an override that is not the enrolled engine is the
   disclosed `supervision arming failed`, and the fixtures assert that
   allowance's shape instead of a block (Decision 4 `up` row; case 9).
   *Revision 9 corrects the last clause: the block on open work survives,
   and the arming failure is disclosed beside it.*
5. *The parent's resolver timing (M5).* The parent never blocks on its own:
   an unresolved root changes only where it can log and whether it can
   record. When the worker ends with an unreadable answer the parent waits
   for its resolver until the deadline before logging and allowing; with a
   valid answer it stops the resolver as today; on timeout the resolver is
   stopped at the deadline and a root that never arrived is the shipped
   record-failure allowance (Decision 1, parent subsection; Decision 2
   parent rows; cases 11b and 13b).
6. *Linked worktrees that the engine has made a world (M6).* The exception
   exists, restated on the page's terms: a linked worktree whose candidate
   installation carries the steward identity record the engine itself
   writes when it arms a world (`artifacts/agents/steward/identity.json`,
   `internal/steward/install.go:12-14`) is its own installation; every
   other linked worktree maps to its primary. The engine's presence cannot
   be the test — 85 of the 221 delegate worktrees on this machine carry a
   built `bin/metasystem` and none carries an identity (Decision 1,
   linked-worktree rule; case 15). *Superseded by revision 8: the engine
   never arms a linked worktree, so no such record can be the engine's;
   the exception is withdrawn and the question goes to Wido (below).*
7. *The verb's answer and the transport (m1, m2, m3).* `path state-root`
   returns the validated installation, because trunk `up` arms
   `canonical(--metasystem-root)` and not `RootForInstallation` of it
   (`cmd/metasystem/up.go:103-107,141`, c905ca8d); `$repo` equals
   `$world_installation` in every layout and stays the one name consumers
   use; the collector is invoked at `$world_installation`; the parent
   transports the session and the root in separate files and accepts a root
   of exactly one line (Decision 1; Decision 4 `up` and collector rows).
8. *Landing order and the docs paragraph (m7, m8).* A new section before
   the consistency pass states what the landing note tells every seat and
   what the one docs paragraph must say.

No point is left open for Wido: each is decided from the landed code and
rulings. The two he is most likely to want to see are points 1 (skew as an
allowance) and 6 (the identity exception); each names its alternative and
that alternative's consequence where it is decided. *(Revision 8 reopens
one: point 6 is an ownership invariant and goes to Wido, below.)*

**Revision 8, 2026-09-13**: written by the same design delegate, same
worktree, folding the design read of revision 7
(scratchpad g10, codex-design-read-r7). Provenance of that read, as its
author reports it: the requested gpt-5.6-sol child could not start in the
read-only sandbox (`codex exec` failed before session creation with
"failed to initialize in-process app-server client: Operation not
permitted"), so the read ran on the forwarding Codex session itself and
carries no Codex task id; it was read-only (`bash -n` and `git diff
--check` only). Five material and three minor findings; dispositions:

- **M1 folded, and the invariant put to Wido.** The read showed that
  revision 7's premise for the identity exception is false in the engine:
  `steward arm` refuses every linked worktree before it mints anything
  ("not armed: linked worktree (the primary checkout owns the watchdog)",
  `internal/steward/runner.go:578-589,594-609`, c905ca8d), `EnsureRunner`
  reports the runner excluded there (410-420), and `up` opens an existing
  enrollment through `VerifyIdentity` (owner, mode, JSON, repository
  identity; `identity.go:125-150,406-410`) and never mints a first one. So
  an `identity.json` inside a linked worktree can only be fixture-written,
  copied, stale or malformed, and the shell's `-f` test authenticates none
  of that. The exception is withdrawn: revision 6's unconditional
  linked-worktree-to-primary mapping stands, the port's test and its
  "governed linked worktree" fixture are deleted, and case 15 is withdrawn
  (Decision 1, linked-worktree rule; the replacement block; the case table;
  header item (2); the docs paragraph; the reject condition). **The
  invariant for Wido:** may a linked worktree ever own an independent
  steward world? Option A — no, the engine's landed rule: the primary
  checkout owns the watchdog; every linked worktree maps to its primary;
  the seven engine-carrying linked checkouts of `agentic-tools` on this
  machine map to `agentic-tools/metasystem`; nobody can make a worktree
  its own world, by hand or otherwise, and a copied or stale identity file
  cannot redirect evidence. Option B — yes, when the engine says so: that
  needs the steward ownership contract changed (the exclusion at
  runner.go:578-589 and the refusal in `arm`), an authenticated identity
  check in the engine before the hook routes evidence (the shell cannot do
  `VerifyIdentity`), and a new design member for it; until that member
  lands the hook cannot implement B safely, and a `-f` test would route a
  delegate's evidence into its disposable sandbox on the strength of a
  file anyone can create. The page proceeds on A, which is the landed
  engine behaviour; Wido decides whether B is wanted as new scope.
- **M2 folded**: every remaining normative `RootForInstallation` passage
  now names the validated installation (`RootForCandidate`) or is marked as
  the revision it belonged to — the override paragraph and uniqueness
  argument (b) of Decision 1, the compiled-authority consequence and member
  A's paragraph, Decision 4's cwd row and `up` row, and three clauses of
  the reject condition; the round-4 hole description and the killed-attempt
  trace are marked as history with the value unchanged.
- **M3 folded**: case 14 is re-specified as the goal's discriminating DONE
  proof — it fires first in the scenario with its payload cwd inside a
  distinct nested Git checkout beneath the wrapper, which trunk's
  cwd-derived resolver selects and revision 8's ignores; it fails on
  c905ca8d and passes after (Decision 3, case 14).
- **M4 folded**: the landing note now states the three rollout states (old
  hook; new hook with an old primary engine; new hook with a rebuilt,
  re-armed primary) and that an in-flight worktree carrying the old tracked
  hook gains nothing until its hook is refreshed (Landing and
  documentation).
- **M5 folded**: the worker's answer must be exactly one line — empty, or a
  newline or carriage return inside it, is the skew allowance — written
  into the replacement block and the failure map; the worker and the parent
  share the one rule (Decision 1; Decision 2).
- **m1 folded**: residual risk (r) now says skew has no durable repair
  signal until member stop-incidents-reach-the-steward lands; the per-Stop
  notice is the only visibility (the stop-infrastructure page, section 3).
- **m2 folded**: residual risk (q) says the wait after an unreadable worker
  can consume only the remainder of the same 57-second window the worker
  and the resolver share (`deadline_expires`, c905ca8d 76), never a second
  budget.
- **m3 folded**: the landing table's list of blocking cases drops case 9
  (an allowance) and, with revision 8, case 15.

Line numbers this revision introduces are at c905ca8d, marked as such;
revision 7's are unchanged.

**Revision 9, 2026-09-13**: written by the same design delegate on the
worktree carried onto trunk 3a6353c3 (scratchpad g10, wt3), closing the
one gap the builder stopped on without choosing (codex-report-r3, item 2).
Revision 8 said three incompatible things about a Stop that carries both an
arming failure and an unseen plan line: Decision 2 said seat-actionable
findings still block; fixture rule 2 said an arming failure is "an
allowance carrying the verdict display, never a block"; case 9 wrote a
fresh sentinel and then required no block. Trunk implements one
precedence and the preserved Mac output shows it (`artifacts/agents/
suite-failures/20260913T121447Z-supervision-12059/nested-override.out`:
`"decision":"block"` with the reason quoting `nested-override sentinel` and
a `systemMessage` whose second line is `Cause: supervision arming failed`).
**The decision: a seat-actionable finding keeps its block when an
infrastructure condition is present; the infrastructure condition never
suppresses a block and never creates one — it is disclosed in the
`systemMessage`, logged as a stop-condition line, and counted in the
stop-refusal record; an infrastructure condition alone is the allowance.**
That is the landed rule of the stop-infrastructure page ("seat-actionable
... block as today") as `compose_failed_stop` implements it
(`supervision-hook.sh:1005-1104` at 3a6353c3: `external_stop_json`
(862-866) writes the record and returns the infrastructure text,
`internal/report/stopblock.go:176-184`; a blocking verdict is then
rendered by `stop_block_json` with the verdict display as the reason and
that text as the `systemMessage`, 1088-1094; without a block, the
allowance, 1095-1103). Written into Decision 2 (a precedence paragraph),
Decision 3 (fixture rule 2 restated — arming is a fidelity choice, not a
block precondition; cases 9 and 16 re-asserted as blocks with the
disclosed failure), the case table's override row, Decision 4's `up` row,
the landing table, the consistency pass and the reject condition. The
hook at 3a6353c3 is c905ca8d's plus ten lines at 1372-1381 (the start
path's `session start --root "$state_root"`), so every hook line cited by
revisions 7 and 8 holds and the sweep gains that one site.

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
  shell mapper adopts below. **Step (3) is revised by revision 7 (the
  read's m1):** the verb returns the validated installation itself, not
  `RootForInstallation` of it, and makes no git call at all. Trunk's `up`
  no longer arms `RootForInstallation(--metasystem-root)`: it canonicalizes
  the explicit root (`cmd/metasystem/up.go:26-29,103-107`, c905ca8d) and,
  after the scheduler print, sets `options.Root = root` — the installation
  — with the comment that supervision control, enrollment and accounting
  belong to the authenticated installation while the census scope stays
  the containing repository (`up.go:138-141`). The operator-layout scenario
  asserts that state today: census state at the vendored installation and
  nothing at the scope (`supervision-fixtures.sh:884-887`, c905ca8d). Had
  the verb kept revision 4's answer, an adopted installation's hook would
  have named the scope while `up` armed the installation — the split this
  page exists to close. So `RootForCandidate(<installation>)` is:
  `filepath.Abs`, then `filepath.EvalSymlinks` (falling back to `Clean`),
  then the shape gate, then the result; the only refusal is the shape gate
  (exit 1). `RootForInstallation` and `templateMode` keep their semantics
  for the executable-anchored writers this hook never reaches, and stop
  being the hook's function. The port's `RootForCandidate` and
  `validateInstallationShape` (shared with `installationRoot()`) are this
  contract.

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
override. **Revision 7 re-grounds the sentence before this one on trunk
c905ca8d:** `up`'s state world is `canonical(--metasystem-root)`
(`cmd/metasystem/up.go:26-29,103-107,141`), the verb returns the
canonicalized, shape-gated installation, and so `$repo` is byte-identical
to `$world_installation` in every layout once the gate passes; the two
names stay because `$repo` is the validated form (a canonical path that an
engine accepted as an installation) and `$world_installation` the
candidate the mapper produced, and a consumer that names `$repo` names the
engine's answer, never the mapper's. The collector honors the same override (`evidence-gc.sh:17` reads
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
  hook ends on the missing-engine outcome of Decision 2 (a degraded
  allowance since 529d8a64, revision 7), whatever
  `METASYSTEM_BIN` says; if
  an engine does live there, that engine answers for that installation, not
  for where the link pointed. Either way no pathname is believed.
- *Copied or relocated hook*: identical rule. A bare copy finds no engine
  and ends on the missing-engine outcome. A full relocated installation with
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
is false (`stateroot.go:100-108`) — under the revision 4 verb; under the
revision 7 verb the same copy would have adopted its own directory, a
world with no ledger, which is the same hole — and the round-two hole
reopens by pathname. The rule now: **the candidate must carry an engine at
`<candidate>/bin/metasystem` for the world to be governed, override or
not.** That file is the installation's own evidence of being an
installation; `METASYSTEM_BIN` may replace which engine RUNS the turn but
never waives it. Mechanically the hook computes
`canonical="$world_installation/bin/metasystem"` and
`ms="${METASYSTEM_BIN:-$canonical}"`, and requires BOTH `-x "$canonical"`
and `-x "$ms"` before any governed step; either absent is the
missing-engine outcome of Decision 2 (revision 5 wrote "a block on Stop";
since 529d8a64 it is the degraded allowance, revision 7). Without an override
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
fire under `METASYSTEM_BIN` as well as without it. (Revision 7: on
c905ca8d that row sits at `supervision-hook-fixtures.sh:1181-1204`, fires
under `METASYSTEM_BIN=$failure_engine` forwarding to `$line_root`'s engine,
and already rejects `"decision":"block"` while requiring `degraded
infrastructure`; the double test produces the missing-engine allowance,
which satisfies it, and the compiled-authority subsection adds `engine
missing` and the no-writes check.)

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
the world is the override engine's validated answer (`RootForCandidate`,
revision 7; revision 4 wrote `RootForInstallation`) for the
same `$world_installation` every consumer names, so the override cannot
split a turn; revision 3's sentence that the world "follows the override
engine's own answer" is withdrawn as the split round 3 proved. Fixture
consequence, traced: the killed-attempt fixture fires
`$line_root/scripts/agents/supervision-hook.sh` under
`METASYSTEM_BIN=$tmp/kill-engine` forwarding to the harness engine
(`supervision-hook-fixtures.sh:356-381`); the candidate is `$line_root`
(not a linked worktree), `$line_root/bin/metasystem` exists (staged by `cp`
at line 164), the verb returns `$line_root` (revision 4 wrote
`RootForInstallation($line_root)`; the installation and its toplevel
coincide, so the value is the same under the revision 7 verb), and
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
(`RootForCandidate`, the canonical shape-gated installation — revision 8
supersedes revision 4's `RootForInstallation` here, because `up` applies
`canonicalPath` to the same bytes) applied by *the same engine* to *the same
installation bytes* that every state-writing consumer of the turn
receives, so the hook cannot disagree with `up` about the world under
symbolic links, copies, relocations, or an engine override
(SHR-R2-INSTALL-01's residue closed; SHR-R3-ENGINE-INSTALLATION-PAIR-01's
split closed). (c) Presence of `metasystem.conf` is
not part of the answer, so a `.local`-only template installation resolves
identically to a committed-conf one. (d) Every unresolvable or unprovable
input is benign exit 0, and the only non-silent degradations are the two
fixed reports of Decision 2 (missing engine, engine/hook skew) — blocks
under revisions 5 and 6, degraded allowances under revision 7 — never a
guess.

### The linked-worktree rule (folds SHR-R2-WORKTREE-ENGINE-01 and SHR-R2-WORKTREE-FALLBACK-01)

The decision itself stands from revision 2: **a hook firing inside a linked
worktree reports the primary checkout's world** — without exception;
revision 7 added one and revision 8 withdraws it below, because the engine
itself never arms a linked worktree. The turn verdict and the
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

**No exception: the engine never arms a linked worktree (revision 8,
folds the design read's M1 and withdraws revision 7's exception).**
Revision 7 wrote that a linked worktree carrying
`artifacts/agents/steward/identity.json` is "a world the engine has armed"
and kept the port's test on that file. The design read showed the premise
false: `steward arm` refuses every linked worktree before it mints anything
— `runnerExclusion` returns "linked worktree (the primary checkout owns the
watchdog)" whenever the git dir and the common dir differ, and `arm`
returns "not armed" on it (`internal/steward/runner.go:578-589,594-609`,
c905ca8d; the test at `runner_test.go:535-536` pins the refusal);
`EnsureRunner` reports the runner "excluded" there (410-420); and `up`
never mints a first identity, it opens an existing one through
`VerifyIdentity`, which checks existence, owner-only mode, the calling
user's uid, JSON shape and the repository identity (`identity.go:125-150`,
`406-410`). So an identity file inside a linked worktree is never the
engine's record: it is a fixture helper's (`enroll_fixture_engine`), a
copy, a stale or a malformed file, and a shell `-f` test cannot tell those
apart — where `stat` can see the file the hook would route a delegate's
attempt evidence into its disposable sandbox before `up` ever looked at
the record, and where a permission hides it the same checkout would map.
The exception is withdrawn: **a hook firing inside a linked worktree
reports the primary checkout's world, whatever files the worktree
carries.** The port's test (its `hook_world_installation`, the branch on
`identity.json`) is deleted, its "governed linked worktree" fixture with
it, and case 15 below is withdrawn. Whether a linked worktree may ever own
an independent steward world is an invariant only Wido can settle; the
revision record puts it to him with both options and proceeds on the
engine's landed rule. The rest of revision 7's paragraph — the delegate
census and why the engine cannot be the test — stands as the reason the
mapping is unconditional rather than engine-gated. The mapping exists
because a delegate worktree ships the tracked half of the world and never
the ignored half; the natural test for "ignored half present" would be the
engine at `<candidate>/bin/metasystem`, but that test is false on this
fleet: read on this machine on 2026-09-13, 85 of the 221 delegate worktrees
under the seats' `artifacts/agents/worktrees/` carry a built
`bin/metasystem` (builders run `go build` in their sandbox) and not one
carries an identity file. An engine test would make those 85 sandboxes
their own worlds — the structurally false verdict this rule was written to
prevent. Revision 7 then argued that the identity file is different in
kind — the engine writes it, and only the engine, when `steward arm` takes
an installation as a world (`steward.RepoIdentityPath(repoRoot)` =
`<repoRoot>/artifacts/agents/steward/identity.json`,
`internal/steward/install.go:12-14`, c905ca8d; `up` opens the enrolled
binary from exactly that root, `internal/up/up.go:522`). That is true of a
primary checkout, and it is exactly why no such record can exist in a
linked worktree: `steward arm` refuses a linked worktree before minting
(`runner.go:606-609`), so the only writers of that path inside a worktree
are a fixture helper, a copy or a stale file — revision 8's correction, and
the end of the exception. *(Revision 7 continued from here with the
exception's rule and its consequences; that text is withdrawn by revision 8
and replaced by what follows.)* What the
withdrawal leaves in force: every linked worktree maps to its primary, so
every delegate maps whether or not its sandbox carries an engine or a
file named `identity.json`; the linked-worktree checkouts of `agentic-tools`
on this machine that carry an engine and no identity (`-l10`, `-l13`,
`-mrspeed`, `-paper`, `-pmimpl`, `-synclock`, `-witness`; `git worktree
list` and a stat of each, 2026-09-13) map to `agentic-tools/metasystem`,
which is armed — a change from trunk, where each is its own world by cwd
and its every Stop ends in `supervision arming failed` because the engine
refuses to arm it; and nobody can make a linked worktree its own world by
hand, because `steward arm` refuses. The "hand-armed worktree" revision 7
worried about cannot exist under the landed engine, so option A costs
nothing that exists today. Cases 4, 8 and 10-13 keep pinning the mapping.

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
printed (SHR-R4-FAIL-CLOSED-REGRESSION-01, Decision 2). **Revision 7**: on
c905ca8d `$raw_missing_engine_stop` (line 32) is the degraded allowance
529d8a64 made it, and `$raw_engine_skew_stop` is an allowance of the same
shape (Decision 2); the block below is shown with revision 7's comments
and the identity test of the linked-worktree rule, and the worker's own
resolution block it replaces sits at c905ca8d lines 423-425 (engine),
456-463 (missing engine), 464-472 (registry), 474-481 (payload), 487-503
(cwd, session-env and toplevel) and 508-517 (the shell marker block):

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
  # Every linked worktree is the primary's world: the engine itself never
  # arms one (internal/steward/runner.go, runnerExclusion: "the primary
  # checkout owns the watchdog"), so no file inside the worktree can make
  # it a world, and none is consulted (revision 8).
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
# evidence. A missing engine allows a Stop under the degraded notice
# (529d8a64; the literal at c905ca8d line 32): no safe verdict can be
# made, and the seat is not held by its own broken infrastructure.
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
# installation): silent here; the Stop parent answers an empty worker
# with its unreadable-output allowance (c905ca8d 248-251). Any other
# failure is engine/hook skew in a governed world: it allows the Stop
# under a degraded notice that names the rebuild, so a fleet that
# rebuilds its engines daily can tell a skewed hook from one that never
# fired (revision 7; Decision 2). The answer must be exactly one line:
# empty, or a newline or carriage return inside it, is a broken engine
# and the same skew allowance — the parent applies the identical rule to
# its own resolver's answer (revision 8).
repo_rc=0
repo=$("$ms" path state-root "$world_installation" 2>/dev/null) || repo_rc=$?
if (( repo_rc == 1 )); then
  exit 0
elif (( repo_rc != 0 )) || [[ -z "$repo" || "$repo" == *$'\n'* || "$repo" == *$'\r'* ]]; then
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
**Revision 7 replaces that literal** with the allowance of the same shape
as c905ca8d's line 32:
`{"systemMessage":"Metasystem engine and hook are out of step: this engine does not answer path state-root; stopping is allowed with degraded infrastructure. Rebuild bin/metasystem; the steward owns repair."}`.
The text keeps the two phrases the fixtures key on — `does not answer path
state-root` for case 7 and `degraded infrastructure` as in every landed
allowance — and carries no quotes or backslashes, so it stays a plain
`printf`. The parent's no-validator fallback (HEAD 172-175) accepts this literal
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

**The parent on trunk c905ca8d (revision 7, folds the read's M5 and m3).**
Between revision 6 and today the parent moved twice. Goal
stop-infrastructure-allows-the-seat-to-stop (529d8a64, fed9f5d9e, 286efe41e)
made every outcome the parent decides on its own an allowance: an
unreadable or empty worker answer is logged as the stop condition
`stop-hook-output-was-unreadable` and answered with
`emit_raw_stop_allowance` (c905ca8d 27-31, 248-251) — the generic block
revision 6 leaned on (`emit_raw_stop_block`, "today 191-193") no longer
exists in the file; the deadline itself is an infrastructure-class
allowance with a record (`report stop-block --class infrastructure`,
362-373, detail "Metasystem Stop deadline expired before a turn verdict;
stopping is allowed with degraded infrastructure."), and the record-failure
branch prints its fixed notice without an engine (396-404). And 7c4e9cd5
gave the parent `steward hook-expire --repo` on the timeout path (349-352),
today rooted at the marker-derived `deadline_open_work_root`. At c905ca8d
the parent occupies lines 46-412: engines 61-64, worker launch 72-75,
resolver subshell 85-93 (a two-line file, session then cwd), the shell
`sed` fallbacks 94-95, `deadline_resolve_record` 100-117 with the cwd
toplevel query at 103 and the marker test at 107-111, the synchronous call
at 118, `deadline_log_stop_outcome` 119-140 with its marker test at
123-127, `deadline_log_stop_condition` 147-161, the harvest 162-175, the
resolver stop 176-187, the wait loop 191-194, the completion branch
196-259 (validation on `deadline_validator` 216-243, the fallback 244-247,
the allowance 248-251), `deadline_check_published` 265-305, the timeout
path 306-411. Revision 7 binds the parent as follows; everything revision
6 said about one engine and one installation stands.

- **The parent never blocks on its own.** The port re-introduced
  `emit_raw_stop_block` for a root that had not resolved when the worker's
  answer was unreadable (the read's M5: with an override whose `path
  state-root` sleeps two seconds, a malformed payload and an unregistered
  runtime both blocked, where trunk allows). That block is withdrawn with
  its function. Under the landed class rule an unresolvable world in the
  parent is a failure of the hook's own infrastructure, and it changes only
  where the parent can log and whether it can record: with a root, the
  stop-condition line and the outcome line go to
  `$deadline_repo/artifacts/agents/supervision/hooks.log`; without one,
  `deadline_log_stop_condition` says so in the notice with its shipped text
  ("the payload named no checkout", 154 — kept verbatim, because the
  hook-fixtures bed keys on the allowance's shape and the text is fixed
  vocabulary; the root now comes from the engine, and the sentence is read
  as "no checkout was named for this Stop"). The parent's three emissions
  stay exactly the shipped three: the worker's own answer passed through,
  the unreadable-output allowance, and the deadline allowance (with record,
  or the record-failure notice).
- **"Not answered yet" and "unresolvable" are told apart by waiting, not
  by guessing.** The resolver subshell publishes the root only after the
  engine has answered; before that the parent knows nothing. When the
  worker ends and its answer validates, the parent stops the resolver as
  today (197) and passes the answer through — no root is needed. When the
  worker ends and its answer does NOT validate (nonzero exit, empty or
  malformed output), the parent needs the root for the log line and only
  then: it keeps polling `deadline_capture_engine_coordinates` until the
  ready marker appears or `deadline_expires` passes (the worker is dead, so
  the whole remaining worker budget is free), then stops the resolver,
  logs where it can, and emits the unreadable-output allowance. On timeout
  the resolver is stopped at the deadline (308) as today: a root that
  arrived inside the worker's wait yields the record under
  `$deadline_repo` through `$deadline_engine`; a root that did not — the
  engine slow for the verb, the verb's exit 1, the engine lacking the verb,
  identification failed — leaves `deadline_record` empty and the shipped
  record-failure notice goes out (396-404). No engine call is added to the
  main flow between the worker launch and the wait loop; the wait after an
  unreadable worker is a wait on a file, bounded by the deadline the
  parent already keeps.
- **Transport: two files and a ready marker, the root exactly one line
  (m3).** The port replaced revision 6's two-line resolution file with
  `resolution.session`, `resolution.root` and an empty `resolution.ready`
  written last, after an independent critique showed that a decoded session
  id carrying a newline would shift the root onto the wrong line of a
  two-line file. The page adopts that shape: the subshell writes the
  session and the root each to its own file whatever each engine call
  returned, then touches the ready marker; the harvest reads both files,
  takes a non-empty session, and takes the root only if it is non-empty and
  contains neither a newline nor a carriage return, then physically
  normalizes it (`cd -- ... && pwd -P`) and derives the record path; a root
  that fails those tests is treated as not resolved. The hook-fixtures bed
  carries the port's regression row (a session id with a newline, the
  installation's log present, no injected artifact) as the pin. The worker
  applies the same one-line rule to its own `path state-root` answer
  (revision 8: the replacement block's predicate and Decision 2's skew
  row), so the two halves of the hook accept exactly the same answers.
- **One root name in the parent.** `deadline_open_work_root` existed only
  because the marker test could make it differ from `deadline_repo`
  (106-111, 151); with the marker tests deleted it is deleted, and
  `steward hook-expire --repo`, `report stop-block --open-work-root` and
  the two log helpers all name `$deadline_repo`. `steward hook-expire`
  runs on `$deadline_engine` (the one engine), never on
  `deadline_canonical`, and only when `deadline_repo` is set.
- **The completion path's outcome line format is as shipped.** The port
  appended a `root=<status>` field to the `stop response outcome=` line;
  the hook-fixtures bed anchors that line's end (`elapsed=5[0-9]s$`, 1037).
  The field is not adopted: the trail line keeps its shipped format, and
  whether the root resolved is visible in the outcome name itself
  (`deadline-expired-allow` versus `deadline-expired-record-failure-allow`).

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
computed inside `path state-root` — by `RootForCandidate`, the validated
installation (revision 7; revision 6 wrote "`templateMode` and
`RootForInstallation`, `stateroot.go:157-163,100-108`", superseded by
revision 8 here) — so the shell
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
  trail sites name `$repo` and `$deadline_repo`, which are the validated
  installation in every layout (revision 8; revision 6 wrote
  "`RootForInstallation(world_installation)`: the installation in the
  self-hosting layout, the repository scope in the vendored-operator
  layout" — the scope half is withdrawn with the verb's old function, and
  on c905ca8d `up` arms the vendored installation too,
  `supervision-fixtures.sh:884-887`). The template-layout assertions member A added
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

**Where trunk c905ca8d stands on this (revision 7).** Two landings moved
the shipped shell authority's consumers since revision 6, and the page must
say what that leaves for the fold. 7c4e9cd5 made `steward hook-attempt`,
`hook-complete` and `hook-expire`, `health --hook-preview`, `steward
digest-pending` and `digest-advance` take the marker-derived `$state_root`
(c905ca8d 820, 970-997, 915, 921, 992; the parent's `hook-expire` at 350),
where revision 6 had found `hook-attempt` on the cwd toplevel. So on the
fleet's template layout the component record, the health read, the digest
cursor and the turn verdict already meet at the installation on trunk, and
hook-freshness is alive there through the shell authority. What the shell
authority still gets wrong on trunk, and the fold corrects: the worker's
refusal record (`stop_refusal_record="$repo/..."`, 1108), `supervise
watchdog-report --repo "$repo"` (1179) and `steward pending --repo "$repo"`
(1335) name the cwd toplevel; the parent's refusal record root is the cwd
toplevel (103, 115); `up`'s `--repo "$repo"` is the cwd toplevel and its
scope query runs unscrubbed (`cmd/metasystem/up.go:41-48`); the two
layouts the shell block refuses (above) stay refused; and every answer
still depends on the payload's cwd. The `missing-template` row is at
c905ca8d `supervision-hook-fixtures.sh:1181-1204`, already asserting no
`"decision":"block"` and the presence of `degraded infrastructure`; its
revision 7 assertions are exit 0, `engine missing` and `degraded
infrastructure` present, `"decision":"block"` absent, and
`[[ ! -e "$missing_template_outer/artifacts" && ! -e
"$missing_template_root/artifacts" ]]`. Before the fold the row's output is
the parent's unreadable-output allowance (the shell block's `exit 1` on
Stop, converted at 248-251), which carries `degraded infrastructure` but
not `engine missing`; after the fold it is the missing-engine allowance,
which carries both. The row therefore still discriminates, on the phrase
rather than on the decision. The two shell copies to delete sit at
c905ca8d 508-517 (worker) and 107-111 and 123-127 (parent), together with
`deadline_open_work_root`.

Case table — every hook firing resolves to exactly one world or one
defined degradation:

The table is restated by revision 7 for the verb's answer (the validated
installation, never a git toplevel) and the landed Stop contract (degraded
allowances, never a parent block); revision 6's rows are superseded row for
row.

| Layout | Installation (script location) | Result |
| --- | --- | --- |
| Flat adopted (make_repo fixtures, adopted repositories) | the repository root | the engine at the root answers: the root itself; byte-identical to today, where the installation and the toplevel coincide |
| Fleet template nested (m0b, m2, m3) | `<wrapper>/metasystem`, template marker tracked at the wrapper | the engine at `<wrapper>/metasystem/bin` answers: `<wrapper>/metasystem` — the fix, now for every consumer and independent of cwd |
| Operator nested adopted (`operator-layout` fixture: no template marker at the scope) | `<scope>/metasystem` | the engine answers: `<scope>/metasystem` — the installation `up` arms on trunk (`cmd/metasystem/up.go:141`; `supervision-fixtures.sh:884-887`, c905ca8d); revision 6's "`<scope>`" is withdrawn with the verb's old function; see Decision 4 |
| Linked delegate worktree of the fleet checkout (tracked files only, or with a builder's engine; no steward identity in the sandbox) | `<worktree>/metasystem` | mapped to `<primary>/metasystem` **before** engine work; the primary's engine answers: the primary installation |
| Linked worktree whose sandbox carries an `identity.json` (fixture-written, copied or stale — the engine never mints one there, `runner.go:578-589,594-609`) | `<worktree>/metasystem` | mapped to `<primary>/metasystem` like every linked worktree; the file is not consulted (revision 8 withdraws revision 7's exception) |
| Hook copy or terminal hook symlink inside some repository, no engine at the candidate — with or without `METASYSTEM_BIN` | anywhere | the missing-engine degraded ALLOWANCE on stop (c905ca8d line 32), exit 0; no world, no writes (revisions 5 and 7) |
| Governed installation whose engine predates this design | any supported layout | the engine/hook-skew degraded ALLOWANCE on stop, exit 0, naming the rebuild (Decision 2, revision 7) |
| Governed installation whose own engine predates this design, under a `METASYSTEM_BIN` override that is a rebuilt engine (the fleet's daily post-landing state) | any supported layout | the override answers for the installation in the worker AND in the parent: the verdict path runs, and because the override is not the enrolled engine, `up` refuses it and the refusal is disclosed as `Cause: supervision arming failed` in the `systemMessage` — beside the verdict's block when there is one, or as the allowance when there is nothing to block on (revision 9's precedence); the refusal record and the stop-condition line land under the installation; on timeout the record lands under the state world through the override (revisions 6, 7 and 9; Decision 4 `up` row) |
| Adopted installation (not named `metasystem`) inside a repository that also carries `development/metasystem-design.md` | `<repo>/<anything-else>` | the engine answers: the installation itself; today's shell marker block refuses this world with exit 1 on Stop (c905ca8d 511-513) and is deleted (revisions 6 and 7; case 16) |
| Template installation with `.local`-only configuration (no `metasystem.conf`) | `<wrapper>/metasystem` | the engine answers: the installation (the shape gate accepts `scripts/agents`); today's shell marker block refuses it and is deleted (revisions 6 and 7; case 16) |
| Hook staged outside any git repository | anywhere | identification fails → worker silent exit 0; on stop the parent answers with its unreadable-output allowance (c905ca8d 248-251), logged nowhere because no world was named |

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
Items (6) and (7) are revision 6's. **Revision 7 rewords three items to
the landed Stop contract**, since c905ca8d's header (lines 4-12) still says
"a missing engine blocks a Stop" while its line 32 allows: item (2) is
unchanged from revision 6 (revision 7's addition of an armed-worktree
exception is withdrawn by revision 8); item (3) ends "and a missing engine allows a
Stop under a fixed degraded notice, because no safe verdict can be made
and the seat is not held by its own broken infrastructure"; item (5) ends
"its exit-1 refusal is benign silence, any other failure allows a Stop
under a fixed degraded notice naming engine/hook skew and the rebuild";
item (6) adds "and never blocks on its own: an unresolvable world changes
where the parent logs and whether it records, never the allowance". The B1
guarantee ("a missing engine with an unusable TMPDIR must still exit 0
benign") is preserved: the mapping uses no temporary files, the
missing-engine allowance is a literal `printf`, and payload staging still
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

**The contract moved again, and revision 7 follows it (folds the read's
M1).** Goal stop-infrastructure-allows-the-seat-to-stop landed on 2026-09-13
(529d8a64, fed9f5d9e, 286efe41e; its page is
`plans/stop-infrastructure-allows-the-seat-to-stop-design.md`, class rule at
lines 40-60): every refusal source carries a class decided in the engine; a
failure of the hook's or the verdict's own state — "every
`record_stop_failure` site, the deadline, every `failClosedTurnVerdict`
producer, the session-stop marker's read and consume, the launcher's
bootstrap" — is *infrastructure*, and the stop is ALLOWED under a degraded
notice naming the condition and its owner, with one hook-log line and
nothing else (no seen marker, no counter, no ALL CLEAR); only the verdict's
own findings (*seat-actionable*) and *idle-with-backlog* block. Under that
landing `raw_missing_engine_stop` became the allowance at c905ca8d line 32
(`Metasystem engine missing; stopping is allowed with degraded
infrastructure. Reinstall or rebuild bin/metasystem; the steward owns
repair.`), the missing-engine fixture flipped to "must not block"
(`supervision-hook-fixtures.sh:170-176`), and the parent's conversion of an
unreadable worker into a block became the unreadable-output allowance
(248-251). The port put the block back (its `raw_missing_engine_stop` is
the pre-529d8a64 string; its fixtures say "must block"), which reverses a
landed goal — the read's M1. Revision 7 decides both rows by the landed
class rule, not by revision 5's reading of a contract that no longer
ships: **a missing engine is the shipped allowance, unchanged; an engine
that does not answer `path state-root` is an allowance of the same shape,
`raw_engine_skew_stop` as restated in Decision 1.** Skew is the hook's own
infrastructure failing — the engine is the hook's tool, and neither the
verdict's findings nor the backlog are involved — so it is the
infrastructure class by the landed definition, exactly as the missing
engine is. What the round-2 and round-4 folds asked for survives intact:
skew is not silence (the notice names the drift and the rebuild on every
Stop, and the hook-fixtures bed keys on `does not answer path state-root`),
and skew is not weaker than the missing-engine outcome (it is the same
outcome). The alternative the read's smallest fix kept — allow the missing
engine, block skew — is named here and not taken: it would make an old
engine the one infrastructure failure that holds a seat until a person
rebuilds, the exact class of refusal the landed goal retired, and on this
fleet (engines rebuilt daily, the landing note of point 8) it would hold
every seat that had not rebuilt on the morning this lands. Wido may
overrule; the page proceeds on the landed reading. "Visible" below now
means "allows and names itself in the notice", and the skew emission is a
`systemMessage` object, which the parent validates through the structured
path's second predicate (decision and reason absent, a string
`systemMessage`; c905ca8d 240-241).

**Precedence when one Stop carries both (revision 9).** A Stop can record
an infrastructure condition (`supervision arming failed`, a health engine
with no verdict, a digest that could not be read — every
`record_stop_failure` site) and still reach a verdict with a
seat-actionable finding, an unseen plan line included. The rule the page
binds is trunk's: **the seat-actionable finding keeps its block; the
infrastructure condition neither suppresses that block nor creates one.**
Mechanically (3a6353c3, unchanged from c905ca8d at these lines): the
failure is recorded at 912 and the verdict computed at 1203-1235; every
recorded condition is appended to the hook log as
`stop-condition infrastructure <cause-code> <component> <generation>
<deadline> degraded-allow` (1132-1138, called at 1241-1245); the response
goes through `compose_failed_stop` (1292-1293, 1005-1104), which first
calls `external_stop_json` — `report stop-block --class infrastructure
--refusal-record "$stop_refusal_record" ...` (862-866), which updates the
stop-refusal record under `$repo` with the cause and its occurrence count
and returns the fixed text `Metasystem allowed stopping with degraded
infrastructure (occurrence N). Cause: <cause> Remedy: <remedy>`
(`stopblock.go:176-184`) — and then, when the verdict blocks, renders
`stop_block_json` with the verdict display as the `reason` and that text,
the extras and the check-in tail as the `systemMessage` (1088-1094); when
the verdict does not block, the same text leads a `systemMessage`-only
allowance (1095-1103). So a firing with open work and a failed `up` is a
block whose reason quotes the plan line and whose `systemMessage` opens
with the infrastructure notice; a firing with a failed `up` and nothing to
block on is the allowance; and the once-per-line marker is spent by the
block exactly as by any other (the verdict verb marks the line before it
decides, `cmd/metasystem/goal.go:646-656`). The fixed text "allowed
stopping" inside a block is trunk's wording and is not this design's to
change. This is the precedence the stop-infrastructure page states
("seat-actionable ... block as today"; its page, lines 52-54), and nothing
in this design's resolver changes it; what the resolver changes is only
WHERE the record and the log line land — under the installation.
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
| engine executable test at the world: `-x "$canonical"` AND `-x "$ms"` | no engine at `<installation>/bin/metasystem`, or an override that is not executable | **the shipped missing-engine degraded ALLOWANCE on stop** (`raw_missing_engine_stop`, c905ca8d 32), exit 0 — revision 5 wrote "BLOCK" against the contract then shipped; 529d8a64 changed the literal and revision 7 follows it; the candidate's own engine is required whether or not `METASYSTEM_BIN` is set (SHR-R4-COPIED-HOOK-OVERRIDE-01) |
| `path state-root "$world_installation"`, exit 1 | the verb's own refusal: `$world_installation` fails the shape gate (no `metasystem.conf` and no `scripts/agents` directory) | worker silent exit 0; on stop the parent's unreadable-output allowance (c905ca8d 248-251), whose stop-condition line lands under `$deadline_repo` when the parent's own resolver answered and is said in the notice otherwise |
| `path state-root "$world_installation"`, any other nonzero exit, or exit 0 with an answer that is empty or not exactly one line (a newline or carriage return inside it; revision 8, the design read's M5) | verb absent from an older engine (verified exit 2), family absent, a usage refusal (exit 2), or a broken answer — including a broken override that prints a directory name carrying a line break, which the worker must not `cd` into and judge | **the engine/hook-skew degraded ALLOWANCE on stop** (`raw_engine_skew_stop`, a literal `printf` of the same shape as the missing-engine allowance, naming the rebuild), exit 0 (SHR-R4-FAIL-CLOSED-REGRESSION-01 as re-read under 529d8a64, revision 7); the worker and the parent apply the one-line rule identically, so no world the parent would reject is ever judged by the worker |
| final `repo` physical normalization | directory vanished or unreadable | worker silent exit 0; on stop the parent's unreadable-output allowance |
| evidence-trail `mkdir -p "$supervision_dir"` (HEAD 580, reachable with a write-denied primary from a sandboxed worktree session) | permission denied | gains `2>/dev/null || true`; the appends that follow already carry their own guards (HEAD 582, 613-614, 640-641); at c905ca8d the sites are 962-965 and 1129-1135, and 1129's `mkdir` already reports its failure through `hook_log_failure` |
| parent: `hook_world_installation` (SHR-R4-DEADLINE-PARENT-01) | any identification or mapping failure | `deadline_engine` and `deadline_canonical` empty; completion takes the no-validator fallback (c905ca8d 244-247) and otherwise the unreadable-output allowance (248-251), timeout takes the record-failure notice (396-404) — the shipped shape for an unresolvable world; no block |
| parent: `-x "$deadline_canonical" && -x "$deadline_engine"` (revision 6) | no engine at the mapped installation, or an override that is not executable | `deadline_engine` empty; same as the row above; the worker in the same state emitted the missing-engine allowance, which the fallback accepts byte-for-byte (246) |
| parent resolver subshell: `"$deadline_engine" path state-root "$deadline_installation"` (revisions 6 and 7) | exit 1 (ungoverned), any other failure, an answer that is empty or not one line, or no answer before the deadline (a slow engine) | the root file is empty, invalid or never published; `deadline_repo` and `deadline_record` stay empty → on timeout the record-failure notice (396-404), never a record under a cwd-derived root and never a wait in the parent's main flow; after an unreadable worker the parent waits for the file until the deadline before it allows (Decision 1, parent subsection); the worker in the same world already allowed its completed Stops with the skew notice, so the drift is visible before the first timeout |
| parent: `"$deadline_engine" report stop-block --class infrastructure` on timeout (revisions 6 and 7) | the one engine cannot write the record | `deadline_record_failure` set → the record-failure notice, as today (366-373, 396-404); the canonical engine writes nothing in an overridden turn |
| parent: structured validation of the skew notice (revisions 6 and 7) | the one engine's `json get` or `json strip` fails on the literal | the unreadable-output allowance (248-251), as for any unparseable worker answer; a real pre-verb engine answers both verbs, and case 7's stub forwards them, so the notice validates as a lone `systemMessage` (240-241) and passes through |
| parent: the worker's answer is unreadable and the root has not been published yet (revision 7, the read's M5) | the worker died fast (exit 2 on an unregistered runtime, a malformed payload, an early abort) while the engine was still starting for `path state-root` | the parent waits on the ready marker until `deadline_expires`, then logs the stop condition where the root allows and emits the unreadable-output allowance; it never emits a block, and the port's `emit_raw_stop_block` is deleted |

Worker silence and the parent: every "worker silent exit 0" row above is
the worker's own behavior; on a Stop the shipped parent converts an empty
worker answer into its generic block (`emit_raw_stop_block`, today
191-193; the contract stated at 286-288). Revision 5 inherits that
conversion unchanged — it is the fail-closed floor beneath every silent
row — and the "exactly two fixed outputs" sentence below counts the
worker's own emissions. **Revision 7:** on c905ca8d that floor is the
unreadable-output allowance (248-251), not a block; every silent row
above inherits it, and the count of the worker's own emissions is
unchanged at two.

How the parent sees the two literals (revision 6): the missing-engine
literal is emitted by a worker with no `deadline_engine` counterpart in
the parent, so it reaches the no-validator fallback and is accepted
byte-for-byte (today's line 189). The skew literal is emitted by a worker
whose two executable tests passed, so the parent in the same world has
`deadline_engine` and validates the literal through the structured path
(today 159-186): decision `block`, a non-empty string reason, no
`systemMessage`, nothing else — which the literal satisfies. The fallback
therefore never needs to know the skew literal, and revision 5's widening
of it is withdrawn. **Revision 7:** both literals are now `systemMessage`
objects. The missing-engine allowance still reaches the fallback and is
accepted byte-for-byte (c905ca8d 246). The skew allowance validates through
the structured path's second predicate — decision and reason absent, a
non-empty string `systemMessage`, nothing else (240-241) — which it
satisfies; case 7's stub forwards the `json` verbs, so the parent passes the
worker's bytes through unchanged.

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
existing missing-engine block (HEAD 229-233). **Revision 7 restates the
legs under the landed contract:** a too-strict resolution degrades to
silence, which the parent turns into the unreadable-output allowance; a
guessed resolution recreates this defect somewhere else; and drift in a
governed world allows the Stop under a notice that names itself and the
rebuild. The one new output is the skew allowance, on the same channel, in
the same shape, and under the same stop-only condition as the missing-engine
allowance (c905ca8d 456-463).

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

**Three fixture rules added by revision 7**, each forced by trunk c905ca8d
and each binding on every case below and on every bed row the sweep
touches:

- **One plan line per block (the read's M3).** The block-once rule is no
  longer per session. `report turn-verdict` marks every open plan line it
  sees in a durable record under the state world
  (`report.MarkOpenWorkSeen`, `internal/report/openwork.go:271-323`, the
  file `<root>/artifacts/agents/supervision/open-work-seen.json`,
  `openwork.go:221-223`; called from `cmd/metasystem/goal.go:646-656`), a
  line already marked is `PreviouslyRefused` and drops out of the
  open-work signature (`internal/goal/turnverdict.go:88-96`), and the
  verdict blocks on open work only when the signature holds an unmarked
  line (`turnverdict.go:947-959`: "marked plan lines do not receive another
  refusal in any session"). The mark is forgotten only when the line
  changes. So a second Stop on the same `Next step:` line does not block,
  whatever its session id, and the read saw the port's nested-root bed fail
  at case 2 for exactly this. The rule: **every case that asserts
  `"decision":"block"` writes its own `Next step:` line into the world it
  expects to answer, immediately before firing, and asserts the block
  quotes that line.** The session id stays fresh per case (the shipped
  per-session state still exists), but the plan line is what earns the
  block. A sentinel per case is also the stronger identification: a block
  quoting `<case> sentinel` proves both which world answered and which
  firing it answered. The sentinel texts below are the case names.
- **Arming is a fidelity choice, not a block precondition (restated by
  revision 9).** Revision 8 wrote here that a recorded arming failure
  routes the Stop through `compose_failed_stop` "— an allowance carrying
  the verdict display, never a block", and required every blocking world
  to be armed first. The "never a block" was wrong: `compose_failed_stop`
  preserves a blocking verdict and attaches the infrastructure notice as
  the `systemMessage` (Decision 2, precedence paragraph; 3a6353c3
  1088-1094), so an unarmed world's Stop with an unseen plan line blocks,
  quoting the line, with `Cause: supervision arming failed` in its
  `systemMessage`. Revision 6's "the fixture world is unenrolled, so arming
  fails, and the assertions are only the block reason" was therefore
  possible all along. The rule now: a case that asserts a block asserts
  `"decision":"block"` quoting its sentinel, and says whether it expects
  the arming-failure `systemMessage` (an unarmed world, or a refused
  override) or not (an armed world). The `nested-root` scenario keeps
  enrolling its world with the bed's `enroll_fixture_engine` (c905ca8d
  `supervision-fixtures.sh:552-562`) and arming it by a held main through
  `live_arm_driver` (566-575) before its first firing, as `stop-hook-monitor`
  already does (2715) and as the port's bed does — for fidelity to the
  fleet (the health roles, the re-arm notice, the worktree firings'
  advisor standing against an armed primary) and so that no case but 9 and
  16 carries an arming failure in its output; a worktree case arms nothing
  itself. Case 16's worlds stay unarmed on purpose, and case 9's override
  is refused by the enrollment gate on purpose: both assert the block
  with the disclosed failure.
- **No bed fires the checkout's own hook, on any event (the read's M2).**
  With cwd inert the checkout's own `$hook` resolves the checkout's own
  installation — on this machine a linked worktree's copy maps to the live
  seat `agentic-tools-m1c/metasystem` — so a brain `start` row that fires
  `$hook` runs `up` and `brain boot` against the seat. Revision 6 stated
  the rule for `stop` only (Decision 4, existing-fixture blast); it now
  covers every event. Every firing in `supervision-hook-fixtures.sh`,
  `supervision-fixtures.sh` and `runtime-hook-fixtures.sh` runs a hook copy
  staged in a fixture installation that carries its own `cp`'d engine, the
  way the template rows (c905ca8d `supervision-hook-fixtures.sh:300-325,
  361`) and the `line_root` rows (495-497, 612) already do. The rows to
  re-root at c905ca8d: the membership and missing-engine rows (147, 156),
  the brain `start` rows (278, 397, 413, 422, 429, 480) and the two
  no-engine `start` rows (435, 437). The implementer greps `"$hook" claude`
  and `run_brain_hook "$hook"` in that file after the edit and finds no
  hit; the same grep over the other two beds finds none. The read's m6
  (`runtime-hook-fixtures.sh` re-rooted to installed hook copies) is this
  rule applied there, not a weakening.

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
  so no waits and no owned pids. **Revision 7**: after the commit, stage
  the engine, `enroll_fixture_engine "$scope/metasystem"
  "$scope/metasystem/bin/metasystem"`, and arm the world through
  `live_arm_driver` from `$scope/development/sub` with a held main that
  stays alive for the scenario (the port's shape, released after the last
  case); the owned pid joins `owned_pids`. Every Stop of the scenario is
  fired below that held main (the port's `run_nested_holder_stop` stages
  the firing as a request the held main's driver executes, so the fake
  ancestor pid is the announced holder), which is what makes the hook's
  `up` succeed as the holder — including for the worktree cases, whose
  `up` names the primary. Each case below sets its own
  plan line with a one-line helper that rewrites the third line of the
  world's `plans/stream.md` to `- Next step: <sentinel>` before firing.
- **Case 1, session `nested-sibling`** — payload cwd
  `$scope/development/sub`, event `stop`, fired through
  `$scope/metasystem/scripts/agents/supervision-hook.sh`, plan line
  `nested-sibling sentinel`. Assert `"decision":"block"` with a reason
  quoting that sentinel. (The payload still carries a cwd because runtimes
  send one; the fix makes it inert.) **Revision 7 corrects the before/after
  claim:** on c905ca8d this firing already blocks quoting the sentinel,
  because the shell marker block resolves the template layout to the
  installation and 7c4e9cd5 moved the verdict, the attempt evidence and
  health onto that answer; the case is a pin of the fleet layout, and the
  regression the goal demanded is proven by the cases trunk still fails
  (4-13, 16) and by the `$repo` sites the sweep moves (Decision 1,
  compiled-authority subsection).
- **Case 2, session `nested-inside`** — payload cwd
  `$scope/metasystem/scripts/agents`, same hook, plan line `nested-inside
  sentinel`, same assertion against its own sentinel: two firings, one
  world, one answer.
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
  "$tmp/nested-wt/artifacts" ]]`. **Revision 7**: the plan line written
  into the primary is this case's own (`recover the primary sentinel` is
  kept as its text), and it is what earns the block under the
  once-per-line rule; the primary is enrolled and armed by the scenario's
  construction, so the worktree firing's `up` runs the primary's enrolled
  engine against the armed primary and takes advisor standing there — no
  `arming failed` line is expected or tolerated, and revision 6's sentence
  about an unenrolled world is withdrawn. Before the fold (c905ca8d) this
  firing resolves the sandbox by cwd, finds no engine and emits the
  missing-engine allowance; after it, the block quoting the primary's
  sentinel: fails before, passes after.
- **Case 5, no world** — a separate root holding copies of the hook,
  adapters, engine, and fake conf, with no git repository and no template
  marker: the hook exits 0 with empty output and creates no `artifacts/`
  anywhere under it. Under this revision the exit happens at the
  identification stage — the inverted fallback burden observed directly.
  (The existing `idle-hook` scenario at lines 1382-1400 stays the
  *governed*-idle coverage; this is the ungoverned case.) **Revision 7
  corrects the output claim:** a Stop always runs through the deadline
  parent, and the parent answers an empty worker with its unreadable-output
  allowance, so the assertions are exit 0, an output that contains
  `stop-hook-output-was-unreadable` and `the payload named no checkout`
  (the parent could name no world to log under) and not
  `"decision":"block"`, and no `artifacts/` anywhere under the root. On
  c905ca8d the same firing ends the same way (the cwd toplevel query
  fails), so this case pins, and does not discriminate.
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
  missing-engine allowance (revision 7: `engine missing` and `degraded
  infrastructure` present, `"decision":"block"` absent), exit
  0, and write nothing: no `artifacts/` under `$scope/development`, and
  no new bytes in the world's `hooks.log` or
  `stop-refusals/` from these firings. Under revision 4 the two override
  firings would have passed the shape gate, adopted `$scope` by pathname
  through the substituted engine, and blocked quoting the world's
  sentinel — so those two fail before the fold and pass after. On c905ca8d
  all four end in the shell marker block's `exit 1` on Stop (the marker is
  at `$scope`, the candidate is not `$scope/metasystem`), which the parent
  answers with the unreadable-output allowance — `degraded infrastructure`
  without `engine missing` — so the case still fails before and passes
  after, on the phrase. A copied
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
  state-root query and so wrote nothing). **Revision 7 re-specifies the
  assertions for the allowance:** exit 0; `does not answer path
  state-root` and `degraded infrastructure` each exactly once in the
  output; `"decision":"block"` absent; no `HEALTH unknown` line; no
  `stop-hook-output-was-unreadable` (the parent's own allowance, which is
  what an unvalidated answer would have produced); no `hooks.log` and no
  `stop-refusals/` under `skew_root`. The parent sees a lone
  `systemMessage` and passes it through (Decision 2). `skew_root` needs no
  enrollment or arming: the firing never reaches `up`. Against today's hook the same
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
  case 4's outcomes against this case's own plan line (`recover the steered
  sentinel`, revision 7): `"decision":"block"` quoting it, evidence
  appended to the primary's `hooks.log`, and no `bin/`
  or `artifacts/` under `$tmp/nested-wt` — plus the negative that the
  output contains no `engine missing`. Without the scrub the
  identification query returns two equal paths, the mapper keeps the
  engine-less sandbox as the world, and the turn ends on the
  missing-engine allowance quoting no sentinel, so this case fails before the
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
  **Re-specified by revision 7 (the read's M4).** The override is not the
  world's enrolled engine (`os.Executable()` inside `up` is the harness
  binary, the enrolled path is `$scope/metasystem/bin/metasystem`), so
  `up` refuses it with `ErrEnrollmentDrift` (`internal/up/up.go:532-535`,
  c905ca8d), the hook records `supervision arming failed` (907-913), and —
  **as revision 9 states the precedence** — the verdict's block on the
  unseen plan line survives that failure, which is disclosed beside it
  (Decision 2). The port widened the gate so this case could block
  without the disclosure; that widening is refused (Decision 4, `up`
  row), and the case asserts what the landed contract produces, which the
  preserved Mac output shows byte for byte
  (`artifacts/agents/suite-failures/20260913T121447Z-supervision-12059/
  nested-override.out`). Plan line `nested-override sentinel`. Assertions,
  exactly: exit 0; `"decision":"block"` present; the `reason` field
  contains `nested-override sentinel` (the verdict provably ran against
  `$scope/metasystem`'s ledger through the override); the `systemMessage`
  field contains `Cause: supervision arming failed` (read the two fields
  with `json get --value ... --field reason` and `--field systemMessage`,
  never by grepping the whole object, because the `systemMessage` is
  bounded and trimmed — the preserved output shows `[system message
  trimmed to fit the Stop refusal]` after the `up` component lines, so
  nothing after the `Cause:` line may be relied on, and the phrase
  `Infrastructure condition, not a refusal` in the detail is NOT asserted);
  `stop-hook-output-was-unreadable` absent; the refusal record exists at
  `$scope/metasystem/artifacts/agents/supervision/stop-refusals/
  nested-override.json` with `sessionId` equal to `nested-override` and a
  cause entry whose `cause` is `supervision arming failed`
  (`external_stop_json` writes it at `$stop_refusal_record`, 862-866,
  which is `$repo/...`, 1108; `stopblock.go:176-177`); the installation's
  `hooks.log` gained a line matching `stop-condition infrastructure
  supervision-arming-failed supervision-arming ` (1132-1138, the cause
  code from `stop_cause_code`, 543-545); the component record at
  `$scope/metasystem/artifacts/agents/steward/components/supervision-hook.json`
  exists; and `[[ ! -e "$scope/artifacts" ]]`. Before the fold the
  refusal record and the stop-condition line land under `$scope`, because
  `$repo` at 1108 and `$state_root`'s trail are the cwd toplevel and the
  installation respectively — the record moves, the line does not; after
  the fold both are under the installation. Fails before on the record
  location, passes after, and it is the pin for the `stop_refusal_record`
  site of the sweep and for the precedence paragraph of Decision 2.
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
    fails before and passes after. **Revision 7**: plan line
    `nested-wt-parent sentinel`, quoted by the block; the negative becomes
    "does not contain `stop-hook-output-was-unreadable`", the allowance
    c905ca8d's parent substitutes when it has no validator (244-251);
    before the fold the sandbox has no engine and the worker emits the
    missing-engine allowance, so the case still fails before and passes
    after.
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
    this case fails before and passes after. **Revision 7**: the deadline
    is an infrastructure allowance on c905ca8d (362-373; the hook-fixtures
    bed's own deadline row asserts it at 1028-1047: no `"decision":"block"`,
    `deadline expired before a turn verdict`, the outcome line
    `deadline-expired-allow`, the condition line
    `stop-deadline-expired stop-deadline`). The assertions become: exit 0;
    elapsed under the sixty-second budget (the shipped deadline is 57 s of
    worker time, c905ca8d 47-48 — revision 5's "under five seconds" was the
    fixture's 4.5 s engine, not the parent's budget); `"decision":"block"`
    absent; `deadline expired before a turn verdict` present; `could not
    update the stop-refusal record` absent; the record at the path above
    with `sessionId` equal to the session; the outcome line
    `deadline-expired-allow` in the primary's `hooks.log`; and the three
    absences. The override wrapper's sleep is `runtime list`, as in the
    shipped row.
  - **Case 12, `nested-wt-parent-steered`**: case 10 repeated with case
    8's steering exported (`GIT_DIR=$scope/.git
    GIT_WORK_TREE=$tmp/nested-wt`); same assertions as case 10, against its
    own plan line `nested-wt-parent-steered sentinel`.
  - **Case 13, `nested-wt-parent-steered-timeout`**: case 11 repeated
    with the same steering; same assertions as case 11, including the
    record location. Steering would have sent the cwd-derived record root
    to the worktree; the state-root verb, computed for the mapped
    installation under the scrub, sends it to the primary.
  - **Cases 11b and 13b, `nested-wt-parent-timeout-slow-root` and its
    steered twin (revision 7, the read's M5).** Cases 11 and 13 repeated
    with an override whose `path state-root` also sleeps past the deadline
    (the wrapper honours a second switch, sleeping on `path state-root`
    before it `exec`s). The parent's root never arrives: assert exit 0,
    elapsed under the budget, `"decision":"block"` absent, `could not
    update the stop-refusal record; stopping is allowed` present, no
    refusal record for the session anywhere under `$scope`,
    `$scope/metasystem` or `$tmp/nested-wt`, and the three absences. This
    is the "not answered" half of M5 pinned on the timeout path: an
    unresolved root is the shipped record-failure notice, never a block
    and never a guessed record path. The port's bed carries both cases
    already (its `run_nested_timeout` with the slow-root switch); the page
    adopts them.

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
  line 234). (Revision 7: read "case 11's and 13's as re-specified above"
  — the allowance shape, the record, the outcome line, the absences — with
  the engine swap and the compatible override unchanged; the verb's answer
  for `$scope/metasystem` is the installation, no longer "template
  mode".) Under revision 5 the parent asked the stub for the state
  root, got exit 2, left `deadline_record` empty, and emitted exactly that
  allow line with no record anywhere, so both cases fail before the fold
  and pass after. Cases 10 and 12 (completion) are unchanged and run
  before the engine swap; they already exercise the override as the
  parent's validator, and their outcome does not depend on which engine
  derives the state root, which is why they could not carry this pin.
  After case 13 nothing in the scenario fires the primary's engine again;
  the stub is left in place. **Revision 7**: cases 11b and 13b follow 13
  with the stub still in place; after 13b the scenario copies the harness
  engine back over `$scope/metasystem/bin/metasystem` (the port does this
  at its line 1317) so that case 16, which follows, runs against a
  working primary, and the held main is released at the scenario's end.

- **Case 14, session `nested-freshness` — hook-freshness alive on a nested
  checkout (revision 6; this goal's DONE).** Fired in the `nested-root`
  scenario immediately after case 3, before any worktree case and long
  before the case-11 engine swap, through
  `$scope/metasystem/scripts/agents/supervision-hook.sh` on `stop` with
  payload cwd `$scope/development/sub` (case 1's payload, fresh session).
  **Re-shaped by revision 8 (the design read's M3) so that it
  discriminates:** it is the FIRST Stop of the scenario, before case 1, and
  its payload cwd is `$scope/development/inner` — a distinct Git checkout
  created after the wrapper's commit (`git -C "$scope/development/inner"
  init`, one commit of a placeholder file, no template marker, no
  `plans/`, left untracked by the wrapper). On c905ca8d the hook resolves
  by that cwd: `repo` is the inner checkout, the marker is absent there,
  `state_root` is the inner checkout, and the attempt record, the trail and
  the verdict all land under `$scope/development/inner/artifacts/` — the
  verdict finds no plans there and cannot quote the installation's
  sentinel, and `health` at the installation finds no record and says
  `hook-freshness=dead`. After the fold the cwd is inert and everything
  lands at the installation. The added assertion is
  `[[ ! -e "$scope/development/inner/artifacts" ]]`; the rest are as
  written below, read at the installation. Fails before, passes after,
  without changing the production design; this is the fixture the goal's
  DONE names. Case 3's trail assertion still runs after cases 1-2.
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
  **Revision 7**: plan line `nested-freshness sentinel`, quoted by the
  block; the DONE claim rests on the component record and the health line,
  and the block is the turn-completing side. Revision 7 also wrote that the
  case pins rather than discriminates, because with case 1's payload cwd
  trunk's `steward hook-attempt` already takes `$state_root` (820, moved by
  7c4e9cd5) — true for that cwd, and the reason revision 8 moved the cwd
  into a distinct nested checkout, where trunk's shell authority follows
  the cwd and the compiled authority does not. It is still the first
  fixture that reads hook-freshness on a nested layout.
  The world is enrolled and armed by construction, so `health` reports the
  supervision roles as well; only `hook-freshness` is the claim.
- **Case 15 — withdrawn by revision 8.** Revision 7 specified a "linked
  worktree the engine has armed" here; the engine arms no linked worktree
  (`runner.go:578-589,594-609`), so the case could only prove a state the
  fixture helper manufactured. The port's "governed linked worktree"
  fixture is deleted with it. The text that follows is kept as the record
  of what was withdrawn and binds nothing. *Revision 7 wrote:* After case 13
  and after the primary's engine is restored (see (m) in the self-grade),
  `git -C "$scope" worktree add "$tmp/nested-governed-wt" HEAD`; `cp` the
  harness engine to `$tmp/nested-governed-wt/metasystem/bin/metasystem`;
  `enroll_fixture_engine` that installation with that engine (this writes
  the identity record the rule reads, at the path `steward.RepoIdentityPath`
  names); arm it through `live_arm_driver` from
  `$tmp/nested-governed-wt/development/sub` with its own held main; write
  the worktree's own `metasystem/plans/stream.md` third line to `Next
  step: nested-governed-wt sentinel` (uncommitted, so the primary keeps its
  own text). Register the installation in `fixture_harness_roots`. Fire the
  worktree's own hook copy on `stop` with payload cwd inside the worktree
  and no `METASYSTEM_BIN`. Assert `"decision":"block"` quoting
  `nested-governed-wt sentinel`; evidence appended to
  `$tmp/nested-governed-wt/metasystem/artifacts/agents/supervision/hooks.log`;
  the component record under that installation; and no new bytes in the
  primary's `hooks.log` from this firing (compare a byte count taken before
  the firing). Under revision 6's rule the firing would have mapped to the
  primary and quoted no worktree sentinel; on c905ca8d it resolves the
  worktree by cwd and blocks quoting the sentinel, so this case holds
  before and after and pins the exception, not a regression. The port's
  bed carries this case as "governed linked worktree"; the page adopts it
  with the sentinel and the primary-trail negative added.
- **Case 16, the two layouts the shell authority refuses (revision 7).**
  Two firings, neither armed, each proving that the compiled authority
  accepts what c905ca8d's shell block refuses with `exit 1` on Stop
  (511-513). (a) Adopted installation under a marker-carrying repository:
  `mkdir -p "$scope/tools"`, copy the hook, adapters, `evidence-gc.sh`,
  the fake conf and a `plans/stream.md` with `Next step:
  nested-tools sentinel` into it, `cp` the harness engine to
  `$scope/tools/bin/metasystem`, `git -C "$scope" add tools` and commit;
  fire `$scope/tools/scripts/agents/supervision-hook.sh` on `stop`, cwd
  `$scope/tools`, fresh session. (b) `.local`-only template installation:
  a second scope `$tmp/nested-local` built like `nested-root` but with the
  fake conf written to `metasystem/metasystem.conf.local` and no
  `metasystem.conf`; plan line `nested-local sentinel`; fire its hook the
  same way. Both are unarmed, so `up` fails and records `supervision
  arming failed`, and — revision 9's precedence — the verdict's block on
  the unseen sentinel survives it; assert for each: exit 0;
  `"decision":"block"` present; the `reason` field contains the firing's
  own sentinel (the verdict ran against the installation's ledger); the
  `systemMessage` field contains `Cause: supervision arming failed` (read
  the fields as case 9 does); `stop-hook-output-was-unreadable` absent;
  the component record at `<installation>/artifacts/agents/steward/
  components/supervision-hook.json` exists; the refusal record at
  `<installation>/artifacts/agents/supervision/stop-refusals/<session>.json`
  exists; and no `artifacts/` at the scope. Before the fold each firing
  exits 1 on Stop and the parent emits `stop-hook-output-was-unreadable`
  with no block, no sentinel and no record; after it, the block above.
  Fails before, passes after, for the two rows of the case table that
  motivated deleting the shell authority.

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

**The sweep re-cited at c905ca8d (revision 7).** The file grew from 750 to
1384 lines between 3c753c5ef and c905ca8d, and 7c4e9cd5 moved several
sites onto `$state_root` already; the table above keeps revision 6's
meaning and this one gives the implementer today's lines. Where the site
already names `$state_root`, the edit is the rename to `$repo`; where it
names `$repo` today, the text is unchanged and the value moves from the cwd
toplevel to the installation; where it names `$harness_root` as an
argument, it becomes `$world_installation`.

| c905ca8d line | Site | After the sweep |
| --- | --- | --- |
| 423-425 | `script_dir`, `harness_root`, `ms` | `hook_world_installation`, `canonical`, `ms`, the double test (Decision 1) |
| 437-455 | the delegate hint: `"$delegate_installation_hint/bin/metasystem" lease hook-delegate --root "$delegate_state_hint" --metasystem-root "$delegate_installation_hint"` | unchanged: the adapter supplies both roots explicitly and authenticates them; it runs before resolution and does not name `$repo` |
| 456-463 | the missing-engine emission | the same `printf` behind the double test; the `start` branch's brain notice is unchanged |
| 464-472 | registry membership | unchanged |
| 474-481 | payload staging | unchanged |
| 487-503 | payload cwd, `runtime session-env`, `git rev-parse --show-toplevel` | deleted; `repo=$("$ms" path state-root "$world_installation")` with Decision 2's three-way exit map |
| 508-517 | the shell marker block: `state_root`, `template_marker`, `template_installation` | deleted |
| 561 | `lease hook-delegate --root "$state_root" --metasystem-root "$harness_root"` | `--root "$repo" --metasystem-root "$world_installation"` |
| 574, 576 | `proc find-ancestor --repo "$harness_root"` | `--repo "$world_installation"` |
| 606, 836 | `lease classify --root "$state_root" --metasystem-root "$harness_root"` | `--root "$repo" --metasystem-root "$world_installation"` |
| 672, 726, 802 | brain: `brain digest-advance --root "$state_root" --repo "$state_root"`, `brain boot --root "$state_root" --repo "$state_root"`, and the boot-failure remedy text naming `$state_root` twice | `$repo` in each; the remedy text prints the same value under its new name |
| 820 | `steward hook-attempt --repo "$state_root"` | `--repo "$repo"` — the hook-freshness site; case 14 is its proof |
| 863-865 | `external_stop_json`: `report stop-block --class infrastructure --refusal-record "$stop_refusal_record"` | unchanged text; the record path is `$repo`'s (1108) |
| 886-888, 892-893 | `up --metasystem-root "$harness_root" --repo "$repo"` | `--metasystem-root "$world_installation" --repo "$repo"`; the re-arm notice keyed at 895-906 reads this call's aggregate line and needs no edit |
| 915 | `health --hook-preview --repo "$state_root" --metasystem-root "$harness_root"` | `--repo "$repo" --metasystem-root "$world_installation"` |
| 921, 992 | `steward digest-pending` / `digest-advance --repo "$state_root"` | `--repo "$repo"` |
| 962-965 | trail directory in `emit_stop_payload`, `supervision_dir="$state_root/..."` | `"$repo/artifacts/agents/supervision"` |
| 970, 977, 984, 997 | `complete_stop_attempt --repo "$state_root"` (→ `steward hook-complete`, 946-949) | `--repo "$repo"`; the completion that makes case 14's record `OK`/`EMITTED` |
| 1108 | `stop_refusal_record="$repo/..."` | unchanged text, new value: the installation, not the cwd toplevel — case 9's pin |
| 1113, 1163, 1169, 1326 | `lease protocol-growth` / `protocol-advance` / `renew --root "$state_root"` | `--root "$repo"` |
| 1129-1135 | trail directory for the stop-condition lines, `supervision_dir="$state_root/..."` | `"$repo/artifacts/agents/supervision"` |
| 1179 | `supervise watchdog-report --repo "$repo"` | unchanged text, new value |
| 1192 | `"$script_dir/evidence-gc.sh"` | `"$world_installation/scripts/agents/evidence-gc.sh"` (m2: the design's name; the port wrote `$repo`, equal in value and changed to the name the pairing rule gives it) |
| 1203 | `report turn-verdict --root "$state_root"` | `--root "$repo"` |
| 1269, 1272, 1273 | brain status: `json get --file "$state_root/artifacts/agents/brain-status.json"`, `channel status --post --root "$state_root"` | `$repo` in each |
| 1335 | `steward pending --repo "$repo"` | unchanged text, new value |
| 1340 | `session end --root "$state_root"` | `--root "$repo"` |
| 1350-1351, 1364-1365 | `up --metasystem-root "$harness_root" --repo "$repo"` on end and start | `--metasystem-root "$world_installation" --repo "$repo"` |
| 3a6353c3 1372 (new since c905ca8d; revision 9) | `session start --root "$state_root"` on the start path | `--root "$repo"`; the ten lines at 1372-1381 are the only hook change between the two commits |
| parent 61-64 | `deadline_script_dir`, `deadline_harness_root`, `deadline_validator`, `deadline_canonical` | `deadline_installation=$(hook_world_installation)` after the worker launch, `deadline_canonical`, `deadline_engine` (set only when both executables exist) |
| parent 85-93 | resolver subshell on `deadline_canonical`: session and cwd into one two-line file | on `deadline_engine`: session into `resolution.session`, `path state-root "$deadline_installation"` into `resolution.root`, then the empty `resolution.ready` |
| parent 94-95, 100-118 | the `sed` cwd fallback, `deadline_resolve_record` with the cwd toplevel query (103), the marker test (107-111), `deadline_open_work_root`, the synchronous call (118) | the cwd `sed`, the git query, the marker test, `deadline_open_work_root` and the synchronous call are deleted; `deadline_resolve_record` takes `deadline_repo` from the harvest and computes only the slug and the record path; the session `sed` (94) stays as the shell fallback for the session id |
| parent 119-140 | `deadline_log_stop_outcome` with the marker test (123-127) | writes under `"$deadline_repo/artifacts/agents/supervision"`, no marker test, the shipped line format |
| parent 147-161 | `deadline_log_stop_condition`, `supervision_root=${deadline_open_work_root:-${deadline_repo:-}}` | `supervision_root=${deadline_repo:-}`; the fixed texts unchanged |
| parent 162-175 | `deadline_capture_engine_coordinates`: two lines into `deadline_session` and `deadline_cwd` | reads the two files; the root only if non-empty and one line; then `deadline_resolve_record` |
| parent 196-197 | on worker exit: `deadline_stop_resolver` at once | if the worker's answer validates, as today; if it does not, poll the ready marker until `deadline_expires` first (Decision 1, parent subsection) |
| parent 216-247 | validation on `deadline_validator`; the fallback (246) | on `deadline_engine`; the fallback unchanged |
| parent 248-251 | the unreadable-output allowance | unchanged; no block is added |
| parent 265-305 | `deadline_check_published` on `deadline_validator` | on `deadline_engine` |
| parent 349-352 | `steward hook-expire --repo "${deadline_open_work_root:-$deadline_repo}"` on `deadline_canonical` | on `"$deadline_engine"`, `--repo "$deadline_repo"`, only when `deadline_repo` is set |
| parent 366-373 | `report stop-block --class infrastructure ... --open-work-root "$deadline_open_work_root"` on `deadline_canonical` | on `"$deadline_engine"`, `--open-work-root "$deadline_repo"` |
| parent 382-384 | the prefix carry on `deadline_canonical` | on `"$deadline_engine"` |

The sweep leaves `$script_dir` one use, the `dirname` at line 244 that
seeds the candidate, and leaves `$harness_root` only as the candidate the
mapper starts from. `grep -n 'harness_root\|state_root\|deadline_cwd\|
deadline_validator\|deadline_harness_root' supervision-hook.sh` after the
edit shows `harness_root` inside `hook_world_installation` and nowhere
else. Revision 7 adds `deadline_open_work_root`, `template_marker`,
`template_installation` and `emit_raw_stop_block` to that grep, each with
no hit, and notes that the `$script_dir` use at c905ca8d 870
(`receipt.sh`) is a script location, not a world, and stays.

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
without writing when the fixture's caller holds no lease. **Revision 7
inverts that paragraph's direction to match trunk:** on c905ca8d the
scenario asserts census state AT the vendored installation and nothing at
the scope (`[[ -s "$operator_harness/artifacts/agents/supervision/
last-census.json" ]]` and `[[ ! -e "$operator_scope/artifacts" ]]`,
884-887), fires its Stop from inside the installation at 896-901, and
shuts down at 907. The hook's world there is `$operator_scope/metasystem`
— the installation `up` armed — so the check to repeat after the Stop is
`[[ ! -e "$operator_scope/artifacts" ]]`, not the harness one; the
read's bed run passed exactly that on the port (its M7 note). The SessionStart
ancestry bed starts at c905ca8d `supervision-hook-fixtures.sh:1206`
(`nested_outer=$tmp/nested-root`) and fires at 1237-1243.

Extension to the existing `stop-hook-monitor` scenario — **flat, deep
firing, session `t-deep`**: immediately after the block-once replay
assertion (line 1555), one more payload with `session_id` `t-deep` and cwd
`$stop_root/scripts/agents`, asserting the same block quoting `dispatch
the runner`. It must run before the scenario settles the plans to `Next
step: none` (line 1558), and its distinct session is what lets it assert a
fresh block. **Revision 7**: on c905ca8d the replay assertion is at
2736-2738 and the settle at 2741-2745; the row writes its own plan line
`Next step: dispatch the deep runner` into `$stop_root/plans/stream.md`
before firing and asserts the block quotes it, because the distinct
session no longer earns a block (the read saw the port's row fail at its
3210-3214 for this reason). With cwd inert the row proves only that a deep
payload cwd does not change the flat installation's answer; it is kept as
that pin.

## Decision 4 — blast: every consumer of `$repo` in the hook (folds SHR-R2-CONSUMER-01)

Line numbers at commit 5aad591f (unchanged from revision 2's commit for
this file). On the flat layout the resolved value is byte-identical to
today's, so every row is a no-op there. "Fleet" means the template-nested
layout; "worktree" means the mapped-primary case.

| Line | Consumer | Behavior under the new resolution |
| --- | --- | --- |
| 23-31 | engine resolution `$ms` | **New row (SHR-R2-WORKTREE-ENGINE-01).** The engine resolves at `$world_installation`, after the mapping. Non-mapped layouts: byte-identical to today (`$world_installation == $harness_root`). Worktree: the primary's engine runs the whole turn — the sandbox ships none, so revision 2's harness-anchored engine claim is withdrawn. `METASYSTEM_BIN` replaces the engine only (revision 4): `$world_installation`, every flag riding it, and the verb's argument are untouched by the override. |
| 50-66 | cwd resolution and toplevel derivation | Replaced by the Decision 1 verb call; payload cwd and the session-env fallback are deleted. `$repo` is now the engine's `RootForInstallation` answer for `$world_installation`, physically normalized — the same derivation `up` applies to the same bytes (revision 4). **Revision 8:** the engine's validated-installation answer (`RootForCandidate`), byte-equal to `$world_installation` once the gate passes; `up` applies `canonicalPath` to the same bytes (`cmd/metasystem/up.go:26-29,141`). |
| 92 | `steward hook-attempt --repo` | Takes the world directly from the flag (`cmd/metasystem/steward_verbs.go:114-123`). Fleet: attempt evidence lands beside the enrolled steward state under `metasystem/artifacts/`, so `generation`/`attemptSeq` resolve and hook-freshness revives — the goal's DONE condition. Worktree: lands in the primary trail; a write-denied sandbox degrades to the disclosed `hook_evidence_failure`. |
| 109 | `proc find-ancestor --repo` | Reads runtime adapters beneath the flag root; the wrapper root has no `scripts/agents/adapters/`, so fleet identity resolution was structurally empty. Now reads the real adapters. |
| 122, 131 | `lease classify --root "$repo" --metasystem-root "$world_installation"` | Fleet: root and metasystem-root now both name `metasystem/`, where `up`-armed sessions actually write announcements and leases (observed live). Worktree: both name the primary. Non-worktree firings pass a byte-identical metasystem-root to today's. |
| 148-155, 342, 348 (HEAD 413-420, 677-679, 690-692) | `up --metasystem-root "$world_installation" --repo "$repo"` | `up`'s state world was `stateroot.RootForInstallation(--metasystem-root)` at 5aad591f (`up.go:104-113,139-144`) and is `canonical(--metasystem-root)` on c905ca8d (revision 8 marks the old sentence; the revision 7 note at the end of this row is the binding one); in both it is independent of `--repo`, which sets the census scope as its git toplevel (`up.go:42-49,109,130`). **Revision 5 (SHR-R4-UP-GIT-STEERING-01):** that scope query ran git with the inherited environment, so the round-three steering (`GIT_DIR` at the primary, `GIT_WORK_TREE` at a worktree) moved the scope — and with it the census fingerprint, the owner scope and the owner tag prefix (`up.go:336-344,597`) — to the worktree while the state world stayed primary: a mapped turn could write primary state while re-arming it for a sandbox census scope. The claim that the query "never selects the state world" and is out of scope is withdrawn. `upRepositoryScope` now runs under the compiled authority's scrub: `stateroot.go` exports `RepositoryTop(path string) (string, error)`, a one-line wrapper returning `repositoryTop(path)` (the existing scrubbed query at `stateroot.go:42-50`; the private variable stays, so its test substitution at `stateroot_test.go:39,114,128,147` and `owner_test.go:37,111` is untouched), and `upRepositoryScope` becomes `top, err := stateroot.RepositoryTop(supplied)`, the same `--repo is not inside a git repository` error on failure, then `canonicalPath(top)`. One scrub list, one git-query implementation in the engine. Why the census scope may still differ from the state world, and what pins it: in template mode the state world is `<wrapper>/metasystem` while the scope is the wrapper toplevel — whole-repository process coverage, by design — and the scope is now exactly `RepositoryTop($repo)` under the scrub, a function of the state-root bytes alone, so the pair (state world, scope) is determined by `$world_installation` and nothing inherited. Pinned by a Go test beside `cmd/metasystem/up_test.go:36` (create a repository with a commit and a linked worktree, `t.Setenv` the two steering variables at the primary `.git` and the worktree, call `upRepositoryScope(primary)`, assert the primary toplevel) and by case 8's scheduler-entry assertion. On the fleet nothing observable changes: the scope stays the wrapper toplevel because that is the git toplevel of `metasystem/` too. Worktree: both flags point at the primary; `up` verifies the already-armed rings, a delegate session gains at most advisor standing (the up contract: a second live session receives advisor, without displacement), and a sandboxed failure surfaces as the non-fatal `up_failure` line. **Revision 7 (the read's M4 and m1):** on c905ca8d `up`'s state world is `canonical(--metasystem-root)` — `upMetasystemRoot` (`cmd/metasystem/up.go:26-29`) then `options.Root = root` after the scheduler print (138-141) — not `RootForInstallation`; the scope is still `upRepositoryScope(--repo)` (41-48, unscrubbed today) and the port's `stateroot.RepositoryTop` wrapper puts it under the scrub as this row prescribes. The verb returns the same canonical installation, so the pairing holds by construction. **The enrollment gate is not widened.** `openInvokingEnrollment` (`internal/up/up.go:520-537`) opens the enrolled binary at `options.Root`, re-arms a rebuilt engine at its enrolled path, and refuses any invoking engine whose canonical path is not the enrolled one (`ErrEnrollmentDrift`, 532-535). The port added an `EngineOverride` option (`METASYSTEM_BIN` set) that skipped that refusal and let ordinary `up` launch the supervision owner from an unenrolled binary — a widening neither revision 6 nor the goal asks for, whose only purpose was case 9's block. It is refused: which engines may arm a world is the engine-rebuild-rearm design's decision (ea8c3ead7 and its page), and an override that is not the enrolled engine is, on trunk, the disclosed `supervision arming failed` (`supervision-hook.sh:907-913`), which rides the Stop as its `systemMessage` beside the verdict's own outcome — a block on open work survives it (`compose_failed_stop`, 1005-1104, 1088-1094; revision 9's precedence in Decision 2; revision 8's "turns that Stop into the infrastructure allowance" was wrong for a firing with something to block on). The fixtures assert exactly that: case 9 asserts the block quoting its sentinel, the `Cause: supervision arming failed` line in the `systemMessage`, and the record locations; the beds' own firings run under the auditor override that `exec`s the world's enrolled engine (`supervision-fixtures.sh:80-128`), so `os.Executable()` inside `up` is the enrolled path and they arm; `stop-hook-monitor`'s expectation that the fixture-supplied engine is not rejected (2727-2731) is unchanged. A fleet seat that sets `METASYSTEM_BIN` to a rebuilt engine elsewhere sees `supervision arming failed` on its Stops until it re-arms through the enrolled path — trunk's behaviour today, not this design's to change. |
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
  actually runs. **Revision 7 re-traces this against c905ca8d
  (762-914):** the scenario now asserts the opposite placement — census
  state at the vendored installation and nothing at the scope (884-887) —
  because `up` arms `canonical(--metasystem-root)`; the hook's Stop fires
  from inside the installation at 896-901 under the auditor override with
  `METASYSTEM_SUPERVISION_FIXTURE_ENGINE=$operator_engine`. Under revision
  7 the hook's world there is `<scope>/metasystem`, the same value `up`
  armed, so the scenario's assertions survive unchanged and the after-Stop
  check to add is `[[ ! -e "$operator_scope/artifacts" ]]` (Decision 3).
  The read ran this scenario green on a build of the port carrying trunk's
  `digest.go` (its M7): the digest package (`internal/narratordigest`,
  c7397a09) maps a template toplevel to the installation and uses any other
  root as given, so `$repo` — always an installation now — is used as given
  and the answer is the same; the port's `digestRoot` hunk and its test are
  dropped in favour of trunk's file (the read's smallest fix, adopted).

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
file and re-roots every hit. **Revision 7** widens that rule to every
event (Decision 3, fixture rules) and re-cites the rows at c905ca8d: the
deadline rows already fire `$line_root`'s copy (1024-1026, 1049-1051,
1069-1071) and assert the allowance shape (1032-1047, 1054-1062,
1073-1083); the membership row (146-148) and the missing-engine row
(156-157) still fire `$hook`; the brain `start` rows (278, 397, 413, 422,
429, 480) and the no-engine `start` rows (435, 437) fire `$hook` with a
real engine and resolve the seat once cwd is inert — every one moves to a
staged copy in a fixture installation with its own engine, as the brain
template rows already do (300-325).

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
operational residue already listed. Revision 7 adds: the enrollment gate
of `up` (which engines may arm or re-arm a world is the
engine-rebuild-rearm design's, and the port's `EngineOverride` is
refused); the classes of the Stop contract (infrastructure allows,
seat-actionable and idle-with-backlog block — the stop-infrastructure
design's, applied here, not amended); the trail line formats (the port's
`root=` field is not adopted); the fixed texts of the parent's notices
(kept verbatim so the hook-fixtures bed's keys hold); the semantics of
`RootForInstallation` and `templateMode`, which stay for the
executable-anchored writers and stop being the hook's function; and the
narrator digest's root mapping (c7397a09), taken as trunk has it.

### Where each round-5 finding lands

| Finding | Section that answers it | What changes | What proves it |
| --- | --- | --- | --- |
| SHR-R5-DEADLINE-OVERRIDE-ENGINE-SPLIT-01 | Decision 1, "One engine in the parent"; Decision 2 failure map (parent rows); Decision 4 parent rows | `deadline_engine` replaces `deadline_validator` and is the only engine the parent runs; the state-root query moves into the deadline-killed resolver subshell; `report stop-block` runs on it; `deadline_canonical` is the executable test only | cases 11 and 13 re-pinned: an old canonical engine at the primary (the case-7 stub) and a compatible slow override; the record lands under the primary and the allow line is absent; both fail under revision 5 |
| SHR-R5-SKEW-FIXTURE-VALIDATOR-01 | Decision 3 case 7; Decision 2 ("How the parent sees the two literals"); Decision 1 parent subsection (fallback as shipped) | the stub refuses exactly `path state-root` with exit 2 and forwards every other verb, so the parent validates the literal structurally; the fallback's skew acceptance and self-grade (k)'s second firing are withdrawn | case 7 observes the skew literal's own text once, no generic block, no trail, no record; it fails against today's hook, which never calls the verb |
| SHR-R5-TEMPLATE-MARKER-SECOND-AUTHORITY-01 | Decision 1, "The compiled authority is the only one"; header contract item (7); Decision 3 sweep; Decision 4 rows for today's 293-306 and 95-109 | the worker's marker block and the parent's marker test are deleted; `state_root` disappears; every consumer names `$repo` / `$deadline_repo`, the engine's answer | the repurposed `missing-template` fixture asserts `engine missing`, no `HEALTH unknown`, no writes; member A's template trail assertions and case 3 (self-hosting), the operator-layout after-Stop check (vendored); the case-table rows for the two layouts the shell block refused |
| carry-over: hook-freshness dead on nested checkouts | Decision 3 case 14 and the consumer sweep (line 337) | `steward hook-attempt --repo "$repo"` writes under the state world because `$repo` is that world | case 14: the record at the nested installation, none at the wrapper, and `hook-freshness=alive` from `health --repo "$(path state-root ...)"` — this goal's DONE |
| carry-over: operator-layout check before the Stop only | Decision 3, after the sweep table | the `[[ ! -e "$operator_harness/artifacts" ]]` assertion is repeated after the hook Stop | the scenario itself; traced read-only through the collector's lease gate |
| new fact: the re-arm notice (ea8c3ead7) | Decision 4 row for today's 442-445, 474-475, 730-742 | none; consistency stated | the row |

### Where each revision-7 point lands

| Point (read finding) | Section that answers it | What changes | What proves it |
| --- | --- | --- | --- |
| 1. missing engine and skew (M1) | Decision 2 ("The contract moved again"); Decision 1 replacement block, skew literal, header items (3) and (5); case table | both outcomes are fixed degraded allowances; `raw_missing_engine_stop` as c905ca8d 32; `raw_engine_skew_stop` restated as a `systemMessage`; no block anywhere in the resolver | the shipped missing-engine row (170-176) unchanged; the `missing-template` row on `engine missing`; case 6; case 7 on `does not answer path state-root` with no block |
| 2. blocks under once-per-plan-line (M3) | Decision 3, fixture rules and every case | one plan line per block, written before the firing, quoted by the block; case 14's DONE on the record and the health line | cases 1, 2, 4, 8, 10, 12, 14 each on its own sentinel (revision 8: case 9 is an allowance and case 15 is withdrawn); the t-deep row on its own line |
| 3. bed firings run from a staged installation (M2) | Decision 3, fixture rules; Decision 4 existing-fixture blast | no bed fires `$hook` on any event; nested-root is enrolled and armed for fidelity (revision 9: not as a block precondition) | the post-edit greps; the brain rows on a staged copy; nested-root armed by construction |
| 4. override and enrollment (M4) | Decision 4 `up` row; case 9; out of scope | `EngineOverride` refused; the gate as landed | case 9 asserts the block quoting its sentinel with `Cause: supervision arming failed` in the `systemMessage` (revision 9), the refusal record and the stop-condition line under the installation, nothing at the scope |
| 5. the parent's resolver timing (M5) | Decision 1, "The parent on trunk c905ca8d"; Decision 2 parent rows | the parent never blocks; after an unreadable worker it waits on the ready marker until the deadline; on timeout an unresolved root is the record-failure notice | cases 11b and 13b; the read's reproduction (a malformed payload and an unregistered runtime under a slow `path state-root`) now allows as trunk does |
| 6. linked worktrees the engine armed (M6) | Decision 1, linked-worktree rule; replacement block; case table | revision 7's identity-record exception; **withdrawn by revision 8** — the engine never arms a linked worktree (`runner.go:578-589,594-609`), the mapping is unconditional, and the ownership question goes to Wido | cases 4, 8, 10-13 pin the mapping; case 15 withdrawn |
| 7. verb answer and transport (m1, m2, m3) | Decision 1 verb contract and pairing rule; parent subsection; Decision 4 `up`, collector and operator-layout rows; the sweep re-cited | `RootForCandidate` returns the validated installation; `$repo == $world_installation`; the collector at `$world_installation`; two resolution files and a one-line root | the operator-layout scenario; case 9's record location; the deadline newline regression row in the hook-fixtures bed |
| 8. landing order and docs (m7, m8) | "Landing and documentation" below | the landing note and the one docs paragraph | the note's text; the paragraph's text |

## Landing and documentation (revision 7)

**The landing note (the read's m7; the three states of the design read's
M4).** The hook calls a verb the enrolled engines of every seat lack until
they rebuild, and the hook itself and the engine change together, so a
seat passes through three states and the note names each: (1) *old hook* —
the seat has not synced; it runs the cwd-and-marker hook, never calls
`path state-root`, and behaves as today, with no skew detection at all;
(2) *new hook, old primary engine* — the seat has synced but not rebuilt;
every Stop ends in the skew allowance (the seat can stop, the notice names
the rebuild, nothing is recorded for that turn), and so does every Stop of
a delegate worktree created from the new trunk, because it maps to the
primary and calls the primary's old engine; (3) *new hook, rebuilt and
re-armed primary* — governed behaviour. The instruction, in order: sync,
rebuild `bin/metasystem` (`scripts/agents/go-build.sh` or the seat's build
recipe) BEFORE the first Stop, then re-arm once by hand as the three
stop-infrastructure landings already required ("Every seat rebuilds and
re-arms once by hand after this lands", 529d8a64 and fed9f5d9e). Delegates
need no engine build of their own — they run the primary's — but a
worktree in flight that carries the OLD tracked hook keeps state (1) until
its tracked hook is refreshed (a rebase or checkout of the new hook file),
whatever the primary's engine is; the primary's rebuild alone gives it
nothing. A seat that runs under `METASYSTEM_BIN` keeps the enrolled engine
at the installation's own `bin/metasystem`, or its Stops report
`supervision arming failed` (Decision 4). Had point 1 gone the other way
(skew as a block), state (2) would have been a seat that cannot stop until
it rebuilds — the reason the order matters is the same under both readings,
and the cost of forgetting it is what the reading decides.

**The one docs paragraph (the read's m8).** The port added a paragraph to
`docs/orchestration.md` (in the launched-run list, after the `metasystem up`
paragraph) that called the identity rule "unarmed" and omitted the two
outcomes. It says, in plain words and in this order: the hook starts from
the physical installation its script sits in and identifies that checkout
with inherited Git steering removed; a linked worktree always maps to the
same installation beneath its primary checkout, because the primary
checkout owns the watchdog and the engine arms no linked worktree
(revision 8; revision 7's exception for a worktree carrying an identity
file is withdrawn); the installation must carry its own `bin/metasystem` even
when `METASYSTEM_BIN` runs another engine; the running engine validates the
installation through `metasystem path state-root <installation>` and that
answer is the one state world the hook and its Stop-deadline parent use;
and the two outcomes a seat can meet on a Stop from this resolution are
both degraded allowances, never refusals: "Metasystem engine missing" when
the installation has no engine, and "Metasystem engine and hook are out of
step" when the engine predates the verb — each names the rebuild, and the
steward owns repair. The word "unarmed" does not appear; "governed" is
used only for a world the engine has armed.

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

Revision 7 was re-read end to end against itself after the eight points:
the missing-engine and skew outcomes are degraded allowances in the
revision record, Decision 1's replacement block and skew literal, the
header contract items (3) and (5), the case table, Decision 2's rule
paragraph and failure map, cases 6 and 7, the `missing-template` row, the
landing table and the docs paragraph, and every sentence of revisions 5 and
6 that says "block" for either outcome is either quoted as history or
followed by the revision 7 sentence that supersedes it; the parent never
blocks on its own in the parent subsection, the failure map's parent rows,
the sweep's parent rows and cases 10-13b, and `emit_raw_stop_block` appears
only as the name being deleted; the verb returns the validated
installation in the verb contract, the pairing rule, the case table, the
operator-layout paragraph and the `up` row, and no surviving sentence has
the hook's world at a git toplevel; the identity exception is stated once
in the linked-worktree rule and appears in the replacement block, the case
table, header item (2), case 15 and the docs paragraph with one meaning
(revision 8 withdraws that exception at every one of those places; its own
pass below records the withdrawal);
the three fixture rules govern every case that asserts a block, and each
such case names its own sentinel; `deadline_open_work_root`,
`template_marker`, `template_installation` and `emit_raw_stop_block` join
the names that must not survive; and the "fails before, passes after"
claim is made only where c905ca8d actually fails (cases 4, 6-13b and 16,
the `missing-template` row and case 6 on the phrase, case 9 on the record
location) and is corrected to "pins" where trunk already holds (cases 1-3,
5, 14, 15).

Revision 8 was re-read end to end against itself after the fold: the
linked-worktree mapping is unconditional in the rule, the replacement
block, the case table, header item (2), the docs paragraph, the landing
table and the reject condition, and revision 7's exception survives only
as quoted, superseded text (its record item 6, the withdrawn paragraph
marker, the withdrawn case 15); `RootForInstallation` appears in normative
text nowhere — every remaining occurrence is marked as the revision it
belonged to or names the executable-anchored writers this hook never
reaches — and `RootForCandidate` or "the validated installation" carries
the verb's answer in the verb contract, the pairing rule, the override
paragraph, uniqueness (b), the compiled-authority subsection, member A's
paragraph, Decision 4's cwd and `up` rows and the reject condition; the
one-line answer rule is stated for the worker in the replacement block and
the skew row and for the parent in the transport bullet, and each names the
other; case 14 fires first with its cwd in the inner checkout and is the
one case the DONE names, so the "fails before" list becomes cases 4, 6-14
and 16 and the "pins" list cases 1-3 and 5; the landing note has three
states and the old-hook worktree; the blocking-case list is 1, 2, 4, 8, 10,
12, 14; and the invariant for Wido is stated in one place, the revision
record, with the page proceeding on option A everywhere.

Revision 9 was re-read against itself after the precedence decision: the
rule "a seat-actionable finding keeps its block; an infrastructure
condition neither suppresses nor creates one" is stated once, in
Decision 2's precedence paragraph, and every place that touches a Stop
carrying both now agrees with it — the revision 7 record items 3 and 4
(marked), fixture rule 2 (arming a fidelity choice), case 9 and case 16
(blocks with the disclosed failure, read field by field), the case table's
override row, Decision 4's `up` row, the landing table's rows 3 and 4, and
the reject condition; "never a block" survives only as the quoted sentence
revision 9 withdraws; and the blocking-case list of the landing table is
now 1, 2, 4, 8, 9, 10, 12, 14, 16 — case 9 and case 16 block, with the
`systemMessage` disclosure, which the table's row 2 does not enumerate
because it lists the cases whose sentinel earns a block, and those two
earn it the same way.

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

Revision 7 grounding: every "c905ca8d" citation was read in this worktree
at that commit — the hook's header (4-12), the allowance literal (32), the
parent (46-412, at the lines the parent subsection lists), the worker's
resolution and marker blocks (423-517), the delegate hint (437-455), the
arming and health path (878-921), the stop composer (849-866, 1005-1035),
the verdict path (1195-1329) and every consumer site the sweep re-cites;
the landings 529d8a64, fed9f5d9e, 286efe41e, 7c4e9cd5 and c7397a09 as
committed, and the stop-infrastructure design's class rule (its page,
lines 40-60); `cmd/metasystem/up.go:26-29,41-48,103-141`,
`internal/up/up.go:142-147,520-537`, `internal/stateroot/stateroot.go`
whole (`RootForInstallation` 111-120, `ResolveLayout` 126-173,
`installationShape` 209-213, `installationRoot` 241-259, `templateMode`
261-267), `internal/steward/install.go:12-14`,
`internal/report/openwork.go:221-223,262-323`, `cmd/metasystem/goal.go:
630-660`, `internal/goal/turnverdict.go:86-96,901-959`; the beds
(`supervision-hook-fixtures.sh:78-176,222-325,361,435-437,480-497,
1003-1083,1181-1204,1206-1243`; `supervision-fixtures.sh:78-128,552-575,
762-914,2700-2790`); the port's diff (`git diff HEAD -- metasystem`, 14
files) and its hook (`hook_world_installation` 49-91, the parent from
104, read through 440);
and two observations on this machine on 2026-09-13: `git worktree list` for
`agentic-tools` and `agentic-tools-m1c` with a stat of each checkout's
`metasystem/bin/metasystem` and `metasystem/artifacts/agents/steward/
identity.json` (seven linked checkouts with an engine and no identity; the
five seats primaries with both), and the same stat over the 221 delegate
worktrees (85 with an engine, none with an identity). Nothing was executed
against a built engine in this round beyond `git` reads; the read's bed
runs on the port (hook bed exit 1 at the brain row, nested-root exit 1 at
case 2, stop-hook-monitor exit 1 at the t-deep row, operator-layout exit
0) are taken as recorded. Residual risks added: (p) *withdrawn by
revision 8 with the exception it described* — the identity test is gone,
and no runtime file decides the mapping; (q) the parent's wait for the
resolver after an unreadable worker is bounded by the deadline and costs
nothing on the ordinary path; the worker and the resolver start together
and share one absolute `deadline_expires` (c905ca8d 76, the wait loop at
191-194), so the wait
can consume only the remainder of the same 57-second window, never a
second budget (revision 8, the design read's m2) — visible in the outcome
line, never a block; (r) the skew allowance means a seat can stop through
an old engine indefinitely if it ignores the notice: the hook exits at
`path state-root` before any verdict, so the turn is unjudged, and today
there is NO durable repair signal — the health line carries no role for
engine skew, case 7 requires no log line and no record, and the steward's
incident record and drain for repeated infrastructure allowances belong to
member stop-incidents-reach-the-steward, not yet landed (the
stop-infrastructure design, section 3, "What does not change"); until that
member lands the per-Stop notice is the only visibility, and this design
adds none (revision 8, the design read's m1); (s) *revised by revision 8*:
cases 1-3 pin rather than discriminate on the fleet layout; case 14 now
discriminates through its inner-checkout cwd; the two `$repo` read sites
(watchdog, pending) have no fixture of their own — their wrong root on
trunk produced an empty answer, and none is added; (t) the delegate census
is one machine on one day; under revision 8 it no longer carries a rule,
only the reason the mapping is not engine-gated, and a delegate that ran
`steward arm` in its sandbox would be refused by the engine
(`runner.go:606-609`).

Revision 8 grounding: the design read (codex-design-read-r7) and the code
it cites, re-read at c905ca8d — `internal/steward/runner.go:368,398-420,
560-609` (`runnerExclusion`, `EnsureRunner`, `arm`), `runner_test.go:
535-536`, `identity.go:112-150,406-420` (`MintIdentity`, `VerifyIdentity`,
`OpenEnrolledBinary`), `internal/up/up.go:520-537`, `cmd/metasystem/
up.go:26-29,103-141`, `supervision-hook.sh:76,250` for the shared deadline,
`internal/report/openwork.go:269-323` and `internal/goal/turnverdict.go:
947-959` for the re-armed refusal on a changed line, and the
stop-infrastructure page's section 3 for the incident member — and the
page's own passages the read numbered (its 393, 422, 502, 700, 1034, 1311,
1665, 1816, 1861, 2077, 2224, 2233, 2551, 2555, 2570 at the revision 7
file), each found and folded or marked.

Revision 9 grounding: at 3a6353c3, `supervision-hook.sh:543-545`
(`stop_cause_code`), 849-866 (`stop_block_json`, `external_stop_json`),
907-913, 1005-1104 (`compose_failed_stop` whole), 1132-1138
(`append_stop_condition`), 1203-1235, 1241-1245, 1292-1294, and the diff
against c905ca8d (ten inserted lines at 1372-1381, nothing else);
`internal/report/stopblock.go:170-199`; the stop-infrastructure page's
class rule (lines 40-60); the builder's report (codex-report-r3, item 2);
and the preserved output `nested-override.out` in the suite-failures
directory named in the record, read whole. Residual risk added: (u) the
`systemMessage` of a block that carries an infrastructure notice is bounded
and trimmed (`BoundSystemMessage`, `boundSystemMessageWithTail`), so the
verdict detail and the health line can be cut in the response; the
refusal record, the stop-condition line and the component record carry
the durable facts, and the fixtures assert those, not the trimmed tail.

**Reject condition — reject this design if any of the following is
shown:** a state-writing engine verb reachable from this hook whose world
is neither its explicit `--repo`/`--root` flag nor the canonical
installation named by an explicit metasystem root (revision 8; revision 4
wrote `RootForInstallation` of it) (an authority split the consumer table
missed); a supported layout in which neither the firing script's own
physical installation nor its mapped primary counterpart holds the
governing engine (the hook would go benign inside a governed world); any
path on which a governed decision rides a world that the turn's engine did
not compute, by `RootForCandidate` (revision 8; `RootForInstallation` at
revision 4), from the same installation bytes
every `--metasystem-root` consumer of that turn receives (the
SHR-R2-INSTALL-01 recreation); any turn — under a `METASYSTEM_BIN`
override included — in which the verb's answer and any shell-owned
consumer name different installations, or in which the collector runs a
different engine than the hook (the SHR-R3-ENGINE-INSTALLATION-PAIR-01
split); any git invocation of the resolver whose result an inherited
variable from the `stateroot.go:32-40` list can change, or a scrub list
that is not name-for-name that list (the SHR-R3-GIT-STEERING-01
exposure); `up` writing supervision state at any root other than
`canonical(--metasystem-root)` (revision 8; `RootForInstallation` of it at
revision 4); a linked worktree whose primary
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

**Revision 7 amends the reject condition** where the landed Stop contract
and the eight points changed it, and adds five clauses. Read "any
missing-engine or old-engine Stop that ends without `"decision":"block"`,
or any fixture weakened from that expectation" as: any missing-engine or
old-engine Stop that ends in a block, in silence, or in anything other than
its fixed degraded allowance naming the rebuild, or any fixture that
asserts a block for either (the 529d8a64 reversal recreated). Read "the
two fixed blocking reports" in the last clause as "the two fixed degraded
allowances". Added: any Stop on which the deadline parent emits a block of
its own — for an unresolved root, an unreadable worker, or a timeout — or
on which it treats a root the engine has not yet published as
unresolvable without waiting to the deadline (the M5 exposure); any linked
worktree that is not mapped to its primary, or any mapping decision that
reads a file inside the worktree — `identity.json` included — rather than
the git common-dir identity alone (revision 8, replacing revision 7's
two-direction clause: the engine arms no linked worktree, so no file there
can be its record); any `up` path that arms or re-arms a world from an
engine that is not the enrolled one because `METASYSTEM_BIN` named it (the
M4 widening); any fixture in any bed that fires the checkout's own hook on
any event, or asserts a block on a plan line an earlier firing already
marked, or expects a block from a world that is not enrolled and armed (the
M2 and M3 exposures); and any hook path on which `$repo` and
`$world_installation` differ after the verb answered, or on which the
verb's answer is a git toplevel rather than the validated installation
(the m1 split). **Revision 9 adds:** any Stop on which a recorded
infrastructure condition suppresses a seat-actionable block, or on which a
seat-actionable block hides the infrastructure condition from the
`systemMessage`, the hook log or the refusal record, or any fixture that
asserts an allowance where the world's plan line is unseen (the precedence
of Decision 2 inverted in either direction).
