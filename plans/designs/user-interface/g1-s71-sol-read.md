# Sol's read of the g1-s71 build

Produced 2026-09-29 by Codex on `gpt-6-sol`, read-only, against branch `whiteboard` at `386813ac1` (base `f60cc6b7f`), brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/sol-code-read-brief-whiteboard.md`; the re-read of fix round 1 against branch `board-landing` (the build merged with main `de8bf4e3e` at `c50ae307c`, the fixes after it), brief `sol-code-reread-brief-whiteboard.md`. Verbatim, with the Codex session lines removed.

## Read 1

---

**Verdict: two material findings.** The evidence listing can exhaust a server request on a large directory, and a shaping remark can be described as current after a return to the Board face without checking the edited lines. This is a read-only review of `f60cc6b7f..386813ac1`; I made no product edits.

## Findings

**SOL-S71-01 — High; material: yes.** The evidence listing applies its 500-entry limit only after walking and storing every eligible file. [EvidenceList](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/review/evidence.go:135) appends every entry during `fs.WalkDir`, then sorts and truncates at [lines 162–167](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/review/evidence.go:162). A review whose `Evidence:` path names a very large or deep tree can make **The evidence** or the Behaves walk stall or consume unbounded memory before either receives the promised bounded listing. The test covers 503 files, so it does not exercise this failure. **Fix:** bound traversal work and retained entries, and report truncation without first collecting the entire tree; test a large, deep fixture. **R-124:** yes—the evidence walk can stop working at first use of such a valid path.

**SOL-S71-02 — Medium; material: yes.** A shaping remark on the Board face has no fresh line read after Step out and reload. The provider clears `deskRead` on conversation load ([store.tsx:994–995](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/partner/store.tsx:994)); the room mounts either Desk or Board ([ReviewRoom.tsx:375](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/review/ReviewRoom.tsx:375)); and the Board labels a remark as being on the file when no read is available ([remarks.ts:106–116](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/review/remarks.ts:106)). If the saved face is Board, the human edits the remarked lines without committing, then returns, no `SourceView` runs to discover the changed bytes. The Board keeps saying “on lines … of …” indefinitely instead of “at an earlier reading.” The screen test supplies an edited `deskRead` directly rather than exercising this return path ([whiteboard.screens.test.tsx:136](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/review/whiteboard.screens.test.tsx:136)). **Fix:** refresh the relevant source before asserting a shaping remark’s current location on the Board, or use provenance wording until a fresh comparison is available; test a return with Board as the saved face. **R-124:** yes—this is the slice’s core safeguard against presenting a remark as attached to changed code.

**SOL-S71-03 — Low; material: no.** The new `source` sticky subject may be saved without a commit. [The design](/Users/wido/LocalStorage/GitHub/agentic-tools-board/plans/designs/user-interface/g1-s71-the-whiteboard.md:147) includes `commit`; [validation](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/stickies/stickies.go:406) checks its shape only when nonempty, and a shaping read silently permits an unresolved `HEAD` ([review.go:640](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/review/review.go:640)). The concrete failure would be a remark without the provenance D1 promises if `HEAD` resolution fails. **Fix:** require a commit for new source remarks and give a readable refusal when it cannot be obtained. **R-124:** no demonstrated first-use failure in the normal committed checkout; record this for later.

## Departures from the design

| Design | Implementation |
| --- | --- |
| D4 promises a bounded listing ([design:120](/Users/wido/LocalStorage/GitHub/agentic-tools-board/plans/designs/user-interface/g1-s71-the-whiteboard.md:120)). | The response has at most 500 entries, but traversal and allocation are unbounded ([evidence.go:143–167](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/review/evidence.go:143)). SOL-S71-01. |
| D1 and its named return fixture require the Board to say “at an earlier reading” after edited lines are reread ([design:89–95](/Users/wido/LocalStorage/GitHub/agentic-tools-board/plans/designs/user-interface/g1-s71-the-whiteboard.md:89), [design:179–182](/Users/wido/LocalStorage/GitHub/agentic-tools-board/plans/designs/user-interface/g1-s71-the-whiteboard.md:179)). | Returning directly to Board makes no read, and the no-read branch uses current-location wording ([remarks.ts:106–116](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/review/remarks.ts:106)). SOL-S71-02. |
| The payload names a commit for a source remark ([design:147](/Users/wido/LocalStorage/GitHub/agentic-tools-board/plans/designs/user-interface/g1-s71-the-whiteboard.md:147)). | The server accepts an empty one ([stickies.go:406–415](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/stickies/stickies.go:406)). SOL-S71-03. |
| §6 lists the new `evidence` desk item ([design:153](/Users/wido/LocalStorage/GitHub/agentic-tools-board/plans/designs/user-interface/g1-s71-the-whiteboard.md:153)). | The build also adds a `drawing` item ([room.ts:29–35](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/review/room.ts:29)), needed to fulfill D3’s **Put on the desk** press. This addition is nonmaterial. |
| D2 says “one lazily loaded chunk” ([design:97–100](/Users/wido/LocalStorage/GitHub/agentic-tools-board/plans/designs/user-interface/g1-s71-the-whiteboard.md:97)). | There is one lazy **entrypoint**, `render.js`, plus Mermaid’s supporting chunk files ([bundle.json:1656](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/bundle/bundle.json:1656)). The physical file count exceeds one; the dependency loading boundary still appears to be the drawing. |
| D4 says evidence text uses the desk’s line bounds ([design:120–122](/Users/wido/LocalStorage/GitHub/agentic-tools-board/plans/designs/user-interface/g1-s71-the-whiteboard.md:120)). | The builder chose the desk’s largest bound, 3,000 diff lines ([evidence.go:38–42](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/review/evidence.go:38)), rather than the 400-line source view. The design does not resolve which bound was intended; I did not count this interpretation as material. |

## Deferred

I could not run the browser CSP proof or other tests in this read-only sandbox. `go test` stopped before running a test because it could not create its temporary build directory. The builder’s reported green runs and walkthrough screenshots were read as claims, not independently verified here.

For the added prototype attack target, [render.ts:36–43](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/drawing/render.ts:36) queues drawing renders, and [render.ts:86–93](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/drawing/render.ts:86) restores the patched methods when a render resolves or throws. A different component creating a style element during that interval also receives the drawing’s page nonce. I found no concrete first-use defect from that interaction. A throw *during patch installation*, before the `try`, would leave earlier assignments in place, but I found no reachable cause in this build and could not exercise the browser path.

## What I verified holds

**Read:** The evidence file read uses `os.Root`, refuses escapes and unsupported extensions, limits a file to 4 MB, and truncates text by line count ([evidence.go:182–230](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/review/evidence.go:182)). Keep it writes through the recorder, places a marked fence under `## Drawings`, and detects an existing drawing after rereading the record ([drawings.ts:152–185](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/drawing/drawings.ts:152)). The current source marker checks the review commit or shaping bytes ([remarks.ts:70–85](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/review/remarks.ts:70)).

