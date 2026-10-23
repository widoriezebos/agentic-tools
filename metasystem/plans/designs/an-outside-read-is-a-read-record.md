# An outside read is a read record

- Kind: design
- Id: 01M6AN0VTS1DEREADRECRD0001
- Status: accepted
- Goals: an-outside-read-is-a-read-record
- Critique: accepted at the stop by the steward in Wido's name, 2026-10-23: Astra rounds 1-3 found 5 → 2 → 1 material; the last (ORR-7, rare) folded under D0 as a build obligation (section "Round 3 fold")

Tier 2, revision 4 (ACCEPTED, 2026-10-23; revision 3 plus the round 3 fold). Design lane (Opus 5.5) for Wido's steward, 2026-10-23. Revisions 1 and 2 are kept beside this page (`…-revision-1.md`, `…-revision-2.md`). Code read at origin/main 068fa764f5 (`git archive` into the designer's scratchpad; nothing in a checkout was changed); `cmd/metasystem`, `internal/goal/branch` and `internal/launch` have no diff from 1e3952a54e, the revision Astra read in round 2. Paths are relative to `metasystem/`. Starting material: the goal page (Intent, Next step) and `analysis/lane-hand-in-2026-10-22.md` (Path B). Rulings relied on: R-104-m1e (`memory/rulings.md:163`, artificial clocks and injected seams), R-141-m1e (`:200`, cooperative same-user controls), R-142-m1e and R-143-m1e (`:201-202`, no HAL9000), R-145-m1e (`:204`, all work through the machinery). Wido's rule of 2026-10-22 08:45 applies: every case is COMMON (handled here) or RARE (the generic rule D0, one test). Wido's answers (decision 29): Q1 a record must end with `Verdict: LAND`; Q2 any claim holder may record an outside read; Q3 the lane's line keeps `critic`. The steward accepted on 2026-10-23: stop line 150; the eleven test setups gain a `Verdict: LAND` line (no extra request flag).

## Problem

