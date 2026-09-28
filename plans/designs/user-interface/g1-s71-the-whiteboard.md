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
`evidence/room-1280-light.png` and the desk shows the screenshot large;
the report beside it is a section item. You step out and come back;
remarks, drawings and the desk are where you left them.

## 4. Decisions

- D1. **Remarks are stickies with two new subject kinds.** A file and
  range at a tip, and a record section: created from a desk selection
  (Remark beside Ask and Finding) or from the board, private as every
  sticky, shown on the board under Remarks and on the desk item they
  belong to as a marker on the lines, each with Record as a fact and
  Make a finding, which open the matching card with the remark's words
  and anchor filled.
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
- D4. **The evidence read.** One desk read, `evidence`, bounded to the
  record's `Evidence:` path: an image renders on the desk; a text
  report is a section item; anything else is refused in words. The
  Partner's `present` gains kind `evidence`, and the Behaves walk's
  request names the path so the Partner presents from it.
- D5. **Nothing else changes.** The piles, the recorder, the desk's
  reads and bounds, the mark's room state (the drafts carry remarks
  being written).

## 5. Step 1, the smallest thing that works

D1 to D5 as one slice, with D2's proof as its first task. Not in it:
the human drawing; a drawing edited after keeping; remarks shared with
another human; evidence outside the record's path; a diagram of the
whole design generated by the Partner unasked.

## 6. Payload and routes

Stickies gain subject kinds `source` `{record, path, from, to, tip}`
and `section` `{record, section}`; the stickies routes are unchanged.
`GET /api/review/<record>/evidence?path=` answers a file under the
record's `Evidence:` path (images and text, bounded to 4 MB and the
existing line bounds), refusing a path outside it. The mermaid chunk is
bundled and served under `script-src 'self'`; its styles carry the
page's nonce or are not written. Keep it is a whole-source edit
appending under `## Drawings`, marked `[d:<id>]`. `present` gains
kind `evidence`. The cut guard's call sites gain the read and the chunk.

## 7. Not here, later

The human's own drawings; editing a kept drawing; evidence from
another goal on the same desk; remarks that survive the sitting as
public annotations; a picture of the change itself.

## 8. Verification and box

Go: the evidence read inside and outside the path, image and text and
refused kinds; `present` with kind evidence; the Behaves request
naming the path. Frontend: the CSP proof test under the real header;
the chunk loading lazily and the source shown on failure; Remark from a
selection and from the board with both presses; the marker on the
lines; Keep it appending once across a reload and the board showing
it; the drafts carrying an unwritten remark across Step out; the cut
guard's rows. Walkthrough: a remark, a drawing kept, a screenshot on
the desk; screenshots at 1280 and 400, light and dark. Landing checks
as g1-s67 §8. Box: Astra's critique (two rounds), one Opus 5.5 lane,
one Sol read with one fix round; 150 to 240 job-minutes.

## 9. Self-grade

High on D1, D3 and D4: a subject kind, an append, a bounded read.
Medium on D2: the CSP proof is the risk, and the design says what
happens when it fails rather than hoping. Weakest: a kept drawing is
the Partner's picture of the code at one moment, and the record will
carry it after the code moves; the caption's question and the record's
date are the reader's warning.
