# Astra's critique of g1-s71

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against [g1-s71-the-whiteboard.md](g1-s71-the-whiteboard.md) at `81acda402`, under R-121 and R-124, brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-brief-whiteboard.md`. Verbatim; the dispositions are at the design's foot.

## Round 1

---

Three material findings remain. Step 1 is defined, and deferring human drawing does not invalidate its value. These findings concern first-use behavior, not completeness.

All evidence below was checked by reading commit `81acda402`; failure scenarios are inferred, not executed.

**S71-01 — High — material: yes — A remark’s saved tip cannot survive the unchanged desk contract honestly.**

**Claim:** D1 stores `{record, path, from, to, tip}`, but does not define what happens when the sitting advances beyond that tip. D5 leaves desk reads unchanged.

**Evidence:** [Design D1](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s71-the-whiteboard.md:64) and [payload](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s71-the-whiteboard.md:104). A source [DeskItem](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/room.ts:22) carries no tip; [Source](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/review/review.go:528) chooses its commit from the record’s current Reviewed head. After “Review the new tip,” [ReviewRoom](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/ReviewRoom.tsx:221) clears the moved warning.

**Concrete failure:** Write “this lock is taken twice” against lines 41–46 at A, then review B, where those lines contain different code. Opening the remark through the existing desk reads B. Matching only path/range mislabels new code; matching the saved tip hides the marker without explaining what happened. Carrying the existing textual anchor into a fact or finding also loses the tip.

**Change to the design:** Specify the smallest honest mismatch behavior: retain the remark on the board, visibly name its original tip, and do not attach it to current lines as though unchanged. Promotion must preserve that provenance. Historical reads and automatic reanchoring can wait. Define shaping remarks against g1-s67’s live-checkout rule without inventing a commit pin.

**Test 1 — DIFFERENT/WRONG:** Changes marker matching, opening behavior and promoted anchors; otherwise the original location can become a false claim.  
**Test 2 — WORKS/SAFE:** **Fails SAFE:** silently associates the human’s words with different code. Fixture: remark at A → advance to B → open and promote it.

**S71-02 — High — material: yes — Naming an external evidence directory does not let the Partner discover its files.**

**Claim:** D4 supplies the Behaves walk with the evidence path and adds `present(kind: evidence)`, but neither supplies the filenames needed to present evidence outside the checkout.

**Evidence:** [D4](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s71-the-whiteboard.md:86). The creator copies the design’s Evidence value [verbatim](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/review.go:129); its [fixture](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/review_test.go:17) uses `~/evidence/ledger-sync/`. g1-s65’s [Built section](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s65-the-review-sitting.md:591) likewise names an external directory. The Partner’s [permission owner](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/permission.go:159) limits native reads to the checkout; [present](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/uitools/review.go:43) only prepares a display suggestion.

**Concrete failure:** A record names an external evidence directory containing a screenshot and report whose filenames appear nowhere in checkout records. On the first Behaves walk, the Partner cannot enumerate that directory. The proposed HTTP file read helps the browser only after a filename is known. The Partner must refuse or guess.

**Change to the design:** Have the evidence owner provide a bounded file listing to the Behaves request, or an equivalent bounded Partner read. Keep discovery under the same Evidence boundary; no general filesystem grant is needed. Specify one path convention shared by that listing, `present` and the browser read.

**Test 1 — DIFFERENT/WRONG:** Adds the missing discovery input; a directory string alone cannot deliver the stated walk.  
**Test 2 — WORKS/SAFE:** **Fails WORKS:** the first walk cannot reliably put existing external evidence on the desk. Fixture: external directory, filenames absent from all checkout records, screenshot and report both presented.

**S71-03 — Medium — material: yes — A text evidence report cannot use the existing section item unchanged.**

**Claim:** D4 explicitly makes a text report a section item, but that item reads through the checkout’s Markdown document route, losing the evidence boundary.

**Evidence:** [D4 and D5](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s71-the-whiteboard.md:86). [SectionView](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/Desk.tsx:354) calls `loadDocument(record)` and selects a named heading. The document reader opens under the [checkout root](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/document.go:130), and [admissibleID](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/document.go:330) rejects absolute paths and non-`.md` files.

**Concrete failure:** The screenshot renders through the new evidence read, but its adjacent external `report.md` fails when opened as a section. A plain `report.txt` fails even inside the checkout. Successful Go tests of the new evidence endpoint would not catch this frontend routing failure.

**Change to the design:** Keep both images and reports as evidence-origin desk items, carrying the sitting record and evidence-relative path through restoration. Text may reuse the section renderer, but its bytes must come through the evidence read. Allow a report without a Markdown heading.

**Test 1 — DIFFERENT/WRONG:** Changes the text item’s read dispatch and retained identity.  
**Test 2 — WORKS/SAFE:** **Fails WORKS:** the specified report cannot be opened on first use. Fixture: external `.md` and headingless `.txt` reports open through `present`, then reopen from the restored desk.

**Deferred and non-material**

Human drawing, editing kept drawings and automatic remark reanchoring remain deferred. **Test 1: DIFFERENT** if added; **Test 2: WORKS/SAFE passes** without them, provided stale remarks are explicit. The human can already request, inspect and retain a Partner drawing; a drawing editor is not necessary for that first-use value.

**What I verified holds**

- D2 correctly puts CSP proof first and permits an explicit source-only fallback. The actual [policy](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/httpd/httpd.go:300) and [page nonce](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/httpd/httpd.go:540) exist. The current nonce test is only a Node stub, so it cannot substitute for the required browser proof.
- Same-origin JavaScript chunks have a serving path: the bundle script inventories generated files, embedding includes the bundle tree, and `serveFile` serves `.js`. Mermaid is absent from both package manifests at this commit; no pinned-version rendering claim is yet proven.
- D3 can use the existing [recorder rewrite queue](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/recording.ts:151), preserving record identity, serialization and conflict handling. The design already requires first-section creation and duplicate prevention across reload.
- Stickies already have private per-human storage. D5 and §8 explicitly require unfinished remarks to join the room drafts and survive Step out; that requirement is not missing.
- Evidence containment must be filesystem containment: `inside` is lexical, whereas the document reader demonstrates an anchored-root open. The new read must preserve that distinction.
- g1-s67 keeps findings review-only. Its purpose rule should govern remark promotion in shaping rooms.

**VERDICT: 3 material findings (fail test 2): S71-01, S71-02, S71-03**

Commit reviewed: `81acda402b39acf547bdac7a9af2f9a6e4c503b6`.

Limitations: read-only review; no tests, browser proof or Mermaid execution. g1-s67 is a closed design prerequisite, not implemented in this commit’s reviewed room code. No subagents, edits, local configuration or `artifacts/` reads. Proposed receipt, unwritten: “S71 design critique round 1: three material first-use findings; static review at 81acda402.”

