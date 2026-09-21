# g1-s10 Backlog data path: the list

- Gate 1, state `designed`, **revision 2**, author Claude on Fable as a D23 delegate, 2026-09-21. Delivery step 3 of D21 (plan `:153`): a thin cut through `g1-s2`, `g1-s4`, `g1-s7`, and the list half of `g1-s10`, none in full.
- Revision 2 applies Sol's [review](g1-s10-backlog-data-path-design-sol-review.md): the tree is read once per tip through one owner export that runs every at-rest rule and keeps the typed problems (M1); Ready is the owner's admission verdict from `goal.Next` (M2); the drafts directory is neither read nor counted (M3); only the validated tree and its commit time are cached and every configuration-dependent gate runs per observation (M4); the interface runs no git of its own and the commit time comes through the owner with a poisoned-environment test (M5); the horizon comes from one exported owner constructor under the planner's ruling (M6); the three open questions are settled by the planner; the concurrent Project slice is reconciled; and, so that this revision carries no unverified claim, the parked-from derivation (M8), the expired-plus-blocked placement (M9), the size and field-bound premises (M10), the Unknown lane's reachability (M11), and the causal wording of the absent and no-ledger panes (M7) are corrected.
- Refines the [master](../user-interface-design.md) at `ui-development` commit `f5768f525`: Backlog and the lane table (`:492` to `:535`), Rules for an intuitive structure (`:433` to `:449`), Workspace identity (`:451` to `:461`), freshness (`:257`), Trustworthy state (`:769` to `:777`), the gate 1 row (`:809`); M5, M6, M7 (plan `:141`, `:142`, `:145`).
- Depends on `g1-s9` first cut (merged at `548514ad5`), `g1-s8` revision 5, `g1-s1` revision 6; D1, D2, D11, D20, D21. Contributes to scenarios 34, 21, 47, 15; discharges none in full.

Paths are relative to `metasystem/` unless they start with `plans/`. Every code claim was read whole in source at `e2e7b6a40`, whose `metasystem/` tree is byte-identical to `f5768f525`'s (`git diff --stat` between them is empty), so revision 1's surviving citations stand. Nothing was executed except read-only `git` queries and two `curl` reads of the running server's `/api/workspace`.

## Outcome

A human opens Backlog and, once the checkout has an accepted tip, sees this repository's real goals as a list grouped by lane: every live goal placed once, Ready meaning what the engine's claim gate would admit today, badges for what the record says, named gaps for what it does not, the tip's identity and age, and when the server read it. The 429 done and 1 abandoned goals sit behind one toggle. Nothing is mutable; the Brain stays absent. Before the tip exists, the pane says so, counts the live and archived goal files the working tree holds without reading them, and names the one-time step the human runs from a terminal. A tip the engine would refuse is refused here too, by name.

## Scope and non-goals

| Slice | Taken | Left |
| --- | --- | --- |
| `g1-s2` | The root rule; the accepted ref only, never worktree bytes; the validated tree cached by tip; every time- or configuration-dependent answer per observation; tip identity and observation time | Local sources; the two banner strings, replaced by the facts behind them; other consumers |
| `g1-s4` | A package outside `internal/ui` placing every goal once with Waiting precedence, the closed set, badges, M5 and M6 as data, the owner's admission verdict as Ready's input, `LaneOf` for `g1-s16` | Dependencies as a graph, history beyond the last line, arcs, budgets, risk, the CLI adopting it |
| `g1-s7` | One JSON route; identity by kind, id, revision; `schemaVersion`; the per-request `Info` seam | The event stream and its three facts; `sourceHead`; every other route; the written conventions |
| `g1-s10` | The list by lane; the closed-items filter | Board, other filters, search, selection, drag, outline, dependencies |

Non-goals: fetching (`g1-s3`), goal detail (`g1-s11`), a `goal` kind in the resolver, pagination, virtualisation, the shell's second cut, dependencies.

## Existing code this builds on

