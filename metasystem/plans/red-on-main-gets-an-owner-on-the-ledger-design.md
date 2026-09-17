# Design: red on main gets an owner on the ledger

Revision 3, 2026-09-17: records Wido's rulings of 05:52Z (option A; a fix branch on every entry), m1e's closed-entry rule, what U0 and U2a built, and the option A detail R1 builds; no critique round follows it and R1 takes this page as its whole specification. Author: Claude Fable 5.1 design delegate, launched headless by seat m1b.
Goal: red-on-main-gets-an-owner-on-the-ledger (tier 3, claimed by m1b). Paths are under `metasystem/`; revision 2 refs are at R0 commit c87e30eee, revision 3 refs (sections 4a to 4c, 5a, 7, 9, 10a) at a87c021fd. Brief-b is `plans/units-land-in-batches-under-one-proof-brief-b.md`. Revision 2 folded critique round 1 (section 13) and the seat rulings F1 to F4.

## 0. What this page decides

The landing machine, not a person, records a trunk red with an owner. Wido ruled on 2026-09-17 05:52Z: option A. The entries live in the red register `plans/goals/trunk-red.json`, written by the machine through the ledger transaction, and every entry references the branch that carries its fix. Sections 4 to 4c, 5a and 7 hold the detail R1 builds; section 9 keeps the consequences of A; section 10a maps the builds.

## 1. Facts the design stands on

- Only the batch landing verb sees a trunk red. BA12a runs the fresh base diagnostic and "base red or no named unit produces the narrow trunk-red hold" (brief-b:55); BA12b owns ejection and reopening (brief-b:56); BA17 is this goal's capability unit (brief-b:57, :63); brief-b:125 moves the trunk-red record to BA17, called by BA12a.
- `Store.Update` takes the batch flock, runs the mutation, and writes once after it returns (`internal/landing/batch/store.go:62-85`, flock :87-101, durable atomic write :103-119). `NewStore` takes a root and a prober; the seams struct is unexported (store.go:17-31). A unit's claim carries machine, lineage, epoch and revisions, never a process id (record.go:9-15); units and claims are immutable once written (store.go:76-79).
- The ledger publish runs git through `exec.Command` with no timeout (`internal/goal/txn.go:74-99`); the 60-second deadline bounds only the compare-and-set retry loop (txn.go:47, :764-775). A journal entry is created before anything (txn.go:566-569), marked pushed before the push leaves (:711), and confirmed only after the trailer is seen on a refetch (:727-744). A replay of a confirmed opid is idempotent; a non-terminal entry belongs to recovery, never to a second publish (:541-565). Recovery classifies each entry by trailer presence and owner liveness (`recover.go:60-73`, `journal.go:556-584`) and rebuilds a dead owner's verb from its stored intent through a per-verb switch (recover.go:280-308). An opid is `<26-char ulid>-<machine>-<8 hex of sha256(lineage)>` (`file.go:2130-2133`); recovery refuses any other shape (recover.go:281-283). The journal lives at `<root>/artifacts/agents/goal-transactions/<opid>.json` (journal.go:104-112).
- A red group carries id, status, `NotRunReason`, log path and digest, and `Observed` tests with report, classname, name, status and reason (`internal/proofrun/test_result.go:22-28, :30-66`). A group may fail with no failing test: failed, invalid, unavailable, cancelled, runaway and dead need none (:119), and the reason then reads like `process exit 2 with no failing test in the evidence (collection complete)` (`test_build.go:995-1001`). A `passed` group was launched natively with no reuse; `reused` is a separate status (:111, :115).
- The 160-byte intent and 240-byte next-step caps bind the legacy backlog block only (`internal/goal/goal.go:264`, :361, :370); the goal-file parser has no such bound, and this goal's own intent is longer. An approved intent cannot be edited (`verbs.go:2658-2659`); editing or concluding another pair's claimed goal is a human act (verbs.go:2703-2704, :2025-2026); approve, set-priority, set-pin and steal are human acts (`approval.go:486-489`, `order.go:57-61`, verbs.go:3528-3531, :2847-2850), and a pin on a goal claimed elsewhere refuses (verbs.go:3578-3583). The tree validator refuses any file under `plans/goals/` that is not a goal, the archive or the root record (`validate.go:133-135`); the reconcile snapshot reads only `.md` files (`reconcile.go:186-188`).
- Health is a fixed list of role checks (`internal/steward/health.go:411-431`) rendered `role=status (reason; remedy: ...)` (:263-270); a role reads the ledger through `goal.Project` (`delivery.go:39-50`). The stop report renders every non-alive role (`internal/report/stoppresentation.go:1213-1230`).
- Nothing registers a batch capability in production; the tagged build registers all (`cmd/metasystem/landing_batch_capability_batchtest.go:5-19`); `TestBatchProductionRegistryEmpty` asserts emptiness only when run as the helper under `GO_WANT_BATCH_RUNTIME_INPUT_HELPER=1` (`landing_batch_verbs_test.go:45-60`), and production is empty today.
- `internal/goal` imports `internal/proofrun` (verbs.go:30); the batch package imports neither and keeps it so.

## 2. The seams into brief-b (DONE 2 and DONE 3)

DONE 2, the narrow hold, is BA12b's rule (brief-b:85), witness `TestTrunkRedHookAndNarrowHold`, mutation "hold every batch" (brief-b:56). DONE 3, the ejected unit's fix round on its own Next step, is BA12b's return-pending with attempt, group, log and failing names (brief-b:87) plus BA6b's tick that prepends the failure and fix round to Next (brief-b:46, :124).

- F1 (ruled): the witness that asserts the ejected unit's Next text belongs to BA12b. This page specifies nothing more.
- F2 (ruled): this page grants no exemption from the hold; BA8 stays the lock plus the start rule. One question goes to BA12b with the critic's argument: the base diagnostic runs only after a red candidate (brief-b:79, :83-85), so a fix batch that proves green never reaches the hold, and exempting a still-red fix would bypass the hold exactly when the fix did not prove. The entry keeps a `fixGoal` field because the owner line of section 7 and the health remedy name it; no hold or start rule reads it.
- F3 (ruled): U0 lands before BA12a is briefed; BA12a's diagnose calls U0's two functions (section 3).
- F4 (ruled): one rollout flip. U0 registers nothing; production stays BATCH_UNAVAILABLE (brief-b:63) and `TestBatchProductionRegistryEmpty` stays until the flip; section 10 says what U5 contributes.