1. **Goal 3 landed with its reads waived.** Its box at the commit before `goal done` (7869dedfe7^) is `reviewRoundLimit=0` (`plans/goals/a-proved-main-deploys-itself.md:14`). Zero review rounds waive every read (`internal/goal/landgate.go:663-665`), and the hand-in line says `Read: "waived"` (`cmd/metasystem/landing_plain.go:288-290`). Main records "waived", not the Opus reads that were made.
2. **The record source exists; no verb passes it.** An attestation already has a `reader-record` source: a repository-relative file under `metasystem/records/misc` (`internal/goal/branch/attest.go:278-281`) that names the unit commit and its unit digest (`:339-355`). The read commit carries the file (`:1055-1070`, `:1118-1120`), and every later status validates it again from Git (`:624-631`). Only tests set `CommitReadRequest.ReaderRecord`. `goal branch read` has `--unit-read` and no `--reader-record` (`cmd/metasystem/goal_branch.go:247-260`); `RunBranchRead`'s only non-critic branch is the unit read (`internal/goal/branch/read.go:715-752`).
3. **The machinery's own read cannot start from the pane.** A critic dispatch needs `StopCapability.ClaimEpoch > 0` (`internal/dispatch/stop.go:87-88`); a person's reservation has 0 (`plans/goals/a-persons-channel-carries-only-acts-and-deliveries.md:18`). The dispatch is reached only after the unit-read branch (`read.go:753` on). A record branch beside the unit read starts no critic.
4. **A commit read moves the checkout it runs in.** Installing a read commit checks out the new tip and attaches HEAD to `goal/G` (`internal/goal/branch/commit.go:572-585`). Its preflight refuses only an unstaged overwrite or `goal/G` checked out in another worktree (`:510-545`). `work review --commit` runs in the goal worktree when one exists, else in this checkout (`cmd/metasystem/intent_delivery.go:1301`, `:1354-1368`). From the pane in the m1e checkout on main, a read would switch the seat's checkout to `goal/G`.
5. **The publication check refuses an outside read.** With no machinery work behind the commit, `commitReviewChecked` binds publication to the worktree (`cmd/metasystem/intent_unit_review.go:947-1118`). An uncommitted non-excluded path refuses (`:1026-1042`); `records/misc` is `ClassReadProse`, not excluded (`internal/goal/branch/range.go:247-249`), so the untracked record itself refuses. HEAD not descending from the unit, or a later commit touching the unit's paths, retires the read for good (`:1043-1090`). On a hand-built goal U2 often touches U1's files.
6. **A record carries no verdict, and the commit owner reads it twice.** The attestation's verdict is fixed `LAND` (`attest.go:950`). The record check is only that the text names the commit and the digest (`:351-354`). That check, and the hash that goes into the attestation, run in `directSource` (`:999`), after the gate. The bytes committed come from a third read: `git add` of the worktree file (`:882`, called at `:1070`). Real Opus reads end `Verdict: LAND` (`briefs/a-proved-main-deploys-itself-U1a-read.md:72`, the last line) or carry `Verdict: CORRECT` (`briefs/a-proved-main-deploys-itself-U2a-read.md:63`, four lines before the end). Without a check, a CORRECT read records as LAND. A check made only before the gate does not bind what is hashed (Astra ORR-1).
7. **A refused read commit deletes what sat at its attestation path.** The commit owner writes the attestation file over its path in the worktree (`attest.go:1105-1112`). On a staging or install refusal its rollback restores the index and removes that path (`:1090-1103`, `:1121-1125`); it does not restore earlier bytes. An attestation path is `ClassRead` (`range.go:238-240`), so today the publication check's dirty refusal stops an untracked file there first. Skipping that check (D3) removes the stop (Astra ORR-3). A check before the gate does not close it: the gate runs after it (`read.go:246-268`) and nothing checks again before the write (Astra round 2, ORR-3 reopened).
9. **The publication arm also owns the worktree.** Besides the comparison, the `check == nil` arm reserves the worktree (`ReserveMutation`, `intent_unit_review.go:949-953`), runs the read inside the tree transition lock (`:1093-1102`) and publishes inside it (`:1105-1117`). The reservation refuses while another live run owns the tree (`internal/launch/tree_reservation.go:128-143`, `:496-526`); the accepted design asks for it at every tree-writing entry (`plans/designs/review-chain-stops-and-records.md:119`). The branch read lock is per commit (`read.go:634-641`), so two reads of different units are not serialized by it. Revision 2's D3 skipped the reservation with the comparison (Astra ORR-6).
10. **A plain error is hidden.** `commitReviewChecked` shows an `OpError`'s message (`intent_unit_review.go:1257-1259`); any other error becomes "the review couldn't be requested" with the cause only under `--verbose` (`:1261-1262`). `directSource`'s record errors are plain (`attest.go:340`, `:353`) (Astra ORR-N4).
8. **A hand-built unit has no row on the hand-in line.** `handIn` builds its unit rows by looping over the goal's named machinery work (`landing_plain.go:282-301`). `goalWork` lists the unit runner's named entries (`cmd/metasystem/intent_selection.go:31-54`); without a goal worktree it returns none (`:46`), and a store with no entries returns none (`internal/launch/unit_named.go:553-556`). The `reader-record` to `critic` mapping (`landing_plain.go:296`) is inside that loop, so a hand-built goal lands with no unit rows at all (Astra ORR-5). Revision 1 said no change in `landing_plain.go`; that was false.

## Premises checked

