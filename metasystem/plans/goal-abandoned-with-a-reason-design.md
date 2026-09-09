# A goal that will never be worked, and why (goal goal-abandoned-with-a-reason)

Revision: 2. Date: 2026-09-09. Design authoring job: `gawr-design2`, folding
the round-1 read `metasystem/records/misc/goal-abandoned-with-a-reason-critique-r1.md`
of revision 1 (`gawr-design1`, landed 03f94dfc). The revision record at the
end names each finding and what moved.

This page specifies the build; it implements nothing. Independent critique,
dispositions, certification, the ledger and the receipt belong to the
orchestrator. All paths are relative to the repository root. Symbols marked
"new" do not exist in the tree yet. Line numbers carried over from revision 1
were read at `96c6098b`; every line number this revision adds (`observe.go`,
`land.sh`, `commit.sh`, `admission.go`, `budget.go`, `gc.go`, `lock.go`,
`goalrevision/lock.go`, `dispatch.sh`, `goalsync_mutations.go`, `split.go`,
`stop.go`, `order.go`, `root.go`, `file.go:1303-1421`, `disk.go`,
`go-build.sh`, `project.go`) was read at `cab73164`, and the revision-1
citations the brief asked me to check (`admission.go:132`, `observe.go:748`,
`goalsync_mutations.go:59`, `:117`, `:677`, `:1130`, `split.go:229`) hold at
that commit.

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
completed work, it is never pruned, and it comes back only through a proven
human reopen into queued, unranked and unapproved. The transition is a new
verb, `goal abandon`, a human act at the enrolled terminal. It does not go
through `clearClaimBinding`, so it reaches a breach-stopped goal: it removes
the claim (the execution authority) and freezes the stop fence on the record
as evidence. A job that was already running against the claim cannot land
its work above the abandonment: the landing binds the claim revision it
observed and the engine re-checks that binding at the rebased base before
every push. The ledger tree gains a third map, `Abandoned`, beside `Live`
and `Done`, so that every reader that today equates "in the archive map"
with "done" keeps meaning done, and every reader that needs abandoned
records opts in by name. The first abandon on a ledger refuses until a human
has recorded the fleet's engine floor on the root record, because the
machine cannot see the fleet.

## 1. Grounding in the current tree

