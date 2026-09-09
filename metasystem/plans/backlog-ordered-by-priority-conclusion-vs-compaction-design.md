# Backlog ordered by priority: a survivor's `done` event, and the goal the claim gate refuses

Revision: 1. Date: 2026-09-09. Design authoring job: `bolboc-design3`.

Parent specification: `metasystem/plans/backlog-ordered-by-priority-design.md`
revision 2, whose critique ladder is closed. This page answers the two findings
that revision left to the design lane:

- BOC-01, from `metasystem/records/misc/backlog-ordered-by-priority-critique-r3.md`:
  a goal that survives a neighbour's conclusion carries a history event whose
  verb is `done`, and one metrics reader reports it as concluded.
- BOQ-02, from `metasystem/records/misc/backlog-ordered-by-priority-critique-r7.md`:
  routing the frontier through claim admission dropped an approved goal whose
  budget exceeds its tier box out of every frontier category, and with it out
  of the idle-stop refusal, with nothing naming the cause.

Both are the same shape: a reader states something untrue about a goal.
Neither can wedge the queue, refuse a human act or corrupt the ledger. This
page specifies the build; it implements nothing. Independent critique,
dispositions, certification and receipts belong to the orchestrator. Paths are
relative to the repository root. Line numbers name the revision read for this
page, tree `a960af0c`.

What stays fixed, from the brief: the rank is the human's judgement and
nothing here sets, infers or reorders a priority or sequence; `goal next`
stays a read, true at the accepted tip it read; the claim gate stays the one
admission owner; and the ledger's record grammar does not change. Question 1
does not force a grammar change, and section 1.3 says why.

## Grounding in the current tree

Source observations, not runtime proof.

