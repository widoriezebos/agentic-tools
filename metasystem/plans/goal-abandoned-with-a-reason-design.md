# A goal that will never be worked, and why (goal goal-abandoned-with-a-reason)

Revision: 1. Date: 2026-09-09. Design authoring job: `gawr-design1`.

This page specifies the build; it implements nothing. Independent critique,
dispositions, certification, the ledger and the receipt belong to the
orchestrator. All paths are relative to the repository root. Symbols marked
"new" do not exist in the tree yet. Line numbers name the revision read for
this design, worktree `agent/gawr-design1` at `96c6098b`.

The contract is the Intent of `metasystem/plans/goals/goal-abandoned-with-a-reason.md`:

> DONE means an explicit abandoned terminal state carrying its reason,
> excluded from goal next, the idle refusal, the ranked listing and the claim
> quota, satisfying no dependency, exempt from prune, reopenable by a human
> into queued, reachable for a breach-stopped goal, and proven by fixtures
> for each of those.

## 0. The decision in one paragraph

`abandoned` becomes the sixth state of the closed state list. An abandoned
record leaves the live set the way a done record does: its file moves to the
archive directory `records/goals/`, its rank compacts out of the live
sequence, and its claim is gone. It differs from done in every reader that
gives done a meaning: it satisfies no dependency, it is never counted as
completed work, it is never pruned, and it comes back only through a human
reopen into queued, unranked and unapproved. The transition is a new verb,
`goal abandon`, a human act at the enrolled terminal. It does not go through
`clearClaimBinding`, so it reaches a breach-stopped goal: it removes the
claim (the execution authority) and freezes the stop fence on the record as
evidence. The ledger tree gains a third map, `Abandoned`, beside `Live` and
`Done`, so that every reader that today equates "in the archive map" with
"done" keeps meaning done, and every reader that needs abandoned records
opts in by name.

## 1. Grounding in the current tree

| Responsibility | File and symbol | Observed behavior and consequence |
| --- | --- | --- |
| Closed state list | `metasystem/internal/goal/file.go:354` (`StateQueued` … `StateDone`), `:363` `validState`, `:430` the parse problem naming the five states | A sixth state is a grammar change. An older binary refuses any tree that carries it (section 10). |
| Record grammar | `file.go:699` `parseFileField` (unknown field refused at `:985`), `:1040` `parseKVRecord` ("the record grammar is closed" at `:1058`), `:976` the `Parked` case, `:1104` `splitParkTail` | New field lines and new keys must be added to these closed sets; free-text tails are byte-exact. |
| History grammar | `file.go:287` `HistoryLine`, `ParseHistoryLine` (unknown key refused at `:1421`), `RenderHistoryLine` | Optional keys are a closed set with duplicate guards; `reason=` consumes the rest of the line. |
| Tree shape | `metasystem/internal/goal/validate.go:27` `TreeGoals{Root, Live, Done, DonePaths}`, `:63` `ParseTreeFiles`, `:113` `parseDoneAt` | Placement is by path: everything under `records/goals/` (or the read-only legacy `plans/goals/done/`) lands in `Done`. There is no notion of an archived record that is not done. |
| Placement rule | `validate.go:153-167` | Live records may not be done; archived records must be done and carry a conclusion. |
| Dependency reader | `metasystem/internal/goal/verbs.go:1184` `depState` | Returns `StateDone` for every id in `Done`. `Next` (`project.go:573`), `doneRequest` (`verbs.go:1146`), and the validator's `stateOf` (`validate.go:219`) all compare against `StateDone`. |
| Referential integrity and cycles | `validate.go:214` `exists`, `:230` `forAll` loop, `:253` `findCycle` | Every edge must name a live or archived goal; cycles are found over the composed graph. |
| Done-under-blocker rule | `validate.go:289` | A done goal's blockers must be done. This is why no done goal can be blocked by a live goal, and so no done goal can be blocked by a goal that is later abandoned. |
| Claim clearing | `verbs.go:276` `clearClaimBinding` | Refuses while `StopFence` is set. `done` (`:1156`), `park` (`:1260`), `release` (`:895`) all call it. `set-budget` refuses at `:730`; `steal` at `:1736`; `resume` (`stop.go:403`) needs the exact stop batch complete. This is the wedge. |
| Stop authority at rest | `file.go:468`, `:535-558` | Stop authority is allowed only on a claimed record; a fence needs its capability; the capability must match the claim binding. |
| Done transition | `verbs.go:1106` `doneRequest` | Refuses open review obligations, foreign claims and parked goals for agents, human-origin goals for agents, and unfinished blockers; moves the file, compacts the departed priority, records displacement, acks displacements. |
| Reopen | `verbs.go:1337` `reopenRequest` | From `Done` only; refuses decomposed parents and claimed dependents; clears conclusion, approval, budget and norm; keeps priority and appends its sequence; arc join rules; clears Goal-free. |
| Prune | `verbs.go:1813` `pruneRequest` | Retains closure(live blockers ∪ newest keep done) walked through done-to-done edges; deletes the rest of `Done`; retains relayed-authority lines on the root. |
| Frontier | `metasystem/internal/goal/project.go:543` `Next` | Iterates `Live` only. |
| Idle refusal | `project.go:280` `ReadClaimableBudgetedWork` → `Next`; `metasystem/internal/goal/turnverdict.go:426` `enforceIdleBacklog`; `metasystem/internal/steward/openwork.go:51` | Both derive from the frontier and the live queued count. |
| Listing and show | `metasystem/cmd/metasystem/goal.go:339` `listSynced`, `:420` `runGoalShow` | The pretty table lists live goals in rank order and prints `done: N archived`; JSON groups by state; show falls back from `Live` to `Done`. |
| Counts | `metasystem/internal/metrics/compute.go:148` `archiveVerbs`, `:156` `concludedAt`, `:197`, `:624` debt age; `metasystem/internal/counselor/sources.go:578` `goalVerbClass`, `:542` history over `Done` | Concluded means State done with a done or split line; the counselor's product count is the `done` verb class. |
| Archived lookups | `metasystem/internal/evidence/gc.go:502`, `metasystem/internal/goal/norm.go:87`, `metasystem/internal/dispatch/slice.go:160`, `metasystem/internal/validate/conformance.go:1206`, `verbs.go:1089`, `split.go:342`, `reconcilepub.go:150`, `metasystem/internal/channel/report.go:73` | Each reads `Done[id]` as "the archived record of id", not as "completed". |
| Recovery replay | `metasystem/internal/goal/recover.go:309-313`, `:413` | Proof-bearing verbs are not replayed from journal text. |
| Hand edits | `metasystem/internal/goal/reconcilemap.go:117`, `:264-283` | The archive has no hand-edit grammar; a state change outside the pinned cases refuses. |
| Human proof | `metasystem/cmd/metasystem/goalsync_mutations.go:677` `proveGoalHumanAuthority("set-priority", shared, prove)`, `:682` `syncReqWithProof` | The enrolled-terminal proof pattern for a human-only verb. |
| Dispatch and landing on a non-claimed goal | `metasystem/internal/dispatch/admission.go:132`, `metasystem/internal/landing/observe.go:748` | Both require State claimed with a claim record. They need no change; they are what makes an abandoned goal inert. |
| Drop rule | `metasystem/docs/backlog-mechanism.md:102-106` | "losing that, it concludes" teaches the false-done workaround. |
| Open-work scanner | `metasystem/internal/report/openwork.go:50` `settledStep`, `:435-446`; `scan.go:158-178` | A page is open work when its `- Next step:` value is not exactly a settled word; a non-empty `- Waiting on the human:` line exempts it. |