| Fact | Where |
| --- | --- |
| `ResolveEndpoint`: `goal.sync-remote` (default `origin`), `goal.sync-branch` (default `refs/heads/main`); `AcceptedRef`; `LocalMode` is `Remote == "local"` | `internal/goal/txn.go:51`, `:61` to `:76` |
| `Project(e, false, now)`: rev-parse, `loadTree`, horizon, sync-mode gate, two banners; `true` fetches and moves refs. `loadTree` is `ReadCommitGoals` then `ParseTreeFiles` and nothing else: **`ValidateTree` never runs on this path**, so a parseable tree with duplicate ranks, a missing dependency, or a cycle projects | `project.go:72` to `:125`; `verbs.go:353` to `:363` |
| `ProjectAt(root, tip, at...)`: the same `loadTree` and horizon over a named commit; no gate, no git beyond the read, no validation | `attention.go:558` to `:569` |
| `AcceptedLedgerTip`: absent `("", false, nil)`; ref without `backlog.md` `(tip, false, nil)`; unreadable ref file or non-commit is an error. It proves the ref's presence and the tip's ledger presence and nothing about how either came to be | `attention.go:541` to `:556`; `txn.go:153` to `:204` |
| The at-rest law: `ValidateCommit` reads, parses, runs `ValidateTree`, then `ValidateChannelTree`, and collapses every problem into one error string; `validateWaitCommit` repeats the sequence unexported; `ValidateTree`, `ValidateChannelTree`, and `ParseTreeFiles` are exported; `Problem` is a string type; `*TreeReadError` carries typed problems, but only parse problems reach it today | `validate.go:93`, `:199`, `:672` to `:692`; `channel.go:137` to `:161`; `attention.go:409` to `:438`; `goal.go:157`; `verbs.go:336` to `:351` |
| `SyncModeGate(e, tip)`: the two refusals `Project` makes, in the same words, over the tip's root record read fresh; a tip without a ledger gates nothing; an unreadable root record refuses | `txn.go:206` to `:237`; `project.go:98` to `:106` |
| `Live`, `Done`, `Abandoned`; six states, closed at parse: a seventh state is a parse problem and never becomes a `GoalFile`; the fence beats landing; only relayed approvals expire, against `now` and the first enrollment | `validate.go:27`, `:154` to `:180`; `file.go:95` to `:105`, `:511` to `:525`, `:590` to `:595`, `:288` to `:306` |
| The horizon: `approvalHorizon(t, now)` is unexported and is the one owner of the observation `ApprovalExpired` judges; `ApprovalHorizon` is documented as that complete observation | `approval.go:253` to `:259`; `file.go:256` to `:262` |
| Claim admission: `Next(p, machine)` walks `OrderedOpenGoalIDs`; an approved goal goes, in this order, expired to `Awaiting`, pinned to another machine skipped, an open blocker to `Blocked`, else through `requireApprovedForClaimWithContext` to `Ready` or to `Refused` with the gate's cause; a gate error that is not a refusal fails the whole frontier ("cannot answer claimable backlog"). The gate takes no machine; it needs the tier-box set, loaded from `metasystem.conf` and its `.local` sibling under `p.Root`, with defaults when the file is absent. Its refusals are all approval-shaped: no approval or budget, an invalid approval record, a budget beyond the tier norm without a norm approval, expiry. The engine builds a `Projection` over a tree already in memory from exported fields, as this slice will | `project.go:599` to `:724`; `approval.go:270` to `:379`; `config/budget.go:121` to `:140`, `:229` to `:245`; `attention.go:522` |
| `depState`: a blocker is done when it is in `Done`; live, abandoned, or absent is not done | `verbs.go:2279` to `:2290` |
| The git contract: every engine git call runs through `commandWithEnvironment`, which drops `GIT_DIR`, `GIT_WORK_TREE`, the `GIT_CONFIG*` family, and the rest of the steering set, because `-C` does not defeat them; `goalGit`, `goalGitWithEnvironment`, and `gitIn` are unexported; `ReadCommitGoals` takes an environment for tests, and `testEnvironment(os.Environ(), "GIT_DIR=…")` is how the engine's own tests poison one | `genesis.go:89` to `:186`; `txn.go:81` to `:97`; `validate.go:599`; `testmain_test.go:47`; `genesis_test.go:132`; `reconcile_test.go:401` |
| The interface server's child inherits the launcher's environment untouched | `ui/lifecycle/launch.go:95` to `:101` |
| Park: from queued, approved, or claimed only; `Displaced` is set when the parked claim was another pair's, a human's park included; the own pair's park of its own claim keeps an `Episode` whose `Released` is the park's stamp, the same `r.stamp()` the `ParkRecord.At` and the History line carry | `verbs.go:2354`, `:2370` to `:2399`, `:441` to `:457`, `:404` to `:435`; `file.go:83` to `:86`, `:406` to `:415` |
| `listSynced` groups by state, done and abandoned each by id; `goal fetch --root` resolves the endpoint and runs `FetchAdvance`, refs only | `cmd/metasystem/goal.go:394` to `:443`; `goalsync_verbs.go:396` to `:412`; `fetchadvance.go:30` |
| Three roots; `Info.Describe` per request; `/api/workspace` matched exactly beside `/-/health` in `ServeHTTP`; `/api` is a reserved prefix so anything unserved beneath it is 404; the wiring; the workspace resource carries `stateRoot`; the shell's call site, guard, empties, routes | `ui/lifecycle/roots.go:14`, `:31`; `ui/httpd/httpd.go:20` to `:30`, `:104` to `:158`, `:246` to `:259`; `cmd/metasystem/ui.go:130` to `:141`; `_app/src/shell/workspace.ts`, `identity.tsx`, `cuts.test.ts:313` to `:326`, `panes/empties.ts`, `routes.ts:44` to `:78` |

The checkout today: no accepted ref, no `goal.sync-*` configuration, `origin` is `github.com/widoriezebos/agentic-tools`, `origin/main` is `c5d517f42` (2026-09-21T18:39:42Z). Tracked: 156 files under `plans/goals/`, one the root record, **155 goals** (119 queued, 33 parked, 2 approved, 1 claimed); 430 under `records/goals/` (429 done, 1 abandoned); `plans/goals-drafts/` exists and is not read by Go, nor by this slice. Plan `:153`'s "156 live" counts the root record. Sol's independent read-only checks agreed with this inventory (review §4). The running server reports `stateRoot` `/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem`, equal to the installation, as the self-hosted layout has it.

## Contracts

### `internal/goal`: two additive exports

The planner's ruling admits an exported constructor to `internal/goal` so the approval horizon keeps one owner; this slice takes that and one more export of the same kind, for the same reason, so that the validated read and the commit time keep one owner too. One new file, `internal/goal/validatedread.go`, and its test; nothing that exists in the package changes, `ValidateCommit` included.

```go
// ValidatedTree is one immutable read of an identified commit: the ledger tree
// every at-rest rule accepts, and the commit's committer time.
type ValidatedTree struct {
    Tip         string
    Tree        *TreeGoals
    CommittedAt time.Time // zero when git could not report it; never fatal
}

// ReadValidatedTree reads the commit's ledger subtree once, through the engine's
// own git contract, and refuses it whole, by name, unless every rule ValidateCommit
// applies passes: ParseTreeFiles, then ValidateTree, then ValidateChannelTree.
// A parse or validation problem is a *TreeReadError carrying the typed problems;
// any other error is the read's own. environments is the test seam ReadCommitGoals has.
func ReadValidatedTree(root, tip string, environments ...[]string) (ValidatedTree, error)

// NewApprovalHorizon is the exported name of approvalHorizon: the one observation
// ApprovalExpired judges, built exactly as claim admission builds it.
func NewApprovalHorizon(t *TreeGoals, now time.Time) ApprovalHorizon
```

`ReadValidatedTree` is `ValidateCommit`'s sequence (`validate.go:672` to `:692`) with the tree kept and the problems typed, plus one `log -1 --format=%ct <tip>` through `goalGitWithEnvironment` as `project.go:113` reads it, parsed as `project.go:114` to `:117` parse it. `ValidateChannelTree` reads the goal tree a second time when the commit carries channel files (`channel.go:150`, `:158`); that is the owner's cost inside one call, once per tip, and not this slice's to refactor. `NewApprovalHorizon` is `return approvalHorizon(t, now)`. Neither function moves a ref or writes anything.

### `internal/backlog`

Outside `internal/ui/` (plan `:183`); imports `internal/goal`, nothing under `internal/ui`; no HTTP. Its one read outside the tree is `goal.Next`, which loads the tier-box configuration through the owner; everything else is pure.