| Responsibility | File and symbol | Observed behaviour and the consequence for this page |
| --- | --- | --- |
| Compaction on `done` | `metasystem/internal/goal/verbs.go:1106`, `doneRequest`; `:1154`, state set; `:1162`, `compactDepartedPriorities`; `:1163-1167`, event targets; `:1172-1176`, survivor events | The departed goal's own `done` event receives the whole compaction target set (departed plus shifted survivors) at 1163-1166 before `touchDisplaced` writes it at 1167. Each shifted survivor then gets an event through `mergePriorityEvent` with the same operation id, timestamp, verb and actor. Only the survivor's `Reason` differs. |
| Compaction event writer | `metasystem/internal/goal/order.go:188`, `compactDepartedPriorities`; `:226`, `mergePriorityEvent`; `metasystem/internal/goal/verbs.go:249`, `touch` | 228-233: when the survivor's last event already carries this operation id, targets are unioned into it; otherwise `touch` appends a fresh event stamped with the request's time and id. 234-239: the reason `priority-order from=<pair> to=<pair>` is set, or appended after `; ` when a reason already exists. |
| Compaction on split and reconcile | `metasystem/internal/goal/split.go:301-315`; `metasystem/internal/goal/reconcilepub.go:124-140` | Split rewrites the parent's and every touched survivor's targets to the union at 307-312, then merges with verb `split`. Reconcile unions the compaction targets into each departed goal's own last event at 125-131 and merges survivors with verb `done` at 132-139. The departed record therefore never has a lone target when a rank was involved. |
| History grammar | `metasystem/internal/goal/file.go:287`, `HistoryLine`; `:1303`, `ParseHistoryLine`; `:1318-1322`, reason; `:1340-1422`, key switch; `:1421`, unknown-key refusal | The key set is closed and an unknown key refuses the line. `reason=` consumes the rest of the line. The verb is a free token; nothing validates it against a vocabulary. |
| Record state and placement | `metasystem/internal/goal/file.go:355-361`, states; `metasystem/internal/goal/validate.go:29-32`, `TreeGoals.Live` and `Done` | State is a closed field. `doneRequest` sets `done` and moves the record to `Done` in the same commit that compacts survivors, whose state is untouched. |
| The misreading reader | `metasystem/internal/metrics/compute.go:511`, `concludingEpoch`; `:543`, `computeWaiting` | Scans history backwards for the first `done` verb, takes its timestamp as the conclusion, counts `claim` and `steal` events before it as epochs, and never consults `State`. `computeWaiting` calls it at 550 for every selected goal. |
| Sibling readers of the same verb | `metasystem/internal/metrics/compute.go:144`, `historyTime`; `:157`, `goalBounds`; `:166`, `selectedGoals`; `:191`, `computeOverhead`; `metasystem/internal/metrics/report.go:203`, `ConcludedInWindow` | `historyTime(file, "done")` is the same backward scan in generic form. `selectedGoals` filters on `State == done` at 181 for the whole-period path but returns any record for `--goal <id>` at 167-171. `computeOverhead` re-checks state at 201. `ConcludedInWindow` has no state check and no caller in the tree. |
| Metrics goal loader | `metasystem/internal/metrics/data.go:815`, `loadGoals` | Parses every goal path on the accepted tip except `backlog.md`, so live survivors sit in `w.Goals` beside archived records. |
| Counselor event classes | `metasystem/internal/counselor/sources.go:515-533`, `addHistory`; `:578`, `goalVerbClass`; `metasystem/internal/counselor/compute.go:113-131` | Classifies by verb per operation id across root, live and done records. A second line with the same id, timestamp and verb is skipped at 524-527; a differing verb marks a conflict and excludes the id. Counts are per window, never per goal. |
| Other history-verb readers | `metasystem/internal/report/scan.go:132-136`; `metasystem/internal/metrics/compute.go:725,737`; `metasystem/internal/goal/file.go:316-323,348-352,593`; `metasystem/internal/goal/verbs.go:82,87`; `metasystem/internal/goal/norm.go:73-81` | Read `open`, `steal` and displacement, the relayed-authority verbs, `answer`, and a human operation's reason for a strict approval token. None reads `done`. No shell script parses a `History:` block; the two matches under `metasystem/scripts/agents/` are fixture writers. |
| Frontier | `metasystem/internal/goal/project.go:493-500`, `NextVerdict`; `:520`, `SelectNext`; `:533`, `Next`; `:571-575`, admission branch | Four categories: Claimed, Ready, Blocked, Awaiting. At 571 the approved arm calls the claim gate; success appends to Ready, a non-refusal error fails the whole frontier, and a goal-level refusal falls through both branches and is recorded nowhere. |
| Admission gate | `metasystem/internal/goal/approval.go:345`, `requireApprovedForClaimWithContext`; `:270-283`, `goalAdmissionRefusal`; `:298-321`, one configuration load per frontier; `metasystem/internal/goal/norm.go:95`, `refuseGoalNorm`; `metasystem/internal/goal/approval.go:261`, `approvalRequired` | Goal-level refusals are `APPROVAL_REQUIRED` (no approval or no budget, or an invalid approval record) and `GOAL_NORM_REFUSED` (budget above the tier box without norm coverage). `APPROVAL_EXPIRED` exists at 359-362 but `Next` filters expiry at 552 before calling the gate. Each carries the remedy in its text. |
| Claim verb | `metasystem/internal/goal/verbs.go:587-598`, `Claim`; `:648`, gate call | The brain fence and the human-actor refusal run before any record is read; they are facts about the checkout, not the goal. The gate at 648 is the same predicate the frontier calls. |
| Seat readers | `metasystem/internal/goal/project.go:193-200`, `ClaimableBudgetedWork`; `:283-332`, `readClaimableBudgetedWork`; `metasystem/internal/goal/turnverdict.go:410`, `enforceIdleBacklog`; `:473`, `idleBacklogNames`; `:483`, `idleBacklogDigest`; `:940`, `queuedFrontier`; `metasystem/internal/steward/revive.go:177`, `claimSeatIdleGoal`; `:259-289`, seat-idle recheck; `metasystem/internal/steward/ledgerattention.go:147`, `snapshotLedger` | Claimable is the frontier's Ready slice; Queued is the Awaiting count. An empty Claimable resets the idle counter and returns at 442-446. The digest hashes Claimable, Claimed and non-terminal jobs. The steward's continuation re-reads Claimed or Claimable and claims through the real verb. The attention snapshot keeps Ready, Pinned and Queue. |
| Command surface | `metasystem/cmd/metasystem/goal.go:463`, `nextSynced`; `:509-519`, the `none` explanation; `:339`, `listSynced` | The `none` line appends, in order: first blocked goal, awaiting count and first, empty backlog, or `no matching eligible work`. `listSynced` computes no frontier and no admission. |
| Channel status | `metasystem/internal/channel/report.go:76-81`; `:137-167`, `backlogStatusLines` | The `Next for <machine>` line prints the selection or `none claimable`. The frontier is already in hand at 137. |
| Guidance owners | `metasystem/docs/backlog-mechanism.md:120-125`; `metasystem/AGENTS.md:34`; `metasystem/scripts/agents/roles/steward-continuation.md:10-14` | The mechanism page owns the frontier paragraph. The goal record states that `AGENTS.md` is at its audited word ceiling, so this page adds nothing there. |
| Existing fixtures | `metasystem/internal/goal/order_test.go:133-157`, `over-norm`; `:445-479`, `done-reopen`, with the survivor event asserted at 462-465; `:756`, `rankedGoalBed`; `metasystem/internal/metrics/obligations_test.go:351-386`, world-struct shape; `:418`, `detailsContain`; `metasystem/cmd/metasystem/goal_priority_test.go:109-142`, `:264-296`; `metasystem/internal/channel/channel_test.go:325-340`, `:771`, `reportGoal`; `metasystem/internal/goal/turnverdict_idle_test.go:17`, `budgetedQueuedGoal`; `:103-157`; `metasystem/internal/goal/servingprojection_test.go:12`, `servingBed`; `metasystem/scripts/agents/goal-cli-fixtures.sh:798-813` | The `over-norm` subtest already builds an approved goal whose 2400 reserved minutes exceed the default tier-3 box and asserts only the selection. `done-reopen` pins the survivor's last event: verb `done`, targets `b,c`, reason containing `from=1:3 to=1:2`. `reportGoal` builds tier-1 goals with a 30-minute budget. The shell bed asserts two exact `none` lines word for word. |
| Tier boxes | `metasystem/metasystem.conf:14-16` | `elapsed/attempts/minutes/active-jobs/review-rounds`: tier 1 is 360 minutes and 0 rounds, tier 2 720 and 2, tier 3 1200 and 3. Fixture roots that declare `metasystem.runtimes=fake` resolve the same defaults without a file (`order_test.go:16-18`, `split_test.go:23-28`). |

