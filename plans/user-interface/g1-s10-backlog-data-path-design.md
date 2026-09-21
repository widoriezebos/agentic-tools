# g1-s10 Backlog data path: the list

- Gate 1, state `designed`, revision 1, author Claude on Fable as a D23 delegate, 2026-09-21. Delivery step 3 of D21 (plan `:153`): a thin cut through `g1-s2`, `g1-s4`, `g1-s7`, and the list half of `g1-s10`, none in full.
- Refines the [master](../user-interface-design.md) at `ui-development` commit `f5768f525`: Backlog and the lane table (`:492` to `:535`), Rules for an intuitive structure (`:433` to `:449`), Workspace identity (`:451` to `:461`), freshness (`:257`), Trustworthy state (`:769` to `:777`), the gate 1 row (`:809`); M5, M6, M7 (plan `:141`, `:142`, `:145`).
- Depends on `g1-s9` first cut (merged at `548514ad5`), `g1-s8` revision 5, `g1-s1` revision 6; D1, D2, D11, D20, D21. Contributes to scenarios 34, 21, 47, 15; discharges none in full.

Paths are relative to `metasystem/` unless they start with `plans/`. Every code claim was read whole in source at `f5768f525`; nothing was executed except two `curl` reads of the running server and read-only `git` and `ls` queries.

## Outcome

A human opens Backlog and, once the checkout has an accepted tip, sees this repository's real goals as a list grouped by lane: every live goal placed once, badges for what the record says, named gaps for what it does not, the tip's identity and age, and when the server read it. The 429 done and 1 abandoned goals sit behind one toggle. Nothing is mutable; the Brain stays absent. Before the tip exists, the pane says so, counts what the working tree holds that it will not read, and names the one-time step.

## Scope and non-goals

| Slice | Taken | Left |
| --- | --- | --- |
| `g1-s2` | The root rule; the accepted ref only, never worktree bytes; the tree cached by tip; time-dependent answers per observation; tip identity and observation time | Local sources; the two banner strings, replaced by the facts behind them; other consumers |
| `g1-s4` | A package outside `internal/ui` placing every goal once with Waiting precedence, the closed set, badges, M5 and M6 as data, `LaneOf` for `g1-s16` | Dependencies as a graph, history beyond the last line, arcs, budgets, risk, the CLI adopting it |
| `g1-s7` | One JSON route; identity by kind, id, revision; `schemaVersion`; the per-request `Info` seam | The event stream and its three facts; `sourceHead`; every other route; the written conventions |
| `g1-s10` | The list by lane; the closed-items filter | Board, other filters, search, selection, drag, outline, dependencies |

Non-goals: fetching (`g1-s3`), goal detail (`g1-s11`), a `goal` kind in the resolver, pagination, virtualisation, the shell's second cut, dependencies.

## Existing code this builds on

| Fact | Where |
| --- | --- |
| `ResolveEndpoint`: `goal.sync-remote` (default `origin`), `goal.sync-branch` (default `refs/heads/main`); `AcceptedRef` | `internal/goal/txn.go:51`, `:61` to `:73` |
| `Project(e, false, now)`: rev-parse, `loadTree`, horizon, sync-mode gate, two banners; `true` fetches and moves refs | `project.go:72` to `:125` |
| `AcceptedLedgerTip`: absent `("", false, nil)`; ref without `backlog.md` `(tip, false, nil)`; unreadable ref file or non-commit is an error | `attention.go:543`; `txn.go:158` to `:204` |
| A tree with any problem refuses whole, naming file and rule (`*TreeReadError`); the tree is `plans/goals/` and `records/goals/` of one commit, relative to the root | `validate.go:3`, `:66`, `:90` to `:151`, `:599` to `:667`; `verbs.go:339`; `stateroot.go:243` |
| `Live`, `Done`, `Abandoned`; six states; the fence beats landing; only relayed approvals expire, against `now` and the first enrollment | `validate.go:27`, `:154` to `:180`; `file.go:95` to `:105`, `:290` to `:306`, `:511`; `approval.go:253` |
| Readiness (approved, unexpired, blockers done); `Next` needs a machine and tier-box config; scheduling order; `StaleThreshold`; the `%ct` banner | `project.go:37`, `:113`, `:652` to `:724`; `verbs.go:2279`; `order.go:31` |
| `listSynced` groups by state, done by id; `goal fetch --root` runs `FetchAdvance`, refs only | `cmd/metasystem/goal.go:394`; `goal_list.go:145`; `goalsync_verbs.go:396`; `fetchadvance.go:30` |
| Three roots; `Info.Describe` per request; `/api/workspace` exact; the wiring; the shell's call site, state, guard, empties, routes | `ui/lifecycle/roots.go:31`; `ui/httpd/httpd.go:20`, `:126`, `:208`; `cmd/metasystem/ui.go:130`; `_app/src/shell/workspace.ts`, `identity.tsx`, `cuts.test.ts:313`, `panes/empties.ts`, `shell/Shell.tsx:107` |