```go
type Lane string  // draft, to-do, ready, in-progress, review, waiting, done, abandoned, unknown; LaneOrder in that order
type Ref struct{ Kind, ID string; Revision uint64 }             // Kind is "goal"
type Row struct { Ref; Where string /* live | archived */; Lane Lane; Phase string
    State, Intent, NextStep, Concluded, Origin string; Priority uint8; Sequence uint64; Tier uint8
    Labels []string; Arc, Pinned string; BlockedBy, OpenBlockers []string
    Approved *Approval /* By, At, Authority, ReviewBy, Expired, ExpiredWhy */; Claim *Claim /* Machine, Lineage, At, LandingAt */
    Waiting *Waiting /* Reason, Since, By, From, Blocker */; Abandoned *Abandoned /* By, At, Because */; Fence *Fence /* Reason, ClosedAt */
    Sliced, Decomposed bool; OpenedAt, LastChangeAt, LastVerb string; Gaps []string }  // JSON: lower camel case, omitempty on pointers
type Board struct{ Rows, Closed []Row; Counts map[Lane]int; Draft DraftGap }
type DraftGap struct{ Statement string }

// Admission is the owner's claim-admission answer for every approved live goal at
// one observation: goal.Next's buckets, merged over the machines the tree pins to.
type Admission struct {
    Answered bool                     // false: goal.Next refused the whole frontier
    Message  string                   // the engine's words when !Answered
    Ready, Blocked, Awaiting map[string]bool
    Refused  map[string]string        // id → the gate's cause
}
func Admit(p goal.Projection) Admission
func Project(tree *goal.TreeGoals, horizon goal.ApprovalHorizon, admission Admission) Board
func LaneOf(f *goal.GoalFile, tree *goal.TreeGoals, horizon goal.ApprovalHorizon, admission Admission) (Lane, phase string, gaps []string)
```

`Admit` composes the owner's frontier without repeating its law. `goal.Next(p, "")` places every approved goal that is not pinned and skips the pinned ones (`project.go:698`), so `Admit` calls `Next` once with `""` and once per distinct `Pinned` value among approved live goals, in sorted order, and takes each goal's bucket from the run whose machine equals its pin. The gate takes no machine (`approval.go:360`), so a goal's verdict is the same in every run that places it. The first error from any run makes `Answered` false with that message and every map empty: the engine says the whole frontier is indeterminate then (`project.go:649` to `:651`), and this slice does not know better. `Claimed`, `Fenced`, `Landing`, and `TrunkRed*` are not read: the record places claimed work. `p.Root` is the state root, the same root every goal verb takes, so the tier-box set is read from where the engine reads it.

`Rows` is every live goal in `goal.OrderedOpenGoalIDs` order, lane-tagged; `Closed` is `Done` then `Abandoned`, each by `goal.SortedGoalIds`, as `listSynced` orders them (settled, below). Every goal appears once. `Counts` has every lane but `draft`. Placement, phase per M5:

| Record | Lane, phase | Gaps and badges |
| --- | --- | --- |
| claimed, `StopFence` set | waiting, not recorded | `Fence`; reason is the fence's; landing ignored (`file.go:104`) |
| claimed, `Landing` set | review, `landing` | `Claim.LandingAt` |
| claimed otherwise | in-progress, not recorded | gap `phase not recorded` |
| parked | waiting, not recorded | `Waiting{Reason: Because, Since: At, By, Blocker, From}`. `From` is `"claimed"` when `Displaced` is set (another pair's claim, a human's park included) or when `Episode != nil && Episode.Released == Parked.At` (the own pair parked its own claim); otherwise `"not recorded"`: queued and approved are not told apart by the record, and history beyond the last line is out of scope |
| approved, in `Awaiting` | to-do | gap `approval expired: <why>` from `f.ApprovalExpired(horizon)`; blockers listed, placement kept. Expiry is judged before blockers, as `Next` judges it (`project.go:692`) and as the master's To Do rule allows (`:531`) |
| approved, in `Blocked` | waiting | `OpenBlockers`: every `Blocked` id that is not in `tree.Done`, by `depState`'s rule; an id absent from the tree is named `unknown goal`; `From: "approved"`, the recorded state, never "ready", because admission is not judged for a blocked goal |
| approved, in `Ready` | ready | none |
| approved, in `Refused` | to-do | gap `claim refused: <cause>`, the gate's own words. Every cause is an approval-shaped gap (`approval.go:361` to `:377`), which the master's To Do rule covers; Waiting would claim a blocker where what applies is the approval |
| approved, `!Answered` | unknown | gap `claim admission not answered: <message>`; Ready is empty and its card says why |
| approved, in no bucket | unknown | gap `the engine's frontier did not place this goal`; unreachable by construction, kept |
| queued | to-do | gap `not approved`; blockers listed, placement kept |
| done | done | `Decomposed` when the id is in `Root.Decomposed` (`root.go:32`), shown "split into goals" (master `:533`) |
| abandoned | abandoned | `Abandoned{Because}` |
| any other live state | unknown | gap `state <s> is not placed by this build`, never dropped. Unreachable today, because the parser refuses a seventh state before projection (`file.go:590` to `:595`); kept for the master's future-facing rule (`:809`) |

Waiting precedence, stated exactly: Waiting beats the active and ready lanes, never To Do. A fence or a landing, a park, and an open blocker on an unexpired approval all place in Waiting; an expired approval stays in To Do with its gap, blockers or not, because the engine judges expiry first and the master's precedence applies "when an authoritative blocker applies" to "the normal active or ready lane" (`:531`).

`Pinned`, `Labels`, `Arc`, `Tier`, `Priority:Sequence`, `Sliced`, `Origin` are badges, never lanes; a pinned goal takes its bucket like any other and shows `pinned to <machine>`. `LastChangeAt` and `LastVerb` are the last History line. `DraftGap.Statement` is fixed: "plans/goals-drafts/ has no reader in the engine; a drafts owner arrives with gate 5 (M6)". No count accompanies it: counting the directory would be reading the source the statement says nothing reads (M3).

### `internal/ui/snapshot`