## 1. A survivor's `done` event is not its conclusion (BOC-01)

### 1.1 What the ledger writes

When a ranked goal `b` at 1:2 concludes beside `a` at 1:1 and `c` at 1:3,
one transaction writes three things: `b` moves to the archive with state
`done`; `c` becomes 1:2; and both `b` and `c` receive a history line under the
one operation id, at the one timestamp, with the one verb and actor. On `b`
the line reads `done actor=<actor> targets=b,c`. On `c` it reads
`done actor=<actor> targets=b,c reason=priority-order from=1:3 to=1:2`. The
lines are identical except for the reason. The record of this very goal shows
the survivor form: its `done` line of 2026-09-08 names thirty targets and
`from=1:3 to=1:2`, and its state is `claimed`.

This is the design working as specified. A history line's verb names the
operation that touched the record, not a transition of the record it sits
in. That was already true before ranks existed: a `set-priority` line lands
on every displaced peer, a `split` line lands on a dependent whose blockers
changed, and an `ack` line lands on a displaced claimant. `done` joining that
list made an old reader assumption observable, it did not create it.

### 1.2 Reader census: the "only place" claim, confirmed in effect and refuted in letter

The critic reported that `concludingEpoch` is the only place inferring meaning
from a history verb on a goal record. A whole-tree search of every `.Verb`
access and every `"done"` literal outside tests finds four functions that read
the `done` verb, all in `metasystem/internal/metrics`:

| Reader | State check | Reaches a user today |
| --- | --- | --- |
| `compute.go:511`, `concludingEpoch` | none | Yes. `computeWaiting` for `metasystem metrics report --goal <survivor>` reports a finished lifecycle dated to the neighbour's conclusion, provided the survivor holds or held a claim; without a claim it already says `lifecycle incomplete`. |
| `compute.go:144`, `historyTime`, through `:157`, `goalBounds` | none in the reader; `computeOverhead` re-checks state at 201 | No, by caller discipline only. |
| `compute.go:166`, `selectedGoals` | at 181, whole-period path only | No. The `--goal` path returns the record unfiltered, and `computeWaiting` then misreads it as above. |
| `report.go:203`, `ConcludedInWindow` | none | Not today: exported, no caller in the tree. Its name promises a conclusion, so it is a latent misreader. |

The counselor at `sources.go:522` reads the verb too, but per operation: the
survivor's line has the same id, timestamp and verb as the departed goal's, so
it is skipped as a duplicate and the operation counts once as `GoalDone`. A
reconcile batch that mixes verbs under one id was already excluded as a
conflict before ranks existed; compaction adds no new conflict, because a
survivor only carries a non-`done` verb when its own row already did.
Everything else that reads a history verb reads `open`, `steal`, the
authority verbs or `answer`.

So the reading-side repair stays local: it is one package, and the equivalent
`split` compaction event stays harmless for the same reason the critic gave.
The census also settles the blast radius the other way: no reader outside
metrics needs a change, and no writer does.

### 1.3 The discriminator is the record's own State

Three discriminators were offered. The reader uses the third.