## 2. The state and its record fields

New constant `StateAbandoned = "abandoned"` in the closed list at
`file.go:354`; `validState` admits it; the parse problem at `:430` names six
states.

New record field, rendered after `- Parked:` in `RenderFile` and parsed by a
new `Abandoned` case in `parseFileField`:

```
- Abandoned: by=human:<name> at=<RFC3339> revision=<n> [displaced=<machine>+<lineage>@<at>] [stopId=<id>] [carried=<goal-id>] because=<free text to end of line>
```

Closed keys: required `by`, `at`, `revision`; optional `displaced`, `stopId`,
`carried`; free tail `because`, extracted byte-exact by a new
`splitAbandonTail` that generalizes `splitParkTail` (`file.go:1104`) over the
three optional keys. New struct:

```go
type AbandonRecord struct {
    By, At    string
    Revision  uint64 // the History event of the abandon act, like ApprovalRecord.Revision
    Displaced string
    StopID    string // the frozen fence's stopId; empty when the goal was not stopped
    Carried   string // the successor goal named by --carried; empty otherwise
    Because   string
}
```

`GoalFile` gains `Abandoned *AbandonRecord`. The History grammar gains two
optional keys, `stopId=` and `carried=`, parsed with the same duplicate guard
as `displaced=` and rendered immediately after `displaced=` (or after
`targets=` when there is no displacement) and before `ack`. They appear only
on `abandon` lines. They exist so that a reopen, which removes the
`Abandoned` record from the fields, leaves the stop id and the successor in
the append-only History (section 6).

The history line of the act:

```
- <at> <opid> abandon actor=human:<name> targets=<id>[,<compaction targets>] [displaced=<pair>@<at>] [stopId=<id>] [carried=<goal-id>] reason=<because>
```

Fields kept on the abandoned record, unchanged: `Intent`, `Origin`, `Next
step`, `OpenedAt`, `Priority` and `Sequence` (historical, exactly as on a done
record), `Tier`, `Risk`, `Labels`, `Arc`, `Pinned`, `Budget`,
`BudgetExceptions`, `NormApproval`, `Approved` (audit evidence, as the comment
at `file.go:54` already says), `Sliced`, `Ratified`, `ReviewObligations`
(open ones stay open and readable: the record shows the debts that were
abandoned with the goal), `AcceptedRisks`, `BlockedBy` (an abandoned goal keeps
its own prerequisites, finished or not), `History`, `LegacyNotes`.