```go
func New(stateRoot string, now func() time.Time) *Holder
type State string // read | absent | no-ledger | broken | unreadable | refused
type Observation struct { ObservedAt time.Time; StateRoot string; State State; Tip string; CommittedAt time.Time
    Message string; Problems []goal.Problem
    Tree *goal.TreeGoals; Horizon goal.ApprovalHorizon; SyncMode string; Admission backlog.Admission
    LiveFiles, ArchivedFiles *int }
func (h *Holder) Observe() Observation
```

`stateRoot` is `lifecycle.Roots.StateRoot`, derived by `stateroot.RootForInstallation` (`roots.go:60`): `metasystem/` here, the application's repository root when adopted (D20); the root every ledger read takes. `Observe`, under one mutex:

1. `now := h.now().UTC()`. `goal.AcceptedLedgerTip(root)`: an error is `broken` with the engine's message; not existing is `absent` when the tip is empty, else `no-ledger`. `LiveFiles` (`plans/goals/*.md` less `backlog.md`) and `ArchivedFiles` (`records/goals/*.md`) are counted in these two states only; an unreadable directory gives `nil`, never zero. Working-tree counts, labelled so; no goal is read from them.
2. The tree. If the held tip equals this one, reuse the held `goal.ValidatedTree`. Else `goal.ReadValidatedTree(root, tip)`: a `*goal.TreeReadError` is `unreadable`, `Problems` its typed problems, `Message` its `Error()`, and it is held under the tip as well, since a commit's refusal is as immutable as its tree; any other error is `broken` with the engine's message and nothing is held. Only a `ValidatedTree`, good or refused, is ever held, keyed by `Tip`.
3. Configuration, every observation: `goal.ResolveEndpoint(root)`, then `goal.SyncModeGate(e, tip)`; an error from either is `refused` with the engine's message. `SyncMode` is `Tree.Root.SyncMode` when the root record is present.
4. `Horizon = goal.NewApprovalHorizon(Tree, now)`.
5. `Admission = backlog.Admit(goal.Projection{Root: root, Tip: tip, Tree: Tree, Horizon: Horizon})`, every observation, because it reads `metasystem.conf` and its `.local` sibling.
6. State `read`.

Held by tip: the `ValidatedTree`, that is the tip, the parsed and validated tree, its commit time, or its typed refusal, all immutable facts of one commit. Not held, and why: `ObservedAt`, `Horizon`, expiry, and staleness change while the tip stands still; the ledger state, because the ref can appear, break, or move; the endpoint, the sync-mode verdict, and the admission, because git configuration and `metasystem.conf` change independently of the tip (M4); the counts, because the worktree is not the tip. A configuration flip is therefore seen on the next observation, at the same tip, with the tree still held. This package imports no `os/exec` and runs no git: every git invocation is the owner's, under its scrub (M5). `goal.Project` is not called at all: it re-reads the tree on every call, never validates it, and gates only when it reads; its two banners are not carried, since one embeds a terminal command and both derive from facts the observation holds; this remains a wording deviation from the `g1-s2` row, recorded for the planner.

### The route

`GET /api/backlog` in `internal/ui/httpd`, exact; beneath it 404, by the existing reserved-prefix rule (`httpd.go:146`, `:252`). `Info` gains `Observe func() snapshot.Observation`, called per request; `nil` answers 500 `{"error":"this engine was built without a ledger reader"}`. In state `read` the route calls `backlog.Project(obs.Tree, obs.Horizon, obs.Admission)`:

```json
{"schemaVersion":1,"observedAt":"2026-09-21T19:05:12Z",
 "ledger":{"state":"read","tip":"c5d517f4…","committedAt":"2026-09-21T18:39:42Z","stale":true,"staleAfterSeconds":1800,
           "syncMode":"remote","stateRoot":"/Users/wido/…/metasystem","message":"","problems":[]},
 "admission":{"answered":true,"message":""},
 "workingTree":{"liveFiles":null,"archivedFiles":null}, "counts":{"to-do":119,"…":0}, "draft":{"statement":"…"},
 "rows":[{"ref":{"kind":"goal","id":"tests-parallel-and-deterministic","revision":9},"where":"live","lane":"in-progress",
          "phase":"not recorded","state":"claimed","gaps":["phase not recorded"],"…":"…"}], "closed":[]}
```

`committedAt` is the commit's committer time, which is what `%ct` reports; revision 1 called it `acceptedAt`, which named a time nothing records. `stale` is `observedAt − committedAt > goal.StaleThreshold`, `false` and `committedAt` `""` when unknown. `problems` are the typed problems as strings, one each. In every state but `read`: `rows` and `closed` empty, `counts` `{}`, `admission.answered` false with an empty message, `message` and `problems` the engine's words, status **200**, because the workspace answered and the answer is why the ledger cannot be read; 500 is for the engine not answering. Identity is `ref`: kind, id, revision (master `:319`, `:325`); no route, no component name. A Go test asserts the field names verbatim and that `draftFiles` and `acceptedAt` are absent.

`g1-s7` still owes: the event stream, its path, event shape, and view names (`backlog`, `workspace`); `sourceHead`; the detail, decisions, and fleet routes; the written reference and refusal conventions; the payload rules the brain's tools consume. This route is shaped to survive them.

### Frontend

New under `_app/src/backlog/`: `api.ts` (`loadBacklog(signal)`: `fetch("/api/backlog", {headers: {Accept: "application/json"}})`, the second call site, and the types), `state.ts` (`useBacklog()`: `loading | failed | known`, the `identity.tsx` pattern with an `AbortController`, on mount and on Refresh), `lanes.ts` (ids, titles, one-sentence meanings, order), `format.ts` (RFC 3339 to local `YYYY-MM-DD HH:MM`; ages), `BacklogPane.tsx`, `LedgerLine.tsx`, `LaneGroup.tsx`, `GoalRow.tsx`, `Statement.tsx`, `backlog.css`. Edited: `shell/Shell.tsx` routes `/backlog` to `BacklogPane`; `panes/empties.ts` loses its `backlog` row and `empties.test.ts` its expectation and `wanted` entry; `cuts.test.ts` as under Verification. `routes.ts` is untouched by this slice: `/backlog/x` stays not found and `routeFor({kind:"goal"})` stays `null` until the goal detail slice adds the goal route; nothing here freezes `/backlog/*`. `package.json` and the lockfile do not change.