**The record's State.** A survivor's state is `approved`, `claimed`, `parked`
or `queued`; a concluded goal's is `done`, and it lives in `Done`. This is the
ledger's closed truth, written by the same transaction that writes the event,
and every validator already reads it. Under `State == done`, the last `done`
verb in history is always the goal's own conclusion: an archived record never
receives a compaction line, because every compaction loop walks `t.Live`
(`order.go:201`, `split.go:307`, `reconcilepub.go:132`) and the parent page
forbids a live write for an archived id; `reopen` returns the record to the
live set, where its state is no longer `done`; and a later conclusion appends
a later `done`. The reader therefore needs one new test and no new data.

**Why `reason=priority-order` is weaker.** The parent page defines the reason
payload as evidence, never authority or replay input. Reconcile appends it to
whatever reason a row already wrote, joined by `; ` (BOC-04 notes an exact
word-count reader elsewhere). A reader keyed on it would match prose to decide
lifecycle, and would elevate a diagnostic string into semantics that the page
says it does not carry.

**Why `targets` cannot work at all.** The brief's second discriminator says a
real conclusion's targets name the concluded goal alone. That is true only for
an unranked goal, which has no compaction; the observed line
`done ... targets=claude-critic-shell-network-deny` is one. For a ranked goal,
`doneRequest` writes the compaction target set onto the departed goal's own
event at `verbs.go:1163-1167`, reconcile unions it in at
`reconcilepub.go:125-131`, and split rewrites it at `split.go:312`. The
departed goal's line and each survivor's line carry the same targets. Targets
discriminate nothing for exactly the records in question, and a reader built
on them would have marked every ranked conclusion as a compaction.

**No marker, no grammar change.** The three discriminators are not
insufficient; one of them is sufficient and already exists. A marker on the
compaction line would be a second spelling of the fact that `State` spells
once, which is the lesson this goal already paid for twice in the frontier.
It would also extend the closed key set at `file.go:1421`, so every deployed
reader would have to land before any writer could write, the same fleet
constraint the rank fields carried. And a marker changes nothing until a
reader consults it, so the reader changes either way. Therefore `file.go`,
`validate.go`, and every writer in `verbs.go`, `split.go` and
`reconcilepub.go` are untouched, and the archived-record rules need no
account because no record byte changes.

### 1.4 The mechanism

All changes are in `metasystem/internal/metrics`.

1. Add an unexported helper in `compute.go`, beside `historyTime`:

   ```go
   // concludedAt is the one reader of a goal's conclusion time. A done verb
   // in history is not a conclusion by itself: a ranked survivor receives the
   // departing act's verb when a neighbour concludes. The record's State says
   // whether this goal concluded; the last done verb then says when.
   func concludedAt(file *goal.GoalFile) (time.Time, bool) {
       if file == nil || file.State != goal.StateDone {
           return time.Time{}, false
       }
       return historyTime(file, "done")
   }
   ```

2. `concludingEpoch` gains the same test as its first statement and returns
   `time.Time{}, time.Time{}, 0, false` for a record whose state is not
   `done`. The backward scan, the epoch count and the claim lookup are
   unchanged after it. Because the early return precedes the epoch count, an
   open survivor reports `epochs=0`, which matches what a goal with no `done`
   line reports today (`TestO10LifecycleEdgesStayIncompleteOrNameEpochs`, the
   `no-done` record at `obligations_test.go:369-382`).

3. `goalBounds` at `:162` and `selectedGoals` at `:184` call `concludedAt`
   instead of `historyTime(record.File, "done")`. The state filter at `:181`
   stays; the helper makes the reader safe on its own rather than by caller
   discipline.

4. `ConcludedInWindow` in `report.go:203` calls `concludedAt`. It keeps its
   name and signature; whether an exported predicate with no callers should be
   deleted is not decided here.

5. After this change `historyTime` has no `"done"` caller outside
   `concludedAt`. A new call site passing `"done"` to `historyTime` is a
   defect under this page.

Report text is unchanged. For an open survivor the waiting row prints
`unavailable` with the existing detail `lifecycle incomplete: goal=<id>
epochs=0`, and the overhead row keeps its existing unavailable text. No new
wording is introduced, so no existing assertion on report text moves.

