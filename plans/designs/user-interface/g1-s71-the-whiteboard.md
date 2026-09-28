# g1-s71: the whiteboard — remarks, drawings and evidence on the desk; the review room's slice B

- Kind: design
- Id: 01M3MPCEHFKB0DD8ZD35H11TBK
- Status: draft
- Goals: browser-interface

Wido, 2026-09-28: "ok, build all". This page is the step-1 cut of
slice B of the review room (g1-s65 D11 to D13). Author Fable. Every
cite re-read at `12611b98c`. Builds on g1-s67 (the board is every
sitting's).

## 1. What exists and binds

1. **The board as landed** (g1-s65 slice A, g1-s67): the desk pane's
   second face shows the record's piles, each entry pinned to its
   anchor; pressing an anchor puts it on the desk. Drawings are code
   blocks: a mermaid fence in the conversation or a record renders as
   text (`web/_app/src/project/Markdown.tsx:127`).
2. **Stickies** are private remarks with a subject (`web/_app/src/
   stickies/stickies.ts`), shown on the page of their subject; today a
   subject is a goal or a document.
3. **The CSP** is `default-src 'none'; script-src 'self'; style-src
   'self'` plus a per-page nonce (`internal/ui/httpd/httpd.go:300,
   541`; `web/_app/src/nonce.ts`). A library that writes a `<style>`
   element into an SVG it renders needs that nonce on the element or a
   mode that writes no styles; otherwise the diagram renders unstyled or
   not at all, silently.
4. **Evidence** of a run lives under the record's `Evidence:` path
   (screenshots and reports the build lane wrote), and the Behaves walk
   is the walk that asks for it (`internal/ui/partner/review.go:48`).
   The desk reads source, changes, diffs and record sections, bounded
   to the reviewed tree; it has no read for a file outside it.

## 2. What you want on a whiteboard

- **B1. A remark where it belongs.** A note on these lines, on this
  section, that stays with the sitting and shows up on the board and
  on the item.
- **B2. A drawing I can keep.** The Partner draws the flow; I see the
  picture, not the fence; I keep it, and the record keeps it with the
  question that produced it.
- **B3. The evidence in the room.** The screenshots the builder took,
  the report it wrote, on the desk beside the code, without leaving.
- **B4. Everything on one board.** Piles, remarks and kept drawings,
  each pinned to where it belongs.

## 3. The moment

You select six lines on the desk and press Remark: a small sticky opens
beside them, you write "this lock is taken twice", it sits on the
lines and on the board under Remarks, with Record as a fact and Make a
finding. You ask the Partner how the handoff works; it answers with a
mermaid sequence; the picture renders in the conversation; you press
Put on the desk, then Keep it: the drawing goes under the record's
Drawings section with your question as its caption, and the board
shows it. You press the Behaves walk; the Partner presents
`room-1280-light.png` from the listing it was handed and the desk shows
the screenshot large; the report beside it opens the same way, its
text on the desk. You step out and come back; remarks, drawings and
the desk are where you left them. You press Review the new tip; the
remark leaves the lines and stays on the board as "on lines 41-46 at
9c1f0a2", since the desk now reads other code.

## 4. Decisions

- D1. **Remarks are stickies with two new subject kinds.** A file and
  range at a commit, and a record section: created from a desk
  selection (Remark beside Ask and Finding) or from the board, private
  as every sticky, shown on the board under Remarks and on the desk
  item they belong to as a marker on the lines, each with Record as a
  fact and Make a finding, which open the matching card with the
  remark's words and anchor filled, the anchor carrying the remark's
  commit. The commit is the one the desk read the lines at: the review
  record's reviewed tip in a review room, and in a shaping room, where
  the desk reads the checkout as it stands (g1-s67 D2), the checkout's
  head at the moment of the remark, kept as provenance only, never as
  a pin. When the desk no longer reads at that commit (Review the new
  tip in a review, or the head moved under a shaping sitting), the
  remark stays on the board under Remarks with its commit named,
  "on lines 41-46 at 9c1f0a2", and is not drawn as a marker on lines
  that may be other code; opening it puts the path and range on the
  desk as the desk reads now, with the remark's commit beside it, and
  a fact or finding made of it keeps that commit in its anchor. A
  desk item carries no commit of its own (`review/room.ts:22`; the
  source read takes the record's current head, `review/review.go:528`),
  so the marker's match is path, range and commit, all three.
  Findings from a remark stay review-only, as g1-s67 rules.
- D2. **Drawings render, and the CSP is proven first.** A mermaid fence
  renders in the conversation, on the desk and on the board through one
  lazily loaded chunk; the source stays a press away and is shown
  instead when the chunk fails to load or the parse fails, in words.
  Before any of this, the build proves the CSP: the nonce carried onto
  every style the library writes, or a mode that writes none, held by a
  test that renders under the real policy header; if neither can be
  had, D2 waits, the build says so in its report, drawings stay code
  blocks, and the rest of this slice lands.
- D3. **Keep it.** A drawing in the conversation offers Put on the desk
  and Keep it; Keep it appends the fence under the record's Drawings
  section (created on first keep) with the question that produced it
  as the caption, through the record writer under the sitting's reading
  rule, marked so a reload cannot keep it twice; the board shows kept
  drawings under Drawings, each opening on the desk.
- D4. **The evidence read, and the listing that makes it findable.**
  The record's `Evidence:` path is copied from the design's line and is
  usually outside the checkout (`project/review.go:129`, the fixture's
  `~/evidence/…`), where the Partner's own reads do not go
  (`partner/permission.go:159`), so naming the directory tells the
  Partner nothing about what is in it. One evidence owner, bounded to
  that path by an anchored root open (as the document reader is,
  `project/document.go:130`; the lexical `inside` is not containment),
  answers two reads: a listing (relative path, kind, size, at most 500
  entries, images and text only) and one file (an image, or text up to
  the desk's line bounds, 4 MB). The Behaves walk's request carries
  the listing, so the Partner presents from what exists; `present`
  gains kind `evidence` with the evidence-relative path, the one path
  convention the listing, `present` and the browser read share. On the
  desk an evidence file is a desk item of its own kind, `evidence`
  `{record, path}`, whatever its type: an image renders large; text is
  shown with the section renderer over the bytes the evidence read
  returned, headings or none (a headingless report is a report), never
  through the document route, which is the checkout's and Markdown's
  only (`Desk.tsx:354`, `document.go:330`); the item is restored from
  the mark's desk with the record and the relative path, so a reload
  reads it the same way. Anything else is refused in words.
- D5. **Nothing else changes.** The piles, the recorder, the desk's
  reads and bounds, the mark's room state (the drafts carry remarks
  being written).

## 5. Step 1, the smallest thing that works

D1 to D5 as one slice, with D2's proof as its first task. Not in it:
the human drawing; a drawing edited after keeping; remarks shared with
another human; evidence outside the record's path; a diagram of the
whole design generated by the Partner unasked.

## 6. Payload and routes

Stickies gain subject kinds `source` `{record, path, from, to, commit}`
and `section` `{record, section}`; the stickies routes are unchanged.
`GET /api/review/<record>/evidence` answers the listing and `GET
/api/review/<record>/evidence?path=` one file under the record's
`Evidence:` path (images and text, bounded to 4 MB and the existing
line bounds), both through an anchored root open of that path,
refusing a path outside it; `DeskItem` gains `{kind: "evidence",
record, path}`. The Behaves request carries the listing. The mermaid
chunk is bundled and served under `script-src 'self'`; its styles
carry the page's nonce or are not written. Keep it is a whole-source
edit appending under `## Drawings`, marked `[d:<id>]`. `present` gains
kind `evidence`. The cut guard's call sites gain the two reads and the
chunk.

## 7. Not here, later

The human's own drawings; editing a kept drawing; evidence from
another goal on the same desk; remarks that survive the sitting as
public annotations; a picture of the change itself.

## 8. Verification and box

Go: the listing and the file read inside the path, an escaping path
and a symlink out of it refused, image and text and refused kinds, a
path outside the checkout served; `present` with kind evidence; the
Behaves request carrying the listing. Frontend: the CSP proof test
under the real header; the chunk loading lazily and the source shown
on failure; Remark from a selection and from the board with both
presses; the marker on the lines at the remark's commit and not after
Review the new tip, where the board names the commit and a fact made
of it keeps it; a remark in a shaping room carrying the head as
provenance; an external `.md` report and a headingless `.txt` opened
through `present` and again from the restored desk after a reload;
Keep it appending once across a reload and the board showing it; the
drafts carrying an unwritten remark across Step out; the cut guard's
rows. Walkthrough: a remark, a drawing kept, a screenshot on
the desk; screenshots at 1280 and 400, light and dark. Landing checks
as g1-s67 §8. Box: Astra's critique (two rounds), one Opus 5.5 lane,
one Sol read with one fix round; 150 to 240 job-minutes.

## 9. Self-grade

High on D3: an append. High on D1 and D4 since revision 2: a remark
that names its commit and steps off lines that moved, and an evidence
owner that lists before it serves. Medium on D2: the CSP proof is the
risk, and the design says what happens when it fails rather than
hoping; mermaid is in neither package manifest yet, so the pinned
version is the build's first choice. Weakest: a kept drawing is the
Partner's picture of the code at one moment, and the record will carry
it after the code moves; the caption's question and the record's date
are the reader's warning.

## Dispositions (Astra round 1, 2026-09-28, under R-121 and R-124)

Read of revision 1 at `81acda402`, verbatim in
`g1-s71-astra-critique.md`. Three material findings, all folded; every
cited line re-read at whole-function depth before folding.

| id | finding | fold |
|---|---|---|
| S71-01 | a remark saved at a tip has no defined fate when the desk reads another commit: a desk item carries no commit (`room.ts:22`), the source read takes the record's current head (`review.go:528`), Review the new tip clears the warning (`ReviewRoom.tsx:221`); matching path and range alone mislabels new code | D1: the marker matches path, range and commit; after the desk's commit changes the remark stays on the board naming its commit and is not drawn on the lines; opening it and promoting it keep the commit; in a shaping room the head is provenance, not a pin |
| S71-02 | the `Evidence:` path is copied from the design and usually lies outside the checkout (`project/review.go:129`, the fixture), where the Partner's native reads do not go (`permission.go:159`); `present` prepares a suggestion only (`uitools/review.go:43`): the Partner cannot learn a filename | D4: one evidence owner answers a bounded listing and one file; the Behaves request carries the listing; one evidence-relative path convention for the listing, `present` and the browser read |
| S71-03 | a text report as a section item reads through the document route, which is the checkout's and Markdown's only (`Desk.tsx:354`, `document.go:130`, `:330`): an external `report.md` or a `report.txt` cannot open | D4 and §6: an `evidence` desk item of its own, images and text alike, text rendered over the evidence read's bytes with or without headings, restored from the mark by record and relative path |

Astra also verified, and the design leans on, that the policy header
and the page nonce exist (`httpd.go:300`, `:540`) while the nonce test
is a Node stub and no substitute for the browser proof; that
same-origin chunks have a serving path; that Keep it can use the
recorder's rewrite queue (`recording.ts:151`); and that evidence
containment must be an anchored root open, since `inside` is lexical.
