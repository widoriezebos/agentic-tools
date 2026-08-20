# The wall's snapshot scope: HEAD, the index, and the whole repository

Owns HIW-O15 (CRITICAL) and HIW-O16 (HIGH) from
`plans/host-implementer-wall-design.md` as one snapshot-scope design:
the two obligations fail for the same reason — the wall judges exactly
one observable (the worktree projection, workspace-scoped) while a
repository has three ways to carry bytes (worktree, index, committed
history) and, in a nested checkout, a whole repository around the
workspace. Design only; no implementation in this pass.

- Status: PARKED AT CRITIQUE EXHAUSTION — waiting on the human. Six
  rounds ran (trajectory 12, 13, 10, 5, 5, 6 material findings; the
  declared failsafe at round 4 held; rounds 4-6 were the second
  focused budget). Round 6 returned six material findings, none
  fixture-expressible, and the skill's exhaustion rule stops the
  loop: a human decision recorded here is the only remedy. The six
  open findings are WSS-R6-01..06
  (`plans/wall-snapshot-scope-critique-r6.md` records each with an
  assessed remedy sketch, deliberately NOT folded — the loop is
  closed). The decision the cluster actually needs, stated plainly:
  WSS-R6-01 proves any finite probe chain has a last probe, so
  either the wall accepts a BOUNDED-AND-RECORDED detection window as
  its posture (each verification records its capture instant; later
  motion lands in the next probe or the next admission — consistent
  with the detector tier) or it gets repository-wide custody during
  acceptance, which is isolation-tier machinery this design was told
  not to build. The other five findings are mechanical folds once
  that ruling lands.
- Loop discipline, declared AT LOOP START (D119, IL-25): codex
  gpt-5.6-sol, reasoning xhigh, read-only sandbox, both-must-agree.
  THE FAILSAFE ROUND IS ROUND 4. Early close: the first round whose
  material findings are all fixture-expressible closes the loop and
  those findings land as obligation rows. After round 4,
  fixture-expressible findings become matrix rows without reopening
  prose; only a demonstrated requirement failure (an O15/O16 attack
  the design admits) or a shape-level defect reopens prose. Rounds
  4-6 form the one budget extension; exhaustion parks.
- Goal: host-implementer-wall (Current).

## Fact sheet (verified anchors; a mechanism claim without an anchor is a review defect)

- The snapshot primitive projects THE WORKTREE ONLY, through an
  isolated index: `internal/gittree/gittree.go:188` (Snapshot: seeds
  a throwaway index from a caller-named baseline via read-tree, then
  `add -A -- .` FROM THE WORKSPACE, then write-tree, then
  subtree-scopes to the workspace prefix). The real index is never
  read; committed HEAD is never compared. The primitive's own comment
  states the nested boundary: "worktree changes outside a nested
  workspace are not the workspace's to project"
  (`internal/gittree/gittree.go:186`).
- The baseline seed matters only for tracked-and-ignored membership,
  and every wall caller passes the LIVE string "HEAD"
  (`internal/missionrunner/wall.go:383`, `wall.go:443`), resolved at
  call time — a mid-turn HEAD move changes what the post snapshot
  projects.
- Turn open records and anchors preTree and nothing else:
  `internal/missionrunner/loop.go:1478-1494` (wallSnapshot + ref
  anchor + `"preTree"` in turn.json/openTurn). No HEAD commit, no
  toplevel tree, no staged tree is recorded anywhere in the turn
  lifecycle.
- The between-turns drift check compares the worktree projection to
  the expected tree only (`internal/missionrunner/loop.go:1558`).
- Post-exit inspection snapshots the worktree only
  (`internal/missionrunner/wall.go:127`, inspectWall).
- Admission (mission start) compares the filtered worktree projection
  to HEAD's filtered tree, binds the live contract byte-and-mode to
  committed HEAD, and re-checks the core.fileMode pin
  (`internal/missionrunner/wall.go:433` admittedBaseline,
  `wall.go:424` checkFileModePinned).
- Delegates already have the whole-repository fence the host lacks:
  `internal/validate/conformance.go:263` refuseOutsideProjectChanges
  compares TOPLEVEL trees (boundary base vs repo snapshot) and
  refuses any changed path outside the install prefix, at both the
  review stage (`conformance.go:311`) and the merge stage
  (`conformance.go:477`). At a toplevel install (empty prefix) it is
  vacuous by construction (`conformance.go:271`).
- Path-space rule for every comparison: `TreeOf`/`HeadTree` return
  workspace-scoped trees (`internal/gittree/gittree.go:149-165`);
  toplevel comparisons use a toplevel Workspace exactly as
  conformance does (`conformance.go:274`).
- Merge-target reality: conformance runs from the merge-target
  checkout and computes a merge-base against ITS HEAD
  (`conformance.go:192-199`); HIW-O14 records that a lawful mid-turn
  merge can move the baseline away from committed HEAD. The lawful
  integration lane must therefore admit merge commits.