Recorded without action: the overhead row's unavailable text says `not
concluded with a done history row`, and a survivor does have a `done` row. The
verdict is right and the words are slightly off. Changing them is a string
edit other tests may pin, and it is not what the finding is about.

### 1.5 Fixtures and the canary

Every command runs from the repository root with a two-minute ceiling. The
canary is written first and run on the untouched tree; it must fail there
with the text named below before `compute.go` is edited.

New file `metasystem/internal/metrics/conclusion_test.go`, test
`TestSurvivorOfANeighboursConclusionIsNotConcluded`, built on the world-struct
shape of `obligations_test.go:351-386`. Timestamps: T0 `2026-09-01T00:00:00Z`,
T1 `2026-09-01T01:00:00Z`, T2 `2026-09-02T00:00:00Z`, T3
`2026-09-03T00:00:00Z`. Period instant T3 plus one day. Every history line
carries an actor and `Keep: -1` as the writer would produce.

| Subtest | Record | Assertion | Before the repair |
| --- | --- | --- | --- |
| `waiting-row` (canary) | `c`: state `claimed`, priority 1, sequence 2, `Claimed{mac-a, lin-1, T2}`; history `open` T0, `approve` T1 by `human:wido`, `claim` T2, `done` T3 by `human:wido` with targets `b,c` and reason `priority-order from=1:3 to=1:2`. One landing attributed to `c` at T2 plus twelve hours. | `computeWaiting(w, period, "c", limits)` has `Value == "unavailable"` and a details element whose text equals, exactly, `lifecycle incomplete: goal=c epochs=0`. Use equality on the element, not `detailsContain`, so a `no attributable landing` variant cannot satisfy it. | Value is `c building_hours=12.000 proving_hours=12.000 waiting_share=0.500 epochs=1`: a finished lifecycle for an open goal, dated to `b`'s conclusion. The subtest fails on `Value`. |
| `reopened-then-survived` (canary) | `d`: state `approved`; history `open` T0, `claim` T1, `done` T2 with targets `d`, `reopen` T2 plus one hour, `done` T3 with targets `b,d` and the compaction reason. One landing attributed to `d` between T1 and T2. | Same two assertions with `goal=d`. | Value reports a lifecycle from T1 to T3, because the last `done` is the compaction and the reopen is invisible to the scan. |
| `concluded-in-window` (canary) | The `c` record above. | `ConcludedInWindow(c, T3 minus one hour, T3 plus one hour)` is false. | It is true. |
| `departed-neighbour-still-concludes` (guard) | `b`: state `done`; history `open` T0, `claim` T2, `done` T3 with targets `b,c` (its own conclusion carries the compaction targets, as the writer produces). One landing attributed to `b` at T2 plus twelve hours. | Value is `b building_hours=12.000 proving_hours=12.000 waiting_share=0.500 epochs=1`. | Passes before and after. It proves the state discriminator did not need targets, and it would have failed under a targets-based rule. |

Command for the canary, then each subtest by name:

```sh
cd metasystem && go test ./internal/metrics -run '^TestSurvivorOfANeighboursConclusionIsNotConcluded$/^waiting-row$' -count=1 -timeout=2m
```

Widen only to the package: `cd metasystem && go test ./internal/metrics
-count=1 -timeout=2m`. `TestO10LifecycleEdgesStayIncompleteOrNameEpochs`,
`TestWaitingZeroDurationLifecycleIsLabelledWithoutJudgment` and the
attribution tests all use `State: done` records and must not move.

**Pin the premise the fixture copies.** The metrics fixture hand-writes the
survivor's history, so it must mirror a shape the writer's own test pins.
Extend `TestPriorityLifecycle/done-reopen` in
`metasystem/internal/goal/order_test.go` after line 465 with three
assertions: `tree.Live["c"].State != StateDone`; the survivor's last event and
`tree.Done["b"]`'s last event have equal `Opid`, `At`, `Verb`, `Actor` and
`Targets`; and `tree.Done["b"]`'s last event has an empty `Reason`. Run:

```sh
cd metasystem && go test ./internal/goal -run '^TestPriorityLifecycle$/^done-reopen$' -count=1 -timeout=2m
```

**Independent observation, orchestrator only.** The delegate sandbox cannot
run the process-owning beds, so this is the orchestrator's check after the
build. In a scratch clone with three ranked goals where the survivor holds a
claim, conclude the middle goal, then run `metasystem metrics report --goal
<survivor>` and read `artifacts/agents/metrics/goal-<survivor>.md`. Before the
repair the waiting metric carries `detail=lifecycle incomplete:
goal=<survivor> epochs=1 no attributable landing` (the reader believed the goal
concluded and went looking for landings). After it the detail is
`lifecycle incomplete: goal=<survivor> epochs=0` with no suffix.

## 2. The goal the claim gate refuses (BOQ-02)

### 2.1 What slice 2 changed, stated correctly

Before slice 2 the frontier admitted an approved goal to Ready after labels,
approval expiry, pin and blockers. A goal whose approved budget exceeds its
tier box passed all four, so it was in Ready, in `Claimable`, and in the idle
refusal: a seat with only such work was blocked three times and then handed a
continuation whose `claimSeatIdleGoal` call reached the real claim verb and
was refused with `GOAL_NORM_REFUSED`. That is a livelock, and the brief that
called the goal "invisible exactly as before" was wrong.

After slice 2 the frontier calls the claim gate and drops a refused goal from
every category at `project.go:571-575`. The seat may end its turn, which is
right, because no seat act can claim the goal. But the goal now belongs to no
category, `goal next` says `no matching eligible work` while `goal list` shows
the goal approved at the head, the channel says `none claimable` beneath a
`Backlog first` line naming that same goal, and nothing anywhere says why.
BOO-01 fixed the fleet-wide half of this (a configuration error fails the
whole frontier) and deliberately left the goal-level half for this page.

### 2.2 The category: Refused

A **refused** goal is one that is approved, matches the label filter, has an
unexpired approval, is unpinned or pinned to this machine, has every blocker
done, and that the claim gate refuses for a cause that is a fact about the
goal. It is the fifth frontier category, in rank order like the other four.

It is not Awaiting. Awaiting means the human has not approved, or a relayed
approval expired; the record says `queued` or the listing says `EXPIRED`, and
the `none` line counts them as awaiting the human's approval. A refused goal
says `approved` in every listing. Folding it into Awaiting would count
approved goals as unapproved and would lose the cause, which is the whole
finding. It is not Blocked: no other goal concluding unblocks it. It is not
Ready: the parent page's fixture `over-norm` exists to keep it out.

What resolves a refusal is a human act, named by the gate's own text:
`goal set-budget` within the tier box, the strict norm approval by
`--approved-ref` or a verified `budget-above-norm` channel answer
(`docs/backlog-mechanism.md:36`), or repair of an invalid approval record. No
seat act resolves it. So seats are not blocked on it, are never handed it, and
are told the cause once so they do not look for work that is not there.

The refusal causes that can reach the frontier are `APPROVAL_REQUIRED`
(approved without a budget, or an approval record that fails
`ValidateApprovalRecord`) and `GOAL_NORM_REFUSED`. `APPROVAL_EXPIRED` is
filtered into Awaiting before the gate is called. The brain fence and the
human-actor refusal in `Claim` are facts about the checkout and stay outside
the frontier; that is BOQ-01, still parked on the goal record, and it is not a
category of goal.

### 2.3 The mechanism, and what each reader does

**Frontier, `metasystem/internal/goal/project.go`.** Add

```go
// AdmissionRefusal records a goal the claim gate would refuse, with the
// gate's own cause. It is a fact about the goal, never about the machine.
type AdmissionRefusal struct {
    GoalID string
    Cause  string
}
```

and a fifth field `Refused []AdmissionRefusal` on `NextVerdict`, documented
as `approved and otherwise eligible, but the claim gate refuses; the cause is
the gate's text`. At `:571-575` the branch becomes: success appends to Ready;
`isGoalAdmissionRefusal(err)` appends `AdmissionRefusal{GoalID: id, Cause:
err.Error()}` to Refused; anything else returns the existing frontier error.
This is the same call to the same predicate; the only change is that its
answer is kept instead of dropped. `SelectNext` is unchanged and never reads
Refused.