| Responsibility | File and symbol | Observed behavior and consequence |
| --- | --- | --- |
| Closed state list | `metasystem/internal/goal/file.go:354` (`StateQueued` … `StateDone`), `:363` `validState`, `:430` the parse problem naming the five states | A sixth state is a grammar change. An older binary refuses any tree that carries it (section 10). |
| Record grammar | `file.go:699` `parseFileField` (unknown field refused at `:985`), `:1040` `parseKVRecord` ("the record grammar is closed" at `:1058`), `:976` the `Parked` case, `:1104` `splitParkTail` | New field lines and new keys must be added to these closed sets; free-text tails are byte-exact. `file.go:94` refuses a `Basis` that is blank or contains `\r` or `\n`: the one-line grammar has a precedent for refusing newlines in free text. |
| History grammar | `file.go:287` `HistoryLine`, `:1303` `ParseHistoryLine` (verb word taken as-is at `:1328`; unknown key refused at `:1421`), `RenderHistoryLine` | Optional keys are a closed set with duplicate guards; `reason=` consumes the rest of the line. The verb word is not validated, so a history line with a new verb and only known keys parses on every engine. |
| Root record grammar | `metasystem/internal/goal/root.go:236` `parseRootField` (unknown field refused at `:301-302`), `:139-143` `FormatVersion` must be `1` | A new root field or a format bump wedges every older engine exactly as a new state does. The root's `History` (`:96-108`) is the only part of the root record an older engine accepts new content in. |
| Tree shape | `metasystem/internal/goal/validate.go:27` `TreeGoals{Root, Live, Done, DonePaths}`, `:63` `ParseTreeFiles`, `:113` `parseDoneAt` | Placement is by path: everything under `records/goals/` (or the read-only legacy `plans/goals/done/`) lands in `Done`. There is no notion of an archived record that is not done. |
| Placement rule | `validate.go:153-167` | Live records may not be done; archived records must be done and carry a conclusion. |
| Dependency reader | `metasystem/internal/goal/verbs.go:1184` `depState` | Returns `StateDone` for every id in `Done`. `Next` (`project.go:573`), `doneRequest` (`verbs.go:1146`), and the validator's `stateOf` (`validate.go:219`) all compare against `StateDone`. |
| Referential integrity and cycles | `validate.go:214` `exists`, `:230` `forAll` loop, `:253` `findCycle` | Every edge must name a live or archived goal; cycles are found over the composed graph. |
| Done-under-blocker rule | `validate.go:289` | A done goal's blockers must be done. This is why no done goal can be blocked by a live goal, and so no done goal can be blocked by a goal that is later abandoned. |
| Claim clearing | `verbs.go:276` `clearClaimBinding` | Refuses while `StopFence` is set. `done` (`:1156`), `park` (`:1260`), `release` (`:895`) all call it. `set-budget` refuses at `:730`; `steal` at `:1736`; `resume` (`stop.go:403`) needs the exact stop batch complete. This is the wedge. |
| Claim binding | `verbs.go:261` `bindClaim` | Mints a fresh `StopCapability` for the new revision and sets `StopFence = nil` (`:269`). A queued record that carried a fence would have it silently overwritten by the next claim, which is why stop authority may never rest on a queued record (section 6). |
| Stop authority at rest | `file.go:468`, `:535-558` | Stop authority is allowed only on a claimed record; a fence needs its capability; the capability must match the claim binding. |
| Stop batch | `metasystem/internal/goal/stop.go:82` `stopBatchPath` (`artifacts/agents/goal-stops/<stopId>.json`, checkout-local), `:249` `VerifyStopBatchComplete` (binds goal, revision, fence epoch, capability generation, machine, claim epoch and reason; requires `COMPLETE`) | The only proof that the jobs of a stopped revision are gone. It is read only through `resume` (`:403`) today, and only on the checkout that holds the batch. |
| Done transition | `verbs.go:1106` `doneRequest` | Refuses open review obligations, foreign claims and parked goals for agents, human-origin goals for agents, and unfinished blockers; moves the file, compacts the departed priority, records displacement, acks displacements. |
| Reopen | `verbs.go:1337` `reopenRequest` | From `Done` only; refuses decomposed parents and claimed dependents; clears conclusion, approval, budget and norm; keeps priority and appends its sequence; arc join rules; clears Goal-free. Reached through `trySyncMutation`'s `case "reopen"` (`metasystem/cmd/metasystem/goalsync_mutations.go:495-500`) with a request built by `syncReq`, no proof. |
| Caller identity without proof | `goalsync_mutations.go:59` `syncReqClassified`: when a lineage is supplied by `--lineage` or `METASYSTEM_OWNER_LINEAGE` (`:71-74`) the enrolled-terminal proof branch (`:75-107`) is skipped, and `--by` becomes `Actor.Human` at `:117` | Any `r.Actor.Human != ""` guard is satisfied by an agent on a lineage-bearing checkout that passes `--by`. A human-only verb must take the proof, not the name. |
| Human proof | `goalsync_mutations.go:599` `proveGoalHumanAuthority`, `:677` its use by `set-priority`, `:682` `syncReqWithProof`; `metasystem/internal/goal/order.go:59-65` `SetPriority` refuses without `Actor.Human` and without `proof.ValidFor(root)` | The enrolled-terminal proof pattern for a human-only verb, on both sides of the package boundary. |
| Split readers | `goalsync_mutations.go:1128-1136` `runGoalSplit` (`Tree.Done[f.id]` decides "in the archive" versus "does not exist"); `metasystem/internal/goal/split.go:229-234` the split parent precheck (`t.Done[parentID]`, `AlreadyApplied` when the opid landed) | Two direct `Done` readers the revision-1 census missed (GAW-01). |
| Dispatch admission | `metasystem/internal/dispatch/admission.go:112-145` iterates `Tree.Live`, requires State claimed with a claim by this machine and lineage; `:180-198` `EvaluateGoalRevisionAdmission` refuses when the accepted claim revision moved | New work is admitted only against a live claim at the exact revision, and only on the claimant's machine. |
| Goal-revision lock | `metasystem/internal/goalrevision/lock.go:23-30` `Path` (`artifacts/agents/locks/goal-revisions/<id>-r<revision>.d`, checkout-local), `:125` `Acquire` (bounded, one second); held by dispatch from admission through the reservation write (`metasystem/scripts/agents/dispatch.sh:559-572`), by breach-stop (`dispatch/stop.go:130`), by resume (`goalsync_mutations.go:1076`) and by split on a claimed parent (`:1167`) | The lock every writer of a claim revision's reservations holds. Abandon takes it (section 4a). |
| Checkout lease lock | `metasystem/internal/lease/lock.go:31` `acquireBounded`, `:67` `LockBounded`; `lease.go:54-62` the path `artifacts/agents/mains/worktree-lease.lock`; `lease/verbs.go:464` `RunHeld` holds it around `commit.sh`'s `__lease-held` child (`metasystem/scripts/agents/commit.sh:31-41`) | The lock under which an agent commit's observation and `git commit` run. It serializes commits on one checkout; it says nothing about the ledger. |
| Landing observation | `metasystem/internal/landing/observe.go:742-753` `heldGoal`: reads `plans/goals/<id>.md` from the base tree, requires State claimed and the actor's machine and lineage, binds no revision. Base tree is `HeadTree()` at observation (`:254`), or the frozen target for a recertification (`:252`). Called at `:277` (chain), `:665` (register carriage), `:1175` (exact revert) and `tierone.go:34` (tier 1). Chain roots are read at `:159-167` as a map; `goalRevision` is not consulted. | The observation proves the goal was held at the checkout's HEAD when the commit was made. Nothing binds the revision, and nothing re-observes later. |
| Job record revision | `metasystem/internal/dispatch/claim.go:280-296` writes `goalRevision` on every goal-bound record; `dispatch/budget.go:344-350` refuses a goal-bound record without one | Every chain root that names a goal carries the claim revision it was dispatched under. |
| Landing sequence | `metasystem/scripts/agents/land.sh:495-596`: verify, stage, `commit.sh` (`:547`, observation inside), clean check, `fetch_origin` (`:570`), `rebase_origin` (`:571`), then up to three `push_origin` attempts (`:573-591`) each preceded on retry by fetch and rebase (`:588-589`); a non-fast-forward rejection is the retry trigger (`:412-414`) | The commit is rebased onto whatever origin holds and pushed with no second look at the goal. The push itself is a compare-and-swap: plain `git push` accepts only when origin's tip is the pushed commit's parent. |
| Trailers | `commit.sh:552-557` stamps `Machine`, `Landing-Provenance`, `Landing-Provenance-Verdict` and `Goal-Item`; `:559-578` re-proves the landed tree and exactly one `Goal-Item` | The commit message is where the observation's facts survive to push time. `metasystem/internal/channel/report.go:243` and `metasystem/internal/refusal/register.go:197` read `Goal-Item` by key; a new trailer key is invisible to them. |
| Budget projection | `dispatch/budget.go:242-254` projects only a claimed live goal, from `Claimed.Revision` and `Claimed.AccountingRevision`; `:313-361` reads every job record naming the goal, skips records below the accounting revision, refuses records above the claim revision | A goal that is not live is never projected; a record from a dead claim revision is skipped by a later claim. |
| Job evidence retention | `metasystem/internal/evidence/gc.go:482-512` `keepsSpendingFact`: a record of a goal that is neither live nor in `Done` is kept; a record of a done goal is not | Archived means "may age out"; unknown means "keep, the evidence may be needed". |
| Job return path | `dispatch` calls only `goal.MarkSliced` (`claim.go:402`), `goal.DeferFindings` (`finding_register.go:512`, refuses "goal %s is not live" at `verbs.go:963`) and `goal.CloseStop` (`stop.go:173`) | A delegate's return writes its job record and nothing on the goal. Landing is the separate act. |
| Live job visibility | `metasystem/internal/goal/project.go:414` `readLiveBacklogActivity` reads `artifacts/agents/jobs/*.json` of the checkout; `turnverdict.go:517` prints them as `non-terminal-jobs` | Job records are checkout-local. The human's checkout sees the claimant's jobs only when it is the claimant's checkout. |
| Supervision registry | `~/.metasystem/armed-checkouts.jsonl` (`metasystem/docs/design/supervision-registry.md`, REG-1: one file per user per machine; REG-2: records carry `event`, `checkoutPath`, `at`, `ownerTag`, `custodyId`); `metasystem/internal/registry/reduce.go:188` `Reduce`, `slots.go:52` `Slots` | Names the checkouts with live supervision claims on this machine under this user. Carries no engine build. |
| Engine build per checkout | `metasystem/internal/supervise/disk.go:110-122` `stateDocument.EngineBuild` = `BuildStamp`, set by `metasystem/scripts/agents/go-build.sh:74` to the commit built from (`dev` when unstamped); printed by `metasystem supervise status --repo <checkout>` as `engineBuild` | The one machine-readable fact about which engine a seat runs, per armed checkout, on this machine. |
| Counts | `metasystem/internal/metrics/compute.go:148` `archiveVerbs`, `:156` `concludedAt`, `:197`, `:624` debt age; `metasystem/internal/counselor/sources.go:578` `goalVerbClass`, `:542` history over `Done`, `:537` root history | Concluded means State done with a done or split line; the counselor's product count is the `done` verb class; an unmapped verb is excluded. |
| Archived lookups | `metasystem/internal/evidence/gc.go:502`, `metasystem/internal/goal/norm.go:87`, `metasystem/internal/dispatch/slice.go:160`, `metasystem/internal/validate/conformance.go:1206`, `verbs.go:1089`, `split.go:342`, `reconcilepub.go:150`, `metasystem/internal/channel/report.go:73` | Each reads `Done[id]` as "the archived record of id", not as "completed". |
| Recovery replay | `metasystem/internal/goal/recover.go:309-313`, `:413` | Proof-bearing verbs are not replayed from journal text. |
| Hand edits | `metasystem/internal/goal/reconcilemap.go:117`, `:264-283` | The archive has no hand-edit grammar; a state change outside the pinned cases refuses. |
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

What the shape guarantees, and what it does not. Because `Done` keeps its
name and its contents, no reader that was not touched by this build can
answer "done" for an abandoned goal: the record is not in the map it reads.
That is the whole guarantee. A reader that was forgotten by the census
answers "absent" instead: a `Done[id]` lookup returns nil for an abandoned id,
and the reader behaves as it does for a goal that never existed. Revision 1
called this failing closed; it is not, because "absent" is a wrong answer
too (a split precheck says "does not exist" for a goal that is in the
archive; an evidence retention rule keeps records it should let age). The
residual risk of the third map is therefore a forgotten reader that treats
an abandoned goal as nonexistent, never one that treats it as completed. The
census in section 5 is the control, and the implementer's own
`rg -n --glob '*.go' --glob '!**/*_test.go' '\.(Done|DonePaths)\b' metasystem`
is the check that the census is complete on the tree being built.

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

Pre-publish refusals (in `Abandon`, before `Publish`, in this order):

0. The fleet engine floor (section 10): no `engine-floor` line on the root
   record, a floor this engine's own build does not descend from, or an
   armed checkout on this machine below the floor. The refusal text is
   section 10's.
1. `r.Actor.Human == ""`: "abandon is a human act and names its human
   (--by)". `proof == nil || !proof.ValidFor(r.Endpoint.Root)`: "abandon
   requires freshly observed enrolled-terminal human authority". These are
   `SetPriority`'s two checks (`order.go:60-65`) with the verb renamed.
2. `because` blank after trimming, or containing `\r` or `\n`: "abandon
   needs its reason on one line; a goal that will never be worked owes the
   reader why". The newline rule is `file.go:94`'s, because `because=` is a
   line tail.