- No staged-tree primitive exists in gittree today (Snapshot,
  FilterTree, TreeOf, ChangedPaths, Diff, Apply, Anchor — no
  real-index projection).
- The gittree config pins (`internal/gittree/gittree.go:43`) pin
  fileMode, diff prefixes, apply whitespace, and reflogs — nothing
  disables git object replacement, so a replace ref currently rewrites
  what every tree and commit comparison sees.
- The only registered wall event is `wall-violation`
  (`internal/missionrunner/wall.go:690`,
  `scripts/agents/event-registry.json:783`); passes emit no event.
- The expected composition at inspection iterates the adjudicated
  `certified[]` entries (`internal/missionrunner/wall.go:186`); no
  durable pre-integration order artifact exists yet.
- Delegate worktrees branch as `refs/heads/agent/<job>`
  (`scripts/agents/dispatch.sh:933`).
- Fresh mission states are schemaVersion 3; the loader accepts {2,3}
  with a downgrade barrier, and every payload is validated by EXACT
  key sets (`internal/mission/state.go:709`, `:236`, `:166`) — new
  authoritative fields require a schema decision, never a silent
  addition.
- Consumed patches are PAIRWISE DISJOINT by the parent's own overlap
  rule (`plans/host-implementer-wall-design.md` §tree composition:
  overlap refuses unless a combined authorization is issued), so any
  subset of a turn's consumed patches composes to one well-defined
  tree, order-free.

## The invariant, extended

A mission host turn never ships implementer work — and "ship" has
three carriers, not one. The wall's judgment space becomes three
observables, each in the workspace path space with the same machine
filters (mission ledger; contract at admission):