**`ReadClaimableBudgetedWork`, `project.go:283-332`.** `ClaimableBudgetedWork`
gains `Refused []AdmissionRefusal`, copied from the frontier at 326. Claimable,
Claimed and Queued are unchanged. The legacy path has no refusals.

**Turn verdict, `metasystem/internal/goal/turnverdict.go:410`.** In
`enforceIdleBacklog`, after the `work == nil` return at 438-440 and before the
digest is computed at 441, when `len(work.Refused) > 0` append one line to
`verdict.Display` (trimmed, newline-joined as the existing lines are) and to
`verdict.Diagnostics`:

```text
CLAIM WOULD REFUSE: <first id>: <first cause>
```

with ` (and <n> more)` appended when more than one goal is refused. Nothing
else changes: `ShouldBlock`, `BlockSource`, `IdleRefusal`, `IdleBlocks`, the
early return on an empty Claimable, and `escalateIdleBacklog` are untouched.
`idleBacklogDigest` does not read Refused; the fixture asserts that two
readings differing only in Refused hash the same, so a cause changing does not
reset a refusal count.

**Steward.** `claimSeatIdleGoal`, `decideForRevival` and
`IdleEscalationEvent` are unchanged; a refused goal is never in Claimable, so
it is never a continuation target. `snapshotLedger` is unchanged and Refused
is not in the attention snapshot: attention events report ledger movement,
and a refusal is a reading of configuration against a record. The human's
surface for it is the channel below.

**`goal next`, `metasystem/cmd/metasystem/goal.go:509-519`.** The `none`
explanation gains a first case, ahead of blocked:

```text
no claimable goal for machine <nick>; claim would refuse <n> (first: <id>): <cause>
```

where `<n>` is `len(frontier.Refused)` and the cause is the first refusal's
text verbatim. Refused goes first because it is the one category where the
listing says `approved` and the frontier disagrees; blocked and awaiting goals
already explain themselves in `goal list`. The `continue` and `next ready
goal` lines are unchanged: orientation stays one line. The label-only message
`no goal matches --label ...` and the remaining cases are unchanged, so
`goal-cli-fixtures.sh:805` and `:812` keep their exact text.

**Channel status, `metasystem/internal/channel/report.go:155-165`.** Whenever
`frontier.Refused` is non-empty, the `Next for <machine>` line ends with

```text
; skipped <feature name>: <cause head>
```

naming the first refused goal, where the cause head is the gate's text up to
its first `;`, folded through `statusLineText`. It applies to all three
selection kinds, so `Next for m1: none claimable; skipped over norm:
GOAL_NORM_REFUSED: goal over-norm tuple minutes=2400 reviewRounds=3 exceeds
its tier box minutes=1200 reviewRounds=3` and `Next for m1: local candidate
— priority 2, sequence 1, unpinned; skipped global head: GOAL_NORM_REFUSED:
...` are both possible. The `Backlog first` line is unchanged, so the human
sees the head and, on the next line, why it is skipped. The line count is
unchanged and the digest changes only when the cause changes, which honours
the BOP2-01 fold.

**`goal list`.** Unchanged, deliberately. It is machine-agnostic and computes
no frontier; showing a refusal there would need either a machine or a second
admission traversal outside `Next`, and the brief keeps admission to one
caller. The human reaches the cause through `goal next --machine <nick>` at a
terminal or the channel line remotely. If Wido wants it in the listing, that
is a separate small decision, not a gap in this one.

**Guidance.** Add one sentence to the frontier paragraph at
`metasystem/docs/backlog-mechanism.md:120-125`: a goal the claim gate would
refuse is reported by `goal next` and the channel status with the gate's own
cause, is never handed to a seat, and is repaired by the human act the cause
names. `AGENTS.md` is at its word ceiling and gets nothing; the `goal next`
line now carries the cause, so the seat contract needs no new words.
`steward-continuation.md` is unchanged for the same reason.

### 2.4 What the human sees

At a terminal, `metasystem goal next --machine m1` on a backlog whose only
approved work is over the box prints the `claim would refuse` line with the
remedy in the gate's text. In the channel the status post shows the head at
its rank and state on the `Backlog first` line, and `skipped <name>: <cause>`
on the `Next for` line beneath it. The seat's own turn verdict shows one
`CLAIM WOULD REFUSE` line and lets the turn end. Nothing changes the rank, the
approval, the budget or the claim gate.

### 2.5 Fixtures

The unit fixtures compile only once the new field exists, so the fail-before
proof for this question is carried by the command and channel canaries, which
assert on output text that the current tree produces differently.