The checkout today: no accepted ref, no `goal.sync-*` configuration, `origin` is `github.com/widoriezebos/agentic-tools`, `origin/main` is `c5d517f42` (2026-09-21T18:39:42Z). Tracked: 156 files under `plans/goals/`, one the root record, **155 goals** (119 queued, 33 parked, 2 approved, 1 claimed); 430 under `records/goals/` (429 done, 1 abandoned); 7 under `plans/goals-drafts/`, unread by Go. Plan `:153`'s "156 live" counts the root record.

## Contracts

### `internal/backlog`

Outside `internal/ui/` (plan `:183`); imports `internal/goal`, nothing under `internal/ui`; pure.

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
type DraftGap struct{ Files *int; Statement string }
func Project(tree *goal.TreeGoals, horizon goal.ApprovalHorizon, draftFiles *int) Board
func LaneOf(f *goal.GoalFile, tree *goal.TreeGoals, horizon goal.ApprovalHorizon) (Lane, phase string, gaps []string)
```

`Rows` is every live goal in `goal.OrderedOpenGoalIDs` order, lane-tagged; `Closed` is `Done` then `Abandoned`, each by `goal.SortedGoalIds`, as `listSynced` orders them. Every goal appears once. `Counts` has every lane but `draft`. Placement, Waiting first (master `:531`), phase per M5:

| Record | Lane, phase | Gaps and badges |
| --- | --- | --- |
| claimed, `StopFence` set | waiting, not recorded | `Fence`; reason is the fence's; landing ignored (`file.go:104`) |
| claimed, `Landing` set | review, `landing` | `Claim.LandingAt` |
| claimed otherwise | in-progress, not recorded | gap `phase not recorded` |
| parked | waiting, not recorded | `Waiting{Reason: Because, Since, By, Blocker, From: "claimed" if Displaced set, else "not recorded"}` |
| approved, relayed, `ApprovalExpired` | to-do | gap `approval expired: <why>` |
| approved, a `Blocked` id not `done` | waiting | `OpenBlockers`; an id absent from the tree is named `unknown goal`; `From: "ready"` |
| approved otherwise | ready | none; claim admission is not evaluated, since `Next` needs a machine and answers what one seat may claim |
| queued | to-do | gap `not approved`; blockers listed, placement kept |
| done | done | `Decomposed` when the id is in `Root.Decomposed` (`root.go:32`), shown "split into goals" (master `:533`) |
| abandoned | abandoned | `Abandoned{Because}` |
| any other live state | unknown | gap `state <s> is not placed by this build`, never dropped |

`Pinned`, `Labels`, `Arc`, `Tier`, `Priority:Sequence`, `Sliced`, `Origin` are badges, never lanes. `LastChangeAt` and `LastVerb` are the last History line. `DraftGap.Statement` is fixed: "plans/goals-drafts/ has no reader in the engine; a drafts owner arrives with gate 5 (M6)".

### `internal/ui/snapshot`

```go
func New(stateRoot string, now func() time.Time) *Holder
type State string // read | absent | no-ledger | broken | unreadable | refused
type Observation struct { ObservedAt time.Time; State State; Tip string; AcceptedAt time.Time; Message string; Problems []string
    Tree *goal.TreeGoals; Horizon goal.ApprovalHorizon; LiveFiles, ArchivedFiles, DraftFiles *int }