- **P1 (read): a pane reservation passes the claim check.** `RunBranchRead` checks access first (`read.go:606`) and `CommitRead` again (`attest.go:910`), both through `goalBranchClaimCheckWith` (`goal_branch.go:340-345`, `:529-557`). (a) `lease.RequireHolder`: a person's process returns holder without gating (`internal/lease/verbs.go:450-452`); delegate and supervision classes return no error (`:453-454`); only a main that is not the holder is refused (`:475-477`). (b) The lease read is the main checkout's: a linked goal worktree maps to it (`goal_branch.go:629-643`). (c) `Claimed.Machine` must equal this machine and `Claimed.Lineage` the holder's announced lineage (`:548`). A person's claim records `METASYSTEM_OWNER_LINEAGE` when set, else `terminal-<id>-<generation>` (`cmd/metasystem/goalsync_mutations.go:553`, `:677-721`). Wido's pane reservations carry `lineage=main-1790454088-93948-21671b`, the seat's main id (`plans/goals/a-persons-channel-carries-only-acts-and-deliveries.md:17`). So the check passes while the seat's holder still announces that lineage. Hypothesis: the pane's metasystem process classifies as a person (process ancestry not traced). The rehearsal (Tests, R1) settles it.
- **P2 (read): what makes the read count for the lane.** The lane route's only read gate is the seat's `work land` (`handLandingSubject`, `intent_delivery.go:2462-2518`). A unit is clean when its status is `read clean` (`goalProgress`, `:2380-2383`). Status sets `read clean` for a `Goal-Read` commit naming the unit whose attestation validates (`internal/goal/branch/status.go:122-149`). For this source: the record file at the branch tip, its sha256 equal to the recorded one, the text naming the commit and the unit digest (`attest.go:624-631`). `work land` validates each read unit again and keeps its source kind (`intent_delivery.go:449-470`). The lane itself (`internal/landing/plain`) reads no unit's read. `landing observe --attested` refuses this source (`internal/landing/attested.go:47-49`); that route is not the lane's (`cmd/metasystem/landing_verbs.go:32`).
- **P3 (read): the gate still runs.** The read commit needs a passing static gate on the unit's tree (`attest.go:917-918`, `:936-938`), run in a detached tree of the unit (`read.go:241-265`), never in the checkout.
- **P4 (read): an empty value passes the public parser.** `--reader-record=` keeps the empty value (`cmd/metasystem/intent.go:382-415`, `:420-454`), and `has` tests presence only (`:347`). The `--units` row already refuses an empty value (`intent_delivery.go:1283-1285`); the owner has a nonempty option type (`goal_branch.go:81-95`).

## Decisions

**D0. The generic rule.** Every refusal on this path leaves `goal/G`, both checkouts, the reader's record and the branch read record as they were (a passing gate run may stay saved, `read.go:262-268`). It is an `operationRefusal` (`internal/goal/branch/commit.go:42-44`), so its one plain line and next command show without `--verbose` (Problem 10). All record checks run before anything is written. There is no bespoke handling for a rare case.

**D1. The flag.** `metasystem work review --commit SHA --goal G --reader-record PATH`. PATH is repository-relative, under `metasystem/records/misc/`. A row beside `--units` in the flag table (`intent_delivery.go:101-104`); the commit form's usage (`intent_review_help.go:11`), flags and option note (`:64-65`). `reviewCommit` (`intent_delivery.go:1282-1348`) refuses, code 2: an empty PATH, as the `--units` row does (`:1283-1285`); and `--reader-record` with `--brief`, `--model`, `--retry`, `--units` or `--dispositions`. Otherwise it forwards the flag. `goal branch read` gains `--reader-record` as a `goalBranchReadOption` (nonempty, once; `goal_branch.go:81-95`, declared beside `:251`), refuses it with `--unit-read`, `--brief`, `--runtime`, `--model`, `--collect`, `--join`, `--retry` or `--units`, and passes it as `BranchReadRequest.ReaderRecord` (`:336-339`; struct `read.go:34-65`).

**D2. Only in a goal worktree.** With `--reader-record`, `reviewCommit` asks `inv.goalWorktree(G)` (`cmd/metasystem/intent_work.go:1292-1328`). This checkout counts when it is on `goal/G`. When none holds `goal/G`, it refuses: "no worktree has the goal/G branch checked out; nothing was recorded". When the local `goal/G` exists, the next command is the existing `git worktree add` (`:1322-1324`). When it does not, the next command is `git -C ROOT fetch <remote> refs/heads/goal/G:refs/heads/goal/G`, with the add as the detail line, so the worktree starts at origin's tip, not at main. COMMON: the pane sits in the m1e checkout on main (Problem 4).