Fields removed by the act: `Claimed`, `Obligation`, `Parked`. `Concluded` is
never present on an abandoned record.

Stop authority: when the goal carries a `StopFence` at the moment of
abandonment, both `StopCapability` and `StopFence` are kept byte-identical
(frozen evidence) and `Abandoned.StopID` equals the fence's stop id. When
there is no fence, `StopCapability` is removed (a capability without a fence
is live authority, not evidence).

## 3. The tree shape

`TreeGoals` gains `Abandoned map[string]*GoalFile` and `AbandonedPaths
map[string]string`. `parseDoneAt` becomes `parseArchivedAt`: it parses the
file, refuses a duplicate id across `DonePaths` and `AbandonedPaths`, then
routes by the parsed State: `done` to `Done`, `abandoned` to `Abandoned`, any
other state to the problem "State %s inside the archive" (which moves from
`ValidateTree` to parse time; the validator keeps its own check for the
legacy `plans/goals/done/` prefix, where an abandoned record is always a
problem because the engine never writes there).

Two new helpers on `TreeGoals`:

```go
func (t *TreeGoals) Archived(id string) (*GoalFile, bool) // Done first, then Abandoned
func (t *TreeGoals) Exists(id string) bool                // Live, Done, or Abandoned
```

`doneLocation` gains a sibling `abandonedLocation`; `archivedPath` consults
both maps. `forAll` visits Live, Done, then Abandoned. `SortedGoalIds` is
unchanged.

## 4. The transition: `goal abandon`

Command surface (new, `metasystem/cmd/metasystem/goalsync_mutations.go`,
registered in the goal verb table at `main.go:440`):

```
metasystem goal abandon --root <checkout> --id <goal> --by <human> --because <text>
    [--carried <successor-goal>] [--waive <dependent>=<reason>]... [--also <dependent>]...
```

Authority: the same enrolled-terminal proof as `set-priority`
(`proveGoalHumanAuthority("abandon", shared, prove)` and `syncReqWithProof`).
No relayed temporary word, no channel answer, no agent path. On an
unconverted checkout (legacy `plans/goals.md`) the verb refuses with the
`set-priority` wording. The Go entry point is new `Abandon(r VerbRequest,
id string, spec AbandonSpec, proof *humanauthority.Proof)` with
`abandonRequest` as its transaction builder, following the `Park`/`parkRequest`
split so that recovery and reconcile have the one mutation to refer to.

Pre-publish refusals (in `Abandon`, before `Publish`):

1. `r.Actor.Human == ""` or `proof == nil`: "abandon is a human act at the enrolled terminal".
2. `because` blank: "abandon needs its reason; a goal that will never be worked owes the reader why".
3. A non-terminal job record on this checkout names the goal: refuse naming
   the job id; the remedy is `metasystem delegate --cancel <job>`. This read is
   the same job-record read `readClaimableBudgetedWork` uses and is
   checkout-local; the fleet-wide guarantee is the state itself (dispatch
   admission and landing both require a claimed goal, section 5, rows 12 and 13).

Transaction refusals (in `abandonRequest.Mutate`, in this order):

4. Not live: if `Done[id]` or `Abandoned[id]` exists, `AlreadyApplied` when the
   opid landed, else `LostToCompetitor{Winner: lastOpid}`; if nowhere, "goal
   %s is not live; nothing to abandon".
5. `--also` names a goal that is not live or is not a transitive live
   dependent of the abandoned set: "abandon takes one goal; --also names only
   its live dependents".
6. `--carried` names a goal that is not live, or names the abandoned goal or
   an `--also` goal: "carried must name a live successor".
7. Uncovered dependents. Compute the abandoned set A = {id} ∪ `--also`. For
   every live goal D with a `BlockedBy` edge into A, D is covered when it is
   itself in A, or when `--waive` names D, or when `--carried` is given.
   Any uncovered D refuses: "goal %s is blocked by %s; re-point it with
   --carried, waive it with --waive %s=<reason>, or abandon it with --also %s".
   The refusal lists every uncovered dependent.
8. `--waive` names a goal that is not a live dependent of A: refuse by name.

There is no refusal for open review obligations, for unfinished blockers of
the abandoned goal, for parked state, for a foreign claim, or for human
origin: the actor is the human.

Effects, in one transaction (all files rendered from the same tip):

- For each goal X in A, in sorted id order: record `displaced` when X is
  claimed by any pair (the human is never the claimant); set `State =
  abandoned`; set `Abandoned = {By: actor, At: stamp, Revision: X.Revision+1,
  Displaced, StopID: X.StopFence.StopID or "", Carried: --carried or "",
  Because}`; `Claimed = nil`, `Obligation = nil`, `Parked = nil`; keep
  `StopCapability` and `StopFence` when the fence is set, else
  `StopCapability = nil`; move `t.Live[X]` to `t.Abandoned[X]`; delete
  `livePath(X)`, write `donePath(X)` (the same `records/goals/<id>.md`
  location as done). `clearClaimBinding` and `bindClaim` are not called.