3. `--waive` grammar, checked before any tree is read: every entry must be
   `<id>=<reason>` with a non-empty id that is a valid goal id and a reason
   that is non-blank after trimming and contains no `\r` or `\n`; an entry
   without `=` or with a blank reason refuses "waive names its dependent and
   its reason: --waive <dependent>=<reason>"; the same dependent named more
   than once across `--waive` entries refuses "dependent %s is waived twice;
   name it once" (this covers the conflicting-reasons case and the
   duplicate-with-equal-reasons case with one rule); a dependent named in
   both `--waive` and `--also` refuses "dependent %s is both waived and
   abandoned; choose one". The refusal lists every offending entry.
4. Under the goal-revision lock (section 4a): a non-terminal job record on
   this checkout names the goal or any `--also` goal: refuse naming the job
   ids; the remedy is `metasystem delegate --cancel <job>`. This read is
   `readLiveBacklogActivity`'s read of `artifacts/agents/jobs/*.json`
   (`project.go:414`) and is checkout-local; section 4a says what holds
   across checkouts.

Transaction refusals (in `abandonRequest.Mutate`, in this order):

5. Not live: if `Done[id]` or `Abandoned[id]` exists, `AlreadyApplied` when the
   opid landed, else `LostToCompetitor{Winner: lastOpid}`; if nowhere, "goal
   %s is not live; nothing to abandon".
6. Claim moved: for every goal in the abandoned set that the pre-publish
   projection saw claimed at revision `n`, the tip's record must still be
   claimed at revision `n` (or unclaimed); a different claim revision refuses
   "goal %s's claim moved from revision %d to %d under abandon's lock;
   retry". This is the compare half of the lock-then-compare in section 4a.
7. `--also` names a goal that is not live or is not a transitive live
   dependent of the abandoned set: "abandon takes one goal; --also names only
   its live dependents".
8. `--carried` names a goal that is not live, or names the abandoned goal or
   an `--also` goal: "carried must name a live successor".
9. Uncovered dependents. Compute the abandoned set A = {id} ∪ `--also`. For
   every live goal D with a `BlockedBy` edge into A, D is covered when it is
   itself in A, or when `--waive` names D, or when `--carried` is given.
   Any uncovered D refuses: "goal %s is blocked by %s; re-point it with
   --carried, waive it with --waive %s=<reason>, or abandon it with --also %s".
   The refusal lists every uncovered dependent.
10. `--waive` names a goal that is not a live dependent of A: refuse by name.

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

## 4a. The running job, the landing, and the atomic transition

Revision 1 claimed that dispatch admission and landing both requiring a
claimed goal make a running job inert once the goal is abandoned. Admission
does (`admission.go:112-145`, `:196-198`). Landing does not: `heldGoal`
(`observe.go:742-753`) reads the goal at the checkout's HEAD when the commit
is made, binds no revision, and `land.sh` then fetches, rebases and pushes
(`:570-591`) without looking again. A job dispatched while the goal was
claimed can therefore finish after the abandonment, commit with a passing
observation against its stale HEAD, rebase cleanly over the abandonment (the
goal file moved from `plans/goals/` to `records/goals/`, and the job did not
touch either), and push. This section closes that with two mechanisms and
names the lock for the transition itself.

**Mechanism 1: the observation binds the claim revision.** `heldGoal` gains
a `want uint64` parameter and a fifth check: when `want != 0` and
`file.Claimed.Revision != want`, it refuses with a new carriage code
`goal-revision-moved` ("goal item %s is claimed at revision %d; this landing
was dispatched under revision %d"). The four call sites pass:

- chain (`observe.go:277`): the chain root's `goalRevision`, read from the
  record map at `:164-167`; a goal-bound root without a positive
  `goalRevision` is `chain-record-malformed` (the budget projection already
  refuses such a record at `budget.go:344-347`, so this adds no new
  invariant);
- tier 1 (`tierone.go:34`): the root job's `goalRevision`, same rule;
- register carriage (`:665`) and exact revert (`:1175`): `0`, which binds
  whatever revision the base tree holds. The register-carriage call from a
  chain (`:289` after `:277`) reads the same tree twice; passing `0` the
  second time is correct because the first call already bound the chain's
  revision.

`Observation` gains a field `GoalRevision uint64` with the JSON key
`goalRevision`, omitted when zero, set from `file.Claimed.Revision` whenever
a goal was checked and the check passed. The provenance string does not change. `commit.sh` reads the new
field beside the others at `:463-477` and, when `--goal` was given and the
field is positive, adds `--trailer "Goal-Revision: <n>"` to the trailer list
at `:552-557`. The postcondition at `:561-578` gains the same shape it has
for `Goal-Item`: with `--goal` and a passing observation, exactly one
`Goal-Revision:` line whose value is the observed one; without `--goal`,
none; otherwise the soft rollback with the existing sentence extended to
name the trailer. The new code needs a case in the refusal table at
`:481-531`: "agent commit refused: the Goal-Item's claim revision moved since
this chain was dispatched ($landing_verdict)", repair "the work belongs to a
claim that no longer exists; re-dispatch under the current claim, or abandon
the work". Human commits keep their sovereignty exactly as at `:434-437`:
the trailer is stamped when the observation passed, and the refusal bit is
never consumed.

**Mechanism 2: the engine re-checks the binding at the rebased base.** New
verb `metasystem landing held --root <root> --commit <commit>`, implemented
by `landing.Held(root, commit string) (HeldVerdict, error)` in a new
`metasystem/internal/landing/held.go`. It reads the commit's `Goal-Item`,
`Goal-Revision` and `Machine` trailers, resolves the commit's first parent
and the workspace subtree of the parent's tree (the same subtree resolution
`staged_candidate_tree` performs at `land.sh:351-359`), and decides:

- no `Goal-Item`: prints `held: no goal binding` and exits 0;
- `Goal-Item` without `Goal-Revision`: the commit was made by an engine that
  did not bind (an older build, or a human commit whose observation
  refused); for a `Machine` trailer whose lineage part is `human` it prints
  `held: unbound goal item <id>` and exits 0; for any other actor it refuses
  `goal-revision-unbound` and exits 1, and the seat re-commits the same tree
  through `commit.sh` so the trailer is stamped;