1. the WORKTREE projection (today's equation, unchanged);
2. the STAGED projection — the workspace's staged entries, computed
   by RECONSTRUCTION (new primitive `gittree.Workspace.StagedTree()`:
   the workspace-restricted `ls-files --stage` entries loaded into a
   fresh isolated index and write-tree'd there; the worktree and the
   real index untouched — the full contract in §new primitives);
3. COMMITTED HEAD — the commit identity HEAD names, and its filtered
   workspace tree.

In a nested checkout, all three sit inside a repository that is
bigger than the workspace, and the host is fenced at the whole
repository exactly as delegates are (HIW-O16): sibling motion
surfaces as a violation or refusal, never invisibly.

## O15 — HEAD and index accounting

### What openTurn additionally records and anchors (before host launch)

- `headCommit`: the commit id HEAD names at turn open, kept reachable
  by a runner-owned ref (`refs/metasystem/missions/<m>/turn-open-head`,
  CAS-updated at each open, deleted at mission close with the tree
  anchors) so a mid-turn reset cannot orphan the accounting origin.
- `headTree`: that commit's filtered workspace tree.
- `topTree`: the toplevel repository snapshot at open (nested
  checkouts only; see O16).
- `refMap`: ALL repository ref tips at open (`for-each-ref` with no
  namespace filter), so ref-retained bytes are accountable in every
  namespace (rule 5 below).
- `topStagedTree`: the toplevel real-index projection at open (nested
  checkouts), the origin for staged sibling motion.
- At MISSION BIRTH the initialize-state write records the same
  origins beside E0 — initial `headCommit`, `topTree`,
  `topStagedTree`, and `refMap` — so turn-1 continuity (a
  detached-HEAD switch before the first open included) and a crash
  between admission and first open have a durable authority, not a
  silent adoption.
- Staged admission: at turn open the staged projection must equal
  `headTree` or `preTree`. Anything else is unaccounted staged bytes
  from between turns — the same class as the existing worktree drift
  refusal at `loop.go:1558`, and it takes the same shape: evidence,
  taint, park. At MISSION start the staged projection must equal
  HEAD's filtered tree OR the admitted baseline itself — the clean or
  human-sealed tree admittedBaseline returns — so a sealed mission
  whose index simply mirrors the sealed state is lawful (the sealed
  tree is reviewable by equality); anything else is refused like a
  dirty worktree.

### Repository preconditions (admission, beside the fileMode pin)

The EFFECTIVE replacement namespace must be empty at admission and
stay empty: not the literal `refs/replace/*` alone but whatever
hierarchy the replacement machinery would consult
(GIT_REPLACE_REF_BASE redirects it). And the scrub is the WHOLE
repository-steering environment, not replacement alone: every runner
git surface — gittree, the anchor machinery, measurement and gate
invocations — runs with `core.useReplaceRefs=false` and with
GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR, GIT_INDEX_FILE (beyond its
own isolated one), GIT_OBJECT_DIRECTORY, alternates, namespaces, and
GIT_REPLACE_REF_BASE stripped (the D120 environment-stripping
posture) — an inherited GIT_DIR demonstrably redirects `git -C` to
another repository's HEAD, letting the wall inspect a repository the
mission does not live in. A replacement mapping PRESENT at
admission refuses the mission; one CREATED OR MOVED at any inspection
or between turns is a violation outright, regardless of target —
an active mapping re-routes later unpinned git operations, the
completion gate above all, to bytes the wall never judged.

### What every inspection proves (turn conclude, every abnormal host
### exit, recovery, resume — the HIW-O5 ordering: before measurement,
### before any gate success)

In order, each failure a named violation with persisted evidence,
taint, and park. THE OUTCOME SPLIT IS BY FAILURE CLASS, and it is
implementable because the probes are TYPED, never string-matched:
gittree gains outcome-typed helpers (a probe distinguishes "the
command ran and answered X" — including "HEAD is unborn", "ref
absent" — from "the command could not run"). Ran-and-answered
failures of the repository's state are WALL VIOLATIONS (the host had
custody; fail toward the wall); could-not-run failures (git absent,
spawn refusal, timeout) stay runner errors, the split inspectWall
already draws for its own snapshot (`wall.go:122-124`).

ONE RESOLUTION PER INSPECTION, RE-VERIFIED AT THE COMMIT POINT: the
inspection begins by capturing the full observable posture ONCE —
resolved HEAD, the ref map, the pseudoref census, the worktree
census with each admitted worktree's posture, both staged
projections, the worktree projection, and the toplevel projection —
and every rule judges that capture; the census members are INSIDE
the capture precisely so the final comparison sees them (a pseudoref
written after rule evaluation would otherwise slip between capture
and verdict) (snapshots seed from resolved ids, never the symbolic name
"HEAD": `gittree.go:192` resolves at exec time, which would leave a
gap between judging and projecting). The SAME capture is re-taken
and compared whole immediately before the verdict is acted on — and
again around the ACCEPTANCE WRITE, which happens after the drain and
the arbitrary-bash measurement (`loop.go:1267`, `measure.go:271`).
The write itself is TWO-PHASE WITH DURABLE PENDING STATE, because
repository state and mission state share no transaction
(`state.go:978` locks only the mission state, and a carrier can move
while the writer waits on that lock). The acceptance append remains
THE single commit point HIW-O13 requires — verdict, trees,
consumption, and the captured posture land in one write — but it no
longer concludes the turn: `openTurn` survives it, and a separate
POST-VERIFICATION entry re-captures the posture after publication
and concludes the turn only on a clean match, a mismatch appending a
violation entry over the acceptance (taint, park) before any success
surfaces. The interval is a DEFINED state, not a race: an acceptance
entry with no verification entry means consumed-but-unconcluded, and
resume re-runs the post-publication verification deterministically
against the recorded posture — a crash between the two writes can
therefore never leave a completed mission over unprobed motion, and
consumption is never double-spent because the commit point already
landed. Motion after the post-verification entry of a CONCLUDED
mission is post-mission by timestamp and belongs to interactive
custody, outside the wall's boundary as ruled. A changed capture re-runs the inspection; after a
bounded number of retries the turn is a violation — "repository
would not hold still during inspection".

THE ACCOUNTED SET of a turn, used by every rule below — membership is
checked by DECOMPOSITION, never enumeration. A candidate tree T is
accounted against `openTurn.preTree` when:
- every consumed authorization's changed paths in T carry either that
  patch's entire pre-side or its entire post-side entries (whole
  patches only; the patches are pairwise disjoint by the parent's
  overlap rule, so any subset composes deterministically — no
  recorded order is needed for membership, and the durably recorded
  order remains the parent's replay concern);
- DECLARED host-artifact paths are content-free at commit
  granularity: the declaration authorizes exact PATHS, not final
  bytes (`host-implementer-wall-design.md` §host-artifact delta), so
  a lawful draft-then-final sequence of commits is admitted — the
  conclusion equation still binds the FINAL delta;
- every other path equals preTree.
The mission's named E-sequence points are accounted directly.
ACCOUNTING IS BY CONTENT, NOT CEREMONY: a commit whose tree is
accounted ships only reviewed-or-declared bytes, which is the
invariant HIW-O15 protects — an empty commit, a commit of a named
E-point, or a return to an earlier accounted tree moves no unreviewed
byte and is lawful (strictness guards invariants; the FORMER O15 row wording
"only certified integrations advance HEAD" meant this content
reading, and the row now says so).

1. FIRST-PARENT REACHABILITY. `openTurn.headCommit` lies ON the
   first-parent chain of the resolved HEAD (plain ancestry is not
   enough: an open commit reachable only through a merge's second
   parent leaves the walk below no terminal). A retreat, an amend of
   pre-open history, or a switch to foreign lineage fails here.
   Violation: "committed HEAD retreated or rewrote history (open
   <sha>, now <sha>)".