**D3. The worktree stays owned; only the comparison is skipped.** In `commitReviewChecked` a new arm after `alreadyPublished` and before `check == nil` (`intent_unit_review.go:947`), taken only when `check == nil` and `args` holds `--reader-record`. It keeps the owner of the `check == nil` arm: `ReserveMutation(root, goalID, "read-publication")` with its deferred release (`:949-953`); `branchRead` runs `owners.branchRead` inside `runner.MutationSection` (as `:1093-1102`, without `check`); `wait` publishes inside `MutationSection` with `CommandWait` (as `:1105-1117`, without `check`). It drops only the comparison and its `read-publication` journal (`:954-1092`). Another live owner of the worktree gives the existing waiting answer (`treeFailure`, `cmd/metasystem/intent_tree.go:101-104`) before the gate or any write (Astra ORR-6). D1 makes `--reader-record` in `args` imply a nonempty PATH, so the arm never covers a machinery read. A commit with machinery work keeps its own check (`:890-911`). Why skip the comparison: it binds a machinery read to the worktree it read. An outside read's subject is the commit, bound by the id and digest in the record and re-checked from Git at every status (`attest.go:624-631`). Every guard of `CommitRead` stays (Brief: guard list). COMMON: the untracked record, and U2 touching U1's paths (Problem 5).

**D4. The record branch in `RunBranchRead`.** It reuses the unit-read branch (`read.go:715-752`). The inherited-change refusal (`:627-629`) and the "already started with a critic" refusal (`:694-696`) apply to a record too. `already-collected` (`:700-714`) comes first, so a repeat changes nothing. The covered-units refusal (`:716-718`) applies. Then D5, then `resolveReadGate` (`:723-727`). Each changed test path gets the reader word "reviewed by reader record PATH" (as `:732-735`). Then `commit(CommitReadRequest{…, ReaderRecord: PATH})` without `UnitRead`. The record is saved and the state is `collected`. No critic is dispatched.

**D5. The record names the build and ends with LAND, checked before the gate.** In D4's branch, in this order, all before `resolveReadGate`:
1. `safeReaderRecord(PATH)` (`attest.go:278-281`), with directSource's message (`:340`).
2. Read `<top>/PATH`, top being the worktree's top level as in `directSource` (`:342-343`). Missing: "no reader's record at <absolute path>; the reader writes it there".
3. The text names `subject.Commit` and `subject.UnitDigest` (subject from `read.go:622`), with directSource's message (`attest.go:353`). The person learns the digest before any gate runs.
4. The record ends with LAND: its last nonblank line, trimmed and lower-cased, is exactly `verdict: land`. Otherwise: "the reader's record does not end with Verdict: LAND, so nothing was recorded". A later correction line after the verdict refuses (Astra ORR-4).

Each refusal is an `operationRefusal(ReadInvalidCode, …)` with a `run:` line (D0). Revision 2's step 5 (a check for a file at the attestation path before the gate) is replaced by D9, which checks at the write itself.

**D6. The verdict is checked on the bytes that are hashed.** One helper, `readerRecordEndsLand(text)`, holds step 4. `directSource` calls it on the bytes it hashes (`attest.go:347-355`), after the commit and digest check, with the D5.4 refusal. So a record edited between D5 and `directSource` refuses before anything is written (Astra ORR-1). `directSource` runs only when a read is committed (`:999` is its one caller), never at status, so records already on a branch stay valid. Test fixtures that commit a record with no verdict line gain one; the brief lists the eleven sites. The commit takes the file by `git add` after the hash (`:882`); an edit in that window gives a read commit whose hash does not match, and status says `needs read` (`status.go:146`). That is RARE and fails closed: D0.

**D7. A hand-built unit gets its row.** In `handIn` (`landing_plain.go:282-301`), after the named-work rows, each committed unit of the branch status (`state.Status.Units`, those with a source in `state.Sources`) that has no row gets one: `handInUnitRounds(launch.NamedWork{Unit: name})`, the row a unit with no record already gets (`:461-463`: no counted rounds, proof `check`, read `none`). The read label is set for every row by the existing rules (`:287-299`): `waived`, `promoted`, `critic`. No build history is invented. COMMON: every hand-built goal (Problem 8).