`Statement` is the one new component, with its reason: `EmptyState` renders a static table row by id (`Pane.tsx:31`), and these statements carry counts and engine text per response. It takes `heading`, `body`, `note`, children, and uses the `ms-empty*` classes unchanged. Wording keeps the shell's conventions: a heading is a name without a full stop, a body is sentences, the note is the 12px line, nothing asserts the absence of records.

**The pane**, top down: `LedgerLine`, 13px `text-2`, "Accepted tip `c5d517f`, committed 2026-09-21 20:39 · observed 21:05:12", a Refresh `Button`; when `stale`, a `text-3` line "The accepted tip is 3 h old. This build does not fetch: publications from other machines appear after a goal verb or `goal fetch` runs in this checkout". Then the lane index, one wrapping line of in-page anchors "To Do 119 · Ready 2 · … · Draft not read", no number for Draft. Then one `ms-card` per lane in `LaneOrder`, title "<Lane> (<n>)", the meaning in 12px `text-3`; an empty lane reads "No goal is in this lane at tip `c5d517f`", a fact about a named projection, except Ready when `admission.answered` is false, which reads "Ready could not be answered at tip `c5d517f`: <message>; the approved goals it would judge are under Unknown"; the Draft card is the M6 `Statement` with no count. Then a `Button` with `aria-pressed`, "Show closed items (430)", mounting the Done and Abandoned cards, off by default, React state only. A `text-3` footer: "Board, outline, dependencies, filters, and goal detail arrive with g1-s10 and g1-s11".