2. FIRST-PARENT TREE ACCOUNTING. Every commit on the first-parent
   chain from resolved HEAD back to `openTurn.headCommit` has an
   accounted filtered workspace tree. INTEGRATION IS --no-ff BY
   CONTRACT: a reviewed multi-commit implementer branch enters as a
   merge whose SIDE TIP is the reviewed tree (rule 3), never by
   fast-forward — a fast-forward puts the implementer's intermediate
   trees on the first-parent chain where whole-patch membership
   rightly fails, and the violation names the remedy ("integrate
   with --no-ff"). Violation otherwise: "commit <sha> advances HEAD
   with an unaccounted tree", changed paths against preTree as
   evidence.
3. SIDE-TIP ACCOUNTING. For every merge on that first-parent chain,
   each non-first parent's filtered workspace tree equals a consumed
   authorization's REVIEWED tree (the conformance-bound R — the O14
   lane) or is itself accounted. An `ours`-merge burying an illicit
   commit fails here by its tip. Interior side-branch commits below
   an accounted-or-reviewed tip remain the stated detector-tier
   boundary, bounded by their tip.
4. PER-COMMIT REPOSITORY SCOPE (nested checkouts). Each first-parent
   commit and each side tip changes only workspace-prefixed paths
   against its first parent, judged at toplevel. Vacuous at a
   toplevel install, exactly like the delegate fence.
5. THE REF TRANSITION FENCE — exact, record-bound, no semantic tip
   judging. Against `openTurn.refMap`, the only lawful ref
   transitions during a turn are enumerable:
   - refs the runner's own records name (tree anchors, the
     turn-open-head ref, the state-anchors ref) must EQUAL their
     recorded values — motion AND deletion of a runner ref is a
     violation (a deleted anchor breaks the reachability the wall
     depends on);
   - `refs/heads/agent/<job>` branches that THIS mission's dispatch
     job records name move FREELY while the job chain's
     authorization is unconsumed (a live implementer's branch is its
     workspace; its content ships only through conformance), and
     must be STATIONARY after consumption;
   - every other ref, in every namespace — heads, tags, remotes,
     custom hierarchies — is UNCHANGED: creation, motion, and
     deletion all violate. There is no accounted-tip lane: O15/O16
     name no lawful need for arbitrary host ref motion, and judging
     tips semantically re-opened namespace, lifecycle, and
     sibling-scope gaps every round.
   THE ACTIVE BRANCH is the one lawful non-runner transition: the
   checked-out candidate branch the mission is pinned to
   (`anchor.go:258`) moves with every lawful commit rules 1-4
   admit, so the fence requires exactly that ref to EQUAL the
   resolved HEAD of the same capture — content is rules 1-4's
   business, the fence only forbids it pointing elsewhere (a
   same-tip detach or a switch to another branch violates). Every
   other non-runner, non-agent ref stays stationary.
   PSEUDOREFS join the fence BY CLASS, not by list: the root-ref
   census enumerates every pseudoref file directly under the git
   directory (the all-caps `*_HEAD` family — ORIG_HEAD, MERGE_HEAD,
   FETCH_HEAD, CHERRY_PICK_HEAD, REBASE_HEAD, REVERT_HEAD,
   BISECT_HEAD, and kin — plus AUTO_MERGE), parses every OID each
   contains (FETCH_HEAD and MERGE_HEAD are multi-OID formats), and
   requires each to be accounted-or-reviewed or the file absent — a
   lawful --no-ff integration leaves exactly that (ORIG_HEAD at the
   accounted pre-merge tip), while parking an unaccounted commit in
   ANY of them violates. Stated consequences, accepted: a mid-turn
   fetch refuses (FETCH_HEAD and remote-tracking motion); fetch
   before the turn opens or after it concludes.
   LINKED WORKTREES are carriers too, and join the transition fence
   as a census WITH POSTURE: each capture enumerates the worktrees
   (`git worktree list --porcelain`) and records, per admitted
   worktree, its HEAD OID and the content digest of its private
   index file — the porcelain listing alone exposes no index, and a
   worktree's private HEAD and index are exactly the carriers that
   moved unobserved in the round-4 shape. Each worktree must be the
   mission workspace itself or one the runner's records name: the
   runner's own detached measurement worktrees (`measure.go:251`)
   at their recorded tips, and dispatch-record delegate worktrees,
   free until their chain's consumption like their branches and
   STATIONARY in posture (HEAD and index digest) from consumption
   on, judged from the posture the acceptance payload recorded. An
   UNRECORDED worktree created, moved, or carrying a HEAD/index
   posture of its own is a violation outright: a detached worktree
   under ignored space is otherwise a complete private carrier (its
   HEAD retains commits) that no main-checkout observable ever
   sees.