| Fixture | Smallest proving run | Required observation | Before the repair |
| --- | --- | --- | --- |
| Frontier: extend `TestNextPriority/over-norm` in `metasystem/internal/goal/order_test.go:133`; add subtest `refused-only` | `cd metasystem && go test ./internal/goal -run '^TestNextPriority$/^over-norm$' -count=1 -timeout=2m` | `frontier.Refused` is exactly one entry, `GoalID == "over-norm"`, cause containing `GOAL_NORM_REFUSED`; Ready is exactly `claimable`. `refused-only` has the over-norm goal alone: `SelectNext` is `none`, Refused names it, Ready, Blocked and Awaiting are empty. | Does not compile. |
| Command canary: add `refused` to `TestGoalPrioritySelection` in `metasystem/cmd/metasystem/goal_priority_test.go:109` | `cd metasystem && go test ./cmd/metasystem -run '^TestGoalPrioritySelection$/^refused$' -count=1 -timeout=2m` | `prioritySelectionFixture` with one goal from `commandApprovedPriorityGoal("over-norm", 1, 1, "")` whose `ReservedJobMinutesLimit` is set to 2400 before rendering (the approval digest must be computed over that budget). `runGoalNext --machine m1` exits 0 and prints a line containing `no claimable goal for machine m1; claim would refuse 1 (first: over-norm): GOAL_NORM_REFUSED` and not containing `no matching eligible work`. | Prints `no claimable goal for machine m1; no matching eligible work`. Fails on the text. |
| Channel canary: add `refused-head` and `refused-only` to `TestReportPriority` in `metasystem/internal/channel/channel_test.go:325` | `cd metasystem && go test ./internal/channel -run '^TestReportPriority$/^refused-head$' -count=1 -timeout=2m` | `refused-head`: `reportGoal("global-head", ..., approved, "", now)` at 1:1 with `ReservedJobMinutesLimit` 2400 (over the tier-1 box of 360; recompute the digest), `local-candidate` at 2:1 within the box. The text contains `Backlog first: global head — priority 1, sequence 1, approved, unpinned` unchanged and `Next for m1: local candidate — priority 2, sequence 1, unpinned; skipped global head: GOAL_NORM_REFUSED`. `refused-only`: the over-box goal alone; text contains `Next for m1: none claimable; skipped global head: GOAL_NORM_REFUSED` and the whole report has the same line count as `no-delivery`. | Both `Next for` lines end without the `skipped` clause. Fails on the text. |
| Seat: new `TestRefusedBacklogIsReportedWithoutBlocking` in `metasystem/internal/goal/turnverdict_idle_test.go` | `cd metasystem && go test ./internal/goal -run '^TestRefusedBacklogIsReportedWithoutBlocking$' -count=1 -timeout=2m` | `servingBed("bed-m1", ...)` with one `budgetedQueuedGoal` whose `ReservedJobMinutesLimit` is 2400, ranked 1:1; `servingBed` approves it over that budget (`servingprojection_test.go:48-63`), so the refusal is the norm, not a missing approval. `ReadClaimableBudgetedWork` returns empty Claimable and Refused naming it with a cause containing `GOAL_NORM_REFUSED`. `enforceIdleBacklog` on that work leaves `ShouldBlock` false, `IdleRefusal` false and `IdleBlocks` zero, and `Display` contains `CLAIM WOULD REFUSE: <id>: GOAL_NORM_REFUSED`. `idleBacklogDigest` of the work equals the digest of a copy with Refused cleared. Then three `TurnVerdict` calls with `PrepareIdleContinuation` and `RecordIdleIncident` hooks installed as in `TestPriorityIdleProjection`: no verdict has `BlockSource` `idle-backlog`, no verdict sets `IdleRefusal`, and neither hook is called, which is the livelock not returning. The assertion is on the idle path alone so an unrelated ladder block cannot be mistaken for it. | Does not compile. |
| Shell bed | `scripts/agents/goal-cli-fixtures.sh` through the gate | Steps at `:800-813` keep their exact `none` text; no scenario there has a refused goal. | Green, and must stay green. |

### 2.6 What this page does not decide

- BOQ-01, the brain checkout whose seat is told to take work and refused
  every time, stays on the goal record as the deferred seat-guidance sentence.
  It is a checkout fact, not a frontier category, and the `AGENTS.md` ceiling
  is its blocker, not this design.
- BOP2-02, a tier outside 1 to 3 riding the fleet-wide could-not-answer path,
  is unchanged; the brief did not ask and widening the separation is the kind
  of unasked change that cost the parent chain rounds.
- The refusal text is the gate's. The channel truncates at the first
  semicolon for length; if the gate's text changes shape, the whole text
  shows, which is honest rather than wrong.
- Whether `goal list --pretty` should carry the refusal is left to Wido, for
  the reason in section 2.3.

## 3. Build boundary and gate

The build changes: `metasystem/internal/metrics/compute.go` and `report.go`;
new `metasystem/internal/metrics/conclusion_test.go`;
`metasystem/internal/goal/project.go`, `turnverdict.go`, `order_test.go` and
`turnverdict_idle_test.go`; `metasystem/cmd/metasystem/goal.go` and
`goal_priority_test.go`; `metasystem/internal/channel/report.go` and
`channel_test.go`; and one sentence in `metasystem/docs/backlog-mechanism.md`.

It does not change: `metasystem/internal/goal/file.go`, `validate.go`,
`verbs.go`, `split.go`, `reconcilepub.go`, `order.go`, `approval.go` or
`norm.go`; any goal record under `plans/goals/` or `records/goals/`;
`metasystem/AGENTS.md`; or the steward's continuation, revival and attention
code. No rank is set, inferred or moved; `goal next` remains a read; admission
remains one predicate with one more branch recording its answer.

Order of work: write the question 1 canary and run it red on the untouched
tree; repair `compute.go` and `report.go`; run the canary green and the metrics
package; extend `done-reopen`; then the question 2 frontier field, its
readers, the command and channel canaries red then green, and the seat test.
A canary that passes before its repair is a defect in the canary, not
evidence of health, and stops the build until it fails for the right reason.

The requested gate, from the repository root:

```sh
cd metasystem && scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh
```

Process-owning beds run outside the delegate sandbox by the orchestrator, as
the parent page records. This design-only artifact claims no runtime gate.