## 3. The hook seam (ROM-HOOK-001)

The batch package defines the interface and a constructor option; an adapter in `cmd/metasystem` binds it. `Store` is a value, so the option returns a copy:

```go
// internal/landing/batch/trunkred.go
type Failure struct{ Report, Classname, Name, Reason string }   // NativeTestIdentity, test_result.go:22-28
type RedGroup struct{ ID, Status, NotRunReason, LogPath, LogDigest string; Failures []Failure }
type TrunkRed struct{ BatchID, AttemptID, BaseCommit, BaseTree string; Groups []RedGroup; Joiners []Claim; SeenAt time.Time }
type EntryRef struct{ ID, Group string }
type Green struct{ AttemptID, BaseCommit, BaseTree, Group string }
type OpenEntry struct{ ID, Group, OwnerMachine, FixGoal string; Holds []string; LastBaseCommit string }
type LedgerOwner interface {
    Record(opid string, red TrunkRed) ([]EntryRef, error)   // one ref per group; idempotent under one opid and by identity
    Clear(opid string, ref EntryRef, green Green) error
    Open() ([]OpenEntry, error)
}
func (s Store) WithLedgerOwner(o LedgerOwner) Store
func (s Store) LedgerOwner() LedgerOwner                       // UnboundLedgerOwner{} by default
```

`UnboundLedgerOwner` refuses every call with `TRUNK_RED_OWNER_UNBOUND`, registered with its first emitter (brief-b:3). `cmd/metasystem/landing_batch_owner_seam.go` (U0, untagged, ten lines) holds `var batchLedgerOwner = func(root string) (batch.LedgerOwner, error) { return batch.UnboundLedgerOwner{}, nil }`; BA9a's owner verb builds its store as `batch.NewStore(root, prober).WithLedgerOwner(owner)` from it. The tagged build assigns BA12a's file hook to that variable in an `init` under the `batchtest` tag; the flip assigns the production adapter (section 10). The variable is a binding, not a registration: with it alone every production verb still refuses BATCH_UNAVAILABLE (`landing_batch_verbs.go:29-33`). Rejected: an adapter inside the batch package (it would import `internal/goal`); a setter on the unexported seams struct (nothing in cmd/ could name it).

Inputs come from the diagnostic's result: attempt id, base commit and tree (test_result.go:173, :180-181); group id, status, log path, digest and reason (:31, :42, :58-59, :65); failures from the `Observed` tests whose status is not passed (:53); joiners from the batch record's units in join order (record.go:17-23).

### 3a. Phases (ROM-HOOK-002)

The publish never runs under the batch flock, and the hold is durable before it. BA12a's diagnose calls two U0 functions; the owner tick calls the second until it is done.

| Phase | Lock | Function | Writes | Durable when |
|---|---|---|---|---|
| 1 hold | batch flock, one `Store.Update` | `HoldTrunkRed(id, red, opid, at, actor)` | state `held-trunk-red`; `trunkRed{opid, opids:[opid], red, entries:null}`; a history line with attempt, groups and opid | the atomic replace returns (store.go:112-118) |
| 2 record | none | `EnsureTrunkRedRecorded(id)` step 1 | nothing in the batch root; the owner's `Record(opid, red)` publishes through the ledger's journal | the ledger's rules (txn.go:566-569, :711, :744) |
| 3 refs | batch flock, one `Store.Update` | `EnsureTrunkRedRecorded(id)` step 2 | `trunkRed.entries`, `recordedAt`, history line `trunk-red-recorded` | as phase 1 |

The opid is minted at phase 1 through `goal.Opid(ulid, machine, lineage)` (file.go:2130-2133) with the landing pair (`m1l`, `landing-m1l`) and stored opaque in the batch record, so every later phase and every recovery finds it there. Phase 2 reads the record without the flock (store.go:35-49); a stalled fetch or push stalls only the owner's tick; joins, withdrawals and other batches' updates proceed. A refusal in phase 2 leaves the batch held with empty entries and the history line `trunk-red-record refused: <reason>`; nothing is ejected or re-proved. `validateRecord` (store.go:127-136) learns one rule: `held-trunk-red` requires `trunkRed` with a non-empty opid. Rejected: publishing inside the `Update` mutation (the hold would be durable only after the publish, store.go:70-84, and a hung push would hold the one flock for every batch).

### 3b. Recovery (ROM-HOOK-003)

The batch package never reads the journal; the adapter does, under the stored opid, and the driver reacts to three outcomes of `Record`:

1. Refs returned: phase 3 writes them.
2. `TRUNK_RED_RECORD_PENDING`: the journal entry is `created` or `pushed` and belongs to another process, or recovery is running; the tick tries again. The adapter's `Record` first reads the entry (journal.go:217); on a non-terminal one it runs `goal.Recover` (recover.go:38) and re-reads. Recovery confirms a pushed entry whose trailer is on the tip (recover.go:75-93), completes a dead owner's created entry from its intent (recover.go:104-117, :142-223), and expires or abandons the rest. Confirmed replay then returns the refs without a second commit (txn.go:541-564).
3. `TRUNK_RED_RECORD_FAILED <outcome>: <evidence>`: the entry is terminal and not confirmed. The driver mints one new opid, appends it to `trunkRed.opids` with a history line naming the failed one, and records under it on the next tick. Every opid the batch ever used stays on the record.

Both shapes must add a `case "trunk-red-record"` to `requestForEntry` (recover.go:308) that rebuilds the mutation from the stored intent alone: `Args["red"]` carries the `TrunkRed` value as JSON, so recovery reads nothing outside the journal. The mutation merges by identity (section 4), so a completed dead-owner entry and a fresh publish converge on one entry with one sighting per attempt. Rejected: calling `Record` again as recovery (a second publish under a non-terminal opid refuses, txn.go:565); an opid of the page's old form (recovery refuses it, recover.go:281-283).

## 4. The entry and its identity (ROM-IDENTITY-001)