- both present: `goal.ParseFile` of `plans/goals/<id>.md` at the parent's
  subtree must produce no problems, State claimed, a claim whose
  `Machine+"+"+Lineage` equals the `Machine` trailer, and
  `Claimed.Revision` equal to `Goal-Revision`. A missing file or any other
  state refuses `goal-item-not-held` ("goal %s is %s at %s", the state or
  `absent`, and the parent's short id); a different claim revision refuses
  `goal-revision-moved`. Exit 1 with `held refused: <code>: <detail>` on
  stderr, except that for a `+human` actor the same line is printed as
  `held: warning <code>: <detail>` and the exit is 0, because a human
  commit stays sovereign;
- a commit with no parent or more than one parent, or an unreadable
  message, exits 2 with `held: unreadable`.

`land.sh` calls it as `run_required_step "goal held at the rebased base"
held_check` where `held_check() { "$ms" landing held --root "$root" --commit
HEAD; }`, immediately before every `push_origin`: after `rebase_origin` at
`:571`, after the retry rebase at `:589`, and in the recertified branch
before `:560` (there the parent is already frozen to the target commit by
`verify_recertified_commit`, so the step passes or names a real change). A
refusal leaves the commit made and unpushed, which is the state every
land.sh repair route starts from.

**Why the two together close it.** The push is the compare-and-swap. Plain
`git push` (`land.sh:408-410`) accepts only when origin's tip is an ancestor
of the pushed commit, and a rebased landing is exactly one commit above the
tip it was rebased onto, so the push succeeds only when origin's tip is the
commit's parent. `held` checked the goal in that parent. If the abandonment
lands on origin between the check and the push, the push is rejected as
non-fast-forward (`:412-414`), land.sh fetches, rebases onto the abandonment
and runs `held` again, which now finds the goal absent from `plans/goals/`
and refuses. The window between observation and push is closed by the pair
"check the parent, push only above that parent". A force push is outside
every rule on this page. On a branch that is not the ledger's branch the
check reads that branch's copy of the ledger, exactly as the observation
does today; this design does not change what the landing treats as the
ledger.

**The transition is atomic against this checkout's dispatch.** `Abandon`,
before pre-publish refusal 4, takes the goal-revision lock
(`goalrevision.Acquire(root, X, X.Claimed.Revision, "goal-abandon")`) for
every goal in the abandoned set that the pre-publish projection shows
claimed, in sorted id order, and holds every one of them until `Publish`
returns. That is the lock dispatch holds from admission through the
reservation write (`dispatch.sh:559-572`), breach-stop holds
(`dispatch/stop.go:130`), resume holds (`goalsync_mutations.go:1076`) and
split holds for a claimed parent (`:1167`), so on this checkout no
reservation against the locked revision can be written between refusal 4's
read and the publication. A busy lock refuses with `goalrevision.BusyError`'s
`LOCK_BUSY` line (`lock.go:159-167`) and the human retries. The compare half
is refusal 6: the transaction re-reads the tip and refuses when the claim
revision it locked is not the one it finds. `Publish` itself is the ledger's
compare-and-swap (`txn.go:347-365`, an explicit expected old value on the
push), so two abandons, or an abandon and a claim, cannot both land on one
tip. The checkout lease lock (`lease/lock.go`) is deliberately not taken:
it serializes commits on a checkout and would add nothing to a ledger
transaction that is already a compare-and-swap.

**Across checkouts, the state is the guarantee.** Job records and the
goal-revision lock are checkout-local. When the human's `--root` is not the
claimant's checkout, refusal 4 sees nothing and the lock guards nothing. A
dispatch on the claimant's checkout that admitted against an accepted ref
which had not yet fetched the abandonment can launch. What it cannot do is
land: its chain root carries the dead claim revision, and `held` refuses at
the parent that carries the abandonment. Section 12's specimen therefore
runs `goal abandon` with `--root` at the claimant's checkout, where refusal 4
and the lock are real; section 14 records the limit.

**What the running job's return means.** The delegate finishes, the adapter
writes `return.json` and the job record goes terminal, exactly as today; the
return path calls no goal verb (the only goal calls in `dispatch` are
`MarkSliced` at slice start, `DeferFindings` at register close, and
`CloseStop` at breach stop). The terminal record still names the abandoned
goal and the dead revision in `goalId` and `goalRevision`. Its terminal
status means the delegate returned, not that anything landed; the landing is
the act that is refused. `job critique-register-close` on such a chain
refuses at `DeferFindings` with "goal %s is not live" (`verbs.go:963`), which
is right: there is no live goal to owe the obligation to. While it runs, the
job stays visible in the checkout's `non-terminal-jobs` line
(`turnverdict.go:517`), which is where a steward or the human sees a
straggler to cancel.

**What its budget accounting does.** Nothing charges it after the act. The
projection is reached only for claimed live goals (`admission.go:112-145`,
`budget.go:247-254`), so an abandoned goal is never projected; the record's
reserved minutes and attempt belong to a claim that no longer exists, which
is what happens today when `done` or `park` drops a claim. If the goal is
reopened and claimed again, the new claim's revision is greater than every
old record's `goalRevision` (the revision is the history length, and the
abandon and reopen lines sit between), `newClaimRecord` (`verbs.go:257-259`)
starts the accounting revision at the new claim revision, and `budget.go:359`
skips the old records. The refusal at `:348-350` needs a record revision
above the claim's, which no old record has, so no `BUDGET_UNKNOWN` can come
from them. Retention: `keepsSpendingFact` at `gc.go:500-503` becomes
`if file == nil { _, archived := s.tree.Archived(goalID); return !archived }`,
so the records of an abandoned goal age out on the same terms as a done
goal's, and the records of a pruned goal are still kept.

**Where this meets the parked landing chain.** Goal
`landing-receipt-survives-records-drift` (parked, chain fully proven) replaces
`rebase_origin` at `land.sh:404-406` with `metasystem landing advance`
(its design, section 3b and 3e) and holds the checkout lease lock inside
that verb. This design changes neither `rebase_origin` nor the lock. It
inserts one step, `held_check`, before each push, after whichever of
`rebase_origin` or `landing advance` produced the rebased commit; `held`
reads only the commit and its parent, so it is indifferent to how the rebase
was done. The two chains touch adjacent lines of `land.sh` (`:571`, `:589`);
whichever lands second re-bases its `land.sh` hunk, and no other file is
shared between them.

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
| 11 | Counselor (`sources.go:542`, `:578`) | history over Live and Done; `done` verb is the product class | Also walks `Abandoned` history (its open, edit, claim events happened). `goalVerbClass("abandon")`, `("engine-floor")` stay unmapped: excluded from the ratio and counted in the existing "Goal-event exclusions" limitation. The Done count cannot move. |
| 12 | Dispatch admission (`dispatch/admission.go:112-145`) | requires State claimed in `Live` | Unchanged; refuses. |
| 13 | Landing observe (`landing/observe.go:742`) and `land.sh` | requires State claimed at observation; binds no revision; no second look | Section 4a: the observation binds the claim revision (`goal-revision-moved`), `commit.sh` stamps `Goal-Revision`, and `landing held` re-checks at the rebased base before every push. A straggler job cannot land above an abandonment. |
| 14 | Archived lookups: `gc.go:502`, `norm.go:87`, `slice.go:160`, `conformance.go:1206`, `verbs.go:1089`, `split.go:342`, `reconcilepub.go:150`, `report.go:73` | `Done[id]` | Use `Archived(id)` (or iterate both maps where the site iterates). An abandoned goal's approval, norm, slice and accepted-risk evidence stays readable; gc treats its job evidence as it treats a done goal's (section 4a gives the exact edit). |
| 15 | `open` (`verbs.go:556`), `split` member collision and arc-in-use (`split.go:401`, `:414`), split member blockers (`:425`) | `Done` | Collision and arc-in-use include `Abandoned`. A split member whose `BlockedBy` names an abandoned goal refuses with the rule-5 wording. |
| 16 | `done` on an archived id (`verbs.go:1118`) | AlreadyApplied or LostToCompetitor | Same for `Abandoned`. |
| 17 | `ackDisplacements` (`verbs.go:373`) | collects Live and Done | Also Abandoned. |
| 18 | Reconcile row on an archived id (`reconcilepub.go:239`) | conflict | Same for Abandoned. Reconcile never produces an abandoned record. |
| 19 | Hand edits (`reconcilemap.go`) | archive refused; unknown state change refused | Unchanged code, now with a named test: a live file hand-edited to `State: abandoned` hits the default refusal at `:282`; a hand-created archive file is refused at `:117`. |
| 20 | Recovery (`recover.go`) | proof-bearing verbs not replayed | New cases: `"abandon"` and `"engine-floor"` return the `set-priority` style refusal ("%s is proof-bearing and cannot be replayed from journal text; re-run it from the enrolled terminal"); `"reopen"` refuses the same way when the intent carries `from=abandoned` (section 6) and replays as today otherwise. Confirmation of a landed act by opid works as for every verb. |
| 21 | Legacy single-file ledger (`goal.go`, `goalverbs.go`) | no such state | Out of scope; the verb refuses on an unconverted checkout. |
| 22 | Open-work scanners (`report/scan.go:127` drafts, plan pages) | drafts from `Live`; pages by fields | Drafts: an abandoned goal is not live, so it disappears. Pages: unchanged; see section 12. |
| 23 | `runGoalSplit` precheck (`goalsync_mutations.go:1128-1136`) | `Tree.Done[id]` decides "in the archive; nothing to split" versus "does not exist" | `Tree.Archived(id)`: an abandoned parent prints "goal %s is in the archive; there is nothing to split" (the same sentence; the archive is the archive). Without the change it prints "does not exist", the absent-not-completed degradation section 3 names. |
| 24 | Split parent precheck (`split.go:229-234`) | `t.Done[parentID]`: `AlreadyApplied` when the opid landed, else "in the archive" | `t.Archived(parentID)` with the same two outcomes. Same degradation without it. |

