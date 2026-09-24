# g1-s35 A sheet leaves the Partner free

- Kind: design
- Id: 01M392EC50QQZ0NWD44BHWD4QM
- Status: done
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-24, Claude on Fable, on Wido's ask about the New goal sheet: "it's a modal pop up, which means I cannot use the project partner to discuss whatever is in the goal. It would be really nice if the pop up is modal only for the top half and that I would still have access to the project partner." Ruled "do as proposed". Frontend only; the acts and their routes are unchanged.

## The principle

A sheet blocks the page it sits on, so a stray click cannot drag a card while a form is being filled. It has no reason to block the Partner. What is being filled in reaches the Partner only when the human hands it over, as an identified draft.

## The design

1. **Modal for the work area only.** The scrim and the click-blocking cover the work area; the drawer beneath stays live: it opens, resizes and takes questions. Focus is not trapped in the sheet: Tab moves from the sheet into the drawer and back in document order; Escape acts on whichever of the two has focus, closing the sheet from the sheet and the drawer from the drawer. The sheet stays above the work area and never above the drawer. On the focused conversation page, where there is no drawer, a sheet is modal for the page as before.
2. **The sheet is a subject.** "Ask about this" in every sheet's head, the same affordance every object has. It attaches the sheet's current fields as a draft chip above the composer, "Draft: New goal · id refund-worker · intent …", removable, with its source named as the sheet; the Partner is given the draft with the next question, marked "a draft the human is filling in, not saved". Nothing about a sheet reaches the Partner without this act. Fields that hold a secret (the sign-in code) are never attached.
3. **The Seeing line says where you are.** While a sheet is open, the line reads "… · New goal sheet open" (or the sheet's name); the capture carries the sheet's name so a message chip returns to the page with that sheet named in its status line, not reopened.
4. **One rule for every sheet**: new goal, approve, priority, new record, new question, the record status, the editor's discard, the font chooser, the Seeing sheet. Sign-in keeps the whole-window modal: a code is typed once and nothing else should be reachable while it is.

## Not in this slice

The Partner filling a sheet's fields: that is the proposal step, where its suggestion lands in the sheet for the human to confirm.