6. STAGED ACCOUNTING, both scopes. The workspace staged projection
   must be ACCOUNTED (the same decomposition membership as commits —
   an index holding a lawful subset while the worktree carries more
   is lawful; equal-to-HEAD and equal-to-post are just two members),
   and the accepted value is recorded as `stagedTreePost` in the
   acceptance payload so the next open judges MOTION from it, never
   re-judges the state itself (a subset lawful at conclusion cannot
   falsely park the next open). The toplevel staged POSTURE in
   nested checkouts is not a write-tree at all: it is the
   serialization of `ls-files --stage` (path, mode, object id, stage
   number), so unmerged sibling entries are representable and
   preexisting sibling conflicts refuse nothing — motion is judged
   by serialization diff, and only workspace-prefixed changes from
   the anchored origin are lawful. Violation: "staged bytes
   unaccounted" with the paths.
7. The existing worktree equation, unchanged.

### Between-turns and resume continuity (the same rules, prior
### acceptance as origin)

Turn open and mission resume run the SAME accounting with the
PREVIOUS acceptance's recorded posture as the origin: headCommitPost,
refMapPost, topTreePost, topStagedPost from the hash-chained
acceptance payload (turn 1 uses the birth record). Between-turns
motion is judged with the previous turn's accepted state as the
accounted set — no host or peer motion between turns escapes: an
illicit commit made after one acceptance and before the next open is
examined at the next open and refuses exactly like mid-turn motion
(the existing worktree drift refusal at `loop.go:1558` becomes one
clause of this continuity check). Violations here refuse the turn
open: evidence, taint, park.

### Projection stability: seed from the comparison target

Every in-turn snapshot seeds from the tree it will be COMPARED
against, not from HEAD in any form: a new primitive
`SnapshotSeeded(expectedTree, declaredPaths)` grafts the expected
workspace tree into the toplevel seed (read-tree the resolved
toplevel HEAD tree, replace the workspace prefix subtree with the
expected tree, `read-tree --prefix` plumbing, then `add -A` as
today, then `add -f -- <declaredPaths>`): declared host-artifact
paths are content-free and may be ignored files the expected tree
cannot yet name, so their membership is an explicit forced input —
without it, a force-added committed declared file would vanish from
the post projection and falsely read as drift at the next open.
Membership of
tracked-and-ignored paths then follows the comparison's own
right-hand side: a patch-added ignored-but-tracked path projects
whether or not HEAD has reached the full composition (killing both
the frozen-seed false rejection and the intermediate-subset
conservative edge in one move), and a worktree that fails to carry
the expected file surfaces as a deletion mismatch, which is the
correct refusal. Turn open and admission, which DEFINE the origin,
seed from the resolved HEAD commit exactly as today.

### What acceptance does NOT require

A turn with HEAD unmoved is lawful (today's model: integration lives
in the worktree, acceptance is tree-level). The wall never demands
that committed HEAD reach the accepted post-tree — whether the cohort
requires committed delivery is the completion gate's business. The
wall only refuses UNACCOUNTED motion.

## O16 — the repository fence for the host

Nested checkouts only (empty prefix: every rule below is vacuous by
construction, delegate-fence style).

- openTurn anchors `topTree` (toplevel snapshot, seeded from the
  anchored open commit).
- Every inspection takes a fresh toplevel snapshot and computes
  ChangedPaths(open topTree, now). Every changed path must carry the
  workspace prefix; workspace-prefixed changes are then governed by
  the workspace equation (no double jeopardy, one owner per path).
  Any path outside the prefix: violation "sibling paths changed in a
  nested checkout on the host's watch", naming the paths.
  ATTRIBUTION HONESTY (detector tier): the violation asserts the
  change happened during the host's turn, never that the host
  authored it — a peer writing into the repository mid-turn surfaces
  the same way, which is O16's own requirement: visibly.
- The between-turns drift check extends to the toplevel: turn N+1's
  open toplevel snapshot must equal turn N's concluded one; sibling
  drift between turns refuses turn open (evidence, taint, park), so
  quiet-period sibling motion is also never invisible.
- Machine filters translate to toplevel path space (prefix + ledger
  path; prefix + contract path at admission) — the same exclusions,
  one path-space conversion, owned next to the existing filter list.
- Mission-start admission anchors the toplevel tree as the sibling
  origin. Preexisting sibling state is NOT judged (it is not the
  mission's product and not the workspace's to demand clean — the
  delegate fence likewise judges motion from a boundary base, not
  preexisting state).

## Refusal shapes (all: evidence in wall.json, taint, park before any gate success)

- `wall violation: committed HEAD retreated or rewrote history (open <sha>, now <sha>)`
- `wall violation: commit <sha> advances HEAD with an unaccounted tree` (+ paths vs nearest accounted tree)
- `wall violation: staged bytes unaccounted` (+ paths)
- `wall violation: sibling paths changed in a nested checkout on the host's watch` (+ paths)
- Turn-open and mission-start admission failures reuse the existing
  drift/refusal shapes with the new observables named.

## Evidence, authority, and events

