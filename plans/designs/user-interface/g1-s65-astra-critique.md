# Astra's critique of g1-s65 revision 2

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against revision 2 of [g1-s65-the-review-sitting.md](g1-s65-the-review-sitting.md) at `2bfac8174` (code at `647234149`), under R-121 and R-124, brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-brief-review-room.md`. Verbatim; the dispositions are at the design's foot.

---

Three material findings in slice A.

Reviewed **revision 2, design commit `2bfac8174bbb4511b65a1bc64b58954961fc8daf`**, with code evidence at **`647234149`**. All references below use that code revision.

The report could not be saved to `/Users/wido/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-r1-review-room.md`: this session permits filesystem reads only. The complete report follows.

**S65-01 — High — material: yes**

**Claim:** Findings written only after an answer cannot support the specified record-derived unanswered count.

**Evidence read:** [Design D8–D10](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s65-the-review-sitting.md:249) writes a finding when answered but derives the door’s counts from the review record. [store.tsx:1468](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:1468) derives the existing table exclusively from record contents. [sittings.go:128](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/sittings.go:128) likewise counts recorded entries. Pending cards instead come from the transcript and browser marks at `store.tsx:1323`.

**Concrete failure:** The Partner offers one finding; the human leaves it unanswered and steps out. D8 has written no Findings entry, so D9’s record-derived door cannot report “1 finding, 1 unanswered.” It reports an empty review despite the outstanding card. D10 also cannot use that same record-only account to identify everything that must block Clear to land.

**Change to the design:** Name the existing private sitting state as the owner of pending finding identities, and make the door and End use pending findings together with recorded answers. Remove a pending finding only after its answer’s single record write succeeds.

**Test 1 — DIFFERENT/WRONG:** yes; the count and End need a source that includes unanswered findings.  
**Test 2 — WORKS/SAFE without it:** fails WORKS and SAFE; the first unanswered finding produces a false account of the review.

**S65-02 — High — material: yes**

**Claim:** The merge-base-to-tip comparison specified for the desk yields an empty diff for an ordinary completed goal whose commit is already on main.

**Evidence read:** [Design D1–D4](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s65-the-review-sitting.md:201) admits completed goals, resolves their `Goal-Item` commits, and specifies the change index between main’s merge base and the reviewed tip. [Section 6](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s65-the-review-sitting.md:333) repeats that comparison. [held.go:307](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/held.go:307) parses `Goal-Item` as a goal association; it supplies no review comparison base.

**Concrete failure:** Goal G’s single commit C changes `owner.go` and is landed on main. The human opens Review it on G’s completed page. D1 finds C. Because C is an ancestor of main, their merge base is C; D4 therefore compares C with C. The change index and hunks are empty, although G changed code. This is a static inference from the specified comparison, not a runtime observation.

**Change to the design:** Define the completed-goal comparison separately: show the parent-to-commit changes of the trailer-linked commits and identify the tree used for source reads. Keep the merge-base comparison for the waiting candidate.

**Test 1 — DIFFERENT/WRONG:** yes; the completed-goal diff must use different endpoints.  
**Test 2 — WORKS/SAFE without it:** fails WORKS and SAFE; a supported first review silently shows no changes for changed work.

**S65-03 — Medium — material: yes**

**Claim:** Persisting only desk state and face does not preserve the human’s unfinished finding or acceptance reason when they leave and return.

**Evidence read:** [Design D7–D9](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s65-the-review-sitting.md:245) introduces human-written findings, reuses the Decide sheet, and persists `desk` and `face`. [Deposit.tsx:361](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/Deposit.tsx:361) keeps the decision text and reason in component-local `useState`. [store.tsx:678](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:678) keeps unrecorded deposit edits in browser memory; edits at `store.tsx:1373` do not persist them. [conversation.go:544](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/conversation.go:544) contains no such drafts in the existing sitting mark.

**Concrete failure:** The human writes an acceptance reason but has not pressed Record. They step out, close the browser, and return through the door. The desk is restored under D9, but the reused Decide sheet starts with an empty reason. The record and transcript cannot reconstruct words that were never submitted.

**Change to the design:** Preserve unfinished finding text and answer-sheet fields in the existing private sitting state and restore them on return. Keep these drafts separate from the authoritative record until the human answers.

**Test 1 — DIFFERENT/WRONG:** yes; the persisted working state must include the human’s unfinished edits.  
**Test 2 — WORKS/SAFE without it:** fails SAFE; the ordinary leave-and-return flow loses what the human wrote.

## Deferred and non-material

**S65-04 — Low — material: no**

**Claim:** Section 8 names a code-review model that the repository’s current roster forbids.

**Evidence read:** [Design:384](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s65-the-review-sitting.md:384) specifies Codex on Sol for code reads. [project-rules-local.md:8](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/development/project-rules-local.md:8) assigns review of Opus-built code to Claude on Fable and forbids Sol delegates.

**Concrete failure:** Executing the stated build box dispatches an unauthorized reviewer; it does not change the room’s product behavior.

**Change to the design:** Correct the reviewer assignment in §8 to the authorized roster.

**Test 1 — DIFFERENT/WRONG:** no for what step 1 builds.  
**Test 2 — WORKS/SAFE without it:** passes both for the product slice; this is a coordination correction.

Slices B, C and D remain deferred. Their missing drawings, evidence presentation, runtime controls and landing enforcement are not material findings against A.

**What I verified holds**

- **The premise and subject fit.** The paper’s review sitting produces findings and recorded human judgments (`15-the-sitting.md:49`). The Review lane actually represents claimed work waiting to land (`internal/backlog/project.go:203`). A record-only first slice is consistent with the explicitly agreed cut.
- **The existing recorder is a usable foundation.** `recording.ts:102` serializes writes against its current reading and rereads after conflict. Deposit marks support reload recognition. The Outcome writer preserves multiline text, so a leading `Verdict:` line is representable. D8 explicitly requires extending the single-clause grammar to carry both Anchor and Answer.
- **The existing sheets provide the stated hooks.** `OpenSheet.tsx:192` calls `onDone` after `openGoal` succeeds. The Decide sheet requires a reason. These support the proposed ordering; they do not themselves prove the new follow-up workflow’s once-only behavior.
- **D3 correctly demands a new isolation boundary.** Existing `service.go:629` restores conversation history into fresh sessions, and `service.go:690` separately includes earlier proposal outcomes. D3’s “none of the conversation’s earlier turns” requirement must cover both paths, including recovery. Merely starting a new provider session would not satisfy the design.
- **The cited reader and guards exist.** The document reader is checkout-relative and Markdown-only; it is not a source-tree reader. The cut guard checks exact network call sites, and page responses install a fresh CSP nonce. The public-form text guard also scans interface source.

**Actual limitations:** This was a static design review. No tests, browser walkthrough, provider session, or Git-tree endpoint implementation were run. New endpoint security, layout behavior, follow-up recovery and persistence remain implementation proof obligations. I read the complete design and the cited behavior owners; I did not exhaustively audit every transitive implementation path. No subagents, product edits, secret configuration or files under `artifacts/` were used.

Proposed receipt, unwritten: “g1-s65 revision 2 independently reviewed against `647234149`; three material slice-A findings; static evidence only.”

VERDICT: 3 material findings (fail test 2): S65-01, S65-02, S65-03

Codex session ID: 01a0e6b3-840d-70c2-be8b-da2e9d0ad23f
Resume in Codex: codex resume 01a0e6b3-840d-70c2-be8b-da2e9d0ad23f