**Read:** The CSP test constructs a page with the policy taken from the server constants and renders with the Vite-built module in Chromium ([csp.test.ts:58–125](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/drawing/csp.test.ts:58)); I could not rerun it. The built HTML loads `index.js`; that file dynamically imports `render.js`, and the source imports Mermaid only in [render.ts:1](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/drawing/render.ts:1). I found no other import of the render module or Mermaid in the application source. Chunk-load and parse failures show the source with words explaining the refusal ([drawings.ts:199–211](/Users/wido/LocalStorage/GitHub/agentic-tools-board/metasystem/internal/ui/web/_app/src/drawing/drawings.ts:199)).

**Run:** No tests completed. The attempted focused Go test failed at temporary-directory creation, before compilation.

## Limitations

This was a source and diff review, with selective inspection of the generated bundle’s entry and import relationships; I did not inspect every generated Mermaid chunk or the builder’s screenshots. The proposed review receipt remains **unwritten**: “Whiteboard code read at `386813ac1`: two material findings on evidence-list traversal and shaping-board remark freshness; independent tests blocked by the read-only sandbox.”

`VERDICT: 2 material findings: SOL-S71-01, SOL-S71-02; commit read 386813ac1caee03c0dce22425fb4e733a94a8c77`


## Re-read of fix round 1

---

**Verdict: one material finding remains.** The walk is bounded, but a cut listing with no files found can still tell the Partner that the evidence path contains nothing.

| Prior finding | Ruling |
| --- | --- |
| **SOL-S71-01 — HELD** | [The walk](/Users/wido/LocalStorage/GitHub/agentic-tools-board-land/metasystem/internal/ui/review/evidence.go:172) stops at 5,000 visited entries or depth 16, retains at most 500 files, and sorts them. Counting further files without retaining them conforms to the author’s refinement. But if the bound is reached before any eligible file is found, `Cut` is true and the listing is empty. [EvidenceNote](/Users/wido/LocalStorage/GitHub/agentic-tools-board-land/metasystem/internal/ui/partner/review.go:167) then says the path “holds no image or text” and omits the cut. [The desk](/Users/wido/LocalStorage/GitHub/agentic-tools-board-land/metasystem/internal/ui/web/_app/src/review/Desk.tsx:546) makes the same empty claim before displaying its cut notice. A valid screenshot below depth 16 is enough to reproduce this path. **R-124: yes**—the Behaves walk can falsely say no evidence was recorded on its first use of that path. The empty-and-cut branches and their tests need correction. |
| **SOL-S71-02 — FIXED** | [Board wording](/Users/wido/LocalStorage/GitHub/agentic-tools-board-land/metasystem/internal/ui/web/_app/src/review/remarks.ts:98) calls a shaping remark current only when the session’s source read covers the same file and full range with matching bytes. No read, another file, or another range yields “at an earlier reading”; the review-remark branch is unchanged. The new Board-face tests cover the return without a read, matching bytes, the remark’s words and Open press, and review wording. |

**New findings:** none; the remaining cut-disclosure defect keeps SOL-S71-01 open.

**Coverage:** The builder reports `internal/ui/review` at **87.5%** against its **86.0%** floor, and `internal/ui/httpd` at **88.7%** against **88.5%**. I read those figures; I did not rerun coverage.

**Limitations:** This was a read-only review of `c50ae307c..7ffceee08`, including the generated bundle’s changed tokens. I ran no tests or browser checks under the brief’s command restriction. No files were edited; the review receipt remains unwritten.

`VERDICT: 1 material findings: SOL-S71-01; commit read 7ffceee0801e4dd85401d6dcff0bc6c942de0db3`

