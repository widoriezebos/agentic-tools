# g1-s31 The conversation, tidy

- Kind: design
- Id: 01M37Z5QMVNDMH0MYCF8AXY2B8
- Status: done
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-23, Claude on Fable, on Wido's review of the Partner's drawer: "the layout of the text and the questions that I can click on is a bit messy. It just doesn't look tidy ... make that just a bit more tight, elegant, more intuitive, easier to adjust, easier on the eyes." A presentation slice over g1-s28 and g1-s29: nothing the Partner does changes; how the conversation looks and reads does. Not sent to Astra: it changes no behaviour.

## What is wrong

In one drawer view there are seven things at four alignments: the Seeing line, the clipped tail of an answer, a stamp carrying a forty-character hash and an ISO timestamp, two floating chips, a monospace composer, a hint line, and a Send button in the far corner. Nothing groups, nothing aligns, and prose runs the full width of the work area.

## The design

1. **One measure.** The conversation is one column, at most 72 characters of the reading face wide, left-aligned with the page's gutter; the drawer and the focused page share it. Prose never runs the width of the screen.
2. **Two voices, one column.** A human turn is a block in the surface colour, aligned right, at most two thirds of the measure, 14px, with its page chip as a tiny muted label beneath it. A Partner turn is plain prose in the reading face and size, no box, no bubble: an answer is read, not received.
3. **One meta line per answer.** Under a Partner turn, one muted 12px line: "Saw tip 761586c · 21:34 · Looked at 2 things ▸". A tip is seven characters; a time is the clock, with the date only when it is not today; the hash and the ISO form live in the Seeing sheet and the tooltip. Nothing else stands between two turns.
4. **One composer, one object.** Everything the human touches is one bordered card at the foot: a slim top row with "Seeing: Project · Designs · 15 records" (a chosen subject shows there with its ×; the row opens the sheet); the field, in the sans face at 15px, one line that grows to six; a footer row with the hint at the left in 12px muted, "Enter to send · Shift+Enter for a new line", and the primary Send at the right, small; Stop takes Send's place while a turn runs. The card, not the drawer, is where the eye goes to act.
5. **Suggestions inside the composer.** With an empty field, the suggested questions sit inside the card between the Seeing row and the field, as quiet outlined pills in one row, 13px, wrapping at phone width; they leave when the human types and return when the field is empty. Never floating in the transcript.
6. **The working line.** While a turn runs, the answer's place holds one italic muted line with what the Partner is doing now, and a quiet pulse that reduced-motion turns off. The Looked list appears only when the turn is done.
7. **Room to read.** The drawer opens to 40% of the work area by default, remembered per viewer; when the transcript is scrolled up and a new turn arrives, a small "Latest ↓" pill offers the bottom. The header stays as it is.
8. **Rhythm.** 20px between turns, 6px between an answer and its meta line, 12px inside the composer card, 8px between pills. Light and dark both right; every colour a token.

## Not in this slice

Behaviour of any kind: what is captured, sent, looked at or linked stays exactly as g1-s28 and g1-s29 built it.