- Compact departed priorities with `compactDepartedPriorities(t.Live, A)` and
  land compaction lines on ranked survivors through `mergePriorityEvent` with
  verb `abandon` (the exact analogue of `done` at `verbs.go:1162-1176`); the
  abandoned goal's own line takes the compaction targets, as done's does.
- Dependents. For each live D with edges into A, in sorted id order: if D is
  in A, nothing (it is abandoned). Else if `--waive` names D: remove every
  edge from D into A, `touch(D, r, "abandon", [X...])`, `reason = "blocker <X>
  waived: <reason>"` (one line per D listing every removed edge). Else
  (`--carried`): rewrite each edge into A to the successor S, deduplicate
  `BlockedBy`, `touch(D, r, "abandon", [X...])`, `reason = "blockedBy <X>
  re-pointed to <S>"`. A re-point that creates a cycle is refused by
  `ValidateCommit`'s cycle check; the refusal text is the validator's and
  the verb adds nothing.
- The abandoned goal's history line carries `stopId=` when a fence was
  frozen and `carried=` when `--carried` was given, even when there were no
  dependents (the successor pointer is the reader's answer to "where did the
  work go").
- `ackDisplacements` runs as on every publication.
- The root record is untouched. Goal-free is not cleared (done does not
  clear it either). No arc retro debt is raised: the abandon reason is the
  record of why the arc stopped; `retrodebt.Raise` belongs to completion.
- Journal intent: `Intent{Verb: "abandon", Targets: [id], Args: {because,
  carried, waive: "<d>=<reason>;...", also: "<d>,..."}}` through
  `intentArgs`.

## 5. What each existing reader does with an abandoned goal

| # | Reader | Today | After the build |
| --- | --- | --- | --- |
| 1 | `depState` (`verbs.go:1184`) | done for any archived id | `Live` → `f.State`; `Done` → `f.State` (which the validator pins to done); `Abandoned` → `StateAbandoned`; else `""`. Every caller compares to `StateDone`, so abandoned never satisfies. |
| 2 | Validator `stateOf` and `exists` (`validate.go:214-227`) | same as 1 | `stateOf` mirrors the new `depState`; `exists` uses `Exists`. New rule (section 8, rule 5): a live goal blocked by an abandoned goal is a problem. |
| 3 | Prune (`verbs.go:1813`) | walks live blockers and newest keep done; deletes the rest of `Done` | Additionally seeds the walk from every `Abandoned` record's `BlockedBy`. Deletion still iterates `Done` only; abandoned records are never candidates and never counted against `keep`. The relayed-authority retention loop is unchanged. |
| 4 | Frontier `Next` (`project.go:543`) | `Live` only | Unchanged code. An abandoned goal is in none of Claimed, Ready, Blocked, Awaiting, Refused. A dependent re-pointed to a live successor is Blocked until the successor is done. |
| 5 | `goal next` and `SelectNext` | from 4 | Unchanged; never offers it. |
| 6 | Idle refusal (`ReadClaimableBudgetedWork`, `enforceIdleBacklog`, steward `classifySharedBacklog`) | from 4 and the live queued count | Unchanged code. Abandoning the only claimable goal ends the refusal; `Queued` never counts it. |
| 7 | Quota (`validate.go:298-331`) | claimed live goals per machine | Unchanged; an abandoned record has no `Claimed`. The new rule 3 in section 8 forbids one. |
| 8 | Listing `listSynced` | live table + `done: N archived` | The ranked table is `Live` only, unchanged. JSON gains `"abandoned": [...]` sorted by id. Pretty prints `abandoned: N retained with reasons` after the done line. The done count is `len(Done)` and does not move. |
| 9 | `goal show` | `Live` then `Done` | `Live`, `Done`, then `Abandoned`; prints the `Abandoned` line with its because. |
| 10 | Metrics (`metrics/compute.go`) | `concludedAt` needs State done; debt age skips done | The metrics world (`data.go:824`) includes `Abandoned` records in `w.Goals`. `concludedAt` is unchanged (State check excludes abandoned; `abandon` is not an archive verb). Debt age skips `StateAbandoned` exactly as it skips done and does not count it in `usable`. |
| 11 | Counselor (`sources.go:542`, `:578`) | history over Live and Done; `done` verb is the product class | Also walks `Abandoned` history (its open, edit, claim events happened). `goalVerbClass("abandon")` stays unmapped: excluded from the ratio and counted in the existing "Goal-event exclusions" limitation. The Done count cannot move. |
| 12 | Dispatch admission (`dispatch/admission.go:132`) | requires State claimed | Unchanged; refuses. |
| 13 | Landing observe (`landing/observe.go:748`) | requires State claimed | Unchanged; a straggler job cannot land on an abandoned goal. |
| 14 | Archived lookups: `gc.go:502`, `norm.go:87`, `slice.go:160`, `conformance.go:1206`, `verbs.go:1089`, `split.go:342`, `reconcilepub.go:150`, `report.go:73` | `Done[id]` | Use `Archived(id)` (or iterate both maps where the site iterates). An abandoned goal's approval, norm, slice and accepted-risk evidence stays readable; gc treats its job evidence as it treats a done goal's. |
| 15 | `open` (`verbs.go:556`), `split` member collision and arc-in-use (`split.go:401`, `:414`), split member blockers (`:425`) | `Done` | Collision and arc-in-use include `Abandoned`. A split member whose `BlockedBy` names an abandoned goal refuses with the rule-5 wording. |
| 16 | `done` on an archived id (`verbs.go:1118`) | AlreadyApplied or LostToCompetitor | Same for `Abandoned`. |
| 17 | `ackDisplacements` (`verbs.go:373`) | collects Live and Done | Also Abandoned. |
| 18 | Reconcile row on an archived id (`reconcilepub.go:239`) | conflict | Same for Abandoned. Reconcile never produces an abandoned record. |
| 19 | Hand edits (`reconcilemap.go`) | archive refused; unknown state change refused | Unchanged code, now with a named test: a live file hand-edited to `State: abandoned` hits the default refusal at `:282`; a hand-created archive file is refused at `:117`. |
| 20 | Recovery (`recover.go`) | proof-bearing verbs not replayed | New case: `"abandon"` returns the `set-priority` style refusal ("abandon is proof-bearing and cannot be replayed from journal text; re-run it from the enrolled terminal"). Confirmation of a landed abandon by opid works as for every verb. |
| 21 | Legacy single-file ledger (`goal.go`, `goalverbs.go`) | no such state | Out of scope; the verb refuses on an unconverted checkout. |
| 22 | Open-work scanners (`report/scan.go:127` drafts, plan pages) | drafts from `Live`; pages by fields | Drafts: an abandoned goal is not live, so it disappears. Pages: unchanged; see section 12. |

## 6. Reopen from abandoned

`reopenRequest` looks up `Done` then `Abandoned`. From `Abandoned`:

- Human act: `r.Actor.Human == ""` refuses with "goal %s was abandoned by
  %s; reopening it is a human act" (the same bar as lifting a human park at
  `verbs.go:1306`; no terminal proof beyond `--by`, matching unpark).
