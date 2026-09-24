# g1-s34 New goal, one screen

- Kind: design
- Id: 01M3926GXBJPGZR3EWREV8CNGG
- Status: accepted
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-24, Claude on Fable, on Wido's review of the New goal sheet: "This looks awful. We need to do much better from a UX perspective." The bands of empty space were a stylesheet bug, fixed the same hour (`3e4567000`). What remains is the design: an eyebrow in the engine's words, a warning banner inside the form, four risk questions in the engine's vocabulary in front of every goal, placeholders doing the work of hints, and the one button that matters below the fold. Presentation and form design only; the act and its route are unchanged.

## The principle

The common case is three answers: what the goal is called, what done looks like, and what to take on first. Everything else is disclosed when it is wanted, and the form is one screen at desk height.

## The sheet

1. **Head.** "New goal", with the eyebrow "Opens in To Do, not yet approved" in place of "INTAKE → TO DO". No banner: an act that needs a human opens the sign-in sheet by itself, as every act does now. If the seat cannot open goals at all (no ledger read), one muted line says so under the head and the button is disabled.
2. **Three fields, then the button.** *Id*: a short name every seat will use, with an example in the placeholder ("e.g. refund-worker") and a live suggestion made from the intent's first words as a slug while the id is untouched; checked live against the ledger's rule and its existing ids, the refusal in one line under the field. *Intent*: "what done looks like", a field of two lines that grows; the hint, in 12px muted under the field, "One line saying what done looks like: the outcome, not the work." *First next step*: two lines that grow; hint "What to take on, and what is free. A different machine has to be able to claim this without asking you what you meant." The rule "A goal is named by one id…" becomes the Id field's hint, not a footer.
3. **Risk and tier, disclosed.** A disclosure row closed by default: "Tier 1 · from the four answers below ▸", the tier live from the answers. Open, four rows, each a plain question in the kit's own definition of the score (the builder takes the words from the kit's glossary or doctrine, never invents them) and a 1 · 2 · 3 segmented control with a one-word meaning under each stop; then *Basis*, one line, "why those four answers". The tier is text, derived; the override select exists only inside the disclosure and only when the engine allows one.
4. **More, disclosed.** *Labels* and *Unblocks*, each with its example placeholder and one-line hint.
5. **Foot, always visible.** The primary "Open goal" and "Cancel", pinned to the sheet's foot; "Opening…" while it sends; a refusal from the server in one line above the buttons, in the danger colour, keeping the fields.
6. **Measure.** The sheet 560px wide, 20px padding, 16px between fields, 6px label to field, 6px field to hint; the body scrolls only when the window is shorter than the closed form. Light and dark; every colour a token.

## Not in this slice

The act's route and rules; the other act sheets (approve, priority, record, question), which take the same form design in a following slice once this one has been seen.