**D9. The attestation file is created, never overwritten.** In `CommitRead`'s write loop (`attest.go:1105-1112`), for a `reader-record` source the file is opened with `O_CREATE|O_EXCL` instead of `os.WriteFile`. When it exists: `operationRefusal(ReadInvalidCode, "an earlier read file is in the way at <path>; move it aside, then repeat this command")`, returned before any rollback, so the existing file is never removed. This source generates only the attestation file (`directSource` returns no bundle bytes, `:355`), so nothing of this invocation was written yet; the index was not touched (staging is at `:1121`). A file that appears at any time before the write is caught, also during the gate (Astra ORR-3 reopened). The rollback (`:1090-1103`) then removes only the file this invocation created. Other read sources keep `os.WriteFile`.

**D8. Counting is unchanged.** The new read is a `Goal-Read` commit with a `reader-record` attestation; P2 says why the seat's land gate counts it clean. No change in `status.go`, `validateAttestation`, `goalProgress`, `handLandingSubject` or `internal/landing`.

### Cases

| Case | Class | Where |
|---|---|---|
| Pane in the seat checkout on main | COMMON | D2 |
| Record untracked in the goal worktree | COMMON | D3 |
| A later unit touches the read unit's paths | COMMON | D3 |
| Reader does not know the digest | COMMON | D5.3, before the gate |
| Read says CORRECT or REWORK | COMMON | D5.4 |
| Record missing or outside `records/misc` | COMMON | D5.1-2 |
| Same command run again | COMMON | `already-collected` (D4); outcome `unchanged` (`intent_unit_review.go:1320-1323`) |
| The unit changed tests | COMMON | D4 reader word |
| A hand-built goal is handed in | COMMON | D7 |
| Claim lineage no longer the holder's | COMMON | existing refusal and `goal claim G --take-over` (`goal_branch.go:565-570`) |
| `--reader-record=` with an empty value | RARE | D1 refusal, code 2 |
| A line after `Verdict: LAND` withdraws it | RARE | D5.4 |
| Record edited while the gate runs | RARE | D6, refused before writing |
| Record edited between hash and `git add` | RARE | D6: status `needs read`; land refuses; D0 |
| A file sits at the attestation path, before or during the gate | RARE | D9; D0 |
| Another run owns the goal worktree | RARE | D3 reservation: waiting, `work wait run:RUN` |
| A critic already started for this build | RARE | D0 (`read.go:694-696`) |
| Inherited transfer obligations | RARE | D0 (`read.go:627-629`) |
| The commit has a machinery work record | RARE | its own check stays (`intent_unit_review.go:890-911`); D0 |
| Record edited after recording | RARE | status `needs read` (`status.go:146`); land refuses; restore the file |
| Two runs at once | RARE | `lockBranchRead` (`read.go:634-641`) |

## Units

One unit. Inventory read at 1e3952a54e. Production lines are additions plus deletions in non-test Go, comments included, estimated per owner from a sketch of each change (not compiled: hypothesis until the builder measures). Revision 1 said about 55; Astra's sizing said about 85 before the folds; this count includes the folds.

| Owner | file:line | Change | Lines |
|---|---|---|---|
| work review flag table | `cmd/metasystem/intent_delivery.go:101-104` | one row | 1 |
| commit form help | `cmd/metasystem/intent_review_help.go:11`, `:64-65` | usage, flags, option note (three edited lines) | 6 |
| `reviewCommit` | `cmd/metasystem/intent_delivery.go:1282-1348` | empty refusal 3, combinations 8, D2 14, forward 3 | 28 |
| `commitReviewChecked` | `cmd/metasystem/intent_unit_review.go:947` | D3 arm: reservation, read and publish in the section | 21 |
| `goalBranchReadRun` | `cmd/metasystem/goal_branch.go:251`, `:336-339` | option 2, combinations 6, pass 2 | 10 |
| `BranchReadRequest`, `RunBranchRead` | `internal/goal/branch/read.go:34-65`, `:627`, `:694`, `:715-752` | field 2, two conditions 4, branch split 14, record check D5.1-4 20 | 40 |
| `readerRecordEndsLand`, `directSource` | `internal/goal/branch/attest.go:347-355` | helper 9, call 3 | 12 |
| `CommitRead` write loop | `internal/goal/branch/attest.go:1105-1112` | D9 exclusive create for this source | 12 |
| `handIn` | `cmd/metasystem/landing_plain.go:282-301` | D7 rows | 10 |
| `CommitRead` rollback, `validateAttestation`, status, land gate, lane | named above | none | 0 |