- The decomposed-parent guard applies. The claimed-dependent guard is moot
  (rule 5 forbids any live dependent) and is kept as is.
- Effects: `State = queued`; `Abandoned = nil`; `StopCapability = nil`;
  `StopFence = nil`; `Approved = nil`; `Budget = nil`; `NormApproval = nil`;
  `Conclude` is already empty; `Priority = 0`, `Sequence = 0` (rank not
  restored: the goal sorts last like a grandfathered one, and a human ranks
  it again with `set-priority`). The arc join rules at `verbs.go:1386-1401`
  apply unchanged. `touch(f, r, "reopen", [id])`; Goal-free clears as today.
- What stays: every History line, including the `breach-stop` line, the
  `abandon` line with its `reason=`, `stopId=` and `carried=`, and the
  `reopen` line. Nothing is rewritten. The stop is not erased: it is in the
  record forever, and a fresh claim after reopen mints fresh stop authority
  for a fresh revision through `bindClaim`, so the old fence has nothing to
  govern.
- Reopen does not run `VerifyStopBatchComplete`. The frozen fence bound a
  revision whose claim no longer exists; demanding the old batch would make
  the wedged specimen permanently un-reopenable, which is the dishonesty this
  goal removes. The place where a still-running job matters is section 4,
  refusal 3, at abandon time.

## 7. Prune

Retained = closure(every live goal's `BlockedBy` ∪ every abandoned goal's
`BlockedBy` ∪ the newest `keep` done goals by (OpenedAt, id)), walked through
`Done` edges as today. Abandoned records are outside the `keep` allowance and
outside the deletion loop; they are retained by construction, with their
reasons. An abandoned goal blocked by an abandoned goal needs no walk (both
are retained). A done prerequisite of an abandoned goal survives every prune,
which is the retention rule the goal asks for.

## 8. Validation at rest (`ValidateTree` and `ParseFile`)

1. Placement: a live file with State abandoned is "State abandoned outside
   the archive"; the legacy `done/` prefix with State abandoned is "abandoned
   record in the read-only legacy archive".
2. `abandoned without an Abandoned record`; `Abandoned record on a %s goal`
   for every other state.
3. `Claimed record on a abandoned goal` is already produced by `file.go:465`.
   `Parked record on a abandoned goal` by `:562`. Keep both.
4. `Abandoned.By` must start with `human:`; `At` RFC3339; `Revision` in
   `1..len(History)`, `History[Revision-1].Verb == "abandon"` and
   `.At == Abandoned.At` (the `ValidateApprovalRecord` binding pattern);
   `Because` non-blank ("abandoned without its because").
5. Dependency: for every live goal, every `BlockedBy` that resolves to an
   abandoned goal is "%s: blocked by abandoned goal %s; re-point, waive with a
   reason, or abandon it too". For done goals the existing rule at
   `validate.go:289` already fires because `stateOf` returns abandoned.