## 6. Reopen from abandoned

Two facts decide this section. First, `syncReqClassified` turns a caller's
`--by` into `Actor.Human` without proof whenever a lineage is already at
hand (`goalsync_mutations.go:71-77`, `:117`), so revision 1's guard
`r.Actor.Human != ""` admitted any agent on a lineage-bearing checkout.
Second, a frozen fence is evidence that a revision was stopped and its jobs
were never proven gone; a history line that remembers the stop id is
provenance, not authority, and clearing the fence on its strength would let
a reopen do what `clearClaimBinding` (`verbs.go:276-279`) refuses to do for
every other verb.

**Command.** `trySyncMutation`'s `case "reopen"` (`goalsync_mutations.go:495`)
projects the tree first (`goal.Project(req.Endpoint, false, req.Now)`, as
`runGoalSplit` does at `:1123`). When the id is in `Tree.Abandoned` it
discards the proof-less request, requires `--by` (exit 2: "goal reopen from
abandoned needs --by at the enrolled terminal"), proves with
`proveGoalHumanAuthority("reopen", f, proveEnrolledGoalHumanAuthority)`,
rebuilds the request with `syncReqWithProof("reopen", ...)` (the `set-priority`
sequence at `:676-686`, so the brain checkout's refusal of a carried word
applies), and calls new `goal.ReopenAbandoned(req, id, &proof)`. Otherwise it
calls `goal.Reopen(req, id)` unchanged: reopen from done keeps today's
authority. The `reopen` help text gains "or an abandoned goal, as a proven
human act".

**Goal package.** `ReopenAbandoned(r VerbRequest, id string, proof
*humanauthority.Proof)` refuses before `Publish` exactly as `SetPriority`
does (`order.go:60-65`): `r.Actor.Human == ""` → "reopen from abandoned is a
human act and names its human (--by)"; `proof == nil ||
!proof.ValidFor(r.Endpoint.Root)` → "reopen from abandoned requires freshly
observed enrolled-terminal human authority". Its transaction builder
`reopenAbandonedRequest` uses the history verb `reopen` and the journal
intent `Intent{Verb: "reopen", Targets: [id], Args: {from: "abandoned"}}`;
`reopenRequest` for done goals is untouched. In the mutation, in order:

- `Live[id]` exists: `AlreadyApplied` when the opid landed, else
  `LostToCompetitor`. Not in `Abandoned`: "goal %s is not abandoned; reopen
  --id names a done goal or an abandoned one".
- The decomposed-parent guard and the claimed-dependent guard from
  `reopenRequest` (`verbs.go:1357-1373`) apply unchanged; the second is moot
  under rule 5 and is kept.
- The fence. When `f.StopFence != nil`, call
  `VerifyStopBatchComplete(r.Endpoint.Root, id, *f.StopCapability, *f.StopFence)`
  with the frozen bytes, exactly as `resume` does at `stop.go:402-405`. Any
  error refuses: "goal %s stays abandoned: %v; advance stop batch %s on the
  checkout that holds it (metasystem job stop-batch ...), or carry the work
  in a successor goal". The batch is checkout-local
  (`artifacts/agents/goal-stops/<stopId>.json`, `stop.go:82-86`), so a
  fenced reopen runs where resume would have run, on the claimant's
  checkout; anywhere else it refuses "cannot prove stop batch %s complete".
  A record without a fence verifies nothing.
- Effects: `State = queued`; `Abandoned = nil`; `StopCapability = nil`;
  `StopFence = nil`; `Approved = nil`; `Budget = nil`; `NormApproval = nil`;
  `Conclude` is already empty; `Priority = 0`, `Sequence = 0` (rank not
  restored: the goal sorts last like a grandfathered one, and a human ranks
  it again with `set-priority`). The arc join rules at `verbs.go:1386-1401`
  apply unchanged. `touch(f, r, "reopen", [id])`; the file moves from
  `t.Abandoned` to `t.Live` and from `archivedPath` to `livePath`; Goal-free
  clears as today.
- What stays: every History line, including the `breach-stop` line, the
  `abandon` line with its `reason=`, `stopId=` and `carried=`, and the
  `reopen` line. Nothing is rewritten. A fresh claim after reopen mints
  fresh stop authority for a fresh revision through `bindClaim`.

**Why the batch proof, and not a fence kept frozen on the queued record.**
The brief's other option keeps `StopFence` on the reopened queued record
until a resume-like act clears it, and extends rule 6 to allow it. It is
wrong for four reasons. It puts stop authority on a queued record, and
`bindClaim` (`verbs.go:269`) overwrites a fence unconditionally on the next
claim, so either `bindClaim` changes (a function section 15 pins) or the
fence is silently lost at the first claim, which is the provenance-clears-
authority defect in a new place. The frontier would list the goal Ready
while its claim refuses, which is the kind of lie this goal exists to
remove. It needs a new clearing act that is `resume` without a claim, a
third path into `VerifyStopBatchComplete`. And it proves nothing the first
option does not: in both, the only quiescence proof is the batch. Requiring
the batch at reopen is the same bar `resume` sets, taken at the same place,
with the same verifier.

**What this costs, said plainly.** A goal whose batch can never complete
(the wedge `stop-batch-strands-a-resumable-goal` records) can be abandoned
but not reopened. That is honest: its stopped jobs were never proven gone,
and the answer to that wedge is the successor goal `--carried` names, not a
reopen that pretends. Section 4a means such a job can no longer land, so
what the batch proves at reopen is that nothing is still spending against
the dead revision, not that the ledger is at risk. The specimen in section
12 carries no fence today, so its reopen would verify nothing.

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
   goal" for every state other than claimed and abandoned; a queued record
   never carries it, and section 6 clears it only behind the batch proof.
   On an abandoned record: `StopCapability` present without `StopFence` is
   "frozen stop capability without its fence"; `StopFence` present requires
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
10. Root `engine-floor` lines (section 10): `actor=` starts with `human:` and
    the first whitespace-separated token of `reason=` is a 40-character
    lowercase hex commit id; otherwise "engine-floor line without a commit".
    Older engines do not know this rule and accept the line as history.

## 9. Recovery, reconcile, and the command table

- `recover.go`: `case "abandon"` and `case "engine-floor"` refuse replay;
  `case "reopen"` refuses when `Args["from"] == "abandoned"` (section 5, row
  20).
- `reconcilemap.go`: no new mapping; the two existing refusals cover hand
  edits. Add the tests named in section 13.
- `main.go:440` table: `{"abandon", "human-only: record that a goal will never be worked and why; retained with its reason, satisfies no dependency, never pruned, reopenable", runGoalAbandon}`
  and `{"engine-floor", "human-only: record the oldest engine commit every enrolled seat runs; the first abandon refuses without it", runGoalEngineFloor}`.
  The `reopen` help text gains "or an abandoned goal, as a proven human act".
- The landing verb table gains `{"held", "re-check a landing commit's Goal-Item claim binding at its parent; land.sh runs it before every push", runLandingHeld}`.
- `metasystem help` output changes accordingly; no other command changes.

## 10. Rollout under the closed grammar

The hazard: an older binary parsing a tree with an abandoned record fails at
`file.go:430` (unknown state) or `:1421` (unknown History key). The
read-side validator (`fetchadvance.go:65`) then refuses to advance the
accepted ref on that machine, every read there goes stale, and every publish
there rebuilds against a tip it cannot validate. That machine's goal work is
wedged until it upgrades. Revision 1 printed a warning and continued; the
first abandon now refuses until a precondition holds. Two candidates were
weighed. Here is what each can prove.

**The machine-wide registry.** `~/.metasystem/armed-checkouts.jsonl` names
every checkout with a live supervision claim for this user on this machine
(REG-1, REG-2). It carries no engine build; that lives in each armed
checkout's supervision state (`disk.go:110-122`, `EngineBuild`), which is
the commit `go-build.sh:74` stamped, or `dev` for an unstamped build. Joined,
they prove: every armed checkout of this user on this machine runs an engine
built from a known commit. They cannot see another machine, another user on
this machine, a checkout that is not armed (a human's terminal checkout
reading the ledger with an old `bin/metasystem`), or an old binary that will
be armed later (`metasystem up` refuses engine drift against the landing
ref, which is a separate guard). There is no fleet binary registry in the
ledger, and the root record cannot gain one without the very grammar change
this section guards against (`root.go:301-302` refuses an unknown root field;
`:139-143` pins `FormatVersion` to `1`).

**The recorded ruling.** A human, after rebuilding and re-arming every seat
and reading `engineBuild` from `metasystem supervise status --repo <checkout>`
on every machine, records the floor on the ledger. The machine cannot check
that the human looked everywhere; it can check that the ruling exists, that
the engine executing the abandon is at or above it, and that nothing this
machine can see contradicts it.

**Decision: the ruling gates, and the local registry may only refuse.** The
verb `goal abandon` refuses (pre-publish refusal 0) unless all three hold:

1. The root record's History carries an `engine-floor` line. The newest one
   is the floor. The line is minted by a new human verb
   `metasystem goal engine-floor --root <checkout> --commit <sha> --by <human>`
   with `set-priority`'s proof, and is written as
   `- <at> <opid> engine-floor actor=human:<name> reason=<sha> every enrolled seat runs this engine or newer`
   with `Root.Revision++`, through `Publish`. It is a History line and not a
   root field because a root field wedges every older engine
   (`root.go:301-302`), while a History line with a new verb word and only
   known keys parses on every engine (`file.go:1328`, `:1421`); the readers
   of root history (`counselor/sources.go:537`, `approval.go:713`,
   `verbs.go:337`) ignore a verb they do not map. The refusal without the
   line: "abandon writes a state that engines older than this build refuse,
   and the ledger has no record that the fleet runs it; after every enrolled
   seat has rebuilt and re-armed (metasystem supervise status --repo
   <checkout> on each machine), a human records the floor: metasystem goal
   engine-floor --root . --commit <this engine's build commit> --by <name>".
   The verb prints this engine's `BuildStamp` in that sentence.
2. This engine's own `BuildStamp` is the floor or descends from it
   (`git merge-base --is-ancestor <floor> <BuildStamp>` in the checkout);
   a `dev` stamp or an unknown commit refuses "this engine's build (%s)
   cannot be placed against the fleet floor %s; build it with
   scripts/agents/go-build.sh". An engine below its own fleet's floor is
   the wrong engine to write with.
3. Every checkout with a live claim in the registry (`registry.ReadFrames`,
   `Reduce`, `Slots`, the live ones) has a supervision state whose
   `EngineBuild` is the floor or descends from it. New
   `supervise.EngineFloorProblems(floor string, isAncestor func(a, b string)
   (bool, error)) ([]string, error)` performs the join with the same
   checkout-to-state resolution `supervise status --repo` uses and returns
   one line per offending checkout; any line refuses "checkout %s runs
   engine %s, below the fleet floor %s; rebuild and re-arm it (metasystem
   up), or record a lower floor only if every other seat runs that". An
   unreadable state or a `dev` stamp is an offending line. A registry that
   cannot be read refuses; an empty registry passes this check and proves
   nothing, which the sentence says.

What the page says plainly: check 3 disproves the ruling where the machine
can see; it does not prove it anywhere else. Checks 1 and 2 make the first
abandon a deliberate, recorded, human act on an engine that carries the
grammar. What no check can prove is that a seat on another machine, or an
unarmed checkout, is not still reading the ledger with an old binary. That
seat wedges at its next read until it rebuilds, and the `engine-floor` line
on the root names the commit it must rebuild to.

The order:

1. Land the engine: parser, tree shape, validator, readers, verb, reopen,
   `engine-floor`, `landing held`, the observation binding, `commit.sh` and
   `land.sh`, prune, recovery cases, listing, docs, fixtures, all in one
   landing. The landing itself runs `land.sh` with `METASYSTEM_BIN` at a
   proof build of the candidate, the recipe the seat already uses, because
   the candidate's `land.sh` calls `landing held`, which the installed
   engine does not have. The verbs existing changes no bytes in any ledger.
2. Rebuild and re-arm every enrolled seat on the new engine (`metasystem up`
   refuses drift and re-arms a rebuilt engine from a commit reachable from
   the landing ref; the seat-boot recipe). Confirm each seat's `engineBuild`
   through `metasystem supervise status --repo <checkout>`.
3. At the enrolled terminal, `goal engine-floor --commit <the landing commit
   of step 1> --by Wido`. This write is an ordinary root History line and is
   safe for any seat that missed step 2.
4. The first `goal abandon`. The two specimens (section 12) are the first
   two uses and are Wido's.

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
this specimen. After step 3 of section 10, at the enrolled terminal, with
`--root` at the checkout that last held it (section 4a):

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
published tree on the untouched engine. Every canary below that names a new
symbol (`TreeGoals.Abandoned`, `Abandon`, `ReopenAbandoned`, `Held`, the
`want` parameter of `heldGoal`) fails to compile on the untouched tree; the
rest fail at the boundary named. After the build they pin the reader's
behavior.

Go tests, package `metasystem/internal/goal` unless noted; names are new.

| Canary | Untouched tree: fails at, because | After the build: proves |
| --- | --- | --- |
| `TestDepStateAnswersAbandonedFromTheAbandonedMap` (`verbs_test.go`) | Builds `TreeGoals` in memory with a `GoalFile{State: StateAbandoned}` placed in `Abandoned` and a live goal blocked by it. Compile failure: `TreeGoals` has no `Abandoned` field and `StateAbandoned` does not exist. | `depState == StateAbandoned` for the archived id, and `!= StateDone`; the same id placed in `Done` with State done still answers done. An implementation that never reads the new map cannot return `StateAbandoned`. |
| `TestArchivedAbandonedRecordParsesIntoItsOwnMap` (`validate_test.go`) | `ParseTreeFiles` over a `records/goals/x.md` with `State: abandoned` and a well-formed `Abandoned:` line. Fails at `file.go:430`: unknown state. | Routed to `Abandoned`, absent from `Done`, no problems. |
| `TestValidateTreeRefusesALiveGoalBlockedByAnAbandonedGoal` (`validate_test.go`) | Same parse failure. | Rule 5's exact text; a done goal blocked by it fails the rule at `validate.go:289`. |
| `TestAbandonRefusesUncoveredLiveDependents` (`verbs_test.go`) | `Abandon` undefined: compile failure, the verb does not exist. | Refusal 9 lists every uncovered dependent; nothing lands. |
| `TestAbandonCarriedRepointsEveryDependent` | Same. | Dependents' `BlockedBy` names the successor, each with an `abandon` line and the re-point reason; the abandoned record's line carries `carried=`; `Next` reports the dependent Blocked, not Ready, while the successor is open. This is the constraint-1 defect pinned at the frontier. |
| `TestAbandonWaiveRemovesTheEdgeWithARecordedReason` | Same. | The edge is gone, the dependent's line carries the waive reason, and `--waive` naming a non-dependent refuses. |
| `TestAbandonWaiveRefusesBlankAndDuplicateReasons` | Same. | Refusal 3: `d=` and `d=   ` refuse naming the entry; `d=a` with `d=b` refuses "waived twice"; `d=a` twice refuses the same; a `--waive` and `--also` naming one dependent refuses; a reason with a newline refuses. In every case no tree is read (the test passes an endpoint whose root does not exist). |
| `TestAbandonAlsoCascadesToNamedDependentsOnly` | Same. | Named dependents are abandoned in the same commit with the same because; an unnamed transitive dependent of an `--also` goal refuses under rule 9; `--also` naming a non-dependent refuses. |
| `TestAbandonToleratesItsOwnUnfinishedPrerequisites` | Same. | A goal blocked by a queued goal abandons; the queued prerequisite is untouched. |
| `TestAbandonRefusesWithoutProofOrAReason` | Same. | Refusals 1 and 2: no `--by`, a nil proof, a proof that is not `ValidFor` the root, a blank because, a because with a newline. |
| `TestAbandonHoldsTheGoalRevisionLockAndRefusesAMovedClaim` (`verbs_test.go`) | Same. | With a claimed goal: during `Abandon` the lock at `goalrevision.Path(root, id, rev)` is held (a concurrent `Acquire` from the test returns `LOCK_BUSY`); after `Publish` returns it is released. When the tip's claim revision differs from the projected one (the test re-claims between projection and mutation through a hook on the tree loader, as the existing competitor tests do), refusal 6's text. |
| `TestAbandonRefusesWithoutTheFleetFloor` (`verbs_test.go`) | Same. | Refusal 0's three sentences: no `engine-floor` line; a floor the engine's stamp does not descend from (the test injects the stamp and the ancestry function); an armed checkout below the floor in a fixture registry home (`METASYSTEM_SUPERVISION_REGISTRY_HOME`, the scenario-scoped registry the fixture rules already require). With a floor, a descending stamp and an empty registry it proceeds to refusal 1. |
| `TestEngineFloorIsAProvenHumanRootHistoryLine` (`verbs_test.go`) | `EngineFloor` undefined. | Without proof refuses; with proof appends the line, `Root.Revision` increments, `ParseRoot` of the rendered root reports no problems, and rule 10 refuses a hand-written line whose reason has no commit. |
| `TestAbandonCompactsTheDepartedPriorityLikeDone` | Same. | Survivors re-sequence with `abandon` compaction lines; the abandoned record keeps its historical rank. |
| `TestPruneRetainsAbandonedGoalsAndTheirPrerequisitesOutsideKeep` (`verbs_test.go`, beside `TestPruneKeepsTheClosureAndTheNewest`) | Same, and publication would fail validation. | With keep=0: an abandoned goal, its done prerequisite, and the done prerequisite of that prerequisite survive; an unrelated older done goal is dropped; the abandoned goal is not counted against keep. |
| `TestNextAndIdleRefusalIgnoreAnAbandonedGoal` (`turnverdict_idle_test.go`) | Same. | An approved unclaimed goal produces IDLE WITH BACKLOG; after abandoning it (the only claimable goal) the verdict is not an idle refusal and `Queued` is zero; `Next` returns it in no category. |
| `TestReopenFromAbandonedIsProvenHumanUnrankedUnapprovedAndKeepsTheEvent` (`verbs_test.go`, beside `TestReopenGuardsClaimedDependents`) | `ReopenAbandoned` undefined. | `Reopen` (the done path) called directly on an abandoned id still refuses at `verbs.go:1353-1356` because the id is not in `Done`, and the command never routes an abandoned id there; `ReopenAbandoned` without `Actor.Human` or without a valid proof refuses; with both, an unfenced record lands queued with `Priority=0, Sequence=0, Approved=nil, Budget=nil, Abandoned=nil, StopFence=nil`; the `abandon` (with `carried=`, `reason=`) and `reopen` lines are present in order; a fresh approve and claim mint a new `StopCapability` for the new revision. |
| `TestReopenFromAbandonedRequiresTheStopBatchComplete` (`stop_test.go`) | Same. | A fenced abandoned record: reopen with no batch file refuses "cannot prove stop batch"; with a batch in state other than `COMPLETE` refuses "not COMPLETE" and the record is unchanged; with a `COMPLETE` batch written through `WriteStopBatch` that binds the frozen capability and fence, reopen lands and the `breach-stop`, `abandon` (with `stopId=`) and `reopen` lines are all present. |
| `TestSplitPrechecksReadTheArchive` (`split_test.go`) | The state word does not parse. | `splitRequest` on an abandoned parent refuses "in the archive; there is nothing to split" (row 24), and `AlreadyApplied` when the opid landed. The command-side precheck (row 23) is pinned by the shell scenario below. |
| `TestHandEditToAbandonedHasNoReconcileGrammar` (`reconcilemap_test.go`) | The lenient parse rejects the state word, so the mapping fails earlier than `:282` with a parse error. | A live file edited to abandoned is refused at `:282`; a new archive file with State abandoned is refused at `:117`. |
| `TestRecoveryRefusesToReplayAbandonEngineFloorAndAbandonedReopen` (`recover_test.go`) | Journal entries reach the default at `recover.go:413`, a different message. | The proof-bearing refusal text for `abandon`, `engine-floor`, and `reopen` with `from=abandoned`; a `reopen` without it replays; a landed act is confirmed by opid. |
| `TestAbandonEventsNeverCountAsDone` (`metasystem/internal/counselor`) | A tree with an abandoned record cannot be read (parse). | The Done count over a window with one done and one abandon is exactly one; the abandon and the engine-floor line show in the exclusions limitation. |
| `TestMetricsExcludeAbandonedFromConcludedAndDebt` (`metasystem/internal/metrics`) | Same. | The concluded set and the debt-age rows omit the abandoned goal; `usable` does not count it. |
| `TestObservationBindsTheChainRootsGoalRevision` (`metasystem/internal/landing`, `observe_test.go`) | `heldGoal`'s new parameter and `Observation.GoalRevision` do not exist: compile failure. | A closed chain whose root says `goalRevision: 3` against a base tree claimed at revision 3 passes and the observation carries `goalRevision: 3`; against a base claimed at revision 4 it would-refuse `goal-revision-moved`; a goal-bound root without `goalRevision` is `chain-record-malformed`; register carriage with `--goal` and no record binds the base tree's revision. |
| `TestHeldRecheckReadsTheParent` (`metasystem/internal/landing`, `held_test.go`) | `Held` undefined. | A commit with `Goal-Item`, `Goal-Revision: 3` and `Machine: m+l` above a parent whose goal is claimed by `m+l` at revision 3 passes; above a parent where the goal is in `records/goals/` with State abandoned refuses `goal-item-not-held` naming `abandoned`; above a parent claimed at revision 5 refuses `goal-revision-moved`; the same with `Machine: m+human` exits 0 with the warning line; no `Goal-Item` passes; `Goal-Item` without `Goal-Revision` refuses `goal-revision-unbound` for an agent actor. |
| `TestKeepsSpendingFactTreatsAbandonedLikeDone` (`metasystem/internal/evidence`) | The state word does not parse. | A terminal record of an abandoned goal is not kept; of a pruned (unknown) goal it is kept; of a live claimed goal at the claim revision it is kept. |

The specimen, as a Go test and as a shell scenario:

`TestAbandonOfABreachStoppedClaimKeepsTheFenceFreesTheQuotaAndEnforcesTheDependencyRule`
(`stop_test.go`, seeded exactly like `TestBreachStopFenceAndHumanResumeAreOneWayTransactions`
at `stop_test.go:52`: open, approve, claim with a claim epoch, close the
stop through `CloseStopRequest`). Then: `release`, `done`, `park`, `set-budget`
and `steal` each refuse with the `clearClaimBinding` or fence wording (the
wedge, reproduced). A second approved goal `dependent` blocked by it exists.
A third goal `successor` is opened live (queued) before any `Abandon` call,
because refusal 8 requires `--carried` to name a live goal. `Abandon` without
`--carried` refuses naming `dependent`. `Abandon --carried successor` lands.
Assertions on the landed tree: the record is at `records/goals/<id>.md`
with `State: abandoned`; `StopCapability` and `StopFence` bytes are identical
to the pre-abandon render; `Abandoned.StopID` equals the fence's stop id;
`Claimed` is absent; `ValidateTree` reports no problems; the machine's claim
count is zero, so a claim of a fourth approved goal on the same machine lands
(the quota freed); `dependent.BlockedBy == [successor]` and `Next` lists it
Blocked; the dispatch admission read for the abandoned goal refuses without
a `BUDGET_` verdict; `ProjectBudget` is never reached for it (the goal is
not in `Live`). Untouched tree: compile failure, the verb does not exist.

Shell scenario `abandoned-with-a-reason` in
`metasystem/scripts/agents/goal-cli-fixtures.sh`, added to the scenario list
at line 82 and the case at line 86, through the real binary in the two-clone
sandbox: open A and B with B blocked by A; approve both; claim A; run `job
breach-stop` (the verb at `main.go:141`) to land A's fence; assert `goal
release` refuses with "only goal resume may clear its launch fence"; assert
`goal abandon` refuses with the fleet-floor sentence; `goal engine-floor
--commit <the sandbox engine's build commit> --by Wido` with the fixture
human authority; assert `goal abandon` without `--carried` refuses naming B;
open successor S; `goal abandon --id A --by Wido --because "fixture"
--carried S` with the fixture human authority the other human verbs use;
assert `records/goals/A.md` exists at the origin tip with `State: abandoned`,
`Abandoned: by=human:Wido`, `stopId=`, `carried=S` and a `StopFence:` line;
assert `plans/goals/A.md` is gone; assert `goal list --pretty` prints the
same `done: N archived` count as before the abandon and a new `abandoned: 1
retained with reasons` line; assert `goal list` JSON has `abandoned` with A
and `done` without A; assert `goal next` names neither A nor B as ready;
assert `goal show --id A` prints the reason; assert `goal split --id A`
prints "in the archive; there is nothing to split" (row 23); `goal prune
--keep 0` and assert A survives; `goal reopen --id A` without `--by` refuses
naming the enrolled terminal; with `--by Wido` and the fixture authority it
refuses "not COMPLETE" while the batch is open; mark the batch `COMPLETE`
through the stop-batch verb the existing breach-stop scenario uses; `goal
reopen --id A --by Wido` lands A queued and unranked in `plans/goals/A.md`
with the abandon and reopen lines in History and no `StopFence:` line.
Untouched tree: the first `goal abandon` fails with the unknown-verb exit,
which is the named reason.

Shell scenario `held-refuses-the-landing-of-an-abandoned-goal` in
`metasystem/scripts/agents/land-fixtures.sh` (the bed that drives
`land.sh` and `commit.sh` through the real engine; not in the required gate
below, in the full battery): in clone A claim goal G and dispatch a closed
fake chain against it; run `commit.sh --chain <root> --goal G` on a candidate
and assert the commit carries `Goal-Item: G` and `Goal-Revision: <the
claim's revision>`; in clone B `goal engine-floor` then `goal abandon --id G`
(published to the shared origin); in clone A `git fetch` and `git rebase`
onto origin, then `metasystem landing held --root . --commit HEAD` and assert
exit 1 with `goal-item-not-held` naming `abandoned`; then assert that
`land.sh`'s existing passing scenario with `--goal` still passes, which is
the call site proven. Untouched tree: `landing held` is an unknown verb,
which is the named reason.

The required gate for the landing is unchanged:
`scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh`.

## 14. What the code cannot answer

- Whether every enrolled seat on every machine runs the new engine at the
  moment of the first abandon. Section 10 makes the machine refuse where it
  can see (this user's armed checkouts on this machine) and makes a
  recorded human ruling the gate for what it cannot; the page does not
  claim more.
- Whether a job on another checkout is still running against the goal at
  abandon time. Job records and the goal-revision lock are checkout-local
  (section 4a). The design guarantees that such a job cannot land, and that
  its return touches no goal; it does not see the job.
- Whether the stop batch of a frozen fence ever completes. Abandon does not
  ask; reopen from a fenced record does, and refuses until it does, on the
  checkout that holds the batch. A goal whose batch never completes is
  abandonable and not reopenable, and its work continues in a successor.
- The counselor's ratio treats `abandon` and `engine-floor` as excluded
  verbs. If a later ruling wants abandonment counted as its own class, that
  is a counselor change, not a ledger change.
- Whether a landing on a branch other than the ledger's branch sees the
  ledger's current state. `held` reads the branch being pushed, as the
  observation does today.

## 15. What must not change

Done keeps its meaning and its counts; `concludedAt`, `archiveVerbs` and the
`done` verb class are untouched. The frontier's categories gain nothing.
`clearClaimBinding`, `bindClaim`, `BreachStop`, `Resume` and
`VerifyStopBatchComplete` are untouched. No History line is rewritten or
removed on any path. The legacy archive prefix stays read-only. The record
grammar stays closed: the new state, field, keys and verbs are the whole
extension. The root record gains no field. `Landing-Provenance` keeps its
grammar; the revision binding is its own trailer.

## Revision record

Revision 2 folds the seven findings of the round-1 read, every one accepted
by the coordinator. Sections 2, 7, 11 and 12 are unchanged except where
named.

- **GAW-02 (critical).** Revision 1's inertness premise withdrawn: landing's
  `heldGoal` binds no revision and `land.sh` never re-observes (grounded in
  section 1's new rows). New section 4a: the observation binds the chain
  root's `goalRevision` (`goal-revision-moved`), `commit.sh` stamps a
  `Goal-Revision` trailer, new `metasystem landing held` re-checks at the
  rebased base before every push, and the push's non-fast-forward rule is
  the compare-and-swap that makes the check exact. The transition takes the
  goal-revision lock (the lock dispatch, breach-stop, resume and split
  already share) from before the job-record read through `Publish`, with
  refusal 6 as the compare; the checkout lease lock is named and not taken,
  with the reason. The running job's return, its terminal record, its
  budget accounting, its evidence retention and its visibility are each
  stated. Row 13 rewritten; row 14's gc edit made exact. Section 4a's last
  paragraph says where this meets the parked `landing advance` chain. Four
  Go canaries (observation binding, `held`, the lock, evidence retention)
  and one land-fixtures scenario added.
- **GAW-03 (high).** Section 6 rewritten: reopen from abandoned takes
  `set-priority`'s enrolled-terminal proof on both sides of the package
  boundary (`proveGoalHumanAuthority`, `syncReqWithProof`, `ValidFor`), and a
  fenced record reopens only after `VerifyStopBatchComplete` with the frozen
  bytes, exactly as resume. The frozen-fence-on-a-queued-record option is
  rejected with four reasons, the first of which is `bindClaim`'s
  unconditional fence overwrite. The cost (a never-completing batch makes a
  goal abandonable but not reopenable) is stated. Reopen from done is
  untouched. Recovery refuses to replay a reopen marked `from=abandoned`.
  Abandon's own refusal 1 tightened to `proof.ValidFor` to match
  `SetPriority` exactly.
- **GAW-04 (high).** Section 10 rewritten: the first abandon refuses without
  a human-minted `engine-floor` root History line (new verb `goal
  engine-floor`), refuses on an engine whose own build stamp does not
  descend from the floor, and refuses when any armed checkout of this user
  on this machine reports an older engine (registry joined to each
  checkout's supervision state). What each source can and cannot prove is
  written out; the root field option is shown to be impossible under the
  closed root grammar. Rule 10 added to section 8; recovery, counselor and
  the command table updated; the landing recipe for this chain (`METASYSTEM_BIN`
  at a proof build) named because the candidate's `land.sh` calls a verb the
  installed engine lacks.
- **GAW-01 (medium).** Rows 23 and 24 added for `runGoalSplit` and the split
  parent precheck; "fails closed" withdrawn and section 3 now states the
  guarantee (never "completed") and the residual risk (a forgotten reader
  answers "absent"), with the implementer's census command.
- **GAW-05 (medium).** Refusal 3 added to section 4: `--waive` grammar,
  blank reason, newline, duplicate dependent, and waive-plus-also, all
  before any tree is read; refusal 2 gains the newline rule from `file.go:94`.
  One canary added.
- **GAW-06 (medium).** The dependency canary places the record in
  `TreeGoals.Abandoned` and asserts `StateAbandoned`; its untouched-tree
  failure is now a compile failure, and section 13's preamble says so.
- **GAW-07 (medium).** The Go specimen opens `successor` live before the
  first `Abandon` call, as the shell specimen does.