Call sites: `goalBranchReadRun` has one production caller (`intent_delivery.go:366`); `RunBranchRead` one (`goal_branch.go:336`); `commitReview` three (`intent_delivery.go:1337`, `intent_manual_submit.go:372`, `intent_selection.go:754`), of which only the first can carry `--reader-record`; `directSource` one (`attest.go:999`); `handIn`'s rows are read by the queue line (`internal/landing/plain/queue.go:249`, `:438`).

| Unit | Decisions | Production lines | Stop | Usable alone |
|---|---|---|---|---|
| U1 an outside read is recorded | D0-D9 | about 140 | 150 | yes: a hand-built goal lands through the lane with its reads recorded and named |

Revision 2 counted about 115 with zero lines for the write and three for D3. With round 2's folds it is about 140 (D3 +18, D9 +12, revision 2's D5.5 dropped -5). That is over the steward's 120 split line but under the accepted stop line of 150, with little margin. One unit. If the builder measures over 150, he stops and reports per owner; the split is named: U1a the record path (D0-D6, D8, D9, about 130) and U1b the hand-in rows (D7, about 10). Each is usable alone: U1a records and counts outside reads; U1b gives every hand-built unit its row, also for reads waived or promoted. The 250 ceiling does not replace the 150 stop.

## Tests

Through the production composition (R-104-m1e): `newWorktreeCollectFixture` (`cmd/metasystem/goal_branch_worktree_collect_test.go:31-73`): a real primary checkout on main with an unstaged change, a real goal worktree, a hand-built unit commit by `goal branch commit` with no named work, a real lease. The invocation is built as in `TestReviewCommitFromThePrimaryCollectsInTheGoalWorktree` (`:173-200`): `inv.reviewCommit` through the real `goalBranchReadRun` (Astra ORR-N1), with only the gate and the delegate injected as counters, and the real publish (`goalBranchPublishRead`). The hand-in line through `plainLaneBed` (`landing_plain_handin_test.go:197`), which has no named work. No wall clock, no sleep.