6. Stop authority: the rule at `file.go:468` becomes "stop authority on a %s
   goal" for every state other than claimed and abandoned. On an abandoned
   record: `StopCapability` present without `StopFence` is "frozen stop
   capability without its fence"; `StopFence` present requires
   `Abandoned.StopID == StopFence.StopID`; `Abandoned.StopID` non-empty
   requires a `StopFence`; the fence-versus-capability checks at `:547-557`
   run unchanged; the claim-binding cross-check at `:539` is skipped when
   `State == abandoned` (there is no claim to bind).
7. One id, one file: Live versus Done, Live versus Abandoned, Done versus
   Abandoned.
8. Referential integrity, label canon, budget-versus-norm and the cycle
   check visit abandoned records through `forAll` and `Exists`.
9. Goal-free exclusivity is unchanged (abandoned is not queued, approved or
   claimed).

## 9. Recovery, reconcile, and the command table

- `recover.go`: `case "abandon"` refuses replay (section 5, row 20).
- `reconcilemap.go`: no new mapping; the two existing refusals cover hand
  edits. Add the tests named in section 13.
- `main.go:440` table: `{"abandon", "human-only: record that a goal will never be worked and why; retained with its reason, satisfies no dependency, never pruned, reopenable", runGoalAbandon}`.
  The `reopen` help text gains "or an abandoned goal, as a human act".
- `metasystem help` output changes accordingly; no other command changes.

## 10. Rollout order under the closed grammar

The hazard: an older binary parsing a tree with an abandoned record fails at
`file.go:430` (unknown state) or `:1421` (unknown History key). The
read-side validator (`fetchadvance.go:65`) then refuses to advance the
accepted ref on that machine, every read there goes stale, and every publish
there rebuilds against a tip it cannot validate. That machine's goal work is
wedged until it upgrades. The ledger cannot prove which binary each machine
runs; there is no fleet binary registry in the root record. So the order is
procedural, with one mechanical assist:

1. Land the engine: parser, tree shape, validator, readers, verb, reopen,
   prune, recovery case, listing, docs, fixtures, all in one landing. The
   verb existing changes no bytes in any ledger.
2. Rebuild and re-arm every enrolled seat on the new engine (`metasystem up`
   refuses drift and re-arms a rebuilt engine from a commit reachable from
   the landing ref; the seat-boot recipe). Confirm each seat's `engineBuild`
   through `metasystem supervise status --repo <checkout>`.
3. Only then run the first `goal abandon` on the fleet ledger. The mechanical
   assist: before publishing, the verb prints one line, "abandon writes a
   state that engines older than <this build's landing commit> refuse; every
   enrolled seat must run this engine or newer", and continues. It does not
   refuse, because it cannot know.
4. The two specimens (section 12) are the first two uses and are Wido's.

There is no compatibility shim and no feature flag. Hand-editing a record to
`abandoned` on an old engine is refused by reconcile on every engine.

## 11. Doc amendment

`metasystem/docs/backlog-mechanism.md`, section "The drop rule", becomes two
cases:

> **The want evaporated.** The behavior or pain that justified the item is
> gone. Conclude it: `goal done --concluded "<what changed so this is no
> longer wanted>"`. Done means the ledger's promise is discharged, and this
> case discharges it honestly.
>
> **The want survives; the pursuit stops.** The item is still wanted but will
> not be worked: the record is wedged, the work moved to a successor, or the
> cost is no longer worth it. Abandon it: `goal abandon --id <goal> --by
> <human> --because "<why>" [--carried <successor>]`. An abandoned goal leaves
> the live set, carries its reason forever, satisfies no dependency, is never
> pruned, and a human can reopen it into the queue unranked and unapproved.
> Never conclude such a goal: a conclusion that says the work was not done
> corrupts every count of completed work. Never park it: parked means resume
> later.

The "Concluding a goal" section gains one sentence: "An abandoned goal writes
no journey chapter; its reason on the record is its story."

`metasystem/AGENTS.md` names no state list and needs no change.

## 12. The two specimens

**`account-provenance`.** Parked at 3:116 with the reason "do not claim or
revive", which is abandonment written in park's vocabulary. Its work is
carried by `account-provenance-carried` (queued, 2:3). No live goal is
blocked by it (checked: no `BlockedBy:` names it under `plans/goals/` or
`records/goals/`). It carries no fence today (Wido's forced landing cleared
it), so the fence half of this design is exercised by the fixture, not by
this specimen. After step 2 of section 10, at the enrolled terminal:

```
metasystem goal abandon --root . --id account-provenance --by Wido \
  --carried account-provenance-carried \
  --because "wedged by the stop-batch revision defect (records/misc/account-provenance-resume-is-wedged.md); the ask continues as account-provenance-carried"
```

Expected: file moves to `records/goals/account-provenance.md`, priority 3
compacts, `goal list` done count unchanged, `goal next` and the channel
status never name it, `stop-batch-strands-a-resumable-goal` (which keeps the
wedge evidence) is unaffected because it holds no edge to it.