| Field | Content |
|---|---|
| `id` | the entry's name: equal to `identity` for the first entry with that identity; `<identity>-<n>` for the n-th entry with that identity, where n counts every earlier entry with it, open or closed, plus one; a red that returns after a close is `tr-fast-0a1b2c3d4e5f-2` |
| `identity` | `batch.TrunkRedID` of the group (`internal/landing/batch/trunkred.go:109-127`), computed once by the adapter and carried in the intent; the ledger never recomputes it |
| `group`, `status` | group id and the status of the newest sighting (test_result.go:31, :42) |
| `failures[]` | report, classname, name, status and the newest reason; the set of `failed` names is the identity when non-empty |
| `notRunReason` | the newest raw group reason (:65) |
| `sightings[]` | attempt, batch, base commit, base tree, log path, log digest, seen at, recording opid; one per `Record` call, in recording order |
| `owner` | machine, since, how (`joiner`, `taken`, `hand`), by (the human's name when how is `hand`, else empty); all four empty when nobody owns it |
| `fixGoal` | the goal the owner fixes it under; set by take-up, read by the owner line and the health remedy only |
| `fixBranch` | `{name, commit, state}` (Wido, 2026-09-17 05:52Z): the branch that carries the fix, the commit it pointed at when the entry was written, and `open` or `merged`; never a diff or a patch id; all three empty until a take-up names a branch |
| `holds[]` | batch ids held; `Record` appends, a close (machine or hand) empties |
| `opened`, `closed{at, attempt, baseCommit, how, opid, by, why}` | injected clock; `how` is `green` or `hand`; attempt and baseCommit are set for `green`, by and why for `hand` |

Identity, as landed in U0 (48cf3e7da). The identity text is the group id, a newline, and then either the sorted, deduplicated JSON lines `[report, classname, name]` of every failure whose status is `failed`, or, when no failure has that status, the group status alone. The id is `tr-<group id with / as ->-<12 hex of sha256 of that text>`. Tree, attempt, batch, log, time and every reason are never identity; the not-run reason carries a per-attempt dump path, so U0 dropped revision 2's digit-normalised reason and keys a nameless red on its status (a runaway and an invalid build are two identities; two `failed` groups with no named test are one). The same red seen again, on the same or a newer tree, is one more sighting and one more held batch on the open entry; two red groups in one attempt are two entries in one publish; the same test name in another classname is another entry. Rejected: identity by attempt (every sighting a new owner) or by group alone (two defects would share an owner).

Merge rule (m1e, 2026-09-17 04:55Z, agreed with the seat). A shared identity dedupes only against an OPEN entry: the mutation looks for the entry with the same `identity` and `closed` null. Found, it appends the sighting, appends the batch to `holds` when absent, and takes the newest status, not-run reason and failure reasons. Not found, it opens a new entry with a new owner, even when a closed entry has the same identity: the closed one stays in the register for identity, and the new one's `id` carries the next suffix. Two entries with one identity are never both open.

Atomicity. One landing owner records reds, so nothing races on phase 1. The publish races with every goal verb of every seat, and the ledger's compare-and-set serializes that (txn.go:713-716, :764-775): the mutation re-reads the tree on every rebuilt tip and merges by identity, so two landing roots recording the same red converge on one entry with two sightings.

### 4a. The register file (R1)

Path `plans/goals/trunk-red.json` (`goalsPrefix + "trunk-red.json"`, validate.go:64-66). The file is absent until the first record; an absent file is an empty register, and a register with zero entries is never written because entries are never removed. Top-level shape:

```json
{"schema": 1, "entries": [ ... ]}
```

Entries are ordered on disk by `opened` ascending, then `id` ascending, closed entries in place. The renderer `RenderTrunkRed(entries)` sorts, writes `json.MarshalIndent` with two-space indent and a trailing newline, and is the only writer. Every time field (`sightings[].seenAt`, `owner.since`, `opened`, `closed.at`) uses one layout, `2006-01-02T15:04:05Z`: 20 bytes, UTC, whole seconds, produced by `t.UTC().Truncate(time.Second).Format(time.RFC3339)`. Under one layout the strings compare as times, which the list order relies on.

The writer guarantees, and the parser refuses anything less: `id`, `identity`, `group` and `status` non-empty; `id` equal to `identity`, or to `identity` plus `-<n>` with n at least 2; every failure has report, classname and name; at least one sighting, each with attempt, base commit, seen at in the layout, and an opid that passes `validOpidShape` (file.go:2108); owner either all empty, or machine and since set with how in `joiner`, `taken`, `hand` and `by` non-empty exactly when how is `hand`; `fixBranch` valid (4b); no duplicate batch in `holds`; `opened` in the layout; `closed` null, or with `at` in the layout, `how` in `green`, `hand`, an opid of the shape above, attempt and base commit when green, by and why when hand. No string field contains CR or LF: the writer replaces both with a space in every string it copies from a proof record, and the parser refuses one anywhere. Two entries never share an `id`, and never share an `identity` while both are open.

### 4b. The parser and every reader it teaches (R1)

`ParseTrunkRed(data []byte) ([]TrunkRedEntry, []Problem)` in `internal/goal/trunkred.go` decodes the file, applies 4a's rules and names each problem `plans/goals/trunk-red.json: <rule>`. `TrunkRedBranch.Validate` (U2a) tightens: a named branch needs a non-empty commit, because the commit is what the reference means; an unnamed branch still needs commit and state empty. R1 adds `identity`, `owner.by`, `closed.by` and `closed.why` to U2a's structs.

Readers, against a87c021fd:

- `ParseTreeFiles` (validate.go:90-141): the switch at :113 gains `case rel == "trunk-red.json"` before the default at :133-135, filling `t.TrunkRed`; every other non-goal name still refuses with the default's text. `ValidateTree` (:187) adds no cross-file rule: `fixGoal` may name a pruned goal, and a dangling name must not fail the tree.
- `ReadCommitGoals` (validate.go:587-601) already lists every path under `plans/goals/`, so `ValidateCommit` (:657-676) and `loadTree` (verbs.go:304-314) see the register once the parser knows it.
- `CaptureSnapshot` (reconcile.go:176-200) keeps `.md` only (:187); it also keeps the exact name `trunk-red.json` under the goals root. Without that the base count at :263 (`len(snap.Files) != len(headFiles)`) is one short after the first record and the base is never stamped. `DiffAgainstBase` (:291-320) and `Refresh` (:322) then treat the register as any other ledger file: captured, counted, diffed, written into the checkout, never skipped by name.
- `goal list`: the text path already receives `p.Tree.TrunkRed` (U2a); the JSON map (goal.go:434-440) gains `"trunkRed": p.Tree.TrunkRed`, every entry open and closed in file order, `[]` when none. So `goal list --json` carries entries; `goal show` does not know them.
- Fixtures that build ledger trees from files: `seedLedger` (internal/goal/verbs_test.go:156), `syncedClaimedGoalFixture` and `syncedStoppedGoalFixture` (cmd/metasystem/goalsync_mutations_test.go:1313), `writeLedger` (cmd/metasystem/goal_test.go:111, internal/steward/openwork_test.go:9) write `.md` files only and stay as they are: an absent file is an empty register. R1 adds one helper in the internal/goal tests, `publishTrunkRed(t, root, entries)`, that publishes a rendered register through `Publish` the way `seedLedger` does, so CLI, steward and recovery tests read it through `Project`.
- Health reads `Project(...).Tree.TrunkRed` (section 7). The stop report (stoppresentation.go:1213-1230) renders every non-alive role and needs no change.

### 4c. The record mutation and its recovery (R1)

Transaction `trunk-red-record`, built by `trunkRedRecordRequest(r VerbRequest, args TrunkRedRecordArgs) PublishRequest` in `internal/goal/trunkred.go` on the pattern of `carryingRequest` (verbs.go:4688-4715): `Opid: r.opid()`, no targets, `Intent.Args["red"]` = the JSON of `args`, message `trunk-red record <batch> <group ids>`, `Validate: ValidateCommit`. `TrunkRedRecordArgs` is `{batch, attempt, baseCommit, baseTree, seenAt, ownerMachine, groups[]}`, each group `{identity, group, status, notRunReason, logPath, logDigest, failures[]{report, classname, name, status, reason}}`. The adapter fills it from `batch.TrunkRed` (trunkred.go:33-41), `identity` from `batch.TrunkRedID`, `ownerMachine` from `red.OwnerMachine()` (:101-106), `seenAt` in 4a's layout. The journal therefore holds everything the mutation needs.

`Mutate(tip)`: `loadTree(root, tip)`; when every group's identity already has an entry with a sighting under this opid, return `AlreadyApplied{}`; otherwise, per group in request order, apply section 4's merge rule. A new entry gets `owner{machine: ownerMachine, since: seenAt, how: joiner}` when `ownerMachine` is non-empty and an all-empty owner otherwise (`HoldTrunkRed` refuses a batch without units, trunkred_hold.go:30-32, so the empty case is a foreign writer's); an open entry with an all-empty owner takes the request's joiner the same way; an owned entry keeps its owner. Sightings and holds append; `opened` is the first sighting's `seenAt`. One `Change` at the register path with `RenderTrunkRed` of the whole list. Two groups in one attempt are two entries in one commit.

Refs: `TrunkRedRefs(tree *TreeGoals, opid string) []EntryRef` returns `{ID, Group}` for every entry with a sighting under `opid`, in file order. The adapter reads them from the confirmed tip (`PublishResult.Tip`), on a first publish and on the idempotent replay alike (txn.go:543-566 returns the tip that carries the trailer).

Recovery (3b): `requestForEntry` (recover.go:280) gains `case "trunk-red-record"` in the switch at :307: unmarshal `in.Args["red"]` into `TrunkRedRecordArgs`, refuse a malformed value with the file's "close it by hand" text, else return `trunkRedRecordRequest(r, args)`. The actor is `actorFromEntry` (:539), so `r.opid()` derives the entry's opid (:298) exactly when the adapter minted the request from the landing pair and the stored opid's ulid. The intent carries no `by`, so `completeFromIntent`'s human refusal (:149-152) never fires. Without the case the fallthrough at :523 rejects the entry and the tick mints a new opid: correct but blind, and the page requires the case.

Two more transactions share the file and the pattern. `trunk-red-own` and `trunk-red-close` are 5a's. `trunk-red-clear` (section 6, the adapter's `Clear`): `Intent.Args` `{"entry", "attempt", "baseCommit", "baseTree", "group", "branchMerged"}`, targets the entry id; `Mutate` returns `AlreadyApplied{}` on a closed entry whose `closed.opid` is this opid, refuses any other closed entry with `TRUNK_RED_CLOSED`, sets `closed{at, attempt, baseCommit, how: green, opid}`, empties `holds`, and sets `fixBranch.state` to `merged` when `branchMerged` is `true` (the adapter computes it with `git merge-base --is-ancestor <fixBranch.commit> <baseCommit>` in the endpoint root; false when the entry has no branch). `requestForEntry` gains `case "trunk-red-clear"` and `case "trunk-red-own"` (the agent form) the same way; the human forms carry `by`, and recovery refuses them with the rerun remedy it gives steal (recover.go:149-152, :170-177).

## 5. The owner (ROM-OWNER-001)

The owner is a machine nickname, never a live process: `owner.machine` is the machine of the first unit in join order, `Units[0].Claim.Machine` (record.go:10, :17-20), written at join and immutable (store.go:76-79). It is defined for a one-unit batch whose seat has exited, because the nickname outlives the process, and observable from the batch record and the entry. `how` starts as `joiner`. Reason: that seat queued the work the red holds.

Take-up. Any seat takes an entry with the shape's take-up verb (A: `goal trunk-red own --id <id> --goal <fixGoal>`, an agent act like a claim, verbs.go:929; B: claiming the pinned goal), which sets `owner{machine, since, how=taken}` and `fixGoal`; a person takes it with `--by` and `how=hand`. There is no automatic move on liveness: no record on trunk joins a joiner lineage to a process (record.go:9-15 has no pid; `identity.AliveRef` needs pid and a start token, `internal/identity/identity.go:21-27, :190-221`), and inventing that registry is out of scope. An entry whose owner never acts is visible on every machine (section 7) with its age; a person reassigns it. Rejected: a rota (a seat roster in the core); the landing owner (no brain, brief-b:9); "first live joiner" (not observable).

### 5a. The take-up and close verbs (R1)

One row in the goal verb table (`cmd/metasystem/main.go:505-560`): `{"trunk-red", "own or close a trunk-red register entry (own is an agent act; close, and own --by, are human acts)", runGoalTrunkRed}`. `runGoalTrunkRed` takes the first argument as the sub-verb, `own` or `close`, and hands the rest to `runSyncOnly("trunk-red "+sub, ...)` (goalsync_mutations.go:1174-1210) with `"id"` required; any other first argument prints the two forms and exits 2. `syncFlags` (goalsync_mutations.go:670-690) gains `goal`, `branch` and `to`; `--by` and `--why` exist.

`goal trunk-red own --id <id> --goal <fixGoal> [--branch <name>] [--by <human> [--to <machine>]]`

- Agent act without `--by`, checked in the verb as `Claim` checks the mirror (verbs.go:929-935): the owner becomes `{machine: r.Actor.Machine, since: r.stamp(), how: taken, by: ""}`. When the entry is owned by another machine the verb refuses `TRUNK_RED_OWNED_ELSEWHERE`. When it is owned by the actor's machine already (a joiner owner taking its own red, or a re-own), `how` becomes `taken`, `since` stays, `fixGoal` and `fixBranch` update.
- Human act with `--by`, checked in the verb as `Steal` checks it (verbs.go:2911-2914): `Actor.Human` non-empty, no proof, the same as steal today (section 14, question 3). The owner becomes `{machine: --to or the actor's machine, since: r.stamp(), how: hand, by: <human>}`, whatever the current owner. `--to` without `--by` is a command-edge error, exit 2.
- `--goal` is required and must name a live goal at the tip (`tree.Live[id] != nil`), else `TRUNK_RED_FIX_GOAL_UNKNOWN`. Any claim state is fine.
- `--branch <name>` is resolved at the command edge, never in the mutation: `git rev-parse --verify --quiet refs/heads/<name>` in the root, then `refs/remotes/<endpoint remote>/<name>`; neither found is exit 1 with `branch <name> is not in this checkout`. The commit rides in `Intent.Args["branchCommit"]`, so the mutation and recovery never touch refs; the entry gets `fixBranch{name, commit, open}`. Without `--branch` the field keeps its value.
- Unknown `--id` refuses `TRUNK_RED_UNKNOWN`; a closed entry refuses `TRUNK_RED_CLOSED`.
- Intent verb `trunk-red-own`, targets `[id]`, args `goal`, `branch`, `branchCommit`, `to`, `by`.

`goal trunk-red close --id <id> --by <human> --why <text>`

- Human act: without `--by` the verb refuses `TRUNK_RED_CLOSE_IS_HUMAN`; without `--why` the command edge exits 2. Sets `closed{at: r.stamp(), how: hand, opid: r.opid(), by, why}` and empties `holds`; attempt and base commit stay empty. The machine path is `Clear` (section 6, 4c). Unknown and closed ids refuse as above.
- Intent verb `trunk-red-close`, targets `[id]`, args `by`, `why`.

Both print through `printSyncResult` (goalsync_mutations.go:656): the outcome and tip on success, the refusal on stderr with exit 1. Register rows (`internal/refusal/register.go:50-52` is the pattern): `TRUNK_RED_UNKNOWN`, `TRUNK_RED_CLOSED` and `TRUNK_RED_FIX_GOAL_UNKNOWN` as `Question`; `TRUNK_RED_OWNED_ELSEWHERE` as `Agent` with Override `goal trunk-red own --by`, Commands 1; `TRUNK_RED_CLOSE_IS_HUMAN` as `Agent` with Override `goal trunk-red close --by --why`, Commands 1. Owner `internal/goal`, sites the emitting lines.

## 6. Clearing

An entry closes when a group with its id runs `passed` (natively launched, no reuse, test_result.go:111; never `reused`, :115) on a base commit that descends from the entry's newest sighting base commit. Two machine paths, both on the owner's tick: (a) BA12b's reopen on a new tree runs the base diagnostic first (brief-b:85); green closes every entry the batch holds through `Clear` before the reopen, red is one more sighting through `Record` and the batch stays held; (b) with no batch held, a green tip proof (BA11a) whose executed set includes the entry's group closes it. A green on the same base commit never clears, matching "reopening requires a new tree" (brief-b:56). The close carries the green attempt, its base commit and a clearing opid minted as in section 3. A person closes with the shape's close verb, `--by` and `--why`.

## 7. Visibility

As built in U2a (worktree implementer-e590dd086e323a810350e5c2, waiting for its lane proof). `TreeGoals` (validate.go:27-35 plus U2a's field) has `TrunkRed []TrunkRedEntry`, empty until 4b's parser fills it. `NextVerdict` (project.go:578-586 plus U2a's two fields) has `TrunkRedOwned` and `TrunkRedElsewhere`, the open entries split by `owner.machine == machine`; `SelectNext` is unchanged, and a tree with an empty field prints exactly what it printed before.

`goal next` (cmd/metasystem/goal.go, `nextSyncedWithProjector`): before the fenced lines, one line per owned entry, `trunk red <id>: <group> <report/classname/name of the first failure, or the status> on <base commit of the newest sighting>, holds <n> batches, since <owner.since>; fix it under <fixGoal or 'take it first'>`, followed by ` on branch <name>@<commit> (<state>)` when the entry has a branch. After the selection line, one line per other entry: `trunk red <id> owned by <machine> since <since>`. `goal list` (goal_list.go, `goalListSummary`): the count `trunk-red=<n>` before `tip=`, only when n is above zero; then, after the notices and before the goal rows, one row per open entry, oldest `opened` first, then id: `! trunk red <id> <group> owned by <machine or nobody> since <t>; holds <n> batches`. The rows count toward the summary's byte cap and its footer. Closed entries are neither counted nor printed. An entry with an empty owner prints `owned by nobody` on both commands.

Health (R1). A role `trunk-red` joins the constants (health.go:45-64), `healthRoleOrder` (:66-85; the health tests compare the verdict count with it) and the check list (:411-431), after `claimed-goal-delivery`. `checkTrunkRed(repoRoot, now)` in `internal/steward/trunkred.go` follows `checkClaimedGoalDelivery` (delivery.go:39-50): bootstrap ledger, alive `the bootstrap ledger has no trunk-red register`; then `goal.Project(endpoint, false, now)` and the entries of `Tree.TrunkRed` with `closed` null, read through the parser and never through `goal.Next`, whose split puts an empty-owner entry on the owned side when the machine is empty. Alive: `no open trunk red`, or `<n> open, all owned; oldest <age>`. Dead: an open entry with an empty owner machine, reason naming the ids, remedy `metasystem goal trunk-red own --id <id> --goal <goal>`; or a file under `<batch root>/artifacts/agents/landing-batches/` whose state is `held-trunk-red` with empty `trunkRed.entries` and a `trunk-red-hold` history line (trunkred_hold.go:44-45) older than `goal.DefaultPublishDeadline` (txn.go:47) at the role's `now`, reason with batch id and opid, remedy `metasystem landing batch tick`, then the take-up verb. The batch root comes from `config.ResolveBatchLanding` (resolve.go:37) with the seat's `metasystem.conf`, as goalsync_mutations.go:43 calls it; an unset `landing.batch-root` skips the batch half. The role decodes each batch file into a local struct with those fields and imports nothing from the batch package. Unknown: the ledger unreadable, or the batch root set but unreadable. Age alone is never a violation.

## 8. DONE 5: the live evidence

After the flip, the next base red is machine-recorded when the seat reads all of the following with no act of its own first:

1. `<landing root>/artifacts/agents/landing-batches/<id>.json`: state `held-trunk-red`, `trunkRed.opid` = O, the hold line's actor `m1l+landing-m1l`, `trunkRed.entries` non-empty.
2. `<landing root>/artifacts/agents/goal-transactions/O.json` (journal.go:104-112): machine `m1l`, lineage `landing-m1l`, intent verb `trunk-red-record`, phase terminal, outcome confirmed, evidence `opid verified on <tip>` (txn.go:744). O's last eight characters equal the first eight hex of sha256 of `landing-m1l` (file.go:2130-2133); the seat recomputes them. This file is the positive provenance: it exists only where the transaction ran.
3. On origin after `goal fetch`, the entry carries O (A: `sightings[0].opid`; B: the opening history line's opid, actor `m1l+landing-m1l`), and `git log -1 --format=%B <commit>` shows the `Goal-Transaction: O` trailer (txn.go:286).
4. `<landing root>/artifacts/agents/proof-runs/attempts/<attempt>.json` (`internal/proofrun/attempt.go:233-235`): goal Gk, purpose diagnostic, the entry's group red.
5. No `--by` history line on the entry before the take-up. The seat's receipt quotes 1 to 3.

## 9. The shape: option A, ruled

Wido ruled A on 2026-09-17 05:52Z. B, a goal opened in the landing machine's name under an amended R-93, was not chosen. The consequences of A, kept for the record and now built by R1:

| Consequence | A: the red register |
|---|---|
| Storage | One file `plans/goals/trunk-red.json` (4a), written only through the transaction engine; no command form for `record`. |
| Readers it must teach (ROM-OPTION-A-001) | The list of 4b: `ParseTreeFiles`, `CaptureSnapshot` and the base count, `goal list --json`, the test fixtures, health. |
| Entry fields | The JSON of section 4 and 4a. |
| Rank (ROM-RANK-001) | A display line; `SelectNext` (project.go:606-614) is unchanged, so "ranked ahead" is print order; this page adds no selection rule. |
| Human acts folded into one machine act (ROM-OPTION-B-002) | None; R-93 untouched. |
| Owner and take-up | `owner` field; `goal trunk-red own` is an agent act, `--by` its human form (5a). |
| Close | `Clear` by the machine (section 6, 4c); `goal trunk-red close --by --why` by a person (5a). |
| DONE 5 provenance on the entry (ROM-LIVE-EVIDENCE-001) | `sightings[].opid` = O; `owner.how` is `joiner` and `owner.by` empty before any take-up. |
| Recovery | `case "trunk-red-record"`, `"trunk-red-own"`, `"trunk-red-clear"` in recover.go:307 (4c). |
| Risks | A second record kind in the ledger tree; closed entries stay for identity, so a sweep is later work. |

## 10. Units and witnesses (common)

Injected clocks and fake proof verdicts only; at most 300 changed lines per unit including tests; landing order; the first unit is U0. Builds are Codex sol in a worktree, reads Opus.

| Unit | Files | Rule | Witness | Mutation | After |
|---|---|---|---|---|---|
| U0 seam, hold, identity (about 280 lines) | `internal/landing/batch/trunkred.go`, `record.go`, `store.go`, `trunkred_test.go`, `cmd/metasystem/landing_batch_owner_seam.go` | Section 3: the types, `LedgerOwner`, `WithLedgerOwner`, the unbound default and its refusal, `HoldTrunkRed`, `EnsureTrunkRedRecorded`, the record field and validation rule, `TrunkRedID`. Registers nothing. | `TestTrunkRedHoldIsDurableBeforeAnyLedgerCall`: a fake owner, from inside `Record`, reads the batch file and finds `held-trunk-red` with the opid and empty entries, and runs `Store.Update` on a second batch, which returns. `TestTrunkRedRefusalLeavesHeldWithEmptyEntries`: a refusal leaves the hold and its history line; the next call with the fake accepting fills entries once; a third call makes no owner call. `TestTrunkRedFailedOutcomeMintsOneNewOpid`: a failed error appends one opid and records under it next tick; a pending error changes nothing. `TestTrunkRedUnboundOwnerRefusesByName`. `TestTrunkRedIdentity`: same names on a new tree, one id; two groups, two; classname differs, two; reason changed, one; empty set with statuses runaway and invalid, two; empty set, same status, reasons differing only in digits, one. | Call `Record` inside the `Update` mutation; write entries before `Record` returns; swallow the refusal; mint on pending; identity from tree or attempt; drop classname; include reason; ignore status when no names. | none |
| U2 common reader (about 290 lines) | `internal/goal/trunkred.go`, `validate.go`, `project.go`, `cmd/metasystem/goal.go`, `goal_list.go`, `internal/steward/trunkred.go`, `health.go`, tests | Section 7: `TrunkRedEntry`, the `TreeGoals` field (empty until the shape's parser fills it), the two frontier lists, the two `goal next` lines, the list count and lines, the `trunk-red` role. `SelectNext` unchanged. | `TestNextPrintsOwnedTrunkRedBeforeFencedAndOthersAfterSelection` on a tree whose `TrunkRed` field the test sets: owned line first, other machine's line last, selection identical with and without entries. `TestGoalListCountsOpenTrunkReds`. `TestTrunkRedRoleVerdicts`: none alive; owned alive with age; empty owner dead; held batch with empty entries past the deadline dead naming batch and opid; inside the deadline alive; unreadable unknown. | Print after the selection; change `SelectNext`; alive always; age as a violation; ignore the batch root. | U0 |
| U3 reconcile and clear (about 270 lines) | `internal/landing/batch/trunkred_clear.go`, `owner.go`, `reopen.go`, `prove.go`, tests | Section 3b's tick loop over held batches with empty entries; section 6's two clear paths; red on the new tree records a sighting. | `TestTickRecordsHeldBatchesWithoutEntries` (fake owner counts calls; pending waits, failed mints once, confirmed replay fills refs with no second publish). `TestReopenClearsOnGreenDescendantOnly` with fake verdicts: green descendant clears then reopens; green same commit stays held with no clear; red descendant adds a sighting and stays held. `TestGreenTipProofClearsHeldlessEntryOnlyWhenExecuted`: a `passed` group clears; `reused` does not; a group outside the executed set does not. | Reopen without clearing; clear on the same commit; clear on red; accept `reused`; skip the tick loop. | U0, BA11a, BA12b |
| U5 = BA17 production adapter (about 250 lines; brief-b's 110 cover its capability part) | `cmd/metasystem/landing_batch_trunkred.go`, `landing_batch_trunkred_test.go`, `landing_batch_capability.go` | `ledgerTrunkRedOwner{endpoint, actor, clock}` implements `batch.LedgerOwner` over the chosen shape's verbs: `Record` reads the journal entry, runs `goal.Recover` on a non-terminal one, returns pending, failed, or the refs (section 3b). `newLedgerTrunkRedOwner(root)` and `isLedgerTrunkRedOwner(owner)` exist for the flip. Registers nothing. | `TestLedgerTrunkRedOwnerRecordsIdempotently` on a local-mode ledger (txn.go:67, :351-365): one publish with trailer O; a replay returns the refs and publishes nothing; a `pushed` entry whose trailer is on the tip returns refs after recovery with no new commit; a `created` entry of a dead owner completes from intent; an `abandoned` entry returns failed with no publish; an opid outside the ledger's form refuses. | Skip the journal read (the pushed case then refuses, txn.go:565); publish under a fresh opid every call; accept the old opid form. | the shape's unit |

The flip is brief-b's rollout commit after BA15 (brief-b:63): one untagged file whose one `init` registers every required marker and assigns `batchLedgerOwner = newLedgerTrunkRedOwner`; `TestBatchProductionRegistryEmpty` becomes `TestBatchProductionRegistryComplete`, asserting that the registered set equals `requiredBatchCapabilities` (`landing_batch_capability.go:31-41`) and that `isLedgerTrunkRedOwner(batchLedgerOwner(root))` holds, so the marker cannot exist without the binding. U5 contributes the adapter, its constructor, the test helper and the witness above; the flip contributes the `init` and the replaced test. Until the flip the tagged build binds BA12a's file hook (`landing_batch_capability_batchtest.go:11`) and production refuses BATCH_UNAVAILABLE.

### 10a. Build grouping after the process reset of 2026-09-17

The 300-line cap in the table above no longer applies. A build is one design section or a few closely tied sections, sized by what they need; the counts are estimates, not caps (design-principles.md, Implementation Slicing). Builds are Codex sol in a worktree; one independent read per build.

| Build | State | Page sections | Estimate | After |
|---|---|---|---|---|
| U0 seam, hold, record | landed: seam 48cf3e7da, hold 7d9c7fdbb, recording c4afe7394 | 3, 3a, 4 identity | 3 commits | none |
| U2a common reader | built, 337 lines with its tests, queued for m1e's lane proof | 7 without the health role: the `TreeGoals` field, the `NextVerdict` split, the two `goal next` lines, the list count and rows | 300 | U0 |
| R1 the red register | not started | 4a file; 4b parser and readers; 4c the three record-side transactions with their recovery cases; 5a the two verbs; 7 health role; 10 U5 adapter (`Record`, `Clear`, `Open`, constructor, `isLedgerTrunkRedOwner`) | about 1,600 | U2a lands |
| R2 clearing | not started | 3b owner tick over held batches with empty entries; 6 both clear paths calling `Clear`; F-4, F-5, F-8 below | about 450 | R1, BA11a, BA12b |

R1 is one unit: every piece reads or writes the register, and the adapter is the only production caller of the transactions. If the builder finds it does not hold together, the seam is the adapter file with its local-ledger witness (about 400 lines) as R1b; the seat does not pre-split. R2 is not a leaf of R1: BA11a and BA12b (m1e's batches B and C) wait between them, so the two stay separate builds.

U5 in R1. `cmd/metasystem/landing_batch_trunkred.go`: `ledgerTrunkRedOwner{endpoint goal.Endpoint, actor goal.Actor, now func() time.Time}` implements `batch.LedgerOwner` (trunkred.go:60-64). `Record(opid, red)`: refuse unless `opid == goal.Opid(opid[:26], actor.Machine, actor.Lineage)` (file.go:2132); `goal.ReadEntry(root, opid)` (journal.go:217); no entry, or a terminal confirmed one: call the transaction (`Publish` creates or replays, txn.go:543-569) and return `TrunkRedRefs` at the result tip; a terminal entry with another outcome: return `*batch.TrunkRedRecordFailed{outcome, evidence}`; a non-terminal entry: run `goal.Recover(endpoint)` (recover.go:38) once, re-read, and map the same way, returning `batch.ErrTrunkRedRecordPending` while it stays non-terminal; a publish that ends with the journal pushed and the postcondition unresolved (txn.go:727-740) is pending too. `Clear(opid, ref, green)` runs `trunk-red-clear` under the same journal logic. `Open()` projects the ledger and maps the open entries to `batch.OpenEntry{ID, Group, OwnerMachine, FixGoal, Holds, LastBaseCommit}`. `newLedgerTrunkRedOwner(root, machine, lineage string) (batch.LedgerOwner, error)` resolves the endpoint; brief-b's owner verb supplies the landing pair at the flip, the same pair BA12a mints the hold opid with. `isLedgerTrunkRedOwner(owner) bool` is a type assertion. Registers nothing; the capability flip stays brief-b's rollout commit, as the paragraph above says. The witness is section 10's U5 row, on a local-mode ledger.

Carried points, placed:

- unit 3 F-4: `EnsureTrunkRedRecorded`'s second update returns nil when the record moved on (trunkred_record.go:39-42), and the refs the ledger returned are dropped, so the caller cannot tell recorded from skipped. R2: the function returns a typed outcome (`recorded`, `already`, `moved-on`) and the tick logs it.
- unit 3 F-7: `store.Update` writes the file even when the mutation changed nothing (store.go:69-101, :120). Out of scope here: the store is brief-b's; R2's tick avoids the no-op update through the F-4 change.
- hold F-5: `HoldTrunkRed` replaces the hold outright (trunkred_hold.go:39), so a re-hold after a reopen loses the earlier opids and entries. R2: a re-hold appends the new opid to `opids`, sets `opid`, clears `entries` and keeps the earlier `trunk-red-recorded` history line; the ledger merges the new record into the same open entry, or opens a suffixed one if it was closed meanwhile.
- hold F-8: a hold written but not acknowledged (a crash between the atomic replace and the return) makes the retry refuse from state `held-trunk-red`, and the caller must load and compare the opid. R2: `HoldTrunkRed` returns nil when the record is already `held-trunk-red` under the same opid, so the retry is idempotent and BA12a needs no rule.

## 11. Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| The trunk-red record on the ledger | The seat, by hand: a flake row and a hand-opened goal | U5's adapter, called through U0 by BA12a, publishing through the ledger transaction | `metasystem/docs/flake-registry.md` is the hand protocol; `metasystem/internal/goal/txn.go` is the publish it now runs through |
| The `held-trunk-red` transition and its opid | BA12a's diagnose writing the state itself (brief-b:55) | U0's `HoldTrunkRed`, called by BA12a | `metasystem/internal/landing/batch/record.go` owns the transition; `metasystem/internal/landing/batch/store.go` the write |
| Choosing the first owner | The seat, by hand (m1e sent the 2026-09-16 reds to Codex) | U0's `HoldTrunkRed`, from the first joiner's claim | `metasystem/internal/landing/batch/record.go` holds the claim it reads |
| Taking or moving ownership | A seat's word in a lane report | The shape's take-up verb, an agent act | `metasystem/internal/goal/verbs.go` owns today's take-up act, `Claim` |
| Clearing a red after a green | The seat editing the flake row | U3 on the reopen and prove paths through `Clear` | `metasystem/internal/proofrun/test_result.go` is the green evidence it reads |
| Recording a held batch that has no entry yet | The seat, noticing a held batch by hand | U3 in the owner tick, through the ledger's recovery in U5 | `metasystem/internal/goal/recover.go` is the recovery it runs |
| Showing the red to seats and stewards | The seat's lane report | U2 in `goal next`, `goal list` and the `trunk-red` health role | `metasystem/cmd/metasystem/goal.go` and `metasystem/internal/steward/health.go` are the surfaces |

## 12. Out of scope

The brief-b units and their eject, hold and reopen rules beyond section 2; any exemption from the hold (BA12b's question); the flake grading and alert-episode work; fixture reaping; a timeout on the ledger's git transport (shared by every goal verb); a lineage-to-process registry; R-93 amendment text beyond section 9; the hand lane, still the fallback (brief-b:63).

## 13. Fold of critique r1

- MOVED-EFFECTS-001: closed; section 11.
- ROM-HOOK-001: closed; constructor option plus a cmd/ variable, section 3.
- ROM-HOOK-002: closed; three phases, no publish under the flock, section 3a.
- ROM-HOOK-003: closed; opid in the ledger's form stored at hold, adapter-driven recovery, a `requestForEntry` case, section 3b.
- ROM-IDENTITY-001: closed; status plus normalized reason when no test is named, classname kept, reason never identity, section 4.
- ROM-OWNER-001: closed; the owner is the first joiner's machine nickname, defined and observable, no liveness move, section 5.
- ROM-RANK-001: moved to the option table as a consequence of A.
- ROM-OPTION-A-001: moved to the option table, with the readers A must teach.
- ROM-OPTION-B-001: moved to the option table, corrected: the 160-byte cap binds the legacy block only (goal.go:264); the immutability and holder rules stand.
- ROM-OPTION-B-002: moved to the option table; seven human acts listed.
- ROM-BRIEF-SEAM-001: closed; the exemption is deleted and the question handed to BA12b, section 2.
- ROM-PROOF-001: closed; section 10 covers empty name sets, classname, changed reason, reused groups, a pending transport and the created and pushed journal cases.
- ROM-LIVE-EVIDENCE-001: closed by the landing root's transaction journal, section 8; the per-shape field is in the option table.
- ROM-SEAM-NM-001: closed; the line-14 wording is corrected in section 1.

## 14. Open questions for the seat

1. Closed. Wido ruled A on 2026-09-17 05:52Z, with the fix-branch reference on every entry; sections 0, 4 and 9 record it.
2. BA12b: whether any batch is ever exempt from the narrow hold, with the argument of section 2 (F2).
3. Human forms of the new verbs: 5a checks `own --by` and `close --by` the way `steal` does today (a named human, no enrolled-terminal proof), not the way `approve` does (a proof classified at the command edge). Recommendation: keep steal's form; both acts move or close a machine record and fold no human act into a machine one, so R-93 stays untouched. R1 builds that. If Wido wants the proof, it is one `requireHuman` row per verb (verbs.go:257-275) and a `syncReqWithProof` call at the command edge, nothing else.