**A row**, 8px vertical padding, 1px `border` between, no hover since nothing selects: line one, the id in mono 13px, then `Chip`s: state, `tier n`, `p:s` or `unranked`, `pinned to <machine>`, each label, `arc <name>`, `sliced`, `split into goals`, `rev n`; line two, the intent, 14px, in full; line three, 13px `text-2`: the next step (live), the concluded text (done), the reason (abandoned); line four when present, 12px `text-3`: gaps and reasons ("phase not recorded"; "not approved"; "approval expired: …"; "claim refused: …"; "claim admission not answered: …"; "parked by human:Wido since …, from claimed: <because>"; "blocked by a, b (open)"; "landing since …"; "claimed by m1e since …"), then "opened <date> · last <verb> <date>". Nothing is truncated; no field of `GoalFile` is bounded by the record (the bounds at `goal.go:26` to `:39` govern the legacy `Goal`, as Sol's M10 found), so the walkthrough measures rather than assumes. Ids are copyable text; no row is a link.

Selectable: nothing. Controls in DOM order: Refresh, the anchors, the toggle; rows are not tab stops. Narrow: rows and chips wrap, no `min-width`, no horizontal scroll. `backlog.css` sets layout and token-backed classes (`ms-goal-row`, `ms-goal-id`, `ms-goal-intent`, `ms-goal-next`, `ms-goal-gaps`, `ms-lane-index`, `ms-lane-meaning`, `ms-ledger-line`) only; no colour literal, no `style` attribute.

### Scale and freshness

One request, one payload of all 585 rows. Sol's read-only estimate is roughly 1.2 MB of Intent, Next step, and Concluded text alone (review M10), so revision 1's "under 500 KB" is withdrawn; the walkthrough measures the serialized size, the initial paint, and the closed-toggle paint. The 155 live rows render at once; the 430 closed only while the toggle is on. No pagination, no virtualisation: the simplest arrangement is measured before a cleverer one is designed; more than a second from response to paint is a finding for the board slice.

No stream, no timer. The list is read when the pane mounts (each visit, each reload) and on Refresh; leaving aborts a request in flight. Newer data reaches the checkout only when something moves the accepted ref: a goal verb or `goal fetch`, run by the human; whether a later slice fetches is `g1-s3`'s question and is not asserted here. The human sees the tip, its commit time, the observation time, and the stale line. After this slice there are exactly **two** network call sites, `shell/workspace.ts` and `backlog/api.ts`. The shell's first-cut guard asserted one; it changes as under Verification, and the shell's second cut later raises the allowlist to four. This amends `g1-s9`'s O14; the planner notes it on that row.

## Behaviour

Main flow: the pane mounts, shows a `Skeleton` in the ledger line, requests `/api/backlog`; the server observes (the tree held after the first read at a tip; the gates and admission fresh), projects, answers; the pane renders.

Failure and edge cases:

| Case | Required result |
| --- | --- |
| The ref is absent (today) | 200, `absent`, the two counts. Heading "No accepted tip in this checkout"; body "This build reads goals only from the accepted ledger ref, never from the working tree. The ref refs/metasystem/goals/accepted does not exist in this checkout. The working tree holds 155 live goal files and 430 archived records, none read until the tip exists."; note "The tip appears when a goal verb or `goal fetch` runs against this state root. One-time step, from a terminal, on your word: `metasystem goal fetch --root <ledger.stateRoot>`", the path rendered absolutely from the payload, so the command is right in either layout. The probe proves absence only (`txn.go:158` to `:189`); nothing says why the ref is missing. Refresh stays. Naming a command is master `:733`'s uncovered workflow, stated as such; the interface never runs the fetch (settled, below) |
| The ref's commit has no `backlog.md` | `no-ledger`; "The accepted tip carries no ledger"; body "The ref points at `<tip>`, whose tree has no plans/goals/backlog.md. This build cannot say how the ref came to point there."; the same note, since `goal fetch` is the verb that advances the ref, without a claim that the remote holds a ledger |
| The ref file is unreadable or not a commit, or the tip's tree cannot be read from git | `broken`; "The accepted ref cannot be read"; the engine's message (`txn.go:179`, `:188`; the read's own); note "Repair refs/metasystem/goals/accepted before continuing" |
| A file at the tip fails to parse, or the tree fails an at-rest rule: duplicate ranks, a missing dependency, a cycle, a channel problem, any rule `ValidateTree` or `ValidateChannelTree` applies | `unreadable`; "The ledger at `<tip>` does not validate"; "The engine refuses a tree with any problem whole rather than showing part of it"; the typed problems one per line in mono; never a partial list; the refusal held by tip |
| `goal.sync-branch` is not `refs/…`, or the tip's root record contradicts `goal.sync-remote` | `refused`; "The ledger could not be projected"; the engine's message; note "Check goal.sync-remote and goal.sync-branch in this checkout's git configuration" |
| The configuration flips while the tip stands still | The next observation is `refused` (or `read` again after the flip back) with the same tree held; no restart, no reload beyond Refresh |
| `goal.Next` cannot answer (the tier-box configuration does not load or parse) | `read`; `admission.answered` false with the engine's message; the approved goals the gate would judge in Unknown with `claim admission not answered: <message>`; Ready empty with its statement; every other lane as usual |
| The gate refuses an approved, unexpired, unblocked goal | To Do; gap `claim refused: <cause>` |
| An expired approval with an open blocker | To Do; gap `approval expired: <why>`; the blockers listed as open; not Waiting |
| A goal pinned to another machine | Its bucket as any other goal's, with the `pinned to <machine>` chip; never skipped |
| 404 (older server), 500, unparsable, or the fetch fails | "The backlog could not be read" 14px `text-2`, the message 12px `text-3`, Retry: the shell's workspace-failure pattern; rail and header work |
| A blocker id is not in the tree | Waiting; `openBlockers` lists it; the reason says "unknown goal" |
| A live state outside the six | Lane `unknown`, "Unknown (n)", the state named; unreachable through today's parser, kept |
| `committedAt` unreadable | "committed at: unknown"; `stale` false; no stale line |
| The tip moves; a review date passes with no tip change | The next observation loads the new tree; the next observation moves the goal to To Do with the gap, no reload |
| The pane is left before the response; toggle on, then Refresh | Aborted, no state written; the toggle stays on and closed rows re-render |

## Change boundary

New: `internal/goal/validatedread.go`, `validatedread_test.go` (the two exports, under the planner's ruling); `internal/backlog/` (`lanes.go`, `admit.go`, `project.go`, tests, `testmain_test.go`); `internal/ui/snapshot/` (`snapshot.go`, `counts.go`, tests, `testmain_test.go`); `internal/ui/httpd/backlog.go`, `backlog_test.go`; `_app/src/backlog/**`. Edited, and nothing else in them: `internal/ui/httpd/httpd.go` (the `Observe` field, the exact-path branch beside `workspacePath`); `cmd/metasystem/ui.go` (one `snapshot.New(roots.StateRoot, time.Now)` in the `serve` case, `holder.Observe` in `Info`; wiring only); `docs/architecture.md` (rows for `backlog` and `ui/snapshot`; the `ui` family row gains `ui/snapshot`, `ui/workspace`, `backlog`, `goal`); `shell/Shell.tsx`, `panes/empties.ts`, `panes/empties.test.ts`, `cuts.test.ts`; `bundle/**` regenerated last (D7). `httpd_test.go` may gain the route in an existing route table only.

Must not be touched: every existing file under `internal/goal/**`, `ValidateCommit`, `Project`, `ProjectAt`, and `Next` included (the two new files are the whole of this slice's presence there; `git diff --stat` on the package shows nothing else); `cmd/metasystem/goal.go`, `goal_list.go`, `goalsync_verbs.go` (`g1-s16`'s); `main.go`; the plan's list at `:107` (`testing-parallel-ratchet.json`, `testing.json`, `internal/testenv`, `testutil`, `parallelratchet`, `testselect`, `testpolicy`, `testexec`, `hostload`, `proofrun`, `cmd/metasystem/test*.go`, `audit.go`); `go.mod`, `go.sum`, `gopackages`, `stateroot`, `config` (no new key), `ui/lifecycle`, `ui/web/*.go`, `ui/workspace`, `scripts/**`, `metasystem.conf`, `metasystem.conf.local` (never read by the designer; the engine reads it through `LoadTierBoxSet` as it always has); `g1-s8`'s `vite.config.ts`, `tsconfig.json`, `scripts/*.mjs`, its policy and page rule; `package.json`, `package-lock.json`; any other existing `*_test.go` or `testmain_test.go`. The interface never calls `Project`, `FetchAdvance`, `AdvanceAccepted`, or anything that moves a ref; `internal/backlog` and `internal/ui/snapshot` import no `os/exec`; nothing in this slice opens `plans/goals-drafts/`.

Conventions, repeated because the builder sees only this document: every new Go package has `testmain_test.go` with `func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }`; tests never call `os.Setenv`; every top-level test and independent subtest calls `t.Parallel()`, the ratchet allowing an unlisted package zero serial tests; no sleeps, polling, or fixed ports; the clock is injected; temporary repositories under `t.TempDir()` seeded with `exec.Command("git", "-C", root, ...)` as `internal/goal/servingprojection_test.go:12` to `:75` does, with `goal.RenderRoot`, `goal.RenderFile`, and `git update-ref refs/metasystem/goals/accepted HEAD`; `testutil.Expect` and `Require`; `cmd/metasystem/ui.go` wires and prints only; no new Go module dependency (`encoding/json`, `sync`, `time` suffice; `os/exec` only in tests). Frontend: exact versions, `npm ci --ignore-scripts`, the Node pin (D27), Vitest in `node` with no DOM library, no colour literal outside `tokens.css`, no `style` attribute, no dependency this design does not name (none), the bundle rebuilt last.

## Verification

**Go, `internal/goal`, the new test file only.** On a seeded repository: a good tip returns the tree and `CommittedAt` equal to `git log -1 --format=%ct` read by the test; a tip whose file does not parse returns a `*TreeReadError` whose `Problems` are the parse problems; a tip that parses but breaks a tree rule (two live goals at one priority:sequence pair, or a `Blocked` id that names no goal, whichever `ValidateTree` refuses; the builder picks the rule by reading `validate.go:199` to `:500`) returns a `*TreeReadError` whose `Problems` equal `ValidateTree`'s; on each of those fixtures `ValidateCommit`'s error is nil exactly when `ReadValidatedTree`'s is, and its message contains every problem line: one law, asserted rather than assumed. The poisoned environment: a second repository under `t.TempDir()` with a different ledger and an older commit; `environment := testEnvironment(os.Environ(), "GIT_DIR="+other+"/.git", "GIT_WORK_TREE="+other, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=…", "GIT_CONFIG_VALUE_0=…")` as `reconcile_test.go:401` builds one; `ReadValidatedTree(root, tip, environment)` returns `root`'s tree and `root`'s commit time, not `other`'s. `NewApprovalHorizon`: with and without `FleetEnrollment`, a relayed approval's `ApprovalExpired` answers at D and D+2 agree with `Next`'s `Awaiting` bucket on the same tree. All `t.Parallel()`; no `os.Setenv`.

**Go, `internal/backlog`.** A table test over `LaneOf` for every placement row, each a hand-built `goal.GoalFile` in a hand-built `goal.TreeGoals` with a `RootRecord` and a hand-built `Admission`: queued; queued with an open blocker; approved in `Ready`; approved in `Blocked` by a live goal, by an absent id (waiting, `unknown goal`), and one whose only blocker is done and is therefore in `Ready`; approved in `Awaiting` with `ReviewBy` past, and the same with an open blocker (to-do, both shown); approved in `Refused` with a cause; approved with `FleetEnrollment` set; approved under `Answered: false` (unknown); approved in no bucket (unknown); claimed plain, with `Landing`, with `StopFence`, with both; parked with `Displaced`, parked with an `Episode` whose `Released` equals `Parked.At`, parked with an older `Episode`, parked with neither; done; done in `Decomposed`; abandoned; a live state `"weird"`. `Admit` on a `t.TempDir()` root with no `metasystem.conf` (defaults apply, `budget.go:234`) and an in-memory tree: an unpinned approved goal in `Ready`; one pinned to `m1` in `Ready`, not skipped; one whose `Budget.ReservedJobMinutesLimit` exceeds the tier-3 box without a `NormApproval` in `Refused` with the gate's cause; one behind a live blocker in `Blocked`; one expired in `Awaiting`; then, with a `metasystem.conf` whose tier-3 box value (the key `tierBudgetKey(3)` names, `budget.go:230`) does not parse, `Answered` false, `Message` beginning "cannot answer claimable backlog", every map empty. `Project`: every id once across `Rows` and `Closed`; `Rows` in `OrderedOpenGoalIDs` order; `Closed` done by id then abandoned by id; `Counts` complete. A JSON round trip asserts every `Row` field name verbatim.

**Go, `internal/ui/snapshot`.** On a seeded repository (`goal.sync-remote` `local`, a `SyncLocal` root): no ref, `absent` with the two counts; a ref at a commit without `backlog.md`, `no-ledger`; a loose ref file holding garbage, `broken`; a corrupted Integrity line, `unreadable` with `Problems` naming the file; a tip whose files parse but whose tree `ValidateTree` refuses, `unreadable` with the rule's problem, and a second `Observe` at that tip answering the same; a `SyncMode: remote` root against the local configuration, `refused`; the flip: at a good tip, `git config goal.sync-remote origin`, `refused`; `git config goal.sync-remote local`, `read` with the same `*goal.TreeGoals` pointer as before the flip; a good tip, `read`, forty-hex `Tip`, `CommittedAt` equal to the fixture commit's `%ct`, `Admission.Answered` true, `StateRoot` the root; cache: a second `Observe` returns the same pointer, then `git update-ref` to a commit adding one goal and a third `Observe` holds it under a new pointer; time: a relayed approval with `ReviewBy` on day D, the injected clock at D unexpired and at D+2 expired, same pointer; eight goroutines observing at once under `-race`. The package's source imports no `os/exec` outside `_test.go` files.

**Go, `internal/ui/httpd`.** From a faked `Observe`: 200 and the field names for a three-goal `read` observation, `admission` present, `draftFiles` and `acceptedAt` absent; an unanswered admission serialises; `absent` answers 200 with `state`, counts, empty `rows`; `nil Observe` answers 500 with the error shape; `/api/backlog/x` and `/api/backlogs` are 404; the headers and policy are on the response; `Observe` runs once per request.

**Frontend, Vitest in `node`.** `lanes.test.ts` (ids, order, titles, meanings verbatim, `unknown` present); `format.test.ts` (RFC 3339 to local text, ages, empty `committedAt` gives "unknown"); `api.test.ts` (with `globalThis.fetch` replaced: a non-2xx status throws naming the status, a 200 returns the payload); `empties.test.ts` revised as above; `cuts.test.ts` revised: the one-site guard becomes an allowlist `CALL_SITES` of `[file, fetchCount, resources[]]` holding `["backlog/api.ts", 1, ["/api/backlog"]]` and `["shell/workspace.ts", 1, ["/api/workspace"]]`, "exactly one place" becomes "exactly the listed places, each with its count, each naming its resources", and every other rule (no `EventSource`, `WebSocket`, `XMLHttpRequest`, `sendBeacon`, timer, lifecycle event, or health path; the second-cut files absent) stays as written.

**Walkthrough**, in order, in a real browser, by Claude then the human, from `metasystem/`: `go build ./...`; from `_app/`: `npm ci --ignore-scripts`, `npm run typecheck`, `npm test`, `npm run bundle`; `bin/metasystem ui restart`; open `/backlog`: the absent statement with 155 and 430 and the command naming this checkout's state root, `/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem`, Refresh, header and rail intact; `git status --porcelain` clean. On the human's word, from a terminal, never from the interface: `bin/metasystem goal fetch --root .`, expected `advanced=true tip=c5d517f4… accepted c5d517f`; `git rev-parse refs/metasystem/goals/accepted` equals `origin/main` if the remote has not moved; `git status` still clean; HEAD unchanged. Press Refresh without reloading: the list; lane counts summing to 155 and closed to 430, recorded in the evidence note; the two approved goals each in Ready or in To Do with `claim refused` (the engine's answer, recorded either way); `tests-parallel-and-deterministic` in In Progress, "phase not recorded", revision 9; the parked goals in Waiting with reasons and, where the record proves it, "from claimed"; the ledger line with `c5d517f`, 20:39 local, and the stale line; `bin/metasystem goal list --root .` printing `claimed=1 approved=2 queued=119 parked=33 done=429 abandoned=1 tip=…`, matching the page. On the human's word, the flip: `git config goal.sync-remote local`, Refresh, the `refused` statement with the engine's words; `git config --unset goal.sync-remote`, Refresh, the list again, no restart. Network panel: one `/api/backlog` per visit and per Refresh; one `/api/workspace` on load; nothing on refocus, on an offline and online toggle, or over two idle minutes; the response size, the time to first paint, and the closed-toggle paint noted. `curl -s -H 'Accept: application/json' http://127.0.0.1:7878/api/backlog | head -c 300`; `/api/backlog/x` gives 404. Toggle closed items on, scroll, Refresh, still on. Widths 1,280, 800, 599, 400: no horizontal scroll. Keyboard: Refresh, the anchors, the toggle, nothing else. VoiceOver reads each lane heading with its count. Light and dark; zero policy violations; no request off the origin. `bin/metasystem audit parallel-ratchet` without `--update`; `go vet` and `go test` on `./internal/goal ./internal/backlog ./internal/ui/...`; `g1-s8`'s Git-fed gofmt pipeline.

Obligations the code critique checks by name:

- **O1, no ref moves.** Nothing in the slice calls `Project`, `FetchAdvance`, or `AdvanceAccepted`; `ReadValidatedTree`, `AcceptedLedgerTip`, `SyncModeGate`, and `Next` only read; `git status` and HEAD are unchanged around every request.
- **O2, the tip, not the worktree.** No goal is read from disk; the two counts are a labelled `ReadDir` and reach no row; no code in the slice opens `plans/goals-drafts/` (M3).
- **O3, cache by tip only.** The held value is a `goal.ValidatedTree` keyed by `Tip`; endpoint, sync-mode gate, horizon, admission, and `ObservedAt` are per observation; the flip test and the pointer-identity and clock tests pass (M4).
- **O4, every goal once.** The uniqueness test passes; `Counts` sums to `len(Live)` over live lanes and `len(Done)+len(Abandoned)` over closed.
- **O5, Waiting precedence and expiry first.** Fence and landing together is waiting; blocked unexpired approved is waiting; blocked queued stays to-do; expired with an open blocker is to-do with both shown (M9).
- **O6, honest gaps.** Every in-progress row carries `phase not recorded`, every queued row `not approved`; the draft statement names M6 and gate 5 and carries no count; the Ready card says why when admission is unanswered; the absent and no-ledger panes state the observed condition and no cause; `empties.test.ts`'s `ABSENCE` list applied to `lanes.ts` and every `Statement` caller finds nothing.
- **O7, six ledger states.** Each renders from a faked payload; 200 in every state; 500 only for `nil Observe`.
- **O8, identity by reference.** Every row carries `ref.kind`, `id`, `revision`; no route or component name in the payload; `routeFor({kind:"goal"})` is `null`; no row links.
- **O9, two call sites.** `cuts.test.ts` passes as revised; the browser request pattern holds.
- **O10, shared owner.** `LaneOf`, `Project`, and `Admit` live in `internal/backlog` with no `internal/ui` import; `cmd/metasystem/goal*.go` untouched.
- **O11, shell untouched.** No shell token, class, or component changes; `literals.test.ts`, `contrast.test.ts`, the revised `empties.test.ts` pass.
- **O12, bundle current.** The last commit rebuilds the bundle; `go test ./internal/ui/web/` passes; the notices file is unchanged.
- **O13, one validated tree (M1).** The snapshot's only tree read is `goal.ReadValidatedTree`; `ProjectAt`, `Project`, and `ValidateCommit` are not called from the interface; the one-law test passes; `Problems` are `[]goal.Problem` end to end and reach the page one per line.
- **O14, Ready is the owner's verdict (M2).** No code outside `internal/goal` evaluates the tier box, norm coverage, or the approval record; `LaneOf` places approved goals from `Admission` alone; the unanswered case renders as specified; a pinned goal is never skipped.
- **O15, no git of our own (M5).** `internal/backlog` and `internal/ui/snapshot` import no `os/exec`; the poisoned-environment test in `internal/goal` passes.
- **O16, one horizon owner (M6).** `goal.ApprovalHorizon{…}` is constructed nowhere outside `internal/goal`; `NewApprovalHorizon` is the only source.
- **O17, additive only in `internal/goal`.** `git diff --stat` on `internal/goal` shows the two new files and nothing else.

## Settled

The three questions revision 1 left open; the planner ruled on all three.

1. **Closed-items order.** Done by id, then Abandoned by id, as `listSynced` orders them (`cmd/metasystem/goal.go:416` to `:425`), so terminal and browser agree until `g1-s16`.
2. **Expired approval placement.** To Do with the gap visible, judged before blockers, as `Next` judges it (`project.go:692` to `:695`); the combined expired-plus-blocked fixture is in the `LaneOf` table.
3. **Who runs the one-time fetch.** The human, from a terminal, never the interface; the absent pane names the verb with the served state root and runs nothing.

## Reconciliation with `g1-s21`

This slice's side of the four items the planner named; no shared contract is invented here.

1. **Route registration.** One exact-path `if` beside `workspacePath` in `ServeHTTP` (`httpd.go:130` to `:133`) and one `Info` field; no mux. `/api` is already reserved (`httpd.go:252`, `routes.ts:44`), so `/api/backlog/x` is 404 by the existing rule. The Project slice's two branches sit beside it in the same chain; whichever lands second adds its lines and its `Info` fields, and neither introduces a mux.
2. **The `cuts.test.ts` allowlist.** The guard becomes an allowlist of `[file, fetchCount, resources[]]`; this slice's entry is `["backlog/api.ts", 1, ["/api/backlog"]]` beside the shell's `["shell/workspace.ts", 1, ["/api/workspace"]]`; the Project slice appends `["project/api.ts", 1, ["/api/project", "/api/documents/"]]` to the same array. The final array has three entries; the second slice to land appends, it does not replace.
3. **The identity triple.** `{"kind":"goal","id":<goal id>,"revision":<integer>}`. The keys are the triple's; `revision` is typed by kind: the ledger's own revision number here (`GoalFile.Revision`, what `metarun.ClaimableGoal` carries), `"blob:<sha>"` for a document. A consumer switches on `kind` before reading `revision`. Whether to unify the revision's type is the planner's; this slice does not.
4. **The roots triple.** `internal/ui/snapshot` declares no triple; it takes the state root alone, `New(roots.StateRoot, time.Now)`, because every ledger read takes that one root (D20); `internal/backlog` takes none. The payload's `ledger.stateRoot` is that same value, which `/api/workspace` already reports.

Also stated for the planner: this slice adds no goal route and does not touch `routes.ts`; the Project design's line that "the goal route is `g1-s10`'s" (its `:94`) is the planner's to reconcile with the plan's `g1-s11` row.
