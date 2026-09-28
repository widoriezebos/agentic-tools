# Sol's read of the g1-s67 build

Produced 2026-09-28 by Codex on `gpt-6-sol`, read-only, against branch `room-every-sitting` at `698765dff` (base `12611b98c`), brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/sol-code-read-brief-room-every-sitting.md`. Verbatim.

## Read 1

---

## Findings

**SOL-S67-01 — High · material: yes.** The new checkout source route can return files that the document reader refuses. I read `reviewRead`, `reviewedOf`, `AsItStands`, `insideOf`, and the document reader’s `read` and `admissibleID` at whole-function depth. A valid design record is enough to reach `AsItStands`; its path check prevents escape from the checkout, but does not reject protected segments such as `.git` or `artifacts`, or the local configuration path. It also accepts NUL-free invalid UTF-8, which JSON can display with replacement characters while labeling the result as the checkout’s current text. See [review.go](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/httpd/review.go:54), [checkout reader](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/review/review.go:579), and [document refusals](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/project/document.go:43).

A caller who can reach the interface can request a protected, readable file through a design’s source URL; no standing sitting is checked. An invalid-text read can silently show different characters from the file. **Fix:** give checkout source reads an explicit admission rule that excludes protected targets, including aliases through symlinks within the root, and refuse invalid UTF-8 before returning lines. Test both refusals through the route. **R-124:** the slice is not safe without this fix: the first crafted read can disclose checkout-local data, and a text read can give a silent false answer.

## Departures from the design

- **An extra size refusal:** §6 specifies the shared source bounds; the implementation adds an 8 MiB whole-file cap in [review.go](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/review/review.go:65) and applies it at [line 605](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/review/review.go:605). This can refuse a file the Partner can read under [D2](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:102). It does not block the demonstrated first use.

- **A general conversation correction:** [loaded](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/partner/conversation.ts:172) now retains beats newer than an act’s snapshot. D1–D7 did not request a general conversation change, but the builder disclosed it and tied it to the walkthrough’s drafted Outcome card.

- **Board anchors are displayed, but shaping entries do not offer a desk press:** [D5](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:129) calls for entries pinned to their anchors. [Board](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/review/Board.tsx:77) shows a shaping entry’s anchor inside its link to the record section; the desk-opening `AnchorPress` is used for findings at [line 65](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/review/Board.tsx:65). The anchor remains visible, so this is not an R-124 blocker.

I found no undisclosed omitted step-1 behavior. The report expressly discloses the missing Start-press component test and that its pre-D16 service fixture offers a fact rather than recording it; the walkthrough supplies the recorded-fact observation.

## Deferred

The extra size cap and shaping-board anchor press above can wait under R-124. The checkout test also sets up real Git at [checkout_test.go:41](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/review/checkout_test.go:41), contrary to the local test rule, although the checkout-read behavior itself needs no Git. That is a proof-maintenance correction, not a first-use product blocker.

## What I verified holds

The diff routes both addresses into one room and chrome; the drawer and `/brain` no longer mount sitting controls. The shaping desk uses the document reader’s checkout root, opens on record sections, and refuses `/changes` in words; review source reads still take the reviewed tree. Walks are keyed by purpose, `present` reaches a shaping sitting, and `finding` remains review-only. I traced these through [routes](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/routes.ts:232), [Room](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/review/ReviewRoom.tsx:54), [reviewRead](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/httpd/review.go:38), and [Walk/admitPresent](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/review.go:136).

For the pre-D16 case, the resolver chooses the ordinary conversation only when the record conversation has no mark and the ordinary mark names that record. It does not cache that fallback under the record key. Snapshots and beats carry the room address, while the live-session comparison uses the resolved conversation’s key. I read [conversationOf/openedOf](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/service.go:314), [SnapshotIn](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/service.go:618), [turn admission](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/service.go:704), and the [pre-D16 fixture](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/room_service_test.go:79) at whole-function depth. The shaping End sheet has its two actions without verdict controls; the review sheet retains its three verdicts.

**Failing-first check:** yes, at least four reported tests necessarily fail against `12611b98c` by direct comparison with its code: the base lacks `AsItStands` for [checkout_test.go:51](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/review/checkout_test.go:51), `sittingPath` for [routes.test.ts:272](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/routes.test.ts:272), `firstDesk` for [shaping.test.tsx:98](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/review/shaping.test.tsx:98), and `EndShapingWays` for [ending.test.tsx:114](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/partner/ending.test.tsx:114). I read the base with `git show`; I did not execute tests on a base checkout.

At HEAD, `npm test -- --run src/review/shaping.test.tsx src/routes.test.ts src/partner/ending.test.tsx` reported **“Test Files 3 passed”** and **“Tests 50 passed”**. `go test ./internal/ui/review ./internal/ui/partner ./internal/ui/httpd` could not start: **“go: creating work dir … operation not permitted”** under the read-only sandbox. I also inspected the supplied opening-room, board, phone End-sheet, and pre-D16 room screenshots.

**VERDICT: 1 material finding: SOL-S67-01.** Read commit `698765dff7506c6c0e53abfa8f7ce855b0c00b6d` against `12611b98c`. The worktree remained unchanged. Go runtime behavior and the proposed unsafe-read cases were assessed from code, not executed.


## Re-read of fix round 1

---

**VERDICT: 1 material finding: SOL-S67-02.** SOL-S67-01 is **FIXED** for the requested path, alias, text, and sitting checks. I read commit `d018745523a35a4ce1d6792811ef6b5f01487191` against `698765dff`; the worktree was clean.

### Findings

**SOL-S67-02 — High · material: yes.** The new timer can still leave older draft words on the sitting mark. Each call to `keepRoomNow` sends its snapshot independently, and the server accepts whichever keep arrives last ([store.tsx](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/partner/store.tsx:1763), [store.tsx](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/partner/store.tsx:1795), [review.go](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/review.go:157)). For example, a change sends “hal”; the user finishes “half a finding”; the silence timer sends the full words. If the timer’s request arrives first and the earlier request arrives later, the earlier words overwrite the full draft. No further change schedules a repair. **Fix:** order or version keeps so an older snapshot cannot overwrite a newer one; test a delayed first keep. **R-124:** yes. Losing the last words during an ordinary delayed request defeats the drafts’ first use.

**SOL-S67-03 — Low · material: no.** New source comments cite the reviewer and finding IDs ([review.go](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/review/review.go:580), [room.ts](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/review/room.ts:530)), contrary to the repository’s source-comment rule. The concrete effect is process history in application code; remove those references when the code is next edited. **R-124:** no effect on whether the slice works or is safe.

### What holds

SOL-S67-01’s requested-name and resolved-name checks reject case-folded `.GIT`, protected segments, local configuration in any directory, and static symlink aliases, including a symlink in a parent directory. The descriptor check rejects a leaf link swapped after resolution; invalid UTF-8 is refused ([document.go](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/project/document.go:357), [review.go](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/review/review.go:585)). A concurrent parent-directory swap remains possible; it requires a separate writer in the checkout and does not change the R-124 verdict on SOL-S67-01. The shared rule does not change admitted document reads because `admissibleID` already requires a `.md` ending.

A shaping source read now requires a standing sitting for the request’s human; another signed-in human’s sittings do not satisfy that lookup. The review-source branch still calls `Source` without this new condition ([review.go](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/httpd/review.go:38), [httpd.go](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/httpd/httpd.go:637)). These identity and review checks were traced in code; the new route test directly covers no sitting, but not a second human.

The timer is reset and cancelled on room changes, while the immediate keep and the blur, Step out, and pagehide calls remain. The cut guard permits one timer in `review/room.ts` and still rejects other timer sites, intervals, and timer names as strings. The fact anchor is a desk-opening press beside its record link. The checkout owner test needs no Git; the route test also reads a file created after the owner was set up.

I confirmed the builder’s failing-first accounts **against base code**, without executing tests on a base checkout: base admitted the protected and invalid-text reads, had no sitting check or `keepAfterSilence`, required zero timer sites, and rendered the fact anchor as text inside the link. At the fix commit, focused `npm test` passed **46 tests across three files**. Go tests could not start because the read-only sandbox refused Go’s temporary build directory. The new timer’s store wiring is tested by a source-text assertion rather than a mounted store, and I did not run the builder’s full gates.


## Scoped re-read of fix round 2

---

**SOL-S67-02 HELD.** The new sequence check prevents an older request from being accepted after a newer one, but it does not guarantee that accepted keeps reach the sitting mark in order. Under R-124, the slice is not safe without fixing that write path: a delayed write can still replace the latest draft.

| ID | Material finding | Evidence and required change |
| --- | --- | --- |
| **SOL-S67-02** | **High · held.** Two keeps can pass the sequence check in order and write the mark file in reverse order. | [Conversation.Keep](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/conversation.go:1365) releases its mutex before `writeState`. That function captures a state and then calls `os.WriteFile` outside the mutex. An older keep can capture sequence 1, pause, let sequence 2 write, then write sequence 1 last. This is reachable when pagehide sends alongside an ordinary keep, or from two pages. Serialize the check and persistence, or otherwise prevent an older captured state from publishing last. The new Go test sends its keeps sequentially, so it does not cover this interleaving. |
| **SOL-S67-04** | **High · new.** Pagehide can fail to send its keepalive request when an ordinary keep of the same words is pending. | [RoomKeeper.keep](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/review/room.ts:583) returns the pending ordinary request’s promise when the words match `last`, before reaching `send(..., leaving)`. Pagehide therefore sends no request with `keepalive: true`; the ordinary request may be lost as the page closes. Make the leaving path send immediately even for an identical in-flight snapshot. Under R-124, losing unfinished words on an ordinary departure makes this material. |

**What I checked.** A keep requested during a wait reads the room after the wait, and waiting calls with the same words coalesce. `open()` sets the counter to at least the held sequence, so its next number is above a mark with sequence 0 or a clock value below the held sequence. A failed ordinary keep clears the last-kept marker for a later change. The silence timer, blur, Step out, and pagehide still call `keepRoomNow`; its new guard skips rooms that have not been taken. I found no existing production keep call that omits a sequence. Two pages and a reload remain subject to the server write race above. **SOL-S67-03 is confirmed:** no comment added by this commit names a reviewer or finding ID; older comments outside this fix remain.

Against `d01874552`, I confirmed by reading the behaviors behind the builder’s failing-first lines: the old client sent keeps independently without a sequence, and the old server accepted the later arrival. Thus the Go test’s “newer words stand” and “kept when the newer words were” expectations, and the three new frontend ordering expectations, distinguish the intended fix from the base behavior. I did not reproduce the builder’s exact failing output on a base checkout. Focused `npm test` passed **43 tests in two files**.

I read commit `4ad1ac5394ab09481f285eff4269774eb205f484` against `d018745523a35a4ce1d6792811ef6b5f01487191` and the “Fix round 2” report. The generated minified bundle appeared in the diff but was not semantically inspected; I did not run Go tests or the full npm suite. Proposed review receipt, **unwritten**: “Fix round 2 remains open: serialize mark persistence and preserve pagehide keepalive for identical in-flight words.”

**VERDICT: 2 material findings: SOL-S67-02, SOL-S67-04**


## Scoped re-read of fix round 3

---

**SOL-S67-02 FIXED; SOL-S67-04 FIXED.** I found one material issue on the same state-file path: sign-in can miss a sitting while a keep writes its mark.

| Item | Read-only judgment |
| --- | --- |
| **SOL-S67-02 — reverse publication** | **FIXED.** [Conversation.Keep](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/conversation.go:1358) holds its mutex from the sequence check through publication and rollback. A later admitted keep cannot write first, and a failed earlier keep cannot roll back the later one. The injected [Go test](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/sitting_conversation_test.go:138) holds the first write back and distinguishes the old behavior. |
| **SOL-S67-04 — missing leaving request** | **FIXED.** [RoomKeeper.keep](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/review/room.ts:577) sends a numbered leaving request immediately when an ordinary keep is out, including when its words match. The [test](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/web/_app/src/review/room.test.ts:385) holds an ordinary keep back and checks for the second request. With different words, the existing test checks that the leaving request has the higher number. |

**SOL-S67-05 — high, material.** [Adopt](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/service.go:423) discovers seat sittings through `sittingsOn`, which [reads and silently skips an unreadable state file](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/conversation.go:1324). That read does not take the conversation mutex. If it occurs between the truncate and write in [publishState](/Users/wido/LocalStorage/GitHub/agentic-tools-sit/metasystem/internal/ui/partner/conversation.go:1424), `Adopt` can return success without moving the sitting. This is reachable at ordinary first use: a person can sign in while the room’s automatic keep is writing. Their newly named conversation then has no sitting or unfinished words. Under **R-124**, the slice is not safe without a fix; the artifact to change is sitting discovery during `Adopt`, coordinated with state publication. This race predates round 3; the new mutex does not cover the directory scan.

I found no new lock cycle: `Sit`, `Rise`, and `RecordSession` release the mutex before `writeState` takes it, while `Adopt` calls `writeStateHeld` under the two locks it already owns. Those writers capture the current state at publication, so they do not publish an older captured state. A failed `Keep` rolls back under its lock. A slow disk holds that conversation’s operations and can delay `Adopt`, but does not by itself deadlock them.

**Limitations:** I read commit `96647e2cf242e2676094c3fd570bee2c598c9a88` against `4ad1ac539`, the Fix round 3 report, the source diff, bundle manifest, and changed minified bundle tokens. I ran no tests under the read-only instruction and did not semantically inspect the entire generated bundle. Proposed receipt, unwritten: “Round 3 fixes ordered keeps and leaving keepalive; coordinate Adopt’s sitting scan with state writes.”

**VERDICT: 1 material findings: SOL-S67-05**