AUTHORITY: the hash-chained acceptance-entry payload gains
`headCommitPost`, `refMapPost`, `stagedTreePost`, `topTreePost` and
`topStagedPost` (nested), beside the trees it already carries, and
the birth record carries the admission origins. ONE SCOPED
EXCLUSION, forced by causality: `refMapPost` omits the runner's
self-owned publication refs (the state-anchors ref above all — its
post-acceptance tip is a commit containing the very state hash the
payload would freeze, `anchor.go:271/:343`, so recording it is a
content-addressed self-reference). Those refs are not less guarded
for it: the fence judges them at every capture against the anchor
machinery's own independent authentication (the anchored
ledger/state truth the runner already verifies), which is stronger
than OID equality and has no circularity. The state's exact-key validation
means these are a SCHEMA DECISION, not an addition: mission state
bumps to schemaVersion 4; a pre-4 state refuses resume through the
existing version barrier, the same posture as HIW-O11 and the
ledgerSemantics-3 precedent (missions are short-lived cohort
artifacts; re-provision). Every between-turns comparison reads the
CHAIN, never wall.json: evidence files are rewritable and prove
nothing forward.

EVIDENCE: `wall.json` gains `headCommitOpen`, `headCommitNow`,
`stagedTree`, `topTreeOpen`/`topTreeNow` (nested), `refMap` deltas,
and, on a rule violation, the offending commit or ref list with
per-item verdicts.

EVENTS: the one registered wall event today is `wall-violation`
(`wall.go:690`) — its payload extends with the new observables.
WITHIN THIS DESIGN'S SCOPE no pass event is added; the parent's
wall-passed/pass-event contract stays HIW-O10's own obligation,
whose named-contract list is reconciled to the registered
`wall-violation` spelling at this landing. Records stay the
authority, events stay observability.

## Crash and resume

All new anchors are written and ref-anchored BEFORE the host
launches, in the same write that records preTree today (HIW-O1's
pattern; the openTurn transition rule — immutable in flight — covers
the new fields unchanged). Recovery and resume run the full
inspection above from the anchors and the live repository; a crash
between anchor and launch leaves a fully accountable open turn.
Resume on an openTurn that PREDATES this schema (no headCommit
anchor) is unaccountable by construction and fails toward the wall:
refuse resume with a named error, exactly the HIW-O11 legacy posture
(missions are short-lived cohort artifacts; re-provision).

## Honest boundaries (detector tier, D100 — stated, not solved)

- NON-FIRST-PARENT HISTORY: a merge can carry side-branch commits
  whose blobs never appear in any first-parent tree. Those bytes ride
  history but are never the checked-out product at any accounted
  point; auditing interior history is the isolation tier's business.
  Recorded in the laundering table's register.
- PUSHES: publishing mid-turn to an external remote is outside the
  tree equation entirely; network egress is the permission layer's
  fence, not the wall's.
- REFLOG AND GC FORENSICS: the wall proves live state against
  anchors; it does not do archaeology.
- SUBMODULE INTERIORS: unchanged stated boundary of the shared
  primitive.
- A root-capable host can still forge anything (D100 ruling); the
  cooperative posture is unchanged.

## New primitives (named here, built in the implementation pass)

- `gittree.Workspace.StagedTree() (string, error)` — the workspace
  staged projection, computed by RECONSTRUCTION, never by write-tree
  over the real index or a whole copy of it: `git write-tree`
  refuses ANY unmerged entry, so a preexisting sibling conflict in
  the copied repository-wide index would refuse a lawful nested
  workspace. The primitive reads the workspace-restricted staged
  entries (`ls-files --stage -- <workspace>`), loads them into a
  fresh isolated index (`update-index --index-info`), and write-trees
  THAT — sibling entries, conflicted or not, never enter; a
  conflicted WORKSPACE entry still has no tree and refuses toward
  the wall; the real index is never opened for writing (observer-only
  preserved, `gittree.go:28`).
- `gittree.Workspace.AnchorCommit(mission, name, commit)` — the
  runner-owned commit ref with CAS update, sibling of Anchor.
- `gittree.Workspace.SnapshotSeeded(expectedTree, declaredPaths)` —
  the comparison-target-seeded projection above, declared paths as
  forced membership.
- Outcome-typed probes (`HeadCommit`, `RefMap`, staged projections)
  that distinguish ran-and-answered repository states from
  could-not-run environment failures, so the violation/error split
  never string-matches.
- First-parent walk + membership check live beside inspectWall in
  `internal/missionrunner/wall.go` (one owner for accounting).

