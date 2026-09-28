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