- **T1 (COMMON).** A unit that modifies a test file seeded in its parent (a new test file gets no entry: `attest.go:223-225`); the record names commit and digest and ends `Verdict: LAND`; `work review --commit U --goal G --reader-record P` from the primary: confirmed; zero delegate calls; origin's `goal/G` holds the record and an attestation of kind `reader-record` whose test entry has the reader word; branch status from Git shows the unit `read clean` with source `reader-record`; the primary's HEAD, branch and unstaged change unchanged. Run again: `unchanged`, origin unchanged.
- **T2 (COMMON, D7).** `plainLaneBed(t, "reader-record", "reader-record")`: `work land` hands in; the queue line has one row per unit, each `Read: "critic"`, zero counted rounds. (A valid source overrides `waived`, `landing_plain.go:288-297`; no waiver case here.)
- **T3 (COMMON, table).** Refused with the D5.4 line in the non-verbose summary, zero gate calls, origin and the record unchanged: (a) the last line is `Verdict: CORRECT`; (b) `Verdict: LAND` followed by a correction line. (c) RARE, D6: the injected gate rewrites the record's last line to `Verdict: CORRECT`: refused by `directSource`, no read commit, origin, branch, both checkouts and the record unchanged.
- **T4 (COMMON).** No digest in the record: the non-verbose summary names the commit and the digest; zero gate calls.
- **T5 (COMMON, table).** The worktree detached so no worktree holds `goal/G`: (a) local `goal/G` present: refused with the D2 line and the `worktree add` command; (b) local `goal/G` deleted: the fetch command, the add as detail. In both the primary's HEAD, branch and index unchanged.
- **T6 (COMMON).** A second unit commit touches the first unit's file; the first unit's record is untracked: the first unit's read is recorded (D3).
- **T7 (table, through the public parser).** Code 2, the read owner never called, nothing published: `--reader-record` with `--brief`, `--model`, `--retry`, `--units`, `--dispositions`; and `--reader-record=` (empty, RARE).
- **T8 (RARE, D0's one test, table).** Refused, branch read record unchanged: (a) a critic root already recorded for the build; (b) a sentinel file at the attestation path before the command: refused with the D9 line, the sentinel's bytes unchanged, no read commit. (c) the injected gate creates the sentinel and passes: the same. (d) another runner holds `ReserveMutation` on the goal worktree (as `TestManualReadPublicationReservationGitAdapter`, `cmd/metasystem/intent_read_publication_2_test.go:500-562`): waiting with `work wait run:RUN`, zero gate calls, nothing written.
- **R1 (rehearsal, after landing).** From Wido's pane: `goal claim G` (person), then the record step on a real hand-built unit, then `work land`. It settles P1's hypothesis.

Mutations, each failing the named test: drop D2 (T5: HEAD moves); keep the publication check for records (T6); drop the verdict check in D5 (T3a); check "last line starting with verdict:" instead of the last nonblank line (T3b); drop the `directSource` verdict call (T3c); move D5 after the gate (T3a, T4: gate calls); write the record source's attestation with `os.WriteFile` (T8b, T8c: sentinel gone); drop the D3 reservation (T8d: the read runs); drop the empty refusal (T7 empty row); drop the `ReaderRecord` pass in `goalBranchReadRun` (T1); omit the reader word (T1); let a record fall through to the critic dispatch (T1: delegate count); drop the D7 rows (T2); accept `--reader-record` with `--model` (T7); drop the record half of `:694` (T8a).

## Round 1 folds

| Finding | Class | Verified at | Fold |
|---|---|---|---|
| ORR-1 checked bytes are not the hashed bytes | MATERIAL, RARE | `attest.go:347-355`, `:950`, `:999`, `:882` | D6: the verdict check runs in `directSource` on the hashed bytes; T3c. The later `git add` window fails closed (status `needs read`), D0. Problem 6 corrected. |
| ORR-2 empty `--reader-record=` widens D3 | MATERIAL, RARE | `intent.go:347`, `:382-454`; `goal_branch.go:81-95` | D1: empty refused at the public verb and at the owner (`goalBranchReadOption`); D3's arm also needs `check == nil`; T7 empty row. P4 added. |
| ORR-3 rollback deletes a file at the attestation path | MATERIAL, RARE | `attest.go:1090-1125`; `range.go:238-240` | D5.5: refuse an occupied attestation path before the gate; T8b. Problem 7 added. Replaced by D9 in round 2. |
| ORR-4 "last line starting with verdict:" is not "ends with" | MATERIAL, RARE | design rev 1 D5; decision 29 Q1 | D5.4: the last nonblank line must be `verdict: land` (trimmed, any case); T3b. |
| ORR-5 hand-built units get no hand-in row | MATERIAL, COMMON | `landing_plain.go:282-301`; `intent_selection.go:31-54`; `unit_named.go:553-556` | D7: a row per committed unit without named work; T2 through `plainLaneBed`. Problem 8 added; revision 1's "no change in `landing_plain.go`" withdrawn. |
| ORR-N1 the journey bed bypasses the flag owner | MINOR | `intent_unit_review_test.go:296-317` | Tests now run through `newWorktreeCollectFixture` and the real `goalBranchReadRun`; mutation "drop the `ReaderRecord` pass". |
| Size: 55 is low, about 85 before folds | note | inventory above | Recounted with the folds: about 115, stop 150, split named. |
| No design record for the goal | note | `intent_operations.go:689` | Brief, step 0: the steward registers the accepted page on main before the build. |

## Round 2 folds

| Finding | Class | Verified at | Fold |
|---|---|---|---|
| ORR-6 D3 drops the worktree reservation | MATERIAL, RARE | `intent_unit_review.go:949-953`, `:1093-1117`; `tree_reservation.go:128-143`, `:496-526`; `read.go:634-641` | D3 keeps `ReserveMutation`, `MutationSection` and the publish in the section; drops only the comparison and journal. T8d. Problem 9 added. |
| ORR-3 reopened: a file can appear during the gate | MATERIAL, RARE | `read.go:246-268`; `attest.go:1090-1112` | D9: exclusive create at the write for this source; refusal before rollback. Revision 2's D5.5 dropped (one mechanism, not two). T8b, T8c. |
| ORR-N2 a new test file gets no entry | MINOR | `attest.go:223-225`, `:246-250` | T1 seeds the test file in the parent and modifies it in the unit. |
| ORR-N3 `waived` is overridden by a valid source | MINOR | `landing_plain.go:288-297` | T2's waiver case removed. |
| ORR-N4 plain errors are hidden | MINOR | `intent_unit_review.go:1257-1262` | D0, D5: every refusal is an `operationRefusal`; T3, T4 assert the non-verbose summary. Problem 10 added. |
| Size not proven | note | inventory | Recounted: about 140, stop 150, split named. |

## Critique record

- Round 1, 2026-10-23 08:46 CEST, Astra (Codex), log `/Users/wido/agentic-tools-evidence/lane-test-20261009/designs/an-outside-read-is-a-read-record-critique-1.log`: 5 material (ORR-1 to ORR-4 RARE, ORR-5 COMMON), 1 minor (ORR-N1). Folded as D6 (ORR-1), D1 and D3 (ORR-2), D5.5 (ORR-3), D5.4 (ORR-4), D7 (ORR-5), the test composition (ORR-N1). Size recounted from 55 to about 115.
- Round 2, 2026-10-23 09:08 CEST, Astra (Codex), log `/Users/wido/agentic-tools-evidence/lane-test-20261009/designs/an-outside-read-is-a-read-record-critique-2.log`: 2 material (ORR-6 RARE, ORR-3 reopened RARE), 3 minor (ORR-N2, ORR-N3, ORR-N4). ORR-1, ORR-2, ORR-4, ORR-5 and ORR-N1 judged resolved. Folded as D3 (ORR-6), D9 (ORR-3), T1 (ORR-N2), T2 (ORR-N3), D0 and D5 refusal form (ORR-N4). Size recounted from 115 to about 140.

## Round 3 fold (steward, 2026-10-23 09:25; the last round by the stop rule)

Round 3 (critique-3.log): the round-2 folds ORR-6 and ORR-3 resolved; one new MATERIAL·RARE finding, ORR-7: a person who edits the reader record between the first staging (`attest.go:882`) and the second staging into the real index (`attest.go:1118-1121`, `attest_repository.go:152-158`) has the correction overwritten by checkout (`commit.go:565-572`), and status still reports clean because the committed bytes match their hash. Astra: a bounded fixture, no fourth prose round.

Fold under D0 (rare case, generic fail-safe), a build obligation: for a `reader-record` source only, immediately before the second staging the command re-reads the record and compares its bytes with the bytes hashed in `directSource`; any difference refuses with one plain line ("the reader record changed while it was being recorded; nothing was recorded; run the command again"), before the real index or the worktree is touched, through the existing rollback. Test T3d: a fixture seam edits the record between the two staging steps; the command refuses, the corrected record stays on disk, no attestation is committed. Mutation: drop the re-read and T3d fails. About +8 production lines (estimate ~148 of the 150 stop line; if the builder measures over 150, the named split U1a/U1b applies).

Status: accepted by the steward in Wido's name (decisions 29 and 31 standing grant; Wido's rare-case rule, three rounds 5 → 2 → 1).
