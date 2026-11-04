# One host admits heavy work

- Kind: design
- Id: one-host-admits-heavy-work
- Status: accepted
- Goals: one-host-admits-heavy-work

Accepted under decision 195 (a1 in Wido's name, 2026-11-04 01:1x): one arbiter, `proofrun`'s host admission, as `plans/designs/the-steward-sees-and-acts.md` D5 (approved by Wido 2026-10-30) rules; its slot cap is R-107-m1e's, so the R-35-m3 question of revision 3 is moot.

## Revision 4 (decision 195)

Written 2026-11-04 by opus-designer (Claude Opus 5.5), after reading the ui seat's draft brief `agentic-tools-evidence/lane-test-20261009/briefs/the-lanes-proof-goes-first-U1-brief.md` (revision 1, 2026-11-04 00:55; sections 1, 3 to 6, 10 to 12) and the `proofrun` admission code it cites, at m1e main `02730a35de`.

Why. The ui seat showed that revision 3 collides with the-lanes-proof-goes-first (goal G3 of the-steward-sees-and-acts, D5: one owner inside `proofrun`'s existing admission, no second lease, the proof-beside-builds hold retires) in the same files: revision 3's U1c changed the lane's `Start`, `Run`, `KeeperContentionHold` and the hold readers that G3's U3 removes, and its host lock was a second lease (brief section 3). One arbiter wins: `proofrun`'s admission.

What changed:

1. Gone: `~/.metasystem/heavy/host.lock`, the shared and exclusive modes, descriptor inheritance, `METASYSTEM_HEAVY_FD`, the package `internal/heavy`, the lane part (revision 3 U1c, D4's atomic lane start, the keeper probe, the person's lane proof beside a check). The lane side is the-lanes-proof-goes-first U1: the lane's check reserves a heavy slot first, and a held wait marker makes every other heavy run wait.
2. U1 of this page is now only the carry check. It reserves a normal `proofrun` slot before its subject and read locks, on all three paths that reach it: `work review --check-only`, `work rebase`, and the amended-round review. So it waits behind the lane's marker and other heavy runs under `proofrun`'s cap, says what it waits for and records the wait. The person's `--now` stays, with the impact said and recorded. A check that cannot reach the admission from a sandbox refuses and never runs unadmitted.
3. Handed over: the unit check's child wait (revision 3 U2) is the-lanes-proof-goes-first U3 (brief section 5, U3 item 3). The lane-fix allowance goes with it: the brief names it a rare case (section 10). The person's unit-run override bound to a launch id goes with them, as a question for that brief (section 6, Q2).
4. Limit L1 goes away with the lock. The same gap exists in `proofrun`'s slot custody: section 1, L2. It is shared with the lane's and the unit check's reservations, and the brief already names it for the lane.
5. Tests: those that still apply are kept, now through `proofrun`'s seams (section 5). U1 is re-estimated by file (section 4).

Revision 3 is not kept beside this page; the critique logs of rounds 1 and 2 quote the lines they judged. Their dispositions are at the end. Most decisions they shaped are superseded here; the ones that survive are named.

## 1. Problem

### Intent

Heavy work on one host is admitted in a host order, so no two of them contend. The waiting side says what it waits for and starts by itself when its turn comes. A person is never refused. Under decision 195 there is one arbiter, `proofrun`'s admission. The lane and the unit check join it in the-lanes-proof-goes-first. This page brings in the carry check.

### Evidence

- 2026-11-03. The carry check `work review --commit 49fcbb696f --goal a-check-guards-its-own-inputs --units U1c1 --check-only` started 18:07 while the lane began a two-goal proof (start order not established). It ended 18:29 with "✗ cheap timed out: no output for 15m0s (proof.deadline)" (`panes/umbrella-u1c1f-check-1807.out`). That is `boundedexec`'s silence bound (`metasystem/internal/boundedexec/boundedexec.go:167`) on the check's first command, "cheap" (`metasystem/internal/launch/unit_check.go:120`, `:138-140`).
- Alone it took 45 minutes and was green (`panes/recheck-umbrella-chain-2-20261103-1837.log`, 18:39:04 to 19:24:49). Wido paused the lane for it. a1's hand lock and gate are at `panes/recheck-umbrella-chain-2.sh:13-22`.

### What the code does

- Nothing a carry check runs takes a slot. `carrySubjectCheck` runs the frozen check in its own process (`metasystem/cmd/metasystem/intent_carry_check.go:143-145`). Its cheap command is `metasystem test impact` (`metasystem/metasystem.conf:17`), whose groups only inherit a parent's lease (`metasystem/cmd/metasystem/test_impact.go:295`, `metasystem/internal/proofrun/groups.go:184`). `work review` is no legacy launcher in the census (brief section 1; `metasystem/internal/proofrun/attemptload.go:490-496`).
- Three paths reach it, each under a lock:
  - `work review --commit … --goal … --check-only` calls `branch.ResolveReadGate` (`metasystem/cmd/metasystem/intent_delivery.go:755-759`), which takes the branch read lock (`metasystem/internal/goal/branch/read.go:299-306`) and then calls the subject check (`read.go:223`).
  - `work rebase` calls it inside `conn.section` (`metasystem/cmd/metasystem/intent_work_rebase.go:132-135`, through `owners.subjectCheck`, `metasystem/cmd/metasystem/intent_worktree.go:97-98`).
  - The amended-round review calls it inside `conn.commitToken` (`metasystem/cmd/metasystem/intent_unit_review.go:474-481`).
- The selection of a retained execution comes before the run (`intent_carry_check.go:93-142`). A concurrent invocation finds a folder without `result.json` and refuses (`:116-126`).
- `proofrun`'s admission. `AcquireHostResourcesWithWaitCheck` (`metasystem/internal/proofrun/host_resources.go:716-722`) takes the cap `proof.admission.top-level-max` (default 1, `metasystem/internal/config/defaults.go:111-112`; `metasystem/internal/proofrun/admission.go:20-45`). It loops under the admission guard and calls the wait observer once at the first failed scan (`WithHostResourceWaitObserver`, `host_resources.go:665-682`, `:915-920`). It polls on a real 100 ms timer (`:921-924`) and returns a lease whose `Waited` is real elapsed time (`:152-157`, `:913`). `Close` closes without unlocking (`:159-170`). The directory is `~/.metasystem/proof-admission`, created and secured on first use (`:180-204`); a test namespace is selected by `METASYSTEM_PROOF_ADMISSION_TEST_DIR` with its fixture root (`:182-188`). Lease and slot files are opened with `os.OpenFile` (`:285-306`), so Go makes them close-on-exec.
- A Codex launch runs `workspace-write` with `-C` its working directory (`metasystem/internal/launch/codex.go:129-136`): no write to `~/.metasystem`.

### L2, the custody gap, checked

Revision 3's L1 goes away with the host lock. `proofrun`'s slot custody has the same gap for this caller. The slot and lease descriptors are close-on-exec and are not passed down (`host_resources.go:285-306`; the brief's V3 keeps it so on purpose). The check's commands run in their own process group (`boundedexec.go:119-121`). So if the carry check's own process dies while its cheap command runs on, the slot frees and other heavy work may start beside the rest of that check. A lease marked dirty would not close it: reclaim judges the owner's process group (`metasystem/internal/proofrun/lease_reclaim.go:190-194`), not the command's. The brief names the same gap for the lane's check (section 10, first rare case). It is one limit of the one arbiter, owned by `proofrun`. This page accepts it and adds no custody of its own. Closing it would mean running the check's commands under `proofrun`'s resource custody (`metasystem/internal/proofrun/resource_custody.go`), for every holder at once. That is not this goal.

## 2. Decisions

- **D1 The carry check reserves a normal slot before its locks.** A new `carrySlot` in cmd acquires class `heavy`, no exclusive resources, for the installation and its `metasystem.conf`, with no lane priority. Each entry reserves it before its lock:
  - before `ResolveReadGate` (`intent_delivery.go:759`);
  - before `conn.section` in `work rebase` (`intent_work_rebase.go:132`);
  - before `conn.commitToken` in the amended-round review, only when `subject.Amends` is set (`intent_unit_review.go:474-476`).

  The lease is held until the entry returns, then closed. So the check waits behind the lane's held wait marker and behind any slot holder under the cap, with no lock of its own held while it waits. The order is always slot, then lock; no holder of those locks waits for a slot. Commands the check starts that ask for a slot (a nested `test run` in another project's `proof.cheap`) take none, by the brief's ancestry rule (dependency 3).
  - A cost, named: a rebase or amended review that turns out to need no check still waits for a slot and holds it while it runs. The lock order allows no later point. Q1 asks whether that is acceptable.
  - Rejected: reserve inside `carrySubjectCheck` (`intent_carry_check.go:22`). It runs under the read, section or commit lock, so a waiting check would hold them.
  - Rejected: a lease of this page's own (revision 3). Decision 195.

- **D2 It says what it waits for and records the wait.** The wait observer writes once to standard error: "carry check: waiting for a test slot on this host (the lane's check or another test run holds it)". When the-lanes-proof-goes-first U2 lands, `ReadHostAdmission` (dependency 5) can name the holder; that is deferred unit U2. After admission the check writes to `records/waits.jsonl` in its records folder (`intent_carry_check.go:44`): when it began waiting, when it was admitted, and the lease's `Waited`. A wait that ends without admission (the command stopped, or its caller timed out) writes nothing there, because the subject is not known before the locks. Its standard-error line is the record, nothing ran, and no `check-*` folder exists, so a repeat is not blocked.

- **D3 Selection after admission, under a subject lock.** In `carrySubjectCheck`, a per-subject lock, exclusive and blocking, in the records folder comes before the selection (`:93`). The selection and the run happen under it. Admission already came first (D1). So two invocations of one subject (cap above 1, or a person's `--now`) do not race: the second reselects and reuses the green result. The progress timer starts only through `After`, now passed by the carry check (`:144` passes `Now` only today).

- **D4 A person says now.** `--now` on the three entries (`work review … --check-only`, `work rebase`, the amended-round review) needs the person proof at the enrolled terminal (`checkDirectPersonProof`, `metasystem/cmd/metasystem/intent_work.go:2988-2990`). Before anything runs it prints: "This runs now, beside the test run that holds the slot on this host. Both run slower, and the check may end on its 15-minute silence clock. Stopping this command stops the check's own process, but a command it already started runs on in its own process group until it ends or its clock ends it." The same text, who, when and the subject go to the installation's decision log before it runs (Q3 names which log). It tries the slot once, as the brief's V5 does for a person's lane check. If no slot is free it runs beside without one. An agent's `--now` is refused with `humanauthority.PersonActRemedy` (`metasystem/internal/humanauthority/authority.go:1149`), and nothing runs. Finding 494 is why a person is not detected automatically.

- **D5 Unreachable refuses; unreadable fails open.** If the admission directory cannot be opened or created because the process may not write there (a permission error, as in a Codex `workspace-write` sandbox), no slot can ever be held from this process. The check refuses with `CARRY_CHECK_UNADMITTED`: "this carry check cannot reach the host's test admission at ~/.metasystem/proof-admission from here; nothing ran; run it from the seat's shell". Every other admission error fails open with a named line, "the host's test admission could not be read (<error>); this carry check runs without a slot", on standard error and in `waits.jsonl`. That follows R-107-m1e ("an unreadable slot state fails open with a named record, never closed") and the brief's V4. Before the brief, rollout step 1 lists who runs these entries and from where.

- **D6 What this page does not decide.** The lane's priority, its wait marker, the ancestry rule, the unit check's slot, the hold's retirement and every reader of the admission belong to the-lanes-proof-goes-first U1 to U3.

## 3. What does not change

- `proofrun`'s cap, key, census, nested and authenticated branches, and its custody (`host_resources.go:733-830`).
- The lane, its hold and its readers, here. The-lanes-proof-goes-first changes them.
- The unit check, replays and comparisons, here. The unit check joins in the-lanes-proof-goes-first U3; replays and comparisons stay as today.
- The check's progress clock and ceiling (`unit_check.go:61`, `:138-140`).
- A raw `go test` typed by a builder bypasses admission; the-lanes-proof-goes-first U2 shows it as a bypass.

## 4. Unit U1, the carry check takes a test slot

Production lines are additions plus deletions; a replaced line counts twice. Nothing of revision 3 was built, so nothing is deleted. Estimates run double.

| Part | Decisions | Production (estimate) per owner file | Doubled | Tests (estimate) |
|---|---|---|---|---|
| U1a the slot and the review path | D1, D2, D4, D5 | about 46: `metasystem/cmd/metasystem/carry_slot.go` 38 (new: installation and conf 4; the observer's line 4; a person's single try, the impact text and the log line 10; an agent's `--now` refused 3; unreachable refuses, unreadable fails open with its line 9; release by `Close` 3; the acquire seam on the work owners, defaulting to `proofrun.AcquireHostResourcesWithWaitCheck` 5); `metasystem/cmd/metasystem/intent_delivery.go` 6 (reserve before `:759`, release, render a refusal); `metasystem/cmd/metasystem/intent_review_help.go` 2 (`now` among the check-only flags, `:122`, `:15`) | about 92 | about 200 |
| U1b the other paths and the record | D1 to D3 | about 27: `metasystem/cmd/metasystem/intent_work_rebase.go` 5 (reserve before `:132`); `metasystem/cmd/metasystem/intent_unit_review.go` 5 (reserve before `:476` when amended); `metasystem/cmd/metasystem/intent_carry_check.go` 12 (the subject lock before `:93` 5; `waits.jsonl` 5; `After` 2); `--now` on the two other entries' flag tables 5 | about 54 | about 170 |
| **U1 total** | | **about 73** | **about 146** | about 370 |

Order: U1 is built after the-lanes-proof-goes-first U1 lands. Before that the lane takes no slot, so the carry check would wait only for other test runs, not for the lane. Parts a then b, one branch, landing together. Briefs get `Read each round: yes`. Reverse dependents: `metasystem/cmd/metasystem` by name, `metasystem/internal/proofrun` whole (no change; its callers grow).

Step value: a carry check never starts beside the lane's check or another heavy run under the cap, on any of its three paths. It says that it waits and records how long. A person runs it now and is told what that costs. After U1 on every engine, a1 drops the mkdir lock for carry checks.

Deferred: **U2, name the holder.** The waiting line names who holds and who waits, from `ReadHostAdmission`. About 10 lines (20 doubled). Trigger: the-lanes-proof-goes-first U2 landed, and a person asks what a carry check waits for.

## 5. Tests

Seams (R-104-m1e):

- **Admission.** The acquire function is a seam on the work owners. Tests hand the exported `proofrun.AcquireHostResourcesWithWaitCheck` inside a fixture admission namespace they select (`host_resources.go:182-188`), or a fake for the error rows.
- **Ordering.** Ordering comes from slots the test holds and releases. The release happens inside the wait observer, which fires once at the first failed scan (`:915-920`). `proofrun`'s poll is a real 100 ms timer with no clock seam (`:921-924`), and `Waited` is real elapsed time. So no test asserts a duration; `Waited` is asserted present and non-negative.
- **Clock and timer.** `Manager.Now` and `CheckContext.After` (`metasystem/internal/launch/launch.go:87`, `unit_check.go:56-57`). The fake `After` records each registration.
- **Commands.** Fixture commands block on a fifo the test controls.

U1a:

- **T1** `TestACarryCheckWaitsForASlotBeforeItsReadLock` (new, `metasystem/cmd/metasystem/intent_carry_slot_test.go`, the bed of `metasystem/cmd/metasystem/intent_carry_check_test.go:97`), through `work review --commit … --goal … --check-only`. The test holds the namespace's only slot (cap 1). Inside the observer it asserts four things: the observer's line was written once; the branch read lock for the goal is free (the test's own non-blocking flock on it succeeds and is released); no `check-*` folder exists; no `After` is registered. Then it releases the slot. The check runs and passes, and `waits.jsonl` has the wait with `Waited`. Fails first on main: the check runs at once, and the observer never fires. Kills: the reservation moved inside `carrySubjectCheck` (the read lock is held in the observer); no observer line; `After` registered before admission.
- **T2** `TestAPersonsNowRunsBesideAndIsRecordedFirst` (new, same file, enrolled person prover as `metasystem/cmd/metasystem/fleet_build_admission_test.go:194`). The slot is held by the test, and an automatic invocation of subject S waits. The person's `--now` for S prints the impact text and writes the log line before the fixture command starts (the fixture fails the test if it finds no log line). It tries the slot once, runs beside, and the waiting one is still waiting. When the slot is released, the waiting one reselects and reuses the person's green result. With a helm grant or the fixture prover, `--now` is refused with the person remedy and nothing runs. Kills: a person's wait not single-try (the person blocks); `--now` without the proof; the log line after the run.
- **T3** `TestACarryCheckThatCannotReachTheAdmissionRefuses` (new, same file). The acquire seam returns a wrapped `fs.ErrPermission`: `CARRY_CHECK_UNADMITTED` with D5's words, and the fixture command never runs. Row "unreadable": a wrapped decode error makes it run, with the named line on standard error and in `waits.jsonl`. Kills: fail open on permission; fail closed on a read error.

U1b:

- **T4** `TestEveryCarryPathReservesBeforeItsLock` (new, same file). Through `work rebase` and an amended-round `work review`, with the slot held by the test: inside the observer, the section lock and the commit token lock are free (probed as in T1). After release each path's check runs. Kills: one path left unreserved (it runs at once, and the observer never fires).
- **T5** `TestACarryCheckReselectsAfterAdmission` (new, same file).
  - Row "selection after admission": while the invocation waits, the test writes a green retained execution for the subject. After admission it is reused, and the cheap command ran zero times.
  - Row "stopped while waiting": the command's context ends inside the observer. "nothing ran", no `check-*` folder, no `waits.jsonl` line, and a repeat with the slot free runs.
  - Row "concurrent": cap 2 in the namespace's conf, and two invocations of one subject. The first's cheap command blocks on a fifo; the second waits on the subject lock, then reuses the green result. The cheap command ran once.

  Kills: selection before admission (row 1); no subject lock (row 3 refuses "unavailable"); a folder made before the wait (row 2).
- **T6** `TestACarryCheckWaitsBehindTheLanesMarker` (new, same file; built after the-lanes-proof-goes-first U1). A held lane wait marker in the namespace and a free slot: the carry check waits. When the marker is released, it runs. Kills: a carry acquisition given the lane priority.

Changed wants: none expected. The existing carry tests (`intent_carry_check_test.go`) get a free fixture namespace through the default seam, or the bed's fake; the brief confirms by inventory.

## 6. Rollout, dependencies and questions

Rollout:

1. Before U1a's brief: on m1e, list who runs `work review --check-only`, `work rebase` and the amended-round review, and from where. Hypothesis: no Codex builder runs them.
2. The-lanes-proof-goes-first U1 lands first; then U1 here.
3. Every engine that runs a carry check or the lane runs both: m1e's seat engine, agentic-tools-ui's seat engine, the lane's engine. Then a1 runs the next carry check without the mkdir lock and without pausing the lane. One clean day ends the hand lock for carry checks.
4. The final brief of revision 3's U1 (`agentic-tools-evidence/lane-test-20261009/briefs/one-host-admits-heavy-work-U1-brief.md`, decision 194) is superseded. It is re-cut from this page.

Names this page depends on from the-lanes-proof-goes-first (brief revision 1). The re-cut confirms them when that brief is final:

1. The reservation call: `proofrun.AcquireHostResourcesWithWaitCheck(ctx, controlRoot, confPath, "heavy", nil, check)` (exists, `host_resources.go:716`); the carry check passes no lane priority, so it does not use `proofrun.WithLaneCheckPriority` (brief U1 item 1).
2. The wait words hook: `proofrun.WithHostResourceWaitObserver` (exists, `:671`).
3. The ancestry rule: a process whose strict ancestor owns a live heavy lease in the same directory takes no slot and never waits for capacity (brief U1 item 4, in `acquireHostResourcesWith` after `:741-750`).
4. The marker: `wait-lane-<32 hex>` in the admission directory, held by the lane's waiting acquisition; while it is held by another description, a heavy acquisition without the lane priority finds no room (brief U1 items 2 and 3).
5. The read view, for deferred U2: `proofrun.ReadHostAdmission() (HostAdmission, error)` with `Held` and `Waiting` (brief U2 item 2).
6. The person's single try: an observer that cancels the context, then running beside with a named line (brief U1 item 5, V5).
7. Fail open on an admission error with a named line (brief V4).
8. The unit check's slot and its managed mark around `plan.Check.Run` (brief U3 item 3; `proofrun.MarkManagedProofProcess`, `:932`). The carry check needs no managed mark, because `work review` is no legacy launcher.
9. The builder guard: the brief stops on `internal/heavy` or `METASYSTEM_HEAVY_FD`. Revision 4 has neither.

Questions:

1. D1's cost: a rebase or amended review that needs no check still waits for a slot. Accept, or reserve only on the `--check-only` path and let the other two wait in the subject check (they would then hold their lock while waiting)?
2. For the-lanes-proof-goes-first U3: a person's override for a unit check that waits in its detached child. It should be bound to that launch id, read once, and kept as evidence. Revision 3 designed it (`now-<launch id>`, found with `CheckParent`, `metasystem/internal/launch/check_evidence.go:15-37`); does U3 take it, or does the person's single try at launch (V5) suffice?
3. Which decision log D4 writes to: the installation's `intent_decision` log (`metasystem/cmd/metasystem/intent_decision.go:263-278`) or `proofrun`'s admission directory. The draft says the installation's.
4. L2 is accepted here as `proofrun`'s limit. Wido may want it closed for every holder at once, in `proofrun`.

## 7. Rulings

- Decision 195 (a1 in Wido's name, 2026-11-04): one arbiter, `proofrun`'s admission. The-steward-sees-and-acts D5, approved by Wido 2026-10-30.
- R-107-m1e: the cap, slot custody by proven identity, nested receipts exempt, an unreadable state fails open with a named record (D5).
- R-35-m3: superseded for this arbiter by R-107-m1e; no new hold between seats on this page.
- R-104-m1e: artificial clocks where the code has them; the `proofrun` poll's real timer is named, and no test asserts time (section 5).
- R-142-m1e and R-143-m1e: a person is never held; the impact, with what stopping does not undo, is said and recorded first (D4).
- R-121-m0: the smallest thing first (one caller, about 73 lines).
- R-129-ui: idempotent (D3, a repeat reuses the green result).

## Critique round 2 (Astra)

2026-11-03, `agentic-tools-evidence/lane-test-20261009/designs/one-host-admits-heavy-work-critique-2.log`: 5 material, 3 minor, redesign. Folded in revision 3. Status under revision 4:

1. Authority: moot under decision 195 (R-107-m1e governs the one arbiter).
2. Custody through native tests: L1 gone with the lock; the same gap in `proofrun`'s custody is L2 (section 1), accepted as the arbiter's limit.
3. Subject lock blocks `--now`: holds. The subject lock comes after admission (D3); T2.
4. Unit override binding: handed to the-lanes-proof-goes-first U3 as Q2.
5. Impact explanations: holds for the carry check (D4). The lane proof's explanation goes with the lane part to the-lanes-proof-goes-first.
6. Mutation claims: T1, T4 and T5 keep the fixes (lock probed in the observer, selection isolated); the fix-cap test went with the unit check.
7. Process seams: the lane's kernel reads are no longer this page's; `proofrun`'s real poll is named (section 5).
8. Size: U1 is about 73, doubled about 146; parts 92 and 54 doubled.

## Critique round 1 (Astra)

2026-11-03 about 20:45, `agentic-tools-evidence/lane-test-20261009/designs/one-host-admits-heavy-work-critique-1.log`: 9 material, 5 minor, redesign. Folded in revision 2. Status under revision 4:

1. Authority: moot (decision 195).
2. Override in the first slice: holds (D4).
3. Lane start paths: moved to the-lanes-proof-goes-first.
4. Replays: unchanged by this page.
5. Dead owner versus ended command: L2 (section 1).
6. Repeat and timeout: holds (D2, D3, T5).
7. Sandbox writer: holds (D5, T3).
8. Lane-fix cap: handed over with the unit check.
9. Size: holds (section 4).
10. to 14. (minors): the ticket and setting items are gone with the queue; the readers are deferred U2 and the-lanes-proof-goes-first U2; the test and seam folds are kept (section 5).