## Design obligations

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| WSS-1 | CRITICAL | this design §O15 | openTurn records and ref-anchors headCommit, headTree, topTree, topStagedTree (nested), and the full-namespace refMap before host launch, immutable in flight; the acceptance payload carries headCommitPost, refMapPost, stagedTreePost, topTreePost, topStagedPost under state schemaVersion 4 (exact keys; pre-4 refuses resume); the birth record carries the admission origins — headCommit included — beside E0 | `internal/missionrunner/loop.go` turn open + `internal/gittree` AnchorCommit + mission state schema 4 | none yet | crash-at-every-boundary fixture; anchored-commit survives reset fixture; chain-not-wall.json fixture; pre-4-state-refuses fixture; birth-record-origins fixture | forced kill between anchor and launch on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-2 | CRITICAL | this design §O15 rules 1-3 | Every host-exit inspection resolves HEAD and the ref map once, judges the resolved values (first-parent reachability, subset-decomposition accounting with content-free declared paths, side-tip accounting, --no-ff integration contract), and re-verifies stability at the end (moved repository re-runs, bounded, then violates); typed probes split repository-state violations from environment errors | `internal/missionrunner/wall.go` inspection + `internal/gittree` typed probes | none yet | retreat, amend, foreign-branch, unaccounted-commit, ours-merge-buried-commit, side-tip-reviewed-lawful (O14), ff-integration-refuses-with-remedy, artifact-draft-commit-lawful, subset-state-commit-lawful, no-commit-lawful, unborn-HEAD-violates, moved-during-inspection fixtures | scripted host commits smuggled staged bytes mid-turn on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-3 | CRITICAL | this design §O15 rule 6 + admissions | The workspace staged projection is ACCOUNTED (decomposition membership — lawful subsets pass) at every inspection; toplevel staged posture is the ls-files --stage serialization judged by motion only (preexisting sibling conflicts refuse nothing); StagedTree reconstructs a workspace-only isolated index from ls-files --stage (sibling conflicts never enter; observer-only); mission start admits HEAD's tree or the admitted clean-or-sealed baseline; a conflicted WORKSPACE index refuses toward the wall | `internal/missionrunner/wall.go` + `internal/gittree` StagedTree (reconstructed workspace-only index) | none yet | staged-smuggle-then-revert, staged-lawful-subset-passes-and-recorded, staged-sibling-motion, preexisting-sibling-conflict-not-blamed-even-in-workspace-projection, index-not-mutated-by-projection, dirty-index-at-start, sealed-index-at-start-lawful, conflicted-workspace-index fixtures | host stages bytes it never commits across a turn boundary on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-4 | HIGH | this design §O15 rule 3 | Each first-parent commit changes only workspace-prefixed paths at repository scope; a commit with an accounted workspace subtree but sibling payload refuses | `internal/missionrunner/wall.go` per-commit toplevel diff | none yet | sibling-payload-commit fixture in a nested repo; toplevel-install vacuity fixture | host commit touching a sibling path on a nested VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-5 | HIGH | this design §O16 | Worktree sibling motion is fenced at open, every inspection, and between turns in nested checkouts, machine filters translated, refusals attribution-honest | `internal/missionrunner/wall.go` toplevel fence | none yet | sibling-edit mid-turn, sibling-drift between turns, machine-path-translated, toplevel-vacuous fixtures | host edits a sibling path mid-turn on a nested VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-6 | HIGH | this design §projection stability | In-turn snapshots seed from the comparison target with declared paths as forced-membership inputs (SnapshotSeeded(expectedTree, declaredPaths)), so tracked-and-ignored membership follows the comparison's right-hand side and a committed ignored declared artifact cannot vanish from the projection | `internal/gittree` SnapshotSeeded + `internal/missionrunner/wall.go` call sites | none yet | ignored-tracked-path-projects-at-subset-HEAD passes; absent-expected-file-refuses; committed-ignored-declared-artifact-projects; graft-in-nested-path-space fixture | mid-turn HEAD move with tracked-and-ignored files on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-7 | MEDIUM | this design §evidence | wall.json carries the new observables and per-item verdicts; the registered wall-violation event's payload extends (this design adds no pass event; HIW-O10 remains parent-owned); the acceptance chain, not evidence files, answers every forward-looking comparison | `internal/missionrunner/wall.go` evidence writer + event registry | none yet | wall.json schema fixture; wall-violation payload fixture | violation evidence readback on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-8 | HIGH | this design §acceptance non-requirement + O14 | Lawful lanes stay lawful, proven positively: no-commit turn, single post-tree commit, subset-state commits, empty and E-point commits, the merge-integration lane, and the sealed-index mission start all accept with the new rules active | `internal/missionrunner/wall.go` fixtures | none yet | positive-lane fixtures beside the violation fixtures, one per lane | a real implementer chain integrated by merge on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-9 | MEDIUM | this design §crash and resume | An openTurn predating the new anchors refuses resume with a named error; no migration machinery | `internal/missionrunner` state loader | none yet | preserved pre-schema openTurn resume fixture asserting the exact error | resume attempt on a preserved pre-schema state | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-10 | HIGH | critique r1/r3/r5 (WSS-R1-04, WSS-R3-09, WSS-R5-01) | Every runner git surface runs with core.useReplaceRefs=false and the WHOLE repository-steering environment stripped (GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR, foreign GIT_INDEX_FILE, object-directory/alternates, namespaces, GIT_REPLACE_REF_BASE — the D120 posture); admission refuses a nonempty EFFECTIVE replacement namespace | `internal/gittree/gittree.go` env scrub + config pins + every runner exec surface | none yet | replace-ref-cannot-alter-accounting; replace-base-redirect-refused; git-dir-redirect-cannot-steer-probes; anchor-and-measure-surfaces-pinned fixtures | replace ref planted mid-turn on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-11 | HIGH | critique r1-r4 (WSS-R1-03, WSS-R2-03/04/05, WSS-R3-01/09, WSS-R4-01/02/05) | The exact ref transition fence: runner-record refs equal their recorded values (motion or deletion violates); dispatch-record agent/* branches move freely until consumption then hold; the checked-out candidate branch equals the capture's resolved HEAD (content is rules 1-4's business); every other ref in every namespace is unchanged; the pseudoref census covers the whole *_HEAD family plus AUTO_MERGE, multi-OID formats parsed, each OID accounted-or-reviewed or absent; the worktree census records per-worktree posture (HEAD OID, index digest) and admits only the workspace and runner-recorded worktrees (measurement at recorded tips; delegate free-until-consumption then stationary in posture), unrecorded worktrees violating outright | `internal/missionrunner/wall.go` ref fence + worktree census | none yet | retain-under-branch/tag/custom-namespace, rebase-head-retention-violates, multi-oid-fetch-head-parsed, no-ff-merge-pseudorefs-lawful, anchor-ref-deletion-violates, delegate-branch-free-then-held, active-branch-tracks-lawful-commit, same-tip-detach-violates, unrecorded-worktree-violates, measurement-worktree-lawful, consumed-delegate-worktree-posture-motion-violates fixtures | commit-tag-reset shape on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-12 | CRITICAL | critique r2-r3 (WSS-R2-01/02, WSS-R3-02/03/04) | The whole observable posture (HEAD, refs, staged both scopes, worktree, toplevel, worktree census) is captured once, judged, and re-verified whole at the verdict; the acceptance append stays HIW-O13's single commit point but no longer concludes the turn: a separate post-verification entry re-captures and concludes on a clean match (mismatch taints over the acceptance), and an acceptance without its verification entry is the defined consumed-but-unconcluded state that resume completes deterministically; the pseudoref and worktree censuses are inside the capture the verification compares; turn open and resume run the full accounting from the previous acceptance's recorded posture, turn 1 from the birth record including its headCommit; refMapPost omits the runner's self-owned publication refs, which the fence authenticates through the anchor machinery instead | `internal/missionrunner/wall.go` + `internal/missionrunner/loop.go` continuity | none yet | moved-during-inspection re-runs then violates; gate-mutates-after-inspection caught by post-publication probe; peer-motion-during-state-lock-wait caught; peer-index-mutation-caught-by-whole-capture; illicit-commit-between-turns refuses next open; detached-switch-before-first-open refuses via birth headCommit; state-anchors-self-move-lawful; crash-between-acceptance-and-verification-resumes-deterministically; pseudoref-written-after-rules-caught-by-final-capture fixtures | commit between acceptance and next open on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |
| WSS-13 | HIGH | critique r3 (WSS-R3-10) | Resolution entries record the full carrier posture as the next origin; RESTORE verifies worktree and staged equal the named tree and refuses while any other carrier fails accounting; ADOPT adopts the observed posture wholesale under named waived claims | `internal/missionrunner/resolve.go` + mission state resolution entries | none yet | restore-refused-while-HEAD-unaccounted, restore-verifies-staged, adopt-records-carrier-posture, post-resolution-continuity fixtures | resolve an O15 violation both ways on a VM target | MISSING | await the WSS-R6-01 ruling, fold WSS-R6-02..06, then implement |

## Interactions, stated

- HIW-O14 composes: the merge lane here is the generic accounting
  that lawful mid-turn merges need; O14's sharper provenance-backed
  diagnosis remains its own row.
- Resolution segments (RESTORE / ADOPT_DISPUTED_TREE) already start
  new E-segments; their trees are named points and therefore
  accounted — a post-resolution commit of the ruled tree is lawful by
  rule 2. THE RESOLUTION POSTURE EXTENDS TO EVERY CARRIER: a
  resolution entry records the full observed posture (headCommit,
  refMap, staged both scopes, topTree) as the next origin. RESTORE
  verifies the worktree and staged carriers equal the named tree AND
  refuses while any other carrier still fails accounting ("restore
  refused: committed HEAD still carries unaccounted commits") —
  restoring a worktree never un-ships committed bytes, so those need
  adoption or human git surgery first. ADOPT records the observed
  posture wholesale under the named waived claims; adoption of an
  O15/O16 violation is adoption of its carriers, stated in the
  record.
- The unmeasurable-fixture mechanism (deleting the gate's pinned ref)
  moves no HEAD and stages nothing; unaffected.
- HIW-O11's legacy barrier covers pre-wall states; WSS-9 covers
  pre-WSS openTurns with the same posture.