**`metasystem/plans/metasystem-stop-design.md`.** A page, not a goal record.
The scanner reports it because line 6 reads `- Next step: critique once,
then build`. The state machine cannot silence it: no ledger state is read by
`openWork` (`openwork.go:435`). The page needs one of the two settled forms
the scanner already honors. The exact edit: line 6 becomes `- Next step:
none` (the value must be exactly a settled word; `settledStep` at
`openwork.go:50` rejects any trailing prose), and line 3 becomes `- Status:
superseded by metasystem-stop-verb-design.md on 2026-09-09; kept as the
record of the first design round`. The alternative, a non-empty `- Waiting on
the human:` line, would be false: nothing is waited on. This edit is outside
the engine build and is not part of its diff; the orchestrator lands it as a
page edit.

## 13. Fixtures

Every canary names where it fails on the untouched tree. Two facts bound what
a canary can reach today: the parser refuses the state word (`file.go:430`)
and the validator refuses any archived record that is not done
(`validate.go:161`), so no canary can carry an abandoned record through a
published tree on the untouched engine. The one reader reachable in
isolation is `depState`, through an in-memory tree; that canary fails at the
reader itself. The others fail at the boundary named, and after the build
they pin the reader's behavior.

Go tests, package `metasystem/internal/goal` unless noted; names are new.

| Canary | Untouched tree: fails at, because | After the build: proves |
| --- | --- | --- |
| `TestDepStateNeverSatisfiesFromAnArchivedRecordThatIsNotDone` (`verbs_test.go`) | Builds `TreeGoals` in memory with a `GoalFile{State: "abandoned"}` placed in `Done` (the only archive map that exists) and a live goal blocked by it; asserts `depState != StateDone`. Fails at `verbs.go:1188`: depState returns done for every archived id. Compiles before and after. | The reader never answers done from the State of an archived record. |
| `TestArchivedAbandonedRecordParsesIntoItsOwnMap` (`validate_test.go`) | `ParseTreeFiles` over a `records/goals/x.md` with `State: abandoned` and a well-formed `Abandoned:` line. Fails at `file.go:430`: unknown state. | Routed to `Abandoned`, absent from `Done`, no problems. |
| `TestValidateTreeRefusesALiveGoalBlockedByAnAbandonedGoal` (`validate_test.go`) | Same parse failure. | Rule 5's exact text; a done goal blocked by it fails rule at `validate.go:289`. |
| `TestAbandonRefusesUncoveredLiveDependents` (`verbs_test.go`) | `Abandon` undefined: compile failure, the verb does not exist. | Refusal 7 lists every uncovered dependent; nothing lands. |
| `TestAbandonCarriedRepointsEveryDependent` | Same. | Dependents' `BlockedBy` names the successor, each with an `abandon` line and the re-point reason; the abandoned record's line carries `carried=`; `Next` reports the dependent Blocked, not Ready, while the successor is open. This is the constraint-1 defect pinned at the frontier. |
| `TestAbandonWaiveRemovesTheEdgeWithARecordedReason` | Same. | The edge is gone, the dependent's line carries the waive reason, and `--waive` naming a non-dependent refuses. |
| `TestAbandonAlsoCascadesToNamedDependentsOnly` | Same. | Named dependents are abandoned in the same commit with the same because; an unnamed transitive dependent of an `--also` goal refuses under rule 7; `--also` naming a non-dependent refuses. |
| `TestAbandonToleratesItsOwnUnfinishedPrerequisites` | Same. | A goal blocked by a queued goal abandons; the queued prerequisite is untouched. |
| `TestAbandonRefusesWithoutAHumanOrAReason` | Same. | Refusals 1 and 2. |
| `TestAbandonCompactsTheDepartedPriorityLikeDone` | Same. | Survivors re-sequence with `abandon` compaction lines; the abandoned record keeps its historical rank. |
| `TestPruneRetainsAbandonedGoalsAndTheirPrerequisitesOutsideKeep` (`verbs_test.go`, beside `TestPruneKeepsTheClosureAndTheNewest`) | Same, and publication would fail validation. | With keep=0: an abandoned goal, its done prerequisite, and the done prerequisite of that prerequisite survive; an unrelated older done goal is dropped; the abandoned goal is not counted against keep. |
| `TestNextAndIdleRefusalIgnoreAnAbandonedGoal` (`turnverdict_idle_test.go`) | Same. | An approved unclaimed goal produces IDLE WITH BACKLOG; after abandoning it (the only claimable goal) the verdict is not an idle refusal and `Queued` is zero; `Next` returns it in no category. |
| `TestReopenFromAbandonedIsHumanUnrankedUnapprovedAndKeepsTheEvent` (`verbs_test.go`, beside `TestReopenGuardsClaimedDependents`) | Same. | Agent reopen refuses; human reopen lands queued with `Priority=0, Sequence=0, Approved=nil, Budget=nil, Abandoned=nil, StopFence=nil`; the `breach-stop`, `abandon` (with `stopId=`, `carried=`, `reason=`) and `reopen` lines are all present in order; a fresh approve and claim mint a new `StopCapability` for the new revision. |
| `TestHandEditToAbandonedHasNoReconcileGrammar` (`reconcilemap_test.go`) | The lenient parse rejects the state word, so the mapping fails earlier than `:282` with a parse error. | A live file edited to abandoned is refused at `:282`; a new archive file with State abandoned is refused at `:117`. |
| `TestRecoveryRefusesToReplayAbandon` (`recover_test.go`) | Journal entry with verb `abandon` reaches the default at `recover.go:413`, a different message. | The proof-bearing refusal text; a landed abandon is confirmed by opid. |
| `TestAbandonEventsNeverCountAsDone` (`metasystem/internal/counselor`) | A tree with an abandoned record cannot be read (parse). | The Done count over a window with one done and one abandon is exactly one; the abandon shows in the exclusions limitation. |
| `TestMetricsExcludeAbandonedFromConcludedAndDebt` (`metasystem/internal/metrics`) | Same. | The concluded set and the debt-age rows omit the abandoned goal; `usable` does not count it. |

