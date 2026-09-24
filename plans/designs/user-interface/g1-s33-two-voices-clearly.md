# g1-s33 Two voices, clearly

- Kind: design
- Id: 01M38Z622SJ4Q3Z3R3RXKY7TFV
- Status: done
- Goals: project-partner
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-24, Claude on Fable, on Wido's ask: "I also would like to see a bit more clearly the difference between my message and the partner message." A presentation slice over g1-s31; not sent to Astra: it changes no behaviour.

## What is wrong

g1-s31 told the two voices apart by shape alone: the human's turn a block aligned right, the Partner's plain prose. Now that the conversation takes the drawer's width the block sits far from the prose it answers, and a short question and a short answer look alike. Shape is one cue, and one is not enough for a reader scanning back through a long conversation.

## The design

One column, and every turn attributed the same way, so a reader never has to infer who spoke.

1. **A header row on every turn.** A small mark, the speaker's name and the time, in one 12px muted row above the text: for the human, a person mark and their name ("Wido" when the seat knows it, "You" otherwise); for the Partner, the rail's own conversation mark and "Project Partner". The time sits at the row's end in the muted mono, the date only when not today. The human turn's page chip joins this row after the time, not beneath the text.
2. **The human's turn is a tinted block.** Its text sits on the surface colour with a small radius and 12px padding, spanning the column; the Partner's answer stays plain prose on the page ground. Colour is the second cue, and it reads at a glance and from a distance.
3. **The Partner's answer carries its mark's colour in a hairline.** A 2px rule in the accent colour at the left of the answer, from the header to the meta line, so an answer is one object even when it is long; the human's block needs none, it is one object already.
4. **Alignment stays one column.** Nothing is right-aligned; the eye reads down one edge and attribution is in the header, not in the position.
5. **In a monospace face** the header reads like a prompt line, which is what a terminal reader expects: the mark, the name, the time.

Dark and light both right; every colour a token; the marks are the lucide icons the rail already uses.

## Not in this slice

Behaviour of any kind; avatars or images; a per-human colour.
