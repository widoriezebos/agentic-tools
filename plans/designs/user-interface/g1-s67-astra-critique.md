# Astra's critique of g1-s67

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against [g1-s67-the-room-for-every-sitting.md](g1-s67-the-room-for-every-sitting.md) at `bea2168e1`, under R-121 and R-124, brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-brief-room-every-sitting.md`. Verbatim; the dispositions are at the design's foot.

## Round 1

---

Three material findings remain in step 1. All evidence below is **read at `bea2168e1`**; failures are inferred from the specified changes and existing code.

**S67-01 — High · material: yes — The Partner and the desk can argue from different source.**

**Claim and evidence:** D2 pins the desk to the mark’s commit, while D3 preserves the shaping opening request and §6 changes only the browser’s source routes ([design:98](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:98), [design:143](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:143)). The Partner’s `document` reader returns the file “as it stands” through the checkout reader ([uitools.go:353](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/uitools/uitools.go:353), [document.go:139](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/document.go:139)). Its review reader accepts only review records and exposes changes/diffs, while `present` supplies a display suggestion without reading source ([uitools/review.go:20](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/uitools/review.go:20), [uitools/review.go:127](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/uitools/review.go:127)).

**Concrete failure:** Start a shaping sitting with an uncommitted change to `owner.go`. Today reads that changed file and presents its line anchor; the desk displays the committed version. HEAD has not moved, so the moved-head banner cannot expose the discrepancy. The supposed evidence beside the explanation is different code.

**Change to the design:** Require shaping code reads by the Partner and browser to resolve the same sitting’s recorded tree through the shared owner. Name the Partner-facing read and carry the commit into its evidence. Keep live record-section reads distinct from pinned code reads.

**Test 1 — DIFFERENT/WRONG:** Yes; the Partner needs a pinned source-read contract, beyond admitting `present`.  
**Test 2 — WORKS/SAFE:** No; the first Today walk can give an answer supported by different lines from those displayed.

**S67-02 — Medium · material: yes — Existing standing sittings have no defined tree.**

**Claim and evidence:** D2 and §6 write `tree` only at Start, while D6 promises access to every standing sitting ([design:98](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:98), [design:124](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:124)). Existing marks contain subject, purpose, start time and room state, but no commit ([conversation.go:552](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/conversation.go:552)). Opening a standing sitting resumes it rather than running Start again ([service.go:1177](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:1177), [ProjectPane.tsx:1590](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/project/ProjectPane.tsx:1590)).

**Concrete failure:** The first return after deployment to an already-standing design sitting supplies no tree to the desk. Refusing source reads makes that sitting unusable for the promised code discussion; silently substituting HEAD falsely treats today’s commit as the commit at Start. Restarting the sitting replaces its standing mark instead of resuming it.

**Change to the design:** Define the missing-tree case. Preserve the sitting and its words, state that no earlier code snapshot was recorded, and let “Read at the head now” establish and persist its first tree. Do not claim a comparison with its original Start.

**Test 1 — DIFFERENT/WRONG:** Yes; initialization on resume and the unknown-baseline behavior must be specified.  
**Test 2 — WORKS/SAFE:** No; an explicitly supported existing sitting otherwise has either a broken desk or invented provenance.

**S67-03 — Medium · material: yes — Removing the shaping End sheet loses its closing contract.**

**Claim and evidence:** D7 says End remains unchanged, but §6 removes `EndSittingSheet` ([design:129](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:129), [design:151](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:151)). That sheet owns both drafting a shaping Outcome and ending without recording ([StartSitting.tsx:293](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/StartSitting.tsx:293), [StartSitting.tsx:333](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/StartSitting.tsx:333)). The room unconditionally mounts its review End sheet; every choice drafts an Outcome through `closeSitting(verdict)` ([ReviewRoom.tsx:291](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/ReviewRoom.tsx:291), [ReviewRoom.tsx:323](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/ReviewRoom.tsx:323)).

**Concrete failure:** After the specified deletion, a shaping sitting inherits review verdict choices and loses “End without recording.” The review’s “no verdict” choice still requests an Outcome; it is not the removed action.

**Change to the design:** Explicitly make the shared room’s End sheet purpose-dependent: shaping retains its two existing actions without verdict controls; review retains its current behavior. Add a shaping End obligation to §8.

**Test 1 — DIFFERENT/WRONG:** Yes; the deletion must relocate the shaping closing behavior into the room.  
**Test 2 — WORKS/SAFE:** No; the first shaping sitting cannot perform an existing closing action promised unchanged.

**Deferred and non-material**

Structured intent items, drawings, multiple records and further phone layout work remain deferred as declared. The previously recorded persistence and multi-tab residuals do not justify additional machinery in this slice.

**What I verified holds**

- Step 1 and its deferrals are explicit.
- The shared-room premise fits the paper. Its independence rule concerns participation in the shaping conversation, and it explicitly permits the same form for human review ([paper:27](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/docs/paper/15-the-sitting.md:27), [paper:51](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/docs/paper/15-the-sitting.md:51)). Conversation switching closes the previous provider session and replays the selected conversation’s history ([service.go:704](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:704), [service.go:762](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:762)).
- `present` admission is separate from finding admission. The existing test’s rejection of shaping presentation must change, while its finding refusal remains ([review_service_test.go:444](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/review_service_test.go:444)).
- The Outcome recorder already applies the missing-verdict refusal only to reviews; the service clears verdict metadata on shaping deposits ([room.ts:328](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/room.ts:328), [service.go:1293](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:1293)). S67-03 concerns the closing interface, not that guard.
- D2 requires extending the shared owner: existing `Reviewed` has no standalone head case, source reads enter review comparisons, and movement resolves a goal branch ([review.go:138](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/review/review.go:138), [review.go:533](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/review/review.go:533), [review.go:315](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/review/review.go:315)). The shaping tree must also participate in the desk’s read key when “Read at the head now” succeeds ([room.ts:562](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/room.ts:562)).

VERDICT: 3 material findings (fail test 2): S67-01, S67-02, S67-03

Commit reviewed: `bea2168e1cabeb0f60b6c2904546b37d5beb1591`.

Limitations: static design/code review only; no tests or application execution, no edits, no subagents, and no reads of the prohibited paths. Proposed receipt, unwritten: “g1-s67 design critique, round 1: three material first-use findings.”

Codex session ID: 01a0e909-d5a3-7aa1-9cbf-5bfe1c0a8c4d
Resume in Codex: codex resume 01a0e909-d5a3-7aa1-9cbf-5bfe1c0a8c4d

## Round 2, the declared failsafe

---

All three prior findings are **CONFIRMED ANSWERED**. One additional material finding remains in the drawer retirement path. Evidence below is **read at `ead3a5cdb`**; the failure is inferred from the design and code, not reproduced.

**Prior findings**

- **S67-01 — CONFIRMED ANSWERED.** [D2, design:102](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:102) removes the committed/live-source mismatch: both reads now target the checkout, including uncommitted edits. The document reader opens `roots.Checkout` ([document.go:130](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/document.go:130)); native-read admission uses that checkout boundary ([permission.go:159](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/permission.go:159)). My earlier pinned-read proposal is unnecessary for this revised contract. **Test 1:** no further change required for the original failure. **Test 2:** passes; ordinary uncommitted edits no longer necessarily produce different evidence beside the answer.

- **S67-02 — CONFIRMED ANSWERED.** [Design:108](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:108) eliminates the required `tree`, and [design:206](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:206) covers a mark without room state. Existing marks need no invented baseline; their optional room field is consistent with this ([conversation.go:552](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/conversation.go:552)). **Test 1:** no further tree initialization is required. **Test 2:** passes for the missing-tree failure. S67-04 below concerns a different issue: which conversation owns an older mark.

- **S67-03 — CONFIRMED ANSWERED.** [D7, design:137](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:137) and [design:173](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:173) explicitly move the shaping sheet and retain its two actions. Those actions already call `closeSitting()` and `endWithoutRecording()` ([StartSitting.tsx:293](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/StartSitting.tsx:293)). **Test 1:** the necessary purpose-dependent sheet selection is specified. **Test 2:** passes for the lost closing contract.

**S67-04 — Medium · material: yes — Retiring the drawer strands a standing sitting still owned by the ordinary conversation.**

**Claim and evidence:** D6 promises that every standing sitting opens its room, while D7 leaves the conversation store unchanged ([design:132](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s67-the-room-for-every-sitting.md:132)). The current service explicitly includes a sitting marked on the ordinary conversation in `Standing` ([partner/review.go:155](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/review.go:155)); the project route publishes it under its subject’s record path ([httpd.go:591](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/httpd/httpd.go:591)).

Opening that record’s conversation does **not** recover the ordinary conversation’s mark or transcript: `openOf` opens the separate record-keyed files ([service.go:346](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:346), [conversation.go:628](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/conversation.go:628)). Today the ordinary drawer can still display this older sitting and offer End ([store.tsx:850](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:850), [StartSitting.tsx:228](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/StartSitting.tsx:228)).

**Concrete failure:** Upgrade with a standing shaping mark and its conversation in the ordinary files, and no record-keyed conversation. Project → Sittings lists it as standing. Opening `/sitting/<record>` reads an empty, unmarked conversation, so resume cannot run and End cannot close the original sitting. D6 removes the remaining drawer controls. This is a supported pre-existing sitting becoming unusable on its first return; it does not require concurrent tabs or a failed write.

**Change to the design:** Define how a room recovers a standing shaping sitting owned by the ordinary conversation before retiring its drawer controls. Preserve its words and original sitting; do not implement recovery by Start. Extend §8 with a fixture containing an ordinary-conversation mark and transcript but no record-keyed files: opening the room must recover that sitting, and End must act on it. The existing “no room state” fixture does not cover this ownership distinction.

**Test 1 — DIFFERENT/WRONG:** **Yes.** The implementation needs an explicit recovery rule at the conversation-opening boundary; routing every row directly to separate record-keyed files builds the wrong behavior.

**Test 2 — WORKS/SAFE:** **WORKS: no.** The promised existing sitting cannot resume or end. No destructive write is needed to demonstrate the failure.

**Deferred and non-material**

Anchor drift after a read, refresh while remaining in the room, stronger provenance for uncommitted bytes, and the landed review Partner’s native-read mismatch remain deferred. They do not establish another first-use failure under this revision’s explicit live-checkout contract. Structured intent items, drawings and further phone layout work remain outside step 1.

**What I verified holds**

- **D2 specifies the correct root boundary.** The document reader uses an anchored `os.OpenRoot`/`root.Open` ([document.go:130](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/document.go:130)). `Owner.Source`’s lexical path check alone would not provide filesystem containment ([review.go:627](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/review/review.go:627)); the design’s root-based open and escape-refusal obligation matter. Binary refusal and range limits already exist ([review.go:548](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/review/review.go:548)).
- **Return reads are feasible without a mark commit.** Desk reads run on mounting through `useRead` ([Desk.tsx:87](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/Desk.tsx:87)). Removing shaping comparisons, diffs and the banner avoids falsely reporting commit movement as working-tree freshness.
- **Presentation does not grant finding or verdict authority.** Finding admission remains independently review-only ([service.go:1284](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:1284)). Revision 2 explicitly changes the presentation test while retaining its finding refusal.
- **Shaping Outcomes remain recordable without verdicts.** `outcomeShape` applies the refusal only to reviews ([room.ts:328](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/room.ts:328)); the service clears verdict metadata before conditionally attaching a review verdict ([service.go:1293](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:1293)).
- **The shared-room premise fits the paper.** Its independence rule concerns participation in the shaping conversation ([paper:27](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/docs/paper/15-the-sitting.md:27)); it explicitly permits the same sitting form for human review ([paper:51](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/docs/paper/15-the-sitting.md:51)). Existing session switching closes the previous session and replays the selected conversation’s history ([service.go:704](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:704)).

VERDICT: 1 material finding (fail test 2): S67-04

Commit reviewed: `ead3a5cdb5609d0642d8f7276e7766a600c5dc9e`.

Limitations: static review only; no tests or application execution, edits, subagents, or prohibited-path reads. I did not inspect private persisted conversations, so I have not established whether this checkout currently contains the older mark described in S67-04. Proposed receipt, unwritten: “g1-s67 failsafe design critique: three prior findings answered; one material sitting-recovery finding.”

Codex session ID: 01a0e916-3a00-7c71-8f72-46c41339b5a9
Resume in Codex: codex resume 01a0e916-3a00-7c71-8f72-46c41339b5a9