The specimen, as a Go test and as a shell scenario:

`TestAbandonOfABreachStoppedClaimKeepsTheFenceFreesTheQuotaAndEnforcesTheDependencyRule`
(`stop_test.go`, seeded exactly like `TestBreachStopFenceAndHumanResumeAreOneWayTransactions`
at `stop_test.go:52`: open, approve, claim with a claim epoch, close the
stop through `CloseStopRequest`). Then: `release`, `done`, `park`, `set-budget`
and `steal` each refuse with the `clearClaimBinding` or fence wording (the
wedge, reproduced). A second approved goal `dependent` blocked by it exists.
`Abandon` without `--carried` refuses naming `dependent`. `Abandon --carried
successor` lands. Assertions on the landed tree: the record is at
`records/goals/<id>.md` with `State: abandoned`; `StopCapability` and
`StopFence` bytes are identical to the pre-abandon render; `Abandoned.StopID`
equals the fence's stop id; `Claimed` is absent; `ValidateTree` reports no
problems; the machine's claim count is zero, so a claim of a third approved
goal on the same machine lands (the quota freed); `dependent.BlockedBy ==
[successor]` and `Next` lists it Blocked; the dispatch admission read for the
abandoned goal refuses without a `BUDGET_` verdict. Untouched tree: compile
failure, the verb does not exist.

Shell scenario `abandoned-with-a-reason` in
`metasystem/scripts/agents/goal-cli-fixtures.sh`, added to the scenario list
at line 82 and the case at line 86, through the real binary in the two-clone
sandbox: open A and B with B blocked by A; approve both; claim A; run `job
breach-stop` (the verb at `main.go:141`) to land A's fence; assert `goal
release` refuses with "only goal resume may clear its launch fence"; assert
`goal abandon` without `--carried` refuses naming B; open successor S; `goal
abandon --id A --by Wido --because "fixture" --carried S` with the fixture
human authority the other human verbs use; assert `records/goals/A.md`
exists at the origin tip with `State: abandoned`, `Abandoned: by=human:Wido`,
`stopId=`, `carried=S` and a `StopFence:` line; assert `plans/goals/A.md` is
gone; assert `goal list --pretty` prints the same `done: N archived` count
as before the abandon and a new `abandoned: 1 retained with reasons` line;
assert `goal list` JSON has `abandoned` with A and `done` without A; assert
`goal next` names neither A nor B as ready; assert `goal show --id A` prints
the reason; `goal prune --keep 0` and assert A survives; `goal reopen --id A`
without `--by` refuses; with `--by Wido` lands A queued and unranked in
`plans/goals/A.md` with the abandon and reopen lines in History and no
`StopFence:` line. Untouched tree: the first `goal abandon` fails with the
unknown-verb exit, which is the named reason.

The required gate for the landing is unchanged:
`scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh`.

## 14. What the code cannot answer

- Whether every enrolled seat runs the new engine at the moment of the
  first abandon. There is no binary registry in the ledger. Section 10 makes
  the order procedural and prints the hazard.
- Whether a job on another machine is still running against the goal at
  abandon time. Job records are checkout-local. The design relies on the
  state making such a job inert (rows 12 and 13) rather than on seeing it.
- Whether the stop batch of a frozen fence ever completes. Abandon and reopen
  deliberately do not ask; the evidence stays on the record and in
  `records/misc/`.
- The counselor's ratio treats `abandon` as an excluded verb. If a later
  ruling wants abandonment counted as its own class, that is a counselor
  change, not a ledger change.

## 15. What must not change

Done keeps its meaning and its counts; `concludedAt`, `archiveVerbs` and the
`done` verb class are untouched. The frontier's categories gain nothing.
`clearClaimBinding`, `bindClaim`, `BreachStop`, `Resume` and
`VerifyStopBatchComplete` are untouched. No History line is rewritten or
removed on any path. The legacy archive prefix stays read-only. The record
grammar stays closed: the new state, field, keys and verb are the whole
extension.