func (h *Holder) Observe() Observation
```

`stateRoot` is `lifecycle.Roots.StateRoot`, derived by `stateroot.RootForInstallation` (`roots.go:60`): `metasystem/` here, the application's repository root when adopted (D20); the root `ReadCommitGoals` needs. `Observe`, under one mutex:

1. `now := h.now().UTC()`. `goal.AcceptedLedgerTip(root)`: an error is `broken` with the engine's message; not existing is `absent` when the tip is empty, else `no-ledger`.
2. If the tip is the held one, reuse the tree. Else `goal.ResolveEndpoint(root)` (error: `refused`), then `goal.Project(e, false, now)`, the literal `false`: `*goal.TreeReadError` is `unreadable` with `Problems`; any other error is `refused` (the sync-mode gate). Hold `p.Tree` under `p.Tip`, the tip `Project` read, not the one probed. Read `AcceptedAt` once per tip with `git -C <root> log -1 --format=%ct <tip>` through `os/exec`, this package's only git call, mirroring `project.go:113`; failure leaves it zero, never fatal.
3. `Horizon` is `goal.ApprovalHorizon{Now: now}` with `EnrolledAt` parsed from `Tree.Root.FleetEnrollment.At` when present: `approval.go:253` to `:259` written out from exported fields, recorded so `g1-s16` can export the original.
4. `DraftFiles` counts `*.md` in `plans/goals-drafts/`, always; `LiveFiles` (`plans/goals/*.md` less `backlog.md`) and `ArchivedFiles` (`records/goals/*.md`) only in `absent` and `no-ledger`. An unreadable directory gives `nil`, never zero. Working-tree counts, labelled so; no goal is read from them.

Cached by tip: the parsed tree. Not cached, and why: `ObservedAt`, `Horizon`, expiry, and staleness change while the tip stands still; the ledger state, because the ref can appear, break, or move; the counts, because the worktree is not the tip. The sync-mode gate runs at tip change only; a later configuration flip is seen at the next change, stated rather than hidden. The two banners `Project` emits (`project.go:108`, `:119`) are not carried: one embeds a terminal command, both derive from facts the observation holds, and a string computed at load would go stale under the cache; a wording deviation from the `g1-s2` row, recorded for the planner.

### The route

`GET /api/backlog` in `internal/ui/httpd`, exact; beneath it 404. `Info` gains `Observe func() snapshot.Observation`, called per request; `nil` answers 500 `{"error":"this engine was built without a ledger reader"}`. In state `read` the route calls `backlog.Project(obs.Tree, obs.Horizon, obs.DraftFiles)`:

```json
{"schemaVersion":1,"observedAt":"2026-09-21T19:05:12Z",
 "ledger":{"state":"read","tip":"c5d517f4…","acceptedAt":"2026-09-21T18:39:42Z","stale":true,"staleAfterSeconds":1800,
           "syncMode":"remote","message":"","problems":[]},
 "workingTree":{"liveFiles":null,"archivedFiles":null,"draftFiles":7}, "counts":{"to-do":119,"…":0}, "draft":{"files":7,"statement":"…"},
 "rows":[{"ref":{"kind":"goal","id":"tests-parallel-and-deterministic","revision":9},"where":"live","lane":"in-progress",
          "phase":"not recorded","state":"claimed","gaps":["phase not recorded"],"…":"…"}], "closed":[]}
```

`stale` is `observedAt − acceptedAt > goal.StaleThreshold`, `false` and `acceptedAt` `""` when unknown. In every state but `read`: `rows` and `closed` empty, `counts` `{}`, `message` and `problems` the engine's words, status **200**, because the workspace answered and the answer is why the ledger cannot be read; 500 is for the engine not answering. Identity is `ref`: kind, id, revision (master `:319`, `:325`); no route, no component name. A Go test asserts the field names verbatim.

`g1-s7` still owes: the event stream, its path, event shape, and view names (`backlog`, `workspace`); `sourceHead`; the detail, decisions, and fleet routes; the written reference and refusal conventions; the payload rules the brain's tools consume. This route is shaped to survive them.

### Frontend

New under `_app/src/backlog/`: `api.ts` (`loadBacklog(signal)`: `fetch("/api/backlog", {headers: {Accept: "application/json"}})`, the second call site, and the types), `state.ts` (`useBacklog()`: `loading | failed | known`, the `identity.tsx` pattern with an `AbortController`, on mount and on Refresh), `lanes.ts` (ids, titles, one-sentence meanings, order), `format.ts` (RFC 3339 to local `YYYY-MM-DD HH:MM`; ages), `BacklogPane.tsx`, `LedgerLine.tsx`, `LaneGroup.tsx`, `GoalRow.tsx`, `Statement.tsx`, `backlog.css`. Edited: `shell/Shell.tsx` routes `/backlog` to `BacklogPane`; `panes/empties.ts` loses its `backlog` row and `empties.test.ts` its expectation and `wanted` entry; `cuts.test.ts` as under Verification. `routes.ts` is unchanged: `/backlog/x` stays not found, `routeFor({kind:"goal"})` stays `null`. `package.json` and the lockfile do not change.

`Statement` is the one new component, with its reason: `EmptyState` renders a static table row by id (`Pane.tsx:31`), and these statements carry counts and engine text per response. It takes `heading`, `body`, `note`, children, and uses the `ms-empty*` classes unchanged. Wording keeps the shell's conventions: a heading is a name without a full stop, a body is sentences, the note is the 12px line, nothing asserts the absence of records.

**The pane**, top down: `LedgerLine`, 13px `text-2`, "Accepted tip `c5d517f`, committed 2026-09-21 20:39 · observed 21:05:12", a Refresh `Button`; when `stale`, a `text-3` line "The accepted tip is 3 h old. This build does not fetch: publications from other machines appear after a goal verb or `goal fetch` runs in this checkout; the fetch loop arrives with g1-s3". Then the lane index, one wrapping line of in-page anchors "To Do 119 · Ready 2 · … · Draft not read". Then one `ms-card` per lane in `LaneOrder`, title "<Lane> (<n>)", the meaning in 12px `text-3`; an empty lane reads "No goal is in this lane at tip `c5d517f`", a fact about a named projection; the Draft card is the M6 `Statement` with the file count. Then a `Button` with `aria-pressed`, "Show closed items (430)", mounting the Done and Abandoned cards, off by default, React state only. A `text-3` footer: "Board, outline, dependencies, filters, and goal detail arrive with g1-s10 and g1-s11".

**A row**, 8px vertical padding, 1px `border` between, no hover since nothing selects: line one, the id in mono 13px, then `Chip`s: state, `tier n`, `p:s` or `unranked`, `pinned to <machine>`, each label, `arc <name>`, `sliced`, `split into goals`, `rev n`; line two, the intent, 14px, in full; line three, 13px `text-2`: the next step (live), the concluded text (done), the reason (abandoned); line four when present, 12px `text-3`: gaps and reasons ("phase not recorded"; "not approved"; "approval expired: …"; "parked by human:Wido since …, from claimed: <because>"; "blocked by a, b (open)"; "landing since …"; "claimed by m1e since …"), then "opened <date> · last <verb> <date>". Nothing is truncated; the record bounds every field (`goal.go:28`). Ids are copyable text; no row is a link.

Selectable: nothing. Controls in DOM order: Refresh, the anchors, the toggle; rows are not tab stops. Narrow: rows and chips wrap, no `min-width`, no horizontal scroll. `backlog.css` sets layout and token-backed classes (`ms-goal-row`, `ms-goal-id`, `ms-goal-intent`, `ms-goal-next`, `ms-goal-gaps`, `ms-lane-index`, `ms-lane-meaning`, `ms-ledger-line`) only; no colour literal, no `style` attribute.

### Scale and freshness

One request, one payload of all 585 rows, estimated under 500 KB on loopback and measured in the walkthrough. The 155 live rows render at once; the 430 closed only while the toggle is on. No pagination, no virtualisation: the simplest arrangement is measured before a cleverer one is designed; more than a second from response to paint is a finding for the board slice.

No stream, no timer. The list is read when the pane mounts (each visit, each reload) and on Refresh; leaving aborts a request in flight. Newer data reaches the checkout only when something moves the accepted ref: a goal verb, `goal fetch`, or the `g1-s3` loop. The human sees the tip, its commit time, the observation time, and the stale line. After this slice there are exactly **two** network call sites, `shell/workspace.ts` and `backlog/api.ts`. The shell's first-cut guard asserted one; it changes as under Verification, and the shell's second cut later raises the allowlist to four. This amends `g1-s9`'s O14; the planner notes it on that row.

## Behaviour

Main flow: the pane mounts, shows a `Skeleton` in the ledger line, requests `/api/backlog`; the server observes (a cache hit after the first read at a tip), projects, answers; the pane renders.

Failure and edge cases:

| Case | Required result |
| --- | --- |
| The ref is absent (today) | 200, `absent`, the counts. Heading "No accepted tip in this checkout"; body "This build reads goals only from the accepted ledger ref, never from the working tree. The ref refs/metasystem/goals/accepted does not exist here, because no goal verb has run in this checkout. The working tree holds 155 live goal files, 430 archived records, and 7 draft files, none read until the tip exists."; note "One-time step, from metasystem/: bin/metasystem goal fetch --root . ; the interface will do this itself when g1-s3 lands". Refresh stays. Naming a command is master `:733`'s uncovered workflow, stated as such and closed by `g1-s3`; the fetch runs on the human's word (plan `:13`) |
| The ref's commit has no `backlog.md` | `no-ledger`; "The accepted tip carries no ledger"; the tip named, "it predates the ledger"; the same note |
| The ref file is unreadable or not a commit | `broken`; "The accepted ref cannot be read"; the engine's message (`txn.go:179`, `:188`); note "Repair refs/metasystem/goals/accepted before continuing" |
| A file at the tip fails to parse or validate | `unreadable`; "The ledger at `<tip>` does not parse"; "The engine refuses a tree with any problem whole rather than showing part of it"; the problems one per line in mono; never a partial list |
| Sync mode contradicts the configuration, or no endpoint | `refused`; "The ledger could not be projected"; the engine's message; note "Check goal.sync-remote and goal.sync-branch in this checkout's git configuration" |
| 404 (older server), 500, unparsable, or the fetch fails | "The backlog could not be read" 14px `text-2`, the message 12px `text-3`, Retry: the shell's workspace-failure pattern; rail and header work |
| A blocker id is not in the tree | Waiting; `openBlockers` lists it; the reason says "unknown goal" |
| A live state outside the six | Lane `unknown`, "Unknown (n)", the state named |
| `acceptedAt` unreadable | "committed at: unknown"; `stale` false; no stale line |
| The tip moves; a review date passes with no tip change | The next observation loads the new tree; the next observation moves the goal to To Do with the gap, no reload |
| The pane is left before the response; toggle on, then Refresh | Aborted, no state written; the toggle stays on and closed rows re-render |

## Change boundary

New: `internal/backlog/` (`lanes.go`, `project.go`, tests, `testmain_test.go`); `internal/ui/snapshot/` (`snapshot.go`, `counts.go`, tests, `testmain_test.go`); `internal/ui/httpd/backlog.go`, `backlog_test.go`; `_app/src/backlog/**`. Edited, and nothing else in them: `internal/ui/httpd/httpd.go` (the `Observe` field, the exact-path branch beside `workspacePath`); `cmd/metasystem/ui.go` (one `snapshot.New(roots.StateRoot, time.Now)` in the `serve` case, `holder.Observe` in `Info`; wiring only); `docs/architecture.md` (rows for `backlog` and `ui/snapshot`; the `ui` family row gains `ui/snapshot`, `ui/workspace`, `backlog`, `goal`); `shell/Shell.tsx`, `panes/empties.ts`, `panes/empties.test.ts`, `cuts.test.ts`; `bundle/**` regenerated last (D7). `httpd_test.go` may gain the route in an existing route table only.

Must not be touched: `internal/goal/**` (composition only); `cmd/metasystem/goal.go`, `goal_list.go`, `goalsync_verbs.go` (`g1-s16`'s); `main.go`; the plan's list at `:107` (`testing-parallel-ratchet.json`, `testing.json`, `internal/testenv`, `testutil`, `parallelratchet`, `testselect`, `testpolicy`, `testexec`, `hostload`, `proofrun`, `cmd/metasystem/test*.go`, `audit.go`); `go.mod`, `go.sum`, `gopackages`, `stateroot`, `config` (no new key), `ui/lifecycle`, `ui/web/*.go`, `ui/workspace`, `scripts/**`, `metasystem.conf`, `metasystem.conf.local` (never read); `g1-s8`'s `vite.config.ts`, `tsconfig.json`, `scripts/*.mjs`, its policy and page rule; `package.json`, `package-lock.json`; any other existing `*_test.go` or `testmain_test.go`. The interface never calls `FetchAdvance`, `Project` with `true`, `AdvanceAccepted`, or anything that moves a ref.

Conventions, repeated because the builder sees only this document: every new Go package has `testmain_test.go` with `func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }`; tests never call `os.Setenv`; every top-level test and independent subtest calls `t.Parallel()`, the ratchet allowing an unlisted package zero serial tests; no sleeps, polling, or fixed ports; the clock is injected; temporary repositories under `t.TempDir()` seeded with `exec.Command("git", "-C", root, ...)` as `internal/goal/servingprojection_test.go:12` to `:75` does, with `goal.RenderRoot`, `goal.RenderFile`, and `git update-ref refs/metasystem/goals/accepted HEAD`; `testutil.Expect` and `Require`; `cmd/metasystem/ui.go` wires and prints only; no new Go module dependency (`os/exec`, `encoding/json`, `sync`, `time` suffice). Frontend: exact versions, `npm ci --ignore-scripts`, the Node pin (D27), Vitest in `node` with no DOM library, no colour literal outside `tokens.css`, no `style` attribute, no dependency this design does not name (none), the bundle rebuilt last.

## Verification

**Go, `internal/backlog`.** A table test over `LaneOf` for every placement row, each a hand-built `goal.GoalFile` in a hand-built `goal.TreeGoals` with a `RootRecord`: queued; queued with an open blocker; approved unblocked; approved blocked by a live goal, by an absent id, by a done goal (waiting, waiting, ready); approved relayed with `ReviewBy` before and after the horizon's date; approved with `FleetEnrollment` set; claimed plain, with `Landing`, with `StopFence`, with both; parked with and without `Displaced`; done; done in `Decomposed`; abandoned; a live state `"weird"`. `Project`: every id once across `Rows` and `Closed`; `Rows` in `OrderedOpenGoalIDs` order; `Closed` done by id then abandoned by id; `Counts` complete; `draftFiles` nil and 7. A JSON round trip asserts every `Row` field name verbatim.

**Go, `internal/ui/snapshot`.** On a seeded repository (`goal.sync-remote` `local`, a `SyncLocal` root): no ref, `absent` with the three counts, a missing drafts directory giving `nil`; a ref at a commit without `backlog.md`, `no-ledger`; a loose ref file holding garbage, `broken`; a corrupted Integrity line, `unreadable`, `Problems` naming the file; a `SyncMode: remote` root against the local configuration, `refused`; a good tip, `read`, forty-hex `Tip`, `AcceptedAt` near the fixture commit's time; cache: a second `Observe` returns the same `*goal.TreeGoals` pointer, then `git update-ref` to a commit adding one goal and a third `Observe` holds it under a new pointer; time: a relayed approval with `ReviewBy` on day D, the injected clock at D unexpired and at D+2 expired, same pointer; eight goroutines observing at once under `-race`.

**Go, `internal/ui/httpd`.** From a faked `Observe`: 200 and the field names for a three-goal `read` observation; `absent` answers 200 with `state`, counts, empty `rows`; `nil Observe` answers 500 with the error shape; `/api/backlog/x` and `/api/backlogs` are 404; the headers and policy are on the response; `Observe` runs once per request.

**Frontend, Vitest in `node`.** `lanes.test.ts` (ids, order, titles, meanings verbatim, `unknown` present); `format.test.ts` (RFC 3339 to local text, ages, empty `acceptedAt` gives "unknown"); `api.test.ts` (with `globalThis.fetch` replaced: a non-2xx status throws naming the status, a 200 returns the payload); `empties.test.ts` revised as above; `cuts.test.ts` revised: `CALL_SITES` is `[["backlog/api.ts", "/api/backlog"], ["shell/workspace.ts", "/api/workspace"]]`, "exactly one place" becomes "exactly the named places, one `fetch` each, each naming its resource", and every other rule (no `EventSource`, `WebSocket`, `XMLHttpRequest`, `sendBeacon`, timer, lifecycle event, or health path; the second-cut files absent) stays as written.

**Walkthrough**, in order, in a real browser, by Claude then the human, from `metasystem/`: `go build ./...`; from `_app/`: `npm ci --ignore-scripts`, `npm run typecheck`, `npm test`, `npm run bundle`; `bin/metasystem ui restart`; open `/backlog`: the absent statement with 155, 430, and 7, Refresh, header and rail intact; `git status --porcelain` clean. On the human's word: `bin/metasystem goal fetch --root .`, expected `advanced=true tip=c5d517f4… accepted c5d517f`; `git rev-parse refs/metasystem/goals/accepted` equals `origin/main` if the remote has not moved; `git status` still clean; HEAD unchanged. Press Refresh without reloading: the list; lane counts summing to 155 and closed to 430, recorded in the evidence note; `tests-parallel-and-deterministic` in In Progress, "phase not recorded", revision 9; the parked goals in Waiting with reasons; the ledger line with `c5d517f`, 20:39 local, and the stale line; `bin/metasystem goal list --root .` printing `claimed=1 approved=2 queued=119 parked=33 done=429 abandoned=1 tip=…`, matching the page. Network panel: one `/api/backlog` per visit and per Refresh; one `/api/workspace` on load; nothing on refocus, on an offline and online toggle, or over two idle minutes; the response size noted. `curl -s -H 'Accept: application/json' http://127.0.0.1:7878/api/backlog | head -c 300`; `/api/backlog/x` gives 404. Toggle closed items on, scroll, Refresh, still on. Widths 1,280, 800, 599, 400: no horizontal scroll. Keyboard: Refresh, the anchors, the toggle, nothing else. VoiceOver reads each lane heading with its count. Light and dark; zero policy violations; no request off the origin. `bin/metasystem audit parallel-ratchet` without `--update`; `go vet` and `go test` on `./internal/backlog ./internal/ui/...`; `g1-s8`'s Git-fed gofmt pipeline.

Obligations the code critique checks by name:

- **O1, no ref moves.** `Project` is called with the literal `false`; `FetchAdvance` and `AdvanceAccepted` are never called; `git status` and HEAD are unchanged around every request.
- **O2, the tip, not the worktree.** No goal is read from disk; the counts are a labelled `ReadDir` and reach no row.
- **O3, cache by tip only.** The pointer-identity and clock tests pass; `Horizon` and `ObservedAt` are per observation; the key is `p.Tip`.
- **O4, every goal once.** The uniqueness test passes; `Counts` sums to `len(Live)` over live lanes and `len(Done)+len(Abandoned)` over closed.
- **O5, Waiting precedence.** Fence and landing together is waiting; blocked approved is waiting; blocked queued stays to-do.
- **O6, honest gaps.** Every in-progress row carries `phase not recorded`, every queued row `not approved`; the draft statement names M6 and gate 5; `empties.test.ts`'s `ABSENCE` list applied to `lanes.ts` and every `Statement` caller finds nothing.
- **O7, six ledger states.** Each renders from a faked payload; 200 in every state; 500 only for `nil Observe`.
- **O8, identity by reference.** Every row carries `ref.kind`, `id`, `revision`; no route or component name in the payload; `routeFor({kind:"goal"})` is `null`; no row links.
- **O9, two call sites.** `cuts.test.ts` passes as revised; the browser request pattern holds.
- **O10, shared owner.** `LaneOf` and `Project` live in `internal/backlog` with no `internal/ui` import; `cmd/metasystem/goal*.go` untouched.
- **O11, shell untouched.** No shell token, class, or component changes; `literals.test.ts`, `contrast.test.ts`, the revised `empties.test.ts` pass.
- **O12, bundle current.** The last commit rebuilds the bundle; `go test ./internal/ui/web/` passes; the notices file is unchanged.

## Open questions

The human's, each with a recommendation and the consequence of each option.

1. **Closed-items order.** Recommended: by id, as `goal list` prints them, so terminal and browser agree until `g1-s16`. Alternative: newest concluded first by the last History time; easier to read, a second rule to reconcile later.
2. **Expired approval placement.** Recommended: To Do with the gap visible, since the goal is not eligible and the master lets an item keep To Do with its approval gap shown. Alternative: Waiting, which would claim a blocker where what applies is a lapsed authority.
3. **Who runs the one-time fetch.** Recommended: the human, once, from a terminal; it moves refs only. Alternative: the planner runs it on the human's word during the walkthrough. The interface never runs it in this slice.
